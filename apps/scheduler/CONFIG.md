# Scheduler Configuration Guide

The scheduler supports configuration through both YAML files and environment variables. Environment variables take precedence over YAML configuration.

## Configuration Methods

### 1. Environment Variables Only (Default)
The scheduler will use default values if no config file is provided:

```bash
PORT=8081
DB_HOST=postgres
DB_PORT=5432
DB_USER=taskflow
DB_PASSWORD=taskflow
DB_NAME=taskflow
NATS_URL=nats://nats:4222
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
  port: "8081"

database:
  host: "postgres"
  port: "5432"
  user: "taskflow"
  password: "taskflow"
  database: "taskflow"
  ssl_mode: "disable"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime_minutes: 5

nats:
  url: "nats://nats:4222"
  task_schedule_subject: "tasks.schedule"
  task_result_subject: "tasks.results"
  reconnect_wait_seconds: 2
  max_reconnects: 60

logging:
  level: "info"
  format: "json"
```

### 3. Hybrid Approach (Recommended)
Use YAML for base configuration and override with environment variables:

```yaml
# config.yaml - Production defaults
server:
  port: "8081"

database:
  host: "postgres-cluster"
  max_open_conns: 50
```

```bash
# Override in different environments
PORT=9000                           # Override port
DB_HOST=localhost                   # Override DB host
DB_PASSWORD=secret                  # Override password
NATS_URL=nats://prod-nats:4222     # Override NATS URL
```

## Configuration Reference

### Server Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `server.port` | string | `"8081"` | HTTP server port |

**Environment Override:** `PORT`

### Database Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `database.host` | string | `"postgres"` | PostgreSQL host |
| `database.port` | string | `"5432"` | PostgreSQL port |
| `database.user` | string | `"taskflow"` | Database user |
| `database.password` | string | `"taskflow"` | Database password |
| `database.database` | string | `"taskflow"` | Database name |
| `database.ssl_mode` | string | `"disable"` | SSL mode (disable/require/verify-full) |
| `database.max_open_conns` | int | `25` | Maximum open connections |
| `database.max_idle_conns` | int | `5` | Maximum idle connections |
| `database.conn_max_lifetime_minutes` | int | `5` | Connection max lifetime in minutes |

**Environment Overrides:**
- `DB_HOST` - Overrides `database.host`
- `DB_PORT` - Overrides `database.port`
- `DB_USER` - Overrides `database.user`
- `DB_PASSWORD` - Overrides `database.password`
- `DB_NAME` - Overrides `database.database`

### NATS Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `nats.url` | string | `"nats://nats:4222"` | NATS server URL |
| `nats.task_schedule_subject` | string | `"tasks.schedule"` | Subject to publish tasks to |
| `nats.task_result_subject` | string | `"tasks.results"` | Subject to consume results from |
| `nats.reconnect_wait_seconds` | int | `2` | Seconds between reconnect attempts |
| `nats.max_reconnects` | int | `60` | Maximum reconnection attempts (-1 for unlimited) |

**Environment Overrides:**
- `NATS_URL` - Overrides `nats.url`

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
scheduler:
  environment:
    - PORT=8081
    - DB_HOST=postgres
    - DB_PASSWORD=secret
    - NATS_URL=nats://nats:4222
```

### Using Config File
```yaml
scheduler:
  environment:
    - CONFIG_PATH=/app/config.yaml
  volumes:
    - ./apps/scheduler/config.yaml:/app/config.yaml:ro
```

### Hybrid Approach
```yaml
scheduler:
  environment:
    - CONFIG_PATH=/app/config.yaml
    - PORT=9000  # Override config file
    - DB_PASSWORD=secret  # Override config file
  volumes:
    - ./apps/scheduler/config.yaml:/app/config.yaml:ro
```

## Validation

The scheduler validates configuration on startup and will exit with an error if:
- Required fields are missing
- Values are invalid (e.g., negative connection counts)
- Config file is malformed YAML
- Cannot connect to database or NATS

## Viewing Current Configuration

Access the `/config` endpoint to see the active configuration:

```bash
curl http://localhost:8081/config
```

Response:
```json
{
  "server": {
    "port": "8081"
  },
  "database": {
    "host": "postgres",
    "port": "5432",
    "database": "taskflow"
  },
  "nats": {
    "url": "nats://nats:4222"
  }
}
```

## Examples

### Development (Local)
```yaml
server:
  port: "8081"

database:
  host: "localhost"
  port: "5432"
  user: "postgres"
  password: "postgres"
  database: "taskflow_dev"
  ssl_mode: "disable"
  max_open_conns: 10
  max_idle_conns: 2

nats:
  url: "nats://localhost:4222"

logging:
  level: "debug"
  format: "text"
```

### Production
```yaml
server:
  port: "8081"

database:
  host: "postgres-primary.production.svc.cluster.local"
  port: "5432"
  user: "taskflow"
  password: "${DB_PASSWORD}"  # From environment
  database: "taskflow"
  ssl_mode: "verify-full"
  max_open_conns: 100
  max_idle_conns: 25
  conn_max_lifetime_minutes: 30

nats:
  url: "nats://nats-01:4222,nats-02:4222,nats-03:4222"
  reconnect_wait_seconds: 5
  max_reconnects: -1  # Unlimited reconnects

logging:
  level: "info"
  format: "json"
```

### Testing
```yaml
server:
  port: "8081"

database:
  host: "localhost"
  port: "5433"
  user: "test"
  password: "test"
  database: "taskflow_test"
  ssl_mode: "disable"
  max_open_conns: 5
  max_idle_conns: 1

nats:
  url: "nats://localhost:4222"

logging:
  level: "debug"
  format: "text"
```

## Database Connection String

The scheduler builds the PostgreSQL connection string from config:

```
host={host} port={port} user={user} password={password} dbname={database} sslmode={ssl_mode}
```

You can verify the connection string is correct by checking the logs on startup.
