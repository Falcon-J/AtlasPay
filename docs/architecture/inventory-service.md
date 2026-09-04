# Inventory Service Extraction

AtlasPay now runs the Inventory domain as its own application process in
Compose:

```text
Client -> API Gateway -> private HTTP contract -> Inventory Service -> PostgreSQL/Redis
                       \-> Order/Saga -> Kafka -> Order workers
```

## Contract

The gateway owns public authentication and authorization. The Inventory
service owns stock reads, availability checks, reservations, releases,
commits, restocking, reservation idempotency, and its cache-aside behavior.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/internal/v1/inventory/{sku}` | Read one inventory item |
| `POST` | `/internal/v1/inventory/availability` | Check requested stock |
| `POST` | `/internal/v1/inventory/reserve` | Reserve stock for an order |
| `POST` | `/internal/v1/inventory/release` | Release a reservation |
| `POST` | `/internal/v1/inventory/commit` | Commit a reservation |
| `GET` | `/internal/v1/inventory/reservations/{orderID}` | List order reservations |
| `POST` | `/internal/v1/inventory/restock` | Add stock |
| `GET` | `/health/live` | Process liveness |
| `GET` | `/health/ready` | Database/cache readiness check |
| `GET` | `/health` | Legacy health compatibility route |

The gateway uses `internal/inventory.Client`, while local tests can use the
existing `inventory.Service` adapter. The private contract requires
`X-Internal-Token` when `INVENTORY_SERVICE_TOKEN` is configured. In production,
replace the local Compose token with a secret or mTLS identity and do not
publish port 8082 publicly.

## Ownership rule

This is a process extraction, not yet a database extraction. Both processes use
the current PostgreSQL schema and Redis instance so the change remains
reversible. The next boundary is Order/Saga ownership and its event contract;
the gateway must remain the single public authentication boundary until then.
