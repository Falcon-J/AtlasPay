#!/bin/bash
# User data script for AtlasPay EC2 instance
# This runs as root on instance startup

set -e

echo "=== AtlasPay EC2 Setup Starting ==="

# Update system
yum update -y
yum install -y docker git curl wget

# Start Docker
systemctl start docker
systemctl enable docker
usermod -aG docker ec2-user

# Install Docker Compose
curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose

# Wait for docker to be ready
sleep 5

# Create app directory
mkdir -p /opt/atlaspay
cd /opt/atlaspay

# Clone repository
git clone https://github.com/Falcon-J/AtlasPay.git .
cd /opt/atlaspay

# Create environment file
cat > .env.production << 'EOF'
# Server
SERVER_PORT=8080

# Database (RDS)
DB_HOST=${db_host}
DB_PORT=5432
DB_USER=${db_user}
DB_PASSWORD=${db_password}
DB_NAME=${db_name}
DB_SSLMODE=require

# Redis (self-hosted in container)
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# Kafka (self-hosted in container)
KAFKA_BROKERS=kafka:9092

# JWT
JWT_ACCESS_SECRET=$(openssl rand -base64 32)
JWT_REFRESH_SECRET=$(openssl rand -base64 32)

# Timeouts
SERVER_READ_TIMEOUT=15s
SERVER_WRITE_TIMEOUT=15s
EOF

# Create docker-compose for EC2 (only app services, RDS is managed)
cat > docker-compose.ec2.yml << 'EOF'
version: '3.8'

services:
  api-gateway:
    build: .
    container_name: atlaspay-api-gateway
    ports:
      - "8080:8080"
    environment:
      - SERVER_PORT=8080
      - DB_HOST=${RDS_HOST}
      - DB_PORT=5432
      - DB_USER=${DB_USER}
      - DB_PASSWORD=${DB_PASSWORD}
      - DB_NAME=${DB_NAME}
      - DB_SSLMODE=require
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - KAFKA_BROKERS=kafka:9092
    depends_on:
      - redis
      - kafka
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  redis:
    image: redis:7-alpine
    container_name: atlaspay-redis
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

  kafka:
    image: confluentinc/cp-kafka:7.5.0
    container_name: atlaspay-kafka
    ports:
      - "9092:9092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:29092,PLAINTEXT_HOST://kafka:9092
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
    depends_on:
      - zookeeper
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "kafka-broker-api-versions", "--bootstrap-server=localhost:9092"]
      interval: 10s
      timeout: 5s
      retries: 3

  zookeeper:
    image: confluentinc/cp-zookeeper:7.5.0
    container_name: atlaspay-zookeeper
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
    restart: unless-stopped

volumes:
  redis_data:
EOF

# Set permissions
chown -R ec2-user:ec2-user /opt/atlaspay
chmod +x /opt/atlaspay/.env.production

# Start services
cd /opt/atlaspay
/usr/local/bin/docker-compose -f docker-compose.ec2.yml up -d

# Wait for services to be healthy
echo "=== Waiting for services to be healthy ==="
sleep 30

# Check health
if curl -f http://localhost:8080/health; then
    echo "✅ AtlasPay is healthy!"
else
    echo "⚠️ Health check failed, services may still be starting..."
fi

# Setup CloudWatch logs (optional)
yum install -y amazon-cloudwatch-agent

echo "=== AtlasPay EC2 Setup Complete ==="
echo "API Gateway: http://$(hostname -f):8080"
