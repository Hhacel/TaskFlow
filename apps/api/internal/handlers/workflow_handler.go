package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hhace/taskflow/apps/api/config"
	"github.com/hhace/taskflow/internal/task"
	"github.com/hhace/taskflow/internal/workflow"
	"github.com/hhace/taskflow/pkg/messaging"
	"github.com/hhace/taskflow/pkg/persistence"
)

// WorkflowHandler implements the 5 TaskFlow use cases (PU-001..PU-005) as
// plain Gin handlers. It persists workflow definitions directly and delegates
// process-control decisions (start/cancel) to the Orchestrator via NATS.
type WorkflowHandler struct {
	cfg    *config.Config
	repo   persistence.RepositoryInterface
	broker messaging.Broker
}

// NewWorkflowHandler creates a new WorkflowHandler.
func NewWorkflowHandler(cfg *config.Config, repo persistence.RepositoryInterface, broker messaging.Broker) *WorkflowHandler {
	return &WorkflowHandler{cfg: cfg, repo: repo, broker: broker}
}

// CreateWorkflow implements PU-001 Create Workflow.
func (h *WorkflowHandler) CreateWorkflow(c *gin.Context) {
	var req CreateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := validateTaskGraph(req.Tasks); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	wf := &workflow.Workflow{
		Name:   req.Name,
		Status: workflow.StatusCreated,
	}

	err := h.repo.Transaction(func(repo persistence.RepositoryInterface) error {
		if err := repo.CreateWorkflow(wf); err != nil {
			return err
		}

		refToID := make(map[string]uint, len(req.Tasks))
		for _, def := range req.Tasks {
			t := &task.Task{
				WorkflowID: wf.ID,
				Name:       def.Name,
				Command:    def.Command,
				Status:     task.TaskStatusPending,
				Timeout:    def.Timeout,
			}
			if err := repo.CreateTask(t); err != nil {
				return err
			}
			refToID[def.Ref] = t.ID
		}

		for _, def := range req.Tasks {
			for _, dep := range def.DependsOn {
				d := &task.TaskDependency{
					TaskID:          refToID[def.Ref],
					DependsOnTaskID: refToID[dep],
				}
				if err := repo.CreateTaskDependency(d); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		slog.Error("Failed to create workflow", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create workflow"})
		return
	}

	slog.Info("Workflow created", "workflowId", wf.ID)
	c.JSON(http.StatusCreated, CreateWorkflowResponse{ID: wf.ID})
}

// StartWorkflow implements PU-002 Start Workflow.
func (h *WorkflowHandler) StartWorkflow(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}

	wf, err := h.repo.GetWorkflowByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "workflow not found"})
		return
	}

	if !wf.CanTransitionTo(workflow.StatusRunning) {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "workflow cannot be started from status " + string(wf.Status)})
		return
	}

	if err := h.publishCommand(h.cfg.NATS.WorkflowStartSubject, id); err != nil {
		slog.Error("Failed to publish start command", "workflowId", id, "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to accept start request"})
		return
	}

	slog.Info("Start workflow command accepted", "workflowId", id)
	c.JSON(http.StatusAccepted, AcceptedResponse{ID: id, Message: "start request accepted"})
}

// CancelWorkflow implements PU-005 Cancel Workflow.
func (h *WorkflowHandler) CancelWorkflow(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}

	wf, err := h.repo.GetWorkflowByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "workflow not found"})
		return
	}

	if wf.IsTerminal() {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "workflow already in terminal status " + string(wf.Status)})
		return
	}

	if err := h.publishCommand(h.cfg.NATS.WorkflowCancelSubject, id); err != nil {
		slog.Error("Failed to publish cancel command", "workflowId", id, "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to accept cancel request"})
		return
	}

	slog.Info("Cancel workflow command accepted", "workflowId", id)
	c.JSON(http.StatusAccepted, AcceptedResponse{ID: id, Message: "cancel request accepted"})
}

// GetWorkflowStatus implements PU-003 Get Workflow Status.
func (h *WorkflowHandler) GetWorkflowStatus(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}

	wf, err := h.repo.GetWorkflowByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "workflow not found"})
		return
	}

	tasks, err := h.repo.GetTasksByWorkflowID(id)
	if err != nil {
		slog.Error("Failed to load tasks", "workflowId", id, "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to retrieve workflow status"})
		return
	}

	taskResponses := make([]TaskStatusResponse, 0, len(tasks))
	for _, t := range tasks {
		taskResponses = append(taskResponses, TaskStatusResponse{ID: t.ID, Name: t.Name, Status: t.Status})
	}

	c.JSON(http.StatusOK, WorkflowStatusResponse{
		ID:        wf.ID,
		Name:      wf.Name,
		Status:    wf.Status,
		CreatedAt: wf.CreatedAt,
		UpdatedAt: wf.UpdatedAt,
		Tasks:     taskResponses,
	})
}

// GetTaskResult implements PU-004 Get Task Results.
func (h *WorkflowHandler) GetTaskResult(c *gin.Context) {
	idParam := c.Param("taskId")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid task id"})
		return
	}

	if _, err := h.repo.GetTaskByID(uint(id)); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "task not found"})
		return
	}

	results, err := h.repo.GetTaskResultsByTaskID(uint(id))
	if err != nil {
		slog.Error("Failed to load task results", "taskId", id, "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to retrieve task results"})
		return
	}

	responses := make([]TaskResultResponse, 0, len(results))
	for _, r := range results {
		responses = append(responses, TaskResultResponse{
			ID:        r.ID,
			TaskID:    r.TaskID,
			Attempt:   r.Attempt,
			Success:   r.Success,
			Output:    r.Output,
			Error:     r.Error,
			StartTime: r.StartTime,
			EndTime:   r.EndTime,
		})
	}

	c.JSON(http.StatusOK, TaskResultListResponse{TaskID: uint(id), Results: responses})
}

// --- helpers -------------------------------------------------------------

func (h *WorkflowHandler) parseID(c *gin.Context) (uint, bool) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid workflow id"})
		return 0, false
	}
	return uint(id), true
}

func (h *WorkflowHandler) publishCommand(subject string, workflowID uint) error {
	if h.broker == nil {
		return errors.New("messaging broker not configured")
	}
	data, err := json.Marshal(workflow.CommandMessage{WorkflowID: workflowID})
	if err != nil {
		return err
	}
	return h.broker.Publish(subject, data)
}
