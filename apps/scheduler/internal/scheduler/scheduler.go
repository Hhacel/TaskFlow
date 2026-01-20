package scheduler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hhace/taskflow/apps/scheduler/config"
	"github.com/hhace/taskflow/models"
	"github.com/hhace/taskflow/pkg/persistence"
	"github.com/nats-io/nats.go"
	"github.com/robfig/cron/v3"
)

// TaskScheduler manages scheduled task execution
type TaskScheduler struct {
	config      *config.Config
	repo        persistence.RepositoryInterface
	natsConn    *nats.Conn
	cron        *cron.Cron
	jobs        map[uuid.UUID]cron.EntryID // maps task ID to cron entry ID
	mu          sync.RWMutex
	stopRefresh chan struct{}
}

// NewTaskScheduler creates a new task scheduler
func NewTaskScheduler(cfg *config.Config, repo persistence.RepositoryInterface, natsConn *nats.Conn) *TaskScheduler {
	return &TaskScheduler{
		config:      cfg,
		repo:        repo,
		natsConn:    natsConn,
		cron:        cron.New(cron.WithSeconds()), // Enable seconds in cron expressions
		jobs:        make(map[uuid.UUID]cron.EntryID),
		stopRefresh: make(chan struct{}),
	}
}

// Start loads all created tasks and starts the cron scheduler
func (ts *TaskScheduler) Start() error {
	slog.Info("Starting task scheduler")

	// Load all created tasks from database
	if err := ts.loadTasks(); err != nil {
		return fmt.Errorf("failed to load tasks: %w", err)
	}

	// Start the cron scheduler
	ts.cron.Start()
	slog.Info("Task scheduler started successfully", "jobCount", len(ts.jobs))

	// Start periodic refresh in background
	go ts.periodicRefresh()

	return nil
}

// Stop stops the cron scheduler
func (ts *TaskScheduler) Stop() {
	slog.Info("Stopping task scheduler")
	close(ts.stopRefresh)
	ts.cron.Stop()
}

// loadTasks loads all created tasks from the database and schedules them
func (ts *TaskScheduler) loadTasks() error {
	tasks, err := ts.repo.GetAllTasks(0, 0)
	if err != nil {
		return fmt.Errorf("failed to fetch tasks from database: %w", err)
	}

	slog.Debug("Found created tasks in database", "count", len(tasks))

	// Build a map of task IDs from database
	dbTaskIDs := make(map[uuid.UUID]bool)
	for _, task := range tasks {
		dbTaskIDs[task.ID] = true
	}

	// Remove tasks that are no longer in the database
	ts.mu.Lock()
	for taskID := range ts.jobs {
		if !dbTaskIDs[taskID] {
			entryID := ts.jobs[taskID]
			ts.cron.Remove(entryID)
			delete(ts.jobs, taskID)
			slog.Info("Unscheduled task", "taskId", taskID)
		}
	}
	ts.mu.Unlock()

	// Add new tasks
	for _, task := range tasks {
		if err := ts.AddTask(&task); err != nil {
			slog.Error("Failed to schedule task", "taskId", task.ID, "error", err)
			continue
		}
	}

	return nil
}

// AddTask schedules a new task
func (ts *TaskScheduler) AddTask(task *models.Task) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Check if task is already scheduled
	if _, exists := ts.jobs[task.ID]; exists {
		slog.Debug("Task already scheduled, skipping", "taskId", task.ID)
		return nil // Return nil instead of error - not scheduling is not an error
	}

	// Add task to cron
	entryID, err := ts.cron.AddFunc(task.Schedule, func() {
		ts.executeTask(task)
	})
	if err != nil {
		return fmt.Errorf("invalid cron expression '%s': %w", task.Schedule, err)
	}

	ts.jobs[task.ID] = entryID
	slog.Info("Task scheduled", "taskId", task.ID, "schedule", task.Schedule)

	return nil
}

// RemoveTask removes a task from the scheduler
func (ts *TaskScheduler) RemoveTask(taskID uuid.UUID) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	entryID, exists := ts.jobs[taskID]
	if !exists {
		return fmt.Errorf("task not scheduled: %s", taskID)
	}

	ts.cron.Remove(entryID)
	delete(ts.jobs, taskID)
	slog.Info("Task unscheduled", "taskId", taskID)

	return nil
}

// executeTask publishes the task to NATS for worker execution
func (ts *TaskScheduler) executeTask(task *models.Task) {
	slog.Info("Executing scheduled task", "taskId", task.ID, "schedule", task.Schedule)

	// Update task status to pending (ready for worker)
	if err := ts.repo.UpdateTaskStatus(task.ID, models.TaskStatusPending); err != nil {
		slog.Error("Failed to update task status to pending", "taskId", task.ID, "error", err)
		return
	}

	// Serialize task to JSON
	taskJSON, err := json.Marshal(task)
	if err != nil {
		slog.Error("Failed to marshal task", "taskId", task.ID, "error", err)
		return
	}

	// Publish task to NATS
	if err := ts.natsConn.Publish(ts.config.NATS.TaskScheduleSubject, taskJSON); err != nil {
		slog.Error("Failed to publish task to NATS", "taskId", task.ID, "error", err)

		// Revert status back to created
		if updateErr := ts.repo.UpdateTaskStatus(task.ID, models.TaskStatusCreated); updateErr != nil {
			slog.Error("Failed to revert task status to created", "taskId", task.ID, "error", updateErr)
		}
		return
	}

	slog.Info("Task published to NATS", "taskId", task.ID, "subject", ts.config.NATS.TaskScheduleSubject)
}

// periodicRefresh periodically checks for new created tasks and schedules them
func (ts *TaskScheduler) periodicRefresh() {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	slog.Info("Started periodic task refresh", "interval", "10s")

	for {
		select {
		case <-ticker.C:
			slog.Debug("Refreshing tasks from database")
			if err := ts.loadTasks(); err != nil {
				slog.Error("Failed to refresh tasks", "error", err)
			}
		case <-ts.stopRefresh:
			slog.Info("Stopped periodic task refresh")
			return
		}
	}
}
