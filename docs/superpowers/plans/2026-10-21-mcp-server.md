# MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose all context-scoped Automata functionality (contacts, forms, banners, placements, campaigns, tracking, email, webhooks) as 49 MCP tools over stateless streamable HTTP at `POST /api/mcp`, authenticated per-request by API key, plus an MCP tab in the web UI Settings.

> Note: the spec header says "51 tools", but its own catalog tables (section 4) total 49 (7+8+5+5+5+5+8+6). This plan follows the tables.

**Architecture:** The MCP adapter (`internal/adapter/mcp`) wraps the official Go SDK's `mcp.NewStreamableHTTPHandler` with a `net/http` auth middleware that validates the Bearer API key via `authService.ValidateAPIKey`. A `sync.Map` caches one `*mcp.Server` per context; each server's tools capture the `contextID` in closures at registration, so tools are tenant-bound by construction. Tools call usecases/repos directly (never REST). The route is registered in `NewServer` before the `ContextResolver` middleware so no `Context` header is needed.

**Tech Stack:** Go 1.26.5, `github.com/modelcontextprotocol/go-sdk` v1.8.0, gin (existing), PostgreSQL (cicd testcontainers), Ginkgo (cicd), Vue 3 + Vite (webui).

**Spec:** `docs/superpowers/specs/2026-10-21-mcp-server-design.md`

## Global Constraints

- SDK version: `github.com/modelcontextprotocol/go-sdk@v1.8.0` (add to `go.mod`).
- Tool names: `^[a-z0-9_]{1,64}$`, pattern `<domain>_<action>` (e.g. `contacts_list`). No dots. One tool per REST endpoint; no `context_id` parameter on any tool.
- Paging convention: inputs `page` (1-based, default 1) and `limit` (default 20, max 100 — clamp `limit > 100` to 100, `page < 1` to 1). Usecases take `Offset`/`Limit`: `offset = (page-1)*limit`. Paged output: `items`, `total`, `page`, `limit`, `has_more` (`has_more = page*limit < total`). Unpaged list tools return `items` (+ `total` only where the usecase returns a count).
- Tool responses: single `mcp.TextContent` with the `Out` struct JSON-marshalled. Tool errors: return the Go `error` from the handler → SDK sets `isError: true`.
- Auth: `Authorization: Bearer <api-key>`; missing/invalid/expired → HTTP 401 with JSON body `{"error":"..."}`. JWTs are rejected (key store only).
- Routes in `internal/adapter/api/server.go` must be registered **before** `api.Use(ContextResolver(contextRepo))` (line ~172) so they never require a context header.
- The skill file lives at `internal/adapter/mcp/skill.md` and is served with `//go:embed` (spec deviation: `go:embed` cannot reference `../`, so the file cannot live in `docs/skills/`).
- cicd tests: package `cicd_test`, Ginkgo; focused run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP"`. The suite has known pre-existing failures (tracking/form/banner/placement/campaign specs) — compare against baseline, don't treat them as regressions.
- MCP JSON-RPC (stateless, JSON responses): every request is a full JSON-RPC POST to `/api/mcp` with `{"jsonrpc":"2.0","id":N,"method":"...","params":{...}}`; responses are JSON-RPC `{"jsonrpc":"2.0","id":N,"result":{...}}`.

## Review Focus

1. **Expired API key** (key with `expires_at` in the past) → 401, no tool executes — pinned by Task 1 test "rejects expired key".
2. **JWT presented as Bearer token** (admin JWT is valid for REST, must NOT work on `/api/mcp`) → 401 — pinned by Task 1 test "rejects JWT".
3. **Missing/empty Authorization header** → 401 (not 500, not a tool result) — pinned by Task 1 test "rejects missing header".
4. **Cross-tenant read** (key for context A lists contacts) → never returns context B's data, even with a warm server cache — pinned by Task 3 test "does not leak other context's contacts".
5. **Out-of-range paging** (`limit: 500`, `page: 0`) → clamped to limit 100 / page 1, no SQL blowup — pinned by Task 3 test "clamps limit and page".

---

### Task 1: SDK dependency + mcp package core (auth middleware, stateless handler, server cache)

**Files:**
- Delete: `internal/adapter/mcp/server.go`, `internal/adapter/mcp/contacts_tools.go`, `internal/adapter/mcp/forms_tools.go` (unwired stubs, unreferenced anywhere)
- Create: `internal/adapter/mcp/server.go`
- Create: `cicd/mcp_test.go`
- Modify: `go.mod`, `go.sum` (via `go get`)

**Interfaces:**
- Consumes: `auth.AuthService.ValidateAPIKey(key string) (*auth.APIKey, error)` from `internal/domain/auth` (bcrypt scan; `APIKey{ID, ContextID, UserID, KeyHash, Name, ExpiresAt, CreatedAt}`); SDK `mcp.NewServer`, `mcp.NewStreamableHTTPHandler(getServer func(*http.Request) *mcp.Server, opts *mcp.StreamableHTTPOptions)`, `mcp.StreamableHTTPOptions{Stateless, JSONResponse}`.
- Produces (used by every later task):
  - `type Deps struct` in package `mcp` (import `github.com/hekemen/automata/internal/adapter/mcp`): fields `AuthService auth.AuthService`, `ContactRepo contact.Repository` (`internal/domain/contact`), `FormRepo form.Repository` (`internal/domain/form`), `BannerUsecase *banner.BannerUsecase` (`internal/usecase/banner`), `TrackingRepo tracking.Repository` (`internal/domain/tracking`), `EmailUsecase *emailusecase.EmailUsecase` (`internal/usecase/email`), `WebhookSvc *webhooksvc.Service` (`internal/infrastructure/webhook`).
  - `func NewHandler(deps Deps) http.Handler` — wraps SDK handler + auth middleware.
  - `func registerAllTools(s *mcp.Server, d Deps, contextID string)` — empty for now; each later task adds `registerXTools(s, d, contextID)` calls here.
  - cicd helpers (in `cicd/mcp_test.go`, package-level): `mcpCall(engine http.Handler, key string, id int, method string, params map[string]any) *httptest.ResponseRecorder` (POSTs JSON-RPC to `/api/mcp` with `Authorization: Bearer <key>`); `mcpToolText(w *httptest.ResponseRecorder) string` (unmarshals JSON-RPC response, asserts `result.isError == false`, returns `result.content[0].text`).

- [ ] **Step 1: Add the SDK dependency**

Run: `go get github.com/modelcontextprotocol/go-sdk@v1.8.0`
Expected: `go.mod` gains the require line; `go build ./...` still passes (stub compiles).

- [ ] **Step 2: Delete the stub files and write the failing cicd spec**

Delete the three stub files, then create `cicd/mcp_test.go` with package `cicd_test` containing:

- `mcpCall` and `mcpToolText` helpers exactly as in the Interfaces block above. `mcpCall` builds body `{"jsonrpc":"2.0","id":id,"method":method,"params":params}`, sets `Content-Type: application/json` and `Authorization: Bearer <key>` (empty key → no header).
- `Describe("MCP server (POST /api/mcp)", ...)` with a `BeforeEach` that (a) calls `infraauth.InitForTesting("test-secret-key-for-jwt-signing-32bytes", "test-cookie-secret-32bytes-xxxx!")`, (b) builds a **minimal** engine — `gin.New()` with `engine.POST("/api/mcp", gin.WrapH(mcpadapter.NewHandler(deps)))` where `deps = mcpadapter.Deps{AuthService: infraauth.NewServiceWithAPIKeys(ctxrepo.NewUserPostgresRepo(pool), auth_repo.NewApiKeyPostgresRepo(pool))}` — and (c) creates one context via SQL (`INSERT INTO contexts (id, slug, name, created_at, updated_at)`) plus one real API key by POSTing `{"name":"mcp-test-key-<uuid>"}` to `/api/admin/api-keys` with the admin JWT (from `loginAsAdmin(mini-engine-with-admin-routes)`) — simpler: reuse the `setupCreationServer()` + `loginAsAdmin(engine)` + POST-to-`/api/admin/api-keys` flow from `swagger_endpoints_test.go` to mint the key, then attach the MCP route to that same engine is NOT possible (route already built) — so instead mint the key through `setupCreationServer()` and use the **minimal engine** (built from `pool` + auth service) for the MCP requests themselves. Keep the minted `apiKey` + `contextID` in vars. `AfterEach` deletes `api_keys` rows and the test context.

  Tests (use `mcpCall` against the minimal engine):
  - "rejects missing header" → `mcpCall(engine, "", 1, "initialize", initParams)` → `Expect(w.Code).To(Equal(http.StatusUnauthorized))`.
  - "rejects invalid key" → same with `"sk-invalid"` → 401.
  - "rejects expired key" → insert a key row directly via SQL with `expires_at = NOW() - interval '1 hour'` (hash with bcrypt via `infraauth.NewService(...).HashPassword` is NOT needed — `ValidateAPIKey` bcrypt-compares, so store a bcrypt hash of a known plaintext: `bcrypt.GenerateFromPassword([]byte("sk-expired-test"), bcrypt.DefaultCost)`), then `mcpCall` with `"sk-expired-test"` → 401.
  - "rejects JWT" → mint an admin JWT via `loginAsAdmin` on `setupCreationServer()`, `mcpCall` with it → 401.
  - "initialize with valid key" → `mcpCall(engine, apiKey, 1, "initialize", map[string]any{"protocolVersion":"2025-06-18","capabilities":map[string]any{},"clientInfo":map[string]any{"name":"cicd","version":"1.0"}})` → 200, body contains `"serverInfo"`.
  - "tools/list returns empty tool list" → `tools/list` → 200, `result.tools` is an empty array (no tools registered yet).

- [ ] **Step 3: Run the tests, verify they fail**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP server" -v`
Expected: FAIL — `mcpadapter.NewHandler` undefined (compile error).

- [ ] **Step 4: Implement `internal/adapter/mcp/server.go`**

```go
package mcp

// (imports: context, crypto/subtle or strings, net/http, sync,
//  "github.com/modelcontextprotocol/go-sdk/mcp", domain + usecase packages)

type Deps struct { /* exact fields from Interfaces */ }

type apiKeyCtxKey struct{}

// NewHandler returns an http.Handler serving stateless streamable MCP at the
// mounted path. Every request must carry a valid Bearer API key.
func NewHandler(deps Deps) http.Handler {
	inner := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			v := r.Context().Value(apiKeyCtxKey{})
			apiKey, ok := v.(*auth.APIKey)
			if !ok {
				return nil // defensive: middleware always sets it; nil → 400
			}
			return getOrCreateServer(deps, apiKey.ContextID)
		},
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)
	return authMiddleware{deps: deps, inner: inner}
}

type authMiddleware struct {
	deps  Deps
	inner http.Handler
}

func (m authMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := bearerToken(r) // "Bearer X" header → X; "" when missing/malformed
	if key == "" {
		http.Error(w, `{"error":"missing api key"}`, http.StatusUnauthorized)
		return
	}
	apiKey, err := m.deps.AuthService.ValidateAPIKey(key)
	if err != nil {
		http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), apiKeyCtxKey{}, apiKey))
	m.inner.ServeHTTP(w, r)
}

var servers sync.Map // contextID -> *mcp.Server

func getOrCreateServer(deps Deps, contextID string) *mcp.Server {
	if v, ok := servers.Load(contextID); ok {
		return v.(*mcp.Server)
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "automata", Version: "v1.0.0"}, nil)
	registerAllTools(s, deps, contextID)
	servers.Store(contextID, s)
	return s
}

func registerAllTools(s *mcp.Server, d Deps, contextID string) {
	// per-domain registrations added in later tasks
}
```

Approach notes: `bearerToken` parses `strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))`; the `sync.Map` cache is package-level (one process, few contexts, unbounded by design); do NOT add an SDK `SchemaCache` option.

- [ ] **Step 5: Run the tests, verify they pass**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP server" -v`
Expected: PASS (all 6 tests).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/adapter/mcp/ cicd/mcp_test.go
git commit -m "feat(mcp): stateless streamable HTTP handler with per-request API key auth"
```

---

### Task 2: Wire `/api/mcp` + `/api/mcp-skill.md` into the API server

**Files:**
- Modify: `internal/adapter/api/server.go` (insert MCP block right after the swagger block, line ~59-66; before `api.Use(ContextResolver(...))` at ~172)
- Create: `internal/adapter/mcp/skill.md` (placeholder: `# Automata MCP\n\n(Full guide added in Task 9.)`)

**Interfaces:**
- Consumes: `mcpadapter.NewHandler(deps Deps)` (Task 1); vars already in scope in `NewServer`: `pool`, `authService`, `contactRepo` (~line 150), `formRepo` (~193), `trackingRepo` (~194), `bannerRepo` (~208), `emailUsecase` (~152, built via `emailusecase.NewEmailUsecaseWithContact(emailRepo, emailQueue, contactRepo)`), `webhookSvc` (~164, `webhook_svc.NewService(webhook_repo.New(pool), queue.NewWebhookQueue(pool))`).
- Produces: production routes `POST /api/mcp` and `GET /api/mcp-skill.md` (auth: `middleware.AuthMiddleware(authService)` — JWT **or** API key, matching the swagger endpoints); `mcpadapter.SkillMD` string variable (embedded `skill.md`) for the route handler.

- [ ] **Step 1: Write the failing tests (extend `cicd/mcp_test.go`)**

New `Describe("MCP wiring (full engine)", ...)` with `BeforeEach` = `setupCreationServer()` + `loginAsAdmin(engine)` + context + API key via `POST /api/admin/api-keys` (copy the `createAPIKey` closure pattern from `swagger_endpoints_test.go`), `AfterEach` cleans `api_keys` + context. Tests:
- "initialize over the production engine" → `mcpCall(engine, apiKey, 1, "initialize", initParams)` → 200.
- "GET /api/mcp-skill.md requires auth" → plain `GET` without header → 401.
- "GET /api/mcp-skill.md serves markdown with a valid API key" → `GET` with `Authorization: Bearer <apiKey>` → 200, `Content-Type` starts with `text/markdown`, body contains "Automata MCP".

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP wiring" -v`
Expected: FAIL — 404 on `/api/mcp` and `/api/mcp-skill.md`.

- [ ] **Step 3: Implement the wiring in `internal/adapter/api/server.go`**

Add import `mcpadapter "github.com/hekemen/automata/internal/adapter/mcp"` and `bannerusecase "github.com/hekemen/automata/internal/usecase/banner"`. After the swagger block, add:

```go
// MCP server (streamable HTTP, stateless). Registered before
// ContextResolver so it never requires a context header.
mcpDeps := mcpadapter.Deps{
	AuthService:   authService,
	ContactRepo:   contactRepo,
	FormRepo:      formRepo,
	BannerUsecase: bannerusecase.NewBannerUsecase(bannerRepo),
	TrackingRepo:  trackingRepo,
	EmailUsecase:  emailUsecase,
	WebhookSvc:    webhookSvc,
}
api.POST("/mcp", gin.WrapH(mcpadapter.NewHandler(mcpDeps)))
api.GET("/mcp-skill.md", middleware.AuthMiddleware(authService), func(c *gin.Context) {
	c.Data(http.StatusOK, "text/markdown", []byte(mcpadapter.SkillMD))
})
```

Placement constraint: `contactRepo`, `formRepo`, `trackingRepo`, `bannerRepo`, `emailUsecase`, `webhookSvc` are declared **later** in the function (lines 150-164) — move the MCP block to after the webhook routes (~line 173) but **before** `api.Use(ContextResolver(contextRepo))` (~line 175). Add `var SkillMD string //go:embed skill.md` in `internal/adapter/mcp/server.go` (same package as the directive; `skill.md` sits beside it).

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP" -v`
Expected: PASS (Tasks 1 + 2 specs).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/api/server.go internal/adapter/mcp/skill.md internal/adapter/mcp/server.go cicd/mcp_test.go
git commit -m "feat(mcp): wire /api/mcp and /api/mcp-skill.md routes into the API server"
```

---

### Task 3: Contacts tools (7)

**Files:**
- Create: `internal/adapter/mcp/contacts_tools.go`
- Modify: `internal/adapter/mcp/server.go` (call `registerContactTools` from `registerAllTools`)
- Test: `cicd/mcp_test.go` (extend)

**Interfaces:**
- Consumes: `Deps.ContactRepo contact.Repository`; usecases in `internal/usecase/contact`: `ListContacts(repo, contextID, ListOptions{Offset, Limit, Filters contact.FilterOptions{Tags []string, Source, MinSubmissions int, Company, Search string, Hidden *bool}}) (*contact.ContactWithCount, error)`; `GetContact(repo, id, contextID)`; `CreateContact(repo, contextID, CreateInput{Email *string, FirstName, LastName, Phone, Company string, CustomFields map[string]interface{}, Source, SourceID string, Tags []string})`; `UpdateContact(repo, id, contextID, UpdateInput{Email *string, FirstName, LastName, Phone, Company string, CustomFields map[string]interface{}, Tags []string})`; `DeleteContact(repo, id, contextID)`; `MergeContacts(repo, contextID, keepID, mergeIntoID)`; `GetActivity(repo, contactID, contextID, offset, limit int, activityType *string) (*contact.ActivityResult, error)`.
- Produces: tools `contacts_list`, `contacts_get`, `contacts_create`, `contacts_update`, `contacts_delete`, `contacts_merge`, `contacts_activity` (names/arguments pinned by Task 9 catalog test). Shared pattern code in this task is copied verbatim (with field changes) by Tasks 4-8:

  - **Paged list pattern** (`contacts_list`):

    ```go
    type contactsListIn struct {
        Page            int      `json:"page" jsonschema:"1-based page number, default 1"`
        Limit           int      `json:"limit" jsonschema:"page size, default 20, max 100"`
        Search          string   `json:"search"`
        Tags            []string `json:"tags"`
        Source          string   `json:"source"`
        Company         string   `json:"company"`
        MinSubmissions  int      `json:"min_submissions"`
        Hidden          *bool    `json:"hidden"`
    }
    type pageMeta struct {
        Total   int64 `json:"total"`
        Page    int   `json:"page"`
        Limit   int   `json:"limit"`
        HasMore bool  `json:"has_more"`
    }
    type contactsListOut struct {
        Items []contact.Contact `json:"items"` // from ContactWithCount.Contact list; check its field name
        pageMeta
    }

    func clampPage(page, limit int) (offset, page, limit int) {
        if page < 1 { page = 1 }
        if limit < 1 { limit = 20 }
        if limit > 100 { limit = 100 }
        return (page - 1) * limit, page, limit
    } // put clampPage + pageMeta in server.go (shared by all domains)

    mcp.AddTool(s, &mcp.Tool{Name: "contacts_list",
        Description: "List contacts for this context, paginated. Optional filters: search (email/name), tags, source, company, min_submissions, hidden."},
        func(ctx context.Context, req *mcp.CallToolRequest, in contactsListIn) (*mcp.CallToolResult, contactsListOut, error) {
            offset, page, limit := clampPage(in.Page, in.Limit)
            cw, err := contactusecase.ListContacts(d.ContactRepo, contextID, contactusecase.ListOptions{
                Offset: offset, Limit: limit,
                Filters: contact.FilterOptions{Tags: in.Tags, Source: in.Source, Company: in.Company,
                    MinSubmissions: in.MinSubmissions, Search: in.Search, Hidden: in.Hidden},
            })
            if err != nil { return nil, contactsListOut{}, err }
            return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextOfJSON(jsonOf(contactsListOut{Items: cw.Contact, pageMeta: pageMeta{cw.Count, page, limit, page*limit < cw.Count}}))}}, contactsListOut{}, nil
        })
    ```

    `jsonOf(v)` helper (server.go): marshals `v` and returns `*mcp.TextContent` — use the SDK's JSON content helper if present (`mcp.TextOfJSON` takes `any` in v1.8.0; if not, build `&mcp.TextContent{Text: string(b), Meta: ...}` with a json mime type — check `go doc mcp` and use what exists).
  - **Non-paged list** (`forms_list` etc. in Task 4): `items` array only.
  - **Create/update/delete/get**: In struct fields mirror the usecase input (snake_case `json` tags + `jsonschema` descriptions); Out is the returned domain object; delete returns `{"deleted":true}`.
  - Every handler: `(ctx, req, in In) (*mcp.CallToolResult, Out, error)`; on usecase error return `(nil, Out{}, err)`.

- [ ] **Step 1: Write the failing tests (extend `cicd/mcp_test.go`)**

New `Describe("MCP contacts tools", ...)` — same engine/key setup as Task 2's Describe (extract the BeforeEach into a package-level helper `setupMCPEngine() (engine http.Handler, apiKey, contextID string, adminJWT string)` if it would duplicate 3+ times; keep the existing Describes working). Tests:
- "contacts_create then contacts_get round-trip" → `contacts_create` with `{"email":"mcp-<uuid>@example.com","first_name":"MCP","company":"Acme"}` → toolText contains the email; `contacts_get` with the returned `id` → 200 tool, body contains `"Acme"`.
- "contacts_list returns paged shape" → after creating one contact: `contacts_list` `{"page":1,"limit":20}` → toolText JSON has `"items"`, `"total":1`, `"page":1`, `"limit":20`, `"has_more":false`.
- "contacts_update and contacts_delete" → update `company` → get shows new value; delete → `contacts_get` returns `isError: true`.
- "contacts_merge" → create two contacts, `contacts_merge` `{"keep_contact_id":A,"merge_contact_id":B}` → get A → 200; get B → `isError: true`.
- "contacts_activity" → `contacts_activity` `{"contact_id":A,"page":1,"limit":10}` → 200, body has `"items"` (empty array ok).
- **"does not leak other context's contacts"** (Review Focus #4): create a second context + key (same pattern as Task 1's second key); create a contact in context B; `contacts_list` with key A → toolText does NOT contain context B's email; with key B → it does.
- **"clamps limit and page"** (Review Focus #5): `contacts_list` `{"page":0,"limit":500}` → 200, toolText has `"limit":100` and `"page":1`.

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP contacts" -v`
Expected: FAIL — `tools/call` returns "unknown tool" JSON-RPC error.

- [ ] **Step 3: Implement `registerContactTools(s *mcp.Server, d Deps, contextID string)`**

Seven tools per the pattern above. Exact tool → usecase mapping:
- `contacts_list` → `ListContacts` (paged pattern above).
- `contacts_get` → `GetContact(repo, in.ContactID, contextID)`; In: `contact_id string`.
- `contacts_create` → `CreateContact`; In mirrors `CreateInput` (`email *string` etc.).
- `contacts_update` → `UpdateContact`; In: `contact_id` + `UpdateInput` fields.
- `contacts_delete` → `DeleteContact`; Out: `{"deleted":true}`.
- `contacts_merge` → `MergeContacts(repo, contextID, in.KeepContactID, in.MergeContactID)`; In: `keep_contact_id`, `merge_contact_id`.
- `contacts_activity` → `GetActivity`; In: `contact_id`, `page`, `limit`, `activity_type *string`; Out: items + `total` from `ActivityResult` (check its fields).

Call `registerContactTools` from `registerAllTools`.

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP" -v`
Expected: PASS (all MCP specs).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/ contacts tools cicd/mcp_test.go
git commit -m "feat(mcp): contacts tools (list/get/create/update/delete/merge/activity)"
```

---

### Task 4: Forms tools (8)

**Files:**
- Create: `internal/adapter/mcp/forms_tools.go`
- Modify: `internal/adapter/mcp/server.go` (call `registerFormTools` from `registerAllTools`)
- Test: `cicd/mcp_test.go`

**Interfaces:**
- Consumes: `Deps.FormRepo form.Repository`; usecases in `internal/usecase/form`: `ListForms(repo, contextID)` (unpaged); `GetForm(repo, contextID, slug)`; `CreateForm(repo, contextID, CreateFormInput{Slug, Name, Description string, Fields []form.FormField, Settings map[string]interface{}})`; `UpdateForm(repo, contextID, id, CreateFormInput)`; `DeleteForm(repo, contextID, id)`; `SubmitForm(repo, contextID, SubmitFormInput{Slug string, Data map[string]interface{}, Files []form.FileUpload, VisitorID string})`; `ListSubmissions(repo, formID, form.ListSubmissionsOptions{Offset, Limit int}) ([]*form.Submission, int64, error)`; `GetSubmission(repo, id)`.
- Produces: tools `forms_list`, `forms_get`, `forms_create`, `forms_update`, `forms_delete`, `forms_submit`, `form_submissions_list`, `form_submissions_get`. Argument names: `form_id` (= slug, since `GetForm` takes a slug and REST routes use `:id` = slug), `form_ids` never — `submission_id`, `fields` (`[]form.FormField` embedded directly in the In struct), `data` (map), `visitor_id`.

- [ ] **Step 1: Write the failing tests (extend `cicd/mcp_test.go`)**

`Describe("MCP forms tools", ...)`:
- "forms_create then forms_get" → create `{"slug":"mcp-<rand>","name":"MCP Form","fields":[{"name":"email","type":"email"}]}` → toolText contains the slug; `forms_get` `{"form_id":slug}` → 200.
- "forms_submit then form_submissions_list" → `forms_submit` `{"form_id":slug,"data":{"email":"sub-<uuid>@example.com"}}` → 200; `form_submissions_list` `{"form_id":slug,"page":1,"limit":10}` → `"total":1`.
- "form_submissions_get" → with the submission id from the create response → 200.

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP forms" -v`
Expected: FAIL — unknown tool.

- [ ] **Step 3: Implement `registerFormTools(s *mcp.Server, d Deps, contextID string)`**

Eight tools using Task 3's patterns. `form_submissions_list` is paged (items/total/page/limit/has_more); `forms_list` is unpaged (`items` only).

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/ cicd/mcp_test.go
git commit -m "feat(mcp): forms tools (CRUD, submit, submissions)"
```

---

### Task 5: Banner, placement, campaign tools (15)

**Files:**
- Create: `internal/adapter/mcp/banners_tools.go`, `internal/adapter/mcp/placements_tools.go`, `internal/adapter/mcp/campaigns_tools.go`
- Modify: `internal/adapter/mcp/server.go` (three register calls in `registerAllTools`)
- Test: `cicd/mcp_test.go`

**Interfaces:**
- Consumes: `Deps.BannerUsecase *banner.BannerUsecase` with methods: `ListBanners(contextID, bdomain.ListOptions{CampaignID string, IsActive bool, Offset, Limit int}) ([]*bdomain.Banner, int64, error)`, `GetBanner(id)`, `CreateBanner(contextID, *bdomain.CreateBannerRequest{Name string, Type bdomain.BannerType, Content, LinkURL, ImageURL, AltText, CampaignID string, Placements []string, Priority int, StartDate, EndDate time.Time, ABTest bool, ABVariants []bdomain.ABVariant})`, `UpdateBanner(contextID, id, *bdomain.UpdateBannerRequest)`, `DeleteBanner(id)`; `ListPlacements(contextID, isActive bool) ([]*bdomain.Placement, error)` (unpaged), `GetPlacement/UpdatePlacement/DeletePlacement` with `CreatePlacementRequest{Name, Location, CSSSelector string, MaxBanners, Priority int}`; `ListCampaigns(contextID, isActive bool) ([]*bdomain.Campaign, error)` (unpaged), `GetCampaign/UpdateCampaign/DeleteCampaign` with `CreateCampaignRequest{Name, Description string, StartDate, EndDate *time.Time, TargetURL, TrackingCode string}`.
- Produces: tools `banners_list` (paged), `banners_get/create/update/delete`; `placements_list` (unpaged, `is_active`), `placements_get/create/update/delete`; `campaigns_list` (unpaged, `is_active`), `campaigns_get/create/update/delete`.

- [ ] **Step 1: Write the failing tests (extend `cicd/mcp_test.go`)**

`Describe("MCP banner/placement/campaign tools", ...)`:
- "banners create/update/list/delete" → `campaigns_create` `{"name":"MCP Camp"}` → `campaigns_list` shows it; `banners_create` `{"name":"MCP Banner","campaign_id":<id>}` → `banners_list` `{"limit":100}` shows it with `"total":>=1`; `banners_update` `{"banner_id":id,"name":"Renamed"}` → get shows "Renamed"; `banners_delete` → get `isError`.
- "placements create/list/delete" → `placements_create` `{"name":"P1","css_selector":".slot"}` → `placements_list` contains it; delete → gone.
- "campaigns get/update/delete" → update name, get shows new name; delete → get `isError`.

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP banner" -v`
Expected: FAIL — unknown tool.

- [ ] **Step 3: Implement the three register functions**

Fifteen tools using Task 3's patterns. In structs mirror the `Create*Request` fields (snake_case; `is_active` for list filters; time fields as `time.Time` with `jsonschema` "RFC3339" hint).

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/ cicd/mcp_test.go
git commit -m "feat(mcp): banner, placement, and campaign tools"
```

---

### Task 6: Tracking tools (5)

**Files:**
- Create: `internal/adapter/mcp/tracking_tools.go`
- Modify: `internal/adapter/mcp/server.go` (call `registerTrackingTools`)
- Test: `cicd/mcp_test.go`

**Interfaces:**
- Consumes: `Deps.TrackingRepo tracking.Repository`; usecases in `internal/usecase/tracking`: `GetDashboard(repo, contextID, GetDashboardOptions{Start, End time.Time, ActiveMin, TopN int}) (*tracking.DashboardMetrics, error)`; `GetEvents(repo, contextID, tracking.EventFilter{Type tracking.EventType, EventName, URL string, Offset, Limit int}) ([]*tracking.Event, int64, error)`; `ListVisitors(repo, contextID, tracking.ListVisitorsOpts{Offset, Limit int, Sort string}) ([]*tracking.Visitor, int64, error)`; `GetVisitorEvents(repo, contextID, visitorID, tracking.GetVisitorEventsOpts{Offset, Limit int, Type tracking.EventType}) ([]*tracking.Event, int64, error)`; `GetVisitorDetail(repo, contextID, visitorID, limit int) (*VisitorDetailResponse, error)`.
- Produces: tools `tracking_dashboard` (args `start`, `end` RFC3339 strings, `active_min`, `top_n` — parse with `time.Parse(time.RFC3339, ...)`; zero values → "no filter"), `tracking_events` (paged; `type`, `event_name`, `url`), `tracking_visitors` (paged; `sort` in `first_seen|last_seen|page_views`), `tracking_visitor_events` (paged; `visitor_id`, `type`), `tracking_visitor_get` (`visitor_id`, `limit`).

- [ ] **Step 1: Write the failing tests (extend `cicd/mcp_test.go`)**

`Describe("MCP tracking tools", ...)`:
- "tracking_dashboard returns metrics" → seed one event via SQL (fixture `support.NewTestEvent(contextID, visitorID)` shape) then `tracking_dashboard` `{"top_n":5}` → 200, toolText contains `"page_views"` or another stable metrics key (assert on the JSON structure keys present in `DashboardMetrics` — check `internal/domain/tracking` for exact field names before writing assertions).
- "tracking_events and tracking_visitors are paged" → seed ≥2 events → `tracking_events` `{"limit":2}` → `"has_more"` matches total>2; `tracking_visitors` `{"limit":10}` → `"items"` non-empty after seeding a visitor.
- "tracking_visitor_get" → with a seeded visitor id → 200.

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP tracking" -v`
Expected: FAIL — unknown tool.

- [ ] **Step 3: Implement `registerTrackingTools(s *mcp.Server, d Deps, contextID string)`**

Five tools; paged ones use `clampPage`/`pageMeta`; `tracking_dashboard` and `tracking_visitor_get` are non-paged objects.

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/ cicd/mcp_test.go
git commit -m "feat(mcp): tracking tools (dashboard, events, visitors)"
```

---

### Task 7: Email tools (8)

**Files:**
- Create: `internal/adapter/mcp/email_tools.go`
- Modify: `internal/adapter/mcp/server.go` (call `registerEmailTools`)
- Test: `cicd/mcp_test.go`

**Interfaces:**
- Consumes: `Deps.EmailUsecase *emailusecase.EmailUsecase` with `Repo() email.Repository` (methods `List(contextID)`, `GetByID(id)`, `Create(tmpl *email.EmailTemplate)`, `Update(tmpl *email.EmailTemplate)`, `Delete(id)`), `SendEmail(contextID, email.SendEmailRequest{To string, ToArray []string, Template string, Variables map[string]string, SubjectOverride, BodyOverride *string, HTML *bool}) (string /*job id*/, error)`, `TestSend(contextID, email.TestSendRequest{To, Template string, Variables map[string]string}) (jobID, subject, preview string, err)`, `SendToContact(contextID, templateID, contactID string) (string, error)`.
- Produces: tools `email_templates_list` (unpaged), `email_templates_get`, `email_templates_create` (In: `key`, `subject`, `body_text`, `body_html`, `is_default bool`, `kind`, `form_id *string` — build `email.EmailTemplate{ContextID: contextID, ...}`), `email_templates_update` (`template_id` + same fields; get-then-merge-then-Update), `email_templates_delete`, `email_send` (In mirrors `SendEmailRequest`), `email_send_to_contact` (`template_id`, `contact_id`), `email_test_send` (In mirrors `TestSendRequest`; Out: `job_id`, `subject`, `preview`). Send tools return `{"job_id": "..."}`.

- [ ] **Step 1: Write the failing tests (extend `cicd/mcp_test.go`)**

`Describe("MCP email tools", ...)`:
- "email_templates create/list/get/update/delete" → `email_templates_create` `{"key":"mcp-<rand>","subject":"Hi {{name}}","body_text":"Hello"}` → list contains the key → get → update subject → get shows new subject → delete → get `isError`.
- "email_test_send renders a template" → `email_test_send` `{"to":"t@example.com","template":"mcp-<rand>","variables":{"name":"World"}}` → 200, toolText has `"subject"` and `"preview"`. (Do NOT assert a real SMTP send — TestSend queues a job; SMTP config is absent in tests, so if the job enqueue fails on SMTP config, assert the tool returns `isError: true` with a message INSTEAD — check behavior first with a quick manual run and pin whichever the current system does.)

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP email" -v`
Expected: FAIL — unknown tool.

- [ ] **Step 3: Implement `registerEmailTools(s *mcp.Server, d Deps, contextID string)`**

Eight tools per the mapping above.

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/ cicd/mcp_test.go
git commit -m "feat(mcp): email tools (templates CRUD, send, test send)"
```

---

### Task 8: Webhook tools (6)

**Files:**
- Create: `internal/adapter/mcp/webhooks_tools.go`
- Modify: `internal/adapter/mcp/server.go` (call `registerWebhookTools`)
- Test: `cicd/mcp_test.go`

**Interfaces:**
- Consumes: `Deps.WebhookSvc *webhooksvc.Service` with `CreateEndpoint(ctx, webhook.CreateWebhookInput{ContextID, Name, URL string, Events []string, Secret string, Active *bool}) (*webhook.WebhookResponse, error)` (MCP handler sets `ContextID: contextID` from the bound context), `UpdateEndpoint(ctx, id, webhook.UpdateWebhookInput{Name, URL string, Events []string, Secret string, Active *bool})`, `DeliverTest(ctx, endpointID) (string /*message*/, error)`, `Repo() webhook.Repository` (methods `ListByContext(contextID)`, `GetByID(id)`, `Delete(id)`).
- Produces: tools `webhooks_list` (unpaged), `webhooks_get`, `webhooks_create` (In: `name`, `url`, `events`, `secret`, `active *bool` — no `context_id`), `webhooks_update` (`webhook_id` + same fields), `webhooks_delete`, `webhooks_test` (`webhook_id`; Out: `{"message": ...}`).

- [ ] **Step 1: Write the failing tests (extend `cicd/mcp_test.go`)**

`Describe("MCP webhook tools", ...)`:
- "webhooks create/list/get/update/delete" → `webhooks_create` `{"name":"MCP Hook","url":"http://127.0.0.1:9/collect","events":["form.submitted"],"secret":"s3cret"}` → list contains name; get 200; update name → get shows it; delete → get `isError`.
- "webhooks_test" → `webhooks_test` with the created id → tool may be `isError: true` (delivery to a dead URL fails) — assert the call returns a JSON-RPC result (either success text or `isError: true`), never a 500.

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP webhook" -v`
Expected: FAIL — unknown tool.

- [ ] **Step 3: Implement `registerWebhookTools(s *mcp.Server, d Deps, contextID string)`**

Six tools per the mapping above.

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/ cicd/mcp_test.go
git commit -m "feat(mcp): webhook tools (CRUD, test delivery)"
```

---

### Task 9: Skill file content + full catalog test (49 tools)

**Files:**
- Modify: `internal/adapter/mcp/skill.md` (replace Task 2 placeholder with the full guide)
- Test: `cicd/mcp_test.go`

**Interfaces:**
- Consumes: all tools from Tasks 3-8.
- Produces: `GET /api/mcp-skill.md` serving the final guide; complete 49-tool catalog.

- [ ] **Step 1: Write the failing catalog test (extend `cicd/mcp_test.go`)**

`Describe("MCP tool catalog", ...)`:
- "exposes exactly the 49 catalog tools" → `tools/list` with a valid key → collect tool names from `result.tools[].name` → `Expect(names).To(HaveLen(49))` and `ContainElement` for each of the 49 names (list them all in the test, sorted): `banners_create, banners_delete, banners_get, banners_list, banners_update, campaigns_create, campaigns_delete, campaigns_get, campaigns_list, campaigns_update, contacts_activity, contacts_create, contacts_delete, contacts_get, contacts_list, contacts_merge, contacts_update, email_send, email_send_to_contact, email_templates_create, email_templates_delete, email_templates_get, email_templates_list, email_templates_update, email_test_send, form_submissions_get, form_submissions_list, forms_create, forms_delete, forms_get, forms_list, forms_submit, forms_update, placements_create, placements_delete, placements_get, placements_list, placements_update, tracking_dashboard, tracking_events, tracking_visitor_events, tracking_visitor_get, tracking_visitors, webhooks_create, webhooks_delete, webhooks_get, webhooks_list, webhooks_test, webhooks_update`.
- "skill file served from the production engine lists domains" → `GET /api/mcp-skill.md` with API key → body contains "## Contacts" and "contacts_list" and "Authorization".

- [ ] **Step 2: Run the test, verify it fails**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP tool catalog" -v`
Expected: FAIL (49-tool list and/or skill content missing).

- [ ] **Step 3: Write the full skill file `internal/adapter/mcp/skill.md`**

Structure (markdown, ~100 lines): title `# Automata MCP`; 1-line what-it-is; **Connecting** (endpoint `<base>/api/mcp`, auth `Authorization: Bearer <API key>` from Settings → API Keys, one key = one tenant, tools take no `context_id`); **Paging** (`page` 1-based, `limit` default 20 max 100; responses `items`/`total`/`page`/`limit`/`has_more`); **Errors** (tool errors come back as MCP tool errors — read the message); then per-domain sections (`## Contacts`, `## Forms`, `## Banners`, `## Placements`, `## Campaigns`, `## Tracking`, `## Email`, `## Webhooks`) each listing tool name + one-line purpose + key arguments.

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./cicd/ -count=1 -timeout 300s -run TestCICD -ginkgo.focus "MCP" -v`
Expected: PASS — all MCP specs green.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/skill.md cicd/mcp_test.go
git commit -m "feat(mcp): skill file and full 49-tool catalog test"
```

---

### Task 10: Web UI MCP settings tab

**Files:**
- Create: `webui/src/components/settings/McpPanel.vue`
- Modify: `webui/src/views/SettingsView.vue` (tabs array ~line 13-18; import + render `McpPanel` in the panel switch)
- Modify: `webui/src/api/client.ts` (add `fetchMcpSkill` next to `fetchApiDocs` ~line 281)
- Rebuild: `web/` (build artifact)

**Interfaces:**
- Consumes: `fetchMcpSkill(): Promise<string>` → `client.get('/mcp-skill.md', { responseType: 'text' })` (mirrors `fetchApiDocs`); panel UI pattern from `ApiKeysPanel.vue` (Card, Button, lucide icons); SettingsView `tabs` array entry `{ id: 'mcp', label: 'MCP' }`.
- Produces: an MCP tab showing (1) the endpoint URL `{window.location.origin}/api/mcp` with a copy button, (2) a copy-ready JSON connection block `{"url":"<endpoint>","headers":{"Authorization":"Bearer <YOUR_API_KEY>"}}` with a copy button, (3) the skill file loaded via `fetchMcpSkill()` with View (pre block) / Copy / Download buttons (download via `Blob` + `URL.createObjectURL` + anchor click, filename `automata-mcp.md`).

- [ ] **Step 1: Add `fetchMcpSkill` to `webui/src/api/client.ts`**

```ts
// MCP skill file (auth required — plain links would 401 without a Bearer header)
export async function fetchMcpSkill(): Promise<string> {
  const response = await client.get('/mcp-skill.md', { responseType: 'text' })
  return response.data as string
}
```

- [ ] **Step 2: Create `webui/src/components/settings/McpPanel.vue`**

Vue 3 `<script setup lang="ts">` + Tailwind, following `ApiKeysPanel.vue` conventions: `computed` endpoint from `window.location.origin + '/api/mcp'`; copy via `navigator.clipboard.writeText` with a transient "Copied" state; skill markdown loaded on `onMounted` into a ref, shown in a `<pre>` block; download as above.

- [ ] **Step 3: Wire the tab in `webui/src/views/SettingsView.vue`**

Add `{ id: 'mcp', label: 'MCP' }` to `tabs`, import `McpPanel`, and add its case to the panel rendering (same shape as `api-keys`).

- [ ] **Step 4: Build and verify**

Run: `cd webui && npm run build`
Expected: build succeeds, `web/` regenerated. Then run `go build ./...` at the repo root (unchanged, sanity). Verify the built output includes the new panel: `grep -r "mcp-skill" web/assets/ | head -3` (minified chunk contains the route string).

- [ ] **Step 5: Commit**

```bash
git add webui/src/ web/
git commit -m "feat(webui): MCP settings tab with endpoint, connection block, skill file"
```

---

### Task 11: Full suite verification against baseline

**Files:**
- None (verification only; fix any regressions found)

**Interfaces:**
- Consumes: everything from Tasks 1-10.

- [ ] **Step 1: Run the full cicd suite**

Run: `go test ./cicd/... -count=1 -timeout 600s -v 2>&1 | tail -40`
Expected: the MCP specs all pass; non-MCP failures match the known pre-existing baseline (tracking/form/banner/placement/campaign specs). Any NEW failure outside the baseline is a regression to fix before finishing.

- [ ] **Step 2: Race-check the mcp package (server cache is shared state)**

Run: `go test ./cicd/ -count=1 -timeout 600s -race -run TestCICD -ginkgo.focus "MCP"`
Expected: PASS, no data races.

- [ ] **Step 3: Final commit (if Step 1/2 produced fixes)**

```bash
git add -A
git commit -m "fix(mcp): full-suite fixes"
```

- [ ] **Step 4: Update spec/plan status**

Append an "Execution Status" section (per repo convention in `docs/superpowers/plans/`) to this plan file noting all tasks complete, the suite result vs baseline, and the `web/` rebuild. Commit: `git add docs/superpowers/plans/2026-10-21-mcp-server.md && git commit -m "docs: mark MCP plan executed"`.

## Execution Status

**Completed.** All 11 tasks executed on branch `feat/mcp-server` (commits `aa655be..15bbf20`); per-task rulings are in the SDD ledger at `.superpowers/sdd/2026-10-21-mcp-server/progress.md`.

- Full cicd suite vs baseline: MCP focus 32/32 specs green (incl. the 49-tool catalog and skill-file content); full suite 438/441 with only the known pre-existing `tracking_server` 200/204 failure plus the known repo-CRUD full-suite flakes (contact/banner CRUD), both green in isolation. Race check on the MCP focus (`-race`) passes.
- Notable fixes along the way: latent `webhookRepo.Create` double-UUID bug (returned ID never matched the stored row) and the tracking `get_visitor_detail` nil-context fix.
- `web/` rebuilt via `webui && npm run build` (Task 10); `go build ./...` clean.