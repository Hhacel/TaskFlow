# Worker Configuration Guide

The worker supports configuration through both YAML files and environment variables. Environment variables take precedence over YAML configuration.

## Configuration Methods

### 1. Environment Variables Only (Default)
The worker will use default values if no config file is provided:

```bash
PORT=8082
NATS_URL=nats://nats:4222
TASK_TIMEOUT=5m
LOG_LEVEL=info
```

### 2. YAML Configuration File
Create a `config.yaml` file and specify its path:

```bash
CONFIG_PATH=/path/to/config.yaml
```

Example `config.yaml`:
```yaml
server:
  port: "8082"

nats:
  url: "nats://nats:4222"
  task_schedule_subject: "tasks.schedule"
  task_result_subject: "tasks.results"
  queue_group_name: "workers"
  reconnect_wait_seconds: 2
  max_reconnects: 60

worker:
  task_timeout_minutes: 5
  max_concurrent_tasks: 10

logging:
  level: "info"
  format: "json"
```

### 3. Hybrid Approach (Recommended)
Use YAML for base configuration and override with environment variables:

```yaml
# config.yaml - Production defaults
server:
  port: "8082"

nats:
  url: "nats://nats-cluster:4222"
  queue_group_name: "workers"
```

```bash
# Override in different environments
PORT=9000                    # Override port
NATS_URL=nats://localhost:4222  # Override NATS URL
TASK_TIMEOUT=10m             # Override timeout
```

## Configuration Reference

### Server Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `server.port` | string | `"8082"` | HTTP server port |

**Environment Override:** `PORT`

### NATS Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `nats.url` | string | `"nats://nats:4222"` | NATS server URL |
| `nats.task_schedule_subject` | string | `"tasks.schedule"` | Subject to consume tasks from |
| `nats.task_result_subject` | string | `"tasks.results"` | Subject to publish results to |
| `nats.queue_group_name` | string | `"workers"` | Queue group for load balancing |
| `nats.reconnect_wait_seconds` | int | `2` | Seconds between reconnect attempts |
| `nats.max_reconnects` | int | `60` | Maximum reconnection attempts (-1 for unlimited) |

**Environment Overrides:**
- `NATS_URL` - Overrides `nats.url`

### Worker Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `worker.task_timeout_minutes` | int | `5` | Maximum task execution time in minutes |
| `worker.max_concurrent_tasks` | int | `10` | Maximum tasks to process simultaneously |

**Environment Overrides:**
- `TASK_TIMEOUT` - Overrides timeout (supports duration strings like `5m`, `1h`, `30s`)

### Logging Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `logging.level` | string | `"info"` | Log level: `debug`, `info`, `warn`, `error` |
| `logging.format` | string | `"json"` | Log format: `json` or `text` |

**Environment Overrides:**
- `LOG_LEVEL` - Overrides `logging.level`

## Docker Compose Usage

### Using Environment Variables Only
```yaml
worker:
  environment:
    - PORT=8082
    - NATS_URL=nats://nats:4222
    - TASK_TIMEOUT=5m
```

### Using Config File
```yaml
worker:
  environment:
    - CONFIG_PATH=/app/config.yaml
  volumes:
    - ./apps/worker/config.yaml:/app/config.yaml:ro
```

### Hybrid Approach
```yaml
worker:
  environment:
    - CONFIG_PATH=/app/config.yaml
    - PORT=9000  # Override config file
    - NATS_URL=nats://custom:4222
  volumes:
    - ./apps/worker/config.yaml:/app/config.yaml:ro
```

## Validation

The worker validates configuration on startup and will exit with an error if:
- Required fields are missing
- Values are invalid (e.g., negative timeout)
- Config file is malformed YAML

## Viewing Current Configuration

Access the `/config` endpoint to see the active configuration:

```bash
curl http://localhost:8082/config
```

Response:
```json
{
  "server": {
    "port": "8082"
  },
  "nats": {
    "url": "nats://nats:4222",
    "queue_group": "workers"
  },
  "worker": {
    "task_timeout_minutes": 5,
    "max_concurrent_tasks": 10
  }
}
```

## Examples

### Development (Local)
```yaml
server:
  port: "8082"

nats:
  url: "nats://localhost:4222"

worker:
  task_timeout_minutes: 1
  max_concurrent_tasks: 5

logging:
  level: "debug"
  format: "text"
```

### Production
```yaml
server:
  port: "8082"

nats:
  url: "nats://nats-cluster-01:4222,nats-cluster-02:4222,nats-cluster-03:4222"
  reconnect_wait_seconds: 5
  max_reconnects: -1  # Unlimited reconnects

worker:
  task_timeout_minutes: 10
  max_concurrent_tasks: 50

logging:
  level: "info"
  format: "json"
```

### Testing
```yaml
server:
  port: "8082"

nats:
  url: "nats://localhost:4222"

worker:
  task_timeout_minutes: 1
  max_concurrent_tasks: 1

logging:
  level: "debug"
  format: "text"
```
