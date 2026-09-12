package task

import "testing"

func TestTaskResult_TableName(t *testing.T) {
	result := TaskResult{}
	if got := result.TableName(); got != "task_results" {
		t.Errorf("expected table name 'task_results', got %s", got)
	}
}
