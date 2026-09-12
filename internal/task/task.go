package task

import (
	"fmt"
	"time"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "PENDING"
	TaskStatusRunning   TaskStatus = "RUNNING"
	TaskStatusSucceeded TaskStatus = "SUCCEEDED"
	TaskStatusFailed    TaskStatus = "FAILED"
	TaskStatusCancelled TaskStatus = "CANCELLED"
)

// Task represents a single unit of work belonging to a Workflow.
type Task struct {
	ID         uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	WorkflowID uint       `gorm:"not null;index" json:"workflow_id"`
	Name       string     `gorm:"type:varchar(255);not null" json:"name"`
	Command    string     `gorm:"type:text;not null" json:"command"`
	Status     TaskStatus `gorm:"type:varchar(255);not null;default:PENDING" json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Timeout    *int       `gorm:"column:timeout" json:"timeout"` // timeout in seconds, nullable
}

// TableName specifies the table name for GORM
func (Task) TableName() string {
	return "tasks"
}

// IsValidStatus checks if the task status is valid
func (t *Task) IsValidStatus() bool {
	switch t.Status {
	case TaskStatusPending, TaskStatusRunning, TaskStatusSucceeded, TaskStatusFailed, TaskStatusCancelled:
		return true
	default:
		return false
	}
}

// CanTransitionTo checks if the task can transition to the given status
func (t *Task) CanTransitionTo(newStatus TaskStatus) bool {
	switch t.Status {
	case TaskStatusPending:
		return newStatus == TaskStatusRunning || newStatus == TaskStatusCancelled
	case TaskStatusRunning:
		return newStatus == TaskStatusSucceeded || newStatus == TaskStatusFailed || newStatus == TaskStatusCancelled
	case TaskStatusFailed:
		// Retry: a failed task may be re-run, which brings it back to RUNNING.
		return newStatus == TaskStatusRunning || newStatus == TaskStatusCancelled
	case TaskStatusSucceeded, TaskStatusCancelled:
		return false // Terminal states
	default:
		return false
	}
}

// UpdateStatus updates the task status with validation
// Note: This method only validates the transition. Use repository.UpdateTaskStatus for persistence.
func (t *Task) UpdateStatus(newStatus TaskStatus) error {
	if !t.CanTransitionTo(newStatus) {
		return fmt.Errorf("cannot transition from %s to %s", t.Status, newStatus)
	}

	t.Status = newStatus
	return nil
}
