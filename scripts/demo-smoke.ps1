param(
    [string]$BaseUrl = "http://localhost:8080"
)

$ErrorActionPreference = "Stop"

function Invoke-AtlasPay {
    param(
        [string]$Method,
        [string]$Path,
        [object]$Body = $null,
        [string]$Token = ""
    )

    $headers = @{}
    if ($Token) {
        $headers["Authorization"] = "Bearer $Token"
    }

    $args = @{
        Method = $Method
        Uri = "$BaseUrl$Path"
        Headers = $headers
    }

    if ($null -ne $Body) {
        $args["ContentType"] = "application/json"
        $args["Body"] = ($Body | ConvertTo-Json -Depth 10)
    }

    Invoke-RestMethod @args
}

function Wait-Saga {
    param(
        [string]$OrderId,
        [string]$Token,
        [string[]]$ExpectedStatuses
    )

    for ($i = 0; $i -lt 20; $i++) {
        try {
            $sagaResponse = Invoke-AtlasPay -Method GET -Path "/api/orders/$OrderId/saga" -Token $Token
            $status = $sagaResponse.data.status
            if ($ExpectedStatuses -contains $status) {
                return $sagaResponse.data
            }
        } catch {
            # In Kafka mode the order is committed before the worker creates
            # in-memory saga state, so a short initial 404 is expected.
        }
        Start-Sleep -Milliseconds 500
    }

    throw "Saga for order $OrderId did not reach expected status: $($ExpectedStatuses -join ', ')"
}

Write-Host "Checking health..."
$health = Invoke-RestMethod "$BaseUrl/health"
if ($health.status -ne "healthy") {
    throw "Health check did not return healthy"
}

$stamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$email = "demo-$stamp@example.com"
$password = "demoPass123"

Write-Host "Registering demo user $email..."
$auth = Invoke-AtlasPay -Method POST -Path "/api/auth/register" -Body @{
    email = $email
    password = $password
    first_name = "Demo"
    last_name = "User"
}
$token = $auth.data.access_token
if (-not $token) {
    throw "Registration did not return an access token"
}

Write-Host "Creating successful saga order..."
$successOrderResponse = Invoke-AtlasPay -Method POST -Path "/api/orders/" -Token $token -Body @{
    items = @(
        @{ sku = "LAPTOP-001"; quantity = 1 },
        @{ sku = "HEADPHONES-001"; quantity = 1 }
    )
}
$successOrderId = $successOrderResponse.data.order.id
$successSaga = Wait-Saga -OrderId $successOrderId -Token $token -ExpectedStatuses @("completed")
$successOrder = Invoke-AtlasPay -Method GET -Path "/api/orders/$successOrderId" -Token $token
if ($successOrder.data.order.status -ne "confirmed") {
    throw "Successful order did not become confirmed"
}

Write-Host "Creating failing saga order to prove compensation..."
$failedOrderResponse = Invoke-AtlasPay -Method POST -Path "/api/orders/" -Token $token -Body @{
    items = @(
        @{ sku = "FAIL-PAYMENT-001"; quantity = 1 }
    )
}
$failedOrderId = $failedOrderResponse.data.order.id
$failedSaga = Wait-Saga -OrderId $failedOrderId -Token $token -ExpectedStatuses @("compensated")
$failedOrder = Invoke-AtlasPay -Method GET -Path "/api/orders/$failedOrderId" -Token $token
if ($failedOrder.data.order.status -ne "failed") {
    throw "Failed order did not become failed"
}

Write-Host "Checking idempotent payment behavior..."
$idemKey = "DEMO-IDEMP-$stamp"
$paymentBody = @{
    order_id = $successOrderId
    amount = 12.34
    currency = "USD"
    payment_method = "demo_card"
    idempotency_key = $idemKey
}
$paymentOne = Invoke-AtlasPay -Method POST -Path "/api/payments/" -Token $token -Body $paymentBody
$paymentTwo = Invoke-AtlasPay -Method POST -Path "/api/payments/" -Token $token -Body $paymentBody
if ($paymentOne.data.payment.id -ne $paymentTwo.data.payment.id) {
    throw "Idempotent payment retry created a different payment"
}

Write-Host "Checking metrics endpoint..."
$metrics = Invoke-WebRequest "$BaseUrl/metrics" -UseBasicParsing
if ($metrics.Content -notmatch "http_requests_total") {
    throw "Metrics endpoint does not include http_requests_total"
}

Write-Host ""
Write-Host "AtlasPay demo smoke test passed."
Write-Host "Successful order: $successOrderId, saga: $($successSaga.status), order: $($successOrder.data.order.status)"
Write-Host "Failed order: $failedOrderId, saga: $($failedSaga.status), order: $($failedOrder.data.order.status)"
Write-Host "Idempotent payment reused payment id: $($paymentOne.data.payment.id)"
