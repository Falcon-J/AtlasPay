# Local Compose fixtures only. Add stock without overwriting reservations or existing prices.
function Add-SmokeInventory {
    param(
        [Parameter(Mandatory)][ValidatePattern('^[A-Za-z0-9_-]{1,50}$')][string]$Sku,
        [ValidateRange(1, 1000000)][int]$Quantity = 1
    )
    $id = [guid]::NewGuid().ToString()
    $sql = "INSERT INTO inventory (id, sku, name, description, quantity, unit_price) VALUES ('$id', '$Sku', 'Smoke $Sku', '', $Quantity, 99.99) ON CONFLICT (sku) DO UPDATE SET quantity = inventory.quantity + EXCLUDED.quantity, version = inventory.version + 1, updated_at = CURRENT_TIMESTAMP;"
    docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U atlaspay -d atlaspay -c $sql | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Failed to seed local Compose inventory for $Sku" }
}

function Test-SmokePaymentReplay {
    param([Parameter(Mandatory)][guid]$OrderId)
    $rows = docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U atlaspay -d atlaspay -At -c "SELECT row_to_json(p) FROM payments p WHERE order_id = '$OrderId';"
    if ($LASTEXITCODE -ne 0) { throw 'Failed to read saga payment fixture' }
    if (@($rows).Count -ne 1) { throw 'Expected exactly one saga payment before replay' }
    $payment = $rows | ConvertFrom-Json
    if ($payment.status -ne 'completed') { throw 'Saga payment is not completed' }
    $body = @{
        user_id = $payment.user_id
        order_id = $payment.order_id
        amount = $payment.amount
        currency = $payment.currency
        payment_method = $payment.payment_method
        idempotency_key = $payment.idempotency_key
    } | ConvertTo-Json -Compress
    # The internal token stays inside the payment container and never enters browser code or output.
    $command = 'test -n "$PAYMENT_SERVICE_TOKEN" && wget -q -O - --header="Content-Type: application/json" --header="X-Internal-Token: $PAYMENT_SERVICE_TOKEN" --post-data="$(cat)" "http://localhost:${PAYMENT_SERVICE_PORT:-8081}/internal/v1/payments/process"'
    for ($attempt = 0; $attempt -lt 2; $attempt++) {
        $result = $body | docker compose exec -T payment-service sh -c $command
        if ($LASTEXITCODE -ne 0) { throw 'Internal payment fixture replay failed' }
        $response = $result | ConvertFrom-Json
        if (-not $response.success -or $response.data.payment.id -ne $payment.id) {
            throw 'Internal replay did not return the original saga payment'
        }
    }
    $count = docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U atlaspay -d atlaspay -At -c "SELECT count(*) FROM payments WHERE order_id = '$OrderId';"
    if ($LASTEXITCODE -ne 0 -or [int]$count -ne 1) { throw 'Payment replay changed the payment count' }
    return $payment.id
}
