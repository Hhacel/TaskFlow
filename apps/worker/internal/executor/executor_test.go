package executor

import (
	"runtime"
	"testing"
	"time"

	"github.com/hhace/taskflow/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getOSSpecificCommand returns a shell command appropriate for the current OS.
func getOSSpecificCommand(kind string) string {
	switch kind {
	case "echo":
		return "echo hello"
	case "exit_error":
		if runtime.GOOS == "windows" {
			return "exit /b 1"
		}
		return "exit 1"
	case "sleep":
		if runtime.GOOS == "windows" {
			return "ping -n 5 127.0.0.1"
		}
		return "sleep 5"
	default:
		return "echo test"
	}
}

func TestNewTaskExecutor(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Minute)
	require.NotNil(t, executor)
	assert.Equal(t, 5*time.Minute, executor.defaultTimeout)
}

func TestTaskExecutor_Execute_Success(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Second)

	result := executor.Execute(&task.DispatchMessage{
		TaskID:  1,
		Attempt: 1,
		Command: getOSSpecificCommand("echo"),
	})

	assert.True(t, result.Success)
	assert.Empty(t, result.Error)
	assert.Contains(t, result.Output, "hello")
	assert.False(t, result.StartTime.IsZero())
	assert.False(t, result.EndTime.IsZero())
	assert.Equal(t, uint(1), result.TaskID)
	assert.Equal(t, 1, result.Attempt)
}

func TestTaskExecutor_Execute_EmptyCommand(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Second)

	result := executor.Execute(&task.DispatchMessage{TaskID: 2, Attempt: 1, Command: ""})

	assert.False(t, result.Success)
	assert.Equal(t, "empty command", result.Error)
	assert.Empty(t, result.Output)
}

func TestTaskExecutor_Execute_NonExistentCommand(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Second)

	result := executor.Execute(&task.DispatchMessage{TaskID: 3, Attempt: 1, Command: "nonexistentcommand12345"})

	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}

func TestTaskExecutor_Execute_NonZeroExit(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Second)

	result := executor.Execute(&task.DispatchMessage{TaskID: 4, Attempt: 1, Command: getOSSpecificCommand("exit_error")})

	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}

func TestTaskExecutor_Execute_Timeout(t *testing.T) {
	executor := NewTaskExecutor(500 * time.Millisecond)

	result := executor.Execute(&task.DispatchMessage{TaskID: 5, Attempt: 1, Command: getOSSpecificCommand("sleep")})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "timed out")
}

func TestTaskExecutor_Execute_PerTaskTimeoutOverridesDefault(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Minute)
	shortTimeout := 1

	result := executor.Execute(&task.DispatchMessage{
		TaskID:         6,
		Attempt:        1,
		Command:        getOSSpecificCommand("sleep"),
		TimeoutSeconds: &shortTimeout,
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "timed out")
}
