package executor

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/hhace/taskflow/internal/task"
)

// TaskExecutor handles the execution of tasks
type TaskExecutor struct {
	timeout time.Duration
}

// NewTaskExecutor creates a new task executor
func NewTaskExecutor(timeout time.Duration) *TaskExecutor {
	return &TaskExecutor{
		timeout: timeout,
	}
}


// Execute runs the task command and returns the result
func (e *TaskExecutor) Execute(t *task.Task) *task.TaskExecutionResult {
	result := &task.TaskExecutionResult{
		TaskID:    t.ID,
		StartTime: time.Now(),
	}

	slog.Info("Executing task", "taskId", t.ID, "command", t.Command)

	// Validate command
	if len(t.Command) == 0 {
		result.Success = false
		result.Error = "empty command"
		result.EndTime = time.Now()
		return result
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	// Prepare command
	var cmd *exec.Cmd
	if len(t.Command) == 1 {
		cmd = exec.CommandContext(ctx, t.Command[0])
	} else {
		cmd = exec.CommandContext(ctx, t.Command[0], t.Command[1:]...)
	}

	// Execute command and capture output
	output, err := cmd.CombinedOutput()
	result.EndTime = time.Now()
	result.Output = string(output)

	if err != nil {
		result.Success = false
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Sprintf("command timed out after %v", e.timeout)
		} else {
			result.Error = err.Error()
		}
		slog.Error("Task execution failed",
			"taskId", t.ID,
			"error", result.Error,
			"output", result.Output,
			"duration", result.EndTime.Sub(result.StartTime))
		return result
	}

	result.Success = true
	slog.Info("Task execution completed",
		"taskId", t.ID,
		"duration", result.EndTime.Sub(result.StartTime))
	return result
}
