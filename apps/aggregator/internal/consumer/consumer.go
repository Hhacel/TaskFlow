package consumer

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hhace/taskflow/apps/aggregator/config"
	"github.com/hhace/taskflow/models"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
)

// ResultConsumer handles consuming task results from NATS queue
type ResultConsumer struct {
	config   *config.Config
	natsConn *nats.Conn
	db       *gorm.DB
	sub      *nats.Subscription
}

// TaskResult represents the result message from worker
type TaskResult struct {
	TaskID    string    `json:"task_id"`
	Success   bool      `json:"success"`
	Output    string    `json:"output"`
	Error     string    `json:"error,omitempty"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  string    `json:"duration"`
}

// NewResultConsumer creates a new result consumer
func NewResultConsumer(cfg *config.Config, db *gorm.DB) (*ResultConsumer, error) {
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
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	slog.Info("Connected to NATS", "url", cfg.NATS.URL)

	return &ResultConsumer{
		config:   cfg,
		natsConn: nc,
		db:       db,
	}, nil
}

// Start begins consuming results from the queue
func (c *ResultConsumer) Start() error {
	var err error
	c.sub, err = c.natsConn.Subscribe(
		c.config.NATS.TaskResultSubject,
		c.handleResult,
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe to result queue: %w", err)
	}

	slog.Info("Aggregator started consuming results",
		"subject", c.config.NATS.TaskResultSubject)

	return nil
}

// handleResult processes a single result message
func (c *ResultConsumer) handleResult(msg *nats.Msg) {
	slog.Debug("Received result message", "subject", msg.Subject)

	// Parse result from message
	var result TaskResult
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		slog.Error("Failed to unmarshal result", "error", err)
		return
	}

	slog.Info("Processing result", "taskId", result.TaskID, "success", result.Success)

	// Parse task UUID
	taskUUID, err := uuid.Parse(result.TaskID)
	if err != nil {
		slog.Error("Invalid task ID format", "taskId", result.TaskID, "error", err)
		return
	}

	// Update task status in database
	if err := c.updateTaskStatus(taskUUID, result.Success); err != nil {
		slog.Error("Failed to update task status", "taskId", result.TaskID, "error", err)
		return
	}

	// Save task result in database
	if err := c.saveTaskResult(taskUUID, &result); err != nil {
		slog.Error("Failed to save task result", "taskId", result.TaskID, "error", err)
		return
	}

	slog.Info("Result processed successfully", "taskId", result.TaskID)

	// TODO: Send notification to Notifier service via gRPC
}

// updateTaskStatus updates the task status in the database
func (c *ResultConsumer) updateTaskStatus(taskID uuid.UUID, success bool) error {
	status := models.TaskStatusCompleted
	if !success {
		status = models.TaskStatusFailed
	}

	result := c.db.Model(&models.Task{}).
		Where("id = ?", taskID).
		Update("status", status)

	if result.Error != nil {
		return fmt.Errorf("failed to update task status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	slog.Debug("Task status updated", "taskId", taskID, "status", status)
	return nil
}

// saveTaskResult saves the execution result in the database
func (c *ResultConsumer) saveTaskResult(taskID uuid.UUID, result *TaskResult) error {
	// Create a TaskResult model
	taskResult := &models.TaskExecutionResult{
		ID:        uuid.New(),
		TaskID:    taskID,
		Success:   result.Success,
		Output:    result.Output,
		Error:     result.Error,
		StartTime: result.StartTime,
		EndTime:   result.EndTime,
		Duration:  result.Duration,
		CreatedAt: time.Now(),
	}

	if err := c.db.Create(taskResult).Error; err != nil {
		return fmt.Errorf("failed to create task result: %w", err)
	}

	slog.Debug("Task result saved", "taskId", taskID, "resultId", taskResult.ID)
	return nil
}

// Stop gracefully shuts down the consumer
func (c *ResultConsumer) Stop() error {
	slog.Info("Stopping result consumer")

	if c.sub != nil {
		if err := c.sub.Unsubscribe(); err != nil {
			slog.Error("Failed to unsubscribe", "error", err)
		}
	}

	if c.natsConn != nil {
		c.natsConn.Close()
		slog.Info("NATS connection closed")
	}

	return nil
}
