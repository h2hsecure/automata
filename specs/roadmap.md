# Automata — Project Roadmap

Automata is a self-hosted marketing automation tool for small-to-mid businesses. It helps users track website visitors, collect leads via forms, manage contacts, and display banners on their websites.

## Architecture

- Multi-tenant ("context"): one Automata instance manages multiple websites. Each context has its own contacts, forms, tracking data, banners, and configuration.
- Dual-server runtime (since 2026-09-19): a main API server (port 8080 — auth, admin API, SPA fallback) and a public tracking server (port 8081 — snippet, `/track`, form submission/rendering; no auth, no SPA). A third BFF server (port 8082, `cmd/bff`) fronts the Web UI and proxies `/api/*` with the `automata_session` cookie.
- Reverse proxy: routes tenant traffic via subdomain or path.
- AI MCP interface: full MCP tool server (`internal/adapter/mcp`, 52 tools covering contacts, forms, tracking, banners, campaigns, placements, email, webhooks — count pinned in `cicd/mcp_test.go`).
- Web UI: Vue 3 + Vite + TypeScript + Tailwind CSS (source in `webui/`, built assets served from `web/`).
- Platform-level users: `users` table with role-based `user_contexts` memberships; `context_users` retained for backward compatibility. Admin is bootstrapped on first startup.

## Feature Areas

### 1. Core Platform
Multi-tenant authentication, reverse proxy, shared infrastructure (logging, config, email queue). **Done.** Includes dual-token JWT auth with refresh flow and `jti` claims, admin bootstrap, and API key management.

### 2. Contact Management
Store and manage contacts collected via forms, tracking events, and manual entry. Deduplication, segmentation, import/export, activity timeline. **Done** (activity timeline added 2026-09-25).

### 3. Form Management
Form builder, form rendering via reverse proxy, submission handling, webhook and email notifications, spam protection. **Done.**

### 4. User Tracking
JavaScript snippet for website embedding, page view and event tracking, visitor identification, analytics dashboard data. **Done**, including admin visitor tracking (visitor list, detail, event timeline — 2026-09-25).

### 5. Banner Management
Banner CRUD, placement rules, A/B testing, impression and click tracking. **Done.**

### 6. Email System
Email queue, template management, and notification integration with forms and webhooks. **Done** (2026-09-25). SMTP delivery: **Done** (2026-10-14, [plan](../docs/superpowers/plans/2026-10-14-email-delivery.md)). Per-website email templates (standalone + form-integrated) with contact/form placeholders, IMAP inbox poller, email reach stats, and Email UI: **Done** (2026-10-19, [plan](../docs/superpowers/plans/2026-10-19-email-templates-inbox.md)).

### 7. Webhook Management
Per-context webhook CRUD, queue, delivery, and retry. **Done** (2026-09-25; delivery worker wired into the main API process and retry pickup fixed 2026-10-14, [plan](../docs/superpowers/plans/2026-10-14-webhook-delivery-wiring.md)).

### 8. Settings & Profile
Per-context settings (CORS, domain, display), user profile management, and password change. **Done** (2026-09-25).

### 9. Web UI
Vue 3 admin dashboard: login, contexts, contacts, forms, banners, tracking/visitors, settings, API keys. **Done** (phases 1–2, 2026-09-16 → 2026-09-17; BFF server added 2026-09-25).

### 10. October 2026 Releases
- Form website integration (forms rendered on the tenant website): **Done** (2026-10-15, [plan](../docs/superpowers/plans/2026-10-15-form-website-integration.md)).
- Shadow-user tracking (identified → anonymous visitor linkage): **Done** (2026-10-16, [plan](../docs/superpowers/plans/2026-10-16-shadow-user-tracking.md)).
- Tenant (context) removal: **Done** (2026-10-16, [plan](../docs/superpowers/plans/2026-10-16-tenant-removal.md)).
- Settings page (Web UI): **Done** (2026-10-17, [plan](../docs/superpowers/plans/2026-10-17-settings-page.md)).
- Per-website SMTP + delivery queue (per-context SMTP config, queue + ledger): **Done** (2026-10-19, [plans](../docs/superpowers/plans/2026-10-19-per-website-smtp-queue.md)).
- Visitor/contacts dashboard, onboarding, hidden contacts: **Done** (2026-10-20, [plan](../docs/superpowers/plans/2026-10-20-visitor-contacts-dashboard-onboarding-plan.md)).
- MCP server: full 52-tool server replacing the earlier foundation (2026-10-21, [plan](../docs/superpowers/plans/2026-10-21-mcp-server.md)).
- **Open:** Profile/Security Web UI (password change, session management) — backend routes exist (`/api/auth/profile`, `/api/auth/password`, `/api/auth/logout-all`); frontend needs its own design pass.

## Build Order (Sequential)

1. Core Platform — foundation for everything else ✅
2. Contact Management — first feature, simplest domain ✅
3. Form Management — builds on core, introduces email queue ✅
4. User Tracking — separate driver adapter, shared with Banners ✅
5. Banner Management — extends tracking, adds placement logic ✅
6. Web UI + tracking server split (2026-09-16 → 2026-09-19) ✅
7. Email system, webhooks, contact activity, settings/profile, auth refresh, visitor tracking (2026-09-25) ✅
8. Context rename + login simplification, Phase 2: platform-level users, memberships, admin bootstrap (2026-09-24 → 2026-10-13) ✅
9. October releases: form website integration, shadow-user tracking, tenant removal, settings page, per-website SMTP + queue, dashboard/onboarding/hidden contacts, MCP tool server (2026-10-14 → 2026-10-21) ✅

## Current Build Status (2026-10-26)

- `go build ./...`, `go vet`, and `make build` pass.
- CICD suite (`make test`): 456 specs — 455 pass, 1 known pre-existing failure (`POST /track/batch` returns 200, spec expects 204 — `cicd/tracking_server_test.go:138`; skipped in `cicd-tests-ci`).
- Unit tests (`go test ./internal/...`): all packages pass except 2 known pre-existing failures — `TestContextMiddleware` default-context fallback (`internal/adapter/api`, 1/18) and a test build failure in `internal/adapter/proxy` (mock missing `GetDefaultContext`).
- Login verified end-to-end on the local k3s cluster (Helm release `automata`): JWT + refresh token + context membership listing.

## Testing Strategy

- Unit and integration tests using Ginkgo + testcontainers
- E2E tests for API and MCP layers in `cicd/` directory
- Mail and IMAP mocks for email/IMAP integration validation