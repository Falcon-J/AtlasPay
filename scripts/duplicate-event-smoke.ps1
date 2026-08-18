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

Invoke-AtlasPay -Method POST -Path "/api/inventory/restock" -Token $token -Body @{
    sku = "LAPTOP-001"
    quantity = 1
} | Out-Null

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

foreach ($eventId in @("duplicate-a-$stamp", "duplicate-b-$stamp")) {
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

Start-Sleep -Seconds 4
$after = Query-Reservations -OrderId $orderId
if ($before -ne "1|1" -or $after -ne "1|1") {
    throw "Duplicate event changed reservation state: before=$before after=$after"
}

Write-Host "AtlasPay duplicate-event smoke test passed."
Write-Host "Order status: $orderStatus"
Write-Host "Reservations before: $before"
Write-Host "Reservations after: $after"
