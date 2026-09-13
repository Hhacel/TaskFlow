package task

import "time"

// DispatchMessage is the payload the Orchestrator publishes to workers when a
// task is ready to run. Workers are stateless and only need this data to
// execute the command; they never access the database directly.
type DispatchMessage struct {
	TaskID     uint   `json:"task_id"`
	WorkflowID uint   `json:"workflow_id"`
	Attempt    int    `json:"attempt"`
	Command    string `json:"command"`
	// TimeoutSeconds is nil when the task has no explicit timeout.
	TimeoutSeconds *int `json:"timeout_seconds,omitempty"`
}

// ResultMessage is the payload a Worker publishes back to the Orchestrator
// once a dispatched task has finished executing (successfully or not).
type ResultMessage struct {
	TaskID    uint      `json:"task_id"`
	Attempt   int       `json:"attempt"`
	Success   bool      `json:"success"`
	Output    string    `json:"output"`
	Error     string    `json:"error"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}
