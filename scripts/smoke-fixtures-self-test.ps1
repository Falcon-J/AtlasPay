$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/smoke-fixtures.ps1"
$script:commands = @()
function docker {
    $script:commands += ,$args
    $global:LASTEXITCODE = 0
}
Add-SmokeInventory -Sku 'TEST-001' -Quantity 2
$sql = $script:commands[0][-1]
if ($sql -notmatch 'quantity = inventory.quantity \+ EXCLUDED.quantity' -or $sql -match 'reserved_qty\s*=') {
    throw 'Fixture must add stock without changing reserved quantity'
}
if ($sql -notmatch 'name, description, quantity' -or $sql -notmatch "'Smoke TEST-001', ''") {
    throw 'Fixture must provide the non-null description required by the inventory reader'
}
foreach ($sku in @("TEST'; DROP TABLE inventory; --", '', ('x' * 51))) {
    $rejected = $false
    try { Add-SmokeInventory -Sku $sku } catch { $rejected = $true }
    if (-not $rejected) { throw 'Invalid SKU was accepted' }
}
foreach ($quantity in @(0, -1, 1000001)) {
    $rejected = $false
    try { Add-SmokeInventory -Sku 'TEST-001' -Quantity $quantity } catch { $rejected = $true }
    if (-not $rejected) { throw 'Invalid stock quantity was accepted' }
}
function docker { $global:LASTEXITCODE = 1 }
$rejected = $false
try { Add-SmokeInventory -Sku 'TEST-001' } catch { $rejected = $true }
if (-not $rejected) { throw 'Failed database command was accepted' }
Write-Host 'Smoke inventory fixture self-check passed.'
$script:replayCalls = 0
$script:paymentId = [guid]::NewGuid().ToString()
$script:orderId = [guid]::NewGuid()
function docker {
    $global:LASTEXITCODE = 0
    if ($args -contains 'psql') {
        if ($args[-1] -match 'row_to_json') {
            return (@{ id=$script:paymentId; order_id=$script:orderId; user_id='fixture-user'; amount=99.99; currency='USD'; payment_method='demo_card'; idempotency_key='original-key'; status='completed' } | ConvertTo-Json -Compress)
        }
        return '1'
    }
    if ($args -notcontains 'payment-service' -or $args[-1] -notmatch 'X-Internal-Token: \$PAYMENT_SERVICE_TOKEN') {
        throw 'Replay must authenticate inside payment container'
    }
    $body = @($input) -join '' | ConvertFrom-Json
    if ($body.idempotency_key -ne 'original-key' -or $body.order_id -ne $script:orderId.ToString() -or $body.amount -ne 99.99) {
        throw 'Replay must reuse the original saga payment contract'
    }
    $script:replayCalls++
    return (@{ success=$true; data=@{payment=@{id=$script:paymentId}} } | ConvertTo-Json -Depth 4 -Compress)
}
$result = Test-SmokePaymentReplay -OrderId $script:orderId
if ($result -ne $script:paymentId -or $script:replayCalls -ne 2) { throw 'Replay assertions failed' }
function docker { $global:LASTEXITCODE = 1 }
$rejected = $false
try { Test-SmokePaymentReplay -OrderId $script:orderId } catch { $rejected = $true }
if (-not $rejected) { throw 'Failed replay database query was accepted' }
Write-Host 'Smoke payment replay self-check passed.'
