package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcapi "github.com/jinziqi/execraft/internal/api/grpcserver"
	httpapi "github.com/jinziqi/execraft/internal/api/http"
	"github.com/jinziqi/execraft/internal/app"
	"github.com/jinziqi/execraft/internal/config"
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
		Handler: httpapi.NewRouter(rt.Store, rt.Journal, rt.Scheduler, rt.Metrics).Handler(),
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
