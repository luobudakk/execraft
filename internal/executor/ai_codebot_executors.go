package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jinziqi/execraft/internal/domain"
	"github.com/jinziqi/execraft/internal/llm"
	"github.com/jinziqi/execraft/internal/observability"
)

type RuntimeDeps struct {
	LLM              *llm.Runtime
	LLMRouter        *llm.Router
	Metrics          *observability.Metrics
	CodebotBaseURL   string
	CodebotToken     string
	CodebotTimeoutMS int
	CodebotWebhook   string
}

type LLMPlanExecutor struct {
	runtime *llm.Runtime
	router  *llm.Router
	metrics *observability.Metrics
}

func (e LLMPlanExecutor) Kind() string { return "llm_plan" }

func (e LLMPlanExecutor) Execute(ctx context.Context, task domain.TaskSpec) Result {
	if e.runtime == nil {
		return Result{Err: errors.New("llm runtime is not configured")}
	}
	var req struct {
		Objective         string   `json:"objective"`
		Context           string   `json:"context"`
		Constraints       []string `json:"constraints"`
		PreferredProvider string   `json:"preferred_provider"`
		PreferredModel    string   `json:"preferred_model"`
		FallbackProviders []string `json:"fallback_providers"`
		MaxLatencyMS      int64    `json:"max_latency_ms"`
		MinQuality        int      `json:"min_quality"`
	}
	if len(task.Input) > 0 {
		if err := json.Unmarshal(task.Input, &req); err != nil {
			return Result{Err: err}
		}
	}
	if strings.TrimSpace(req.Objective) == "" {
		return Result{Err: errors.New("objective is required")}
	}
	prompt := fmt.Sprintf("Objective: %s\nContext: %s\nConstraints: %v\nReturn concise execution plan as bullet points.", req.Objective, req.Context, req.Constraints)
	messages := []llm.Message{
		{Role: "system", Content: "You are an execution planner for automation DAG tasks."},
		{Role: "user", Content: prompt},
	}
	start := time.Now()
	usedProvider := "runtime"
	usedModel := ""
	fallbackUsed := false
	var text string
	var err error
	if e.router != nil {
		routed, routeErr := e.router.RouteChat(ctx, llm.RouteRequest{
			PreferredProvider: req.PreferredProvider,
			PreferredModel:    req.PreferredModel,
			FallbackProviders: req.FallbackProviders,
			MaxLatencyMS:      req.MaxLatencyMS,
			MinQuality:        req.MinQuality,
			Messages:          messages,
		})
		if routeErr != nil {
			err = routeErr
		} else {
			text = routed.Content
			usedProvider = routed.Provider
			usedModel = routed.Model
			fallbackUsed = routed.Fallback
		}
	} else {
		text, err = e.runtime.Chat(ctx, messages)
		usedModel = req.PreferredModel
	}
	latency := time.Since(start).Milliseconds()
	if err != nil {
		if e.metrics != nil {
			e.metrics.OnLLMRequest(latency, fallbackUsed, true, 0, 0)
		}
		return Result{Err: err}
	}
	quality := 70
	if len(req.Constraints) > 0 {
		quality = 75
	}
	if e.metrics != nil {
		e.metrics.OnLLMRequest(latency, fallbackUsed, false, 1, int64(quality))
	}
	out, _ := json.Marshal(map[string]any{
		"type":      "llm_plan",
		"plan":      text,
		"provider":  usedProvider,
		"model":     usedModel,
		"fallback":  fallbackUsed,
		"latencyMs": latency,
		"quality":   quality,
	})
	return Result{Output: out}
}

type CodebotScanExecutor struct {
	baseURL   string
	token     string
	timeoutMS int
	webhook   string
	client    *http.Client
}

func (e CodebotScanExecutor) Kind() string { return "codebot_scan" }

func (e CodebotScanExecutor) Execute(ctx context.Context, task domain.TaskSpec) Result {
	var req struct {
		Target      string `json:"target"`
		Mode        string `json:"mode"`
		CallbackURL string `json:"callback_url"`
	}
	if err := json.Unmarshal(task.Input, &req); err != nil {
		return Result{Err: err}
	}
	if strings.TrimSpace(req.Target) == "" {
		return Result{Err: errors.New("target is required")}
	}
	if req.Mode == "" {
		req.Mode = "scan"
	}
	base := strings.TrimRight(e.baseURL, "/")
	body, _ := json.Marshal(map[string]any{"target": req.Target, "mode": req.Mode})
	subReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/tasks", bytes.NewReader(body))
	if err != nil {
		return Result{Err: err}
	}
	subReq.Header.Set("Content-Type", "application/json")
	subReq.Header.Set("x-codebot-token", e.token)
	subResp, err := e.client.Do(subReq)
	if err != nil {
		return Result{Err: err}
	}
	defer subResp.Body.Close()
	subRaw, _ := io.ReadAll(io.LimitReader(subResp.Body, 1<<20))
	if subResp.StatusCode >= 400 {
		return Result{Err: fmt.Errorf("codebot submit status %d: %s", subResp.StatusCode, string(subRaw))}
	}
	var subParsed struct {
		Ok   bool `json:"ok"`
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(subRaw, &subParsed); err != nil {
		return Result{Err: err}
	}
	taskID := subParsed.Data.ID
	if taskID == "" {
		return Result{Err: errors.New("codebot returned empty task id")}
	}
	timeout := time.Duration(e.timeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	watchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var finalStatus string
	var finalError string
	for {
		select {
		case <-watchCtx.Done():
			return Result{Err: fmt.Errorf("codebot task wait timeout: %s", taskID)}
		case <-time.After(1500 * time.Millisecond):
		}
		getReq, _ := http.NewRequestWithContext(watchCtx, http.MethodGet, base+"/api/tasks/"+taskID, nil)
		getReq.Header.Set("x-codebot-token", e.token)
		getResp, err := e.client.Do(getReq)
		if err != nil {
			return Result{Err: err}
		}
		getRaw, _ := io.ReadAll(io.LimitReader(getResp.Body, 1<<20))
		getResp.Body.Close()
		if getResp.StatusCode >= 400 {
			return Result{Err: fmt.Errorf("codebot get task status %d: %s", getResp.StatusCode, string(getRaw))}
		}
		var getParsed struct {
			Data struct {
				Status         string `json:"status"`
				ResultJSONPath string `json:"resultJsonPath"`
				Error          string `json:"error"`
			} `json:"data"`
		}
		if err := json.Unmarshal(getRaw, &getParsed); err != nil {
			return Result{Err: err}
		}
		finalStatus = getParsed.Data.Status
		finalError = getParsed.Data.Error
		if finalStatus == "succeeded" || finalStatus == "failed" || finalStatus == "cancelled" {
			out, _ := json.Marshal(map[string]any{
				"type":            "codebot_scan",
				"codebot_task_id": taskID,
				"status":          finalStatus,
				"result_json_path": getParsed.Data.ResultJSONPath,
				"error":           finalError,
			})
			callbackURL := strings.TrimSpace(req.CallbackURL)
			if callbackURL == "" {
				callbackURL = strings.TrimSpace(e.webhook)
			}
			if callbackURL != "" {
				_, _ = notifyWebhook(watchCtx, e.client, callbackURL, map[string]any{
					"type":             "codebot_scan_callback",
					"codebot_task_id":  taskID,
					"status":           finalStatus,
					"error":            finalError,
					"result_json_path": getParsed.Data.ResultJSONPath,
					"finished_at":      time.Now().UTC().Format(time.RFC3339),
				})
			}
			if finalStatus != "succeeded" {
				return Result{Output: out, Err: fmt.Errorf("codebot task %s ended with %s: %s", taskID, finalStatus, finalError)}
			}
			return Result{Output: out}
		}
	}
}

type MCPAdapterExecutor struct{}

func (e MCPAdapterExecutor) Kind() string { return "mcp_adapter" }

func (e MCPAdapterExecutor) Execute(ctx context.Context, task domain.TaskSpec) Result {
	var req struct {
		Mode       string            `json:"mode"`
		Endpoint   string            `json:"endpoint"`
		Method     string            `json:"method"`
		Headers    map[string]string `json:"headers"`
		Payload    json.RawMessage   `json:"payload"`
		AuthType   string            `json:"auth_type"`
		AuthToken  string            `json:"auth_token"`
		TimeoutMS  int               `json:"timeout_ms"`
		MaxRetries int               `json:"max_retries"`
		Schema     struct {
			RequiredHeaders []string `json:"required_headers"`
			RequirePayload  bool     `json:"require_payload"`
		} `json:"schema"`
	}
	if err := json.Unmarshal(task.Input, &req); err != nil {
		return Result{Err: err}
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "discover" {
		out, _ := json.Marshal(map[string]any{
			"type": "mcp_adapter_discovery",
			"capabilities": map[string]any{
				"modes":      []string{"discover", "invoke"},
				"auth_types": []string{"none", "bearer"},
				"supports":   []string{"headers", "schema_validation", "retry", "timeout"},
			},
		})
		return Result{Output: out}
	}
	if strings.TrimSpace(req.Endpoint) == "" {
		return Result{Err: errors.New("endpoint is required")}
	}
	if req.Method == "" {
		req.Method = http.MethodPost
	}
	client := &http.Client{Timeout: 12 * time.Second}
	if req.TimeoutMS > 0 {
		client.Timeout = time.Duration(req.TimeoutMS) * time.Millisecond
	}
	retries := req.MaxRetries
	if retries < 0 {
		retries = 0
	}
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if err := validateMCPRequest(req); err != nil {
			return Result{Err: err}
		}
		httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.Endpoint, bytes.NewReader(req.Payload))
		if err != nil {
			return Result{Err: err}
		}
		httpReq.Header.Set("Content-Type", "application/json")
		for k, v := range req.Headers {
			httpReq.Header.Set(k, v)
		}
		if strings.EqualFold(req.AuthType, "bearer") && strings.TrimSpace(req.AuthToken) != "" {
			httpReq.Header.Set("Authorization", "Bearer "+req.AuthToken)
		}
		resp, err := client.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode >= 500 && attempt < retries {
			lastErr = fmt.Errorf("mcp adapter retryable status %d", resp.StatusCode)
			continue
		}
		out, _ := json.Marshal(map[string]any{
			"type":        "mcp_adapter",
			"status_code": resp.StatusCode,
			"body":        string(raw),
			"attempt":     attempt + 1,
		})
		if resp.StatusCode >= 400 {
			return Result{Output: out, Err: fmt.Errorf("mcp adapter status %d", resp.StatusCode)}
		}
		return Result{Output: out}
	}
	if lastErr == nil {
		lastErr = errors.New("mcp adapter request failed")
	}
	return Result{Err: lastErr}
}

func RegisterAgentExecutors(r *Registry, deps RuntimeDeps) {
	if deps.LLM != nil {
		r.Register(LLMPlanExecutor{runtime: deps.LLM, router: deps.LLMRouter, metrics: deps.Metrics})
	}
	r.Register(CodebotScanExecutor{
		baseURL:   deps.CodebotBaseURL,
		token:     deps.CodebotToken,
		timeoutMS: deps.CodebotTimeoutMS,
		webhook:   deps.CodebotWebhook,
		client:    &http.Client{Timeout: 20 * time.Second},
	})
	r.Register(MCPAdapterExecutor{})
}

func validateMCPRequest(req struct {
	Mode       string            `json:"mode"`
	Endpoint   string            `json:"endpoint"`
	Method     string            `json:"method"`
	Headers    map[string]string `json:"headers"`
	Payload    json.RawMessage   `json:"payload"`
	AuthType   string            `json:"auth_type"`
	AuthToken  string            `json:"auth_token"`
	TimeoutMS  int               `json:"timeout_ms"`
	MaxRetries int               `json:"max_retries"`
	Schema     struct {
		RequiredHeaders []string `json:"required_headers"`
		RequirePayload  bool     `json:"require_payload"`
	} `json:"schema"`
}) error {
	for _, k := range req.Schema.RequiredHeaders {
		if strings.TrimSpace(req.Headers[k]) == "" {
			return fmt.Errorf("missing required header: %s", k)
		}
	}
	if req.Schema.RequirePayload && len(req.Payload) == 0 {
		return errors.New("payload is required by schema")
	}
	return nil
}

func notifyWebhook(ctx context.Context, client *http.Client, callbackURL string, payload map[string]any) (int, error) {
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
