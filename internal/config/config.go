package config

import (
	"flag"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr         string
	DataDir          string
	MaxWorkers       int
	QueueSize        int
	SnapshotInterval time.Duration
}

func Load() Config {
	cfg := Config{
		HTTPAddr:         envOrDefault("EXECRAFT_HTTP_ADDR", ":8090"),
		DataDir:          envOrDefault("EXECRAFT_DATA_DIR", "data"),
		MaxWorkers:       envIntOrDefault("EXECRAFT_MAX_WORKERS", 8),
		QueueSize:        envIntOrDefault("EXECRAFT_QUEUE_SIZE", 64),
		SnapshotInterval: time.Duration(envIntOrDefault("EXECRAFT_SNAPSHOT_SEC", 20)) * time.Second,
	}

	flag.StringVar(&cfg.HTTPAddr, "http", cfg.HTTPAddr, "http listen address")
	flag.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "directory for snapshots and event log")
	flag.IntVar(&cfg.MaxWorkers, "workers", cfg.MaxWorkers, "worker pool size")
	flag.IntVar(&cfg.QueueSize, "queue", cfg.QueueSize, "bounded queue size")
	flag.DurationVar(&cfg.SnapshotInterval, "snapshot-interval", cfg.SnapshotInterval, "snapshot interval duration")
	flag.Parse()
	return cfg
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
