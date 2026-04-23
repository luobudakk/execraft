package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	mu   sync.RWMutex
	impl map[string]TaskExecutor
}

func NewRegistry() *Registry {
	return &Registry{impl: map[string]TaskExecutor{}}
}

func (r *Registry) Register(exec TaskExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.impl[exec.Kind()] = exec
}

func (r *Registry) Get(kind string) (TaskExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	exec, ok := r.impl[kind]
	if !ok {
		return nil, fmt.Errorf("unknown task kind: %s", kind)
	}
	return exec, nil
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
