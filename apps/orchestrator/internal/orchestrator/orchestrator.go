// Package orchestrator implements the TaskFlow Orchestrator: it analyzes the
// TaskDependency graph of a Workflow, decides which tasks are ready to run,
// dispatches them to Workers over NATS, and reacts to task results published
// back by Workers in order to progress (or fail/complete) the Workflow.
package orchestrator

import (
	"encoding/json"
	"log/slog"

	"github.com/hhace/taskflow/apps/orchestrator/config"
	"github.com/hhace/taskflow/internal/task"
	"github.com/hhace/taskflow/internal/workflow"
	"github.com/hhace/taskflow/pkg/messaging"
	"github.com/hhace/taskflow/pkg/persistence"
)

// Orchestrator is the engine described in Refactor.MD section 3: it decides
// which tasks are ready, publishes dispatch commands, and reacts to results.
type Orchestrator struct {
	cfg    *config.Config
	repo   persistence.RepositoryInterface
	broker messaging.Broker
	subs   []messaging.Subscription
}

// New creates a new Orchestrator.
func New(cfg *config.Config, repo persistence.RepositoryInterface, broker messaging.Broker) *Orchestrator {
	return &Orchestrator{cfg: cfg, repo: repo, broker: broker}
}

// Start subscribes to every subject the Orchestrator reacts to. Every
// subscription uses a Queue Group so that, if scaled horizontally, only one
// Orchestrator instance handles a given message.
func (o *Orchestrator) Start() error {
	group := o.cfg.NATS.QueueGroupName

	startSub, err := o.broker.QueueSubscribe(o.cfg.NATS.WorkflowStartSubject, group, o.handleStartCommand)
	if err != nil {
		return err
	}
	o.subs = append(o.subs, startSub)

	cancelSub, err := o.broker.QueueSubscribe(o.cfg.NATS.WorkflowCancelSubject, group, o.handleCancelCommand)
	if err != nil {
		return err
	}
	o.subs = append(o.subs, cancelSub)

	resultSub, err := o.broker.QueueSubscribe(o.cfg.NATS.TaskResultSubject, group, o.handleTaskResult)
	if err != nil {
		return err
	}
	o.subs = append(o.subs, resultSub)

	slog.Info("Orchestrator subscribed to control and result subjects",
		"start", o.cfg.NATS.WorkflowStartSubject,
		"cancel", o.cfg.NATS.WorkflowCancelSubject,
		"results", o.cfg.NATS.TaskResultSubject,
		"queueGroup", group)

	return nil
}

// Stop unsubscribes every subscription and closes the broker connection.
func (o *Orchestrator) Stop() error {
	for _, sub := range o.subs {
		if err := sub.Unsubscribe(); err != nil {
			slog.Error("Failed to unsubscribe", "error", err)
		}
	}
	return o.broker.Close()
}

// --- NATS handlers -------------------------------------------------------------

func (o *Orchestrator) handleStartCommand(data []byte) {
	var cmd workflow.CommandMessage
	if err := json.Unmarshal(data, &cmd); err != nil {
		slog.Error("Failed to unmarshal start command", "error", err)
		return
	}
	if err := o.StartWorkflow(cmd.WorkflowID); err != nil {
		slog.Error("Failed to start workflow", "workflowId", cmd.WorkflowID, "error", err)
	}
}

func (o *Orchestrator) handleCancelCommand(data []byte) {
	var cmd workflow.CommandMessage
	if err := json.Unmarshal(data, &cmd); err != nil {
		slog.Error("Failed to unmarshal cancel command", "error", err)
		return
	}
	if err := o.CancelWorkflow(cmd.WorkflowID); err != nil {
		slog.Error("Failed to cancel workflow", "workflowId", cmd.WorkflowID, "error", err)
	}
}

func (o *Orchestrator) handleTaskResult(data []byte) {
	var msg task.ResultMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		slog.Error("Failed to unmarshal task result", "error", err)
		return
	}
	if err := o.ProcessTaskResult(msg); err != nil {
		slog.Error("Failed to process task result", "taskId", msg.TaskID, "error", err)
	}
}

// --- Use case entry points -------------------------------------------------------------

// StartWorkflow implements PU-002: transitions CREATED -> RUNNING and
// dispatches every task that has no unmet dependency.
func (o *Orchestrator) StartWorkflow(workflowID uint) error {
	wf, err := o.repo.GetWorkflowByID(workflowID)
	if err != nil {
		return err
	}

	if !wf.CanTransitionTo(workflow.StatusRunning) {
		slog.Warn("Ignoring start command: workflow cannot transition to RUNNING",
			"workflowId", workflowID, "status", wf.Status)
		return nil
	}

	if err := o.repo.UpdateWorkflowStatus(workflowID, workflow.StatusRunning); err != nil {
		return err
	}

	slog.Info("Workflow started", "workflowId", workflowID)
	return o.dispatchReadyTasks(workflowID)
}

// CancelWorkflow implements PU-005: transitions the workflow to CANCELLED and
// cancels every PENDING/RUNNING task, halting further dispatch.
func (o *Orchestrator) CancelWorkflow(workflowID uint) error {
	wf, err := o.repo.GetWorkflowByID(workflowID)
	if err != nil {
		return err
	}

	if wf.IsTerminal() {
		slog.Warn("Ignoring cancel command: workflow already terminal",
			"workflowId", workflowID, "status", wf.Status)
		return nil
	}

	if err := o.repo.UpdateWorkflowStatus(workflowID, workflow.StatusCancelled); err != nil {
		return err
	}

	slog.Info("Workflow cancelled", "workflowId", workflowID)
	return o.repo.CancelTasksByWorkflowID(workflowID)
}

// ProcessTaskResult records a task execution attempt and decides how the
// workflow should proceed: retry the task, dispatch newly-ready tasks,
// complete the workflow, or fail it.
func (o *Orchestrator) ProcessTaskResult(msg task.ResultMessage) error {
	t, err := o.repo.GetTaskByID(msg.TaskID)
	if err != nil {
		return err
	}

	result := &task.TaskResult{
		TaskID:    msg.TaskID,
		Attempt:   msg.Attempt,
		Success:   msg.Success,
		Output:    msg.Output,
		Error:     msg.Error,
		StartTime: msg.StartTime,
		EndTime:   msg.EndTime,
	}
	if err := o.repo.CreateTaskResult(result); err != nil {
		return err
	}

	wf, err := o.repo.GetWorkflowByID(t.WorkflowID)
	if err != nil {
		return err
	}

	// A cancelled/failed workflow no longer schedules new work; the result is
	// still recorded above for auditing purposes.
	if wf.Status != workflow.StatusRunning {
		slog.Info("Discarding result for non-running workflow", "workflowId", wf.ID, "status", wf.Status)
		return nil
	}

	if msg.Success {
		if err := o.repo.UpdateTaskStatus(t.ID, task.TaskStatusSucceeded); err != nil {
			return err
		}
		return o.onTaskSucceeded(wf.ID)
	}

	return o.onTaskFailed(*t, msg.Attempt)
}

// --- internal orchestration logic -------------------------------------------------------------

func (o *Orchestrator) onTaskSucceeded(workflowID uint) error {
	tasks, err := o.repo.GetTasksByWorkflowID(workflowID)
	if err != nil {
		return err
	}

	allSucceeded := true
	for _, t := range tasks {
		if t.Status != task.TaskStatusSucceeded {
			allSucceeded = false
			break
		}
	}

	if allSucceeded {
		slog.Info("Workflow completed", "workflowId", workflowID)
		return o.repo.UpdateWorkflowStatus(workflowID, workflow.StatusCompleted)
	}

	return o.dispatchReadyTasks(workflowID)
}

func (o *Orchestrator) onTaskFailed(t task.Task, attempt int) error {
	if attempt < o.cfg.Orchestrator.MaxAttempts {
		if err := o.repo.UpdateTaskStatus(t.ID, task.TaskStatusFailed); err != nil {
			return err
		}
		slog.Info("Retrying failed task", "taskId", t.ID, "nextAttempt", attempt+1)
		return o.dispatchTask(t, attempt+1)
	}

	if err := o.repo.UpdateTaskStatus(t.ID, task.TaskStatusFailed); err != nil {
		return err
	}

	slog.Warn("Task permanently failed, failing workflow", "taskId", t.ID, "workflowId", t.WorkflowID)
	if err := o.repo.UpdateWorkflowStatus(t.WorkflowID, workflow.StatusFailed); err != nil {
		return err
	}
	return o.repo.CancelTasksByWorkflowID(t.WorkflowID)
}

// dispatchReadyTasks publishes every PENDING task whose dependencies have all
// SUCCEEDED.
func (o *Orchestrator) dispatchReadyTasks(workflowID uint) error {
	tasks, err := o.repo.GetTasksByWorkflowID(workflowID)
	if err != nil {
		return err
	}

	statusByID := make(map[uint]task.TaskStatus, len(tasks))
	for _, t := range tasks {
		statusByID[t.ID] = t.Status
	}

	for _, t := range tasks {
		if t.Status != task.TaskStatusPending {
			continue
		}

		deps, err := o.repo.GetDependenciesForTask(t.ID)
		if err != nil {
			return err
		}

		ready := true
		for _, dep := range deps {
			if statusByID[dep.DependsOnTaskID] != task.TaskStatusSucceeded {
				ready = false
				break
			}
		}

		if ready {
			if err := o.dispatchTask(t, 1); err != nil {
				return err
			}
		}
	}

	return nil
}

// dispatchTask transitions t to RUNNING and publishes a DispatchMessage for
// Workers to pick up via the Queue Group.
func (o *Orchestrator) dispatchTask(t task.Task, attempt int) error {
	if err := o.repo.UpdateTaskStatus(t.ID, task.TaskStatusRunning); err != nil {
		return err
	}

	msg := task.DispatchMessage{
		TaskID:         t.ID,
		WorkflowID:     t.WorkflowID,
		Attempt:        attempt,
		Command:        t.Command,
		TimeoutSeconds: t.Timeout,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	slog.Info("Dispatching task", "taskId", t.ID, "attempt", attempt)
	return o.broker.Publish(o.cfg.NATS.TaskDispatchSubject, data)
}
