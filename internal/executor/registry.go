package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jinziqi/execraft/internal/domain"
)

type Result struct {
	Output json.RawMessage
	Err    error
}

type TaskExecutor interface {
	Kind() string
	Execute(ctx context.Context, task domain.TaskSpec) Result
}

type Registry struct {
	mu   sync.RWMutex
	impl map[string]TaskExecutor
}

func NewRegistry() *Registry {
	return &Registry{impl: map[string]TaskExecutor{}}
}

func (r *Registry) Register(exec TaskExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.impl[exec.Kind()] = exec
}

func (r *Registry) Get(kind string) (TaskExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	exec, ok := r.impl[kind]
	if !ok {
		return nil, fmt.Errorf("unknown task kind: %s", kind)
	}
	return exec, nil
}
