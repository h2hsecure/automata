# Visitor Contacts, Dashboard Fixes, and Context Onboarding — Design

Date: 2026-10-20
Status: Approved in conversation (design presented section-by-section, user confirmed)

## Overview

Rework how Automata connects anonymous page visitors to marketing contacts, fix the
broken dashboard charts, relocate the Settings entry point, and replace the
implicit "default context" with an explicit onboarding flow.

1. **Hidden contact per page visitor** — every anonymous page visitor gets a
   `hidden` contact (`source="visitor"`, `source_id=visitor_id`, no name/email).
   Hidden contacts are excluded from the default contact list but filterable.
2. **Page-visit activity** — every pageview logs a `page_visit` activity on the
   visitor's contact with the full event payload (URL, title, referrer, IP,
   user-agent, visitor id).
3. **Uplift on form submission** — when a visitor submits a form with an email,
   their hidden contact is "uplifted": identity fields filled, `hidden=false`,
   `source="form:{slug}"`, and if the email collides with an existing real
   contact the hidden contact is merged into it (visitor link repointed).
4. **Dashboard fixes** — stop the metrics overwrite that nulls TopPages/
   TopReferrers/breakdowns; real device/browser classification via an in-house
   user-agent parser (`pkg/ua`) instead of raw-UA grouping; remove the
   dead "Visitors" tab.
5. **Settings in header** — gear icon in `AppHeader` (admin-only) instead of a
   sidebar nav item.
6. **No default context + onboarding** — fresh installs create the admin user
   only; a zero-context user is redirected to a new `/onboarding` view that
   creates their first website (context) and grants them owner membership.
   Context resolution with zero contexts returns a clean 400.
7. **Production data migration** — one-off kubectl SQL moving the default
   context's data into the `pdaccess` context (merge-on-collision), then
   deleting the default context.
8. **Batch tracking hardening** — `/track/batch` routes each event through the
   same `TrackEvent` usecase (so visitor upsert, hidden contact, and
   page_visit activity all apply), normalizes `page_view` → `pageview`, and
   rejects events with empty type (reported in the response).

## Non-Goals (YAGNI)

- No cross-domain visitor stitching, no cookie consent UI, no server-side
  cookie management beyond the existing client snippet.
- No third-party UA database; the parser classifies a small fixed set of
  devices/browsers and buckets the rest into "Other".
- No automatic merging of hidden contacts that are never uplifted.
- No per-context onboarding variants; the onboarding screen is a single
  name-entry flow that creates exactly one context.
- No UI for editing a contact's hidden flag (set/unset only via the visitor /
  uplift lifecycle).

## Data Model

### `contacts` (existing table — extended)

New column (idempotent, appended to `internal/infrastructure/contact/migration.sql`):

| Column | Type | Notes |
|---|---|---|
| hidden | BOOLEAN NOT NULL DEFAULT FALSE | true while the contact represents only an anonymous page visitor |

All existing `hidden` reads/defaults are backwards compatible; the cicd
migration runner already applies `internal/infrastructure/contact/migration.sql`.

### Domain (`internal/domain/contact`)

- `Contact` gains `Hidden bool`.
- `FilterOptions` gains:
  - `Hidden *bool` — nil = no filter, `true` = hidden only, `false` = exclude hidden.
  - `ContextID string` — empty = no scope (legacy callers), set = scope list to
    this context. (Fixes the existing cross-context leak in `repo.List`, which
    currently builds its WHERE without `context_id`.)
- Repository port gains:
  - `FindBySourceID(contextID, source, sourceID string) (*Contact, error)` —
    returns nil (no error) when absent; used for visitor dedupe and uplift.
  - `Count(contextID string, filters FilterOptions) (int64, error)` — filtered
    count sharing the same WHERE builder as `List`; `CountByContext` remains as
    a thin wrapper for existing callers.

### Contact repo (`internal/infrastructure/contact/repo/postgres.go`)

- `Create` / `Update` write `hidden`; `GetByID` / `List` scan it.
- `List` WHERE builder gains `context_id = $n` (when `filters.ContextID != ""`)
  and `hidden = $n` / `hidden <> $n` (when `filters.Hidden != nil`).
- `Count` implemented with the shared WHERE builder (extract helper
  `buildListWhere(filters, contextID) (string, []interface{})`).
- `FindBySourceID`: `SELECT ... WHERE context_id=$1 AND source=$2 AND source_id=$3`
  (source_id is VARCHAR(128), already exists).

### Usecases (`internal/usecase/contact`)

- `ListContacts` sets `opts.Filters.ContextID = contextID` and computes the
  total via `repo.Count(contextID, opts.Filters)` (filtered total).
- `ExportContacts` and `SegmentContacts` also set `filters.ContextID` so all
  read paths are context-scoped.
- `CreateContact` (used by uplift's fallback) unchanged.

### Tracking (`internal/domain/tracking`)

- New `UACount struct { UserAgent string; Count int64 }`.
- Repository port: replace `GetDeviceBreakdown` / `GetBrowserBreakdown`
  (both raw-UA duplicates) with
  `GetUAGroups(contextID string, dateRange DateRange) ([]UACount, error)` —
  one query grouping pageview events by raw `user_agent`.

### Tracking repo (`internal/infrastructure/tracking/repo/postgres.go`)

- `GetUAGroups`: `SELECT user_agent, COUNT(*) FROM tracking_events
  WHERE context_id=$1 AND type='pageview' AND created_at BETWEEN $2 AND $3
  GROUP BY user_agent` (empty UA rows excluded by `user_agent != ''`).

## Behavior

### Hidden contact + page-visit activity (TrackEvent usecase)

`internal/usecase/tracking/track_event.go` — signature becomes
`TrackEvent(contactRepo contact.Repository, trackingRepo tracking.Repository, contextID string, event *tracking.Event) error`.

After `CreateEvent` + `UpsertVisitor` succeed, for `pageview` events with a
non-empty `event.VisitorID`:

1. `contactRepo.FindBySourceID(contextID, "visitor", visitorID)`.
2. If nil → create a contact: `source="visitor"`, `sourceID=visitorID`,
   `hidden=true`, no email/name/phone/company. Creation failures are logged,
   not returned (tracking must not fail on contact-store issues).
3. Create a `page_visit` activity on that contact with
   `Data = {url, title, referrer, ip, user_agent, visitor_id}`. Activity
   failures are logged, not returned.

Non-pageview events with a visitor id: contact lookup/creation + activity are
skipped (only page visits are logged).

Callers to update: `internal/adapter/tracking/track_handler.go` (single
`/track`) and the batch path (below). `cmd/automata/main.go` wires the contact
repo into the tracking server.

### Batch endpoint (`/track/batch`)

`internal/adapter/tracking/track_handler.go` `TrackBatch` stops calling
`repo.CreateEventsBatch` directly. It loops events and calls
`TrackEvent(contactRepo, trackingRepo, contextID, e)` sequentially, so visitor
metrics, hidden contact creation, and page_visit activities all apply to batch
events.

- Events whose `type` is empty after the usecase's normalization
  (`page_view` → `pageview`) are skipped and counted as rejected.
- Per-event errors are logged and counted as rejected; the response is always
  `200 {"accepted": n, "rejected": m}` (was 204).

### Uplift (`internal/usecase/form/identify_visitor.go`)

`IdentifyVisitor(contactRepo, trackingRepo, contextID, slug, visitorID, data)`
starts with a hidden-contact lookup:

1. `hidden := contactRepo.FindBySourceID(contextID, "visitor", visitorID)`.
2. If found:
   - Collision check first (the unique `(context_id, email)` constraint makes
     the ordering mandatory): `existing := contactRepo.FindByEmail(contextID, email)`.
   - If `existing != nil && existing.ID != hidden.ID` →
     `contactRepo.Merge(existing.ID, hidden.ID)` (keep the real contact; its
     email stays), then repoint the visitor link:
     `trackingRepo.ReassignVisitorContactID(contextID, hidden.ID, existing.ID)`.
     Return the existing contact. (The hidden contact is deleted by the merge;
     its page_visit activities move with it.)
   - Otherwise: fill `Email`, `FirstName`, `LastName`, `Phone`, `Company` from
     form data; set `Hidden = false`, `Source = "form:" + slug` (keep
     `SourceID`); `contactRepo.Update(hidden)`. Return the hidden contact.
3. If not found → existing path: `CreateContact` (email dedupe) +
   `SetVisitorContactID` (unchanged).

New tracking port method:
`ReassignVisitorContactID(contextID, oldContactID, newContactID string) error` —
`UPDATE tracking_visitors SET contact_id=$3 WHERE context_id=$1 AND contact_id=$2`
(unconditional, unlike `SetVisitorContactID` which only fills NULL).

`form_handler.go` already calls `IdentifyVisitor` with the form slug.

### Dashboard usecase (`internal/usecase/tracking/get_dashboard.go`)

- Line 71: replace the whole-metrics assignment with a merge — fetch
  `m, err := repo.GetDashboardMetrics(...)` and copy only
  `m.TotalVisitors` and `m.PageViews` into `metrics`, preserving
  TopPages/TopReferrers/breakdowns.
- Replace the two breakdown repo calls with one
  `repo.GetUAGroups(...)`; aggregate in the usecase via `pkg/ua.Parse` into
  `DeviceBreakdown` (keys `desktop`/`mobile`/`tablet`) and
  `BrowserBreakdown` (keys `Chrome`/`Firefox`/`Safari`/`Edge`/`Other`).
  Unknown UA strings count under the "Other" bucket in both maps.

### UA parser (`pkg/ua`) — new package

- `func Parse(userAgent string) (Device, Browser)`:
  - Device: `mobile` (Android|iPhone|iPad|Mobile|iPad-like), `tablet`
    (iPad|Tablet|Kindle|Silk), else `desktop`. Tablet checked before mobile
    where needed; empty UA → `desktop`, `Other`.
  - Browser: substring match — Edge (`Edg/`), Firefox (`Firefox`), Safari
    (`Safari` without Chrome/Edg), Chrome (`Chrome`), else `Other`.
  - Pure function, stdlib only; unit-tested with a representative table.
- Note: iOS Safari UA contains both "Safari" and (when in Chrome mode) "CriOS";
  the fixed ordering above keeps the small bucket set stable.

### Web UI

- **DashboardView.vue** — remove the `visitors` tab (tab ref type, tab array,
  template section, `loadVisitors` + its trigger, `listVisitors` import,
  visitor refs/pagination). Dashboard contact count request passes the
  `hidden=false` filter.
- **client.ts** — `listContacts(page, limit, search, contextId, hidden?)` adds
  `?hidden=` when the argument is a boolean; `createContext` already exists
  (POST `/admin/contexts`).
- **ContactsView.vue** — default list request sends `hidden=false`; add a
  visible "Hidden visitors" filter toggle that switches to `hidden=true`.
- **AppHeader.vue** — admin-only gear icon (lucide `Settings`) in the actions
  area linking to `/settings` (`v-if="authStore.user?.is_admin"`).
- **AppSidebar.vue** — remove the `settings` nav item.
- **OnboardingView.vue** — new view: website name input → `createContext` →
  append to `authStore.contexts`, set `currentContextId` + localStorage
  `context_id` → `router.push('/dashboard')`.
- **router/index.ts** — guard: if authenticated with
  `authStore.contexts.length === 0` and target is not `/onboarding` (or an
  auth page) → redirect to `/onboarding`.

### Context lifecycle

- `internal/infrastructure/bootstrap/admin.go` — remove default-context
  creation and the admin membership row; keep admin user creation (and
  password generation/logging). Drop the now-unused `contextRepo` parameter and
  update `cmd/automata/main.go` + `cicd/bootstrap_test.go` expectations
  (fresh bootstrap → 1 admin user, 0 contexts, 0 memberships).
- `internal/adapter/api/context_middleware.go` — when no explicit context is
  resolvable and `GetDefaultContext` fails (zero contexts), respond `400
  {"error":"no context available"}` (clean message, no debug details).
- `internal/adapter/api/handler/context_handler.go` — `Create` grants the
  creating user owner membership: add a user-repo dependency to
  `ContextHandler` (wiring in main.go), read `c.GetString("user_id")`, insert
  a `user_contexts` row (role `owner`) after context creation. Without this,
  onboarding loops forever because `GetByUserEmail` only joins
  `user_contexts`.

### Production data migration (one-off, kubectl SQL)

Runs against production Postgres via
`kubectl -n automata exec automata-postgresql-0 -c postgresql -- psql ...`.
Not a versioned migration.

1. Move from `default` (`8c61ddc4-debd-445e-b90b-3c62f89dfcfc`) to `pdaccess`
   (`e90a16c1-2178-4bb5-81f9-9c710d170b11`): `contacts`, `contact_activities`
   (+tags/memberships if present), `tracking_events`, `tracking_visitors`
   (preserving `contact_id` links), and `email_*` rows (jobs/history/inbox) —
   by rewriting `context_id`.
2. Email collisions: a default-context contact whose email already exists in
   pdaccess → merge (move its activities, repoint `tracking_visitors.contact_id`
   to the pdaccess contact, drop the duplicate).
3. Delete the default context row and its `user_contexts` memberships.

Precondition: the `hidden` column migration ships in code before the
production migration runs (new deployment).

## Tests

### Unit (go test)

- `pkg/ua`: table-driven tests — iOS Safari, Android Chrome, iPad, Linux
  Firefox, Edge, empty string, unknown agent.
- Dashboard usecase: fake repos — verify TopPages/TopReferrers survive the
  metrics merge and that UA groups aggregate into the expected buckets.
- `TrackEvent` with fake contact+tracking repos: pageview with visitor id
  creates a hidden contact once (idempotent on second call), writes one
  page_visit activity per event; empty visitor id creates nothing.
- `IdentifyVisitor` uplift: hidden contact found → unhidden + fields filled;
  email collision → merge called with (existing, hidden) and
  `ReassignVisitorContactID` called; no hidden contact → legacy create path.
- Contact repo `Count`/`List` hidden+context filters (covered by cicd, see below).

### cicd (Ginkgo, Postgres testcontainer)

- Dashboard: seed pageview events with known URLs/referrers/UAs → GET
  `/api/context/tracking/dashboard` returns non-null `top_pages`,
  non-null `device_breakdown` with expected device keys, `browser_breakdown`
  with expected browser keys, and `page_views` > 0.
- Hidden contact: POST `/track` with a visitor id → contact exists with
  `hidden=true`, `source='visitor'`; GET `/api/context/contacts?hidden=false`
  excludes it, `?hidden=true` includes it; a second pageview does not create
  a second contact.
- Uplift: form submission from the same visitor with an email → contact
  unhidden, `source='form:{slug}'`; collision case merges into the pre-existing
  contact and the visitor's `contact_id` points at the kept contact.
- Batch: POST `/track/batch` with `page_view` + empty-type + valid event →
  200 `{"accepted":2,"rejected":1}`, events normalized to `pageview`,
  visitor upserted, hidden contact created.
- Bootstrap: fresh `bootstrap.Run` → 0 contexts, 1 admin user, 0
  memberships (update `bootstrap_test.go`).
- Context resolver: zero contexts → 400 "no context available".

### Gates

- `go test -count=1 ./cicd/` — baseline 395 specs; known pre-existing failures
  in tracking/form/banner/placement/campaign specs (compare to baseline).
- `webui`: `npm run build` (tsc + vite) must pass.

## Rollout

1. Code (migration + features) → cicd green → deploy.
2. Production data migration via kubectl (one-off).
3. Live verification via HTTP/DOM only (project rule: no screenshots):
   dashboard JSON fields, contacts `?hidden=` responses, onboarding redirect
   for a zero-context user, header gear visible for admin.
4. BFF `:latest` image optional upgrade (out of scope).