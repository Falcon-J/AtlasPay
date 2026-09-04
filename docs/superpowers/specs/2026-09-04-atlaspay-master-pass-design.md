# AtlasPay Master Pass Design

**Goal:** Improve AtlasPay's portfolio evidence through small, verified reliability, operational, and documentation slices while preserving the existing four-process checkout architecture.

**Approved scope:** Slice 1 begins with malformed Kafka payload handling, DLQ durability and publication retry state, acknowledgement semantics, and deterministic tests. Later slices cover health contracts, probe alignment, CI gates, database evidence, operational documentation, and reproducible load/failure evidence. OpenTelemetry is optional and lower priority.

## Design decisions

1. Kafka consumers assume at-least-once delivery. They do not claim exactly-once processing or introduce a second event-deduplication framework.
2. A Kafka offset is committed only after business handling succeeds, or after the failed message has been durably recorded and successfully published to the configured DLQ. Malformed payloads follow the same rule.
3. PostgreSQL remains the durable source for DLQ state. Each source delivery gets a stable DLQ identity so a publication retry updates one record instead of creating duplicate rows.
4. DLQ publication state records attempt count, last publication error, and publication time. A publication or state-update failure leaves the source message uncommitted for retry.
5. The existing outbox, saga, four-process topology, Compose workflows, and truthful performance limitations remain unchanged unless a concrete correctness defect requires a local change.

## Non-goals

- Rewriting the services or replacing Kafka, PostgreSQL, Redis, Docker Compose, or the current saga/outbox design.
- Claiming production-scale throughput, exactly-once delivery, live Kubernetes validation, or financial-provider correctness.
- Adding a new message broker, service mesh, migration framework, or observability platform.

## Acceptance evidence

- Deterministic tests cover successful duplicate delivery, malformed payloads, retry exhaustion, persistence failure, publication failure, and no-ack behavior.
- The migration remains idempotent for an existing Compose database.
- Focused Go tests, `go vet`, formatting checks, and the existing Compose validation are run when the environment permits.
- The final documentation distinguishes local evidence from external deployment or production gates.
