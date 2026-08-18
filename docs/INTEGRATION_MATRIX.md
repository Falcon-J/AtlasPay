# AtlasPay Integration Matrix

This matrix names the behavior covered by the Docker Compose smoke workflows.
It is a local validation map, not a claim that GitHub-hosted CI has passed.

| Case | Observable assertion | Workflow |
| --- | --- | --- |
| Health and dependencies | `/health` reports healthy database/cache | `demo-smoke.ps1` |
| Payment service health | Private payment process reports healthy | Compose healthcheck |
| Payment contract auth | Invalid internal token is rejected | `payment-service-contract-smoke.ps1` |
| Inventory service health | Private inventory process reports healthy | Compose healthcheck |
| Inventory contract auth | Invalid internal token is rejected | `inventory-service-contract-smoke.ps1` |
| Order service health | Private order process reports healthy | Compose healthcheck |
| Order contract auth | Invalid internal token is rejected | `order-service-contract-smoke.ps1` |
| User registration | Registration returns a bearer token | `demo-smoke.ps1` |
| Inventory restock | Seeded SKU accepts stock | `demo-smoke.ps1` |
| Successful checkout | Saga completes and order becomes confirmed | `demo-smoke.ps1` |
| Payment failure | Saga compensates and order becomes failed | `demo-smoke.ps1` |
| Payment idempotency | Same key reuses the payment ID | `demo-smoke.ps1` |
| Gateway-to-payment contract | Checkout calls the standalone payment process | `demo-smoke.ps1` + service logs |
| Gateway-to-inventory contract | Checkout calls the standalone inventory process | `demo-smoke.ps1` + service logs |
| Gateway-to-order contract | Gateway proxies checkout to Order/Saga | `demo-smoke.ps1` + service logs |
| Payment concurrency | 20 concurrent requests produce one payment row | evidence output |
| Idempotency conflict | Same key with different data returns `409` | evidence output |
| Kafka publication | Order event is published to `atlaspay.orders` | `demo-smoke.ps1` |
| Kafka consumption | Order event is processed by the worker | `demo-smoke.ps1` |
| Retry backoff | Invalid event is attempted three times | `dlq-smoke.ps1` |
| DLQ persistence | Exhausted event is stored in PostgreSQL | `dlq-smoke.ps1` |
| DLQ publication | Exhausted event is published to `atlaspay.dlq` | `dlq-smoke.ps1` |
| Consumer continuation | Follow-up valid event is still processed | `dlq-smoke.ps1` |
| Duplicate reservation | Replayed events leave one reservation | `duplicate-event-smoke.ps1` |
| Outbox atomicity | Order and outbox row are created together | `outbox-recovery-smoke.ps1` |
| Outbox recovery | Kafka outage is recovered after restart | `outbox-recovery-smoke.ps1` |
| Saga persistence | Five step logs are persisted | `saga-restart-smoke.ps1` |
| Saga restart | Completed state survives API restart | `saga-restart-smoke.ps1` |
| In-flight saga claim | Same-event retry refreshes, active different replay is rejected, expired lease is reclaimable | `TestRepositoryClaimSagaLease` |
| Saga process crash replay | Order/Saga crash after claim redelivers and converges to one payment/reservation | `saga-crash-replay-smoke.ps1` |

The local reliability matrix covers terminal redelivery, in-flight claims, and
a controlled Order/Saga process crash. Production multi-node failover,
retention, replay administration, and alerting remain external gates.
