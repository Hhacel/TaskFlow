package models

import (
	"time"

	"github.com/google/uuid"
)

// TaskExecutionResult represents a task execution result in the database
type TaskExecutionResult struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	TaskID    uuid.UUID `gorm:"type:uuid;not null;index" json:"task_id"`
	Success   bool      `gorm:"not null" json:"success"`
	Output    string    `gorm:"type:text" json:"output"`
	Error     string    `gorm:"type:text" json:"error"`
	StartTime time.Time `gorm:"not null" json:"start_time"`
	EndTime   time.Time `gorm:"not null" json:"end_time"`
	Duration  string    `gorm:"type:varchar(50)" json:"duration"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName specifies the table name for GORM
func (TaskExecutionResult) TableName() string {
	return "task_execution_results"
}
