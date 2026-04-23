package eventlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/jinziqi/execraft/internal/domain"
)

type Snapshot struct {
	CreatedAt time.Time                    `json:"created_at"`
	Tasks     map[string]domain.TaskRecord `json:"tasks"`
}

func SnapshotPath(dataDir string) string {
	return filepath.Join(dataDir, "snapshot.json")
}

func SaveSnapshot(dataDir string, tasks map[string]domain.TaskRecord) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	file := SnapshotPath(dataDir)
	tmp := file + ".tmp"
	payload, err := json.MarshalIndent(Snapshot{
		CreatedAt: time.Now().UTC(),
		Tasks:     tasks,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

func LoadSnapshot(dataDir string) (Snapshot, error) {
	var snap Snapshot
	file := SnapshotPath(dataDir)
	raw, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{Tasks: map[string]domain.TaskRecord{}}, nil
		}
		return snap, err
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		return snap, err
	}
	if snap.Tasks == nil {
		snap.Tasks = map[string]domain.TaskRecord{}
	}
	return snap, nil
}
