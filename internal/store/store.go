package store

import (
	"github.com/jinziqi/execraft/internal/domain"
)

type TaskFilter struct {
	Status domain.TaskStatus
	Kind   string
}

type TaskStore interface {
	Put(task domain.TaskRecord) error
	Update(task domain.TaskRecord) error
	Get(id string) (domain.TaskRecord, bool, error)
	List(filter TaskFilter) ([]domain.TaskRecord, error)
}

type EventSink interface {
	Append(event domain.RuntimeEvent) error
	ListSince(offset int64) ([]domain.RuntimeEvent, int64, error)
}

