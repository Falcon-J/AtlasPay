# AtlasPay API Demo - PowerShell Version
# Simple, robust demo without encoding issues

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

# STEP 2: Register
Write-Host "[2] Register User" -ForegroundColor Yellow
$regBody = @{email=$email; password=$password; name="Demo"} | ConvertTo-Json
$regResp = Invoke-WebRequest -Uri "$ApiUrl/api/auth/register" -Method POST -Headers @{"Content-Type"="application/json"} -Body $regBody -UseBasicParsing | ConvertFrom-Json
Write-Host ("Registered: " + $regResp.data.email) -ForegroundColor Green

# STEP 3: Login
Write-Host "[3] Login & Get Token" -ForegroundColor Yellow
$loginBody = @{email=$email; password=$password} | ConvertTo-Json
$loginResp = Invoke-WebRequest -Uri "$ApiUrl/api/auth/login" -Method POST -Headers @{"Content-Type"="application/json"} -Body $loginBody -UseBasicParsing | ConvertFrom-Json
$token = $loginResp.data.access_token
Write-Host ("Token: " + $token.Substring(0, 30) + "...") -ForegroundColor Green

# STEP 4: Create Order
Write-Host "[4] Create Order (Triggers Saga)" -ForegroundColor Yellow
$orderBody = @{
    customer_id = "cust-$timestamp"
    items = @(@{product_id="PROD-001"; quantity=5; unit_price=99.99})
    payment_method = "credit_card"
    idempotency_key = "order-$timestamp"
} | ConvertTo-Json

$orderResp = Invoke-WebRequest -Uri "$ApiUrl/api/orders" -Method POST `
    -Headers @{"Content-Type"="application/json"; "Authorization"="Bearer $token"} `
    -Body $orderBody -UseBasicParsing | ConvertFrom-Json

$orderId = $orderResp.data.id
$orderStatus = $orderResp.data.status
Write-Host ("Order ID: " + $orderId) -ForegroundColor Green
Write-Host ("Status: " + $orderStatus) -ForegroundColor Cyan

# STEP 5: Monitor Saga
Write-Host "[5] Monitor Saga Progress" -ForegroundColor Yellow
for ($i = 1; $i -le 10; $i++) {
    Start-Sleep -Seconds 2
    
    $statusResp = Invoke-WebRequest -Uri "$ApiUrl/api/orders/$orderId" -Method GET `
        -Headers @{"Content-Type"="application/json"; "Authorization"="Bearer $token"} `
        -UseBasicParsing | ConvertFrom-Json
    
    $status = $statusResp.data.status
    $sagaState = $statusResp.data.saga_state
    Write-Host ("  Poll $i`: Status=$status | Saga=$sagaState") -ForegroundColor Cyan
    
    if ($status -eq "completed" -or $status -eq "confirmed") {
        Write-Host "Order Complete!" -ForegroundColor Green
        break
    }
}

# STEP 6: Verify Payment
Write-Host "[6] Verify Payment" -ForegroundColor Yellow
$payResp = Invoke-WebRequest -Uri "$ApiUrl/api/payments?order_id=$orderId" -Method GET `
    -Headers @{"Content-Type"="application/json"; "Authorization"="Bearer $token"} `
    -UseBasicParsing | ConvertFrom-Json

if ($payResp.data -and $payResp.data[0]) {
    Write-Host ("Payment ID: " + $payResp.data[0].id) -ForegroundColor Green
    Write-Host ("Amount: $" + $payResp.data[0].amount) -ForegroundColor Green
}

# STEP 7: Test Idempotency
Write-Host "[7] Test Idempotency (Submit Same Order)" -ForegroundColor Yellow
$retryResp = Invoke-WebRequest -Uri "$ApiUrl/api/orders" -Method POST `
    -Headers @{"Content-Type"="application/json"; "Authorization"="Bearer $token"} `
    -Body $orderBody -UseBasicParsing | ConvertFrom-Json

if ($retryResp.data.id -eq $orderId) {
    Write-Host "SUCCESS: Same order returned (no duplicate)" -ForegroundColor Green
} else {
    Write-Host "Different order ID returned" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=== Demo Complete ===" -ForegroundColor Green
Write-Host "You demonstrated:" -ForegroundColor Yellow
Write-Host "  - User authentication (JWT)" -ForegroundColor Cyan
Write-Host "  - Order creation triggering saga" -ForegroundColor Cyan
Write-Host "  - Saga orchestration progress" -ForegroundColor Cyan
Write-Host "  - Payment processing" -ForegroundColor Cyan
Write-Host "  - Idempotency (safe retries)" -ForegroundColor Cyan
Write-Host ""
