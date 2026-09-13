package executor

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"time"

	"github.com/hhace/taskflow/internal/task"
)

// TaskExecutor handles the execution of dispatched tasks.
type TaskExecutor struct {
	defaultTimeout time.Duration
}

// NewTaskExecutor creates a new task executor with a fallback timeout used
// when a dispatched task does not specify its own.
func NewTaskExecutor(defaultTimeout time.Duration) *TaskExecutor {
	return &TaskExecutor{
		defaultTimeout: defaultTimeout,
	}
}

// Execute runs the dispatched task's command and returns the result message
// to be published back to the Orchestrator.
func (e *TaskExecutor) Execute(msg *task.DispatchMessage) *task.ResultMessage {
	result := &task.ResultMessage{
		TaskID:    msg.TaskID,
		Attempt:   msg.Attempt,
		StartTime: time.Now(),
	}

	slog.Info("Executing task", "taskId", msg.TaskID, "attempt", msg.Attempt, "command", msg.Command)

	if msg.Command == "" {
		result.Success = false
		result.Error = "empty command"
		result.EndTime = time.Now()
		return result
	}

	timeout := e.defaultTimeout
	if msg.TimeoutSeconds != nil {
		timeout = time.Duration(*msg.TimeoutSeconds) * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Prepare command: run via platform shell (cmd on Windows, sh otherwise)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", msg.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", msg.Command)
	}

	output, err := cmd.CombinedOutput()
	result.EndTime = time.Now()
	result.Output = string(output)

	if err != nil {
		result.Success = false
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Sprintf("command timed out after %v", timeout)
		} else {
			result.Error = err.Error()
		}
		slog.Error("Task execution failed",
			"taskId", msg.TaskID,
			"error", result.Error,
			"output", result.Output,
			"duration", result.EndTime.Sub(result.StartTime))
		return result
	}

	result.Success = true
	slog.Info("Task execution completed",
		"taskId", msg.TaskID,
		"duration", result.EndTime.Sub(result.StartTime))
	return result
}
