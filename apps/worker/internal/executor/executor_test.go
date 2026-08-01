package executor

import (
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hhace/taskflow/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTaskExecutor(t *testing.T) {
	tests := []struct {
		name            string
		timeout         time.Duration
		expectedTimeout time.Duration
	}{
		{
			name:            "creates executor with 5 minute timeout",
			timeout:         5 * time.Minute,
			expectedTimeout: 5 * time.Minute,
		},
		{
			name:            "creates executor with 10 second timeout",
			timeout:         10 * time.Second,
			expectedTimeout: 10 * time.Second,
		},
		{
			name:            "creates executor with 1 hour timeout",
			timeout:         1 * time.Hour,
			expectedTimeout: 1 * time.Hour,
		},
		{
			name:            "creates executor with zero timeout",
			timeout:         0,
			expectedTimeout: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewTaskExecutor(tt.timeout)

			require.NotNil(t, executor)
			assert.Equal(t, tt.expectedTimeout, executor.timeout)
		})
	}
}

func TestTaskExecutor_Execute(t *testing.T) {
	tests := []struct {
		name           string
		timeout        time.Duration
		task           *task.Task
		validateResult func(t *testing.T, result *task.TaskExecutionResult)
	}{
		{
			name:    "executes simple echo command successfully",
			timeout: 5 * time.Second,
			task: &task.Task{
				ID:      uuid.New(),
				Command: getOSSpecificCommand("echo"),
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				assert.True(t, result.Success)
				assert.Empty(t, result.Error)
				assert.Contains(t, result.Output, "hello")
				assert.False(t, result.StartTime.IsZero())
				assert.False(t, result.EndTime.IsZero())
				assert.True(t, result.EndTime.After(result.StartTime) || result.EndTime.Equal(result.StartTime))
			},
		},
		{
			name:    "executes single word command",
			timeout: 5 * time.Second,
			task: &task.Task{
				ID:      uuid.New(),
				Command: getOSSpecificCommand("single"),
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				assert.True(t, result.Success)
				assert.Empty(t, result.Error)
				assert.NotEmpty(t, result.Output)
			},
		},
		{
			name:    "handles empty command with error",
			timeout: 5 * time.Second,
			task: &task.Task{
				ID:      uuid.New(),
				Command: "",
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				assert.False(t, result.Success)
				assert.Equal(t, "empty command", result.Error)
				assert.Empty(t, result.Output)
				assert.False(t, result.StartTime.IsZero())
				assert.False(t, result.EndTime.IsZero())
			},
		},
		{
			name:    "handles non-existent command with error",
			timeout: 5 * time.Second,
			task: &task.Task{
				ID:      uuid.New(),
				Command: "nonexistentcommand12345",
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				assert.False(t, result.Success)
				assert.NotEmpty(t, result.Error)
				assert.False(t, result.StartTime.IsZero())
				assert.False(t, result.EndTime.IsZero())
			},
		},
		{
			name:    "handles command that exits with non-zero status",
			timeout: 5 * time.Second,
			task: &task.Task{
				ID:      uuid.New(),
				Command: getOSSpecificCommand("exit_error"),
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				assert.False(t, result.Success)
				assert.NotEmpty(t, result.Error)
				assert.False(t, result.StartTime.IsZero())
				assert.False(t, result.EndTime.IsZero())
			},
		},
		{
			name:    "handles command timeout",
			timeout: 100 * time.Millisecond,
			task: &task.Task{
				ID:      uuid.New(),
				Command: getOSSpecificCommand("sleep"),
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				assert.False(t, result.Success)
				assert.Contains(t, result.Error, "timed out")
				assert.False(t, result.StartTime.IsZero())
				assert.False(t, result.EndTime.IsZero())
			},
		},
		{
			name:    "captures command output",
			timeout: 5 * time.Second,
			task: &task.Task{
				ID:      uuid.New(),
				Command: getOSSpecificCommand("output"),
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				assert.True(t, result.Success)
				assert.Contains(t, result.Output, "test output")
				assert.Empty(t, result.Error)
			},
		},
		{
			name:    "sets correct task ID in result",
			timeout: 5 * time.Second,
			task: &task.Task{
				ID:      uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				Command: getOSSpecificCommand("id_test"),
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				assert.Equal(t, uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"), result.TaskID)
				assert.True(t, result.Success)
			},
		},
		{
			name:    "handles command with multiple arguments",
			timeout: 5 * time.Second,
			task: &task.Task{
				ID:      uuid.New(),
				Command: getOSSpecificCommand("multi_args"),
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				assert.True(t, result.Success)
				assert.Contains(t, result.Output, "arg1")
				assert.Contains(t, result.Output, "arg2")
				assert.Contains(t, result.Output, "arg3")
			},
		},
		{
			name:    "execution time is within timeout",
			timeout: 5 * time.Second,
			task: &task.Task{
				ID:      uuid.New(),
				Command: getOSSpecificCommand("timing"),
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				duration := result.EndTime.Sub(result.StartTime)
				assert.True(t, duration < 5*time.Second)
				assert.True(t, duration >= 0)
				assert.True(t, result.Success)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewTaskExecutor(tt.timeout)
			result := executor.Execute(tt.task)

			require.NotNil(t, result)
			assert.Equal(t, tt.task.ID, result.TaskID)
			tt.validateResult(t, result)
		})
	}
}

// getOSSpecificCommand returns platform-specific commands for testing
func getOSSpecificCommand(commandType string) string {
	if runtime.GOOS == "windows" {
		switch commandType {
		case "echo":
			return "cmd /C echo hello"
		case "single":
			return "cmd /C echo test"
		case "exit_error":
			return "cmd /C exit 1"
		case "sleep":
			// Sleep for 2 seconds on Windows
			return "powershell -Command Start-Sleep -Seconds 2"
		case "output":
			return "cmd /C echo test output"
		case "id_test":
			return "cmd /C echo id test"
		case "multi_args":
			return "cmd /C echo arg1 arg2 arg3"
		case "timing":
			return "cmd /C echo timing test"
		default:
			return "cmd /C echo default"
		}
	} else {
		// Unix-like systems (Linux, macOS)
		switch commandType {
		case "echo":
			return "echo hello"
		case "single":
			return "echo test"
		case "exit_error":
			return "sh -c 'exit 1'"
		case "sleep":
			// Sleep for 2 seconds on Unix
			return "sleep 2"
		case "output":
			return "echo test output"
		case "id_test":
			return "echo id test"
		case "multi_args":
			return "echo arg1 arg2 arg3"
		case "timing":
			return "echo timing test"
		default:
			return "echo default"
		}
	}
}
