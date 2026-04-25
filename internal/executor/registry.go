package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/jinziqi/execraft/internal/domain"
)

type Result struct {
	Output json.RawMessage
	Err    error
}

type TaskExecutor interface {
	Kind() string
	Execute(ctx context.Context, task domain.TaskSpec) Result
}

type Plugin interface {
	Name() string
	Register(reg *Registry) error
}

type Registry struct {
	mu        sync.RWMutex
	impl      map[string]TaskExecutor
	alias     map[string]string
	fallbacks map[string][]string
}

func NewRegistry() *Registry {
	return &Registry{
		impl: map[string]TaskExecutor{},
		alias: map[string]string{
			"shell_command":   "shell",
			"http":            "http_request",
			"http_get":        "http_request",
			"mcp_http":        "http_request",
			"plan":            "llm_plan",
			"code_quality":    "codebot_scan",
			"codebot_quality": "codebot_scan",
		},
		fallbacks: map[string][]string{
			"codebot_scan": {"llm_plan", "http_request"},
			"llm_plan":     {"echo"},
			"mcp_http":     {"http_request"},
		},
	}
}

func (r *Registry) Register(exec TaskExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.impl[exec.Kind()] = exec
}

func (r *Registry) Get(kind string) (TaskExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	origin := kind
	if mapped, ok := r.alias[kind]; ok {
		kind = mapped
	}
	exec, ok := r.impl[kind]
	if ok {
		return exec, nil
	}
	for _, candidate := range r.fallbacks[kind] {
		if fallbackExec, has := r.impl[candidate]; has {
			return fallbackExec, nil
		}
	}
	for _, candidate := range r.fallbacks[origin] {
		if fallbackExec, has := r.impl[candidate]; has {
			return fallbackExec, nil
		}
	}
	return nil, fmt.Errorf("unknown task kind: %s", kind)
}

func (r *Registry) Kinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.impl))
	for k := range r.impl {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (r *Registry) Matrix() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	aliases := map[string]string{}
	for k, v := range r.alias {
		aliases[k] = v
	}
	fallbacks := map[string][]string{}
	for k, v := range r.fallbacks {
		fallbacks[k] = append([]string(nil), v...)
	}
	kinds := make([]string, 0, len(r.impl))
	for k := range r.impl {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return map[string]any{
		"kinds":     kinds,
		"aliases":   aliases,
		"fallbacks": fallbacks,
	}
}

func LoadPlugins(reg *Registry, enabled []string) error {
	available := map[string]Plugin{
		"http-request": HTTPRequestPlugin{},
	}
	for _, name := range enabled {
		plugin, ok := available[name]
		if !ok {
			return fmt.Errorf("unknown plugin: %s", name)
		}
		if err := plugin.Register(reg); err != nil {
			return fmt.Errorf("register plugin %s: %w", plugin.Name(), err)
		}
	}
	return nil
}

type HTTPRequestPlugin struct{}

func (p HTTPRequestPlugin) Name() string { return "http-request" }

func (p HTTPRequestPlugin) Register(reg *Registry) error {
	reg.Register(HTTPRequestExecutor{})
	return nil
}

type HTTPRequestExecutor struct{}

func (h HTTPRequestExecutor) Kind() string { return "http_request" }

func (h HTTPRequestExecutor) Execute(ctx context.Context, task domain.TaskSpec) Result {
	var req struct {
		Method    string          `json:"method"`
		URL       string          `json:"url"`
		Body      json.RawMessage `json:"body"`
		TimeoutMS int             `json:"timeout_ms"`
	}
	if err := json.Unmarshal(task.Input, &req); err != nil {
		return Result{Err: err}
	}
	if req.URL == "" {
		return Result{Err: fmt.Errorf("url is required")}
	}
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	client := &http.Client{Timeout: 10 * time.Second}
	if req.TimeoutMS > 0 {
		client.Timeout = time.Duration(req.TimeoutMS) * time.Millisecond
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return Result{Err: err}
	}
	if len(req.Body) > 0 {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Result{Err: err}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	out, _ := json.Marshal(map[string]any{
		"status_code": resp.StatusCode,
		"body":        string(raw),
	})
	if resp.StatusCode >= 400 {
		return Result{Output: out, Err: fmt.Errorf("http status %d", resp.StatusCode)}
	}
	return Result{Output: out}
}
