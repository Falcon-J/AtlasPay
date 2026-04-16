# 📖 AtlasPay: User Stories & Business Scenarios

These scenarios explain the business value and failure modes behind the AtlasPay implementation.

---

## 1. The Happy Path: "A Seamless Multi-Service Purchase"
**User Story**: As a customer, I want to buy a laptop so that I can complete checkout reliably.

1.  **Trigger**: User clicks "Buy Now" on the frontend.
2.  **Order Service**: Creates an order with status `PENDING`.
3.  **Inventory Service**: Checks stock. Laptop is available! It places a **Reservation** (decrementing available stock but keeping it in a "reserved" state).
4.  **Payment Service**: Processes the credit card. Success!
5.  **Completion**: Order Service updates status to `CONFIRMED`.
6.  **Outcome**: User is happy; data is consistent across all three services.

---

## 2. Inventory Failure: "The Graceful Out-of-Stock Sale"
**User Story**: As a customer, I want to buy a limited-edition sneaker, but someone beat me to it.

1.  **Trigger**: User clicks "Buy Now".
2.  **Order Service**: Creates order `PENDING`.
3.  **Inventory Service**: Checks stock. **0 items left**.
4.  **Saga Failure**: The Inventory service returns a "Stock Insufficient" error.
5.  **Compensation**: Since NO money was taken and NO stock was reserved, the system simply marks the order as `FAILED`.
6.  **Outcome**: User is informed immediately. No orphan reservations or incorrect charges exist.

---

## 3. Payment Failure: "The Complex Distributed Reversal"
**User Story**: As a customer, I have enough items in my cart, but my bank declined the transaction.

1.  **Trigger**: User clicks "Buy Now".
2.  **Order Service**: Creates order `PENDING`.
3.  **Inventory Service**: Success! SKU `LE-SNEAKER-01` is reserved.
4.  **Payment Service**: Attempts to charge the card. **Declined (Insufficient Funds)**.
5.  **Saga Compensation (CRITICAL)**:
    - The Saga Orchestrator sees the Payment failure.
    - It sends a **Compensating Command** back to the Inventory Service.
    - **Inventory Service**: Releases the reservation for SKU `LE-SNEAKER-01`, making it available for other customers again.
6.  **Outcome**: The order is marked `FAILED`. Data consistency is perfectly maintained—we didn't "lose" a sneaker in a ghost reservation.

---

## 💡 Why This Matters
Scenario #3 is the key consistency case. In a distributed checkout flow, a single database transaction is not enough once inventory and payment become independent boundaries, so every successful action needs a clear compensating action.
