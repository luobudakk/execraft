package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpapi "github.com/jinziqi/execraft/internal/api/http"
	"github.com/jinziqi/execraft/internal/app"
	"github.com/jinziqi/execraft/internal/config"
	"github.com/jinziqi/execraft/internal/domain"
)

func TestHTTPSubmitAndQuery(t *testing.T) {
	cfg := config.Config{
		DataDir:          t.TempDir(),
		MaxWorkers:       2,
		QueueSize:        16,
		SnapshotInterval: time.Second,
		AuthEnabled:      true,
		AuthTokens:       "admin:dev-admin,operator:dev-operator,viewer:dev-viewer",
		TenantRequired:   true,
		TenantDefault:    "default",
	}
	rt, err := app.Bootstrap(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer rt.Stop()
	rt.Start(ctx)

	server := httptest.NewServer(httpapi.NewRouter(cfg, rt.Store, rt.Journal, rt.Scheduler, rt.Metrics, rt.Executors.Kinds(), rt.Executors.Matrix()).Handler())
	defer server.Close()

	graph := domain.TaskGraph{
		Tasks: []domain.TaskSpec{
			{ID: "a", Kind: "echo", Input: []byte(`{"msg":"ok"}`)},
			{ID: "b", Kind: "sleep", Input: []byte(`{"duration_ms":10}`), DependsOn: []string{"a"}},
		},
	}
	body, _ := json.Marshal(graph)
	req, err := http.NewRequest(http.MethodPost, server.URL+"/tasks", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dev-operator")
	req.Header.Set("x-tenant-id", "tenant-a")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d body=%s", resp.StatusCode, string(raw))
	}
	var accepted struct {
		TaskIDs []string `json:"task_ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&accepted); err != nil {
		t.Fatal(err)
	}
	if len(accepted.TaskIDs) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(accepted.TaskIDs))
	}

	waitTask(t, server.URL, accepted.TaskIDs[1], domain.StatusSuccess)
}

func waitTask(t *testing.T, baseURL, id string, want domain.TaskStatus) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/tasks/"+id, nil)
		req.Header.Set("Authorization", "Bearer dev-viewer")
		req.Header.Set("x-tenant-id", "tenant-a")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var task domain.TaskRecord
		if err := json.NewDecoder(resp.Body).Decode(&task); err == nil && task.Status == want {
			resp.Body.Close()
			return
		}
		resp.Body.Close()
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("task %s did not reach %s", id, want)
}
