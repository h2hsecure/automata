# Automata Security & Integrity Fixes — Design Spec

**Date:** 2026-11-04
**Status:** Approved design (cookie-only auth, full finding scope)
**Source:** Full codebase review (2026-11-04) — 17 Major + 18 Medium findings, all verified at file:line.

## 1. Context

Automata is a multi-tenant marketing automation platform (Go, hexagonal, PostgreSQL; gin API + BFF reverse proxy + Vue SPA). The 2026-11-04 review confirmed, in order of blast radius:

1. Any authenticated user can mint a platform admin (`user_handler.go:83` accepts `is_admin` from body; `/admin` group has no admin check).
2. `X-Context-ID` resolves to any tenant with only an existence check — no membership validation (`api/context_middleware.go:38-50`); API-key handlers trust the header directly.
3. Cross-tenant inbox mutation (`MarkInboxRead` ignores route context).
4. Anonymous reflected XSS on `/api/snippet/*path` (`webui/handler/proxy.go:65`).
5. Plaintext webhook signing secret in DB; tenant-configurable SSRF via unanchored URL regex.
6. No request-size limits anywhere (proxy, import, static-file, upload, track/batch, MCP CSV).
7. BFF auth injection is dead code; the SPA's long-lived JWT lives in `localStorage`.
8. Race/atomicity defects: duplicate hidden contacts, non-atomic visitor merge, un-transactional tag replace, duplicate queue deliveries.

Plus 18 Medium findings (rate limiting, PII in logs, refresh rotation, cookie `Secure`, correctness bugs, webui stale-response race, swallowed delete errors).

## 2. Goals / Non-goals

**Goals**
- Close all 17 Major + 18 Medium findings from the 2026-11-04 review.
- Make the BFF the single, real auth boundary for the WebUI (cookie-only).
- Enforce tenant isolation server-side with one shared membership check.
- Every Major finding gets a regression test.

**Non-goals**
- No changes to MCP/API-key direct API access (they keep Bearer JWT validation).
- No re-architecture of queues, routers, or the SPA framework.
- No new dependencies beyond a stdlib-compatible rate limiter (token bucket) if one is needed.

## 3. Phase 1 — Security core

### 3.1 Admin-only middleware (`/admin` group)

- New `middleware.RequireAdmin` in `internal/adapter/api/middleware/`:
  - After `AuthMiddleware`, loads the user **from the DB** (not just the JWT `is_admin` claim) and checks `IsAdmin`. 403 otherwise.
- Wired on the `/admin` group in `server.go` (all `admin.Use(...)` routes) and on the scattered `middleware.AuthMiddleware(authService)` admin-style routes (api-keys list/revoke, inbox routes).
- `UserHandler.Create`/`Update`: remove `is_admin` from request structs; admin flag settable only via a dedicated admin-only action.
- Regression tests: non-admin gets 403 on `/admin/*`; admin passes; body-supplied `is_admin` is ignored.

### 3.2 Context membership middleware (tenant isolation)

- New `middleware.RequireContextMembership` wired immediately after `ContextResolver` in `server.go`:
  - Resolved user (from `AuthMiddleware`) + resolved context (from `ContextResolver`).
  - Platform admin (DB-verified) → pass.
  - Else membership lookup `userRepo` for `(user_id, context_id)` → pass or 403.
- Admin routes skip `ContextResolver` and take explicit context IDs: a shared helper `requireContextAccess(user, contextID)` (same package) is used by:
  - `auth_handler.go` API-key create/list/revoke (replaces raw `X-Context-ID` trust),
  - `email_handler.go` inbox list/mark-read/send routes,
  - `config_handler.go`, `contextSettingsHandler`, `emailQueueHandler`, tracking admin routes that accept `:contextId`/`:id` context params.
- `MarkInboxRead`: SQL gains `AND context_id = $2` (defense in depth).
- Regression tests: user of context A gets 403 on context B's contacts/forms/pages/tracking/api-keys/inbox; owner/admin/member pass; platform admin passes.

### 3.3 WebUI admin guard

- `webui/src/router/index.ts` `beforeEach`: after auth check, `if (to.meta.requiresAdmin && !authStore.isAdmin) → redirect Dashboard`. `isAdmin` comes from the hydrated profile (see 3.4).

### 3.4 Cookie-only auth (findings #7 + #17)

**Principle:** the API keeps validating `Bearer` JWTs; only the WebUI channel changes. The BFF becomes the token custodian.

- **Session:** extend `webui_cookie.SessionData` with `Token` (JWT) and `RefreshToken`. Cookie manager (AES-GCM) unchanged.
- **Login:** SPA keeps calling `POST /api/auth/login` through the proxy. `ProxyHandler` intercepts a successful (200) response on `/api/auth/login` and `/api/auth/refresh`: parse the JSON body, extract `token`/`refresh_token`, write them into a new encrypted session cookie (`HttpOnly; Secure; SameSite=Lax; MaxAge=24h`), add `Set-Cookie` to the response.
- **Request auth:** for `/api/*` (excluding `/api/auth/login`, `/api/auth/refresh`, `/api/health`): decrypt session; if valid → strip any client `Authorization` header, inject `Authorization: Bearer <session.Token>` on the outbound request. No/invalid session → forward without Authorization.
- **SPA:**
  - `auth.ts`: stop reading/storing `response.token`; after login, hydrate via `GET /api/auth/me` (profile + contexts, non-secret) stored in the Pinia store.
  - `client.ts`: axios interceptor no longer sets `Authorization`; on 401 → call `/api/auth/refresh` once (cookie is updated server-side via interception) → retry original request; on refresh failure → clear profile, redirect login.
  - Remove all `localStorage` token reads/writes.
- **Cleanup:** delete vestigial `HandleLoginPost` / `RequireAuth` form-login path in `webui/handler/auth.go` (dual login paths); keep `HandleLogout` (clears cookie).
- **Regression tests:** login sets cookie; request with client `Authorization` header is stripped; no cookie → no Authorization forwarded; logout clears; SPA has zero `localStorage` token references (grep-asserted in a test).

### 3.5 Anonymous XSS — `/api/snippet/*path`

- `webui/handler/proxy.go:65`: validate the path segment as UUID before interpolating (404 otherwise); escape defensively. The route is anonymous (ContextResolver falls through), so validation is mandatory, not just escaping.

### 3.6 Webhook SSRF + plaintext secret

- `domain/webhook/webhook_endpoint.go:75`: replace regex with anchored host check — `^https://` (any) OR `^http://(localhost|127\.0\.0\.1)(:\d+)?/`. Reject `http://localhost.evil.com` etc.
- `infrastructure/webhook/service.go` + `repo/postgres.go`: stop persisting `signing_secret` plaintext. Migration: for existing rows, keep only `secret_hash`-based verification (derive HMAC key from the hash is not possible — so: re-derive the signing key deterministically from a per-endpoint random value stored **encrypted** at rest using the existing cookie-manager AES-GCM key material, or force secret rotation). **Decision:** encrypted column `signing_secret_enc` (AES-GCM with a dedicated key from config); plaintext column dropped. Verification path unchanged.
- Migration in the existing advisory-locked framework: add encrypted column → backfill (encrypt existing plaintext values) → drop plaintext column.

## 4. Phase 2 — DoS + hardening

### 4.1 Body-size limits

- New config value `max_body_bytes` (default 10 MB, configurable).
- `MaxBytesReader` applied: BFF proxy (all `/api/*`), `contact_handler.go` import, `staticfile_handler.go` upload (also enforce `header.Size` vs cap before reading), `upload_handler.go` (close the missing-field bypass: unknown field → reject, not skip), `track/batch` (≤100 events/request), MCP `contacts_import` (csv_data ≤10 MB).
- CSV importer: `FieldLineLimit` = 1 MB, row cap 100k.

### 4.2 Rate limiting

- Token-bucket middleware (stdlib implementation, no new dep):
  - `POST /api/auth/login` + `/auth/refresh`: 10/min per IP+email.
  - Public tracking routes (`/track/*`, `/page/*`): 300/min per IP.
- Configurable per-route limits; 429 with `Retry-After`.

### 4.3 Unbounded tracking events query

- `tracking_handler.go` `GetEvents`: default `limit=50`, max `100`; repo always applies `LIMIT`.

### 4.4 CSV formula injection

- `pkg/export/csv_exporter.go`: neutralize cells starting with `=`, `+`, `-`, `@`, tab, CR by prefixing a single quote. Single choke point (export); import stores raw by design.

### 4.5 Auth hygiene

- `webui/handler/auth.go` session cookie: add `Secure: true` (config flag `insecure_cookie` for local HTTP dev).
- `auth_handler.go:51`: log a hash of the email, not the email, on login attempts.
- `cmd/bff/main.go`: graceful shutdown (`http.Server` + `signal.Notify` + `Shutdown`, mirroring `cmd/automata`); `RunMigrations` errors → `log.Fatal`.

### 4.6 Refresh token rotation (moved from Phase 4 for coherence with 3.4)

- New `refresh_tokens` table (`jti PK, user_id, expires_at, revoked_at`, advisory-locked migration).
- `RefreshToken`: parse → check jti exists and not revoked → revoke old jti, issue new; jti reuse → reject + warn log (theft signal).

## 5. Phase 3 — Data integrity

### 5.1 Duplicate hidden contacts

- Migration: dedupe `(context_id, source, source_id)` for `source='visitor'` (keep oldest, reassign activities to it), then `CREATE UNIQUE INDEX ... WHERE source='visitor'`.
- `track_event.go`: on unique-violation, re-read and continue with the existing contact.

### 5.2 Non-atomic visitor merge

- `identify_visitor.go`: wrap `contactRepo.Merge` + `trackingRepo.ReassignVisitorContactID` (and the legacy create+link path) in a single transaction. Requires the repos to expose tx-aware variants (pass `*pgx.Tx` or a tx-scoped interface).

### 5.3 Tag replace transaction

- `usecase/contact/tags.go` `ApplyTags`: DELETE + INSERTs inside one transaction.

### 5.4 Queue duplicate delivery

- Both `email_worker.go` and `webhook_worker.go`: atomic claim — `UPDATE ... SET status='processing' WHERE id=$1 AND status='pending'` (returning the row); only the worker that won the claim delivers. Crash recovery: a reaper flips stale `processing` rows back to `pending` after a timeout.

### 5.5 Correctness mediums

| Fix | Location |
|-----|----------|
| Log (not discard) activity-creation errors | `create_contact.go:87`, `merge_contacts.go:60` |
| Import duplicates counted `Skipped`, not `Updated` | `import_contacts.go:69` |
| Tags clearable: `applyTags` when explicitly provided (nil vs empty) | `update_contact.go:142` |
| Normalize `""` email → `nil`; on 23505 re-read + return existing | `create_contact.go:39`, `update_contact.go:47` |
| `TestSend` validates `req.To` | `usecase/email/email.go:146` |
| Dashboard: default each zero bound independently | `get_dashboard.go:33` |
| Export: paginate to exhaustion (or explicit truncation flag in response) | `export_contacts.go:20` |
| `Form.Validate()`: reject duplicate field slugs | `domain/form/form.go:32` |
| `isValidEmail`: exactly one `@` | `domain/contact/contact.go:58` |
| `Decision.Validate()`: `Action` restricted to documented button actions | `domain/page/page.go` + `page_public_handler.go:119` |
| SMTP: sanitize `from` + each `to` (strip CR/LF) as `subject` is | `email/smtp/smtp.go:196-202` |
| `email_queue.go`: bind `pgtype.Array[string]` instead of hand-built literal | `queue/email_queue.go:93` |
| Admin bootstrap password: print to secret file / obfuscated handle, not stdout | `bootstrap/admin.go:76` |
| `staticfile` `List`: add limit/offset | `staticfile/repo/postgres.go:72` |
| Raw `err.Error()` → generic client message, log real error (context, contact, tracking, user handlers) | adapter handlers (see review list) |

## 6. Phase 4 — WebUI mediums

- `DashboardView.vue`: generation counter (reuse `WebsiteEmailPanel.vue:59-78` pattern) for `loadOverview`/`loadMapData`; discard stale writes.
- `UsersPanel.vue` / `WebsitesPanel.vue`: surface delete errors (`e.response?.data?.error`) in the existing error banner/toast.

## 7. Testing strategy

- TDD per fix (red → green).
- Integration tests for `RequireAdmin` and `RequireContextMembership` following the existing `auth_handler_test.go` pattern (real gin engine, stub repos).
- Regression test per Major finding (≥17): privilege escalation, cross-tenant contacts/forms/pages/tracking/api-keys/inbox, proxy header strip + client header removal, snippet XSS 404, SSRF regex unit, secret-drop migration (pre/post state), body limits (413), batch cap (413/422), unique-index dedupe migration, tag transaction (simulate mid-tx failure), queue claim concurrency (2 workers, 1 job).
- Migration tests extend existing `migration_test.go` style.
- SPA: vitest unit for router guard (`requiresAdmin`); grep-assert no `localStorage.getItem('token')`.
- `go build ./... && go test ./...` green per phase.

## 8. Execution order & phasing

1. **Phase 1** (3.1 → 3.2 → 3.4 → 3.3 → 3.5 → 3.6): admin gate first (smallest blast radius), membership second, cookie-only third (largest change, needs both middlewares' tests as the baseline), then point fixes.
2. **Phase 2** (4.1 → 4.6 → 4.2 → 4.3 → 4.4 → 4.5).
3. **Phase 3** (5.1 → 5.4 → 5.2 → 5.3 → 5.5).
4. **Phase 4** (6).

Each phase ends with a green test run; commits grouped per phase with the finding IDs referenced.

## 9. Open items (decided)

- Secret storage: AES-GCM encrypted column (not secrets-manager) — matches existing cookie-manager key handling, deployable without new infra.
- Rate limiter: stdlib token bucket, in-process (per-pod). Multi-pod precision is acceptable for this threat (brute-force throttling), documented as a limitation.
- Export truncation: paginate to exhaustion (simple, correct), no flag.