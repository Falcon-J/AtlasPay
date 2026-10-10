param(
    [string]$BaseUrl = "http://localhost:8080"
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot/smoke-fixtures.ps1"

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
        Method  = $Method
        Uri     = "$BaseUrl$Path"
        Headers = $headers
    }
    if ($null -ne $Body) {
        $args["ContentType"] = "application/json"
        $args["Body"] = ($Body | ConvertTo-Json -Depth 10)
    }

    Invoke-RestMethod @args
}

function Query-Reservations {
    param([string]$OrderId)

    $result = docker compose exec -T postgres psql -U atlaspay -d atlaspay -At -F "|" -c `
        "SELECT count(*), COALESCE(sum(quantity), 0) FROM reservations WHERE order_id = '$OrderId'"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to query reservations"
    }
    $result.Trim()
}

function Publish-Event {
    param([hashtable]$Event)

    ($Event | ConvertTo-Json -Depth 10 -Compress) |
        docker compose exec -T kafka kafka-console-producer `
            --bootstrap-server localhost:9092 `
            --topic atlaspay.orders | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to publish duplicate order event"
    }
}

$stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$auth = Invoke-AtlasPay -Method POST -Path "/api/auth/register" -Body @{
    email = "duplicate-$stamp@example.com"
    password = "duplicatePass123"
    first_name = "Duplicate"
    last_name = "Event"
}
$token = $auth.data.access_token

Add-SmokeInventory -Sku "LAPTOP-001"

$orderResponse = Invoke-AtlasPay -Method POST -Path "/api/orders/" -Token $token -Body @{
    items = @(@{ sku = "LAPTOP-001"; quantity = 1 })
}
$orderId = $orderResponse.data.order.id

$orderStatus = ""
for ($attempt = 1; $attempt -le 30; $attempt++) {
    $order = Invoke-AtlasPay -Method GET -Path "/api/orders/$orderId" -Token $token
    $orderStatus = $order.data.order.status
    if ($orderStatus -eq "confirmed") {
        break
    }
    Start-Sleep -Milliseconds 500
}
if ($orderStatus -ne "confirmed") {
    throw "Order $orderId did not reach confirmed status; actual status: $orderStatus"
}

$before = Query-Reservations -OrderId $orderId
$payload = @{
    order_id = $orderId
    user_id = "duplicate-proof"
    items = @(@{
        sku = "LAPTOP-001"
        name = "Product LAPTOP-001"
        quantity = 1
        unit_price = 99.99
    })
    total_price = 99.99
    currency = "USD"
}

$replayIds = @("duplicate-a-$stamp", "duplicate-b-$stamp")
foreach ($eventId in $replayIds) {
    Publish-Event -Event @{
        id = $eventId
        type = "order.created"
        aggregate_id = $orderId
        correlation_id = "duplicate-$stamp"
        timestamp = [DateTimeOffset]::UtcNow.ToString("o")
        version = 1
        payload = $payload
    }
}

$consumed = $false
for ($attempt = 0; $attempt -lt 60; $attempt++) {
    # The consumer emits this message only after its synchronous offset commit.
    $logs = docker compose logs --no-color --tail=1000 order-service
    if ($LASTEXITCODE -ne 0) { throw "Failed to inspect duplicate consumption" }
    $missing = @($replayIds | Where-Object {
        $id = $_
        -not ($logs | Where-Object { $_ -match 'event processed' -and $_ -match [regex]::Escape($id) })
    })
    if ($missing.Count -eq 0) { $consumed = $true; break }
    Start-Sleep -Milliseconds 500
}
if (-not $consumed) { throw "Duplicate events were not acknowledged: $($missing -join ', ')" }
$after = Query-Reservations -OrderId $orderId
if ($before -ne "1|1" -or $after -ne "1|1") {
    throw "Duplicate event changed reservation state: before=$before after=$after"
}
$paymentCount = docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U atlaspay -d atlaspay -At -c "SELECT count(*) FROM payments WHERE order_id = '$orderId';"
if ($LASTEXITCODE -ne 0 -or [int]$paymentCount -ne 1) {
    throw "Duplicate event did not preserve exactly one payment: $paymentCount"
}
$orderAfterReplay = Invoke-AtlasPay -Method GET -Path "/api/orders/$orderId" -Token $token
if ($orderAfterReplay.data.order.status -ne "confirmed") { throw "Duplicate replay changed the confirmed order" }

Write-Host "AtlasPay duplicate-event smoke test passed."
Write-Host "Order status: $orderStatus"
Write-Host "Reservations before: $before"
Write-Host "Reservations after: $after"
Write-Host "Payments after replay: $paymentCount"
Write-Host "Acknowledged replay events: $($replayIds -join ', ')"
