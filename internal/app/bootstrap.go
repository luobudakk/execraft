package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jinziqi/execraft/internal/config"
	"github.com/jinziqi/execraft/internal/engine"
	"github.com/jinziqi/execraft/internal/executor"
	"github.com/jinziqi/execraft/internal/llm"
	"github.com/jinziqi/execraft/internal/observability"
	"github.com/jinziqi/execraft/internal/store"
	"github.com/jinziqi/execraft/internal/store/eventlog"
	"github.com/jinziqi/execraft/internal/store/memory"
	sqlitestore "github.com/jinziqi/execraft/internal/store/sqlite"
)

type Runtime struct {
	Config    config.Config
	Logger    *slog.Logger
	Store     store.TaskStore
	Journal   *eventlog.Journal
	Scheduler *engine.Scheduler
	Metrics   *observability.Metrics
	Executors *executor.Registry
	closeFn   func() error
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func Bootstrap(cfg config.Config, logger *slog.Logger) (*Runtime, error) {
	var taskStore store.TaskStore
	var closeFn func() error
	switch cfg.StoreBackend {
	case "", "memory":
		taskStore = memory.New()
	case "sqlite":
		sqliteStore, err := sqlitestore.Open(cfg.SQLitePath)
		if err != nil {
			return nil, err
		}
		taskStore = sqliteStore
		closeFn = sqliteStore.Close
	default:
		return nil, fmt.Errorf("unknown store backend: %s", cfg.StoreBackend)
	}

	journal, err := eventlog.Open(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	if err := restoreFromSnapshot(cfg.DataDir, taskStore); err != nil {
		return nil, err
	}

	reg := executor.NewRegistry()
	executor.RegisterBuiltins(reg)
	if err := executor.LoadPlugins(reg, cfg.EnabledPlugins); err != nil {
		return nil, err
	}
	metrics := observability.NewMetrics()
	var llmRuntime *llm.Runtime
	var llmRouter *llm.Router
	llmCfg := llm.Config{
		Provider: cfg.LLMProvider,
		Model:    cfg.LLMModel,
		BaseURL:  cfg.LLMBaseURL,
		APIKey:   cfg.LLMAPIKey,
	}
	if rt, err := llm.NewRuntime(llm.Config{
		Provider: llmCfg.Provider,
		Model:    llmCfg.Model,
		BaseURL:  llmCfg.BaseURL,
		APIKey:   llmCfg.APIKey,
	}); err == nil {
		llmRuntime = rt
		llmRouter = llm.NewRouter(llmCfg)
	} else {
		return nil, err
	}
	executor.RegisterAgentExecutors(reg, executor.RuntimeDeps{
		LLM:              llmRuntime,
		LLMRouter:        llmRouter,
		Metrics:          metrics,
		CodebotBaseURL:   cfg.CodebotBaseURL,
		CodebotToken:     cfg.CodebotToken,
		CodebotTimeoutMS: cfg.CodebotTimeoutMS,
		CodebotWebhook:   cfg.CodebotWebhook,
	})
	sched := engine.NewScheduler(taskStore, journal, reg, metrics, cfg.MaxWorkers, cfg.QueueSize, logger)

	return &Runtime{
		Config:    cfg,
		Logger:    logger,
		Store:     taskStore,
		Journal:   journal,
		Scheduler: sched,
		Metrics:   metrics,
		Executors: reg,
		closeFn:   closeFn,
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
	if r.closeFn != nil {
		_ = r.closeFn()
	}
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
