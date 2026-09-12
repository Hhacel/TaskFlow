package task

// TaskDependency represents an edge in the DAG of a Workflow: TaskID depends on
// DependsOnTaskID, meaning TaskID may only start once DependsOnTaskID has succeeded.
type TaskDependency struct {
	TaskID          uint `gorm:"primaryKey;autoIncrement:false" json:"task_id"`
	DependsOnTaskID uint `gorm:"primaryKey;autoIncrement:false" json:"depends_on_task_id"`
}

// TableName specifies the table name for GORM
func (TaskDependency) TableName() string {
	return "task_dependencies"
}
