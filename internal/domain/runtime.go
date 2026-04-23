package domain

import "time"

type EventType string

const (
	EventTaskSubmitted EventType = "task_submitted"
	EventTaskQueued    EventType = "task_queued"
	EventTaskStarted   EventType = "task_started"
	EventTaskSucceeded EventType = "task_succeeded"
	EventTaskFailed    EventType = "task_failed"
	EventTaskSkipped   EventType = "task_skipped"
)

type RuntimeEvent struct {
	Offset    int64     `json:"offset"`
	RunID     string    `json:"run_id"`
	TaskID    string    `json:"task_id"`
	Type      EventType `json:"type"`
	Status    TaskStatus`json:"status"`
	Attempt   int       `json:"attempt"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
