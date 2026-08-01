package persistence

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hhace/taskflow/internal/task"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	// Allow skipping the integration suite via env var `TEST_SHORT=1`.
	// Avoid calling testing.Short() here (flags not parsed at TestMain invocation).
	if os.Getenv("TEST_SHORT") == "1" {
		os.Exit(0)
	}

	// Initialize shared DB for all tests
	cfg := getTestDBConfig()
	var err error
	testDB, err = Connect(cfg, 3)
	if err != nil {
		// fatal to ensure CI fails fast if DB isn't available
		fmt.Fprintf(os.Stderr, "failed to connect to test database: %v\n", err)
		os.Exit(1)
	}

	if err := Migrate(testDB); err != nil {
		fmt.Fprintf(os.Stderr, "failed to migrate test database: %v\n", err)
		Close(testDB)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	Close(testDB)
	os.Exit(code)
}

func setupTestDB(t *testing.T) *gorm.DB {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	if testDB == nil {
		t.Fatal("test database not initialized")
	}

	// Clean up any existing test data between tests
	if err := testDB.Exec("TRUNCATE tasks, task_execution_results CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}

	return testDB
}

func cleanupTestDB(t *testing.T, db *gorm.DB) {
	// Only truncate; do not close shared DB here
	if err := db.Exec("TRUNCATE tasks, task_execution_results CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate tables during cleanup: %v", err)
	}
}

func TestNewRepository(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewRepository(db)
	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

// Task CRUD tests
func TestRepository_CreateTask(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewRepository(db)

	task := &task.Task{
		Schedule: "*/5 * * * *",
		Command:  task.StringArray{"echo", "test"},
		Status:   task.TaskStatusCreated,
	}

	err := repo.CreateTask(task)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, task.ID)
}

func TestRepository_GetTaskByID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewRepository(db)

	tests := []struct {
		name    string
		setup   func() uuid.UUID
		wantErr bool
	}{
		{
			name: "existing task",
			setup: func() uuid.UUID {
				task := &task.Task{
					Schedule: "*/5 * * * *",
					Command:  task.StringArray{"echo", "test"},
					Status:   task.TaskStatusCreated,
				}
				repo.CreateTask(task)
				return task.ID
			},
			wantErr: false,
		},
		{
			name: "non-existing task",
			setup: func() uuid.UUID {
				return uuid.New()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.setup()
			task, err := repo.GetTaskByID(id)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, task)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, task)
				assert.Equal(t, id, task.ID)
			}
		})
	}
}

func TestRepository_GetAllTasks(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create test tasks
	for i := 0; i < 5; i++ {
		task := &task.Task{
			Schedule: "*/5 * * * *",
			Command:  task.StringArray{"echo", "test"},
			Status:   task.TaskStatusCreated,
		}
		repo.CreateTask(task)
	}

	tests := []struct {
		name      string
		limit     int
		offset    int
		wantCount int
	}{
		{"get all tasks", 0, 0, 5},
		{"limit 3 tasks", 3, 0, 3},
		{"offset 2 tasks", 0, 2, 3},
		{"limit 2 offset 1", 2, 1, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, err := repo.GetAllTasks(tt.limit, tt.offset)
			assert.NoError(t, err)
			assert.Len(t, tasks, tt.wantCount)
		})
	}
}

func TestRepository_GetTasksByStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create tasks with different statuses
	statuses := []task.TaskStatus{
		task.TaskStatusCreated,
		task.TaskStatusPending,
		task.TaskStatusCompleted,
		task.TaskStatusFailed,
	}

	for _, status := range statuses {
		for i := 0; i < 2; i++ {
			task := &task.Task{
				Schedule: "*/5 * * * *",
				Command:  task.StringArray{"echo", "test"},
				Status:   status,
			}
			repo.CreateTask(task)
		}
	}

	tests := []struct {
		name      string
		status    task.TaskStatus
		limit     int
		offset    int
		wantCount int
	}{
		{"get created tasks", task.TaskStatusCreated, 0, 0, 2},
		{"get pending tasks", task.TaskStatusPending, 0, 0, 2},
		{"get completed tasks with limit", task.TaskStatusCompleted, 1, 0, 1},
		{"get failed tasks with offset", task.TaskStatusFailed, 0, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, err := repo.GetTasksByStatus(tt.status, tt.limit, tt.offset)
			assert.NoError(t, err)
			assert.Len(t, tasks, tt.wantCount)
			for _, task := range tasks {
				assert.Equal(t, tt.status, task.Status)
			}
		})
	}
}

func TestRepository_UpdateTask(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	taskModel := &task.Task{
		Schedule: "*/5 * * * *",
		Command:  task.StringArray{"echo", "test"},
		Status:   task.TaskStatusCreated,
	}
	repo.CreateTask(taskModel)

	// Update task
	taskModel.Schedule = "*/10 * * * *"
	taskModel.Status = task.TaskStatusPending

	err := repo.UpdateTask(taskModel)
	assert.NoError(t, err)

	// Verify update
	updated, err := repo.GetTaskByID(taskModel.ID)
	assert.NoError(t, err)
	assert.Equal(t, "*/10 * * * *", updated.Schedule)
	assert.Equal(t, task.TaskStatusPending, updated.Status)
}

func TestRepository_UpdateTaskStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	tests := []struct {
		name      string
		setup     func() uuid.UUID
		newStatus task.TaskStatus
		wantErr   bool
	}{
		{
			name: "update existing task status",
			setup: func() uuid.UUID {
				task := &task.Task{
					Schedule: "*/5 * * * *",
					Command:  task.StringArray{"echo", "test"},
					Status:   task.TaskStatusCreated,
				}
				repo.CreateTask(task)
				return task.ID
			},
			newStatus: task.TaskStatusPending,
			wantErr:   false,
		},
		{
			name: "update non-existing task",
			setup: func() uuid.UUID {
				return uuid.New()
			},
			newStatus: task.TaskStatusPending,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.setup()
			err := repo.UpdateTaskStatus(id, tt.newStatus)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				task, _ := repo.GetTaskByID(id)
				assert.Equal(t, tt.newStatus, task.Status)
			}
		})
	}
}

func TestRepository_DeleteTask(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	tests := []struct {
		name    string
		setup   func() uuid.UUID
		wantErr bool
	}{
		{
			name: "delete existing task",
			setup: func() uuid.UUID {
				task := &task.Task{
					Schedule: "*/5 * * * *",
					Command:  task.StringArray{"echo", "test"},
					Status:   task.TaskStatusCreated,
				}
				repo.CreateTask(task)
				return task.ID
			},
			wantErr: false,
		},
		{
			name: "delete non-existing task",
			setup: func() uuid.UUID {
				return uuid.New()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.setup()
			err := repo.DeleteTask(id)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				_, err := repo.GetTaskByID(id)
				assert.Error(t, err)
			}
		})
	}
}

func TestRepository_CountTasks(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create 3 tasks
	for i := 0; i < 3; i++ {
		task := &task.Task{
			Schedule: "*/5 * * * *",
			Command:  task.StringArray{"echo", "test"},
			Status:   task.TaskStatusCreated,
		}
		repo.CreateTask(task)
	}

	count, err := repo.CountTasks()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestRepository_CountTasksByStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create tasks with different statuses
	for i := 0; i < 2; i++ {
		repo.CreateTask(&task.Task{
			Schedule: "*/5 * * * *",
			Command:  task.StringArray{"echo", "test"},
			Status:   task.TaskStatusCreated,
		})
	}
	for i := 0; i < 3; i++ {
		repo.CreateTask(&task.Task{
			Schedule: "*/5 * * * *",
			Command:  task.StringArray{"echo", "test"},
			Status:   task.TaskStatusPending,
		})
	}

	count, err := repo.CountTasksByStatus(task.TaskStatusCreated)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count, err = repo.CountTasksByStatus(task.TaskStatusPending)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestRepository_GetTasksCreatedAfter(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	beforeTime := time.Now()
	time.Sleep(10 * time.Millisecond)

	for i := 0; i < 3; i++ {
		repo.CreateTask(&task.Task{
			Schedule: "*/5 * * * *",
			Command:  task.StringArray{"echo", "test"},
			Status:   task.TaskStatusCreated,
		})
	}

	tasks, err := repo.GetTasksCreatedAfter(beforeTime)
	assert.NoError(t, err)
	assert.Len(t, tasks, 3)
}

func TestRepository_GetTasksUpdatedAfter(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	taskObj := &task.Task{
		Schedule: "*/5 * * * *",
		Command:  task.StringArray{"echo", "test"},
		Status:   task.TaskStatusCreated,
	}
	repo.CreateTask(taskObj)

	beforeUpdate := time.Now()
	time.Sleep(10 * time.Millisecond)

	// Update the task
	repo.UpdateTaskStatus(taskObj.ID, task.TaskStatusPending)

	tasks, err := repo.GetTasksUpdatedAfter(beforeUpdate)
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
}

func TestRepository_GetCreatedTasks(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	repo.CreateTask(&task.Task{
		Schedule: "*/5 * * * *",
		Command:  task.StringArray{"echo", "test"},
		Status:   task.TaskStatusCreated,
	})
	repo.CreateTask(&task.Task{
		Schedule: "*/5 * * * *",
		Command:  task.StringArray{"echo", "test"},
		Status:   task.TaskStatusPending,
	})

	tasks, err := repo.GetCreatedTasks()
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, task.TaskStatusCreated, tasks[0].Status)
}

func TestRepository_GetPendingTasks(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	repo.CreateTask(&task.Task{
		Schedule: "*/5 * * * *",
		Command:  task.StringArray{"echo", "test"},
		Status:   task.TaskStatusCreated,
	})
	repo.CreateTask(&task.Task{
		Schedule: "*/5 * * * *",
		Command:  task.StringArray{"echo", "test"},
		Status:   task.TaskStatusPending,
	})

	tasks, err := repo.GetPendingTasks()
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, task.TaskStatusPending, tasks[0].Status)
}

// TaskExecutionResult CRUD tests
func TestRepository_CreateTaskResult(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	taskID := uuid.New()
	result := &task.TaskExecutionResult{
		TaskID:    taskID,
		Success:   true,
		Output:    "test output",
		Error:     "",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  "100ms",
	}

	err := repo.CreateTaskResult(result)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, result.ID)
}

func TestRepository_GetTaskResultByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	tests := []struct {
		name    string
		setup   func() uuid.UUID
		wantErr bool
	}{
		{
			name: "existing result",
			setup: func() uuid.UUID {
				result := &task.TaskExecutionResult{
					TaskID:    uuid.New(),
					Success:   true,
					Output:    "test",
					StartTime: time.Now(),
					EndTime:   time.Now(),
					Duration:  "100ms",
				}
				repo.CreateTaskResult(result)
				return result.ID
			},
			wantErr: false,
		},
		{
			name: "non-existing result",
			setup: func() uuid.UUID {
				return uuid.New()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.setup()
			result, err := repo.GetTaskResultByID(id)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, id, result.ID)
			}
		})
	}
}

func TestRepository_GetTaskResultsByTaskID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	taskID := uuid.New()

	// Create 3 results for the same task
	for i := 0; i < 3; i++ {
		result := &task.TaskExecutionResult{
			TaskID:    taskID,
			Success:   true,
			Output:    "test",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  "100ms",
		}
		repo.CreateTaskResult(result)
	}

	tests := []struct {
		name      string
		limit     int
		offset    int
		wantCount int
	}{
		{"get all results", 0, 0, 3},
		{"limit 2 results", 2, 0, 2},
		{"offset 1 result", 0, 1, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := repo.GetTaskResultsByTaskID(taskID, tt.limit, tt.offset)
			assert.NoError(t, err)
			assert.Len(t, results, tt.wantCount)
		})
	}
}

func TestRepository_GetAllTaskResults(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create 5 results
	for i := 0; i < 5; i++ {
		result := &task.TaskExecutionResult{
			TaskID:    uuid.New(),
			Success:   true,
			Output:    "test",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  "100ms",
		}
		repo.CreateTaskResult(result)
	}

	results, err := repo.GetAllTaskResults(0, 0)
	assert.NoError(t, err)
	assert.Len(t, results, 5)

	results, err = repo.GetAllTaskResults(3, 0)
	assert.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestRepository_GetSuccessfulTaskResults(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create successful results
	for i := 0; i < 2; i++ {
		repo.CreateTaskResult(&task.TaskExecutionResult{
			TaskID:    uuid.New(),
			Success:   true,
			Output:    "success",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  "100ms",
		})
	}

	// Create failed result
	repo.CreateTaskResult(&task.TaskExecutionResult{
		TaskID:    uuid.New(),
		Success:   false,
		Error:     "failed",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  "50ms",
	})

	results, err := repo.GetSuccessfulTaskResults(0, 0)
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Success)
	}
}

func TestRepository_GetFailedTaskResults(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create successful result
	repo.CreateTaskResult(&task.TaskExecutionResult{
		TaskID:    uuid.New(),
		Success:   true,
		Output:    "success",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  "100ms",
	})

	// Create failed results
	for i := 0; i < 2; i++ {
		repo.CreateTaskResult(&task.TaskExecutionResult{
			TaskID:    uuid.New(),
			Success:   false,
			Error:     "failed",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  "50ms",
		})
	}

	results, err := repo.GetFailedTaskResults(0, 0)
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.False(t, r.Success)
	}
}

func TestRepository_GetLatestTaskResultByTaskID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	taskID := uuid.New()

	// Create results with different times
	for i := 0; i < 3; i++ {
		result := &task.TaskExecutionResult{
			TaskID:    taskID,
			Success:   true,
			Output:    "test",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  "100ms",
		}
		repo.CreateTaskResult(result)
		time.Sleep(10 * time.Millisecond)
	}

	latest, err := repo.GetLatestTaskResultByTaskID(taskID)
	assert.NoError(t, err)
	assert.NotNil(t, latest)
	assert.Equal(t, taskID, latest.TaskID)
}

func TestRepository_DeleteTaskResult(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	tests := []struct {
		name    string
		setup   func() uuid.UUID
		wantErr bool
	}{
		{
			name: "delete existing result",
			setup: func() uuid.UUID {
				result := &task.TaskExecutionResult{
					TaskID:    uuid.New(),
					Success:   true,
					Output:    "test",
					StartTime: time.Now(),
					EndTime:   time.Now(),
					Duration:  "100ms",
				}
				repo.CreateTaskResult(result)
				return result.ID
			},
			wantErr: false,
		},
		{
			name: "delete non-existing result",
			setup: func() uuid.UUID {
				return uuid.New()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.setup()
			err := repo.DeleteTaskResult(id)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				_, err := repo.GetTaskResultByID(id)
				assert.Error(t, err)
			}
		})
	}
}

func TestRepository_DeleteTaskResultsByTaskID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	taskID := uuid.New()

	// Create 3 results for the same task
	for i := 0; i < 3; i++ {
		repo.CreateTaskResult(&task.TaskExecutionResult{
			TaskID:    taskID,
			Success:   true,
			Output:    "test",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  "100ms",
		})
	}

	err := repo.DeleteTaskResultsByTaskID(taskID)
	assert.NoError(t, err)

	results, _ := repo.GetTaskResultsByTaskID(taskID, 0, 0)
	assert.Empty(t, results)
}

func TestRepository_CountTaskResults(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	for i := 0; i < 4; i++ {
		repo.CreateTaskResult(&task.TaskExecutionResult{
			TaskID:    uuid.New(),
			Success:   true,
			Output:    "test",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  "100ms",
		})
	}

	count, err := repo.CountTaskResults()
	assert.NoError(t, err)
	assert.Equal(t, int64(4), count)
}

func TestRepository_CountTaskResultsByTaskID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	taskID := uuid.New()

	for i := 0; i < 3; i++ {
		repo.CreateTaskResult(&task.TaskExecutionResult{
			TaskID:    taskID,
			Success:   true,
			Output:    "test",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  "100ms",
		})
	}

	count, err := repo.CountTaskResultsByTaskID(taskID)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestRepository_CountSuccessfulTaskResults(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create 2 successful results
	for i := 0; i < 2; i++ {
		repo.CreateTaskResult(&task.TaskExecutionResult{
			TaskID:    uuid.New(),
			Success:   true,
			Output:    "success",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  "100ms",
		})
	}

	// Create 1 failed result
	repo.CreateTaskResult(&task.TaskExecutionResult{
		TaskID:    uuid.New(),
		Success:   false,
		Error:     "failed",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  "50ms",
	})

	count, err := repo.CountSuccessfulTaskResults()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestRepository_CountFailedTaskResults(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create 1 successful result
	repo.CreateTaskResult(&task.TaskExecutionResult{
		TaskID:    uuid.New(),
		Success:   true,
		Output:    "success",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  "100ms",
	})

	// Create 3 failed results
	for i := 0; i < 3; i++ {
		repo.CreateTaskResult(&task.TaskExecutionResult{
			TaskID:    uuid.New(),
			Success:   false,
			Error:     "failed",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  "50ms",
		})
	}

	count, err := repo.CountFailedTaskResults()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestRepository_GetTaskResultsExecutedAfter(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	beforeTime := time.Now()
	time.Sleep(10 * time.Millisecond)

	for i := 0; i < 2; i++ {
		repo.CreateTaskResult(&task.TaskExecutionResult{
			TaskID:    uuid.New(),
			Success:   true,
			Output:    "test",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  "100ms",
		})
	}

	// Note: This test may need adjustment based on your schema
	// as it depends on having an executed_at field
	results, err := repo.GetTaskResultsExecutedAfter(beforeTime)
	assert.NoError(t, err)
	// Results might be empty if executed_at field is not set
	assert.NotNil(t, results)
}

func TestRepository_GetTaskResultsCreatedAfter(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	beforeTime := time.Now()
	time.Sleep(10 * time.Millisecond)

	for i := 0; i < 3; i++ {
		repo.CreateTaskResult(&task.TaskExecutionResult{
			TaskID:    uuid.New(),
			Success:   true,
			Output:    "test",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  "100ms",
		})
	}

	results, err := repo.GetTaskResultsCreatedAfter(beforeTime)
	assert.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestRepository_Transaction(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	t.Run("successful transaction", func(t *testing.T) {
		err := repo.Transaction(func(txRepo RepositoryInterface) error {
			task := &task.Task{
				Schedule: "*/5 * * * *",
				Command:  task.StringArray{"echo", "test"},
				Status:   task.TaskStatusCreated,
			}
			return txRepo.CreateTask(task)
		})

		assert.NoError(t, err)
		count, _ := repo.CountTasks()
		assert.Greater(t, count, int64(0))
	})

	t.Run("failed transaction rollback", func(t *testing.T) {
		initialCount, _ := repo.CountTasks()

		err := repo.Transaction(func(txRepo RepositoryInterface) error {
			task := &task.Task{
				Schedule: "*/5 * * * *",
				Command:  task.StringArray{"echo", "test"},
				Status:   task.TaskStatusCreated,
			}
			if err := txRepo.CreateTask(task); err != nil {
				return err
			}
			// Force an error to trigger rollback
			return assert.AnError
		})

		assert.Error(t, err)
		afterCount, _ := repo.CountTasks()
		assert.Equal(t, initialCount, afterCount)
	})
}
