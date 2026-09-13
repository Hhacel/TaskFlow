package task

import "testing"

func TestTaskDependency_TableName(t *testing.T) {
	dep := TaskDependency{}
	if got := dep.TableName(); got != "task_dependencies" {
		t.Errorf("expected table name 'task_dependencies', got %s", got)
	}
}
