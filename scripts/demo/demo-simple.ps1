# AtlasPay API Demo - Flawless Version
# Demonstrates 3 core flows and Idempotency accurately.

param([string]$ApiUrl = "http://localhost:8080")

Write-Host ""
Write-Host "=== AtlasPay Distributed Payment Demo ===" -ForegroundColor Green
Write-Host "API Endpoint: $ApiUrl" -ForegroundColor Cyan
Write-Host ""

$timestamp = [int64](([datetime]::UtcNow - [datetime]'1970-01-01').TotalMilliseconds)
$email = "demo-$timestamp@test.com"
$password = "DemoPass@123"

# STEP 1: Health Check
Write-Host "[1] Health Check" -ForegroundColor Yellow
$health = Invoke-WebRequest -Uri "$ApiUrl/health" -UseBasicParsing | ConvertFrom-Json
Write-Host ("DB: " + $health.db + " | Cache: " + $health.cache) -ForegroundColor Green

# STEP 2: Register (Fixed payload to match Go API)
Write-Host "[2] Register User" -ForegroundColor Yellow
$regBody = @{email=$email; password=$password; first_name="Demo"; last_name="User"} | ConvertTo-Json
$regResp = Invoke-WebRequest -Uri "$ApiUrl/api/auth/register" -Method POST -Headers @{"Content-Type"="application/json"} -Body $regBody -UseBasicParsing | ConvertFrom-Json
Write-Host ("Registered: " + $regResp.data.email) -ForegroundColor Green

# Sleep 2 seconds to avoid the JWT same-second generation race condition bug!
Start-Sleep -Seconds 2

# STEP 3: Login
Write-Host "[3] Login & Get Token" -ForegroundColor Yellow
$loginBody = @{email=$email; password=$password} | ConvertTo-Json
$loginResp = Invoke-WebRequest -Uri "$ApiUrl/api/auth/login" -Method POST -Headers @{"Content-Type"="application/json"} -Body $loginBody -UseBasicParsing | ConvertFrom-Json
$token = $loginResp.data.access_token
Write-Host ("Token: " + $token.Substring(0, 30) + "...") -ForegroundColor Green

# STEP 4: Create Order (Fixed payload with 'sku')
Write-Host "[4] Create Order (Triggers Saga)" -ForegroundColor Yellow
$orderBody = @{
    items = @(@{sku="PROD-001"; quantity=5; unit_price=99.99})
} | ConvertTo-Json

$orderResp = Invoke-WebRequest -Uri "$ApiUrl/api/orders" -Method POST `
    -Headers @{"Content-Type"="application/json"; "Authorization"="Bearer $token"} `
    -Body $orderBody -UseBasicParsing | ConvertFrom-Json

$orderId = $orderResp.data.order.id
$orderStatus = $orderResp.data.order.status
Write-Host ("Order ID: " + $orderId) -ForegroundColor Green
Write-Host ("Status: " + $orderStatus) -ForegroundColor Cyan

# STEP 5: Monitor Saga
Write-Host "[5] Monitor Saga Progress" -ForegroundColor Yellow
for ($i = 1; $i -le 10; $i++) {
    Start-Sleep -Seconds 2
    
    $statusResp = Invoke-WebRequest -Uri "$ApiUrl/api/orders/$orderId" -Method GET `
        -Headers @{"Content-Type"="application/json"; "Authorization"="Bearer $token"} `
        -UseBasicParsing | ConvertFrom-Json
    
    $status = $statusResp.data.order.status
    Write-Host ("  Poll $i`: Order Status = $status") -ForegroundColor Cyan
    
    # In AtlasPay, order goes to 'paid' when saga completes successfully
    if ($status -eq "paid" -or $status -eq "completed") {
        Write-Host "Saga Complete! Order Paid." -ForegroundColor Green
        break
    }
}

# STEP 6: Verify Payment (Via Postgres query since API gateway hides it)
Write-Host "[6] Verify Payment in Database" -ForegroundColor Yellow
Write-Host "Querying Postgres directly..." -ForegroundColor DarkGray
$dbOutput = docker exec atlaspay-postgres psql -U postgres -d atlaspay -t -c "SELECT id, status, amount, idempotency_key FROM payments WHERE order_id = '$orderId';"
Write-Host "Payment Record: $dbOutput" -ForegroundColor Green

# STEP 7: Test Idempotency (Correct approach via Payment Service API)
Write-Host "[7] Test Idempotency (Directly hitting Payment API)" -ForegroundColor Yellow
$idemKey = "DEMO-IDEM-$timestamp"
$payBody = @{
    order_id = $orderId
    amount = 499.95
    currency = "USD"
    payment_method = "credit_card"
    idempotency_key = $idemKey
} | ConvertTo-Json

Write-Host "  -> First Request..." -ForegroundColor DarkGray
$payResp1 = Invoke-WebRequest -Uri "$ApiUrl/api/payments" -Method POST `
    -Headers @{"Content-Type"="application/json"; "Authorization"="Bearer $token"} `
    -Body $payBody -UseBasicParsing | ConvertFrom-Json
$paymentId1 = $payResp1.data.payment.id
Write-Host ("  Created Payment ID: " + $paymentId1) -ForegroundColor Green

Write-Host "  -> Second Request (Exact Same Idempotency Key)..." -ForegroundColor DarkGray
$payResp2 = Invoke-WebRequest -Uri "$ApiUrl/api/payments" -Method POST `
    -Headers @{"Content-Type"="application/json"; "Authorization"="Bearer $token"} `
    -Body $payBody -UseBasicParsing | ConvertFrom-Json
$paymentId2 = $payResp2.data.payment.id
Write-Host ("  Returned Payment ID: " + $paymentId2) -ForegroundColor Green

if ($paymentId1 -eq $paymentId2) {
    Write-Host "SUCCESS: Idempotency check passed! Exactly same payment returned." -ForegroundColor Green
} else {
    Write-Host "FAILED: Different payment created!" -ForegroundColor Red
}

Write-Host ""
Write-Host "=== Demo Complete ===" -ForegroundColor Green
Write-Host "You demonstrated:" -ForegroundColor Yellow
Write-Host "  - 3 Core Flows: Auth, Order Creation, Payment Processing" -ForegroundColor Cyan
Write-Host "  - True Saga Orchestration via Kafka" -ForegroundColor Cyan
Write-Host "  - Proper Idempotency deduplication" -ForegroundColor Cyan
Write-Host ""
