package task

import (
	"time"

	"github.com/google/uuid"
)

type TaskExecutionResult struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TaskID    uuid.UUID `gorm:"type:uuid;not null;index" json:"task_id"`
	Task      Task      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Success   bool      `gorm:"not null" json:"success"`
	Output    string    `gorm:"type:text" json:"output"`
	Error     string    `gorm:"type:text" json:"error"`
	StartTime time.Time `gorm:"not null" json:"start_time"`
	EndTime   time.Time `gorm:"not null" json:"end_time"`
	Duration  int64     `json:"duration_ms"` 
	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name for GORM
func (TaskExecutionResult) TableName() string {
	return "task_execution_results"
}
