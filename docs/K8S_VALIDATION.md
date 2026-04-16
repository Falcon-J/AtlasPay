# AtlasPay Kubernetes Validation

No Kubernetes validation output is committed yet.

Use this file to record the exact commands and output used to validate Kubernetes deployment, HPA, and monitoring.

## Dry Run

```powershell
kubectl apply --dry-run=client -f deployments/kubernetes/infrastructure.yaml
kubectl apply --dry-run=client -f deployments/kubernetes/api-gateway.yaml
```

## Minikube Validation

```powershell
minikube start --cpus=4 --memory=8192
kubectl apply -f deployments/kubernetes/infrastructure.yaml
kubectl apply -f deployments/kubernetes/api-gateway.yaml
kubectl get pods
kubectl get svc
kubectl get hpa
kubectl describe hpa api-gateway-hpa
```

## Result Template

```text
Date:
Cluster:
Image:

kubectl get pods:

kubectl get svc:

kubectl get hpa:

Prometheus target status:

Notes:
```

Do not claim sustained uptime until a deployed environment has historical monitoring data.
