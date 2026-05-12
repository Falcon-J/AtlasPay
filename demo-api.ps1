# AtlasPay API Demo Script (Windows PowerShell)
# Demonstrates the complete order → payment → inventory saga workflow
# Usage: .\demo-api.ps1 -ApiUrl "http://localhost:8080"

param(
    [string]$ApiUrl = "http://localhost:8080"
)

$ErrorActionPreference = "Stop"
$timestamp = [int64](([datetime]::UtcNow - [datetime]'1970-01-01').TotalMilliseconds)
$userEmail = "demo-user-${timestamp}@atlaspay.local"
$userPassword = "DemoPass@123456"

Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  AtlasPay Distributed Payment System - Live API Demo (Windows)" -ForegroundColor Green
Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host ""
Write-Host "📍 API Endpoint: $ApiUrl" -ForegroundColor Yellow
Write-Host "👤 Demo User: $userEmail" -ForegroundColor Yellow
Write-Host ""

function Section {
    param([string]$Title)
    Write-Host ""
    Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Blue
    Write-Host $Title -ForegroundColor Green
    Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Blue
}

function RunApi {
    param(
        [string]$Method,
        [string]$Endpoint,
        [string]$Body,
        [string]$Token,
        [string]$Description
    )
    
    Write-Host ""
    Write-Host "📤 $Description" -ForegroundColor Yellow
    Write-Host "Endpoint: $Method $Endpoint" -ForegroundColor Gray
    Write-Host ""
    
    $headers = @{
        "Content-Type" = "application/json"
    }
    
    if ($Token) {
        $headers["Authorization"] = "Bearer $Token"
    }
    
    $uri = "$ApiUrl$Endpoint"
    $response = if ($Body) {
        Invoke-WebRequest -Uri $uri -Method $Method -Headers $headers -Body $Body -UseBasicParsing
    } else {
        Invoke-WebRequest -Uri $uri -Method $Method -Headers $headers -UseBasicParsing
    }
    
    $json = $response.Content | ConvertFrom-Json
    $json | ConvertTo-Json -Depth 10
    
    return $json
}

# ============================================================================
# STEP 1: Health Check
# ============================================================================
Section "Step 1: Health Check - Verify System is Running"
Write-Host "Checking if API, Database, and Cache are healthy..." -ForegroundColor Cyan

$health = Invoke-WebRequest -Uri "$ApiUrl/health" -UseBasicParsing | ConvertFrom-Json
Write-Host ""
$health | ConvertTo-Json -Depth 10
Write-Host ""

if ($health.db -eq "up" -and $health.cache -eq "up") {
    Write-Host "✅ Database: up" -ForegroundColor Green
    Write-Host "✅ Cache (Redis): up" -ForegroundColor Green
} else {
    Write-Host "❌ System not healthy" -ForegroundColor Red
    exit 1
}

# ============================================================================
# STEP 2: User Registration
# ============================================================================
Section "Step 2: Register a Demo User"
Write-Host "Creating a new user account for this demo session..." -ForegroundColor Cyan

$regBody = @{
    email = $userEmail
    password = $userPassword
    name = "Demo User"
} | ConvertTo-Json

$regResponse = RunApi "POST" "/api/auth/register" $regBody $null "Register new user"

if ($regResponse.status -and $regResponse.status -eq "success") {
    Write-Host ""
    Write-Host "✅ User registered successfully" -ForegroundColor Green
    Write-Host "   Email: $userEmail" -ForegroundColor Gray
} else {
    Write-Host "❌ Registration failed" -ForegroundColor Red
    exit 1
}

# ============================================================================
# STEP 3: User Login & Get JWT Token
# ============================================================================
Section "Step 3: Login & Obtain JWT Authentication Token"
Write-Host "Authenticating with credentials to receive access token..." -ForegroundColor Cyan

$loginBody = @{
    email = $userEmail
    password = $userPassword
} | ConvertTo-Json

$loginResponse = RunApi "POST" "/api/auth/login" $loginBody $null "Login and get JWT token"

$accessToken = $loginResponse.data.access_token
$refreshToken = $loginResponse.data.refresh_token

if (-not $accessToken) {
    Write-Host "❌ Login failed" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "✅ Authentication successful" -ForegroundColor Green
Write-Host "   Access Token: $($accessToken.Substring(0, [Math]::Min(50, $accessToken.Length)))..." -ForegroundColor Gray
Write-Host "   Token Type: JWT (expires in 15 minutes)" -ForegroundColor Gray

# ============================================================================
# STEP 4: Create Order (Triggers Saga Orchestration)
# ============================================================================
Section "Step 4: Place an Order (This Triggers the Saga!)"
Write-Host "Creating a new order with 5 units..." -ForegroundColor Cyan
Write-Host ""
Write-Host "Behind the scenes, this will:" -ForegroundColor Gray
Write-Host "  1. Create order in system" -ForegroundColor Gray
Write-Host "  2. Reserve inventory" -ForegroundColor Gray
Write-Host "  3. Process payment" -ForegroundColor Gray
Write-Host "  4. Update stock" -ForegroundColor Gray
Write-Host "  5. Emit event via Kafka" -ForegroundColor Gray

$orderPayload = @{
    customer_id = "demo-customer-$timestamp"
    items = @(
        @{
            product_id = "PROD-001"
            quantity = 5
            unit_price = 99.99
        }
    )
    payment_method = "credit_card"
    idempotency_key = "order-$timestamp"
} | ConvertTo-Json

$orderResponse = RunApi "POST" "/api/orders" $orderPayload $accessToken "Create order (triggers distributed saga)"

$orderId = $orderResponse.data.id
$orderStatus = $orderResponse.data.status

if (-not $orderId) {
    Write-Host "❌ Order creation failed" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "✅ Order created successfully" -ForegroundColor Green
Write-Host "   Order ID: $orderId" -ForegroundColor Gray
Write-Host "   Status: $orderStatus" -ForegroundColor Gray
Write-Host "   Total: `$$('{0:N2}' -f $orderResponse.data.total_amount)" -ForegroundColor Gray

# ============================================================================
# STEP 5: Poll Order Status (Watch Saga Progress)
# ============================================================================
Section "Step 5: Monitor Saga Orchestration Progress"
Write-Host "Polling order status to watch the saga complete..." -ForegroundColor Cyan
Write-Host "The saga will: Order → Inventory Reserve → Payment Process → Finalize" -ForegroundColor Gray

$pollCount = 0
$maxPolls = 15

while ($pollCount -lt $maxPolls) {
    $pollCount++
    Write-Host ""
    Write-Host "Poll #$pollCount (waiting 2 seconds...)" -ForegroundColor Gray
    Start-Sleep -Seconds 2
    
    $headers = @{
        "Content-Type" = "application/json"
        "Authorization" = "Bearer $accessToken"
    }
    
    $statusResponse = Invoke-WebRequest -Uri "$ApiUrl/api/orders/$orderId" -Method GET -Headers $headers -UseBasicParsing | ConvertFrom-Json
    
    $currentStatus = $statusResponse.data.status
    $sagaState = if ($statusResponse.data.saga_state) { $statusResponse.data.saga_state } else { "N/A" }
    $paymentStatus = if ($statusResponse.data.payment_status) { $statusResponse.data.payment_status } else { "pending" }
    
    Write-Host "  Status: $currentStatus | Saga: $sagaState | Payment: $paymentStatus" -ForegroundColor Cyan
    
    if ($currentStatus -eq "completed" -or $currentStatus -eq "confirmed") {
        Write-Host ""
        Write-Host "✅ Saga completed! Order finalized." -ForegroundColor Green
        Write-Host ""
        Write-Host "Final Order Details:" -ForegroundColor Cyan
        $statusResponse.data | ConvertTo-Json -Depth 10
        break
    }
    
    if ($currentStatus -eq "failed" -or $sagaState -eq "FAILED") {
        Write-Host ""
        Write-Host "X Saga failed!" -ForegroundColor Red
        $statusResponse.data | ConvertTo-Json -Depth 10
        break
    }
}

# ============================================================================
# STEP 6: Verify Payment Processing
# ============================================================================
Section "Step 6: Verify Payment Was Processed"
Write-Host "Checking payment status in the system..." -ForegroundColor Cyan

$headers = @{
    "Content-Type" = "application/json"
    "Authorization" = "Bearer $accessToken"
}

$paymentResponse = Invoke-WebRequest -Uri "$ApiUrl/api/payments?order_id=$orderId" -Method GET -Headers $headers -UseBasicParsing | ConvertFrom-Json
Write-Host ""
$paymentResponse | ConvertTo-Json -Depth 10

if ($paymentResponse.data -and $paymentResponse.data[0]) {
    Write-Host ""
    Write-Host "✅ Payment confirmed" -ForegroundColor Green
    Write-Host "   Payment ID: $($paymentResponse.data[0].id)" -ForegroundColor Gray
    Write-Host "   Amount: `$$('{0:N2}' -f $paymentResponse.data[0].amount)" -ForegroundColor Gray
}

# ============================================================================
# Summary
# ============================================================================
Section "Demo Complete ✅"
Write-Host ""
Write-Host "What You Just Saw:" -ForegroundColor Green
Write-Host "  Check User authentication (JWT tokens)" -ForegroundColor Cyan
Write-Host "  Check Order placement triggering saga" -ForegroundColor Cyan
Write-Host "  Check Saga orchestration (Order to Inventory to Payment)" -ForegroundColor Cyan
Write-Host "  Check Distributed transaction coordination" -ForegroundColor Cyan
Write-Host "  Check Inventory management and cache" -ForegroundColor Cyan
Write-Host "  Check Payment processing and idempotency" -ForegroundColor Cyan
Write-Host ""
Write-Host "Key Architecture Patterns Demonstrated:" -ForegroundColor Green
Write-Host "  * Saga Orchestration - Managing distributed transactions" -ForegroundColor Gray
Write-Host "  * Idempotency - Safe retries with deduplication" -ForegroundColor Gray
Write-Host "  * Cache-Aside - Redis for inventory caching" -ForegroundColor Gray
Write-Host "  * JWT Auth - Token-based API security" -ForegroundColor Gray
Write-Host "  * Event-Driven - Kafka integration for async events" -ForegroundColor Gray
Write-Host ""
Write-Host "System Details:" -ForegroundColor Green
Write-Host "  • Language: Go 1.21" -ForegroundColor Gray
Write-Host "  • Database: PostgreSQL (primary data store)" -ForegroundColor Gray
Write-Host "  • Cache: Redis (inventory/order cache)" -ForegroundColor Gray
Write-Host "  • Queue: Kafka (optional, graceful degradation if down)" -ForegroundColor Gray
Write-Host "  • Deployment: Docker Compose on AWS EC2" -ForegroundColor Gray
Write-Host ""
Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
