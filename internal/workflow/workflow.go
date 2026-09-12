package workflow

import (
	"fmt"
	"time"
)

// Status represents the lifecycle state of a Workflow.
type Status string

const (
	StatusCreated   Status = "CREATED"
	StatusRunning   Status = "RUNNING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
	StatusCancelled Status = "CANCELLED"
)

// Workflow represents an instance of a business process managed by the Orchestrator.
type Workflow struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Status    Status    `gorm:"type:varchar(255);not null;default:CREATED" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the table name for GORM.
func (Workflow) TableName() string {
	return "workflows"
}

// IsValidStatus reports whether the status is one of the known Workflow states.
func (w *Workflow) IsValidStatus() bool {
	switch w.Status {
	case StatusCreated, StatusRunning, StatusCompleted, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

// CanTransitionTo reports whether the workflow can move from its current status to newStatus.
func (w *Workflow) CanTransitionTo(newStatus Status) bool {
	switch w.Status {
	case StatusCreated:
		return newStatus == StatusRunning || newStatus == StatusCancelled
	case StatusRunning:
		return newStatus == StatusCompleted || newStatus == StatusFailed || newStatus == StatusCancelled
	case StatusCompleted, StatusFailed, StatusCancelled:
		return false // terminal states
	default:
		return false
	}
}

// UpdateStatus validates and applies a status transition.
func (w *Workflow) UpdateStatus(newStatus Status) error {
	if !w.CanTransitionTo(newStatus) {
		return fmt.Errorf("cannot transition workflow from %s to %s", w.Status, newStatus)
	}
	w.Status = newStatus
	return nil
}

// IsTerminal reports whether the workflow is in one of its terminal states.
func (w *Workflow) IsTerminal() bool {
	switch w.Status {
	case StatusCompleted, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}
