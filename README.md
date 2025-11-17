# 🔄 TaskFlow: A Distributed Task Scheduling System

> **A modern, scalable microservices-based system for distributed task orchestration**

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://golang.org/)

TaskFlow is a **production-ready microservices ecosystem** built in Go for scheduling, executing, and monitoring distributed tasks. Designed with **scalability**, **resilience**, and **observability** at its core, using gRPC for type-safe inter-service communication and Prometheus for comprehensive monitoring.

## 🏗️ High-Level Architecture

The platform consists of **specialized microservices** that communicate via gRPC and message queues (RabbitMQ/NATS) for robust, fault-tolerant task processing.

### 📊 System Architecture
```mermaid
flowchart LR
    %% Row 1: User Input
    User["User<br/>Client"] 
    
    %% Row 2: Gateway
    GW["API Gateway<br/>apps/api-gateway"]
    
    %% Row 3: Core Services (horizontal alignment)
    SCH["Scheduler<br/>apps/scheduler"]
    AGG["Aggregator<br/>apps/aggregator"]
    NOT["Notifier<br/>apps/notifier"]
    
    %% Row 4: Worker
    WRK["Worker<br/>apps/worker"]
    
    %% Row 5: External
    External["External APIs"]
    Email["Email/Slack<br/>Webhooks"]
    
    %% Data Layer (bottom row)
    DB[("Database")]
    MQ_Task["Task Queue"]
    MQ_Result["Result Queue"]
    
    %% Supporting (separate area)
    Proto["Proto<br/>Contracts"]
    Prom["Prometheus<br/>Monitoring"]

    %% Main Flow (numbered for clarity)
    User -->|"1. Request"| GW
    GW -->|"2. Create"| SCH
    SCH -->|"3. Store"| DB
    SCH -->|"4. Queue"| MQ_Task
    MQ_Task -->|"5. Consume"| WRK
    WRK -->|"6. Execute"| External
    WRK -->|"7. Result"| MQ_Result
    MQ_Result -->|"8. Process"| AGG
    AGG -->|"9. Update"| DB
    AGG -->|"10. Status"| GW
    AGG -->|"11. Notify"| NOT
    NOT -->|"12. Alert"| Email

    %% Support Connections (minimal intersections)
    Proto -.-> GW
    Proto -.-> SCH  
    Proto -.-> AGG
    Proto -.-> NOT
    Proto -.-> WRK
    
    Prom -.-> GW
    Prom -.-> SCH
    Prom -.-> AGG
    Prom -.-> NOT
    Prom -.-> WRK

    %% Styling
    classDef service fill:#e3f2fd,stroke:#1976d2,stroke-width:2px,color:#000000
    classDef data fill:#f3e5f5,stroke:#7b1fa2,stroke-width:2px,color:#000000
    classDef external fill:#e8f5e8,stroke:#388e3c,stroke-width:2px,color:#000000
    classDef support fill:#fff3e0,stroke:#f57c00,stroke-width:2px,color:#000000
    
    class GW,SCH,AGG,NOT,WRK service
    class DB,MQ_Task,MQ_Result data
    class User,External,Email external
    class Proto,Prom support
```

## 🧩 Core Components

### 🚀 Microservices (`apps/`)
Each service is a specialized, independently deployable component:

#### 🚪 **API Gateway** (`apps/api-gateway`)
> *The system's front door*

- **Authentication & Authorization** - JWT validation and user management
- **Request Validation** - Input sanitization and schema validation  
- **Rate Limiting** - Traffic control and abuse prevention
- **Request Routing** - Intelligent traffic distribution to downstream services

#### 📅 **Scheduler** (`apps/scheduler`) 
> *The orchestration brain*

- **Task Validation** - Business logic and constraint checking
- **Persistence Management** - Database operations and state tracking
- **Queue Publishing** - Message broker integration for task dispatch
- **Scheduling Logic** - Cron-based and event-driven task triggering

#### ⚡ **Worker** (`apps/worker`)
> *The execution engine*

- **Queue Consumption** - Real-time task processing from message queues
- **Task Execution** - Pluggable task handlers for diverse workloads
- **Result Publishing** - Status updates and output data management
- **Error Handling** - Retry logic and failure recovery mechanisms

#### 📈 **Aggregator** (`apps/aggregator`)
> *The data consolidation hub*

- **Result Processing** - Task outcome analysis and storage
- **Status Management** - Real-time state updates and history tracking
- **API Services** - RESTful endpoints for status queries
- **Event Triggering** - Notification and alerting coordination

#### 🔔 **Notifier** (`apps/notifier`)
> *The communication gateway*

- **Multi-channel Delivery** - Email, Slack, webhooks, and more
- **Event Processing** - Smart filtering and routing based on conditions
- **Delivery Confirmation** - Reliable notification with retry mechanisms
- **Template Management** - Customizable message formatting

---

### 📚 Shared Libraries (`pkg/`)
Reusable components across all services:

#### 🔐 **Authentication** (`pkg/auth`)
- JWT token generation, validation, and middleware
- Role-based access control (RBAC) utilities
- Session management and security helpers

---

### 🔌 Integration Layer

#### 📋 **Protocol Definitions** (`proto/`)
> *Single source of truth for all service contracts*

- **gRPC Service Definitions** - Type-safe API specifications
- **Message Schemas** - Structured data contracts
- **Code Generation** - Auto-generated client/server stubs
- **Version Management** - Backward-compatible API evolution

#### 🗄️ **Database Migrations** (`migrations/`)
- **Schema Evolution** - Version-controlled database changes
- **Data Migrations** - Safe data transformation scripts
- **Rollback Support** - Reversible database operations

#### 🛠️ **Deployment Configuration** (`deployments/`)

##### 📊 **Prometheus Setup** (`deployments/prometheus/`)
- **Metrics Collection** - Service health and performance monitoring
- **Alerting Rules** - Proactive issue detection and notification
- **Dashboard Configuration** - Grafana integration and visualization

## 🔄 Task Lifecycle & Data Flow

> **End-to-end journey of a task through the TaskFlow ecosystem**

### 📋 Process Overview

```
User Request → Authentication → Scheduling → Persistence → Execution → Aggregation → Notification
```

### 🔢 Detailed Flow Steps

| Step | Component | Action | Description |
|------|-----------|--------|-------------|
| **1** | 👤 **User** | `HTTP Request` | Client submits task creation request |
| **2** | 🚪 **API Gateway** | `Authentication` | JWT validation via `pkg/auth` |
| **3** | 🚪 **API Gateway** | `Route Request` | gRPC call to scheduler (via `proto` contracts) |
| **4** | 📅 **Scheduler** | `Validate & Store` | Task validation, ID assignment, DB persistence (`pending` status) |
| **5** | 📅 **Scheduler** | `Queue Dispatch` | Publish task message to **Task Queue** |
| **6** | ⚡ **Worker** | `Consume & Execute` | Pick up task, update status to `running`, execute business logic |
| **7** | ⚡ **Worker** | `External Integration` | Call external APIs, process data, perform work |
| **8** | ⚡ **Worker** | `Publish Result` | Send outcome (success/failure/logs) to **Result Queue** |
| **9** | 📈 **Aggregator** | `Process Result` | Consume result, update final status (`completed`/`failed`) |
| **10** | 📈 **Aggregator** | `Store Logs` | Persist execution logs and task history |
| **11** | 📈 **Aggregator** | `Trigger Event` | Send notification event to notifier (if configured) |
| **12** | 🔔 **Notifier** | `Send Alert` | Deliver notifications via email, Slack, webhooks |

### 🔍 Continuous Monitoring

Throughout the entire lifecycle, **Prometheus** continuously scrapes `/metrics` endpoints from all services, providing:

- 📊 **Real-time Metrics** - Performance, throughput, and health indicators
- 🚨 **Alerting** - Proactive issue detection and escalation
- 📈 **Dashboards** - Visual monitoring and operational insights
- 🔍 **Troubleshooting** - Detailed logs and trace correlation

---

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/Hhacel/TaskFlow.git
cd TaskFlow

# Start the entire stack
docker-compose up -d

# Check service health
curl http://localhost:8080/health
```

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<div align="center">

**Built with ❤️ using Go, gRPC, and modern cloud-native technologies**

</div>