# Runbook: high error rate

Use this when API failures or saga failures rise.

1. Check service state and readiness:

   ```powershell
   docker compose ps
   Invoke-RestMethod http://localhost:8080/health/ready
   ```

2. Inspect application logs and metrics. Relevant signals include
   `http_requests_total`, `sagas_total`, `payment_processing_duration_seconds`,
   `circuit_breaker_state`, and `db_connections_active`.
3. If the issue is payment or inventory availability, keep traffic flowing
   only within the existing retry and compensation behavior; do not retry
   payment mutations outside their idempotency contract.
4. Reproduce the known workflows with `scripts/demo-smoke.ps1` and the
   service contract smoke scripts after the dependency is restored.

Do not call an HTTP acceptance result a completed-checkout result. Use the
completion-drain workflow and the limits in `docs/PERFORMANCE_RESULTS.md` for
that claim.
