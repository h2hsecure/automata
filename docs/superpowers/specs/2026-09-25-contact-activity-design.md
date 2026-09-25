# Design Spec: Contact Activity Timeline

**Date:** 2026-09-25  
**Status:** Draft  
**References:**  
- [Web UI PRD — Section 7.3 Contacts](../webui-prd.md#73-contacts) (Open Question 8)  
- [Existing Activity Domain](../../internal/domain/contact/activity.go)  
- [Activity Migration](../../internal/infrastructure/contact/migration.sql)  

---

## 1. Problem Statement

The platform tracks contacts via form submissions, page visits, and banner interactions, but there is no unified view of a contact's historical behavior. Administrators cannot see the timeline of interactions for any given contact, which limits their ability to:

- Understand contact engagement patterns
- Identify high-value prospects
- Debug form submission issues (was this contact even on the site?)
- Attribute banner impressions/clicks to specific contacts

The `contact_activities` table exists in the database schema and the domain model (`Activity` type) is defined, but the activity endpoints are not wired into the server routing, no type filtering is supported, and the activity-creation plumbing from domain events (form submission, tracking) is missing.

---

## 2. Goals & Non-Goals

### Goals

1. **Expose a paginated activity timeline** for each contact via a new API endpoint.
2. **Support type-based filtering** so the frontend can show "all", "form submissions", "page visits", etc.
3. **Return an activity count** on the contact detail endpoint, enabling UI to show a badge (e.g., "5 interactions").
4. **Wire activity creation** from domain events — form submission, page visit tracking, banner impression/click, contact update, tag changes, contact creation.
5. **Ensure context scoping** — users only see activities for contacts in their context (or all contexts if admin).

### Non-Goals

- Real-time activity push (WebSocket) — polling via paginated endpoint is sufficient for v1.
- Activity export or bulk operations.
- Activity deduplication or merging.
- Activity retention policy (TTL / archival) — out of scope for v1.
- Activity search across all contacts (global search).
- Activity notification or alerting.
- Admin audit log separate from contact activities.

---

## 3. Architecture Overview

### 3.1 High-Level Data Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────────┐
│ Form Submit  │     │ Page Visit   │     │ Banner View/Click│
│ (tracking)   │     │ (tracking)   │     │ (tracking)       │
└──────┬───────┘     └──────┬───────┘     └───────┬──────────┘
       │                    │                     │
       ▼                    ▼                     ▼
┌──────────────────────────────────────────────────────────┐
│  Tracking Event → Contact Resolution → Activity Creation  │
│  (internal/infrastructure/tracking/repo/activity.go)      │
└──────────────────────────┬───────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────┐
│  contact_activities (PostgreSQL table)                    │
│  ┌──────┬──────────┬────────┬───────────────┬──────────┐ │
│  │ id   │ contact_id│ type  │ data (JSONB)  │ created_at│ │
│  └──────┴──────────┴────────┴───────────────┴──────────┘ │
└──────────────────────────┬───────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────┐
│  GET /api/context/contacts/:id/activity                   │
│  → Paginated, filtered, sorted by created_at DESC         │
└──────────────────────────────────────────────────────────┘
```

### 3.2 Activity Creation Flow

Activities are created by **domain events** through a dedicated repository method. The pattern:

1. **Form Submission**: When `POST /api/forms/submit` completes successfully, after creating the submission record, call `activityRepo.CreateActivity()` with `type: "form_submission"`, `data: { form_name, submitted_at, field_values }`.

2. **Page Visit**: When a tracking pageview event is processed (in the tracking server), if the visitor cookie matches a known contact (via email lookup or cookie → contact_id mapping), call `activityRepo.CreateActivity()` with `type: "page_visit"`.

3. **Banner Impression/Click**: When the tracking snippet fires a `banner_view` or `banner_click` event, if matched to a contact, create an activity.

4. **Contact Updated**: In the `UpdateContact` usecase (after successful DB update), create an activity with `type: "contact_updated"`, `data: { changed_fields, updated_at }`.

5. **Tag Added/Removed**: In the `ApplyTags` usecase, for each added/removed tag, create a `tag_added` / `tag_removed` activity.

6. **Contact Created**: In the `CreateContact` usecase, after successful DB insert, create a `contact_created` activity.

7. **Contact Merged**: In the `MergeContacts` usecase, after the merge transaction commits, create a `contact_merged` activity.

### 3.3 Contact-Tracking Resolution

When a tracking event (page visit, banner impression) arrives, the system must determine which contact (if any) it belongs to. The resolution strategy:

1. **Cookie match**: The tracking snippet sets a unique cookie per visitor. When a form is submitted with that contact's email, store the cookie-to-contact mapping in a new table (`contact_cookie_mappings`) or in the `contacts` table itself (`source_id` could be used).

2. **Email match fallback**: If the visitor later submits any form with an email, match to the existing contact.

3. **No match**: If no contact is found, the event is still logged in `tracking_events` but no activity is created. The activity is created retroactively when the contact is eventually identified.

### 3.4 Context Resolution & Access Control

```
┌──────────────────────────────────────────────────┐
│  Request arrives at /api/context/contacts/:id/.. │
└──────────────────────┬───────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────┐
│  AuthMiddleware: validate JWT / API key           │
│  → sets "user_id", "is_admin" in gin.Context     │
└──────────────────────┬───────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────┐
│  ContextResolver: resolve context from            │
│  X-Context-ID header / subdomain / path          │
│  → sets "context_id" in gin.Context              │
└──────────────────────┬───────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────┐
│  ContactHandler.Get / GetActivity:               │
│  - Verify contact.ContextID == context_id         │
│  - If user is_admin → skip context check          │
│  → return activity data                          │
└──────────────────────────────────────────────────┘
```

Admin users (JWT has `is_admin: true`) bypass context scoping and can view activities for any contact across all contexts.

---

## 4. Data Model

### 4.1 Existing: `contact_activities` Table

Already exists in `internal/infrastructure/contact/migration.sql`:

```sql
CREATE TABLE IF NOT EXISTS contact_activities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id      UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    context_id      UUID NOT NULL,
    type            VARCHAR(64) NOT NULL,
    data            JSONB DEFAULT '{}',
    source_id       VARCHAR(128),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_activities_contact ON contact_activities(contact_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activities_context ON contact_activities(context_id, created_at DESC);
```

### 4.2 New: `contact_cookie_mappings` Table (for tracking resolution)

Needed to link tracking cookies to contacts for page visit / banner activities:

```sql
CREATE TABLE IF NOT EXISTS contact_cookie_mappings (
    contact_id  UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    context_id  UUID NOT NULL,
    cookie      TEXT NOT NULL,           -- tracking cookie value
    source      VARCHAR(32) NOT NULL,    -- 'form_submission' or 'manual'
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (context_id, cookie)
);

CREATE INDEX IF NOT EXISTS idx_cookie_contact ON contact_cookie_mappings(contact_id);
```

### 4.3 Activity Types (Extended)

The domain already defines these types in `activity.go`. The spec adds the missing ones:

```go
const (
    ActivityFormSubmission  ActivityType = "form_submission"
    ActivityPageVisit       ActivityType = "page_visit"
    ActivityBannerClick     ActivityType = "banner_click"
    ActivityBannerImpression ActivityType = "banner_impression"  // NEW
    ActivityContactCreated  ActivityType = "contact_created"
    ActivityContactUpdated  ActivityType = "contact_updated"
    ActivityContactMerged   ActivityType = "contact_merged"
    ActivityTagAdded        ActivityType = "tag_added"          // NEW
    ActivityTagRemoved      ActivityType = "tag_removed"       // NEW
)
```

### 4.4 Activity Data Schemas

Each activity type carries structured data in the `data` (JSONB) column:

| Type | Data Schema |
|------|-------------|
| `form_submission` | `{ "form_name": string, "form_slug": string, "submitted_at": string (ISO 8601), "field_values": object }` |
| `page_visit` | `{ "url": string, "referrer": string, "title": string, "visited_at": string (ISO 8601), "user_agent": string }` |
| `banner_impression` | `{ "banner_name": string, "banner_id": string, "placement": string, "viewed_at": string (ISO 8601) }` |
| `banner_click` | `{ "banner_name": string, "banner_id": string, "placement": string, "link_url": string, "clicked_at": string (ISO 8601) }` |
| `contact_updated` | `{ "changed_fields": string[], "updated_at": string (ISO 8601), "previous_values": object }` |
| `tag_added` | `{ "tag_name": string, "tag_id": string, "acted_at": string (ISO 8601) }` |
| `tag_removed` | `{ "tag_name": string, "tag_id": string, "acted_at": string (ISO 8601) }` |
| `contact_created` | `{ "source": string, "created_at": string (ISO 8601) }` |
| `contact_merged` | `{ "merged_contact_id": string, "merged_contact_email": string, "acted_at": string (ISO 8601) }` |

---

## 5. API Contracts

### 5.1 `GET /api/context/contacts/:id/activity`

Get a contact's activity timeline, paginated and optionally filtered by type.

**Path Parameters:**
- `id` (required, UUID): The contact's UUID.

**Query Parameters:**
- `limit` (optional, integer, default: 50, max: 200): Number of activities to return.
- `offset` (optional, integer, default: 0): Number of activities to skip.
- `type` (optional, string): Filter by activity type. Supported values: `form_submission`, `page_visit`, `banner_impression`, `banner_click`, `contact_updated`, `tag_added`, `tag_removed`, `contact_created`, `contact_merged`.

**Response (200 OK):**

```json
{
  "activities": [
    {
      "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "type": "form_submission",
      "title": "Submitted 'Contact Us' form",
      "description": "Contact Us",
      "data": {
        "form_name": "Contact Us",
        "form_slug": "contact_us",
        "submitted_at": "2026-09-20T14:32:00Z",
        "field_values": {
          "email": "jane@example.com",
          "message": "I'm interested in your service."
        }
      },
      "created_at": "2026-09-20T14:32:00Z"
    },
    {
      "id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
      "type": "page_visit",
      "title": "Visited /pricing",
      "description": "Pricing page",
      "data": {
        "url": "/pricing",
        "referrer": "/home",
        "visited_at": "2026-09-18T09:15:00Z"
      },
      "created_at": "2026-09-18T09:15:00Z"
    }
  ],
  "total": 42
}
```

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `activities` | `array` | Array of activity objects, sorted by `created_at` descending. |
| `activities[].id` | `string` | Activity UUID. |
| `activities[].type` | `string` | Activity type identifier. |
| `activities[].title` | `string` | Human-readable summary (e.g., "Submitted 'Contact Us' form"). Generated from type + data. |
| `activities[].description` | `string` | Brief description (e.g., form name, page title). |
| `activities[].data` | `object` | Type-specific structured data (see §4.4). |
| `activities[].created_at` | `string` | ISO 8601 timestamp. |
| `total` | `integer` | Total number of matching activities (for pagination). |

**Error Responses:**

| Status | Body | Condition |
|--------|------|-----------|
| 400 | `{ "error": "invalid contact ID" }` | `id` is not a valid UUID. |
| 401 | `{ "error": "missing authorization header" }` | No auth token. |
| 403 | `{ "error": "forbidden" }` | Non-admin user trying to access another context's contact. |
| 404 | `{ "error": "contact not found" }` | Contact doesn't exist or user has no access. |

### 5.2 `GET /api/context/contacts/:id` (Extended)

Extend the existing contact detail endpoint to include an `activity_count` field.

**Response (200 OK) — new field added:**

```json
{
  "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
  "context_id": "d4e5f6a7-b8c9-0123-defa-234567890123",
  "email": "jane@example.com",
  "first_name": "Jane",
  "last_name": "Doe",
  "company": "Acme Corp",
  "tags": [
    { "id": "e5f6a7b8-c9d0-1234-efab-345678901234", "name": "lead", "color": "#6366f1" }
  ],
  "source": "website",
  "created_at": "2026-09-15T10:00:00Z",
  "updated_at": "2026-09-20T14:32:00Z",
  "activity_count": 42
}
```

**New Field:**
| Field | Type | Description |
|-------|------|-------------|
| `activity_count` | `integer` | Total number of activities for this contact. |

### 5.3 New Endpoint: `POST /api/context/contacts/:id/record-activity` (Internal)

An internal endpoint used by form submission, tracking, and update flows to create activities. This endpoint is **not** exposed to the public API — it's called directly from Go code (not via HTTP). See §3.2 for usage.

---

## 6. Key Design Decisions

### 6.1 Activity Creation: Synchronous vs Event Bus

**Decision: Synchronous (inline) for v1.**

Activities are created synchronously within the same transaction as the triggering operation (e.g., form submission + activity creation in one DB transaction). This ensures consistency: if the form submission fails, the activity is not created.

**Rationale:**
- The current codebase has no message bus infrastructure.
- Activity creation is low-throughput (one per form submit / update / tag change).
- Simplicity over complexity for v1.

**Future consideration:** If tracking events from the tracking server need to create activities asynchronously, a simple NATS queue or polling-based worker can be added later.

### 6.2 Contact-Tracking Resolution

**Decision: Store cookie-to-contact mapping in `contact_cookie_mappings` table.**

When a contact submits a form, store their tracking cookie value. Later, when tracking events arrive, resolve the cookie to the contact and create an activity.

**Alternative considered:** Embed cookie in `contacts.source_id`. **Rejected** because `source_id` is already used for form submission source and would be overwritten by subsequent form submissions. A dedicated mapping table supports one contact having multiple cookies (e.g., different browsers).

### 6.3 Context Scoping for Admin Users

**Decision: Admin users bypass context check entirely.**

If `is_admin: true` in the JWT, skip the `contact.ContextID == context_id` check. Admin users can view any contact and its activities regardless of the `X-Context-ID` header.

**Rationale:** The PRD explicitly states admin users should see "all tenants, all contacts, all forms, all banners across all tenants" (§5.3). This applies to activity timeline as well.

### 6.4 Activity Data: JSONB vs Dedicated Columns

**Decision: Keep all activity-specific data in the `data` JSONB column.**

Each activity type has a different data shape. Using a single JSONB column avoids a large number of nullable columns and keeps the schema flexible.

**Trade-off:** Queries that need to filter on specific data fields (e.g., "find all page visits to /pricing") require GIN indexes on the JSONB column. This is acceptable for the expected query volume.

**Index recommendation:** Add a GIN index on `contact_activities.data` for full JSONB search:
```sql
CREATE INDEX IF NOT EXISTS idx_activities_data ON contact_activities USING GIN (data);
```

### 6.5 Title & Description Generation

**Decision: Generate `title` and `description` in the repository layer (Go), not in the database.**

The `title` is a human-readable summary derived from the activity type and data. The `description` is a brief contextual label. These are computed in Go when reading from the database.

**Rationale:**
- Activity types are domain constants in Go.
- Generation logic is simple (type switch + data lookup).
- Keeps the database schema clean.
- Title/description don't need to be queryable — they're display-only.

**Implementation:** A helper function `formatActivityTitle(activity Activity) string` and `formatActivityDescription(activity Activity) string` in the repository or usecase layer.

### 6.6 Pagination: Limit/Offset vs Cursor

**Decision: Use limit/offset for v1.**

The existing codebase already uses limit/offset pagination for contacts, forms, and other list endpoints. Consistency is preferred.

**Future consideration:** For very high-activity contacts (10k+ activities), cursor-based pagination could be more efficient. Monitor performance and migrate if needed.

### 6.7 Activity Count Optimization

**Decision: Use a subquery or separate `COUNT(*)` for the `activity_count` field on the contact detail endpoint.**

Two approaches considered:

1. **Subquery in contact select:**
   ```sql
   SELECT c.*, (SELECT COUNT(*) FROM contact_activities ca WHERE ca.contact_id = c.id) AS activity_count
   FROM contacts c WHERE c.id = $1
   ```
   **Preferred** — single query, no extra round trip.

2. **Separate COUNT query:** Two queries (GET contact + COUNT activities). **Rejected** — more round trips.

### 6.8 Type Filter Implementation

**Decision: Add type filter as a WHERE clause on the `type` column in the repository layer.**

```go
func (r *repo) GetActivity(contactID, contextID string, offset, limit int, activityType *string) ([]contact.Activity, int64, error)
```

The `activityType` parameter is a pointer — `nil` means "no filter", non-nil means filter by that type.

### 6.9 Total Count Strategy

**Decision: Use a separate `COUNT(*)` query for the `total` field.**

```go
countQuery := "SELECT COUNT(*) FROM contact_activities WHERE contact_id = $1"
// Add type filter to count query too if present
```

This avoids the overhead of `COUNT(*) OVER ()` which requires scanning the full result set.

---

## 7. Implementation Plan

### Phase 1: Domain & Repository Layer

1. **Extend `ActivityType` constants** in `activity.go` with `banner_impression`, `tag_added`, `tag_removed`.
2. **Add `CreateActivity` method** to the contact repository interface and PostgreSQL implementation.
3. **Extend `GetActivity`** in the repository to accept an optional `activityType` filter and return `(activities, total, error)`.
4. **Update usecase layer**:
   - `GetActivity`: Accept type filter, call updated repo, add title/description formatting, return enriched activity list + total.
   - `GetContact`: After fetching contact, compute `activity_count` (subquery).
5. **Create helper functions** for activity title/description formatting.
6. **Create `contact_cookie_mappings` migration** in `migration.sql`.

### Phase 2: Handler & Server Routing

7. **Register the existing `GetActivity` handler** in `server.go` under `/api/context/contacts/:id/activity`.
8. **Extend `Get` handler** to include `activity_count` in response.
9. **Add context scoping** to `GetActivity`: verify `contact.ContextID == context_id` unless user is admin.
10. **Add type filter parsing** in the handler (query param → usecase → repo).

### Phase 3: Activity Creation Plumbing

11. **Add `CreateActivity` call** in `CreateContact` usecase (after successful insert).
12. **Add `CreateActivity` call** in `UpdateContact` usecase (after successful update, compute `changed_fields`).
13. **Add `CreateActivity` call** in `ApplyTags` usecase (for tag_added / tag_removed).
14. **Add `CreateActivity` call** in `MergeContacts` usecase (for contact_merged).
15. **Add `CreateActivity` call** in form submission handler (for form_submission).
16. **Create tracking activity resolver**: new file `internal/infrastructure/tracking/repo/activity.go` with function `CreateActivityFromEvent(contactID, contextID, eventType string, eventData map[string]interface{})`.
17. **Wire tracking events** to call the activity resolver when a cookie matches a contact.

### Phase 4: Testing

18. **Unit tests** for `CreateActivity`, `GetActivity` (with and without type filter).
19. **Unit tests** for title/description formatting functions.
20. **Integration tests** (Ginkgo + testcontainers) for the full flow: create contact → create activities → retrieve timeline.
21. **HTTP integration tests** for the new endpoints.

### Phase 5: Frontend Integration (out of scope for this spec)

22. **BFF / Vue UI**: Fetch activities via `GET /api/context/contacts/:id/activity?limit=50&type=form_submission`.
23. **Display**: Timeline component with type-based filtering tabs.
24. **Contact detail page**: Show `activity_count` badge.

---

## 8. Open Questions

1. **How should the tracking system resolve cookies to contacts?** Should this be a real-time lookup during event processing, or a batch job that runs periodically?

2. **Should activities be deletable?** If a contact is deleted (CASCADE on the FK), activities are deleted. Should admins be able to delete individual activities? (Probably not — they're an audit trail.)

3. **What is the maximum reasonable activity count per contact?** If a contact has 10,000+ page visits, should the API support lazy loading (infinite scroll) or should we paginate with "load more"?

4. **Should the `source_id` field on activities be used to link back to the original form submission ID or tracking event ID?** This would enable "view source" functionality from an activity item.

5. **Should there be a rate limit on activity creation?** If a contact is tracked heavily (e.g., a bot), could this flood the table? Consider deduplication or throttling.

---

## 9. File Changes Summary

| File | Change |
|------|--------|
| `internal/domain/contact/activity.go` | Add new `ActivityType` constants |
| `internal/domain/contact/repository.go` | Add `CreateActivity`, update `GetActivity` signature |
| `internal/infrastructure/contact/migration.sql` | Add `contact_cookie_mappings` table |
| `internal/infrastructure/contact/repo/postgres.go` | Implement `CreateActivity`, update `GetActivity` with type filter + total count |
| `internal/usecase/contact/get_activity.go` | Add type filter, title/description formatting, activity count on contact |
| `internal/usecase/contact/create_contact.go` | Create `contact_created` activity on insert |
| `internal/usecase/contact/update_contact.go` | Create `contact_updated` activity on update |
| `internal/usecase/contact/tags.go` | Create `tag_added`/`tag_removed` activities on tag changes |
| `internal/usecase/contact/merge_contacts.go` | Create `contact_merged` activity on merge |
| `internal/adapter/api/handler/contact_handler.go` | Register handler, add type filter parsing, add context scoping |
| `internal/adapter/api/server.go` | Register `GET /api/context/contacts/:id/activity` route |
| `internal/infrastructure/tracking/repo/activity.go` | New file: activity creation from tracking events |
| `internal/infrastructure/tracking/repo/postgres.go` | Wire tracking events → activity creation |
| `internal/adapter/proxy/proxy.go` or form handler | Create `form_submission` activity on form submit |
