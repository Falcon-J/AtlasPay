# ADR 001: Kafka acknowledgement boundary

Date: 2026-09-04
Status: accepted

## Decision

The shared Kafka consumer uses at-least-once processing. It parses and
validates an event, runs the business handler with bounded retries, and only
then commits the source message. If handling exhausts retries, the consumer
must durably record and publish a DLQ event before committing. A persistence,
publication, or offset-commit failure leaves the message unacknowledged and is
retried with context-aware backoff.

## Why

Kafka delivery and process failure can duplicate work. Acknowledging before
the side effect risks silent loss; claiming exactly-once delivery would be an
unsupported guarantee. Downstream database constraints and saga claims own
business idempotency.

## Consequences

Consumers may process a message more than once, and a stuck dependency can
delay the partition. This is observable through Kafka consumption/retry
metrics and logs. DLQ rows remain available for inspection and publication
state survives process restart.
