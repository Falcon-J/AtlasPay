$ErrorActionPreference = "Stop"

docker compose exec -T payment-service wget --no-verbose --tries=1 --spider http://localhost:8081/health/ready | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "Payment service readiness check failed"
}

docker compose exec -T payment-service sh -c "wget -q --spider --header='X-Internal-Token: wrong' http://localhost:8081/internal/v1/payments/missing"
if ($LASTEXITCODE -eq 0) {
    throw "Payment service accepted an invalid internal token"
}

Write-Host "AtlasPay payment-service contract smoke test passed."
Write-Host "Readiness: ready"
Write-Host "Invalid internal token: rejected"
