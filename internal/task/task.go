package task

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusCreated   TaskStatus = "created"   // Task created, not yet scheduled
	TaskStatusPending   TaskStatus = "pending"   // Task published to NATS, waiting for worker
	TaskStatusCompleted TaskStatus = "completed" // Task completed successfully
	TaskStatusFailed    TaskStatus = "failed"    // Task execution failed
)

// Task represents a task in the system
type Task struct {
	ID        uuid.UUID   `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	Schedule  string      `gorm:"type:varchar(255);not null" json:"schedule"`
	Command   StringArray `gorm:"type:text[]" json:"command"`
	Status    TaskStatus  `gorm:"type:varchar(20);not null;default:'created'" json:"status"`
	CreatedAt time.Time   `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time   `gorm:"autoUpdateTime" json:"updated_at"`
}

// StringArray is a custom type for PostgreSQL text arrays
type StringArray []string

// Scan implements the Scanner interface for database/sql
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return s.scanString(string(v))
	case string:
		return s.scanString(v)
	case pq.StringArray:
		*s = []string(v)
		return nil
	default:
		return fmt.Errorf("cannot scan %T into StringArray", value)
	}
}

// scanString parses PostgreSQL array format: {item1,item2,item3}
func (s *StringArray) scanString(str string) error {
	if str == "{}" || str == "" {
		*s = []string{}
		return nil
	}

	// Remove braces and split by comma
	str = strings.Trim(str, "{}")
	if str == "" {
		*s = []string{}
		return nil
	}

	*s = strings.Split(str, ",")
	return nil
}

// Value implements the driver Valuer interface
func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return pq.Array(s).Value()
}

// TableName specifies the table name for GORM
func (Task) TableName() string {
	return "tasks"
}

// BeforeCreate generates a UUID if not set
func (t *Task) BeforeCreate() error {
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
