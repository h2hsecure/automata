# Automata Security & Integrity Fixes — Implementation Plan

**Spec:** `docs/superpowers/specs/2026-11-04-security-fixes-design.md` (approved 2026-11-04)
**Scope:** All 17 Major + 18 Medium findings. Go backend (`internal/`, `pkg/`, `cmd/`) + webui frontend.
**Prereqs:** none — plan starts from `main`.

## Global constraints (apply to every task)

- **TDD.** For each task: write the failing test first (red), then implement (green). Commit per task (or per sub-task for large ones) with message format `fix(security): <what> [<finding refs>]`.
- **Every Major finding gets a named regression test** (see the "Regression test" line in each task).
- **Phase gate:** at the end of each phase run `go build ./... && go vet ./... && go test ./...` (and `cd webui && npm run test` for phases touching the SPA). All green before the next phase.
- **Hexagonal boundaries:** domain packages never import `pgx`; infra implements domain interfaces; adapters wire.
- **Migrations:** idempotent SQL (`IF NOT EXISTS` / `IF EXISTS`), run under the existing advisory-lock framework. Pattern to copy: `internal/infrastructure/database/migration_users_runner.go` + its embedded SQL files, invoked from `cmd/automata/main.go` alongside the other `Run*Migrations` calls.
- **Config:** `internal/infrastructure/config` is a string map — `config.Get("key")`. Add a `GetInt(key string, def int) int` helper in `config.go` (parses, falls back to default; `Reset()` test support already exists).
- **Logging:** zerolog (`log.Info()/Warn()/Error()`); never log secrets or raw email addresses (use a short sha256 prefix: `fmt.Sprintf("%x", sha256.Sum256([]byte(email)))[:12]`).
- **API-key / MCP direct API access is unchanged** — they keep Bearer JWT/API-key validation against the API server.
- Test infra notes: Go integration tests use real PostgreSQL (existing `*_test.go` pattern, e.g. `internal/infrastructure/queue/webhook_queue_test.go`, `internal/infrastructure/database/migration_test.go`); frontend tests use vitest (`webui/package.json` → `npm run test`).

---

## Phase 1 — Security core

### Task 1: `RequireAdmin` middleware on `/admin` (spec 3.1, Major #1)

**Files**
- Create `internal/adapter/api/middleware/admin.go`
- Edit `internal/adapter/api/server.go`
- Edit `internal/adapter/api/handler/user_handler.go`

**Changes**
1. `middleware/admin.go`:
   ```go
   func RequireAdmin(userRepo context.UserRepository) gin.HandlerFunc {
       return func(c *gin.Context) {
           userID := c.GetString("user_id")
           user, err := userRepo.GetByID(userID)
           if err != nil || user == nil || !user.IsAdmin {
               c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin required"})
               return
           }
           c.Set("user", user) // DB-verified user for downstream
           c.Next()
       }
   }
   ```
   Must run **after** `AuthMiddleware` (which sets `user_id`). Admin status is read from the DB row, never from the JWT claim.
2. `server.go`:
   - `admin` group (line 85-86): `admin.Use(middleware.AuthMiddleware(authService), middleware.RequireAdmin(userRepo))`.
   - Scattered admin-style routes registered *outside* the group (they currently have only `AuthMiddleware`): add `RequireAdmin` to:
     - `admin.Group("/email-templates")...` (lines 168-173) — each group gets `.Use(middleware.AuthMiddleware(authService), middleware.RequireAdmin(userRepo))`;
     - `admin.POST("/contexts/:id/email-templates/:tid/send", ...)` (174), `admin.GET("/contexts/:id/inbox", ...)` (175), `admin.POST("/contexts/:id/inbox/:mid/read", ...)` (176);
     - `api.GET("/admin/api-keys", ...)` and `api.DELETE("/admin/api-keys/:id", ...)` (265-266).
3. `user_handler.go`: remove `IsAdmin` from `CreateRequest` (line ~25) and `UpdateRequest` (line ~31). Keep it in response payloads. The DB value can no longer be set through these endpoints (bootstrap sets the first admin out-of-band via `internal/infrastructure/bootstrap/admin.go`).

**Regression tests** (new `internal/adapter/api/middleware/admin_test.go`, pattern: real gin engine + stub `UserRepository`, mirroring `auth_handler_test.go` style)
- `TestRequireAdmin_NonAdminGets403` — stub user with `IsAdmin=false` → 403 on a protected route.
- `TestRequireAdmin_AdminPasses` — `IsAdmin=true` → 200.
- `TestUserHandler_CreateIgnoresIsAdminFromBody` — POST `/admin/users` with `"is_admin": true` → created user has `is_admin=false` in response.

**Commit:** `fix(security): DB-verified admin gate on /admin group; strip is_admin from user bodies [M1]`

---

### Task 2: Context membership middleware (spec 3.2, Major #2 + #3)

**Files**
- Create `internal/adapter/api/middleware/membership.go`
- Create `internal/adapter/api/middleware/context_access.go`
- Edit `internal/adapter/api/server.go` (line 207)
- Edit `internal/adapter/api/middleware/auth.go` (API-key branch)
- Edit `internal/infrastructure/email/repo/postgres_inbox.go` + its interface/usecase chain (`MarkInboxRead`)
- Edit `internal/adapter/api/handler/auth_handler.go` (`resolveAPIKeyContext`, line 425)

**Changes**
1. `middleware/auth.go` — in the API-key success branch (line ~76) add `c.Set("auth_via_api_key", true)`.
2. `middleware/membership.go`:
   ```go
   func RequireContextMembership(userRepo context.UserRepository) gin.HandlerFunc {
       return func(c *gin.Context) {
           if _, ok := c.Get("auth_via_api_key"); ok { c.Next(); return } // API keys are scoped to their context by AuthMiddleware
           userID := c.GetString("user_id")
           contextID := c.GetString("context_id")
           if userID == "" || contextID == "" { c.Next(); return } // anonymous/public routes
           user, err := userRepo.GetByID(userID)
           if err == nil && user != nil && user.IsAdmin { c.Next(); return } // platform admin
           mc, err := userRepo.GetByUserIDContext(c.Request.Context(), userID, contextID)
           if err != nil || mc == nil {
               c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "no access to this context"})
               return
           }
           c.Next()
       }
   }
   ```
3. `server.go` line 207: `api.Use(ContextResolver(contextRepo), middleware.RequireContextMembership(userRepo))`. (Gin mid-group `Use` only affects routes registered after it — admin/MCP routes above line 207 are untouched, which is what we want.)
4. `middleware/context_access.go` — shared helper for routes that take an explicit context ID (admin routes skip `ContextResolver`):
   ```go
   // RequireContextAccess verifies the caller (platform admin, DB-verified, OR
   // member of the target context) may access the context identified by the
   // "contextId"/"id" path param, falling back to the X-Context-ID header.
   func RequireContextAccess(userRepo context.UserRepository) gin.HandlerFunc { ... }
   ```
   Implementation: resolve `contextID` from `c.Param("contextId")` → `c.Param("id")` → `c.GetHeader("X-Context-ID")`; if empty → `c.Next()`; then platform-admin check (`userRepo.GetByID` → `IsAdmin`) else membership via `userRepo.GetByUserIDContext`; 403 on failure.
   Apply in `server.go` to: `admin/configs/:contextId/*` (122-124), `admin/contexts/:id/settings*` (137-139), `admin/contexts/:id/email-queue*` (143-146), `admin/contexts/:id/users*` (110-115), `admin/contexts/:id/email-templates/:tid/send` (174), `admin/contexts/:id/inbox*` (175-176), and the API-key routes (265-266).
5. `auth_handler.go` `resolveAPIKeyContext` (line 425): it currently trusts `X-Context-ID` raw. Keep the header read, but the routes are now behind `RequireContextAccess`, so the header value is validated before the handler runs. (No handler logic change needed — verify with a test.)
6. `MarkInboxRead`: add `contextID` parameter to the repo method; SQL becomes `... WHERE id = $1 AND context_id = $2`. Thread through: repo interface (`internal/infrastructure/email/repo/`), `email_handler.go MarkInboxRead` (passes `c.Param("id")` from route `/admin/contexts/:id/inbox/:mid/read`), and any usecase in between.

**Regression tests**
- `internal/adapter/api/middleware/membership_test.go` (stub repos, real gin):
  - `TestMembership_UserOfContextAGets403OnContextB` — user in context A, `X-Context-ID` = B → 403 on `/context/contacts`.
  - `TestMembership_OwnerAdminMemberPass` — all three roles → 200.
  - `TestMembership_PlatformAdminPasses` → 200.
  - `TestMembership_APIKeyRequestsSkip` — `auth_via_api_key` set → 200 without membership row.
- `internal/infrastructure/email/repo/postgres_inbox_test.go` (real PG): `TestMarkInboxRead_CrossTenantIgnored` — mark a message read with a *different* context ID → `RowsAffected()==0`, message stays unread.

**Commit:** `fix(security): enforce context membership server-side; scope MarkInboxRead to context [M2,M3]`

---

### Task 3: Cookie-only BFF auth (spec 3.4, Major #7 + #17)

**Files**
- Edit `internal/infrastructure/webui/cookie/manager.go` (`SessionData`)
- Rewrite `internal/adapter/webui/handler/proxy.go` (`HandleProxy`)
- Edit `internal/adapter/webui/handler/auth.go` (delete `HandleLoginPost`) + its route registration (find in webui server setup — grep `HandleLoginPost`)
- Edit SPA: `webui/src/stores/auth.ts`, `webui/src/api/client.ts`

**Changes**
1. `cookie/manager.go` — `SessionData` already has `Token string`; add `RefreshToken string ` + json tag `refresh_token,omitempty``. Manager (AES-GCM) unchanged.
2. `proxy.go` `HandleProxy` — new behavior, in order:
   1. `c.Request.Header.Del("Authorization")` — strip any client-supplied header (the SPA stops sending it; this is defense in depth).
   2. `session, _ := h.cookieManager.Decrypt(c.Cookie("automata_session"))` (existing code).
   3. Outbound auth: `if session != nil && session.Token != "" { c.Request.Header.Set("Authorization", "Bearer "+session.Token) }`. **Delete** the current `Bearer <session.UserID>` line (line 49) — it is dead/broken (the API rejects non-JWT bearers).
   4. Token-custodian interception: if `strings.HasPrefix(path, "/api/auth/login") || strings.HasPrefix(path, "/api/auth/refresh")`, serve through a body-capturing writer:
      ```go
      type captureWriter struct {
          w http.ResponseWriter
          status int
          body *bytes.Buffer
          headerWritten bool
      }
      ```
      (implements `http.ResponseWriter` + `http.Flusher` to be safe; `WriteHeader` records status, `Write` tees into buffer **and** passes through so the client still streams). After `h.proxy.ServeHTTP(cw, c.Request)`: if `cw.status == http.StatusOK`, unmarshal `cw.body` into `map[string]interface{}`; if a string `token` is present:
      - start from existing session (if any) else `SessionData{UserID: body["user_id"], Email: body["email"]}`;
      - set `Token`, `RefreshToken` (from `refresh_token`, may be absent);
      - encrypt, `c.Header("Set-Cookie", "automata_session="+val+"; Path=/; Max-Age=86400; HttpOnly; Secure; SameSite=Lax")`;
      - delete `token`/`refresh_token` from the map, re-marshal, reset `Content-Length`.
      Non-200 responses pass through untouched.
3. `auth.go`: delete `HandleLoginPost` and its route. Keep `HandleLogin` (redirect), `HandleLogout` (clears cookie — also verify it clears the full cookie attrs), `GetSessionData`, `RequireAuth` (audit usages; keep — it guards page rendering, not a login path).
4. SPA:
   - `stores/auth.ts`: remove all `localStorage` token reads/writes. `login()` POSTs `/api/auth/login` (cookie set server-side via interception) then hydrates profile via `GET /api/auth/me` into the store (`is_admin` included).
   - `api/client.ts`: axios request interceptor no longer sets `Authorization`. Response interceptor: on 401 (and not already retried, not the refresh call itself) → `POST /api/auth/refresh` → retry original request once; refresh failure → clear profile → redirect `/login`.
   - Grep-assert (vitest): no `localStorage.getItem('token')` / `localStorage.setItem('token'` anywhere under `webui/src`.

**Regression tests**
- `internal/adapter/webui/handler/proxy_test.go` (httptest fake backend that echoes status/body/Authorization it received):
  - `TestProxy_LoginSetsSessionCookie` — 200 login body with token → response has `Set-Cookie: automata_session=...`; cookie decrypts to `SessionData.Token==token`.
  - `TestProxy_StripsClientAuthorization` — request with `Authorization: Bearer evil` on `/api/context/contacts` + valid session → backend receives `Bearer <session.Token>`, never `evil`.
  - `TestProxy_NoSessionNoAuthHeader` — no cookie, non-login path → backend receives no `Authorization` header.
  - `TestProxy_RefreshUpdatesCookie` — 200 refresh body → cookie token rotated.
  - `TestProxy_LoginResponseStripsTokens` — login response body to client contains no `token`/`refresh_token` keys.
- `webui/src/api/__tests__/client.spec.ts` (vitest): no `localStorage` references (grep-assert over `webui/src`); 401→refresh→retry flow with mocked axios.

**Commit:** `fix(security): BFF becomes JWT custodian; cookie-only WebUI auth; remove localStorage tokens [M7,M17]`

---

### Task 4: WebUI admin route guard (spec 3.3)

**Files**
- `webui/src/router/index.ts`
- Admin views: add `meta: { requiresAdmin: true }` to admin routes (grep `router/index.ts` for admin route definitions)

**Changes**
1. `beforeEach`: after the existing auth check, `if (to.meta.requiresAdmin && !authStore.isAdmin) return { path: '/dashboard' }`.
2. `authStore.isAdmin` comes from the `/api/auth/me` hydration done in Task 3.

**Regression test**
- `webui/src/router/__tests__/guard.spec.ts` (vitest): non-admin navigating to a `requiresAdmin` route is redirected to dashboard; admin passes.

**Commit:** `fix(security): SPA admin route guard [M1-frontend]`

---

### Task 5: Anonymous snippet XSS (spec 3.5, Major #4)

**Actual location:** `internal/adapter/proxy/proxy.go` — `snippetHandler` (lines 51-60) interpolates the user-controlled path into generated JS. (Spec pointed at `webui/handler/proxy.go:65`; this is the code it describes — route `GET /snippet/*path` registered at line 23, anonymous.)

**Files**
- `internal/adapter/proxy/proxy.go`

**Changes**
1. In `snippetHandler`: after trimming, validate `contextID` with UUID regex `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$` (lowercase; `uuid.Parse` from `github.com/google/uuid` also acceptable). Non-UUID → `c.AbortWithStatus(http.StatusNotFound)`.
2. Defensively escape the value in `generateSnippet` (treat it as untrusted even after validation): JSON-escape the interpolated string (it sits inside single-quoted JS string literals, so minimally escape `'`, `\`, newlines).

**Regression tests** (`internal/adapter/proxy/proxy_test.go`)
- `TestSnippet_NonUUIDPath404` — `/snippet/<script>alert(1)</script>` → 404, body does not contain the input.
- `TestSnippet_UUIDServesJS` — valid UUID → 200 `application/javascript`, contains the UUID.

**Commit:** `fix(security): validate snippet path as UUID; escape interpolation [M4]`

---

### Task 6: Webhook SSRF + encrypted signing secret (spec 3.6, Major #5)

**Files**
- `internal/domain/webhook/webhook_endpoint.go`
- `internal/infrastructure/auth/` (new secret-key accessor)
- `internal/infrastructure/webhook/repo/migration.go` + new `migration_webhook_secret.sql`
- `internal/infrastructure/webhook/repo/postgres.go`
- `cmd/automata/main.go` (migration call)
- `internal/infrastructure/webhook/service.go` (dispatch re-check)

**Changes**
1. **SSRF regex** — in `webhook_endpoint.go` replace the current URL pattern (around line 75) with:
   ```go
   // https to any host, or plain http to loopback only (local dev webhooks).
   var webhookURLPattern = regexp.MustCompile(`^(https://|http://(localhost|127\.0\.0\.1)(:\d+)?/)`)
   ```
   Keep existing length/format rules.
2. **Secret key accessor** — in `internal/infrastructure/auth` (next to `GetJWTSecret`/`GetCookieSecret`): `GetSecretKey() [32]byte` = `sha256.Sum256(config.Get("secret_key"))`; if `secret_key` unset, fall back to `config.Get("cookie_secret")` (same material the cookie manager uses — no new deploy requirement; document in the accessor's comment).
3. **Migration** — new embedded file `internal/infrastructure/webhook/repo/migration_webhook_secret.sql`:
   ```sql
   ALTER TABLE webhook_endpoints ADD COLUMN IF NOT EXISTS signing_secret_enc BYTEA;
   ```
   Runner: extend the webhook repo `RunMigrations` (copy the `RunUnderMigrationLock` pattern — the webhook repo currently runs its SQL directly; wrap it the same way `internal/infrastructure/database` does, **or** add the step to the existing runner in `migration.go`) to execute, under the advisory lock, in order:
   1. the SQL above;
   2. **Go backfill**: `SELECT id, signing_secret FROM webhook_endpoints WHERE signing_secret IS NOT NULL` → AES-GCM encrypt each with `GetSecretKey()` (nonce prepended, base64) → `UPDATE ... SET signing_secret_enc=$2, signing_secret=NULL`;
   3. `ALTER TABLE webhook_endpoints DROP COLUMN IF EXISTS signing_secret;`
   Called from `cmd/automata/main.go` next to the other `RunMigrations` calls.
4. **Repo mapping** (`postgres.go`): every place that reads/writes `signing_secret` (lines ~37, 61, 97, 137-144, 207) switches to `signing_secret_enc`: read = decrypt into domain `SigningSecret` (plaintext in memory only, at delivery time); write = encrypt. Domain `webhook_endpoint.go` struct unchanged (`SigningSecret *string` stays the app-layer field).
5. **Dispatch re-check** (defense in depth): in `service.go` where the delivery HTTP call is made, re-validate the URL against `webhookURLPattern` and skip (log + mark failed) if it fails — protects rows created before the migration.

**Regression tests**
- `internal/domain/webhook/webhook_endpoint_test.go`:
  - `TestWebhookURL_Anchored` — table: `https://example.com/x` ok; `http://localhost/x` ok; `http://127.0.0.1:8080/x` ok; `http://localhost.evil.com/x` **rejected**; `http://169.254.169.254/latest` **rejected**; `ftp://x` **rejected**; `https:evil.com` **rejected**.
- `internal/infrastructure/webhook/repo/postgres_test.go` (real PG):
  - `TestSecretMigration_PrePostState` — insert endpoint with plaintext `signing_secret` (via raw SQL before running migration) → run migrations → `signing_secret` column gone, `signing_secret_enc` decrypts to original; repo `Get` returns the plaintext secret in `SigningSecret`.
  - `TestSecret_RoundTrip` — create via repo → column encrypted at rest (raw SQL check: `signing_secret_enc` ≠ plaintext), read back via repo → matches.

**Commit:** `fix(security): anchored webhook URL allowlist; AES-GCM signing secret, plaintext dropped [M5]`

---

**Phase 1 gate:** `go build ./... && go vet ./... && go test ./...` + `cd webui && npm run test` — all green.

---

## Phase 2 — DoS + hardening

### Task 7: Body-size limits (spec 4.1)

**Files**
- `internal/infrastructure/config/config.go` (add `GetInt`)
- Create `internal/adapter/api/middleware/body_limit.go`
- `internal/adapter/api/server.go`
- `internal/adapter/webui/handler/proxy.go` (BFF side)
- `internal/adapter/api/handler/contact_handler.go` (Import)
- `internal/adapter/api/handler/staticfile_handler.go` (upload: enforce declared `header.Size` vs cap before reading)
- `internal/adapter/api/handler/upload_handler.go` (unknown field → reject)
- `internal/adapter/api/handler/tracking_handler.go` (batch cap)
- MCP `contacts_import` handler (find: grep `contacts_import` in `internal/adapter/mcp/`)
- `pkg/import/csv_importer.go`

**Changes**
1. `config.go`: `func GetInt(key string, def int) int` (strconv parse, fallback).
2. `middleware/body_limit.go`:
   ```go
   func LimitBody(limit int64) gin.HandlerFunc {
       return func(c *gin.Context) {
           if cl := c.Request.ContentLength; cl > limit {
               c.AbortWithStatus(http.StatusRequestEntityTooLarge) // 413
               return
           }
           c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
           c.Next()
       }
   }
   ```
   Limit = `config.GetInt("max_body_bytes", 10*1024*1024)`.
3. Apply `LimitBody(...)` to the `api` group in `server.go` (after Task 2's `Use` chain) and to the BFF proxy in `webui/handler/proxy.go` (wrap `c.Request.Body` in `HandleProxy` for `/api/*`).
4. `tracking_handler.go` batch endpoint: cap 100 events/request → 422 (or 413 — pick 422, test asserts the status) with a clear error.
5. `contact_handler.go` Import + `staticfile_handler.go` upload + MCP `contacts_import`: rely on the group `LimitBody` (already applied) + staticfile additionally compares the multipart part's declared size to the cap before reading; `upload_handler.go`: unknown upload field → 400 (currently skipped).
6. `csv_importer.go`: `reader.FieldLineLimit = 1 << 20` (1 MB); stop after 100,000 rows with a returned error.

**Regression tests**
- `internal/adapter/api/middleware/body_limit_test.go`: 11-byte body over limit → 413; under → 200.
- `tracking_handler_test.go` (or integration): 101-event batch → 422.
- `staticfile_handler_test.go`: declared `header.Size` over cap → 413 before body read.
- `upload_handler_test.go`: unknown field in multipart → 400.
- `pkg/import/csv_importer_test.go`: field >1MB → error; 100,001 rows → error.

**Commit:** `fix(security): request body caps (10 MB default, batch ≤100, CSV 1MB/100k) [M6]`

---

### Task 8: Refresh token rotation (spec 4.6)

**Files**
- `internal/infrastructure/database/migration_user_contexts.sql` (append) + runner already executes it
- Create `internal/infrastructure/auth/repo/refresh_tokens.go`
- `internal/infrastructure/auth/service.go`
- `internal/adapter/api/handler/auth_handler.go` (login inserts jti)
- `cmd/automata/main.go` (wire store)

**Changes**
1. Append to `migration_user_contexts.sql` (idempotent, runs under existing advisory lock via `RunUserContextMigrations`):
   ```sql
   CREATE TABLE IF NOT EXISTS refresh_tokens (
       jti        UUID PRIMARY KEY,
       user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
       expires_at TIMESTAMPTZ NOT NULL,
       revoked_at TIMESTAMPTZ,
       created_at TIMESTAMPTZ DEFAULT NOW()
   );
   CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
   ```
2. `internal/infrastructure/auth/repo/refresh_tokens.go`:
   ```go
   type RefreshTokenStore interface {
       Insert(ctx context.Context, jti, userID string, expiresAt time.Time) error
       // RevokeIfActive revokes an active jti. Outcome:
       //  "ok"              – was active, now revoked
       //  "already_revoked" – reuse attempt (theft signal)
       //  "missing"         – unknown jti
       RevokeIfActive(ctx context.Context, jti string) (string, error)
       Prune(ctx context.Context, olderThan time.Time) (int64, error)
   }
   ```
   `RevokeIfActive` = single `UPDATE refresh_tokens SET revoked_at=NOW() WHERE jti=$1 AND revoked_at IS NULL` (+ expiry check) → branch on `RowsAffected`; `missing` distinguished by prior `EXISTS` check (or RETURNING + `pgx.ErrNoRows` fallback SELECT).
3. `service.go` `service` struct: add `refreshStore RefreshTokenStore` (nil = legacy stateless behavior, keeps existing tests green); new constructor `NewServiceWithRefreshTokens(userRepo, apiKeyStore, refreshStore)`. Wire in `cmd/automata/main.go`.
   - `Login`: after minting the refresh JWT (handler does this today — move the jti insert here or pass jti back; simplest: `auth_handler.go Login` inserts `refreshStore.Insert(ctx, jti, userID, exp)` using the same jti it put in the claims).
   - `RefreshToken(refreshToken)`: parse → `jti` claim → `RevokeIfActive`: `missing` → `ErrRefreshTokenInvalid`; `already_revoked` → `log.Warn().Str("jti", ...).Msg("refresh token reuse detected")` + `ErrRefreshTokenInvalid`; `ok` → issue new pair (fresh jti) and insert new row.
4. `auth_handler.go` `HandleRefresh` unchanged beyond the service now enforcing rotation.

**Regression tests**
- `internal/infrastructure/auth/service_test.go` (stub store + real PG for the repo):
  - `TestRefreshToken_Rotation` — refresh once → old jti revoked, new token works.
  - `TestRefreshToken_ReuseRejected` — use the *same* refresh token twice → second call errors (and warn log emitted, assert via zerolog test writer).
- `internal/infrastructure/auth/repo/refresh_tokens_test.go` (real PG): insert/revoke/prune.

**Commit:** `fix(security): server-side refresh token rotation with reuse detection`

---

### Task 9: Rate limiting (spec 4.2)

**Files**
- Create `internal/adapter/api/middleware/rate_limit.go`
- `internal/adapter/api/server.go`

**Changes**
1. Stdlib token bucket, in-process (per-pod — documented limitation, per spec §9):
   ```go
   type Limiter struct{ mu sync.Mutex; buckets map[string]*bucket }
   func NewLimiter() *Limiter
   func (l *Limiter) Allow(key string, capacity int, refill time.Duration) bool // capacity tokens, refilled capacity per refill window
   func (l *Limiter) PurgeIdle(olderThan time.Time) // bounded map growth
   ```
   Middleware: `func RateLimit(l *Limiter, capacity int, window time.Duration, keyFn func(*gin.Context) string) gin.HandlerFunc` → 429 + `Retry-After: 60` on deny. Background purge goroutine (or lazy purge on each call).
2. Wiring in `server.go`:
   - `api.POST("/auth/login", ...)` and `api.POST("/auth/refresh", ...)`: `RateLimit(loginLimiter, 10, time.Minute, loginKeyFn)` where `loginKeyFn` = `c.ClientIP() + "|" + email` — `email` read by peeking the request body (buffer up to 4 KB, `io.ReadAll`, re-wrap with `io.NopCloser(bytes.NewReader(buf))`; malformed/missing email → IP only).
   - Public tracking routes: find the public `/track` + `/page` routes (grep server.go / proxy package — the public tracking endpoints served without auth) and apply `RateLimit(trackLimiter, 300, time.Minute, ipKeyFn)`. If public tracking lives in the BFF/proxy layer instead, apply there — check both, apply at the serving layer.
   - Limits overridable via config keys `rate_login_per_min` (default 10), `rate_tracking_per_min` (default 300).

**Regression tests**
- `internal/adapter/api/middleware/rate_limit_test.go`:
  - `TestRateLimit_AllowsUnderCapacity` — 10 logins pass, 11th → 429 with `Retry-After`.
  - `TestRateLimit_RefillsAfterWindow` — with a small window (e.g. 100 ms), capacity restores.
  - `TestRateLimit_TrackingPerIP` — 300 pass, 301st → 429.

**Commit:** `fix(security): token-bucket rate limiting on login/refresh (10/min) and public tracking (300/min/IP)`

---

### Task 10: Tracking events query bound (spec 4.3)

**Files**
- `internal/adapter/api/handler/tracking_handler.go` (`GetEvents`)
- `internal/infrastructure/tracking/repo/` (list query)

**Changes**
1. `GetEvents`: parse `limit` query param → default 50, clamp to max 100; `offset` default 0.
2. Repo list method must always bind `LIMIT $n` (verify the existing query; if it already takes a limit, just clamp in the handler).

**Regression tests**
- `tracking_handler_test.go`: no `limit` → response ≤50 rows (seed 60); `limit=1000` → capped at 100.

**Commit:** `fix(security): bound tracking events list (default 50, max 100)`

---

### Task 11: CSV formula injection neutralization (spec 4.4)

**Files**
- `pkg/export/csv_exporter.go`

**Changes**
- Single choke point at export: helper
  ```go
  func sanitizeCSVCell(s string) string {
      if s == "" { return s }
      switch s[0] { case '=', '+', '-', '@', '\t', '\r': return "'" + s }
      return s
  }
  ```
  Applied to every cell written by the exporter. Import intentionally stores raw (per spec).

**Regression test**
- `pkg/export/csv_exporter_test.go`: `TestCSVInjection_Neutralized` — values `=SUM(A1)`, `+cmd`, `-1`, `@x`, tab-prefixed → exported cells start with `'`; plain values untouched.

**Commit:** `fix(security): neutralize CSV formula injection at export`

---

### Task 12: Auth hygiene + BFF ops (spec 4.5, mediums)

**Files**
- `internal/adapter/webui/handler/proxy.go` / `auth.go` (cookie `Secure`)
- `internal/adapter/api/handler/auth_handler.go` (line 51 log)
- `cmd/bff/main.go`

**Changes**
1. WebUI session cookie: `Secure: true` in `HandleLoginPost`'s successor (the proxy's `Set-Cookie` in Task 3 — add `Secure` there) and `HandleLogout`. Config escape hatch: `config.Get("insecure_cookie") == "true"` → omit `Secure` (local HTTP dev).
2. `auth_handler.go:51`: `log.Info().Str("email", shortHash(req.Email))` (12-hex-char sha256 prefix) instead of raw email.
3. `cmd/bff/main.go`: graceful shutdown mirroring `cmd/automata` (explicit `*http.Server`, `signal.NotifyContext`/`os.Signal` handler, `srv.Shutdown(ctx)` with timeout); `RunMigrations`/setup errors → `log.Fatal` (not silently ignored).

**Regression tests**
- Proxy test (extend Task 3's suite): `TestProxy_CookieHasSecureFlag` — `Set-Cookie` contains `Secure` (and `HttpOnly; SameSite=Lax`); with `insecure_cookie=true` it does not.
- `TestLoginLog_DoesNotContainEmail` — capture zerolog output during login, assert the raw email string is absent.

**Commit:** `fix(security): Secure session cookie, hashed email in logs, BFF graceful shutdown + fatal migrations`

---

**Phase 2 gate:** full Go + vitest runs green.

---

## Phase 3 — Data integrity

### Task 13: Hidden-contact dedupe + unique index (spec 5.1)

**Files**
- `internal/infrastructure/contact/migration.sql` (append; runs via `contact.RunMigrations`)
- `internal/usecase/tracking/track_event.go` (`trackPageVisit`)

**Changes**
1. Migration (idempotent — guard dedupe so re-runs are no-ops; read `migration.sql` first for the exact column names of `contacts` and the tables that reference `contact_id` — at minimum `tracking_events` / form submissions / tags tables):
   ```sql
   -- Reassign dependent rows to the oldest duplicate, then delete the younger ones.
   WITH dups AS (
       SELECT id, MIN(id) OVER (PARTITION BY context_id, source_id) AS keep_id
       FROM contacts WHERE source = 'visitor'
   )
   UPDATE <ref_table> SET contact_id = d.keep_id
   FROM dups WHERE <ref_table>.contact_id = d.id AND d.id <> d.keep_id;   -- for each ref table
   DELETE c FROM contacts c
   JOIN contacts k ON k.context_id = c.context_id AND k.source_id = c.source_id
       AND k.source = 'visitor' AND k.created_at < c.created_at;
   CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_visitor_unique
       ON contacts(context_id, source_id) WHERE source = 'visitor';
   ```
   (Dedupe runs only while duplicates exist, so it stays idempotent; the index is `IF NOT EXISTS`.)
2. `track_page_visit` in `track_event.go`: on unique violation (pg error code `23505`) when creating the visitor contact, re-read the existing contact by `(context_id, source, source_id)` and continue with it (update client info / record the event against it) instead of failing.

**Regression tests**
- Migration test (extend `internal/infrastructure/contact/migration_test.go`, real PG): seed 3 duplicate visitor contacts + dependent rows → run migration → 1 contact remains (oldest), dependents reattached, index rejects a second insert.
- `track_event_test.go`: pre-existing visitor contact present → `TrackEvent` page-visit succeeds and attaches to the existing contact (no duplicate row).

**Commit:** `fix(integrity): dedupe hidden visitor contacts + unique index; upsert on conflict`

---

### Task 14: Atomic queue claims (spec 5.4)

**Files**
- `internal/infrastructure/queue/webhook_queue.go`, `webhook_worker.go`
- `internal/infrastructure/queue/email_queue.go`, `email_worker.go`, `migration.sql`
- tests in `webhook_queue_test.go`, new `email_queue_test.go`

**Changes**
1. **Webhook** (schema already has `status`):
   - Interface: add `Claim(ctx context.Context, limit int) ([]*WebhookDelivery, error)` and `ResetStaleProcessing(ctx context.Context, olderThan time.Time) (int64, error)`.
   - `Claim` SQL (single statement = atomic):
     ```sql
     WITH picked AS (
         SELECT id FROM webhook_deliveries
         WHERE status IN ('pending','retrying') AND next_retry <= NOW()
         ORDER BY created_at LIMIT $1
         FOR UPDATE SKIP LOCKED
     )
     UPDATE webhook_deliveries d SET status = 'processing', updated_at = NOW()
     FROM picked p WHERE d.id = p.id
     RETURNING d.id, d.context_id, COALESCE(d.form_id::text,''), d.url, d.payload,
               d.status, d.attempts, d.max_retries, d.next_retry, d.error_msg,
               d.created_at, d.updated_at;
     ```
   - `ResetStaleProcessing`: `UPDATE webhook_deliveries SET status='retrying', updated_at=NOW() WHERE status='processing' AND updated_at < $1` (reaper: 5 min).
   - `webhook_worker.go`: replace `GetPending` with `Claim`; run the reaper once per poll tick (cheap single UPDATE); success/failure paths unchanged (`MarkSuccess`/`MarkFailed`/`UpdateNextRetry`).
2. **Email** (no `status` column today):
   - `migration.sql` append: `ALTER TABLE email_jobs ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'pending';`
   - `EmailQueue` interface: same `Claim`/`ResetStaleProcessing` pair (claim from `'pending'` — email jobs use `next_retry`/attempts, no `'retrying'` state today; claim condition `status = 'pending'`).
   - `email_worker.go`: claim → send → success: insert history + delete job (existing behavior); failure: `UPDATE email_jobs SET status='processing'→ keep processing until retry scheduled` — concretely: on failure `UPDATE ... SET status='pending', next_retry=..., attempts=attempts+1 WHERE id=$1` (back to claimable), or terminal handling per existing max-retries logic. Reaper resets `'processing'` → `'pending'` after 5 min.
   - `CountPending`: add `AND status IN ('pending')` (keep the attempts check).

**Regression tests**
- `webhook_queue_test.go`: `TestClaim_ConcurrentWorkersSingleWinner` — seed 1 job, run 2 goroutines claiming → exactly one gets it; `TestClaim_StaleReaper` — force a row to `'processing'` with old `updated_at` → reaper flips it to `'retrying'`.
- `email_queue_test.go`: same two tests for email.

**Commit:** `fix(integrity): atomic queue claims (processing state + SKIP LOCKED) with stale reaper`

---

### Task 15: Non-atomic visitor merge (spec 5.2)

**Files**
- `internal/usecase/form/identify_visitor.go`
- `internal/infrastructure/contact/repo/` (tx seam)
- `internal/infrastructure/tracking/repo/` (tx seam)

**Changes**
1. Add a tx seam to the concrete infra repos (they wrap `pgxpool`):
   ```go
   func (r *repo) WithTx(ctx context.Context, fn func(tx *pgx.Tx) error) error {
       tx, err := r.pool.Begin(ctx); defer tx.Rollback(ctx);
       if err := fn(tx); err != nil { return err }
       return tx.Commit(ctx)
   }
   ```
   (contact repo and tracking repo each get it; or a shared small helper if they share a base.)
2. `identify_visitor.go`: the usecase's repo params stay the domain interfaces; add an optional narrow interface:
   ```go
   type txProvider interface{ WithTx(ctx context.Context, fn func(tx *pgx.Tx) error) error }
   ```
   — if the injected repos implement it, run `contactRepo.Merge` + `trackingRepo.ReassignVisitorContactID` (and the legacy create+link path) inside one `WithTx` by adding `*pgx.Tx`-aware method variants to the concrete repos (e.g. `MergeTx(tx, ...)`, `ReassignVisitorContactIDTx(tx, ...)`). If the repos don't implement `txProvider`, fall back to the current sequential path (keeps MCP/other callers and existing tests working).
3. Keep the domain interfaces untouched.

**Regression tests**
- `identify_visitor_test.go` (real PG): `TestIdentifyVisitor_MergeAtomic` — seed source+target visitor; force the reassign step to fail (e.g. via a savepoint/deferred constraint or a test hook) → contact merge is rolled back (source contact intact, no partial state).

**Commit:** `fix(integrity): atomic visitor merge (contact merge + event reassign in one tx)`

---

### Task 16: Tag replace in one transaction (spec 5.3)

**Files**
- `internal/usecase/contact/tags.go` (`tagRepo.ApplyTags` — it holds `*pgxpool.Pool` directly)

**Changes**
- `ApplyTags`: begin tx → `DELETE FROM <tags> WHERE context_id=$1 AND contact_id=$2` → INSERT each tag → commit. Any error → rollback, return error.

**Regression test**
- `tags_test.go` (real PG): `TestApplyTags_MidTxFailureRollsBack` — inject a failing INSERT (second tag name that violates a constraint, or test hook) → prior tags for the contact are unchanged.

**Commit:** `fix(integrity): tag replacement transactional`

---

### Task 17: Correctness mediums (spec 5.5 table)

Split into three commits for reviewability.

**17a — domain/pkg level**
| Fix | File | Detail |
|---|---|---|
| Duplicate field slugs rejected | `internal/domain/form/form.go` `Validate` (~line 32) | track seen slugs; duplicate → error |
| `isValidEmail`: exactly one `@` | `internal/domain/contact/contact.go:58` | `strings.Count(email, "@") == 1` (keep existing checks) |
| `Decision.Validate`: restrict `Action` | `internal/domain/page/page.go` (+ call site `page_public_handler.go:119`) | whitelist documented button actions; unknown → error |
| SMTP sanitize `from`/`to` | `internal/infrastructure/email/smtp/smtp.go:196-202` | strip `\r`/`\n` like `subject` is handled |
| `pgtype.Array` binding | `internal/infrastructure/queue/email_queue.go:93` | bind `pgtype.Array[string]` instead of hand-built SQL literal |

**17b — usecase level**
| Fix | File | Detail |
|---|---|---|
| Log activity-creation errors | `create_contact.go:87`, `merge_contacts.go:60` | `log.Error().Err(err)` instead of discard |
| Import dups counted `Skipped` | `import_contacts.go:69` | fix the counter branch |
| Tags clearable (nil vs empty) | `update_contact.go:142` | `*[]string` param: nil = leave, empty = clear |
| `""` email → nil; 23505 → re-read + return existing | `create_contact.go:39`, `update_contact.go:47` | normalize; on `23505` re-read by (context,email) and return the existing contact |
| `TestSend` validates `req.To` | `usecase/email/email.go:146` | non-empty, valid email |
| Dashboard zero bounds independent | `get_dashboard.go:33` | default each zero-valued bound separately (no cross-contamination) |
| Export paginates to exhaustion | `export_contacts.go:20` | loop pages until exhausted (per spec §9: no truncation flag) |

**17c — adapter/infra level**
| Fix | File | Detail |
|---|---|---|
| Bootstrap admin password not on stdout | `internal/infrastructure/bootstrap/admin.go:76` | write to a secret file path (config) / obfuscated log handle; never plaintext to stdout |
| `staticfile List` limit/offset | `staticfile/repo/postgres.go:72` | add `limit`/`offset` params (default 100/0, max 500) |
| Generic client error messages | context/contact/tracking/user handlers (see review list) | replace raw `err.Error()` in responses with a generic message; log the real error server-side |

**Tests:** one focused unit test per row (17a: table-driven domain tests; 17b: usecase tests with stubs/real PG where DB matters; 17c: handler tests asserting generic message + presence of logged error, repo test for limit/offset).

**Commits:** `fix(integrity): domain validations (form slugs, email, decision actions, SMTP headers, pgtype)`, `fix(integrity): usecase correctness fixes (5.5)`, `fix(integrity): adapter/infra correctness (bootstrap secret, staticfile paging, generic errors)`

---

**Phase 3 gate:** full Go run green.

---

## Phase 4 — WebUI mediums (spec §6)

### Task 18: Dashboard stale-response race + delete error surfacing

**Files**
- `webui/src/views/DashboardView.vue`
- `webui/src/components/settings/UsersPanel.vue`, `WebsitesPanel.vue`

**Changes**
1. `DashboardView.vue`: add a generation counter (copy the pattern from `webui/src/components/settings/WebsiteEmailPanel.vue:59-78`): capture `const gen = ++loadSeq` before each `loadOverview`/`loadMapData` await; after the await, `if (gen !== loadSeq) return;` before writing to reactive state.
2. `UsersPanel.vue` / `WebsitesPanel.vue`: on delete failure, surface `e.response?.data?.error` in the existing error banner/toast (currently swallowed).

**Regression tests** (vitest, component or unit-level):
- `DashboardView` spec: two overlapping `loadOverview` calls → stale (first) response does not overwrite fresh state (mock delayed promises, assert final state = second call's data).
- Panel spec: failed delete → error banner shows server message.

**Commit:** `fix(webui): dashboard race guard (generation counter); surface delete errors`

---

## Final gate & wrap-up

1. `go build ./... && go vet ./... && go test ./...` — green.
2. `cd webui && npm run test` — green.
3. Verify all ≥17 Major regression tests exist by name (cross-check against spec §7 list).
4. Update `codereview_findings.md`: mark 2026-11-04 findings resolved with commit refs (replace the stale 2026-09-28 report or clearly supersede it).
5. Deploy note for the user: push to `main`, watch the CI pipeline (`test` → `publish`); verify via `https://m.h2hsecure.com/api/health`.

## Notes for the executor

- `SessionData.Token` already exists in the cookie package — do not re-add it; only `RefreshToken` is new.
- `api.Use(...)` in `server.go` only affects routes registered *after* the `Use` call (gin semantics) — that is why Task 2 wires membership at line 207 without touching admin/MCP routes.
- The snippet XSS lives in `internal/adapter/proxy/proxy.go` (not `webui/handler/proxy.go` as the spec text says — same fix, correct file).
- Webhook repo migrations run without the advisory lock today; Task 6 wraps them in `RunUnderMigrationLock` (copy `internal/infrastructure/database/migration_lock.go`).
- Email jobs have no `status` column today — Task 14 adds it; existing rows default to `'pending'`.
- Rate limiter is in-process per pod by design (spec §9) — no Redis.
- Do not touch MCP/API-key auth paths (spec non-goal).