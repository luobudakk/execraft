package module

import (
	"testing"
	"time"

	"github.com/jinziqi/execraft/internal/domain"
	sqlitestore "github.com/jinziqi/execraft/internal/store/sqlite"
)

func TestSQLiteStoreCRUD(t *testing.T) {
	store, err := sqlitestore.Open(t.TempDir() + "/execraft.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC()
	task := domain.TaskRecord{
		ID:          "run-1:a",
		RunID:       "run-1",
		Kind:        "echo",
		Status:      domain.StatusPending,
		SubmittedAt: now,
		UpdatedAt:   now,
	}
	if err := store.Put(task); err != nil {
		t.Fatal(err)
	}

	got, ok, err := store.Get(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.ID != task.ID {
		t.Fatalf("expected stored task")
	}

	got.Status = domain.StatusSuccess
	got.UpdatedAt = time.Now().UTC()
	if err := store.Update(got); err != nil {
		t.Fatal(err)
	}

	items, err := store.List(struct {
		Status domain.TaskStatus
		Kind   string
	}{Status: domain.StatusSuccess, Kind: "echo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one filtered item, got %d", len(items))
	}
}
