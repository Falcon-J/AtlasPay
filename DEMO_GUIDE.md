# AtlasPay - Live API Demo Guide

**For Interviews at Harness (or any platform company)**

This guide walks you through demonstrating the AtlasPay distributed payment system in action. Rather than showing a UI, we'll demonstrate the **actual system behavior** through live API calls.

---

## 🎯 Why This Demo Approach?

Platform companies like Harness care about:
- ✅ **Architecture decisions** - How you design distributed systems
- ✅ **Problem solving** - Handling failures, retries, idempotency
- ✅ **Technology knowledge** - Databases, caching, message queues
- ✅ **Real-time system behavior** - Not polished UIs

This demo shows all of that in ~5 minutes.

---

## 🚀 Quick Start

### Option 1: Run Locally (Against EC2 Backend)

```bash
# Using bash (Mac/Linux or WSL on Windows)
chmod +x ./demo-api.sh
./demo-api.sh http://52.23.219.80:8080
```

### Option 2: PowerShell (Windows Native)

```powershell
# Windows PowerShell
.\demo-api.ps1 -ApiUrl "http://52.23.219.80:8080"
```

### Option 3: Manual curl Commands (For Interview Control)

Run each command individually to explain what's happening:

```bash
API_URL="http://52.23.219.80:8080"

# 1. Health check
curl -s $API_URL/health | jq '.'

# 2. Register user
curl -s -X POST $API_URL/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@test.com","password":"DemoPass@123","name":"Demo User"}' | jq '.'

# 3. Login
curl -s -X POST $API_URL/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@test.com","password":"DemoPass@123"}' | jq '.data.access_token'

# 4. Create order (with token)
TOKEN="<paste token from login>"
curl -s -X POST $API_URL/api/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "customer_id": "cust-001",
    "items": [{"product_id": "PROD-001", "quantity": 5, "unit_price": 99.99}],
    "payment_method": "credit_card",
    "idempotency_key": "order-unique-123"
  }' | jq '.'
```

---

## 📋 What the Demo Shows (Step by Step)

### Step 1: Health Check ✅
```json
{
  "status": "healthy",
  "db": "up",
  "cache": "up"
}
```
**Interview Point:** "System has three core components - API, PostgreSQL, and Redis - all verified healthy."

---

### Step 2: User Registration ✅
The demo creates a unique user for each run (prevents duplicates).

```json
{
  "status": "success",
  "data": {
    "id": "user-uuid",
    "email": "demo-user-1234@atlaspay.local"
  }
}
```
**Interview Point:** "Demonstrates JWT auth system, password hashing, and user lifecycle."

---

### Step 3: Authentication & JWT Token ✅
Login returns two tokens:

```json
{
  "data": {
    "access_token": "eyJhbGc...",  # Short-lived (15 min)
    "refresh_token": "...",         # Long-lived (7 days)
  }
}
```
**Interview Point:** "Token rotation strategy prevents stale sessions. Access tokens are short-lived for security."

---

### Step 4: Create Order (THE SAGA TRIGGER) 🔥

This is where it gets interesting. The order triggers a **distributed saga**:

```json
{
  "data": {
    "id": "order-uuid",
    "status": "pending",
    "total_amount": 499.95,
    "items": [
      {
        "product_id": "PROD-001",
        "quantity": 5,
        "unit_price": 99.99
      }
    ],
    "idempotency_key": "order-1234567890"
  }
}
```

**What happens behind the scenes (distributed saga):**
```
CREATE ORDER (pending) 
    ↓
RESERVE INVENTORY (from cache via Redis)
    ↓
PROCESS PAYMENT (deduct funds)
    ↓
FINALIZE ORDER (mark complete)
    ↓
EVENT EMITTED (Kafka - could trigger downstream services)
```

**Interview Points:**
- "Saga pattern coordinates across 3 microservices"
- "Each step has retry logic with exponential backoff"
- "If payment fails, inventory is released (compensating transaction)"
- "Idempotency key prevents duplicate charges on network retry"

---

### Step 5: Monitor Saga Progress 📊

The demo polls the order status to show saga completion:

```
Poll #1:  Status: pending | Saga: ORDER_CREATED | Payment: pending
Poll #2:  Status: pending | Saga: INVENTORY_RESERVED | Payment: pending
Poll #3:  Status: pending | Saga: PAYMENT_PROCESSING | Payment: processing
Poll #4:  Status: completed | Saga: FINALIZED | Payment: completed
```

**Interview Points:**
- "Real-time visibility into distributed transaction state"
- "Each saga step is logged with correlation IDs for tracing"
- "If any step fails (inventory shortage, payment decline), saga compensation kicks in"

---

### Step 6: Verify Payment ✅

```json
{
  "data": [
    {
      "id": "payment-uuid",
      "order_id": "order-uuid",
      "amount": 499.95,
      "status": "completed",
      "idempotency_key": "order-1234567890"
    }
  ]
}
```

**Interview Point:** "Idempotency key ensures the same payment isn't processed twice, even if the client retries."

---

### Step 7: Verify Inventory Updated ✅

Show inventory decreased after order:
```json
{
  "data": {
    "PROD-001": {
      "available": 95,  # Was 100, now 95 after order
      "reserved": 0,
      "last_updated": "2026-05-12T05:21:45Z"
    }
  }
}
```

**Interview Point:** "Cache-aside pattern - reads from Redis (fast), writes to PostgreSQL (durable)."

---

### Step 8: Demonstrate Idempotency 🔄

Submit the **exact same order again** with the same `idempotency_key`:

```json
Response: Same order returned, status unchanged
No duplicate charge, no new payment
```

**Interview Point:** "Idempotency is critical for distributed systems where network failures cause retries. We deduplicate using idempotency_key (UUID + timestamp)."

---

## 🗣️ Interview Talking Points

### When They Ask: "How Does This Differ From Render?"

**You Can Say:**
- "Originally on Render, but free tier had async DB provisioning delays"
- "Added exponential backoff retry logic (20 attempts, 2s→60s) to handle async startup"
- "For demo stability, moved to AWS EC2 free tier with pre-built Docker image"
- "Eliminates build overhead on resource-constrained hardware"

---

### When They Ask: "Tell Me About the Saga Pattern"

**You Can Say:**
- "AtlasPay uses orchestrated saga for distributed transactions"
- "Order creation triggers a sequence: create → reserve inventory → charge payment → finalize"
- "Each step is idempotent - can be safely retried"
- "If payment fails, inventory reservation is released (compensating transaction)"
- "Saga state is persisted in PostgreSQL with correlation IDs for debugging"

---

### When They Ask: "How Do You Handle Failures?"

**You Can Say:**
- "Saga has automatic retry with exponential backoff"
- "Failed steps are logged to Dead Letter Queue (DLQ) for manual inspection"
- "Graceful degradation: if Kafka is unavailable, saga still completes (events queued locally)"
- "Redis failures don't break the system (falls back to database)"
- "All operations include correlation IDs for distributed tracing"

---

### When They Ask: "How is This Different From a Monolith?"

**You Can Say:**
- "This is a distributed system with 3 loosely-coupled services"
- "Each service has its own database schema (order_schema, inventory_schema, payment_schema)"
- "Communication is event-driven via Kafka (async + resilient)"
- "Scaling: can deploy payment service separately from order service"
- "Failure isolation: payment service can go down without crashing orders"

---

### When They Ask: "How Do You Deploy This?"

**You Can Say:**
- "Docker Compose locally with PostgreSQL + Redis"
- "For cloud, built Docker image on laptop (1GB RAM on EC2 insufficient for Go build)"
- "SCP image to EC2, load with `docker load`, start with `docker compose up`"
- "Services auto-restart on failure (`restart: unless-stopped`)"
- "Health checks via `/health` endpoint (db + cache verification)"

---

## 🛠️ Troubleshooting During Demo

### API Returns 401 (Unauthorized)
- Token may have expired (15-min lifetime)
- Re-run login to get fresh token
- **Interview point:** "Explains why short-lived tokens are important"

### Order Status Stays "Pending" Too Long
- Saga may be processing payment
- Check backend logs: `docker compose logs api --tail=50`
- **Interview point:** "Shows how to debug distributed systems"

### Payment Returns 500 Error
- Database might be restarting after EC2 restart
- Wait 30 seconds, try again
- **Interview point:** "Demonstrates importance of retry logic"

### Inventory Not Updating
- Redis cache might be stale
- Check if DB update succeeded: query PostgreSQL directly
- **Interview point:** "Cache consistency trade-offs"

---

## 📸 Screenshots/Videos You Can Show

1. **API Health Check** - Shows all 3 components healthy
2. **Order Creation Response** - Shows saga_state: "ORDER_CREATED"
3. **Polling Output** - Shows saga progression (INVENTORY_RESERVED → PAYMENT_PROCESSING → FINALIZED)
4. **Idempotency Test** - Same order returned, no duplicate charge
5. **Logs with Correlation IDs** - Shows distributed tracing across services

---

## 🎬 Demo Script (5 Minutes)

**Opening:**
> "I built AtlasPay as a distributed payment system demonstrating patterns used at companies like Stripe and Square. Let me show you how it handles a real transaction."

**During Demo:**
> "You'll see how a single order triggers multiple coordinated services, with built-in failure recovery."

**After Demo:**
> "This shows real challenges in distributed systems: saga orchestration, idempotency, distributed tracing. All implemented in Go with proper testing and error handling."

---

## 📚 Architecture Quick Reference

```
┌─────────────────────────────────────────────────────┐
│          HTTP/REST API (Gin Router)                 │
│              :8080                                   │
└────────────┬────────────────────────────────────────┘
             │
    ┌────────┴────────┬──────────────┐
    ▼                 ▼              ▼
┌─────────┐    ┌─────────────┐  ┌─────────┐
│ Order   │    │   Payment   │  │Inventory│
│Service  │    │   Service   │  │Service  │
└────┬────┘    └─────┬───────┘  └────┬────┘
     │               │               │
     └───────────────┼───────────────┘
                     │
        ┌────────────┼────────────┐
        ▼            ▼            ▼
    ┌────────┐  ┌───────┐  ┌──────────┐
    │ Saga   │  │Postgres│  │  Redis   │
    │Engine  │  │ (Main) │  │ (Cache)  │
    └────────┘  └───────┘  └──────────┘
```

---

## ✅ Demo Checklist

Before the interview:
- [ ] EC2 instance is running (`ping 52.23.219.80`)
- [ ] Docker services are healthy (`docker compose ps`)
- [ ] `/health` endpoint returns db:up, cache:up
- [ ] Have the demo script tested locally
- [ ] Understand each step (you'll be asked to explain)
- [ ] Have a backup: can fall back to manual curl commands
- [ ] Know how to check logs if something fails

---

## 🚀 Go Get 'Em!

This demo proves you can:
- ✅ Design distributed systems
- ✅ Implement production patterns (saga, idempotency, caching)
- ✅ Deploy to cloud (AWS)
- ✅ Debug real issues
- ✅ Explain complex concepts clearly

Perfect for Harness interviews. Good luck! 🎯
