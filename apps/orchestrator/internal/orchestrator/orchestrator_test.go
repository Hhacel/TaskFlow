package orchestrator

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hhace/taskflow/apps/orchestrator/config"
	"github.com/hhace/taskflow/internal/task"
	"github.com/hhace/taskflow/internal/workflow"
	"github.com/hhace/taskflow/pkg/messaging"
	"github.com/hhace/taskflow/pkg/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestOrchestrator() (*Orchestrator, *persistence.MockRepository, *messaging.MockBroker) {
	cfg := config.DefaultConfig()
	repo := persistence.NewMockRepository()
	broker := messaging.NewMockBroker()
	return New(cfg, repo, broker), repo, broker
}

func TestStartWorkflow_DispatchesRootTasks(t *testing.T) {
	o, repo, broker := newTestOrchestrator()

	repo.On("GetWorkflowByID", uint(1)).Return(&workflow.Workflow{ID: 1, Status: workflow.StatusCreated}, nil)
	repo.On("UpdateWorkflowStatus", uint(1), workflow.StatusRunning).Return(nil)
	repo.On("GetTasksByWorkflowID", uint(1)).Return([]task.Task{
		{ID: 10, WorkflowID: 1, Command: "echo a", Status: task.TaskStatusPending},
		{ID: 11, WorkflowID: 1, Command: "echo b", Status: task.TaskStatusPending},
	}, nil)
	repo.On("GetDependenciesForTask", uint(10)).Return([]task.TaskDependency{}, nil)
	repo.On("GetDependenciesForTask", uint(11)).Return([]task.TaskDependency{{TaskID: 11, DependsOnTaskID: 10}}, nil)
	repo.On("UpdateTaskStatus", uint(10), task.TaskStatusRunning).Return(nil)

	err := o.StartWorkflow(1)
	assert.NoError(t, err)

	// Only task 10 (no dependencies) should have been dispatched.
	assert.Len(t, broker.Sent, 1)
	var msg task.DispatchMessage
	assert.NoError(t, json.Unmarshal(broker.Sent[0].Data, &msg))
	assert.Equal(t, uint(10), msg.TaskID)
	assert.Equal(t, 1, msg.Attempt)
}

func TestStartWorkflow_IgnoredWhenNotCreated(t *testing.T) {
	o, repo, broker := newTestOrchestrator()

	repo.On("GetWorkflowByID", uint(2)).Return(&workflow.Workflow{ID: 2, Status: workflow.StatusRunning}, nil)

	err := o.StartWorkflow(2)
	assert.NoError(t, err)
	assert.Empty(t, broker.Sent)
}

func TestCancelWorkflow_CancelsTasks(t *testing.T) {
	o, repo, _ := newTestOrchestrator()

	repo.On("GetWorkflowByID", uint(3)).Return(&workflow.Workflow{ID: 3, Status: workflow.StatusRunning}, nil)
	repo.On("UpdateWorkflowStatus", uint(3), workflow.StatusCancelled).Return(nil)
	repo.On("CancelTasksByWorkflowID", uint(3)).Return(nil)

	err := o.CancelWorkflow(3)
	assert.NoError(t, err)
	repo.AssertCalled(t, "CancelTasksByWorkflowID", uint(3))
}

func TestProcessTaskResult_SuccessDispatchesNextTask(t *testing.T) {
	o, repo, broker := newTestOrchestrator()

	now := time.Now()
	repo.On("GetTaskByID", uint(10)).Return(&task.Task{ID: 10, WorkflowID: 1, Status: task.TaskStatusRunning}, nil)
	repo.On("CreateTaskResult", mock.AnythingOfType("*task.TaskResult")).Return(nil)
	repo.On("GetWorkflowByID", uint(1)).Return(&workflow.Workflow{ID: 1, Status: workflow.StatusRunning}, nil)
	repo.On("UpdateTaskStatus", uint(10), task.TaskStatusSucceeded).Return(nil)
	repo.On("GetTasksByWorkflowID", uint(1)).Return([]task.Task{
		{ID: 10, WorkflowID: 1, Status: task.TaskStatusSucceeded},
		{ID: 11, WorkflowID: 1, Command: "echo b", Status: task.TaskStatusPending},
	}, nil)
	repo.On("GetDependenciesForTask", uint(11)).Return([]task.TaskDependency{{TaskID: 11, DependsOnTaskID: 10}}, nil)
	repo.On("UpdateTaskStatus", uint(11), task.TaskStatusRunning).Return(nil)

	err := o.ProcessTaskResult(task.ResultMessage{
		TaskID: 10, Attempt: 1, Success: true, Output: "ok", StartTime: now, EndTime: now,
	})
	assert.NoError(t, err)

	assert.Len(t, broker.Sent, 1)
	var msg task.DispatchMessage
	assert.NoError(t, json.Unmarshal(broker.Sent[0].Data, &msg))
	assert.Equal(t, uint(11), msg.TaskID)
}

func TestProcessTaskResult_CompletesWorkflowWhenAllSucceeded(t *testing.T) {
	o, repo, broker := newTestOrchestrator()

	now := time.Now()
	repo.On("GetTaskByID", uint(10)).Return(&task.Task{ID: 10, WorkflowID: 1, Status: task.TaskStatusRunning}, nil)
	repo.On("CreateTaskResult", mock.AnythingOfType("*task.TaskResult")).Return(nil)
	repo.On("GetWorkflowByID", uint(1)).Return(&workflow.Workflow{ID: 1, Status: workflow.StatusRunning}, nil)
	repo.On("UpdateTaskStatus", uint(10), task.TaskStatusSucceeded).Return(nil)
	repo.On("GetTasksByWorkflowID", uint(1)).Return([]task.Task{
		{ID: 10, WorkflowID: 1, Status: task.TaskStatusSucceeded},
	}, nil)
	repo.On("UpdateWorkflowStatus", uint(1), workflow.StatusCompleted).Return(nil)

	err := o.ProcessTaskResult(task.ResultMessage{
		TaskID: 10, Attempt: 1, Success: true, StartTime: now, EndTime: now,
	})
	assert.NoError(t, err)
	assert.Empty(t, broker.Sent)
	repo.AssertCalled(t, "UpdateWorkflowStatus", uint(1), workflow.StatusCompleted)
}

func TestProcessTaskResult_FailureRetries(t *testing.T) {
	o, repo, broker := newTestOrchestrator()

	now := time.Now()
	repo.On("GetTaskByID", uint(10)).Return(&task.Task{ID: 10, WorkflowID: 1, Command: "echo a", Status: task.TaskStatusRunning}, nil)
	repo.On("CreateTaskResult", mock.AnythingOfType("*task.TaskResult")).Return(nil)
	repo.On("GetWorkflowByID", uint(1)).Return(&workflow.Workflow{ID: 1, Status: workflow.StatusRunning}, nil)
	repo.On("UpdateTaskStatus", uint(10), task.TaskStatusFailed).Return(nil)
	repo.On("UpdateTaskStatus", uint(10), task.TaskStatusRunning).Return(nil)

	err := o.ProcessTaskResult(task.ResultMessage{
		TaskID: 10, Attempt: 1, Success: false, Error: "boom", StartTime: now, EndTime: now,
	})
	assert.NoError(t, err)

	assert.Len(t, broker.Sent, 1)
	var msg task.DispatchMessage
	assert.NoError(t, json.Unmarshal(broker.Sent[0].Data, &msg))
	assert.Equal(t, 2, msg.Attempt)
}

func TestProcessTaskResult_PermanentFailureFailsWorkflow(t *testing.T) {
	o, repo, broker := newTestOrchestrator()

	now := time.Now()
	maxAttempts := o.cfg.Orchestrator.MaxAttempts

	repo.On("GetTaskByID", uint(10)).Return(&task.Task{ID: 10, WorkflowID: 1, Status: task.TaskStatusRunning}, nil)
	repo.On("CreateTaskResult", mock.AnythingOfType("*task.TaskResult")).Return(nil)
	repo.On("GetWorkflowByID", uint(1)).Return(&workflow.Workflow{ID: 1, Status: workflow.StatusRunning}, nil)
	repo.On("UpdateTaskStatus", uint(10), task.TaskStatusFailed).Return(nil)
	repo.On("UpdateWorkflowStatus", uint(1), workflow.StatusFailed).Return(nil)
	repo.On("CancelTasksByWorkflowID", uint(1)).Return(nil)

	err := o.ProcessTaskResult(task.ResultMessage{
		TaskID: 10, Attempt: maxAttempts, Success: false, Error: "boom", StartTime: now, EndTime: now,
	})
	assert.NoError(t, err)
	assert.Empty(t, broker.Sent)
	repo.AssertCalled(t, "UpdateWorkflowStatus", uint(1), workflow.StatusFailed)
	repo.AssertCalled(t, "CancelTasksByWorkflowID", uint(1))
}

func TestProcessTaskResult_DiscardedWhenWorkflowNotRunning(t *testing.T) {
	o, repo, broker := newTestOrchestrator()

	now := time.Now()
	repo.On("GetTaskByID", uint(10)).Return(&task.Task{ID: 10, WorkflowID: 1, Status: task.TaskStatusCancelled}, nil)
	repo.On("CreateTaskResult", mock.AnythingOfType("*task.TaskResult")).Return(nil)
	repo.On("GetWorkflowByID", uint(1)).Return(&workflow.Workflow{ID: 1, Status: workflow.StatusCancelled}, nil)

	err := o.ProcessTaskResult(task.ResultMessage{
		TaskID: 10, Attempt: 1, Success: true, StartTime: now, EndTime: now,
	})
	assert.NoError(t, err)
	assert.Empty(t, broker.Sent)
}
