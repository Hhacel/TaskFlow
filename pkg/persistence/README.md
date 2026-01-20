# 🗄️ Working with GORM in TaskFlow

This guide explains how to use GORM (Go Object-Relational Mapping) with PostgreSQL in your TaskFlow project.

## 📦 **What's Included**

### **Persistence Package (`pkg/persistence/`)**
- `persistence.go` - Connection management and configuration
- `repo.go` - Database operations (Repository pattern)
- `mock_repo.go` - Mock repository for testing

### **Examples (`examples/`)**
- `persistence_demo.go` - Complete GORM usage demonstration
- `scheduler_service.go` - Real-world service implementation

## 🚀 **Quick Start**

### **1. Install Dependencies**
```bash
go mod tidy
```

### **2. Set Environment Variables**
```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=taskflow
export DB_USER=taskflow
export DB_PASSWORD=taskflow
```

### **3. Initialize Database in Your Service**
```go
import "github.com/hhace/taskflow/pkg/persistence"

func main() {
    // Load config from environment
    config := persistence.LoadConfig()
    
    // Connect to database
    if err := persistence.Connect(config); err != nil {
        log.Fatal("Database connection failed:", err)
    }
    defer persistence.Close()
    
    // Run auto-migration
    if err := persistence.Migrate(); err != nil {
        log.Fatal("Migration failed:", err)
    }
    
    // Create repository
    repo := persistence.NewRepository(db)
    
    // Use repository...
}
```

## 🎯 **Core Concepts**

### **1. Model Definition**
```go
type Task struct {
    ID        uuid.UUID    `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
    Schedule  string       `gorm:"type:varchar(255);not null"`
    Command   StringArray  `gorm:"type:text[]"`  // PostgreSQL array
    Status    TaskStatus   `gorm:"type:varchar(20);not null;default:'pending'"`
    CreatedAt time.Time    `gorm:"autoCreateTime"`
    UpdatedAt time.Time    `gorm:"autoUpdateTime"`
}
```

### **2. Repository Pattern**
```go
// Create repository
taskRepo := database.NewTaskRepository(database.DB)

// Basic operations
task := &database.Task{
    Schedule: "0 0 * * *",
    Command:  database.StringArray{"echo", "hello"},
    Status:   database.TaskStatusPending,
}

// Create
err := taskRepo.Create(task)

// Read
task, err := taskRepo.GetByID(taskID)
tasks, err := taskRepo.GetAll(10, 0) // limit, offset

// Update
err := taskRepo.UpdateStatus(taskID, database.TaskStatusRunning)

// Delete
err := taskRepo.Delete(taskID)
```

## 📊 **Common Operations**

### **CRUD Operations**
```go
// CREATE
task := &database.Task{
    Schedule: "*/15 * * * *",
    Command:  database.StringArray{"curl", "https://api.example.com"},
}
err := taskRepo.Create(task)

// READ
task, err := taskRepo.GetByID(taskID)
pendingTasks, err := taskRepo.GetByStatus(database.TaskStatusPending, 0, 0)

// UPDATE
err := taskRepo.UpdateStatus(taskID, database.TaskStatusCompleted)

// DELETE
err := taskRepo.Delete(taskID)
```

### **Advanced Queries**
```go
// Get tasks by status with pagination
tasks, err := taskRepo.GetByStatus(database.TaskStatusPending, 10, 20)

// Get recent tasks
oneHourAgo := time.Now().Add(-time.Hour)
recentTasks, err := taskRepo.GetTasksCreatedAfter(oneHourAgo)

// Get statistics
stats, err := taskRepo.GetTaskStats()
// Returns: map[TaskStatus]int64{"pending": 5, "running": 2, "completed": 10}

// Batch operations
taskIDs := []uuid.UUID{id1, id2, id3}
err := taskRepo.BatchUpdateStatus(taskIDs, database.TaskStatusRunning)
```

### **Transactions**
```go
err := taskRepo.Transaction(func(txRepo *database.TaskRepository) error {
    // Create task
    task := &database.Task{...}
    if err := txRepo.Create(task); err != nil {
        return err // Automatically rolls back
    }
    
    // Update another task
    if err := txRepo.UpdateStatus(otherID, database.TaskStatusCompleted); err != nil {
        return err // Automatically rolls back
    }
    
    return nil // Commits transaction
})
```

## 🔧 **Custom Types**

### **PostgreSQL Arrays**
```go
// StringArray handles PostgreSQL text[] columns
type StringArray []string

// Usage
task.Command = database.StringArray{"python", "script.py", "--verbose"}

// In database: {"python","script.py","--verbose"}
```

### **Task Status Enum**
```go
// Predefined statuses
const (
    TaskStatusPending   TaskStatus = "pending"
    TaskStatusRunning   TaskStatus = "running" 
    TaskStatusCompleted TaskStatus = "completed"
    TaskStatusFailed    TaskStatus = "failed"
)

// Validation
if !task.IsValidStatus() {
    return errors.New("invalid status")
}

// State transitions
if !task.CanTransitionTo(newStatus) {
    return errors.New("invalid transition")
}
```

## 🏗️ **Integration Examples**

### **In Scheduler Service**
```go
func (s *SchedulerService) CreateTask(c *gin.Context) {
    var req CreateTaskRequest
    c.ShouldBindJSON(&req)
    
    task := &database.Task{
        Schedule: req.Schedule,
        Command:  database.StringArray(req.Command),
        Status:   database.TaskStatusPending,
    }
    
    if err := s.taskRepo.Create(task); err != nil {
        c.JSON(500, gin.H{"error": "Failed to create task"})
        return
    }
    
    // TODO: Publish to NATS
    c.JSON(201, task)
}
```

### **In Worker Service**
```go
func (w *WorkerService) ProcessTask(taskID uuid.UUID) error {
    // Get task
    task, err := w.taskRepo.GetByID(taskID)
    if err != nil {
        return err
    }
    
    // Update to running
    if err := w.taskRepo.UpdateStatus(taskID, database.TaskStatusRunning); err != nil {
        return err
    }
    
    // Execute command
    err = w.executeCommand(task.Command)
    
    // Update final status
    finalStatus := database.TaskStatusCompleted
    if err != nil {
        finalStatus = database.TaskStatusFailed
    }
    
    return w.taskRepo.UpdateStatus(taskID, finalStatus)
}
```

## 🎯 **Best Practices**

### **1. Use Repository Pattern**
- ✅ Encapsulates database logic
- ✅ Easy to test with mocks
- ✅ Consistent error handling
- ✅ Transaction management

### **2. Handle Errors Properly**
```go
task, err := taskRepo.GetByID(id)
if err != nil {
    if err == gorm.ErrRecordNotFound {
        return nil, fmt.Errorf("task not found")
    }
    return nil, fmt.Errorf("database error: %w", err)
}
```

### **3. Use Transactions for Related Operations**
```go
// Good: Atomic operations
err := taskRepo.Transaction(func(tx *database.TaskRepository) error {
    // Multiple related operations
    return nil
})

// Bad: Separate operations that should be atomic
taskRepo.Create(task1)
taskRepo.Create(task2) // Could fail, leaving inconsistent state
```

### **4. Validate Status Transitions**
```go
// Use built-in validation
if !task.CanTransitionTo(newStatus) {
    return errors.New("invalid status transition")
}
```

### **5. Connection Pooling**
```go
// Configured in database.go
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(100)
sqlDB.SetConnMaxLifetime(time.Hour)
```

## 🐛 **Common Issues & Solutions**

### **UUID Generation**
```go
// Ensure UUID extension is enabled in PostgreSQL
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

// Or use Go-generated UUIDs
task.ID = uuid.New()
```

### **Array Handling**
```go
// PostgreSQL arrays need special handling
import "github.com/lib/pq"

// Use custom StringArray type provided
task.Command = database.StringArray{"cmd", "arg1", "arg2"}
```

### **Time Zones**
```go
// Always use UTC in database
// GORM handles this automatically with time.Time
CreatedAt time.Time `gorm:"autoCreateTime"`
```

## 🔍 **Debugging**

### **Enable SQL Logging**
```go
// In database.go, logger is already configured
// Set log level to see all queries
logger.Config{
    LogLevel: logger.Info, // Shows all SQL queries
}
```

### **Raw SQL**
```go
// When you need custom queries
var tasks []database.Task
database.DB.Raw("SELECT * FROM tasks WHERE created_at > ?", time.Now().Add(-24*time.Hour)).Scan(&tasks)
```

This setup gives you a production-ready database layer with GORM that's specifically designed for your TaskFlow architecture! 🚀