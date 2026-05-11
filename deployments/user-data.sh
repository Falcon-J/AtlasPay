#!/bin/bash
# Lean AWS EC2 Deployment for AtlasPay (Free Tier Optimized)
# Requires: Ubuntu 22.04, t2.micro/t3.micro, 1GB RAM

set -e

echo "=== AtlasPay EC2 Deployment Starting ==="

# Update system
sudo apt update
sudo apt upgrade -y

# Install Docker & Docker Compose
sudo apt install -y docker.io docker-compose-v2 git curl

# Add user to docker group (avoid sudo)
sudo usermod -aG docker ubuntu

# Start Docker
sudo systemctl start docker
sudo systemctl enable docker

# Create app directory
mkdir -p ~/atlaspay
cd ~/atlaspay

# Clone repository
git clone https://github.com/Falcon-J/AtlasPay.git .
cd ~/atlaspay

# Create optimized docker-compose for free tier
cat > docker-compose.yml << 'EOF'
version: "3.9"

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: atlaspay
      POSTGRES_PASSWORD: atlaspay_secret
      POSTGRES_DB: atlaspay
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U atlaspay"]
      interval: 10s
      timeout: 5s
      retries: 3

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 3

  zookeeper:
    image: confluentinc/cp-zookeeper:7.5.0
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
    restart: unless-stopped

  kafka:
    image: confluentinc/cp-kafka:7.5.0
    depends_on:
      - zookeeper
    ports:
      - "9092:9092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
      KAFKA_HEAP_OPTS: "-Xmx256M -Xms256M"
    restart: unless-stopped

  api:
    build: .
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      kafka:
        condition: service_started
    ports:
      - "8080:8080"
    environment:
      DB_HOST: postgres
      DB_PORT: 5432
      DB_USER: atlaspay
      DB_PASSWORD: atlaspay_secret
      DB_NAME: atlaspay
      DB_SSLMODE: disable
      
      REDIS_HOST: redis
      REDIS_PORT: 6379
      
      KAFKA_BROKERS: kafka:9092
      
      SERVER_PORT: 8080
      SERVER_READ_TIMEOUT: 15s
      SERVER_WRITE_TIMEOUT: 15s
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  postgres_data:
  redis_data:
EOF

echo "✅ Docker Compose configuration created"

# Start services
echo "=== Starting services ==="
docker compose up -d

# Wait for services to initialize
echo "⏳ Waiting for services to be healthy (30-45 seconds)..."
sleep 45

# Check health
echo "🏥 Checking API health..."
if curl -f http://localhost:8080/health 2>/dev/null; then
    echo "✅ AtlasPay is healthy!"
    curl -s http://localhost:8080/health | jq .
else
    echo "⚠️ Health check pending, services may still be starting..."
    echo "Monitor with: docker compose logs -f api"
fi

# Display logs
echo ""
echo "=== Service Status ==="
docker compose ps

echo ""
echo "=== Setup Complete ==="
echo "🌐 Access API at: http://$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4):8080"
echo "📊 Monitor logs: cd ~/atlaspay && docker compose logs -f"
echo ""
echo "🛑 To stop services: docker compose down"
echo "🔄 To restart services: docker compose restart"

