# Payment Service Extraction

AtlasPay runs the Payment domain as a standalone application process in
Compose, alongside standalone Inventory and Order/Saga processes:

```text
Client -> API Gateway -> private HTTP contract -> Payment Service -> PostgreSQL
                       \-> Order/Saga -> Kafka -> Order workers
```

## Contract

The gateway owns public authentication and authorization. The Payment service
owns payment persistence, idempotency, simulated processor behavior, status
transitions, and refunds.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/internal/v1/payments/process` | Process an idempotent payment |
| `GET` | `/internal/v1/payments/{id}` | Read a payment |
| `POST` | `/internal/v1/payments/{id}/refund` | Refund a completed payment |
| `GET` | `/health` | Database-backed readiness check |

The gateway uses `internal/payment.Client`, while local tests can use the
existing `payment.Service` adapter. The private contract requires
`X-Internal-Token` when `PAYMENT_SERVICE_TOKEN` is configured. In production,
replace the local Compose token with a secret or mTLS identity and do not
publish port 8081 publicly.

## Ownership rule

This is a process extraction, not yet a database extraction. The Payment,
Inventory, Order/Saga, and gateway processes use the current shared PostgreSQL
schema so the change stays reversible. The next production boundary is to give
each bounded context owned storage and replace local Compose tokens with a
stronger service identity.
