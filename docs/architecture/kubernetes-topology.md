# Kubernetes topology

The Kubernetes manifests mirror the local four-process deployment:

```text
LoadBalancer: api-gateway:80
        |
        +--> order-service:8083  (Order/Saga + Kafka consumers)
        +--> payment-service:8081 (private ClusterIP)
        +--> inventory-service:8082 (private ClusterIP)

All application processes --> PostgreSQL
Inventory and Order/Saga --> Redis
Order/Saga                --> Kafka
Prometheus                --> /metrics on all four processes
```

`deployments/kubernetes/api-gateway.yaml` is the public JWT boundary. It uses
the service URLs from `atlaspay-config` and internal tokens from
`atlaspay-secrets`; it does not run the Kafka order consumer when the standalone
Order/Saga deployment is enabled.

`deployments/kubernetes/application-services.yaml` keeps Payment, Inventory,
and Order/Saga behind `ClusterIP` services. The Order/Saga deployment uses two
replicas with eight Kafka workers per pod, giving sixteen worker slots across
the sixteen-partition local benchmark topology. This is a deployment shape,
not proof of multi-node failover or production capacity.

Each extracted process has startup, readiness, and liveness probes backed by
`/health`. Each also exposes `/metrics`; Prometheus targets the service DNS
names through `deployments/prometheus.yml`.

The current manifests intentionally share PostgreSQL and Redis while the
bounded-context ownership extraction remains in progress. Production rollout
still requires external secret management, image publication, database
migrations, Kafka topic provisioning, network policy, and a real failover/
rollback exercise.
