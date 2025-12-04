package consumer

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hhace/taskflow/apps/worker/config"
	"github.com/hhace/taskflow/apps/worker/internal/executor"
	"github.com/hhace/taskflow/models"
	"github.com/nats-io/nats.go"
)

// TaskConsumer handles consuming tasks from NATS queue
type TaskConsumer struct {
	config   *config.Config
	natsConn *nats.Conn
	executor *executor.TaskExecutor
	sub      *nats.Subscription
}

// NewTaskConsumer creates a new task consumer
func NewTaskConsumer(cfg *config.Config) (*TaskConsumer, error) {
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

	return &TaskConsumer{
		config:   cfg,
		natsConn: nc,
		executor: executor.NewTaskExecutor(cfg.GetTaskTimeout()),
	}, nil
}

// Start begins consuming tasks from the queue
func (c *TaskConsumer) Start() error {
	var err error
	c.sub, err = c.natsConn.QueueSubscribe(
		c.config.NATS.TaskScheduleSubject,
		c.config.NATS.QueueGroupName,
		c.handleTask,
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe to task queue: %w", err)
	}

	slog.Info("Worker started consuming tasks",
		"subject", c.config.NATS.TaskScheduleSubject,
		"queueGroup", c.config.NATS.QueueGroupName)

	return nil
}

// handleTask processes a single task message
func (c *TaskConsumer) handleTask(msg *nats.Msg) {
	slog.Debug("Received task message", "subject", msg.Subject)

	// Parse task from message
	var task models.Task
	if err := json.Unmarshal(msg.Data, &task); err != nil {
		slog.Error("Failed to unmarshal task", "error", err)
		return
	}

	slog.Info("Processing task", "taskId", task.ID)

	// Execute the task
	result := c.executor.Execute(&task)

	// Publish result to results queue
	if err := c.publishResult(result); err != nil {
		slog.Error("Failed to publish task result", "taskId", task.ID, "error", err)
		return
	}

	slog.Info("Task completed and result published", "taskId", task.ID, "success", result.Success)
}

// TaskResult represents the result message sent to aggregator
type TaskResult struct {
	TaskID    string    `json:"task_id"`
	Success   bool      `json:"success"`
	Output    string    `json:"output"`
	Error     string    `json:"error,omitempty"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  string    `json:"duration"`
}

// publishResult sends the execution result to the results queue
func (c *TaskConsumer) publishResult(result *executor.ExecutionResult) error {
	taskResult := TaskResult{
		TaskID:    result.TaskID,
		Success:   result.Success,
		Output:    result.Output,
		Error:     result.Error,
		StartTime: result.StartTime,
		EndTime:   result.EndTime,
		Duration:  result.EndTime.Sub(result.StartTime).String(),
	}

	resultJSON, err := json.Marshal(taskResult)
	if err != nil {
		return fmt.Errorf("failed to marshal task result: %w", err)
	}

	if err := c.natsConn.Publish(c.config.NATS.TaskResultSubject, resultJSON); err != nil {
		return fmt.Errorf("failed to publish result to NATS: %w", err)
	}

	slog.Debug("Task result published",
		"taskId", result.TaskID,
		"subject", c.config.NATS.TaskResultSubject)

	return nil
}

// Stop gracefully shuts down the consumer
func (c *TaskConsumer) Stop() error {
	slog.Info("Stopping task consumer")

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
