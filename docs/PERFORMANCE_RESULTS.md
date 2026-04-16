# AtlasPay Performance Results

This file records measured results only. Keep public claims and key talking points aligned with numbers that are actually shown here.

## Latest Docker Compose Run

Date: 2026-04-17
Environment: Local Docker Compose on Windows, API + PostgreSQL + Redis + Kafka + Zookeeper
API deployment: `api-gateway` container, Kafka worker enabled
Database: PostgreSQL container
Redis: Redis container
Kafka: Kafka container with `atlaspay.orders` event flow enabled
Command:

```powershell
docker run --rm --network atlaspay_atlaspay-network -e BASE_URL=http://api-gateway:8080 -v "${PWD}\scripts\k6:/scripts" grafana/k6 run /scripts/load-test.js
```

Observed results:

```text
Duration: 9m02.4s
Total HTTP Requests: 116,121
HTTP Throughput: 214.105613 req/s
HTTP Throughput: 12,846 RPM
Failed HTTP Requests: 0.00%
Total Orders Created: 23,224
Order Creation Throughput: 42.820754 orders/s
Order Creation Throughput: 2,569 orders/min
Order Success Rate: 100.00%
HTTP Response Time Avg: 634.53ms
HTTP Response Time Median: 67.09ms
HTTP Response Time P90: 1.37s
HTTP Response Time P95: 4.96s
HTTP Response Time Max: 14.71s
Order Latency Avg: 2.59s
Order Latency Median: 901.5ms
Order Latency P90: 7.28s
Order Latency P95: 8.02s
Checks Passed: 255,464 / 255,464
```

Outcome against stated numbers:

```text
10k+ RPM: Supported for mixed HTTP traffic in this Docker Compose run (12,846 RPM).
p95 <= 120ms: Not supported by this run. Mixed HTTP p95 was 4.96s and order p95 was 8.02s.
Kafka event-driven flow: Supported by the demo smoke test and API logs showing order.created publish/process.
Saga retries/DLQ: Implemented in code; needs a dedicated before/after failure-rate experiment to defend "reduced failures by 40%".
Kubernetes + HPA + Prometheus: Manifests validate with kubectl dry-run; 99.9% uptime is not proven without a long-running availability test.
```

Interview-safe wording from this evidence:

```text
In a local Docker Compose benchmark with Kafka enabled, AtlasPay sustained about 12.8k mixed HTTP requests per minute with 0% HTTP failures. The same run did not meet the original 120ms p95 latency target, so I would describe 120ms as a target or optimization goal unless I have a separate benchmark that proves it.
```

## Required Command For Future Runs

```powershell
k6 run scripts/k6/load-test.js
```

For a remote or Kubernetes deployment:

```powershell
$env:BASE_URL="https://your-atlaspay-api.example.com"
k6 run scripts/k6/load-test.js
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

Total Requests:
Failed Requests:
Average Response Time:
P95 Response Time:
P99 Response Time:
Total Orders:
Order Success Rate:

Did it meet 10k+ RPM?
Did it meet p95 <= 120ms?
Notes:
```

Only keep numeric claims in public docs or talking points when this file contains a matching measured run.
