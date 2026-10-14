# Automata — Project Roadmap

Automata is a self-hosted marketing automation tool for small-to-mid businesses. It helps users track website visitors, collect leads via forms, manage contacts, and display banners on their websites.

## Architecture

- Multi-tenant ("context"): one Automata instance manages multiple websites. Each context has its own contacts, forms, tracking data, banners, and configuration.
- Dual-server runtime (since 2026-09-19): a main API server (port 8080 — auth, admin API, SPA fallback) and a public tracking server (port 8081 — snippet, `/track`, form submission/rendering; no auth, no SPA). A third BFF server (port 8082, `cmd/bff`) fronts the Web UI.
- Reverse proxy: routes tenant traffic via subdomain or path.
- AI MCP interface: exposes all features via MCP protocol for AI tool integration.
- Web UI: Vue 3 + Vite + TypeScript + Tailwind CSS (source in `webui/`, built assets served from `web/`).
- Platform-level users: `users` table with role-based `user_contexts` memberships; `context_users` retained for backward compatibility. Admin is bootstrapped on first startup.

## Feature Areas

### 1. Core Platform
Multi-tenant authentication, reverse proxy, MCP server foundation, shared infrastructure (logging, config, email queue). **Done.** Includes dual-token JWT auth with refresh flow, admin bootstrap, and API key management.

### 2. Contact Management
Store and manage contacts collected via forms, tracking events, and manual entry. Deduplication, segmentation, import/export, activity timeline. **Done** (activity timeline added 2026-09-25).

### 3. Form Management
Form builder, form rendering via reverse proxy, submission handling, webhook and email notifications, spam protection. **Done.**

### 4. User Tracking
JavaScript snippet for website embedding, page view and event tracking, visitor identification, analytics dashboard data. **Done**, including admin visitor tracking (visitor list, detail, event timeline — 2026-09-25).

### 5. Banner Management
Banner CRUD, placement rules, A/B testing, impression and click tracking. **Done.**

### 6. Email System
Email queue, template management, and notification integration with forms and webhooks. **Done** (2026-09-25). SMTP delivery: **Done** (2026-10-14, [plan](../docs/superpowers/plans/2026-10-14-email-delivery.md)).

### 7. Webhook Management
Per-context webhook CRUD, queue, delivery, and retry. **Done** (2026-09-25).

### 8. Settings & Profile
Per-context settings (CORS, domain, display), user profile management, and password change. **Done** (2026-09-25).

### 9. Web UI
Vue 3 admin dashboard: login, contexts, contacts, forms, banners, tracking/visitors, settings, API keys. **Done** (phases 1–2, 2026-09-16 → 2026-09-17; BFF server added 2026-09-25).

## Build Order (Sequential)

1. Core Platform — foundation for everything else ✅
2. Contact Management — first feature, simplest domain ✅
3. Form Management — builds on core, introduces email queue ✅
4. User Tracking — separate driver adapter, shared with Banners ✅
5. Banner Management — extends tracking, adds placement logic ✅
6. Web UI + tracking server split (2026-09-16 → 2026-09-19) ✅
7. Email system, webhooks, contact activity, settings/profile, auth refresh, visitor tracking (2026-09-25) ✅
8. Context rename + login simplification, Phase 2: platform-level users, memberships, admin bootstrap (2026-09-24 → 2026-10-13) ✅

## Current Build Status (2026-10-13)

- `go build ./...`, `go vet`, and `make build` pass.
- CICD suite (`make test`): 302 specs — 279 pass, 23 pre-existing failures in tracking/form/banner/placement/campaign specs (unrelated to auth work; failure set confirmed stable).
- Login verified end-to-end on the local k3s cluster (Helm release `automata`, image `ghcr.io/hekemen/automata:latest`): JWT + refresh token + context membership listing.
- Known issue: `config.example.yaml`'s `cookie_secret` is 25 chars, which is invalid for the AES refresh-token cipher (needs 16/24/32 bytes) — the docker-compose smoke test fails on token generation until the secret is set. The k3s chart uses a valid 32-char secret.

## Testing Strategy

- Unit and integration tests using Ginkgo + testcontainers
- E2E tests for API and MCP layers in `cicd/` directory
- Mail and IMAP mocks for email/IMAP integration validation