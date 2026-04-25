package observability

import (
	"sync"
	"sync/atomic"
)

type Metrics struct {
	submitted int64
	running   int64
	success   int64
	failed    int64
	skipped   int64

	mu               sync.Mutex
	llmRequests      int64
	llmFallbacks     int64
	llmErrors        int64
	llmLatencyTotal  int64
	llmLatencyMax    int64
	llmCostMilliUSD  int64
	qualityScoreSum  int64
	qualityScoreHits int64
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

func (m *Metrics) OnLLMRequest(latencyMS int64, fallback bool, err bool, costMilliUSD int64, qualityScore int64) {
	atomic.AddInt64(&m.llmRequests, 1)
	if fallback {
		atomic.AddInt64(&m.llmFallbacks, 1)
	}
	if err {
		atomic.AddInt64(&m.llmErrors, 1)
	}
	if latencyMS > 0 {
		atomic.AddInt64(&m.llmLatencyTotal, latencyMS)
		for {
			current := atomic.LoadInt64(&m.llmLatencyMax)
			if latencyMS <= current {
				break
			}
			if atomic.CompareAndSwapInt64(&m.llmLatencyMax, current, latencyMS) {
				break
			}
		}
	}
	if costMilliUSD > 0 {
		atomic.AddInt64(&m.llmCostMilliUSD, costMilliUSD)
	}
	if qualityScore > 0 {
		atomic.AddInt64(&m.qualityScoreSum, qualityScore)
		atomic.AddInt64(&m.qualityScoreHits, 1)
	}
}

func (m *Metrics) AISnapshot() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	requests := atomic.LoadInt64(&m.llmRequests)
	totalLatency := atomic.LoadInt64(&m.llmLatencyTotal)
	var avgLatency int64
	if requests > 0 {
		avgLatency = totalLatency / requests
	}
	qualityHits := atomic.LoadInt64(&m.qualityScoreHits)
	var avgQuality int64
	if qualityHits > 0 {
		avgQuality = atomic.LoadInt64(&m.qualityScoreSum) / qualityHits
	}
	return map[string]any{
		"requests":         requests,
		"fallbacks":        atomic.LoadInt64(&m.llmFallbacks),
		"errors":           atomic.LoadInt64(&m.llmErrors),
		"avg_latency_ms":   avgLatency,
		"max_latency_ms":   atomic.LoadInt64(&m.llmLatencyMax),
		"cost_milli_usd":   atomic.LoadInt64(&m.llmCostMilliUSD),
		"avg_quality_score": avgQuality,
	}
}
