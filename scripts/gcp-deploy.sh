#!/bin/bash
# GCP Deployment Script for AtlasPay
# Run: bash scripts/gcp-deploy.sh YOUR_PROJECT_ID

set -e

PROJECT_ID=${1:-"your-project-id"}
REGION="asia-south1"
CLUSTER_NAME="atlaspay-cluster"
REPO_NAME="atlaspay"

echo "🚀 AtlasPay GCP Deployment Script"
echo "=================================="
echo "Project ID: $PROJECT_ID"
echo "Region: $REGION"
echo ""

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo "❌ gcloud CLI not found. Install from: https://cloud.google.com/sdk/docs/install"
    exit 1
fi

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl not found. Install from: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

# Set project
echo "📋 Setting project..."
gcloud config set project $PROJECT_ID

# Enable APIs
echo "🔧 Enabling required APIs..."
gcloud services enable container.googleapis.com --quiet
gcloud services enable artifactregistry.googleapis.com --quiet
gcloud services enable cloudbuild.googleapis.com --quiet

# Check if cluster exists
if gcloud container clusters describe $CLUSTER_NAME --region=$REGION &> /dev/null; then
    echo "✅ Cluster already exists"
else
    echo "🏗️ Creating GKE Autopilot cluster (this takes ~5 minutes)..."
    gcloud container clusters create-auto $CLUSTER_NAME \
        --region=$REGION \
        --project=$PROJECT_ID
fi

# Get credentials
echo "🔑 Getting cluster credentials..."
gcloud container clusters get-credentials $CLUSTER_NAME --region=$REGION

# Create Artifact Registry if not exists
if gcloud artifacts repositories describe $REPO_NAME --location=$REGION &> /dev/null; then
    echo "✅ Artifact Registry already exists"
else
    echo "📦 Creating Artifact Registry..."
    gcloud artifacts repositories create $REPO_NAME \
        --repository-format=docker \
        --location=$REGION \
        --description="AtlasPay container images"
fi

# Configure Docker
echo "🐳 Configuring Docker authentication..."
gcloud auth configure-docker ${REGION}-docker.pkg.dev --quiet

# Build and push image
IMAGE_URI="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO_NAME}/api-gateway:latest"
echo "🔨 Building Docker image..."
docker build -t $IMAGE_URI .

echo "📤 Pushing image to Artifact Registry..."
docker push $IMAGE_URI

# Update K8s manifests with correct image
echo "📝 Updating Kubernetes manifests..."
sed -i.bak "s|atlaspay/api-gateway:latest|${IMAGE_URI}|g" deployments/kubernetes/api-gateway.yaml

# Deploy infrastructure
echo "🏗️ Deploying infrastructure (Postgres, Redis)..."
kubectl apply -f deployments/kubernetes/infrastructure.yaml

# Wait for infrastructure
echo "⏳ Waiting for infrastructure to be ready..."
kubectl wait --for=condition=available deployment/postgres --timeout=300s || true
kubectl wait --for=condition=available deployment/redis --timeout=300s || true

# Deploy API Gateway
echo "🚀 Deploying API Gateway..."
kubectl apply -f deployments/kubernetes/api-gateway.yaml

# Wait for deployment
echo "⏳ Waiting for API Gateway to be ready..."
kubectl wait --for=condition=available deployment/api-gateway --timeout=300s

# Get external IP
echo ""
echo "✅ Deployment complete!"
echo ""
echo "Getting external IP (may take a minute)..."
kubectl get service api-gateway

echo ""
echo "📊 Cluster Status:"
kubectl get pods
kubectl get hpa

echo ""
echo "🎉 AtlasPay is deployed!"
echo "Run 'kubectl get service api-gateway' to get the external IP"
echo "Access: http://EXTERNAL_IP/health"
