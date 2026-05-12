# AtlasPay - Live Demo (Manual Commands)

This guide shows you how to manually run the demo using `curl` commands. This approach is more reliable and gives you complete control during interviews.

## Quick Start (Copy & Paste)

```bash
# Set the API endpoint
API_URL="http://52.23.219.80:8080"

# Create unique user for this demo
TIMESTAMP=$(date +%s%N | cut -b1-13)
EMAIL="demo-$TIMESTAMP@test.com"
PASS="DemoPass@123"

# 1. Health Check
echo "1. Checking System Health..."
curl -s $API_URL/health | jq '.'

# 2. Register User
echo -e "\n2. Registering User ($EMAIL)..."
curl -s -X POST $API_URL/api/auth/register \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\",\"name\":\"Demo User\"}" | jq '.'

# 3. Login & Get Token
echo -e "\n3. Logging in & getting JWT token..."
TOKEN=$(curl -s -X POST $API_URL/api/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" | jq -r '.data.access_token')

echo "Token received: ${TOKEN:0:50}..."

# 4. Place Order (Triggers Saga!)
echo -e "\n4. Creating Order (this triggers the saga)..."
ORDER_ID=$(curl -s -X POST $API_URL/api/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"customer_id\": \"cust-$TIMESTAMP\",
    \"items\": [{\"product_id\": \"PROD-001\", \"quantity\": 5, \"unit_price\": 99.99}],
    \"payment_method\": \"credit_card\",
    \"idempotency_key\": \"order-$TIMESTAMP\"
  }" | jq -r '.data.id')

echo "Order ID: $ORDER_ID"

# 5. Monitor Order Status
echo -e "\n5. Monitoring saga progress (watching it complete)..."
for i in {1..8}; do
  sleep 2
  STATUS=$(curl -s -X GET "$API_URL/api/orders/$ORDER_ID" \
    -H "Authorization: Bearer $TOKEN" | jq -r '.data | "\(.status) | saga: \(.saga_state) | payment: \(.payment_status)"')
  echo "  Poll $i: $STATUS"
  
  if [[ $STATUS == *"completed"* ]] || [[ $STATUS == *"confirmed"* ]]; then
    echo "  [SUCCESS] Saga complete!"
    break
  fi
done

# 6. Check Payment Status
echo -e "\n6. Verifying Payment..."
curl -s -X GET "$API_URL/api/payments?order_id=$ORDER_ID" \
  -H "Authorization: Bearer $TOKEN" | jq '.data[0] | {id, amount, status}'

# 7. Test Idempotency (Submit same order again)
echo -e "\n7. Testing Idempotency (resubmit same order)..."
curl -s -X POST $API_URL/api/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"customer_id\": \"cust-$TIMESTAMP\",
    \"items\": [{\"product_id\": \"PROD-001\", \"quantity\": 5, \"unit_price\": 99.99}],
    \"payment_method\": \"credit_card\",
    \"idempotency_key\": \"order-$TIMESTAMP\"
  }" | jq '.data | {id, status}'

echo -e "\n=== Demo Complete ==="
```

## Step-by-Step Breakdown

### Step 1: Health Check

```bash
curl -s http://52.23.219.80:8080/health | jq '.'
```

Expected output:
```json
{
  "status": "healthy",
  "db": "up",
  "cache": "up"
}
```

**Interview Point:** "All three components (API, PostgreSQL, Redis) are healthy and ready."

---

### Step 2: Register User

```bash
curl -s -X POST http://52.23.219.80:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@test.com","password":"DemoPass@123","name":"Demo User"}'
```

Expected output:
```json
{
  "status": "success",
  "data": {
    "id": "user-uuid",
    "email": "demo@test.com",
    "name": "Demo User"
  }
}
```

---

### Step 3: Login & Get Token

```bash
curl -s -X POST http://52.23.219.80:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@test.com","password":"DemoPass@123"}'
```

Expected output:
```json
{
  "status": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 900
  }
}
```

**Save the `access_token` - you'll need it for the next requests.**

---

### Step 4: Create Order (The Saga Begins!)

```bash
TOKEN="<paste your access_token here>"

curl -s -X POST http://52.23.219.80:8080/api/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "customer_id": "cust-demo-001",
    "items": [
      {
        "product_id": "PROD-001",
        "quantity": 5,
        "unit_price": 99.99
      }
    ],
    "payment_method": "credit_card",
    "idempotency_key": "order-unique-abc-123"
  }'
```

Expected output:
```json
{
  "status": "success",
  "data": {
    "id": "order-uuid",
    "customer_id": "cust-demo-001",
    "status": "pending",
    "saga_state": "ORDER_CREATED",
    "total_amount": 499.95,
    "items": [...],
    "idempotency_key": "order-unique-abc-123"
  }
}
```

**Interview Point:** 
- "Notice `saga_state: ORDER_CREATED` - this is the first step in the distributed saga"
- "The `idempotency_key` ensures this order can be safely retried without duplicates"
- "Order status is `pending` - it will transition through saga steps"

---

### Step 5: Monitor Saga Progress (Watch It Complete)

Replace `ORDER_ID` with the ID from Step 4:

```bash
TOKEN="<your token>"
ORDER_ID="<order id from step 4>"

for i in {1..10}; do
  sleep 2
  echo "Poll $i:"
  curl -s -X GET "http://52.23.219.80:8080/api/orders/$ORDER_ID" \
    -H "Authorization: Bearer $TOKEN" | jq '.data | {status, saga_state, payment_status}'
done
```

You'll see progression like:
```
Poll 1: status: pending | saga_state: ORDER_CREATED | payment_status: pending
Poll 2: status: pending | saga_state: INVENTORY_RESERVED | payment_status: pending
Poll 3: status: pending | saga_state: PAYMENT_PROCESSING | payment_status: processing
Poll 4: status: completed | saga_state: FINALIZED | payment_status: completed
```

**Interview Points:**
- "This shows the saga orchestrating across multiple services"
- "Each step is tracked with correlation IDs for distributed tracing"
- "If payment failed, inventory would be released (compensating transaction)"

---

### Step 6: Verify Payment Was Processed

```bash
curl -s -X GET "http://52.23.219.80:8080/api/payments?order_id=ORDER_ID" \
  -H "Authorization: Bearer $TOKEN" | jq '.'
```

Expected output:
```json
{
  "status": "success",
  "data": [
    {
      "id": "payment-uuid",
      "order_id": "order-uuid",
      "amount": 499.95,
      "status": "completed",
      "idempotency_key": "order-unique-abc-123",
      "created_at": "2026-05-12T05:21:45Z"
    }
  ]
}
```

**Interview Point:** "Notice the payment has the SAME `idempotency_key` as the order - this is how we prevent duplicate charges."

---

### Step 7: Demonstrate Idempotency

Submit the **exact same order** with the **exact same `idempotency_key`**:

```bash
curl -s -X POST http://52.23.219.80:8080/api/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "customer_id": "cust-demo-001",
    "items": [
      {
        "product_id": "PROD-001",
        "quantity": 5,
        "unit_price": 99.99
      }
    ],
    "payment_method": "credit_card",
    "idempotency_key": "order-unique-abc-123"
  }'
```

**Result:** You'll get back the **same order ID** as before.

**Interview Point:** 
- "This is critical for distributed systems where network failures cause client retries"
- "Without idempotency, a network timeout could cause duplicate charges"
- "Our system uses `idempotency_key` to deduplicate requests"

---

## 🎬 Interview Demo Script (5 Minutes)

**Opening:**
> "I've built AtlasPay, a distributed payment system that handles real-world challenges like saga orchestration, payment idempotency, and failure recovery. Let me show you how it works."

**During health check:**
> "First, I'm verifying all three components are healthy: API Gateway, PostgreSQL database, and Redis cache."

**During order creation:**
> "Now I'm placing an order. Behind the scenes, this triggers a distributed saga that will: reserve inventory, process payment, and finalize the order. All coordinated across multiple services."

**During saga polling:**
> "Notice we're tracking the saga state through each step. If payment failed, inventory would be automatically released—that's called a compensating transaction."

**During idempotency test:**
> "Finally, I'm resubmitting the same order with the same idempotency key. Notice we get back the same order—no duplicate charge. This is how production systems handle network retries."

**Closing:**
> "This demonstrates real distributed system patterns: saga orchestration, idempotency, distributed tracing, and graceful failure handling. All built in Go with proper testing and monitoring."

---

## Troubleshooting

### 401 Unauthorized
- Token might have expired (15-minute lifetime)
- Re-run the login step to get a fresh token

### 500 Error on Payment
- Database might be restarting
- Wait 10 seconds and try again
- Shows importance of retry logic

### Status Stays "Pending" Too Long
- Saga might be processing
- Check logs: `docker compose logs api --tail=50`
- Demonstrates monitoring & debugging distributed systems

---

## Key Takeaways for Interview

1. **Saga Pattern** - Demonstrates how to handle distributed transactions
2. **Idempotency** - Shows understanding of network failure resilience
3. **Caching** - Redis for fast reads, PostgreSQL for durability
4. **Authentication** - JWT with token rotation
5. **Graceful Degradation** - Works even if Kafka is down
6. **Distributed Tracing** - All requests have correlation IDs
7. **Error Handling** - Dead Letter Queue for failed transactions

This is a realistic demo of production-grade system design. Good luck! 🚀
