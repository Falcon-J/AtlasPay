# AtlasPay Learning Path

AtlasPay is intentionally learned in stages. Each stage adds one distributed-
systems idea and leaves runnable evidence behind.

## Stage 1: Understand the current system

The current deployment is a Go gateway plus extracted Payment, Inventory, and
Order/Saga processes:

```text
API gateway process
├── Auth domain
├── Order domain
├── Payment client (remote process)
├── Inventory client (remote process)
└── Order client (remote process)

Standalone Order/Saga process
├── Kafka order consumers (16 in Compose)
├── Transactional outbox publisher
└── Saga orchestration and durable snapshots
```

The Payment and Inventory clients call standalone services over private,
token-protected HTTP contracts. PostgreSQL and Redis are still shared during
these reversible process-extraction steps.

PostgreSQL is the durable store, Redis is cache-aside state, and Kafka carries
the asynchronous `order.created` trigger and DLQ events.

Run:

```powershell
docker compose up -d --build --wait postgres redis zookeeper kafka payment-service inventory-service order-service api-gateway
docker run --rm -v "${PWD}:/src" -w /src golang:1.25 go test -buildvcs=false ./...
.\scripts\demo-smoke.ps1
.\scripts\dlq-smoke.ps1
.\scripts\duplicate-event-smoke.ps1
.\scripts\payment-service-contract-smoke.ps1
.\scripts\inventory-service-contract-smoke.ps1
.\scripts\order-service-contract-smoke.ps1
```

## Stage 2: Make failure safe

Study these guarantees first:

- Payment idempotency is enforced atomically by PostgreSQL.
- Reusing a payment key with different data returns `409 Conflict`.
- Reservation identity is `(order_id, sku)`, so duplicate order events do not
  reserve inventory twice.
- Kafka handlers retry three times and then persist/publish a DLQ event.
- Order creation and Kafka publication use a transactional outbox, so a Kafka
  outage leaves a durable event to retry.
- Saga snapshots and step logs are persisted, so the saga endpoint survives an
  API restart.
- A PostgreSQL lease claim prevents a different in-flight replay from entering
  the same order saga; the lease is reclaimable after a worker crash.

Evidence lives under `docs/evidence/`. Terminal redeliveries are ignored after
the durable saga snapshot reaches `completed` or `compensated`. The repository
integration test and `saga-crash-replay-smoke.ps1` cover the claim invariant and
a live local Kafka redelivery after an Order/Saga crash. Production multi-node
failover remains a separate operational exercise.

## Stage 3: Measure the real behavior

Do not confuse total HTTP requests with completed checkouts. The repeatable
benchmark is `scripts/completion-drain-load.ps1`: it spreads orders across a
configurable SKU cohort, measures submit throughput with k6, then counts the
same cohort from PostgreSQL and calculates completion latency. The controlled
1,000-RPM run completed 167/167 orders at 520.3 ms p95. A 12,500-RPM burst
eventually drained, but at 32.2 seconds p95, so it is not sustained capacity
or 120 ms SLO evidence. The older polling scenario remains a useful lesson in
how a benchmark can accidentally overload the status endpoint.

## Stage 4: Extract real services only when the boundary is understood

When independent deployment is required, extract in this order:

1. Payment service — implemented with contract and restart proof
2. Inventory service — implemented with contract and restart proof
3. Gateway/Auth service

Each extracted service needs its own process, contract, state ownership,
failure tests, and restart proof. Four packages in one binary are domains, not
four deployed services.
