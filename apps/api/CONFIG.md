# API Gateway Configuration Guide

The API Gateway supports configuration through both YAML files and environment variables. Environment variables take precedence over YAML configuration.

## Configuration Methods

### 1. Environment Variables Only (Default)
The API Gateway will use default values if no config file is provided:

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
  workflow_start_subject: "workflow.commands.start"
  workflow_cancel_subject: "workflow.commands.cancel"
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
| `nats.workflow_start_subject` | string | `"workflow.commands.start"` | Subject to publish "start workflow" commands to |
| `nats.workflow_cancel_subject` | string | `"workflow.commands.cancel"` | Subject to publish "cancel workflow" commands to |
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
