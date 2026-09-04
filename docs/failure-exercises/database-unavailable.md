# Failure exercise: database unavailable

## Goal

Demonstrate readiness failure and absence of false success when PostgreSQL is
unavailable.

## Procedure

1. Start the stack and verify `/health/live` is HTTP 200 and
   `/health/ready` is HTTP 200.
2. Stop only the local PostgreSQL container without removing its volume:

   ```powershell
   docker compose stop postgres
   ```

3. Capture the HTTP status for `/health/live` and `/health/ready`, application
   logs, and `docker compose ps`.
4. Start PostgreSQL again, wait for readiness, and rerun the relevant smoke
   workflow.

The exercise validates local process behavior only; it does not model managed
database failover or recovery time objectives.
