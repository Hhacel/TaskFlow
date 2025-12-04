# TaskFlow Worker

The Worker service is responsible for executing scheduled tasks in the TaskFlow system. It consumes task messages from NATS queues, executes the commands, and publishes results back to the aggregator.

## Architecture

```
NATS Queue --[tasks.schedule]--> Worker --[execute]--> External Commands
                                   |
                                   v
                         NATS Queue [tasks.results]
```

## Features

- **Queue-based Task Consumption** - Subscribes to NATS `tasks.schedule` subject
- **Load Balancing** - Uses NATS queue groups to distribute tasks across multiple workers
- **Command Execution** - Executes arbitrary shell commands with timeout protection
- **Result Publishing** - Sends execution results to `tasks.results` queue
- **Graceful Shutdown** - Handles SIGINT/SIGTERM signals properly
- **Health Checks** - HTTP endpoint for monitoring worker status

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8082` | HTTP server port for health checks |
| `NATS_URL` | `nats://nats:4222` | NATS server connection URL |
| `TASK_TIMEOUT` | `5m` | Maximum execution time for tasks |

## Task Message Format

Workers consume JSON messages from the `tasks.schedule` queue:

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "schedule": "0 */5 * * * *",
  "command": ["echo", "Hello World"],
  "status": "pending",
  "created_at": "2025-12-04T10:00:00Z",
  "updated_at": "2025-12-04T10:00:00Z"
}
```

## Result Message Format

Workers publish JSON messages to the `tasks.results` queue:

```json
{
  "task_id": "550e8400-e29b-41d4-a716-446655440000",
  "success": true,
  "output": "Hello World\n",
  "error": "",
  "start_time": "2025-12-04T10:05:00Z",
  "end_time": "2025-12-04T10:05:01Z",
  "duration": "1.234s"
}
```

## API Endpoints

### Health Check
- `GET /health` - Check worker status

Response:
```json
{
  "status": "ok",
  "service": "worker",
  "nats": "connected"
}
```

## Running the Worker

### Docker Compose
```bash
docker-compose up worker
```

### Standalone
```bash
cd apps/worker
go run main.go
```

## Scaling Workers

Workers use NATS queue groups for automatic load balancing. To scale horizontally:

```yaml
# docker-compose.yaml
worker:
  deploy:
    replicas: 3  # Run 3 worker instances
```

Or with Docker Compose:
```bash
docker-compose up --scale worker=3
```

All workers subscribe to the same queue group, so NATS automatically distributes tasks across them.

## Command Execution

The worker executes commands using Go's `os/exec` package with the following safeguards:

- **Timeout Protection** - Commands are killed if they exceed `TASK_TIMEOUT`
- **Output Capture** - Both stdout and stderr are captured
- **Error Handling** - Execution failures are logged and reported
- **Context Cancellation** - Commands can be interrupted gracefully

### Supported Commands

Any command available in the worker's container environment can be executed:

```json
{"command": ["echo", "hello"]}
{"command": ["curl", "https://api.example.com"]}
{"command": ["python3", "script.py"]}
{"command": ["sh", "-c", "ls -la && pwd"]}
```

## Monitoring

- **Logs** - Structured JSON logs with slog
- **Health Endpoint** - HTTP health check at `/health`
- **Metrics** - Can be integrated with Prometheus (future enhancement)

## Error Handling

The worker handles various error scenarios:

1. **Invalid Task JSON** - Logs error and skips message
2. **Empty Command** - Returns failure result with error
3. **Command Timeout** - Returns failure with timeout error
4. **Execution Failure** - Returns failure with stderr output
5. **NATS Connection Loss** - Worker will reconnect automatically (NATS built-in)

## Development

### Project Structure
```
apps/worker/
├── main.go                      # Entry point and HTTP server
├── config/
│   └── config.go               # Queue subject constants
├── internal/
│   ├── consumer/
│   │   └── consumer.go         # NATS consumer and message handling
│   └── executor/
│       └── executor.go         # Command execution logic
└── README.md
```

### Testing Locally

1. Start dependencies:
```bash
docker-compose up -d postgres nats
```

2. Run worker:
```bash
cd apps/worker
go run main.go
```

3. Publish a test task to NATS:
```bash
nats pub tasks.schedule '{"id":"test-123","command":["echo","test"],"status":"pending"}'
```

4. Check results:
```bash
nats sub tasks.results
```
