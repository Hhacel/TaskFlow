package database

import (
	"time"

	"github.com/google/uuid"
	"github.com/hhace/taskflow/models"
	"github.com/stretchr/testify/mock"
)

type MockRepository struct {
	mock.Mock
}

func NewMockRepository() *MockRepository {
	return &MockRepository{}
}

// Task methods
func (m *MockRepository) CreateTask(task *models.Task) error {
	args := m.Called(task)
	return args.Error(0)
}

func (m *MockRepository) GetTaskByID(id uuid.UUID) (*models.Task, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockRepository) GetAllTasks(limit, offset int) ([]models.Task, error) {
	args := m.Called(limit, offset)
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockRepository) GetTasksByStatus(status models.TaskStatus, limit, offset int) ([]models.Task, error) {
	args := m.Called(status, limit, offset)
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockRepository) UpdateTask(task *models.Task) error {
	args := m.Called(task)
	return args.Error(0)
}

func (m *MockRepository) UpdateTaskStatus(id uuid.UUID, status models.TaskStatus) error {
	args := m.Called(id, status)
	return args.Error(0)
}

func (m *MockRepository) DeleteTask(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockRepository) CountTasks() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) CountTasksByStatus(status models.TaskStatus) (int64, error) {
	args := m.Called(status)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) GetTasksCreatedAfter(after time.Time) ([]models.Task, error) {
	args := m.Called(after)
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockRepository) GetTasksUpdatedAfter(after time.Time) ([]models.Task, error) {
	args := m.Called(after)
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockRepository) GetCreatedTasks() ([]models.Task, error) {
	args := m.Called()
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockRepository) GetPendingTasks() ([]models.Task, error) {
	args := m.Called()
	return args.Get(0).([]models.Task), args.Error(1)
}

// TaskExecutionResult methods
func (m *MockRepository) CreateTaskResult(result *models.TaskExecutionResult) error {
	args := m.Called(result)
	return args.Error(0)
}

func (m *MockRepository) GetTaskResultByID(id uuid.UUID) (*models.TaskExecutionResult, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskExecutionResult), args.Error(1)
}

func (m *MockRepository) GetTaskResultsByTaskID(taskID uuid.UUID, limit, offset int) ([]models.TaskExecutionResult, error) {
	args := m.Called(taskID, limit, offset)
	return args.Get(0).([]models.TaskExecutionResult), args.Error(1)
}

func (m *MockRepository) GetAllTaskResults(limit, offset int) ([]models.TaskExecutionResult, error) {
	args := m.Called(limit, offset)
	return args.Get(0).([]models.TaskExecutionResult), args.Error(1)
}

func (m *MockRepository) GetSuccessfulTaskResults(limit, offset int) ([]models.TaskExecutionResult, error) {
	args := m.Called(limit, offset)
	return args.Get(0).([]models.TaskExecutionResult), args.Error(1)
}

func (m *MockRepository) GetFailedTaskResults(limit, offset int) ([]models.TaskExecutionResult, error) {
	args := m.Called(limit, offset)
	return args.Get(0).([]models.TaskExecutionResult), args.Error(1)
}

func (m *MockRepository) GetLatestTaskResultByTaskID(taskID uuid.UUID) (*models.TaskExecutionResult, error) {
	args := m.Called(taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskExecutionResult), args.Error(1)
}

func (m *MockRepository) DeleteTaskResult(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockRepository) DeleteTaskResultsByTaskID(taskID uuid.UUID) error {
	args := m.Called(taskID)
	return args.Error(0)
}

func (m *MockRepository) CountTaskResults() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) CountTaskResultsByTaskID(taskID uuid.UUID) (int64, error) {
	args := m.Called(taskID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) CountSuccessfulTaskResults() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) CountFailedTaskResults() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) GetTaskResultsExecutedAfter(after time.Time) ([]models.TaskExecutionResult, error) {
	args := m.Called(after)
	return args.Get(0).([]models.TaskExecutionResult), args.Error(1)
}

func (m *MockRepository) GetTaskResultsCreatedAfter(after time.Time) ([]models.TaskExecutionResult, error) {
	args := m.Called(after)
	return args.Get(0).([]models.TaskExecutionResult), args.Error(1)
}

// Transaction method
func (m *MockRepository) Transaction(fn func(RepositoryInterface) error) error {
	args := m.Called(fn)
	return args.Error(0)
}
