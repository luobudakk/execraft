package observability

import (
	"sync/atomic"
)

type Metrics struct {
	submitted int64
	running   int64
	success   int64
	failed    int64
	skipped   int64
}

func NewMetrics() *Metrics { return &Metrics{} }

func (m *Metrics) OnSubmitted() {
	atomic.AddInt64(&m.submitted, 1)
}

func (m *Metrics) OnStarted() {
	atomic.AddInt64(&m.running, 1)
}

func (m *Metrics) OnFinishedSuccess() {
	atomic.AddInt64(&m.running, -1)
	atomic.AddInt64(&m.success, 1)
}

func (m *Metrics) OnFinishedFailed() {
	atomic.AddInt64(&m.running, -1)
	atomic.AddInt64(&m.failed, 1)
}

func (m *Metrics) OnSkipped() {
	atomic.AddInt64(&m.skipped, 1)
}

func (m *Metrics) OnRetry() {
	atomic.AddInt64(&m.running, -1)
}

func (m *Metrics) Snapshot() map[string]int64 {
	return map[string]int64{
		"submitted": atomic.LoadInt64(&m.submitted),
		"running":   atomic.LoadInt64(&m.running),
		"success":   atomic.LoadInt64(&m.success),
		"failed":    atomic.LoadInt64(&m.failed),
		"skipped":   atomic.LoadInt64(&m.skipped),
	}
}
