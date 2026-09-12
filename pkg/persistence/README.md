# 🗄️ Working with GORM in TaskFlow

This guide explains how to use the `pkg/persistence` package: GORM/PostgreSQL persistence for the Workflow/Task DAG model shared by the API Gateway and the Orchestrator.

## 📦 **What's Included**

- `persistence.go` - Connection management (`Connect`, `Close`, `Health`) and schema migration (`Migrate`)
- `repo.go` - `RepositoryInterface` and its GORM-backed implementation, `Repository`
- `mock_repo.go` - `MockRepository` (testify `mock.Mock`-based) for unit tests

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
    cfg := persistence.Config{
        Host:     "localhost",
        Port:     "5432",
        User:     "taskflow",
        Password: "taskflow",
        Database: "taskflow",
        SSLMode:  "disable",
    }

    db, err := persistence.Connect(cfg, 10)
    if err != nil {
        log.Fatal("Database connection failed:", err)
    }
    defer persistence.Close(db)

    if err := persistence.Migrate(db); err != nil {
        log.Fatal("Migration failed:", err)
    }

    repo := persistence.NewRepository(db)
    // Use repo...
}
```

## 🎯 **Domain Model**

`Migrate` auto-migrates four tables:

| Model | Table | Notes |
|-------|-------|-------|
| `workflow.Workflow` | `workflows` | `ID uint` PK, `Status workflow.Status` |
| `task.Task` | `tasks` | `WorkflowID uint` FK, `Status task.TaskStatus`, nullable `Timeout *int` (seconds) |
| `task.TaskDependency` | `task_dependencies` | Composite PK `(TaskID, DependsOnTaskID)` |
| `task.TaskResult` | `task_results` | One row per execution attempt |

## 🔧 **Repository Pattern**

`RepositoryInterface` (see [repo.go](./repo.go)) exposes every operation needed by the API Gateway and Orchestrator:

```go
type RepositoryInterface interface {
    // Workflow
    CreateWorkflow(wf *workflow.Workflow) error
    GetWorkflowByID(id uint) (*workflow.Workflow, error)
    UpdateWorkflowStatus(id uint, status workflow.Status) error

    // Task
    CreateTask(t *task.Task) error
    GetTaskByID(id uint) (*task.Task, error)
    GetTasksByWorkflowID(workflowID uint) ([]task.Task, error)
    UpdateTaskStatus(id uint, status task.TaskStatus) error
    CancelTasksByWorkflowID(workflowID uint) error

    // TaskDependency
    CreateTaskDependency(dep *task.TaskDependency) error
    GetDependenciesForTask(taskID uint) ([]task.TaskDependency, error)
    GetDependentsOfTask(taskID uint) ([]task.TaskDependency, error)

    // TaskResult
    CreateTaskResult(result *task.TaskResult) error
    GetTaskResultsByTaskID(taskID uint) ([]task.TaskResult, error)
    GetLatestTaskResultByTaskID(taskID uint) (*task.TaskResult, error)

    // Transaction executes fn with a repository bound to a single DB transaction.
    Transaction(fn func(RepositoryInterface) error) error
}
```

### Example: creating a workflow with its task graph

```go
err := repo.Transaction(func(tx persistence.RepositoryInterface) error {
    wf := &workflow.Workflow{Name: "example", Status: workflow.StatusCreated}
    if err := tx.CreateWorkflow(wf); err != nil {
        return err // rolled back automatically
    }

    a := &task.Task{WorkflowID: wf.ID, Name: "task-a", Command: "echo a", Status: task.TaskStatusPending}
    if err := tx.CreateTask(a); err != nil {
        return err
    }

    b := &task.Task{WorkflowID: wf.ID, Name: "task-b", Command: "echo b", Status: task.TaskStatusPending}
    if err := tx.CreateTask(b); err != nil {
        return err
    }

    return tx.CreateTaskDependency(&task.TaskDependency{TaskID: b.ID, DependsOnTaskID: a.ID})
})
```

## 🏗️ **Usage in Services**

### In the API Gateway
Create/Status/Results are served directly against PostgreSQL via the `Repository`; Start/Cancel only flip the workflow's status once the Orchestrator has been notified over NATS.

### In the Orchestrator
`GetTasksByWorkflowID` + `GetDependenciesForTask` are used to resolve which `PENDING` tasks are ready to dispatch; `UpdateTaskStatus`/`UpdateWorkflowStatus`/`CreateTaskResult` persist the outcome of each execution attempt.

## 🧪 **Testing**

Use `persistence.NewMockRepository()` (backed by `testify/mock`) to unit test handlers and the Orchestrator without a real database — see `apps/api/internal/handlers/workflow_handler_test.go` and `apps/orchestrator/internal/orchestrator/orchestrator_test.go` for examples.

Integration tests against a real PostgreSQL instance live in `persistence_test.go` and are run via `make test-persistence` (they are excluded from `make test`/`make test-unit`).
