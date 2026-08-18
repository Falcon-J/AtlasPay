# AtlasPay Performance Results

This file records measured results only. Keep public claims and key talking points aligned with numbers that are actually shown here.

## Historical HTTP benchmark (May 2026)

Date: 2026-05-15
Environment: Local Docker Compose on Windows, API + PostgreSQL + Redis + Kafka + Zookeeper
Command: `k6 run scripts/k6/load-test.js`

The older `scripts/k6/load-test.js` run measured mixed HTTP traffic. Its raw
output is not present here, so the historic 25k-RPM figure is retained only as
an unverified historical note, not as a current checkout claim.

## Current completed-checkout probes

The authoritative local benchmark is
`scripts/completion-drain-load.ps1`. It submits a unique multi-SKU cohort,
then measures completion from PostgreSQL rather than turning per-order status
polling into a second load generator. The older
`scripts/k6/checkout-load.js` remains useful for demonstrating why polling can
saturate a benchmark, but its results are diagnostic rather than release
evidence. See:

- `docs/evidence/checkout-load-probe-2026-08-18.txt`
- `docs/evidence/checkout-load-probe-workers-2026-08-18.txt`
- `docs/evidence/order-submit-load-probe-2026-08-18.txt`
- `docs/evidence/completion-drain-load-output-2026-08-19.txt`

The controlled 1,000-RPM, 10-second run accepted and completed 167/167 orders
with 520.3 ms completion p95. A 12,500-RPM, 5-second, 64-SKU burst accepted
and eventually completed 1,041/1,041 orders, but completion p95 was 32.2
seconds. The latter is burst-and-drain evidence, not sustained completed-
checkout capacity or a 120 ms SLO result.

## Required Command For Future Runs

```powershell
powershell -ExecutionPolicy Bypass -File scripts/completion-drain-load.ps1 `
  -TargetRPM 1000 -Duration 10s -SkuCount 32 -DrainTimeoutSeconds 60
```

For a remote or Kubernetes deployment:

```powershell
$env:BASE_URL="https://your-atlaspay-api.example.com"
k6 run scripts/k6/checkout-load.js
```

## Result Template

```text
Date:
Environment:
API deployment:
Database:
Redis:
Kafka:
Command:

Target RPM:
Checkout Attempts:
Failed HTTP Requests:
Average Response Time:
P95 Response Time:
P99 Response Time:
Total Orders:
Order Success Rate:

Did it meet the target completed-checkout RPM?
Did it meet p95 <= 120ms?
Notes:
```

Only keep numeric claims in public docs or talking points when this file contains a matching measured run.
