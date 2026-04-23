package engine

import (
	"context"
	"log/slog"
	"sync"
)

type workItem struct {
	runID     string
	taskID    string
	attempt   int
	timeoutMS int
}

type workerPool struct {
	queue   chan workItem
	wg      sync.WaitGroup
	workers int
	handle  func(context.Context, workItem)
	logger  *slog.Logger
}

func newWorkerPool(size int, queueSize int, logger *slog.Logger, handle func(context.Context, workItem)) *workerPool {
	return &workerPool{
		queue:   make(chan workItem, queueSize),
		workers: size,
		handle:  handle,
		logger:  logger,
	}
}

func (p *workerPool) Start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case it := <-p.queue:
					p.handle(ctx, it)
				}
			}
		}()
	}
}

func (p *workerPool) Stop() {
	p.wg.Wait()
}

func (p *workerPool) Enqueue(item workItem) bool {
	select {
	case p.queue <- item:
		return true
	default:
		return false
	}
}
