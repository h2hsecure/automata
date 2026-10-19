# Email Templates, Contact Email Tracking, IMAP Inbox — Implementation Plan

Spec: `docs/superpowers/specs/2026-10-19-email-templates-inbox-design.md` (commit `ec0e8fd`)

Goal: extend the **existing** email template subsystem with two template kinds (standalone / form-integrated), `email_sent` contact activity, an IMAP inbox per website, and two dashboard cards (queue status + reach stats).

Approach: TDD, bite-sized tasks. Each task: failing test → run → implement → run → commit. Unit tests for pure functions (`go test ./internal/...`); integration tests in `cicd/` (Ginkgo + Postgres testcontainer, `make test` or `go test ./cicd/ -v --ginkgo.focus="..."`). cicd baseline is **376/376** — compare against it, known pre-existing failures in tracking/form/banner/placement/campaign specs.

**Constraints:**
- WIP files must stay untouched and uncommitted: `AGENTS.md`, `automata` binary, `helm/automata/values.yaml`, `docs/phase1.md`, `.playwright-mcp/`, and cicd WIP specs (`config_handler_test.go`, `contact_form_integration_test.go`, `email_queue_test.go`, `global_login_test.go`, `snippet_test.go`, `tracking_server_test.go`, `webhook_queue_test.go`).
- No image-based verification (no screenshots, no PNG/JPG reading). Verify via text/DOM/HTTP.
- UI gate is `vite build` (project does not pass clean `vue-tsc --noEmit`). UI commits include rebuilt `web/` assets.
- `github.com/emersion/go-imap` is NOT in go.mod — Task 6 adds it.

## Key existing code (extend, don't rebuild)

- `internal/infrastructure/email/repo/migration.sql` — `email_templates(id, context_id, key, subject, body_text, body_html, is_default, created_at, updated_at)`, unique `(context_id, key)`; `RunMigrations(pool)` called from `cmd/automata/main.go:78`.
- `internal/domain/email/email.go` — `EmailTemplate`, `CreateTemplateRequest`, `UpdateTemplateRequest`, `TemplateResponse`, `SendEmailRequest`, `TestSendRequest`.
- `internal/usecase/email/email.go` — `EmailUsecase{repo, queue, log}`; `RenderTemplate(contextID, key, variables map[string]string)` (Go `text/template`); `SendEmail` → jobID; `TestSend`.
- `internal/adapter/api/handler/email_handler.go` — CRUD + TestSend + Send handlers; `toTemplateResponse`.
- Routes (server.go ~140–145): `/api/admin/email-templates` CRUD + `/:id/test`; context-resolved `/api/context/:slug/email/send`.
- `FormHandler` (`internal/adapter/api/handler/form_handler.go`) — `SubmitForm` at line 179; constructed via `NewFormHandlerWithIdentifiers(repo, contactRepo, trackingRepo)` in `server.go:181` AND `cmd/automata/main.go:129` (tracking server).
- `contact.Repository` has `FindByEmail(contextID, email)` and `CreateActivity(a *Activity)`.
- `contact_activities(id, contact_id, context_id, type, data JSONB, source_id, created_at)`.
- `queue.Queue` interface: `Enqueue(job *EmailJob)`, `ListPending`, `ListHistory`, …; `EmailJob{ID, ContextID, To []string, Subject, Body, HTMLBody, Attempts, MaxRetries, NextRetry, CreatedAt}`.
- `mailsmtp.NewMailerWithProvider(func(ctxID string) mailsmtp.Config, smtpCfg, dialer)` — the per-context config provider closure in `main.go:216` is reused for IMAP credentials.
- `context.Repository.List(offset, limit int) ([]*Context, error)`.
- UI: `webui/src/api/client.ts` (axios interceptor sets `X-Context-ID` from localStorage — context-resolved routes work with no extra work), `AppSidebar.vue`, `router/index.ts`, `DashboardView.vue` (`loadOverview` ~line 100, stats grid ~line 203), components: `DataTable`, `Modal`, `Card`, `Button`, `Input`, `Textarea`, `SearchBar`, `EmptyState`, `Badge`.

---

## Task 1: Schema + domain — `kind` and `form_id` on `email_templates`

**Files:**
- Modify: `internal/infrastructure/email/repo/migration.sql`
- Modify: `internal/domain/email/email.go`
- Modify: `internal/infrastructure/email/repo/postgres.go` (Create/Get/List/Update column handling)
- Test: `cicd/email_template_test.go` (extend existing spec)

**Steps:**

1. **Failing cicd test** — in `cicd/email_template_test.go`, add a new `Describe("Email Template kind and form binding")` block:
   - Create template with `kind: "form"` + `form_id` → 201, response echoes `kind` and `form_id`.
   - Create template with `kind: "form"` and no `form_id` → 400 (`form_id required for form templates`).
   - Create template with `kind: "bulk"` → 400 (`invalid kind`).
   - Create with no kind → 201, response `kind == "standalone"` (default).
   - Seeded defaults: list templates for a fresh context → all have `kind == "standalone"`.
   - Update flips `kind` to `form` + sets `form_id` → 200.
2. Run: `go test ./cicd/ -v --ginkgo.focus="kind and form binding"` → FAIL (fields missing / validation absent).
3. **Migration** — append to `migration.sql` (idempotent, matching existing style):
   ```sql
   ALTER TABLE email_templates ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'standalone';
   ALTER TABLE email_templates ADD COLUMN IF NOT EXISTS form_id UUID;
   ```
4. **Domain** — `internal/domain/email/email.go`:
   - `EmailTemplate`: add `Kind string` and `FormID *string`.
   - `CreateTemplateRequest`: add `Kind string \`json:"kind"\`` and `FormID *string \`json:"form_id"\``.
   - `UpdateTemplateRequest`: same two fields (pointer semantics for optional update — `FormID **string` if the existing pattern uses double pointers for nullable overrides; match the file's existing style).
   - `TemplateResponse`: add `Kind string \`json:"kind"\`` and `FormID *string \`json:"form_id"\``.
   - Add `ValidateKind(kind string) error` (allowed: `""`, `standalone`, `form`) and extend `ValidateCreateRequest` to require `FormID != nil` when `Kind == "form"`.
5. **Repo** — `postgres.go`: add `kind`, `form_id` to all column lists (Create INSERT, Get/List/GetByKey SELECT, Update SET). `form_id` handled as nullable (`pgtype.UUID` or `*string` — match existing nullable handling in the file).
6. Run the focused spec → PASS. Run full `cicd/email_template_test.go` suite → PASS (existing CRUD stays green).
7. Commit: `feat(email): add kind and form_id to email_templates`.

## Task 2: Variable builders + generalize `RenderTemplate`

**Files:**
- Create: `internal/usecase/email/variables.go`
- Test: `internal/usecase/email/variables_test.go` (plain `go test`, table-driven)
- Modify: `internal/usecase/email/email.go` (`RenderTemplate` signature + 2 internal call sites)

**Steps:**

1. **Failing unit test** — `variables_test.go`:
   - `BuildContactVariables(c *contact.Contact) map[string]interface{}`: returns `first_name`, `last_name`, `email`, `company`, `phone` (strings), `custom` (nested `map[string]interface{}` from `c.CustomFields`; empty map when nil). Nil-safe: `BuildContactVariables(nil)` returns a map with empty strings + empty `custom`.
   - `BuildFormVariables(f *form.Form, s *form.Submission, c *contact.Contact) map[string]interface{}`: `form` (nested map from `s.Data`), `form_name` (`f.Name`), `submit_date` (`s.CreatedAt` RFC 1123), plus all contact variables (empty-safe when `c` is nil).
2. Run: `go test ./internal/usecase/email/...` → FAIL (functions don't exist).
3. **Implement** `variables.go` with the two functions above. Imports: `internal/domain/contact`, `internal/domain/form`, `time`.
4. **Generalize** `RenderTemplate` in `email.go`: `variables map[string]string` → `map[string]interface{}`. At the two internal call sites (`SendEmail` line ~87, `TestSend` line ~126), convert `req.Variables` (still `map[string]string` in the request structs — leave the API shape unchanged) via a small helper:
   ```go
   func toInterfaceVars(m map[string]string) map[string]interface{} {
       if m == nil { return nil }
       out := make(map[string]interface{}, len(m))
       for k, v := range m { out[k] = v }
       return out
   }
   ```
5. Run: `go test ./internal/usecase/email/...` → PASS. `go build ./...` → PASS.
6. Commit: `feat(email): contact/form variable builders; generalize template variables`.

## Task 3: `email_sent` activity type + title formatting

**Files:**
- Modify: `internal/domain/contact/activity.go`
- Test: `internal/domain/contact/activity_test.go` (create if absent; `contact_test.go` exists in this package)

**Steps:**

1. **Failing unit test**: `FormatActivityTitle` for `ActivityEmailSent` with `Data{"subject": "Hi"}` → `"Email sent: Hi"`; without subject → `"Email sent"`.
2. Run: `go test ./internal/domain/contact/...` → FAIL.
3. **Implement**: add `ActivityEmailSent ActivityType = "email_sent"` to the const block; add the case to `FormatActivityTitle` (read `a.Data["subject"].(string)`).
4. Run → PASS. Commit: `feat(contact): add email_sent activity type`.

## Task 4: Standalone send (usecase + handler + route)

**Files:**
- Modify: `internal/usecase/email/email.go` (or new `send.go` in the package)
- Modify: `internal/adapter/api/handler/email_handler.go`
- Modify: `internal/adapter/api/server.go` (route + wiring)
- Test: `cicd/email_template_test.go` (new `Describe("Standalone template send")`)

**Steps:**

1. **Failing cicd test**:
   - Seed a context + contact (with email) + standalone template (subject/body with `{{ .first_name }}`).
   - `POST /api/admin/contexts/:id/email-templates/:tid/send` body `{"contact_id": "..."}` → 202 `{"job_id": "..."}`.
   - Job visible in `email_jobs` (mock SMTP from `support.NewMockSMTP()` — pattern already in this spec file) with rendered subject/body.
   - Contact activity feed contains an `email_sent` activity with `data.subject`.
   - Send to a contact with no email → 400.
   - Send with a form-kind template → 400 (or 404 — pick 400 with message `template is not standalone`).
   - Unknown template id → 404; unknown contact id → 404.
2. Run focused spec → FAIL.
3. **Usecase** — `EmailUsecase` gains `contactRepo contact.Repository` (new field; add constructor `NewEmailUsecaseWithContact(repo, q, contactRepo)`; keep `NewEmailUsecase` delegating with nil contactRepo — all new methods nil-safe). Implement:
   ```go
   func (u *EmailUsecase) SendToContact(contextID, templateID, contactID string) (string, error)
   ```
   - Load template by ID (repo needs `GetByID` — check existing; `GetByKey` exists, add `GetByID` if missing) → 404-equivalent error.
   - `template.Kind != "standalone"` → error.
   - Load contact (`contactRepo.Get` — check exact method name in `contact.Repository`) → not found error; `contact.Email == nil || *contact.Email == ""` → error.
   - `vars := BuildContactVariables(contact)`; render subject+body via `RenderTemplate` (by key — or render inline by ID; prefer adding `renderByID` helper in the usecase to avoid key lookup).
   - Enqueue `queue.EmailJob{ContextID, To: []string{*contact.Email}, Subject, Body, HTMLBody: template.BodyHTML}`.
   - `contactRepo.CreateActivity(&contact.Activity{ContactID, ContextID, Type: contact.ActivityEmailSent, Data: map[string]interface{}{"subject": subject, "template_name": template.Key}})`.
   - Return jobID.
4. **Handler** — `EmailHandler.SendToContact`: bind `{"contact_id"}` (required), `:id` from path = context id, `:tid` = template id. Map errors: not-found → 404, validation → 400, success → 202 `{"job_id": ...}`.
5. **Route** — in the admin group (server.go, near existing email routes):
   ```go
   admin.POST("/contexts/:id/email-templates/:tid/send", emailHandler.SendToContact)
   ```
6. Run focused spec → PASS. Commit: `feat(email): standalone template send to contact with activity tracking`.

## Task 5: Form-integrated send (hook in `FormHandler.SubmitForm`)

**Files:**
- Modify: `internal/usecase/email/` (new `send.go` method `SendForSubmission`)
- Modify: `internal/adapter/api/handler/form_handler.go` (optional `emailUC` + hook)
- Modify: `internal/adapter/api/server.go` + `cmd/automata/main.go` (wiring both FormHandler constructions)
- Test: `cicd/email_template_test.go` (new `Describe("Form-integrated template send")`)

**Steps:**

1. **Failing cicd test**:
   - Create form with an `email`-type field + text field; create template `kind: "form"`, `form_id: <form id>`, body using `{{ .form.<field> }}` and `{{ .first_name }}`.
   - Submit via the existing submission endpoint used by other cicd specs (find the one `email_template_test.go` / `form_http_test.go` uses for submissions) with data including the email → 200/201.
   - Job in `email_jobs` addressed to the submitter's email, rendered body.
   - If a contact exists with that email → `email_sent` activity on it.
   - Submission without an email value → 200, no job.
   - Template with a broken template syntax → submission still succeeds (error logged only).
2. Run focused spec → FAIL.
3. **Usecase**:
   ```go
   func (u *EmailUsecase) SendForSubmission(contextID, formID string, s *form.Submission) error
   ```
   - List templates for the form (repo: add `ListByForm(contextID, formID)` returning kind=form rows — or filter `List` by form_id in SQL).
   - Recipient: first value in `s.Data` whose field slug matches a form field of type `email` (need the form's fields — pass `*form.Form` instead of formID: `SendForSubmission(contextID string, f *form.Form, s *form.Submission) error`; the handler already loads the form).
   - No email value → return nil (skip silently).
   - `contact, _ := contactRepo.FindByEmail(contextID, email)` (nil-safe).
   - `vars := BuildFormVariables(f, s, contact)`; render per template; enqueue; activity if contact matched.
   - Any per-template error: log and continue; never return fatal for individual template failures.
4. **FormHandler** — add `emailUC *emailusecase.EmailUsecase` field; new constructor `NewFormHandlerWithEmail(repo, contactRepo, trackingRepo, emailUC)` (keep existing constructors; nil `emailUC` = disabled). In `SubmitForm` after the submission is stored (line ~179+):
   ```go
   if h.emailUC != nil {
       if err := h.emailUC.SendForSubmission(sub.ContextID, formEntity, sub); err != nil {
           log.Warn().Err(err).Msg("form-integrated email send failed")
       }
   }
   ```
   (Confirm exact variable names in `SubmitForm` — form entity + submission — and the contextID source when writing.)
5. **Wiring** — `server.go:181` and `main.go:129`: construct with the email usecase (both servers handle submissions).
6. Run focused spec → PASS. Commit: `feat(email): form-integrated template send on submission`.

## Task 6: IMAP inbox (table + repo + poller + API)

**Files:**
- Create: `internal/domain/email/inbox.go`
- Modify: `internal/infrastructure/email/repo/migration.sql`
- Create: `internal/infrastructure/email/repo/postgres_inbox.go`
- Create: `internal/infrastructure/email/imap/poller.go`
- Modify: `internal/adapter/api/handler/email_handler.go` (ListInbox, MarkInboxRead)
- Modify: `internal/adapter/api/server.go` (routes)
- Modify: `cmd/automata/main.go` (poller startup, alongside email worker ~line 232)
- Modify: `go.mod` / `go.sum` (`github.com/emersion/go-imap`)
- Test: `cicd/email_template_test.go` (new `Describe("Email inbox")` — repo/API level only, NO live IMAP)

**Steps:**

1. **Failing cicd test** (inbox repo + API, no IMAP):
   - `GET /api/admin/contexts/:id/inbox?limit=50` on empty → 200 `{"messages": [], "count": 0}`.
   - Seed rows directly via `pool` (INSERT into `email_inbox`) → list returns newest-first, `limit` respected.
   - `POST /api/admin/contexts/:id/inbox/:mid/read` → 200; subsequent list shows `is_read: true`.
   - Dedupe at repo level: upsert with same `(context_id, uid)` twice → one row (test the repo method directly via the postgres repo constructor, pattern: `repo.New(pool)`).
2. Run focused spec → FAIL.
3. **Domain** — `internal/domain/email/inbox.go`:
   ```go
   type InboxMessage struct {
       ID, ContextID, UID, From, To, Subject, BodyText string
       MessageDate time.Time
       IsRead bool
       CreatedAt time.Time
   }
   type InboxRepository interface {
       List(contextID string, limit int) ([]*InboxMessage, error)
       MarkRead(id string) error
       Upsert(m *InboxMessage) error // dedupe on (context_id, uid)
       MaxUID(contextID string) (string, error) // "" when empty
   }
   ```
4. **Migration** — append to `migration.sql`:
   ```sql
   CREATE TABLE IF NOT EXISTS email_inbox (
       id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
       context_id UUID NOT NULL,
       uid TEXT NOT NULL,
       from TEXT NOT NULL DEFAULT '',
       to TEXT NOT NULL DEFAULT '',
       subject TEXT NOT NULL DEFAULT '',
       body_text TEXT NOT NULL DEFAULT '',
       message_date TIMESTAMPTZ NOT NULL DEFAULT now(),
       is_read BOOLEAN NOT NULL DEFAULT false,
       created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
       UNIQUE (context_id, uid)
   );
   ```
5. **Repo** — `postgres_inbox.go`: `NewInboxRepo(pool)` implementing the interface. `Upsert` = `INSERT ... ON CONFLICT (context_id, uid) DO UPDATE SET subject=EXCLUDED.subject, ...`. `MaxUID` = `SELECT max(uid)` (UIDs are numeric strings; lexicographic max is wrong for >9999 — store/compare as `BIGINT`? **Decision: store `uid` as `BIGINT`** — IMAP UIDs are numeric; change the column to `uid BIGINT NOT NULL` and `MaxUID` to `SELECT max(uid)`. Adjust the interface: `MaxUID(contextID string) (int64, error)`.)
6. **Poller** — `internal/infrastructure/email/imap/poller.go`:
   ```go
   type Poller struct {
       inboxRepo   email.InboxRepository
       contexts    context.Lister   // interface { List(offset, limit int) ([]*context.Context, error) } — satisfied by context.Repository
       configFor   func(ctxID string) mailsmtp.Config
       interval    time.Duration
   }
   func NewPoller(inboxRepo email.InboxRepository, lister ..., configFor func(ctxID string) mailsmtp.Config) *Poller
   func (p *Poller) Start(ctx context.Context) // 60s ticker + immediate first run
   ```
   Per context per tick: resolve config; skip if `Username == ""` (not configured); connect `host:993` via `github.com/emersion/go-imap/client` (`client.DialTLS(host:port, &tls.Config{ServerName: host}, username, password)`); `client.Select(true, imap.MailboxInbox)`; `last, _ := inboxRepo.MaxUID(ctxID)`; fetch UID range `last+1:*` (or all, take last 50 when `last == 0`); for each message fetch `UID BODY.PEEK[TEXT]` + headers (From, To, Subject, Date via `client.Fetch`); extract first text/plain body (fall back to full text body if no part); `Upsert`. Per-context errors: `log.Warn` and continue to next context. Cap stored messages per context at e.g. 500 (delete oldest beyond cap after upsert — keep it simple: `DELETE FROM email_inbox WHERE context_id=$1 AND id NOT IN (SELECT id FROM email_inbox WHERE context_id=$1 ORDER BY message_date DESC LIMIT 500)`).
7. **API** — `EmailHandler` gains `inboxRepo email.InboxRepository` (extend existing constructor or add setter — match existing wiring style). Handlers: `ListInbox` (`GET`, `limit` query param default 50, max 200), `MarkInboxRead` (`POST`, 404 when not found).
8. **Routes** (admin group):
   ```go
   admin.GET("/contexts/:id/inbox", emailHandler.ListInbox)
   admin.POST("/contexts/:id/inbox/:mid/read", emailHandler.MarkInboxRead)
   ```
9. **Wiring** — `main.go`: after `emailQueue.StartWorker` (~line 232):
   ```go
   imapPoller := imap.NewPoller(email_repo.NewInboxRepo(pool), contextRepo, func(ctxID string) mailsmtp.Config { /* same provider closure as mailer — factor into a named var reused by both */ })
   go imapPoller.Start(ctx)
   ```
   Factor the provider closure at `main.go:216` into a `configFor` variable used by both the mailer and the poller.
10. `go get github.com/emersion/go-imap@latest` (pulls `go-imap/client` transitively — verify module path).
11. Run focused spec → PASS. `go build ./...` → PASS. Commit: `feat(email): IMAP inbox per context with poller and API`.

## Task 7: Email stats endpoint (dashboard data)

**Files:**
- Modify: `internal/infrastructure/queue/email_queue.go` (add count methods to `Queue` interface + impl)
- Modify: `internal/domain/contact/repository.go` + `internal/infrastructure/contact/repo` (activity count method)
- Create: `internal/usecase/email/stats.go`
- Modify: `internal/adapter/api/handler/email_handler.go` (GetEmailStats)
- Modify: `internal/adapter/api/server.go` (context-resolved route)
- Test: `cicd/email_template_test.go` (new `Describe("Email stats")`)

**Steps:**

1. **Failing cicd test**:
   - Seed: 2 pending jobs (INSERT `email_jobs`), 3 sent + 1 failed history rows (INSERT `email_history` with `finished_at` values: 2 within 24h sent, 1 older sent, 1 within 24h failed), `email_sent` activities on 2 distinct contacts (1 within 7d, 1 older).
   - `GET /api/context/email-stats` (with `X-Context-ID` header) → 200:
     ```json
     {"pending": 2, "sent_24h": 2, "failed_24h": 1, "emails_sent_total": 3, "contacts_reached": 2, "reached_7d": 1}
     ```
2. Run focused spec → FAIL.
3. **Queue counts** — add to `Queue` interface + `emailQueue` impl:
   ```go
   CountPending(contextID string) (int, error)
   CountHistory(contextID, status string, since *time.Time) (int, error) // since=nil → all time
   ```
4. **Contact count** — add to `contact.Repository` + postgres impl:
   ```go
   CountDistinctContactsByActivity(contextID string, activityType string, since *time.Time) (int, error)
   ```
   SQL: `SELECT count(DISTINCT contact_id) FROM contact_activities WHERE context_id=$1 AND type=$2 [AND created_at >= $3]`.
5. **Usecase** — `stats.go`:
   ```go
   type EmailStats struct {
       Pending int `json:"pending"`
       Sent24h int `json:"sent_24h"`
       Failed24h int `json:"failed_24h"`
       EmailsSentTotal int `json:"emails_sent_total"`
       ContactsReached int `json:"contacts_reached"`
       Reached7d int `json:"reached_7d"`
   }
   func (u *EmailUsecase) GetEmailStats(contextID string) (*EmailStats, error)
   ```
   (Needs `contactRepo` — nil-safe: when nil, activity counts are 0.)
6. **Handler + route** — `GetEmailStats` reads `c.GetString("context_id")` (context-resolved middleware sets it; same pattern as `trackingHandler.GetDashboard`). Route in the context-resolved group (server.go ~line 219 area):
   ```go
   api.GET("/context/email-stats", middleware.AuthMiddleware(authService), emailHandler.GetEmailStats)
   ```
   (Place inside the same group that has `ContextResolver` middleware — verify exact group at `server.go:216-222` when writing.)
7. Run focused spec → PASS. Commit: `feat(email): email-stats endpoint for dashboard cards`.

## Task 8: UI — Email page + dashboard cards

**Files:**
- Modify: `webui/src/types/api.ts` (types)
- Modify: `webui/src/api/client.ts` (functions)
- Create: `webui/src/views/EmailView.vue`
- Modify: `webui/src/components/AppSidebar.vue` (nav item between Forms and Settings, `adminOnly: true`)
- Modify: `webui/src/router/index.ts` (route `/email`, `adminOnly`)
- Modify: `webui/src/views/DashboardView.vue` (two cards in Overview stats grid)
- Rebuild: `web/` assets via `vite build`

**Steps:**

1. **client.ts** — add types + functions (mirror existing email-queue functions at lines ~357–370):
   ```ts
   // types in types/api.ts
   interface EmailTemplate { id: string; context_id: string; key: string; subject: string; body_text: string; body_html: string; is_default: boolean; kind: 'standalone' | 'form'; form_id: string | null; created_at: string; updated_at: string }
   interface InboxMessage { id: string; uid: string; from: string; to: string; subject: string; body_text: string; message_date: string; is_read: boolean; created_at: string }
   interface EmailStats { pending: number; sent_24h: number; failed_24h: number; emails_sent_total: number; contacts_reached: number; reached_7d: number }

   listEmailTemplates(contextId = ''): GET /admin/email-templates?context_id=
   createEmailTemplate(data): POST /admin/email-templates
   updateEmailTemplate(id, data): PUT /admin/email-templates/:id
   deleteEmailTemplate(id): DELETE /admin/email-templates/:id
   sendTemplateToContact(contextId, templateId, contactId): POST /admin/contexts/:id/email-templates/:tid/send
   listInbox(contextId, limit = 50): GET /admin/contexts/:id/inbox?limit=
   markInboxRead(contextId, messageId): POST /admin/contexts/:id/inbox/:mid/read
   getEmailStats(): GET /context/email-stats
   ```
2. **EmailView.vue** — model on `FormsView.vue` (338 lines) structure. Two tabs (Templates / Inbox):
   - **Templates tab:** `DataTable` (key, kind badge, form name, updated) + "New template" button. Create/edit `Modal`: key, kind select (standalone/form), form picker (`listForms`) shown when kind=form, subject `Input`, body `Textarea`, placeholder hint text (standalone: `{{ .first_name }} {{ .last_name }} {{ .email }} {{ .company }} {{ .phone }} {{ .custom.<field> }}`; form: `{{ .form.<field> }} {{ .form_name }} {{ .submit_date }}` + contact vars). Standalone rows: **Send** button → contact picker modal (`listContacts` with search) → confirm → `sendTemplateToContact` → toast with job result.
   - **Inbox tab:** message list (from, subject, date, unread `Badge`), click to expand body (inline expansion, no new component), expand marks read via `markInboxRead`. `EmptyState` when empty. Refresh button (poller runs server-side every 60s).
3. **Sidebar + router** — nav item `Email` (icon matching existing style) between Forms and Settings, `adminOnly: true`; route `/email` → `EmailView`, `adminOnly`.
4. **DashboardView.vue** — in `loadOverview` add `getEmailStats()` (fire alongside existing calls, tolerate failure — card hides on error, matching existing card behavior). Two new cards in the stats grid:
   - **Email queue:** pending, sent (24h), failed (24h).
   - **Email reach:** contacts reached (all-time), reached (7d), total emails sent.
5. **Build** — `cd webui && npx vite build` (verify output lands in `web/` per existing build config). `vite build` must pass (do NOT run `vue-tsc --noEmit` as a gate).
6. Commit: `feat(ui): email page (templates + inbox) and dashboard email cards` (include `web/` assets).

## Task 9: Full suite, deploy, live verification

**Steps:**

1. `go build ./...` + `go vet ./internal/...` (known pre-existing vet failure in `proxy_test.go:23` — ignore).
2. `make test` (full cicd) — compare against baseline **376/376**; new specs must all pass; pre-existing failures (tracking/form/banner/placement/campaign) compared to baseline, not assumed regressions.
3. Build image + deploy to k3s `automata` namespace (same flow as the SMTP feature deploy).
4. **Live verification (text/HTTP only — no screenshots):**
   - `curl` the email-stats endpoint → sane JSON.
   - Create a standalone template via API with `{{ .first_name }}`, send to a real contact → 202, job appears in queue, `email_sent` activity on the contact.
   - Create a form-bound template, submit the form via the tracking endpoint → email enqueued to submitter.
   - Inbox: poller runs against `mail.h2hsecure.com:993` with the context's SMTP credentials → `GET /api/admin/contexts/:id/inbox` returns messages from the real mailbox (confirm via curl, compare subject lines against known mailbox contents).
   - UI smoke via DOM/HTTP: `curl` the BFF-served `/email` page HTML; Playwright accessibility/DOM snapshot if needed (no screenshots).
5. Commit any follow-up fixes. Update `specs/roadmap.md` build status if it tracks this feature.

## Open items (do NOT block this plan)

- `main` is 92 commits ahead of `origin/main` — push decision belongs to the user.
- pweb ingress lacks `/form` path (405 on real submissions) — user decision.
- Dead form route `server.go:174` — user decision.
- Admin routes don't enforce admin role (pre-existing) — separate task.