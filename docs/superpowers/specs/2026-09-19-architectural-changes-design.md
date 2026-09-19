# Architectural Changes: Dual-Server, Admin Config, Banner Management, Tracking

**Date:** 2026-09-19
**Status:** Draft
**Scope:** Backend architecture split, new admin config subsystem, banner management wiring, tracking server, frontend fixes

---

## 1. Problem Statement

The current Automata platform has four issues to resolve:

1. **Form creation frontend broken** — Form routes exist in Go handlers but are not registered in `server.go`. The frontend API client calls endpoints that don't exist.
2. **Banner management missing** — Full banner/placement/campaign CRUD handlers exist in Go code but are never wired to HTTP routes and have no frontend page.
3. **Single server handles everything** — Admin API, public tracking, form submissions, and banner serving all run on one Gin server. Tracking endpoints (public-facing, behind reverse proxy) should be on a separate server for isolation.
4. **No per-tenant configuration** — Tenants can't configure their own settings (CORS origins, domain). Configuration is file-based only.

---

## 2. Architecture Overview

### Current State

```
┌─────────────────────────────────────────────────┐
│              Single Gin Server (8080)            │
│                                                  │
│  /api/health          → public                   │
│  /api/auth/*          → public                   │
│  /api/admin/*         → JWT auth                 │
│  /api/contacts/*      → tenant-resolved          │
│  /api/forms/*         → NOT registered           │
│  /api/banners/*       → NOT registered           │
│  /api/tracking/*      → tenant-resolved          │
│  /form/*              → proxy stub               │
│  /snippet/*           → proxy stub               │
│  /static/*            → static files             │
│  /*                   → SPA fallback             │
└─────────────────────────────────────────────────┘
```

### Target State

```
┌──────────────────────┐     ┌──────────────────────────┐
│  Main Server (8080)  │     │  Tracking Server (8081)   │
│                      │     │                          │
│  /api/health         │     │  /snippet/:id.js         │
│  /api/auth/*         │     │  /track                  │
│  /api/admin/*        │     │  /track/batch            │
│  /api/contacts/*     │     │  /track/banner           │
│  /api/forms/*        │     │  /form/submit/:slug      │
│  /api/banners/*      │     │  /form/render/:slug      │
│  /api/tracking/*     │     │                          │
│  /api/config/*       │     │  (no auth, no SPA)       │
│  /* → SPA fallback   │     │  /* → 404               │
└──────────────────────┘     └──────────────────────────┘
         │                              │
         └──────────┬───────────────────┘
                    │
            Reverse Proxy (nginx/envoy)
            /api/* → Main Server
            /*     → Tracking Server
```

**Reverse proxy routing:**
- Requests to `/api/*` → Main server (admin API, auth, tenant-resolved routes)
- Requests to `/` or `/snippet/*` or `/track/*` or `/form/*` → Tracking server
- The SPA at root `/` is served by the Main server (it handles `/*` SPA fallback)

---

## 3. Design Sections

### 3.1 Tenant Model Change: Users Are Global, Tenants Are Websites

**Current:** Users are scoped to tenants (`tenant_users` table with `tenant_id` FK). Login requires a tenant slug. JWT contains `tenant_slug`.

**Problem:** This models "users belong to one tenant." But the actual use case is "users manage multiple websites (tenants)."

**Approach (minimal change):**
- Keep `tenant_users` table as-is (users are tenant-scoped by design — each website has its own user accounts). The login flow changes:
  - Login no longer requires a `tenant` field
  - Instead, login by email only (finds user across all tenants)
  - Login returns JWT with `user_id` + `user_email` claims (no `tenant_slug`)
  - Login response body includes `tenants: [{id, slug, name}]` — all tenants the user belongs to
  - All tenant-resolved routes switch from `c.GetString("tenant_id")` to `c.GetHeader("X-Tenant-ID")`
  - Frontend login page removes the tenant selector/input
  - Frontend stores selected tenant in Pinia store, adds `X-Tenant-ID` header to all API requests

**Files affected:**
- `internal/domain/tenant/user_repository.go` — add `GetByEmailGlobal(email string)` method
- `internal/infrastructure/tenant/repo/user_postgres.go` — implement `GetByEmailGlobal`
- `internal/domain/auth/auth.go` — update `Login` signature: `Login(email, password string) (token, userID string, tenants []Tenant, err error)`
- `internal/infrastructure/auth/service.go` — update `Login` to find user across all tenants
- `internal/adapter/api/handler/auth_handler.go` — remove `Tenant` from `LoginRequest`, update handler
- `internal/adapter/api/middleware/auth.go` — update to extract tenant from `X-Tenant-ID` header instead of JWT
- `internal/adapter/api/tenant_middleware.go` — update tenant resolution to prefer `X-Tenant-ID` header
- `webui/src/api/client.ts` — add `X-Tenant-ID` header from Pinia store
- `webui/src/stores/auth.ts` — store tenant list, add tenant switcher
- `webui/src/views/LoginView.vue` — remove tenant field from login form
- All handler files that use `c.GetString("tenant_id")` — switch to `c.GetHeader("X-Tenant-ID")`

### 3.2 Admin Config Subsystem

**Purpose:** Allow admins to configure per-tenant settings (CORS origins, domain, display name) via API and frontend.

**Database:**
```sql
CREATE TABLE IF NOT EXISTS admin_configs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key         VARCHAR(128) NOT NULL,
    value       JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, key)
);

CREATE INDEX IF NOT EXISTS idx_admin_configs_tenant ON admin_configs(tenant_id);
```

**Config keys per tenant:**
- `cors` → `{ "origins": ["https://mysite.com", "https://www.mysite.com"] }`
- `domain` → `{ "primary": "mysite.com", "aliases": ["www.mysite.com"] }`
- `display` → `{ "name": "My Website", "logo_url": "..." }`

**API endpoints (admin-only, under `/api/admin/configs`):**
- `GET /api/admin/configs/:tenantId` — get all config for a tenant
- `PUT /api/admin/configs/:tenantId/cors` — update CORS origins
- `PUT /api/admin/configs/:tenantId/domain` — update domain settings
- `PUT /api/admin/configs/:tenantId/display` — update display settings

**Repository:**
- `internal/infrastructure/config/repo/postgres.go` — new file, CRUD for admin_configs
- `internal/domain/config/config.go` — new file, domain types and interface

**CORS middleware:**
- `internal/adapter/api/middleware/cors.go` — new file, reads CORS origins from admin config repo, applies `Access-Control-Allow-Origin` header
- Applied to tenant-resolved routes (forms, banners, contacts)

**Frontend:**
- `webui/src/views/ConfigView.vue` — new page for managing tenant configurations
- `webui/src/api/index.ts` — add `configsApi` module
- `webui/src/types/api.ts` — add config types
- `webui/src/components/layout/Sidebar.vue` — add "Config" nav item

### 3.3 Banner Management Wiring

**Current state:** Banner, Placement, and Campaign handlers exist in `internal/adapter/api/handler/banner_handler.go` with full CRUD. Banner usecase exists in `internal/usecase/banner/banner.go`. Banner PostgreSQL repository exists in `internal/infrastructure/banner/repo/postgres.go`. Banner migrations exist. **None are wired into `server.go`.**

**Change:** Register banner routes in the main server under `/api/admin/banners`, `/api/admin/placements`, `/api/admin/campaigns` with auth middleware.

**Routes to add to `server.go`:**
```go
// Under admin group (auth required):
admin.POST("/banners", bannerHandler.CreateBanner)
admin.GET("/banners", bannerHandler.ListBanners)
admin.GET("/banners/:id", bannerHandler.GetBanner)
admin.PUT("/banners/:id", bannerHandler.UpdateBanner)
admin.DELETE("/banners/:id", bannerHandler.DeleteBanner)

admin.POST("/placements", placementHandler.CreatePlacement)
admin.GET("/placements", placementHandler.ListPlacements)
admin.GET("/placements/:id", placementHandler.GetPlacement)
admin.PUT("/placements/:id", placementHandler.UpdatePlacement)
admin.DELETE("/placements/:id", placementHandler.DeletePlacement)

admin.POST("/campaigns", campaignHandler.CreateCampaign)
admin.GET("/campaigns", campaignHandler.ListCampaigns)
admin.GET("/campaigns/:id", campaignHandler.GetCampaign)
admin.PUT("/campaigns/:id", campaignHandler.UpdateCampaign)
admin.DELETE("/campaigns/:id", campaignHandler.DeleteCampaign)
```

**Frontend:**
- `webui/src/views/BannersView.vue` — new page with banner/placement/campaign management
- `webui/src/api/index.ts` — add `bannersApi`, `placementsApi`, `campaignsApi` modules
- `webui/src/types/api.ts` — add banner types
- `webui/src/components/layout/Sidebar.vue` — add "Banners" nav item

### 3.4 Form Routes Registration

**Current state:** `FormHandler` exists with full CRUD + submissions. Form repository exists. Form migrations exist. **Routes are NOT registered in `server.go`.**

**Change:** Register form routes in the main server.

**Routes to add to `server.go`:**
```go
// Under admin group (auth required) — form management:
admin.POST("/forms", formHandler.Create)
admin.GET("/forms", formHandler.List)
admin.GET("/forms/:id", formHandler.Get)
admin.PUT("/forms/:id", formHandler.Update)
admin.DELETE("/forms/:id", formHandler.Delete)
admin.GET("/forms/:form_id/submissions", formHandler.ListSubmissions)
admin.GET("/forms/submissions/:id", formHandler.GetSubmission)

// Under tenant-resolved (public, for form submissions):
api.POST("/forms/slug/:slug/submit", formHandler.SubmitForm)
api.GET("/forms/slug/:slug", formHandler.Get)  // for rendering
```

**Frontend fix:** The frontend `formsApi` in `webui/src/api/index.ts` already calls the correct endpoints (`/forms`, `/forms/slug/:slug/submit`). The issue is that these routes don't exist on the server. Once registered, the frontend should work.

### 3.5 Tracking Server Split

**Purpose:** Public-facing tracking endpoints run on a separate server, behind reverse proxy. No auth, no SPA, no admin routes.

**Server:** `internal/adapter/tracking/server.go` already exists as a standalone Gin engine. Needs enhancement.

**Tracking server routes:**
```
GET  /snippet/:id.js              — tracking script (JS)
POST /track                       — single event tracking
POST /track/batch                 — batch event tracking
GET  /track/banner                — banner pixel tracking
GET  /form/render/:slug           — render form HTML for tenant
POST /form/submit/:slug           — submit form data (public)
GET  /form/config/:slug           — get form config (fields, settings)
```

**Tenant resolution on tracking server:**
- `X-Tenant-ID` header (from tracking script or form embed)
- Subdomain fallback (e.g., `demo.automata.local` → tenant slug "demo")
- No auth required — reverse proxy handles network-level isolation

**Banner serving on tracking server:**
The banner serving handler (`internal/adapter/banner/serving_handler.go`) needs to be integrated into the tracking server:
```
GET  /banner/:placementId         — get banners for placement
GET  /track/banner/:bannerId      — track banner impression/click
```

**Tracking script generation:**
The snippet generator (`pkg/snippet/generator.go`) generates a self-contained JS snippet. It needs to be updated to:
- Use the tracking server's URL (configurable via `AUTOMATA_TRACKING_URL` env var)
- Include `X-Tenant-ID` header from subdomain or be passed as a parameter
- Track: pageviews, clicks, custom events

**Updated snippet format:**
```javascript
(function(w, d, s, u) {
  w['_automata'] = w['_automata'] || { queue: [] };
  var q = w['_automata'];
  function push(fn) { q.queue.push(fn); }
  
  // Auto pageview
  push(function() { track('pageview', { url: location.pathname }); });
  
  // Auto click tracking
  d.addEventListener('click', function(e) {
    var el = e.target.closest('a, button');
    if (el) push(function() { track('click', { tag: el.tagName, text: el.textContent?.trim()?.slice(0,50) }); });
  });
  
  // Custom API
  w._automata.track = function(type, data) { push(function() { track(type, data); }); };
  
  function track(type, data) {
    data = data || {};
    data.type = type;
    data.url = data.url || location.pathname;
    data.referrer = document.referrer;
    data.title = d.title;
    data.user_agent = navigator.userAgent;
    data.utm_source = getUTM('utm_source');
    data.utm_medium = getUTM('utm_medium');
    data.utm_campaign = getUTM('utm_campaign');
    
    // Batch send
    q.events = q.events || [];
    q.events.push(data);
    if (q.events.length >= 5) sendBatch();
    setTimeout(sendBatch, 2000);
  }
  
  function sendBatch() {
    if (!q.events || q.events.length === 0) return;
    var events = q.events;
    q.events = [];
    fetch(u + '/track/batch', {
      method: 'POST',
      headers: {'Content-Type': 'application/json', 'X-Tenant-ID': q.tenantId},
      body: JSON.stringify(events)
    });
  }
  
  function getUTM(name) {
    var match = location.search.match(new RegExp(name + '=([^&]*)'));
    return match ? decodeURIComponent(match[1]) : '';
  }
  
  // Flush on page unload
  addEventListener('beforeunload', sendBatch);
  
  // Start
  q.tenantId = 'TENANT_ID_FROM_EMBED';
  var script = d.createElement('script');
  script.src = u + '/snippet/' + q.tenantId + '.js';
  d.head.appendChild(script);
})(window, document, 'script', 'http://tracking.automata.local');
```

**Config for tracking server:**
```yaml
server:
  host: 0.0.0.0
  port: "8080"
tracking:
  host: 0.0.0.0
  port: "8081"
```

**Files affected:**
- `cmd/automata/main.go` — start both servers with graceful shutdown
- `internal/adapter/tracking/server.go` — add banner serving, form rendering, form submission routes
- `internal/adapter/tracking/track_handler.go` — already exists, keep as-is
- `internal/adapter/tracking/snippet_handler.go` — update to generate new snippet format
- `internal/adapter/banner/serving_handler.go` — integrate into tracking server
- `internal/adapter/proxy/proxy.go` — remove form/snippet handlers (moved to tracking server)
- `internal/infrastructure/config/config.go` — add `tracking.host` and `tracking.port` config keys
- `config.example.yaml` — add tracking section
- `Dockerfile` — expose port 8081
- `docker-compose.yml` — add tracking port mapping
- `pkg/snippet/generator.go` — update snippet generator

### 3.6 Frontend: Form Creation Fix

**Root cause:** Form routes are not registered in `server.go`, so frontend API calls to `/forms` return 404.

**Fix:** Register form routes (Section 3.4). No frontend code changes needed beyond ensuring the API client calls the right endpoints (which it already does).

### 3.7 Frontend: Login Page Changes

**Changes:**
- Remove tenant selector from login form
- After login, show tenant switcher in sidebar (user may belong to multiple tenants)
- All API requests include `X-Tenant-ID` header from selected tenant
- If no tenant is selected, show a "select a website" screen

**Files affected:**
- `webui/src/views/LoginView.vue` — remove tenant field
- `webui/src/stores/auth.ts` — store tenant list, add `selectedTenant` state
- `webui/src/api/client.ts` — add `X-Tenant-ID` header from store
- `webui/src/components/layout/Sidebar.vue` — add tenant switcher dropdown

### 3.8 Frontend: New Pages

**ConfigView.vue** — per-tenant configuration:
- CORS origins: list with add/remove
- Domain settings: primary domain + aliases
- Display settings: name, logo URL

**BannersView.vue** — banner management:
- Tabs: Banners, Placements, Campaigns
- Banner CRUD: name, type (html/image/video/popup/sticky), content, link URL, campaign, placements, priority, date range, AB testing
- Placement CRUD: name, location, CSS selector, max banners
- Campaign CRUD: name, description, date range, target URL, tracking code

---

## 4. Data Flow

### Login Flow (New)
```
1. User enters email + password (no tenant)
2. POST /api/auth/login { email, password }
3. Backend finds user across all tenants
4. Returns { token, user_id, email, tenants: [{id, slug, name}] }
5. Frontend stores token + tenant list
6. User selects a tenant from switcher
7. All subsequent requests include X-Tenant-ID header
```

### Form Submission Flow (Tracking Server)
```
1. Tenant embeds tracking script on their website
2. Script loads, tracks pageviews/clicks → POST /track/batch on tracking server
3. Tenant creates form in Admin Console (main server)
4. Form gets a public URL: /form/render/:slug (tracking server)
5. Visitor fills form → POST /form/submit/:slug (tracking server)
6. Tracking server resolves tenant from X-Tenant-ID header or subdomain
7. Form data stored in form_submissions table
```

### Banner Serving Flow (Tracking Server)
```
1. Tenant creates banner + placement in Admin Console (main server)
2. Tenant embeds banner script on their website
3. Script requests /banner/:placementId from tracking server
4. Tracking server resolves tenant from X-Tenant-ID header
5. Returns active banners for placement
6. Banner impressions/clicks tracked via /track/banner
```

### Admin Config Flow
```
1. Admin navigates to Config page
2. GET /api/admin/configs/:tenantId
3. Backend reads from admin_configs table
4. Admin edits CORS origins, domain, display settings
5. PUT /api/admin/configs/:tenantId/cors
6. Backend upserts into admin_configs table
7. CORS middleware reads config on each request (cached per tenant)
```

---

## 5. Error Handling

- **Tracking server:** No auth middleware. If tenant not found, return 400 with descriptive error.
- **Main server:** Standard JWT auth + tenant resolution. If tenant not found in header, return 401.
- **Admin config:** If config key doesn't exist, create on first write. GET returns empty object if no config.
- **Form submission:** If form not found, return 404. If form is disabled, return 403.
- **Banner serving:** If placement not found, return 404. If no active banners, return empty array.

---

## 6. Testing

### E2E Tests (Playwright)
- **Login without tenant:** Test login page has no tenant field, returns tenant list
- **Tenant switcher:** Test switching between tenants changes API requests
- **Form creation:** Test creating a form via API and frontend
- **Form submission:** Test submitting a form via tracking server endpoint
- **Banner CRUD:** Test creating/editing/deleting banners, placements, campaigns
- **Admin config:** Test updating CORS origins, domain, display settings
- **Tracking script:** Test that `/snippet/:id.js` returns valid JS
- **Tracking events:** Test POST /track and POST /track/batch
- **Banner pixel:** Test GET /track/banner returns 1x1 PNG

### Unit Tests
- Admin config repository CRUD
- CORS middleware
- Tracking server tenant resolution
- Form submission validation
- Banner serving logic

---

## 7. Files Summary

### New Files
| File | Purpose |
|------|---------|
| `internal/domain/config/config.go` | Config domain types and repository interface |
| `internal/infrastructure/config/repo/postgres.go` | Admin config PostgreSQL repository |
| `internal/adapter/api/middleware/cors.go` | CORS middleware reading from admin config |
| `webui/src/views/ConfigView.vue` | Admin config management page |
| `webui/src/views/BannersView.vue` | Banner/placement/campaign management page |

### Modified Files
| File | Change |
|------|--------|
| `internal/domain/tenant/user_repository.go` | Add `GetByEmailGlobal` method |
| `internal/infrastructure/tenant/repo/user_postgres.go` | Implement `GetByEmailGlobal` |
| `internal/domain/auth/auth.go` | Update `Login` signature |
| `internal/infrastructure/auth/service.go` | Update `Login` to find user across tenants |
| `internal/adapter/api/handler/auth_handler.go` | Remove tenant from login, return tenant list |
| `internal/adapter/api/middleware/auth.go` | Extract tenant from `X-Tenant-ID` header |
| `internal/adapter/api/tenant_middleware.go` | Prefer `X-Tenant-ID` header |
| `internal/adapter/api/server.go` | Register form, banner, config routes; start tracking server |
| `internal/adapter/proxy/proxy.go` | Remove form/snippet handlers (moved to tracking server) |
| `internal/adapter/tracking/server.go` | Add banner serving, form rendering/submission |
| `internal/adapter/tracking/snippet_handler.go` | Update snippet format |
| `internal/adapter/banner/serving_handler.go` | Integrate into tracking server |
| `cmd/automata/main.go` | Start both servers |
| `internal/infrastructure/config/config.go` | Add tracking config keys |
| `config.example.yaml` | Add tracking section |
| `internal/infrastructure/database/migration.sql` | Add admin_configs table |
| `Dockerfile` | Expose port 8081 |
| `docker-compose.yml` | Add tracking port |
| `pkg/snippet/generator.go` | Update snippet generator |
| `webui/src/api/client.ts` | Add `X-Tenant-ID` header |
| `webui/src/api/index.ts` | Add configsApi, bannersApi, placementsApi, campaignsApi |
| `webui/src/types/api.ts` | Add config and banner types |
| `webui/src/stores/auth.ts` | Store tenant list, add selectedTenant |
| `webui/src/views/LoginView.vue` | Remove tenant field |
| `webui/src/components/layout/Sidebar.vue` | Add tenant switcher + new nav items |
| `tests/e2e/app.spec.ts` | Update tests for new architecture |

---

## 8. Implementation Order

1. **Database migration** — Add `admin_configs` table, update `tenant_users` for global users
2. **Auth changes** — Global login, JWT changes, tenant header resolution
3. **Admin config subsystem** — Repository, API, CORS middleware
4. **Form routes** — Register in main server
5. **Banner routes** — Register in main server
6. **Tracking server** — Integrate banner serving, form rendering/submission
7. **Dual server startup** — `main.go` changes
8. **Frontend** — Login page, tenant switcher, new pages (Config, Banners)
9. **Tracking script** — Update snippet generator
10. **Tests** — E2E and unit tests
