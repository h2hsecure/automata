# PRD: User Tracking

## Overview

Track website visitors, page views, and events via a JavaScript snippet, store the data, and display it in a dashboard. Target user: small-to-mid business owners who want to understand who visits their website and what they do.

## Goals

- Provide a drop-in JS snippet that tenants embed on their websites
- Capture page views, events, and visitor metadata (referrer, browser, device, geo)
- Display real-time and historical analytics in a dashboard
- GDPR-friendly by default: no PII stored, opt-out mechanism available

## Scope

### In Scope

- JS snippet served by Automata, customizable per tenant
- Tracking API endpoint (`/track`) accepting page views and custom events
- Visitor identification via cookie + browser fingerprint (no auth required)
- Client-side event batching (every 30s or on page unload)
- PostgreSQL storage for events with tenant isolation
- Dashboard: visitor counts, top pages, referrers, real-time activity, device/browser breakdown
- Custom event tracking via JS API (`automata.track('event_name', data)`)
- UTM parameter capture and attribution

### Out of Scope

- Server-side tracking (Phase 2+)
- Advanced funnel analysis
- A/B testing for tracked pages (deferred to Banner Management)
- Real-time collaboration on dashboards

## Architecture

Clean architecture with hexagonal layout. KISS and DRY principles. Tracking is a separate driver adapter.

### Layer Structure

```
internal/
  domain/tracking      — PageView, Event, Visitor (entities, value objects, domain interfaces)
  usecase/tracking     — TrackEvent, GetDashboard, GetEvents
internal/adapter/mcp   — MCP server exposing tracking tools
internal/adapter/api   — Tracking endpoint (public, no auth)
internal/infrastructure/
  tracking/repo        — PostgreSQL repository implementation
  analytics/query      — Aggregation query builders
pkg/snippet            — JS snippet generator (domain-agnostic, shared with Banner Management)
```

### Driver Adapter: Tracking

The tracking endpoint is a separate driver adapter from the API server. It handles high-throughput event ingestion independently from admin API traffic. Both share the same tenant resolution middleware.

### Data Flow

```
Tenant website → JS snippet → POST /track → Event ingestion → PostgreSQL → Aggregation → Dashboard
```

### Key Decisions

- **Visitor identification:** Cookie-based with browser fingerprint fallback. No login required.
- **Event batching:** Client-side batching reduces API calls. Events queued in JS, flushed periodically.
- **Tenant isolation:** All events tagged with `tenant_id`. Queries always scoped to tenant.
- **GDPR compliance:** No PII stored by default. IP address not stored. Optional custom fields for tenant-defined data.
- **Storage retention:** Configurable per tenant via key-value config (`tracking.retention_days`, default: 365).
- **Shared snippet:** `pkg/snippet` is shared between User Tracking and Banner Management. JS snippet can load both tracking and banner modules.

## API Design

### Tracking Endpoint

```
POST /track
Headers: X-Tenant-ID: <tenant-id>
Body: {
  "type": "pageview" | "event",
  "url": "/current-page",
  "title": "Page Title",
  "referrer": "https://google.com",
  "event_name": "click_signup",  // only for type=event
  "properties": {},               // custom event data
  "visitor_id": "abc123",         // from cookie, auto-set by snippet
  "timestamp": "2025-01-01T00:00:00Z"
}
```

### Snippet Endpoint

```
GET /snippet/<tenant-id>.js
Returns: JavaScript snippet for embedding
```

## Database Schema (tentative)

```sql
-- Events table
CREATE TABLE events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    visitor_id    VARCHAR(128),
    type          VARCHAR(32) NOT NULL,  -- pageview, event
    url           VARCHAR(2048),
    title         VARCHAR(512),
    referrer      VARCHAR(2048),
    event_name    VARCHAR(128),
    properties    JSONB DEFAULT '{}',
    user_agent    VARCHAR(512),
    ip_hash       VARCHAR(64),           -- hashed, not stored raw
    utm_source    VARCHAR(128),
    utm_medium    VARCHAR(128),
    utm_campaign  VARCHAR(128),
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_events_tenant_created ON events(tenant_id, created_at DESC);
CREATE INDEX idx_events_visitor ON events(tenant_id, visitor_id);
```

## Testing

- Unit tests for event validation and tenant resolution
- Integration tests for tracking endpoint with mock tenant
- E2E tests for API and MCP layers using Ginkgo + testcontainers, executed via `cicd/` directory
- Load test: simulate 1000 events/sec ingestion

## Success Criteria

- JS snippet loads in < 100ms, tracks page view within 200ms of page load
- Dashboard loads aggregated metrics in < 2 seconds for up to 100k events
- Event ingestion handles 100 events/sec sustained without errors
- Visitor identification accuracy > 95% (same browser = same visitor)
