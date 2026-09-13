package persistence

import (
	"errors"
	"fmt"
	"time"

	"github.com/hhace/taskflow/internal/task"
	"github.com/hhace/taskflow/internal/workflow"
	"gorm.io/gorm"
)

// RepositoryInterface defines every persistence operation needed by the API
// Gateway and the Orchestrator to manage Workflows, Tasks, their dependency
// graph and their execution results.
type RepositoryInterface interface {
	// Workflow
	CreateWorkflow(wf *workflow.Workflow) error
	GetWorkflowByID(id uint) (*workflow.Workflow, error)
	UpdateWorkflowStatus(id uint, status workflow.Status) error

	// Task
	CreateTask(t *task.Task) error
	GetTaskByID(id uint) (*task.Task, error)
	GetTasksByWorkflowID(workflowID uint) ([]task.Task, error)
	UpdateTaskStatus(id uint, status task.TaskStatus) error
	CancelTasksByWorkflowID(workflowID uint) error

	// TaskDependency
	CreateTaskDependency(dep *task.TaskDependency) error
	GetDependenciesForTask(taskID uint) ([]task.TaskDependency, error)
	GetDependentsOfTask(taskID uint) ([]task.TaskDependency, error)

	// TaskResult
	CreateTaskResult(result *task.TaskResult) error
	GetTaskResultsByTaskID(taskID uint) ([]task.TaskResult, error)
	GetLatestTaskResultByTaskID(taskID uint) (*task.TaskResult, error)

	// Transaction executes fn with a repository bound to a single DB transaction.
	Transaction(fn func(RepositoryInterface) error) error
}

// Repository is the GORM/PostgreSQL implementation of RepositoryInterface.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository backed by db.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// --- Workflow -------------------------------------------------------------

func (r *Repository) CreateWorkflow(wf *workflow.Workflow) error {
	return r.db.Create(wf).Error
}

func (r *Repository) GetWorkflowByID(id uint) (*workflow.Workflow, error) {
	var wf workflow.Workflow
	err := r.db.Where("id = ?", id).First(&wf).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("workflow with id %d not found", id)
		}
		return nil, err
	}
	return &wf, nil
}

func (r *Repository) UpdateWorkflowStatus(id uint, status workflow.Status) error {
	result := r.db.Model(&workflow.Workflow{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("workflow with id %d not found", id)
	}
	return nil
}

// --- Task -------------------------------------------------------------

func (r *Repository) CreateTask(t *task.Task) error {
	return r.db.Create(t).Error
}

func (r *Repository) GetTaskByID(id uint) (*task.Task, error) {
	var t task.Task
	err := r.db.Where("id = ?", id).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("task with id %d not found", id)
		}
		return nil, err
	}
	return &t, nil
}

func (r *Repository) GetTasksByWorkflowID(workflowID uint) ([]task.Task, error) {
	var tasks []task.Task
	err := r.db.Where("workflow_id = ?", workflowID).Order("id ASC").Find(&tasks).Error
	return tasks, err
}

func (r *Repository) UpdateTaskStatus(id uint, status task.TaskStatus) error {
	result := r.db.Model(&task.Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task with id %d not found", id)
	}
	return nil
}

// CancelTasksByWorkflowID transitions every non-terminal task of a workflow to CANCELLED.
func (r *Repository) CancelTasksByWorkflowID(workflowID uint) error {
	return r.db.Model(&task.Task{}).
		Where("workflow_id = ? AND status IN ?", workflowID, []task.TaskStatus{task.TaskStatusPending, task.TaskStatusRunning}).
		Updates(map[string]interface{}{
			"status":     task.TaskStatusCancelled,
			"updated_at": time.Now(),
		}).Error
}

// --- TaskDependency -------------------------------------------------------------

func (r *Repository) CreateTaskDependency(dep *task.TaskDependency) error {
	return r.db.Create(dep).Error
}

// GetDependenciesForTask returns the dependency rows describing what taskID depends on.
func (r *Repository) GetDependenciesForTask(taskID uint) ([]task.TaskDependency, error) {
	var deps []task.TaskDependency
	err := r.db.Where("task_id = ?", taskID).Find(&deps).Error
	return deps, err
}

// GetDependentsOfTask returns the dependency rows describing what depends on taskID.
func (r *Repository) GetDependentsOfTask(taskID uint) ([]task.TaskDependency, error) {
	var deps []task.TaskDependency
	err := r.db.Where("depends_on_task_id = ?", taskID).Find(&deps).Error
	return deps, err
}

// --- TaskResult -------------------------------------------------------------

func (r *Repository) CreateTaskResult(result *task.TaskResult) error {
	return r.db.Create(result).Error
}

func (r *Repository) GetTaskResultsByTaskID(taskID uint) ([]task.TaskResult, error) {
	var results []task.TaskResult
	err := r.db.Where("task_id = ?", taskID).Order("attempt ASC").Find(&results).Error
	return results, err
}

func (r *Repository) GetLatestTaskResultByTaskID(taskID uint) (*task.TaskResult, error) {
	var result task.TaskResult
	err := r.db.Where("task_id = ?", taskID).Order("attempt DESC").First(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no results found for task %d", taskID)
		}
		return nil, err
	}
	return &result, nil
}

// --- Transaction -------------------------------------------------------------

func (r *Repository) Transaction(fn func(RepositoryInterface) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepo := &Repository{db: tx}
		return fn(txRepo)
	})
}
