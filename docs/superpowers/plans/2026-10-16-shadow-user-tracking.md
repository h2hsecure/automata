# Shadow-User (Anonymous Visitor) Tracking — Implementation Plan

**Date:** 2026-10-16
**Branch:** `phase2-auth-changes` (HEAD `534ee53`)
**Design:** agreed in brainstorming — shadow users are the existing `tracking_visitors` rows (identity key `(context_id, cookie_value)`, cookie `automata_visitor` from the snippet). We add: raw IP capture, form-submit → contact identification (visitor "identified"), a context-scoped visitors API, a dedicated Add Contact page, and a merged Dashboard (Overview / Events / Visitors tabs; Tracking page deleted).

## Ground rules

- **TDD**: each task writes its failing test first, then implements, then commits.
- **Do NOT touch uncommitted WIP files** (leave them uncommitted): `AGENTS.md`, `automata` binary, `helm/automata/values.yaml`, `docs/phase1.md`, `.playwright-mcp/`, and cicd: `config_handler_test.go`, `contact_form_integration_test.go`, `email_queue_test.go`, `global_login_test.go`, `snippet_test.go`, `tracking_server_test.go`, `webhook_queue_test.go`. All new cicd tests go in a **new file** `cicd/shadow_user_test.go`.
- **Do NOT change** `NewFormHandler(repo form.Repository)` signature — `cicd/tracking_server_test.go` (WIP) calls it. Add a second constructor instead.
- UI gate is `vite build` (project does not pass clean `vue-tsc --noEmit`; pre-existing).
- cicd baseline: **320 passed | 10 failed of 330** (all 10 in `banner_campaign_http_test.go`, pre-existing). Focused runs: `go test ./cicd/ -run TestCICD -args -ginkgo.focus="..."`.
- Leave `ip_hash` column and `TrackBatch` untouched (out of scope; snippet uses single-event POST).
- Leave `/track/batch` visitor-upsert behavior as-is.

## Architecture notes (verified)

- Migrations are `//go:embed`-ed per repo package and run idempotently (`CREATE TABLE IF NOT EXISTS` style). New columns use `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`.
- BFF (`cmd/bff`) proxies all `/api/*` to the main API — new context-group routes reach the UI automatically.
- `TrackingHandler` already has `ListVisitors`, `GetVisitorDetail`, `GetVisitorEvents` handlers (mounted on admin routes at `server.go:107-109`). Context-group mounting reuses them.
- `SubmitForm` handler (`form_handler.go`) binds `SubmitFormBody{Data}`; context ID comes from `c.GetString("context_id")` (set by context middleware).
- `CreateContact` (usecase/contact) already dedupes by email and records activity.
- Form field slugs used for mapping: `email`, `first_name`, `last_name`, `phone`, `company`.

---

## Task 1 — Storage: `ip`, `last_ip`, `contact_id` + IP capture in `/track`

**Files:**
- `internal/infrastructure/tracking/repo/migration.sql` (append)
- `internal/domain/tracking/event.go`, `visitor.go`, `repository.go`
- `internal/infrastructure/tracking/repo/postgres.go`
- `internal/usecase/tracking/track_event.go`
- `internal/adapter/tracking/track_handler.go`
- `cicd/shadow_user_test.go` (new)

**Steps:**

1. **Test first** — `cicd/shadow_user_test.go`:
   ```go
   package cicd_test
   // imports mirror cicd/tracking_server_test.go:
   // bytes, encoding/json, net/http, net/http/httptest, gin, support,
   // ttracking, tdomain, trepo, ginkgo, gomega

   var _ = Describe("Shadow User Tracking", func() {
       var (
           w *httptest.ResponseRecorder
           engine *gin.Engine
           trackingRepo tdomain.Repository
       )

       BeforeEach(func() {
           gin.SetMode(gin.TestMode)
           w = httptest.NewRecorder()
           trackingRepo = trepo.New(pool)
           engine = ttracking.NewServer(formHandler, bannerHandler, placementHandler, campaignHandler, trackingRepo)
       })
   ```
   NOTE: `formHandler`/`bannerHandler`/`placementHandler`/`campaignHandler` vars are declared in the WIP `tracking_server_test.go` `BeforeEach` — since we cannot share its scope, declare our own local handlers in our `BeforeEach` (copy the construction lines from `tracking_server_test.go:34-43`).

   Tests:
   - `POST /track` body `{"contextId":"<uuid>","type":"page_view","url":"/home","visitor_id":"<vid>"}` with `req.RemoteAddr = "203.0.113.7:1234"` → 204; then `SELECT ip FROM tracking_events` = `203.0.113.7` and `SELECT last_ip FROM tracking_visitors WHERE cookie_value='<vid>'` = `203.0.113.7`.
   - `SetVisitorContactID` via repo: set contact_id for visitor, then a second `POST /track` with same visitor_id → `SELECT contact_id` unchanged (upsert never un-identifies).
   - `AfterEach`: `DELETE FROM tracking_events; DELETE FROM tracking_visitors;` (same pattern as WIP file).

2. **Migration** — append to `migration.sql`:
   ```sql
   ALTER TABLE tracking_events ADD COLUMN IF NOT EXISTS ip VARCHAR(64);
   ALTER TABLE tracking_visitors ADD COLUMN IF NOT EXISTS last_ip VARCHAR(64);
   ALTER TABLE tracking_visitors ADD COLUMN IF NOT EXISTS contact_id VARCHAR(128);
   ```

3. **Domain**:
   - `event.go`: add `IP string` to `Event` (keep `IPHash`).
   - `visitor.go`: add `LastIP string` and `ContactID string` to `Visitor`.
   - `repository.go`: add to `Repository` interface:
     ```go
     SetVisitorContactID(ctx context.Context, contextID, visitorID, contactID string) error
     ```

4. **Repo** (`postgres.go`):
   - `CreateEvent`: add `ip` to INSERT column list (after `user_agent`) and `e.IP` to args.
   - `CreateEventsBatch`: same addition to its batch INSERT.
   - `UpsertVisitor`: add `last_ip` to INSERT columns/VALUES (`v.LastIP`); add `last_ip = EXCLUDED.last_ip` to `ON CONFLICT ... DO UPDATE`. Do **not** touch `contact_id` (new rows get NULL; never un-identify).
   - `ListVisitors` dataQuery and `GetVisitorByID`: add `last_ip, contact_id` to SELECT; scan `&v.LastIP, &v.ContactID`.
   - `GetVisitorEvents` dataQuery: add `ip` to SELECT; scan `&e.IP`.
   - New method:
     ```go
     func (r *repo) SetVisitorContactID(ctx context.Context, contextID, visitorID, contactID string) error {
         _, err := r.pool.Exec(ctx,
             `UPDATE tracking_visitors SET contact_id = $3
              WHERE context_id = $1 AND id = $2 AND (contact_id IS NULL OR contact_id = '')`,
             contextID, visitorID, contactID)
         if err != nil {
             return fmt.Errorf("set visitor contact: %w", err)
         }
         return nil
     }
     ```

5. **Usecase** (`track_event.go`): in both `TrackEvent` and `TrackEventWithTime`, add `LastIP: event.IP` to the `tracking.Visitor` literal.

6. **Handler** (`track_handler.go`): in `Track`, after `userAgent := c.GetHeader("User-Agent")`, add `event.IP = c.ClientIP()` (set on the `tracking.Event` literal).

7. Run focused cicd: `go test ./cicd/ -run TestCICD -args -ginkgo.focus="Shadow User Tracking"` → green. `go build ./...` → clean. Commit: `feat(tracking): store raw IP on events/visitors and contact_id link`.

---

## Task 2 — Form JS sends `visitor_id` on submit

**Files:**
- `pkg/snippet/form_generator.go`
- `pkg/snippet/form_generator_test.go` (new)

**Steps:**

1. **Test first** — `pkg/snippet/form_generator_test.go`:
   ```go
   func TestGenerateFormSendsVisitorID(t *testing.T) {
       js, err := GenerateForm("00000000-0000-4000-8000-000000000000", "test")
       if err != nil { t.Fatal(err) }
       for _, want := range []string{`_getCookie("automata_visitor")`, `visitor_id:visitorId`} {
           if !strings.Contains(js, want) { t.Errorf("snippet missing %q", want) }
       }
   }
   ```
   Run: `go test ./pkg/snippet/` → fails.

2. **Implement** — in the `js` template in `form_generator.go`:
   - Add after `var submitUrl=...`:
     ```js
     function _getCookie(n){var m=n+"=";var c=document.cookie.split(";");for(var i=0;i<c.length;i++){var v=c[i].trim();if(v.indexOf(m)===0)return v.substring(m.length,v.length)}return null}
     var visitorId=_getCookie("automata_visitor");
     ```
   - Change submit body: `body:JSON.stringify({data:data,visitor_id:visitorId})`.

3. `go test ./pkg/snippet/` → green. Commit: `feat(snippet): form JS sends visitor_id from automata_visitor cookie`.

---

## Task 3 — Form submit identifies the visitor (create contact + link)

**Files:**
- `internal/usecase/form/submit_form.go` (add field)
- `internal/usecase/form/identify_visitor.go` (new)
- `internal/adapter/api/handler/form_handler.go`
- `internal/adapter/api/server.go` (line ~158)
- `cmd/automata/main.go` (line ~122)
- `cicd/shadow_user_test.go` (extend)

**Steps:**

1. **Test first** — extend `cicd/shadow_user_test.go`:
   - Setup: create context (`support.NewTestContextID()`), create form via `formRepo.New(pool)` with fields `[email, first_name]` slug `test` (mirror construction used in WIP `contact_form_integration_test.go` — read it for the exact `form.Form`/`FormField` shape).
   - Seed a visitor: `POST /track` with `visitor_id` (as Task 1).
   - Build a main-API-style gin engine that mounts `formHandler.SubmitForm` at `POST /form/submit/:slug` with a middleware stub setting `c.Set("context_id", ctxID)` (check how existing cicd handler tests mount context — grep `context_id` in `cicd/*_test.go`).
   - `POST /form/submit/test` body `{"data":{"email":"shadow@example.com","first_name":"Ada"},"visitor_id":"<vid>"}` → 201.
   - Assert: contact exists via `contactRepo.FindByEmail(ctxID, "shadow@example.com")` with `Source == "form:test"` and `SourceID == vid`; `SELECT contact_id FROM tracking_visitors WHERE cookie_value='<vid>'` = contact ID.
   - Negative: same email again with a *different* visitor_id → same contact returned (dedupe), and the *first* visitor's contact_id is preserved (second upsert/link is a no-op for the first row).

2. **Usecase**:
   - `submit_form.go`: add `VisitorID string` to `SubmitFormInput`.
   - New `identify_visitor.go`:
     ```go
     package form

     // IdentifyVisitor creates a contact from form data (email dedupe built in)
     // and links it to the tracking visitor. It only identifies when an email
     // is present in the submission.
     func IdentifyVisitor(contactRepo contact.Repository, trackingRepo tracking.Repository,
         contextID, slug, visitorID string, data map[string]interface{}) (*ct.Contact, error)
     ```
     - Imports: `contact "github.com/hekemen/automata/internal/usecase/contact"`, `ct "github.com/hekemen/automata/internal/domain/contact"`, `tracking "github.com/hekemen/automata/internal/domain/tracking"`.
     - Read `email` from `data["email"]` (string, non-empty) — if absent, return `nil, nil` (skip, not an error).
     - `contact.CreateContact(contactRepo, contextID, contact.CreateInput{Email: &email, FirstName: str(data["first_name"]), LastName: str(data["last_name"]), Phone: str(data["phone"]), Company: str(data["company"]), Source: "form:" + slug, SourceID: visitorID})`.
     - `trackingRepo.SetVisitorContactID(context.Background(), contextID, visitorID, c.ID)`.
     - `str(v interface{}) string` helper for the string fields.

3. **Handler** (`form_handler.go`):
   - `SubmitFormBody` gains `VisitorID string \`json:"visitor_id"\``.
   - Struct fields: add `contactRepo contact.Repository` and `trackingRepo tracking.Repository` (import usecase/contact as `contactuc`).
   - Keep `NewFormHandler(repo form.Repository)` unchanged (nil repos). Add:
     ```go
     func NewFormHandlerWithIdentifiers(repo form.Repository, contactRepo contactuc.Repository, trackingRepo tracking.Repository) *FormHandler
     ```
   - In `SubmitForm`: pass `VisitorID: body.VisitorID` into `uc.SubmitFormInput`. After successful submission, before `c.JSON(http.StatusCreated, ...)`:
     ```go
     if body.VisitorID != "" && h.contactRepo != nil && h.trackingRepo != nil {
         if _, err := uc.IdentifyVisitor(h.contactRepo, h.trackingRepo, contextID, slug, body.VisitorID, data); err != nil {
             log.Warn().Err(err).Msg("identify visitor from form submission")
         }
     }
     ```
     (import `github.com/rs/zerolog/log` if not already).

4. **Wiring**:
   - `server.go:158`: `formHandler := handler.NewFormHandlerWithIdentifiers(formRepo, contactRepo, trackingRepo)` (vars exist at lines 144/105/157).
   - `cmd/automata/main.go:122`: add `contactRepo := contact_repo.New(pool)` (new import `contact_repo "github.com/hekemen/automata/internal/infrastructure/contact/repo"`; note existing `contact` import is the infra package used for migrations) and use `handler.NewFormHandlerWithIdentifiers(formRepo, contactRepo, trackingRepo)`.

5. Focused cicd → green; `go build ./...` → clean. Commit: `feat(form): identify shadow user as contact on form submit`.

---

## Task 4 — Context-scoped visitors API

**Files:**
- `internal/adapter/api/server.go`
- `cicd/shadow_user_test.go` (extend)

**Steps:**

1. **Test first** — extend `cicd/shadow_user_test.go`:
   - Engine with `c.Set("context_id", ctxID)` middleware; mount `trackingHandler.ListVisitors` at `GET /context/tracking/visitors` and `trackingHandler.GetVisitorEvents` at `GET /context/tracking/visitors/:id/events` (construct `handler.NewTrackingHandler(trepo.New(pool))`).
   - Seed two `POST /track` events for two visitor IDs.
   - `GET /context/tracking/visitors?limit=10` → 200, `total == 2`, each visitor has `LastIP` and `ContactID` fields present (null/empty ok).
   - `GET /context/tracking/visitors/<vid>/events` → 200, `events[0].IP` == the IP from Task 1.

2. **Implement** — in `server.go` context tracking group (lines 195-198), add:
   ```go
   api.GET("/context/tracking/visitors", middleware.AuthMiddleware(authService), trackingHandler.ListVisitors)
   api.GET("/context/tracking/visitors/:id/events", middleware.AuthMiddleware(authService), trackingHandler.GetVisitorEvents)
   ```

3. Focused cicd → green. Commit: `feat(api): context-scoped visitors + visitor events routes`.

---

## Task 5 — UI: merge Dashboard + Tracking into "Dashboard"

**Files:**
- `webui/src/types/api.ts`
- `webui/src/api/client.ts`
- `webui/src/views/DashboardView.vue` (rewrite)
- `webui/src/views/TrackingView.vue` (delete)
- `webui/src/router/index.ts`
- `webui/src/components/AppSidebar.vue`

**Steps:**

1. **Types** (`types/api.ts`):
   - Add to `TrackingEvent`: `IP: string;`
   - Add:
     ```ts
     export interface Visitor {
       ID: string;
       ContextID: string;
       CookieValue: string;
       Fingerprint: string;
       FirstSeen: string;
       LastSeen: string;
       PageViews: number;
       LastIP: string;
       ContactID: string;
     }
     ```

2. **Client** (`client.ts`, next to `getTrackingEvents`):
   ```ts
   export async function listVisitors(page = 1, limit = 20, sort = 'last_seen'): Promise<PaginatedResponse<Visitor>> {
     const params: Record<string, string> = { offset: String((page - 1) * limit), limit: String(limit), sort }
     const response = await client.get('/context/tracking/visitors', { params })
     const d = response.data
     return { data: d.visitors ?? [], total: d.total ?? 0, page, limit }
   }

   export async function getVisitorEvents(visitorId: string, page = 1, limit = 20): Promise<PaginatedResponse<TrackingEvent>> {
     const params: Record<string, string> = { offset: String((page - 1) * limit), limit: String(limit) }
     const response = await client.get(`/context/tracking/visitors/${visitorId}/events`, { params })
     const d = response.data
     return { data: d.events ?? [], total: d.total ?? 0, page, limit }
   }
   ```
   (import `Visitor` type).

3. **DashboardView.vue** — rewrite as a tabbed page (`const tab = ref<'overview' | 'events' | 'visitors'>('overview')`, tab buttons in the header row):
   - **Overview tab**: keep existing stat cards (Total Visitors, Page Views, Contacts, Forms, Banners) **minus** the fake `+12.5% from last week` line and the fake `Avg. CTR 3.2%` card. Keep the Top Pages card (real `top_pages` data). Add the TrackingView content: Page Views bar chart (from `top_pages`, real data — delete the hardcoded Mon–Sun `pageViews` array), Devices breakdown card (`device_breakdown`), Top Referrers card (`top_referrers`). Copy those computed mappings + templates from `TrackingView.vue` (`loadStats` logic, `maxPageViews`, `deviceBreakdown`, `referrers`).
   - **Events tab**: TrackingView's Events Log DataTable (columns Type/URL/Event/Referrer/Time) + pagination (`usePagination`), loaded via `getTrackingEvents`.
   - **Visitors tab**: DataTable with columns: `ID` (first 8 chars, `:title="row.ID"`), `FirstSeen`, `LastSeen`, `PageViews`, `LastIP`, `Identified`. Identified cell: `<Badge variant="success">Identified</Badge>` when `row.ContactID`, else `<Badge variant="default">Anonymous</Badge>`; when identified, wrap in `<RouterLink to="/contacts">`. Pagination via `usePagination` + `listVisitors`.
   - All three tabs react to `authStore.currentContextId` changes (single `watch` that reloads the active tab's data).
   - Delete `TrackingView.vue`.

4. **Router**: remove the `{ path: 'tracking', ... }` entry.

5. **AppSidebar**: remove `{ name: 'Tracking', path: '/tracking', icon: BarChart3 }` and drop `BarChart3` from imports if unused.

6. Gate: `cd webui && npx vite build` → succeeds. Commit: `feat(webui): merge Dashboard and Tracking into tabbed Dashboard`.

---

## Task 6 — UI: dedicated Add Contact page

**Files:**
- `webui/src/views/AddContactView.vue` (new)
- `webui/src/views/ContactsView.vue`
- `webui/src/router/index.ts`

**Steps:**

1. **New `AddContactView.vue`** — Card titled "Add Contact" with the modal's field set as a full page: First Name, Last Name, Email (required), Phone; `formError` display; footer buttons Cancel (`router.push('/contacts')`) and Create (`createContact(form.value)` → `router.push('/contacts')` on success, show `e.response?.data?.error` on failure). Mirror the `Input` component usage from the ContactsView modal.

2. **ContactsView.vue**:
   - "Add Contact" button → `router.push('/contacts/new')` (import `useRouter`).
   - Remove `openCreate`, `isEdit` (modal is edit-only; `openEdit` sets it, template title can be static "Edit Contact"), and the create branch in `handleSave` (only `updateContact` remains).
   - Keep the edit modal, list, search, pagination, delete.

3. **Router**: add child route `{ path: 'contacts/new', name: 'AddContact', component: () => import('@/views/AddContactView.vue') }` (before any `contacts/:id` if one is added later).

4. Gate: `npx vite build` → succeeds. Commit: `feat(webui): dedicated add-contact page`.

---

## Task 7 — Full verification + deploy

1. `go build ./...` and `go vet ./...` (ignore pre-existing `proxy_test.go:23` vet failure).
2. Focused cicd: `go test ./cicd/ -run TestCICD -args -ginkgo.focus="Shadow User Tracking"`.
3. Full cicd: `go test ./cicd/ -run TestCICD` → must match baseline **320 passed | 10 failed** (10 pre-existing banner failures).
4. `cd webui && npx vite build`.
5. Commit any stragglers. **Do not commit WIP files** — stage explicitly by path.
6. Deploy from committed HEAD (isolated worktree `/tmp/opencode/automata-deploy` — recreate worktree if pruned):
   - `git -C /tmp/opencode/automata-deploy fetch && git -C /tmp/opencode/automata-deploy reset --hard <HEAD-sha>`
   - `CGO_ENABLED=0 go build -o automata ./cmd/automata && CGO_ENABLED=0 go build -o bff ./cmd/bff` (verify `file` = statically linked)
   - `docker build -f /tmp/opencode/Dockerfile.deploy -t ghcr.io/hekemen/automata:latest -t ghcr.io/hekemen/automata:<sha> .` → push both tags
   - `kubectl -n automata rollout restart deployment/automata-automata deployment/automata-bff`
7. Live verification (text/DOM/HTTP only — no screenshots):
   - `curl -s -o /dev/null -w '%{http_code}' -X POST http://<tracking>/track -H 'Content-Type: application/json' -d '{"contextId":"8c61ddc4-debd-445e-b90b-3c62f89dfcfc","type":"page_view","url":"/deploy-check","visitor_id":"deploy-check-1"}'` → 204; then psql: `SELECT ip FROM tracking_events ORDER BY created_at DESC LIMIT 1;` non-empty; `SELECT last_ip, contact_id FROM tracking_visitors WHERE cookie_value='deploy-check-1';`
   - Form identification: `curl -s -X POST http://<tracking>/form/submit/test -H 'Content-Type: application/json' -H 'X-Context-ID: 8c61ddc4-debd-445e-b90b-3c62f89dfcfc' -d '{"data":{"email":"deploy-check@example.com","first_name":"Deploy"},"visitor_id":"deploy-check-1"}'` → 201; psql: visitor `contact_id` set; contact row has `source='form:test'`.
   - UI: `curl -s http://automata.home.arpa/` (HTTP, not HTTPS) → login page HTML; after login (Playwright DOM snapshot, no screenshots) confirm: Dashboard shows tabs Overview/Events/Visitors, no "Tracking" sidebar entry, `/contacts/new` page renders, Contacts "Add Contact" navigates to `/contacts/new`.
8. Record Execution Status section in this plan file.

## Execution Status

**Complete** — 2026-10-16, branch `phase2-auth-changes`, commits `0801aa9` → `0f154ae` (Tasks 1–6) + plan commit.

- Task 1 (storage: ip/last_ip/contact_id + IP capture): `0801aa9` — review Approved
- Task 2 (form JS sends visitor_id): `01d8626` — review Approved
- Task 3 (form submit identifies visitor; incl. carried ruling: `SetVisitorContactID` matches `cookie_value`): `28f5d6b` — review Approved
- Task 4 (context-scoped visitors API): `11beab0` — review Approved (incl. minimal fixes to latent nil-ctx/NULL-scan bugs the new routes exposed)
- Task 5 (Dashboard + Tracking merged into tabbed Dashboard): `d9dde15` — review Approved
- Task 6 (dedicated Add Contact page): `0f154ae` — review Approved (incl. pre-existing missing Modal import fix)
- Task 7 (verification + deploy): `go build` clean; `go vet` only pre-existing `proxy_test.go:23`; focused cicd green; full cicd **326 passed | 10 failed** (baseline 320|10 + 6 new specs; all 10 failures pre-existing banner/campaign/placement); `vite build` clean; deployed from committed HEAD; live checks passed (see ledger `.superpowers/sdd/2026-10-16-shadow-user-tracking/progress.md`).