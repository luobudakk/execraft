package app

import (
	"github.com/jinziqi/execraft/internal/domain"
	"github.com/jinziqi/execraft/internal/store"
	"github.com/jinziqi/execraft/internal/store/eventlog"
)

func restoreFromSnapshot(dataDir string, taskStore store.TaskStore) error {
	snap, err := eventlog.LoadSnapshot(dataDir)
	if err != nil {
		return err
	}
	for _, task := range snap.Tasks {
		if task.Status == domain.StatusRunning || task.Status == domain.StatusQueued {
			task.Status = domain.StatusPending
		}
		if err := taskStore.Put(task); err != nil {
			return err
		}
	}
	return nil
}

func takeSnapshot(dataDir string, taskStore store.TaskStore) error {
	items, err := taskStore.List(store.TaskFilter{})
	if err != nil {
		return err
	}
	tasks := make(map[string]domain.TaskRecord, len(items))
	for _, item := range items {
		tasks[item.ID] = item
	}
	return eventlog.SaveSnapshot(dataDir, tasks)
}
