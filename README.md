# AtlasPay - Distributed Order & Payment Platform

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://docker.com/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-326CE5?style=flat&logo=kubernetes)](https://kubernetes.io/)

A distributed order and payment platform built with Go, demonstrating saga orchestration, payment idempotency, cache-aside reads, observability, and cloud-native deployment patterns.

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              API Gateway                                 │
│                    (Auth, Rate Limiting, Routing)                       │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
          ┌─────────────────────────┼─────────────────────────┐
          ▼                         ▼                         ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Order Service  │     │ Payment Service │     │Inventory Service│
│                 │     │                 │     │                 │
│ - Order CRUD    │     │ - Process Pay   │     │ - Stock Mgmt    │
│ - State Machine │     │ - Idempotency   │     │ - Reservations  │
│ - Redis Cache   │     │ - Refunds       │     │ - Opt. Locking  │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │     Apache Kafka        │
                    │   (Event Streaming)     │
                    └────────────┬────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              ▼                  ▼                  ▼
      ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
      │  PostgreSQL  │  │    Redis     │  │   Jaeger     │
      │  (Primary)   │  │   (Cache)    │  │  (Tracing)   │
      └──────────────┘  └──────────────┘  └──────────────┘
```

## ✨ Key Features

| Feature | Implementation | Technical Notes |
|---------|---------------|------------------|
| **Saga Pattern** | Orchestrated distributed transactions | Compensating transactions, failure recovery |
| **Event-Driven Order Flow** | Kafka-backed `order.created` processing | Async saga trigger with bounded retries and DLQ persistence |
| **Caching** | Redis cache-aside pattern | Faster repeated reads with PostgreSQL as source of truth |
| **Auth** | JWT with refresh token rotation | RBAC, secure session management |
| **Observability** | Prometheus + Grafana + Jaeger infrastructure | Request metrics, error rates, dashboard-ready telemetry |
| **Rate Limiting** | Token bucket algorithm | Per-IP/user limiting |
| **Chaos Testing** | Failure injection scripts | Kafka down, DB slow, Redis failure |
| **Load Testing** | k6 staged-load script | Throughput and latency validation workflow |
| **Visual Dashboard** | Premium Vanilla JS & CSS | Demo-ready UI, real-time tracking |

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- (Optional) kubectl for Kubernetes deployment

### Local Development

```bash
# 1. Clone and navigate
cd AtlasPay

# 2. Start infrastructure
docker-compose up -d postgres redis kafka

# 3. Install dependencies
go mod download

# 4. Run the API Gateway
go run cmd/api-gateway/main.go
```

### Full Stack with Monitoring

```bash
# Start everything including Prometheus, Grafana, Jaeger
docker-compose up -d

# Access:
# - API: http://localhost:8080
# - Grafana: http://localhost:3000 (admin/admin123)
# - Jaeger: http://localhost:16686
# - Kafka UI: http://localhost:8090

# 5. Live Dashboard (Demo Mode)
# Open web/index.html in your browser
```

## 🎥 Demo & Learning Resources
- **[Premium Dashboard](web/index.html)**: Visualize Saga states and system health.
- **[User Story Scenarios](docs/USER_STORIES.md)**: Real-world business cases (Happy path vs Payment failure).
- **[Local Deployment Notes](docs/FREE_DEPLOYMENT.md)**: Free/local options for validating and recording the system.
- **[Current State](docs/CURRENT_STATE.md)**: Implemented behavior, validation targets, and production-hardening roadmap.
- **[Performance Results](docs/PERFORMANCE_RESULTS.md)**: Template for measured throughput and latency evidence.

## 📊 API Endpoints

### Auth
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/register` | Register new user |
| POST | `/api/auth/login` | Login (returns access + refresh tokens) |
| POST | `/api/auth/refresh` | Rotate tokens |
| POST | `/api/auth/logout` | Revoke refresh token |

### Orders (Protected)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/orders` | Create order |
| GET | `/api/orders` | List user's orders |
| GET | `/api/orders/{id}` | Get order details |
| PATCH | `/api/orders/{id}/cancel` | Cancel order |

### Payments (Protected)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/payments` | Process payment (with idempotency key) |
| GET | `/api/payments/{id}` | Get payment details |
| POST | `/api/payments/{id}/refund` | Refund payment (admin only) |

### Inventory
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/inventory/{sku}` | Check stock |
| POST | `/api/inventory/reserve` | Reserve stock |
| POST | `/api/inventory/release` | Release reservation |

## 🔄 Saga: Order Placement Flow

```mermaid
stateDiagram-v2
    [*] --> OrderCreated
    OrderCreated --> InventoryReserving
    InventoryReserving --> InventoryReserved: success
    InventoryReserving --> OrderFailed: insufficient stock
    InventoryReserved --> PaymentProcessing
    PaymentProcessing --> PaymentSuccess: success
    PaymentProcessing --> InventoryReleasing: payment failed
    InventoryReleasing --> OrderFailed
    PaymentSuccess --> OrderConfirmed
    OrderConfirmed --> [*]
    OrderFailed --> [*]
```

**Compensating Transactions:**
- Payment fails → Inventory automatically released
- Any step fails → Previous steps compensated in reverse order

## 📈 Performance Targets

| Metric | Result |
|--------|--------|
| Requests/min | Validated through k6 load-test runs |
| P95 Latency | Tracked through Prometheus histograms and k6 thresholds |
| P99 Latency | Tracked during load-test runs |
| Cache Hit Rate | Exposed through cache metrics |
| Error Rate | Tracked through HTTP and load-test metrics |

## 🧪 Testing

### Unit Tests
```bash
go test ./... -v -cover
```

### Load Tests (k6)
```bash
# Install k6 first
k6 run scripts/k6/load-test.js
```

### Chaos Tests
```bash
./chaos/run-tests.sh
```

## ☸️ Kubernetes Deployment

```bash
# Apply infrastructure
kubectl apply -f deployments/kubernetes/infrastructure.yaml

# Deploy API Gateway
kubectl apply -f deployments/kubernetes/api-gateway.yaml

# Check HPA status
kubectl get hpa
```

## 🏛️ Project Structure

```
AtlasPay/
├── cmd/                    # Service entrypoints
│   └── api-gateway/
├── internal/
│   ├── common/             # Shared code
│   │   ├── auth/           # JWT + RBAC
│   │   ├── cache/          # Redis wrapper
│   │   ├── config/         # Configuration
│   │   ├── database/       # PostgreSQL
│   │   ├── kafka/          # Producer/Consumer
│   │   ├── logger/         # Structured logging
│   │   ├── metrics/        # Prometheus
│   │   ├── middleware/     # HTTP middleware
│   │   └── saga/           # Saga orchestrator
│   ├── auth/               # Auth domain
│   ├── order/              # Order domain
│   ├── payment/            # Payment domain
│   └── inventory/          # Inventory domain
├── pkg/events/             # Shared event schemas
├── deployments/            # Docker, K8s configs
├── chaos/                  # Chaos testing
├── scripts/                # DB migrations, k6 tests
└── grafana/                # Dashboard configs
```

## 💡 Key Technical Talking Points

1. **Saga flow**
   → Explain saga with order→inventory→payment flow and compensations

2. **Failure handling**
   → Explain compensation, idempotency, retry boundaries, and chaos-test scenarios

3. **Observability**
   → Show Grafana dashboards: p95 latency, error rate, saga metrics

4. **Scaling path**
   → k6 results, HPA configuration, Redis caching strategy

5. **Operational tradeoffs**
   → Cache hit rates, autoscaling policies, connection pooling

## 📄 License

MIT

---

**Built with ❤️ for FUTURE.**
