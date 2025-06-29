# Web Analyzer

A distributed web analysis system with a microservices architecture for comprehensive website analysis, link verification, and content evaluation.

## System Architecture

Web Analyzer is built on a scalable, event-driven microservices architecture. It consists of four main components: a user-facing API, an asynchronous Analyzer service, a real-time Notification backplane, and a React-based frontend.

- **API Service** (`/api`): A RESTful API for submitting analysis jobs and querying results.
- **Analyzer Service** (`/analyzer`): A background worker that performs the core analysis of a given URL.
- **Notification Service** (`/notifications`): A WebSocket service that pushes real-time progress updates to the frontend.
- **Frontend** (`/app`): A React/TypeScript single-page application for user interaction.

Services communicate asynchronously via a **NATS message bus**, ensuring loose coupling and scalability. Data is persisted in **DynamoDB**.

### Architecture Diagram

![Architecture](https://github.com/user-attachments/assets/4f820449-7f1d-4816-b5a6-e0f9adc67230)

## Core Features

- **Comprehensive Site Analysis**: Extracts HTML structure, headings, forms, and internal/external links.
- **In-Depth Link Verification**: Concurrently validates internal and external links, identifying broken links (Maximum concurrency is configurable).
- **Real-Time Progress**: Delivers live updates on analysis progress directly to the UI via WebSockets.
- **Scalable & Distributed**: Designed for horizontal scaling with stateless services and a message-driven workflow.
- **Full Observability**: Integrated metrics, distributed tracing, and health checks for complete system monitoring.

## Tech Stack

| Category         | Technology / Library                                                                                                                              |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Backend** | [Go](https://golang.org/), [yousuf64/shift](https://github.com/yousuf64/shift) (the fastest HTTP Router), [Gorilla WebSocket](https://github.com/gorilla/websocket), [AWS SDK for Go](https://aws.amazon.com/sdk-for-go), [ULID](https://github.com/oklog/ulid), [golang.org/x/net/html](https://pkg.go.dev/golang.org/x/net/html) |
| **Frontend**     | [React](https://reactjs.org/), [TypeScript](https://www.typescriptlang.org/), [Vite](https://vitejs.dev/), [Tailwind CSS](https://tailwindcss.com/)     |
| **Database**     | [Amazon DynamoDB](https://aws.amazon.com/dynamodb/)                                                                                               |
| **Messaging**    | [NATS](https://nats.io/)                                                                                                                          |
| **Testing**      | [GoMock](https://github.com/uber-go/mock), [Testify](https://github.com/stretchr/testify)                                                          |
| **Observability**| [Prometheus](https://prometheus.io/), [OpenTelemetry](https://opentelemetry.io/), [Zipkin](https://zipkin.io/), [slog](https://pkg.go.dev/log/slog) (Structured Logging) |
| **Containerization**| [Docker](https://www.docker.com/), [Docker Compose](https://docs.docker.com/compose/)                                                           |

## Getting Started

You can run the system in two ways: entirely within Docker (recommended for ease of use) or with a hybrid setup for local development.

### Prerequisites
- Go 1.24+
- Node.js 22+
- [Docker](https://docs.docker.com/get-docker/) & [Docker Compose](https://docs.docker.com/compose/install/)

### 1. Run Everything in Docker (Recommended)

This single command builds and starts the entire application stack.

```bash
docker-compose up --build
```
To run in detached mode, add the `-d` flag.

### 2. Hybrid Development (Local Services + Docker Infra)

This setup is ideal for developing the Go or React services locally.

**Step 1: Start Infrastructure**
This command starts NATS, DynamoDB, and other shared infrastructure in Docker.
```bash
docker-compose -f docker-compose.infra.yml up
```

**Step 2: Run Services Locally**
In separate terminal windows, run the services you are working on. The services are configured with sensible defaults to connect to the Dockerized infrastructure.

- **API Service**
  ```bash
  cd api && go run ./cmd/main.go
  ```
- **Analyzer Service**
  ```bash
  cd analyzer && go run ./cmd/main.go
  ```
- **Notifications Service**
  ```bash
  cd notifications && go run ./cmd/main.go
  ```
- **Frontend App**
  ```bash
  cd app && npm install && npm run dev
  ```

## Service Endpoints and Ports

When running locally or in hybrid mode, services are available at the following locations:

| Service | Port | URL / Endpoint |
|---|---|---|
| **Frontend App** | `3000` | [http://localhost:3000](http://localhost:3000) |
| **API Service** | `8080` | [http://localhost:8080](http://localhost:8080) |
| **Notifications** | `8081` | `ws://localhost:8081/ws` |
| **NATS Broker** | `4222` | `nats://localhost:4222` |
| **NATS Monitoring** | `8222` | [http://localhost:8222](http://localhost:8222) |
| **DynamoDB Local** | `8000` | `http://localhost:8000` |
| **Zipkin Tracing** | `9411` | [http://localhost:9411](http://localhost:9411) |

## API Specification

The API service (`:8080`) provides the following endpoints for managing analysis jobs.

### `POST /analyze`

Submits a new URL for analysis. This endpoint is asynchronous and will immediately return a job object with a `pending` status.

- **Request Body**:
  ```json
  {
    "url": "https://example.com"
  }
  ```

- **Success Response (`202 Accepted`)**:
  ```json
  {
    "job": {
      "id": "01H8X8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8",
      "url": "https://example.com",
      "status": "pending",
      "created_at": "2023-01-01T12:00:00Z",
      ...
    }
  }
  ```

### `GET /jobs`

Retrieves a list of all analysis jobs that have been submitted.

- **Success Response (`200 OK`)**:
  ```json
  [
    {
      "id": "01H8X8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8",
      "url": "https://example.com",
      "status": "completed",
      "created_at": "2023-01-01T12:00:00Z",
      ...
    }
  ]
  ```

### `GET /jobs/:job_id/tasks`

Retrieves all analysis tasks associated with a specific `job_id`.

- **Success Response (`200 OK`)**:
  ```json
  [
    {
      "job_id": "01H8X8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8",
      "type": "verifying_links",
      "status": "running",
      "subtasks": { ... }
    }
  ]
  ```

## Messaging Specification

Services communicate via NATS. The `analyzer` service consumes analysis requests and produces status updates.

### Consumed Messages

#### `url.analyze`

The `analyzer` service subscribes to this topic to receive new analysis jobs from the API service.

- **Message Body (`AnalyzeMessage`)**:
  ```json
  {
    "type": "url.analyze",
    "job_id": "01H8X8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8"
  }
  ```

### Produced Messages

#### `job.update`

Published when the overall job status changes (e.g., from `running` to `completed`).

- **Message Body (`JobUpdateMessage`)**:
  ```json
  {
    "type": "job.update",
    "job_id": "01H8X8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8",
    "status": "completed",
    "result": { ... }
  }
  ```

#### `task.status_update`

Published when a high-level task changes state (e.g., `html_analysis` starts or finishes).

- **Message Body (`TaskStatusUpdateMessage`)**:
  ```json
  {
    "type": "task.status_update",
    "job_id": "01H8X8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8",
    "task_type": "verifying_links",
    "status": "running"
  }
  ```

#### `task.subtask_update`

Published to provide real-time progress on individual, granular sub-tasks (e.g., checking a single link).

- **Message Body (`SubTaskUpdateMessage`)**:
  ```json
  {
    "type": "task.subtask_update",
    "job_id": "01H8X8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8",
    "task_type": "verifying_links",
    "key": "1",
    "subtask": {
      "type": "validating_link",
      "status": "completed",
      "url": "https://example.com/some-link"
      "description": "HTTP 200: OK"
    }
  }
  ```

## WebSocket Specification

The Notification service (`:8081`) acts as a WebSocket backplane, consuming NATS messages and broadcasting them to connected frontend clients.

### Connection

Clients can establish a WebSocket connection at the following endpoint:

- **Endpoint**: `ws://localhost:8081/ws`

Upon connection, a client can send messages to subscribe to or unsubscribe from updates for a specific job.

- **Client Subscription Message**:
  ```json
  {
    "action": "subscribe",
    "job_id": "01H8X8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8"
  }
  ```

- **Client Unsubscription Message**:
  ```json
  {
    "action": "unsubscribe",
    "job_id": "01H8X8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8"
  }
  ```

### WebSocket Messages

Once subscribed, the server will push events to the client. The message structures are identical to those in the [Messaging Specification](#messaging-specification).

- **Job Update (`job.update`)**: Broadcast to **all connected clients** when any job's overall status changes.
- **Task Status Update (`task.status_update`)**: Sent only to clients who have subscribed to the relevant `job_id` when a major task's status changes.
- **Sub-Task Update (`task.subtask_update`)**: Sent only to clients subscribed to the relevant `job_id` for granular progress on sub-tasks.

**Example Payload (`task.subtask_update`)**:
```json
{
  "type": "task.subtask_update",
  "job_id": "01H8X8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8",
  "task_type": "verifying_links",
  "key": "1",
  "subtask": {
    "type": "validating_link",
    "status": "completed",
    "url": "https://example.com/some-link"
    "description": "HTTP 200: OK"
  }
}
```

## Observability

Each Go service exposes Prometheus-compatible metrics and a health check endpoint.

| Service | Metrics Endpoint | Health Endpoint |
|---|---|---|
| **API Service** | [http://localhost:9090/metrics](http://localhost:9090/metrics) | [http://localhost:9090/health](http://localhost:9090/health) |
| **Analyzer Service** | [http://localhost:9091/metrics](http://localhost:9091/metrics) | [http://localhost:9091/health](http://localhost:9091/health) |
| **Notification Service** | [http://localhost:9092/metrics](http://localhost:9092/metrics) | [http://localhost:9092/health](http://localhost:9092/health) |

Distributed traces can be viewed in the Zipkin UI at `http://localhost:9411`.

## Future Improvements

This project has a solid foundation, but there are several opportunities for future enhancements:

- **Single-Page Application (SPA) Analysis**: Enhance the `analyzer` to use a headless browser to fully render JavaScript-heavy SPAs, allowing for analysis of the final DOM state rather than just the initial HTML payload.
- **Crawler Evasion Techniques**: Implement strategies to handle websites that block crawlers, such as user-agent rotation and support for proxy services.
- **Granular Sub-Tasks for All Analyses**: Extend the real-time progress reporting to show sub-task updates for all analysis types, not just link verification.
- **Performance & Accessibility Audits**: Incorporate tools like Google's Lighthouse to provide detailed reports on website performance, SEO best practices, and accessibility (WCAG) compliance.
- **User Authentication**: Implement user accounts to allow users to manage their own history of analysis jobs securely.
- **Caching Layer**: Introduce a caching layer (e.g., Redis) to store the results of recent analyses. If a user requests an analysis for a URL that has been recently processed, the cached result can be served immediately, reducing redundant processing and providing a faster user experience.
- **Notification Service Partitioning**: `notifications` service could struggle when having large number of active WebSocket connections. It could be enhanced to support partitioning, where multiple instances of the service handle distinct subsets of WebSocket connections, improving scalability and resilience.

## Development

### Prerequisites
- Go 1.24+
- Node.js 22+
- Docker and docker-compose

### Testing
Each service includes unit tests that can be run with:
```bash
go test ./...
```
### Configuration
All services use environment variables with sensible defaults for local development. 