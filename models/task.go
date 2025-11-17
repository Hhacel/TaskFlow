package models

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// Task represents a task in the system
type Task struct {
	ID        uuid.UUID    `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	Schedule  string       `gorm:"type:varchar(255);not null" json:"schedule"`
	Command   StringArray  `gorm:"type:text[]" json:"command"`
	Status    TaskStatus   `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	CreatedAt time.Time    `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time    `gorm:"autoUpdateTime" json:"updated_at"`
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
		// Parse PostgreSQL array format: {item1,item2,item3}
		str := string(v)
		if str == "{}" {
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
	case pq.StringArray:
		*s = []string(v)
		return nil
	default:
		return fmt.Errorf("cannot scan %T into StringArray", value)
	}
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

// BeforeCreate is a GORM hook that runs before creating a task
func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// IsValidStatus checks if the task status is valid
func (t *Task) IsValidStatus() bool {
	switch t.Status {
	case TaskStatusPending, TaskStatusRunning, TaskStatusCompleted, TaskStatusFailed:
		return true
	default:
		return false
	}
}

// CanTransitionTo checks if the task can transition to the given status
func (t *Task) CanTransitionTo(newStatus TaskStatus) bool {
	switch t.Status {
	case TaskStatusPending:
		return newStatus == TaskStatusRunning || newStatus == TaskStatusFailed
	case TaskStatusRunning:
		return newStatus == TaskStatusCompleted || newStatus == TaskStatusFailed
	case TaskStatusCompleted, TaskStatusFailed:
		return false // Terminal states
	default:
		return false
	}
}

// UpdateStatus updates the task status with validation
func (t *Task) UpdateStatus(tx *gorm.DB, newStatus TaskStatus) error {
	if !t.CanTransitionTo(newStatus) {
		return fmt.Errorf("cannot transition from %s to %s", t.Status, newStatus)
	}
	
	return tx.Model(t).Update("status", newStatus).Error
}