# Failure exercise: duplicate Kafka delivery

## Goal

Demonstrate at-least-once consumer delivery and downstream idempotency.

## Procedure

1. Start the local stack and run `scripts/duplicate-event-smoke.ps1`.
2. Capture the duplicate event ID, order state, inventory reservation count,
   payment count, and service logs.
3. Confirm the duplicate does not create a second business effect. The
   consumer may log two deliveries; that is expected.

The script and existing database constraints are the evidence boundary. This
does not prove Kafka exactly-once processing.
