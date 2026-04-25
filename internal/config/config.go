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
	LLMProvider      string
	LLMModel         string
	LLMBaseURL       string
	LLMAPIKey        string
	CodebotBaseURL   string
	CodebotToken     string
	CodebotTimeoutMS int
	CodebotWebhook   string
	AuthEnabled      bool
	AuthTokens       string
	TenantRequired   bool
	TenantDefault    string
	TenantQuotaActive int
	TenantSubmitPerMinute int
	SLOCheckInterval time.Duration
	SLOFailureRatePct int
	SLOLatencyMS      int64
	SLOAlertWebhook   string
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
		LLMProvider:      envOrDefault("EXECRAFT_LLM_PROVIDER", "mock"),
		LLMModel:         envOrDefault("EXECRAFT_LLM_MODEL", "gpt-4o-mini"),
		LLMBaseURL:       envOrDefault("EXECRAFT_LLM_BASE_URL", ""),
		LLMAPIKey:        envOrDefault("EXECRAFT_LLM_API_KEY", ""),
		CodebotBaseURL:   envOrDefault("EXECRAFT_CODEBOT_BASE_URL", "http://localhost:8711"),
		CodebotToken:     envOrDefault("EXECRAFT_CODEBOT_TOKEN", "dev-token"),
		CodebotTimeoutMS: envIntOrDefault("EXECRAFT_CODEBOT_TIMEOUT_MS", 120000),
		CodebotWebhook:   envOrDefault("EXECRAFT_CODEBOT_WEBHOOK", ""),
		AuthEnabled:      envBoolOrDefault("EXECRAFT_AUTH_ENABLED", true),
		AuthTokens:       envOrDefault("EXECRAFT_AUTH_TOKENS", "admin:dev-admin,operator:dev-operator,viewer:dev-viewer"),
		TenantRequired:   envBoolOrDefault("EXECRAFT_TENANT_REQUIRED", true),
		TenantDefault:    envOrDefault("EXECRAFT_TENANT_DEFAULT", "default"),
		TenantQuotaActive: envIntOrDefault("EXECRAFT_TENANT_QUOTA_ACTIVE", 100),
		TenantSubmitPerMinute: envIntOrDefault("EXECRAFT_TENANT_SUBMIT_PER_MINUTE", 240),
		SLOCheckInterval: time.Duration(envIntOrDefault("EXECRAFT_SLO_CHECK_SEC", 30)) * time.Second,
		SLOFailureRatePct: envIntOrDefault("EXECRAFT_SLO_FAILURE_RATE_PCT", 20),
		SLOLatencyMS:     int64(envIntOrDefault("EXECRAFT_SLO_LATENCY_MS", 8000)),
		SLOAlertWebhook:  envOrDefault("EXECRAFT_SLO_ALERT_WEBHOOK", ""),
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
	flag.StringVar(&cfg.LLMProvider, "llm-provider", cfg.LLMProvider, "llm provider: mock|openai_compat|ollama")
	flag.StringVar(&cfg.LLMModel, "llm-model", cfg.LLMModel, "llm model")
	flag.StringVar(&cfg.LLMBaseURL, "llm-base-url", cfg.LLMBaseURL, "llm base url")
	flag.StringVar(&cfg.LLMAPIKey, "llm-api-key", cfg.LLMAPIKey, "llm api key")
	flag.StringVar(&cfg.CodebotBaseURL, "codebot-base-url", cfg.CodebotBaseURL, "codebot API base url")
	flag.StringVar(&cfg.CodebotToken, "codebot-token", cfg.CodebotToken, "codebot API token")
	flag.IntVar(&cfg.CodebotTimeoutMS, "codebot-timeout-ms", cfg.CodebotTimeoutMS, "codebot wait timeout in ms")
	flag.StringVar(&cfg.CodebotWebhook, "codebot-webhook", cfg.CodebotWebhook, "codebot callback webhook url")
	flag.BoolVar(&cfg.AuthEnabled, "auth-enabled", cfg.AuthEnabled, "enable token auth and RBAC")
	flag.StringVar(&cfg.AuthTokens, "auth-tokens", cfg.AuthTokens, "auth tokens in role:token pairs separated by comma")
	flag.BoolVar(&cfg.TenantRequired, "tenant-required", cfg.TenantRequired, "require x-tenant-id for API requests")
	flag.StringVar(&cfg.TenantDefault, "tenant-default", cfg.TenantDefault, "default tenant id")
	flag.IntVar(&cfg.TenantQuotaActive, "tenant-quota-active", cfg.TenantQuotaActive, "max active tasks per tenant")
	flag.IntVar(&cfg.TenantSubmitPerMinute, "tenant-submit-per-minute", cfg.TenantSubmitPerMinute, "submit quota per tenant per minute")
	flag.DurationVar(&cfg.SLOCheckInterval, "slo-check-interval", cfg.SLOCheckInterval, "slo check interval")
	flag.IntVar(&cfg.SLOFailureRatePct, "slo-failure-rate-pct", cfg.SLOFailureRatePct, "slo failure rate threshold percent")
	flag.Int64Var(&cfg.SLOLatencyMS, "slo-latency-ms", cfg.SLOLatencyMS, "slo average ai latency threshold")
	flag.StringVar(&cfg.SLOAlertWebhook, "slo-alert-webhook", cfg.SLOAlertWebhook, "optional slo alert webhook")
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

func envBoolOrDefault(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
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
