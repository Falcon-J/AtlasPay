# Runbook: DLQ growth

1. Inspect recent rows through the authenticated admin endpoint or PostgreSQL:

   ```powershell
   docker compose exec -T postgres psql -U atlaspay -d atlaspay -c "SELECT event_type, count(*) FROM dead_letter_events GROUP BY event_type ORDER BY count(*) DESC;"
   ```

2. Compare `dead_letter_events_total` with the durable rows and inspect
   `publish_attempts`, `last_publish_error`, and `published_at`.
3. Use `scripts/dlq-smoke.ps1` to distinguish handler failures from DLQ
   persistence/publication failures.
4. Fix the owning dependency or event contract before replaying messages.
   Do not delete rows as a shortcut; preserve the evidence and use the stable
   event ID when designing a replay operation.

The repository provides DLQ recording and publication state, but no automatic
replay or retention job. Retention policy is an operational decision still
required for a production deployment.
