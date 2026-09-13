package consumer

import (
	"encoding/json"
	"testing"

	"github.com/hhace/taskflow/apps/worker/config"
	"github.com/hhace/taskflow/internal/task"
	"github.com/hhace/taskflow/pkg/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Worker.DefaultTimeoutSeconds = 5
	return cfg
}

func TestNewTaskConsumer(t *testing.T) {
	broker := messaging.NewMockBroker()
	c := NewTaskConsumer(testConfig(), broker)

	require.NotNil(t, c)
	assert.NotNil(t, c.executor)
	assert.Nil(t, c.sub)
}

func TestTaskConsumer_StartSubscribes(t *testing.T) {
	broker := messaging.NewMockBroker()
	c := NewTaskConsumer(testConfig(), broker)

	err := c.Start()
	require.NoError(t, err)
	assert.NotNil(t, c.sub)
}

func TestTaskConsumer_HandleTask_PublishesResult(t *testing.T) {
	broker := messaging.NewMockBroker()
	cfg := testConfig()
	c := NewTaskConsumer(cfg, broker)

	require.NoError(t, c.Start())

	msg := task.DispatchMessage{TaskID: 42, WorkflowID: 1, Attempt: 1, Command: "echo hello"}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	require.NoError(t, broker.Publish(cfg.NATS.TaskDispatchSubject, data))

	// Sent[0] is the dispatch message published above; Sent[1] is the result
	// message the consumer publishes back after executing the task.
	require.Len(t, broker.Sent, 2)
	assert.Equal(t, cfg.NATS.TaskResultSubject, broker.Sent[1].Subject)

	var result task.ResultMessage
	require.NoError(t, json.Unmarshal(broker.Sent[1].Data, &result))
	assert.Equal(t, uint(42), result.TaskID)
	assert.True(t, result.Success)
}

func TestTaskConsumer_Stop(t *testing.T) {
	broker := messaging.NewMockBroker()
	c := NewTaskConsumer(testConfig(), broker)

	require.NoError(t, c.Start())
	assert.NoError(t, c.Stop())
}
