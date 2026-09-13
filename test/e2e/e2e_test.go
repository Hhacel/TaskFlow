package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hhace/taskflow/internal/task"
	"github.com/hhace/taskflow/internal/workflow"
)

// The types below mirror the API's public JSON contract (see
// apps/api/internal/handlers/dto.go). They are defined locally rather than
// imported, since a true black-box e2e test should only depend on the wire
// format a real client would see, not on the API's internal Go types.

type taskDefinition struct {
	Ref       string   `json:"ref"`
	Name      string   `json:"name"`
	Command   string   `json:"command"`
	Timeout   *int     `json:"timeout,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"`
}

type createWorkflowRequest struct {
	Name  string           `json:"name"`
	Tasks []taskDefinition `json:"tasks"`
}

type createWorkflowResponse struct {
	ID uint `json:"id"`
}

type taskStatusResponse struct {
	ID     uint            `json:"id"`
	Name   string          `json:"name"`
	Status task.TaskStatus `json:"status"`
}

type workflowStatusResponse struct {
	ID     uint                 `json:"id"`
	Name   string               `json:"name"`
	Status workflow.Status      `json:"status"`
	Tasks  []taskStatusResponse `json:"tasks"`
}

type taskResultResponse struct {
	ID      uint   `json:"id"`
	TaskID  uint   `json:"task_id"`
	Attempt int    `json:"attempt"`
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error"`
}

type taskResultListResponse struct {
	TaskID  uint                 `json:"task_id"`
	Results []taskResultResponse `json:"results"`
}

// TestE2E_CreateWorkflow covers PU-001 Create Workflow: it POSTs a workflow
// definition (two tasks, one depending on the other) to the real HTTP API and
// verifies both the response and the resulting rows in the workflows, tasks
// and task_dependencies tables.
func TestE2E_CreateWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	resetDB(t)

	reqBody := createWorkflowRequest{
		Name: "e2e-create",
		Tasks: []taskDefinition{
			{Ref: "a", Name: "task-a", Command: "echo a"},
			{Ref: "b", Name: "task-b", Command: "echo b", DependsOn: []string{"a"}},
		},
	}

	status, body := doRequest(t, http.MethodPost, "/api/v1/workflows", reqBody)
	require.Equal(t, http.StatusCreated, status)

	var resp createWorkflowResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	require.NotZero(t, resp.ID)

	wf, err := testEnv.repo.GetWorkflowByID(resp.ID)
	require.NoError(t, err)
	assert.Equal(t, "e2e-create", wf.Name)
	assert.Equal(t, workflow.StatusCreated, wf.Status)

	tasks, err := testEnv.repo.GetTasksByWorkflowID(resp.ID)
	require.NoError(t, err)
	require.Len(t, tasks, 2)

	var taskA, taskB *task.Task
	for i := range tasks {
		switch tasks[i].Name {
		case "task-a":
			taskA = &tasks[i]
		case "task-b":
			taskB = &tasks[i]
		}
	}
	require.NotNil(t, taskA)
	require.NotNil(t, taskB)
	assert.Equal(t, task.TaskStatusPending, taskA.Status)
	assert.Equal(t, task.TaskStatusPending, taskB.Status)
	assert.Equal(t, "echo a", taskA.Command)

	deps, err := testEnv.repo.GetDependenciesForTask(taskB.ID)
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, taskA.ID, deps[0].DependsOnTaskID)
}

// TestE2E_StartWorkflow_RunsDAGToCompletion covers PU-002 Start Workflow: it
// starts a two-task chain and waits for the real Orchestrator + Worker
// pipeline (communicating over an embedded NATS server) to execute both
// tasks, then verifies the tasks and task_results tables directly.
func TestE2E_StartWorkflow_RunsDAGToCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	resetDB(t)

	reqBody := createWorkflowRequest{
		Name: "e2e-start",
		Tasks: []taskDefinition{
			{Ref: "a", Name: "task-a", Command: "echo hello-a"},
			{Ref: "b", Name: "task-b", Command: "echo hello-b", DependsOn: []string{"a"}},
		},
	}
	status, body := doRequest(t, http.MethodPost, "/api/v1/workflows", reqBody)
	require.Equal(t, http.StatusCreated, status)
	var created createWorkflowResponse
	require.NoError(t, json.Unmarshal(body, &created))

	status, _ = doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/workflows/%d/start", created.ID), nil)
	require.Equal(t, http.StatusAccepted, status)

	waitForCondition(t, 10*time.Second, func() bool {
		wf, err := testEnv.repo.GetWorkflowByID(created.ID)
		return err == nil && wf.Status == workflow.StatusCompleted
	})

	tasks, err := testEnv.repo.GetTasksByWorkflowID(created.ID)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	for _, tk := range tasks {
		assert.Equal(t, task.TaskStatusSucceeded, tk.Status)

		results, err := testEnv.repo.GetTaskResultsByTaskID(tk.ID)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].Success)
		assert.Contains(t, results[0].Output, "hello")
	}
}

// TestE2E_GetWorkflowStatus covers PU-003 Get Workflow Status: after running
// a workflow to completion it GETs the status endpoint and cross-checks every
// field of the response against what is actually stored in the database.
func TestE2E_GetWorkflowStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	resetDB(t)

	reqBody := createWorkflowRequest{
		Name:  "e2e-status",
		Tasks: []taskDefinition{{Ref: "a", Name: "task-a", Command: "echo status"}},
	}
	status, body := doRequest(t, http.MethodPost, "/api/v1/workflows", reqBody)
	require.Equal(t, http.StatusCreated, status)
	var created createWorkflowResponse
	require.NoError(t, json.Unmarshal(body, &created))

	status, _ = doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/workflows/%d/start", created.ID), nil)
	require.Equal(t, http.StatusAccepted, status)

	waitForCondition(t, 10*time.Second, func() bool {
		wf, err := testEnv.repo.GetWorkflowByID(created.ID)
		return err == nil && wf.Status == workflow.StatusCompleted
	})

	status, body = doRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/workflows/%d", created.ID), nil)
	require.Equal(t, http.StatusOK, status)

	var got workflowStatusResponse
	require.NoError(t, json.Unmarshal(body, &got))

	dbWf, err := testEnv.repo.GetWorkflowByID(created.ID)
	require.NoError(t, err)
	dbTasks, err := testEnv.repo.GetTasksByWorkflowID(created.ID)
	require.NoError(t, err)

	assert.Equal(t, dbWf.ID, got.ID)
	assert.Equal(t, dbWf.Name, got.Name)
	assert.Equal(t, dbWf.Status, got.Status)
	require.Len(t, got.Tasks, len(dbTasks))
	assert.Equal(t, dbTasks[0].ID, got.Tasks[0].ID)
	assert.Equal(t, dbTasks[0].Status, got.Tasks[0].Status)
}

// TestE2E_GetTaskResults covers PU-004 Get Task Results: after a task has
// executed, it GETs the task results endpoint and verifies the response
// matches the task_results rows persisted by the Worker.
func TestE2E_GetTaskResults(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	resetDB(t)

	reqBody := createWorkflowRequest{
		Name:  "e2e-results",
		Tasks: []taskDefinition{{Ref: "a", Name: "task-a", Command: "echo result-check"}},
	}
	status, body := doRequest(t, http.MethodPost, "/api/v1/workflows", reqBody)
	require.Equal(t, http.StatusCreated, status)
	var created createWorkflowResponse
	require.NoError(t, json.Unmarshal(body, &created))

	status, _ = doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/workflows/%d/start", created.ID), nil)
	require.Equal(t, http.StatusAccepted, status)

	waitForCondition(t, 10*time.Second, func() bool {
		wf, err := testEnv.repo.GetWorkflowByID(created.ID)
		return err == nil && wf.Status == workflow.StatusCompleted
	})

	tasks, err := testEnv.repo.GetTasksByWorkflowID(created.ID)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	taskID := tasks[0].ID

	status, body = doRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/results", taskID), nil)
	require.Equal(t, http.StatusOK, status)

	var got taskResultListResponse
	require.NoError(t, json.Unmarshal(body, &got))

	dbResults, err := testEnv.repo.GetTaskResultsByTaskID(taskID)
	require.NoError(t, err)
	require.Len(t, got.Results, len(dbResults))
	require.Len(t, got.Results, 1)
	assert.Equal(t, taskID, got.TaskID)
	assert.True(t, got.Results[0].Success)
	assert.Contains(t, got.Results[0].Output, "result-check")
	assert.Equal(t, dbResults[0].ID, got.Results[0].ID)
}

// TestE2E_CancelWorkflow covers PU-005 Cancel Workflow: it cancels a
// newly-created (not yet started) workflow and verifies both the workflow and
// its tasks transition to CANCELLED in the database, and that a second
// cancel attempt is rejected since the workflow is now terminal.
func TestE2E_CancelWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	resetDB(t)

	reqBody := createWorkflowRequest{
		Name: "e2e-cancel",
		Tasks: []taskDefinition{
			{Ref: "a", Name: "task-a", Command: "echo a"},
			{Ref: "b", Name: "task-b", Command: "echo b", DependsOn: []string{"a"}},
		},
	}
	status, body := doRequest(t, http.MethodPost, "/api/v1/workflows", reqBody)
	require.Equal(t, http.StatusCreated, status)
	var created createWorkflowResponse
	require.NoError(t, json.Unmarshal(body, &created))

	status, _ = doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/workflows/%d/cancel", created.ID), nil)
	require.Equal(t, http.StatusAccepted, status)

	waitForCondition(t, 5*time.Second, func() bool {
		wf, err := testEnv.repo.GetWorkflowByID(created.ID)
		return err == nil && wf.Status == workflow.StatusCancelled
	})

	tasks, err := testEnv.repo.GetTasksByWorkflowID(created.ID)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	for _, tk := range tasks {
		assert.Equal(t, task.TaskStatusCancelled, tk.Status)
	}

	status, _ = doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/workflows/%d/cancel", created.ID), nil)
	assert.Equal(t, http.StatusConflict, status)
}
