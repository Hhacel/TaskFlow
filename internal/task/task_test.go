package task

import (
	"database/sql/driver"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestTask_TableName(t *testing.T) {
	task := Task{}
	assert.Equal(t, "tasks", task.TableName())
}

func TestTask_BeforeCreate(t *testing.T) {
	tests := []struct {
		name    string
		task    Task
		wantNil bool
	}{
		{
			name:    "generates UUID when nil",
			task:    Task{ID: uuid.Nil},
			wantNil: false,
		},
		{
			name:    "preserves existing UUID",
			task:    Task{ID: uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalID := tt.task.ID
			err := tt.task.BeforeCreate()
			assert.NoError(t, err)

			if originalID == uuid.Nil {
				assert.NotEqual(t, uuid.Nil, tt.task.ID, "should generate new UUID")
			} else {
				assert.Equal(t, originalID, tt.task.ID, "should preserve existing UUID")
			}
		})
	}
}

func TestTask_IsValidStatus(t *testing.T) {
	tests := []struct {
		name   string
		status TaskStatus
		want   bool
	}{
		{"created status is valid", TaskStatusCreated, true},
		{"pending status is valid", TaskStatusPending, true},
		{"completed status is valid", TaskStatusCompleted, true},
		{"failed status is valid", TaskStatusFailed, true},
		{"invalid status", TaskStatus("invalid"), false},
		{"empty status", TaskStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := Task{Status: tt.status}
			assert.Equal(t, tt.want, task.IsValidStatus())
		})
	}
}

func TestTask_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name       string
		fromStatus TaskStatus
		toStatus   TaskStatus
		canTransit bool
	}{
		// From Created
		{"created -> pending", TaskStatusCreated, TaskStatusPending, true},
		{"created -> completed", TaskStatusCreated, TaskStatusCompleted, false},
		{"created -> failed", TaskStatusCreated, TaskStatusFailed, false},
		{"created -> created", TaskStatusCreated, TaskStatusCreated, false},

		// From Pending
		{"pending -> completed", TaskStatusPending, TaskStatusCompleted, true},
		{"pending -> failed", TaskStatusPending, TaskStatusFailed, true},
		{"pending -> created", TaskStatusPending, TaskStatusCreated, false},
		{"pending -> pending", TaskStatusPending, TaskStatusPending, false},

		// From Completed (terminal state)
		{"completed -> pending", TaskStatusCompleted, TaskStatusPending, false},
		{"completed -> failed", TaskStatusCompleted, TaskStatusFailed, false},
		{"completed -> created", TaskStatusCompleted, TaskStatusCreated, false},

		// From Failed (terminal state)
		{"failed -> pending", TaskStatusFailed, TaskStatusPending, false},
		{"failed -> completed", TaskStatusFailed, TaskStatusCompleted, false},
		{"failed -> created", TaskStatusFailed, TaskStatusCreated, false},

		// Invalid from status
		{"invalid -> pending", TaskStatus("invalid"), TaskStatusPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := Task{Status: tt.fromStatus}
			assert.Equal(t, tt.canTransit, task.CanTransitionTo(tt.toStatus))
		})
	}
}

func TestTask_UpdateStatus(t *testing.T) {
	tests := []struct {
		name        string
		fromStatus  TaskStatus
		newStatus   TaskStatus
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid transition from created to pending",
			fromStatus:  TaskStatusCreated,
			newStatus:   TaskStatusPending,
			expectError: false,
		},
		{
			name:        "invalid transition from created to completed",
			fromStatus:  TaskStatusCreated,
			newStatus:   TaskStatusCompleted,
			expectError: true,
			errorMsg:    "cannot transition from created to completed",
		},
		{
			name:        "valid transition from pending to completed",
			fromStatus:  TaskStatusPending,
			newStatus:   TaskStatusCompleted,
			expectError: false,
		},
		{
			name:        "valid transition from pending to failed",
			fromStatus:  TaskStatusPending,
			newStatus:   TaskStatusFailed,
			expectError: false,
		},
		{
			name:        "invalid transition from completed (terminal state)",
			fromStatus:  TaskStatusCompleted,
			newStatus:   TaskStatusPending,
			expectError: true,
			errorMsg:    "cannot transition from completed to pending",
		},
		{
			name:        "invalid transition from failed (terminal state)",
			fromStatus:  TaskStatusFailed,
			newStatus:   TaskStatusCompleted,
			expectError: true,
			errorMsg:    "cannot transition from failed to completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{Status: tt.fromStatus}
			err := task.UpdateStatus(tt.newStatus)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Equal(t, tt.fromStatus, task.Status, "status should not change on error")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.newStatus, task.Status, "status should be updated")
			}
		})
	}
}

func TestStringArray_Scan(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    StringArray
		wantErr bool
	}{
		{
			name:  "scan nil value",
			value: nil,
			want:  nil,
		},
		{
			name:  "scan byte slice with PostgreSQL format",
			value: []byte("{echo,test,command}"),
			want:  StringArray{"echo", "test", "command"},
		},
		{
			name:  "scan string with PostgreSQL format",
			value: "{echo,test}",
			want:  StringArray{"echo", "test"},
		},
		{
			name:  "scan empty array",
			value: "{}",
			want:  StringArray{},
		},
		{
			name:  "scan empty string",
			value: "",
			want:  StringArray{},
		},
		{
			name:  "scan pq.StringArray",
			value: pq.StringArray{"ls", "-la"},
			want:  StringArray{"ls", "-la"},
		},
		{
			name:    "scan unsupported type",
			value:   123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s StringArray
			err := s.Scan(tt.value)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, s)
			}
		})
	}
}

func TestStringArray_scanString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  StringArray
	}{
		{
			name:  "scan normal PostgreSQL array",
			input: "{echo,test,command}",
			want:  StringArray{"echo", "test", "command"},
		},
		{
			name:  "scan empty braces",
			input: "{}",
			want:  StringArray{},
		},
		{
			name:  "scan empty string",
			input: "",
			want:  StringArray{},
		},
		{
			name:  "scan single element",
			input: "{echo}",
			want:  StringArray{"echo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s StringArray
			err := s.scanString(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, s)
		})
	}
}

func TestStringArray_Value(t *testing.T) {
	tests := []struct {
		name    string
		array   StringArray
		wantNil bool
	}{
		{
			name:    "nil array returns nil",
			array:   nil,
			wantNil: true,
		},
		{
			name:  "empty array",
			array: StringArray{},
		},
		{
			name:  "array with values",
			array: StringArray{"echo", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.array.Value()
			assert.NoError(t, err)

			if tt.wantNil {
				assert.Nil(t, value)
			} else {
				assert.NotNil(t, value)
				// Verify it's a valid driver.Value type
				var _ driver.Value = value
			}
		})
	}
}
