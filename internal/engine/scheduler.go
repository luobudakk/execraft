package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/jinziqi/execraft/internal/domain"
	"github.com/jinziqi/execraft/internal/executor"
	"github.com/jinziqi/execraft/internal/observability"
	"github.com/jinziqi/execraft/internal/store"
)

type Scheduler struct {
	mu      sync.Mutex
	store   store.TaskStore
	events  store.EventSink
	execs   *executor.Registry
	metrics *observability.Metrics
	logger  *slog.Logger
	pool    *workerPool
	runs    map[string]*runState
}

type runState struct {
	plan       *domain.ExecutionPlan
	remaining  map[string]int
	recordByID map[string]string
	attempts   map[string]int
}

func NewScheduler(taskStore store.TaskStore, eventSink store.EventSink, execs *executor.Registry, metrics *observability.Metrics, workers, queueSize int, logger *slog.Logger) *Scheduler {
	s := &Scheduler{
		store:   taskStore,
		events:  eventSink,
		execs:   execs,
		metrics: metrics,
		logger:  logger,
		runs:    map[string]*runState{},
	}
	s.pool = newWorkerPool(workers, queueSize, logger, s.processWork)
	return s
}

func (s *Scheduler) Start(ctx context.Context) {
	s.pool.Start(ctx)
}

func (s *Scheduler) Submit(graph domain.TaskGraph) (string, []string, error) {
	runID := fmt.Sprintf("run-%d-%04d", time.Now().UnixMilli(), rand.Intn(10000))
	plan, err := domain.BuildPlan(runID, graph)
	if err != nil {
		return "", nil, err
	}
	now := time.Now().UTC()

	state := &runState{
		plan:       plan,
		remaining:  map[string]int{},
		recordByID: map[string]string{},
		attempts:   map[string]int{},
	}
	taskIDs := make([]string, 0, len(plan.Tasks))
	s.mu.Lock()
	s.runs[runID] = state
	s.mu.Unlock()

	for taskID, spec := range plan.Tasks {
		state.remaining[taskID] = plan.InDegree[taskID]
		recordID := runID + ":" + taskID
		state.recordByID[taskID] = recordID
		record := domain.NewTaskRecord(runID, spec, now)
		record.ID = recordID
		if err := s.store.Put(record); err != nil {
			return "", nil, err
		}
		s.metrics.OnSubmitted()
		s.emit(runID, taskID, domain.EventTaskSubmitted, domain.StatusPending, 0, "")
		taskIDs = append(taskIDs, recordID)
	}

	for taskID, deps := range state.remaining {
		if deps == 0 {
			if err := s.queueTask(runID, taskID); err != nil {
				return "", nil, err
			}
		}
	}
	return runID, taskIDs, nil
}

func (s *Scheduler) queueTask(runID, taskID string) error {
	recID, record, err := s.getRecord(runID, taskID)
	if err != nil {
		return err
	}
	record.Status = domain.StatusQueued
	record.UpdatedAt = time.Now().UTC()
	if err := s.store.Update(record); err != nil {
		return err
	}
	s.emit(runID, taskID, domain.EventTaskQueued, domain.StatusQueued, record.Attempt, "")
	ok := s.pool.Enqueue(workItem{
		runID:     runID,
		taskID:    taskID,
		attempt:   record.Attempt + 1,
		timeoutMS: s.timeoutFor(runID, taskID),
	})
	if !ok {
		record.Status = domain.StatusFailed
		record.Error = "queue overloaded"
		now := time.Now().UTC()
		record.UpdatedAt = now
		record.FinishedAt = &now
		_ = s.store.Update(record)
		s.emit(runID, taskID, domain.EventTaskFailed, domain.StatusFailed, record.Attempt, "queue overloaded")
		s.propagateSkip(runID, taskID, "dependency queue overload")
		return fmt.Errorf("queue overloaded for %s", recID)
	}
	return nil
}

func (s *Scheduler) timeoutFor(runID, taskID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	spec := s.runs[runID].plan.Tasks[taskID]
	return spec.TimeoutMS
}

func (s *Scheduler) processWork(baseCtx context.Context, item workItem) {
	recID, record, err := s.getRecord(item.runID, item.taskID)
	if err != nil {
		s.logger.Error("failed to load record", "error", err, "task", item.taskID)
		return
	}
	s.metrics.OnStarted()
	now := time.Now().UTC()
	record.Status = domain.StatusRunning
	record.Attempt = item.attempt
	record.UpdatedAt = now
	record.StartedAt = &now
	_ = s.store.Update(record)
	s.emit(item.runID, item.taskID, domain.EventTaskStarted, domain.StatusRunning, record.Attempt, "")

	s.mu.Lock()
	spec := s.runs[item.runID].plan.Tasks[item.taskID]
	s.runs[item.runID].attempts[item.taskID] = item.attempt
	s.mu.Unlock()

	ctx := baseCtx
	cancel := func() {}
	if item.timeoutMS > 0 {
		ctx, cancel = context.WithTimeout(baseCtx, time.Duration(item.timeoutMS)*time.Millisecond)
	}
	defer cancel()

	exec, err := s.execs.Get(spec.Kind)
	if err != nil {
		s.finalizeFailure(item, record, err.Error())
		return
	}
	result := exec.Execute(ctx, spec)
	if result.Err != nil {
		s.handleFailure(item, record, result.Err.Error())
		return
	}
	record.Status = domain.StatusSuccess
	record.Output = json.RawMessage(result.Output)
	now = time.Now().UTC()
	record.UpdatedAt = now
	record.FinishedAt = &now
	_ = s.store.Update(record)
	s.metrics.OnFinishedSuccess()
	s.emit(item.runID, item.taskID, domain.EventTaskSucceeded, domain.StatusSuccess, record.Attempt, "")
	s.onSuccess(item.runID, item.taskID)
	_ = recID
}

func (s *Scheduler) handleFailure(item workItem, record domain.TaskRecord, reason string) {
	if record.Attempt < record.MaxAttempts {
		delay := RetryDelay(record.Attempt + 1)
		time.Sleep(delay)
		record.Status = domain.StatusQueued
		record.UpdatedAt = time.Now().UTC()
		_ = s.store.Update(record)
		if ok := s.pool.Enqueue(workItem{
			runID:     item.runID,
			taskID:    item.taskID,
			attempt:   record.Attempt + 1,
			timeoutMS: s.timeoutFor(item.runID, item.taskID),
		}); ok {
			s.emit(item.runID, item.taskID, domain.EventTaskQueued, domain.StatusQueued, record.Attempt+1, "retry")
			s.metrics.OnRetry()
			return
		}
	}
	s.finalizeFailure(item, record, reason)
}

func (s *Scheduler) finalizeFailure(item workItem, record domain.TaskRecord, reason string) {
	record.Status = domain.StatusFailed
	record.Error = reason
	now := time.Now().UTC()
	record.UpdatedAt = now
	record.FinishedAt = &now
	_ = s.store.Update(record)
	s.metrics.OnFinishedFailed()
	s.emit(item.runID, item.taskID, domain.EventTaskFailed, domain.StatusFailed, record.Attempt, reason)
	s.propagateSkip(item.runID, item.taskID, "upstream failed")
}

func (s *Scheduler) onSuccess(runID, taskID string) {
	s.mu.Lock()
	state := s.runs[runID]
	children := append([]string(nil), state.plan.Dependents[taskID]...)
	for _, child := range children {
		state.remaining[child]--
	}
	s.mu.Unlock()

	for _, child := range children {
		s.mu.Lock()
		remaining := s.runs[runID].remaining[child]
		s.mu.Unlock()
		if remaining == 0 {
			_ = s.queueTask(runID, child)
		}
	}
}

func (s *Scheduler) propagateSkip(runID, failedTaskID, reason string) {
	s.mu.Lock()
	children := append([]string(nil), s.runs[runID].plan.Dependents[failedTaskID]...)
	s.mu.Unlock()
	for _, child := range children {
		_, rec, err := s.getRecord(runID, child)
		if err != nil {
			continue
		}
		if rec.Status.Terminal() {
			continue
		}
		rec.Status = domain.StatusSkipped
		rec.Error = reason
		now := time.Now().UTC()
		rec.UpdatedAt = now
		rec.FinishedAt = &now
		_ = s.store.Update(rec)
		s.metrics.OnSkipped()
		s.emit(runID, child, domain.EventTaskSkipped, domain.StatusSkipped, rec.Attempt, reason)
		s.propagateSkip(runID, child, reason)
	}
}

func (s *Scheduler) getRecord(runID, taskID string) (string, domain.TaskRecord, error) {
	s.mu.Lock()
	recID := s.runs[runID].recordByID[taskID]
	s.mu.Unlock()
	record, ok, err := s.store.Get(recID)
	if err != nil {
		return "", domain.TaskRecord{}, err
	}
	if !ok {
		return "", domain.TaskRecord{}, fmt.Errorf("task not found: %s", recID)
	}
	return recID, record, nil
}

func (s *Scheduler) emit(runID, taskID string, eventType domain.EventType, status domain.TaskStatus, attempt int, message string) {
	_ = s.events.Append(domain.RuntimeEvent{
		RunID:     runID,
		TaskID:    taskID,
		Type:      eventType,
		Status:    status,
		Attempt:   attempt,
		Message:   message,
		Timestamp: time.Now().UTC(),
	})
}
