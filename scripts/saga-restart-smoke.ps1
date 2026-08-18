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

$stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$auth = Invoke-AtlasPay -Method POST -Path "/api/auth/register" -Body @{
    email = "saga-restart-$stamp@example.com"
    password = "restartPass123"
    first_name = "Saga"
    last_name = "Restart"
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

$before = $null
for ($attempt = 1; $attempt -le 30; $attempt++) {
    try {
        $before = Invoke-AtlasPay -Method GET -Path "/api/orders/$orderId/saga" -Token $token
        if ($before.data.status -eq "completed") {
            break
        }
    } catch {
        # The event worker may create saga state after the order is committed.
    }
    Start-Sleep -Milliseconds 500
}
if ($null -eq $before -or $before.data.status -ne "completed") {
    throw "Saga did not complete before restart"
}

Write-Host "Restarting order-service..."
docker compose restart order-service | Out-Null
docker compose up -d --wait order-service | Out-Null
if ($LASTEXITCODE -ne 0) {
	throw "order-service did not become healthy after restart"
}

$after = Invoke-AtlasPay -Method GET -Path "/api/orders/$orderId/saga" -Token $token
$stepCount = @($after.data.step_logs).Count
if ($after.data.status -ne "completed" -or $stepCount -lt 5) {
    throw "Saga state was not reconstructed after restart: status=$($after.data.status), steps=$stepCount"
}

Write-Host "AtlasPay saga restart smoke test passed."
Write-Host "Saga status before restart: $($before.data.status)"
Write-Host "Saga status after restart: $($after.data.status)"
Write-Host "Persisted step log count: $stepCount"
