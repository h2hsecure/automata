# Page/Banner Merge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Merge `Banner` into `Page` — one unified `Page` entity where a banner is just a page with `position` (`""|top|bottom|left|right|full`) and `buttons[]`; retire all banner/placement/campaign code, endpoints, and tables.

**Architecture:** The `page` domain package absorbs `Button`, `Decision`, and `Impression` types from the `banner` package. The page Postgres repo gains `position`/`buttons` columns plus `page_decisions`/`page_impressions` tables (and drops the six `banner_*` tables). Public tracking routes move from `/banner/*` to `/page/config|decision|snippet/*` alongside the existing `/page/:ctx/:slug`. The webui `BannersView` is deleted and its position/buttons editor moves into `PagesView`. No data migration — prod has no banner data.

**Tech Stack:** Go (gin, pgx/v5, zerolog), PostgreSQL, Vue 3 + TypeScript + Vitest (webui), Helm chart, nginx (website repo), Ginkgo/Gomega (cicd suite, real Postgres).

**Spec:** `/home/hekemen/.opencode/plan/page-banner-merge.md`

## Global Constraints

- No image-based verification anywhere: verify via DOM/text/HTTP only (curl, accessibility snapshots, reading source).
- `Button` JSON shape stays exactly `{id, label, action, link_url, dismisses}` — copied verbatim from the current `banner.Button`.
- Position values are exactly `""`, `top`, `bottom`, `left`, `right`, `full`. `fullpage` is retired; `left`/`right` render as ~320px fixed edge strips, `full` as a full-viewport opaque overlay.
- gin ≥ 1.12: every public GET route must also register an explicit HEAD route.
- Decision/impression tracking keeps the trust model: anonymous `visitor_id`, SHA-256 IP hash, no PII; decision POST body is JSON in a `text/plain` Content-Type (no CORS preflight).
- CI tags are 7-char (`git rev-parse --short HEAD`); AGENTS.md says `sha8` — CI wins.
- Pre-existing failures to NOT chase: `TestContextMiddleware`, `proxy_test.go` build failure, cicd batch test, pre-existing gofmt-unformatted files.
- Old `git stash@{0}` — do not pop. Push with explicit `origin main`.
- Hexagonal layout: domain in `internal/domain/page`, repo in `internal/infrastructure/page/repo`, usecase in `internal/usecase/page`, adapters in `internal/adapter/{api,tracking}`.

## Review Focus

1. **Unknown/invalid position** (e.g. `"fullpage"`, `"topp"` sent via API): `Page.Validate()` rejects at create/update (400); the snippet does not render a page with an unknown position (no CSS branch matches → skip). Pinned in Task 1 (domain) and Task 5 (snippet) tests.
2. **HEAD requests** to `/page/config/:ctx` and `/page/snippet/:ctx.js`: must return 200, not 404, because gin ≥ 1.12 does not auto-map HEAD. Pinned in Task 6 tests.
3. **Legacy URL `/banner/snippet/:ctx.js` after deploy**: clean 404 JSON response, no panic, no partial snippet. Pinned in Task 7 cicd test.
4. **Positioned page with no buttons / no `dismisses` button**: snippet renders it and it stays visible (blocking is intended for `full`); no crash, no empty-button crash when `buttons` is `null` in JSONB. Pinned in Task 5 tests.
5. **Deleting a positioned page**: its `page_decisions` and `page_impressions` rows cascade-delete (FK `ON DELETE CASCADE`), so the admin decisions endpoint can never 500 on stale rows. Pinned in Task 2 cicd repo test.

---

### Task 1: Domain — extend `page` package with Position, Button, Decision, Impression

**Files:**
- Modify: `internal/domain/page/page.go`
- Modify: `internal/domain/page/repository.go`
- Test: `internal/domain/page/page_test.go`

**Interfaces:**
- Consumes: nothing (foundation task).
- Produces: `page.Position` + constants `PositionNone|PositionTop|PositionBottom|PositionLeft|PositionRight|PositionFull`, `Position.Valid() bool`, `page.Button{ID,Label,Action,LinkURL string; Dismisses bool}` with json tags `id/label/action/link_url/dismisses`, `page.Decision{ID,ContextID,PageID,VisitorID,Action,IPHash,UserAgent string; CreatedAt time.Time}`, `page.Impression{ID,ContextID,PageID,VisitorID string; CreatedAt time.Time}`, extended `page.Page{..., Position Position, Buttons []Button}`, and the extended `page.Repository` interface (methods below). Every later task uses these exact names.

- [ ] **Step 1: Write the failing test** in `internal/domain/page/page_test.go`

Add tests (keep existing ones):
- `TestValidatePositions`: a Page with each of `""`, `top`, `bottom`, `left`, `right`, `valid full` passes `Validate()`; positions `fullpage`, `topp`, `BOTTOM` fail with an error containing `position`.
- `TestPositionValid`: `Position("top").Valid()` true, `Position("fullpage").Valid()` false.

```go
func TestValidatePositions(t *testing.T) {
	valid := []page.Position{"", "top", "bottom", "left", "right", "full"}
	for _, pos := range valid {
		p := &page.Page{Slug: "s", Title: "t", Position: pos}
		if err := p.Validate(); err != nil {
			t.Errorf("position %q should be valid, got %v", pos, err)
		}
	}
	invalid := []page.Position{"fullpage", "topp", "BOTTOM"}
	for _, pos := range invalid {
		p := &page.Page{Slug: "s", Title: "t", Position: pos}
		if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "position") {
			t.Errorf("position %q should be invalid with 'position' in error, got %v", pos, err)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/page/ -run 'TestValidatePositions|TestPositionValid' -v`
Expected: FAIL — `Position` undefined / compile error.

- [ ] **Step 3: Implement in `internal/domain/page/page.go`**

Add the `Position` type + six constants + `Valid()` method, `Button`, `Decision`, `Impression` structs (Button json tags exactly `id/label/action/link_url/dismisses`). Extend `Page` with `Position Position` and `Buttons []Button` fields. Extend `Validate()` to also reject `!p.Position.Valid()` (error text must contain `position`).

- [ ] **Step 4: Extend the Repository interface** in `internal/domain/page/repository.go`

```go
type Repository interface {
	Create(p *Page) error
	GetByID(id string) (*Page, error)
	GetBySlug(contextID, slug string) (*Page, error)
	List(contextID string) ([]*Page, error)
	ListActive(contextID string) ([]*Page, error)
	Update(p *Page) error
	Delete(id string) error
	RecordImpression(contextID, pageID, visitorID string) error
	RecordDecision(d *Decision) error
	ListDecisions(pageID string, offset, limit int) ([]*Decision, int64, error)
	DecisionCounts(pageID string) (map[string]int64, error)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/domain/page/ -v`
Expected: PASS (the `repo` type in the infra package will not compile until Task 2 — that is expected; only test the domain package here).

- [ ] **Step 6: Commit**

```bash
git add internal/domain/page/
git commit -m "feat(page): add Position, Button, Decision, Impression to page domain"
```

---

### Task 2: Infrastructure — page repo: position/buttons columns, decisions/impressions tables, drop banner tables

**Files:**
- Modify: `internal/infrastructure/page/repo/migration.sql`
- Modify: `internal/infrastructure/page/repo/postgres.go`
- Test: `cicd/page_repo_test.go` (new — cicd suite provides the real `pool`)

**Interfaces:**
- Consumes: `page.Repository` (Task 1), `page.Page{Position, Buttons}`, `page.Decision`.
- Produces: `repo.New(pool) page.Repository` implementing all ten methods; `RunMigrations(pool)` applying the new migration (columns + `page_decisions` + `page_impressions` + drops of the six `banner_*` tables).

- [ ] **Step 1: Write the failing test** `cicd/page_repo_test.go`

Follow the harness pattern of `cicd/banner_repo_test.go` (Ginkgo `Describe`, shared `pool`, `support.NewTestContextID()`):
- Round-trip: create a page with `Position: "left"` and two `Button`s via `repo.Create`, then `GetByID` returns the same position and buttons (compare label/action/dismisses); `GetBySlug` and `List` also return them.
- Null buttons: create a page with `Buttons: nil`, `GetByID` returns a non-nil empty or nil slice without error.
- `ListActive` returns only `IsActive: true` pages for the context.
- Decisions: `RecordDecision` twice (actions `accept`, `decline`) for the page; `ListDecisions(id, 0, 10)` returns 2 rows + total 2; `DecisionCounts(id)` = `{"accept":1, "decline":1}`.
- Cascade (Review Focus 5): create a page, record one decision + one impression (`RecordImpression`), `repo.Delete(page.ID)`, then `ListDecisions` returns 0 rows and no error.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cicd/ -run TestCicd --ginkgo.focus "page repo" 2>&1 | tail -20` (use the suite runner name from `cicd/suite_test.go`)
Expected: FAIL — methods not implemented / columns missing.

- [ ] **Step 3: Extend `migration.sql`**

Append (order matters — children before parents):

```sql
ALTER TABLE page_pages ADD COLUMN IF NOT EXISTS position VARCHAR(16) DEFAULT '';
ALTER TABLE page_pages ADD COLUMN IF NOT EXISTS buttons JSONB DEFAULT '[]';

CREATE TABLE IF NOT EXISTS page_decisions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id  UUID NOT NULL,
    page_id     UUID NOT NULL REFERENCES page_pages(id) ON DELETE CASCADE,
    visitor_id  VARCHAR(128),
    action      VARCHAR(128),
    ip_hash     VARCHAR(64),
    user_agent  VARCHAR(512),
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS page_impressions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id  UUID NOT NULL,
    page_id     UUID NOT NULL REFERENCES page_pages(id) ON DELETE CASCADE,
    visitor_id  VARCHAR(128),
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_page_decisions ON page_decisions(page_id, created_at);
CREATE INDEX IF NOT EXISTS idx_page_impressions ON page_impressions(page_id, created_at);

-- Retire banner subsystem (banner_banners references banner_campaigns and is
-- referenced by the three event tables, so drop in this order).
DROP TABLE IF EXISTS banner_impressions;
DROP TABLE IF EXISTS banner_clicks;
DROP TABLE IF EXISTS banner_decisions;
DROP TABLE IF EXISTS banner_banners;
DROP TABLE IF EXISTS banner_campaigns;
DROP TABLE IF EXISTS banner_placements;
```

- [ ] **Step 4: Extend `postgres.go`**

- `Create`: insert `position, buttons` (encode `p.Buttons` with `json.Marshal`, use `[]byte`; empty/nil slice → `"[]"`).
- `GetByID` / `GetBySlug` / `List` / `Update` / `scanPage`: add `position, buttons` to the column lists; scan `position` into a `string` then cast to `page.Position`, and `buttons` into `[]byte` then `json.Unmarshal` into `p.Buttons` (treat `NULL`/empty as empty slice).
- Add `ListActive(contextID)` — `WHERE context_id = $1 AND is_active = TRUE ORDER BY created_at`.
- Add `RecordImpression(contextID, pageID, visitorID)` — insert into `page_impressions` with `gen_random_uuid()`.
- Add `RecordDecision(d *page.Decision)` — insert into `page_decisions` (port the SQL from `internal/infrastructure/banner/repo/postgres.go:421`, renaming table + `banner_id` → `page_id`).
- Add `ListDecisions(pageID, offset, limit)` and `DecisionCounts(pageID)` — port from banner repo `postgres.go:437` and `:476` with the same renames; `ListDecisions` returns rows + total count.

- [ ] **Step 5: Run test to verify it passes**

Run: same command as Step 2
Expected: PASS. Also run `go build ./internal/...` — the infra package compiles; `internal/adapter/...` and `cicd` may still fail because they reference banner types (fixed in Tasks 4 and 7).

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/page/ cicd/page_repo_test.go
git commit -m "feat(page): repo position/buttons columns, page_decisions/page_impressions, drop banner tables"
```

---

### Task 3: Usecase — CreatePageInput gains Position and Buttons

**Files:**
- Modify: `internal/usecase/page/create_page.go`
- Modify: `internal/usecase/page/update_page.go`
- Test: `internal/usecase/page/page_test.go`

**Interfaces:**
- Consumes: `page.Page{Position, Buttons}`, `page.Repository` (Task 1).
- Produces: `pageuc.CreatePageInput{Slug, Title, Content string; IsActive *bool; Position page.Position; Buttons []page.Button}` used by the admin handler in Task 4.

- [ ] **Step 1: Write the failing test** in `internal/usecase/page/page_test.go`

Add a test using the existing fake repo pattern in that file: `CreatePage` with `Position: "bottom"` and one button stores both on the returned page; `UpdatePage` changes position from `""` to `full` and replaces buttons.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/usecase/page/ -v`
Expected: FAIL — `CreatePageInput` has no `Position` field.

- [ ] **Step 3: Implement**

Add `Position page.Position` and `Buttons []page.Button` to `CreatePageInput`. In `CreatePage`, set `p.Position = input.Position` and `p.Buttons = input.Buttons` (nil stays nil — repo stores `[]`). In `UpdatePage`, set the same two fields on the loaded page before `repo.Update`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/usecase/page/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/usecase/page/
git commit -m "feat(page): usecase create/update accept position and buttons"
```

---

### Task 4: Admin API — page handler gains position/buttons + decisions endpoint; banner routes retired

**Files:**
- Modify: `internal/adapter/api/handler/page_handler.go`
- Modify: `internal/adapter/api/server.go` (banner route block lines 247–268, banner handler construction ~line 168, `bannerRepo` import line 18)
- Test: `internal/adapter/api/handler/page_handler_test.go` (existing file — extend)

**Interfaces:**
- Consumes: `pageuc.CreatePageInput` with `Position`/`Buttons` (Task 3), `page.Repository.ListDecisions/DecisionCounts` (Task 1).
- Produces: `GET /api/context/pages/:id/decisions?offset=&limit=` → `200 {"decisions": [...], "counts": {...}, "total": N}` (404 if page missing or in another context); `POST/PUT /api/context/pages` accept `position` (string) and `buttons` (array) in the JSON body; all `/api/context/banners|placements|campaigns` routes gone.

- [ ] **Step 1: Write the failing tests** in `internal/adapter/api/handler/page_handler_test.go`

Follow the existing test patterns in that file (fake repo + gin test mode):
- Create with `{"slug":"s","title":"t","position":"left","buttons":[{"label":"A","action":"accept","dismisses":true}]}` → 201, returned body has `Position: "left"` and the button.
- Create with `"position":"fullpage"` → 400 (invalid position, from domain `Validate`).
- `ListDecisions` on a page with two recorded decisions → 200 with `decisions` array length 2, `counts` map, `total: 2`.
- `ListDecisions` with a page ID from a *different* context → 404.
- `ListDecisions` for a non-existent ID → 404.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/api/handler/ -run 'TestPageHandler' -v`
Expected: FAIL — no `position` field on `pageInput`, no `ListDecisions` method.

- [ ] **Step 3: Extend the handler**

In `page_handler.go`: add `Position string \`json:"position"\`` and `Buttons []page.Button \`json:"buttons"\`` to `pageInput`; pass `page.Position(input.Position)` and `input.Buttons` into `pageuc.CreatePageInput` in both `Create` and `Update`. Add method:

```go
func (h *PageHandler) ListDecisions(c *gin.Context)
```

— port the logic from `internal/adapter/api/handler/banner_handler.go:140-179` (offset/limit query parsing, context-ownership check via `repo.GetByID`, `repo.ListDecisions`, `repo.DecisionCounts`), returning `{"decisions", "counts", "total"}`.

- [ ] **Step 4: Retire banner routes in `server.go`**

Delete the banner route block (lines 247–268: `bannerHandler`, `placementHandler`, `campaignHandler` and all 16 routes), the `bannerRepo` construction (~line 168) and import (line 18), and any now-unused `handler`/`bannerRepo` variables. In the page route block add:

```go
api.GET("/context/pages/:id/decisions", middleware.AuthMiddleware(authService), pageHandler.ListDecisions)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/adapter/api/... -v` and `go build ./cmd/...`
Expected: handler tests PASS. `cmd/automata` still fails to build (main.go references banner handlers) — fixed in Task 7.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/api/
git commit -m "feat(api): pages accept position/buttons, add decisions endpoint, retire banner routes"
```

---

### Task 5: Snippet — `page_generator.go` with five position branches

**Files:**
- Create: `pkg/snippet/page_generator.go`
- Test: `pkg/snippet/page_generator_test.go` (new)
- Delete (Task 7, not here — banner generator stays until wiring is done): `pkg/snippet/banner_generator.go`

**Interfaces:**
- Consumes: the `quote()` helper from `pkg/snippet/snippet_handler.go` (same package).
- Produces: `snippet.GeneratePageConfigSnippet(contextID string) (string, error)` — returns JS that fetches `/page/config/<ctx>?visitor_id=...`, iterates `cfg.pages`, skips pages with empty/unknown position or an existing `ab_<pageId>` cookie, renders per position, and posts decisions to `/page/decision/<ctx>` as JSON in `text/plain` body `{page_id, action, visitor_id}`.

- [ ] **Step 1: Write the failing tests** `pkg/snippet/page_generator_test.go`

Assert on the returned JS string (same style as `pkg/snippet/banner_generator_test.go`):
- Contains `/page/config/<ctx>` and `/page/decision/<ctx>`; does NOT contain `/banner/`.
- `positionCss` branches: for `top` → `top:0`, `bottom` → `bottom:0`, `left` → `left:0` + `width:320px`, `right` → `right:0` + `width:320px`, `full` → `top:0;left:0;right:0;bottom:0` with the opaque `rgba(15,15,15,0.92)` background.
- Unknown positions: the JS `positionCss` returns `""` (falsy) for anything not in the five, and `renderPage` returns early on falsy CSS (Review Focus 1). Add an assertion that the generated JS contains the guard.
- Null buttons (Review Focus 4): JS iterates `b.buttons||[]` (assert the literal `||[]` is present).
- Empty `contextID` → error (mirrors current validation test).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/snippet/ -run TestGeneratePageConfigSnippet -v`
Expected: FAIL — function not defined.

- [ ] **Step 3: Implement `page_generator.go`**

Port `pkg/snippet/banner_generator.go` with these changes:
- Function `GeneratePageConfigSnippet`, URLs `/page/config/` + `/page/decision/`.
- Iterate `var pages=cfg.pages||[]`.
- `positionCss(p)` returns `""` for unknown positions; `top`/`bottom` as today; `left`: `position:fixed;left:0;top:0;bottom:0;width:320px;z-index:2147483000;`; `right`: same with `right:0`; `full`: today's fullpage overlay.
- `renderPage(p)`: skip if `!positionCss(p.position)` (covers empty AND unknown — Review Focus 1) or if `_getCookie("ab_"+p.id)`.
- `left`/`right` cards: full-height, `overflow-y:auto`, padding; `full` keeps the 480px centered card; `top`/`bottom` keep the current bar card.
- Decision body: `JSON.stringify({page_id:p.id,action:btn.action,visitor_id:visitorId})`.
- Keep: `automata_visitor` cookie (16-hex, 365d), `ab_<pageId>` decision cookie, `text/plain` POST headers, `link_url` starting with `/` → navigate to `/page/<ctx><link_url>`, `b.content+(b.page||"")` inlining, `data-automata-page` attribute on the wrapper (rename from `data-automata-banner`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/snippet/ -v`
Expected: PASS (banner generator tests still pass — that file is untouched).

- [ ] **Step 5: Commit**

```bash
git add pkg/snippet/page_generator.go pkg/snippet/page_generator_test.go
git commit -m "feat(snippet): self-rendering page snippet with top/bottom/left/right/full positions"
```

---

### Task 6: Public tracking handler — `/page/config`, `/page/decision`, `/page/snippet`

**Files:**
- Create: `internal/adapter/tracking/page_public_handler.go`
- Test: `internal/adapter/tracking/page_public_handler_test.go` (new)

**Interfaces:**
- Consumes: `page.Repository` (Tasks 1–2), `snippet.GeneratePageConfigSnippet` (Task 5).
- Produces:
  - `tracking.NewPagePublicHandler(repo pagedomain.Repository) *PagePublicHandler`
  - `tracking.RegisterPagePublicRoutes(r *gin.Engine, h *PagePublicHandler)` registering GET+HEAD `/page/config/:contextID`, POST `/page/decision/:contextID`, GET+HEAD `/page/snippet/:contextID.js`.
  - `GET /page/config/:ctx` → `200 {"pages":[{id,title,slug,position,content,buttons,page?}]}` (all active pages; `page` = inlined content of the first button whose `link_url` is a page slug; recorded as impression when `visitor_id` query param present).
  - `POST /page/decision/:ctx` body `{page_id, action, visitor_id}` → 204; 400 on missing fields/bad JSON; 404 when page missing or context mismatch.

- [ ] **Step 1: Write the failing tests** `internal/adapter/tracking/page_public_handler_test.go`

Use a fake `page.Repository` (same style as `internal/adapter/tracking/banner_public_handler_test.go`):
- Config: two active pages (one `bottom` with a button linking `/cookie-policy`, one plain) → 200, `pages` array has both, the positioned one has `page` inlined from the slug page's content, `position: "bottom"`.
- Config HEAD → 200 (Review Focus 2).
- Config with `?visitor_id=v1` → fake repo records one impression per page.
- Decision: valid body → 204 and fake repo received `Decision{PageID, Action, VisitorID, ContextID}`; missing `action` → 400; page from other context → 404; unknown ID → 404.
- Snippet: 200, `Content-Type` `application/javascript`, body contains `/page/config/<ctx>`; snippet HEAD → 200 (Review Focus 2); path without `.js` → 400.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/tracking/ -run TestPagePublicHandler -v`
Expected: FAIL — `NewPagePublicHandler` undefined.

- [ ] **Step 3: Implement `page_public_handler.go`**

Port `internal/adapter/tracking/banner_public_handler.go` with:
- `PagePublicHandler{repo pagedomain.Repository}` (single repo — banner repo gone).
- `Config`: `h.repo.ListActive(contextID)`; per page, if `visitor_id` query present call `RecordImpression(contextID, p.ID, visitorID)`; output `gin.H{"id","title","slug","position","content","buttons","page": h.inlinePage(...)}`; respond `gin.H{"pages": out}`.
- `inlinePage(contextID string, buttons []page.Button) *string` — same slug-inlining logic as today (`strings.HasPrefix(btn.LinkURL, "/")`, `GetBySlug`, active check).
- `Decision`: body `{page_id, action, visitor_id}`; require both `page_id` and `action`; `repo.GetByID` + context check; `RecordDecision` with `IPHash: ipHash(c.ClientIP())` (reuse the `ipHash` helper in the same package) and `UserAgent` header; 204.
- `ServeSnippet`: same path-slicing with prefix `/page/snippet/`; `snippet.GeneratePageConfigSnippet(contextID)`.
- Keep GET+HEAD pairs for all three routes (Review Focus 2).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/tracking/ -v`
Expected: PASS (banner handler tests still pass — file untouched until Task 7).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/tracking/page_public_handler.go internal/adapter/tracking/page_public_handler_test.go
git commit -m "feat(tracking): public /page/config, /page/decision, /page/snippet routes"
```

---

### Task 7: Delete the banner subsystem — code, wiring, tests (one compilable unit)

**Files:**
- Delete: `internal/domain/banner/`, `internal/infrastructure/banner/`, `internal/usecase/banner/`, `internal/adapter/api/handler/banner_handler.go` + `banner_handler_test.go`, `internal/adapter/tracking/banner_public_handler.go` + `banner_public_handler_test.go`, `pkg/snippet/banner_generator.go` + `banner_generator_test.go`, `cicd/banner_repo_test.go`, `cicd/banner_position_repo_test.go`, `cicd/banner_campaign_http_test.go`
- Modify: `internal/adapter/tracking/server.go`, `cmd/automata/main.go`, `internal/domain/contact/activity.go`, `cicd/banner_pages_integration_test.go` → renamed to `cicd/pages_integration_test.go`, plus every cicd file that calls `tracking.NewServer(...)` with the old signature: `cicd/form_snippet_test.go`, `cicd/shadow_user_test.go`, `cicd/tracking_server_test.go`, `cicd/hidden_contact_test.go`, `cicd/tracking_http_test.go`, `internal/adapter/tracking/server_test.go` (if it references banner handlers)
- Test: `cicd/pages_integration_test.go` (the rewrite)

**Interfaces:**
- Consumes: everything from Tasks 1–6.
- Produces: `tracking.NewServer(formHandler *handler.FormHandler, trackingRepo Repository, geoLookup geodomain.Lookup, contactRepo ...ContactRepository) *gin.Engine` — new signature, no banner/placement/campaign handlers, no `/track/banner/*`, `/track/placement/*`, `/track/campaign/*` routes. `contact.ActivityPageClicked = "page_clicked"` and `contact.ActivityPageShown = "page_shown"` (renamed from `ActivityBannerClick`/`ActivityBannerImpression`).

- [ ] **Step 1: Rewrite the integration test** — `git mv cicd/banner_pages_integration_test.go cicd/pages_integration_test.go`, then rewrite as `Describe("Pages end-to-end")`:

  - `BeforeEach`: `pageRepo = pagerepo.New(pool)`, `contextID = support.NewTestContextID()`; create via repo: a plain legal page (slug `cookie-policy`, content `<p>policy</p>`) and a positioned page (slug `cookie-banner`, `Position: "bottom"`, buttons `[{Label:"Accept", Action:"accept", Dismisses:true}, {Label:"Policy", Action:"open", LinkURL:"/cookie-policy"}]`); build the tracking engine with `tracking.NewServer(formHandler, trackingRepo, geoLookup)` + `RegisterPageRoutes` + `RegisterPagePublicRoutes(engine, tracking.NewPagePublicHandler(pageRepo))` (new signature — no banner handlers).
  - `It("serves config, decision, snippet, and the plain page")`:
    - `GET /page/config/<ctx>?visitor_id=v1` → 200; `pages` array contains the positioned page with `position:"bottom"`, both buttons, and `page` inlined containing `policy`; plain page present too.
    - `POST /page/decision/<ctx>` body `{"page_id":"<id>","action":"accept","visitor_id":"v1"}` → 204; then `repo.ListDecisions` returns 1 row.
    - `GET /page/snippet/<ctx>.js` → 200, body contains `/page/config/<ctx>`.
    - `GET /page/<ctx>/cookie-policy` → 200 HTML containing `policy`.
  - `It("returns 404 for retired banner URLs")` (Review Focus 3): `GET /banner/config/<ctx>` → 404; `GET /banner/snippet/<ctx>.js` → 404; `GET /track/banner/list` → 404.

- [ ] **Step 2: Delete banner code and update wiring**

  - Delete all files listed in **Files** above (use `git rm`).
  - `internal/adapter/tracking/server.go`: change `NewServer` to the new signature; delete the "Banner endpoints" route block (lines 39–47) and the banner handler parameters/imports.
  - `cmd/automata/main.go`: delete `bannerRepo` (line 123), `bannerHandler`/`placementHandler`/`campaignHandler` (169–171), the banner imports; change `tracking.NewServer(formHandler, trackingRepo, geoLookup, contactRepo)` (line 224); replace line 228 with `tracking.RegisterPagePublicRoutes(trackingEngine, tracking.NewPagePublicHandler(pageRepo))`.
  - `internal/domain/contact/activity.go`: rename `ActivityBannerClick` → `ActivityPageClicked` (value `"page_clicked"`), `ActivityBannerImpression` → `ActivityPageShown` (value `"page_shown"`). (Nothing in Go code emits them — webui labels updated in Task 8.)
  - Fix every remaining `tracking.NewServer(...)` call site in the cicd files listed (drop the three banner handler args and their construction lines, e.g. `bannerRepo := brepo.New(pool)` / `bhandler.NewBannerHandler(...)` etc.).

- [ ] **Step 3: Verify the whole repo builds and tests pass**

Run: `go build ./... && go vet ./... && go test ./internal/... ./pkg/...`
Expected: PASS (ignore the pre-existing failures listed in Global Constraints).

Run: `go test ./cicd/ 2>&1 | tail -30` (the full Ginkgo suite)
Expected: PASS for `Pages end-to-end` and all non-banner suites; only pre-existing failures remain.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor: merge banner into page — delete banner domain, infra, usecase, handlers, routes, tables"
```

---

### Task 8: WebUI — merge BannersView into PagesView

**Files:**
- Delete: `webui/src/views/BannersView.vue`
- Modify: `webui/src/views/PagesView.vue`, `webui/src/views/PagesView.spec.ts`, `webui/src/views/DashboardView.vue`, `webui/src/views/DashboardView.spec.ts`, `webui/src/views/ContactDetailView.vue`, `webui/src/api/client.ts`, `webui/src/types/api.ts`, `webui/src/router/index.ts`, `webui/src/components/AppSidebar.vue`

**Interfaces:**
- Consumes: admin API from Task 4 — `GET /api/context/pages/:id/decisions` → `{decisions, counts, total}`; page create/update bodies include `position` (string) and `buttons` (array of `{id?, label, action, link_url?, dismisses}`); page list items include `Position: string` and `Buttons: PageButton[]`.
- Produces: one Pages view (list + position badge + buttons editor + decisions panel); no `Banner*` symbols anywhere in `webui/src`.

- [ ] **Step 1: Write the failing specs**

Extend `webui/src/views/PagesView.spec.ts` (follow its existing mock pattern for `listPages` etc.):
- Dialog contains a Position select with options None/Top/Bottom/Left/Right/Full; creating a page sends `position` and `buttons` in the payload.
- A page with `Position: "left"` and 2 buttons shows a position badge and buttons count in the list row.
- Opening the decisions panel for a page calls the mocked `listPageDecisions` and renders the counts.
Update `webui/src/views/DashboardView.spec.ts`: the card links to `/pages` and reads `totalPages` from the real `listPages` mock (drop the `listBanners` mock).

- [ ] **Step 2: Run specs to verify they fail**

Run: `cd webui && npx vitest run src/views/PagesView.spec.ts src/views/DashboardView.spec.ts 2>&1 | tail -30`
Expected: FAIL — `listPageDecisions` missing, no position control.

- [ ] **Step 3: Implement the merge**

  - `types/api.ts`: rename `BannerButton` → `PageButton` (same fields); delete `Banner`, `BannerInput`, `BannerDecision`, `Placement`, `Campaign` types (and any other banner-only types); add to `Page`: `Position: string; Buttons: PageButton[]`; add to `PageInput`: `position?: string; buttons?: PageButton[]`; add `PageDecision` interface (`ID, ContextID, PageID, VisitorID, Action, IPHash, UserAgent, CreatedAt` strings).
  - `client.ts`: delete `listBanners`, `createBanner`, `updateBanner`, `deleteBanner`, `listBannerDecisions` and all placement/campaign functions; add:

    ```ts
    export async function listPageDecisions(
      pageId: string,
      offset = 0,
      limit = 50,
    ): Promise<{ decisions: PageDecision[]; counts: Record<string, number>; total: number }> {
      const response = await client.get(`/context/pages/${pageId}/decisions`, { params: { offset, limit } })
      return response.data
    }
    ```

  - `PagesView.vue`: add `position` column (badge, `—` when empty) and a buttons-count column; dialog gains a Position `<select>` (value `''` = None, options `top/bottom/left/right/full`) and a buttons editor ported from `BannersView.vue` (rows of label/action/link_url/dismisses with add/remove); a per-page decisions action that calls `listPageDecisions` and renders counts + recent rows (reuse the pattern from `BannersView.vue`).
  - `DashboardView.vue`: replace the banners card with a pages card (`totalPages`, link `/pages`, real `listPages`).
  - `ContactDetailView.vue`: event map keys `banner_click` → `page_clicked` (label `Page click`), `banner_impression` → `page_shown` (label `Page shown`).
  - `router/index.ts`: delete the `banners` route; `AppSidebar.vue`: delete the Banners nav item.
  - Delete `webui/src/views/BannersView.vue`.

- [ ] **Step 4: Run specs + typecheck**

Run: `cd webui && npx vitest run 2>&1 | tail -30 && npx vue-tsc --noEmit`
Expected: all specs PASS; typecheck clean (no `Banner*` references remain — `grep -rn "Banner" webui/src` returns nothing).

- [ ] **Step 5: Commit**

```bash
git add webui/src
git commit -m "feat(webui): merge BannersView into PagesView with position and buttons editor"
```

---

### Task 9: Chart, website, runbook

**Files:**
- Modify: `helm/automata/templates/ingress.yaml` (line 99 route list; comments lines 17, 70, 72)
- Modify (website repo at `/home/hekemen/github/h2hsecure.github.io`, branch `static-nginx`): `charts/h2hsecure/templates/tracking.yaml`, the customer-site snippet embed
- Modify: `deploy-prod.md`

**Interfaces:**
- Consumes: deployed API from Tasks 4–7 (routes `/page/config|decision|snippet`, `/page/:ctx/:slug`).
- Produces: ingress routing `/page/` (no `/banner/`); customer site loading `/page/snippet/422adc7a-5b07-49a5-bd00-c6a44d61a172.js`; runbook steps matching the new deploy order.

- [ ] **Step 1: Update the ingress**

In `helm/automata/templates/ingress.yaml`: remove `"/banner/"` from the route list on line 99 (leaving `"/snippet" "/track" "/form/" "/static/" "/page/"`) and update the comments on lines 17 and 70–72 to match.

Verify: `helm lint helm/automata` (or the repo's chart test target from the Makefile) → no errors; `grep -c banner helm/automata/templates/ingress.yaml` → 0.

- [ ] **Step 2: Update the website repo**

In `/home/hekemen/github/h2hsecure.github.io` (branch `static-nginx`):
- `charts/h2hsecure/templates/tracking.yaml`: nginx regex `^/(snippet|track|form|banner|page)` → `^/(snippet|track|form|page)`.
- Customer-site embed: replace `/banner/snippet/422adc7a-5b07-49a5-bd00-c6a44d61a172.js` with `/page/snippet/422adc7a-5b07-49a5-bd00-c6a44d61a172.js` (find the embed in the site source with `grep -rn "banner/snippet" --include="*.html" --include="*.tsx" --include="*.vue" .`).
- Verify (text only, no screenshots): `grep -rn "banner" charts/h2hsecure/templates/tracking.yaml` → 0 hits; the embed now points at `/page/snippet/`.

- [ ] **Step 3: Update `deploy-prod.md`**

Rewrite the banner/page section to the new order: (1) merge + push → CI green; (2) merge website `static-nginx` branch → customer site live with `/page/` nginx + re-embedded snippet; (3) deploy new chart; (4) create the cookie banner as a page in webui (`position: bottom`, accept/decline buttons), verify with `curl -s https://m.h2hsecure.com/page/config/422adc7a-5b07-49a5-bd00-c6a44d61a172 | head` and `curl -sI https://m.h2hsecure.com/page/snippet/422adc7a-5b07-49a5-bd00-c6a44d61a172.js`.

- [ ] **Step 4: Commit both repos**

```bash
git add helm/automata/templates/ingress.yaml deploy-prod.md
git commit -m "chore(deploy): drop /banner/ ingress route, update runbook for page merge"

cd /home/hekemen/github/h2hsecure.github.io
git add charts/h2hsecure/templates/tracking.yaml  # + the embed file(s)
git commit -m "chore: route /page/ only and re-embed snippet at /page/snippet"
```

- [ ] **Step 5: Final verification of the whole repo**

Run: `go build ./... && go test ./internal/... ./pkg/... && cd webui && npx vitest run 2>&1 | tail -15`
Expected: PASS (pre-existing failures per Global Constraints excluded).

---

## Out of Scope

- Data migration (no banner data in prod — fresh start).
- A/B testing, campaigns, placements, images, priority, dates, impression/click counters.
- Legal page content and form logic.
- Prod deployment itself (CI pipeline + user actions per `deploy-prod.md`).