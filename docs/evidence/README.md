# Backend Evidence

This folder stores command output used to support AtlasPay backend claims. Files
are evidence only when the named command was actually run; missing evidence is
listed explicitly rather than inferred.

## Current Evidence

Captured on 2026-06-14:

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
docker compose up -d --build --wait postgres redis zookeeper kafka api-gateway
docker compose ps
Invoke-RestMethod http://localhost:8080/health
.\scripts\dlq-smoke.ps1
.\scripts\demo-smoke.ps1
docker compose logs api-gateway | Select-String "event published|event processed"
docker compose down -v
```

Replace the evidence files with fresh command output. Do not append results from
different environments into one file.

## Not Yet Proven Here

- Current load, latency, availability, or failure-rate results.
- k6 scripts are available for local benchmarking, but current performance
  results are not claimed until fresh output is captured in this folder.
- Saga recovery after API or Kafka process restarts.
- Multi-node Kafka behavior or production DLQ operations such as replay,
  retention, alerting, and poison-message administration.
- External production topic provisioning; the proof script provisions its two
  required local topics directly.
- A passing GitHub-hosted CI run for the current commit until GitHub Actions
  executes the configured workflow.

The Vercel/static frontend simulation is not backend evidence and does not run
Kafka, PostgreSQL, or Redis.
