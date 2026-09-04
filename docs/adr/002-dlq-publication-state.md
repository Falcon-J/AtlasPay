# ADR 002: Durable DLQ publication state

Date: 2026-09-04
Status: accepted

## Decision

`dead_letter_events` is the durable source of truth for DLQ publication state.
Each event has a stable ID, publication-attempt count, latest publication
error, and successful publication timestamp. Recording is an upsert that does
not erase publication state. A source redelivery skips Kafka publication when
the row is already marked published.

## Why

The source Kafka offset and the DLQ topic are separate side effects. Without
durable state, a retry can duplicate the DLQ message or acknowledge a source
message whose DLQ record was never published.

## Consequences

The current recovery trigger is source redelivery; the index on unpublished
rows supports inspection and a future explicit republisher but no background
republisher is introduced in this pass. Publication is still at-least-once
until a consumer of the DLQ defines its own idempotency contract.
