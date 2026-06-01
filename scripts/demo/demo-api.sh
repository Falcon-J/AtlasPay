#!/bin/bash

# AtlasPay API Demo Script
# Demonstrates the complete order → payment → inventory saga workflow
# Usage: ./demo-api.sh [api_url] (default: http://localhost:8080)

set -e

API_URL="${1:-http://localhost:8080}"
TIMESTAMP=$(date +%s%N | cut -b1-13)
USER_EMAIL="demo-user-${TIMESTAMP}@atlaspay.local"
USER_PASSWORD="DemoPass@123456"

echo "════════════════════════════════════════════════════════════════"
echo "  AtlasPay Distributed Payment System - Live API Demo"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "📍 API Endpoint: $API_URL"
echo "👤 Demo User: $USER_EMAIL"
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper function to print section headers
section() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}$1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# Helper function to print curl commands
run_curl() {
    local METHOD=$1
    local ENDPOINT=$2
    local DATA=$3
    local DESCRIPTION=$4
    
    echo ""
    echo -e "${YELLOW}📤 $DESCRIPTION${NC}"
    echo "Command: curl -X $METHOD \"$API_URL$ENDPOINT\" -H \"Content-Type: application/json\" ${DATA:+-d '$DATA'}"
    echo ""
    
    if [ -n "$DATA" ]; then
        curl -s -X "$METHOD" "$API_URL$ENDPOINT" \
            -H "Content-Type: application/json" \
            -d "$DATA" | jq '.'
    else
        curl -s -X "$METHOD" "$API_URL$ENDPOINT" \
            -H "Content-Type: application/json" | jq '.'
    fi
}

# ============================================================================
# STEP 1: Health Check
# ============================================================================
section "Step 1: Health Check - Verify System is Running"
echo "Checking if API, Database, and Cache are healthy..."
echo ""

HEALTH=$(curl -s "$API_URL/health" | jq '.')
echo "$HEALTH"
DB_STATUS=$(echo "$HEALTH" | jq -r '.db')
CACHE_STATUS=$(echo "$HEALTH" | jq -r '.cache')

if [ "$DB_STATUS" != "up" ] || [ "$CACHE_STATUS" != "up" ]; then
    echo "❌ System not healthy. Check docker-compose logs."
    exit 1
fi

echo ""
echo "✅ Database: up"
echo "✅ Cache (Redis): up"

# ============================================================================
# STEP 2: User Registration
# ============================================================================
section "Step 2: Register a Demo User"
echo "Creating a new user account for this demo session..."
echo ""

REG_RESPONSE=$(run_curl "POST" "/api/auth/register" \
    "{\"email\":\"$USER_EMAIL\",\"password\":\"$USER_PASSWORD\",\"name\":\"Demo User\"}" \
    "Register new user")

REG_STATUS=$(echo "$REG_RESPONSE" | jq -r '.status // empty')
if [ -z "$REG_STATUS" ]; then
    echo "❌ Registration failed"
    exit 1
fi

echo ""
echo "✅ User registered successfully"
echo "   Email: $USER_EMAIL"

# ============================================================================
# STEP 3: User Login & Get JWT Token
# ============================================================================
section "Step 3: Login & Obtain JWT Authentication Token"
echo "Authenticating with credentials to receive access token..."
echo ""

LOGIN_RESPONSE=$(run_curl "POST" "/api/auth/login" \
    "{\"email\":\"$USER_EMAIL\",\"password\":\"$USER_PASSWORD\"}" \
    "Login and get JWT token")

ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.access_token // empty')
REFRESH_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.refresh_token // empty')

if [ -z "$ACCESS_TOKEN" ]; then
    echo "❌ Login failed"
    echo "Response: $LOGIN_RESPONSE"
    exit 1
fi

echo ""
echo "✅ Authentication successful"
echo "   Access Token: ${ACCESS_TOKEN:0:50}..."
echo "   Token Type: JWT (expires in 15 minutes)"

# ============================================================================
# STEP 4: Check Inventory Status
# ============================================================================
section "Step 4: Check Current Inventory Status"
echo "Before placing an order, let's verify inventory availability..."
echo ""

INVENTORY=$(curl -s -X GET "$API_URL/api/inventory" \
    -H "Authorization: Bearer $ACCESS_TOKEN" | jq '.')

echo "$INVENTORY"

# ============================================================================
# STEP 5: Create Order (Triggers Saga Orchestration)
# ============================================================================
section "Step 5: Place an Order (This Triggers the Saga!)"
echo "Creating a new order with 5 units..."
echo "Behind the scenes, this will:"
echo "  1. Create order in system"
echo "  2. Reserve inventory"
echo "  3. Process payment"
echo "  4. Update stock"
echo "  5. Emit event via Kafka"
echo ""

ORDER_PAYLOAD=$(cat <<EOF
{
  "customer_id": "demo-customer-${TIMESTAMP}",
  "items": [
    {
      "product_id": "PROD-001",
      "quantity": 5,
      "unit_price": 99.99
    }
  ],
  "payment_method": "credit_card",
  "idempotency_key": "order-${TIMESTAMP}"
}
EOF
)

ORDER_RESPONSE=$(run_curl "POST" "/api/orders" \
    "$ORDER_PAYLOAD" \
    "Create order (triggers distributed saga)")

ORDER_ID=$(echo "$ORDER_RESPONSE" | jq -r '.data.id // empty')
ORDER_STATUS=$(echo "$ORDER_RESPONSE" | jq -r '.data.status // empty')

if [ -z "$ORDER_ID" ]; then
    echo "❌ Order creation failed"
    echo "Response: $ORDER_RESPONSE"
    exit 1
fi

echo ""
echo "✅ Order created successfully"
echo "   Order ID: $ORDER_ID"
echo "   Status: $ORDER_STATUS"
echo "   Total: \$$(echo "$ORDER_RESPONSE" | jq -r '.data.total_amount')"

# ============================================================================
# STEP 6: Poll Order Status (Watch Saga Progress)
# ============================================================================
section "Step 6: Monitor Saga Orchestration Progress"
echo "Polling order status to watch the saga complete..."
echo "The saga will: Order → Inventory Reserve → Payment Process → Finalize"
echo ""

for i in {1..10}; do
    echo "Poll #$i (waiting 2 seconds...)..."
    sleep 2
    
    STATUS_RESPONSE=$(curl -s -X GET "$API_URL/api/orders/$ORDER_ID" \
        -H "Authorization: Bearer $ACCESS_TOKEN" | jq '.')
    
    CURRENT_STATUS=$(echo "$STATUS_RESPONSE" | jq -r '.data.status // empty')
    SAGA_STATE=$(echo "$STATUS_RESPONSE" | jq -r '.data.saga_state // "N/A"')
    PAYMENT_STATUS=$(echo "$STATUS_RESPONSE" | jq -r '.data.payment_status // "pending"')
    
    echo "  Status: $CURRENT_STATUS | Saga: $SAGA_STATE | Payment: $PAYMENT_STATUS"
    
    if [ "$CURRENT_STATUS" = "completed" ] || [ "$CURRENT_STATUS" = "confirmed" ]; then
        echo ""
        echo "✅ Saga completed! Order finalized."
        echo ""
        echo "Final Order Details:"
        echo "$STATUS_RESPONSE" | jq '.data'
        break
    fi
    
    if [ "$CURRENT_STATUS" = "failed" ] || [ "$SAGA_STATE" = "FAILED" ]; then
        echo ""
        echo "❌ Saga failed!"
        echo "$STATUS_RESPONSE" | jq '.data'
        break
    fi
done

# ============================================================================
# STEP 7: Verify Payment Processing
# ============================================================================
section "Step 7: Verify Payment Was Processed"
echo "Checking payment status in the system..."
echo ""

PAYMENT_RESPONSE=$(curl -s -X GET "$API_URL/api/payments?order_id=$ORDER_ID" \
    -H "Authorization: Bearer $ACCESS_TOKEN" | jq '.')

echo "$PAYMENT_RESPONSE"

PAYMENT_ID=$(echo "$PAYMENT_RESPONSE" | jq -r '.data[0].id // empty')
if [ -n "$PAYMENT_ID" ]; then
    echo ""
    echo "✅ Payment confirmed"
    echo "   Payment ID: $PAYMENT_ID"
    echo "   Amount: \$(echo "$PAYMENT_RESPONSE" | jq -r '.data[0].amount')"
fi

# ============================================================================
# STEP 8: Check Updated Inventory
# ============================================================================
section "Step 8: Verify Inventory Was Updated"
echo "Confirming inventory was decremented after order completion..."
echo ""

UPDATED_INVENTORY=$(curl -s -X GET "$API_URL/api/inventory" \
    -H "Authorization: Bearer $ACCESS_TOKEN" | jq '.')

echo "$UPDATED_INVENTORY"

# ============================================================================
# STEP 9: Demonstrate Idempotency
# ============================================================================
section "Step 9: Demonstrate Idempotency (Retry Safety)"
echo "Submitting the SAME order again with the SAME idempotency key..."
echo "Expected: Returns same order, doesn't create duplicate"
echo ""

IDEMPOTENT_RESPONSE=$(run_curl "POST" "/api/orders" \
    "$ORDER_PAYLOAD" \
    "Retry order with same idempotency_key")

RETRY_ORDER_ID=$(echo "$IDEMPOTENT_RESPONSE" | jq -r '.data.id // empty')

if [ "$RETRY_ORDER_ID" = "$ORDER_ID" ]; then
    echo ""
    echo "✅ Idempotency works! Same order returned"
    echo "   Prevents duplicate charges on network retry"
else
    echo ""
    echo "⚠️  Different order returned (idempotency may have reset)"
fi

# ============================================================================
# Summary
# ============================================================================
section "Demo Complete ✅"
echo ""
echo "What You Just Saw:"
echo "  ✓ User authentication (JWT tokens)"
echo "  ✓ Order placement triggering saga"
echo "  ✓ Saga orchestration (Order → Inventory → Payment)"
echo "  ✓ Distributed transaction coordination"
echo "  ✓ Inventory management & cache"
echo "  ✓ Payment processing & idempotency"
echo ""
echo "Key Architecture Patterns Demonstrated:"
echo "  • Saga Orchestration - Managing distributed transactions"
echo "  • Idempotency - Safe retries with deduplication"
echo "  • Cache-Aside - Redis for inventory caching"
echo "  • JWT Auth - Token-based API security"
echo "  • Event-Driven - Kafka integration for async events"
echo ""
echo "System Details:"
echo "  • Language: Go 1.21"
echo "  • Database: PostgreSQL (primary data store)"
echo "  • Cache: Redis (inventory/order cache)"
echo "  • Queue: Kafka (optional, graceful degradation if down)"
echo "  • Deployment: Docker Compose on AWS EC2"
echo ""
echo "════════════════════════════════════════════════════════════════"
echo ""
