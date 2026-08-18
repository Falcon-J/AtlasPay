$ErrorActionPreference = "Stop"

foreach ($topic in @("atlaspay.orders", "atlaspay.dlq")) {
    $partitions = if ($topic -eq "atlaspay.orders") { 16 } else { 1 }
    docker compose exec -T kafka kafka-topics `
        --bootstrap-server localhost:9092 `
        --create `
        --if-not-exists `
        --topic $topic `
        --partitions $partitions `
        --replication-factor 1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to provision Kafka topic $topic"
    }
}

function Publish-Event {
    param([hashtable]$Event)

    $Event | ConvertTo-Json -Depth 10 -Compress |
        docker compose exec -T kafka kafka-console-producer `
            --bootstrap-server localhost:9092 `
            --topic atlaspay.orders
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to publish Kafka event"
    }
}

$stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$failedCorrelationId = "dlq-proof-$stamp"
$failedOrderId = "missing-order-$stamp"

Write-Host "Publishing unprocessable order.created event..."
Publish-Event @{
    id = "dlq-event-$stamp"
    type = "order.created"
    aggregate_id = $failedOrderId
    correlation_id = $failedCorrelationId
    timestamp = [DateTimeOffset]::UtcNow.ToString("o")
    version = 1
    payload = @{
        order_id = $failedOrderId
        user_id = "dlq-proof"
        items = @()
        total_price = 0
        currency = "USD"
    }
}

$row = ""
for ($attempt = 1; $attempt -le 20; $attempt++) {
    $row = docker compose exec -T postgres psql -U atlaspay -d atlaspay -At -F "|" -c `
        "SELECT topic, event_type, attempts, error_message FROM dead_letter_events WHERE correlation_id = '$failedCorrelationId' ORDER BY created_at DESC LIMIT 1"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to query dead_letter_events"
    }
    if ($row) {
        break
    }
    Start-Sleep -Milliseconds 500
}

if (-not $row) {
    throw "DLQ event was not persisted after 20 polling attempts"
}

$fields = $row -split "\|", 4
if ($fields[0] -ne "atlaspay.orders" -or $fields[1] -ne "order.created" -or $fields[2] -ne "3") {
    throw "Unexpected DLQ row: $row"
}

$dlqPublished = $false
for ($attempt = 1; $attempt -le 20; $attempt++) {
    $logs = docker compose logs --no-color order-service
    $dlqPublished = $logs |
        Select-String $failedCorrelationId |
        Select-String "event published" |
        Select-String -Quiet "atlaspay.dlq"
    if ($dlqPublished) {
        break
    }
    Start-Sleep -Milliseconds 500
}
if (-not $dlqPublished) {
    throw "Failed event was not published to atlaspay.dlq"
}

$continuationCorrelationId = "dlq-continue-$stamp"
Write-Host "Publishing follow-up event to prove the consumer continues..."
Publish-Event @{
    id = "continue-event-$stamp"
    type = "proof.consumer_continues"
    aggregate_id = "dlq-proof"
    correlation_id = $continuationCorrelationId
    timestamp = [DateTimeOffset]::UtcNow.ToString("o")
    version = 1
    payload = @{}
}

$processed = $false
for ($attempt = 1; $attempt -le 20; $attempt++) {
    $logs = docker compose logs --no-color order-service
    if ($logs | Select-String $continuationCorrelationId | Select-String -Quiet "event processed") {
        $processed = $true
        break
    }
    Start-Sleep -Milliseconds 500
}

if (-not $processed) {
    throw "Consumer did not process the follow-up event"
}

Write-Host ""
Write-Host "AtlasPay DLQ smoke test passed."
Write-Host "Failed correlation ID: $failedCorrelationId"
Write-Host "DLQ row: $row"
Write-Host "DLQ topic publish: atlaspay.dlq"
Write-Host "Follow-up correlation ID processed: $continuationCorrelationId"
