# AtlasPay Current State

This document separates implemented behavior from validation targets.

The Kubernetes topology candidate is documented in
`docs/architecture/kubernetes-topology.md`; it is not production deployment
evidence.

## Implemented

- Go API gateway with auth and order domains, plus typed clients for the
  standalone Payment and Inventory processes.
- Independently built and health-checked payment service process. The gateway
  calls its private HTTP contract through a typed client adapter; both processes
  currently share PostgreSQL while ownership is being separated.
- Independently built and health-checked inventory service process. The gateway
  calls its private HTTP contract through a typed client adapter; it shares
  PostgreSQL and Redis while ownership is being separated.
- Independently built and health-checked Order/Saga service process. It owns
  the Kafka order workers, transactional outbox publisher, saga execution,
  durable saga state, and DLQ path; the gateway calls its private HTTP
  contract. PostgreSQL and Redis remain shared during extraction.
- PostgreSQL persistence for users, orders, payments, inventory, reservations, saga logs, and dead-letter events.
- Redis cache-aside reads for order and inventory lookups.
- Saga-style checkout workflow with inventory reservation, payment processing, inventory commit, order confirmation, and compensation on payment failure.
- Payment idempotency through `idempotency_key` and a unique database constraint.
- Kafka-backed async order trigger when `KAFKA_ENABLED=true`:
  - `POST /api/orders` persists the order and publishes `order.created`.
  - The order worker consumes `atlaspay.orders` and runs the saga.
- Kafka event handling with 3 bounded attempts and backoff.
- Dead-letter persistence in `dead_letter_events` after retry exhaustion.
- Transactional outbox for order events, with retry after Kafka recovery.
- Persisted saga snapshots and step logs, with restart reconstruction.
- Inventory reservation idempotency for duplicate `(order_id, sku)` events.
- Lease-backed saga claims for in-flight duplicate events, with reclaim after
  an Order/Saga process crash.
- Configurable Kafka order workers; Compose runs 16 workers over a 16-partition
  orders topic for benchmarkable parallelism.
- Prometheus metrics for HTTP requests, saga outcomes, cache hits/misses, Kafka events, retries, and DLQ writes (optimized to prevent cardinality explosions via dynamic route parameterization).
- Docker Compose infrastructure for PostgreSQL, Redis, Kafka, Prometheus, Grafana, Jaeger, the API gateway, Payment, Inventory, and Order/Saga services.
- Kubernetes manifests for the API gateway plus standalone Payment, Inventory,
  and Order/Saga deployments, PostgreSQL, Redis, Kafka/Zookeeper, HPA, probes,
  resource requests, private service tokens, and Prometheus scrape targets.

## Validation Targets

- `12.5k RPM` is not currently proven as sustained completed-checkout
  throughput. A multi-SKU 5-second arrival burst accepted and eventually
  drained 1,041 orders with 16 workers and 16 partitions, but its completion
  p95 was 32.2 seconds.
- A separate 5-second submit-only probe accepted 1,042 orders at the 12,500
  RPM arrival target with 0% HTTP failure and 162 ms p95 submit latency. This
  is not completed-checkout or production-SLO evidence.
- The repeatable completion-drain benchmark accepted and completed 167/167
  orders at 1,000 RPM for 10 seconds, with 520.3 ms completion p95. See
  `docs/evidence/completion-drain-load-output-2026-08-19.txt`.
- `p95 <= 120ms` is not currently proven for completed checkouts.
- `99.9% uptime` requires a real deployed environment and historical monitoring.
- `40% failure reduction` requires a before/after failure experiment. The implemented mechanisms prevent duplicate payment records and compensate inventory reservations in tested failure cases, but the percentage must be measured before it is quoted as a result.

## Production Hardening Roadmap

- Validate multi-node failover, retention, replay administration, and alerting
  in a deployed environment; the local Order/Saga process-crash replay is now
  covered.
- Benchmark the configured 16-partition/16-worker Kafka path for completed-checkout throughput.
- Extract Gateway/Auth into an independently deployed process with the same
  contract/restart evidence used for the three bounded contexts.
- Add integration tests against real PostgreSQL, Redis, and Kafka containers.
- Add OpenTelemetry tracing for request, event, and saga step spans.
- Capture k6 and Kubernetes validation outputs under `docs/`.
