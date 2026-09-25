# Design: Banner Management

## Problem Statement

Marketing automation requires a system to create, manage, and serve promotional banners on client websites. Banners drive user engagement through targeted messaging, A/B testing, and campaign tracking. The system must support multiple banner types, placements, A/B testing with traffic splits, frequency capping, and impression/click tracking via transparent beacon pixels. Banner serving is a high-throughput operation that must be architecturally separated from the main API server to handle concurrent requests efficiently.

## Goals

1. Support multiple banner types (HTML, image, video, popup, sticky) with content stored as JSONB
2. Define placements with CSS selectors for targeted injection on client pages
3. Implement A/B testing with configurable traffic weights and variant selection
4. Track impressions via 1×1 transparent beacon pixels and clicks via redirect tracking
5. Provide campaign management with aggregate statistics (impressions, clicks, CTR)
6. Serve a shared JavaScript snippet that handles banner loading and event tracking
7. Run a separate high-throughput banner serving server for production traffic
8. All data strictly isolated per context (tenant)

## Non-Goals

- Server-side banner rendering (client-side JS injection only)
- Real-time banner performance monitoring dashboard
- Geo-targeting or demographic targeting
- Automated creative generation
- Banner approval workflows

## Architecture

### Component Diagram

```
┌────────────────────────────────────────────────────────────────────┐
│                     Banner Architecture                            │
│                                                                    │
│  ┌─────────────────┐    ┌──────────────────┐    ┌──────────────┐  │
│  │ Management API   │    │ Banner Server    │    │ JS Snippet   │  │
│  │ (Gin, /api/*)    │    │ (Gin, high-TPS)  │    │ Generator    │  │
│  │                   │    │                  │    │ (pkg/snippet)│  │
│  │ POST /banners     │    │ GET /placements  │    │              │  │
│  │ GET /banners      │    │ GET /track/banner│    │ Generates:   │  │
│  │ PUT /banners/:id  │    │ GET /snippet/:id│    │ - tenant ID  │  │
│  │ DELETE /banners   │    │ (beacon pixel)   │    │ - API host   │  │
│  │                   │    │                  │    │ - tracking   │  │
│  └────────┬──────────┘    └────────┬─────────┘    └──────────────┘  │
│           │                        │                                │
│  ┌────────┴────────────────────────┼─────────────────────────┐     │
│  │ Use Cases                       │ Domain Layer             │     │
│  │ CreateBanner, ListBanners       │ Banner entity            │     │
│ │ GetPlacement, GetCampaignStats   │ Placement entity         │     │
│ │ GetActiveBanners                 │ Campaign entity          │     │
│ │ RecordImpression, RecordClick    │ Repository interface     │     │
│  └────────┬────────────────────────┴─────────────────────────┘     │
│           │                                                         │
│  ┌────────┴───────────────────────────────────────────────────────┐│
│  │ Infrastructure: PostgreSQL Repository                           ││
│  │ migration.sql (banner_banners, banner_placements,               ││
│  │ banner_campaigns, banner_impressions, banner_clicks)            ││
│  │ repo/postgres.go                                                 ││
│  └────────────────────────────────────────────────────────────────┘│
└────────────────────────────────────────────────────────────────────┘
```

### Banner Server vs Management API

The banner serving endpoint (`GET /api/banners/placements`, `GET /track/banner`, `GET /snippet/:id.js`) runs as a separate high-throughput driver adapter from the management API. This separation ensures:

- Banner serving is not degraded by management API traffic
- Beacon pixel responses are minimal (1×1 PNG, no JSON overhead)
- Tenant resolution uses `X-Tenant-ID` header or subdomain extraction

### Banner Serving Flow

```
Client JS snippet loads
       │
       ▼
GET /api/banners/placements?context_id=xxx
       │
       ▼
Server queries active banners for each placement
       │
       ▼
Returns placements[] with banners[] (JSON)
       │
       ▼
Client JS injects banner content into DOM
       │
       ▼
On display: beacon pixel fires GET /track/banner?banner_id=xxx&type=impression
       │
       ▼
Server records impression, returns 1×1 PNG
       │
       ▼
On click: redirect through /track/banner?banner_id=xxx&type=click→link_url
       │
       ▼
Server records click, redirects to link_url
```

### A/B Testing

A/B variants are stored as a JSONB array on the banner entity. When a variant is selected:
1. Weights (0-100) determine probability distribution
2. A deterministic hash of `visitor_id + banner_id + date` selects the variant
3. Selected variant's content is served instead of the base banner
4. Impressions and clicks are tracked against the selected variant

### GDPR Consent

Banner serving is gated by a consent flag in the banner settings. If `gdpr_consent_required` is true, the snippet checks for a consent cookie before injecting banners.

## Data Model

### Tables

```sql
-- Banners: promotional content pieces
CREATE TABLE banner_banners (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id    UUID NOT NULL,
    name          VARCHAR(255) NOT NULL,
    type          VARCHAR(32) NOT NULL,  -- html, image, video, popup, sticky
    content       TEXT,                  -- HTML content or image URL
    link_url      VARCHAR(2048),         -- destination URL on click
    image_url     VARCHAR(2048),
    alt_text      VARCHAR(512),
    campaign_id   UUID,
    placements    JSONB DEFAULT '[]',    -- placement IDs as JSON array
    priority      INTEGER DEFAULT 0,     -- higher = shown first
    start_date    TIMESTAMPTZ,
    end_date      TIMESTAMPTZ,
    is_active     BOOLEAN DEFAULT TRUE,
    ab_test       BOOLEAN DEFAULT FALSE,
    ab_variants   JSONB DEFAULT '[]',    -- [{id, name, content, weight, priority}]
    impressions   BIGINT DEFAULT 0,
    clicks        BIGINT DEFAULT 0,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_banners_context_active ON banner_banners(context_id, is_active, priority DESC);
CREATE INDEX idx_banners_campaign ON banner_banners(context_id, campaign_id);
CREATE INDEX idx_banners_dates ON banner_banners(context_id, start_date, end_date);

-- Placements: where on a page banners appear
CREATE TABLE banner_placements (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id    UUID NOT NULL,
    name          VARCHAR(255) NOT NULL,
    location      VARCHAR(128),          -- header, footer, sidebar, popup
    css_selector  VARCHAR(512),          -- CSS selector for injection
    max_banners   INTEGER DEFAULT 1,
    priority      INTEGER DEFAULT 0,
    is_active     BOOLEAN DEFAULT TRUE,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

-- Campaigns: grouping of banners for tracking
CREATE TABLE banner_campaigns (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id      UUID NOT NULL,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    start_date      TIMESTAMPTZ,
    end_date        TIMESTAMPTZ,
    is_active       BOOLEAN DEFAULT TRUE,
    target_url      VARCHAR(2048),
    tracking_code   VARCHAR(512),
    impressions     BIGINT DEFAULT 0,
    clicks          BIGINT DEFAULT 0,
    conversion_rate FLOAT8 DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Impressions: detailed tracking log
CREATE TABLE banner_impressions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    banner_id     UUID NOT NULL REFERENCES banner_banners(id) ON DELETE CASCADE,
    context_id    UUID NOT NULL,
    visitor_id    VARCHAR(128),
    placement_id  UUID,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- Clicks: detailed click tracking log
CREATE TABLE banner_clicks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    banner_id     UUID NOT NULL REFERENCES banner_banners(id) ON DELETE CASCADE,
    context_id    UUID NOT NULL,
    visitor_id    VARCHAR(128),
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_impressions_banner ON banner_impressions(banner_id, created_at);
CREATE INDEX idx_clicks_banner ON banner_clicks(banner_id, created_at);
```

### Domain Entities

```go
type BannerType string
const (
    BannerTypeHTML    BannerType = "html"
    BannerTypeImage   BannerType = "image"
    BannerTypeVideo   BannerType = "video"
    BannerTypePopup   BannerType = "popup"
    BannerTypeSticky  BannerType = "sticky"
)

type Banner struct {
    ID          string
    ContextID   string
    Name        string
    Type        BannerType
    Content     string
    LinkURL     string
    ImageURL    string
    AltText     string
    CampaignID  string
    Placements  []string
    Priority    int
    StartDate   time.Time
    EndDate     time.Time
    IsActive    bool
    ABTest      bool
    ABVariants  []ABVariant
    Impressions int64
    Clicks      int64
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type ABVariant struct {
    ID       string
    Name     string
    Content  string
    LinkURL  string
    Weight   int   // percentage (0-100)
    Priority int
}

type Placement struct {
    ID           string
    ContextID    string
    Name         string
    Location     string
    CSSSelector  string
    MaxBanners   int
    Priority     int
    IsActive     bool
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type Campaign struct {
    ID              string
    ContextID       string
    Name            string
    Description     string
    StartDate       time.Time
    EndDate         time.Time
    IsActive        bool
    TargetURL       string
    TrackingCode    string
    Impressions     int64
    Clicks          int64
    ConversionRate  float64
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type BannerStats struct {
    Impressions int64
    Clicks      int64
    CTR         float64  // clicks / impressions
}

type CampaignStats struct {
    TotalImpressions int64
    TotalClicks      int64
    ConversionRate   float64
    DailyImpressions []DailyStat
    DailyClicks      []DailyStat
}
```

## API Contracts

### Management API (Authenticated)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/banners` | Create banner |
| `GET` | `/api/banners` | List banners (filter by campaign_id, is_active, pagination) |
| `GET` | `/api/banners/:id` | Get banner details |
| `PUT` | `/api/banners/:id` | Update banner |
| `DELETE` | `/api/banners/:id` | Delete banner |
| `POST` | `/api/placements` | Create placement |
| `GET` | `/api/placements` | List placements |
| `GET` | `/api/placements/:id` | Get placement |
| `PUT` | `/api/placements/:id` | Update placement |
| `DELETE` | `/api/placements/:id` | Delete placement |
| `POST` | `/api/campaigns` | Create campaign |
| `GET` | `/api/campaigns` | List campaigns |
| `GET` | `/api/campaigns/:id` | Get campaign with stats |
| `PUT` | `/api/campaigns/:id` | Update campaign |
| `DELETE` | `/api/campaigns/:id` | Delete campaign |

### Banner Serving API (Public)

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `GET` | `/api/banners/placements` | Get placements with active banners | X-Tenant-ID header |
| `GET` | `/track/banner?banner_id=xxx&type=impression` | Record impression + 1×1 PNG | X-Tenant-ID |
| `GET` | `/track/banner?banner_id=xxx&type=click` | Record click + redirect | X-Tenant-ID |
| `GET` | `/snippet/banner/:id.js` | Serve JavaScript snippet | None (public) |

### Request/Response Shapes

```json
// POST /api/banners
{
  "name": "Summer Sale Banner",
  "type": "html",
  "content": "<div class='banner'>Summer Sale - 20% off!</div>",
  "link_url": "https://example.com/summer-sale",
  "placements": ["header", "footer"],
  "campaign_id": "uuid",
  "is_active": true,
  "ab_test": true,
  "ab_variants": [
    {"name": "Variant A", "content": "...", "weight": 50},
    {"name": "Variant B", "content": "...", "weight": 50}
  ],
  "start_date": "2026-06-01T00:00:00Z",
  "end_date": "2026-08-31T23:59:59Z"
}
→ 201 { "banner": { ... } }

// GET /api/banners/placements (public, X-Tenant-ID: xxx)
{
  "placements": [
    {
      "id": "uuid",
      "name": "Header",
      "css_selector": ".header-banner-area",
      "banners": [
        {
          "id": "uuid",
          "name": "Summer Sale",
          "content": "<div class='banner'>...</div>",
          "link_url": "https://...",
          "type": "html"
        }
      ]
    }
  ]
}
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Separate banner server | Dedicated high-throughput server | Beacon pixels and banner serving get priority over management API |
| Banner content in JSONB/TEXT | TEXT column for content | HTML content can be large; JSONB for structured fields (placements, variants) |
| Impression tracking | 1×1 transparent PNG beacon | Universal browser support; no JavaScript dependency for tracking |
| A/B testing | JSONB variants with weights | No schema changes needed to add/remove variants; deterministic selection |
| Placement CSS selector | CSS selector string | Flexible targeting without server-side rendering |
| Shared snippet generator | pkg/snippet shared with tracking | DRY; single code path for tenant ID and API host injection |
| Context isolation | No FK to contexts (by design in migration) | Tables have context_id column; foreign key constraint omitted for migration compatibility |
| Click tracking | Separate table from impressions | Enables distinct click analysis; follows impression/click as separate events |
