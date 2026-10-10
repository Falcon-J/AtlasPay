$ErrorActionPreference = "Stop"
. "$PSScriptRoot/contract-response.ps1"

$output = docker compose exec -T order-service wget -S -O /dev/null --tries=1 http://localhost:8083/health/ready 2>&1
Assert-ContractResponse -Output ($output -join "`n") -ExitCode $LASTEXITCODE -ExpectedStatus 200

$output = docker compose exec -T order-service sh -c "wget -S -O /dev/null --tries=1 --header='X-Internal-Token: wrong' --header='X-User-ID: missing' http://localhost:8083/internal/v1/orders/missing" 2>&1
Assert-ContractResponse -Output ($output -join "`n") -ExitCode $LASTEXITCODE -ExpectedStatus 401

$output = docker compose exec -T order-service sh -c "wget -S -O /dev/null --tries=1 --header='X-User-ID: missing' http://localhost:8083/internal/v1/orders/missing" 2>&1
Assert-ContractResponse -Output ($output -join "`n") -ExitCode $LASTEXITCODE -ExpectedStatus 401

Write-Host "AtlasPay order-service contract smoke test passed."
Write-Host "Readiness: ready"
Write-Host "Missing and invalid internal tokens: HTTP 401"
$global:LASTEXITCODE = 0
