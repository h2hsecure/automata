# Design Spec: Webhook Delivery Worker — Production Wiring and Retry Fix

**Date:** 2026-10-14
**Status:** Approved
**Reference:** [Webhook Management Plan](../plans/2026-09-25-webhook-management-plan.md), [Roadmap § Webhook Management](../../specs/roadmap.md)
**Module:** `github.com/hekemen/automata`

---

## 1. Problem Statement

Webhook delivery was shipped on 2026-09-25 (CRUD, queue, worker, retries), but **delivery is dead code in production**:

1. **The worker is never started.** `queue.NewWebhookWorker` (`internal/infrastructure/queue/webhook_worker.go:29`) has zero production callers. `cmd/automata/main.go` wires the email worker but never instantiates or starts the webhook worker.
2. **Jobs are enqueued in production.** `internal/adapter/api/server.go:128` wires `webhook_svc` with `queue.NewWebhookQueue(pool)`; form submissions and the webhook test endpoint call `Enqueue`. Rows accumulate in `webhook_deliveries` and are never delivered.
3. **The retry loop is broken even where it is exercised.** `WebhookQueue.UpdateNextRetry` (`internal/infrastructure/queue/webhook_queue.go:152`) sets `status = 'retrying'`, but `GetPending` (`webhook_queue.go:79`) selects only `WHERE status = 'pending'`. A failed delivery is therefore never re-fetched: it is scheduled for retry and then silently abandoned. The existing partial index `idx_webhooks_status` (`webhook_migration.sql:16`) already anticipates `status IN ('pending', 'retrying')`, confirming the original intent.

Result: every webhook delivery enqueued through form submission or the test endpoint sits in `webhook_deliveries` forever, and deliveries that do fail are never retried.

## 2. Goals & Non-Goals

### Goals

- Start the webhook delivery worker in the main API process so enqueued deliveries are actually POSTed.
- Fix `GetPending` so `retrying` rows are picked up once `next_retry` elapses (repair the retry loop).
- Prove the worker end-to-end with cicd integration specs: success, retry scheduling (HTTP 5xx and unreachable host), and terminal failure after max retries.
- Correct the roadmap, which currently overstates the delivery status.

### Non-Goals

- No row claim/lock in `GetPending` (multi-replica double-delivery protection) — see §6.5.
- No change to the webhook service, endpoints CRUD, or signature verification.
- No change to the backoff schedule (exponential: `3^attempts` minutes) or `max_retries` default (3).
- No worker configuration keys (poll interval, batch size stay hardcoded; a test seam covers the interval).

## 3. Architecture

### 3.1 Delivery Flow (after this change)

```
form submission / webhook test endpoint
        │
        ▼
webhook_svc ──► queue.NewWebhookQueue(pool).Enqueue(url, payload, ctxID, formID)
                                    │
                                    ▼
                            webhook_deliveries (status='pending')
                                    │
        ┌───────────────────────────┘
        ▼
main.go: go queue.NewWebhookWorker(webhookQueue).Start(ctx)
        │  (every 10s: GetPending → POST with X-Webhook-Source / X-Tenant-ID / X-Form-ID)
        ├─ 2xx ─────────────────────────► MarkSuccess (status='success')
        ├─ non-2xx / network error ──┬──► attempts < max_retries:
        │                            │        UpdateNextRetry(now + 3^attempts min, status='retrying')
        │                            │        └─ picked up again by GetPending once next_retry elapses
        │                            └──► attempts >= max_retries:
        │                                         MarkFailed (status='failed', error_msg set)
        └─ ctx.Done() → worker stops
```

### 3.2 Component Changes

| File | Change |
|------|--------|
| `cmd/automata/main.go` | Start the webhook worker goroutine next to the email worker |
| `internal/infrastructure/queue/webhook_queue.go` | `GetPending`: `status IN ('pending', 'retrying')` |
| `internal/infrastructure/queue/webhook_worker.go` | Add `NewWebhookWorkerWithInterval` test seam; `NewWebhookWorker` delegates to it |
| `cicd/support/webhook_mock.go` | Add `SetResponseStatusCode(code int)` (default 200) to the mock server |
| `cicd/webhook_worker_test.go` | New: worker-driven integration specs |
| `cicd/webhook_queue_test.go` | New spec: `GetPending` returns `retrying` rows whose `next_retry` has passed |
| `specs/roadmap.md` | Correct the Webhook Management status line |

### 3.3 Interface Design

No new production interfaces. Two additions:

```go
// internal/infrastructure/queue/webhook_worker.go

// NewWebhookWorkerWithInterval creates a worker with a custom poll interval.
// It is a test seam; production uses NewWebhookWorker.
func NewWebhookWorkerWithInterval(queue WebhookQueue, pollInterval time.Duration) *WebhookWorker

// NewWebhookWorker creates a new webhook worker (10s poll interval).
func NewWebhookWorker(queue WebhookQueue) *WebhookWorker // delegates to WithInterval
```

```go
// cicd/support/webhook_mock.go

// SetResponseStatusCode sets the status code the mock returns for captured
// requests. The default is 200.
func (mws *MockWebhookServer) SetResponseStatusCode(code int)
```

## 4. Data Model

No schema changes. `webhook_deliveries` already has the statuses (`pending`, `retrying`, `success`, `failed`), `attempts`, `max_retries`, `next_retry`, and `error_msg` columns, with no CHECK constraint on `status`. The existing partial index `idx_webhooks_status ON webhook_deliveries(status) WHERE status IN ('pending', 'retrying')` will serve the corrected `GetPending` query.

## 5. Config Contract

No new config keys. The worker uses the same `pgxpool.Pool` as the rest of the process and runs for the lifetime of the process's root context (same lifecycle as the email worker).

## 6. Key Design Decisions

### 6.1 Wire the worker in `main.go`, not in `NewServer`

Mirrors the email worker decision (email spec §6.6): `NewServer` does not take a context, and worker lifecycles belong in the process bootstrap. The server's `webhook_svc` keeps its own `queue.NewWebhookQueue(pool)` instance; the worker in `main.go` creates a second instance. The queue is a stateless struct over the pool, so this is harmless.

### 6.2 Fix `GetPending` rather than `UpdateNextRetry`

The `retrying` status is the correct representation of "failed, scheduled for retry" (it is distinct from `pending` = "never attempted", and the partial index already assumes it). The defect is that `GetPending` ignores it. One-line SQL fix:

```sql
WHERE status IN ('pending', 'retrying') AND next_retry <= NOW()
```

### 6.3 Test seam for the poll interval

The worker's 10s poll interval would make every cicd spec wait 10–20s and sit close to Ginkgo's 30s default spec timeout. `NewWebhookWorkerWithInterval` lets specs run the worker at 1s, keeping waits under 10s with comfortable margin. Production code is unchanged (`NewWebhookWorker` → 10s). Same rationale as the `NewMailerWithDialer` seam in the email work.

### 6.4 Mock server failure mode

`support.MockWebhookServer` always returned 200. `SetResponseStatusCode` (default 200) lets specs exercise the non-2xx path without a second server. Existing specs are unaffected (default behavior unchanged).

### 6.5 No row claim/lock — documented limitation

`GetPending` does not claim or lock rows, so two or more API replicas could double-deliver. This is the same class of limitation as the email worker's `FOR UPDATE SKIP LOCKED` note and is accepted for single-process deployments. It is documented in the plan's Review Focus, not fixed here.

## 7. Implementation Notes

- Worker goroutine in `main.go`: `go queue.NewWebhookWorker(webhookQueue).Start(ctx)` — `Start` is a blocking loop (unlike the email worker's internal goroutine), so the `go` is required.
- Worker specs must stop the worker deterministically: wrap `Start` in a goroutine that closes a `done` channel; `AfterEach` cancels the context and waits on `done` before cleaning up `webhook_deliveries`.
- Terminal-failure spec pre-seeds a row with `attempts = max_retries - 1` (2 of 3) via raw SQL, so a single failed delivery is terminal — avoids waiting out the 3-minute first backoff.
- Backoff note: after the first failure, `next_retry = now + 3^1 min`; specs assert the `retrying` state and do not wait for the actual retry.

## 8. Open Questions

None.

## 9. Security Considerations

- The worker POSTs to URLs stored in `webhook_deliveries` (user-supplied, same as before this change — no new attack surface).
- No credentials or sensitive headers are added; delivery headers remain `X-Webhook-Source`, `X-Tenant-ID`, `X-Form-ID`.
- No TLS/SSRF policy change is in scope.