package executor

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/hhace/taskflow/models"
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
func (e *TaskExecutor) Execute(task *models.Task) *models.TaskExecutionResult {
	result := &models.TaskExecutionResult{
		TaskID:    task.ID,
		StartTime: time.Now(),
	}

	slog.Info("Executing task", "taskId", task.ID, "command", task.Command)

	// Validate command
	if len(task.Command) == 0 {
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
	if len(task.Command) == 1 {
		cmd = exec.CommandContext(ctx, task.Command[0])
	} else {
		cmd = exec.CommandContext(ctx, task.Command[0], task.Command[1:]...)
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
			"taskId", task.ID,
			"error", result.Error,
			"output", result.Output,
			"duration", result.EndTime.Sub(result.StartTime))
		return result
	}

	result.Success = true
	slog.Info("Task execution completed",
		"taskId", task.ID,
		"duration", result.EndTime.Sub(result.StartTime))
	return result
}
