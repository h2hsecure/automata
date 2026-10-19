# Per-Website SMTP Settings + Per-Website Email Queue

**Status:** approved (user: "go for implementation" — recommended options: queue visibility+actions; SMTP UI in Settings → Websites tab)
**Date:** 2026-10-19
**Branch:** `phase2-auth-changes` (deploy from committed HEAD only)

## Goal

Each website (context) can have its own SMTP relay settings (empty = inherit global), and each website's email queue is visible and manageable per website (pending/sent/failed, retry failed, cancel pending, test send).

## Architecture Decisions

1. **History table, not a status column.** Existing tests (committed `cicd/email_delivery_test.go` `waitQueueDrained`, WIP `cicd/email_queue_test.go`) assert `email_jobs` drains to 0 after processing. So `email_jobs` stays the active queue (rows deleted on success and on max-retries), and a new `email_history` table records outcomes:
   - success → INSERT history (`status='sent'`) + DELETE job
   - max retries → INSERT history (`status='failed'`, `last_error`) + DELETE job
   - UI "queue" = pending (`email_jobs`) + history (`email_history`, 30-day retention, pruned hourly by the worker).
2. **Generic context-scoped key-value settings** (`context_settings`), mirroring `global_settings`. SMTP is the only group the UI shows; resolution order at send time: **config.yaml → global settings → context settings** (two `MergeConfig` passes).
3. **Context-aware mailer**: `queue.Mailer.Send(contextID string, to []string, subject, body, htmlBody string) error`; `smtp.ConfigProvider func(contextID string) Config`. `SMTPMailer` resolves config per send per context.
4. **Per-context starvation guard**: worker batch fetch caps 3 jobs per context (`ROW_NUMBER() OVER (PARTITION BY context_id ...)`), batch size 10.
5. **APIs are admin-scoped** (mirror `admin.Group("/contexts/:id/users")`):
   - `GET/PUT /api/admin/contexts/:id/settings`, `DELETE /api/admin/contexts/:id/settings/:key`
   - `GET /api/admin/contexts/:id/email-queue?status=pending|sent|failed&limit=100`
   - `POST /api/admin/contexts/:id/email-queue/:jobId/retry`
   - `DELETE /api/admin/contexts/:id/email-queue/:jobId` (cancel pending)
   - `POST /api/admin/contexts/:id/email-queue/test` body `{to}`
6. **UI**: new `WebsiteEmailPanel.vue` modal opened from an "Email" action button on each row of `WebsitesPanel.vue` (Settings → Websites tab). SMTP form reuses the `smtp` group from `settingsCatalog.ts`.

## Hard Constraints

- **WIP files must stay untouched and uncommitted:** `AGENTS.md`, `automata` (binary), `helm/automata/values.yaml`, `docs/phase1.md`, `.playwright-mcp/`, and cicd: `config_handler_test.go`, `contact_form_integration_test.go`, `email_queue_test.go`, `global_login_test.go`, `snippet_test.go`, `tracking_server_test.go`, `webhook_queue_test.go`. All new tests go in **new files**.
- `cicd/smtp_settings_test.go`, `cicd/email_delivery_test.go`, `cicd/support/*` are committed files — **modifying them is allowed** (required by the Send signature change).
- No image-based verification (text/DOM/HTTP only).
- `vite build` is the UI gate (project does not pass clean `vue-tsc --noEmit`); UI commits include rebuilt `web/` assets.
- Commit per task; include only that task's files (never WIP files).
- Pre-existing baseline issues (not regressions): vet failure `proxy_test.go:23`; gofmt drift in 40+ files.

## Files

### New
- `internal/infrastructure/settings/repo/context.go` — context settings repo
- `internal/adapter/api/handler/context_settings_handler.go`
- `internal/adapter/api/handler/email_queue_handler.go`
- `cicd/context_settings_test.go`
- `cicd/email_queue_context_test.go`
- `cicd/email_queue_api_test.go`
- `webui/src/components/settings/WebsiteEmailPanel.vue`

### Modified
- `internal/domain/settings/settings.go` — add `ContextRepository` port
- `internal/infrastructure/settings/repo/migration.sql` — `context_settings` table
- `internal/infrastructure/queue/migration.sql` — `email_history` table
- `internal/infrastructure/queue/email_queue.go` — interface methods
- `internal/infrastructure/queue/email_worker.go` — history writes, isolation query, pruning
- `internal/infrastructure/email/smtp/smtp.go` — context-aware `Send`/`ConfigProvider`
- `cmd/automata/main.go` — provider closure with context settings
- `internal/adapter/api/server.go` — new routes
- `cicd/support/smtp_mock.go` — `Send(contextID, ...)`
- `cicd/smtp_settings_test.go` — new signatures
- `webui/src/api/client.ts` — new API functions
- `webui/src/components/settings/WebsitesPanel.vue` — Email action button
- `web/` — rebuilt assets (Task 5)

## Task 1 — Context-scoped settings (backend)

**Domain port** — `internal/domain/settings/settings.go`, add:
```go
// ContextRepository is the persistence port for per-context key-value settings.
type ContextRepository interface {
	List(contextID string) (map[string]string, error)
	Get(contextID, key string) (string, error)
	Upsert(contextID, key, value string) error
	Delete(contextID, key string) error
}
```

**Migration** — append to `internal/infrastructure/settings/repo/migration.sql`:
```sql
CREATE TABLE IF NOT EXISTS context_settings (
    context_id UUID NOT NULL,
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (context_id, key)
);
```
(Both `cmd/automata/main.go` (`settings_repo.RunMigrations`) and `cicd/support/testdb.go` (scans `internal/infrastructure/settings/repo`) pick this up.)

**Repo** — `internal/infrastructure/settings/repo/context.go`: `NewContext(pool *pgxpool.Pool) settings.ContextRepository`, same style as `postgres.go` (list: `SELECT key, value FROM context_settings WHERE context_id = $1`; upsert: `INSERT ... ON CONFLICT (context_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`; delete: `DELETE ... WHERE context_id = $1 AND key = $2`, return `settings.ErrNotFound` when 0 rows).

**Handler** — `internal/adapter/api/handler/context_settings_handler.go`: `NewContextSettingsHandler(repo settings.ContextRepository)` with `List`, `Upsert`, `Delete` — mirror `settings_handler.go`, reading `contextID := c.Param("id")` (validate as UUID, 400 if invalid). Responses identical in shape to the global handler (`{"settings": m}`, `{"key","value"}`, 204).

**Routes** — `internal/adapter/api/server.go`, inside the `admin` group next to the global settings routes:
```go
contextSettingsRepo := settingsrepo.NewContext(pool)
contextSettingsHandler := handler.NewContextSettingsHandler(contextSettingsRepo)
admin.GET("/contexts/:id/settings", contextSettingsHandler.List)
admin.PUT("/contexts/:id/settings", contextSettingsHandler.Upsert)
admin.DELETE("/contexts/:id/settings/:key", contextSettingsHandler.Delete)
```

**Test** — `cicd/context_settings_test.go` (mirror `cicd/settings_test.go`): engine setup with the three routes under `/api/admin`; AfterEach `DELETE FROM context_settings`. Cover: repo upsert/get/list/delete across two contexts (isolation), unknown-key Get → ErrNotFound, API list/upsert/delete, invalid context id → 400.

**Verify:** `go build ./...` && `go test ./cicd/ -run TestCICD -v` (full suite green). Commit: `feat: per-context settings storage and API`.

## Task 2 — Context-aware SMTP

**`internal/infrastructure/email/smtp/smtp.go`:**
- `type ConfigProvider func(contextID string) Config`
- `func (m *SMTPMailer) Send(contextID string, to []string, subject, body, htmlBody string) error` — body: `cfg, dialer := m.cfg, m.dialer; if m.provider != nil { cfg = m.provider(contextID); dialer = m.dialerFor(cfg) }` (rest unchanged).
- `MergeConfig` unchanged; context layering done by the caller with two passes.

**`cmd/automata/main.go`** (≈line 214): provider closure becomes:
```go
contextSettingsRepo := settings_repo.NewContext(pool)
mailer := mailsmtp.NewMailerWithProvider(
	func(ctxID string) mailsmtp.Config {
		m, _ := settingsRepo.List()
		cm, _ := contextSettingsRepo.List(ctxID)
		return mailsmtp.MergeConfig(mailsmtp.MergeConfig(smtpCfg, m), cm)
	},
	smtpCfg,
	mailsmtp.DefaultDialer,
)
```

**`cicd/support/smtp_mock.go`:** `func (m *MockSMTP) Send(contextID string, to []string, subject, body, htmlBody string) error` (capture contextID on `Email` as `ContextID string`).

**`cicd/smtp_settings_test.go`:** update the two provider-mailer `It` blocks to `mailer.Send("ctx-1", []string{"a@b.c"}, "s", "b", "")` and provider signature `func(ctxID string) mailsmtp.Config { ... }`.

No new tests here (behavior covered by Task 3). **Verify:** `go build ./...` && full cicd suite green. Commit: `feat: context-aware SMTP config resolution`.

## Task 3 — Queue history, isolation, management

**Migration** — append to `internal/infrastructure/queue/migration.sql`:
```sql
CREATE TABLE IF NOT EXISTS email_history (
    id            UUID PRIMARY KEY,
    context_id    UUID NOT NULL,
    to_addresses  TEXT[] NOT NULL,
    subject       TEXT NOT NULL,
    body          TEXT NOT NULL,
    html_body     TEXT,
    attempts      INT NOT NULL DEFAULT 0,
    status        TEXT NOT NULL,
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL,
    finished_at   TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_email_history_context ON email_history(context_id, status, finished_at DESC);
```

**`internal/infrastructure/queue/email_queue.go`:**
- `Mailer` interface: `Send(contextID string, to []string, subject, body, htmlBody string) error`
- Add to `Queue` interface:
```go
ListPending(contextID string, limit int) ([]*EmailJob, error)
ListHistory(contextID string, status string, limit int) ([]*EmailHistory, error)
Retry(contextID, jobID string) error
Cancel(contextID, jobID string) error
PruneHistory(olderThan time.Time) (int64, error)
```
- New exported struct (with json tags, used by the API layer):
```go
type EmailHistory struct {
	ID         string    `json:"id"`
	ContextID  string    `json:"context_id"`
	To         []string  `json:"to"`
	Subject    string    `json:"subject"`
	Attempts   int       `json:"attempts"`
	Status     string    `json:"status"`
	LastError  string    `json:"last_error"`
	CreatedAt  time.Time `json:"created_at"`
	FinishedAt time.Time `json:"finished_at"`
}
```
- Implementations: `ListPending` = `SELECT ... FROM email_jobs WHERE context_id=$1 AND attempts < max_retries ORDER BY created_at DESC LIMIT $2`; `ListHistory` = same on `email_history` with `AND status=$2 ORDER BY finished_at DESC` (empty status → all); `Retry` = read the failed history row (context+id, `status='failed'`), `Enqueue(&EmailJob{ID: row.ID, ContextID, To, Subject, Body, HTMLBody, MaxRetries: 3})`, then `DELETE FROM email_history` (both in one transaction; error if row missing); `Cancel` = `DELETE FROM email_jobs WHERE id=$1 AND context_id=$2` (error when 0 rows); `PruneHistory` = `DELETE FROM email_history WHERE finished_at < $1`, return row count.

**`internal/infrastructure/queue/email_worker.go`:**
- `processNextBatch` query (isolation, cap 3 per context, batch 10):
```sql
SELECT id, context_id, to_addresses, subject, body, html_body, attempts, max_retries, created_at
FROM (
    SELECT *, ROW_NUMBER() OVER (PARTITION BY context_id ORDER BY created_at) AS rn
    FROM email_jobs
    WHERE next_retry <= NOW() AND attempts < max_retries
) t
WHERE rn <= 3
ORDER BY created_at
LIMIT $1
FOR UPDATE SKIP LOCKED
```
- `sendJob`: `mailer.Send(j.contextID, to, j.subject, j.body, j.htmlBody)`.
- `completeJob(ctx, j jobRow)`: INSERT into `email_history` (`status='sent'`, `attempts=j.attempts`, `created_at=j.createdAt`, `finished_at=NOW()`) then `DELETE FROM email_jobs WHERE id=$1` (same transaction).
- `retryJob` max-retries branch: INSERT history (`status='failed'`, `last_error=err.Error()`, `attempts=newAttempts`) then DELETE job (same transaction). Backoff branch unchanged.
- `StartWorker`: add hourly prune — track `lastPrune time.Time`; in the ticker loop, if `time.Since(lastPrune) >= time.Hour` call `q.PruneHistory(time.Now().Add(-30*24*time.Hour))` and update `lastPrune`.

**Test** — `cicd/email_queue_context_test.go` (new file; `package cicd_test`; uses `pool`, `ctx`, `support.NewTestContextID()`, `support.NewMockSMTP()` per test; AfterEach: `DELETE FROM email_jobs`, `DELETE FROM email_history`, `mockSMTP.Clear()`):
1. successful send → job deleted, history row `status='sent'`, mock captured (and `mockSMTP.Emails()[0].ContextID` matches).
2. unconfigured mailer (`mailsmtp.NewMailerWithDialer(mailsmtp.Config{}, dialer)` from `email_delivery_test.go`'s `dialerAdapter` pattern — replicate the small adapter locally) with `max_retries=1` → after drain, history row `status='failed'` with non-empty `last_error`.
3. isolation: insert 10 jobs for context A + 1 for context B (all `next_retry=NOW()`), run worker once with a failing mailer (so nothing completes) → assert `email_jobs` for B still contains B's job and A has ≥8 remaining (batch took ≤3 from A and 1 from B) — verify via counts after a short sleep + cancel.
4. `Retry`: insert a failed history row, call `Retry(ctxID, id)` → history row gone, pending job exists with attempts=0; `Retry` on missing id → error.
5. `Cancel`: enqueue a job, `Cancel` → gone; `Cancel` on missing id → error.
6. `PruneHistory`: insert history rows with `finished_at` old and new, prune 30 days → only old removed (check returned count).
7. `ListPending`/`ListHistory` return only rows for the requested context.

**Verify:** `go build ./...` && full cicd suite green (349 + new specs). Commit: `feat: email queue history, per-context isolation, retry/cancel`.

## Task 4 — Email queue API

**Handler** — `internal/adapter/api/handler/email_queue_handler.go`: `NewEmailQueueHandler(q queue.Queue)`; all handlers read `contextID := c.Param("id")` (UUID-validated, 400 if bad).
- `List`: `status` query param (default `pending`; accept `pending|sent|failed`; anything else → 400), `limit` query param (default 100, max 500). Response: `{"jobs": [...], "count": n}`. Pending items mapped from `EmailJob` to the same shape as `EmailHistory` plus `max_retries` (status `"pending"`, `finished_at` null); sent/failed from `ListHistory`. Use one response struct with json tags: `id, context_id, to, subject, attempts, max_retries, status, last_error, created_at, finished_at`.
- `Retry`: `POST .../email-queue/:jobId/retry` → 200 `{"id": jobID}`; missing → 404.
- `Cancel`: `DELETE .../email-queue/:jobId` → 204; missing → 404.
- `TestSend`: `POST .../email-queue/test` body `{"to": "x@y"}` → validate non-empty (400), enqueue `&queue.EmailJob{ContextID: contextID, To: []string{to}, Subject: "Automata SMTP test", Body: "This is a test email from Automata.", MaxRetries: 3}` → 202 `{"job_id": id}`.

**Routes** — `internal/adapter/api/server.go`, in the `admin` group:
```go
emailQueueHandler := handler.NewEmailQueueHandler(queue.NewEmailQueue(pool))
admin.GET("/contexts/:id/email-queue", emailQueueHandler.List)
admin.POST("/contexts/:id/email-queue/:jobId/retry", emailQueueHandler.Retry)
admin.DELETE("/contexts/:id/email-queue/:jobId", emailQueueHandler.Cancel)
admin.POST("/contexts/:id/email-queue/test", emailQueueHandler.TestSend)
```

**Test** — `cicd/email_queue_api_test.go` (mirror `cicd/settings_test.go` engine pattern, admin group with the four routes): list pending after enqueue; list sent/failed after worker processed (use mockSMTP like Task 3); invalid status → 400; invalid context id → 400; retry → 200 then job pending; retry missing → 404; cancel → 204 then gone; cancel missing → 404; test send → 202 and job appears in pending; test send with empty `to` → 400. AfterEach: `DELETE FROM email_jobs`, `DELETE FROM email_history`.

**Verify:** `go build ./...` && full cicd suite green. Commit: `feat: per-context email queue management API`.

## Task 5 — Frontend (Websites tab: per-website Email panel)

**`webui/src/api/client.ts`** (append near the global settings functions):
```ts
export async function listContextSettings(contextId: string): Promise<Record<string, string>>
export async function upsertContextSetting(contextId: string, key: string, value: string): Promise<void>
export async function deleteContextSetting(contextId: string, key: string): Promise<void>
export interface EmailQueueJob { id: string; context_id: string; to: string[]; subject: string; attempts: number; max_retries: number; status: string; last_error: string; created_at: string; finished_at: string | null }
export async function listEmailQueue(contextId: string, status: 'pending' | 'sent' | 'failed', limit?: number): Promise<EmailQueueJob[]>
export async function retryEmailJob(contextId: string, jobId: string): Promise<void>
export async function cancelEmailJob(contextId: string, jobId: string): Promise<void>
export async function sendTestEmail(contextId: string, to: string): Promise<void>
```
(Endpoints per Task 4; `listEmailQueue` returns `data.jobs`.)

**`webui/src/components/settings/WebsiteEmailPanel.vue`** (new; props: `open: boolean`, `context: Context | null`; emits `close`):
- Modal (reuse `Modal.vue`) titled `Email — {context.Name}`.
- **SMTP section**: render the `smtp` group from `settingsCatalog.ts` (import `settingGroups`); values loaded via `listContextSettings(context.ID)`; each field placeholder shows "Inherit from global"; boolean field rendered as checkbox like `SettingsPanel.vue`.
- **Save Changes** button: diff against loaded values → `upsertContextSetting` for changed non-empty, `deleteContextSetting` for cleared keys; success toast/message; button disabled when no changes (same dirty-state pattern as `SettingsPanel.vue`).
- **Test send**: email input + `Send test email` button → `sendTestEmail`; show result message.
- **Queue section**: three count badges (pending/sent/failed, from `listEmailQueue` per status), a selector for the active status (default `pending`), a table: To, Subject, Attempts, Created, Last error, Actions — `Retry` button for `failed` rows, `Cancel` button for `pending` rows (use `useConfirm` for cancel); refresh after actions.

**`webui/src/components/settings/WebsitesPanel.vue`**: add a `Mail` icon (lucide) button in the `#actions` template (before View/Edit/Delete) that sets `emailContext` and opens the panel; render `<WebsiteEmailPanel :open="emailContext !== null" :context="emailContext" @close="emailContext = null" />` after the existing Modal.

**Build:** `cd webui && npm run build` (must pass; copies to `web/`). Commit includes `webui/` sources + rebuilt `web/` assets: `feat: per-website email settings and queue UI`.

## Task 6 — Deploy + live verification

1. Advance deploy worktree: `git -C /tmp/opencode/automata-deploy fetch origin phase2-auth-changes && git -C /tmp/opencode/automata-deploy reset --hard phase2-auth-changes` (worktree is detached; recreate `/tmp/opencode/Dockerfile.deploy` if the restart wiped it — alpine:3.20, prebuilt binaries from `cmd/automata` + `cmd/bff`).
2. Build binaries in the worktree, `docker build` image, tag `:latest` + `:<short-sha>`, push both, `kubectl -n automata rollout restart deploy/automata-automata deploy/automata-bff`, wait for rollout success.
3. Live DOM check (Playwright, text/DOM only — no screenshots): log in `admin@automata.local` / `REDACTED` at `http://automata.home.arpa` (use `waitForTimeout(4000)` after login click, token from `localStorage`), navigate to Settings → Websites, open a website's Email panel; verify: SMTP fields render, save persists (re-read via API), clearing a field deletes the key, test send enqueues (job appears in pending), queue lists pending/sent/failed, cancel removes a pending job, retry re-enqueues a failed job.
4. Full cicd suite one final time on the committed HEAD.

## Execution mode

Subagent-driven development (established pattern for this project): cheap tier for implementers, top tier for reviewers; one task per subagent in the order above (Tasks 1→6 are sequential due to dependencies).