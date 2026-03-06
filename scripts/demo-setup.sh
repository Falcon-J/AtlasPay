#!/bin/bash
# Local Demo Setup Script for AtlasPay
# Use this to prepare for interviews - captures all screenshots

set -e

echo "🚀 AtlasPay Local Demo Setup"
echo "============================"
echo ""

# Check prerequisites
check_prereqs() {
    echo "📋 Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        echo "❌ Docker not found. Install from: https://docker.com"
        exit 1
    fi
    echo "✅ Docker installed"
    
    if ! command -v go &> /dev/null; then
        echo "❌ Go not found. Install from: https://go.dev"
        exit 1
    fi
    echo "✅ Go installed"
    
    if command -v minikube &> /dev/null; then
        echo "✅ minikube installed"
        HAS_MINIKUBE=true
    else
        echo "⚠️ minikube not found (optional for K8s demo)"
        HAS_MINIKUBE=false
    fi
    
    if command -v k6 &> /dev/null; then
        echo "✅ k6 installed"
        HAS_K6=true
    else
        echo "⚠️ k6 not found (optional for load testing)"
        HAS_K6=false
    fi
    
    echo ""
}

# Start infrastructure
start_infra() {
    echo "🐳 Starting Docker infrastructure..."
    docker-compose up -d postgres redis
    
    echo "⏳ Waiting for services to be healthy..."
    sleep 10
    
    echo "✅ Infrastructure ready"
    echo ""
}

# Build and run API
run_api() {
    echo "🔨 Building API Gateway..."
    go build -o api-gateway ./cmd/api-gateway
    
    echo "🚀 Starting API Gateway in background..."
    ./api-gateway &
    API_PID=$!
    
    echo "⏳ Waiting for API to start..."
    sleep 5
    
    # Health check
    if curl -s http://localhost:8080/health | grep -q "healthy"; then
        echo "✅ API Gateway running (PID: $API_PID)"
    else
        echo "❌ API failed to start"
        exit 1
    fi
    echo ""
}

# Create demo screenshots folder
setup_screenshots() {
    mkdir -p demo-screenshots
    echo "📁 Screenshots will be saved to: demo-screenshots/"
    echo ""
}

# Generate demo data
generate_demo_data() {
    echo "📝 Generating demo data..."
    
    # Login as admin
    echo "Logging in as admin..."
    LOGIN_RESPONSE=$(curl -s -X POST http://localhost:8080/api/auth/login \
        -H "Content-Type: application/json" \
        -d '{"email":"admin@atlaspay.com","password":"admin123"}')
    
    TOKEN=$(echo $LOGIN_RESPONSE | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)
    
    if [ -z "$TOKEN" ]; then
        echo "⚠️ Could not login (database may not be seeded)"
    else
        echo "✅ Logged in as admin"
        
        # Create some orders
        echo "Creating sample orders..."
        for i in {1..5}; do
            curl -s -X POST http://localhost:8080/api/orders \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer $TOKEN" \
                -d '{"items":[{"sku":"LAPTOP-001","quantity":1}]}' > /dev/null
        done
        echo "✅ Created 5 sample orders"
    fi
    echo ""
}

# Run load test
run_loadtest() {
    if [ "$HAS_K6" = true ]; then
        echo "📊 Running load test (this takes ~30s)..."
        k6 run --duration 30s --vus 50 scripts/k6/load-test.js > demo-screenshots/loadtest-results.txt 2>&1
        echo "✅ Load test complete - results in demo-screenshots/loadtest-results.txt"
    else
        echo "⚠️ Skipping load test (k6 not installed)"
        echo "   Install: winget install k6"
    fi
    echo ""
}

# Start monitoring
start_monitoring() {
    echo "📈 Starting monitoring stack..."
    docker-compose up -d prometheus grafana
    
    echo "⏳ Waiting for Grafana..."
    sleep 10
    
    echo "✅ Monitoring ready:"
    echo "   - Grafana: http://localhost:3000 (admin/admin123)"
    echo "   - Prometheus: http://localhost:9090"
    echo ""
}

# Print demo URLs
print_urls() {
    echo ""
    echo "🎉 Demo Setup Complete!"
    echo "======================="
    echo ""
    echo "URLs for demo:"
    echo "  📡 API:        http://localhost:8080"
    echo "  🏥 Health:     http://localhost:8080/health"
    echo "  📊 Grafana:    http://localhost:3000 (admin/admin123)"
    echo "  📈 Prometheus: http://localhost:9090"
    echo ""
    echo "Demo commands:"
    echo "  # Check health"
    echo "  curl http://localhost:8080/health"
    echo ""
    echo "  # Login"
    echo '  curl -X POST http://localhost:8080/api/auth/login \'
    echo '    -H "Content-Type: application/json" \'
    echo '    -d '\''{"email":"admin@atlaspay.com","password":"admin123"}'\'''
    echo ""
    echo "📸 Take screenshots now for your portfolio!"
    echo ""
}

# Main
main() {
    check_prereqs
    setup_screenshots
    start_infra
    run_api
    generate_demo_data
    start_monitoring
    run_loadtest
    print_urls
}

main
