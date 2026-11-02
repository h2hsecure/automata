# Implementation Plan: Configurable Banners, Pages, and Per-Visitor Decisions

**Spec:** `docs/superpowers/specs/2026-11-02-configurable-banners-pages-design.md`

**Goal:** Extend the generic banner system with `position` + `buttons`, add a
per-context `Page` entity with a public serving endpoint, record per-visitor
banner decisions, render banners from a new self-contained snippet, and expose
everything in the dashboard. The cookie-consent case is then a pure
configuration, not code.

**Architecture notes (binding decisions):**

- **Public tracking routes** use the existing `Register*Routes` pattern
  (`internal/adapter/tracking/static_handler.go:29`, called from
  `cmd/automata/main.go:220`). This avoids changing `tracking.NewServer`
  (signature has 7 call sites incl. cicd) — zero call-site churn.
- **Page domain** mirrors the form domain: free-function usecases over a
  `page.Repository` interface, `Validate()` on the entity, `//go:embed`
  migration + advisory lock.
- **`banner_decisions`** lives in the banner repo's `migration.sql`; the three
  new methods go on `banner.Repository` (single implementation:
  `internal/infrastructure/banner/repo/postgres.go`).
- **`ip_hash`** = lowercase hex `sha256(clientIP)`, computed in the decision
  handler (no existing IPHash-computation code exists; `tracking.Event.IPHash`
  is stored but never populated).
- **Config inline page:** for each banner, the first button whose `LinkURL`
  starts with `/` is treated as a page slug (leading `/` trimmed); if the
  page exists and is active its content is inlined as `page`, else `null`.
- **Impressions:** `GET /banner/config/:contextID?visitor_id=…` records a
  `RecordImpression` per returned banner (best-effort, errors ignored).
- **Decision POST body** is JSON with `Content-Type: text/plain` (matches the
  tracking snippet's no-preflight pattern); handler accepts both text/plain
  and application/json.
- **Page serving:** `GET /page/:contextID/:slug` wraps `content` in a minimal
  HTML document, `Content-Type: text/html; charset=utf-8`, 404 when missing
  or inactive.
- **Banner snippet:** new `snippet.GenerateBannerSnippet(contextID)` served at
  `GET /banner/snippet/:contextID.js`. Banners with empty `position` (legacy)
  are skipped by this snippet — placement injection stays as-is.
- **All additive:** existing banner rows (`position=''`, `buttons=[]`) render
  exactly as today.

**Verification commands:**

- `go build ./...` — note: pre-existing build failure in
  `internal/adapter/proxy`; leave untouched.
- `go test ./internal/... ./pkg/...` — note: pre-existing failure
  `TestContextMiddleware` in `internal/adapter/api`; leave untouched.
- `cd webui && npm test` (vitest)
- `make cicd-tests` (ginkgo + testcontainers; 120s timeout)

---

## Phase 1 — Page entity

### Task 1: Page domain + Validate

**Files:**
- Create: `internal/domain/page/page.go`
- Create: `internal/domain/page/repository.go`
- Create: `internal/domain/page/page_test.go`

`internal/domain/page/page.go`:

```go
package page

import (
	"errors"
	"time"
)

// Page is a per-context editable HTML page served publicly at
// /page/:contextID/:slug and editable in the dashboard.
type Page struct {
	ID        string
	ContextID string
	Slug      string
	Title     string
	Content   string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks required fields.
func (p *Page) Validate() error {
	if p.Slug == "" {
		return errors.New("slug required")
	}
	if p.Title == "" {
		return errors.New("title required")
	}
	return nil
}
```

`internal/domain/page/repository.go`:

```go
package page

// Repository defines persistence for pages.
type Repository interface {
	Create(p *Page) error
	GetByID(id string) (*Page, error)
	GetBySlug(contextID, slug string) (*Page, error)
	List(contextID string) ([]*Page, error)
	Update(p *Page) error
	Delete(id string) error
}
```

`internal/domain/page/page_test.go` — table-test `Validate`: missing slug,
missing title, valid page.

**Steps:**
1. `go test ./internal/domain/page/` (fails: package missing)
2. Implement; re-run (pass)
3. Commit: `feat(page): add Page domain entity`

### Task 2: Page repo + migration

**Files:**
- Create: `internal/infrastructure/page/repo/migration.sql`
- Create: `internal/infrastructure/page/repo/postgres.go`

`migration.sql`:

```sql
CREATE TABLE IF NOT EXISTS page_pages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id  UUID NOT NULL,
    slug        VARCHAR(128) NOT NULL,
    title       VARCHAR(255) NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pages_context_slug ON page_pages(context_id, slug);
```

`postgres.go` — copy the structure of
`internal/infrastructure/form/repo/postgres.go` exactly:
`//go:embed migration.sql`, `RunMigrations` via
`database.RunUnderMigrationLock`, `New(pool) page.Repository`, then CRUD:

- `Create`: `p.ID = uuid.New().String()`, INSERT, wrap errors
  `create page: %w`. A slug duplicate surfaces as the pg unique-index
  violation wrapped as `create page: <err>` (handler maps "duplicate" → 409,
  Task 4).
- `GetByID` / `GetBySlug`: SELECT, `sql.ErrNoRows` → `page not found`.
- `List`: `WHERE context_id = $1 ORDER BY created_at DESC`.
- `Update`: SET slug/title/content/is_active/updated_at, `0 rows` →
  `page not found`.
- `Delete`: `DELETE FROM page_pages WHERE id = $1`, `0 rows` → `page not found`.

**Steps:**
1. `go build ./internal/infrastructure/page/...` (pass)
2. Commit: `feat(page): add postgres repo and migration`

(No unit test — DB-backed; covered by Task 17 cicd test.)

### Task 3: Page usecases

**Files:**
- Create: `internal/usecase/page/create_page.go`
- Create: `internal/usecase/page/get_page.go`
- Create: `internal/usecase/page/list_pages.go`
- Create: `internal/usecase/page/update_page.go`
- Create: `internal/usecase/page/delete_page.go`
- Create: `internal/usecase/page/page_test.go`

`create_page.go` (mirrors `internal/usecase/form/create_form.go`):

```go
package page

import (
	"github.com/h2hsecure/automata/internal/domain/page"
)

// CreatePageInput is the request payload for page create/update.
type CreatePageInput struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	IsActive *bool  `json:"is_active"`
}

// CreatePage validates and creates a page for the context.
func CreatePage(repo page.Repository, contextID string, input CreatePageInput) (*page.Page, error) {
	// contextID required check
	p := &page.Page{
		ContextID: contextID,
		Slug:      input.Slug,
		Title:     input.Title,
		Content:   input.Content,
		IsActive:  true,
	}
	if input.IsActive != nil {
		p.IsActive = *input.IsActive
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := repo.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}
```

`get_page.go` — `GetPage(repo, id)` (by ID) and `GetPageBySlug(repo,
contextID, slug)` (free functions, errors wrapped `get page: %w`).
`list_pages.go` — `ListPages(repo, contextID)`.
`update_page.go` — `UpdatePage(repo, contextID, id, input)`: get by ID,
check `f.ContextID != contextID` → `page does not belong to context`,
apply fields, `p.Validate()`, `repo.Update`.
`delete_page.go` — `DeletePage(repo, contextID, id)`: get by ID, ownership
check, `repo.Delete`.

`page_test.go` — in-memory `fakeRepo` (implements `page.Repository`, mirrors
`internal/adapter/api/handler/form_handler_test.go:20`): Create stores,
GetByID/GetBySlug return, Update persists, Delete removes. Tests: create
validates (empty slug → error, repo not called); create stores; update
foreign context → error; delete own context OK.

**Steps:**
1. Write test, `go test ./internal/usecase/page/` (fails)
2. Implement; re-run (pass)
3. Commit: `feat(page): add usecases`

### Task 4: Dashboard API handler + routes

**Files:**
- Create: `internal/adapter/api/handler/page_handler.go`
- Create: `internal/adapter/api/handler/page_handler_test.go`
- Modify: `internal/adapter/api/server.go` (routes near forms, ~line 242;
  build `pageRepo` near line 166)

`page_handler.go` — mirror `form_handler.go` (context via
`c.GetString("context_id")`, 400 when empty):

- `List` → `pageuc.ListPages` → `{"pages": [...], "total": n}`
- `Get` → `pageuc.GetPage` → 404 on error
- `Create` → bind `pageuc.CreatePageInput`; 400 on validation error;
  409 when the repo error contains `duplicate`
- `Update` → bind, `pageuc.UpdatePage`; 404 on not-found/foreign
- `Delete` → `pageuc.DeletePage`; 200 `{"message":"page deleted"}`

`server.go` — add next to the form routes (line 242):

```go
pageHandler := handler.NewPageHandler(pageRepo)
api.GET("/context/pages", middleware.AuthMiddleware(authService), pageHandler.List)
api.GET("/context/pages/:id", middleware.AuthMiddleware(authService), pageHandler.Get)
api.POST("/context/pages", middleware.AuthMiddleware(authService), pageHandler.Create)
api.PUT("/context/pages/:id", middleware.AuthMiddleware(authService), pageHandler.Update)
api.DELETE("/context/pages/:id", middleware.AuthMiddleware(authService), pageHandler.Delete)
```

(`pageRepo := pageRepoInfra.New(pool)` alongside `formRepo` at ~line 166;
import alias `pageRepoInfra "github.com/h2hsecure/automata/internal/infrastructure/page/repo"`.)

`page_handler_test.go` — fake repo + gin test mode, per
`form_handler_test.go:1-149`: create/list/get/update/delete, 409 on
duplicate slug (fake returns an error containing `duplicate`), 404 on
foreign-context update.

**Steps:**
1. `go test ./internal/adapter/api/handler/ -run TestPage` (fails)
2. Implement; re-run (pass)
3. `go build ./...` (only pre-existing proxy failure)
4. Commit: `feat(page): add dashboard CRUD API`

### Task 5: Public page serving

**Files:**
- Create: `internal/adapter/tracking/page_handler.go`
- Create: `internal/adapter/tracking/page_handler_test.go`
- Modify: `cmd/automata/main.go` (after line 220)

`page_handler.go`:

```go
package tracking

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	pagedomain "github.com/h2hsecure/automata/internal/domain/page"
)

// PageServeHandler serves per-context pages publicly at
// GET /page/:contextID/:slug.
type PageServeHandler struct {
	repo pagedomain.Repository
}

// NewPageServeHandler creates a PageServeHandler.
func NewPageServeHandler(repo pagedomain.Repository) *PageServeHandler {
	return &PageServeHandler{repo: repo}
}

// RegisterPageRoutes registers the public page endpoint on the tracking engine.
func RegisterPageRoutes(r *gin.Engine, h *PageServeHandler) {
	r.GET("/page/:contextID/:slug", h.Serve)
	r.HEAD("/page/:contextID/:slug", h.Serve)
}

// Serve renders the page content inside a minimal HTML document.
func (h *PageServeHandler) Serve(c *gin.Context) {
	p, err := h.repo.GetBySlug(c.Param("contextID"), c.Param("slug"))
	if err != nil || p == nil || !p.IsActive {
		c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
		return
	}
	doc := fmt.Sprintf(
		"<!DOCTYPE html><html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>%s</title></head><body>%s</body></html>",
		p.Title, p.Content,
	)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(doc))
}
```

`main.go` — after `tracking.RegisterStaticRoutes(...)` (line 220):

```go
tracking.RegisterPageRoutes(trackingEngine, tracking.NewPageServeHandler(pageRepo))
```

(`pageRepo` built with the other repos, ~line 133: `pageRepo := page_repo.New(pool)`.)

`page_handler_test.go` — fake repo, gin test mode (per
`static_handler_test.go:114-129`): 200 + HTML containing title and content;
404 missing; 404 inactive; HEAD registered.

**Steps:**
1. `go test ./internal/adapter/tracking/ -run TestPageServe` (fails)
2. Implement; re-run (pass)
3. Commit: `feat(page): add public page serving endpoint`

### Task 6: Web UI — PagesView

**Files:**
- Modify: `webui/src/types/api.ts`
- Modify: `webui/src/api/client.ts`
- Create: `webui/src/views/PagesView.vue`
- Create: `webui/src/views/PagesView.spec.ts`
- Modify: `webui/src/router/index.ts` (after `static-files`, ~line 57)
- Modify: `webui/src/components/AppSidebar.vue` (navItems)

`types/api.ts`:

```ts
export interface Page {
  ID: string;
  ContextID: string;
  Slug: string;
  Title: string;
  Content: string;
  IsActive: boolean;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface PageInput {
  slug?: string;
  title?: string;
  content?: string;
  is_active?: boolean;
}
```

`client.ts` — next to the banner block (~line 222):

```ts
export async function listPages(): Promise<Page[]> {
  const response = await client.get('/context/pages')
  return response.data.pages ?? []
}
export async function createPage(data: PageInput): Promise<Page> { /* POST /context/pages */ }
export async function updatePage(id: string, data: PageInput): Promise<Page> { /* PUT */ }
export async function deletePage(id: string): Promise<void> { /* DELETE */ }
```

`PagesView.vue` — copy the structure of `StaticFilesView.vue` (DataTable +
Modal + ConfirmDialog + `useConfirm` + context watch): columns slug/title/
status/updated/actions; modal with slug, title, `Textarea` for content
(rows=12, placeholder "HTML content..."), active toggle; public-URL hint
`/page/<context>/<slug>` row.

`PagesView.spec.ts` — copy `StaticFilesView.spec.ts`: mock
`listPages/createPage/updatePage/deletePage`, mount with pinia + auth
context, assert rows render, create flow calls API.

`router/index.ts`:

```ts
{ path: 'pages', name: 'Pages', component: () => import('@/views/PagesView.vue') },
```

`AppSidebar.vue` — add `Files` to the lucide import and
`{ name: 'Pages', path: '/pages', icon: Files }` after Static Files.

**Steps:**
1. `cd webui && npx vitest run src/views/PagesView.spec.ts` (fails)
2. Implement; `cd webui && npm test` (all pass)
3. Commit: `feat(webui): add Pages view with CRUD`

---

## Phase 2 — Banner position + buttons

### Task 7: Domain fields + usecase wiring

**Files:**
- Modify: `internal/domain/banner/banner.go`
- Modify: `internal/usecase/banner/banner.go`
- Create: `internal/usecase/banner/banner_test.go`

`banner.go` — add:

```go
// Button is a banner action button. Action is a free-form name ("accept",
// "decline", ...); LinkURL is optional (page slug like "/cookie-policy" or
// an absolute URL); Dismisses declares that clicking closes the banner.
type Button struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Action    string `json:"action"`
	LinkURL   string `json:"link_url"`
	Dismisses bool   `json:"dismisses"`
}
```

On `Banner` (after `Clicks`):

```go
	Position string   // "top" | "bottom" | "fullpage" ("" = legacy placement injection)
	Buttons  []Button
```

On `CreateBannerRequest` and `UpdateBannerRequest`:

```go
	Position string   `json:"position"`
	Buttons  []Button `json:"buttons"`
```

`usecase/banner/banner.go`:
- `CreateBanner`: after existing field assignments —
  `b.Position = req.Position; if req.Buttons != nil { b.Buttons = req.Buttons }`
  (keep nil → `[]` on JSON via the repo's marshal of nil slice → `null`; so
  also `if b.Buttons == nil { b.Buttons = []Button{} }` before Create).
- `UpdateBanner` (after `req.LinkURL` block, ~line 84):

```go
	if req.Position != "" {
		b.Position = req.Position
	}
	if req.Buttons != nil {
		b.Buttons = req.Buttons
	}
```

`banner_test.go` — in-memory fake of `banner.Repository` (implement all
methods; no existing test file exists). Tests: create stores position +
buttons; create with nil buttons → stored as non-nil empty slice; update
with position/buttons applies; update without them keeps existing.

**Steps:**
1. `go test ./internal/usecase/banner/` (fails)
2. Implement; re-run (pass)
3. Commit: `feat(banner): add position and buttons to banner model`

### Task 8: Migration + repo queries

**Files:**
- Modify: `internal/infrastructure/banner/repo/migration.sql` (append)
- Modify: `internal/infrastructure/banner/repo/postgres.go`

Append to `migration.sql`:

```sql
ALTER TABLE banner_banners ADD COLUMN IF NOT EXISTS position VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE banner_banners ADD COLUMN IF NOT EXISTS buttons JSONB NOT NULL DEFAULT '[]';
```

`postgres.go` — four query sites get `position, buttons` added (after
`clicks` in every column list):

- `CreateBanner` (~line 149): marshal `b.Buttons` → `buttonsJSON` (like
  `placementsJSON`); add columns + `$18, $19` args.
- `GetBanner` (~line 162), `ListBanners` (~line 197),
  `ListActiveBannersForPlacement` (~line 274): add columns to SELECT,
  `var buttonsJSON []byte`, scan, `json.Unmarshal(buttonsJSON, &b.Buttons)`
  (guard: unmarshal into `[]Button` — `NULL`/`'[]'` both fine).

**Steps:**
1. `go build ./...` (only pre-existing proxy failure)
2. Commit: `feat(banner): persist position and buttons`

### Task 9: cicd repo round-trip test

**Files:**
- Create: `cicd/banner_position_repo_test.go`

Ginkgo spec (mirror `cicd/banner_repo_test.go` structure; `AfterEach`
cleans `banner_banners`):

1. `INSERT` a banner with `position='fullpage'`,
   `buttons=[{"id":"b1","label":"Accept","action":"accept","dismisses":true}]`
   via the repo (`banner_repo.New(pool).CreateBanner`).
2. `GetBanner` → `Position == "fullpage"`, one button, `Dismisses == true`.
3. Legacy row (insert without position/buttons columns) → `Position == ""`,
   `Buttons` empty non-nil.

**Steps:**
1. `go test ./cicd/... -count=1 -timeout 120s -ginkgo.focus "position"` (fails)
2. Run full `make cicd-tests` (pass)
3. Commit: `test(cicd): banner position/buttons round-trip`

---

## Phase 3 — Per-visitor decisions

### Task 10: Decision struct + repo interface

**Files:**
- Modify: `internal/domain/banner/banner.go`
- Modify: `internal/domain/banner/repository.go`

`banner.go`:

```go
// Decision records a visitor's action on a banner button.
type Decision struct {
	ID        string
	ContextID string
	BannerID  string
	VisitorID string
	Action    string
	IPHash    string
	UserAgent string
	CreatedAt time.Time
}
```

`repository.go` — add to `Repository`:

```go
	// Decision operations
	RecordDecision(d *Decision) error
	ListDecisions(bannerID string, offset, limit int) ([]*Decision, int64, error)
	DecisionCounts(bannerID string) (map[string]int64, error)
```

**Steps:**
1. `go build ./...` (only pre-existing proxy failure)
2. Commit: `feat(banner): add Decision domain and repo interface`

### Task 11: banner_decisions migration + repo implementation

**Files:**
- Modify: `internal/infrastructure/banner/repo/migration.sql` (append)
- Modify: `internal/infrastructure/banner/repo/postgres.go`

Append to `migration.sql`:

```sql
CREATE TABLE IF NOT EXISTS banner_decisions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id  UUID NOT NULL,
    banner_id   UUID NOT NULL REFERENCES banner_banners(id) ON DELETE CASCADE,
    visitor_id  VARCHAR(128),
    action      VARCHAR(64),
    ip_hash     VARCHAR(64),
    user_agent  VARCHAR(512),
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_decisions_banner ON banner_decisions(banner_id, created_at);
CREATE INDEX IF NOT EXISTS idx_decisions_context_visitor ON banner_decisions(context_id, visitor_id);
```

`postgres.go`:

- `RecordDecision`: `d.ID = uuid.New().String()`; INSERT
  `(id, context_id, banner_id, visitor_id, action, ip_hash, user_agent, created_at)`
  with `NOW()`.
- `ListDecisions`: COUNT + SELECT ordered `created_at DESC`, optional
  `LIMIT/OFFSET` (same `$N`-building pattern as `ListBanners`).
- `DecisionCounts`:
  `SELECT action, COUNT(*) FROM banner_decisions WHERE banner_id = $1 GROUP BY action`
  → `map[string]int64`.

**Steps:**
1. `go build ./...`
2. Commit: `feat(banner): store and query banner decisions`

### Task 12: Public config + decision endpoints

**Files:**
- Create: `internal/adapter/tracking/banner_public_handler.go`
- Create: `internal/adapter/tracking/banner_public_handler_test.go`
- Modify: `cmd/automata/main.go` (after Task 5's line)

`banner_public_handler.go`:

```go
package tracking

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	bdomain "github.com/h2hsecure/automata/internal/domain/banner"
	pagedomain "github.com/h2hsecure/automata/internal/domain/page"
	bannerUsecase "github.com/h2hsecure/automata/internal/usecase/banner"
)

// BannerPublicHandler serves the public banner config and records visitor
// decisions. Trust model matches the existing /track/banner/* and /form/*
// endpoints (anonymous visitor_id, hashed IP, no PII).
type BannerPublicHandler struct {
	bannerRepo bdomain.Repository
	pageRepo   pagedomain.Repository
}

func NewBannerPublicHandler(bannerRepo bdomain.Repository, pageRepo pagedomain.Repository) *BannerPublicHandler {
	return &BannerPublicHandler{bannerRepo: bannerRepo, pageRepo: pageRepo}
}

// RegisterBannerPublicRoutes registers the public banner endpoints.
func RegisterBannerPublicRoutes(r *gin.Engine, h *BannerPublicHandler) {
	r.GET("/banner/config/:contextID", h.Config)
	r.HEAD("/banner/config/:contextID", h.Config)
	r.POST("/banner/decision/:contextID", h.Decision)
}
```

`Config` handler:

- `banners, err := bannerUsecase.NewBannerUsecase(h.bannerRepo).
  ListActiveBannersForPlacement(contextID, "", time.Now())`
  (usecase method exists; the placementID argument is unused by the query).
- For each banner build:

```go
type configButton struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Action    string `json:"action"`
	LinkURL   string `json:"link_url"`
	Dismisses bool   `json:"dismisses"`
}
gin.H{
	"id": b.ID, "name": b.Name, "position": b.Position,
	"content": b.Content, "buttons": b.Buttons, "page": inlinePage,
}
```

  where `inlinePage` = content of the first page referenced by a button
  (see "Config inline page" above), `nil` otherwise.
- Impressions: if `c.Query("visitor_id") != ""`, best-effort
  `h.bannerRepo.RecordImpression(b.ID, visitorID)` (ignore errors).
- Respond `{"banners": [...]}`.

`Decision` handler:

- Body: `c.GetRawData()`; unmarshal
  `struct{ BannerID string `json:"banner_id"`; Action string `json:"action"`; VisitorID string `json:"visitor_id"` }`.
  (JSON-in-text/plain works: the Content-Type is irrelevant to
  `json.Unmarshal`.)
- Validate: `banner_id` and `action` non-empty → 400.
- Ownership: `b, err := h.bannerRepo.GetBanner(bannerID)`; 404 on error or
  `b.ContextID != contextID`.
- Insert:

```go
h.bannerRepo.RecordDecision(&bdomain.Decision{
	ContextID: contextID,
	BannerID:  bannerID,
	VisitorID: body.VisitorID,
	Action:    body.Action,
	IPHash:    ipHash(c.ClientIP()),
	UserAgent: c.GetHeader("User-Agent"),
})
```

- Respond 204.

Helper:

```go
func ipHash(ip string) string {
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:])
}
```

`main.go` — after `RegisterPageRoutes`:

```go
tracking.RegisterBannerPublicRoutes(trackingEngine,
	tracking.NewBannerPublicHandler(bannerRepo, pageRepo))
```

`banner_public_handler_test.go` — fakes for both repos (fake banner repo
implements `bdomain.Repository`; fake page repo implements
`pagedomain.Repository`). Tests:

- Config: returns active banners with position/buttons; inlines page when a
  button `LinkURL` is `/cookie-policy` and the page exists; `page: null`
  when not; visitor_id query → `RecordImpression` called.
- Decision: valid body (text/plain) → 204, decision stored with action and
  sha256 IP hash; banner from another context → 404; missing action → 400.

**Steps:**
1. `go test ./internal/adapter/tracking/ -run TestBannerPublic` (fails)
2. Implement; re-run (pass)
3. Commit: `feat(banner): add public config and decision endpoints`

### Task 13: Dashboard decisions endpoint

**Files:**
- Modify: `internal/adapter/api/handler/banner_handler.go`
- Modify: `internal/adapter/api/server.go` (after line 253)
- Modify: `internal/adapter/api/handler/` (add/extend test file)

`banner_handler.go` — new method:

```go
// ListDecisions handles GET /api/context/banners/:id/decisions —
// per-visitor decision log + aggregate counts for a banner.
func (h *BannerHandler) ListDecisions(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" { c.JSON(http.StatusBadRequest, ...); return }

	b, err := h.repo.GetBanner(c.Param("id"))
	if err != nil || b.ContextID != contextID {
		c.JSON(http.StatusNotFound, gin.H{"error": "banner not found"}); return
	}

	var offset, limit int
	// parse c.Query("offset") / c.Query("limit") like ListBanners

	decisions, total, err := h.repo.ListDecisions(b.ID, offset, limit)
	counts, _ := h.repo.DecisionCounts(b.ID)
	c.JSON(http.StatusOK, gin.H{
		"decisions": decisions, "counts": counts, "total": total,
	})
}
```

`server.go`:

```go
api.GET("/context/banners/:id/decisions", middleware.AuthMiddleware(authService), bannerHandler.ListDecisions)
```

Tests (extend the existing handler test file if one exists for banners;
otherwise create `banner_handler_test.go` with a fake repo): own-context
banner → 200 with decisions + counts; foreign context → 404.

**Steps:**
1. `go test ./internal/adapter/api/handler/ -run "TestBanner"` (fails)
2. Implement; re-run (pass)
3. Commit: `feat(banner): add dashboard decisions endpoint`

---

## Phase 4 — Banner snippet

### Task 14: GenerateBannerSnippet

**Files:**
- Create: `pkg/snippet/banner_generator.go`
- Create: `pkg/snippet/banner_generator_test.go`

`GenerateBannerSnippet(contextID string) (string, error)` — self-contained
IIFE (no window globals), mirroring the style of `Generate`
(`generator.go:22`) and `GenerateForm` (`form_generator.go:13`). Inlined
values: `contextId`, same-origin endpoints
`/banner/config/<ctx>`, `/banner/decision/<ctx>`, `/page/<ctx>/<slug>`.

Required behavior (assert each in tests):

1. **Visitor ID:** read/create cookie `automata_visitor` (same pattern as
   `form_generator.go`'s `_getCookie("automata_visitor")`); random 16-hex if
   absent, stored 1 year.
2. **Fetch** `configUrl + "?visitor_id=" + encodeURIComponent(visitorId)`,
   `text/plain` content type on the request; on fetch failure, abort quietly.
3. **Skip decided:** cookie `ab_<bannerId>` → skip that banner.
4. **Skip legacy:** `position` empty → skip (placement injection unchanged).
5. **Render by position** (fixed, `z-index: 2147483000`):
   - `top`: `position:fixed;top:0;left:0;right:0`
   - `bottom`: `position:fixed;bottom:0;left:0;right:0`
   - `fullpage`: `position:fixed;top:0;left:0;right:0;bottom:0;` +
     `background:rgba(15,15,15,0.92)` + centered card
6. **Content:** `banner.content + (banner.page ? banner.page : '')`.
7. **Buttons:** one `<button>` each; on click:
   - POST `decisionUrl` with `Content-Type: text/plain` body
     `{"banner_id":...,"action":...,"visitor_id":...}` (no preflight);
   - set `ab_<bannerId>=<action>` cookie, 365 days, `SameSite=Lax; Path=/`;
   - if `dismisses` → remove the banner element; else keep visible;
   - if `link_url` starts with `/` → after decision,
     `location.href = "/page/" + contextId + "/" + link_url.slice(1)`.
8. Legacy link-click tracking (`banner.link_url` on non-button banners) is
   out of scope for this snippet (those banners have `position=''` and are
   skipped entirely).

`banner_generator_test.go` — string assertions per
`form_generator_test.go:8-24`: contains `/banner/config/ctx-1`,
`/banner/decision/ctx-1`, `ab_`, `SameSite=Lax`, `2147483000`,
`fullpage`, `dismisses`; empty contextID → error.

**Steps:**
1. `go test ./pkg/snippet/ -run TestGenerateBannerSnippet` (fails)
2. Implement; re-run (pass)
3. Commit: `feat(snippet): add banner snippet generator`

### Task 15: Serve the banner snippet

**Files:**
- Modify: `internal/adapter/tracking/banner_public_handler.go` (or a new
  `banner_snippet_handler.go`)
- Modify: test file from Task 12

Add to `RegisterBannerPublicRoutes`:

```go
r.GET("/banner/snippet/:contextID.js", h.ServeSnippet)
r.HEAD("/banner/snippet/:contextID.js", h.ServeSnippet)
```

`ServeSnippet`: extract `contextID` from the path (same slicing pattern as
`snippet_handler.go:19-29`), `snippet.GenerateBannerSnippet(contextID)`,
respond `application/javascript; charset=utf-8`.

Test: request `GET /banner/snippet/ctx-1.js` → 200 + JS containing
`/banner/config/ctx-1`; invalid path → 400.

**Steps:**
1. `go test ./internal/adapter/tracking/ -run TestBannerPublic` (fails)
2. Implement; re-run (pass)
3. Commit: `feat(banner): serve banner snippet`

---

## Phase 5 — Dashboard banner editor

### Task 16: BannersView position + buttons + decisions

**Files:**
- Modify: `webui/src/types/api.ts`
- Modify: `webui/src/api/client.ts`
- Modify: `webui/src/views/BannersView.vue`

`types/api.ts` — extend:

```ts
export interface BannerButton {
  id?: string;
  label: string;
  action: string;
  link_url?: string;
  dismisses: boolean;
}

export interface BannerDecision {
  ID: string;
  ContextID: string;
  BannerID: string;
  VisitorID: string;
  Action: string;
  IPHash: string;
  UserAgent: string;
  CreatedAt: string;
}
```

`Banner` gains `Position: string; Buttons: BannerButton[]`; `BannerInput`
gains `position?: string; buttons?: BannerButton[]`.

`client.ts`:

```ts
export async function listBannerDecisions(
  bannerId: string,
  offset = 0,
  limit = 50,
): Promise<{ decisions: BannerDecision[]; counts: Record<string, number>; total: number }> {
  const response = await client.get(`/context/banners/${bannerId}/decisions`, {
    params: { offset, limit },
  })
  return response.data
}
```

`BannersView.vue`:

- Modal: add **Position** select — `""`/legacy, `top`, `bottom`, `fullpage`
  (same `<select>` styling as the Content Type select).
- Modal: **Buttons** builder — `buttons` ref array; per row: Label input,
  Action input (placeholder `accept`), Link URL input (placeholder
  `/cookie-policy`), Dismisses checkbox; Add button appends
  `{ label: '', action: '', link_url: '', dismisses: false }`; per-row
  delete. `openCreate`/`openEdit` reset/populate; `handleSave` sends
  `position` + `buttons` in `BannerInput`.
- Table: add **Decisions** action button (BarChart3 is already imported).
  Clicking opens a detail modal: counts by action (`Object.entries(counts)`
  badges) + recent decisions DataTable (visitor, action, created).

**Steps:**
1. `cd webui && npm test` (pass — no existing BannersView spec; verify build)
2. `cd webui && npm run build` (catches TS errors)
3. Commit: `feat(webui): banner position, button builder, decisions view`

---

## Phase 6 — End-to-end verification

### Task 17: cicd integration test + full run

**Files:**
- Create: `cicd/banner_pages_integration_test.go`

Ginkgo spec (shared `pool` from `suite_test.go`; build a tracking engine the
way `cicd/tracking_http_test.go:43` does, plus `RegisterPageRoutes` +
`RegisterBannerPublicRoutes`):

1. Create context, page (slug `cookie-policy`, title `Cookie Policy`,
   content `<p>Cookie text</p>`), fullpage banner with buttons
   `[{Accept, accept, dismisses:true}, {Policy, "", link:/cookie-policy}]`.
2. `GET /page/<ctx>/cookie-policy` → 200, HTML contains title + content.
3. `GET /banner/config/<ctx>?visitor_id=v1` → 200, one banner,
   `position=fullpage`, `page` inlines the cookie text; impressions table
   has a row for v1.
4. `POST /banner/decision/<ctx>` (text/plain JSON
   `{"banner_id":...,"action":"accept","visitor_id":"v1"}`) → 204;
   `banner_decisions` has one row with the action and a 64-char ip_hash.
5. `GET /banner/config/<ctx>?visitor_id=v1` again → banner still listed
   (server does not filter by decision — snippet-side cookie skips).

**Full verification (all must pass before deploy):**

```sh
go build ./...                                  # only pre-existing proxy failure
go test ./internal/... ./pkg/...                # only pre-existing TestContextMiddleware failure
cd webui && npm test && npm run build
make cicd-tests
```

**Deploy (user action):**
1. Push `main` → CI publishes image `<sha>` + chart `YYYY.MM.DD-<sha>`.
2. User: `helm upgrade -i automata oci://ghcr.io/h2hsecure/automata-chart/automata --version <chart> -n automata -f values.prod.yaml --set image.tag=<sha>`.
3. External canaries (`m.h2hsecure.com`):
   - `curl -s https://m.h2hsecure.com/page/<ctx>/cookie-policy` → HTML
   - `curl -s https://m.h2hsecure.com/banner/config/<ctx>` → JSON
   - `curl -s -X POST https://m.h2hsecure.com/banner/decision/<ctx> -H 'Content-Type: text/plain' -d '{"banner_id":"...","action":"accept","visitor_id":"canary"}'` → 204
   - `curl -sI https://m.h2hsecure.com/banner/snippet/<ctx>.js` → 200
4. Check the website repo (`h2hsecure.github.io`) nginx proxy location list:
   `/snippet|track|form|banner` is proxied to automata :8081 — verify
   `/banner/config|decision|snippet` and `/page` are covered (add if not).

Commit: `test(cicd): banner + pages end-to-end`