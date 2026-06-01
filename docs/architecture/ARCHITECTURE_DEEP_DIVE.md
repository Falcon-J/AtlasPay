# AtlasPay: Architecture & DevOps Deep Dive
*(Interview Cheat Sheet for Harness)*

## 1. The Core Narrative
**"What is AtlasPay?"**
AtlasPay is a distributed order and payment platform designed to handle complex, multi-step transactions without relying on a monolithic database. 

**The Challenge I Solved:** 
In a microservices architecture, you can't use traditional database transactions (like a simple SQL `COMMIT` or `ROLLBACK`) across different services. If an order is created, but the payment fails, how do you reverse the inventory reservation?

**The Solution:**
I implemented the **Saga Pattern** using an Orchestration approach. The system uses a centralized state machine that dictates the flow of the transaction. If any step fails, the orchestrator triggers **Compensating Transactions** to undo the previous steps, ensuring eventual consistency.

## 2. Technical Component Breakdown

### A. The Saga Orchestrator (The Brain)
- **Role:** Manages the state machine (OrderCreated -> InventoryReserved -> PaymentProcessed -> OrderConfirmed).
- **Why Orchestration over Choreography?** In choreography, services listen to each other's events directly. This creates a "spaghetti" dependency graph which is hard to debug. Orchestration centralizes the logic, making distributed tracing (via Jaeger) much easier—a key requirement for modern DevOps platforms like Harness.

### B. Event-Driven Messaging (Kafka / Zookeeper)
- **Role:** Handles asynchronous communication. When an API request comes in, the Order Service publishes an `order.created` event to Kafka and immediately returns an HTTP 202 Accepted.
- **Why Kafka?** It acts as a buffer and a durable log. If the Payment service goes down, the message isn't lost; it waits in the Kafka topic until the service recovers. This provides **resilience**.

### C. Caching Strategy (Redis)
- **Role:** Cache-aside pattern for order status.
- **Why Redis?** Because the order creation is asynchronous, the frontend needs to poll for the order status. Hitting PostgreSQL every time would crash the DB under load. I introduced Redis so that 90% of status reads hit the in-memory cache, drastically improving P99 latency.

### D. Observability Stack (Prometheus / Grafana / Jaeger)
- **Why this matters for Harness:** Harness is about the "outer loop" of code delivery. You can't deploy confidently if you can't see what's happening. I integrated Prometheus for metrics (e.g., error rates, latency) and Jaeger for distributed tracing (attaching a `TraceID` to track a single request across Order, Inventory, and Payment services).

## 3. The DevOps Evolution (The "Shift to EC2" Story)

*This is a critical story for a DevOps/Adoption role. It shows you understand the entire software lifecycle.*

**Phase 1: Local Development (Docker Compose)**
Initially, I ran everything locally using `docker-compose`. It was great for fast iteration, but it didn't reflect production.

**Phase 2: Cloud Simulation (EC2 + Local Containers)**
I moved to AWS EC2 to simulate a cloud deployment. I used a `user-data.sh` script to automate the installation of Docker and clone the repo. However, I was running my PostgreSQL database inside a container on a single EC2 instance. 

**Phase 3: Production Readiness (Terraform + RDS)**
**The Fix:** I realized that running a stateful database in a transient container on a single EC2 instance is an anti-pattern for production. If the EC2 instance dies, the data could be lost. 
I used **Terraform (Infrastructure as Code)** to provision a managed **AWS RDS** instance for PostgreSQL. I updated my Terraform script to inject the RDS connection strings dynamically into my EC2 `user-data.sh` template, ensuring the API connects to the highly available RDS instance while the stateless app logic remains containerized.

## 4. Key Concepts to Master Before the Interview

*   **Idempotency:** The Payment service uses Idempotency Keys. If a network timeout occurs and a request is retried, the system knows not to charge the user twice.
*   **Eventual Consistency:** The system does not guarantee immediate consistency. It guarantees that, *eventually*, the system will settle into a correct state (either fully processed or fully reverted).
*   **Infrastructure as Code (IaC):** Using Terraform means the infrastructure is version-controlled, reproducible, and immune to manual "click-ops" errors. This is the foundation of GitOps.
