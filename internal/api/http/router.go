package httpapi

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jinziqi/execraft/internal/config"
	"github.com/jinziqi/execraft/internal/engine"
	"github.com/jinziqi/execraft/internal/observability"
	"github.com/jinziqi/execraft/internal/store"
	"github.com/jinziqi/execraft/internal/store/eventlog"
)

type Router struct {
	store   store.TaskStore
	events  *eventlog.Journal
	sched   *engine.Scheduler
	metrics *observability.Metrics
	tools   []string
	matrix  map[string]any
	cfg     config.Config
	auth    map[string]string
	mu      sync.Mutex
	submitWindow map[string][]time.Time
}

func NewRouter(cfg config.Config, taskStore store.TaskStore, events *eventlog.Journal, sched *engine.Scheduler, metrics *observability.Metrics, tools []string, matrix map[string]any) *Router {
	return &Router{
		store:   taskStore,
		events:  events,
		sched:   sched,
		metrics: metrics,
		tools:   tools,
		matrix:  matrix,
		cfg:     cfg,
		auth:    parseAuthTokens(cfg.AuthTokens),
		submitWindow: map[string][]time.Time{},
	}
}

func (r *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", r.handleHealth)
	mux.HandleFunc("GET /metrics", r.handleMetrics)
	mux.HandleFunc("GET /metrics/ai", r.handleAIMetrics)
	mux.HandleFunc("POST /tasks", r.handleSubmit)
	mux.HandleFunc("GET /tasks", r.handleListTasks)
	mux.HandleFunc("GET /tasks/{id}", r.handleGetTask)
	mux.HandleFunc("GET /events/stream", r.handleEventStream)
	mux.HandleFunc("GET /tools", r.handleTools)
	mux.HandleFunc("GET /tools/matrix", r.handleToolMatrix)
	mux.HandleFunc("GET /alerts/recent", r.handleAlerts)
	return mux
}

type userContext struct {
	Role     string
	TenantID string
	Token    string
}

type userContextKey struct{}

func parseAuthTokens(raw string) map[string]string {
	out := map[string]string{}
	parts := strings.Split(raw, ",")
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		kv := strings.SplitN(item, ":", 2)
		if len(kv) != 2 {
			continue
		}
		role := strings.TrimSpace(kv[0])
		token := strings.TrimSpace(kv[1])
		if role != "" && token != "" {
			out[token] = role
		}
	}
	return out
}

func (r *Router) authenticate(req *http.Request, requiredRole string) (userContext, bool) {
	if !r.cfg.AuthEnabled {
		return userContext{Role: "admin", TenantID: r.resolveTenant(req), Token: "auth-disabled"}, true
	}
	token := strings.TrimSpace(req.Header.Get("x-execraft-token"))
	if token == "" {
		auth := strings.TrimSpace(req.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			token = strings.TrimSpace(auth[7:])
		}
	}
	role := r.auth[token]
	if role == "" {
		return userContext{}, false
	}
	if !hasRole(role, requiredRole) {
		return userContext{}, false
	}
	return userContext{
		Role:     role,
		TenantID: r.resolveTenant(req),
		Token:    token,
	}, true
}

func hasRole(actual, required string) bool {
	rank := map[string]int{"viewer": 1, "operator": 2, "admin": 3}
	return rank[actual] >= rank[required]
}

func (r *Router) resolveTenant(req *http.Request) string {
	tenant := strings.TrimSpace(req.Header.Get("x-tenant-id"))
	if tenant == "" {
		tenant = strings.TrimSpace(req.URL.Query().Get("tenant_id"))
	}
	if tenant == "" {
		tenant = strings.TrimSpace(r.cfg.TenantDefault)
	}
	return tenant
}

func (r *Router) enforceSubmitRate(tenant string) bool {
	limit := r.cfg.TenantSubmitPerMinute
	if limit <= 0 {
		return true
	}
	now := time.Now().UTC()
	cutoff := now.Add(-1 * time.Minute)
	r.mu.Lock()
	defer r.mu.Unlock()
	window := r.submitWindow[tenant]
	filtered := window[:0]
	for _, t := range window {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) >= limit {
		r.submitWindow[tenant] = filtered
		return false
	}
	filtered = append(filtered, now)
	r.submitWindow[tenant] = filtered
	return true
}

func withUserContext(req *http.Request, user userContext) *http.Request {
	ctx := context.WithValue(req.Context(), userContextKey{}, user)
	return req.WithContext(ctx)
}

func getUserContext(req *http.Request) userContext {
	if v, ok := req.Context().Value(userContextKey{}).(userContext); ok {
		return v
	}
	return userContext{Role: "viewer", TenantID: "default"}
}
