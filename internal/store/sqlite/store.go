package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/jinziqi/execraft/internal/domain"
	"github.com/jinziqi/execraft/internal/store"
)

type Store struct {
	mu sync.RWMutex
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS execraft_tasks (
  id TEXT PRIMARY KEY,
  payload TEXT NOT NULL
);`); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Put(task domain.TaskRecord) error {
	return s.upsert(task)
}

func (s *Store) Update(task domain.TaskRecord) error {
	return s.upsert(task)
}

func (s *Store) upsert(task domain.TaskRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(task)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO execraft_tasks(id,payload) VALUES(?,?)
ON CONFLICT(id) DO UPDATE SET payload=excluded.payload`, task.ID, string(raw))
	return err
}

func (s *Store) Get(id string) (domain.TaskRecord, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRow(`SELECT payload FROM execraft_tasks WHERE id=?`, id)
	var payload string
	if err := row.Scan(&payload); err != nil {
		if err == sql.ErrNoRows {
			return domain.TaskRecord{}, false, nil
		}
		return domain.TaskRecord{}, false, err
	}
	var task domain.TaskRecord
	if err := json.Unmarshal([]byte(payload), &task); err != nil {
		return domain.TaskRecord{}, false, err
	}
	return task, true, nil
}

func (s *Store) List(filter store.TaskFilter) ([]domain.TaskRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT payload FROM execraft_tasks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.TaskRecord{}
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var task domain.TaskRecord
		if err := json.Unmarshal([]byte(payload), &task); err != nil {
			return nil, fmt.Errorf("decode task payload: %w", err)
		}
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
	return out, rows.Err()
}
