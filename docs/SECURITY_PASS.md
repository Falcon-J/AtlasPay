# Baseline and security pass

## Public contracts

- Payment initiation belongs to `POST /api/orders` and the checkout saga.
  Authenticated `POST /api/payments` returns 405 without processing a charge.
  Payment reads retain owner/admin authorization; refunds remain admin only.
- Inventory reads require authentication. Reserve, release, and commit are
  private service operations; their former public routes are removed.
  Public restock requires an admin token.
- Order and saga reads use the same owner/admin policy, including when the
  gateway calls the standalone Order service.

## Configuration

Only `APP_ENV=dev` enables development exceptions. An absent or unknown value
uses secure validation. The gateway requires distinct, non-default JWT signing
secrets of at least 32 bytes outside development. Internal listeners require
their service token; configured outbound service URLs require matching tokens.
Health and metrics exceptions remain explicit. Never publish internal service
ports or place their tokens in browser code.

`CORS_ALLOWED_ORIGINS` is a comma-separated list of exact HTTP/HTTPS origins,
without paths, query strings, fragments, or credentials. Wildcards are rejected
outside development. An empty list permits same-origin use only. Behind a proxy,
configure the public origin explicitly rather than trusting forwarded headers.
CORS is not an authorization mechanism.

Compose is explicitly development-only. Hosted configuration examples require
operator-supplied secrets; this pass does not deploy them.

## Demo and fixtures

The public static demo remains a browser simulation. Backend mode uses
operator-seeded `LAPTOP-001` and `FAIL-PAYMENT-001`, reads stock, and disables
restocking for its ordinary user. It does not automatically seed inventory.
Its animation is not evidence of durable backend completion.

Run smoke scripts from the local Compose checkout. `scripts/smoke-fixtures.ps1`
adds local database stock without resetting reservations or existing prices.
The demo smoke replays the existing saga payment over the private contract;
it does not create a separate public charge. Load scripts require an operator
`ADMIN_TOKEN` for seeding and use a separate ordinary token for checkout.
Historical evidence that exercised public payment creation describes the old
contract and is not validation of the current API.

## Evidence boundaries

Baseline HEAD: `6b941c888dfa56f7673e15ed43facfa565c0a0ff`, branch `main`.
The ten pre-existing modified status entries had no content diff and were
preserved. Git inspection disabled fsmonitor per command to avoid its daemon
error; repository configuration was not changed.

The [baseline CI run](https://github.com/Falcon-J/AtlasPay/actions/runs/33918190452)
passed build-and-test and failed Compose at the payment contract smoke step.
Local checks of this patch are separate from that baseline remote CI result.

Deferred: payment replay state semantics, reconciliation, cancellation races,
refresh rotation, money migration, and hosted deployment verification.

## Local verification results

- Committed baseline suite passed from an isolated HEAD archive.
- Authorization regressions reproduced unauthorized mutations, direct payment
  creation, and cross-user saga access before the fixes; focused checks passed
  afterward. Full formatting, vet, Go tests, and Linux race checks passed.
- Fresh PostgreSQL migrations/idempotency and saga lease integration passed.
- Real local and remote adapter security smokes passed, including two ordinary
  users, a dedicated local admin fixture, unchanged denied inventory, owned
  checkout, and saga access. All internal listeners rejected missing/wrong
  tokens; all four binaries rejected unsafe production startup.
- Checkout success, failure compensation, internal payment replay, duplicate
  delivery, DLQ continuation, persisted saga restart, and process crash replay
  passed. Crash replay converged to one reservation and one payment.
- The initial outbox recovery run failed: a retained, acknowledged event left
  order `2cbd92f3-356b-42ae-a723-273ad1037961` pending, without a saga claim,
  payment, or reservation. Its Kafka partition had no committed group offset.
  A real isolated Kafka regression reproduced the skip with `LastOffset`.
  Changing the fallback to `FirstOffset` passed both prepublished-event and
  committed-offset restart checks, including race detection. The original order
  then recovered normally on Order/Saga restart: confirmed order, completed
  saga, one completed payment, and one committed reservation. No offset reset,
  event republish, or manual financial state update was used.
- After the offset fix, outbox recovery, duplicate delivery, DLQ continuation,
  saga restart, and crash replay all passed. Duplicate checks now wait for both
  replay event IDs to be acknowledged before asserting one payment, one
  reservation, and a confirmed order. The Kafka regression runs in Compose CI.
- Fixture self-checks, HTTP status assertions, PowerShell parsing, browser-mode
  contract checks, and diff whitespace checks passed. Expected early missing
  saga/claim responses exposed two old polling helper errors; those helpers
  now preserve response diagnostics and tolerate an absent claim while polling.
  An explicit absent-claim guard and its standalone self-check cover native
  command empty output without relying on null-to-string casting.

No commit, push, or deployment was performed. Remote CI and hosted behavior have
not been validated for this uncommitted patch.
