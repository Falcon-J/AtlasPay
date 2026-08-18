param(
    [string]$BaseUrl = "http://localhost:8080"
)

$ErrorActionPreference = "Stop"
$paymentWasStopped = $false

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

function Query-Claim {
    param([string]$OrderId)

    $row = docker compose exec -T postgres psql -U atlaspay -d atlaspay -At -F "|" -c `
        "SELECT event_id, status FROM saga_claims WHERE order_id = '$OrderId'"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to query saga claim"
    }
    $row.Trim()
}

function Query-Count {
    param([string]$Sql)

    $row = docker compose exec -T postgres psql -U atlaspay -d atlaspay -At -c $Sql
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to query database"
    }
    [int]$row.Trim()
}

try {
    Write-Host "Stopping Payment to hold an active saga claim..."
    docker compose stop payment-service | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to stop payment-service"
    }
    $paymentWasStopped = $true

    $stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    $auth = Invoke-AtlasPay -Method POST -Path "/api/auth/register" -Body @{
        email = "crash-replay-$stamp@example.com"
        password = "crashReplayPass123"
        first_name = "Crash"
        last_name = "Replay"
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

    $claim = ""
    for ($attempt = 1; $attempt -le 30; $attempt++) {
        $claim = Query-Claim -OrderId $orderId
        if ($claim -and $claim.Split("|")[1] -eq "running") {
            break
        }
        Start-Sleep -Milliseconds 250
    }
    if (-not $claim -or $claim.Split("|")[1] -ne "running") {
        throw "Order did not reach an active saga claim: $claim"
    }

    Write-Host "Killing Order/Saga after claim acquisition..."
    docker compose kill -s KILL order-service | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to kill order-service"
    }

    Write-Host "Restoring Payment and Order/Saga..."
    docker compose start payment-service | Out-Null
    docker compose up -d --wait payment-service | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "payment-service did not become healthy"
    }
    docker compose up -d --wait order-service | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "order-service did not become healthy"
    }

    $orderStatus = ""
    for ($attempt = 1; $attempt -le 60; $attempt++) {
        $orderState = Invoke-AtlasPay -Method GET -Path "/api/orders/$orderId" -Token $token
        $orderStatus = $orderState.data.order.status
        if ($orderStatus -eq "confirmed") {
            break
        }
        Start-Sleep -Milliseconds 500
    }

    $reservationCount = Query-Count -Sql "SELECT count(*) FROM reservations WHERE order_id = '$orderId'"
    $paymentCount = Query-Count -Sql "SELECT count(*) FROM payments WHERE order_id = '$orderId'"
    $claimAfter = Query-Claim -OrderId $orderId
    if ($orderStatus -ne "confirmed" -or $reservationCount -ne 1 -or $paymentCount -ne 1 -or $claimAfter.Split("|")[1] -ne "completed") {
        throw "Crash replay did not converge: status=$orderStatus reservations=$reservationCount payments=$paymentCount claim=$claimAfter"
    }

    Write-Host "AtlasPay saga crash-replay smoke test passed."
    Write-Host "Order status after replay: $orderStatus"
    Write-Host "Reservations: $reservationCount"
    Write-Host "Payments: $paymentCount"
    Write-Host "Claim after replay: $claimAfter"
}
finally {
    if ($paymentWasStopped) {
        docker compose start payment-service | Out-Null
        docker compose up -d --wait payment-service | Out-Null
    }
    docker compose up -d --wait order-service | Out-Null
}
