package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hhace/taskflow/apps/api/config"
	"github.com/hhace/taskflow/internal/task"
	"github.com/hhace/taskflow/internal/workflow"
	"github.com/hhace/taskflow/pkg/messaging"
	"github.com/hhace/taskflow/pkg/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestHandler() (*WorkflowHandler, *persistence.MockRepository, *messaging.MockBroker) {
	cfg := config.DefaultConfig()
	repo := persistence.NewMockRepository()
	broker := messaging.NewMockBroker()
	return NewWorkflowHandler(cfg, repo, broker), repo, broker
}

func setupRouter(h *WorkflowHandler) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.POST("/workflows", h.CreateWorkflow)
	v1.POST("/workflows/:id/start", h.StartWorkflow)
	v1.GET("/workflows/:id", h.GetWorkflowStatus)
	v1.GET("/tasks/:taskId/results", h.GetTaskResult)
	v1.POST("/workflows/:id/cancel", h.CancelWorkflow)
	return r
}

func TestCreateWorkflow_Success(t *testing.T) {
	h, repo, _ := newTestHandler()
	router := setupRouter(h)

	repo.On("Transaction", mock.AnythingOfType("func(persistence.RepositoryInterface) error")).
		Return(nil).
		Run(func(args mock.Arguments) {
			fn := args.Get(0).(func(persistence.RepositoryInterface) error)
			if err := fn(repo); err != nil {
				t.Fatalf("transaction fn failed: %v", err)
			}
		})
	repo.On("CreateWorkflow", mock.AnythingOfType("*workflow.Workflow")).Return(nil)
	repo.On("CreateTask", mock.AnythingOfType("*task.Task")).Return(nil)
	repo.On("CreateTaskDependency", mock.AnythingOfType("*task.TaskDependency")).Return(nil)

	body := CreateWorkflowRequest{
		Name: "example",
		Tasks: []TaskDefinition{
			{Ref: "t1", Name: "extract", Command: "echo extract"},
			{Ref: "t2", Name: "transform", Command: "echo transform", DependsOn: []string{"t1"}},
		},
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateWorkflow_InvalidGraph(t *testing.T) {
	h, _, _ := newTestHandler()
	router := setupRouter(h)

	body := CreateWorkflowRequest{
		Name: "example",
		Tasks: []TaskDefinition{
			{Ref: "t1", Name: "extract", Command: "echo extract", DependsOn: []string{"missing"}},
		},
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStartWorkflow_Success(t *testing.T) {
	h, repo, broker := newTestHandler()
	router := setupRouter(h)

	repo.On("GetWorkflowByID", uint(1)).Return(&workflow.Workflow{ID: 1, Status: workflow.StatusCreated}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/1/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Len(t, broker.Sent, 1)
	assert.Equal(t, "workflow.commands.start", broker.Sent[0].Subject)
}

func TestStartWorkflow_Conflict(t *testing.T) {
	h, repo, _ := newTestHandler()
	router := setupRouter(h)

	repo.On("GetWorkflowByID", uint(1)).Return(&workflow.Workflow{ID: 1, Status: workflow.StatusRunning}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/1/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCancelWorkflow_Success(t *testing.T) {
	h, repo, broker := newTestHandler()
	router := setupRouter(h)

	repo.On("GetWorkflowByID", uint(2)).Return(&workflow.Workflow{ID: 2, Status: workflow.StatusRunning}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/2/cancel", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Len(t, broker.Sent, 1)
	assert.Equal(t, "workflow.commands.cancel", broker.Sent[0].Subject)
}

func TestCancelWorkflow_AlreadyTerminal(t *testing.T) {
	h, repo, _ := newTestHandler()
	router := setupRouter(h)

	repo.On("GetWorkflowByID", uint(2)).Return(&workflow.Workflow{ID: 2, Status: workflow.StatusCompleted}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/2/cancel", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestGetWorkflowStatus_Success(t *testing.T) {
	h, repo, _ := newTestHandler()
	router := setupRouter(h)

	now := time.Now()
	repo.On("GetWorkflowByID", uint(3)).Return(&workflow.Workflow{ID: 3, Name: "wf", Status: workflow.StatusRunning, CreatedAt: now, UpdatedAt: now}, nil)
	repo.On("GetTasksByWorkflowID", uint(3)).Return([]task.Task{
		{ID: 10, Name: "extract", Status: task.TaskStatusSucceeded},
		{ID: 11, Name: "transform", Status: task.TaskStatusRunning},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/3", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp WorkflowStatusResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, uint(3), resp.ID)
	assert.Len(t, resp.Tasks, 2)
}

func TestGetTaskResult_NotFound(t *testing.T) {
	h, repo, _ := newTestHandler()
	router := setupRouter(h)

	repo.On("GetTaskByID", uint(99)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/99/results", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
