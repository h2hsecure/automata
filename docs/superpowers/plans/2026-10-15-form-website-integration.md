# Form-to-Website Integration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make forms embeddable on websites via the tracking server: context middleware on form routes, public form-definition + self-rendering snippet endpoints, and per-form embed-tag display in the Web UI.

**Architecture:** New `ContextMiddleware()` in `internal/adapter/tracking` (X-Context-ID header → `context_id` query, fail-closed) applied to the `/form` group and `/track/banner/list` in `NewServer` (5-arg signature unchanged). New `FormSnippetHandler` + `FormHandler.Repo()` accessor serve `GET /form/definition/:contextID/:slug` and `GET /form/snippet/:contextID/:slug` (static prefixes — gin panics on conflicting param routes; `.js` suffix is dropped from the route because gin v1.10.0 captures `:slug.js` params as empty, and the handler tolerates a trailing `.js` on the slug). New `snippet.GenerateForm` produces a fetch-based IIFE that renders fields and POSTs `{data}` to `/form/submit/:slug` with `X-Context-ID`. Web UI shows the embed tag per form.

**Tech Stack:** Go 1.26.5, Gin v1.10.0, Ginkgo + testcontainers (cicd), Vue 3 + Vite + TypeScript

**Spec:** `docs/superpowers/specs/2026-10-15-form-website-integration-design.md`

**Constraints:**
- `NewServer` signature stays 5 args (`formHandler, bannerHandler, placementHandler, campaignHandler, trackingRepo`) — `cicd/tracking_server_test.go` (user's uncommitted work) and `cicd/tracking_http_test.go` call it with 5 args.
- Do NOT modify `cicd/tracking_server_test.go` (user's uncommitted work).
- `cicd/form_http_test.go` is committed — updating its `setupFormEngine` is allowed.
- `banner_campaign_http` ×10 failures are out of scope (hand-rolled `setupBannerEngine()`).
- Baseline before this work: 300 passed | 23 failed. Expected after: 313 passed | 10 failed.

---

## Task 1: Context middleware + wiring into NewServer

**Files:**
- Create: `internal/adapter/tracking/context_middleware.go`
- Modify: `internal/adapter/tracking/server.go`
- Modify: `cicd/form_http_test.go` (`setupFormEngine` only)

- [x] 1.1 Create `internal/adapter/tracking/context_middleware.go`:

```go
package tracking

import (
	"github.com/gin-gonic/gin"
)

// ContextMiddleware resolves the context for public tracking endpoints from
// an explicit X-Context-ID header or context_id query parameter. It fails
// closed: when neither is present, context_id is left unset and the handler
// responds with its own error (401/400). Subdomain and default-context
// fallbacks are intentionally excluded.
func ContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		contextID := c.GetHeader("X-Context-ID")
		if contextID == "" {
			contextID = c.Query("context_id")
		}
		if contextID != "" {
			c.Set("context_id", contextID)
		}
		c.Next()
	}
}
```

- [x] 1.2 Modify `internal/adapter/tracking/server.go`: apply the middleware to the form group and to `/track/banner/list`:

```go
	// Banner endpoints
	r.GET("/track/banner/list", ContextMiddleware(), bannerHandler.ListBanners)
	r.GET("/track/banner/:id", bannerHandler.GetBanner)
	r.GET("/track/placement/:id", placementHandler.GetPlacement)
	r.GET("/track/campaign/:id", campaignHandler.GetCampaign)

	// Form endpoints (context via X-Context-ID header or context_id query)
	formGroup := r.Group("/form")
	formGroup.Use(ContextMiddleware())
	formGroup.GET("/list", formHandler.List)
	formGroup.GET("/:id", formHandler.Get)
	formGroup.POST("/submit/:slug", formHandler.SubmitForm)
	formGroup.GET("/:id/submissions", formHandler.ListSubmissions)
```

(Replace the existing `r.GET("/form/...")` block at lines 28-32.)

- [x] 1.3 Modify `cicd/form_http_test.go` `setupFormEngine` — add the middleware to the form group:

```go
func setupFormEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	formRepo := formRepo.New(pool)
	formHandler := bhandler.NewFormHandler(formRepo)

	forms := engine.Group("/form")
	forms.Use(ttracking.ContextMiddleware())
	{
		forms.GET("/list", formHandler.List)
		forms.GET("/:id", formHandler.Get)
		forms.POST("/submit/:slug", formHandler.SubmitForm)
		forms.GET("/:id/submissions", formHandler.ListSubmissions)
	}

	return engine
}
```

(Add the tracking adapter import, aliased `ttracking`.)

- [x] 1.4 Verify RED→GREEN:
  - Before 1.1-1.3: `go test ./cicd/ -run TestCICD -ginkgo.focus="Form Endpoints"` fails (8 form_http specs).
  - After: same command passes (13 of 323 specs, 13 passed); `go test ./cicd/ -run TestCICD -ginkgo.focus="banner/list"` passes (5 specs — tracking_http ×4 + tracking_server ×1).
  - `go build ./...` clean.

  Note: cicd entry point is `TestCICD` (not `TestCicd`); the test package is external (`cicd_test`).

## Task 2: Public form definition + snippet endpoints

**Files:**
- Create: `internal/adapter/tracking/form_snippet_handler.go`
- Create: `cicd/form_snippet_test.go`
- Modify: `internal/adapter/api/handler/form_handler.go` (add `Repo()` accessor)
- Modify: `internal/adapter/tracking/server.go` (register public routes)

- [x] 2.1 Write the failing cicd spec `cicd/form_snippet_test.go` (see Step 2.4 for setup pattern — follow `cicd/form_http_test.go`):

```go
package cicd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ttracking "github.com/hekemen/automata/internal/adapter/tracking"
	"github.com/hekemen/automata/cicd/support"
)

var _ = Describe("HTTP Integration Tests - Public Form Endpoints", func() {
	var (
		w      *httptest.ResponseRecorder
		engine *gin.Engine
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		engine = ttracking.NewServer(
			bhandler.NewFormHandler(formRepo.New(pool)),
			bhandler.NewBannerHandler(bannerRepo.New(pool)),
			bhandler.NewPlacementHandler(placementRepo.New(pool)),
			bhandler.NewCampaignHandler(campaignRepo.New(pool)),
			trackingRepo.New(pool),
		)
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM form_submissions")
		_, _ = pool.Exec(ctx, "DELETE FROM forms")
	})

	// createForm inserts a form row directly via SQL and returns its context ID.
	createForm := func(contextID, slug string) {
		fields, _ := json.Marshal([]map[string]interface{}{
			{"slug": "email", "type": "email", "required": true, "label": "Email"},
			{"slug": "name", "type": "text", "required": false, "label": "Name"},
		})
		settings, _ := json.Marshal(map[string]interface{}{})
		_, err := pool.Exec(ctx,
			`INSERT INTO forms (id, context_id, slug, name, description, fields, settings)
			 VALUES (gen_random_uuid(), $1, $2, 'Snippet Test Form', '', $3, $4)`,
			contextID, slug, fields, settings)
		Expect(err).ToNot(HaveOccurred())
	}

	Describe("GET /form/definition/:contextID/:slug", func() {
		It("should return the form definition as JSON", func() {
			contextID := support.NewTestContextID()
			createForm(contextID, "snip-form")

			engine.ServeHTTP(w, httptest.NewRequest("GET", "/form/definition/"+contextID+"/snip-form", nil))
			Expect(w.Code).To(Equal(http.StatusOK))

			var def map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &def)).To(Succeed())
			Expect(def["slug"]).To(Equal("snip-form"))
			Expect(def["name"]).To(Equal("Snippet Test Form"))
		})

		It("should 404 for an unknown form", func() {
			contextID := support.NewTestContextID()
			engine.ServeHTTP(w, httptest.NewRequest("GET", "/form/definition/"+contextID+"/nope", nil))
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("GET /form/snippet/:contextID/:slug", func() {
		It("should return a JavaScript snippet containing the context ID and URLs", func() {
			contextID := support.NewTestContextID()
			createForm(contextID, "snip-form")

			engine.ServeHTTP(w, httptest.NewRequest("GET", "/form/snippet/"+contextID+"/snip-form", nil))
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(ContainSubstring("javascript"))

			body := w.Body.String()
			Expect(body).To(ContainSubstring(contextID))
			Expect(body).To(ContainSubstring("/form/definition/" + contextID + "/snip-form"))
			Expect(body).To(ContainSubstring("/form/submit/snip-form"))
		})

		It("should 404 for an unknown form", func() {
			contextID := support.NewTestContextID()
			engine.ServeHTTP(w, httptest.NewRequest("GET", "/form/snippet/"+contextID+"/nope", nil))
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should serve the snippet for a .js-suffixed slug", func() {
			contextID := support.NewTestContextID()
			createForm(contextID, "snip-form")

			engine.ServeHTTP(w, httptest.NewRequest("GET", "/form/snippet/"+contextID+"/snip-form.js", nil))
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(ContainSubstring("javascript"))
		})
	})

	Describe("existing form routes", func() {
		It("should still serve /form/list with the context header", func() {
			contextID := support.NewTestContextID()
			createForm(contextID, "snip-form")

			req := httptest.NewRequest("GET", "/form/list", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should 401 /form/list without a context", func() {
			engine.ServeHTTP(w, httptest.NewRequest("GET", "/form/list", nil))
			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})
	})
})
```

- [x] 2.2 Verify RED: `go test ./cicd/ -run TestCICD -ginkgo.focus="Public Form Endpoints"` fails (404 — routes don't exist).

- [x] 2.3 Add `Repo()` accessor to `internal/adapter/api/handler/form_handler.go` (after `NewFormHandler`):

```go
// Repo exposes the form repository for adapters that need direct access.
func (h *FormHandler) Repo() form.Repository {
	return h.repo
}
```

- [x] 2.4 Create `internal/adapter/tracking/form_snippet_handler.go`:

```go
package tracking

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/form"
	"github.com/hekemen/automata/pkg/snippet"
)

// FormSnippetHandler serves public form definition and self-rendering snippet endpoints.
type FormSnippetHandler struct {
	repo form.Repository
}

// ServeDefinition handles GET /form/definition/:contextID/:slug — returns the form definition as JSON.
func (h *FormSnippetHandler) ServeDefinition(c *gin.Context) {
	f, err := h.repo.GetBySlug(c.Param("contextID"), c.Param("slug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}
	// Emit an explicit public contract (lowercase keys) rather than the domain
	// struct, whose untagged fields serialize capitalized and whose shape is
	// the main API's internal contract.
	c.JSON(http.StatusOK, gin.H{
		"id":          f.ID,
		"contextId":   f.ContextID,
		"slug":        f.Slug,
		"name":        f.Name,
		"description": f.Description,
		"fields":      f.Fields,
	})
}

// ServeFormSnippet handles GET /form/snippet/:contextID/:slug — returns a
// self-rendering form snippet. A trailing ".js" is tolerated (the :slug param
// captures the whole segment; gin v1.10.0 does not split ":slug.js" params).
func (h *FormSnippetHandler) ServeFormSnippet(c *gin.Context) {
	slug := strings.TrimSuffix(c.Param("slug"), ".js")
	if _, err := h.repo.GetBySlug(c.Param("contextID"), slug); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}
	js, err := snippet.GenerateForm(c.Param("contextID"), slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", []byte(js))
}
```

- [x] 2.5 Register the public routes in `internal/adapter/tracking/server.go` (after the form group block):

```go
	// Public form definition + self-rendering snippet (context in path, no auth).
	// Registered only when a form handler is provided — some test harnesses
	// construct the tracking server with a nil form handler.
	if formHandler != nil {
		formSnippetHandler := &FormSnippetHandler{repo: formHandler.Repo()}
		r.GET("/form/definition/:contextID/:slug", formSnippetHandler.ServeDefinition)
		r.GET("/form/snippet/:contextID/:slug", formSnippetHandler.ServeFormSnippet)
	}
```

  Notes from implementation:
  - The `.js` suffix was dropped from the route — gin v1.10.0 matches `:slug.js` but captures the `slug` param as empty (verified with a throwaway program against the project's gin version). A trailing `.js` on the request slug is still served (handler strips it).
  - The `formHandler != nil` guard was required by the committed `cicd/track_context_resolution_test.go`, which builds the tracking server with a nil form handler (it only exercises `/track` + `/snippet` context resolution).

- [x] 2.6 Verify GREEN: the Task 2.1 spec passes (7 of 330 specs, 7 passed); `go build ./...` clean.

## Task 3: GenerateForm snippet generator

**Files:**
- Create: `pkg/snippet/form_generator.go`
- Create: `pkg/snippet/form_generator_test.go`

- [x] 3.1 Write the failing unit test `pkg/snippet/form_generator_test.go`:

```go
package snippet

import (
	"strings"
	"testing"
)

func TestGenerateForm(t *testing.T) {
	js, err := GenerateForm("ctx-123", "contact")
	if err != nil {
		t.Fatalf("GenerateForm() error = %v", err)
	}
	for _, want := range []string{
		`"ctx-123"`,
		`/form/definition/ctx-123/contact`,
		`/form/submit/contact`,
		"X-Context-ID",
		"data-automata-form",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("GenerateForm() missing %q", want)
		}
	}
}

func TestGenerateFormValidation(t *testing.T) {
	if _, err := GenerateForm("", "contact"); err == nil {
		t.Error("expected error for empty contextID")
	}
	if _, err := GenerateForm("ctx-123", ""); err == nil {
		t.Error("expected error for empty slug")
	}
}
```

- [x] 3.2 Verify RED: `go test ./pkg/snippet/` fails (undefined: GenerateForm).

- [x] 3.3 Create `pkg/snippet/form_generator.go` — `GenerateForm(contextID, slug string) (string, error)` (same validation style as `Generate`). The snippet is an IIFE that:
  - Stores `contextId`, `slug`, `definitionUrl = "/form/definition/" + contextID + "/" + slug`, `submitUrl = "/form/submit/" + slug`.
  - Fetches the definition (same origin); on failure renders a "This form is not available right now." placeholder.
  - Render target: `[data-automata-form="<slug>"]` → `#automata-form` → `document.body`.
  - Renders a wrapper `<div class="automata-form" data-automata-form="<slug">` containing a title (form name) and a `<form>` with one label+input per field: `textarea` → `<textarea rows=4>`; `select` → `<select>` with a placeholder option; otherwise `<input type="<type or text>">`; `required` when the field requires it.
  - On submit: `preventDefault`, collects `{data: {fieldSlug: value}}`, POSTs to `submitUrl` with headers `Content-Type: application/json` and `X-Context-ID: contextId`; disables the button; shows "Sending…", then success ("Thank you! Your submission was received." + `form.reset()`) or the error message from the response body.
  - Minimal inline styles only (no external CSS).

- [x] 3.4 Verify GREEN: `go test ./pkg/snippet/` passes (TestGenerateForm, TestGenerateFormValidation).

## Task 4: Web UI embed tag

**Files:**
- Modify: `webui/src/views/FormsView.vue`
- Rebuild: `web/` (committed build artifacts)

- [x] 4.1 In `FormsView.vue`, add an **Embed** action to each form row that opens a small modal (or inline disclosure) showing:

```
<script src="/form/snippet/<contextID>/<slug>"></script>
```

where `contextID` is `authStore.currentContextId` and `slug` is the form's slug. Include a **Copy** button (navigator.clipboard with a fallback). Note under the tag: "Add this to your page HTML on a host that routes /form to the tracking server."

- [x] 4.2 Build: `cd webui && npm run build` (vite is the gate; compare vue-tsc errors against the pre-existing baseline if checked).

- [x] 4.3 Verify: open the built page locally or via the deployed BFF DOM; the Embed button exists per row and the tag text contains the current context ID and the form slug. (Verified against the compiled chunk: `title:"Embed"` button present and the tag template `form/snippet/${currentContextId}/${Slug}` is in the build; live DOM verification happens in Task 6 after deploy.)

## Task 5: Full cicd run + commit

- [ ] 5.1 Run the full cicd suite: `make test` (or `go test ./cicd/ -run TestCicd`).
- [ ] 5.2 Compare against baseline (300 passed | 23 failed). Expected: **313 passed | 10 failed**, with all 10 failures in `banner_campaign_http` (documented out-of-scope pre-existing).
- [ ] 5.3 `go vet ./...` — no new failures (pre-existing failure in `internal/adapter/proxy/proxy_test.go:23` is allowed).
- [ ] 5.4 Commit all changes (Go, cicd, spec/plan docs, rebuilt `web/`) on `phase2-auth-changes`. Do NOT commit the user's uncommitted files (`AGENTS.md`, `helm/automata/values.yaml`, `automata` binary, `.playwright-mcp/`, the 7 uncommitted cicd tests, `docs/phase1.md`).

## Task 6: Deploy + ingress + live verification

- [ ] 6.1 Reset `/tmp/opencode/automata-deploy` worktree to the new commit.
- [ ] 6.2 Build static binaries in the worktree: `CGO_ENABLED=0 go build -o automata ./cmd/automata` and `CGO_ENABLED=0 go build -o bff ./cmd/bff`; verify with `file` that both say "statically linked".
- [ ] 6.3 `docker build -f /tmp/opencode/Dockerfile.deploy` → tag `latest` + short-sha; push both tags.
- [ ] 6.4 `kubectl -n automata rollout restart deployment/automata-automata deployment/automata-bff`; wait for `1/1 Running`, 0 restarts.
- [ ] 6.5 Live HTTP verification (text only, no screenshots):
  - `curl -s http://automata-automata.automata.svc.cluster.local:8081/health` or via port-forward → 200.
  - Create a form via the API (or use an existing one in the default context `8c61ddc4-debd-445e-b90b-3c62f89dfcfc`):
    - `GET /form/definition/<ctx>/<slug>` → 200 JSON with the form.
    - `GET /form/snippet/<ctx>/<slug>` → 200, `Content-Type: application/javascript`, body contains the context ID.
    - `GET /form/definition/<ctx>/unknown` → 404.
    - `GET /form/list` without header → 401; with `X-Context-ID` → 200.
- [ ] 6.6 Patch the pweb ingress: add `/form` (Prefix) → `automata-tracking:8081` alongside the existing `/track` and `/snippet` paths; update `/tmp/opencode/pweb-automata-tracking.yaml` to match.
- [ ] 6.7 Verify through the pweb ingress host (`pweb.home.arpa`): `/form/snippet/<ctx>/<slug>` → 200 JS; `/form/definition/<ctx>/<slug>` → 200 JSON.
- [ ] 6.8 Hand the embed tag to the user for on-page testing.

---

## Execution Status

_(to be filled in after completion)_