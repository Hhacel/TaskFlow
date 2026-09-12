package handlers

import (
	"time"

	"github.com/hhace/taskflow/internal/task"
	"github.com/hhace/taskflow/internal/workflow"
)

// TaskDefinition describes a single task within a CreateWorkflowRequest.
// Ref is a client-chosen identifier used only within the request payload to
// express dependencies between tasks that do not have database IDs yet.
type TaskDefinition struct {
	Ref       string   `json:"ref" binding:"required"`
	Name      string   `json:"name" binding:"required"`
	Command   string   `json:"command" binding:"required"`
	Timeout   *int     `json:"timeout,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"`
}

// CreateWorkflowRequest is the body of PU-001 Create Workflow.
type CreateWorkflowRequest struct {
	Name  string           `json:"name" binding:"required"`
	Tasks []TaskDefinition `json:"tasks" binding:"required,min=1,dive"`
}

// CreateWorkflowResponse is returned after a workflow definition is persisted.
type CreateWorkflowResponse struct {
	ID uint `json:"id"`
}

// TaskStatusResponse summarizes a task's current state within a workflow report.
type TaskStatusResponse struct {
	ID     uint            `json:"id"`
	Name   string          `json:"name"`
	Status task.TaskStatus `json:"status"`
}

// WorkflowStatusResponse is the consolidated report returned by PU-003 Get Workflow Status.
type WorkflowStatusResponse struct {
	ID        uint                 `json:"id"`
	Name      string               `json:"name"`
	Status    workflow.Status      `json:"status"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
	Tasks     []TaskStatusResponse `json:"tasks"`
}

// TaskResultResponse mirrors a single task.TaskResult record.
type TaskResultResponse struct {
	ID        uint      `json:"id"`
	TaskID    uint      `json:"task_id"`
	Attempt   int       `json:"attempt"`
	Success   bool      `json:"success"`
	Output    string    `json:"output"`
	Error     string    `json:"error"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// TaskResultListResponse is returned by PU-004 Get Task Results.
type TaskResultListResponse struct {
	TaskID  uint                 `json:"task_id"`
	Results []TaskResultResponse `json:"results"`
}

// AcceptedResponse is returned for asynchronous operations (Start/Cancel Workflow).
type AcceptedResponse struct {
	ID      uint   `json:"id"`
	Message string `json:"message"`
}

// ErrorResponse is the generic error payload returned by every endpoint.
type ErrorResponse struct {
	Error string `json:"error"`
}
