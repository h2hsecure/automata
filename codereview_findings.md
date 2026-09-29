# Automata — Full Codebase Review

**Date:** 2026-09-28  
**Scope:** All production Go code across `internal/`, `pkg/`, and `cmd/` (202 Go files, ~4,000+ lines)  
**Method:** Parallel review across 5 architectural layers (domain, use case, API adapters, infrastructure, shared packages)

---

## Executive Summary

| Severity | Count | Key Themes |
|----------|-------|------------|
| **CRITICAL** | **22** | Hardcoded secrets, SQL injection, XSS, CSV injection, broken auth, missing shutdown |
| **HIGH** | **36** | No rate limiting, no CORS, missing RBAC, error message leaks, transaction gaps |
| **MEDIUM** | **41** | Error leaks, code duplication, weak validation, missing tests, fragile patterns |
| **LOW** | **28** | Inconsistent patterns, dead code, minor edge cases |

---

## Top 10 Immediate Fixes (Priority Order)

| # | Severity | Area | Issue | File |
|---|----------|------|-------|------|
| 1 | CRITICAL | API/Middleware | Hardcoded JWT secret key — forgeable tokens | `api/middleware/auth.go:34`, `auth/service.go:45` |
| 2 | CRITICAL | API/Middleware | WebUI proxy sends raw UUID as Bearer token (always fails) | `webui/handler/proxy.go:49` |
| 3 | CRITICAL | pkg/snippet | XSS via unescaped contextID/apiHost in JS snippets | `pkg/snippet/generator.go:34,61` |
| 4 | CRITICAL | pkg/import | CSV formula injection (Excel) — `=cmd|' /C calc` | `pkg/import/csv_importer.go:83` |
| 5 | CRITICAL | Infrastructure | Plaintext signing secret stored in DB | `webhook/service.go:49,107` |
| 6 | CRITICAL | cmd/bff | Zero graceful shutdown — SIGTERM drops requests | `cmd/bff/main.go:142` |
| 7 | CRITICAL | Use Cases | Swallowed errors silently discard failures | `create_contact.go:89`, `update_contact.go:120`, `merge_contacts.go:61`, `tags.go:60` |
| 8 | CRITICAL | pkg/import | No value length limits — DoS via massive CSV cells | `pkg/import/csv_importer.go:83` |
| 9 | CRITICAL | API/Banner | Context resolver trusts Host header subdomain without validation | `banner/context_resolver.go:12`, `tracking/context_resolver.go:12` |
| 10 | CRITICAL | cmd/bff | Weak default cookie secret — "change-me-in-production" | `cmd/bff/main.go:42` |

---

## 1. Domain Layer (`internal/domain/`)

### CRITICAL (3)

| # | File:Line | Issue |
|---|-----------|-------|
| C1 | `auth/auth_test.go:6,10` | Domain tests import `infrastructure/auth` and pass `nil` dependencies — mislabeled as domain tests but actually test infra |
| C2 | `auth/auth.go:38-39` | `AuthService` interface returns `*context.User` — auth depends on context domain, creating tight coupling |
| C3 | `context/user.go:17,23` | `ContextID` and `IsOwner` deprecated fields pollute the domain entity |

### HIGH (13)

| # | File:Line | Issue |
|---|-----------|-------|
| H1 | `banner/banner.go:127-168` | Gin `binding:` tags on request DTOs (4 occurrences) |
| H2 | `email/email.go:23-56` | Gin `binding:` tags on request DTOs (6 occurrences) |
| H3 | `webhook/webhook_endpoint.go:26-42` | Gin `binding:` tags on input DTOs (3 occurrences) |
| H4 | `banner/repository.go:6-32` | Repository interface has 16 methods spanning 4 bounded contexts — should be split |
| H5 | `tracking/repository.go:21-43` | Repository interface has 16 methods mixing CRUD with analytics — should be split |
| H6 | `banner/banner.go:24-45` | `Banner` entity has no `Validate()` method |
| H7 | `form/form.go:32-40` | `Validate()` doesn't check `ContextID` |
| H8 | `form/submission.go:15-22` | `Submission` has no `Validate()` method |
| H9 | `email/email.go:58-87` | Response DTOs (`TemplateResponse`, etc.) belong in adapter layer, not domain |
| H10 | `webhook/webhook_endpoint.go:45-55` | `WebhookResponse`, `WebhookListResponse` DTOs in domain |
| H11 | `webhook/webhook_endpoint.go:19` | Raw `SigningSecret *string` in domain entity — secrets should only exist in infrastructure |
| H12 | `webui/session.go:6-15` | `Session` entity stores raw `Token string` — tokens should be hashed |
| H13 | `context/context.go:79-99` | `Clone()` shallow copies `Settings` map — nested values are shared references |

### MEDIUM (17)

| # | File:Line | Issue |
|---|-----------|-------|
| M1 | `contact/contact.go:27-37` | `Validate()` checks `ContextID` but not `ID` — entity without ID can't be updated |
| M2 | `contact/activity.go:51-92` | `FormatActivityTitle()`/`FormatActivityDescription()` are presentational logic in domain |
| M3 | `context/context.go:272-279` | `FormatCreatedAt()`, `DisplaySlug()` are presentation-layer concerns in domain |
| M4 | `banner/banner.go:77-80` vs `tracking/repository.go:8-11` | `DateRange` duplicated across two packages |
| M5 | `webhook/webhook_endpoint.go:140-142` | Weak email validator (`IsValidEmail`) — only checks `@` and `.`, inconsistent with contact package |
| M6 | `tracking/list_visitors_opts.go:7` | `Sort` field accepts raw strings — should be typed constants |
| M7 | `webui/session.go:10-11` | `ContextName` denormalized data in domain entity |
| M8 | `config/config.go:36` | `Value map[string]interface{}` — no type safety |
| M9–M16 | Various | **Missing test files:** banner, email, tracking, webhook, webui, config — zero coverage |

### LOW (10)

- `contact/contact_test.go` — No tests for `Activity` formatting, `ContactWithCount`, merge behavior
- `context_test.go` — Missing tests for `Clone()`, `IsValidSettings()`, JSON marshal, `Equal()`
- `form_test.go` — No tests for `Submission` validation, field type validation, duplicate slug detection
- `webhook_endpoint.go:5` — Imports `net/http` for `HTTPStatusText()` — couples domain to HTTP stdlib
- Various: generic maps, minor import concerns

---

## 2. Use Case Layer (`internal/usecase/`)

### CRITICAL (7)

| # | File:Line | Issue |
|---|-----------|-------|
| C1 | `create_contact.go:89` | `_ = fmt.Errorf("create activity: %w", err)` — error silently discarded |
| C2 | `update_contact.go:120` | Same pattern — activity creation error swallowed |
| C3 | `merge_contacts.go:61` | Merge-then-activity: if activity fails, merge persists with no activity record |
| C4 | `tags.go:60` | `_ = fmt.Errorf("get current tags: %w", err)` — silently ignores tag fetch failure |
| C5 | `merge_contacts.go:40-62` | No transaction boundary — merge+activity are separate DB ops |
| C6 | `import_contacts.go:33-88` | No transaction — if row 50 fails, rows 1-49 are already persisted with no rollback |
| C7 | `update_form.go:27-31` | Blind overwrites with zero values — empty JSON request `{}` wipes the form |

### HIGH (15)

| # | File:Line | Issue |
|---|-----------|-------|
| H1 | `update_contact.go:125` | Empty tag array does NOT clear existing tags — contradicts comment "replaces all" |
| H2 | `list_contacts.go:28` | `repo.CountByContext` returns total for ALL contacts, not filtered result count |
| H3 | `import_contacts.go:39-56` | Race condition: duplicate check then create without transaction |
| H4 | `export_contacts.go:21` | Magic number 100000 — silently truncates large exports |
| H5 | `export_contacts.go:27` | Empty result returns error instead of empty CSV |
| H6 | `get_form.go:10-20` | No context validation on form ownership |
| H7 | `submit_form.go:33-35` | Honeypot check doesn't handle non-string types — malformed requests bypass |
| H8 | `track_event.go:18-32` | No transaction for event+visitor — analytics inconsistency |
| H9 | `track_event.go:22-28` | `Visitor` always created with `PageViews: 1` — overwrites existing count |
| H10 | `get_dashboard.go:71-74` | `metrics = repo.GetDashboardMetrics()` **overwrites** entire struct — 4 prior queries wasted |
| H11 | `banner.go:108-122,189,274` | Delete operations don't verify entity belongs to context |
| H12 | `banner.go:21-47` | No input validation in `CreateBanner` |
| H13 | `email.go:27` | Logger created internally with `zerolog.New(os.Stdout)` — not injectable |
| H14 | `email.go:100-103` | `BodyOverride` sets both text and HTML to same value — defeats multipart email |
| H15 | `get_visitor_detail.go:43` | No nil check on visitor — potential nil pointer |

### MEDIUM (16)

| # | File:Line | Issue |
|---|-----------|-------|
| M1 | `get_activity.go:17` | `_ = c` — contact looked up but discarded; fragile dependency on handler validation |
| M2 | `import_contacts.go:54` | `_ = existing` — dead code |
| M3 | `tags.go:219-220` | `var _ = json.Unmarshal` — hacky unused import silencer |
| M4 | `create_contact.go:103-132` vs `contact.go:44-73` | `isValidEmail()` duplicated in usecase and domain |
| M5 | `track_event.go:38-63` | `TrackEventWithTime` is exact duplicate of `TrackEvent` — could merge with optional param |
| M6 | `list_visitors.go:26`, `get_visitor_detail.go:30`, `get_visitor_events.go:28` | `nil` context passed to repo calls |
| M7–M16 | Various | Missing tests for all use cases — zero coverage across 27 files |

### LOW (12)

- `create_contact.go:77` vs `merge_contacts.go:45` — Inconsistent time formatting (RFC3339 vs raw time.Now())
- `segment_contacts.go:10-20` — Returns contacts without total count, inconsistent with `ListContacts`
- Various minor issues throughout

---

## 3. API Adapter Layer (`internal/adapter/`)

### CRITICAL (6)

| # | File:Line | Issue |
|---|-----------|-------|
| C1 | `api/middleware/auth.go:34`, `auth_handler.go:60`, `server.go:53`, `webui/server.go:16` | **Four hardcoded secret keys** — any deployment without config has forgeable JWTs and session cookies |
| C2 | `webui/handler/proxy.go:49` | Sends raw UUID as `Bearer` token — `VerifyTokenUserOnly()` expects JWT, always fails 401 |
| C3 | `banner/context_resolver.go:12-46`, `tracking/context_resolver.go:12-46` | Subdomain from Host header accepted without database validation |
| C4 | `proxy/proxy.go:65` | Hardcoded `https://automata.example.com/track.js` in generated snippet |
| C5 | `api/context_middleware.go:102` | SQL error details leaked to client in HTTP response |
| C6 | `api/server.go:10`, `banner/server.go:10`, `tracking/server.go:12`, `webui/server.go:30` | **No panic/recovery middleware** — any nil deref crashes the entire server |

### HIGH (11)

| # | File:Line | Issue |
|---|-----------|-------|
| H1 | All server files | **No rate limiting** on brute-force-prone endpoints (login, email send, tracking, form submit) |
| H2 | All server files | **No CORS middleware** — `config.ConfigKeyCORS` exists but is never applied |
| H3 | `api/server.go:66-114` | Admin endpoints lack role-based access control — any auth user can access admin routes |
| H4 | `api/handler/email_handler.go:31-35` | `List` requires `context_id` as query param, others use middleware-set value — inconsistent |
| H5 | `api/handler/webhook_handler.go:111,142` | SQL error message string-matching — fragile 404 detection |
| H6 | `api/handler/form_handler.go:169-172` | JSON parse failure silently falls back to query binding — no error returned |
| H7 | `api/handler/contact_handler.go:50,121`, `form_handler.go:48,64` | Returns 401 for missing context — should be 400 or 403 |
| H8 | `mcp/server.go:1-68` | **All MCP tools have zero authentication** — full access to any context |
| H9 | `mcp/server.go:28` vs `mcp/forms_tools.go:13` | `registerFormTools` never called — form tools defined but unregistered |
| H10 | `api/server.go:109,198` | `trackingRepo.New(pool)` called twice — duplicate instances |
| H11 | `api/handler/auth_handler.go:60-63` vs `api/middleware/auth.go:32-34` | Secret key resolution duplicated across two files |

### MEDIUM (10)

| # | File:Line | Issue |
|---|-----------|-------|
| M1 | Throughout handlers (~80+) | `gin.H{"error": err.Error()}` — internal DB/validation errors exposed to clients |
| M2 | `api/handler/banner_handler.go` | `NewBannerUsecase()` called 15 times per request handler — should be injected once |
| M3 | `banner/context_resolver.go` vs `tracking/context_resolver.go` | Byte-for-byte identical files (46 lines each) — DRY violation |
| M4 | `proxy/proxy.go:52-54` | Context ID parsing: `/snippet/foo.js.bak` → context ID = `foo.js` |
| M5 | `api/server.go:169` vs `form_handler.go:163` | Route uses `:id` but handler reads `:slug` — always empty |
| M6 | `contact_handler.go:308-313` | Admin bypass is a no-op — sets same value |
| M7 | `form_handler.go:20-34` | Form fields as raw `map[string]interface{}` — no schema validation |
| M8 | `webui/handler/auth.go` | No CSRF protection on login endpoint |
| M9 | `proxy/proxy.go:26` | `./static` relative path — breaks if binary runs from different directory |
| M10 | `tracking/track_handler.go:111-112` | Batch tracking accepts arbitrary nested properties — data injection risk |

### LOW (8)

- Inconsistent pagination patterns (page/limit vs offset/limit)
- Inconsistent delete response codes (204 vs 200)
- `webhook_handler.go:157-159` exposes internal repository
- `context_middleware_test.go` mixes Ginkgo/Gomega with standard testing
- `api/server.go:217-237` uses string concatenation for paths instead of `filepath.Join`
- Various missing Content-Type headers, host header forwarding

---

## 4. Infrastructure Layer (`internal/infrastructure/`)

### CRITICAL (3)

| # | File:Line | Issue |
|---|-----------|-------|
| C1 | `auth/service.go:45` | Hardcoded JWT secret `"automata-dev-secret-key-change-in-production"` — forgeable tokens in any non-configured deployment |
| C2 | `form/repo/postgres.go:260-262` | `fmt.Sprintf(" LIMIT %d", opts.Limit)` — SQL injection via LIMIT/OFFSET interpolation |
| C3 | `webhook/service.go:49,107` | Plaintext `signing_secret` stored in database — every webhook row leaks the full secret |

### HIGH (10)

| # | File:Line | Issue |
|---|-----------|-------|
| H1 | `tracking/repo/postgres.go:176-178` | Same SQL injection pattern: `fmt.Sprintf(" LIMIT %d", opts.Limit)` |
| H2 | `webhook/repo/postgres.go:149` | `active` column handling missing arg — wrong-number-arguments error at runtime |
| H3 | `database/migration.go:17`, `migration_users_runner.go:22-31` | Migrations run as separate `Exec` calls — no transaction, no version tracking |
| H4 | `webhook_worker.go:107` | `d.Attempts++` — unsynchronized mutation of shared struct field |
| H5 | `webhook_queue.go:76-83` | Missing `FOR UPDATE SKIP LOCKED` — duplicate webhook deliveries under concurrent workers |
| H6 | `database/connection.go:38` | Hardcoded `poolConfig.MaxConns = 4` — too small for production, no idle/timeout config |
| H7 | `webhook/service.go:208-225` | `ToResponse` correctly omits `SigningSecret` but `UpdateEndpoint` still stores plaintext in domain model |
| H8 | `auth/service.go:205-207` | `ValidateAPIKey` is a no-op stub — all API key auth rejected |
| H9 | `cookie/manager.go:32` | Default cookie secret `"change-me-in-production"` — forgeable if config is empty |
| H10 | `bootstrap/admin.go:49-51` | Duplicate error check — unreachable dead code (copy-paste bug) |

### MEDIUM (8)

| # | File:Line | Issue |
|---|-----------|-------|
| M1 | `email_worker.go:28` | 100ms polling interval — 10 queries/second per worker, excessive load |
| M2 | Pervasive | `context.Background()` instead of accepting context — no cancellation, no timeouts |
| M3 | `context/repo/postgres.go:91` | Error detail leak: `err_detail=%v` in error string |
| M4 | `email_worker.go:82,98-106` | Double-parsing of `to_addresses` array — potential inconsistency |
| M5 | `tracking/repo/postgres.go:300,275` | Missing `rows.Err()` checks |
| M6 | Pervasive | No `schema_migrations` table — migrations rely on `IF NOT EXISTS` only |
| M7 | `webhook_worker.go:32` | Fixed 30s HTTP client timeout — no per-request context timeout |
| M8 | `webhook_worker.go:68-72` | Batch deliveries sent sequentially — slow webhook blocks all others |

### LOW (5)

- `bootstrap/admin.go:38` — Default `admin@automata.local` printed to stdout
- Missing tests for most repository layers
- Various minor code quality issues

---

## 5. Shared Packages & Entry Points (`pkg/`, `cmd/`)

### CRITICAL (7)

| # | File:Line | Issue |
|---|-----------|-------|
| C1 | `pkg/snippet/generator.go:34,61` | **XSS** — `contextID` and `apiHost`/`serverHost` interpolated unescaped into JavaScript |
| C2 | `pkg/import/csv_importer.go:83` | **CSV injection** — raw cell values starting with `=`, `+`, `-`, `@` become Excel formulas |
| C3 | `pkg/import/csv_importer.go:83` | No value length limits — DoS via massive CSV cells |
| C4 | `cmd/automata/main.go:48` | `defer database.Close(pool)` — pool closed too early, in-flight requests get closed pool error |
| C5 | `cmd/automata/main.go:84` | `Scan()` error on health check ignored — masks startup infrastructure failures |
| C6 | `cmd/bff/main.go:142-146` | **Zero graceful shutdown** — SIGTERM kills process immediately |
| C7 | `cmd/bff/main.go:42` | Cookie secret `"change-me-in-production"` — trivially forgeable |

### HIGH (4)

| # | File:Line | Issue |
|---|-----------|-------|
| H1 | `cmd/automata/main.go:193-201` | Sequential shutdown with same context — if admin server takes 10s, tracking server waits |
| H2 | `cmd/automata/main.go:101` | `List(0, 1000)` — only first 1000 contexts get email template seeding |
| H3 | `cmd/bff/main.go:142` | No TLS/HTTPS — public-facing endpoint (8082) in plaintext |
| H4 | `cmd/bff/main.go:84-89` | Migration errors silently swallowed — app starts broken with no indication |

### MEDIUM (6)

| # | File:Line | Issue |
|---|-----------|-------|
| M1 | `pkg/validation/validator.go:99-100` | `bool: false` treated as non-empty — checkbox fields may not behave as expected |
| M2 | `pkg/validation/validator.go:97-98` | `float64(0)` treated as empty — zero is a valid number in many forms |
| M3 | `pkg/export/csv_exporter.go:22-23` | No UTF-8 BOM — non-ASCII content ambiguous for Excel/importers |
| M4 | `pkg/export/csv_exporter.go:22` | Entire CSV buffered in memory — no streaming for large exports |
| M5 | `cmd/automata/main.go:96-98` | Bootstrap error ignored — no admin user may be created |
| M6 | `cmd/automata/main.go:33` vs `cmd/bff/main.go:28` | Config file loading mechanism inconsistent between services |

### LOW (8)

| # | File:Line | Issue |
|---|-----------|-------|
| L1 | `pkg/validation/validator.go:62` | Regex compiled on every `Validate()` call — wasted CPU under load |
| L2 | `pkg/validation/validator.go:35-38` | Type coercion: `bool: false` → `"false"` (5-char string) — may pass length validation unexpectedly |
| L3 | `pkg/export/csv_exporter.go:27` | No field name sanitization — CSV formula injection in headers |
| L4 | `pkg/snippet/generator.go` | No CSP nonce/integrity support — inline `<script>` blocked by strict CSP |
| L5 | `pkg/snippet/generator.go` | `Math.random()` for visitor IDs — not cryptographically secure |
| L6 | `pkg/import/csv_importer.go:54-58` | Header validation logic is effectively a no-op |
| L7 | `pkg/import/csv_importer.go:77` | Row numbering is sequential, not actual CSV line numbers |
| L8 | `pkg/import/csv_importer.go:151-152` | `var _ = errors.New` — hacky unused import silencer |

---

## Cross-Cutting Architecture Issues

### 1. Hexagonal Architecture Violations
**Severity: HIGH** — 14+ instances of framework coupling in domain layer.

The domain layer should be pure business logic with zero framework dependencies. However, request/response DTOs in `banner/`, `email/`, and `webhook/` packages all contain Gin `binding:` tags. These DTOs should live in `internal/adapter/` where the HTTP framework owns the contract.

**Fix:** Move all request DTOs with `binding:` tags from `internal/domain/` to `internal/adapter/api/handler/`.

### 2. Zero Transaction Usage
**Severity: CRITICAL** — 4+ multi-step operations without transactions.

- Contact creation + activity logging
- Contact merge + activity logging  
- Form import (multiple rows)
- Tracking event + visitor upsert

All are separate repository calls with no transaction boundary. In a multi-tenant PostgreSQL backend, this risks data inconsistency.

### 3. Error Handling Patterns
**Severity: HIGH** — 80+ instances of `err.Error()` leaked to clients + 4 swallowed errors.

Internal database errors, stack traces, and implementation details are embedded in HTTP responses. Four locations create errors and then discard them (`_ = fmt.Errorf(...)`).

### 4. Test Coverage Gaps
**Severity: HIGH** — 0 test files in `usecase/`, 6 domain packages with no tests.

| Package | Test Files | Coverage |
|---------|-----------|----------|
| `internal/usecase/*` | 0 | 0% |
| `internal/domain/banner` | 0 | 0% |
| `internal/domain/email` | 0 | 0% |
| `internal/domain/tracking` | 0 | 0% |
| `internal/domain/webhook` | 0 | 0% |
| `internal/domain/webui` | 0 | 0% |
| `internal/domain/config` | 0 | 0% |
| `internal/infrastructure/*` | 5 of 15 | ~33% (shallow) |
| `internal/adapter/api/*` | 1 of 10 | ~10% |
| `pkg/*` | 0 of 4 | 0% |

### 5. Configuration Hardening
**Severity: CRITICAL** — 5+ hardcoded secret key fallbacks.

| Key | Locations |
|-----|-----------|
| JWT secret | `auth.go:34`, `auth_handler.go:60`, `service.go:45` |
| Cookie secret | `server.go:53`, `webui/server.go:16`, `manager.go:32`, `bff/main.go:42` |
| Admin email | `bootstrap/admin.go:38` |

All use development defaults as production fallbacks.

---

## Recommendations by Priority

### Phase 1: Security Emergency (This Sprint)
1. Remove all hardcoded secret key fallbacks — panic on startup if missing
2. Fix XSS in `pkg/snippet/generator.go` — escape all interpolated JS values
3. Fix CSV injection in `pkg/import/csv_importer.go` — sanitize formula characters
4. Fix WebUI proxy auth — use proper JWT instead of raw UUID
5. Add panic/recovery middleware to all Gin engines
6. Validate subdomain in banner/tracking context resolvers against database

### Phase 2: Data Integrity (Next Sprint)
7. Add database transactions to all multi-step use cases
8. Fix SQL injection via LIMIT/OFFSET string interpolation
9. Stop storing plaintext signing secrets — hash-only storage
10. Fix `UpdateForm` zero-value overwrite bug
11. Add `Validate()` methods to all domain entities missing them

### Phase 3: Architecture Hygiene (2-3 Sprints)
12. Move Gin DTOs out of domain layer
13. Add test coverage to `usecase/` and uncovered domain packages
14. Implement graceful shutdown on BFF server
15. Split wide repository interfaces (banner, tracking)
16. Deduplicate `DateRange` type and context resolvers
17. Add rate limiting and CORS middleware
18. Add authentication to MCP tools

### Phase 4: Hardening (Backlog)
19. Add role-based access control to admin endpoints
20. Standardize pagination, status codes, error formats across handlers
21. Add CSP nonce support to snippet generator
22. Add schema_migrations table for versioned migrations
23. Replace polling queue workers with pg_notify
24. Add TLS support to BFF

---

## File Index (All Reviewed)

### Domain Layer (26 files)
- `internal/domain/auth/auth.go`, `auth_test.go`
- `internal/domain/banner/banner.go`, `repository.go`
- `internal/domain/config/config.go`
- `internal/domain/contact/activity.go`, `contact.go`, `contact_test.go`, `repository.go`, `tag.go`
- `internal/domain/context/context.go`, `context_test.go`, `repository.go`, `user.go`, `user_repository.go`
- `internal/domain/email/email.go`, `repository.go`
- `internal/domain/form/form.go`, `form_test.go`, `repository.go`, `submission.go`
- `internal/domain/tracking/analytics.go`, `event.go`, `list_visitors_opts.go`, `repository.go`, `visitor.go`
- `internal/domain/webhook/repository.go`, `webhook_endpoint.go`
- `internal/domain/webui/repository.go`, `session.go`, `user_repository.go`

### Use Case Layer (27 files)
- `internal/usecase/banner/banner.go`
- `internal/usecase/contact/create_contact.go`, `delete_contact.go`, `export_contacts.go`, `get_activity.go`, `get_contact.go`, `import_contacts.go`, `list_contacts.go`, `merge_contacts.go`, `segment_contacts.go`, `tags.go`, `update_contact.go`
- `internal/usecase/email/email.go`
- `internal/usecase/form/create_form.go`, `delete_form.go`, `get_form.go`, `get_submission.go`, `list_forms.go`, `list_submissions.go`, `submit_form.go`, `update_form.go`
- `internal/usecase/tracking/get_dashboard.go`, `get_events.go`, `get_visitor_detail.go`, `get_visitor_events.go`, `list_visitors.go`, `track_event.go`

### API Adapter Layer (28 files)
- `internal/adapter/api/context_middleware.go`, `context_middleware_test.go`, `server.go`
- `internal/adapter/api/handler/auth_handler.go`, `banner_handler.go`, `config_handler.go`, `contact_handler.go`, `context_handler.go`, `context_user_handler.go`, `email_handler.go`, `form_handler.go`, `tracking_handler.go`, `user_handler.go`, `webhook_handler.go`
- `internal/adapter/api/middleware/auth.go`
- `internal/adapter/banner/context_resolver.go`, `server.go`, `serving_handler.go`, `snippet_handler.go`
- `internal/adapter/mcp/contacts_tools.go`, `forms_tools.go`, `server.go`
- `internal/adapter/proxy/proxy.go`, `proxy_test.go`
- `internal/adapter/tracking/context_resolver.go`, `server.go`, `snippet_handler.go`, `track_handler.go`
- `internal/adapter/webui/handler/auth.go`, `frontend.go`, `proxy.go`, `server.go`

### Infrastructure Layer (31+ files)
- `internal/infrastructure/auth/errors.go`, `service.go`, `service_test.go`, `repo/api_key_postgres.go`
- `internal/infrastructure/banner/repo/postgres.go`
- `internal/infrastructure/bootstrap/admin.go`
- `internal/infrastructure/config/config.go`, `loader.go`, `repo/postgres.go`
- `internal/infrastructure/contact/migration.go`, `migration_test.go`, `repo/postgres.go`
- `internal/infrastructure/context/repo/postgres.go`, `postgres_test.go`, `user_postgres.go`
- `internal/infrastructure/database/connection.go`, `migration.go`, `migration_test.go`, `migration_users_runner.go`
- `internal/infrastructure/email/repo/migration.go`, `postgres.go`
- `internal/infrastructure/form/repo/postgres.go`
- `internal/infrastructure/queue/email_queue.go`, `email_worker.go`, `webhook_queue.go`, `webhook_worker.go`
- `internal/infrastructure/tracking/repo/postgres.go`
- `internal/infrastructure/webhook/repo/migration.go`, `postgres.go`, `service.go`
- `internal/infrastructure/webui/cookie/manager.go`, `manager_test.go`

### Shared & Entry Points (6 files)
- `pkg/validation/validator.go`
- `pkg/snippet/generator.go`
- `pkg/export/csv_exporter.go`
- `pkg/import/csv_importer.go`
- `cmd/automata/main.go`
- `cmd/bff/main.go`

---

*Review performed by automated parallel subagent analysis across 5 architectural domains.*
