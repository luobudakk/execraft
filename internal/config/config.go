package config

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr         string
	GRPCAddr         string
	DataDir          string
	StoreBackend     string
	SQLitePath       string
	MaxWorkers       int
	QueueSize        int
	SnapshotInterval time.Duration
	EnabledPlugins   []string
}

func Load() Config {
	cfg := Config{
		HTTPAddr:         envOrDefault("EXECRAFT_HTTP_ADDR", ":8090"),
		GRPCAddr:         envOrDefault("EXECRAFT_GRPC_ADDR", ""),
		DataDir:          envOrDefault("EXECRAFT_DATA_DIR", "data"),
		StoreBackend:     envOrDefault("EXECRAFT_STORE", "memory"),
		SQLitePath:       envOrDefault("EXECRAFT_SQLITE_PATH", "data/execraft.db"),
		MaxWorkers:       envIntOrDefault("EXECRAFT_MAX_WORKERS", 8),
		QueueSize:        envIntOrDefault("EXECRAFT_QUEUE_SIZE", 64),
		SnapshotInterval: time.Duration(envIntOrDefault("EXECRAFT_SNAPSHOT_SEC", 20)) * time.Second,
		EnabledPlugins:   envListOrDefault("EXECRAFT_PLUGINS", []string{"http-request"}),
	}
	pluginsRaw := strings.Join(cfg.EnabledPlugins, ",")

	flag.StringVar(&cfg.HTTPAddr, "http", cfg.HTTPAddr, "http listen address")
	flag.StringVar(&cfg.GRPCAddr, "grpc", cfg.GRPCAddr, "grpc listen address (empty disables grpc)")
	flag.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "directory for snapshots and event log")
	flag.StringVar(&cfg.StoreBackend, "store", cfg.StoreBackend, "task store backend: memory|sqlite")
	flag.StringVar(&cfg.SQLitePath, "sqlite-path", cfg.SQLitePath, "sqlite db path when store=sqlite")
	flag.IntVar(&cfg.MaxWorkers, "workers", cfg.MaxWorkers, "worker pool size")
	flag.IntVar(&cfg.QueueSize, "queue", cfg.QueueSize, "bounded queue size")
	flag.DurationVar(&cfg.SnapshotInterval, "snapshot-interval", cfg.SnapshotInterval, "snapshot interval duration")
	flag.StringVar(&pluginsRaw, "plugins", pluginsRaw, "comma-separated plugin executors to enable")
	flag.Parse()
	cfg.EnabledPlugins = splitCSV(pluginsRaw)
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

func envListOrDefault(key string, fallback []string) []string {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	out := splitCSV(v)
	if len(out) == 0 {
		return fallback
	}
	return out
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
