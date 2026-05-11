# AtlasPay Live Deployment

## 🌐 Live Environment

**Status:** ✅ Active  
**Platform:** AWS EC2 (Free Tier)  
**Instance:** t2.micro (1 vCPU, 1GB RAM)  
**Region:** US East (N. Virginia)  
**IP Address:** 52.23.219.80

## 📍 Access Points

| Service | URL | Purpose |
|---------|-----|---------|
| **Health Check** | http://52.23.219.80:8080/health | API readiness (JSON response) |
| **API Gateway** | http://52.23.219.80:8080 | Payment/order operations |
| **SSH Access** | `ssh -i <key> ubuntu@52.23.219.80` | Server management |

### Sample Health Response
```json
{
  "status": "healthy",
  "db": "up",
  "cache": "up"
}
```

## 🏗️ Deployed Services

```
┌────────────────────────────────────────────────────┐
│           Docker Compose (EC2 Ubuntu)              │
├────────────────────────────────────────────────────┤
│                                                    │
│  api-gateway:8080 ──────┐                        │
│  (Go 1.21)              │                        │
│                         ▼                        │
│  postgres:16-alpine ◄─────────────────────┐     │
│  (Port: 5432)                              │     │
│                                            │     │
│  redis:7-alpine ────────────────────────────┤   │
│  (Port: 6379)                              │    │
│                                            │    │
└────────────────────────────────────────────┘    │
   All services: ✅ Healthy & Responsive        │
```

### Service Details

| Service | Image | Port | Status | Purpose |
|---------|-------|------|--------|---------|
| **API Gateway** | atlaspay-api:latest | 8080 | Up ✅ | Request routing, auth, business logic |
| **PostgreSQL** | postgres:16-alpine | 5432 | Up ✅ | Persistent data (orders, payments, inventory) |
| **Redis** | redis:7-alpine | 6379 | Up ✅ | In-memory cache (orders, inventory) |

## 🚀 Deployment Architecture

### Technology Stack
- **Language:** Go 1.21
- **Container:** Docker & Docker Compose
- **Database:** PostgreSQL 16
- **Cache:** Redis 7
- **Infrastructure:** AWS EC2 (Ubuntu 22.04 LTS)

### Key Optimizations for Free Tier

1. **Memory Efficiency**
   - t2.micro: 1GB RAM (2GB swap added)
   - Pre-built Docker image to eliminate build overhead on EC2
   - No Kafka (graceful degradation for free tier)

2. **Reliability**
   - Exponential backoff retry logic (20 attempts, 2s→60s)
   - Service health checks via docker-compose
   - Automatic restart policy: `unless-stopped`

3. **Data Persistence**
   - PostgreSQL data stored on 30GB EBS volume
   - Redis data in-memory (ephemeral, OK for cache layer)

## 📊 Operations

### View Live Logs
```bash
# SSH into server
ssh -i <your-key> ubuntu@52.23.219.80

# View API logs (last 50 lines)
docker compose logs -f api --tail=50

# View all services
docker compose ps

# Health metrics
curl http://localhost:8080/health
```

### Service Management
```bash
# Start all services
docker compose up -d

# Stop all services
docker compose down

# Restart API
docker compose restart api

# View resource usage
docker stats
```

## 🔍 Monitoring & Debugging

### Health Checks
- **API Health:** GET `/health` → Returns service status
- **Database:** Connection tested on startup (20 retry attempts with exponential backoff)
- **Cache:** Redis connectivity validated at startup

### Common Issues & Resolution

| Issue | Cause | Resolution |
|-------|-------|-----------|
| API not responding | Container crashed | `docker compose logs api` to view errors |
| Database connection refused | PostgreSQL still booting | Wait 10-15s, API auto-retries with backoff |
| Out of memory errors | Swap exhausted | Monitor with `free -h`, may need to add more swap |
| Port 8080 already in use | Process binding | `sudo lsof -i :8080` to find process |

## 📝 Deployment Checklist

- [x] AWS EC2 instance provisioned (t2.micro)
- [x] Ubuntu 22.04 LTS installed
- [x] Docker & Docker Compose installed
- [x] 2GB swap memory added
- [x] PostgreSQL 16 configured
- [x] Redis 7 configured
- [x] AtlasPay API built and tested locally
- [x] Docker image transferred to EC2
- [x] Services running and healthy
- [x] Health endpoint responding
- [x] Ports 8080 (API), 5432 (PostgreSQL), 6379 (Redis) accessible

## 🔐 Security Notes

- **Free Tier Warnings:**
  - No public SSL/TLS (HTTP only, suitable for demo)
  - Security group allows traffic (permissive for testing)
  - Credentials in environment variables (demo only, use secrets manager for production)

- **For Production:**
  - Enable AWS WAF
  - Configure ACM SSL certificates
  - Use RDS for PostgreSQL
  - Use ElastiCache for Redis
  - Use Secrets Manager for credentials
  - Enable VPC restrictions

## 📚 Related Documentation

- [AWS EC2 Step-by-Step Setup](./deployments/AWS_EC2_STEP_BY_STEP.md) - Detailed manual setup guide
- [Lean Setup for Free Tier](./deployments/AWS_EC2_LEAN_SETUP.md) - Memory-optimized deployment
- [Deployment Fixes](./DEPLOYMENT_FIXES.md) - Technical issues and solutions
- [Architecture Overview](./docs/CURRENT_STATE.md) - System design details

## 📞 Support

For deployment issues:
1. Check logs: `docker compose logs api`
2. Verify services: `docker compose ps`
3. Test connectivity: `curl http://52.23.219.80:8080/health`
4. Review troubleshooting section above

---

**Last Updated:** May 12, 2026  
**Interview Prep Status:** ✅ Complete  
**Live Demo Status:** ✅ Active
