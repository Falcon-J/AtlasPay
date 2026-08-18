$ErrorActionPreference = "Stop"

docker compose exec -T inventory-service wget --no-verbose --tries=1 --spider http://localhost:8082/health | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "Inventory service healthcheck failed"
}

docker compose exec -T inventory-service sh -c "wget -q --spider --header='X-Internal-Token: wrong' http://localhost:8082/internal/v1/inventory/missing"
if ($LASTEXITCODE -eq 0) {
    throw "Inventory service accepted an invalid internal token"
}

Write-Host "AtlasPay inventory-service contract smoke test passed."
Write-Host "Health: healthy"
Write-Host "Invalid internal token: rejected"
