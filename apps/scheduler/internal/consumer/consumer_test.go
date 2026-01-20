package consumer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hhace/taskflow/apps/scheduler/config"
	"github.com/hhace/taskflow/models"
	"github.com/hhace/taskflow/pkg/persistence"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewResultConsumer(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.Config
		repo        persistence.RepositoryInterface
		nc          *nats.Conn
		wantErr     bool
		expectedErr string
	}{
		{
			name:    "success - valid parameters",
			cfg:     &config.Config{},
			repo:    persistence.NewMockRepository(),
			nc:      &nats.Conn{},
			wantErr: false,
		},
		{
			name:        "error - nil NATS connection",
			cfg:         &config.Config{},
			repo:        persistence.NewMockRepository(),
			nc:          nil,
			wantErr:     true,
			expectedErr: "NATS connection is nil",
		},
		{
			name:        "error - nil repository",
			cfg:         &config.Config{},
			repo:        nil,
			nc:          &nats.Conn{},
			wantErr:     true,
			expectedErr: "database connection is nil",
		},
		{
			name:        "error - both nil",
			cfg:         &config.Config{},
			repo:        nil,
			nc:          nil,
			wantErr:     true,
			expectedErr: "NATS connection is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer, err := NewResultConsumer(tt.cfg, tt.repo, tt.nc)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, consumer)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, consumer)
				assert.Equal(t, tt.cfg, consumer.config)
				assert.Equal(t, tt.repo, consumer.repo)
				assert.Equal(t, tt.nc, consumer.natsConn)
			}
		})
	}
}

func TestResultConsumer_HandleResult(t *testing.T) {
	taskID := uuid.New()
	now := time.Now()

	tests := []struct {
		name           string
		result         models.TaskExecutionResult
		mockSetup      func(*persistence.MockRepository)
		expectedStatus models.TaskStatus
	}{
		{
			name: "success - task completed successfully",
			result: models.TaskExecutionResult{
				ID:        uuid.New(),
				TaskID:    taskID,
				Success:   true,
				Output:    "Task completed successfully",
				Error:     "",
				StartTime: now,
				EndTime:   now.Add(5 * time.Second),
				Duration:  "5s",
			},
			mockSetup: func(m *persistence.MockRepository) {
				m.On("UpdateTaskStatus", taskID, models.TaskStatusCompleted).Return(nil).Once()
				m.On("CreateTaskResult", mock.AnythingOfType("*models.TaskExecutionResult")).Return(nil).Once()
			},
			expectedStatus: models.TaskStatusCompleted,
		},
		{
			name: "success - task failed with error",
			result: models.TaskExecutionResult{
				ID:        uuid.New(),
				TaskID:    taskID,
				Success:   false,
				Output:    "",
				Error:     "command not found",
				StartTime: now,
				EndTime:   now.Add(1 * time.Second),
				Duration:  "1s",
			},
			mockSetup: func(m *persistence.MockRepository) {
				m.On("UpdateTaskStatus", taskID, models.TaskStatusFailed).Return(nil).Once()
				m.On("CreateTaskResult", mock.AnythingOfType("*models.TaskExecutionResult")).Return(nil).Once()
			},
			expectedStatus: models.TaskStatusFailed,
		},
		{
			name: "success - task failed with empty error but success=false",
			result: models.TaskExecutionResult{
				ID:        uuid.New(),
				TaskID:    taskID,
				Success:   false,
				Output:    "Some output",
				Error:     "",
				StartTime: now,
				EndTime:   now.Add(2 * time.Second),
				Duration:  "2s",
			},
			mockSetup: func(m *persistence.MockRepository) {
				m.On("UpdateTaskStatus", taskID, models.TaskStatusFailed).Return(nil).Once()
				m.On("CreateTaskResult", mock.AnythingOfType("*models.TaskExecutionResult")).Return(nil).Once()
			},
			expectedStatus: models.TaskStatusFailed,
		},
		{
			name: "error - UpdateTaskStatus fails for completed task",
			result: models.TaskExecutionResult{
				ID:        uuid.New(),
				TaskID:    taskID,
				Success:   true,
				Output:    "Task completed",
				Error:     "",
				StartTime: now,
				EndTime:   now.Add(3 * time.Second),
				Duration:  "3s",
			},
			mockSetup: func(m *persistence.MockRepository) {
				m.On("UpdateTaskStatus", taskID, models.TaskStatusCompleted).Return(assert.AnError).Once()
				// CreateTaskResult should not be called when UpdateTaskStatus fails
			},
			expectedStatus: models.TaskStatusCompleted,
		},
		{
			name: "error - UpdateTaskStatus fails for failed task",
			result: models.TaskExecutionResult{
				ID:        uuid.New(),
				TaskID:    taskID,
				Success:   false,
				Output:    "",
				Error:     "execution error",
				StartTime: now,
				EndTime:   now.Add(1 * time.Second),
				Duration:  "1s",
			},
			mockSetup: func(m *persistence.MockRepository) {
				m.On("UpdateTaskStatus", taskID, models.TaskStatusFailed).Return(assert.AnError).Once()
				// CreateTaskResult should not be called when UpdateTaskStatus fails
			},
			expectedStatus: models.TaskStatusFailed,
		},
		{
			name: "error - CreateTaskResult fails",
			result: models.TaskExecutionResult{
				ID:        uuid.New(),
				TaskID:    taskID,
				Success:   true,
				Output:    "Task completed",
				Error:     "",
				StartTime: now,
				EndTime:   now.Add(4 * time.Second),
				Duration:  "4s",
			},
			mockSetup: func(m *persistence.MockRepository) {
				m.On("UpdateTaskStatus", taskID, models.TaskStatusCompleted).Return(nil).Once()
				m.On("CreateTaskResult", mock.AnythingOfType("*models.TaskExecutionResult")).Return(assert.AnError).Once()
			},
			expectedStatus: models.TaskStatusCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock repository
			mockRepo := persistence.NewMockRepository()
			tt.mockSetup(mockRepo)

			// Create consumer with mock
			cfg := &config.Config{
				NATS: config.NATSConfig{
					TaskResultSubject: "task.result",
				},
			}
			nc := &nats.Conn{}
			consumer := &ResultConsumer{
				config:   cfg,
				natsConn: nc,
				repo:     mockRepo,
			}

			// Marshal result to JSON
			data, err := json.Marshal(tt.result)
			require.NoError(t, err)

			// Create mock NATS message
			msg := &nats.Msg{
				Subject: cfg.NATS.TaskResultSubject,
				Data:    data,
			}

			// Call handleResult
			consumer.handleResult(msg)

			// Verify all expectations were met
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestResultConsumer_HandleResult_InvalidJSON(t *testing.T) {
	// Create mock repository - should not be called
	mockRepo := persistence.NewMockRepository()

	// Create consumer
	cfg := &config.Config{
		NATS: config.NATSConfig{
			TaskResultSubject: "task.result",
		},
	}
	nc := &nats.Conn{}
	consumer := &ResultConsumer{
		config:   cfg,
		natsConn: nc,
		repo:     mockRepo,
	}

	// Create message with invalid JSON
	msg := &nats.Msg{
		Subject: cfg.NATS.TaskResultSubject,
		Data:    []byte("invalid json"),
	}

	// Call handleResult - should log error but not panic
	assert.NotPanics(t, func() {
		consumer.handleResult(msg)
	})

	// Verify no repository methods were called
	mockRepo.AssertExpectations(t)
}

func TestResultConsumer_GetNATSConnection(t *testing.T) {
	nc := &nats.Conn{}
	consumer := &ResultConsumer{
		natsConn: nc,
	}

	result := consumer.GetNATSConnection()
	assert.Equal(t, nc, result)
}

func TestResultConsumer_Stop(t *testing.T) {
	tests := []struct {
		name        string
		setupSub    bool
		setupConn   bool
		expectError bool
	}{
		{
			name:        "success - no subscription or connection",
			setupSub:    false,
			setupConn:   false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer := &ResultConsumer{}

			if tt.setupSub {
				consumer.sub = &nats.Subscription{}
			}

			if tt.setupConn {
				consumer.natsConn = &nats.Conn{}
			}

			err := consumer.Stop()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
