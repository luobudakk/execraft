package module

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jinziqi/execraft/internal/domain"
	"github.com/jinziqi/execraft/internal/engine"
	"github.com/jinziqi/execraft/internal/executor"
	"github.com/jinziqi/execraft/internal/observability"
	"github.com/jinziqi/execraft/internal/store/eventlog"
	"github.com/jinziqi/execraft/internal/store/memory"
)

type flakyExec struct {
	calls atomic.Int32
}

func (f *flakyExec) Kind() string { return "flaky" }

func (f *flakyExec) Execute(_ context.Context, _ domain.TaskSpec) executor.Result {
	n := f.calls.Add(1)
	if n == 1 {
		return executor.Result{Err: errors.New("first call fails")}
	}
	return executor.Result{}
}

func TestSchedulerRetryAndSkip(t *testing.T) {
	store := memory.New()
	journal, err := eventlog.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	reg := executor.NewRegistry()
	flaky := &flakyExec{}
	reg.Register(flaky)
	reg.Register(executor.EchoExecutor{})
	metrics := observability.NewMetrics()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := engine.NewScheduler(store, journal, reg, metrics, 2, 16, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	_, ids, err := s.Submit(domain.TaskGraph{
		Tasks: []domain.TaskSpec{
			{ID: "a", Kind: "flaky", Retries: 1},
			{ID: "b", Kind: "echo", DependsOn: []string{"a"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	waitStatus(t, store, ids[1], domain.StatusSuccess)
	waitStatus(t, store, ids[0], domain.StatusSuccess)
	if flaky.calls.Load() != 2 {
		t.Fatalf("expected two attempts, got %d", flaky.calls.Load())
	}
}

func waitStatus(t *testing.T, st *memory.Store, id string, want domain.TaskStatus) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		task, ok, err := st.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if ok && task.Status == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("task %s did not reach %s", id, want)
}
