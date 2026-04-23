package httpapi

import (
	"net/http"

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
}

func NewRouter(taskStore store.TaskStore, events *eventlog.Journal, sched *engine.Scheduler, metrics *observability.Metrics) *Router {
	return &Router{
		store:   taskStore,
		events:  events,
		sched:   sched,
		metrics: metrics,
	}
}

func (r *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", r.handleHealth)
	mux.HandleFunc("GET /metrics", r.handleMetrics)
	mux.HandleFunc("POST /tasks", r.handleSubmit)
	mux.HandleFunc("GET /tasks", r.handleListTasks)
	mux.HandleFunc("GET /tasks/{id}", r.handleGetTask)
	mux.HandleFunc("GET /events/stream", r.handleEventStream)
	return mux
}
