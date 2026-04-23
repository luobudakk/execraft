package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jinziqi/execraft/internal/domain"
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

func (r *Router) handleSubmit(w http.ResponseWriter, req *http.Request) {
	var graph domain.TaskGraph
	if err := json.NewDecoder(req.Body).Decode(&graph); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json"})
		return
	}
	runID, taskIDs, err := r.sched.Submit(graph)
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
	writeJSON(w, http.StatusOK, task)
}

func (r *Router) handleListTasks(w http.ResponseWriter, req *http.Request) {
	filter := store.TaskFilter{
		Status: domain.TaskStatus(req.URL.Query().Get("status")),
		Kind:   req.URL.Query().Get("kind"),
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
