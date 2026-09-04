# Migration and index audit

Date: 2026-09-04

## Current ownership

`migrations/001_initial.sql` is the baseline schema and
`migrations/002_dlq_publication_state.sql` is the recorded DLQ schema
evolution. The Compose PostgreSQL container runs both files on a new volume,
and each application image carries the same directory and applies pending
files during startup. The runner records applied filenames in
`schema_migrations`, uses a PostgreSQL advisory lock, and applies each ordered
set in one transaction. The SQL remains idempotent for existing local
volumes. There is no rollback framework; down migrations are intentionally
outside this pass.

Every application process fails startup if its migration batch cannot be loaded
or applied; it does not silently continue with a partial schema.

The gateway and standalone services currently share the database schema. This
is an operational coupling to preserve deliberately: schema changes must be
backward-compatible with all four processes during a rolling deployment.

## Constraint and query coverage

| Access pattern | Authoritative constraint/index | Evidence |
| --- | --- | --- |
| User lookup by email or ID | `users.email` unique, primary key on `id` | `internal/auth/repository.go` |
| Refresh-token validation | unique `refresh_tokens.token`, plus `user_id` and expiry indexes | `internal/auth/repository.go` |
| Order lookup and items | primary key on `orders.id`; `order_items.order_id` | `internal/order/repository.go` |
| User/status order listing | `orders.user_id`, `orders.status`, and descending `created_at` indexes | `internal/order/repository.go` |
| Transactional outbox claiming | `(published_at, claimed_at, created_at)` | `ClaimPendingOutbox` uses the same predicates and ordering |
| Saga snapshot and lease recovery | `saga_logs.saga_id`, unique `(saga_id, step_name, status)`, and `saga_claims.lease_expires_at` | `internal/order/repository.go` |
| Payment idempotency | unique `payments.idempotency_key` | `CreateIfAbsent` uses `ON CONFLICT` |
| Inventory reservation concurrency | unique `(order_id, sku)` and row lock on inventory SKU | `internal/inventory/repository.go` |
| DLQ recent listing and publication state | `dead_letter_events.created_at`; `(published_at, created_at)` | `internal/common/dlq/repository.go` |

## Findings

- Required primary keys, foreign keys, uniqueness constraints, check
  constraints, outbox claim index, saga lease index, and DLQ publication-state
  index are present across the baseline and DLQ migrations.
- The current repository queries are simple point lookups or bounded list
  queries. No new composite index is added without representative row counts
  and `EXPLAIN (ANALYZE, BUFFERS)` evidence; local seed data is not a valid
  production workload.
- Several single-column indexes duplicate an existing unique index (for
  example email, SKU, and idempotency key). They are redundant but removing
  them would be a separate write-amplification/locking change. They remain
  untouched in this pass.
- The script is applied by multiple processes. A future deployment should
  keep additive changes backward-compatible or introduce an explicitly owned
  migration step before splitting schema ownership.

## Reproducible local checks

From the repository root:

```text
docker compose up -d --wait postgres
docker compose exec -T postgres psql -U atlaspay -d atlaspay -c "SELECT version, applied_at FROM schema_migrations ORDER BY version;"
docker compose exec -T postgres psql -U atlaspay -d atlaspay -c "SELECT indexname FROM pg_indexes WHERE tablename IN ('outbox_events','saga_claims','dead_letter_events') ORDER BY indexname;"
docker compose down
```

The startup runner is the migration application command for local and deployed
application processes. Rebuilding the image is required after changing SQL.
The local evidence proves schema application and index presence only; it does
not prove production query plans, capacity, or latency.
