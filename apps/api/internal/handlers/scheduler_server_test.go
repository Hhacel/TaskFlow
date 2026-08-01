package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hhace/taskflow/apps/api/config"
	"github.com/hhace/taskflow/apps/api/internal/api"
	"github.com/hhace/taskflow/internal/task"
	"github.com/hhace/taskflow/pkg/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestNewSchedulerServer(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
		repo   *persistence.Repository
	}{
		{
			name: "creates server successfully with valid config and repo",
			config: &config.Config{
				Server: config.ServerConfig{
					Port: "8081",
				},
			},
			repo: nil, // We're using interface, so nil is acceptable for this test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewSchedulerServer(tt.config, tt.repo)

			require.NoError(t, err)
			require.NotNil(t, server)
			assert.NotNil(t, server.config)
			assert.Equal(t, tt.config, server.config)
		})
	}
}

func TestSchedulerServer_CreateTask(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		mockSetup      func(*persistence.MockRepository)
		expectedStatus int
		validateResp   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "creates task successfully",
			requestBody: api.CreateTaskRequest{
				Schedule: "0 */5 * * * *",
				Command:  []string{"echo", "test"},
			},
			mockSetup: func(m *persistence.MockRepository) {
				m.On("CreateTask", mock.AnythingOfType("*task.Task")).Return(nil).Run(func(args mock.Arguments) {
					task := args.Get(0).(*task.Task)
					task.ID = uuid.New()
					task.CreatedAt = time.Now()
					task.UpdatedAt = time.Now()
				})
			},
			expectedStatus: http.StatusCreated,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.TaskResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.NotNil(t, resp.Id)
				assert.NotNil(t, resp.Schedule)
				assert.Equal(t, "0 */5 * * * *", *resp.Schedule)
				assert.NotNil(t, resp.Command)
				assert.Equal(t, []string{"echo", "test"}, *resp.Command)
			},
		},
		{
			name:           "returns bad request for invalid JSON",
			requestBody:    "invalid json",
			mockSetup:      func(m *persistence.MockRepository) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.ErrorResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.NotNil(t, resp.Error)
			},
		},
		{
			name: "returns internal server error when repo fails",
			requestBody: api.CreateTaskRequest{
				Schedule: "0 */10 * * * *",
				Command:  []string{"ls"},
			},
			mockSetup: func(m *persistence.MockRepository) {
				m.On("CreateTask", mock.AnythingOfType("*task.Task")).Return(errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.ErrorResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.NotNil(t, resp.Error)
				assert.Equal(t, "Failed to create task", *resp.Error)
			},
		},
		{
			name: "creates task with multiple command arguments",
			requestBody: api.CreateTaskRequest{
				Schedule: "0 0 * * * *",
				Command:  []string{"bash", "-c", "echo hello"},
			},
			mockSetup: func(m *persistence.MockRepository) {
				m.On("CreateTask", mock.AnythingOfType("*task.Task")).Return(nil).Run(func(args mock.Arguments) {
					task := args.Get(0).(*task.Task)
					task.ID = uuid.New()
					task.CreatedAt = time.Now()
					task.UpdatedAt = time.Now()
				})
			},
			expectedStatus: http.StatusCreated,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.TaskResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, []string{"bash", "-c", "echo hello"}, *resp.Command)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := persistence.NewMockRepository()
			tt.mockSetup(mockRepo)

			cfg := &config.Config{}
			server := &SchedulerServer{
				config: cfg,
				repo:   mockRepo,
			}

			router := setupTestRouter()
			router.POST("/tasks", server.CreateTask)

			var body []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			tt.validateResp(t, rec)

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestSchedulerServer_GetTask(t *testing.T) {
	testID := uuid.New()
	testTime := time.Now()

	tests := []struct {
		name           string
		taskID         uuid.UUID
		mockSetup      func(*persistence.MockRepository)
		expectedStatus int
		validateResp   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:   "retrieves task successfully",
			taskID: testID,
			mockSetup: func(m *persistence.MockRepository) {
				task := &task.Task{
					ID:        testID,
					Schedule:  "0 */5 * * * *",
					Command:   task.StringArray{"echo", "test"},
					Status:    task.TaskStatusPending,
					CreatedAt: testTime,
					UpdatedAt: testTime,
				}
				m.On("GetTaskByID", testID).Return(task, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.TaskResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, testID, *resp.Id)
				assert.Equal(t, "0 */5 * * * *", *resp.Schedule)
				assert.Equal(t, []string{"echo", "test"}, *resp.Command)
				assert.Equal(t, api.TaskResponseStatusPending, *resp.Status)
			},
		},
		{
			name:   "returns not found for non-existent task",
			taskID: uuid.New(),
			mockSetup: func(m *persistence.MockRepository) {
				m.On("GetTaskByID", mock.AnythingOfType("uuid.UUID")).Return(nil, errors.New("not found"))
			},
			expectedStatus: http.StatusNotFound,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.ErrorResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.NotNil(t, resp.Error)
				assert.Equal(t, "Task not found", *resp.Error)
			},
		},
		{
			name:   "retrieves completed task",
			taskID: testID,
			mockSetup: func(m *persistence.MockRepository) {
				task := &task.Task{
					ID:        testID,
					Schedule:  "0 0 * * * *",
					Command:   task.StringArray{"ls", "-la"},
					Status:    task.TaskStatusCompleted,
					CreatedAt: testTime,
					UpdatedAt: testTime,
				}
				m.On("GetTaskByID", testID).Return(task, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.TaskResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, api.TaskResponseStatusCompleted, *resp.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := persistence.NewMockRepository()
			tt.mockSetup(mockRepo)

			cfg := &config.Config{}
			server := &SchedulerServer{
				config: cfg,
				repo:   mockRepo,
			}

			router := setupTestRouter()
			router.GET("/tasks/:id", func(c *gin.Context) {
				id, _ := uuid.Parse(c.Param("id"))
				server.GetTask(c, id)
			})

			req := httptest.NewRequest(http.MethodGet, "/tasks/"+tt.taskID.String(), nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			tt.validateResp(t, rec)

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestSchedulerServer_GetTasks(t *testing.T) {
	testTime := time.Now()

	tests := []struct {
		name           string
		queryParams    string
		mockSetup      func(*persistence.MockRepository)
		expectedStatus int
		validateResp   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "retrieves all tasks successfully",
			queryParams: "",
			mockSetup: func(m *persistence.MockRepository) {
				tasks := []task.Task{
					{
						ID:        uuid.New(),
						Schedule:  "0 */5 * * * *",
						Command:   task.StringArray{"echo", "task1"},
						Status:    task.TaskStatusPending,
						CreatedAt: testTime,
						UpdatedAt: testTime,
					},
					{
						ID:        uuid.New(),
						Schedule:  "0 */10 * * * *",
						Command:   task.StringArray{"echo", "task2"},
						Status:    task.TaskStatusCompleted,
						CreatedAt: testTime,
						UpdatedAt: testTime,
					},
				}
				m.On("GetAllTasks", 50, 0).Return(tasks, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.TaskListResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.NotNil(t, resp.Tasks)
				assert.Equal(t, 2, *resp.Count)
				assert.Len(t, *resp.Tasks, 2)
			},
		},
		{
			name:        "retrieves tasks filtered by pending status",
			queryParams: "?status=pending",
			mockSetup: func(m *persistence.MockRepository) {
				tasks := []task.Task{
					{
						ID:        uuid.New(),
						Schedule:  "0 */5 * * * *",
						Command:   task.StringArray{"echo", "pending"},
						Status:    task.TaskStatusPending,
						CreatedAt: testTime,
						UpdatedAt: testTime,
					},
				}
				m.On("GetTasksByStatus", task.TaskStatusPending, 50, 0).Return(tasks, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.TaskListResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, 1, *resp.Count)
				assert.Equal(t, api.TaskResponseStatusPending, *(*resp.Tasks)[0].Status)
			},
		},
		{
			name:        "retrieves tasks filtered by completed status",
			queryParams: "?status=completed",
			mockSetup: func(m *persistence.MockRepository) {
				tasks := []task.Task{
					{
						ID:        uuid.New(),
						Schedule:  "0 0 * * * *",
						Command:   task.StringArray{"ls"},
						Status:    task.TaskStatusCompleted,
						CreatedAt: testTime,
						UpdatedAt: testTime,
					},
				}
				m.On("GetTasksByStatus", task.TaskStatusCompleted, 50, 0).Return(tasks, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.TaskListResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, 1, *resp.Count)
				assert.Equal(t, api.TaskResponseStatusCompleted, *(*resp.Tasks)[0].Status)
			},
		},
		{
			name:        "retrieves tasks filtered by failed status",
			queryParams: "?status=failed",
			mockSetup: func(m *persistence.MockRepository) {
				tasks := []task.Task{
					{
						ID:        uuid.New(),
						Schedule:  "0 0 * * * *",
						Command:   task.StringArray{"false"},
						Status:    task.TaskStatusFailed,
						CreatedAt: testTime,
						UpdatedAt: testTime,
					},
				}
				m.On("GetTasksByStatus", task.TaskStatusFailed, 50, 0).Return(tasks, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.TaskListResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, 1, *resp.Count)
				assert.Equal(t, api.TaskResponseStatusFailed, *(*resp.Tasks)[0].Status)
			},
		},
		{
			name:        "returns empty list when no tasks exist",
			queryParams: "",
			mockSetup: func(m *persistence.MockRepository) {
				tasks := []task.Task{}
				m.On("GetAllTasks", 50, 0).Return(tasks, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.TaskListResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.NotNil(t, resp.Count)
				assert.Equal(t, 0, *resp.Count)
				// apiTasks is an empty slice, so Tasks should be a pointer to empty slice
				if resp.Tasks != nil {
					assert.Len(t, *resp.Tasks, 0)
				}
			},
		},
		{
			name:        "returns internal server error when repo fails",
			queryParams: "",
			mockSetup: func(m *persistence.MockRepository) {
				m.On("GetAllTasks", 50, 0).Return([]task.Task{}, errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.ErrorResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.NotNil(t, resp.Error)
				assert.Equal(t, "Failed to retrieve tasks", *resp.Error)
			},
		},
		{
			name:        "returns internal server error when filtered repo fails",
			queryParams: "?status=pending",
			mockSetup: func(m *persistence.MockRepository) {
				m.On("GetTasksByStatus", task.TaskStatusPending, 50, 0).Return([]task.Task{}, errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			validateResp: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp api.ErrorResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.NotNil(t, resp.Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := persistence.NewMockRepository()
			tt.mockSetup(mockRepo)

			cfg := &config.Config{}
			server := &SchedulerServer{
				config: cfg,
				repo:   mockRepo,
			}

			router := setupTestRouter()
			router.GET("/tasks", func(c *gin.Context) {
				var params api.GetTasksParams
				if c.Query("status") != "" {
					status := api.GetTasksParamsStatus(c.Query("status"))
					params.Status = &status
				}
				server.GetTasks(c, params)
			})

			req := httptest.NewRequest(http.MethodGet, "/tasks"+tt.queryParams, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			tt.validateResp(t, rec)

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestSchedulerServer_StatusConversions(t *testing.T) {
	server := &SchedulerServer{}

	t.Run("apiStatusToModelStatus conversions", func(t *testing.T) {
		tests := []struct {
			apiStatus   api.GetTasksParamsStatus
			modelStatus task.TaskStatus
		}{
			{api.GetTasksParamsStatusPending, task.TaskStatusPending},
			{api.GetTasksParamsStatusCompleted, task.TaskStatusCompleted},
			{api.GetTasksParamsStatusFailed, task.TaskStatusFailed},
		}

		for _, tt := range tests {
			result := server.apiStatusToModelStatus(tt.apiStatus)
			assert.Equal(t, tt.modelStatus, result)
		}
	})

	t.Run("modelStatusToAPIStatus conversions", func(t *testing.T) {
		tests := []struct {
			modelStatus task.TaskStatus
			apiStatus   api.TaskResponseStatus
		}{
			{task.TaskStatusCreated, api.TaskResponseStatusPending},
			{task.TaskStatusPending, api.TaskResponseStatusPending},
			{task.TaskStatusCompleted, api.TaskResponseStatusCompleted},
			{task.TaskStatusFailed, api.TaskResponseStatusFailed},
		}

		for _, tt := range tests {
			result := server.modelStatusToAPIStatus(tt.modelStatus)
			assert.Equal(t, tt.apiStatus, result)
		}
	})
}

func TestSchedulerServer_TaskToResponse(t *testing.T) {
	server := &SchedulerServer{}
	testID := uuid.New()
	testTime := time.Now()

	tests := []struct {
		name     string
		task     *task.Task
		validate func(*testing.T, api.TaskResponse)
	}{
		{
			name: "converts pending task correctly",
			task: &task.Task{
				ID:        testID,
				Schedule:  "0 */5 * * * *",
				Command:   task.StringArray{"echo", "test"},
				Status:    task.TaskStatusPending,
				CreatedAt: testTime,
				UpdatedAt: testTime,
			},
			validate: func(t *testing.T, resp api.TaskResponse) {
				assert.Equal(t, testID, *resp.Id)
				assert.Equal(t, "0 */5 * * * *", *resp.Schedule)
				assert.Equal(t, []string{"echo", "test"}, *resp.Command)
				assert.Equal(t, api.TaskResponseStatusPending, *resp.Status)
				assert.Equal(t, testTime, *resp.CreatedAt)
				assert.Equal(t, testTime, *resp.UpdatedAt)
			},
		},
		{
			name: "converts completed task correctly",
			task: &task.Task{
				ID:        testID,
				Schedule:  "0 0 * * * *",
				Command:   task.StringArray{"ls", "-la"},
				Status:    task.TaskStatusCompleted,
				CreatedAt: testTime,
				UpdatedAt: testTime,
			},
			validate: func(t *testing.T, resp api.TaskResponse) {
				assert.Equal(t, api.TaskResponseStatusCompleted, *resp.Status)
				assert.Equal(t, []string{"ls", "-la"}, *resp.Command)
			},
		},
		{
			name: "converts failed task correctly",
			task: &task.Task{
				ID:        testID,
				Schedule:  "0 0 * * * *",
				Command:   task.StringArray{"false"},
				Status:    task.TaskStatusFailed,
				CreatedAt: testTime,
				UpdatedAt: testTime,
			},
			validate: func(t *testing.T, resp api.TaskResponse) {
				assert.Equal(t, api.TaskResponseStatusFailed, *resp.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := server.taskToResponse(tt.task)
			tt.validate(t, resp)
		})
	}
}
