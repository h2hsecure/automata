# User Tracking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build JS snippet, event tracking endpoint, visitor identification, and analytics dashboard data.

**Architecture:** Hexagonal architecture with domain entities for Event, Visitor, and Analytics. Tracking is a separate high-throughput driver adapter from the API server. Shared JS snippet generator in pkg/snippet.

**Tech Stack:** Go 1.26+, PostgreSQL, Ginkgo + testcontainers

**Spec:** docs/prds/2025-09-10-user-tracking-prd.md

## Global Constraints

- Hexagonal architecture: domain interfaces in `internal/domain`, use cases in `internal/usecase`, adapters in `internal/adapter`, infrastructure in `internal/infrastructure`
- KISS and DRY principles throughout
- Multi-tenant: all tables include `tenant_id`, data strictly isolated per tenant
- Tracking endpoint is a separate driver adapter from API server (high-throughput)
- GDPR-friendly: no PII stored by default, IP address not stored raw
- Visitor identification via cookie + browser fingerprint
- Event batching on client-side (every 30s or on page unload)
- Storage retention configurable per tenant via key-value config (default: 365 days)
- Shared snippet: pkg/snippet shared between User Tracking and Banner Management

---

### Task 1: Tracking domain entity and repository interface

**Files:**
- Create: `internal/domain/tracking/event.go`
- Create: `internal/domain/tracking/visitor.go`
- Create: `internal/domain/tracking/repository.go`
- Create: `internal/domain/tracking/analytics.go`

**Interfaces:**
- Consumes: none (domain layer)
- Produces: `Event` entity, `Visitor` entity, `Repository` interface, `DashboardMetrics` type

- [ ] **Step 1: Define Event entity**

```go
package tracking

import "time"

type EventType string

const (
    EventTypePageView EventType = "pageview"
    EventTypeEvent    EventType = "event"
)

type Event struct {
    ID           string
    TenantID     string
    VisitorID    string
    Type         EventType
    URL          string
    Title        string
    Referrer     string
    EventName    string
    Properties   map[string]interface{}
    UserAgent    string
    IPHash       string
    UTMSource    string
    UTMMedium    string
    UTMCampaign  string
    CreatedAt    time.Time
}

func (e *Event) IsPageView() bool {
    return e.Type == EventTypePageView
}
```

- [ ] **Step 2: Define Visitor entity**

```go
package tracking

import "time"

type Visitor struct {
    ID            string
    TenantID      string
    CookieValue   string
    Fingerprint   string
    FirstSeen     time.Time
    LastSeen      time.Time
    PageViews     int
}
```

- [ ] **Step 3: Define Analytics types**

```go
package tracking

type DashboardMetrics struct {
    TotalVisitors  int64
    ActiveVisitors int64
    PageViews      int64
    TopPages       []PageViewCount
    TopReferrers   []ReferrerCount
    DeviceBreakdown map[string]int
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

- [ ] **Step 4: Define Repository interface**

```go
package tracking

import "time"

type DateRange struct {
    Start time.Time
    End   time.Time
}

type EventFilter struct {
    Type      EventType
    EventName string
    URL       string
    Offset    int
    Limit     int
}

type Repository interface {
    CreateEvent(e *Event) error
    CreateEventsBatch(events []*Event) error
    GetVisitor(tenantID, visitorID string) (*Visitor, error)
    UpsertVisitor(v *Visitor) error
    GetDashboardMetrics(tenantID string, dateRange DateRange) (*DashboardMetrics, error)
    GetEvents(tenantID string, opts EventFilter) ([]*Event, int64, error)
    GetTopPages(tenantID string, dateRange DateRange, limit int) ([]PageViewCount, error)
    GetTopReferrers(tenantID string, dateRange DateRange, limit int) ([]ReferrerCount, error)
    GetDeviceBreakdown(tenantID string, dateRange DateRange) (map[string]int, error)
    GetBrowserBreakdown(tenantID string, dateRange DateRange) (map[string]int, error)
    GetActiveVisitors(tenantID string, minutes int) (int64, error)
    PurgeOldEvents(tenantID string, retentionDays int) (int64, error)
}
```

- [ ] **Step 5: Write entity tests**

```go
func TestEventIsPageView(t *testing.T) {
    e := &Event{Type: EventTypePageView}
    if !e.IsPageView() {
        t.Error("expected pageview")
    }
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/domain/tracking/... -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/domain/tracking/
git commit -m "feat: add tracking domain entities: Event, Visitor, Analytics, Repository interface"
```

---

### Task 2: Tracking PostgreSQL repository

**Files:**
- Create: `internal/infrastructure/tracking/repo/postgres.go`
- Create: `internal/infrastructure/tracking/repo/postgres_test.go`
- Create: `internal/infrastructure/tracking/repo/migration.sql`

**Interfaces:**
- Consumes: `database.NewPool()` from core platform
- Produces: Full Repository implementation with batch inserts and analytics aggregation

- [ ] **Step 1: Write migration SQL**

```sql
CREATE TABLE events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    visitor_id    VARCHAR(128),
    type          VARCHAR(32) NOT NULL,
    url           VARCHAR(2048),
    title         VARCHAR(512),
    referrer      VARCHAR(2048),
    event_name    VARCHAR(128),
    properties    JSONB DEFAULT '{}',
    user_agent    VARCHAR(512),
    ip_hash       VARCHAR(64),
    utm_source    VARCHAR(128),
    utm_medium    VARCHAR(128),
    utm_campaign  VARCHAR(128),
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_events_tenant_created ON events(tenant_id, created_at DESC);
CREATE INDEX idx_events_visitor ON events(tenant_id, visitor_id);
CREATE INDEX idx_events_type ON events(tenant_id, type);
```

- [ ] **Step 2: Implement CreateEvent and batch insert**

```go
func (r *repo) CreateEvent(e *Event) error {
    propsJSON, _ := json.Marshal(e.Properties)
    query := `INSERT INTO events (tenant_id, visitor_id, type, url, title, referrer, event_name, properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id, created_at`
    return r.pool.QueryRow(ctx, query, e.TenantID, e.VisitorID, e.Type, e.URL, e.Title, e.Referrer, e.EventName, propsJSON, e.UserAgent, e.IPHash, e.UTMSource, e.UTMMedium, e.UTMCampaign).Scan(&e.ID, &e.CreatedAt)
}

func (r *repo) CreateEventsBatch(events []*Event) error {
    // Use pgx.CopyFrom for bulk insert
    return nil
}
```

- [ ] **Step 3: Implement analytics aggregation queries**

```go
func (r *repo) GetTopPages(tenantID string, dateRange DateRange, limit int) ([]PageViewCount, error) {
    query := `SELECT url, COUNT(*) as count FROM events
              WHERE tenant_id = $1 AND type = 'pageview' AND created_at BETWEEN $2 AND $3
              GROUP BY url ORDER BY count DESC LIMIT $4`
    // Execute and scan into []PageViewCount
}

func (r *repo) GetActiveVisitors(tenantID string, minutes int) (int64, error) {
    query := `SELECT COUNT(DISTINCT visitor_id) FROM events
              WHERE tenant_id = $1 AND type = 'pageview' AND created_at > NOW() - INTERVAL '$2 minutes'`
    // Execute and scan
}
```

- [ ] **Step 4: Implement PurgeOldEvents**

```go
func (r *repo) PurgeOldEvents(tenantID string, retentionDays int) (int64, error) {
    query := `DELETE FROM events WHERE tenant_id = $1 AND created_at < NOW() - INTERVAL '$2 days' RETURNING id`
    // Count deleted rows
}
```

- [ ] **Step 5: Write integration tests**

```go
var _ = Describe("TrackingRepository", func() {
    It("creates an event", func() {})
    It("batches events efficiently", func() {})
    It("returns top pages", func() {})
    It("returns active visitor count", func() {})
    It("purges old events", func() {})
})
```

- [ ] **Step 6: Run tests**

Run: `ginkgo internal/infrastructure/tracking/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/infrastructure/tracking/
git commit -m "feat: add PostgreSQL repository for tracking with analytics queries"
```

---

### Task 3: Tracking use cases

**Files:**
- Create: `internal/usecase/tracking/track_event.go`
- Create: `internal/usecase/tracking/get_dashboard.go`
- Create: `internal/usecase/tracking/get_events.go`

**Interfaces:**
- Consumes: `tracking.Repository`
- Produces: `TrackEvent(tenantID string, event *tracking.Event) error`, `GetDashboard(tenantID string, dateRange tracking.DateRange) (*tracking.DashboardMetrics, error)`, `GetEvents(tenantID string, opts tracking.EventFilter) ([]*tracking.Event, int64, error)`

- [ ] **Step 1: Implement TrackEvent**

```go
func TrackEvent(repo tracking.Repository, tenantID string, event *tracking.Event) error {
    event.TenantID = tenantID
    if err := repo.CreateEvent(event); err != nil {
        return err
    }
    // Upsert visitor
    visitor := &tracking.Visitor{
        TenantID: tenantID,
        CookieValue: event.VisitorID,
        LastSeen: time.Now(),
    }
    return repo.UpsertVisitor(visitor)
}
```

- [ ] **Step 2: Implement GetDashboard**

```go
func GetDashboard(repo tracking.Repository, tenantID string, dateRange tracking.DateRange) (*tracking.DashboardMetrics, error) {
    metrics := &tracking.DashboardMetrics{}
    var err error
    metrics.TotalVisitors, _ = repo.GetUniqueVisitors(tenantID, dateRange)
    metrics.ActiveVisitors, _ = repo.GetActiveVisitors(tenantID, 30)
    metrics.PageViews, _ = repo.GetPageViewCount(tenantID, dateRange)
    metrics.TopPages, _ = repo.GetTopPages(tenantID, dateRange, 10)
    metrics.TopReferrers, _ = repo.GetTopReferrers(tenantID, dateRange, 10)
    metrics.DeviceBreakdown, _ = repo.GetDeviceBreakdown(tenantID, dateRange)
    metrics.BrowserBreakdown, _ = repo.GetBrowserBreakdown(tenantID, dateRange)
    return metrics, nil
}
```

- [ ] **Step 3: Implement GetEvents**

```go
func GetEvents(repo tracking.Repository, tenantID string, opts tracking.EventFilter) ([]*tracking.Event, int64, error) {
    return repo.GetEvents(tenantID, opts)
}
```

- [ ] **Step 4: Write unit tests**

```go
func TestTrackEvent(t *testing.T) {
    mockRepo := &mockRepo{}
    event := &tracking.Event{Type: tracking.EventTypePageView, URL: "/home"}
    err := TrackEvent(mockRepo, "tenant-1", event)
    assert.NoError(t, err)
}

func TestGetDashboard(t *testing.T) {
    mockRepo := &mockRepo{}
    metrics, err := GetDashboard(mockRepo, "tenant-1", tracking.DateRange{...})
    assert.NoError(t, err)
    assert.NotNil(t, metrics)
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/usecase/tracking/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/usecase/tracking/
git commit -m "feat: add tracking use cases for event tracking and dashboard metrics"
```

---

### Task 4: JS snippet generator

**Files:**
- Create: `pkg/snippet/generator.go`
- Create: `pkg/snippet/generator_test.go`

**Interfaces:**
- Consumes: none (domain-agnostic, shared with Banner Management)
- Produces: `Generate(tenantID string, options map[string]interface{}) (string, error)` that returns a self-contained JS snippet

- [ ] **Step 1: Implement snippet generator**

```go
package snippet

func Generate(tenantID string, options map[string]interface{}) (string, error) {
    snippet := `
    (function() {
        var Automata = Automata || {};
        Automata.tenantId = "` + tenantID + `";
        Automata.apiHost = "` + options["api_host"].(string) + `";

        // Cookie-based visitor identification
        function getVisitorId() {
            var id = document.cookie.match(/automata_visitor=([^;]+)/);
            if (!id) {
                id = "" + Math.random().toString(36).substr(2, 9);
                document.cookie = "automata_visitor=" + id + "; max-age=31536000";
            }
            return id[1];
        }

        // Track page view
        function trackPageView() {
            fetch(Automata.apiHost + "/track", {
                method: "POST",
                headers: {"Content-Type": "application/json", "X-Tenant-ID": Automata.tenantId},
                body: JSON.stringify({
                    type: "pageview",
                    url: window.location.pathname,
                    title: document.title,
                    referrer: document.referrer,
                    visitor_id: getVisitorId()
                })
            });
        }

        // Custom event tracking API
        Automata.track = function(eventName, properties) {
            fetch(Automata.apiHost + "/track", {
                method: "POST",
                headers: {"Content-Type": "application/json", "X-Tenant-ID": Automata.tenantId},
                body: JSON.stringify({
                    type: "event",
                    event_name: eventName,
                    properties: properties || {},
                    visitor_id: getVisitorId()
                })
            });
        };

        // Event batching
        var events = [];
        setInterval(function() {
            if (events.length > 0) {
                fetch(Automata.apiHost + "/track/batch", {
                    method: "POST",
                    headers: {"Content-Type": "application/json", "X-Tenant-ID": Automata.tenantId},
                    body: JSON.stringify({events: events})
                });
                events = [];
            }
        }, 30000);

        trackPageView();
    })();
    `
    return snippet, nil
}
```

- [ ] **Step 2: Write tests**

```go
func TestGenerateSnippet(t *testing.T) {
    snippet, err := Generate("tenant-1", map[string]interface{}{"api_host": "https://api.example.com"})
    assert.NoError(t, err)
    assert.Contains(t, snippet, "tenant-1")
    assert.Contains(t, snippet, "/track")
    assert.Contains(t, snippet, "automata.track")
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./pkg/snippet/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add pkg/snippet/
git commit -m "feat: add JS snippet generator for tracking (shared with Banner Management)"
```

---

### Task 5: Tracking endpoint adapter

**Files:**
- Create: `internal/adapter/tracking/track_handler.go`
- Create: `internal/adapter/tracking/snippet_handler.go`
- Create: `internal/adapter/tracking/server.go`
- Create: `internal/adapter/tracking/tenant_resolver.go`

**Interfaces:**
- Consumes: tracking use cases, snippet generator
- Produces: High-throughput HTTP server with POST `/track`, POST `/track/batch`, GET `/snippet/<id>.js`

- [ ] **Step 1: Implement tenant resolver for tracking**

```go
func ResolveTenant(c *gin.Context) string {
    // Try X-Tenant-ID header first
    if tid := c.GetHeader("X-Tenant-ID"); tid != "" {
        return tid
    }
    // Fall back to subdomain extraction from Host header
    host := c.GetHeader("Host")
    parts := strings.Split(host, ".")
    if len(parts) > 2 {
        return parts[0]
    }
    c.AbortWithStatusJSON(400, gin.H{"error": "tenant ID required"})
    return ""
}
```

- [ ] **Step 2: Implement track endpoint**

```go
func (h *handler) Track(c *gin.Context) {
    tenantID := ResolveTenant(c)
    if tenantID == "" {
        return
    }
    var req struct {
        Type       string                 `json:"type"`
        URL        string                 `json:"url"`
        Title      string                 `json:"title"`
        Referrer   string                 `json:"referrer"`
        EventName  string                 `json:"event_name"`
        Properties map[string]interface{} `json:"properties"`
        VisitorID  string                 `json:"visitor_id"`
    }
    c.BindJSON(&req)

    event := &tracking.Event{
        Type:       tracking.EventType(req.Type),
        URL:        req.URL,
        Title:      req.Title,
        Referrer:   req.Referrer,
        EventName:  req.EventName,
        Properties: req.Properties,
        VisitorID:  req.VisitorID,
        UserAgent:  c.GetHeader("User-Agent"),
    }

    if err := usecase.TrackEvent(h.repo, tenantID, event); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.Status(204)
}
```

- [ ] **Step 3: Implement batch track endpoint**

```go
func (h *handler) TrackBatch(c *gin.Context) {
    tenantID := ResolveTenant(c)
    if tenantID == "" {
        return
    }
    var req struct {
        Events []map[string]interface{} `json:"events"`
    }
    c.BindJSON(&req)

    var events []*tracking.Event
    for _, e := range req.Events {
        // Convert map to Event
    }
    h.repo.CreateEventsBatch(events)
    c.Status(204)
}
```

- [ ] **Step 4: Implement snippet endpoint**

```go
func (h *handler) ServeSnippet(c *gin.Context) {
    tenantID := c.Param("id")
    snippet, err := snippet.Generate(tenantID, map[string]interface{}{
        "api_host": config.Get("server.host"),
    })
    if err != nil {
        c.String(500, "error")
        return
    }
    c.Header("Content-Type", "application/javascript")
    c.String(200, snippet)
}
```

- [ ] **Step 5: Wire up the tracking server**

```go
func NewServer(repo tracking.Repository) *gin.Engine {
    engine := gin.Default()
    h := &handler{repo: repo}

    engine.POST("/track", h.Track)
    engine.POST("/track/batch", h.TrackBatch)
    engine.GET("/snippet/:id.js", h.ServeSnippet)

    return engine
}
```

- [ ] **Step 6: Write integration tests**

```go
func TestTrackEndpoint(t *testing.T) {
    // POST /track with valid data -> 204
}

func TestTrackBatchEndpoint(t *testing.T) {
    // POST /track/batch with multiple events -> 204
}

func TestSnippetEndpoint(t *testing.T) {
    // GET /snippet/<id>.js -> returns JS with tenant ID
}

func TestTenantResolution(t *testing.T) {
    // X-Tenant-ID header takes precedence over subdomain
}
```

- [ ] **Step 7: Run tests**

Run: `ginkgo internal/adapter/tracking/...`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/adapter/tracking/
git commit -m "feat: add high-throughput tracking endpoint with batch support and snippet serving"
```

---

### Task 6: Dashboard API and MCP tracking tools

**Files:**
- Create: `internal/adapter/api/handler/tracking_handler.go`
- Modify: `internal/adapter/mcp/tracking_tools.go`
- Modify: `internal/adapter/api/server.go` (add tracking routes)

**Interfaces:**
- Consumes: tracking use cases
- Produces: GET `/api/tracking/dashboard` endpoint, MCP tools for tracking.get_dashboard, tracking.get_events, tracking.track

- [ ] **Step 1: Implement dashboard API handler**

```go
func (h *handler) GetDashboard(c *gin.Context) {
    tenantID := getTenantID(c)
    start := c.Query("start")
    end := c.Query("end")
    dateRange := tracking.DateRange{
        Start: parseTime(start),
        End:   parseTime(end),
    }
    metrics, err := usecase.GetDashboard(h.repo, tenantID, dateRange)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, metrics)
}

func (h *handler) GetEvents(c *gin.Context) {
    tenantID := getTenantID(c)
    opts := tracking.EventFilter{
        Type:      tracking.EventType(c.Query("type")),
        EventName: c.Query("event_name"),
        Offset:    parseInt(c.Query("offset"), 0),
        Limit:     parseInt(c.Query("limit"), 50),
    }
    events, total, err := usecase.GetEvents(h.repo, tenantID, opts)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, gin.H{"events": events, "total": total})
}
```

- [ ] **Step 2: Implement MCP tracking tools**

```go
s.AddTool("tracking.get_dashboard", "Get analytics dashboard metrics",
    func(ctx context.Context, req struct{ TenantID string, Start string, End string }) (*tracking.DashboardMetrics, error) {
        return usecase.GetDashboard(repo, req.TenantID, tracking.DateRange{Start: parseTime(req.Start), End: parseTime(req.End)})
    })

s.AddTool("tracking.get_events", "Get tracked events",
    func(ctx context.Context, req struct{ TenantID string, Type string, Limit int }) ([]map[string]interface{}, error) {
        opts := tracking.EventFilter{Type: tracking.EventType(req.Type), Limit: req.Limit}
        events, _, _ := usecase.GetEvents(repo, req.TenantID, opts)
        return events, nil
    })

s.AddTool("tracking.track", "Track an event",
    func(ctx context.Context, req struct{ Event tracking.Event }) error {
        return usecase.TrackEvent(repo, req.Event.TenantID, &req.Event)
    })
```

- [ ] **Step 3: Register routes**

```go
tracking := api.Group("/api/tracking")
tracking.GET("/dashboard", trackingHandler.GetDashboard)
tracking.GET("/events", trackingHandler.GetEvents)
```

- [ ] **Step 4: Write tests**

```go
func TestDashboardAPI(t *testing.T) {
    // GET /api/tracking/dashboard -> returns metrics
}

func TestTrackingMCPTools(t *testing.T) {
    // Test tracking.get_dashboard, tracking.track
}
```

- [ ] **Step 5: Run tests**

Run: `ginkgo internal/adapter/api/... internal/adapter/mcp/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/api/handler/tracking_handler.go internal/adapter/mcp/tracking_tools.go
git commit -m "feat: add tracking dashboard API and MCP tools"
```

---

## Testing Strategy

- **Unit tests:** Event entity, use case logic, snippet generation -- `go test ./...`
- **Integration tests:** Repository CRUD, batch inserts, analytics aggregation -- Ginkgo + testcontainers for PostgreSQL
- **E2E tests:** Full API and MCP layer tests in `cicd/` directory
- **Load tests:** Simulate 1000 events/sec ingestion, dashboard loads < 2 seconds for 100k events

## Task Dependencies

```
Task 1 (domain entities) ──> Task 3 (use cases) ──> Task 6 (API + MCP)
      │                           │
      v                           │
Task 2 (postgres repo) ─────────┘
Task 4 (snippet pkg) ───────────┘
Task 5 (tracking endpoint) ─────┘
```
