# Design: User Tracking

## Problem Statement

Marketing automation platforms need to track visitor behavior on client websites to understand traffic sources, page performance, and user engagement. Without a tracking system, there is no visibility into who visits a website, which pages are popular, where traffic originates, or how campaigns perform. The system must capture page views and custom events, identify visitors via cookies and browser fingerprints, and provide dashboard metrics for analytics. Tracking is a high-throughput data ingestion system that must be architecturally separated from the main API.

## Goals

1. Capture page views and custom events from client websites via a JavaScript snippet
2. Identify visitors using cookie-based identification with browser fingerprinting fallback
3. Support batch event ingestion for efficiency (every 30s or on page unload)
4. Provide analytics dashboard metrics: total/active visitors, top pages, referrers, device/browser breakdown
5. Support UTM parameter capture for campaign attribution
6. Enforce GDPR-friendly data practices (no raw IP storage, configurable retention)
7. Serve a shared JavaScript snippet for self-hosted installation
8. All data strictly isolated per context (tenant)

## Non-Goals

- Real-time analytics dashboard (polling every 30s is acceptable)
- Server-side tracking (only client-side JS snippet)
- Session replay or heatmaps
- Advanced funnel analysis or cohort analysis
- Websocket-based real-time updates

## Architecture

### Component Diagram

```
┌────────────────────────────────────────────────────────────────────┐
│                     Tracking Architecture                           │
│                                                                     │
│  ┌─────────────────┐    ┌────────────────────┐    ┌──────────────┐ │
│  │ Management API   │    │ Tracking Server     │    │ JS Snippet   │ │
│  │ (Gin, /api/*)    │    │ (Gin, high-TPS)    │    │ Generator    │ │
│  │                  │    │                     │    │ (pkg/snippet)│ │
│  │ GET /dashboard   │    │ POST /track         │    │              │ │
│  │ GET /events      │    │ POST /track/batch   │    │ - Visitor ID │ │
│  │                  │    │ GET /snippet/:id.js │    │ - Event batch│ │
│  └────────┬─────────┘    └────────┬───────────┘    └──────────────┘ │
│           │                       │                                 │
│  ┌────────┴───────────────────────┼─────────────────────────┐      │
│  │ Use Cases                       │ Domain Layer             │      │
│  │ TrackEvent, GetDashboard        │ Event entity             │      │
│  │ GetEvents                       │ Visitor entity           │      │
│  │                               │ Analytics types          │      │
│  │                               │ Repository interface     │      │
│  └────────┬────────────────────────┴─────────────────────────┘      │
│           │                                                          │
│  ┌────────┴────────────────────────────────────────────────────────┐│
│  │ Infrastructure: PostgreSQL Repository                           ││
│  │ migration.sql (tracking_visitors, tracking_events)              ││
│  │ repo/postgres.go                                                 ││
│  └────────────────────────────────────────────────────────────────┘│
└────────────────────────────────────────────────────────────────────┘
```

### Tracking Server vs Management API

The tracking endpoint (`POST /track`, `POST /track/batch`, `GET /snippet/:id.js`) runs as a separate high-throughput driver adapter from the management API. This separation ensures:

- Event ingestion is not blocked by API request processing
- Batch endpoints support efficient bulk inserts via `pgx.CopyFrom`
- Snippet generation is a simple read operation with no auth

### Visitor Identification

```
1. Browser loads snippet
2. Check for automata_visitor cookie
3. If missing: generate random ID, set cookie (max-age: 365 days)
4. Include visitor_id in all event payloads
5. Browser fingerprint computed on server as dedup fallback
```

### Event Batching

The client-side snippet collects events in a local array and flushes them every 30 seconds or when the page is about to unload:

```javascript
// Event batching in snippet
var events = [];
setInterval(function() {
    if (events.length > 0) {
        fetch('/track/batch', { method: 'POST', body: JSON.stringify({events}) });
        events = [];
    }
}, 30000);
```

### GDPR Compliance

- IP address is hashed (SHA-256) before storage — never stored raw
- No PII stored by default (name, email not captured by tracking)
- Storage retention is configurable via admin config (default: 365 days)
- `PurgeOldEvents` use case deletes events older than retention period

## Data Model

### Tables

```sql
-- Visitors: identified website visitors
CREATE TABLE tracking_visitors (
    id              VARCHAR(128) PRIMARY KEY,  -- visitor_id from cookie
    context_id      UUID NOT NULL,
    cookie_value    VARCHAR(128) NOT NULL,
    fingerprint     VARCHAR(64),              -- browser fingerprint hash
    first_seen      TIMESTAMPTZ DEFAULT NOW(),
    last_seen       TIMESTAMPTZ DEFAULT NOW(),
    page_views      INT DEFAULT 0,
    UNIQUE(context_id, cookie_value)
);

CREATE INDEX idx_tracking_visitors_context ON tracking_visitors(context_id);

-- Events: tracked page views and custom events
CREATE TABLE tracking_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id      UUID NOT NULL,
    visitor_id      VARCHAR(128),
    type            VARCHAR(32) NOT NULL,     -- pageview, event
    url             VARCHAR(2048),
    title           VARCHAR(512),
    referrer        VARCHAR(2048),
    event_name      VARCHAR(128),
    properties      JSONB DEFAULT '{}',
    user_agent      VARCHAR(512),
    ip_hash         VARCHAR(64),              -- SHA-256 of IP, not raw
    utm_source      VARCHAR(128),
    utm_medium      VARCHAR(128),
    utm_campaign    VARCHAR(128),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_tracking_events_context_created ON tracking_events(context_id, created_at DESC);
CREATE INDEX idx_tracking_events_visitor ON tracking_events(context_id, visitor_id);
CREATE INDEX idx_tracking_events_type ON tracking_events(context_id, type);
CREATE INDEX idx_tracking_events_url ON tracking_events(context_id, url) WHERE type = 'pageview';
```

### Domain Entities

```go
type EventType string
const (
    EventTypePageView EventType = "pageview"
    EventTypeEvent    EventType = "event"
)

type Event struct {
    ID           string
    ContextID    string
    VisitorID    string
    Type         EventType
    URL          string
    Title        string
    Referrer     string
    EventName    string
    Properties   map[string]interface{}  // JSONB
    UserAgent    string
    IPHash       string                  // SHA-256 of IP
    UTMSource    string
    UTMMedium    string
    UTMCampaign  string
    CreatedAt    time.Time
}

type Visitor struct {
    ID           string  // visitor_id
    ContextID    string
    CookieValue  string
    Fingerprint  string
    FirstSeen    time.Time
    LastSeen     time.Time
    PageViews    int
}

type DashboardMetrics struct {
    TotalVisitors    int64
    ActiveVisitors   int64  // last 30 minutes
    PageViews        int64
    TopPages         []PageViewCount
    TopReferrers     []ReferrerCount
    DeviceBreakdown  map[string]int
    BrowserBreakdown map[string]int
}

type PageViewCount struct {
    URL   string
    Count int64
}

type ReferrerCount struct {
    Referrer string
    Count    int64
}
```

## API Contracts

### Management API (Authenticated)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/tracking/dashboard?start=...&end=...` | Get dashboard metrics |
| `GET` | `/api/tracking/events?type=pageview&limit=50` | List events (paginated) |

### Tracking API (Public)

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `POST` | `/track` | Track a single event | X-Tenant-ID header |
| `POST` | `/track/batch` | Track batched events | X-Tenant-ID header |
| `GET` | `/snippet/:context-id.js` | Serve tracking JavaScript snippet | None |

### Request/Response Shapes

```json
// POST /track
{
  "type": "pageview",
  "url": "/products/widget",
  "title": "Widget - Our Products",
  "referrer": "https://google.com/search?q=widgets",
  "visitor_id": "abc123",
  "utm_source": "google",
  "utm_medium": "cpc",
  "utm_campaign": "summer-sale"
}
→ 204 No Content

// POST /track/batch
{
  "events": [
    { "type": "pageview", "url": "/home", "visitor_id": "abc123" },
    { "type": "event", "event_name": "add_to_cart", "properties": {"product_id": "w-001"}, "visitor_id": "abc123" }
  ]
}
→ 204 No Content

// GET /api/tracking/dashboard?start=2026-09-01&end=2026-09-30
{
  "total_visitors": 15230,
  "active_visitors": 42,
  "page_views": 87450,
  "top_pages": [
    { "url": "/products/widget", "count": 12340 },
    { "url": "/pricing", "count": 8920 }
  ],
  "top_referrers": [
    { "referrer": "https://google.com", "count": 23400 },
    { "referrer": "https://twitter.com", "count": 5600 }
  ],
  "device_breakdown": { "desktop": 68, "mobile": 28, "tablet": 4 },
  "browser_breakdown": { "Chrome": 72, "Safari": 18, "Firefox": 8 }
}
```

### MCP Tools

```
tracking.get_dashboard(context_id, start, end) → DashboardMetrics
tracking.get_events(context_id, type, limit, offset) → Event[]
tracking.track(event) → void
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Visitor ID | Cookie-based (automata_visitor) | Universal; survives page reloads; no server state needed |
| Fingerprint fallback | Computed on server | Helps dedup when cookies are cleared |
| IP storage | SHA-256 hash only | GDPR compliance; no raw IP in database |
| Event batching | Client-side array + 30s interval | Reduces HTTP overhead; graceful on page unload |
| Analytics aggregation | SQL GROUP BY + PostgreSQL JSONB | No external analytics engine needed; scales to ~100k events |
| Storage retention | Configurable via admin config | Per-tenant control; automatic purge via scheduled job |
| High-throughput server | Separate driver adapter | Event ingestion is read-heavy and high-volume |
| Shared snippet | pkg/snippet shared with banner | DRY; single generator with configurable options |
