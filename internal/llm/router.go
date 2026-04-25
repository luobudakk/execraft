package llm

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type RouteRequest struct {
	PreferredProvider string
	PreferredModel    string
	FallbackProviders []string
	MaxLatencyMS      int64
	MinQuality        int
	Messages          []Message
}

type RouteResult struct {
	Provider   string
	Model      string
	Content    string
	Fallback   bool
	LatencyMS  int64
	ErrorCount int
}

type Router struct {
	factory func(cfg Config) (*Runtime, error)
	baseCfg Config
	mu      sync.Mutex
	stats   map[string]providerStats
}

type providerStats struct {
	Requests      int64
	Failures      int64
	LatencyTotal  int64
	QualityTotal  int64
}

func NewRouter(baseCfg Config) *Router {
	return &Router{
		baseCfg: baseCfg,
		factory: NewRuntime,
		stats:   map[string]providerStats{},
	}
}

func (r *Router) RouteChat(ctx context.Context, req RouteRequest) (RouteResult, error) {
	providers := buildProviderOrder(strings.TrimSpace(req.PreferredProvider), r.baseCfg.Provider, req.FallbackProviders)
	providers = r.rankProviders(providers, req)
	model := strings.TrimSpace(req.PreferredModel)
	if model == "" {
		model = r.baseCfg.Model
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	var lastErr error
	for idx, provider := range providers {
		cfg := r.baseCfg
		cfg.Provider = provider
		cfg.Model = model
		rt, err := r.factory(cfg)
		if err != nil {
			lastErr = err
			continue
		}
		start := time.Now()
		content, err := rt.Chat(ctx, req.Messages)
		latency := time.Since(start).Milliseconds()
		if err != nil {
			r.record(provider, latency, 0, true)
			lastErr = err
			continue
		}
		quality := estimateQuality(provider, content, req.MinQuality)
		r.record(provider, latency, int64(quality), false)
		if req.MaxLatencyMS > 0 && latency > req.MaxLatencyMS && idx < len(providers)-1 {
			lastErr = fmt.Errorf("provider %s exceeded latency budget (%dms)", provider, latency)
			continue
		}
		return RouteResult{
			Provider:   provider,
			Model:      model,
			Content:    content,
			Fallback:   idx > 0,
			LatencyMS:  latency,
			ErrorCount: idx,
		}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no provider available")
	}
	return RouteResult{}, lastErr
}

func (r *Router) rankProviders(providers []string, req RouteRequest) []string {
	type scored struct {
		Provider string
		Score    int64
		Order    int
	}
	scoredProviders := make([]scored, 0, len(providers))
	for idx, p := range providers {
		s := int64(1000 - idx*10)
		stats := r.getStats(p)
		if stats.Requests > 0 {
			avgLatency := stats.LatencyTotal / stats.Requests
			failRate := (stats.Failures * 100) / stats.Requests
			avgQuality := stats.QualityTotal / stats.Requests
			s -= avgLatency / 10
			s -= failRate * 4
			s += avgQuality / 3
		}
		if p == strings.ToLower(strings.TrimSpace(req.PreferredProvider)) {
			s += 40
		}
		scoredProviders = append(scoredProviders, scored{Provider: p, Score: s, Order: idx})
	}
	sort.SliceStable(scoredProviders, func(i, j int) bool {
		if scoredProviders[i].Score == scoredProviders[j].Score {
			return scoredProviders[i].Order < scoredProviders[j].Order
		}
		return scoredProviders[i].Score > scoredProviders[j].Score
	})
	out := make([]string, 0, len(scoredProviders))
	for _, item := range scoredProviders {
		out = append(out, item.Provider)
	}
	return out
}

func (r *Router) record(provider string, latencyMS int64, quality int64, failed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	stats := r.stats[provider]
	stats.Requests++
	stats.LatencyTotal += max(latencyMS, 1)
	if quality > 0 {
		stats.QualityTotal += quality
	}
	if failed {
		stats.Failures++
	}
	r.stats[provider] = stats
}

func (r *Router) getStats(provider string) providerStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stats[provider]
}

func estimateQuality(provider string, content string, minQuality int) int {
	base := 76
	switch provider {
	case "mock":
		base = 65
	case "ollama":
		base = 74
	case "openai_compat":
		base = 82
	}
	if len(strings.TrimSpace(content)) > 180 {
		base += 3
	}
	if minQuality > 0 && base < minQuality {
		return minQuality
	}
	return base
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func buildProviderOrder(preferred string, base string, fallback []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 1+len(fallback)+1)
	push := func(p string) {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	push(preferred)
	push(base)
	for _, p := range fallback {
		push(p)
	}
	if len(out) == 0 {
		push("mock")
	}
	return out
}
