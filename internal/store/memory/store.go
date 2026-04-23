package memory

import (
	"slices"
	"sync"

	"github.com/jinziqi/execraft/internal/domain"
	"github.com/jinziqi/execraft/internal/store"
)

type Store struct {
	mu    sync.RWMutex
	tasks map[string]domain.TaskRecord
}

func New() *Store {
	return &Store{tasks: map[string]domain.TaskRecord{}}
}

func (s *Store) Put(task domain.TaskRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
	return nil
}

func (s *Store) Update(task domain.TaskRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
	return nil
}

func (s *Store) Get(id string) (domain.TaskRecord, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	return task, ok, nil
}

func (s *Store) List(filter store.TaskFilter) ([]domain.TaskRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.TaskRecord, 0, len(s.tasks))
	for _, task := range s.tasks {
		if filter.Status != "" && task.Status != filter.Status {
			continue
		}
		if filter.Kind != "" && task.Kind != filter.Kind {
			continue
		}
		out = append(out, task)
	}
	slices.SortFunc(out, func(a, b domain.TaskRecord) int {
		if a.SubmittedAt.Before(b.SubmittedAt) {
			return -1
		}
		if a.SubmittedAt.After(b.SubmittedAt) {
			return 1
		}
		return 0
	})
	return out, nil
}
