# Banner Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build banner CRUD, placement system, JS snippet injection, and banner tracking integration.

**Architecture:** Hexagonal architecture with domain entities for Banner, Placement, and Campaign. Banner management is a separate high-throughput driver adapter from the API server. Shared JS snippet generator in pkg/snippet (same as User Tracking).

**Tech Stack:** Go 1.26+, PostgreSQL, Ginkgo + testcontainers

**Spec:** docs/prds/2025-09-10-banner-management-prd.md

## Global Constraints

- Hexagonal architecture: domain interfaces in `internal/domain`, use cases in `internal/usecase`, adapters in `internal/adapter`, infrastructure in `internal/infrastructure`
- KISS and DRY principles throughout
- Multi-tenant: all tables include `tenant_id`, data strictly isolated per tenant
- Banner serving is a separate driver adapter from API server (high-throughput)
- Shared snippet: pkg/snippet shared with User Tracking
- Banner tracking events sent to User Tracking system
- Banner impressions/clicks tracked via beacon (1x1 transparent pixel)
- A/B testing with configurable traffic split (50/50, 70/30, etc.)
- Banner rotation with configurable frequency cap (e.g., max 3 per day)
- GDPR consent required before showing banners (configurable per tenant)

---

### Task 1: Banner domain entity and repository interface

**Files:**
- Create: `internal/domain/banner/banner.go`
- Create: `internal/domain/banner/placement.go`
- Create: `internal/domain/banner/campaign.go`
- Create: `internal/domain/banner/repository.go`

**Interfaces:**
- Consumes: none (domain layer)
- Produces: `Banner` entity, `Placement` entity, `Campaign` entity, `Repository` interface

- [ ] **Step 1: Define Banner entity**

```go
package banner

import "time"

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
    TenantID    string
    Name        string
    Type        BannerType
    Content     string // HTML content or image URL
    LinkURL     string
    ImageURL    string
    AltText     string
    CampaignID  string
    Placements  []string // placement IDs where this banner appears
    Priority    int      // higher = shown first
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
    Weight   int // percentage (0-100)
    Priority int
}
```

- [ ] **Step 2: Define Placement entity**

```go
package banner

import "time"

type Placement struct {
    ID          string
    TenantID    string
    Name        string
    Location    string // e.g., "header", "footer", "sidebar", "popup"
    CSSSelector string // CSS selector for injection
    MaxBanners  int    // max banners shown at this placement
    Priority    int    // placement priority
    IsActive    bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

- [ ] **Step 3: Define Campaign entity**

```go
package banner

import "time"

type Campaign struct {
    ID            string
    TenantID      string
    Name          string
    Description   string
    StartDate     time.Time
    EndDate       time.Time
    IsActive      bool
    TargetURL     string // landing page after click
    TrackingCode  string // UTM parameters
    Impressions   int64
    Clicks        int64
    ConversionRate float64
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

- [ ] **Step 4: Define Repository interface**

```go
package banner

import "time"

type DateRange struct {
    Start time.Time
    End   time.Time
}

type Repository interface {
    CreateBanner(b *Banner) error
    UpdateBanner(b *Banner) error
    GetBanner(tenantID, bannerID string) (*Banner, error)
    ListBanners(tenantID string, opts ListOptions) ([]*Banner, int64, error)
    DeleteBanner(tenantID, bannerID string) error
    CreatePlacement(p *Placement) error
    UpdatePlacement(p *Placement) error
    GetPlacement(tenantID, placementID string) (*Placement, error)
    ListPlacements(tenantID string, isActive bool) ([]*Placement, error)
    DeletePlacement(tenantID, placementID string) error
    CreateCampaign(c *Campaign) error
    UpdateCampaign(c *Campaign) error
    GetCampaign(tenantID, campaignID string) (*Campaign, error)
    ListCampaigns(tenantID string, isActive bool) ([]*Campaign, error)
    DeleteCampaign(tenantID, campaignID string) error
    GetActiveBannersForPlacement(tenantID, placementID string, date time.Time) ([]*Banner, error)
    RecordImpression(bannerID, visitorID string) error
    RecordClick(bannerID, visitorID string) error
    GetCampaignStats(tenantID string, campaignID string, dateRange DateRange) (*CampaignStats, error)
    GetBannerStats(tenantID string, bannerID string, dateRange DateRange) (*BannerStats, error)
}

type ListOptions struct {
    CampaignID string
    IsActive   bool
    Offset     int
    Limit      int
}

type CampaignStats struct {
    TotalImpressions int64
    TotalClicks      int64
    ConversionRate   float64
    DailyImpressions []DailyStat
    DailyClicks      []DailyStat
}

type BannerStats struct {
    Impressions int64
    Clicks      int64
    CTR         float64
}

type DailyStat struct {
    Date  time.Time
    Count int64
}
```

- [ ] **Step 5: Write entity tests**

```go
func TestBannerIsActive(t *testing.T) {
    now := time.Now()
    b := &Banner{StartDate: now.Add(-24 * time.Hour), EndDate: now.Add(24 * time.Hour), IsActive: true}
    if !b.IsActive {
        t.Error("banner should be active")
    }
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/domain/banner/... -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/domain/banner/
git commit -m "feat: add banner domain entities: Banner, Placement, Campaign, Repository interface"
```

---

### Task 2: Banner PostgreSQL repository

**Files:**
- Create: `internal/infrastructure/banner/repo/postgres.go`
- Create: `internal/infrastructure/banner/repo/postgres_test.go`
- Create: `internal/infrastructure/banner/repo/migration.sql`

**Interfaces:**
- Consumes: `database.NewPool()` from core platform
- Produces: Full Repository implementation with banner serving logic and stats aggregation

- [ ] **Step 1: Write migration SQL**

```sql
CREATE TABLE banners (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    name          VARCHAR(255) NOT NULL,
    type          VARCHAR(32) NOT NULL,
    content       TEXT,
    link_url      VARCHAR(2048),
    image_url     VARCHAR(2048),
    alt_text      VARCHAR(512),
    campaign_id   UUID,
    placements    JSONB DEFAULT '[]',
    priority      INTEGER DEFAULT 0,
    start_date    TIMESTAMPTZ,
    end_date      TIMESTAMPTZ,
    is_active     BOOLEAN DEFAULT TRUE,
    ab_test       BOOLEAN DEFAULT FALSE,
    ab_variants   JSONB DEFAULT '[]',
    impressions   BIGINT DEFAULT 0,
    clicks        BIGINT DEFAULT 0,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE placements (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    name          VARCHAR(255) NOT NULL,
    location      VARCHAR(128),
    css_selector  VARCHAR(512),
    max_banners   INTEGER DEFAULT 1,
    priority      INTEGER DEFAULT 0,
    is_active     BOOLEAN DEFAULT TRUE,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE campaigns (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
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

CREATE TABLE banner_impressions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    banner_id     UUID NOT NULL,
    tenant_id     UUID NOT NULL,
    visitor_id    VARCHAR(128),
    placement_id  UUID,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE banner_clicks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    banner_id     UUID NOT NULL,
    tenant_id     UUID NOT NULL,
    visitor_id    VARCHAR(128),
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_banners_tenant_active ON banners(tenant_id, is_active, priority DESC);
CREATE INDEX idx_banners_campaign ON banners(tenant_id, campaign_id);
CREATE INDEX idx_banners_dates ON banners(tenant_id, start_date, end_date);
CREATE INDEX idx_impressions_banner ON banner_impressions(banner_id, created_at);
CREATE INDEX idx_clicks_banner ON banner_clicks(banner_id, created_at);
```

- [ ] **Step 2: Implement banner CRUD**

```go
func (r *repo) CreateBanner(b *Banner) error {
    placementsJSON, _ := json.Marshal(b.Placements)
    variantsJSON, _ := json.Marshal(b.ABVariants)
    query := `INSERT INTO banners (tenant_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, start_date, end_date, is_active, ab_test, ab_variants)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id, created_at, updated_at`
    return r.pool.QueryRow(ctx, query, b.TenantID, b.Name, b.Type, b.Content, b.LinkURL, b.ImageURL, b.AltText, b.CampaignID, placementsJSON, b.Priority, b.StartDate, b.EndDate, b.IsActive, b.ABTest, variantsJSON).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
}

func (r *repo) UpdateBanner(b *Banner) error {
    // UPDATE query with all fields
}

func (r *repo) GetBanner(tenantID, bannerID string) (*Banner, error) {
    // SELECT query
}
```

- [ ] **Step 3: Implement banner serving logic**

```go
func (r *repo) GetActiveBannersForPlacement(tenantID, placementID string, date time.Time) ([]*Banner, error) {
    query := `SELECT * FROM banners
              WHERE tenant_id = $1 AND is_active = TRUE
              AND start_date <= $2 AND end_date >= $2
              AND placements @> $3::jsonb
              ORDER BY priority DESC
              LIMIT 10`
    // Execute and scan into []*Banner
}

func (r *repo) RecordImpression(bannerID, visitorID string) error {
    query := `INSERT INTO banner_impressions (banner_id, visitor_id) VALUES ($1, $2)`
    return r.pool.QueryRow(ctx, query, bannerID, visitorID).Scan()
}

func (r *repo) RecordClick(bannerID, visitorID string) error {
    query := `INSERT INTO banner_clicks (banner_id, visitor_id) VALUES ($1, $2)`
    return r.pool.QueryRow(ctx, query, bannerID, visitorID).Scan()
}
```

- [ ] **Step 4: Implement stats aggregation**

```go
func (r *repo) GetCampaignStats(tenantID, campaignID string, dateRange DateRange) (*CampaignStats, error) {
    // SELECT with date grouping for impressions and clicks
}

func (r *repo) GetBannerStats(tenantID, bannerID string, dateRange DateRange) (*BannerStats, error) {
    // SELECT impressions, clicks, calculate CTR
}
```

- [ ] **Step 5: Write integration tests**

```go
var _ = Describe("BannerRepository", func() {
    It("creates a banner", func() {})
    It("gets active banners for placement", func() {})
    It("records impressions and clicks", func() {})
    It("returns campaign stats", func() {})
})
```

- [ ] **Step 6: Run tests**

Run: `ginkgo internal/infrastructure/banner/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/infrastructure/banner/
git commit -m "feat: add PostgreSQL repository for banner management with serving logic"
```

---

### Task 3: Banner use cases

**Files:**
- Create: `internal/usecase/banner/create_banner.go`
- Create: `internal/usecase/banner/list_banners.go`
- Create: `internal/usecase/banner/get_placement.go`
- Create: `internal/usecase/banner/get_campaign_stats.go`

**Interfaces:**
- Consumes: `banner.Repository`
- Produces: `CreateBanner`, `ListBanners`, `GetPlacement`, `GetCampaignStats` use cases

- [ ] **Step 1: Implement CreateBanner**

```go
func CreateBanner(repo banner.Repository, tenantID string, input CreateBannerInput) (*banner.Banner, error) {
    b := &banner.Banner{
        TenantID: tenantID,
        Name:     input.Name,
        Type:     input.Type,
        Content:  input.Content,
        // ... map other fields
    }
    if err := repo.CreateBanner(b); err != nil {
        return nil, err
    }
    return b, nil
}
```

- [ ] **Step 2: Implement ListBanners**

```go
func ListBanners(repo banner.Repository, tenantID string, opts banner.ListOptions) ([]*banner.Banner, int64, error) {
    return repo.ListBanners(tenantID, opts)
}
```

- [ ] **Step 3: Implement GetActiveBannersForPlacement**

```go
func GetActiveBanners(repo banner.Repository, tenantID, placementID string) ([]*banner.Banner, error) {
    return repo.GetActiveBannersForPlacement(tenantID, placementID, time.Now())
}
```

- [ ] **Step 4: Implement GetCampaignStats**

```go
func GetCampaignStats(repo banner.Repository, tenantID, campaignID string, dateRange banner.DateRange) (*banner.CampaignStats, error) {
    return repo.GetCampaignStats(tenantID, campaignID, dateRange)
}
```

- [ ] **Step 5: Write unit tests**

```go
func TestCreateBanner(t *testing.T) {
    mockRepo := &mockRepo{}
    input := CreateBannerInput{Name: "Test Banner", Type: banner.BannerTypeHTML, Content: "<div>Test</div>"}
    banner, err := CreateBanner(mockRepo, "tenant-1", input)
    assert.NoError(t, err)
    assert.NotNil(t, banner)
}

func TestGetActiveBanners(t *testing.T) {
    mockRepo := &mockRepo{}
    banners, err := GetActiveBanners(mockRepo, "tenant-1", "placement-1")
    assert.NoError(t, err)
    assert.NotNil(t, banners)
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/usecase/banner/... -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/usecase/banner/
git commit -m "feat: add banner use cases for CRUD and stats"
```

---

### Task 4: Banner JS snippet generator

**Files:**
- Create: `pkg/snippet/banner.go`
- Create: `pkg/snippet/banner_test.go`

**Interfaces:**
- Consumes: shared `pkg/snippet` generator
- Produces: Banner-specific JS that injects banners into placements

- [ ] **Step 1: Implement banner snippet generator**

```go
package snippet

func GenerateBannerSnippet(tenantID string, options map[string]interface{}) (string, error) {
    snippet := `
    (function() {
        var Automata = Automata || {};
        Automata.tenantId = "` + tenantID + `";
        Automata.apiHost = "` + options["api_host"].(string) + `";

        // Fetch and inject banners
        function loadBanners() {
            fetch(Automata.apiHost + "/api/banners/placements", {
                headers: {"X-Tenant-ID": Automata.tenantId}
            })
            .then(function(res) { return res.json(); })
            .then(function(data) {
                data.placements.forEach(function(placement) {
                    var el = document.querySelector(placement.css_selector);
                    if (el && placement.banners.length > 0) {
                        el.innerHTML = placement.banners[0].content;
                        // Track impression
                        trackImpression(placement.banners[0].id);
                    }
                });
            });
        }

        // Track banner impression via beacon
        function trackImpression(bannerID) {
            var img = new Image(1, 1);
            img.src = Automata.apiHost + "/track/banner?banner_id=" + bannerID + "&type=impression";
        }

        // Track banner click
        function trackClick(bannerID) {
            var img = new Image(1, 1);
            img.src = Automata.apiHost + "/track/banner?banner_id=" + bannerID + "&type=click";
        }

        // Wait for DOM ready
        if (document.readyState === "loading") {
            document.addEventListener("DOMContentLoaded", loadBanners);
        } else {
            loadBanners();
        }
    })();
    `
    return snippet, nil
}
```

- [ ] **Step 2: Write tests**

```go
func TestGenerateBannerSnippet(t *testing.T) {
    snippet, err := GenerateBannerSnippet("tenant-1", map[string]interface{}{"api_host": "https://api.example.com"})
    assert.NoError(t, err)
    assert.Contains(t, snippet, "tenant-1")
    assert.Contains(t, snippet, "/api/banners/placements")
    assert.Contains(t, snippet, "trackImpression")
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./pkg/snippet/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add pkg/snippet/banner.go
git commit -m "feat: add banner JS snippet generator"
```

---

### Task 5: Banner serving endpoint adapter

**Files:**
- Create: `internal/adapter/banner/serving_handler.go`
- Create: `internal/adapter/banner/snippet_handler.go`
- Create: `internal/adapter/banner/server.go`
- Create: `internal/adapter/banner/tenant_resolver.go`

**Interfaces:**
- Consumes: banner use cases, snippet generator
- Produces: High-throughput HTTP server with GET `/api/banners/placements`, GET `/track/banner`

- [ ] **Step 1: Implement tenant resolver for banners**

```go
func ResolveTenant(c *gin.Context) string {
    if tid := c.GetHeader("X-Tenant-ID"); tid != "" {
        return tid
    }
    host := c.GetHeader("Host")
    parts := strings.Split(host, ".")
    if len(parts) > 2 {
        return parts[0]
    }
    c.AbortWithStatusJSON(400, gin.H{"error": "tenant ID required"})
    return ""
}
```

- [ ] **Step 2: Implement placements endpoint**

```go
func (h *handler) GetPlacements(c *gin.Context) {
    tenantID := ResolveTenant(c)
    if tenantID == "" {
        return
    }

    placements, err := usecase.ListPlacements(h.repo, tenantID, true)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    result := []map[string]interface{}{}
    for _, p := range placements {
        banners, _ := usecase.GetActiveBanners(h.repo, tenantID, p.ID)
        result = append(result, map[string]interface{}{
            "id":         p.ID,
            "name":       p.Name,
            "css_selector": p.CSSSelector,
            "banners":    banners,
        })
    }

    c.JSON(200, gin.H{"placements": result})
}
```

- [ ] **Step 3: Implement banner tracking endpoint**

```go
func (h *handler) TrackBanner(c *gin.Context) {
    tenantID := ResolveTenant(c)
    if tenantID == "" {
        return
    }

    bannerID := c.Query("banner_id")
    visitorID := c.Query("visitor_id")
    btype := c.Query("type") // "impression" or "click"

    if btype == "impression" {
        h.repo.RecordImpression(bannerID, visitorID)
    } else if btype == "click" {
        h.repo.RecordClick(bannerID, visitorID)
        // Also send event to User Tracking system
    }

    // Return 1x1 transparent pixel
    c.Data(200, "image/png", []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x62, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc, 0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82})
}
```

- [ ] **Step 4: Wire up the banner server**

```go
func NewServer(repo banner.Repository) *gin.Engine {
    engine := gin.Default()
    h := &handler{repo: repo}

    engine.GET("/api/banners/placements", h.GetPlacements)
    engine.GET("/track/banner", h.TrackBanner)
    engine.GET("/snippet/banner/:id.js", h.ServeBannerSnippet)

    return engine
}
```

- [ ] **Step 5: Write integration tests**

```go
func TestPlacementsEndpoint(t *testing.T) {
    // GET /api/banners/placements -> returns placements with banners
}

func TestTrackBannerEndpoint(t *testing.T) {
    // GET /track/banner?banner_id=...&type=impression -> 200 with pixel
}

func TestBannerSnippetEndpoint(t *testing.T) {
    // GET /snippet/banner/<id>.js -> returns JS with tenant ID
}
```

- [ ] **Step 6: Run tests**

Run: `ginkgo internal/adapter/banner/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/banner/
git commit -m "feat: add high-throughput banner serving endpoint with tracking"
```

---

### Task 6: Banner management API and MCP tools

**Files:**
- Create: `internal/adapter/api/handler/banner_handler.go`
- Modify: `internal/adapter/mcp/banner_tools.go`
- Modify: `internal/adapter/api/server.go` (add banner routes)

**Interfaces:**
- Consumes: banner use cases
- Produces: Full CRUD API for banners, placements, campaigns + MCP tools

- [ ] **Step 1: Implement banner CRUD API handlers**

```go
func (h *handler) CreateBanner(c *gin.Context) {
    tenantID := getTenantID(c)
    var input CreateBannerInput
    c.BindJSON(&input)
    banner, err := usecase.CreateBanner(h.repo, tenantID, input)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(201, banner)
}

func (h *handler) ListBanners(c *gin.Context) {
    tenantID := getTenantID(c)
    opts := banner.ListOptions{
        CampaignID: c.Query("campaign_id"),
        IsActive:   c.Query("is_active") == "true",
        Offset:     parseInt(c.Query("offset"), 0),
        Limit:      parseInt(c.Query("limit"), 50),
    }
    banners, total, err := usecase.ListBanners(h.repo, tenantID, opts)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, gin.H{"banners": banners, "total": total})
}

func (h *handler) UpdateBanner(c *gin.Context) {
    tenantID := getTenantID(c)
    bannerID := c.Param("id")
    var input UpdateBannerInput
    c.BindJSON(&input)
    banner, err := usecase.UpdateBanner(h.repo, tenantID, bannerID, input)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, banner)
}

func (h *handler) DeleteBanner(c *gin.Context) {
    tenantID := getTenantID(c)
    bannerID := c.Param("id")
    err := usecase.DeleteBanner(h.repo, tenantID, bannerID)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.Status(204)
}
```

- [ ] **Step 2: Implement MCP banner tools**

```go
s.AddTool("banner.create", "Create a new banner",
    func(ctx context.Context, req struct{ TenantID string, Name string, Type string, Content string, LinkURL string }) (*banner.Banner, error) {
        input := CreateBannerInput{Name: req.Name, Type: banner.BannerType(req.Type), Content: req.Content, LinkURL: req.LinkURL}
        return usecase.CreateBanner(repo, req.TenantID, input)
    })

s.AddTool("banner.list", "List banners",
    func(ctx context.Context, req struct{ TenantID string, CampaignID string, Limit int }) ([]map[string]interface{}, error) {
        opts := banner.ListOptions{CampaignID: req.CampaignID, Limit: req.Limit}
        banners, _, _ := usecase.ListBanners(repo, req.TenantID, opts)
        return banners, nil
    })

s.AddTool("banner.get_stats", "Get banner or campaign statistics",
    func(ctx context.Context, req struct{ TenantID string, BannerID string, CampaignID string, Start string, End string }) (*banner.CampaignStats, error) {
        dateRange := banner.DateRange{Start: parseTime(req.Start), End: parseTime(req.End)}
        if req.CampaignID != "" {
            return usecase.GetCampaignStats(repo, req.TenantID, req.CampaignID, dateRange)
        }
        stats, _ := usecase.GetBannerStats(repo, req.TenantID, req.BannerID, dateRange)
        return &banner.CampaignStats{TotalImpressions: stats.Impressions, TotalClicks: stats.Clicks}, nil
    })
```

- [ ] **Step 3: Register routes**

```go
banners := api.Group("/api/banners")
banners.POST("", bannerHandler.CreateBanner)
banners.GET("", bannerHandler.ListBanners)
banners.GET("/:id", bannerHandler.GetBanner)
banners.PUT("/:id", bannerHandler.UpdateBanner)
banners.DELETE("/:id", bannerHandler.DeleteBanner)

placements := api.Group("/api/placements")
placements.POST("", placementHandler.CreatePlacement)
placements.GET("", placementHandler.ListPlacements)
placements.GET("/:id", placementHandler.GetPlacement)
placements.PUT("/:id", placementHandler.UpdatePlacement)
placements.DELETE("/:id", placementHandler.DeletePlacement)

campaigns := api.Group("/api/campaigns")
campaigns.POST("", campaignHandler.CreateCampaign)
campaigns.GET("", campaignHandler.ListCampaigns)
campaigns.GET("/:id", campaignHandler.GetCampaign)
campaigns.PUT("/:id", campaignHandler.UpdateCampaign)
campaigns.DELETE("/:id", campaignHandler.DeleteCampaign)
```

- [ ] **Step 4: Write tests**

```go
func TestBannerCRUD(t *testing.T) {
    // POST /api/banners -> 201
    // GET /api/banners -> 200
    // PUT /api/banners/:id -> 200
    // DELETE /api/banners/:id -> 204
}

func TestBannerMCPTools(t *testing.T) {
    // Test banner.create, banner.list, banner.get_stats
}
```

- [ ] **Step 5: Run tests**

Run: `ginkgo internal/adapter/api/... internal/adapter/mcp/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/api/handler/banner_handler.go internal/adapter/mcp/banner_tools.go
git commit -m "feat: add banner management API and MCP tools"
```

---

## Testing Strategy

- **Unit tests:** Banner entity, use case logic, snippet generation -- `go test ./...`
- **Integration tests:** Repository CRUD, banner serving, impression/click tracking -- Ginkgo + testcontainers for PostgreSQL
- **E2E tests:** Full API and MCP layer tests in `cicd/` directory
- **Load tests:** Simulate 500 banner impressions/sec, placement fetch < 100ms

## Task Dependencies

```
Task 1 (domain entities) ──> Task 3 (use cases) ──> Task 6 (API + MCP)
      │                           │
      v                           │
Task 2 (postgres repo) ─────────┘
Task 4 (banner snippet pkg) ────┘
Task 5 (banner serving endpoint) ─┘
```
