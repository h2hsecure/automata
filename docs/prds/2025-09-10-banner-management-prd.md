# PRD: Banner Management

## Overview

Create, manage, and deploy visual banners/ads to tenant websites. Target user: small-to-mid business owners who want to display promotional content, announcements, or calls-to-action on their sites.

## Goals

- Provide a banner editor for creating visual content (HTML/text/images)
- Define placement rules for where banners appear on tenant sites
- Serve banners to tenant sites via the existing JS snippet
- Track banner impressions and clicks for basic A/B testing

## Scope

### In Scope

- Banner CRUD: title, content (HTML or text), images, target URL, CTA button
- Placement types: header bar, sidebar widget, popup/modal, inline (content injection)
- Placement rules: per-tenant priority ordering, scheduling (start/end dates)
- A/B testing: create variants, track impressions and clicks per variant
- Banner JS loaded alongside tracking snippet via existing `pkg/snippet`
- Dashboard: banner performance (impressions, clicks, CTR)

### Out of Scope

- Audience targeting / segmentation (deferred)
- Automated creative generation (deferred)
- Advanced animation / carousel support (deferred)
- Third-party ad network integration (deferred)
- Server-side banner rendering (client-side only for now)

## Architecture

Clean architecture with hexagonal layout. KISS and DRY principles.

### Layer Structure

```
internal/
  domain/banner        — Banner, Placement, Variant, Impression, Click (entities, value objects, domain interfaces)
  usecase/banner       — CreateBanner, SetPlacements, EvaluatePlacements
internal/adapter/mcp   — MCP server exposing banner tools
internal/infrastructure/
  banner/repo          — PostgreSQL repository implementation
pkg/snippet            — JS snippet generator (shared with User Tracking)
pkg/banner/render      — HTML/JS rendering for tenant sites
```

### Data Flow

```
Tenant creates banner → Placement rules defined → Banner JS loaded on tenant site → JS evaluates rules → Banner rendered → Impression tracked → Click tracked
```

### Key Decisions

- **HTML content:** Banners stored as HTML content for maximum flexibility (not just images). Tenant writes or pastes HTML.
- **Placement priority:** Multiple placements can apply to the same page. Priority ordering determines which banner shows when they conflict.
- **A/B variants:** Variants share the same placement rules. JS randomly selects a variant per visitor. Impressions and clicks tracked per variant.
- **Scheduling:** Simple start/end date filtering. No complex audience or time-of-day rules.
- **Tracking integration:** Impressions and clicks use the same event pipeline as User Tracking (PageView/Event models extended with banner context). Shared via `internal/domain/tracking`.
- **Shared snippet:** `pkg/snippet` is shared between User Tracking and Banner Management. JS snippet can load both tracking and banner modules.
- **KV config:** Placement settings stored via key-value config (`banner.placement.<type>.default_priority`, `banner.ab.enabled`).

## API Design

### Banner CRUD

```
GET    /api/banners                        # List banners (tenant-scoped)
POST   /api/banners                        # Create banner
GET    /api/banners/<id>                   # Get banner details
PUT    /api/banners/<id>                   # Update banner
DELETE /api/banners/<id>                   # Delete banner
POST   /api/banners/<id>/duplicate         # Duplicate banner
```

### Placement Rules

```
PUT /api/banners/<id>/placements
Body: {
  "placements": [
    { "type": "header", "priority": 1, "start_date": "2025-01-01", "end_date": "2025-12-31" },
    { "type": "popup", "priority": 2, "start_date": "2025-01-01" }
  ]
}
```

### A/B Variants

```
POST /api/banners/<id>/variants
Body: {
  "name": "Variant A",
  "content": "<h1>Special Offer!</h1>"
}
```

### Banner Events (extends tracking)

```
POST /track
Body: {
  "type": "banner_impression" | "banner_click",
  "banner_id": "uuid",
  "variant_id": "uuid",       // optional, for A/B
  "placement": "header",
  "url": "/current-page",
  "visitor_id": "abc123"
}
```

## Database Schema (tentative)

```sql
-- Banners table
CREATE TABLE banners (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    name        TEXT NOT NULL,
    content     HTML,            -- HTML content
    image_url   VARCHAR(2048),   -- optional cover image
    target_url  VARCHAR(2048),   -- link destination
    cta_text    VARCHAR(128),    -- CTA button text
    cta_url     VARCHAR(2048),   -- optional override CTA URL
    is_active   BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_banners_tenant_active ON banners(tenant_id, is_active);

-- Placements table
CREATE TABLE banner_placements (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    banner_id   UUID NOT NULL REFERENCES banners(id) ON DELETE CASCADE,
    tenant_id   UUID NOT NULL,
    type        VARCHAR(32) NOT NULL,  -- header, sidebar, popup, inline
    priority    INT NOT NULL DEFAULT 0,
    start_date  DATE,
    end_date    DATE,
    settings    JSONB DEFAULT '{}'     -- { frequency, dismissible, etc. }
);

CREATE INDEX idx_placements_banner ON banner_placements(banner_id);

-- Variants table (for A/B testing)
CREATE TABLE banner_variants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    banner_id   UUID NOT NULL REFERENCES banners(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    content     HTML NOT NULL,
    weight      INT DEFAULT 1,         -- relative weight for random selection
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_variants_banner ON banner_variants(banner_id);

-- Impressions table (extends events table concept)
CREATE TABLE banner_impressions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    banner_id   UUID NOT NULL,
    variant_id  UUID REFERENCES banner_variants(id),
    placement   VARCHAR(32),
    visitor_id  VARCHAR(128),
    url         VARCHAR(2048),
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_impressions_banner ON banner_impressions(banner_id, created_at DESC);

-- Clicks table
CREATE TABLE banner_clicks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    banner_id   UUID NOT NULL,
    variant_id  UUID REFERENCES banner_variants(id),
    visitor_id  VARCHAR(128),
    url         VARCHAR(2048),
    target_url  VARCHAR(2048),
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_clicks_banner ON banner_clicks(banner_id, created_at DESC);
```

## Testing

- Unit tests for placement rule evaluation (priority, scheduling)
- Integration tests for banner rendering via JS snippet
- A/B variant selection tests (random distribution within tolerance)
- E2E tests for API and MCP layers using Ginkgo + testcontainers, executed via `cicd/` directory
- Click tracking test: verify click events fire and are stored

## Success Criteria

- Banner renders within 100ms of JS snippet load
- A/B variant distribution within 10% of expected weight after 1000 impressions
- Impression tracking accuracy > 99% (every rendered banner generates an impression event)
- Dashboard shows CTR and performance metrics within 5 minutes of events
