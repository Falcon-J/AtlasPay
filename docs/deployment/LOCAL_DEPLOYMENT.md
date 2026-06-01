# AtlasPay - 100% FREE Deployment Options

## 🎯 Best Strategy for Portfolio Validation (No Payment Required)

Use these options to validate the project locally or on free tiers and capture evidence that the system works.

---

## Option 1: Local Kubernetes Demo (RECOMMENDED)

**Best for**: Screenshots, recordings, and technical demos

### Setup minikube (Free, Local K8s)

```powershell
# Install minikube (one-time)
winget install minikube

# Start cluster
minikube start --cpus=2 --memory=4096

# Enable addons
minikube addons enable metrics-server
minikube addons enable ingress

# Deploy AtlasPay
kubectl apply -f deployments/kubernetes/infrastructure.yaml
kubectl apply -f deployments/kubernetes/api-gateway.yaml

# Get URL
minikube service api-gateway --url
```

### What You Get:
- ✅ Real Kubernetes cluster
- ✅ HPA autoscaling demos
- ✅ kubectl commands work
- ✅ Perfect for screenshots
- ✅ No internet needed

### Screenshots for Resume:
```bash
# These commands give you useful validation screenshots
kubectl get pods -o wide
kubectl get hpa
kubectl top pods
kubectl describe deployment api-gateway
```

---

## Option 2: Render (Truly Free Tier)

**Best for**: Live URL to share

**Limits**: 750 hours/month, sleeps after 15 min inactivity

### Steps:
1. Go to [render.com](https://render.com)
2. Sign up with GitHub (no credit card)
3. Create New → Web Service
4. Connect your GitHub repo
5. Use these settings:
   - **Runtime**: Docker
   - **Instance Type**: Free
   - **Health Check**: /health

### render.yaml (add to repo root):
```yaml
services:
  - type: web
    name: atlaspay-api
    runtime: docker
    plan: free
    healthCheckPath: /health
    envVars:
      - key: DB_HOST
        value: your-postgres-host
      - key: REDIS_HOST
        value: your-redis-host
```

---

## Option 3: Railway (Free $5/month)

**Best for**: Postgres + Redis included free

### Steps:
1. Go to [railway.app](https://railway.app)
2. Sign up with GitHub
3. New Project → Deploy from GitHub
4. Add PostgreSQL service (free)
5. Add Redis service (free)
6. Deploy API

**Free tier includes**:
- $5/month credits (covers small apps)
- PostgreSQL database
- Redis cache
- Custom domains

---

## Option 4: Koyeb (No Credit Card)

**Best for**: Docker deployment, always-on

### Steps:
1. Go to [koyeb.com](https://koyeb.com)
2. Sign up (no credit card needed)
3. Deploy from Docker Hub or GitHub
4. Free tier: 1 web service, 512MB RAM

---

## Option 5: Grafana Cloud (Free Monitoring)

**Best for**: Real observability dashboards

### Steps:
1. Go to [grafana.com/cloud](https://grafana.com/cloud)
2. Sign up for free tier
3. Connect your Prometheus metrics
4. Import our dashboard

**Free includes**:
- 10k metrics
- 50GB logs
- 50GB traces
- Forever free

---

## 📸 What to Screenshot for Project Notes

### Must-Have Screenshots:

1. **Local K8s Running**
```bash
kubectl get pods
# Shows: api-gateway-xxx Running
```

2. **HPA Autoscaling**
```bash
kubectl get hpa
# Shows: Targets CPU 45%/70%
```

3. **Load Test Results**
```bash
k6 run scripts/k6/load-test.js
# Shows: total requests, p95 latency, and error rate
```

4. **Grafana Dashboard**
- p95 latency graph
- Request rate
- Error rate

5. **Chaos Test Report**
```bash
bash chaos/run-tests.sh
# Shows: System survived failures
```

---

## 🎬 Recording a Demo (Free)

### Option A: OBS Studio (Free)
1. Install [OBS Studio](https://obsproject.com)
2. Record screencast of:
   - Starting minikube
   - Deploying with kubectl
   - Running load test
   - Showing Grafana

### Option B: Loom (Free Tier)
1. Sign up at [loom.com](https://loom.com)
2. Record 5-min demo
3. Add to GitHub README

---

## 📋 Demo-Ready Without Cloud

You can demonstrate ALL of these locally:

| Claim | How to Prove |
|-------|--------------|
| "Handles sustained load" | k6 load test output |
| "Kubernetes deployment" | minikube + kubectl screenshots |
| "Autoscaling" | HPA screenshot showing scale-up |
| "Saga transactions" | Code walkthrough + logs |
| "Chaos tested" | chaos-report.md from script |
| "Grafana dashboards" | Local Grafana screenshot |

---

## 🚀 Quick Start (Completely Free)

```powershell
# 1. Start everything locally
docker-compose up -d

# 2. Run API
go run cmd/api-gateway/main.go

# 3. Run load test
k6 run scripts/k6/load-test.js

# 4. Open Grafana
# http://localhost:3000 (admin/admin123)

# 5. Take screenshots!
```

---

## FAQ

**Q: Do I need cloud to validate this project?**
A: No. A well-documented local project with screenshots and repeatable commands is enough for most technical reviews.

**Q: What if someone asks whether it is deployed?**
A: The repository includes Kubernetes manifests and deployment scripts. During development, it can run locally with minikube and can be deployed to a cloud provider with the same containerized API.

**Q: Free trials require credit card?**
A: Render, Railway, Koyeb, and Grafana Cloud all work without credit cards.
