package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jinziqi/execraft/internal/domain"
	"github.com/jinziqi/execraft/internal/engine"
	"github.com/jinziqi/execraft/internal/store"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (r *Router) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func (r *Router) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, r.metrics.Snapshot())
}

func (r *Router) handleAIMetrics(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, r.metrics.AISnapshot())
}

func (r *Router) handleSubmit(w http.ResponseWriter, req *http.Request) {
	user, ok := r.authenticate(req, "operator")
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}
	if r.cfg.TenantRequired && user.TenantID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "tenant id is required"})
		return
	}
	if !r.enforceSubmitRate(user.TenantID) {
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "tenant submit quota exceeded"})
		return
	}
	var graph domain.TaskGraph
	if err := json.NewDecoder(req.Body).Decode(&graph); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json"})
		return
	}
	graph.TenantID = user.TenantID
	graph.SubmittedBy = user.Role
	if blocked := r.exceedsActiveQuota(user.TenantID); blocked {
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "tenant active task quota exceeded"})
		return
	}
	runID, taskIDs, err := r.sched.SubmitWithMeta(graph, engine.SubmissionMeta{
		TenantID:    user.TenantID,
		SubmittedBy: user.Role,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"run_id":   runID,
		"task_ids": taskIDs,
		"accepted": len(taskIDs),
	})
}

func (r *Router) handleGetTask(w http.ResponseWriter, req *http.Request) {
	user, ok := r.authenticate(req, "viewer")
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}
	req = withUserContext(req, user)
	id := req.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "id is required"})
		return
	}
	task, ok, err := r.store.Get(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "task not found"})
		return
	}
	if user.TenantID != "" && task.TenantID != "" && task.TenantID != user.TenantID && user.Role != "admin" {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (r *Router) handleListTasks(w http.ResponseWriter, req *http.Request) {
	user, ok := r.authenticate(req, "viewer")
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}
	filter := store.TaskFilter{
		Status: domain.TaskStatus(req.URL.Query().Get("status")),
		Kind:   req.URL.Query().Get("kind"),
		TenantID: user.TenantID,
	}
	if user.Role == "admin" {
		filter.TenantID = req.URL.Query().Get("tenant_id")
	}
	tasks, err := r.store.List(filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": tasks,
		"total": len(tasks),
	})
}

func (r *Router) handleTools(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"items": r.tools,
		"total": len(r.tools),
	})
}

func (r *Router) handleToolMatrix(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, r.matrix)
}

func (r *Router) handleAlerts(w http.ResponseWriter, req *http.Request) {
	user, ok := r.authenticate(req, "admin")
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}
	events, _, err := r.events.ListSince(0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	items := make([]domain.RuntimeEvent, 0, 16)
	for _, ev := range events {
		if ev.Type == domain.EventSLOAlert {
			items = append(items, ev)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
		"role":  user.Role,
	})
}

func (r *Router) exceedsActiveQuota(tenant string) bool {
	if r.cfg.TenantQuotaActive <= 0 {
		return false
	}
	tasks, err := r.store.List(store.TaskFilter{TenantID: tenant})
	if err != nil {
		return false
	}
	active := 0
	for _, t := range tasks {
		if t.Status == domain.StatusPending || t.Status == domain.StatusQueued || t.Status == domain.StatusRunning {
			active++
		}
	}
	return active >= r.cfg.TenantQuotaActive
}
