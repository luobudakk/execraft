package eventlog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/jinziqi/execraft/internal/domain"
)

type Journal struct {
	mu         sync.RWMutex
	path       string
	events     []domain.RuntimeEvent
	nextOffset int64
}

func Open(dataDir string) (*Journal, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	j := &Journal{path: filepath.Join(dataDir, "events.log"), events: []domain.RuntimeEvent{}}
	if err := j.load(); err != nil {
		return nil, err
	}
	return j, nil
}

func (j *Journal) load() error {
	file, err := os.OpenFile(j.path, os.O_CREATE|os.O_RDONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var maxOffset int64
	for scanner.Scan() {
		var ev domain.RuntimeEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		j.events = append(j.events, ev)
		if ev.Offset > maxOffset {
			maxOffset = ev.Offset
		}
	}
	j.nextOffset = maxOffset + 1
	return scanner.Err()
}

func (j *Journal) Append(event domain.RuntimeEvent) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	event.Offset = j.nextOffset
	j.nextOffset++
	j.events = append(j.events, event)

	file, err := os.OpenFile(j.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	line, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

func (j *Journal) ListSince(offset int64) ([]domain.RuntimeEvent, int64, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	out := make([]domain.RuntimeEvent, 0, len(j.events))
	var latest int64 = offset
	for _, ev := range j.events {
		if ev.Offset > offset {
			out = append(out, ev)
			latest = ev.Offset
		}
	}
	return out, latest, nil
}
