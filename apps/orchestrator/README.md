# TaskFlow Orchestrator

The Orchestrator is the orchestration engine of TaskFlow. It replaces the
former `scheduler` and `notifier` services: it owns every decision about when
a task is ready to run, dispatches work to Workers, and reacts to their
results to progress, complete, fail or cancel a Workflow.

## Responsibilities

- Analyze the `TaskDependency` graph (DAG) of a Workflow stored in PostgreSQL.
- Decide which `PENDING` tasks have all their dependencies `SUCCEEDED` and
  are therefore ready to run.
- Publish dispatch commands for ready tasks to NATS (`tasks.dispatch`), for
  Workers to pick up via a Queue Group.
- Subscribe to task results published by Workers (`tasks.results`), persist
  them, and update Task/Workflow status accordingly.
- Retry a failed task (up to `orchestrator.max_attempts`) or fail the whole
  Workflow when a task can no longer be retried.
- React to control commands from the API Gateway:
  - `workflow.commands.start`: CREATED -> RUNNING, dispatch initial tasks.
  - `workflow.commands.cancel`: transition to CANCELLED, cancel pending/running tasks.

## NATS subjects

| Subject                     | Direction              | Payload                 |
|------------------------------|------------------------|--------------------------|
| `workflow.commands.start`    | API -> Orchestrator     | `workflow.CommandMessage` |
| `workflow.commands.cancel`   | API -> Orchestrator     | `workflow.CommandMessage` |
| `tasks.dispatch`              | Orchestrator -> Worker  | `task.DispatchMessage`    |
| `tasks.results`               | Worker -> Orchestrator  | `task.ResultMessage`      |

All subscriptions use the `orchestrator` NATS Queue Group so the service can
be scaled horizontally without processing the same message twice.

## Configuration

See [config.yaml](./config.yaml). Every value can be overridden via
environment variables (`PORT`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`,
`DB_NAME`, `NATS_URL`, `LOG_LEVEL`).
