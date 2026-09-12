// Package consumer consumes task dispatch messages published by the
// Orchestrator, executes them, and publishes the execution result back.
package consumer

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hhace/taskflow/apps/worker/config"
	"github.com/hhace/taskflow/apps/worker/internal/executor"
	"github.com/hhace/taskflow/internal/task"
	"github.com/hhace/taskflow/pkg/messaging"
)

// TaskConsumer handles consuming dispatched tasks from the broker.
type TaskConsumer struct {
	cfg      *config.Config
	broker   messaging.Broker
	executor *executor.TaskExecutor
	sub      messaging.Subscription
}

// NewTaskConsumer creates a new task consumer bound to an already-connected broker.
func NewTaskConsumer(cfg *config.Config, broker messaging.Broker) *TaskConsumer {
	return &TaskConsumer{
		cfg:      cfg,
		broker:   broker,
		executor: executor.NewTaskExecutor(cfg.DefaultTimeout()),
	}
}

// Start begins consuming dispatched tasks from the queue group.
func (c *TaskConsumer) Start() error {
	sub, err := c.broker.QueueSubscribe(c.cfg.NATS.TaskDispatchSubject, c.cfg.NATS.QueueGroupName, c.handleTask)
	if err != nil {
		return fmt.Errorf("failed to subscribe to task dispatch subject: %w", err)
	}
	c.sub = sub

	slog.Info("Worker started consuming tasks",
		"subject", c.cfg.NATS.TaskDispatchSubject,
		"queueGroup", c.cfg.NATS.QueueGroupName)

	return nil
}

// handleTask processes a single dispatched task message.
func (c *TaskConsumer) handleTask(data []byte) {
	var msg task.DispatchMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		slog.Error("Failed to unmarshal dispatch message", "error", err)
		return
	}

	slog.Info("Processing task", "taskId", msg.TaskID, "attempt", msg.Attempt)

	result := c.executor.Execute(&msg)

	if err := c.publishResult(result); err != nil {
		slog.Error("Failed to publish task result", "taskId", msg.TaskID, "error", err)
		return
	}

	slog.Info("Task completed and result published", "taskId", msg.TaskID, "success", result.Success)
}

// publishResult sends the execution result to the results subject.
func (c *TaskConsumer) publishResult(result *task.ResultMessage) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal task result: %w", err)
	}

	if err := c.broker.Publish(c.cfg.NATS.TaskResultSubject, data); err != nil {
		return fmt.Errorf("failed to publish result: %w", err)
	}

	slog.Debug("Task result published", "taskId", result.TaskID, "subject", c.cfg.NATS.TaskResultSubject)
	return nil
}

// Stop gracefully shuts down the consumer.
func (c *TaskConsumer) Stop() error {
	slog.Info("Stopping task consumer")

	if c.sub != nil {
		if err := c.sub.Unsubscribe(); err != nil {
			slog.Error("Failed to unsubscribe", "error", err)
		}
	}

	return c.broker.Close()
}
