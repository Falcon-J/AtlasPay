# Runbook: database latency or unavailability

1. Check readiness and PostgreSQL health:

   ```powershell
   Invoke-RestMethod http://localhost:8080/health/ready
   docker compose exec -T postgres pg_isready -U atlaspay
   ```

2. Inspect `db_connections_active`, request duration metrics, and service logs
   for timeout or pool errors.
3. Preserve the existing failure semantics: readiness should fail, writes
   should return an error, and Kafka consumers should not acknowledge messages
   whose database side effects did not complete.
4. Restore the dependency, then run the relevant contract/smoke workflow and
   inspect the saga/outbox state before declaring recovery.

The local stack uses a shared PostgreSQL instance. Production failover,
replica behavior, and capacity are not validated by this repository.
