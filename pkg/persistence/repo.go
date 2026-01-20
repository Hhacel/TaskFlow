package persistence

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hhace/taskflow/models"
	"gorm.io/gorm"
)

type RepositoryInterface interface {
	// Task methods
	CreateTask(task *models.Task) error
	GetTaskByID(id uuid.UUID) (*models.Task, error)
	GetAllTasks(limit, offset int) ([]models.Task, error)
	GetTasksByStatus(status models.TaskStatus, limit, offset int) ([]models.Task, error)
	UpdateTask(task *models.Task) error
	UpdateTaskStatus(id uuid.UUID, status models.TaskStatus) error
	DeleteTask(id uuid.UUID) error
	CountTasks() (int64, error)
	CountTasksByStatus(status models.TaskStatus) (int64, error)
	GetTasksCreatedAfter(after time.Time) ([]models.Task, error)
	GetTasksUpdatedAfter(after time.Time) ([]models.Task, error)
	GetCreatedTasks() ([]models.Task, error)
	GetPendingTasks() ([]models.Task, error)

	// TaskExecutionResult methods
	CreateTaskResult(result *models.TaskExecutionResult) error
	GetTaskResultByID(id uuid.UUID) (*models.TaskExecutionResult, error)
	GetTaskResultsByTaskID(taskID uuid.UUID, limit, offset int) ([]models.TaskExecutionResult, error)
	GetAllTaskResults(limit, offset int) ([]models.TaskExecutionResult, error)
	GetSuccessfulTaskResults(limit, offset int) ([]models.TaskExecutionResult, error)
	GetFailedTaskResults(limit, offset int) ([]models.TaskExecutionResult, error)
	GetLatestTaskResultByTaskID(taskID uuid.UUID) (*models.TaskExecutionResult, error)
	DeleteTaskResult(id uuid.UUID) error
	DeleteTaskResultsByTaskID(taskID uuid.UUID) error
	CountTaskResults() (int64, error)
	CountTaskResultsByTaskID(taskID uuid.UUID) (int64, error)
	CountSuccessfulTaskResults() (int64, error)
	CountFailedTaskResults() (int64, error)
	GetTaskResultsExecutedAfter(after time.Time) ([]models.TaskExecutionResult, error)
	GetTaskResultsCreatedAfter(after time.Time) ([]models.TaskExecutionResult, error)

	// Transaction method
	Transaction(fn func(RepositoryInterface) error) error
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db: db,
	}
}

// Create creates a new task
func (r *Repository) CreateTask(task *models.Task) error {
	return r.db.Create(task).Error
}

// GetByID retrieves a task by its ID
func (r *Repository) GetTaskByID(id uuid.UUID) (*models.Task, error) {
	var task models.Task
	err := r.db.Where("id = ?", id).First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("task with id %s not found", id)
		}
		return nil, err
	}
	return &task, nil
}

// GetAll retrieves all tasks with optional filtering
func (r *Repository) GetAllTasks(limit, offset int) ([]models.Task, error) {
	var tasks []models.Task
	query := r.db.Model(&models.Task{})

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

// GetByStatus retrieves tasks by status
func (r *Repository) GetTasksByStatus(status models.TaskStatus, limit, offset int) ([]models.Task, error) {
	var tasks []models.Task
	query := r.db.Where("status = ?", status)

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

// Update updates a task
func (r *Repository) UpdateTask(task *models.Task) error {
	return r.db.Save(task).Error
}

// UpdateStatus updates only the status of a task
func (r *Repository) UpdateTaskStatus(id uuid.UUID, status models.TaskStatus) error {
	result := r.db.Model(&models.Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("task with id %s not found", id)
	}

	return nil
}

// Delete deletes a task by ID
func (r *Repository) DeleteTask(id uuid.UUID) error {
	result := r.db.Delete(&models.Task{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("task with id %s not found", id)
	}

	return nil
}

// Count returns the total number of tasks
func (r *Repository) CountTasks() (int64, error) {
	var count int64
	err := r.db.Model(&models.Task{}).Count(&count).Error
	return count, err
}

// CountByStatus returns the number of tasks with a specific status
func (r *Repository) CountTasksByStatus(status models.TaskStatus) (int64, error) {
	var count int64
	err := r.db.Model(&models.Task{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// GetTasksCreatedAfter retrieves tasks created after a specific time
func (r *Repository) GetTasksCreatedAfter(after time.Time) ([]models.Task, error) {
	var tasks []models.Task
	err := r.db.Where("created_at > ?", after).Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

// GetTasksUpdatedAfter retrieves tasks updated after a specific time
func (r *Repository) GetTasksUpdatedAfter(after time.Time) ([]models.Task, error) {
	var tasks []models.Task
	err := r.db.Where("updated_at > ?", after).Order("updated_at DESC").Find(&tasks).Error
	return tasks, err
}

// GetCreatedTasks retrieves all created tasks
func (r *Repository) GetCreatedTasks() ([]models.Task, error) {
	return r.GetTasksByStatus(models.TaskStatusCreated, 0, 0)
}

// GetPendingTasks retrieves all pending tasks
func (r *Repository) GetPendingTasks() ([]models.Task, error) {
	return r.GetTasksByStatus(models.TaskStatusPending, 0, 0)
}

// Create creates a new task execution result
func (r *Repository) CreateTaskResult(result *models.TaskExecutionResult) error {
	taskResult := &models.TaskExecutionResult{
		ID:        uuid.New(),
		TaskID:    result.TaskID,
		Success:   result.Success,
		Output:    result.Output,
		Error:     result.Error,
		StartTime: result.StartTime,
		EndTime:   result.EndTime,
		Duration:  result.Duration,
		CreatedAt: time.Now(),
	}

	return r.db.Create(taskResult).Error
}

// GetByID retrieves a task execution result by its ID
func (r *Repository) GetTaskResultByID(id uuid.UUID) (*models.TaskExecutionResult, error) {
	var result models.TaskExecutionResult
	err := r.db.Where("id = ?", id).First(&result).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("task execution result with id %s not found", id)
		}
		return nil, err
	}
	return &result, nil
}

// GetByTaskID retrieves all execution results for a specific task
func (r *Repository) GetTaskResultsByTaskID(taskID uuid.UUID, limit, offset int) ([]models.TaskExecutionResult, error) {
	var results []models.TaskExecutionResult
	query := r.db.Where("task_id = ?", taskID)

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Order("executed_at DESC").Find(&results).Error
	return results, err
}

// GetAll retrieves all task execution results with optional filtering
func (r *Repository) GetAllTaskResults(limit, offset int) ([]models.TaskExecutionResult, error) {
	var results []models.TaskExecutionResult
	query := r.db.Model(&models.TaskExecutionResult{})

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Order("executed_at DESC").Find(&results).Error
	return results, err
}

// GetSuccessful retrieves all successful task execution results
func (r *Repository) GetSuccessfulTaskResults(limit, offset int) ([]models.TaskExecutionResult, error) {
	var results []models.TaskExecutionResult
	query := r.db.Where("success = ?", true)

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Order("executed_at DESC").Find(&results).Error
	return results, err
}

// GetFailed retrieves all failed task execution results
func (r *Repository) GetFailedTaskResults(limit, offset int) ([]models.TaskExecutionResult, error) {
	var results []models.TaskExecutionResult
	query := r.db.Where("success = ?", false)

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Order("executed_at DESC").Find(&results).Error
	return results, err
}

// GetLatestByTaskID retrieves the most recent execution result for a task
func (r *Repository) GetLatestTaskResultByTaskID(taskID uuid.UUID) (*models.TaskExecutionResult, error) {
	var result models.TaskExecutionResult
	err := r.db.Where("task_id = ?", taskID).
		Order("executed_at DESC").
		First(&result).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no execution results found for task %s", taskID)
		}
		return nil, err
	}
	return &result, nil
}

// Delete deletes a task execution result by ID
func (r *Repository) DeleteTaskResult(id uuid.UUID) error {
	result := r.db.Delete(&models.TaskExecutionResult{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("task execution result with id %s not found", id)
	}

	return nil
}

// DeleteByTaskID deletes all task execution results for a specific task
func (r *Repository) DeleteTaskResultsByTaskID(taskID uuid.UUID) error {
	return r.db.Where("task_id = ?", taskID).Delete(&models.TaskExecutionResult{}).Error
}

// Count returns the total number of task execution results
func (r *Repository) CountTaskResults() (int64, error) {
	var count int64
	err := r.db.Model(&models.TaskExecutionResult{}).Count(&count).Error
	return count, err
}

// CountByTaskID returns the number of execution results for a specific task
func (r *Repository) CountTaskResultsByTaskID(taskID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.TaskExecutionResult{}).
		Where("task_id = ?", taskID).
		Count(&count).Error
	return count, err
}

// CountSuccessful returns the number of successful task execution results
func (r *Repository) CountSuccessfulTaskResults() (int64, error) {
	var count int64
	err := r.db.Model(&models.TaskExecutionResult{}).
		Where("success = ?", true).
		Count(&count).Error
	return count, err
}

// CountFailed returns the number of failed task execution results
func (r *Repository) CountFailedTaskResults() (int64, error) {
	var count int64
	err := r.db.Model(&models.TaskExecutionResult{}).
		Where("success = ?", false).
		Count(&count).Error
	return count, err
}

// GetResultsExecutedAfter retrieves task execution results after a specific time
func (r *Repository) GetTaskResultsExecutedAfter(after time.Time) ([]models.TaskExecutionResult, error) {
	var results []models.TaskExecutionResult
	err := r.db.Where("executed_at > ?", after).
		Order("executed_at DESC").
		Find(&results).Error
	return results, err
}

// GetResultsCreatedAfter retrieves task execution results created after a specific time
func (r *Repository) GetTaskResultsCreatedAfter(after time.Time) ([]models.TaskExecutionResult, error) {
	var results []models.TaskExecutionResult
	err := r.db.Where("created_at > ?", after).
		Order("created_at DESC").
		Find(&results).Error
	return results, err
}

// Transaction executes multiple operations in a transaction
func (r *Repository) Transaction(fn func(RepositoryInterface) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepo := &Repository{db: tx}
		return fn(txRepo)
	})
}
