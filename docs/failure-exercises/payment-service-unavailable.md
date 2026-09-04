# Failure exercise: payment service unavailable

## Goal

Demonstrate that the order saga records a failed payment step and performs
compensation without claiming a successful checkout.

## Procedure

1. Start the local stack and run `scripts/demo-smoke.ps1` with its failure SKU
   (`FAIL-PAYMENT-001`).
2. Capture `docker compose ps`, the API response, saga state, and relevant
   service logs.
3. Confirm the failed order reaches a compensated/failed terminal state and
   inventory is not left reserved.
4. Restore the payment process and rerun the normal smoke workflow.

This is a local controlled exercise. It is not production outage evidence.
