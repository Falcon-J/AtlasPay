# AtlasPay architecture

AtlasPay is a four-process checkout system. The public API gateway owns the
JWT boundary and routes requests to the Order/Saga, Payment, and Inventory
processes. The private services use internal tokens; they are not public API
entrypoints.

```text
client
  |
  v
api-gateway:8080 --private HTTP--> order-service:8083
       |                           |
       +--> payment-service:8081   +--> Kafka atlaspay.orders
       +--> inventory-service:8082       |
                                        v
                                  Order/Saga consumer

All processes --> PostgreSQL
Order/Inventory --> Redis
```

The gateway may run local adapters when a standalone service URL is absent,
but the deployment manifests select the four-process topology. The
transactional outbox is owned by Order/Saga persistence: the order and its
`order.created` event commit together, and a publisher later sends the event
to Kafka. A saga then coordinates inventory reservation and payment, records
state in PostgreSQL, and compensates on a failed step.

## Reliability boundaries

- Kafka delivery is at least once. The consumer commits an offset only after
  the handler succeeds, or after a failure is durably recorded and the DLQ
  publication succeeds.
- Invalid JSON and invalid event envelopes are represented as deterministic
  DLQ events. The raw malformed body is retained as base64 JSON so arbitrary
  bytes remain safe to store and inspect.
- DLQ publication attempts, the latest publication error, and successful
  publication time are persisted by stable DLQ event ID. A redelivery does
  not publish an already-published DLQ event again.
- Payment idempotency is enforced by the database unique constraint on
  `payments.idempotency_key`. Inventory reservation uniqueness is enforced by
  `(order_id, sku)` plus the inventory row lock.

## Health and observability

Every application exposes `/health/live` for process-only liveness and
`/health/ready` for dependency-aware readiness. `/health` and the gateway's
`/ready` remain compatibility routes. Each process exposes `/metrics` with
HTTP, database, cache, Kafka, saga, and circuit-breaker metrics already
defined in `internal/common/metrics`.

Kubernetes liveness/startup probes use the live endpoint; readiness probes and
Docker healthchecks use the ready endpoint. This keeps a database or cache
outage from causing Kubernetes to restart an otherwise healthy process.

## Deployment limits

The manifests and Compose stack are deployable examples, not production
capacity evidence. They still require external secret management, managed
PostgreSQL/Redis/Kafka decisions, network policy, topic provisioning,
backward-compatible schema rollout, and a tested rollback/failover procedure.
The measured performance limits and completed-checkout evidence remain in
`docs/PERFORMANCE_RESULTS.md`.
