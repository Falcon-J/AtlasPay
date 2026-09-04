# Runbook: Kafka consumer lag

1. Inspect the process and readiness endpoints:

   ```powershell
   docker compose ps order-service kafka
   docker compose exec -T order-service wget -qO- http://localhost:8083/health/ready
   ```

2. Inspect `kafka_consumer_lag`,
   `kafka_event_processing_attempts_total`, and
   `kafka_event_retries_total` on `/metrics`.
3. Review Order/Saga logs for handler failures, DLQ publication failures, or
   repeated event IDs. A message must remain unacknowledged when durable DLQ
   handling is unavailable.
4. Run `scripts/dlq-smoke.ps1` after recovery. It verifies retry exhaustion,
   DLQ persistence/publication, and continued consumption.

The current consumer retries the source message when DLQ persistence or
publication fails. There is no separate background DLQ republisher.
