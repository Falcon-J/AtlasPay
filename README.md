# AtlasPay — Distributed Payment Platform

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat\&logo=go)](https://go.dev/)
[![Kafka](https://img.shields.io/badge/Kafka-Event--Driven-231F20?style=flat\&logo=apachekafka)](https://kafka.apache.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-Persistence-4169E1?style=flat\&logo=postgresql)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-Cache--Aside-DC382D?style=flat\&logo=redis)](https://redis.io/)
[![Docker](https://img.shields.io/badge/Docker%20Compose-Validated-2496ED?style=flat\&logo=docker)](https://docs.docker.com/compose/)
[![CI](https://github.com/Falcon-J/AtlasPay/actions/workflows/ci.yml/badge.svg)](https://github.com/Falcon-J/AtlasPay/actions/workflows/ci.yml)

AtlasPay is a Kafka-backed distributed checkout and payment platform built with Go, Kafka, PostgreSQL, Redis, and Docker.

It demonstrates backend reliability patterns used in payment and commerce systems: event-driven checkout processing, saga-style coordination, payment idempotency, bounded retries, dead-letter routing, inventory compensation, health checks, metrics, and reproducible Docker-based validation.

---

## Highlights

* Kafka-backed checkout flow using `order.created` events
* Four backend service domains: Auth, Orders, Payments, Inventory
* PostgreSQL persistence for transactional state
* Redis cache-aside reads
* Payment idempotency for safe duplicate/retry handling
* Saga-style inventory compensation on payment failure
* Bounded Kafka consumer retries with backoff
* PostgreSQL dead-letter persistence
* Kafka DLQ publishing to `atlaspay.dlq`
* Docker Compose smoke workflows for success and failure paths
* GitHub Actions workflow for automated validation
* Static frontend demo with live backend mode and browser simulation fallback

---

## Architecture

```mermaid
flowchart TD
    A[Frontend / API Client] --> B[Go API Gateway]

    B --> C[Auth Domain]
    B --> D[Order Domain]
    B --> E[Inventory Domain]
    B --> F[Payment Domain]

    C --> DB[(PostgreSQL)]
    D --> DB
    E --> DB
    F --> DB

    D --> R[(Redis Cache)]
    E --> R

    D --> K[Kafka Topic: order.created]
    K --> W[Kafka Consumer]
    W --> S[Saga Orchestrator]

    S --> E
    S --> F

    W --> Retry[Bounded Retry Handler]
    Retry --> DLQDB[(PostgreSQL dead_letter_events)]
    Retry --> DLQK[Kafka Topic: atlaspay.dlq]

    B --> H[Health Endpoint]
    B --> M[Metrics Endpoint]
```

---

## Core Service Domains

| Domain    | Responsibility                                      |
| --------- | --------------------------------------------------- |
| Auth      | User registration, login, JWT-protected access      |
| Orders    | Order creation, order state, Kafka event publishing |
| Inventory | Stock checks, reservation, release, compensation    |
| Payments  | Payment processing, idempotency, failure handling   |

---

## Checkout Flow

```mermaid
stateDiagram-v2
    [*] --> OrderCreated
    OrderCreated --> EventPublished: publish order.created
    EventPublished --> ConsumerProcessing: Kafka consumer receives event
    ConsumerProcessing --> InventoryReservation
    InventoryReservation --> InventoryReserved: stock available
    InventoryReservation --> OrderFailed: insufficient stock
    InventoryReserved --> PaymentProcessing
    PaymentProcessing --> OrderConfirmed: payment succeeds
    PaymentProcessing --> InventoryCompensation: payment fails
    InventoryCompensation --> OrderCompensated
    OrderConfirmed --> [*]
    OrderFailed --> [*]
    OrderCompensated --> [*]
```

---

## Reliability Behaviors

| Behavior                 | Implementation                                            |
| ------------------------ | --------------------------------------------------------- |
| Event-driven checkout    | Order creation publishes `order.created` to Kafka         |
| Saga processing          | Kafka consumer executes inventory and payment workflow    |
| Payment failure handling | Reserved inventory is released through compensation       |
| Idempotent payments      | Reused idempotency keys return the same payment result    |
| Bounded retries          | Failed consumer handling retries up to a configured limit |
| Dead-letter persistence  | Exhausted events are stored in PostgreSQL                 |
| Kafka DLQ routing        | Dead-lettered events are published to `atlaspay.dlq`      |
| Consumer continuation    | Follow-up events validate the consumer keeps processing   |
| Observability            | Health and metrics endpoints support validation           |

---

## Validation Coverage

AtlasPay includes Docker-based workflows covering success and failure scenarios.

| Case | Covered behavior                           |
| ---- | ------------------------------------------ |
| 1    | Backend health check                       |
| 2    | PostgreSQL connectivity                    |
| 3    | Redis/cache connectivity                   |
| 4    | User registration and login                |
| 5    | Successful order creation                  |
| 6    | Kafka `order.created` publish              |
| 7    | Kafka consumer processing                  |
| 8    | Inventory reservation                      |
| 9    | Payment success and order confirmation     |
| 10   | Payment failure and inventory compensation |
| 11   | Idempotent payment reuse                   |
| 12   | Invalid event retry exhaustion             |
| 13   | PostgreSQL dead-letter persistence         |
| 14   | Kafka DLQ publication                      |
| 15   | Consumer continuation after DLQ event      |
| 16   | Metrics endpoint reachability              |

Evidence files are stored under:

```text
docs/evidence/
```

Key evidence files:

```text
docs/evidence/README.md
docs/evidence/go-test-output.txt
docs/evidence/docker-compose-health.txt
docs/evidence/demo-smoke-output.txt
docs/evidence/dlq-smoke-output.txt
docs/evidence/kafka-smoke-log.txt
```

---

## Tech Stack

| Layer         | Technology                                   |
| ------------- | -------------------------------------------- |
| Backend       | Go 1.25                                      |
| Messaging     | Kafka                                        |
| Database      | PostgreSQL                                   |
| Cache         | Redis                                        |
| Validation    | Docker Compose, Go tests, smoke scripts      |
| Observability | Health checks, Prometheus-compatible metrics |
| Frontend Demo | Static HTML, CSS, JavaScript                 |
| CI            | GitHub Actions                               |

---

## Quick Start

### Prerequisites

* Docker Desktop
* Docker Compose
* PowerShell for smoke scripts
* Go 1.25 optional, because tests can run through Dockerized Go

---

## Run the Full Stack

```bash
docker compose up -d --build
```

Check backend health:

```bash
curl http://localhost:8080/health
```

Expected response shape:

```json
{
  "status": "healthy",
  "db": "up",
  "cache": "up"
}
```

Stop and clean local volumes:

```bash
docker compose down -v
```

---

## Run Tests

Without installing Go locally:

### Bash

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.25 go test ./...
```

### PowerShell

```powershell
docker run --rm -v "${PWD}:/src" -w /src golang:1.25 go test ./...
```

---

## Run Smoke Workflows

Start the stack:

```powershell
docker compose up -d --build
```

Run checkout smoke:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/demo-smoke.ps1
```

Run DLQ smoke:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/dlq-smoke.ps1
```

Check Kafka event logs:

```powershell
docker compose logs api-gateway | Select-String "event published|event processed"
```

Clean up:

```powershell
docker compose down -v
```

---

## GitHub Actions

The CI workflow validates:

* Dockerized `go test ./...`
* Docker Compose configuration
* committed whitespace checks
* bounded Docker Compose startup
* checkout smoke workflow
* DLQ smoke workflow
* Kafka publish/process assertions
* failure-log upload
* unconditional Docker cleanup

Workflow file:

```text
.github/workflows/ci.yml
```

---

## Static Frontend Demo

`web/index.html` is a dependency-free frontend that can run locally or on a static host.

It supports two modes:

### Live Backend Mode

Uses the AtlasPay API after a short `/health` check.

### Demo Simulation Mode

Runs deterministic checkout simulation entirely in the browser when the backend is unavailable.

Demo Simulation Mode visualizes:

* checkout creation
* inventory reservation
* payment failure
* inventory compensation
* revenue correctness
* saga-style logs

To configure a hosted backend API:

```html
<script>
  window.ATLASPAY_API_BASE_URL = "https://api.example.com";
</script>
```

---

## API Endpoints

### Auth

| Method | Endpoint             | Description   |
| ------ | -------------------- | ------------- |
| POST   | `/api/auth/register` | Register user |
| POST   | `/api/auth/login`    | Login         |
| POST   | `/api/auth/refresh`  | Refresh token |
| POST   | `/api/auth/logout`   | Logout        |

### Orders

| Method | Endpoint                  | Description  |
| ------ | ------------------------- | ------------ |
| POST   | `/api/orders`             | Create order |
| GET    | `/api/orders`             | List orders  |
| GET    | `/api/orders/{id}`        | Get order    |
| PATCH  | `/api/orders/{id}/cancel` | Cancel order |

### Payments

| Method | Endpoint                    | Description                          |
| ------ | --------------------------- | ------------------------------------ |
| POST   | `/api/payments`             | Process payment with idempotency key |
| GET    | `/api/payments/{id}`        | Get payment                          |
| POST   | `/api/payments/{id}/refund` | Refund payment, admin only           |

### Inventory

| Method | Endpoint                 | Description       |
| ------ | ------------------------ | ----------------- |
| GET    | `/api/inventory/{sku}`   | Check stock       |
| POST   | `/api/inventory/reserve` | Reserve inventory |
| POST   | `/api/inventory/release` | Release inventory |

### System

| Method | Endpoint   | Description                     |
| ------ | ---------- | ------------------------------- |
| GET    | `/health`  | API, database, and cache health |
| GET    | `/metrics` | Prometheus-compatible metrics   |

---

## Local Load Testing

AtlasPay includes k6 scripts for local load testing.

```bash
k6 run scripts/k6/load-test.js
```

Recommended performance evidence format:

```text
Date:
Machine:
Command:
Duration:
Request rate:
p95 latency:
p99 latency:
Failure rate:
Docker Compose services:
Commit SHA:
```

---

## Project Structure

```text
AtlasPay/
├── .github/
│   └── workflows/              # GitHub Actions CI
├── cmd/
│   └── api-gateway/            # API entrypoint
├── internal/
│   ├── auth/                   # Auth domain
│   ├── order/                  # Order domain
│   ├── inventory/              # Inventory domain
│   ├── payment/                # Payment domain
│   └── common/
│       ├── auth/               # JWT/RBAC helpers
│       ├── cache/              # Redis wrapper
│       ├── config/             # Runtime config
│       ├── database/           # PostgreSQL connection
│       ├── dlq/                # Dead-letter persistence
│       ├── kafka/              # Kafka producer/consumer
│       ├── logger/             # Structured logs
│       ├── metrics/            # Prometheus metrics
│       ├── middleware/         # HTTP middleware
│       └── saga/               # Saga orchestration
├── pkg/
│   └── events/                 # Shared event schemas
├── scripts/
│   ├── demo-smoke.ps1          # Checkout smoke workflow
│   ├── dlq-smoke.ps1           # DLQ smoke workflow
│   └── k6/                     # Load testing scripts
├── docs/
│   ├── evidence/               # Captured validation evidence
│   ├── architecture/           # Architecture notes
│   └── deployment/             # Deployment notes
├── web/
│   └── index.html              # Static checkout simulator
├── docker-compose.yml
├── Dockerfile
├── go.mod
└── README.md
```

---

#Key Tradeoffs 

* event-driven checkout systems
* saga-style compensation
* payment idempotency
* Kafka consumer retries
* dead-letter queues
* PostgreSQL vs Kafka DLQ responsibilities
* Docker Compose based integration validation
* health checks and metrics
* public frontend demo vs backend validation workflow

---

## License

MIT
