# Core Platform Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundation for Automata: multi-tenant auth, reverse proxy, MCP server, config store, and email queue.

**Architecture:** Hexagonal architecture with Go. The API server handles admin operations, the reverse proxy routes tenant traffic, MCP servers expose tools for AI integration, and shared infrastructure (config store, email queue) supports all features.

**Tech Stack:** Go 1.26+, PostgreSQL, zerolog, Ginkgo + testcontainers, Gin (HTTP framework)

**Spec:** docs/prds/2025-09-10-core-platform-prd.md

## Global Constraints

- Hexagonal architecture: domain interfaces in `internal/domain`, use cases in `internal/usecase`, adapters in `internal/adapter`, infrastructure in `internal/infrastructure`
- KISS and DRY principles throughout
- Multi-tenant: all feature tables include `tenant_id`, data strictly isolated per tenant
- Configuration via key-value store backed by YAML file, overridden by environment variables
- Self-hosted deployment as single binary or Docker container
- No UI in Phase 1 — backend API only
- All tests use Ginkgo + testcontainers for PostgreSQL

---

### Task 1: Project scaffolding and configuration store

**Files:**
- Create: `internal/infrastructure/config/config.go`
- Create: `internal/infrastructure/config/loader.go`
- Create: `pkg/config/config_test.go`
- Modify: `go.mod` (add dependencies: gin, gopkg.in/yaml.v3, zerolog)
- Create: `cmd/automata/main.go`

**Interfaces:**
- Consumes: none (foundation layer)
- Produces: `config.Get(key string) string`, `config.Set(key, value string)`, `config.Load(path string) error`

- [ ] **Step 1: Add dependencies to go.mod**

Run: `go get github.com/gin-gonic/gin github.com/rs/zerolog/log gopkg.in/yaml.v3`

- [ ] **Step 2: Write config loader with YAML + env var override**

```go
// internal/infrastructure/config/config.go
package config

import (
    "os"
    "sync"
)

type Config struct {
    mu     sync.RWMutex
    values map[string]string
}

var global = &Config{values: make(map[string]string)}

func Get(key string) string {
    global.mu.RLock()
    defer global.mu.RUnlock()
    return global.values[key]
}

func Set(key, value string) {
    global.mu.Lock()
    defer global.mu.Unlock()
    global.values[key] = value
}

func Load(path string) error {
    // Read YAML file, flatten nested keys to dot-notation
    // Then override with environment variables (format: AUTOMATA_SERVER_HOST)
    return nil
}
```

- [ ] **Step 3: Write unit tests for config loading**

```go
// pkg/config/config_test.go
package config_test

import "testing"

func TestConfigGetDefault(t *testing.T) {
    // Test Get returns empty string for unset keys
}

func TestConfigSetAndGet(t *testing.T) {
    // Test Set/Get round-trip
}

func TestConfigLoadYAML(t *testing.T) {
    // Test loading from YAML file with nested keys flattened to dot notation
}

func TestConfigEnvOverride(t *testing.T) {
    // Test environment variables override YAML values
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./pkg/config/... -v`
Expected: FAIL (config package not yet implemented)

- [ ] **Step 5: Implement minimal config store**

Implement `Load` to parse YAML, flatten keys, merge with env vars.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./pkg/config/... -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/infrastructure/config/ pkg/config/
git commit -m "feat: add key-value configuration store with YAML and env var support"
```

---

### Task 2: Database setup and migration

**Files:**
- Create: `internal/infrastructure/database/migration.go`
- Create: `internal/infrastructure/database/connection.go`
- Create: `internal/infrastructure/database/migration_test.go`

**Interfaces:**
- Consumes: `config.Get("database.host")`, etc.
- Produces: `db.Exec()`, `db.Query()` with tenant-aware queries

- [ ] **Step 1: Add database driver dependency**

Run: `go get github.com/jackc/pgx/v5`

- [ ] **Step 2: Write database connection helper**

```go
// internal/infrastructure/database/connection.go
package database

import "github.com/jackc/pgx/v5/pgxpool"

func NewPool() (*pgxpool.Pool, error) {
    // Build DSN from config.Get("database.*")
    // Return pgxpool.Pool
}
```

- [ ] **Step 3: Write migration SQL for all core tables**

```sql
-- internal/infrastructure/database/migration.sql
CREATE TABLE tenants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    domain          VARCHAR(256),
    is_active       BOOLEAN DEFAULT true,
    settings        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE tenant_users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email       VARCHAR(254) NOT NULL,
    password_hash TEXT NOT NULL,
    is_owner    BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_tenant_users_tenant_email ON tenant_users(tenant_id, email);

CREATE TABLE api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES tenant_users(id) ON DELETE CASCADE,
    key_hash    VARCHAR(64) NOT NULL,
    name        TEXT NOT NULL,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_api_keys_tenant_name ON api_keys(tenant_id, name);
```

- [ ] **Step 4: Write migration runner**

```go
// internal/infrastructure/database/migration.go
func RunMigrations(db *pgxpool.Pool) error {
    // Execute migration SQL in order
}
```

- [ ] **Step 5: Write integration tests with testcontainers**

```go
// internal/infrastructure/database/migration_test.go
package database_test

import . "github.com/onsi/ginkgo/v2"

var _ = Describe("Migration", func() {
    It("creates all tables", func() {})
    It("creates all indexes", func() {})
    It("handles idempotent re-runs", func() {})
})
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `ginkgo internal/infrastructure/database/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/infrastructure/database/
git commit -m "feat: add database schema and migration for tenants, users, and API keys"
```

---

### Task 3: Tenant domain entity and repository interface

**Files:**
- Create: `internal/domain/tenant/tenant.go`
- Create: `internal/domain/tenant/repository.go`
- Create: `internal/infrastructure/tenant/repo/postgres.go`
- Create: `internal/infrastructure/tenant/repo/postgres_test.go`

**Interfaces:**
- Consumes: `database.NewPool()`
- Produces: `Tenant` entity, `Repository` interface with `Create`, `GetBySlug`, `GetByID`, `List`, `Update`, `Delete`

- [ ] **Step 1: Define Tenant entity and Repository interface**

```go
// internal/domain/tenant/tenant.go
package tenant

import "time"

type Tenant struct {
    ID        string
    Slug      string
    Name      string
    Domain    string
    IsActive  bool
    Settings  map[string]interface{}
    CreatedAt time.Time
    UpdatedAt time.Time
}

// internal/domain/tenant/repository.go
package tenant

type Repository interface {
    Create(t *Tenant) error
    GetByID(id string) (*Tenant, error)
    GetBySlug(slug string) (*Tenant, error)
    List(offset, limit int) ([]*Tenant, error)
    Update(t *Tenant) error
    Delete(id string) error
}
```

- [ ] **Step 2: Write failing unit tests for entity validation**

```go
// internal/domain/tenant/tenant_test.go
func TestTenantValidation(t *testing.T) {
    // Slug must be non-empty, unique
    // Name must be non-empty
}
```

- [ ] **Step 3: Implement PostgreSQL repository**

```go
// internal/infrastructure/tenant/repo/postgres.go
func NewPostgresRepo(pool *pgxpool.Pool) Repository {
    // Store pool reference
}

func (r *repo) Create(t *Tenant) error {
    // INSERT INTO tenants ... RETURNING id, created_at, updated_at
}

func (r *repo) GetBySlug(slug string) (*Tenant, error) {
    // SELECT * FROM tenants WHERE slug = $1
}
```

- [ ] **Step 4: Run integration tests**

Run: `ginkgo internal/infrastructure/tenant/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/tenant/ internal/infrastructure/tenant/
git commit -m "feat: add tenant domain entity and PostgreSQL repository"
```

---

### Task 4: Tenant user domain and auth domain

**Files:**
- Create: `internal/domain/tenant/user.go`
- Create: `internal/domain/tenant/user_repository.go`
- Create: `internal/domain/auth/auth.go`
- Create: `internal/domain/auth/token.go`
- Create: `internal/infrastructure/tenant/repo/user_postgres.go`
- Create: `internal/infrastructure/auth/repo/api_key_postgres.go`

**Interfaces:**
- Consumes: `database.NewPool()`
- Produces: `User` entity, `APIKey` entity, `AuthService` interface with `Login`, `VerifyToken`, `CreateAPIKey`, `ValidateAPIKey`

- [ ] **Step 1: Define User, APIKey entities and auth interfaces**

```go
// internal/domain/tenant/user.go
type User struct {
    ID          string
    TenantID    string
    Email       string
    PasswordHash string
    IsOwner     bool
}

// internal/domain/auth/auth.go
type AuthService interface {
    Login(email, password string) (*Session, error)
    VerifyToken(token string) (*Session, error)
    CreateAPIKey(userID, tenantID, name string, expiresAt *time.Time) (*APIKey, error)
    ValidateAPIKey(key string) (*APIKey, error)
    HashPassword(password string) (string, error)
    ComparePassword(hash, password string) error
}
```

- [ ] **Step 2: Write unit tests for password hashing**

```go
func TestPasswordHashing(t *testing.T) {
    // Hash and compare round-trip
    // Different passwords produce different hashes
}
```

- [ ] **Step 3: Implement auth service with bcrypt**

Add dependency: `go get golang.org/x/crypto/bcrypt`

```go
func (s *service) Login(email, password string) (*Session, error) {
    // Find user by email, compare password, generate JWT token
}
```

- [ ] **Step 4: Implement API key repository**

```go
func (r *repo) Create(key *APIKey) error {
    // INSERT INTO api_keys (key_hash = bcrypt of raw key)
}

func (r *repo) ValidateAPIKey(rawKey string) (*APIKey, error) {
    // Lookup by prefix, compare hash
}
```

- [ ] **Step 5: Run integration tests**

Run: `ginkgo internal/domain/auth/... internal/infrastructure/auth/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/domain/tenant/user.go internal/domain/auth/ internal/infrastructure/tenant/repo/user_postgres.go internal/infrastructure/auth/
git commit -m "feat: add tenant user, API key entities and auth service"
```

---

### Task 5: Tenant resolution middleware

**Files:**
- Create: `internal/adapter/api/tenant_middleware.go`
- Create: `internal/adapter/api/tenant_middleware_test.go`

**Interfaces:**
- Consumes: `tenant.Repository.GetBySlug()`, `tenant.Repository.GetByDomain()`
- Produces: Gin middleware that sets `tenant.ContextKey` in request context

- [ ] **Step 1: Write tenant resolution logic**

```go
// internal/adapter/api/tenant_middleware.go
func TenantResolver(tenantRepo tenant.Repository) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Try subdomain first: extract from Host header
        // Fall back to path: /tenant/<slug>
        // Set tenant in context
    }
}
```

- [ ] **Step 2: Write unit tests for subdomain resolution**

```go
func TestSubdomainResolution(t *testing.T) {
    // "tenant.example.com" -> tenant slug "tenant"
    // "app.example.com" -> tenant slug "app"
}

func TestPathResolution(t *testing.T) {
    // "/tenant/myapp/forms" -> tenant slug "myapp"
}

func TestNoTenant(t *testing.T) {
    // No subdomain, no path -> 400 error
}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `go test ./internal/adapter/api/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/api/tenant_middleware.go
git commit -m "feat: add tenant resolution middleware for subdomain and path-based routing"
```

---

### Task 6: API server with auth routes

**Files:**
- Create: `internal/adapter/api/server.go`
- Create: `internal/adapter/api/handler/auth_handler.go`
- Create: `internal/adapter/api/handler/tenant_handler.go`
- Create: `internal/adapter/api/middleware/auth.go`
- Create: `internal/usecase/tenant/list_tenants.go`
- Create: `internal/usecase/tenant/create_tenant.go`
- Create: `internal/usecase/auth/login.go`
- Create: `internal/usecase/auth/create_api_key.go`
- Create: `internal/adapter/api/server_test.go`

**Interfaces:**
- Consumes: `tenant.Repository`, `auth.AuthService`
- Produces: HTTP API routes per PRD spec

- [ ] **Step 1: Define API routes and server setup**

```go
// internal/adapter/api/server.go
func NewServer(cfg config, tenantRepo tenant.Repository, auth auth.AuthService) *gin.Engine {
    engine := gin.Default()
    engine.Use(TenantResolver(tenantRepo))
    engine.Use(AuthMiddleware(auth))

    // Public routes
    engine.POST("/auth/login", authHandler.Login)
    engine.POST("/auth/logout", authHandler.Logout)

    // Admin routes
    admin := engine.Group("/admin")
    admin.GET("/tenants", tenantHandler.List)
    admin.POST("/tenants", tenantHandler.Create)
    // ...
}
```

- [ ] **Step 2: Implement auth handler (login, logout, API keys)**

```go
func (h *handler) Login(c *gin.Context) {
    var req LoginRequest
    // Bind JSON, call auth.Login, return token + tenant_id
}
```

- [ ] **Step 3: Implement tenant handler (CRUD)**

```go
func (h *handler) Create(c *gin.Context) {
    var req CreateTenantRequest
    // Validate, call usecase, return created tenant
}
```

- [ ] **Step 4: Write integration tests for auth flow**

```go
func TestAuthFlow(t *testing.T) {
    // Create tenant -> Create user -> Login -> Get token -> Access protected route
}

func TestAPIKeyAuth(t *testing.T) {
    // Create API key -> Use key in Authorization header -> Access protected route
}
```

- [ ] **Step 5: Run integration tests with testcontainers**

Run: `ginkgo internal/adapter/api/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/api/ internal/usecase/tenant/ internal/usecase/auth/
git commit -m "feat: add API server with auth and tenant management routes"
```

---

### Task 7: Reverse proxy adapter

**Files:**
- Create: `internal/adapter/proxy/proxy.go`
- Create: `internal/adapter/proxy/proxy_test.go`

**Interfaces:**
- Consumes: `tenant.Repository`
- Produces: Gin router that serves tenant content at `/form/<slug>`, `/snippet/<id>.js`, static assets

- [ ] **Step 1: Implement reverse proxy router**

```go
// internal/adapter/proxy/proxy.go
func NewProxy(tenantRepo tenant.Repository) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Resolve tenant from subdomain/path
        // Route /form/* -> form renderer
        // Route /snippet/* -> snippet generator
        // Route /static/* -> static file server
    }
}
```

- [ ] **Step 2: Write routing tests**

```go
func TestFormRoute(t *testing.T) {
    // GET /form/contact -> returns form HTML for tenant
}

func TestSnippetRoute(t *testing.T) {
    // GET /snippet/<tenant-id>.js -> returns JS snippet
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/adapter/proxy/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/proxy/
git commit -m "feat: add reverse proxy for tenant content routing"
```

---

### Task 8: MCP server base and contact tools

**Files:**
- Create: `internal/adapter/mcp/server.go`
- Create: `internal/adapter/mcp/contacts_server.go`
- Create: `internal/adapter/mcp/contacts_tools.go`
- Create: `internal/adapter/mcp/server_test.go`

**Interfaces:**
- Consumes: `contact.Repository` (from later plan), `tenant.Repository`
- Produces: MCP server exposing `contacts.list`, `contacts.get`, `contacts.create`, `contacts.update`, `contacts.delete`, `contacts.merge`, `contacts.import`, `contacts.export`

- [ ] **Step 1: Set up MCP server base**

```go
// internal/adapter/mcp/server.go
package mcp

import (
    "context"
    "github.com/modelcontextprotocol/go/mcp"
)

type Server struct {
    *mcp.Server
    tenantRepo tenant.Repository
}

func NewServer(tenantRepo tenant.Repository) *Server {
    // Initialize MCP server
}
```

- [ ] **Step 2: Implement contact tools**

```go
// internal/adapter/mcp/contacts_tools.go
func (s *Server) registerContactTools() {
    s.AddTool("contacts.list", "List contacts",
        func(ctx context.Context, req struct{ TenantID string, Filters map[string]string }) ([]Contact, error) {
            // Call usecase
        }),
    s.AddTool("contacts.create", "Create a contact",
        func(ctx context.Context, req struct{ TenantID string, Data map[string]interface{} }) (Contact, error) {
            // Call usecase
        }),
    // ... other tools
}
```

- [ ] **Step 3: Write MCP tool tests**

```go
func TestContactsListTool(t *testing.T) {
    // Call contacts.list, verify returns contacts for tenant
}

func TestContactsCreateTool(t *testing.T) {
    // Call contacts.create, verify contact created
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/adapter/mcp/... -v`
Expected: PASS (may need stubs for contact repo)

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/
git commit -m "feat: add MCP server base and contact tools"
```

---

### Task 9: Email queue infrastructure

**Files:**
- Create: `internal/infrastructure/queue/email_queue.go`
- Create: `internal/infrastructure/queue/email_worker.go`
- Create: `internal/infrastructure/queue/email_queue_test.go`
- Create: `internal/infrastructure/storage/storage.go`

**Interfaces:**
- Consumes: `database.NewPool()`
- Produces: `Queue.Enqueue(email *EmailJob) error`, `Queue.StartWorker()` with retry logic

- [ ] **Step 1: Define email job schema and queue interface**

```go
// internal/infrastructure/queue/email_queue.go
type EmailJob struct {
    ID         string
    TenantID   string
    To         []string
    Subject    string
    Body       string
    Attempts   int
    MaxRetries int
    NextRetry  time.Time
}

type Queue interface {
    Enjob(job *EmailJob) error
    StartWorker(ctx context.Context)
}
```

- [ ] **Step 2: Implement DB-backed queue with retry**

```go
func (q *queue) Enqueue(job *EmailJob) error {
    // INSERT INTO email_jobs ...
}

func (q *queue) StartWorker(ctx context.Context) {
    // Poll for jobs where NextRetry <= NOW()
    // Send email, update attempts, schedule retry on failure
}
```

- [ ] **Step 3: Write integration tests with mock mailer**

```go
func TestEmailQueueEnqueueAndProcess(t *testing.T) {
    // Enqueue job, verify worker processes it
}

func TestEmailRetryOnFailure(t *testing.T) {
    // Simulate send failure, verify retry schedule
}
```

- [ ] **Step 4: Run tests**

Run: `ginkgo internal/infrastructure/queue/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/queue/ internal/infrastructure/storage/
git commit -m "feat: add DB-backed email queue with retry logic"
```

---

### Task 10: Main entry point, health checks, and Docker setup

**Files:**
- Modify: `cmd/automata/main.go`
- Create: `internal/adapter/api/health_handler.go`
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Create: `automata.example.yaml`

**Interfaces:**
- Consumes: all previous tasks
- Produces: runnable binary that starts API server, proxy, MCP server, and email worker

- [ ] **Step 1: Wire all components in main.go**

```go
// cmd/automata/main.go
func main() {
    config.Load("automata.yaml")
    db := database.NewPool()
    database.RunMigrations(db)

    tenantRepo := tenant.NewPostgresRepo(db)
    authSvc := auth.NewService(tenantRepo)

    apiServer := api.NewServer(config, tenantRepo, authSvc)
    proxy := proxy.NewProxy(tenantRepo)
    mcpServer := mcp.NewServer(tenantRepo)
    emailQueue := queue.NewEmailQueue(db)

    // Start all servers
}
```

- [ ] **Step 2: Implement health check endpoint**

```go
func (h *handler) Health(c *gin.Context) {
    c.JSON(200, gin.H{"status": "ok"})
}
```

- [ ] **Step 3: Write Dockerfile**

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o automata ./cmd/automata

FROM alpine:3.20
COPY --from=builder /app/automata /usr/local/bin/
EXPOSE 8080 9000
CMD ["automata"]
```

- [ ] **Step 4: Write docker-compose.yml**

```yaml
services:
  automata:
    build: .
    ports:
      - "8080:8080"
      - "9000:9000"
    depends_on:
      - postgres
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: automata
      POSTGRES_USER: automata
      POSTGRES_PASSWORD: changeme
```

- [ ] **Step 5: Write example config**

Copy from PRD config section to `automata.example.yaml`.

- [ ] **Step 6: Run end-to-end test**

Build and run: `go build ./cmd/automata && ./automata`
Verify: `curl http://localhost:8080/health` returns `{"status":"ok"}`

- [ ] **Step 7: Commit**

```bash
git add cmd/automata/ internal/adapter/api/health_handler.go Dockerfile docker-compose.yml automata.example.yaml
git commit -m "feat: add main entry point, health checks, and Docker deployment"
```

---

## Testing Strategy

- **Unit tests:** Config loading, tenant resolution, password hashing, entity validation — `go test ./...`
- **Integration tests:** Database operations, auth flow, email queue — Ginkgo + testcontainers for PostgreSQL
- **E2E tests:** Full API and MCP layer tests in `cicd/` directory
- **Load tests:** 100 concurrent auth requests, 50 concurrent proxy requests
- **Mail/IMAP mocks:** For email queue integration validation

## Task Dependencies

```
Task 1 (config) ──┬──> Task 2 (database) ──> Task 3 (tenant repo) ──> Task 4 (auth) ──> Task 6 (API server)
                   │                          │
                   │                          └──> Task 5 (tenant middleware) ──> Task 6
                   │
                   └──> Task 7 (proxy) ──────────────────────────────────────> Task 10
                                                                              
Task 4 (auth) ──────────────────────────────────────────────────────────────> Task 6
Task 3 (tenant repo) ──────────────────────────────────────────────────────> Task 8 (MCP)
Task 2 (database) ─────────────────────────────────────────────────────────> Task 9 (email queue)
Task 6 (API server) + Task 7 (proxy) + Task 8 (MCP) + Task 9 (email) ──> Task 10 (main)
```
