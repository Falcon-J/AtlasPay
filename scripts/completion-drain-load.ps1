param(
    [int]$TargetRPM = 1000,
    [string]$Duration = "10s",
    [int]$SkuCount = 32,
    [int]$DrainTimeoutSeconds = 60
)

$ErrorActionPreference = "Stop"

if ($TargetRPM -lt 1 -or $SkuCount -lt 1 -or $DrainTimeoutSeconds -lt 1) {
    throw "TargetRPM, SkuCount, and DrainTimeoutSeconds must be positive"
}

$loadSKU = "DRAIN-" + [guid]::NewGuid().ToString("N").Substring(0, 12)
$dockerK6Args = @(
    "run", "--rm", "-i",
    "-e", "TARGET_RPM=$TargetRPM",
    "-e", "DURATION=$Duration",
    "-e", "PREALLOCATED_VUS=$([Math]::Max(32, [Math]::Ceiling($TargetRPM / 60)))",
    "-e", "MAX_VUS=$([Math]::Max(64, [Math]::Ceiling($TargetRPM / 30)))",
    "-e", "LOAD_SKU=$loadSKU",
    "-e", "SKU_COUNT=$SkuCount",
    "-v", "${PWD}/scripts/k6:/scripts:ro",
    "grafana/k6", "run", "/scripts/order-submit-load.js"
)

Write-Host "Running submit phase: target=$TargetRPM RPM, duration=$Duration, sku_count=$SkuCount"
$k6Output = & docker @dockerK6Args 2>&1
$k6Exit = $LASTEXITCODE
$k6Output | ForEach-Object { Write-Host $_ }
if ($k6Exit -ne 0) {
    throw "k6 submit phase failed with exit code $k6Exit"
}

$cohortQuery = @"
SELECT count(*) AS total,
       count(*) FILTER (WHERE o.status = 'confirmed') AS confirmed,
       count(*) FILTER (WHERE o.status IN ('pending', 'processing')) AS active,
       count(*) FILTER (WHERE o.status = 'failed') AS failed
FROM orders o
JOIN order_items oi ON oi.order_id = o.id
WHERE oi.sku = '$loadSKU' OR oi.sku LIKE '$loadSKU-%';
"@

$deadline = [DateTimeOffset]::UtcNow.AddSeconds($DrainTimeoutSeconds)
$lastCohort = ""
do {
    $lastCohort = (docker compose exec -T postgres psql -U atlaspay -d atlaspay -At -F "|" -c $cohortQuery).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to query completion cohort"
    }
    Write-Host "cohort: $lastCohort"
    $parts = $lastCohort -split "\|"
    if ($parts.Count -ge 3 -and [int]$parts[0] -gt 0 -and [int]$parts[2] -eq 0) {
        break
    }
    Start-Sleep -Seconds 1
} while ([DateTimeOffset]::UtcNow -lt $deadline)

$parts = $lastCohort -split "\|"
if ($parts.Count -lt 4 -or [int]$parts[0] -eq 0 -or [int]$parts[2] -ne 0) {
    throw "Completion drain did not finish before timeout: $lastCohort"
}

$latencyQuery = @"
WITH cohort AS (
    SELECT DISTINCT o.id, o.status, o.created_at, o.updated_at
    FROM orders o
    JOIN order_items oi ON oi.order_id = o.id
    WHERE oi.sku = '$loadSKU' OR oi.sku LIKE '$loadSKU-%'
)
SELECT count(*) AS orders,
       count(*) FILTER (WHERE status = 'confirmed') AS confirmed,
       round((percentile_cont(0.50) WITHIN GROUP
           (ORDER BY EXTRACT(EPOCH FROM (updated_at - created_at)) * 1000)
           FILTER (WHERE status = 'confirmed'))::numeric, 1) AS p50_ms,
       round((percentile_cont(0.95) WITHIN GROUP
           (ORDER BY EXTRACT(EPOCH FROM (updated_at - created_at)) * 1000)
           FILTER (WHERE status = 'confirmed'))::numeric, 1) AS p95_ms,
       round((max(EXTRACT(EPOCH FROM (updated_at - created_at)) * 1000)
           FILTER (WHERE status = 'confirmed'))::numeric, 1) AS max_ms
FROM cohort;
"@

$latency = (docker compose exec -T postgres psql -U atlaspay -d atlaspay -c $latencyQuery).Trim()
if ($LASTEXITCODE -ne 0) {
    throw "Failed to query completion latency"
}

Write-Host "AtlasPay completion-drain load test passed."
Write-Host "Cohort SKU prefix: $loadSKU"
Write-Host "Final cohort: $lastCohort"
Write-Host $latency
