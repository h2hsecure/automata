# Tenant Removal — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the dead "Tenants" page from the Web UI and align all tenancy naming to the website/context model ("system runs on different websites, not tenants").

**Architecture:** The tenancy unit is the domain `Context` (UI label: "Websites", managed in `ContextsView.vue`). The Tenants page is 100% dead code (client stubs return empty data; no backend tenant endpoints exist). Phase 1 deletes the UI surface. Phase 2 renames the one real route (`/api/tenant/:slug/email` → `/api/context/:slug/email`), the webhook delivery header (`X-Tenant-ID` → `X-Context-ID`), and all cosmetic `tenant` identifiers in comments, local variables, and test names. Routing conflict with `/context/contacts` and `/context/forms` was empirically verified safe on gin v1.12.0 (static segments win over wildcards).

**Context:**
- Branch: `phase2-auth-changes`. Working tree contains uncommitted user WIP (`AGENTS.md`, `helm/automata/values.yaml`, `docs/phase1.md`, 7 cicd test files) — **never stage or modify these**.
- `web/` build assets are committed; commit rebuilt `web/` alongside webui source changes.
- `migration_contexts.sql` is a historical migration artifact — do NOT touch.
- `cicd/contact_http_test.go:88` (`INSERT INTO tenants`) references a legacy table that no longer exists (silent no-op, error discarded) — out of scope, leave untouched.
- `cicd/contact_form_integration_test.go` (user WIP) uses `X-Tenant-ID` in a self-contained mock — do NOT touch.
- Baseline cicd: **320 passed | 10 failed of 330** (all 10 pre-existing in `banner_campaign_http`).
- Pre-existing `go vet` failure in `internal/adapter/proxy/proxy_test.go` — not a regression target.

---

## Task 1: Phase 1 — Web UI Tenants cleanup

Remove the Tenants page, its route, nav item, API stubs, and types.

**Files:**
- Delete: `webui/src/views/TenantsView.vue`
- `webui/src/router/index.ts` — remove the tenants route block (lines 27–32)
- `webui/src/components/AppSidebar.vue` — remove nav item (line 41) and `Settings` import (line 10)
- `webui/src/api/client.ts` — remove `Tenant` import (line 9), drop `tenant` param from `login` (lines 65–66), remove tenant stub section (lines 244–257)
- `webui/src/types/api.ts` — remove `Tenant` interface (lines 134–141), remove `tenant_id` from `User` (line 210)
- `webui/src/stores/auth.ts` — drop `tenant` param from `login` (line 32) and `loginApi` call (line 36), remove `tenant_id` assignment (line 45)

**Steps:**

- [ ] Delete `webui/src/views/TenantsView.vue` (`rm`).
- [ ] `webui/src/router/index.ts`: remove this block:
  ```ts
        {
          path: 'tenants',
          name: 'Tenants',
          component: () => import('@/views/TenantsView.vue'),
          meta: { requiresAdmin: true },
        },
  ```
- [ ] `webui/src/components/AppSidebar.vue`: remove line `{ name: 'Tenants', path: '/tenants', icon: Settings, adminOnly: true },` and the `Settings,` line from the lucide import. `Settings` is used nowhere else in the file (verified).
- [ ] `webui/src/api/client.ts`:
  - Remove `Tenant,` from the type import (line 9). Keep `PaginatedResponse` (still used by `listContacts`/`getTrackingEvents`).
  - Change `export async function login(email: string, password: string, tenant?: string)` → `export async function login(email: string, password: string)` and body `{ email, password, tenant }` → `{ email, password }`. `LoginView.vue` calls `login(email, password)` — no caller passes a tenant.
  - Remove the whole section (lines 244–257): `// Tenant endpoints (admin only)` comment + `listTenants`, `createTenant`, `updateTenant`, `deleteTenant` stubs.
- [ ] `webui/src/types/api.ts`: remove the `Tenant` interface (lines 134–141) and the `tenant_id: string;` field from `User` (line 210). No view reads `user.tenant_id` (verified by grep).
- [ ] `webui/src/stores/auth.ts`: `login(email, password, tenant?)` → `login(email, password)`; `loginApi(email, password, tenant)` → `loginApi(email, password)`; remove `tenant_id: response.tenant_id || '',` from the user object.
- [ ] Rebuild: `cd webui && npx vite build` (outDir is `../web`).
- [ ] Verify: `grep -rni tenant webui/src/` returns zero matches.

**Expected:** `vite build` succeeds (the project does not pass a clean `vue-tsc --noEmit` — `vite build` is the gate). `web/` output changes (JS bundle hashes). No other webui references to tenant remain.

---

## Task 2: Phase 2 — Backend route, header, and cosmetic renames

**Files:**
- `internal/adapter/api/server.go` — route + comment
- `internal/adapter/api/handler/email_handler.go` — comment
- `internal/infrastructure/queue/webhook_worker.go` — delivery header
- `internal/domain/config/config.go` — 4 comments
- `internal/domain/email/email.go` — comment
- `internal/domain/context/context.go` — comment
- `internal/adapter/api/handler/auth_handler.go` — local var
- `internal/adapter/proxy/proxy_test.go` — mock type name
- `internal/adapter/api/context_middleware_test.go` — capture type name
- `internal/domain/contact/contact_test.go` — test case name
- `internal/infrastructure/context/repo/postgres.go` — local var
- `internal/infrastructure/context/repo/postgres_test.go` — local var

**Steps:**

- [ ] `internal/adapter/api/server.go`:
  - Line 112 comment: `// Email template routes (declared outside admin block for tenant route access)` → `// Email template routes (declared outside admin block for context route access)`
  - Lines 139–142:
    ```go
    tenantEmailGroup := api.Group("/tenant/:slug/email")
    {
        tenantEmailGroup.POST("/send", emailHandler.Send)
    }
    ```
    →
    ```go
    contextEmailGroup := api.Group("/context/:slug/email")
    {
        contextEmailGroup.POST("/send", emailHandler.Send)
    }
    ```
- [ ] `internal/adapter/api/handler/email_handler.go:198`: `// Send handles POST /api/tenant/:slug/email/send — tenant-scoped send.` → `// Send handles POST /api/context/:slug/email/send — context-scoped send.`
- [ ] `internal/infrastructure/queue/webhook_worker.go:99`: `req.Header.Set("X-Tenant-ID", d.ContextID)` → `req.Header.Set("X-Context-ID", d.ContextID)`
- [ ] `internal/domain/config/config.go`: comments on lines 5, 14, 19, 25 — "for a tenant" → "for a context" (4 occurrences).
- [ ] `internal/domain/email/email.go:40`: `// SendEmailRequest is the tenant-scoped send-email request.` → `// SendEmailRequest is the context-scoped send-email request.`
- [ ] `internal/domain/context/context.go:17`: `// Context represents a tenant/website context in the platform.` → `// Context represents a website context in the platform.`
- [ ] `internal/adapter/api/handler/auth_handler.go`: `tenantInfos` → `contextInfos` (lines 51, 93, 94).
- [ ] `internal/adapter/proxy/proxy_test.go`: `mockTenantRepo` → `mockContextRepo` (all occurrences, incl. comment on line 110). No conflict: `mockContextRepo` in `internal/adapter/api` is a different package.
- [ ] `internal/adapter/api/context_middleware_test.go`: `tenantCapture` → `contextCapture` (lines 117–126, incl. comment).
- [ ] `internal/domain/contact/contact_test.go:30`: test case name `"missing tenant_id"` → `"missing context_id"` (the `Contact` field is `ContextID`).
- [ ] `internal/infrastructure/context/repo/postgres.go`: local var `tenants` → `contexts` (lines 119, 133, 136). SQL already uses `contexts`.
- [ ] `internal/infrastructure/context/repo/postgres_test.go`: local var `tenants` → `contexts` (lines 136–154).
- [ ] Verify: `go build ./...` succeeds.
- [ ] Verify: `go vet ./...` — only the pre-existing `proxy_test.go` failure; no new failures.
- [ ] Verify: `grep -rni tenant internal/ --include="*.go"` returns zero matches (excluding `migration_contexts.sql`, which is `.sql`).

**Expected:** Build green. The renamed route coexists with `/context/contacts` and `/context/forms` (verified safe on gin v1.12.0 — static wins over wildcard).

---

## Task 3: Phase 2 — cicd test updates

**Files:**
- `cicd/email_template_test.go`
- `cicd/webhook_worker_test.go`
- `cicd/config_repo_test.go`
- `cicd/support/fixtures.go`

**Steps:**

- [ ] `cicd/email_template_test.go`:
  - Line 788: `Describe("Tenant Send Email", ...)` → `Describe("Context Send Email", ...)`
  - Line 818: `It("enqueues email for tenant send", ...)` → `It("enqueues email for context send", ...)`
  - Lines 823–826: `Key: "tenant_test"` → `"context_test"`; `Subject: "Tenant: {{ .Name }}"` → `"Context: {{ .Name }}"`; `BodyText: "Tenant body for {{ .Name }}"` → `"Context body for {{ .Name }}"`; `BodyHTML: "<p>Tenant HTML</p>"` → `"<p>Context HTML</p>"`
  - Lines 831, 855, 884, 900: `/api/tenant/test-slug/email/send` → `/api/context/test-slug/email/send` (4 occurrences)
  - Lines 837–839: `"template": "tenant_test"` → `"context_test"`; `"tenant@example.com"` → `"context@example.com"`; `"Name": "Tenant User"` → `"Name": "Context User"`
- [ ] `cicd/webhook_worker_test.go:70`: `Expect(req.Headers["X-Tenant-Id"])` → `Expect(req.Headers["X-Context-Id"])` (Go canonicalizes `X-Context-ID` to `X-Context-Id` in the header map).
- [ ] `cicd/config_repo_test.go` (lines 201–215): `"Multiple tenants isolation"` → `"Multiple contexts isolation"`; `"should allow same config key for different tenants"` → `"...different contexts"`; `tenant1`/`tenant2` vars → `context1`/`context2`; `tenant1.com`/`tenant2.com` → `context1.com`/`context2.com`.
- [ ] `cicd/support/fixtures.go:263–264`: `NewTestTenantUser` → `NewTestContextUser` (unused elsewhere; comment already says "test context user").
- [ ] Verify: `go build ./...` and `go vet ./cicd/` clean (no new failures).
- [ ] Verify: `grep -rni tenant cicd/ --include="*.go"` returns matches ONLY in user-WIP files (`contact_form_integration_test.go`, `config_handler_test.go`, `global_login_test.go`, etc.) — none in committed files this task touched.

**Expected:** Focused cicd specs (Context Send Email, webhook worker, config repo) pass against the renamed route/header.

---

## Task 4: Full verification + commit

**Steps:**

- [ ] `go build ./...` — green.
- [ ] `go vet ./...` — only pre-existing `proxy_test.go` failure.
- [ ] `cd webui && npx vite build` — green (also refreshes `web/` if any Task 1 changes were missed).
- [ ] Focused cicd: run the email-template, webhook-worker, and config-repo specs (Ginkgo focus). All pass.
- [ ] Full cicd: `make test` (or `go test ./cicd/ -run TestCICD`). Result must match baseline: **320 passed | 10 failed of 330**, failures confined to `banner_campaign_http`.
- [ ] `git status` — confirm ONLY the intended files are modified (no user-WIP files staged).
- [ ] Stage explicitly (file list only — never `git add -A`):
  - `webui/src/views/TenantsView.vue` (deleted)
  - `webui/src/router/index.ts`, `webui/src/components/AppSidebar.vue`, `webui/src/api/client.ts`, `webui/src/types/api.ts`, `webui/src/stores/auth.ts`
  - `web/` (rebuilt assets)
  - `internal/adapter/api/server.go`, `internal/adapter/api/handler/email_handler.go`, `internal/adapter/api/handler/auth_handler.go`, `internal/adapter/api/context_middleware_test.go`
  - `internal/infrastructure/queue/webhook_worker.go`
  - `internal/domain/config/config.go`, `internal/domain/email/email.go`, `internal/domain/context/context.go`, `internal/domain/contact/contact_test.go`
  - `internal/adapter/proxy/proxy_test.go`
  - `internal/infrastructure/context/repo/postgres.go`, `internal/infrastructure/context/repo/postgres_test.go`
  - `cicd/email_template_test.go`, `cicd/webhook_worker_test.go`, `cicd/config_repo_test.go`, `cicd/support/fixtures.go`
- [ ] Commit: `Remove Tenants feature; align tenancy naming to website/context`

**Expected:** One commit on `phase2-auth-changes`. User WIP files remain modified-unstaged. `git status` after commit shows only the pre-existing WIP entries.

---

## Task 5: Deploy (conditional — only with user go-ahead)

**Steps:**

- [ ] User confirms deploy.
- [ ] Reset `/tmp/opencode/automata-deploy` to the new HEAD (fresh clone/reset of `phase2-auth-changes`).
- [ ] `CGO_ENABLED=0 go build -o automata ./cmd/automata` and `CGO_ENABLED=0 go build -o bff ./cmd/bff`; verify both with `file` (statically linked).
- [ ] Copy committed `web/` assets into the image context.
- [ ] `docker build -f /tmp/opencode/Dockerfile.deploy` — tag `latest` + commit sha; push to `ghcr.io/hekemen/automata`.
- [ ] `kubectl rollout restart` BOTH deployments (`automata-automata`, `automata-bff`) in ns `automata`.
- [ ] Verify pweb EndpointSlice still points at `automata-automata` ClusterIP (repatch `endpointslice automata-tracking` in ns `pdaccess-web` if drifted).
- [ ] Smoke: `curl http://automata.home.arpa/` (BFF 200) and `curl -X POST http://pweb.home.arpa/form/definition/8c61ddc4-debd-445e-b90b-3c62f89dfcfc/test` (200 JSON).

**Expected:** Both deployments roll out to the new image; public endpoints respond.

---

## Execution Status

(To be filled in as tasks complete.)