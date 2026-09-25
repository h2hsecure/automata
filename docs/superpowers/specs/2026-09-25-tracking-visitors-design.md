# Tracking Visitors — Design Spec

**Date:** 2026-09-25  
**Status:** Draft  
**PRD Reference:** Section 7.7 (Tracking & Analytics)

---

## 1. Problem Statement

The Automata platform currently tracks visitors via `tracking_visitors` and `tracking_events` tables, and already provides a dashboard metrics endpoint and an events log. However, the ability to query **individual visitors** — list them, drill into a visitor's profile, and view their full event history — has not been implemented. The PRD (Section 7.7) defines a "Visitors" view with a table of tracked visitors (Visitor ID, Cookie Value, Fingerprint, First Seen, Last Seen, Page Views) with the ability to click into a visitor detail to see their event history.

---

## 2. Goals & Non-Goals

### Goals

1. **List visitors** with pagination and sorting (by `first_seen`, `last_seen`, or `page_views`).
2. **View visitor detail** — a profile showing visitor metadata and a paginated event history.
3. **List a visitor's events** — filterable by type (`pageview` or `event`).
4. **Admin-only access** — visitor listing requires authentication; API keys do not qualify (consistent with `admin` group pattern in `server.go`).
5. **GDPR-compliant** — `visitor_id` is pseudo-anonymous (cookie + fingerprint); no raw IP stored.

### Non-Goals

- **Visitor identity resolution** — no login-to-visitor linking (e.g., associating a visitor with a Contact record).
- **Real-time visitor map** — no live map or WebSocket-based live view.
- **Visitor merge/deduplication** — no logic for merging duplicate visitor records.
- **Visitor export** — no CSV/Excel export of visitor data.
- **Visitor deletion / right-to-be-forgotten** — outside this iteration (noted as future work).

---

## 3. Architecture Overview

```
┌──────────────┐     GET /api/admin/tracking/visitors         ┌──────────────┐
│  pdavui      │ ◄────────────────────────────────────────── │  ws (API)    │
│  Tracking     │  { visitors: [...], total }                  │  Gin router  │
│  View.vue     │                                              └──────┬───────┘
└──────────────┘                                                     │
                                                                     ▼
                                                            ┌──────────────┐
                                                            │ TrackingHandler │
                                                            │ (admin)        │
                                                            └──────┬───────┘
                                                                     │
                                              ┌──────────────────────┼──────────────┐
                                              ▼                      ▼              ▼
                                   ┌──────────────────┐  ┌──────────────────┐  ┌──────────────┐
                                   │ GetVisitors      │  │ GetVisitorDetail │  │ GetVisitor   │
                                   │ (usecase)        │  │ (usecase)        │  │Events       │
                                   └────────┬─────────┘  └────────┬─────────┘  └──────┬─────┘
                                            │                     │                  │
                                            └─────────────────────┼──────────────────┘
                                                                          │
                                                                          ▼
                                                                 ┌──────────────┐
                                                                 │  PostgreSQL  │
                                                                 │  repository  │
                                                                 └──────────────┘
```

### Layering

| Layer | Location | Responsibility |
|-------|----------|----------------|
| **Handler** | `internal/adapter/api/handler/tracking_handler.go` | Parse query params, call usecase, format JSON response |
| **Use Case** | `internal/usecase/tracking/get_visitors.go` (new), `get_visitor_detail.go` (new), `get_visitor_events.go` (new) | Validate context, call repository |
| **Repository Interface** | `internal/domain/tracking/repository.go` | Extend `tracking.Repository` with visitor queries |
| **PostgreSQL Impl** | `internal/infrastructure/tracking/repo/postgres.go` | SQL queries |
| **Router** | `internal/adapter/api/server.go` | Register admin routes under `/api/admin/tracking/visitors` |

### Authentication Decision

Existing `GET /api/tracking/dashboard` and `GET /api/tracking/events` are under the context-resolved route group (`api.Use(ContextResolver)`), meaning they accept both JWT and API keys. However, visitor data is more sensitive — it reveals behavioral profiles. Consistent with the PRD's `adminOnly` pattern (used for Tenants and API Keys), **visitor listing will be gated under the `/admin` group**, which requires a JWT with the `is_admin` claim.

| Endpoint | Route Group | Auth |
|----------|------------|------|
| `GET /api/admin/tracking/visitors` | `/admin` | JWT with `is_admin` |
| `GET /api/admin/tracking/visitors/:id` | `/admin` | JWT with `is_admin` |
| `GET /api/admin/tracking/visitors/:id/events` | `/admin` | JWT with `is_admin` |

---

## 4. Data Model

The `tracking_visitors` table already exists (migration in `repo/migration.sql`):

```sql
CREATE TABLE IF NOT EXISTS tracking_visitors (
    id              VARCHAR(128) PRIMARY KEY,
    context_id       UUID NOT NULL,
    cookie_value    VARCHAR(128) NOT NULL,
    fingerprint     VARCHAR(64),
    first_seen      TIMESTAMPTZ DEFAULT NOW(),
    last_seen       TIMESTAMPTZ DEFAULT NOW(),
    page_views      INT DEFAULT 0,
    UNIQUE(context_id, cookie_value)
);

CREATE INDEX IF NOT EXISTS idx_tracking_visitors_context ON tracking_visitors(context_id);
```

### Index Additions

For the new list/sort endpoints, we need composite indexes to support efficient sorting:

```sql
-- For sorting by first_seen (default desc)
CREATE INDEX IF NOT EXISTS idx_tracking_visitors_context_first_seen
    ON tracking_visitors(context_id, first_seen DESC);

-- For sorting by last_seen
CREATE INDEX IF NOT EXISTS idx_tracking_visitors_context_last_seen
    ON tracking_visitors(context_id, last_seen DESC);

-- For sorting by page_views (descending)
CREATE INDEX IF NOT EXISTS idx_tracking_visitors_context_page_views
    ON tracking_visitors(context_id, page_views DESC);
```

These are **append-only** migrations (no schema changes to existing columns). The indexes are lightweight (~72 bytes per entry for context_id + timestamp/page_views) and support the offset-based pagination pattern already used elsewhere in the codebase.

### Visitor Detail Event History

The existing `tracking_events` table already indexes by `(context_id, visitor_id)`, so the visitor events query is already optimized:

```sql
CREATE INDEX IF NOT EXISTS idx_tracking_events_visitor
    ON tracking_events(context_id, visitor_id);
```

---

## 5. API Contracts

### 5.1 `GET /api/admin/tracking/visitors`

List visitors with pagination and sorting.

**Auth:** JWT with `is_admin` claim (admin route group).

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `offset` | int | 0 | Number of records to skip |
| `limit` | int | 20 | Max records to return (max 100) |
| `sort` | string | `last_seen` | Sort field: `first_seen`, `last_seen`, `page_views` |

**Response (200 OK):**

```json
{
  "visitors": [
    {
      "id": "a1b2c3d4...",
      "cookie_value": "abc123xyz",
      "fingerprint": "7f3a2b...",
      "first_seen": "2026-09-20T14:30:00Z",
      "last_seen": "2026-09-25T09:15:00Z",
      "page_views": 42
    }
  ],
  "total": 1337
}
```

**Error Responses:**

| Status | Body |
|--------|------|
| 400 | `{ "error": "invalid sort field" }` or `{ "error": "limit must be between 1 and 100" }` |
| 401 | `{ "error": "invalid token or API key" }` |

---

### 5.2 `GET /api/admin/tracking/visitors/:id`

Get a single visitor's profile with their event history.

**Auth:** JWT with `is_admin` claim.

**Path Parameters:**

| Param | Description |
|-------|-------------|
| `id` | Visitor ID (VARCHAR 128) |

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 20 | Number of recent events to return (max 50) |

**Response (200 OK):**

```json
{
  "visitor": {
    "id": "a1b2c3d4...",
    "cookie_value": "abc123xyz",
    "fingerprint": "7f3a2b...",
    "first_seen": "2026-09-20T14:30:00Z",
    "last_seen": "2026-09-25T09:15:00Z",
    "page_views": 42
  },
  "recent_events": [
    {
      "type": "pageview",
      "url": "/products/widget",
      "event_name": null,
      "referrer": "https://google.com",
      "created_at": "2026-09-25T09:15:00Z"
    }
  ]
}
```

**Error Responses:**

| Status | Body |
|--------|------|
| 404 | `{ "error": "visitor not found" }` |

---

### 5.3 `GET /api/admin/tracking/visitors/:id/events`

List all events for a specific visitor, paginated and filterable.

**Auth:** JWT with `is_admin` claim.

**Path Parameters:**

| Param | Description |
|-------|-------------|
| `id` | Visitor ID (VARCHAR 128) |

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `offset` | int | 0 | Number of records to skip |
| `limit` | int | 20 | Max records to return (max 100) |
| `type` | string | (all) | Filter by `pageview` or `event` |

**Response (200 OK):**

```json
{
  "events": [
    {
      "type": "pageview",
      "url": "/products/widget",
      "event_name": null,
      "referrer": "https://google.com",
      "created_at": "2026-09-25T09:15:00Z"
    },
    {
      "type": "event",
      "url": "/products/widget",
      "event_name": "add_to_cart",
      "referrer": "https://google.com",
      "created_at": "2026-09-25T09:15:30Z"
    }
  ],
  "total": 57
}
```

**Error Responses:**

| Status | Body |
|--------|------|
| 404 | `{ "error": "visitor not found" }` |
| 400 | `{ "error": "invalid type value" }` |

---

## 6. Key Design Decisions

### 6.1 Admin-Only vs. Context-Resolved Auth

**Decision:** Visitor endpoints live under `/api/admin/` and require JWT with `is_admin` claim.

**Rationale:**
- Visitor data is behavioral — it reveals patterns of who visits a tenant's site. Exposing this via API keys (which may belong to non-admin integrations) increases data leakage risk.
- The PRD marks the Tenants and API Keys pages as `adminOnly` in the sidebar; visitor analytics is at the same sensitivity level.
- The existing `/admin` group in `server.go` already provides `middleware.AuthMiddleware(authService)` which validates JWT tokens and sets `is_admin` — no new middleware needed.

**Trade-off:** Tenant admin users without the `is_admin` JWT claim cannot access visitor data. This is acceptable for V1; future iterations can introduce a granular `can_view_tracking` permission.

### 6.2 Repository Pattern — Extend vs. New Interface

**Decision:** Extend `tracking.Repository` interface with new methods rather than creating a separate interface.

**Rationale:**
- The `tracking.Repository` interface is already used by `TrackingHandler` for dashboard and events. Adding visitor methods keeps all tracking persistence in one cohesive contract.
- The usecase layer (`internal/usecase/tracking/`) already imports `tracking.Repository` — no additional wiring in server setup.

**New interface methods:**

```go
type Repository interface {
    // Existing methods ...

    // New methods for visitor listing
    ListVisitors(ctx context.Context, contextID string, opts ListVisitorsOpts) ([]*Visitor, int64, error)
    GetVisitorByID(ctx context.Context, contextID, visitorID string) (*Visitor, error)
    GetVisitorEvents(ctx context.Context, contextID, visitorID string, opts GetVisitorEventsOpts) ([]*Event, int64, error)
}
```

### 6.3 Pagination Strategy — Offset-Based

**Decision:** Use offset/limit pagination (not cursor-based).

**Rationale:**
- The existing `GetEvents` method already uses offset/limit — consistency with existing patterns.
- Visitor list is an admin analytics view, not a high-throughput real-time stream. Offset pagination is simpler for the frontend to implement and matches the PRD's expectations.
- If the visitor table grows beyond ~100K rows, consider switching to cursor/keyset pagination (deferred).

### 6.4 Sort Flexibility

**Decision:** Allow client-specified sort via `sort` query parameter with a whitelist: `first_seen`, `last_seen`, `page_views`. Default: `last_seen` descending.

**Rationale:**
- The PRD's visitor table spec lists these three columns as sortable.
- Whitelisting sort fields prevents SQL injection and ensures the database indexes can support the sort.
- Defaulting to `last_seen` (descending) surfaces the most active visitors first — the most useful view for admins.

### 6.5 Limit Guards

**Decision:** Enforce upper bounds on `limit` parameters to prevent expensive queries:
- `ListVisitors`: max 100
- `GetVisitorEvents`: max 100
- `VisitorDetail recent_events`: max 50

**Rationale:** The visitor events table can grow very large for active tenants. Without caps, a single request could return thousands of rows. The frontend can always request additional pages.

### 6.6 Fingerprint Display

**Decision:** Display `fingerprint` as a truncated hash (first 8 characters) in the visitor list view. Full fingerprint shown in detail view.

**Rationale:** The fingerprint column stores a 64-character hash. Showing the full value in a table is noisy and adds bandwidth. A short prefix is sufficient for uniqueness identification.

### 6.7 Event History in Visitor Detail

**Decision:** Include a `recent_events` field (up to 20 events) in the visitor detail response, separate from the dedicated events list endpoint.

**Rationale:** The detail view is a summary/profile view. Showing a few recent events inline gives context without requiring a second API call. The dedicated events endpoint serves the full filtered/paginated history.

---

## 7. Implementation Checklist

### Domain Layer
- [ ] Add `ListVisitorsOpts` and `GetVisitorEventsOpts` structs to `internal/domain/tracking/`
- [ ] Add `ListVisitors`, `GetVisitorByID`, `GetVisitorEvents` methods to `tracking.Repository` interface

### Infrastructure Layer
- [ ] Add SQL migration for sort indexes in `repo/migration.sql`
- [ ] Implement `ListVisitors`, `GetVisitorByID`, `GetVisitorEvents` in `postgres.go`

### Use Case Layer
- [ ] Create `internal/usecase/tracking/get_visitors.go`
- [ ] Create `internal/usecase/tracking/get_visitor_detail.go`
- [ ] Create `internal/usecase/tracking/get_visitor_events.go`

### Adapter Layer
- [ ] Add handler methods to `TrackingHandler` in `tracking_handler.go`
- [ ] Register routes in `server.go` under `/admin/tracking/visitors` group

### Frontend Layer (pdavui)
- [ ] Create `src/views/tracking/VisitorsView.vue`
- [ ] Create `src/views/tracking/VisitorDetail.vue`
- [ ] Update sidebar nav to include "Visitors" under Tracking section

---

## 8. GDPR Compliance Notes

| Requirement | Implementation |
|-------------|----------------|
| Pseudo-anonymous ID | `visitor_id` stores cookie value, not real identity |
| No raw IP | `ip_hash` stores only a hashed version (see `track_event.go`) |
| No PII stored | No name, email, or address fields in tracking tables |
| Future: Right to be forgotten | Not in scope for this iteration. Add `DeleteVisitor` repository method in follow-up |

---

## 9. Open Questions

1. **Should API keys be allowed for visitor endpoints?** Currently excluded (admin-only). If third-party tools need this data, we could add a `can_view_analytics` API key scope later.
2. **Should we add a `context_id` filter to the visitor list?** Currently, the middleware resolves `context_id` from the JWT, so each admin sees only their own context. If multi-tenant admins need cross-context views, we'd need an explicit parameter.
3. **What retention policy applies to visitor data?** The existing `PurgeOldEvents` method handles event cleanup. Should we also purge old visitor records? (Deferred — future iteration)
