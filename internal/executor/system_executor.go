package executor

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"time"

	"github.com/jinziqi/execraft/internal/domain"
)

type SleepExecutor struct{}

func (s SleepExecutor) Kind() string { return "sleep" }

func (s SleepExecutor) Execute(ctx context.Context, task domain.TaskSpec) Result {
	var req struct {
		DurationMS int `json:"duration_ms"`
	}
	if len(task.Input) > 0 {
		if err := json.Unmarshal(task.Input, &req); err != nil {
			return Result{Err: err}
		}
	}
	if req.DurationMS < 0 || req.DurationMS > 60000 {
		return Result{Err: errors.New("duration_ms must be between 0 and 60000")}
	}
	timer := time.NewTimer(time.Duration(req.DurationMS) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return Result{Err: ctx.Err()}
	case <-timer.C:
		out, _ := json.Marshal(map[string]any{"slept_ms": req.DurationMS})
		return Result{Output: out}
	}
}

type EchoExecutor struct{}

func (e EchoExecutor) Kind() string { return "echo" }

func (e EchoExecutor) Execute(_ context.Context, task domain.TaskSpec) Result {
	out, _ := json.Marshal(map[string]any{"echo": json.RawMessage(task.Input)})
	return Result{Output: out}
}

type ShellExecutor struct{}

func (s ShellExecutor) Kind() string { return "shell" }

func (s ShellExecutor) Execute(ctx context.Context, task domain.TaskSpec) Result {
	var req struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(task.Input, &req); err != nil {
		return Result{Err: err}
	}
	if req.Command == "" {
		return Result{Err: errors.New("command is required")}
	}
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", req.Command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Result{Err: errors.New(string(output))}
	}
	out, _ := json.Marshal(map[string]any{"output": string(output)})
	return Result{Output: out}
}

func RegisterBuiltins(r *Registry) {
	r.Register(SleepExecutor{})
	r.Register(EchoExecutor{})
	r.Register(ShellExecutor{})
}
