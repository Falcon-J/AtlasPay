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
    $args = @{ Method = $Method; Uri = "$BaseUrl$Path"; Headers = $headers }
    if ($null -ne $Body) {
        $args["ContentType"] = "application/json"
        $args["Body"] = ($Body | ConvertTo-Json -Depth 10)
    }
    Invoke-RestMethod @args
}

function Query-Outbox {
    param([string]$OrderId)

    $row = docker compose exec -T postgres psql -U atlaspay -d atlaspay -At -F "|" -c `
        "SELECT attempts, (published_at IS NOT NULL), COALESCE(last_error, '') FROM outbox_events WHERE aggregate_id = '$OrderId' ORDER BY created_at DESC LIMIT 1"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to query outbox"
    }
    $row.Trim()
}

Write-Host "Stopping Kafka to create an unpublished outbox event..."
docker compose stop kafka | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "Failed to stop Kafka"
}

try {
    $stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    $auth = Invoke-AtlasPay -Method POST -Path "/api/auth/register" -Body @{
        email = "outbox-$stamp@example.com"
        password = "outboxPass123"
        first_name = "Outbox"
        last_name = "Recovery"
    }
    $token = $auth.data.access_token
    Invoke-AtlasPay -Method POST -Path "/api/inventory/restock" -Token $token -Body @{
        sku = "LAPTOP-001"
        quantity = 1
    } | Out-Null
    $order = Invoke-AtlasPay -Method POST -Path "/api/orders/" -Token $token -Body @{
        items = @(@{ sku = "LAPTOP-001"; quantity = 1 })
    }
    $orderId = $order.data.order.id

    Start-Sleep -Seconds 2
    $duringOutage = Query-Outbox -OrderId $orderId
    if ($duringOutage.Split("|")[1] -eq "t") {
        throw "Outbox event was published while Kafka was stopped: $duringOutage"
    }

    Write-Host "Restarting Kafka and waiting for outbox delivery..."
    docker compose start kafka | Out-Null
    docker compose up -d --wait kafka | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Kafka did not become healthy after restart"
    }

    $status = ""
    $afterRecovery = ""
    for ($attempt = 1; $attempt -le 40; $attempt++) {
        $orderState = Invoke-AtlasPay -Method GET -Path "/api/orders/$orderId" -Token $token
        $status = $orderState.data.order.status
        $afterRecovery = Query-Outbox -OrderId $orderId
        if ($status -eq "confirmed" -and $afterRecovery.Split("|")[1] -eq "t") {
            break
        }
        Start-Sleep -Milliseconds 500
    }

    if ($status -ne "confirmed" -or $afterRecovery.Split("|")[1] -ne "t") {
        throw "Outbox event did not recover: status=$status row=$afterRecovery"
    }

    Write-Host "AtlasPay outbox recovery smoke test passed."
    Write-Host "Order status after recovery: $status"
    Write-Host "Outbox before recovery: $duringOutage"
    Write-Host "Outbox after recovery: $afterRecovery"
}
finally {
    docker compose start kafka | Out-Null
}
