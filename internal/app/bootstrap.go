package app

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jinziqi/execraft/internal/config"
	"github.com/jinziqi/execraft/internal/engine"
	"github.com/jinziqi/execraft/internal/executor"
	"github.com/jinziqi/execraft/internal/observability"
	"github.com/jinziqi/execraft/internal/store"
	"github.com/jinziqi/execraft/internal/store/eventlog"
	"github.com/jinziqi/execraft/internal/store/memory"
)

type Runtime struct {
	Config    config.Config
	Logger    *slog.Logger
	Store     store.TaskStore
	Journal   *eventlog.Journal
	Scheduler *engine.Scheduler
	Metrics   *observability.Metrics
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func Bootstrap(cfg config.Config, logger *slog.Logger) (*Runtime, error) {
	mem := memory.New()
	journal, err := eventlog.Open(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	if err := restoreFromSnapshot(cfg.DataDir, mem); err != nil {
		return nil, err
	}

	reg := executor.NewRegistry()
	executor.RegisterBuiltins(reg)
	metrics := observability.NewMetrics()
	sched := engine.NewScheduler(mem, journal, reg, metrics, cfg.MaxWorkers, cfg.QueueSize, logger)

	return &Runtime{
		Config:    cfg,
		Logger:    logger,
		Store:     mem,
		Journal:   journal,
		Scheduler: sched,
		Metrics:   metrics,
	}, nil
}

func (r *Runtime) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	r.Scheduler.Start(ctx)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.snapshotLoop(ctx)
	}()
}

func (r *Runtime) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
}

func (r *Runtime) snapshotLoop(ctx context.Context) {
	ticker := time.NewTicker(r.Config.SnapshotInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = takeSnapshot(r.Config.DataDir, r.Store)
			return
		case <-ticker.C:
			_ = takeSnapshot(r.Config.DataDir, r.Store)
		}
	}
}

