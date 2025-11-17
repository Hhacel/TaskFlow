package database

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TaskRepository handles all task-related database operations
type TaskRepository struct {
	db *gorm.DB
}

// NewTaskRepository creates a new task repository
func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// Create creates a new task
func (r *TaskRepository) Create(task *Task) error {
	return r.db.Create(task).Error
}

// GetByID retrieves a task by its ID
func (r *TaskRepository) GetByID(id uuid.UUID) (*Task, error) {
	var task Task
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
func (r *TaskRepository) GetAll(limit, offset int) ([]Task, error) {
	var tasks []Task
	query := r.db.Model(&Task{})
	
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
func (r *TaskRepository) GetByStatus(status TaskStatus, limit, offset int) ([]Task, error) {
	var tasks []Task
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
func (r *TaskRepository) Update(task *Task) error {
	return r.db.Save(task).Error
}

// UpdateStatus updates only the status of a task
func (r *TaskRepository) UpdateStatus(id uuid.UUID, status TaskStatus) error {
	result := r.db.Model(&Task{}).Where("id = ?", id).Updates(map[string]interface{}{
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
func (r *TaskRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&Task{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("task with id %s not found", id)
	}
	
	return nil
}

// Count returns the total number of tasks
func (r *TaskRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&Task{}).Count(&count).Error
	return count, err
}

// CountByStatus returns the number of tasks with a specific status
func (r *TaskRepository) CountByStatus(status TaskStatus) (int64, error) {
	var count int64
	err := r.db.Model(&Task{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// GetTasksCreatedAfter retrieves tasks created after a specific time
func (r *TaskRepository) GetTasksCreatedAfter(after time.Time) ([]Task, error) {
	var tasks []Task
	err := r.db.Where("created_at > ?", after).Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

// GetTasksUpdatedAfter retrieves tasks updated after a specific time
func (r *TaskRepository) GetTasksUpdatedAfter(after time.Time) ([]Task, error) {
	var tasks []Task
	err := r.db.Where("updated_at > ?", after).Order("updated_at DESC").Find(&tasks).Error
	return tasks, err
}

// GetPendingTasks retrieves all pending tasks
func (r *TaskRepository) GetPendingTasks() ([]Task, error) {
	return r.GetByStatus(TaskStatusPending, 0, 0)
}

// GetRunningTasks retrieves all running tasks
func (r *TaskRepository) GetRunningTasks() ([]Task, error) {
	return r.GetByStatus(TaskStatusRunning, 0, 0)
}

// BatchUpdateStatus updates status for multiple tasks
func (r *TaskRepository) BatchUpdateStatus(ids []uuid.UUID, status TaskStatus) error {
	if len(ids) == 0 {
		return nil
	}
	
	return r.db.Model(&Task{}).Where("id IN ?", ids).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}).Error
}

// Transaction executes multiple operations in a transaction
func (r *TaskRepository) Transaction(fn func(*TaskRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepo := &TaskRepository{db: tx}
		return fn(txRepo)
	})
}

// GetTaskStats returns statistics about tasks
func (r *TaskRepository) GetTaskStats() (map[TaskStatus]int64, error) {
	stats := make(map[TaskStatus]int64)
	
	var results []struct {
		Status TaskStatus
		Count  int64
	}
	
	err := r.db.Model(&Task{}).
		Select("status, count(*) as count").
		Group("status").
		Find(&results).Error
	
	if err != nil {
		return nil, err
	}
	
	for _, result := range results {
		stats[result.Status] = result.Count
	}
	
	return stats, nil
}