package task

import "time"

// TaskResult represents the outcome of a single execution attempt of a Task.
type TaskResult struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID    uint      `gorm:"not null;index" json:"task_id"`
	Attempt   int       `gorm:"not null" json:"attempt"`
	Success   bool      `gorm:"not null" json:"success"`
	Output    string    `gorm:"type:text" json:"output"`
	Error     string    `gorm:"type:text" json:"error"`
	StartTime time.Time `gorm:"not null" json:"start_time"`
	EndTime   time.Time `gorm:"not null" json:"end_time"`
}

// TableName specifies the table name for GORM
func (TaskResult) TableName() string {
	return "task_results"
}
