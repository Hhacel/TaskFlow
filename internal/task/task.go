package task

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusCreated   TaskStatus = "created"
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// Task represents a task in the system
type Task struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Schedule  string     `gorm:"type:varchar(255);not null" json:"schedule"`
	Command   string     `gorm:"type:text" json:"command"`
	Status    TaskStatus `gorm:"type:varchar(20);not null;default:created" json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Note: command is persisted as a single string (e.g. "echo hello world").

// TableName specifies the table name for GORM
func (Task) TableName() string {
	return "tasks"
}

// BeforeCreate generates a UUID if not set
func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// IsValidStatus checks if the task status is valid
func (t *Task) IsValidStatus() bool {
	switch t.Status {
	case TaskStatusCreated, TaskStatusPending, TaskStatusCompleted, TaskStatusFailed:
		return true
	default:
		return false
	}
}

// CanTransitionTo checks if the task can transition to the given status
func (t *Task) CanTransitionTo(newStatus TaskStatus) bool {
	switch t.Status {
	case TaskStatusCreated:
		return newStatus == TaskStatusPending
	case TaskStatusPending:
		return newStatus == TaskStatusCompleted || newStatus == TaskStatusFailed
	case TaskStatusCompleted, TaskStatusFailed:
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
