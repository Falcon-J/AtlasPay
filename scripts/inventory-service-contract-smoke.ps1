$ErrorActionPreference = "Stop"
. "$PSScriptRoot/contract-response.ps1"

$output = docker compose exec -T inventory-service wget -S -O /dev/null --tries=1 http://localhost:8082/health/ready 2>&1
Assert-ContractResponse -Output ($output -join "`n") -ExitCode $LASTEXITCODE -ExpectedStatus 200

$output = docker compose exec -T inventory-service sh -c "wget -S -O /dev/null --tries=1 --header='X-Internal-Token: wrong' http://localhost:8082/internal/v1/inventory/missing" 2>&1
Assert-ContractResponse -Output ($output -join "`n") -ExitCode $LASTEXITCODE -ExpectedStatus 401

$output = docker compose exec -T inventory-service sh -c "wget -S -O /dev/null --tries=1 http://localhost:8082/internal/v1/inventory/missing" 2>&1
Assert-ContractResponse -Output ($output -join "`n") -ExitCode $LASTEXITCODE -ExpectedStatus 401

Write-Host "AtlasPay inventory-service contract smoke test passed."
Write-Host "Readiness: ready"
Write-Host "Missing and invalid internal tokens: HTTP 401"
$global:LASTEXITCODE = 0
