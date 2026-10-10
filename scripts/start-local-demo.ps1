param([switch]$SelfTest)

$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/smoke-fixtures.ps1"

function Initialize-DemoInventory {
    param([Parameter(Mandatory)][ValidatePattern('^[A-Za-z0-9_-]{1,50}$')][string]$Sku)
    $rows = @(docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U atlaspay -d atlaspay -At -c "SELECT quantity - reserved_qty FROM inventory WHERE sku = '$Sku';")
    if ($LASTEXITCODE -ne 0) { throw "Failed to read demo inventory for $Sku" }
    if ($rows.Count -eq 0) {
        Add-SmokeInventory -Sku $Sku -Quantity 10
        return
    }
    $available = 0
    if ($rows.Count -ne 1 -or -not [int]::TryParse([string]$rows[0], [ref]$available)) {
        throw "Invalid demo inventory response for $Sku"
    }
    if ($available -le 0) { Add-SmokeInventory -Sku $Sku -Quantity 10 }
}

if ($SelfTest) {
    $script:seedCalls = @()
    $script:rows = @()
    $script:exitCode = 0
    function docker { $global:LASTEXITCODE = $script:exitCode; $script:rows }
    function Add-SmokeInventory { param($Sku, $Quantity); $script:seedCalls += "$Sku|$Quantity" }
    foreach ($rows in @(@(), @('0'), @('-1'), @('5'))) {
        $script:rows = $rows
        Initialize-DemoInventory -Sku 'LAPTOP-001'
    }
    if ($script:seedCalls.Count -ne 3 -or @($script:seedCalls | Where-Object { $_ -ne 'LAPTOP-001|10' }).Count -ne 0) {
        throw 'Demo seed must add ten only when inventory is absent or unavailable'
    }
    foreach ($rows in @(@('invalid'), @('1', '2'))) {
        $script:rows = $rows
        $rejected = $false
        try { Initialize-DemoInventory -Sku 'LAPTOP-001' } catch { $rejected = $true }
        if (-not $rejected) { throw 'Invalid inventory response was accepted' }
    }
    $script:exitCode = 1
    $rejected = $false
    try { Initialize-DemoInventory -Sku 'LAPTOP-001' } catch { $rejected = $true }
    if (-not $rejected) { throw 'Database failure was accepted' }
    $global:LASTEXITCODE = 0
    Write-Host 'Local demo launcher self-check passed.'
    return
}

Push-Location (Split-Path $PSScriptRoot -Parent)
try {
    docker compose -f docker-compose.yml -f docker-compose.demo.yml up -d --build --wait --wait-timeout 300 api-gateway demo-ui
    if ($LASTEXITCODE -ne 0) { throw 'Local demo services did not become ready' }
    Initialize-DemoInventory -Sku 'LAPTOP-001'
    Initialize-DemoInventory -Sku 'FAIL-PAYMENT-001'
    Write-Host 'Open http://localhost:8085 for the local backend demo.'
    Write-Host 'Use scripts/start-local-demo.ps1 again to restart; existing data is retained.'
    $global:LASTEXITCODE = 0
} finally {
    Pop-Location
}
