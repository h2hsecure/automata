# Automata MCP Server — Design Spec

Date: 2026-10-21
Status: Approved (brainstormed)

## 1. Goal

Expose Automata's context-scoped API functionality as an MCP (Model Context Protocol)
server so AI clients (Claude, MCP-capable IDEs, etc.) can read and write marketing
data — contacts, forms, banners/placements/campaigns, tracking, email, webhooks —
directly.

- Transport: **streamable HTTP** at `POST /api/mcp` on the existing main API (port 8080).
- Auth: **per-request Bearer API key** (context-scoped; one key = one tenant).
- Data access: **direct calls into the usecase/service layer** — no REST proxying.
- Web UI: an **MCP tab** in Settings with the endpoint URL, a copy-ready connection
  block, and view/copy/download of a **skill file** (`docs/skills/automata-mcp.md`).
- SDK: official `github.com/modelcontextprotocol/go-sdk` (v1.8.0).

## 2. Architecture

### 2.1 Wiring

- `internal/adapter/mcp/` (replaces the existing unwired stub — `server.go`,
  `contacts_tools.go`, `forms_tools.go` are removed and rebuilt).
- Constructor (in the mcp package):

  ```go
  func NewHandler(deps Deps) http.Handler // wraps the SDK handler with auth, 2.2
  ```

  where `Deps` carries the auth service plus the usecase/repo objects the tools
  need (contact repo, form repo, `*banner.BannerUsecase`, tracking repo,
  `*email.EmailUsecase`, webhook service + repo).
- Mounted in the API server construction (`internal/adapter/api/server.go`):
  `api.POST("/mcp", gin.WrapH(handler))` (the `api` group already carries the
  `/api` prefix → full path `POST /api/mcp`), registered **before** the
  `ContextResolver` middleware so it never requires a `Context` header.
- `cmd/automata/main.go` builds the `Deps` and passes the handler to the API
  server constructor.
- `go.mod`: add `github.com/modelcontextprotocol/go-sdk` (v1.8.0).

### 2.2 Per-request auth (API key, 401)

A plain `net/http` middleware wraps the SDK handler:

1. Read `Authorization: Bearer <key>` (API keys only — no JWT path).
2. `authService.ValidateAPIKey(key)` → `*domainauth.APIKey` (gives `ContextID`,
   `UserID`, `Name`).
3. Missing/invalid/expired → `401 Unauthorized` (plain JSON body), request stops.
4. Valid → stash the `APIKey` in the request context
   (`context.WithValue(r.Context(), apiKeyKey, apiKey)`) and call the inner handler.

JWTs are naturally rejected because `ValidateAPIKey` only matches bcrypt-hashed API
keys.

### 2.3 Streamable HTTP handler, stateless

```go
mcp.NewStreamableHTTPHandler(
    func(r *http.Request) *mcp.Server {
        apiKey := apiKeyFromRequestContext(r.Context()) // stashed by middleware
        return serverCache.GetOrBuild(apiKey.ContextID)  // 2.4
    },
    &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
)
```

- `Stateless: true` → no session store, one temporary session per request; each
  request re-authenticates.
- `JSONResponse: true` → `application/json` responses (no SSE) — simplest for
  stateless.
- If the middleware let a request through with no key (defensive), `getServer`
  returns `nil` → SDK answers 400.

### 2.4 Per-context server cache

- `sync.Map`: `contextID → *mcp.Server`.
- On miss: `mcp.NewServer(&mcp.Implementation{Name: "automata", Version: ...}, nil)`
  and register every tool via `mcp.AddTool(server, &mcp.Tool{Name, Description},
  handler)`.
- **`contextID` is captured in the tool closures at registration time** — handlers
  never read the tenant from the request context, so there is no cross-tenant
  bleed risk even if caching ever changes.
- Contexts are long-lived and few; the cache is unbounded by design (one entry per
  context).

### 2.5 Tool handler shape (SDK)

```go
mcp.AddTool(server, &mcp.Tool{Name: "contacts_list", Description: "..."},
    func(ctx context.Context, req *mcp.CallToolRequest, in contactsListIn) (*mcp.CallToolResult, contactsListOut, error) {
        // in → usecase call → out
    })
```

- Schema is inferred from the `In`/`Out` Go structs; field descriptions come from
  `jsonschema` struct tags.
- Returning `(nil, zeroOut, err)` makes the SDK mark the result `IsError: true`.

## 3. Tool conventions

- **Names**: `^[a-z0-9_]{1,64}$`, pattern `<domain>_<action>` (`contacts_list`,
  `contacts_get`, `contacts_create`, `contacts_update`, `contacts_delete`, …).
  One tool per REST endpoint — no mega-tools.
- **Tenant binding**: no `context_id` parameter on any tool; every tool is bound to
  the key's context.
- **Paging**: tools that page accept `page` (1-based, default 1) and `limit`
  (default 20, max 100, clamped server-side). List usecases take
  `Offset`/`Limit`, so handlers compute `offset = (page-1)*limit`.
- **Paged output shape**: `items`, `total`, `page`, `limit`, `has_more` (all
  `jsonschema`-tagged). Unpaged list tools return a plain `items` array (+ `total`
  where the usecase provides a count).
- **Responses**: `application/json` text content (single `TextContent` with the
  marshalled `Out` struct).
- **Errors**: usecase errors are returned as Go errors → MCP `IsError` tool results
  with the message text. Validation errors (e.g. bad ID format, empty email) are
  the same.
- **IDs in input**: `snake_case` fields (`contact_id`, `form_id`, …) mapped to the
  usecase arguments.

## 4. Tool catalog (51 tools)

### 4.1 Contacts (7)

| Tool | Usecase | Params | Notes |
|---|---|---|---|
| `contacts_list` | `contact.ListContacts` | page, limit, search, tags, source, company, min_submissions, hidden | paged |
| `contacts_get` | `contact.GetContact` | contact_id | |
| `contacts_create` | `contact.CreateContact` | email, first_name, last_name, phone, company, custom_fields, source, source_id, tags | upserts on duplicate email (returns existing) |
| `contacts_update` | `contact.UpdateContact` | contact_id + same fields | |
| `contacts_delete` | `contact.DeleteContact` | contact_id | |
| `contacts_merge` | `contact.MergeContacts` | keep_contact_id, merge_contact_id | |
| `contacts_activity` | `contact.GetActivity` | contact_id, page, limit, activity_type | paged |

### 4.2 Forms (8)

| Tool | Usecase | Params | Notes |
|---|---|---|---|
| `forms_list` | `form.ListForms` | — | unpaged |
| `forms_get` | `form.GetForm` | form_id (slug) | |
| `forms_create` | `form.CreateForm` | name, slug, description, fields (json), … | mirrors `CreateFormInput` |
| `forms_update` | `form.UpdateForm` | form_id + create fields | |
| `forms_delete` | `form.DeleteForm` | form_id | |
| `forms_submit` | `form.SubmitForm` | form_id, fields (json object) | mirrors `SubmitFormInput` |
| `form_submissions_list` | `form.ListSubmissions` | form_id, page, limit | paged |
| `form_submissions_get` | `form.GetSubmission` | submission_id | |

### 4.3 Banners (5)

| Tool | Usecase | Params | Notes |
|---|---|---|---|
| `banners_list` | `BannerUsecase.ListBanners` | campaign_id, is_active, page, limit | paged |
| `banners_get` | `GetBanner` | banner_id | |
| `banners_create` | `CreateBanner` | title, image_url, link_url, campaign_id, is_active, … | mirrors `CreateBannerRequest` |
| `banners_update` | `UpdateBanner` | banner_id + create fields | |
| `banners_delete` | `DeleteBanner` | banner_id | |

### 4.4 Placements (5)

| Tool | Usecase | Params | Notes |
|---|---|---|---|
| `placements_list` | `ListPlacements` | is_active | unpaged |
| `placements_get` | `GetPlacement` | placement_id | |
| `placements_create` | `CreatePlacement` | name, selector, campaign_id, is_active, … | mirrors `CreatePlacementRequest` |
| `placements_update` | `UpdatePlacement` | placement_id + create fields | |
| `placements_delete` | `DeletePlacement` | placement_id | |

### 4.5 Campaigns (5)

| Tool | Usecase | Params | Notes |
|---|---|---|---|
| `campaigns_list` | `ListCampaigns` | is_active | unpaged |
| `campaigns_get` | `GetCampaign` | campaign_id | |
| `campaigns_create` | `CreateCampaign` | name, description, is_active, … | mirrors `CreateCampaignRequest` |
| `campaigns_update` | `UpdateCampaign` | campaign_id + create fields | |
| `campaigns_delete` | `DeleteCampaign` | campaign_id | |

### 4.6 Tracking (5)

| Tool | Usecase | Params | Notes |
|---|---|---|---|
| `tracking_dashboard` | `GetDashboard` | start, end, active_min, top_n | metrics object |
| `tracking_events` | `GetEvents` | type, event_name, url, page, limit | paged |
| `tracking_visitors` | `ListVisitors` | page, limit, sort (`first_seen`\|`last_seen`\|`page_views`) | paged |
| `tracking_visitor_events` | `GetVisitorEvents` | visitor_id, page, limit, type | paged |
| `tracking_visitor_get` | `GetVisitorDetail` | visitor_id, limit | |

### 4.7 Email (8)

Template CRUD goes through `EmailUsecase.Repo()` (same as the REST handlers).
Templates are context-scoped (`EmailTemplate.ContextID`).

| Tool | Call | Params | Notes |
|---|---|---|---|
| `email_templates_list` | `Repo().List(contextID)` | — | unpaged |
| `email_templates_get` | `Repo().GetByID` | template_id | |
| `email_templates_create` | `Repo().Create` | key, subject, body_text, body_html, is_default, kind, form_id | |
| `email_templates_update` | `Repo().GetByID` + `Repo().Update` | template_id + fields | partial update |
| `email_templates_delete` | `Repo().Delete` | template_id | |
| `email_send` | `SendEmail` | to, subject, body (text/html), … | returns job id |
| `email_send_to_contact` | `SendToContact` | template_id, contact_id | returns job id |
| `email_test_send` | `TestSend` | template_id or ad-hoc fields | returns job id + subject + preview |

### 4.8 Webhooks (6)

Webhook endpoints are context-scoped (`WebhookEndpoint.ContextID`); the REST routes
sit under `/admin` with the auth middleware, but the repo queries are by context, so
MCP tools bind to the key's context via `ListByContext`.

| Tool | Call | Params | Notes |
|---|---|---|---|
| `webhooks_list` | `repo.ListByContext(contextID)` | — | unpaged |
| `webhooks_get` | `repo.GetByID` | webhook_id | |
| `webhooks_create` | `service.CreateEndpoint` | name, url, events, secret, is_active | |
| `webhooks_update` | `service.UpdateEndpoint` | webhook_id + fields | |
| `webhooks_delete` | `repo.Delete` | webhook_id | |
| `webhooks_test` | `service.DeliverTest` | webhook_id | |

## 5. Skill file

- **Path**: `docs/skills/automata-mcp.md` (single source of truth, committed).
- **Served**: `GET /api/mcp-skill.md` with `middleware.AuthMiddleware(authService)`,
  content-type `text/markdown` — mirrors the existing `swagger.yaml` pattern.
  Embedded at build time with `//go:embed` (relative path into `docs/skills/`).
- **Content**: short usage guide an AI client can load as a skill:
  1. What Automata is, and that tools are tenant-bound (no `context_id` args).
  2. Auth: set `Authorization: Bearer <API key>` (key from Settings → API Keys).
  3. Endpoint: `<base>/api/mcp`.
  4. Tool overview by domain (one line per tool) + paging convention
     (`page`/`limit` in, `items`/`total`/`has_more` out, max limit 100).
  5. Error convention: tool errors come back as MCP tool errors.

## 6. Web UI — MCP settings tab

- New tab `mcp` in `webui/src/views/SettingsView.vue` (tabs array) with a new
  `webui/src/components/settings/McpPanel.vue` (pattern: `ApiKeysPanel.vue`).
- Panel contents:
  1. **Endpoint**: the MCP URL derived from the API client base URL
     (`{base}/mcp`) with a copy button.
  2. **Connection block**: ready-to-paste client config (JSON) with `url` and
     `headers: { Authorization: "Bearer <key>" }` placeholder; user pastes their
     key (no key storage on the server for this).
  3. **Skill file**: "View / Copy / Download" — fetched via
     `fetchMcpSkill()` in `webui/src/api/client.ts`
     (`GET /mcp-skill.md`, `responseType: 'text'`, mirroring `fetchApiDocs`).
- Rebuild `web/` assets after webui changes (existing build flow).

## 7. Testing (cicd Ginkgo suite)

New spec `cicd/mcp_test.go` (or `mcp/` package) using the existing suite bootstrap
(testcontainer Postgres + gin engine from `suite_test.go` / `support/`):

- **Auth**: `POST /api/mcp` with no/invalid/expired key → 401.
- **Session**: valid key + `initialize` request → 200 (stateless JSON response);
  `tools/list` returns all 51 tools with names matching the catalog.
- **Read tools**: create a contact via the REST API in a `BeforeEach`, then
  `contacts_list` / `contacts_get` via MCP return it (JSON-RPC `tools/call` over
  HTTP).
- **Write tools**: `contacts_create` via MCP → visible via REST.
- **Paging**: create > limit contacts, `contacts_list` with `limit=2` →
  `has_more=true`, `total` correct; `page=2` returns the rest.
- **Tenant isolation** (best effort): two contexts/keys — key A's `contacts_list`
  does not return key B's contact.
- Compare suite results against the known pre-existing-failure baseline
  (tracking/form/banner/placement/campaign specs).

## 8. Out of scope

- Admin/platform features (users, contexts, API-key management, email queue).
- JWT auth on the MCP endpoint.
- SSE streaming responses, session persistence, MCP prompts/resources.
- Contact import/export/segmentation tools (REST-only for now).
- OAuth / MCP auth-server flow.