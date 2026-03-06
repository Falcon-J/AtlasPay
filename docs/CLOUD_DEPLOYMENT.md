# AtlasPay Cloud Deployment Guide

## 🎯 Recommendation: Start with GCP (GKE)

**Why GCP over AWS for this project:**

| Factor | GCP (GKE) | AWS (EKS) |
|--------|-----------|-----------|
| **Control Plane Cost** | **FREE** ($74.40/mo credit) | $72/month |
| **Free Trial** | $300 credits for 90 days | $100 credits (6 months) |
| **Kubernetes Experience** | Better (invented K8s) | Good |
| **Setup Complexity** | Simpler | More setup |
| **Resume Line** | "Deployed on GKE" ✅ | "Deployed on EKS" ✅ |

> **For portfolio projects, GKE saves ~$72/month** while providing the same interview impact.

---

## 🚀 GCP Setup Guide

### Step 1: Create GCP Account

1. Go to [cloud.google.com](https://cloud.google.com)
2. Sign up with Gmail
3. You'll get **$300 free credit** for 90 days
4. No charge unless you upgrade

### Step 2: Install Tools

```powershell
# Install Google Cloud SDK
# Download from: https://cloud.google.com/sdk/docs/install

# After installation, authenticate:
gcloud auth login
gcloud config set project YOUR_PROJECT_ID
```

### Step 3: Enable Required APIs

```bash
gcloud services enable container.googleapis.com
gcloud services enable artifactregistry.googleapis.com
gcloud services enable cloudbuild.googleapis.com
```

### Step 4: Create GKE Cluster

```bash
# Create a cost-optimized Autopilot cluster (recommended)
gcloud container clusters create-auto atlaspay-cluster \
    --region=asia-south1 \
    --project=YOUR_PROJECT_ID

# OR create a Standard cluster for more control
gcloud container clusters create atlaspay-cluster \
    --zone=asia-south1-a \
    --num-nodes=2 \
    --machine-type=e2-medium \
    --enable-autoscaling \
    --min-nodes=1 \
    --max-nodes=5

# Get credentials
gcloud container clusters get-credentials atlaspay-cluster --region=asia-south1
```

### Step 5: Create Artifact Registry

```bash
# Create Docker repository
gcloud artifacts repositories create atlaspay \
    --repository-format=docker \
    --location=asia-south1 \
    --description="AtlasPay container images"

# Configure Docker auth
gcloud auth configure-docker asia-south1-docker.pkg.dev
```

### Step 6: Build and Push Images

```bash
# Build the image
docker build -t asia-south1-docker.pkg.dev/YOUR_PROJECT_ID/atlaspay/api-gateway:v1 .

# Push to registry
docker push asia-south1-docker.pkg.dev/YOUR_PROJECT_ID/atlaspay/api-gateway:v1
```

### Step 7: Deploy to GKE

```bash
# Update image in K8s manifests
kubectl apply -f deployments/kubernetes/infrastructure.yaml
kubectl apply -f deployments/kubernetes/api-gateway.yaml

# Check deployment
kubectl get pods
kubectl get services
```

### Step 8: Get External IP

```bash
# Get the LoadBalancer IP
kubectl get service api-gateway

# Access your API
curl http://EXTERNAL_IP/health
```

---

## 💰 Cost Optimization Tips

### 1. Use Autopilot Mode
- Pay per pod, not per node
- No charges for unused capacity
- Auto-scales to zero

### 2. Schedule Cluster Scale-Down
```bash
# Scale down when not demoing
kubectl scale deployment api-gateway --replicas=0

# Scale up for interviews
kubectl scale deployment api-gateway --replicas=2
```

### 3. Use Preemptible VMs (Standard Cluster)
```bash
# 70-80% cheaper, good for non-critical workloads
gcloud container node-pools create preemptible-pool \
    --cluster=atlaspay-cluster \
    --preemptible \
    --num-nodes=2
```

### 4. Set Budget Alerts
```bash
# Go to GCP Console → Billing → Budgets
# Set alert at $20 to avoid surprises
```

---

## 📊 Estimated Monthly Costs

### Minimal Setup (Demo Only)
| Resource | Cost |
|----------|------|
| GKE Control Plane | $0 (free credit) |
| 2x e2-small nodes | ~$15 |
| Cloud SQL (Postgres) | ~$10 |
| **Total** | **~$25/month** |

### Full Stack (Active Development)
| Resource | Cost |
|----------|------|
| GKE Control Plane | $0 (free credit) |
| 3x e2-medium nodes | ~$45 |
| Cloud SQL | ~$25 |
| Cloud Memorystore (Redis) | ~$15 |
| **Total** | **~$85/month** |

> **With $300 free credit, you can run full stack for 3+ months!**

---

## 🔄 Alternative: AWS EKS

If you prefer AWS for job requirements, here's the quick setup:

### Prerequisites
```bash
# Install AWS CLI
# Install eksctl: https://eksctl.io/installation/

aws configure
```

### Create EKS Cluster
```bash
eksctl create cluster \
    --name atlaspay-cluster \
    --region ap-south-1 \
    --nodegroup-name standard-workers \
    --node-type t3.medium \
    --nodes 2 \
    --nodes-min 1 \
    --nodes-max 5 \
    --managed
```

### Deploy
```bash
# Build and push to ECR
aws ecr get-login-password --region ap-south-1 | docker login --username AWS --password-stdin YOUR_ACCOUNT_ID.dkr.ecr.ap-south-1.amazonaws.com

docker build -t atlaspay/api-gateway .
docker tag atlaspay/api-gateway:latest YOUR_ACCOUNT_ID.dkr.ecr.ap-south-1.amazonaws.com/atlaspay:latest
docker push YOUR_ACCOUNT_ID.dkr.ecr.ap-south-1.amazonaws.com/atlaspay:latest

kubectl apply -f deployments/kubernetes/
```

---

## 📸 What to Screenshot for Resume

1. **GKE Console** showing cluster with nodes
2. **Grafana Dashboard** with metrics graphs
3. **kubectl get pods** showing 3+ replicas
4. **HPA status** showing autoscaling working
5. **k6 results** showing 10k RPS

Add these to your README for maximum impact!

---

## ❓ FAQ

**Q: Can I run this completely free?**
A: Yes! With GCP's $300 credit and the free control plane, you can run for 3-4 months without paying.

**Q: Which region should I use?**
A: `asia-south1` (Mumbai) for lowest latency in India.

**Q: How do I avoid unexpected charges?**
A: Set budget alerts at $10 and $25. Delete cluster when not using:
```bash
gcloud container clusters delete atlaspay-cluster --region=asia-south1
```

**Q: AWS or GCP for interviews?**
A: Both are equally valued. Pick based on job listings you're targeting.
