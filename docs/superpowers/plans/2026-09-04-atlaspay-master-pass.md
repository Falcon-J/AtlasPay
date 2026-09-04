# AtlasPay Master Pass Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the existing AtlasPay checkout into a stronger, evidence-backed backend portfolio project through incremental reliability and operations improvements.

**Architecture:** Preserve the four application processes, Kafka order topic, transactional outbox, saga orchestration, shared Compose database, and existing service contracts. Correct reliability behavior at the shared Kafka/DLQ boundary first, then align health and deployment contracts, strengthen CI and database evidence, and document only behavior that validation supports.

**Tech Stack:** Go 1.25, Kafka, PostgreSQL, Redis, Docker Compose, GitHub Actions, Kubernetes YAML, Prometheus, PowerShell smoke workflows, and k6.

**Spec:** `docs/superpowers/specs/2026-09-04-atlaspay-master-pass-design.md`

## Global Constraints

- Do not rewrite the project from scratch.
- Do not change the existing four-process architecture or outbox/saga design without a concrete correctness finding.
- Assume Kafka messages can be delivered more than once; do not claim exactly-once delivery.
- Commit offsets only after successful handling or durable, successfully published DLQ handling.
- Preserve truthful performance limitations and do not manufacture production claims or benchmark numbers.
- Do not add production dependencies unless the requested behavior cannot be implemented safely with existing dependencies.
- Do not stage, commit, push, deploy, or modify production data without explicit authorization.

### Task 1: Make Kafka failure handling durable and acknowledgement-safe

**Files:**
- Modify: `internal/common/kafka/kafka.go`
- Modify: `internal/common/dlq/repository.go`
- Modify: `scripts/migrations/001_init.sql`
- Test: `internal/common/kafka/kafka_test.go`

**Interfaces:**
- `Consumer` continues to be constructed by `NewConsumerWithOptions` with the existing concrete producer and repository implementations.
- Tests use narrow in-package seams for the Kafka reader, DLQ recorder, and publisher so acknowledgement behavior can be exercised without a live broker.
- `dlq.Repository` exposes durable publication-state operations for a stable event ID: record, mark attempt, mark failure, and mark published.

- [x] **Step 1: Add failing tests for valid duplicate delivery.**
  Feed the same valid event through the consumer twice with a fake reader and idempotent fake handler. Assert the handler observes two deliveries, the event is not silently dropped by the consumer, and both deliveries are acknowledged. This documents at-least-once ownership: downstream business handlers own idempotency.

- [x] **Step 2: Run the focused Kafka tests and confirm the new cases fail for the intended reason.**
  Run `go test ./internal/common/kafka -run 'TestConsumer.*Duplicate|TestConsumer.*Malformed|TestConsumer.*DLQ|TestConsumer.*Retry' -count=1`.
  Expected: the new malformed/DLQ cases fail because malformed messages are currently committed without DLQ handling and DLQ failures are not represented by a testable acknowledgement result.

- [x] **Step 3: Add failing tests for malformed payload handling.**
  Send invalid JSON bytes through the fake reader. Assert the business handler is not called, the DLQ record contains a stable ID and a JSON-safe base64 representation of the raw bytes, the DLQ publication is attempted, and the message is committed only after both operations succeed.

- [x] **Step 4: Add failing tests for retry exhaustion and DLQ failures.**
  Use a handler that fails three times and assert exactly three handler attempts, one DLQ record, one publication, and one acknowledgement. Add cases where recording fails and where publication fails; assert neither case commits the Kafka message.

- [x] **Step 5: Add failing test for context cancellation during retry backoff.**
  Cancel the context after the first failed attempt and assert retry handling returns promptly without starting the next attempt.

- [x] **Step 6: Implement the smallest consumer seams and message-processing path.**
  Inject only the reader/recorder/publisher interfaces needed by tests. Parse malformed payloads into a deterministic DLQ event, route valid handler failures through the same DLQ path, and call `CommitMessages` only after successful handling or completed DLQ handling. Treat commit failure as unacknowledged and stop the consumer loop so a restart can replay from the last committed offset.

- [x] **Step 7: Implement durable DLQ publication state.**
  Extend `dead_letter_events` with publication attempt count, last publication error, and publication timestamp using idempotent `ADD COLUMN IF NOT EXISTS` statements. Make recording stable by supplying a deterministic ID derived from topic plus source event ID, or topic plus raw payload for malformed input. Add repository methods that update the same row before, after, and on failure of publication.

- [x] **Step 8: Make retry backoff context-aware and protect lazy producer initialization.**
  Replace unconditional sleep with a timer selected against `ctx.Done()`. Serialize per-topic writer initialization and closing so concurrent DLQ publication cannot race on the producer map.

- [x] **Step 9: Run the focused tests and inspect the diff.**
  Run `go test ./internal/common/kafka -run 'TestConsumer.*' -count=1`, then `git diff --check` and review that no acknowledgement path commits before its required outcome.

- [x] **Step 10: Close the reliability review findings.**
  Validate event envelopes before acknowledgement, use synchronous offset commits, preserve DLQ publication state across redelivery, retry transient DLQ/commit failures without acknowledging, stop retry waits on cancellation, and serialize producer close against active writes. Cover each behavior with deterministic tests plus a gated real-Postgres repository test.

### Task 2: Standardize service health contracts and probes

**Files:**
- Modify: `cmd/api-gateway/main.go`
- Modify: `cmd/order-service/main.go`
- Modify: `cmd/payment-service/main.go`
- Modify: `cmd/inventory-service/main.go`
- Modify: `deployments/kubernetes/api-gateway.yaml`
- Modify: `deployments/kubernetes/application-services.yaml`
- Modify: `Dockerfile`
- Modify: `Dockerfile.order`
- Modify: `Dockerfile.payment`
- Modify: `Dockerfile.inventory`
- Test: focused HTTP handler tests beside the affected entrypoints where practical

**Outcome:** Every application exposes process-only `/health/live` and dependency-aware `/health/ready`; existing `/health` compatibility remains until documentation and deployment consumers are updated. Kubernetes liveness probes use live, readiness probes use ready, and Docker healthchecks use readiness.

### Task 3: Strengthen CI quality gates

**Files:**
- Modify: `.github/workflows/ci.yml`

**Outcome:** Pull requests and pushes explicitly run formatting validation, `go vet`, tests, builds for all application entrypoints, Compose configuration validation, and the existing smoke workflows. Any unavailable external runtime is reported as a failed gate rather than hidden.

### Task 4: Audit PostgreSQL schema and migration ownership

**Files:**
- Inspect and modify only as evidence requires: `scripts/migrations/001_init.sql`, database repositories, `docs/INTEGRATION_MATRIX.md`, and a focused database audit note.

**Outcome:** Constraints and indexes are verified against actual query predicates; migration application ownership is documented without introducing a migration framework or unsupported performance claims. Any index change includes reproducible query-plan evidence.

### Task 5: Add operational documentation and controlled failure exercises

**Files:**
- Create: `docs/architecture.md`
- Create: `docs/adr/001-kafka-boundary.md`
- Create: `docs/adr/002-dlq-publication-state.md`
- Create: `docs/runbooks/high-error-rate.md`
- Create: `docs/runbooks/kafka-consumer-lag.md`
- Create: `docs/runbooks/database-latency.md`
- Create: `docs/runbooks/dlq-growth.md`
- Create: `docs/failure-exercises/payment-service-unavailable.md`
- Create: `docs/failure-exercises/duplicate-delivery.md`
- Create: `docs/failure-exercises/database-unavailable.md`
- Modify: `README.md`

**Outcome:** Documentation describes the actual topology, ownership, failure boundaries, metrics, run commands, and validation limits. Controlled exercises reference existing scripts and captured evidence rather than inventing incident or production results.

### Task 6: Reproduce load and failure evidence

**Files:**
- Inspect and modify only as evidence requires: `scripts/k6/*.js`, `scripts/*-smoke.ps1`, `scripts/completion-drain-load.ps1`, `docs/PERFORMANCE_RESULTS.md`, and `docs/evidence/README.md`.

**Outcome:** Normal, burst, duplicate, slowdown, and payment-failure scenarios have reproducible commands and raw outputs. Completed-checkout throughput and latency remain separate from HTTP acceptance metrics.

### Task 7: Optional tracing assessment

**Files:**
- Inspect existing logging/metrics and Jaeger Compose configuration first.
- Modify Go modules and tracing code only if a minimal request/event/saga trace can be locally validated end-to-end without significant dependency or operational complexity.

**Outcome:** Add no tracing code if the validation boundary cannot be demonstrated locally; document the deferral honestly instead.

## Validation order

1. Focused tests for the changed behavior.
2. Nearby package tests and integration tests when their required services are available.
3. `gofmt`, `go vet`, and build checks.
4. `docker compose config --quiet` and relevant Compose smoke workflows.
5. Kubernetes static validation if a schema-capable validator is available; otherwise report the exact limitation.
6. `git diff --check` and final manual diff review.
