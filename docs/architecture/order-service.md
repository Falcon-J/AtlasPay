# Order and Saga Service Extraction

AtlasPay now runs Order and Saga as the Kafka-owning application process:

```text
Client -> API Gateway/Auth -> private HTTP contract -> Order/Saga Service
                                                    -> Kafka order workers
                                                    -> Payment Service
                                                    -> Inventory Service
```

## Contract

The gateway owns public JWT authentication and forwards the authenticated user
identity and role over a token-protected private contract. Order/Saga owns
order persistence, the transactional outbox, Kafka consumers, retries and DLQ
handling, saga snapshots/step logs, and order state transitions.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/internal/v1/orders` | Create an order and durable outbox event |
| `GET` | `/internal/v1/orders` | List the authenticated user's orders |
| `GET` | `/internal/v1/orders/{id}` | Read an order |
| `GET` | `/internal/v1/orders/{id}/saga` | Read durable saga state |
| `PATCH` | `/internal/v1/orders/{id}/cancel` | Cancel an owned order |
| `GET` | `/health/live` | Process liveness |
| `GET` | `/health/ready` | Database/cache readiness check |
| `GET` | `/health` | Legacy health compatibility route |

The gateway uses `internal/order.Client`; the Order/Saga process uses the same
typed Payment and Inventory clients for its saga steps. Only the Order/Saga
process consumes `atlaspay.orders` and publishes `atlaspay.dlq` in Compose.

## Ownership rule

This is still a reversible process extraction over the shared PostgreSQL and
Redis instances. The gateway remains the public entry point and does not own
Kafka workers or saga state in the Compose topology. A later production split
would give Order/Saga its own database and replace forwarded identity headers
with a stronger service identity or mTLS.
