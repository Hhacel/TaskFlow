package persistence

import (
	"github.com/hhace/taskflow/internal/task"
	"github.com/hhace/taskflow/internal/workflow"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a testify-based mock implementing RepositoryInterface.
type MockRepository struct {
	mock.Mock
}

func NewMockRepository() *MockRepository {
	return &MockRepository{}
}

// Workflow

func (m *MockRepository) CreateWorkflow(wf *workflow.Workflow) error {
	args := m.Called(wf)
	return args.Error(0)
}

func (m *MockRepository) GetWorkflowByID(id uint) (*workflow.Workflow, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workflow.Workflow), args.Error(1)
}

func (m *MockRepository) UpdateWorkflowStatus(id uint, status workflow.Status) error {
	args := m.Called(id, status)
	return args.Error(0)
}

// Task

func (m *MockRepository) CreateTask(t *task.Task) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockRepository) GetTaskByID(id uint) (*task.Task, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Task), args.Error(1)
}

func (m *MockRepository) GetTasksByWorkflowID(workflowID uint) ([]task.Task, error) {
	args := m.Called(workflowID)
	return args.Get(0).([]task.Task), args.Error(1)
}

func (m *MockRepository) UpdateTaskStatus(id uint, status task.TaskStatus) error {
	args := m.Called(id, status)
	return args.Error(0)
}

func (m *MockRepository) CancelTasksByWorkflowID(workflowID uint) error {
	args := m.Called(workflowID)
	return args.Error(0)
}

// TaskDependency

func (m *MockRepository) CreateTaskDependency(dep *task.TaskDependency) error {
	args := m.Called(dep)
	return args.Error(0)
}

func (m *MockRepository) GetDependenciesForTask(taskID uint) ([]task.TaskDependency, error) {
	args := m.Called(taskID)
	return args.Get(0).([]task.TaskDependency), args.Error(1)
}

func (m *MockRepository) GetDependentsOfTask(taskID uint) ([]task.TaskDependency, error) {
	args := m.Called(taskID)
	return args.Get(0).([]task.TaskDependency), args.Error(1)
}

// TaskResult

func (m *MockRepository) CreateTaskResult(result *task.TaskResult) error {
	args := m.Called(result)
	return args.Error(0)
}

func (m *MockRepository) GetTaskResultsByTaskID(taskID uint) ([]task.TaskResult, error) {
	args := m.Called(taskID)
	return args.Get(0).([]task.TaskResult), args.Error(1)
}

func (m *MockRepository) GetLatestTaskResultByTaskID(taskID uint) (*task.TaskResult, error) {
	args := m.Called(taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.TaskResult), args.Error(1)
}

// Transaction

func (m *MockRepository) Transaction(fn func(RepositoryInterface) error) error {
	args := m.Called(fn)
	if args.Get(0) != nil {
		return args.Error(0)
	}
	return fn(m)
}
