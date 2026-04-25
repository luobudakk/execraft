package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

type TaskStatus string

const (
	StatusPending TaskStatus = "pending"
	StatusQueued  TaskStatus = "queued"
	StatusRunning TaskStatus = "running"
	StatusSuccess TaskStatus = "success"
	StatusFailed  TaskStatus = "failed"
	StatusSkipped TaskStatus = "skipped"
)

func (s TaskStatus) Terminal() bool {
	return s == StatusSuccess || s == StatusFailed || s == StatusSkipped
}

type TaskSpec struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Input     json.RawMessage `json:"input,omitempty"`
	DependsOn []string        `json:"depends_on,omitempty"`
	TimeoutMS int             `json:"timeout_ms,omitempty"`
	Retries   int             `json:"retries,omitempty"`
}

type TaskGraph struct {
	Tasks      []TaskSpec `json:"tasks"`
	TenantID   string     `json:"tenant_id,omitempty"`
	SubmittedBy string    `json:"submitted_by,omitempty"`
}

type TaskRecord struct {
	ID          string          `json:"id"`
	RunID       string          `json:"run_id"`
	Kind        string          `json:"kind"`
	TenantID    string          `json:"tenant_id,omitempty"`
	SubmittedBy string          `json:"submitted_by,omitempty"`
	Status      TaskStatus      `json:"status"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	DependsOn   []string        `json:"depends_on,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
	SubmittedAt time.Time       `json:"submitted_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
}

func NewTaskRecord(runID string, spec TaskSpec, now time.Time) TaskRecord {
	maxAttempts := spec.Retries + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return TaskRecord{
		ID:          spec.ID,
		RunID:       runID,
		Kind:        spec.Kind,
		Status:      StatusPending,
		Attempt:     0,
		MaxAttempts: maxAttempts,
		DependsOn:   slices.Clone(spec.DependsOn),
		SubmittedAt: now,
		UpdatedAt:   now,
	}
}

func ValidateGraph(g TaskGraph) error {
	if len(g.Tasks) == 0 {
		return errors.New("tasks must not be empty")
	}
	ids := make(map[string]struct{}, len(g.Tasks))
	for _, t := range g.Tasks {
		if t.ID == "" {
			return errors.New("task id is required")
		}
		if t.Kind == "" {
			return fmt.Errorf("task %s kind is required", t.ID)
		}
		if _, ok := ids[t.ID]; ok {
			return fmt.Errorf("duplicate task id: %s", t.ID)
		}
		ids[t.ID] = struct{}{}
	}
	inDegree := make(map[string]int, len(g.Tasks))
	adj := make(map[string][]string, len(g.Tasks))
	for _, t := range g.Tasks {
		for _, dep := range t.DependsOn {
			if dep == t.ID {
				return fmt.Errorf("task %s cannot depend on itself", t.ID)
			}
			if _, ok := ids[dep]; !ok {
				return fmt.Errorf("task %s depends on unknown task %s", t.ID, dep)
			}
			inDegree[t.ID]++
			adj[dep] = append(adj[dep], t.ID)
		}
	}
	queue := make([]string, 0, len(g.Tasks))
	for _, t := range g.Tasks {
		if inDegree[t.ID] == 0 {
			queue = append(queue, t.ID)
		}
	}
	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++
		for _, child := range adj[id] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}
	if visited != len(g.Tasks) {
		return errors.New("task graph contains a cycle")
	}
	return nil
}
