# Webhook Delivery Worker — Implementation Plan

**Spec:** [2026-10-14-webhook-delivery-wiring.md](../specs/2026-10-14-webhook-delivery-wiring.md)
**Date:** 2026-10-14
**Status:** In Progress

Wire the existing webhook delivery worker into the main API process, fix the broken retry pickup in `GetPending`, and prove the worker end-to-end with cicd integration specs.

## Global Constraints

- Go 1.26.5, module `github.com/hekemen/automata`. No new dependencies.
- House test style: cicd = Ginkgo/Gomega, package `cicd_test`, run via `go test ./cicd/ -count=1 -args -ginkgo.focus="..."`.
- cicd baseline: 311 specs / 288 pass / 23 pre-existing failures (banner_campaign_http ×9, form_http ×9, tracking_http ×4, tracking_server ×1). New specs must pass; the baseline must not change.
- TDD gate per task: failing test first, then the minimal change, then commit.
- Do not touch the user's uncommitted files: `webui/`, `web/`, the 7 cicd test files + `cicd/creation_endpoints_test.go`, `AGENTS.md`, the `automata` binary, `helm/automata/values.yaml`, `.playwright-mcp/`.
- Commit per task on branch `phase2-auth-changes`.

## Task 1: Fix `GetPending` to pick up `retrying` rows (TDD)

**Problem:** `UpdateNextRetry` sets `status = 'retrying'`, but `GetPending` selects only `status = 'pending'` — failed deliveries are scheduled for retry and then never re-fetched.

**Files:**
- Modify: `cicd/webhook_queue_test.go` (add a spec to the "Webhook listing" describe block)
- Modify: `internal/infrastructure/queue/webhook_queue.go` (the `GetPending` SQL)

### Steps

- [x] **Step 1: Write the failing spec.** In `cicd/webhook_queue_test.go`, inside the existing "Webhook listing" `Describe` block, add:

```go
		It("should return retrying webhooks whose next_retry has passed", func() {
			contextID := support.NewTestContextID()
			formID := support.NewTestUUID()
			_, err := pool.Exec(ctx, `
				INSERT INTO webhook_deliveries (id, context_id, form_id, url, payload, status, attempts, max_retries, next_retry, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, 'retrying', 1, 3, NOW() - INTERVAL '1 second', NOW(), NOW())`,
				support.NewTestUUID(), contextID, formID, webhookServer.URL(), []byte(`{"test":"data"}`))
			Expect(err).NotTo(HaveOccurred())

			deliveries, err := queue.NewWebhookQueue(pool).GetPending(10)
			Expect(err).NotTo(HaveOccurred())
			Expect(deliveries).To(HaveLen(1))
			Expect(deliveries[0].Status).To(Equal("retrying"))
		})
```

- [x] **Step 2: Run the focused spec — it must FAIL** (GetPending returns 0 rows):

```bash
go test ./cicd/ -count=1 -args -ginkgo.focus="Webhook Queue Integration Tests"
```

- [x] **Step 3: Fix the SQL.** In `internal/infrastructure/queue/webhook_queue.go`, `GetPending` — change the `WHERE` clause from `status = 'pending'` to:

```sql
WHERE status IN ('pending', 'retrying') AND next_retry <= NOW()
```

- [x] **Step 4: Run the focused spec — it must PASS**, and the rest of the webhook queue specs must still pass:

```bash
go test ./cicd/ -count=1 -args -ginkgo.focus="Webhook Queue Integration Tests"
```

- [x] **Step 5: Commit.**

```bash
git add cicd/webhook_queue_test.go internal/infrastructure/queue/webhook_queue.go
git commit -m "fix(queue): GetPending picks up retrying webhook deliveries"
```

## Task 2: Worker integration specs + test seam (TDD)

**Problem:** The worker has never been run in any test. Prove it end-to-end: success path (payload + headers), retry scheduling (HTTP 5xx and unreachable host), and terminal failure after max retries.

**Files:**
- Create: `cicd/webhook_worker_test.go`
- Modify: `internal/infrastructure/queue/webhook_worker.go` (add `NewWebhookWorkerWithInterval`)
- Modify: `cicd/support/webhook_mock.go` (add `SetResponseStatusCode`)

### Steps

- [x] **Step 1: Write the failing specs.** Create `cicd/webhook_worker_test.go`:

```go
package cicd_test

import (
	"context"
	"net/http"
	"time"

	"github.com/hekemen/automata/cicd/support"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	whServer     *support.MockWebhookServer
	workerCtx    context.Context
	workerCancel context.CancelFunc
	workerDone   chan struct{}
)

var _ = BeforeEach(func() {
	whServer = support.NewMockWebhookServer()
	workerCtx, workerCancel = context.WithCancel(context.Background())
	workerDone = nil
})

var _ = Describe("Webhook Delivery Worker", func() {

	AfterEach(func() {
		if workerDone != nil {
			workerCancel()
			<-workerDone
		}
		_, err := pool.Exec(ctx, "DELETE FROM webhook_deliveries")
		Expect(err).NotTo(HaveOccurred())
		whServer.Clear()
	})

	// startWorker runs the worker at a 1s poll interval (test seam) and
	// records completion so AfterEach can stop it deterministically.
	startWorker := func() {
		workerDone = make(chan struct{})
		w := queue.NewWebhookWorkerWithInterval(queue.NewWebhookQueue(pool), time.Second)
		go func() {
			w.Start(workerCtx)
			close(workerDone)
		}()
	}

	It("delivers a queued webhook with the standard headers", func() {
		contextID := support.NewTestContextID()
		formID := support.NewTestUUID()
		payload := map[string]interface{}{"contact_email": "worker-test@example.com"}
		Expect(queue.NewWebhookQueue(pool).Enqueue(whServer.URL(), payload, contextID, formID)).To(Succeed())

		startWorker()

		var status string
		Eventually(func() string {
			err := pool.QueryRow(ctx, "SELECT status FROM webhook_deliveries WHERE context_id = $1", contextID).Scan(&status)
			Expect(err).NotTo(HaveOccurred())
			return status
		}, 10*time.Second, 200*time.Millisecond).Should(Equal("success"))

		Expect(whServer.Count()).To(Equal(1))
		req, ok := whServer.LastRequest()
		Expect(ok).To(BeTrue())
		Expect(req.Headers["X-Webhook-Source"]).To(Equal("automata"))
		Expect(req.Headers["X-Tenant-ID"]).To(Equal(contextID))
		Expect(req.Headers["X-Form-ID"]).To(Equal(formID))
		Expect(req.Payload["contact_email"]).To(Equal("worker-test@example.com"))
	})

	It("schedules a retry when the target returns 5xx or is unreachable", func() {
		contextID := support.NewTestContextID()
		formID := support.NewTestUUID()
		whServer.SetResponseStatusCode(http.StatusServiceUnavailable)
		Expect(queue.NewWebhookQueue(pool).Enqueue(whServer.URL(), map[string]interface{}{"t": 1}, contextID, formID)).To(Succeed())
		Expect(queue.NewWebhookQueue(pool).Enqueue("http://127.0.0.1:1", map[string]interface{}{"t": 2}, contextID, formID)).To(Succeed())

		startWorker()

		Eventually(func() int {
			var n int
			Expect(pool.QueryRow(ctx,
				"SELECT COUNT(*) FROM webhook_deliveries WHERE context_id = $1 AND status = 'retrying'",
				contextID).Scan(&n)).To(Succeed())
			return n
		}, 10*time.Second, 200*time.Millisecond).Should(Equal(2))

		var attempts int
		Expect(pool.QueryRow(ctx,
			"SELECT attempts FROM webhook_deliveries WHERE context_id = $1 AND url = $2",
			contextID, whServer.URL()).Scan(&attempts)).To(Succeed())
		Expect(attempts).To(Equal(1))
	})

	It("marks a delivery failed after max retries", func() {
		contextID := support.NewTestContextID()
		formID := support.NewTestUUID()
		whServer.SetResponseStatusCode(http.StatusServiceUnavailable)
		// Pre-seed at attempts = max_retries - 1 so the next failure is terminal.
		_, err := pool.Exec(ctx, `
			INSERT INTO webhook_deliveries (id, context_id, form_id, url, payload, status, attempts, max_retries, next_retry, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 'pending', 2, 3, NOW(), NOW(), NOW())`,
			support.NewTestUUID(), contextID, formID, whServer.URL(), []byte(`{"t":3}`))
		Expect(err).NotTo(HaveOccurred())

		startWorker()

		var status string
		Eventually(func() string {
			err := pool.QueryRow(ctx, "SELECT status FROM webhook_deliveries WHERE context_id = $1", contextID).Scan(&status)
			Expect(err).NotTo(HaveOccurred())
			return status
		}, 10*time.Second, 200*time.Millisecond).Should(Equal("failed"))

		var errMsg string
		Expect(pool.QueryRow(ctx, "SELECT error_msg FROM webhook_deliveries WHERE context_id = $1", contextID).Scan(&errMsg)).To(Succeed())
		Expect(errMsg).To(ContainSubstring("unexpected status: 503"))
	})
})
```

- [x] **Step 2: Run the focused specs — they must FAIL to compile** (`NewWebhookWorkerWithInterval` and `SetResponseStatusCode` undefined):

```bash
go test ./cicd/ -count=1 -args -ginkgo.focus="Webhook Delivery Worker"
```

- [x] **Step 3: Add the worker seam.** In `internal/infrastructure/queue/webhook_worker.go`, replace `NewWebhookWorker` with:

```go
// NewWebhookWorkerWithInterval creates a worker with a custom poll interval.
// It is a test seam; production uses NewWebhookWorker.
func NewWebhookWorkerWithInterval(queue WebhookQueue, pollInterval time.Duration) *WebhookWorker {
	return &WebhookWorker{
		queue:        queue,
		client:       &http.Client{Timeout: 30 * time.Second},
		pollInterval: pollInterval,
		batchSize:    10,
	}
}

// NewWebhookWorker creates a new webhook worker.
func NewWebhookWorker(queue WebhookQueue) *WebhookWorker {
	return NewWebhookWorkerWithInterval(queue, 10*time.Second)
}
```

- [x] **Step 4: Add the mock failure mode.** In `cicd/support/webhook_mock.go`:
  - Add a `statusCode int` field to `MockWebhookServer`; initialize it to `200` in `NewMockWebhookServer`.
  - In the handler, respond with `mws.statusCode` (read under the lock) instead of the hardcoded 200, and record that code in the captured `WebhookRequest.StatusCode`.
  - Add:

```go
// SetResponseStatusCode sets the status code the mock returns for captured
// requests. The default is 200.
func (mws *MockWebhookServer) SetResponseStatusCode(code int) {
	mws.mu.Lock()
	defer mws.mu.Unlock()
	mws.statusCode = code
}
```

- [x] **Step 5: Run the focused specs — they must PASS** (all 3 worker specs + the Task 1 spec):

```bash
go test ./cicd/ -count=1 -args -ginkgo.focus="Webhook Delivery Worker|Webhook Queue Integration Tests"
```

- [x] **Step 6: Commit.**

```bash
git add cicd/webhook_worker_test.go internal/infrastructure/queue/webhook_worker.go cicd/support/webhook_mock.go
git commit -m "test(queue): webhook worker integration specs + poll-interval seam"
```

## Task 3: Production wiring, roadmap fix, docs

**Files:**
- Modify: `cmd/automata/main.go` (start the worker next to the email worker)
- Modify: `specs/roadmap.md` (correct the Webhook Management line)
- Add: this spec + plan (created before execution; committed here with Execution Status)

### Steps

- [x] **Step 1: Wire the worker.** In `cmd/automata/main.go`, immediately after the email worker wiring (`emailQueue.StartWorker(ctx, mailer)`), add:

```go
	// Webhook delivery worker (API enqueues deliveries; worker POSTs them)
	webhookQueue := queue.NewWebhookQueue(pool)
	go queue.NewWebhookWorker(webhookQueue).Start(ctx)
```

(`Start` is a blocking loop, unlike the email worker's internal goroutine — the `go` is required. `queue` is already imported in `main.go` for the email queue.)

- [x] **Step 2: Build and vet.**

```bash
go build ./...
go vet ./cmd/... ./internal/infrastructure/queue/...
```

- [x] **Step 3: Correct the roadmap.** In `specs/roadmap.md`, the Webhook Management line — change the status to:

```
Per-context webhook CRUD, queue, delivery, and retry. **Done** (2026-09-25; delivery worker wired into the main API process and retry pickup fixed 2026-10-14, [plan](../docs/superpowers/plans/2026-10-14-webhook-delivery-wiring.md)).
```

- [x] **Step 4: Fill in the Execution Status section of this plan** (tasks, commits, verification results).

- [x] **Step 5: Commit.**

```bash
git add cmd/automata/main.go specs/roadmap.md docs/superpowers/specs/2026-10-14-webhook-delivery-wiring.md docs/superpowers/plans/2026-10-14-webhook-delivery-wiring.md
git commit -m "feat: start webhook delivery worker in main API process"
```

- [x] **Step 6: Full-suite verification.**

```bash
make test
```

Expected: the 4 new specs pass; the failure count stays at the 23 pre-existing baseline failures.

## Review Focus

Failure modes this spec implies but the tests do not cover:

1. **Unreachable target host (DNS failure / connection refused)** — expected: delivery goes to `retrying`, not stuck or crashed. Pinned by the Task 2 "unreachable" spec (closed port 127.0.0.1:1).
2. **Rows already in `retrying` state before this fix** (written by `UpdateNextRetry` while the old `GetPending` was live) — expected: picked up by the worker once `next_retry` elapses. Pinned by the Task 1 regression spec.
3. **Double delivery under 2+ API replicas** — known limitation (no row claim/lock in `GetPending`); documented in spec §6.5, not fixed.
4. **Worker goroutine lifecycle on shutdown** — expected: `ctx` cancel stops the loop, "webhook worker stopped" logged, no panic. Verified by code review only (main.go wiring is not unit-testable; same as the email worker).
5. **Mock default-200 behavior must not change** — existing `webhook_queue_test.go` specs depend on it; the Task 2 Step 5 run (both focus sets) pins it.

## File Summary

| File | Action |
|------|--------|
| `internal/infrastructure/queue/webhook_queue.go` | Modify — `GetPending` SQL |
| `internal/infrastructure/queue/webhook_worker.go` | Modify — `NewWebhookWorkerWithInterval` seam |
| `cicd/support/webhook_mock.go` | Modify — `SetResponseStatusCode` |
| `cicd/webhook_worker_test.go` | Create — 3 worker specs |
| `cicd/webhook_queue_test.go` | Modify — 1 regression spec |
| `cmd/automata/main.go` | Modify — worker wiring |
| `specs/roadmap.md` | Modify — status line |
| `docs/superpowers/specs/2026-10-14-webhook-delivery-wiring.md` | Create |
| `docs/superpowers/plans/2026-10-14-webhook-delivery-wiring.md` | Create (this file) |

## Execution Status

(To be filled in at completion.)