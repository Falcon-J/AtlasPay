# Local backend demonstration

The hosted UI is a browser simulation. The local UI connects to the real API gateway, Order/Saga, Inventory, and Payment services backed by PostgreSQL, Redis, and Kafka. This is a development setup for learning and interviews, not a production deployment.

## Start

Install Docker Desktop with Linux containers and PowerShell 7. From the repository root, run:

```powershell
./scripts/start-local-demo.ps1
```

Open [the local demo](http://localhost:8085). The first run builds Go inside Docker; no host Go installation is required. Allow time for image downloads and service startup. The launcher starts the core backend and a static nginx UI; optional Kafka UI, metrics, and tracing containers are not required.

The gateway uses port 8080 and the UI binds to the loopback interface on port 8085. Use `localhost` in the browser so its origin matches the existing CORS configuration. Existing Compose infrastructure also exposes development ports; run this on a trusted local machine.

The setup reads `LAPTOP-001` and `FAIL-PAYMENT-001` through controlled local database access. If either is missing or has no available stock, it uses the shared additive fixture to add ten units. Repeating startup while stock is available adds nothing. Existing prices and reservations are preserved. Browser users remain ordinary users and cannot restock or invoke private payment operations.

## Interview walkthrough

1. Place a checkout with payment failure disabled. Explain that the browser creates an order; Order/Saga owns reservation, payment, inventory commit, and final order state. Inspect the actual order ID and saga result rather than treating animation as proof.
2. Enable payment failure and place another checkout. The seeded failure SKU triggers the backend demo failure path. Show that compensation releases the reservation and leaves the order failed. This demonstrates orchestration against a simulated payment processor, not a charge through a real payment provider.
3. Explain duplicate delivery using `scripts/duplicate-event-smoke.ps1`: the script waits for replay acknowledgement, then verifies one payment and reservation for the order.
4. Explain crash recovery using `scripts/saga-crash-replay-smoke.ps1`, and outbox recovery using `scripts/outbox-recovery-smoke.ps1`. These deliberately interrupt local services; run them after browser checkout finishes. Recovery requires persisted saga state and Kafka retention. Read their output and CI results before claiming that a scenario passed.

Useful implementation entry points are `internal/order/service.go`, `internal/common/saga/order_placement.go`, `internal/common/saga/saga.go`, `internal/common/kafka/kafka.go`, and the payment/inventory internal handlers. Public `POST /api/payments` is disabled: checkout is the payment entry point. See [security verification](SECURITY_PASS.md) and [the integration matrix](INTEGRATION_MATRIX.md) for evidence and boundaries.

## Stop and checks

Stop the demo without deleting database volumes:

```powershell
docker compose -f docker-compose.yml -f docker-compose.demo.yml stop
```

Check the launcher's seed decisions without starting Docker:

```powershell
./scripts/start-local-demo.ps1 -SelfTest
```

The UI demonstration covers checkout and compensation. Recovery coverage stays in the scripts. Local success does not establish hosted backend integration or production readiness; remote CI is a separate gate.
## Recovery verification note

The published CI run on commit `9e8fa95` passed build/tests and the Compose checks through saga restart, then failed crash replay before a claim appeared. Its Kafka logs show an unfinished consumer-group rebalance after the restart. HTTP readiness checks dependencies and listener availability; it does not prove partition assignment. The crash fixture now allows a bounded 90-second claim polling window, still requiring a running claim before killing the worker and exactly one payment and reservation after recovery. The delayed-assignment self-check fails under the old budget and passes under the new budget. Restart followed immediately by crash replay passed locally. Remote CI for this change remains unverified until an authorized commit is tested there.
## Verified locally on 10 October 2026

- One-command setup started all core services and nginx; the browser connected as an ordinary user with restock disabled.
- Browser checkout `6ebffe97-327c-4209-a72c-7371e313fe6e` displayed `confirmed / completed` and five persisted steps; PostgreSQL independently showed one payment and one reservation.
- Browser failure checkout `8e3a83a4-fefc-4812-8f45-1de4fc3786aa` displayed `failed / compensated`; PostgreSQL showed no completed payment and zero reserved failure-SKU stock.
- Stopping the local gateway produced explicit offline state with checkout disabled. Simulation required a button click. Restoring the gateway and retrying recovered the same owner session and existing order.
- At a 390px viewport, document width did not exceed viewport width. This is a basic narrow-screen check, not comprehensive accessibility or cross-browser certification.
- Node regressions cover pending state, denied saga reads, actual totals/steps, explicit modes, and unknown create outcomes without replacement requests. Launcher/fixture/status/polling self-checks and local security smoke passed.

A lost order creation response can leave the accepted order ID unknown. Backend checkout remains blocked after reconnect rather than resubmitting. Inspect that user's orders through the API; durable client retry/reconciliation is outside this demo scope. The browser session is in memory; reloading creates a new ordinary demo user. Expired sessions require reloading only when no unresolved order needs inspection.
