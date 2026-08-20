# AtlasPay Kubernetes Validation

No Kubernetes validation output is committed yet.

Use this file to record the exact commands and output used to validate Kubernetes deployment, HPA, and monitoring.

## Dry Run

```powershell
kubectl apply --dry-run=client -f deployments/kubernetes/infrastructure.yaml
kubectl apply --dry-run=client -f deployments/kubernetes/api-gateway.yaml
kubectl apply --dry-run=client -f deployments/kubernetes/application-services.yaml
```

Apply the three manifests as one bundle. The gateway, Payment, Inventory, and
Order/Saga processes share the PostgreSQL and Redis instances defined in
`infrastructure.yaml`; the standalone Order/Saga process owns the Kafka order
workers. `deployments/prometheus.yml` contains scrape targets but does not
deploy Prometheus.

These manifests are a topology candidate, not production proof. Before a
release, replace placeholder credentials with external secret management,
publish immutable image references, run migrations and topic provisioning, and
exercise failover and rollback in a real cluster.

## Minikube Validation

```powershell
minikube start --cpus=4 --memory=8192
kubectl apply -f deployments/kubernetes/infrastructure.yaml
kubectl apply -f deployments/kubernetes/api-gateway.yaml
kubectl apply -f deployments/kubernetes/application-services.yaml
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
