package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hhace/taskflow/apps/api/config"
	"github.com/hhace/taskflow/apps/api/internal/api"
	"github.com/hhace/taskflow/models"
	"github.com/hhace/taskflow/pkg/database"
	"github.com/hhace/taskflow/pkg/tfutil"
	"github.com/oapi-codegen/runtime/types"
)

// SchedulerServer implements the generated ServerInterface
type SchedulerServer struct {
	config   *config.Config
	repo *database.Repository
}

// NewSchedulerServer creates a new scheduler server
func NewSchedulerServer(cfg *config.Config, repo *database.Repository) (*SchedulerServer, error) {
	return &SchedulerServer{
		config:   cfg,
		repo: repo,
	}, nil
}

// GetHealth implements the health check endpoint
func (s *SchedulerServer) GetHealth(c *gin.Context) {
	if err := database.Health(); err != nil {
		response := api.HealthResponse{
			Status:  tfutil.ToPtr(api.Unhealthy),
			Service: tfutil.StringPtr("scheduler"),
			Error:   tfutil.StringPtr("database connection failed"),
		}
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	response := api.HealthResponse{
		Status:   tfutil.ToPtr(api.Healthy),
		Service:  tfutil.StringPtr("scheduler"),
		Database: tfutil.StringPtr("connected"),
	}
	c.JSON(http.StatusOK, response)
}

// CreateTask implements task creation
func (s *SchedulerServer) CreateTask(c *gin.Context) {
	var req api.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response := api.ErrorResponse{
			Error: tfutil.StringPtr(err.Error()),
		}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	// Create new task
	task := &models.Task{
		Schedule: req.Schedule,
		Command:  models.StringArray(req.Command),
		Status:   models.TaskStatusPending,
	}

	if err := s.repo.CreateTask(task); err != nil {
		slog.Error("Failed to create task", "error", err)
		response := api.ErrorResponse{
			Error: tfutil.StringPtr("Failed to create task"),
		}
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	slog.Info("Task created", "taskId", task.ID)

	// Convert to API response
	response := s.taskToResponse(task)
	c.JSON(http.StatusCreated, response)
}

// GetTask implements task retrieval by ID
func (s *SchedulerServer) GetTask(c *gin.Context, id types.UUID) {
	task, err := s.repo.GetTaskByID(id)
	if err != nil {
		response := api.ErrorResponse{
			Error: tfutil.StringPtr("Task not found"),
		}
		c.JSON(http.StatusNotFound, response)
		return
	}

	response := s.taskToResponse(task)
	c.JSON(http.StatusOK, response)
}

// GetTasks implements task listing with optional filtering
func (s *SchedulerServer) GetTasks(c *gin.Context, params api.GetTasksParams) {
	var tasks []models.Task
	var err error

	if params.Status != nil {
		// Convert API status to models status
		status := s.apiStatusToModelStatus(*params.Status)
		tasks, err = s.repo.GetTasksByStatus(status, 50, 0)
	} else {
		tasks, err = s.repo.GetAllTasks(50, 0)
	}

	if err != nil {
		slog.Error("Failed to retrieve tasks", "error", err)
		response := api.ErrorResponse{
			Error: tfutil.StringPtr("Failed to retrieve tasks"),
		}
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Convert to API response
	var apiTasks []api.TaskResponse
	for _, task := range tasks {
		apiTasks = append(apiTasks, s.taskToResponse(&task))
	}

	response := api.TaskListResponse{
		Tasks: tfutil.ToPtr(apiTasks),
		Count: tfutil.ToPtr(len(apiTasks)),
	}
	c.JSON(http.StatusOK, response)
}

// Helper functions

func (s *SchedulerServer) taskToResponse(task *models.Task) api.TaskResponse {
	return api.TaskResponse{
		Id:        &task.ID,
		Schedule:  &task.Schedule,
		Command:   tfutil.ToPtr([]string(task.Command)),
		Status:    tfutil.ToPtr(s.modelStatusToAPIStatus(task.Status)),
		CreatedAt: &task.CreatedAt,
		UpdatedAt: &task.UpdatedAt,
	}
}

func (s *SchedulerServer) apiStatusToModelStatus(status api.GetTasksParamsStatus) models.TaskStatus {
	switch status {
	case api.GetTasksParamsStatusPending:
		return models.TaskStatusPending
	case api.GetTasksParamsStatusCompleted:
		return models.TaskStatusCompleted
	case api.GetTasksParamsStatusFailed:
		return models.TaskStatusFailed
	default:
		return models.TaskStatusCreated
	}
}

func (s *SchedulerServer) modelStatusToAPIStatus(status models.TaskStatus) api.TaskResponseStatus {
	switch status {
	case models.TaskStatusCreated:
		return api.TaskResponseStatusPending
	case models.TaskStatusPending:
		return api.TaskResponseStatusPending
	case models.TaskStatusCompleted:
		return api.TaskResponseStatusCompleted
	case models.TaskStatusFailed:
		return api.TaskResponseStatusFailed
	default:
		return api.TaskResponseStatusPending
	}
}
