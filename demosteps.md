# AtlasPay Demo Test Steps

Goal: prove the core system behavior in a short technical walkthrough.

- Order flow works.
- Payment idempotency works.
- Failure compensation is explainable.
- Health and metrics endpoints work.

Do not over-demo. The story is:

> AtlasPay handles an order and payment flow where failures do not leave inventory or payments in an inconsistent state.

## Before Starting

Open PowerShell in the project:

```powershell
cd AtlasPay
```

## Step 1: Start Required Infra

For the core demo, you only need Postgres and Redis:

```powershell
docker compose up -d postgres redis
```

Check health:

```powershell
docker compose ps postgres redis
```

Expected:

```text
atlaspay-postgres   Up ... healthy
atlaspay-redis      Up ... healthy
```

If Postgres gives password/auth errors later, reset the local demo DB:

```powershell
docker compose down -v
docker compose up -d postgres redis
```

This deletes local AtlasPay demo data and recreates the DB from the migration seed.

## Step 2: Build API

Build the local API binary:

```powershell
$env:GOCACHE="$PWD\.cache\go-build"
go build -buildvcs=false -o bin\api-gateway.exe .\cmd\api-gateway
```

## Step 3: Run API

Run the API with explicit local Docker hosts. Keep `KAFKA_ENABLED=false` for the fallback local demo, or set it to `true` when Kafka is healthy and you want to prove the Kafka-backed order worker.

```powershell
$env:DB_HOST="127.0.0.1"
$env:DB_PORT="5432"
$env:DB_USER="atlaspay"
$env:DB_PASSWORD="atlaspay_secret"
$env:DB_NAME="atlaspay"
$env:REDIS_HOST="127.0.0.1"
$env:REDIS_PORT="6379"
$env:KAFKA_ENABLED="false"
.\bin\api-gateway.exe
```

Leave this terminal open.

Expected logs:

```text
starting API Gateway
auto-migration completed successfully
API Gateway listening
```

Kafka-enabled mode:

```powershell
$env:KAFKA_ENABLED="true"
$env:KAFKA_BROKERS="127.0.0.1:9092"
$env:KAFKA_GROUP_ID="atlaspay"
.\bin\api-gateway.exe
```

Expected additional log:

```text
Kafka order worker started
```

## Step 4: Open A Second Terminal

Keep the API running. In a second PowerShell terminal:

```powershell
cd AtlasPay
```

Check health:

```powershell
Invoke-RestMethod http://localhost:8080/health
```

Expected:

```text
status : healthy
db     : up
cache  : up
```

Check metrics:

```powershell
Invoke-WebRequest http://localhost:8080/metrics
```

Expected: Prometheus metrics text. You should see names like `http_requests_total`.

Talking point:

> The API exposes health, readiness, and Prometheus metrics so the service can be monitored during demos or deployments.

## Step 5: Register User

```powershell
$stamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$email = "demo-$stamp@example.com"

$auth = Invoke-RestMethod `
  -Method POST `
  -Uri "http://localhost:8080/api/auth/register" `
  -ContentType "application/json" `
  -Body (@{
    email = $email
    password = "demoPass123"
    first_name = "Demo"
    last_name = "User"
  } | ConvertTo-Json)

$token = $auth.data.access_token
$token
```

Expected: token prints.

Talking point:

> The protected APIs use JWT auth. After registration or login, the client sends the access token as a bearer token.

## Step 6: Check Inventory Before Order

```powershell
Invoke-RestMethod `
  -Method GET `
  -Uri "http://localhost:8080/api/inventory/LAPTOP-001" `
  -Headers @{ Authorization = "Bearer $token" }
```

Talking point:

> Inventory is stored in Postgres, and inventory reads can be cached through Redis using a cache-aside pattern.

## Step 7: Create Successful Order

```powershell
$order = Invoke-RestMethod `
  -Method POST `
  -Uri "http://localhost:8080/api/orders/" `
  -Headers @{ Authorization = "Bearer $token" } `
  -ContentType "application/json" `
  -Body (@{
    items = @(
      @{ sku = "LAPTOP-001"; quantity = 1 },
      @{ sku = "HEADPHONES-001"; quantity = 1 }
    )
  } | ConvertTo-Json -Depth 5)

$orderId = $order.data.order.id
$orderId
```

Wait for the background saga:

```powershell
Start-Sleep -Seconds 1
```

Check order:

```powershell
Invoke-RestMethod `
  -Method GET `
  -Uri "http://localhost:8080/api/orders/$orderId" `
  -Headers @{ Authorization = "Bearer $token" }
```

Check saga:

```powershell
Invoke-RestMethod `
  -Method GET `
  -Uri "http://localhost:8080/api/orders/$orderId/saga" `
  -Headers @{ Authorization = "Bearer $token" }
```

Expected:

```text
saga status: completed
order status: confirmed
```

Talking point:

> This proves the happy path: order created, the workflow is triggered asynchronously, inventory is reserved, payment is processed, inventory is committed, and the order is confirmed.

## Step 8: Test Payment Idempotency

Use the successful order ID:

```powershell
$idem = "DEMO-IDEMP-$stamp"

$paymentBody = @{
  order_id = $orderId
  amount = 12.34
  currency = "USD"
  payment_method = "demo_card"
  idempotency_key = $idem
} | ConvertTo-Json

$p1 = Invoke-RestMethod `
  -Method POST `
  -Uri "http://localhost:8080/api/payments/" `
  -Headers @{ Authorization = "Bearer $token" } `
  -ContentType "application/json" `
  -Body $paymentBody

$p2 = Invoke-RestMethod `
  -Method POST `
  -Uri "http://localhost:8080/api/payments/" `
  -Headers @{ Authorization = "Bearer $token" } `
  -ContentType "application/json" `
  -Body $paymentBody

$p1.data.payment.id
$p2.data.payment.id
```

Expected:

```text
same payment id both times
```

Talking point:

> This proves duplicate payment requests with the same idempotency key do not create duplicate payment records.

## Step 9: Test Failure Compensation

This requires the seed item `FAIL-PAYMENT-001`.

```powershell
$failOrder = Invoke-RestMethod `
  -Method POST `
  -Uri "http://localhost:8080/api/orders/" `
  -Headers @{ Authorization = "Bearer $token" } `
  -ContentType "application/json" `
  -Body (@{
    items = @(
      @{ sku = "FAIL-PAYMENT-001"; quantity = 1 }
    )
  } | ConvertTo-Json -Depth 5)

$failOrderId = $failOrder.data.order.id
$failOrderId
```

Wait:

```powershell
Start-Sleep -Seconds 1
```

Check saga:

```powershell
Invoke-RestMethod `
  -Method GET `
  -Uri "http://localhost:8080/api/orders/$failOrderId/saga" `
  -Headers @{ Authorization = "Bearer $token" }
```

Check order:

```powershell
Invoke-RestMethod `
  -Method GET `
  -Uri "http://localhost:8080/api/orders/$failOrderId" `
  -Headers @{ Authorization = "Bearer $token" }
```

Expected:

```text
saga status: compensated
order status: failed
```

Talking point:

> This proves the compensation path. Inventory reservation succeeded, payment intentionally failed, and the saga released inventory and marked the order failed.

## Step 10: Optional Smoke Test Script

Run:

```powershell
.\scripts\demo-smoke.ps1
```

If PowerShell blocks scripts:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\demo-smoke.ps1
```

The script checks:

- Health endpoint.
- Auth.
- Successful order saga.
- Failing order saga.
- Idempotent payment.
- Metrics endpoint.
- Kafka/DLQ metrics when Kafka mode is enabled.

## DLQ Check

If Kafka mode is enabled, failed event handling is retried 3 times and then written to `dead_letter_events`.

Admin DLQ endpoint:

```powershell
Invoke-RestMethod `
  -Method GET `
  -Uri "http://localhost:8080/api/admin/dlq?limit=10" `
  -Headers @{ Authorization = "Bearer <admin-token>" }
```

Talking point:

> Kafka events use bounded retries. If an event cannot be processed after retry exhaustion, it is persisted in the DLQ for debugging or replay.

## What Not To Demo

Do not rely on Kafka in the live demo unless it is healthy.

If asked, say:

> Kafka infrastructure and event abstractions are present, but the current live demo focuses on the in-process saga flow. Kafka is the next production hardening step for durable event-driven orchestration.

## Demo Walkthrough Order

Use this exact order:

1. `/health`
2. Register/login
3. Check inventory
4. Create successful order
5. Show saga completed
6. Show order confirmed
7. Retry payment with same idempotency key
8. Show same payment ID
9. Create failure order
10. Show saga compensated and order failed

That is enough. Keep talking while showing it.
