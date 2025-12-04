package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hhace/taskflow/apps/scheduler/config"
	"github.com/hhace/taskflow/apps/scheduler/internal/api"
	"github.com/hhace/taskflow/models"
	"github.com/hhace/taskflow/pkg/database"
	"github.com/hhace/taskflow/pkg/tfutil"
	"github.com/nats-io/nats.go"
	"github.com/oapi-codegen/runtime/types"
)

// SchedulerServer implements the generated ServerInterface
type SchedulerServer struct {
	config   *config.Config
	taskRepo *database.TaskRepository
	natsConn *nats.Conn
}

// NewSchedulerServer creates a new scheduler server
func NewSchedulerServer(cfg *config.Config, taskRepo *database.TaskRepository) (*SchedulerServer, error) {
	// Connect to NATS with reconnect options
	opts := []nats.Option{
		nats.ReconnectWait(time.Duration(cfg.NATS.ReconnectWait) * time.Second),
		nats.MaxReconnects(cfg.NATS.MaxReconnects),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				slog.Warn("NATS disconnected", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
	}

	nc, err := nats.Connect(cfg.NATS.URL, opts...)
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err)
		return nil, err
	}

	slog.Info("Connected to NATS", "url", cfg.NATS.URL)

	return &SchedulerServer{
		config:   cfg,
		taskRepo: taskRepo,
		natsConn: nc,
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

	if err := s.taskRepo.Create(task); err != nil {
		slog.Error("Failed to create task", "error", err)
		response := api.ErrorResponse{
			Error: tfutil.StringPtr("Failed to create task"),
		}
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Publish task to NATS queue here
	if err := s.publishTaskToQueue(task); err != nil {
		slog.Error("Failed to publish task to queue", "taskId", task.ID, "error", err)
	}
	
	slog.Info("Task created", "taskId", task.ID)

	// Convert to API response
	response := s.taskToResponse(task)
	c.JSON(http.StatusCreated, response)
}

// GetTask implements task retrieval by ID
func (s *SchedulerServer) GetTask(c *gin.Context, id types.UUID) {
	task, err := s.taskRepo.GetByID(id)
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
		tasks, err = s.taskRepo.GetByStatus(status, 50, 0)
	} else {
		tasks, err = s.taskRepo.GetAll(50, 0)
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
	case api.GetTasksParamsStatusRunning:
		return models.TaskStatusRunning
	case api.GetTasksParamsStatusCompleted:
		return models.TaskStatusCompleted
	case api.GetTasksParamsStatusFailed:
		return models.TaskStatusFailed
	default:
		return models.TaskStatusPending
	}
}

func (s *SchedulerServer) modelStatusToAPIStatus(status models.TaskStatus) api.TaskResponseStatus {
	switch status {
	case models.TaskStatusPending:
		return api.TaskResponseStatusPending
	case models.TaskStatusRunning:
		return api.TaskResponseStatusRunning
	case models.TaskStatusCompleted:
		return api.TaskResponseStatusCompleted
	case models.TaskStatusFailed:
		return api.TaskResponseStatusFailed
	default:
		return api.TaskResponseStatusPending
	}
}

func (s *SchedulerServer) publishTaskToQueue(task *models.Task) error {
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task to JSON: %w", err)
	}

	if err := s.natsConn.Publish(s.config.NATS.TaskScheduleSubject, taskJSON); err != nil {
		return fmt.Errorf("failed to publish task to NATS: %w", err)
	}

	slog.Debug("Task published to queue", "taskId", task.ID, "subject", s.config.NATS.TaskScheduleSubject)
	return nil
}