package workflow

// CommandMessage is the payload published by the API Gateway on the
// workflow control subjects (start / cancel) and consumed by the Orchestrator.
type CommandMessage struct {
	WorkflowID uint `json:"workflow_id"`
}
