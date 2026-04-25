package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	grpcapi "github.com/jinziqi/execraft/internal/api/grpcserver"
	httpapi "github.com/jinziqi/execraft/internal/api/http"
	"github.com/jinziqi/execraft/internal/app"
	"github.com/jinziqi/execraft/internal/config"
	"github.com/jinziqi/execraft/internal/domain"
	"github.com/jinziqi/execraft/internal/observability"
	"github.com/jinziqi/execraft/internal/store/eventlog"
)

func runServe() error {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	rt, err := app.Bootstrap(cfg, logger)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer rt.Stop()
	rt.Start(ctx)

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: httpapi.NewRouter(cfg, rt.Store, rt.Journal, rt.Scheduler, rt.Metrics, rt.Executors.Kinds(), rt.Executors.Matrix()).Handler(),
	}
	var grpcSrvStop func()
	if cfg.GRPCAddr != "" {
		grpcSrv, lis, err := grpcapi.Start(cfg.GRPCAddr, grpcapi.New(rt.Store, rt.Scheduler, rt.Metrics))
		if err != nil {
			return err
		}
		log.Printf("execraft grpc listening on %s", lis.Addr().String())
		grpcSrvStop = grpcSrv.GracefulStop
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("execraft listening on %s", cfg.HTTPAddr)
		errCh <- server.ListenAndServe()
	}()
	go runSLOMonitor(ctx, cfg, rt.Metrics, rt.Journal)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
		cancel()
		if grpcSrvStop != nil {
			grpcSrvStop()
		}
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}

	shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	return server.Shutdown(shutdownCtx)
}

func runSLOMonitor(ctx context.Context, cfg config.Config, metrics *observability.Metrics, events *eventlog.Journal) {
	ticker := time.NewTicker(cfg.SLOCheckInterval)
	defer ticker.Stop()
	client := &http.Client{Timeout: 8 * time.Second}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			base := metrics.Snapshot()
			submitted := base["submitted"]
			failed := base["failed"]
			failureRate := int64(0)
			if submitted > 0 {
				failureRate = (failed * 100) / submitted
			}
			ai := metrics.AISnapshot()
			avgLatency := toInt64(ai["avg_latency_ms"])
			if failureRate <= int64(cfg.SLOFailureRatePct) && avgLatency <= cfg.SLOLatencyMS {
				continue
			}
			payload := map[string]any{
				"type":         "slo_alert",
				"failure_rate": failureRate,
				"avg_latency_ms": avgLatency,
				"threshold_failure_rate": cfg.SLOFailureRatePct,
				"threshold_latency_ms":   cfg.SLOLatencyMS,
				"timestamp":    time.Now().UTC().Format(time.RFC3339),
			}
			raw, _ := json.Marshal(payload)
			_ = events.Append(domain.RuntimeEvent{
				RunID:     "system",
				TaskID:    "slo-monitor",
				Type:      domain.EventSLOAlert,
				Status:    domain.StatusFailed,
				Attempt:   0,
				Message:   string(raw),
				Timestamp: time.Now().UTC(),
			})
			if strings.TrimSpace(cfg.SLOAlertWebhook) != "" {
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.SLOAlertWebhook, bytes.NewReader(raw))
				if err == nil {
					req.Header.Set("Content-Type", "application/json")
					resp, err := client.Do(req)
					if err == nil {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
					}
				}
			}
		}
	}
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	default:
		return 0
	}
}
