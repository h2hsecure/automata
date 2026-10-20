# Implementation Plan: Visitor Contacts, Dashboard Fixes, Context Onboarding

Date: 2026-10-20
Spec: `docs/superpowers/specs/2026-10-20-visitor-contacts-dashboard-onboarding-design.md`
Status: checkbox-tracked

## Task List

- [x] **T1 — `pkg/ua` user-agent parser (unit, TDD)**
- [x] **T2 — `contacts.hidden` column + domain/repo/usecase (hidden + context scoping)**
- [x] **T3 — API `?hidden=` param, webui client, ContactsView toggle, dashboard count**
- [x] **T4 — Dashboard usecase merge fix + `GetUAGroups` + bucket aggregation (unit)**
- [x] **T5 — TrackEvent: hidden contact + page_visit activity (unit)**
- [x] **T6 — TrackBatch: route through TrackEvent, normalize, reject empty type**
- [x] **T7 — Uplift: `IdentifyVisitor` hidden-contact path + `ReassignVisitorContactID`**
- [x] **T8 — DashboardView: remove Visitors tab**
- [x] **T9 — Settings gear in AppHeader, remove sidebar item**
- [x] **T10 — Bootstrap: no default context; creator owner membership; resolver 400**
- [x] **T11 — Onboarding view + router guard**
- [x] **T12 — cicd specs (dashboard, hidden contact, uplift, batch, bootstrap, resolver)**
- [x] **T13 — Production data migration (one-off kubectl SQL)**
- [ ] **T14 — Full gates, deploy, live HTTP/DOM verification**

---

## T1 — `pkg/ua` user-agent parser

New package `pkg/ua/ua.go` + `pkg/ua/ua_test.go`.

- `type Device string` with `DeviceDesktop`/`DeviceMobile`/`DeviceTablet` (`"desktop"`/`"mobile"`/`"tablet"`).
- `type Browser string` with `BrowserChrome`/`BrowserFirefox`/`BrowserSafari`/`BrowserEdge`/`BrowserOther` (`"Chrome"` etc.).
- `func Parse(ua string) (Device, Browser)`:
  - Device: tablet first (`iPad`, `Tablet`, `Kindle`, `Silk`, `Playbook`), then mobile (`Android`, `iPhone`, `Mobile`, `Windows Phone`), else desktop.
  - Browser order: `Edg/` → Edge; `Firefox` → Firefox; `Safari` → Safari; `Chrome` → Chrome; else Other. Empty UA → (desktop, Other).
- TDD: write `ua_test.go` table first (iOS Safari, Chrome Android, iPad, Firefox Linux, Edge Win, empty, garbage), run red, implement, green.

Gate: `go test -count=1 ./pkg/ua/`

## T2 — `contacts.hidden` + domain/repo/usecase

1. `internal/infrastructure/contact/migration.sql`: append
   `ALTER TABLE contacts ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT FALSE;`
2. `internal/domain/contact/contact.go`: `Contact.Hidden bool`.
3. `internal/domain/contact/repository.go`: `FilterOptions` += `Hidden *bool`, `ContextID string`; port += `FindBySourceID(contextID, source, sourceID string) (*Contact, error)`, `Count(contextID string, filters FilterOptions) (int64, error)`.
4. `internal/infrastructure/contact/repo/postgres.go`:
   - Extract `buildListWhere(filters ct.FilterOptions, argStart int) (string, []interface{})` from `List` (existing clauses + `context_id` when set + `hidden` when set).
   - `List` uses it; `Count` implemented with `COUNT(*)` + same builder.
   - `Create`/`Update` write `hidden`; `GetByID`/`List` scan it.
   - `FindBySourceID`: SELECT by (context_id, source, source_id); `sql.ErrNoRows` → nil, nil.
5. `internal/usecase/contact/` (port interface in `tags.go` updated): `ListContacts` sets `Filters.ContextID` + uses `repo.Count`; `ExportContacts`/`SegmentContacts` set `Filters.ContextID`.
6. Update every fake/mock implementing the port (grep for `CountByContext` in `_test.go`).

Gate: `go build ./...` + `go vet ./internal/...`

## T3 — API `?hidden=` + webui wiring

1. `internal/adapter/api/handler/contact_handler.go` List: parse `hidden` query (`"true"`/`"false"` → `*bool`) into `opts.Filters.Hidden`.
2. `webui/src/api/client.ts`: `listContacts(page, limit, search, contextId, hidden?: boolean)` — append `hidden` param when set.
3. `webui/src/views/ContactsView.vue`: default request `hidden=false`; add "Hidden visitors" toggle → `hidden=true` (reloads list).
4. `webui/src/views/DashboardView.vue`: contact-count call passes `hidden=false`.

Gate: `go build ./...`; webui `npx vue-tsc --noEmit` (or `npm run build` if fast).

## T4 — Dashboard merge fix + UA buckets

1. `internal/domain/tracking/analytics.go`: add `UACount{UserAgent string; Count int64}`.
2. Port `internal/domain/tracking/repository.go`: replace `GetDeviceBreakdown`/`GetBrowserBreakdown` with `GetUAGroups(contextID string, dateRange DateRange) ([]UACount, error)`.
3. `internal/infrastructure/tracking/repo/postgres.go`: implement `GetUAGroups` (pageview events, `user_agent != ''`, GROUP BY user_agent). Remove the two old methods. Update any fakes.
4. `internal/usecase/tracking/get_dashboard.go`:
   - `m, err := repo.GetDashboardMetrics(...)` → copy only `TotalVisitors`, `PageViews` into `metrics` (merge, not replace).
   - `groups, err := repo.GetUAGroups(...)`; loop `ua.Parse(g.UserAgent)` → `metrics.DeviceBreakdown[device] += count`, `metrics.BrowserBreakdown[browser] += count` (init maps).
5. Unit test `get_dashboard_test.go` (new, in `internal/usecase/tracking/`): fake repos — assert TopPages/TopReferrers survive; UA groups aggregate to expected device/browser maps (reuse T1).

Gate: `go test -count=1 ./internal/usecase/tracking/`

## T5 — TrackEvent hidden contact + activity

1. `internal/usecase/tracking/track_event.go`: new signature `TrackEvent(contactRepo contact.Repository, trackingRepo tracking.Repository, contextID string, event *tracking.Event) error`; same for `TrackEventWithTime`.
2. After successful event+visitor, if `event.Type == pageview && event.VisitorID != ""`:
   - `FindBySourceID(contextID, "visitor", visitorID)`; if nil → create hidden contact (`source="visitor"`, `source_id=visitorID`, `Hidden=true`); on error → `log.Warn`, continue.
   - `CreateActivity{ContactID, ContextID, Type: page_visit, Data: {url,title,referrer,ip,user_agent,visitor_id}}`; on error → `log.Warn`, continue.
3. Update caller `internal/adapter/tracking/track_handler.go` (pass contactRepo — add field to `TrackHandler`); wiring `internal/adapter/tracking/server.go` + `cmd/automata/main.go` (contact repo into `tracking.NewServer`).
4. Unit test `track_event_test.go` (fakes): pageview+visitor → hidden contact created once (2nd call dedupes), one activity per call; empty visitor id → none; non-pageview → none.

Gate: `go test -count=1 ./internal/usecase/tracking/`

## T6 — TrackBatch through usecase

`internal/adapter/tracking/track_handler.go` `TrackBatch`:
- Loop events; empty `type` after `page_view`→`pageview` normalization → rejected++ (skip); else `TrackEvent(...)`; errors → rejected++, logged.
- Response: `200 {"accepted": n, "rejected": m}` (replace 204).

Gate: `go build ./...`; behavior covered in T12 cicd.

## T7 — Uplift IdentifyVisitor

1. `internal/domain/tracking/repository.go` + postgres repo: add `ReassignVisitorContactID(contextID, oldContactID, newContactID string) error` (unconditional UPDATE; 0 rows ok).
2. `internal/usecase/form/identify_visitor.go` (per spec): hidden lookup → collision-check-first → merge+reassign OR update-in-place → legacy path when no hidden contact.
3. Unit test `identify_visitor_test.go` (fakes): 3 cases (uplift clean, uplift collision, no hidden contact).

Gate: `go test -count=1 ./internal/usecase/form/`

## T8 — Remove Visitors tab

`webui/src/views/DashboardView.vue`: remove `visitors` from tab ref type + `tabs` array; delete visitors template block, `loadVisitors` + trigger, visitor refs/pagination, `listVisitors` import. Keep device/browser/overview charts.

Gate: `npm run build` in `webui/`.

## T9 — Settings in header

1. `webui/src/components/AppHeader.vue`: gear icon (lucide `Settings`) in actions area, `v-if="authStore.user?.is_admin"`, `RouterLink` to `/settings`.
2. `webui/src/components/AppSidebar.vue`: remove `settings` nav item.

Gate: `npm run build` in `webui/`.

## T10 — Bootstrap / resolver / membership

1. `internal/infrastructure/bootstrap/admin.go`: remove default-context creation + admin membership; keep admin user; drop unused `contextRepo` param; update `cmd/automata/main.go` call.
2. `internal/adapter/api/context_middleware.go`: on zero resolved contexts (header/param/subdomain/slug all miss and `GetDefaultContext` fails) → `400 {"error":"no context available"}`.
3. `internal/adapter/api/handler/context_handler.go` `Create`: after context insert, insert `user_contexts` membership (role `owner`) for `c.GetString("user_id")`; add user-repo dep to `ContextHandler` + wiring in main.go.
4. Update `cicd/bootstrap_test.go`: fresh bootstrap expects 0 contexts, 1 admin user, 0 memberships.

Gate: `go build ./...`; bootstrap cicd spec.

## T11 — Onboarding view + guard

1. `webui/src/views/OnboardingView.vue`: name input → `createContext({name, slug?})` → update `authStore` contexts (push + `currentContextId` + localStorage `context_id`) → `router.push('/dashboard')`. (Check `createContext` payload shape in client.ts.)
2. `webui/src/router/index.ts`: guard — authenticated + `contexts.length === 0` + target not in `{/login, /onboarding}` → redirect `/onboarding`.

Gate: `npm run build` in `webui/`.

## T12 — cicd specs

New/updated specs in `cicd/` (reuse `support` fixtures; don't touch WIP files):
- `dashboard_test.go` (new): seed pageview events (known URL/referrer/UA: iPhone Safari, Chrome Android, Firefox Linux) → GET `/api/context/tracking/dashboard` → `top_pages` non-null & sorted, `device_breakdown` has `mobile`+`desktop`, `browser_breakdown` has `Safari`/`Chrome`/`Firefox`, `page_views` > 0.
- `hidden_contact_test.go` (new): POST `/track` w/ visitor id → `contacts` row `hidden=true, source='visitor'`; `GET /api/context/contacts?hidden=false` excludes, `?hidden=true` includes; 2nd pageview → no duplicate.
- `uplift_test.go` (new): form submit w/ same visitor + email → contact unhidden, `source='form:{slug}'`; collision → merged into existing, `tracking_visitors.contact_id` repointed.
- Extend `tracking_http_test.go`: batch with `page_view` + empty type + valid → `200 {"accepted":2,"rejected":1}`; stored types normalized; visitor upserted.
- `bootstrap_test.go` (update per T10).
- Resolver: zero contexts → 400 "no context available" (extend existing context spec file if present, else `context_resolver_test.go`).

Gate: `go test -count=1 ./cicd/` — compare to baseline (395 specs; known pre-existing failures in tracking/form/banner/placement/campaign).

## T13 — Production migration (one-off)

Script `/tmp/opencode/migrate_default_to_pdaccess_final.sql` (executed via kubectl psql with `ON_ERROR_STOP=1`, NOT committed). Backup: `/tmp/opencode/default_context_backup.tsv` (63 rows, 11 tables).

As executed (single transaction, precondition guards + post-check):
1. Guards: default + pdaccess context rows exist; no contact email collision; every default `email_inbox` row is a byte-identical duplicate in pdaccess (same uid/sender/subject/body); every default `email_template` key is either byte-identical in pdaccess or a disposable live-verify artifact.
2. Re-point admin membership: `UPDATE user_contexts SET context_id=pdaccess WHERE context_id=default` (1 row, role stays `owner`) — preserves admin access; cascade FKs handle the rest.
3. Context_id rewrite across all 24 context-scoped tables (contacts 4, contact_activities 11, banner_banners 1, email_history 13, forms 1, form_submissions 4, tracking_events 22, tracking_visitors 8, rest 0).
4. Dedupe deletes (unique constraints `uq_email_inbox_context_uid`, `uq_email_templates_context_key`): DELETE 4 default `email_inbox` rows (true duplicates of the same mailbox polled under both contexts), DELETE 7 default `email_templates` rows (5 identical duplicates + 2 disposable live-verify artifacts).
5. `DELETE FROM contexts WHERE id=default`.
6. Post-check: UNION ALL count asserts zero rows reference the old context id; COMMIT.

Gate: counts match pre-migration snapshot; app healthy; dashboard shows merged data. ✅ Executed 2026-10-20 — pdaccess counts (contacts 4, contact_activities 11, banner_banners 1, email_history 13, email_inbox 4, email_templates 5, forms 1, form_submissions 4, tracking_events 22, tracking_visitors 8, user_contexts 1), contexts table = h2hsecure-web + pdaccess; `/api/admin/contexts` + `/api/context/contacts` verified live with admin token.

## T14 — Gates, deploy, verify

1. `go build ./... && go vet ./...`; `go test -count=1 ./pkg/... ./internal/usecase/... ./internal/infrastructure/contact/repo/` (unit).
2. `go test -count=1 ./cicd/` full — compare failures to baseline.
3. `webui`: `npm run build`.
4. Commit (spec + plan + code), push, deploy new image to k3s.
5. Live verify (HTTP/DOM only, no screenshots):
   - Dashboard API JSON: `top_pages` non-null, breakdowns populated.
   - Contacts list `?hidden=true/false` behavior.
   - Fresh user (or admin with no memberships) → SPA redirects to `/onboarding`; context creation works, gear appears in header.
   - Batch endpoint 200 with accepted/rejected.
6. Run T13 production migration; re-verify dashboard + contacts.