# Tracking Visitors — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement admin-only visitor listing, detail view, and event history endpoints so admins can browse tracked visitors, view their profiles, and drill into their full event history.

**Architecture:** Extend the existing `tracking.Repository` interface with three new methods (`ListVisitors`, `GetVisitorByID`, `GetVisitorEvents`), implement them in the PostgreSQL repository with optimized indexes for sorting, create thin usecase wrappers for validation, add handler methods that parse query params and call usecases, and register all three routes under the admin group which requires JWT with `is_admin` claim.

**Tech Stack:** Go 1.26.2, Gin v1.12.0, PostgreSQL (pgx v5), golang-jwt/v5, zerolog

**Spec:** `docs/superpowers/specs/2026-09-25-tracking-visitors-design.md`

## Global Constraints

- Module path: `github.com/hekemen/automata`
- Go version: `1.26.2`
- All UUIDs must be valid UUID format strings (use `gen_random_uuid()`)
- JWT HS256 signing
- Config loading: `config.Get("key")` from `internal/infrastructure/config/config.go`
- Database migrations: use `CREATE TABLE IF NOT EXISTS` and `ON CONFLICT` for idempotency
- Hexagonal architecture: domain interfaces in `internal/domain/`, infrastructure in `internal/infrastructure/`, adapters in `internal/adapter/`
- All new files go in existing package directories
- Visitor endpoints require JWT with `is_admin` claim (admin route group)
- API keys do NOT qualify — only authenticated users with `is_admin` can access

---

### Task 1: Add domain structs and extend repository interface

**Files:**
- Create: `internal/domain/tracking/list_visitors_opts.go`
- Modify: `internal/domain/tracking/repository.go`

**Interfaces:**
- Consumes: none
- Produces: `ListVisitorsOpts`, `GetVisitorEventsOpts` structs; `ListVisitors`, `GetVisitorByID`, `GetVisitorEvents` methods on `tracking.Repository`

Steps:
1. **Create `internal/domain/tracking/list_visitors_opts.go`**:
   ```go
   package tracking

   // ListVisitorsOpts holds options for listing visitors.
   type ListVisitorsOpts struct {
       Offset int
       Limit  int
       Sort   string // "first_seen", "last_seen", "page_views"
   }

   // GetVisitorEventsOpts holds options for listing a visitor's events.
   type GetVisitorEventsOpts struct {
       Offset int
       Limit  int
       Type   EventType
   }
   ```

2. **Extend the `Repository` interface** in `internal/domain/tracking/repository.go`:
   Append after `PurgeOldEvents`:
   ```go
       // ListVisitors returns a paginated list of visitors for a context.
       ListVisitors(ctx context.Context, contextID string, opts ListVisitorsOpts) ([]*Visitor, int64, error)

       // GetVisitorByID returns a single visitor by their ID (not cookie value).
       GetVisitorByID(ctx context.Context, contextID, visitorID string) (*Visitor, error)

       // GetVisitorEvents returns a paginated list of events for a specific visitor.
       GetVisitorEvents(ctx context.Context, contextID, visitorID string, opts GetVisitorEventsOpts) ([]*Event, int64, error)
   ```

3. **Add `context` import** to the repository.go imports:
   ```go
   import (
       "context"
       "time"
   )
   ```

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 2: Implement repository methods in PostgreSQL

**Files:**
- Modify: `internal/infrastructure/tracking/repo/postgres.go`

**Interfaces:**
- Consumes: `pool *pgxpool.Pool`, `context.Context`
- Produces: Implements `ListVisitors`, `GetVisitorByID`, `GetVisitorEvents` on `*repo`

Steps:
1. **Add `ListVisitors` method** at the end of `postgres.go`:
   ```go
   func (r *repo) ListVisitors(ctx context.Context, contextID string, opts tracking.ListVisitorsOpts) ([]*tracking.Visitor, int64, error) {
       if opts.Sort == "" {
           opts.Sort = "last_seen"
       }
       if opts.Limit <= 0 {
           opts.Limit = 20
       }
       if opts.Limit > 100 {
           opts.Limit = 100
       }

       // Build sort clause
       var sortCol string
       switch opts.Sort {
       case "first_seen":
           sortCol = "first_seen"
       case "last_seen":
           sortCol = "last_seen"
       case "page_views":
           sortCol = "page_views"
       default:
           return nil, 0, fmt.Errorf("invalid sort field: %s", opts.Sort)
       }

       countQuery := `SELECT COUNT(*) FROM tracking_visitors WHERE context_id = $1`
       dataQuery := `
           SELECT id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views
           FROM tracking_visitors
           WHERE context_id = $1
           ORDER BY ` + sortCol + ` DESC
       `
       args := []interface{}{contextID}
       argIdx := 2

       dataQuery += fmt.Sprintf(" LIMIT $%d", argIdx)
       args = append(args, opts.Limit)
       argIdx++
       if opts.Offset > 0 {
           dataQuery += fmt.Sprintf(" OFFSET $%d", argIdx)
           args = append(args, opts.Offset)
           argIdx++
       }

       var total int64
       if err := r.pool.QueryRow(ctx, countQuery, contextID).Scan(&total); err != nil {
           return nil, 0, fmt.Errorf("count visitors: %w", err)
       }

       rows, err := r.pool.Query(ctx, dataQuery, args...)
       if err != nil {
           return nil, 0, fmt.Errorf("query visitors: %w", err)
       }
       defer rows.Close()

       var visitors []*tracking.Visitor
       for rows.Next() {
           v := &tracking.Visitor{}
           err := rows.Scan(&v.ID, &v.ContextID, &v.CookieValue, &v.Fingerprint, &v.FirstSeen, &v.LastSeen, &v.PageViews)
           if err != nil {
               return nil, 0, fmt.Errorf("scan visitor: %w", err)
           }
           visitors = append(visitors, v)
       }
       if err := rows.Err(); err != nil {
           return nil, 0, fmt.Errorf("iterate visitors: %w", err)
       }
       return visitors, total, nil
   }
   ```

2. **Add `GetVisitorByID` method**:
   ```go
   func (r *repo) GetVisitorByID(ctx context.Context, contextID, visitorID string) (*tracking.Visitor, error) {
       query := `
           SELECT id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views
           FROM tracking_visitors
           WHERE context_id = $1 AND id = $2
       `
       v := &tracking.Visitor{}
       err := r.pool.QueryRow(ctx, query, contextID, visitorID).Scan(
           &v.ID, &v.ContextID, &v.CookieValue, &v.Fingerprint,
           &v.FirstSeen, &v.LastSeen, &v.PageViews,
       )
       if err != nil {
           if err == sql.ErrNoRows {
               return nil, fmt.Errorf("visitor not found")
           }
           return nil, fmt.Errorf("get visitor by id: %w", err)
       }
       return v, nil
   }
   ```

3. **Add `GetVisitorEvents` method**:
   ```go
   func (r *repo) GetVisitorEvents(ctx context.Context, contextID, visitorID string, opts tracking.GetVisitorEventsOpts) ([]*tracking.Event, int64, error) {
       if opts.Limit <= 0 {
           opts.Limit = 20
       }
       if opts.Limit > 100 {
           opts.Limit = 100
       }

       countQuery := `SELECT COUNT(*) FROM tracking_events WHERE context_id = $1 AND visitor_id = $2`
       dataQuery := `
           SELECT id, context_id, visitor_id, type, url, title, referrer, event_name, properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at
           FROM tracking_events
           WHERE context_id = $1 AND visitor_id = $2
       `
       args := []interface{}{contextID, visitorID}
       argIdx := 3

       if opts.Type != "" {
           countQuery += fmt.Sprintf(" AND type = $%d", argIdx)
           dataQuery += fmt.Sprintf(" AND type = $%d", argIdx)
           args = append(args, opts.Type)
           argIdx++
       }

       var total int64
       if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
           return nil, 0, fmt.Errorf("count visitor events: %w", err)
       }

       dataQuery += " ORDER BY created_at DESC"
       dataQuery += fmt.Sprintf(" LIMIT $%d", argIdx)
       args = append(args, opts.Limit)
       argIdx++
       if opts.Offset > 0 {
           dataQuery += fmt.Sprintf(" OFFSET $%d", argIdx)
           args = append(args, opts.Offset)
           argIdx++
       }

       rows, err := r.pool.Query(ctx, dataQuery, args...)
       if err != nil {
           return nil, 0, fmt.Errorf("query visitor events: %w", err)
       }
       defer rows.Close()

       var events []*tracking.Event
       for rows.Next() {
           e := &tracking.Event{}
           var propertiesJSON []byte
           err := rows.Scan(
               &e.ID, &e.ContextID, &e.VisitorID, &e.Type, &e.URL, &e.Title,
               &e.Referrer, &e.EventName, &propertiesJSON, &e.UserAgent, &e.IPHash,
               &e.UTMSource, &e.UTMMedium, &e.UTMCampaign, &e.CreatedAt,
           )
           if err != nil {
               return nil, 0, fmt.Errorf("scan event: %w", err)
           }
           if err := json.Unmarshal(propertiesJSON, &e.Properties); err != nil {
               return nil, 0, fmt.Errorf("unmarshal event properties: %w", err)
           }
           events = append(events, e)
       }
       if err := rows.Err(); err != nil {
           return nil, 0, fmt.Errorf("iterate events: %w", err)
       }
       return events, total, nil
   }
   ```

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 3: Create usecase functions for visitor operations

**Files:**
- Create: `internal/usecase/tracking/list_visitors.go`
- Create: `internal/usecase/tracking/get_visitor_detail.go`
- Create: `internal/usecase/tracking/get_visitor_events.go`

**Interfaces:**
- Consumes: `tracking.Repository.ListVisitors`, `tracking.Repository.GetVisitorByID`, `tracking.Repository.GetVisitorEvents`
- Produces: Validated usecase results with limit guards

Steps:
1. **Create `internal/usecase/tracking/list_visitors.go`**:
   ```go
   package tracking

   import (
       "fmt"

       "github.com/hekemen/automata/internal/domain/tracking"
   )

   // ListVisitors retrieves a paginated list of visitors for a context.
   func ListVisitors(repo tracking.Repository, contextID string, opts tracking.ListVisitorsOpts) ([]*tracking.Visitor, int64, error) {
       if contextID == "" {
           return nil, 0, fmt.Errorf("contextID required")
       }

       // Validate sort field
       validSorts := map[string]bool{"first_seen": true, "last_seen": true, "page_views": true}
       if opts.Sort != "" && !validSorts[opts.Sort] {
           return nil, 0, fmt.Errorf("invalid sort field: %s", opts.Sort)
       }

       // Validate limit
       if opts.Limit < 1 || opts.Limit > 100 {
           return nil, 0, fmt.Errorf("limit must be between 1 and 100")
       }

       visitors, total, err := repo.ListVisitors(nil, contextID, opts)
       if err != nil {
           return nil, 0, fmt.Errorf("list visitors: %w", err)
       }

       return visitors, total, nil
   }
   ```

2. **Create `internal/usecase/tracking/get_visitor_detail.go`**:
   ```go
   package tracking

   import (
       "fmt"
       "time"

       "github.com/hekemen/automata/internal/domain/tracking"
   )

   // GetVisitorDetail retrieves a visitor's profile along with recent events.
   type VisitorDetailResponse struct {
       Visitor      *tracking.Visitor `json:"visitor"`
       RecentEvents []*tracking.Event  `json:"recent_events"`
   }

   // GetVisitorDetail fetches a visitor by ID and their most recent events.
   func GetVisitorDetail(repo tracking.Repository, contextID, visitorID string, limit int) (*VisitorDetailResponse, error) {
       if contextID == "" {
           return nil, fmt.Errorf("contextID required")
       }
       if visitorID == "" {
           return nil, fmt.Errorf("visitorID required")
       }
       if limit <= 0 {
           limit = 20
       }
       if limit > 50 {
           limit = 50
       }

       visitor, err := repo.GetVisitorByID(nil, contextID, visitorID)
       if err != nil {
           return nil, fmt.Errorf("get visitor: %w", err)
       }

       // Fetch recent events
       events, _, err := repo.GetVisitorEvents(nil, contextID, visitorID, tracking.GetVisitorEventsOpts{
           Limit: limit,
       })
       if err != nil {
           return nil, fmt.Errorf("get visitor events: %w", err)
       }

       return &VisitorDetailResponse{
           Visitor:      visitor,
           RecentEvents: events,
       }, nil
   }
   ```

3. **Create `internal/usecase/tracking/get_visitor_events.go`**:
   ```go
   package tracking

   import (
       "fmt"

       "github.com/hekemen/automata/internal/domain/tracking"
   )

   // GetVisitorEvents retrieves a paginated list of events for a specific visitor.
   func GetVisitorEvents(repo tracking.Repository, contextID, visitorID string, opts tracking.GetVisitorEventsOpts) ([]*tracking.Event, int64, error) {
       if contextID == "" {
           return nil, 0, fmt.Errorf("contextID required")
       }
       if visitorID == "" {
           return nil, 0, fmt.Errorf("visitorID required")
       }

       // Validate type filter
       if opts.Type != "" && opts.Type != tracking.EventTypePageView && opts.Type != tracking.EventTypeEvent {
           return nil, 0, fmt.Errorf("invalid type value: %s", opts.Type)
       }

       // Validate limit
       if opts.Limit < 1 || opts.Limit > 100 {
           return nil, 0, fmt.Errorf("limit must be between 1 and 100")
       }

       events, total, err := repo.GetVisitorEvents(nil, contextID, visitorID, opts)
       if err != nil {
           return nil, 0, fmt.Errorf("get visitor events: %w", err)
       }

       return events, total, nil
   }
   ```

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 4: Add handler methods to `TrackingHandler`

**Files:**
- Modify: `internal/adapter/api/handler/tracking_handler.go`

**Interfaces:**
- Consumes: usecase functions from `trackingUsecase` package
- Produces: JSON responses for visitor listing, detail, and events

Steps:
1. **Add `ListVisitors` handler**:
   ```go
   // ListVisitors handles GET /api/admin/tracking/visitors — returns paginated visitor list.
   func (h *TrackingHandler) ListVisitors(c *gin.Context) {
       context := c.GetString("context")
       if context == "" {
           c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
           return
       }

       var opts tracking.ListVisitorsOpts

       if o := c.Query("offset"); o != "" {
           if p, err := strconv.Atoi(o); err == nil {
               opts.Offset = p
           }
       }
       if l := c.Query("limit"); l != "" {
           if p, err := strconv.Atoi(l); err == nil {
               opts.Limit = p
           }
       }
       if s := c.Query("sort"); s != "" {
           opts.Sort = s
       }

       visitors, total, err := trackingUsecase.ListVisitors(h.repo, context, opts)
       if err != nil {
           if err.Error() == fmt.Sprintf("invalid sort field: %s", opts.Sort) {
               c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sort field"})
               return
           }
           c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
           return
       }

       c.JSON(http.StatusOK, gin.H{
           "visitors": visitors,
           "total":    total,
       })
   }
   ```

2. **Add `GetVisitorDetail` handler**:
   ```go
   // GetVisitorDetail handles GET /api/admin/tracking/visitors/:id — returns visitor profile with recent events.
   func (h *TrackingHandler) GetVisitorDetail(c *gin.Context) {
       context := c.GetString("context")
       if context == "" {
           c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
           return
       }

       visitorID := c.Param("id")
       if visitorID == "" {
           c.JSON(http.StatusBadRequest, gin.H{"error": "visitor ID required"})
           return
       }

       limit := 20
       if l := c.Query("limit"); l != "" {
           if p, err := strconv.Atoi(l); err == nil && p > 0 {
               limit = p
           }
       }

       detail, err := trackingUsecase.GetVisitorDetail(h.repo, context, visitorID, limit)
       if err != nil {
           c.JSON(http.StatusNotFound, gin.H{"error": "visitor not found"})
           return
       }

       c.JSON(http.StatusOK, gin.H{
           "visitor":        detail.Visitor,
           "recent_events":  detail.RecentEvents,
       })
   }
   ```

3. **Add `GetVisitorEvents` handler**:
   ```go
   // GetVisitorEvents handles GET /api/admin/tracking/visitors/:id/events — returns paginated visitor event history.
   func (h *TrackingHandler) GetVisitorEvents(c *gin.Context) {
       context := c.GetString("context")
       if context == "" {
           c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
           return
       }

       visitorID := c.Param("id")
       if visitorID == "" {
           c.JSON(http.StatusBadRequest, gin.H{"error": "visitor ID required"})
           return
       }

       var opts tracking.GetVisitorEventsOpts

       if t := c.Query("type"); t != "" {
           opts.Type = tracking.EventType(t)
       }
       if o := c.Query("offset"); o != "" {
           if p, err := strconv.Atoi(o); err == nil {
               opts.Offset = p
           }
       }
       if l := c.Query("limit"); l != "" {
           if p, err := strconv.Atoi(l); err == nil {
               opts.Limit = p
           }
       }

       events, total, err := trackingUsecase.GetVisitorEvents(h.repo, context, visitorID, opts)
       if err != nil {
           c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
           return
       }

       c.JSON(http.StatusOK, gin.H{
           "events": events,
           "total":  total,
       })
   }
   ```

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 5: Register admin tracking routes in server

**Files:**
- Modify: `internal/adapter/api/server.go`

**Interfaces:**
- Consumes: existing `trackingHandler` instance
- Produces: `GET /api/admin/tracking/visitors`, `GET /api/admin/tracking/visitors/:id`, `GET /api/admin/tracking/visitors/:id/events`

Steps:
1. **Create the tracking handler** inside the admin group scope:
   ```go
   trackingHandler := handler.NewTrackingHandler(repo)
   ```
   (Need to create a tracking repository instance — pass `pool` from function params.)

2. **Register the three visitor routes** inside the `admin` group block (after the existing admin routes, before the closing brace):
   ```go
       // Tracking visitor routes
       admin.GET("/tracking/visitors", trackingHandler.ListVisitors)
       admin.GET("/tracking/visitors/:id", trackingHandler.GetVisitorDetail)
       admin.GET("/tracking/visitors/:id/events", trackingHandler.GetVisitorEvents)
   ```

The full admin block should look like:
```go
    admin := api.Group("/admin")
    admin.Use(middleware.AuthMiddleware(authService))
    {
        // ... existing routes ...

        trackingRepo := repo.New(pool)
        trackingHandler := handler.NewTrackingHandler(trackingRepo)

        // Tracking visitor routes
        admin.GET("/tracking/visitors", trackingHandler.ListVisitors)
        admin.GET("/tracking/visitors/:id", trackingHandler.GetVisitorDetail)
        admin.GET("/tracking/visitors/:id/events", trackingHandler.GetVisitorEvents)
    }
```

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 6: Add database indexes for sorting

**Files:**
- Modify: `internal/infrastructure/tracking/repo/migration.sql`

**Interfaces:**
- Consumes: existing `tracking_visitors` table
- Produces: Composite indexes supporting efficient sorted queries

Steps:
1. **Append index creation statements** to the end of `migration.sql`:
   ```sql
   -- Indexes for visitor listing sort operations
   CREATE INDEX IF NOT EXISTS idx_tracking_visitors_context_first_seen
       ON tracking_visitors(context_id, first_seen DESC);
   CREATE INDEX IF NOT EXISTS idx_tracking_visitors_context_last_seen
       ON tracking_visitors(context_id, last_seen DESC);
   CREATE INDEX IF NOT EXISTS idx_tracking_visitors_context_page_views
       ON tracking_visitors(context_id, page_views DESC);
   ```

These are append-only, idempotent migrations using `CREATE INDEX IF NOT EXISTS`.

**Verification:**
- `go build ./...` compiles clean

---

### Task 7: Add unit tests

**Files:**
- Create: `internal/infrastructure/tracking/repo/postgres_test.go`
- Create: `internal/usecase/tracking/list_visitors_test.go`
- Create: `internal/usecase/tracking/get_visitor_detail_test.go`
- Create: `internal/usecase/tracking/get_visitor_events_test.go`

**Interfaces:**
- Consumes: Ginkgo + testcontainers infrastructure (from `cicd/support/`)
- Produces: Test coverage for repository queries, usecase validation, limit guards

Steps:
1. **Repository tests** (using testcontainers):
   - Test `ListVisitors` with different sort fields returns correct ordering
   - Test `GetVisitorByID` returns visitor or 404
   - Test `GetVisitorEvents` with type filter returns correct events
   - Test limit enforcement (max 100)

2. **Usecase tests** (using mock or inline):
   - Test `ListVisitors` rejects invalid sort fields
   - Test `ListVisitors` enforces limit bounds (1-100)
   - Test `GetVisitorDetail` returns detail with recent events
   - Test `GetVisitorEvents` rejects invalid type values

3. **Handler tests**:
   - Test `ListVisitors` handler returns 400 for invalid sort
   - Test `GetVisitorDetail` handler returns 404 for missing visitor
   - Test `GetVisitorEvents` handler parses query params correctly

**Verification:**
- `go test ./internal/infrastructure/tracking/repo/...` passes
- `go test ./internal/usecase/tracking/...` passes
- `go test ./internal/adapter/api/handler/...` passes
