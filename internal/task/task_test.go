package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTask_TableName(t *testing.T) {
	tk := Task{}
	assert.Equal(t, "tasks", tk.TableName())
}

func TestTask_IsValidStatus(t *testing.T) {
	tests := []struct {
		name   string
		status TaskStatus
		want   bool
	}{
		{"pending status is valid", TaskStatusPending, true},
		{"running status is valid", TaskStatusRunning, true},
		{"succeeded status is valid", TaskStatusSucceeded, true},
		{"failed status is valid", TaskStatusFailed, true},
		{"cancelled status is valid", TaskStatusCancelled, true},
		{"invalid status", TaskStatus("invalid"), false},
		{"empty status", TaskStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := Task{Status: tt.status}
			assert.Equal(t, tt.want, tk.IsValidStatus())
		})
	}
}

func TestTask_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name       string
		fromStatus TaskStatus
		toStatus   TaskStatus
		canTransit bool
	}{
		// From Pending
		{"pending -> running", TaskStatusPending, TaskStatusRunning, true},
		{"pending -> cancelled", TaskStatusPending, TaskStatusCancelled, true},
		{"pending -> succeeded", TaskStatusPending, TaskStatusSucceeded, false},
		{"pending -> failed", TaskStatusPending, TaskStatusFailed, false},

		// From Running
		{"running -> succeeded", TaskStatusRunning, TaskStatusSucceeded, true},
		{"running -> failed", TaskStatusRunning, TaskStatusFailed, true},
		{"running -> cancelled", TaskStatusRunning, TaskStatusCancelled, true},
		{"running -> pending", TaskStatusRunning, TaskStatusPending, false},

		// From Failed (retry allowed)
		{"failed -> running", TaskStatusFailed, TaskStatusRunning, true},
		{"failed -> cancelled", TaskStatusFailed, TaskStatusCancelled, true},
		{"failed -> succeeded", TaskStatusFailed, TaskStatusSucceeded, false},
		{"failed -> pending", TaskStatusFailed, TaskStatusPending, false},

		// From terminal states
		{"succeeded -> running", TaskStatusSucceeded, TaskStatusRunning, false},
		{"cancelled -> running", TaskStatusCancelled, TaskStatusRunning, false},

		// Invalid from status
		{"invalid -> pending", TaskStatus("invalid"), TaskStatusPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := Task{Status: tt.fromStatus}
			assert.Equal(t, tt.canTransit, tk.CanTransitionTo(tt.toStatus))
		})
	}
}

func TestTask_UpdateStatus(t *testing.T) {
	tests := []struct {
		name        string
		fromStatus  TaskStatus
		newStatus   TaskStatus
		expectError bool
	}{
		{"valid transition pending to running", TaskStatusPending, TaskStatusRunning, false},
		{"invalid transition pending to succeeded", TaskStatusPending, TaskStatusSucceeded, true},
		{"valid retry from failed to running", TaskStatusFailed, TaskStatusRunning, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := &Task{Status: tt.fromStatus}
			err := tk.UpdateStatus(tt.newStatus)
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, tt.fromStatus, tk.Status)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.newStatus, tk.Status)
			}
		})
	}
}
