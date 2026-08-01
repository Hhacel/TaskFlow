package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskExecutionResult_TableName(t *testing.T) {
	result := TaskExecutionResult{}
	assert.Equal(t, "task_execution_results", result.TableName())
}
