# Settings Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A single admin-only Settings page (`/settings`) with four tabs — Users, API Keys, Settings (global key-value, e.g. SMTP), Websites — replacing the standalone Websites and API Keys pages, with SMTP parameters live-editable from the DB.

**Architecture:** New `global_settings` key-value table (domain `settings` + postgres repo, standard per-repo migration pattern) exposed via `GET/PUT/DELETE /admin/settings`. The SMTP mailer gains a `ConfigProvider` seam: config is resolved per send (DB values override config.yaml). Frontend: `SettingsView.vue` + four panel components under `webui/src/components/settings/`; old standalone views deleted with router redirects.

**Tech Stack:** Go (gin, pgx/v5, Ginkgo cicd), Vue 3 + Vite + TypeScript + Tailwind.

**Spec:** Approved design from the 2026-10-17 brainstorming session (global key-value store; SMTP live-from-DB with yaml fallback; full user CRUD + roles; merge Websites/API Keys into Settings tabs with redirects).

## Global Constraints

- Hexagonal layout: domain interfaces in `internal/domain/<pkg>/`, postgres repos in `internal/infrastructure/<pkg>/repo/`, HTTP handlers in `internal/adapter/api/handler/`.
- Per-repo migration pattern: `//go:embed migration.sql` + `RunMigrations(pool *pgxpool.Pool) error`, idempotent SQL (`IF NOT EXISTS`). Every new repo's `RunMigrations` MUST be registered in three places: `cmd/automata/main.go` (after `database.RunUserContextMigrations`, ~line 89), `cicd/production_migrations_test.go` (`runProductionMigrations`), and `cicd/support/testdb.go` (`migrationDirs` list).
- Admin routes live in the `admin` group in `internal/adapter/api/server.go` (has `middleware.AuthMiddleware`); mount new routes there.
- Domain structs have NO json tags — handlers must emit explicit snake_case DTO shapes, never marshal domain structs directly.
- UI: reuse `DataTable`, `Modal`, `Card`, `Button`, `Input`, `ConfirmDialog` + `useConfirm` composable. The real UI gate is `vite build` (project does NOT pass clean `vue-tsc --noEmit`; pre-existing). UI commits must include rebuilt `web/` assets.
- cicd: Ginkgo specs in `cicd/` using shared `pool`/`ctx`; HTTP specs build a bare `gin.New()` engine with the handler directly (no auth middleware) — see `cicd/context_http_test.go`. Baseline: **329 passed | 10 failed of 339** (all 10 pre-existing banner/campaign/placement failures in `banner_campaign_http_test.go`).
- WIP files must NOT be touched or committed: `AGENTS.md`, `automata` binary, `helm/automata/values.yaml`, `docs/phase1.md`, `.playwright-mcp/`, and cicd files `config_handler_test.go`, `contact_form_integration_test.go`, `email_queue_test.go`, `global_login_test.go`, `snippet_test.go`, `tracking_server_test.go`, `webhook_queue_test.go`.
- Deploy from committed HEAD via isolated worktree `/tmp/opencode/automata-deploy` (never the working tree).
- No image-based verification: Playwright DOM/accessibility text only.

## Review Focus

1. `PUT /admin/settings` with an empty/whitespace key → 400 (not a silently-created `""` key); empty **value** is allowed (clears the override so yaml fallback applies).
2. `smtp.port` set to a non-numeric DB value → `MergeConfig` ignores it and keeps the base port (never produces port 0, which means "unset" and would silently switch to 587 at boot).
3. `smtp.starttls` = `"false"` in DB must disable TLS even when yaml says true; empty DB value falls back to yaml behavior (default true).
4. `GET /admin/users` must not leak `password_hash` and must use snake_case keys (pre-existing leak; fixed in Task 3).
5. API key list must keep returning only the 8-char prefix (`key` field), never the full hash.
6. `DELETE /admin/settings/:key` for a missing key → 404 (UI delete must not 500).

---

### Task 1: Global settings backend (domain, repo, migration, API)

**Files:**
- Create: `internal/domain/settings/settings.go`
- Create: `internal/infrastructure/settings/repo/postgres.go`
- Create: `internal/infrastructure/settings/repo/migration.sql`
- Create: `internal/adapter/api/handler/settings_handler.go`
- Create: `cicd/settings_test.go`
- Modify: `internal/adapter/api/server.go` (admin group, after the config routes block ~line 101)
- Modify: `cmd/automata/main.go` (migration registration, after line 89)
- Modify: `cicd/production_migrations_test.go` (`runProductionMigrations` + table check)
- Modify: `cicd/support/testdb.go` (`migrationDirs`)

**Interfaces:**
- Produces:
  - `settings.Repository` (domain): `List() (map[string]string, error)`, `Get(key string) (string, error)` (returns `""`, nil when missing), `Upsert(key, value string) error`, `Delete(key string) error` (returns `settings.ErrNotFound` when missing).
  - `settingsrepo.New(pool *pgxpool.Pool) settings.Repository`, `settingsrepo.RunMigrations(pool *pgxpool.Pool) error` (import path `github.com/hekemen/automata/internal/infrastructure/settings/repo`).
  - `handler.NewSettingsHandler(repo settings.Repository) *SettingsHandler` with `List`, `Upsert`, `Delete` gin handlers.
  - HTTP: `GET /api/admin/settings` → `{"settings": {"k": "v"}}`; `PUT /api/admin/settings` body `{"key": "...", "value": "..."}` → 200 `{"key","value"}` (400 on empty key); `DELETE /api/admin/settings/:key` → 204 (404 when missing).

- [ ] **Step 1: Write the failing cicd spec**

Create `cicd/settings_test.go` with a repo describe block and an HTTP describe block. Follow the `cicd/context_http_test.go` engine pattern (bare `gin.New()`, no auth middleware) and `cicd/shadow_user_test.go` for pool usage.

```go
package cicd_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	bhandler "github.com/hekemen/automata/internal/adapter/api/handler"
	settingsrepo "github.com/hekemen/automata/internal/infrastructure/settings/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupSettingsEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	h := bhandler.NewSettingsHandler(settingsrepo.New(pool))
	admin := engine.Group("/api/admin")
	admin.GET("/settings", h.List)
	admin.PUT("/settings", h.Upsert)
	admin.DELETE("/settings/:key", h.Delete)
	return engine
}

var _ = Describe("Global Settings", func() {
	var (
		w      *httptest.ResponseRecorder
		engine *gin.Engine
		repo   = settingsrepo.New(pool)
	)

	BeforeEach(func() {
		w = httptest.NewRecorder()
		engine = setupSettingsEngine()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM global_settings")
	})

	Describe("repo", func() {
		It("upserts, gets, lists, and deletes", func() {
			Expect(repo.Upsert("smtp.host", "relay.example.com")).To(Succeed())
			v, err := repo.Get("smtp.host")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).To(Equal("relay.example.com"))

			Expect(repo.Upsert("smtp.host", "other.example.com")).To(Succeed())
			v, _ = repo.Get("smtp.host")
			Expect(v).To(Equal("other.example.com"))

			all, err := repo.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(all["smtp.host"]).To(Equal("other.example.com"))

			Expect(repo.Delete("smtp.host")).To(Succeed())
			_, err = repo.Get("smtp.host")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.Delete("smtp.host")).ToNot(Succeed()) // missing key
		})
	})

	Describe("HTTP", func() {
		It("upserts and lists settings", func() {
			req := httptest.NewRequest("PUT", "/api/admin/settings",
				bytes.NewBufferString(`{"key":"smtp.from","value":"noreply@automata.local"}`))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))

			w.Reset()
			req = httptest.NewRequest("GET", "/api/admin/settings", nil)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring(`"smtp.from":"noreply@automata.local"`))
		})

		It("rejects an empty key with 400", func() {
			req := httptest.NewRequest("PUT", "/api/admin/settings",
				bytes.NewBufferString(`{"key":"  ","value":"x"}`))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("deletes a setting and 404s when missing", func() {
			_, _ = repo.Upsert("k1", "v1")
			req := httptest.NewRequest("DELETE", "/api/admin/settings/k1", nil)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusNoContent))

			w.Reset()
			req = httptest.NewRequest("DELETE", "/api/admin/settings/nope", nil)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
```

- [ ] **Step 2: Run the spec to verify it fails**

Run: `go test ./cicd/ -run TestCICD -args -ginkgo.focus="Global Settings"`
Expected: compile failure (packages/symbols do not exist yet).

- [ ] **Step 3: Implement domain, repo, migration**

`internal/domain/settings/settings.go`:

```go
package settings

import "errors"

var ErrNotFound = errors.New("setting not found")

// Repository is the persistence port for global key-value settings.
type Repository interface {
	List() (map[string]string, error)
	Get(key string) (string, error)
	Upsert(key, value string) error
	Delete(key string) error
}
```

`internal/infrastructure/settings/repo/migration.sql`:

```sql
CREATE TABLE IF NOT EXISTS global_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

`internal/infrastructure/settings/repo/postgres.go`: mirror `internal/infrastructure/tracking/repo/postgres.go` — package `repo`, `//go:embed migration.sql`, `RunMigrations`, `New(pool) settings.Repository`. `Upsert` = `INSERT INTO global_settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`. `Delete` checks `RowsAffected` and returns `settings.ErrNotFound` when 0. `Get` returns `("", nil)` when no row (use `sql.ErrNoRows`).

- [ ] **Step 4: Implement the handler and wire routes**

`internal/adapter/api/handler/settings_handler.go`: struct `SettingsHandler{repo settings.Repository}`, `NewSettingsHandler`. `List` → `c.JSON(200, gin.H{"settings": m})`. `Upsert` binds `{"key","value"}`; `strings.TrimSpace(key) == ""` → 400 `{"error":"key is required"}`; upsert → 200 `{"key","value"}`. `Delete` → 204, or 404 when `errors.Is(err, settings.ErrNotFound)`.

In `internal/adapter/api/server.go`, inside the `admin` group after the config routes:

```go
settingsRepo := settingsrepo.New(pool)
settingsHandler := handler.NewSettingsHandler(settingsRepo)
admin.GET("/settings", settingsHandler.List)
admin.PUT("/settings", settingsHandler.Upsert)
admin.DELETE("/settings/:key", settingsHandler.Delete)
```

Register the migration in `cmd/automata/main.go` after the `database.RunUserContextMigrations(pool)` block (same error style: `log.Fatal().Err(err).Msg("settings migration failed")`), in `cicd/production_migrations_test.go` (`settings_repo.RunMigrations(pool)` inside `runProductionMigrations` plus `global_settings` added to the table-existence check), and `"internal/infrastructure/settings/repo"` appended to `migrationDirs` in `cicd/support/testdb.go`.

- [ ] **Step 5: Run the spec to verify it passes**

Run: `go test ./cicd/ -run TestCICD -args -ginkgo.focus="Global Settings"`
Expected: PASS (all 4 specs).

- [ ] **Step 6: Commit**

```bash
git add internal/domain/settings internal/infrastructure/settings internal/adapter/api/handler/settings_handler.go internal/adapter/api/server.go cmd/automata/main.go cicd/settings_test.go cicd/production_migrations_test.go cicd/support/testdb.go
git commit -m "feat(settings): global key-value settings store + admin API"
```

---

### Task 2: SMTP live-from-DB (ConfigProvider + MergeConfig)

**Files:**
- Modify: `internal/infrastructure/email/smtp/smtp.go`
- Modify: `cmd/automata/main.go` (mailer construction, ~lines 195–216)
- Create: `cicd/smtp_settings_test.go`

**Interfaces:**
- Consumes: `settingsrepo.New(pool)` + `settings.Repository.List()` (Task 1).
- Produces:
  - `smtp.ConfigProvider` = `func() Config`.
  - `smtp.MergeConfig(base Config, overrides map[string]string) Config` — pure function; applies non-empty `smtp.host`, `smtp.port`, `smtp.username`, `smtp.password`, `smtp.from`, `smtp.starttls` overrides onto `base`; non-numeric `smtp.port` is ignored; `smtp.starttls` uses `v != "false"`.
  - `smtp.NewMailerWithProvider(provider ConfigProvider, fallback Config, dialerFor func(Config) Dialer) *SMTPMailer` — `Send` resolves `cfg = provider()` and `dialer = dialerFor(cfg)` per call; `fallback` is stored as `m.cfg` so `IsConfigured()` (startup warning) reflects the yaml base. Existing `NewMailer` / `NewMailerWithDialer` behavior unchanged.

- [ ] **Step 1: Write the failing cicd spec**

Create `cicd/smtp_settings_test.go`. Reuse the `dialerAdapter` pattern from `cicd/email_delivery_test.go` (wraps `support.SMTPDialer` to satisfy `smtp.Dialer`).

```go
package cicd_test

import (
	"strings"

	"github.com/hekemen/automata/cicd/support"
	mailsmtp "github.com/hekemen/automata/internal/infrastructure/email/smtp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SMTP settings merge", func() {
	base := mailsmtp.Config{Host: "yaml.host", Port: 587, From: "yaml@x", StartTLS: true}

	Describe("MergeConfig", func() {
		It("applies non-empty overrides", func() {
			got := mailsmtp.MergeConfig(base, map[string]string{
				"smtp.host": "db.host", "smtp.port": "2525", "smtp.from": "db@x",
			})
			Expect(got.Host).To(Equal("db.host"))
			Expect(got.Port).To(Equal(2525))
			Expect(got.From).To(Equal("db@x"))
			Expect(got.StartTLS).To(BeTrue()) // untouched
		})

		It("ignores a non-numeric port", func() {
			got := mailsmtp.MergeConfig(base, map[string]string{"smtp.port": "abc"})
			Expect(got.Port).To(Equal(587))
		})

		It("honors starttls=false and keeps base when overrides are empty", func() {
			got := mailsmtp.MergeConfig(base, map[string]string{"smtp.starttls": "false"})
			Expect(got.StartTLS).To(BeFalse())
			Expect(mailsmtp.MergeConfig(base, map[string]string{})).To(Equal(base))
		})
	})

	Describe("provider mailer", func() {
		It("resolves config from the provider at send time", func() {
			mock := support.NewMockSMTP()
			mock.Start()
			defer mock.Stop()
			var got mailsmtp.Config
			mailer := mailsmtp.NewMailerWithProvider(
				func() mailsmtp.Config { return mailsmtp.Config{Host: "db", From: "db@x"} },
				base,
				func(cfg mailsmtp.Config) mailsmtp.Dialer {
					got = cfg
					return &dialerAdapter{d: support.NewSMTPDialer(mock)}
				},
			)
			Expect(mailer.Send([]string{"a@b.c"}, "s", "b", "")).To(Succeed())
			Expect(got.From).To(Equal("db@x"))
			Expect(got.Host).To(Equal("db"))
		})

		It("treats an unconfigured provider result as not configured", func() {
			mailer := mailsmtp.NewMailerWithProvider(
				func() mailsmtp.Config { return mailsmtp.Config{} },
				base,
				func(cfg mailsmtp.Config) mailsmtp.Dialer { return nil },
			)
			err := mailer.Send([]string{"a@b.c"}, "s", "b", "")
			Expect(err).To(MatchError(ContainSubstring("not configured")))
		})
	})
})
```

- [ ] **Step 2: Run the spec to verify it fails**

Run: `go test ./cicd/ -run TestCICD -args -ginkgo.focus="SMTP settings merge"`
Expected: compile failure (`MergeConfig`, `NewMailerWithProvider` undefined).

- [ ] **Step 3: Implement the smtp package changes**

In `smtp.go`: add `type ConfigProvider func() Config` and `MergeConfig` (iterate the six `smtp.*` keys; for port use `strconv.Atoi` and skip on error). Add `provider ConfigProvider` and `dialerFor func(Config) Dialer` fields to `SMTPMailer`; add `NewMailerWithProvider` (sets `cfg: fallback`, `provider`, `dialerFor`). In `Send`, at the top resolve: `cfg, dialer := m.cfg, m.dialer; if m.provider != nil { cfg = m.provider(); dialer = m.dialerFor(cfg) }`, then use local `cfg`/`dialer` throughout (replace `m.cfg`/`m.dialer` references in `Send` only — `IsConfigured` keeps using `m.cfg`).

- [ ] **Step 4: Wire the provider in main.go**

Replace `mailer := mailsmtp.NewMailer(smtpCfg)` with:

```go
mailer := mailsmtp.NewMailerWithProvider(
	func() mailsmtp.Config {
		m, _ := settingsRepo.List()
		return mailsmtp.MergeConfig(smtpCfg, m)
	},
	smtpCfg,
	mailsmtp.DefaultDialer,
)
```

(`settingsRepo` is already created in `main.go` by Task 1's wiring — if it is scoped inside a block, lift the `settingsrepo.New(pool)` call to where the mailer is built.)

- [ ] **Step 5: Run the spec to verify it passes**

Run: `go test ./cicd/ -run TestCICD -args -ginkgo.focus="SMTP settings merge"`
Expected: PASS (5 specs). Also run `go test ./cicd/ -run TestCICD -args -ginkgo.focus="Email Delivery Pipeline"` to confirm the existing mailer path is unaffected.

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/email/smtp/smtp.go cmd/automata/main.go cicd/smtp_settings_test.go
git commit -m "feat(smtp): resolve SMTP config from global settings at send time"
```

---

### Task 3: User list DTO (stop leaking password hash)

**Files:**
- Modify: `internal/adapter/api/handler/user_handler.go` (`List`, ~line 33)
- Modify: `cicd/user_handler_test.go` (add one spec to the existing `GET /admin/users` describe)

**Interfaces:**
- Produces: `GET /api/admin/users` → `{"users": [{"id","email","is_admin","created_at"}]}` (same shape as the existing `Get` handler; `password_hash` never appears).

- [ ] **Step 1: Write the failing spec**

In `cicd/user_handler_test.go`, inside the existing `GET /admin/users` describe, add:

```go
It("returns snake_case DTOs without password hashes", func() {
	userID := support.NewTestUUID()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
		VALUES ($1, 'dto@test.local', '$2a$10$secretsecretsecretsecretsecretse', true, NOW(), NOW())`,
		userID)
	Expect(err).NotTo(HaveOccurred())

	req := httptest.NewRequest("GET", "/admin/users", nil)
	engine.ServeHTTP(w, req)
	Expect(w.Code).To(Equal(http.StatusOK))
	Expect(w.Body.String()).To(ContainSubstring(`"email":"dto@test.local"`))
	Expect(w.Body.String()).To(ContainSubstring(`"is_admin":true`))
	Expect(w.Body.String()).ToNot(ContainSubstring("secretsecret"))
	Expect(w.Body.String()).ToNot(ContainSubstring("PasswordHash"))
})
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./cicd/ -run TestCICD -args -ginkgo.focus="User CRUD"`
Expected: FAIL (body contains `PasswordHash` / capitalized keys).

- [ ] **Step 3: Implement the DTO in `List`**

Add to `user_handler.go`:

```go
type userDTO struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}
```

In `List`, map `[]*context.User` → `[]userDTO` and respond `gin.H{"users": dtos}`. (Import `time` if not already imported.)

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./cicd/ -run TestCICD -args -ginkgo.focus="User CRUD"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/api/handler/user_handler.go cicd/user_handler_test.go
git commit -m "fix(users): list returns sanitized snake_case DTO (no password hash)"
```

---

### Task 4: Settings page shell + Settings (key-value) tab

**Files:**
- Create: `webui/src/views/SettingsView.vue`
- Create: `webui/src/components/settings/SettingsPanel.vue`
- Modify: `webui/src/api/client.ts` (append settings functions)
- Modify: `webui/src/router/index.ts`
- Modify: `webui/src/components/AppSidebar.vue`
- Rebuild: `web/` assets

**Interfaces:**
- Consumes: `GET/PUT/DELETE /admin/settings` (Task 1).
- Produces:
  - Client: `listSettings(): Promise<Record<string, string>>` (GET `/admin/settings` → `data.settings`), `upsertSetting(key: string, value: string): Promise<void>` (PUT), `deleteSetting(key: string): Promise<void>` (DELETE `/admin/settings/${key}`).
  - Route: `/settings` → `SettingsView` (`meta: { requiresAdmin: true }`).
  - `SettingsView` tab bar driven by an array `tabs: { id, label }[]` and `route.query.tab` (validated against the array, default `users`… until Task 6 exists, default `settings`); tab click does `router.push({ query: { tab: id } })`. Panels live in `webui/src/components/settings/`.

- [ ] **Step 1: Add the client functions**

Append to `webui/src/api/client.ts` (same style as the context functions):

```ts
// Global settings endpoints (admin only)
export async function listSettings(): Promise<Record<string, string>> {
  const { data } = await client.get('/admin/settings')
  return data.settings
}

export async function upsertSetting(key: string, value: string): Promise<void> {
  await client.put('/admin/settings', { key, value })
}

export async function deleteSetting(key: string): Promise<void> {
  await client.delete(`/admin/settings/${key}`)
}
```

- [ ] **Step 2: Create `SettingsView.vue`**

`<script setup lang="ts">`: `import { computed } from 'vue'`, `import { useRoute, useRouter } from 'vue-router'`, `import SettingsPanel from '@/components/settings/SettingsPanel.vue'`. Tab array starts as `[{ id: 'settings', label: 'Settings' }]` (later tasks append `users`, `api-keys`, `websites`). `activeTab = computed(() => tabs.some(t => t.id === route.query.tab) ? route.query.tab : 'settings')`. Template: a Card with a horizontal tab bar (buttons, active style like `DashboardView.vue`'s tabs) and `<SettingsPanel v-if="activeTab === 'settings'" />`.

- [ ] **Step 3: Create `SettingsPanel.vue`**

Key-value editor: `DataTable` columns `key`, `value`, `actions` (Edit, Delete). "Add" button opens a `Modal` with `Input` for key + value; Edit pre-fills the same modal; Save calls `upsertSetting` (validate non-empty key client-side, show `formError` on failure); Delete uses `ConfirmDialog` via `useConfirm` then `deleteSetting`. Reload the list after every mutation. Empty state text when there are no settings.

- [ ] **Step 4: Wire router + sidebar**

`router/index.ts`: add inside the `AppLayout` children:

```ts
{
  path: 'settings',
  name: 'Settings',
  component: () => import('@/views/SettingsView.vue'),
  meta: { requiresAdmin: true },
},
```

`AppSidebar.vue`: add `{ name: 'Settings', path: '/settings', icon: Settings, adminOnly: true }` to `navItems` (import `Settings` from `lucide-vue-next`). Keep the existing Websites/API Keys entries for now (removed in Tasks 5 and 7).

- [ ] **Step 5: Build the UI and verify**

Run: `cd webui && npm run build`
Expected: build succeeds, `web/` assets updated.

- [ ] **Step 6: Commit**

```bash
git add webui/src/views/SettingsView.vue webui/src/components/settings/SettingsPanel.vue webui/src/api/client.ts webui/src/router/index.ts webui/src/components/AppSidebar.vue web/
git commit -m "feat(webui): settings page shell with key-value settings tab"
```

---

### Task 5: API Keys tab (extract view, real client calls)

**Files:**
- Create: `webui/src/components/settings/ApiKeysPanel.vue`
- Delete: `webui/src/views/ApiKeysView.vue`
- Modify: `webui/src/api/client.ts` (replace the three stub functions)
- Modify: `webui/src/types/api.ts` (`ApiKey` interface)
- Modify: `webui/src/views/SettingsView.vue` (add `api-keys` tab)
- Modify: `webui/src/router/index.ts` (redirect)
- Modify: `webui/src/components/AppSidebar.vue` (remove the API Keys entry)
- Rebuild: `web/` assets

**Interfaces:**
- Consumes: backend `GET /api/admin/api-keys` → `{"keys": [{id, name, key (8-char prefix + "..."), expires_at, created_at}]}`; `POST /api/admin/api-keys` body `{"name", "expires_in" (days, optional)}` → 201 `{id, name, key (plaintext), expires_at}`; `DELETE /api/admin/api-keys/:id` → 200.
- Produces:
  - `ApiKey` type: `{ id: string; name: string; key: string; expires_at: string | null; created_at: string }` (replaces the old `key_prefix`/`active` shape).
  - Client: `listApiKeys(): Promise<ApiKey[]>` (returns `data.keys`), `createApiKey(data: { name: string; expires_in?: number }): Promise<{ key: string }>` (returns `data`), `revokeApiKey(id: string): Promise<void>`.
  - Tab `api-keys` in `SettingsView`; route `api-keys` redirects to `/settings?tab=api-keys`.

- [ ] **Step 1: Replace the client stubs and fix the type**

In `client.ts` replace the three stubs with real implementations (see Produces). In `types/api.ts` replace the `ApiKey` interface with the new shape.

- [ ] **Step 2: Create `ApiKeysPanel.vue`**

Move the script + template from `ApiKeysView.vue` into `webui/src/components/settings/ApiKeysPanel.vue`. Changes: (a) the expiry `Input` becomes a number input for days bound to `keyExpiry` (number), sent as `expires_in` (omit when empty) — the old date input sent a field the backend ignores; (b) `createApiKey` now returns `{ key }` — keep the "copy key" flow; (c) drop `useRouter` (not used in the panel). Everything else (columns, confirm dialog, copy button) stays as-is.

- [ ] **Step 3: Delete the old view, wire tab + redirect**

Delete `webui/src/views/ApiKeysView.vue`. In `SettingsView.vue` add `{ id: 'api-keys', label: 'API Keys' }` to the tabs and `<ApiKeysPanel v-else-if="activeTab === 'api-keys'" />`. In `router/index.ts` replace the `api-keys` route with `{ path: 'api-keys', redirect: '/settings?tab=api-keys' }`.

- [ ] **Step 4: Build the UI and verify**

Run: `cd webui && npm run build`
Expected: build succeeds (no dangling import of `ApiKeysView`).

- [ ] **Step 5: Commit**

```bash
git add webui/src/components/settings/ApiKeysPanel.vue webui/src/views/ApiKeysView.vue webui/src/api/client.ts webui/src/types/api.ts webui/src/views/SettingsView.vue webui/src/router/index.ts web/
git commit -m "feat(webui): api keys tab in settings with real client calls"
```

---

### Task 6: Users tab (CRUD + website membership)

**Files:**
- Create: `webui/src/components/settings/UsersPanel.vue`
- Modify: `webui/src/api/client.ts` (user + membership functions)
- Modify: `webui/src/types/api.ts` (`User` extended, `ContextUser` added)
- Modify: `webui/src/views/SettingsView.vue` (add `users` tab, make it the default)
- Rebuild: `web/` assets

**Interfaces:**
- Consumes: `GET /api/admin/users` → `{"users": [{id, email, is_admin, created_at}]}` (Task 3); `POST /api/admin/users` `{"email","password","is_admin"}` → 201; `PUT /api/admin/users/:id` `{"email"?,"password"?,"is_admin"?}` → 200; `DELETE /api/admin/users/:id` → 204; `GET /api/admin/contexts/:id/users` → `{"members": [{ID, UserID, ContextID, Role, ...}]}` (capitalized keys — no json tags on `UserContext`); `POST /api/admin/contexts/:id/users` `{"user_id","role"}` → 201; `DELETE /api/admin/contexts/:id/users/:userId` → 204.
- Produces:
  - `User` type: `{ id: string; email: string; is_admin: boolean; created_at: string }`.
  - `ContextUser` type: `{ ID: string; UserID: string; ContextID: string; Role: string }`.
  - Client: `listUsers(): Promise<User[]>` (`data.users`), `createUser(data: { email: string; password: string; is_admin?: boolean }): Promise<User>`, `updateUser(id: string, data: { email?: string; password?: string; is_admin?: boolean }): Promise<User>`, `deleteUser(id: string): Promise<void>`, `listContextUsers(contextId: string): Promise<ContextUser[]>` (`data.members`), `assignContextUser(contextId: string, data: { user_id: string; role: string }): Promise<void>`, `removeContextUser(contextId: string, userId: string): Promise<void>`.
  - Tab `users` (default tab) in `SettingsView`.

- [ ] **Step 1: Add client functions + types**

Add the seven functions to `client.ts` (same axios style). Replace `User` in `types/api.ts` with the extended shape and add `ContextUser`.

- [ ] **Step 2: Create `UsersPanel.vue`**

Two cards:

1. **Users** — `DataTable` columns `email`, `is_admin` (Badge), `created_at`, `actions` (Edit, Delete). "Add user" `Modal`: email, password, is_admin checkbox → `createUser`. Edit `Modal`: email, new password (optional — omit empty), is_admin → `updateUser`. Delete via `ConfirmDialog` → `deleteUser`.
2. **Website memberships** — a `Select` to pick a website (load via existing `listContexts()`). For the selected website: member `DataTable` (`email` resolved by matching `UserID` against `listUsers()`, `Role`, remove action → `removeContextUser`) and an "Add member" row: user `Select` + role `Select` (owner/admin/member) → `assignContextUser`.

- [ ] **Step 3: Wire the tab**

In `SettingsView.vue` add `{ id: 'users', label: 'Users' }` as the first tab, change the default in `activeTab` to `'users'`, and render `<UsersPanel v-if="activeTab === 'users'" />` (adjust the `SettingsPanel` condition to `v-else-if`).

- [ ] **Step 4: Build the UI and verify**

Run: `cd webui && npm run build`
Expected: build succeeds.

- [ ] **Step 5: Commit**

```bash
git add webui/src/components/settings/UsersPanel.vue webui/src/api/client.ts webui/src/types/api.ts webui/src/views/SettingsView.vue web/
git commit -m "feat(webui): users tab with CRUD and website membership"
```

---

### Task 7: Websites tab (extract ContextsView)

**Files:**
- Create: `webui/src/components/settings/WebsitesPanel.vue`
- Delete: `webui/src/views/ContextsView.vue`
- Modify: `webui/src/views/SettingsView.vue` (add `websites` tab)
- Modify: `webui/src/router/index.ts` (redirect; keep `contexts/:id`)
- Modify: `webui/src/components/AppSidebar.vue` (remove the Websites entry)
- Rebuild: `web/` assets

**Interfaces:**
- Consumes: existing context client functions (`listContexts/createContext/updateContext/deleteContext`) and the `Context` type.
- Produces: tab `websites` in `SettingsView`; route `contexts` redirects to `/settings?tab=websites`; route `contexts/:id` (detail page) unchanged; sidebar no longer shows "Websites".

- [ ] **Step 1: Create `WebsitesPanel.vue`**

Move the script + template from `ContextsView.vue` into `webui/src/components/settings/WebsitesPanel.vue`. Keep the "View" action's `router.push({ name: 'ContextDetail', params: { id } })` (the detail route stays). No other changes.

- [ ] **Step 2: Delete the old view, wire tab + redirect + sidebar**

Delete `webui/src/views/ContextsView.vue`. In `SettingsView.vue` add `{ id: 'websites', label: 'Websites' }` and `<WebsitesPanel v-else-if="activeTab === 'websites'" />`. In `router/index.ts` replace the `contexts` route with `{ path: 'contexts', redirect: '/settings?tab=websites' }` — keep the `contexts/:id` route. In `AppSidebar.vue` remove the `Websites` nav item (the `Globe` import too, if unused).

- [ ] **Step 3: Build the UI and verify**

Run: `cd webui && npm run build`
Expected: build succeeds (no dangling import of `ContextsView`).

- [ ] **Step 4: Commit**

```bash
git add webui/src/components/settings/WebsitesPanel.vue webui/src/views/ContextsView.vue webui/src/views/SettingsView.vue webui/src/router/index.ts webui/src/components/AppSidebar.vue web/
git commit -m "feat(webui): websites tab in settings; retire standalone page"
```

---

### Task 8: Full verification + deploy

**Files:**
- Modify: `docs/superpowers/plans/2026-10-17-settings-page.md` (Execution Status section)

**Interfaces:**
- Consumes: all of Tasks 1–7.

- [ ] **Step 1: Backend gate**

Run: `go build ./...` — expect clean.
Run: `go vet ./...` — expect only the pre-existing `proxy_test.go:23` failure.

- [ ] **Step 2: Full cicd suite vs baseline**

Run: `go test ./cicd/ -run TestCICD`
Expected: **≥ 339 passed** (329 baseline + new specs) with **exactly the same 10 pre-existing failures** (banner/campaign/placement in `banner_campaign_http_test.go`). Any new failure is a regression — fix before deploying.

- [ ] **Step 3: UI gate**

Run: `cd webui && npm run build` — expect clean; confirm `web/` assets are committed (they are, per tasks 4–7).

- [ ] **Step 4: Deploy from committed HEAD**

Reset `/tmp/opencode/automata-deploy` to the new HEAD, build both binaries (`CGO_ENABLED=0 go build -o automata ./cmd/automata` and `-o bff ./cmd/bff`), `docker build -f /tmp/opencode/Dockerfile.deploy` tagging `latest` + the sha, push, `rollout restart` BOTH deployments (`automata-automata`, `automata-bff`) in ns `automata`.

- [ ] **Step 5: Live checks (text/DOM only)**

1. API: `POST /api/auth/login` (admin@automata.local / REDACTED) → token; `PUT /api/admin/settings` `{"key":"smtp.from","value":"deploy-check@automata.local"}` → 200; `GET /api/admin/settings` contains it; `DELETE` → 204.
2. UI via Playwright DOM text at `http://automata.home.arpa`: sidebar shows **Settings** and does NOT show "Websites" or "API Keys"; `/settings` renders the four tabs (Users, API Keys, Settings, Websites); Settings tab shows the `smtp.from` entry; `/api-keys` and `/contexts` land on the corresponding Settings tabs.
3. SMTP override check: `kubectl -n automata exec automata-postgresql-0 -c postgresql -- psql ... -c "SELECT * FROM global_settings"` shows the row; no restart needed (provider reads per send).

- [ ] **Step 6: Record execution status + commit**

Append an Execution Status section to this plan (commit range, cicd counts, live-check results) and commit:

```bash
git add docs/superpowers/plans/2026-10-17-settings-page.md
git commit -m "docs(plan): record settings page execution status"
```

## Execution Status

Completed 2026-10-17. Implementation commits `321fd23..28af411` (Tasks 1–7), this status commit closes Task 8.

- **Gates:** `go build ./...` clean; `go vet ./...` only pre-existing `proxy_test.go:23`; full cicd suite **339 passed / 10 failed of 349** — all 10 failures pre-existing in `banner_campaign_http_test.go` (banner/campaign/placement), matching baseline; `npm run build` clean.
- **Deploy:** built from committed HEAD `28af411` in isolated worktree; image `ghcr.io/hekemen/automata:latest` + `:28af411` (digest `sha256:431ed090d619b6c9e70af072cd014e0fb0bfdacad7c2706e494aefc8d5b078e2`); both deployments (`automata-automata`, `automata-bff`) rolled out, pods 1/1 Running.
- **Live checks (text/DOM only):** API — login → token, `PUT /api/admin/settings` `smtp.from` → 200, `GET` contains it, `DELETE` → 204; `global_settings` row present in Postgres (live-from-DB, no restart), test row removed after verification. UI (Playwright DOM, 15/15) — sidebar shows Settings, does NOT show Websites/API Keys; `/settings` renders all four tabs with Users default; Settings tab shows the `smtp.from` entry; `/api-keys` → `/settings?tab=api-keys`; `/contexts` → `/settings?tab=websites`.
- **Per-task reviews:** all seven task reviews Approved (see `.superpowers/sdd/2026-10-17-settings-page/progress.md`).