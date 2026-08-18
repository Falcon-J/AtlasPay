# Backend Evidence

This folder stores command output used to support AtlasPay backend claims. Files
are evidence only when the named command was actually run; missing evidence is
listed explicitly rather than inferred.

## Current Evidence

Historical evidence captured on 2026-06-14:

- Dockerized Go 1.25 `go test ./...` passed. See `go-test-output.txt`.
- `docker compose config --quiet` passed, proving the Compose file parses.
  See `docker-compose-config.txt`.
- Docker Compose services were running; the API, PostgreSQL, Redis, and Kafka
  containers reported healthy, and `/health` reported database/cache up.
  See `docker-backend-status.txt`.
- `scripts/demo-smoke.ps1` passed against the local stack. It proved a successful
  saga reached `completed` with a confirmed order, an injected payment failure
  reached `compensated` with a failed order, duplicate payment submission reused
  the payment ID, and the metrics endpoint responded. See
  `demo-smoke-output.txt`.
- The smoke run used Kafka transport. Compose sets `KAFKA_ENABLED=true`;
  order creation publishes `order.created` to `atlaspay.orders`, and the API's
  Kafka consumer invokes the saga handler. `kafka-smoke-log.txt` contains the
  matching `event published`, saga outcome, and `event processed` entries for
  the passing smoke order IDs. CI also prints and checks those publish/consume
  log markers after the smoke workflow.
- `scripts/dlq-smoke.ps1` passed. It publishes an `order.created` event for a
  nonexistent order, verifies the persisted `dead_letter_events` row reports
  three attempts and `Order not found`, verifies publication to
  `atlaspay.dlq`, then verifies the consumer processes a follow-up event. See
  `dlq-smoke-output.txt`. The script provisions the existing
  `atlaspay.orders` and `atlaspay.dlq` topics first to avoid relying on Kafka's
  asynchronous auto-topic-creation timing.

Fresh evidence captured on 2026-08-18:
- The rebuilt Compose stack ran a standalone `payment-service` on port 8081;
  the gateway selected `PAYMENT_SERVICE_URL`, and checkout/idempotency passed
  through the private HTTP adapter. The payment service healthcheck passed.
- `scripts/payment-service-contract-smoke.ps1` passed: the service healthcheck
  was healthy and an invalid `X-Internal-Token` was rejected with HTTP 401.
  The full current extraction matrix is summarized in
  `payment-service-extraction-output-2026-08-18.txt`.
- The payment idempotency concurrency check passed against the rebuilt local
  stack: 20 concurrent requests with one key returned one payment ID, and a
  conflicting payload returned HTTP 409. See
  `idempotency-concurrency-output.txt`.
- The rebuilt Compose stack ran a standalone `inventory-service` on port 8082;
  the gateway selected `INVENTORY_SERVICE_URL`, and checkout success,
  compensation, and payment idempotency passed through both private HTTP
  adapters. The inventory service healthcheck passed.
- `scripts/inventory-service-contract-smoke.ps1` passed: the service
  healthcheck was healthy and an invalid `X-Internal-Token` was rejected with
  HTTP 401. See `inventory-service-extraction-output-2026-08-18.txt`.
- The rebuilt Compose stack ran a standalone `order-service` on port 8083;
  it owned Kafka workers and saga execution while the gateway used its private
  HTTP contract. Checkout success and compensation passed through the
  four-process topology.
- `scripts/order-service-contract-smoke.ps1` passed: the service healthcheck
  was healthy and an invalid `X-Internal-Token` was rejected with HTTP 401.
  See `order-service-extraction-output-2026-08-18.txt`.
- The duplicate-event inventory check passed against the rebuilt local stack:
  two replayed `order.created` events left one reservation with the same
  quantity. See `duplicate-event-inventory-output.txt`.
- The checkout-specific k6 probe was deliberately diagnostic rather than a
  release claim: a 6,000 RPM arrival target left 380 orders pending because
  the current single consumer processes the saga serially. See
  `checkout-load-probe-2026-08-18.txt`.
- A second 12,500-RPM probe with 8 workers and 16 partitions also remained
  diagnostic: the polling workload saturated, k6 exited on a 97.6% HTTP
  failure rate, and p95 was 37 seconds. See
  `checkout-load-probe-workers-2026-08-18.txt`.
- The separate submit-only k6 probe accepted 1,042 orders at the 12,500-RPM
  arrival target with 0% HTTP failures and 162 ms p95 submit latency. This is
  submit-throughput evidence only; see `order-submit-load-probe-2026-08-18.txt`.
- The repeatable completion-drain benchmark accepted and completed 167/167
  orders at 1,000 RPM for 10 seconds, with 520.3 ms completion p95. It uses a
  unique multi-SKU cohort and authoritative PostgreSQL completion counts; see
  `completion-drain-load-output-2026-08-19.txt`.
- The transactional outbox recovery workflow passed: an order created while
  Kafka was stopped was published and completed after Kafka restarted. See
  `outbox-recovery-output.txt`.
- The saga restart workflow passed: after restarting the Order/Saga process,
  the saga endpoint returned `completed` with five persisted step logs. See
  `saga-restart-output.txt`.
- The PostgreSQL-backed in-flight saga claim integration test passed: same-event
  retries refresh a lease, a different active replay is rejected, and an
  expired lease is reclaimable. See `saga-claim-output-2026-08-19.txt`.
- The controlled Order/Saga crash-replay workflow passed: the process was
  killed after claim acquisition, Kafka redelivered the event, and the order
  converged to `confirmed` with exactly one reservation and one payment. See
  `saga-crash-replay-output-2026-08-19.txt`.

These August workflows seed their own inventory before creating an order, so
they remain repeatable on a persistent local Compose volume.
The complete rebuilt-image matrix is summarized in
`full-smoke-output-2026-08-18.txt`.

The consumer makes at most three handler attempts. Failed attempts wait 250 ms
and then 500 ms before the final attempt. After exhaustion, the current DLQ path
persists the event in PostgreSQL and publishes an `event.dead_lettered` event to
`atlaspay.dlq`. This is bounded local proof, not a production-grade DLQ claim.
CI is configured to rerun the same workflow.

## Continuous Integration

`.github/workflows/ci.yml` is configured to run Dockerized `go test ./...`,
`docker compose config --quiet`, `git diff --check`, and the Docker Compose
checkout and DLQ smoke workflows. The checkout smoke assertions correlate Kafka
publish and consume logs to both generated order IDs. Failed CI runs upload the
Compose and smoke logs. This describes configured CI checks, not a claim that a
GitHub-hosted run has passed; confirm run status in GitHub Actions.

## Regenerate

With local Go 1.25.x:

```powershell
go version
go mod verify
go test ./...
```

Without local Go:

```powershell
docker run --rm -v "${PWD}:/src" -w /src golang:1.25 go test ./...
```

When Docker Desktop is running:

```powershell
docker version
docker compose config --quiet
docker compose up -d --build --wait postgres redis zookeeper kafka payment-service inventory-service order-service api-gateway
docker compose ps
Invoke-RestMethod http://localhost:8080/health
.\scripts\dlq-smoke.ps1
.\scripts\demo-smoke.ps1
docker compose logs order-service | Select-String "event published|event processed"
scripts/completion-drain-load.ps1 -TargetRPM 1000 -Duration 10s -SkuCount 32
docker compose down -v
```

Replace the evidence files with fresh command output. Do not append results from
different environments into one file.

## Not Yet Proven Here

- Long-duration production-like load, availability, and failure-rate results.
  The local completion-drain results are bounded baseline/stress evidence only.
- A passing GitHub-hosted CI run for the current commit until GitHub Actions
  executes the configured workflow.
- Multi-node Kafka behavior or production DLQ operations such as replay,
  retention, alerting, and poison-message administration.
- Sustained 12,500-RPM completed-checkout capacity or p95 <= 120 ms. The local
  12,500-RPM burst eventually drained, but completion p95 was 32.2 seconds.
- Production-grade consumer deduplication across multiple Order/Saga nodes;
  local single-process crash replay is covered by the claim and replay evidence
  above.
- External production topic provisioning; the proof script provisions its two
  required local topics directly.

The Vercel/static frontend simulation is not backend evidence and does not run
Kafka, PostgreSQL, or Redis.
