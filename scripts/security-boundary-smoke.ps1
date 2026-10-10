param([string]$BaseUrl = 'http://localhost:8080')
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/smoke-fixtures.ps1"

function Request-Security {
    param([string]$Method, [string]$Path, [string]$Token, $Body, [int]$Expected = 200)
    $headers = @{}
    if ($Token) { $headers.Authorization = "Bearer $Token" }
    $args = @{ Uri = "$BaseUrl$Path"; Method = $Method; Headers = $headers; SkipHttpErrorCheck = $true }
    if ($null -ne $Body) { $args.Body = ($Body | ConvertTo-Json -Depth 5); $args.ContentType = 'application/json' }
    $response = Invoke-WebRequest @args
    if ([int]$response.StatusCode -ne $Expected) { throw "$Method $Path returned $($response.StatusCode), expected $Expected" }
    if ($response.Content -and $response.Headers['Content-Type'] -match 'application/json') { return $response.Content | ConvertFrom-Json }
}

$stamp = [guid]::NewGuid().ToString('N')
$password = 'LocalSmoke123!'
$owner = Request-Security POST /api/auth/register -Body @{ email="owner-$stamp@example.com"; password=$password; first_name='Owner'; last_name='Smoke' } -Expected 201
$other = Request-Security POST /api/auth/register -Body @{ email="other-$stamp@example.com"; password=$password; first_name='Other'; last_name='Smoke' } -Expected 201
$ownerToken = $owner.data.access_token
$otherToken = $other.data.access_token
$ownerId = [guid]$owner.data.user.id
$adminId = [guid]::NewGuid()
$adminEmail = "admin-$stamp@example.com"
# Create a dedicated local admin fixture; never promote a public registration.
$sql = "INSERT INTO users (id,email,password_hash,first_name,last_name,role) SELECT '$adminId','$adminEmail',password_hash,'Admin','Smoke','admin' FROM users WHERE id='$ownerId';"
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U atlaspay -d atlaspay -c $sql | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Local admin fixture creation failed' }
$admin = Request-Security POST /api/auth/login -Body @{ email=$adminEmail; password=$password }
$adminToken = $admin.data.access_token
$sku = "SECURITY-$stamp"
Add-SmokeInventory -Sku $sku -Quantity 5

Request-Security POST /api/inventory/restock -Body @{sku=$sku;quantity=100} -Expected 401 | Out-Null
foreach ($token in @($ownerToken, $otherToken)) {
    Request-Security POST /api/inventory/restock -Token $token -Body @{sku=$sku;quantity=100} -Expected 403 | Out-Null
    Request-Security POST /api/payments/ -Token $token -Body @{order_id='forged';amount=1;idempotency_key=$stamp} -Expected 405 | Out-Null
    foreach ($operation in @('reserve','release','commit')) {
        Request-Security POST "/api/inventory/$operation" -Token $token -Body @{order_id=$stamp;items=@(@{sku=$sku;quantity=1})} -Expected 405 | Out-Null
    }
}
Request-Security GET "/api/inventory/$sku" -Expected 401 | Out-Null
$stock = Request-Security GET "/api/inventory/$sku" -Token $ownerToken
if ($stock.data.item.quantity -ne 5 -or $stock.data.item.reserved_qty -ne 0) { throw 'Rejected requests changed inventory' }
Request-Security POST /api/inventory/restock -Token $adminToken -Body @{sku=$sku;quantity=1} | Out-Null
$stock = Request-Security GET "/api/inventory/$sku" -Token $otherToken
if ($stock.data.item.quantity -ne 6) { throw 'Admin restock did not add inventory' }
$order = Request-Security POST /api/orders/ -Token $ownerToken -Body @{items=@(@{sku=$sku;quantity=1})} -Expected 201
$orderId = $order.data.order.id
for ($attempt = 0; $attempt -lt 60; $attempt++) {
    $state = Request-Security GET "/api/orders/$orderId" -Token $ownerToken
    if ($state.data.order.status -eq 'confirmed') { break }
    Start-Sleep -Milliseconds 500
}
if ($state.data.order.status -ne 'confirmed') { throw 'Legitimate remote checkout did not confirm' }
Request-Security GET "/api/orders/$orderId/saga" -Expected 401 | Out-Null
Request-Security GET "/api/orders/$orderId/saga" -Token $otherToken -Expected 403 | Out-Null
Request-Security GET "/api/orders/$orderId/saga" -Token $ownerToken | Out-Null
Request-Security GET "/api/orders/$orderId/saga" -Token $adminToken | Out-Null
Write-Host 'Security boundary smoke passed: two users/admin, unchanged denied inventory, remote saga ownership, checkout, and payment initiation.'
$global:LASTEXITCODE = 0
