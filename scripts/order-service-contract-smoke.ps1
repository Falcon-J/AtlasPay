$ErrorActionPreference = "Stop"

docker compose exec -T order-service wget --no-verbose --tries=1 --spider http://localhost:8083/health | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "Order service healthcheck failed"
}

docker compose exec -T order-service sh -c "wget -q --spider --header='X-Internal-Token: wrong' --header='X-User-ID: missing' http://localhost:8083/internal/v1/orders/missing"
if ($LASTEXITCODE -eq 0) {
    throw "Order service accepted an invalid internal token"
}

Write-Host "AtlasPay order-service contract smoke test passed."
Write-Host "Health: healthy"
Write-Host "Invalid internal token: rejected"
