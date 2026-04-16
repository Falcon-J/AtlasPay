# AtlasPay Current State

This document separates implemented behavior from validation targets.

## Implemented

- Go API gateway with auth, order, payment, and inventory domains.
- PostgreSQL persistence for users, orders, payments, inventory, reservations, saga logs, and dead-letter events.
- Redis cache-aside reads for order and inventory lookups.
- Saga-style checkout workflow with inventory reservation, payment processing, inventory commit, order confirmation, and compensation on payment failure.
- Payment idempotency through `idempotency_key` and a unique database constraint.
- Kafka-backed async order trigger when `KAFKA_ENABLED=true`:
  - `POST /api/orders` persists the order and publishes `order.created`.
  - The order worker consumes `atlaspay.orders` and runs the saga.
- Kafka event handling with 3 bounded attempts and backoff.
- Dead-letter persistence in `dead_letter_events` after retry exhaustion.
- Prometheus metrics for HTTP requests, saga outcomes, cache hits/misses, Kafka events, retries, and DLQ writes.
- Docker Compose infrastructure for PostgreSQL, Redis, Kafka, Prometheus, Grafana, Jaeger, and the API gateway.
- Kubernetes manifests for API gateway, PostgreSQL, Redis, Kafka/Zookeeper, HPA, probes, resource requests, and services.

## Validation Targets

- `10k+ RPM` and `p95 <= 120ms` require a saved k6 result from the target environment.
- `99.9% uptime` requires a real deployed environment and historical monitoring.
- `40% failure reduction` requires a before/after failure experiment. The implemented mechanisms prevent duplicate payment records and compensate inventory reservations in tested failure cases, but the percentage must be measured before it is quoted as a result.

## Production Hardening Roadmap

- Add transactional outbox so order database writes and Kafka publication cannot drift apart.
- Persist resumable saga instance state beyond the current in-memory orchestrator map.
- Split order, payment, and inventory into independently deployed services if full microservice deployment is required.
- Add integration tests against real PostgreSQL, Redis, and Kafka containers.
- Add OpenTelemetry tracing for request, event, and saga step spans.
- Capture k6 and Kubernetes validation outputs under `docs/`.
