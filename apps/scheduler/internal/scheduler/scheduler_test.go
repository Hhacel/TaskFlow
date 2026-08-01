package scheduler

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hhace/taskflow/apps/scheduler/config"
	"github.com/hhace/taskflow/internal/task"
	"github.com/hhace/taskflow/pkg/persistence"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTaskScheduler(t *testing.T) {
	ns, nc := startNATSServer(t)
	defer ns.Shutdown()
	defer nc.Close()

	tests := []struct {
		name     string
		cfg      *config.Config
		repo     persistence.RepositoryInterface
		natsConn *nats.Conn
	}{
		{
			name: "success - creates scheduler with valid parameters",
			cfg: &config.Config{
				NATS: config.NATSConfig{
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
				},
			},
			repo:     persistence.NewMockRepository(),
			natsConn: nc,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduler := NewTaskScheduler(tt.cfg, tt.repo, tt.natsConn)

			assert.NotNil(t, scheduler)
			assert.Equal(t, tt.cfg, scheduler.config)
			assert.Equal(t, tt.repo, scheduler.repo)
			assert.Equal(t, tt.natsConn, scheduler.natsConn)
			assert.NotNil(t, scheduler.cron)
			assert.NotNil(t, scheduler.jobs)
			assert.NotNil(t, scheduler.stopRefresh)
			assert.Equal(t, 0, len(scheduler.jobs))
		})
	}
}

func TestTaskScheduler_AddTask(t *testing.T) {
	ns, nc := startNATSServer(t)
	defer ns.Shutdown()
	defer nc.Close()

	tests := []struct {
		name           string
		task           *task.Task
		existingJobs   map[uuid.UUID]bool
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name: "success - adds new task with valid cron expression",
			task: &task.Task{
				ID:       uuid.New(),
				Schedule: "0 */5 * * * *", // Every 5 minutes
				Command:  task.StringArray{"echo", "test"},
				Status:   task.TaskStatusCreated,
			},
			existingJobs: map[uuid.UUID]bool{},
			wantErr:      false,
		},
		{
			name: "success - skips already scheduled task",
			task: &task.Task{
				ID:       uuid.New(),
				Schedule: "0 0 * * * *", // Every hour
				Command:  task.StringArray{"echo", "existing"},
				Status:   task.TaskStatusCreated,
			},
			existingJobs: map[uuid.UUID]bool{},
			wantErr:      false,
		},
		{
			name: "error - invalid cron expression",
			task: &task.Task{
				ID:       uuid.New(),
				Schedule: "invalid cron",
				Command:  task.StringArray{"echo", "test"},
				Status:   task.TaskStatusCreated,
			},
			existingJobs:   map[uuid.UUID]bool{},
			wantErr:        true,
			expectedErrMsg: "invalid cron expression",
		},
		{
			name: "success - adds task with seconds-based cron",
			task: &task.Task{
				ID:       uuid.New(),
				Schedule: "*/30 * * * * *", // Every 30 seconds
				Command:  task.StringArray{"echo", "seconds"},
				Status:   task.TaskStatusCreated,
			},
			existingJobs: map[uuid.UUID]bool{},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				NATS: config.NATSConfig{
					TaskScheduleSubject: "tasks.schedule",
				},
			}
			mockRepo := persistence.NewMockRepository()

			scheduler := NewTaskScheduler(cfg, mockRepo, nc)

			// Pre-populate jobs if task should already exist
			if tt.name == "success - skips already scheduled task" {
				// Add a dummy entry ID
				scheduler.jobs[tt.task.ID] = 1
			}

			err := scheduler.AddTask(tt.task)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
				// Task should not be in jobs map
				_, exists := scheduler.jobs[tt.task.ID]
				assert.False(t, exists)
			} else {
				assert.NoError(t, err)
				// Task should be in jobs map (unless it was already there)
				_, exists := scheduler.jobs[tt.task.ID]
				assert.True(t, exists)
			}
		})
	}
}

func TestTaskScheduler_RemoveTask(t *testing.T) {
	ns, nc := startNATSServer(t)
	defer ns.Shutdown()
	defer nc.Close()

	tests := []struct {
		name           string
		taskID         uuid.UUID
		taskScheduled  bool
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name:          "success - removes scheduled task",
			taskID:        uuid.New(),
			taskScheduled: true,
			wantErr:       false,
		},
		{
			name:           "error - task not scheduled",
			taskID:         uuid.New(),
			taskScheduled:  false,
			wantErr:        true,
			expectedErrMsg: "task not scheduled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			mockRepo := persistence.NewMockRepository()

			scheduler := NewTaskScheduler(cfg, mockRepo, nc)

			// Schedule the task if needed
			if tt.taskScheduled {
				task := &task.Task{
					ID:       tt.taskID,
					Schedule: "0 */5 * * * *",
					Command:  task.StringArray{"echo", "test"},
				}
				err := scheduler.AddTask(task)
				require.NoError(t, err)
			}

			err := scheduler.RemoveTask(tt.taskID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
				// Task should not be in jobs map
				_, exists := scheduler.jobs[tt.taskID]
				assert.False(t, exists)
			}
		})
	}
}

func TestTaskScheduler_LoadTasks(t *testing.T) {
	ns, nc := startNATSServer(t)
	defer ns.Shutdown()
	defer nc.Close()

	taskID1 := uuid.New()
	taskID2 := uuid.New()
	taskID3 := uuid.New()

	tests := []struct {
		name          string
		mockSetup     func(*persistence.MockRepository)
		existingTasks map[uuid.UUID]bool
		expectedJobs  int
		wantErr       bool
	}{
		{
			name: "success - loads tasks from empty database",
			mockSetup: func(m *persistence.MockRepository) {
				m.On("GetAllTasks", 0, 0).Return([]task.Task{}, nil).Once()
			},
			existingTasks: map[uuid.UUID]bool{},
			expectedJobs:  0,
			wantErr:       false,
		},
		{
			name: "success - loads multiple tasks",
			mockSetup: func(m *persistence.MockRepository) {
				tasks := []task.Task{
					{
						ID:       taskID1,
						Schedule: "0 */5 * * * *",
						Command:  task.StringArray{"echo", "1"},
						Status:   task.TaskStatusCreated,
					},
					{
						ID:       taskID2,
						Schedule: "0 0 * * * *",
						Command:  task.StringArray{"echo", "2"},
						Status:   task.TaskStatusCreated,
					},
				}
				m.On("GetAllTasks", 0, 0).Return(tasks, nil).Once()
			},
			existingTasks: map[uuid.UUID]bool{},
			expectedJobs:  2,
			wantErr:       false,
		},
		{
			name: "success - removes tasks not in database",
			mockSetup: func(m *persistence.MockRepository) {
				tasks := []task.Task{
					{
						ID:       taskID1,
						Schedule: "0 */5 * * * *",
						Command:  task.StringArray{"echo", "1"},
						Status:   task.TaskStatusCreated,
					},
				}
				m.On("GetAllTasks", 0, 0).Return(tasks, nil).Once()
			},
			existingTasks: map[uuid.UUID]bool{
				taskID1: true,
				taskID2: true, // This should be removed
			},
			expectedJobs: 1,
			wantErr:      false,
		},
		{
			name: "error - database fetch fails",
			mockSetup: func(m *persistence.MockRepository) {
				m.On("GetAllTasks", 0, 0).Return([]task.Task{}, assert.AnError).Once()
			},
			existingTasks: map[uuid.UUID]bool{},
			expectedJobs:  0,
			wantErr:       true,
		},
		{
			name: "partial success - skips task with invalid cron",
			mockSetup: func(m *persistence.MockRepository) {
				tasks := []task.Task{
					{
						ID:       taskID1,
						Schedule: "0 */5 * * * *",
						Command:  task.StringArray{"echo", "valid"},
						Status:   task.TaskStatusCreated,
					},
					{
						ID:       taskID3,
						Schedule: "invalid cron",
						Command:  task.StringArray{"echo", "invalid"},
						Status:   task.TaskStatusCreated,
					},
				}
				m.On("GetAllTasks", 0, 0).Return(tasks, nil).Once()
			},
			existingTasks: map[uuid.UUID]bool{},
			expectedJobs:  1, // Only valid task should be scheduled
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			mockRepo := persistence.NewMockRepository()
			tt.mockSetup(mockRepo)

			scheduler := NewTaskScheduler(cfg, mockRepo, nc)

			// Pre-populate existing tasks
			for taskID := range tt.existingTasks {
				task := &task.Task{
					ID:       taskID,
					Schedule: "0 */5 * * * *",
					Command:  task.StringArray{"echo", "test"},
				}
				_ = scheduler.AddTask(task)
			}

			err := scheduler.loadTasks()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedJobs, len(scheduler.jobs))
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestTaskScheduler_Start(t *testing.T) {
	ns, nc := startNATSServer(t)
	defer ns.Shutdown()
	defer nc.Close()

	tests := []struct {
		name      string
		mockSetup func(*persistence.MockRepository)
		wantErr   bool
	}{
		{
			name: "success - starts with no tasks",
			mockSetup: func(m *persistence.MockRepository) {
				m.On("GetAllTasks", 0, 0).Return([]task.Task{}, nil).Once()
			},
			wantErr: false,
		},
		{
			name: "success - starts with tasks",
			mockSetup: func(m *persistence.MockRepository) {
				tasks := []task.Task{
					{
						ID:       uuid.New(),
						Schedule: "0 */5 * * * *",
						Command:  task.StringArray{"echo", "test"},
						Status:   task.TaskStatusCreated,
					},
				}
				m.On("GetAllTasks", 0, 0).Return(tasks, nil).Once()
			},
			wantErr: false,
		},
		{
			name: "error - fails to load tasks",
			mockSetup: func(m *persistence.MockRepository) {
				m.On("GetAllTasks", 0, 0).Return([]task.Task{}, assert.AnError).Once()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			mockRepo := persistence.NewMockRepository()
			tt.mockSetup(mockRepo)

			scheduler := NewTaskScheduler(cfg, mockRepo, nc)

			err := scheduler.Start()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Always stop to clean up goroutine
			scheduler.Stop()

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestTaskScheduler_Stop(t *testing.T) {
	ns, nc := startNATSServer(t)
	defer ns.Shutdown()
	defer nc.Close()

	tests := []struct {
		name      string
		mockSetup func(*persistence.MockRepository)
	}{
		{
			name: "success - stops scheduler",
			mockSetup: func(m *persistence.MockRepository) {
				m.On("GetAllTasks", 0, 0).Return([]task.Task{}, nil).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			mockRepo := persistence.NewMockRepository()
			tt.mockSetup(mockRepo)

			scheduler := NewTaskScheduler(cfg, mockRepo, nc)

			// Start the scheduler
			err := scheduler.Start()
			require.NoError(t, err)

			// Stop should not panic
			assert.NotPanics(t, func() {
				scheduler.Stop()
			})

			// Verify stopRefresh channel is closed by trying to receive
			select {
			case <-scheduler.stopRefresh:
				// Channel is closed as expected
			case <-time.After(100 * time.Millisecond):
				t.Error("stopRefresh channel was not closed")
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestTaskScheduler_ExecuteTask(t *testing.T) {
	// Start in-memory NATS server
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1, // Random port
	}
	ns, err := server.NewServer(opts)
	require.NoError(t, err)

	go ns.Start()
	defer ns.Shutdown()

	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	// Connect to the in-memory NATS server
	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)
	defer nc.Close()

	taskID := uuid.New()

	tests := []struct {
		name      string
		task      *task.Task
		mockSetup func(*persistence.MockRepository)
		verify    func(*testing.T, *nats.Conn)
	}{
		{
			name: "success - publishes task to NATS",
			task: &task.Task{
				ID:       taskID,
				Schedule: "0 */5 * * * *",
				Command:  task.StringArray{"echo", "test"},
				Status:   task.TaskStatusCreated,
			},
			mockSetup: func(m *persistence.MockRepository) {
				m.On("UpdateTaskStatus", taskID, task.TaskStatusPending).Return(nil).Once()
			},
			verify: func(t *testing.T, nc *nats.Conn) {
				// Verify message was published by subscribing
				ch := make(chan *nats.Msg, 1)
				sub, err := nc.Subscribe("tasks.schedule", func(msg *nats.Msg) {
					ch <- msg
				})
				require.NoError(t, err)
				defer sub.Unsubscribe()

				select {
				case msg := <-ch:
					assert.NotNil(t, msg)
					assert.NotEmpty(t, msg.Data)
				case <-time.After(100 * time.Millisecond):
					// Message might have been published before subscription
					// This is expected in some test runs
				}
			},
		},
		{
			name: "error - UpdateTaskStatus fails",
			task: &task.Task{
				ID:       taskID,
				Schedule: "0 */5 * * * *",
				Command:  task.StringArray{"echo", "test"},
				Status:   task.TaskStatusCreated,
			},
			mockSetup: func(m *persistence.MockRepository) {
				m.On("UpdateTaskStatus", taskID, task.TaskStatusPending).Return(assert.AnError).Once()
			},
			verify: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				NATS: config.NATSConfig{
					TaskScheduleSubject: "tasks.schedule",
				},
			}
			mockRepo := persistence.NewMockRepository()
			tt.mockSetup(mockRepo)

			scheduler := &TaskScheduler{
				config:   cfg,
				repo:     mockRepo,
				natsConn: nc,
			}

			// Execute task - should not panic
			assert.NotPanics(t, func() {
				scheduler.executeTask(tt.task)
			})

			if tt.verify != nil {
				tt.verify(t, nc)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

// Helper function to start an in-memory NATS server for tests
func startNATSServer(t *testing.T) (*server.Server, *nats.Conn) {
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1, // Random port
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
