package consumer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hhace/taskflow/apps/worker/config"
	"github.com/hhace/taskflow/internal/task"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startNATSServer starts an in-memory NATS server for testing
func startNATSServer(t *testing.T) (*server.Server, *nats.Conn) {
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1, // random port
	}

	ns, err := server.NewServer(opts)
	require.NoError(t, err)

	go ns.Start()

	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)

	return ns, nc
}

func TestNewTaskConsumer(t *testing.T) {
	tests := []struct {
		name        string
		setupServer bool
		config      *config.Config
		expectError bool
	}{
		{
			name:        "creates consumer successfully with valid config",
			setupServer: true,
			config: &config.Config{
				NATS: config.NATSConfig{
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				Worker: config.WorkerConfig{
					TaskTimeoutMinutes: 5,
				},
			},
			expectError: false,
		},
		{
			name:        "fails with invalid NATS URL",
			setupServer: false,
			config: &config.Config{
				NATS: config.NATSConfig{
					URL:                 "nats://invalid-host:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       1,
					MaxReconnects:       1,
				},
				Worker: config.WorkerConfig{
					TaskTimeoutMinutes: 5,
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ns *server.Server
			var nc *nats.Conn

			if tt.setupServer {
				ns, nc = startNATSServer(t)
				defer ns.Shutdown()
				defer nc.Close()
				tt.config.NATS.URL = ns.ClientURL()
			}

			consumer, err := NewTaskConsumer(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, consumer)
			} else {
				require.NoError(t, err)
				require.NotNil(t, consumer)
				assert.NotNil(t, consumer.config)
				assert.NotNil(t, consumer.natsConn)
				assert.NotNil(t, consumer.executor)
				assert.Nil(t, consumer.sub)

				// Clean up
				consumer.Stop()
			}
		})
	}
}

func TestTaskConsumer_Start(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
	}{
		{
			name: "starts successfully with valid configuration",
			config: &config.Config{
				NATS: config.NATSConfig{
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				Worker: config.WorkerConfig{
					TaskTimeoutMinutes: 5,
				},
			},
			expectError: false,
		},
		{
			name: "starts with different queue group name",
			config: &config.Config{
				NATS: config.NATSConfig{
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "custom-workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				Worker: config.WorkerConfig{
					TaskTimeoutMinutes: 10,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, nc := startNATSServer(t)
			defer ns.Shutdown()
			defer nc.Close()

			tt.config.NATS.URL = ns.ClientURL()

			consumer, err := NewTaskConsumer(tt.config)
			require.NoError(t, err)
			defer consumer.Stop()

			err = consumer.Start()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, consumer.sub)
				assert.True(t, consumer.sub.IsValid())
			}
		})
	}
}

func TestTaskConsumer_HandleTask(t *testing.T) {
	tests := []struct {
		name           string
		task           *task.Task
		validateResult func(t *testing.T, result *task.TaskExecutionResult)
	}{
		{
			name: "processes valid task successfully",
			task: &task.Task{
				ID:       uuid.New(),
				Schedule: "0 */5 * * * *",
				Command:  task.StringArray{"echo", "test"},
				Status:   task.TaskStatusPending,
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				assert.NotEqual(t, uuid.Nil, result.TaskID)
				assert.False(t, result.StartTime.IsZero())
				assert.False(t, result.EndTime.IsZero())
			},
		},
		{
			name: "processes task with empty command",
			task: &task.Task{
				ID:       uuid.New(),
				Schedule: "0 */10 * * * *",
				Command:  task.StringArray{},
				Status:   task.TaskStatusPending,
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				assert.False(t, result.Success)
				assert.Equal(t, "empty command", result.Error)
			},
		},
		{
			name: "processes task with invalid command",
			task: &task.Task{
				ID:       uuid.New(),
				Schedule: "0 */15 * * * *",
				Command:  task.StringArray{"nonexistentcommand12345"},
				Status:   task.TaskStatusPending,
			},
			validateResult: func(t *testing.T, result *task.TaskExecutionResult) {
				assert.False(t, result.Success)
				assert.NotEmpty(t, result.Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, nc := startNATSServer(t)
			defer ns.Shutdown()
			defer nc.Close()

			cfg := &config.Config{
				NATS: config.NATSConfig{
					URL:                 ns.ClientURL(),
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				Worker: config.WorkerConfig{
					TaskTimeoutMinutes: 5,
				},
			}

			consumer, err := NewTaskConsumer(cfg)
			require.NoError(t, err)
			defer consumer.Stop()

			err = consumer.Start()
			require.NoError(t, err)

			// Subscribe to results to capture published result
			resultChan := make(chan *task.TaskExecutionResult, 1)
			_, err = nc.Subscribe(cfg.NATS.TaskResultSubject, func(msg *nats.Msg) {
				var result task.TaskExecutionResult
				if err := json.Unmarshal(msg.Data, &result); err == nil {
					resultChan <- &result
				}
			})
			require.NoError(t, err)
			nc.Flush()

			// Publish task
			taskJSON, err := json.Marshal(tt.task)
			require.NoError(t, err)

			err = nc.Publish(cfg.NATS.TaskScheduleSubject, taskJSON)
			require.NoError(t, err)

			// Wait for result
			select {
			case result := <-resultChan:
				assert.Equal(t, tt.task.ID, result.TaskID)
				tt.validateResult(t, result)
			case <-time.After(2 * time.Second):
				t.Fatal("Timeout waiting for task result")
			}
		})
	}
}

func TestTaskConsumer_PublishResult(t *testing.T) {
	tests := []struct {
		name   string
		result *task.TaskExecutionResult
	}{
		{
			name: "publishes successful result",
			result: &task.TaskExecutionResult{
				TaskID:    uuid.New(),
				Success:   true,
				Output:    "test output",
				Error:     "",
				StartTime: time.Now().Add(-1 * time.Second),
				EndTime:   time.Now(),
			},
		},
		{
			name: "publishes failed result",
			result: &task.TaskExecutionResult{
				TaskID:    uuid.New(),
				Success:   false,
				Output:    "",
				Error:     "command failed",
				StartTime: time.Now().Add(-1 * time.Second),
				EndTime:   time.Now(),
			},
		},
		{
			name: "publishes result with empty output",
			result: &task.TaskExecutionResult{
				TaskID:    uuid.New(),
				Success:   true,
				Output:    "",
				Error:     "",
				StartTime: time.Now(),
				EndTime:   time.Now(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, nc := startNATSServer(t)
			defer ns.Shutdown()
			defer nc.Close()

			cfg := &config.Config{
				NATS: config.NATSConfig{
					URL:                 ns.ClientURL(),
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				Worker: config.WorkerConfig{
					TaskTimeoutMinutes: 5,
				},
			}

			consumer, err := NewTaskConsumer(cfg)
			require.NoError(t, err)
			defer consumer.Stop()

			// Subscribe to results
			resultChan := make(chan *task.TaskExecutionResult, 1)
			_, err = nc.Subscribe(cfg.NATS.TaskResultSubject, func(msg *nats.Msg) {
				var result task.TaskExecutionResult
				if err := json.Unmarshal(msg.Data, &result); err == nil {
					resultChan <- &result
				}
			})
			require.NoError(t, err)
			nc.Flush()

			// Publish result
			err = consumer.publishResult(tt.result)
			assert.NoError(t, err)

			// Verify result was published
			select {
			case receivedResult := <-resultChan:
				assert.Equal(t, tt.result.TaskID, receivedResult.TaskID)
				assert.Equal(t, tt.result.Success, receivedResult.Success)
				assert.Equal(t, tt.result.Output, receivedResult.Output)
				assert.Equal(t, tt.result.Error, receivedResult.Error)
			case <-time.After(1 * time.Second):
				t.Fatal("Timeout waiting for published result")
			}
		})
	}
}

func TestTaskConsumer_Stop(t *testing.T) {
	tests := []struct {
		name       string
		setupStart bool
	}{
		{
			name:       "stops consumer that was started",
			setupStart: true,
		},
		{
			name:       "stops consumer that was not started",
			setupStart: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, nc := startNATSServer(t)
			defer ns.Shutdown()
			defer nc.Close()

			cfg := &config.Config{
				NATS: config.NATSConfig{
					URL:                 ns.ClientURL(),
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				Worker: config.WorkerConfig{
					TaskTimeoutMinutes: 5,
				},
			}

			consumer, err := NewTaskConsumer(cfg)
			require.NoError(t, err)

			if tt.setupStart {
				err = consumer.Start()
				require.NoError(t, err)
				assert.NotNil(t, consumer.sub)
			}

			err = consumer.Stop()
			assert.NoError(t, err)

			// Verify connection is closed
			assert.True(t, consumer.natsConn.IsClosed())
		})
	}
}

func TestTaskConsumer_InvalidJSON(t *testing.T) {
	ns, nc := startNATSServer(t)
	defer ns.Shutdown()
	defer nc.Close()

	cfg := &config.Config{
		NATS: config.NATSConfig{
			URL:                 ns.ClientURL(),
			TaskScheduleSubject: "tasks.schedule",
			TaskResultSubject:   "tasks.results",
			QueueGroupName:      "workers",
			ReconnectWait:       2,
			MaxReconnects:       60,
		},
		Worker: config.WorkerConfig{
			TaskTimeoutMinutes: 5,
		},
	}

	consumer, err := NewTaskConsumer(cfg)
	require.NoError(t, err)
	defer consumer.Stop()

	err = consumer.Start()
	require.NoError(t, err)

	// Subscribe to results - should not receive anything for invalid JSON
	resultChan := make(chan *task.TaskExecutionResult, 1)
	_, err = nc.Subscribe(cfg.NATS.TaskResultSubject, func(msg *nats.Msg) {
		var result task.TaskExecutionResult
		if err := json.Unmarshal(msg.Data, &result); err == nil {
			resultChan <- &result
		}
	})
	require.NoError(t, err)

	// Publish invalid JSON
	err = nc.Publish(cfg.NATS.TaskScheduleSubject, []byte("invalid json"))
	require.NoError(t, err)

	// Should not receive any result
	select {
	case <-resultChan:
		t.Fatal("Should not receive result for invalid JSON")
	case <-time.After(500 * time.Millisecond):
		// Expected - no result published
	}
}

func TestTaskConsumer_MultipleTasksSequential(t *testing.T) {
	ns, nc := startNATSServer(t)
	defer ns.Shutdown()
	defer nc.Close()

	cfg := &config.Config{
		NATS: config.NATSConfig{
			URL:                 ns.ClientURL(),
			TaskScheduleSubject: "tasks.schedule",
			TaskResultSubject:   "tasks.results",
			QueueGroupName:      "workers",
			ReconnectWait:       2,
			MaxReconnects:       60,
		},
		Worker: config.WorkerConfig{
			TaskTimeoutMinutes: 5,
		},
	}

	consumer, err := NewTaskConsumer(cfg)
	require.NoError(t, err)
	defer consumer.Stop()

	err = consumer.Start()
	require.NoError(t, err)

	// Subscribe to results
	resultChan := make(chan *task.TaskExecutionResult, 3)
	_, err = nc.Subscribe(cfg.NATS.TaskResultSubject, func(msg *nats.Msg) {
		var result task.TaskExecutionResult
		if err := json.Unmarshal(msg.Data, &result); err == nil {
			resultChan <- &result
		}
	})
	require.NoError(t, err)
	nc.Flush()

	// Create and publish multiple tasks
	tasks := []*task.Task{
		{
			ID:       uuid.New(),
			Schedule: "0 */5 * * * *",
			Command:  task.StringArray{"echo", "task1"},
			Status:   task.TaskStatusPending,
		},
		{
			ID:       uuid.New(),
			Schedule: "0 */10 * * * *",
			Command:  task.StringArray{"echo", "task2"},
			Status:   task.TaskStatusPending,
		},
		{
			ID:       uuid.New(),
			Schedule: "0 */15 * * * *",
			Command:  task.StringArray{"echo", "task3"},
			Status:   task.TaskStatusPending,
		},
	}

	for _, task := range tasks {
		taskJSON, err := json.Marshal(task)
		require.NoError(t, err)
		err = nc.Publish(cfg.NATS.TaskScheduleSubject, taskJSON)
		require.NoError(t, err)
	}

	// Collect results
	receivedResults := make(map[uuid.UUID]bool)
	timeout := time.After(3 * time.Second)

	for i := 0; i < len(tasks); i++ {
		select {
		case result := <-resultChan:
			receivedResults[result.TaskID] = true
		case <-timeout:
			t.Fatalf("Timeout waiting for all results, received %d/%d", i, len(tasks))
		}
	}

	// Verify all tasks were processed
	assert.Equal(t, len(tasks), len(receivedResults))
	for _, task := range tasks {
		assert.True(t, receivedResults[task.ID], "Task %s was not processed", task.ID)
	}
}
