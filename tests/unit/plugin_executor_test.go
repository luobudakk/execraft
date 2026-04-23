package unit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jinziqi/execraft/internal/domain"
	"github.com/jinziqi/execraft/internal/executor"
)

func TestLoadPluginsAndHTTPRequestExecutor(t *testing.T) {
	reg := executor.NewRegistry()
	if err := executor.LoadPlugins(reg, []string{"http-request"}); err != nil {
		t.Fatalf("failed to load plugin: %v", err)
	}
	exec, err := reg.Get("http_request")
	if err != nil {
		t.Fatalf("expected http_request executor: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	result := exec.Execute(context.Background(), domain.TaskSpec{
		ID:    "http",
		Kind:  "http_request",
		Input: []byte(`{"url":"` + server.URL + `"}`),
	})
	if result.Err != nil {
		t.Fatalf("expected successful http request, got %v", result.Err)
	}
}

func TestLoadPluginsRejectsUnknown(t *testing.T) {
	reg := executor.NewRegistry()
	if err := executor.LoadPlugins(reg, []string{"missing-plugin"}); err == nil {
		t.Fatal("expected error for unknown plugin")
	}
}
