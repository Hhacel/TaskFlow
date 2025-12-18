package consumer

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hhace/taskflow/apps/scheduler/config"
	"github.com/hhace/taskflow/models"
	"github.com/hhace/taskflow/pkg/database"
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

	slog.Info("Scheduler started consuming results",
		"subject", c.config.NATS.TaskResultSubject)

	return nil
}

// handleResult processes a single result message
func (c *ResultConsumer) handleResult(msg *nats.Msg) {
	slog.Info("Received result message", "subject", msg.Subject)

	// Parse result from message
	var result models.TaskExecutionResult
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		slog.Error("Failed to unmarshal result", "error", err)
		return
	}

	slog.Info("Processing result", "taskId", result.TaskID, "success", result.Success)

	repo := database.NewRepository(c.db)
	// Update task status in database
	if result.Error == "" && result.Success == true {
		if err := repo.UpdateTaskStatus(result.TaskID, models.TaskStatusCompleted); err != nil {
			slog.Error("Failed to update task status", "taskId", result.TaskID, "error", err)
			return
		}
	} else {
		if err := repo.UpdateTaskStatus(result.TaskID, models.TaskStatusFailed); err != nil {
			slog.Error("Failed to update task status", "taskId", result.TaskID, "error", err)
			return
		}
	}

	// Save task result in database
	if err := repo.CreateTaskResult(&result); err != nil {
		slog.Error("Failed to save task result", "taskId", result.TaskID, "error", err)
		return
	}

	slog.Info("Result processed successfully", "taskId", result.TaskID)

	// TODO: Send notification to Notifier service via gRPC
}

// GetNATSConnection returns the NATS connection
func (c *ResultConsumer) GetNATSConnection() *nats.Conn {
	return c.natsConn
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
