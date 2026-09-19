# Architectural Changes: Dual-Server, Admin Config, Banner Management, Tracking

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the single-server architecture into dual servers (admin + tracking), add global login with tenant switching, create an admin config subsystem, wire up form and banner routes, and update the tracking script.

**Architecture:** The main Gin server (port 8080) handles admin API, auth, SPA serving. A separate tracking Gin server (port 8081) handles public-facing endpoints: tracking events, snippet serving, banner serving, form rendering/submission. Both servers share the same PostgreSQL database. The reverse proxy routes `/api/*` to the main server and everything else to the tracking server.

**Tech Stack:** Go 1.26.5, Gin v1.12.0, PostgreSQL (pgx v5), JWT (golang-jwt/v5), Vue 3 + TypeScript frontend, Pinia, Playwright E2E tests.

**Spec:** `docs/superpowers/specs/2026-09-19-architectural-changes-design.md`

## Global Constraints

- Module path: `github.com/hekemen/automata` (from `go.mod`)
- Go version: `1.26.5`
- All UUIDs must be valid UUID format strings
- JWT secret key: `config.Get("auth.secret_key")`, defaults to `"automata-dev-secret-key-change-in-production"`
- Tracking server port: `8081` (configurable via `AUTOMATA_TRACKING_PORT`, default `"8081"`)
- Main server port: `8080` (configurable via `AUTOMATA_SERVER_PORT`, default `"8080"`)
- Config loading: `config.Load(path)` reads YAML, flattens to dot-notation, overrides with `AUTOMATA_*` env vars
- Database migrations: embedded via `//go:embed migration.sql` in `internal/infrastructure/database/migration.go`
- Hexagonal architecture: domain interfaces in `internal/domain/`, implementations in `internal/infrastructure/`, HTTP handlers in `internal/adapter/`

---

## Phase 1: Database Migration

### Task 1.1: Add admin_configs table to migration.sql

**Files:**
- Modify: `internal/infrastructure/database/migration.sql`

**Interfaces:**
- Consumes: existing `tenants` table (must exist before this migration)
- Produces: `admin_configs` table with `tenant_id` FK to `tenants(id)`

- [ ] **Step 1: Append admin_configs table definition to migration.sql**

Add the following SQL at the end of `internal/infrastructure/database/migration.sql` (after the existing `api_keys` table definition):

```sql
CREATE TABLE IF NOT EXISTS admin_configs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key         VARCHAR(128) NOT NULL,
    value       JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, key)
);

CREATE INDEX IF NOT EXISTS idx_admin_configs_tenant ON admin_configs(tenant_id);
```

**Verification:**
```bash
cd /home/hekemen/automata
grep -c "admin_configs" internal/infrastructure/database/migration.sql
# Expected: 3 (CREATE TABLE, UNIQUE constraint, CREATE INDEX)
```

---

## Phase 2: Auth Changes (Global Login)

### Task 2.1: Add GetByEmailGlobal to UserRepository interface

**Files:**
- Modify: `internal/domain/tenant/user_repository.go`
- Modify: `internal/infrastructure/tenant/repo/user_postgres.go`

**Interfaces:**
- Consumes: existing `UserRepository` interface
- Produces: `GetByEmailGlobal(email string) ([]*User, error)` — returns all users matching the email across all tenants

- [ ] **Step 1: Add GetByEmailGlobal method to UserRepository interface**

In `internal/domain/tenant/user_repository.go`, add the method to the interface:

**OLD:**
```go
type UserRepository interface {
	Create(u *User) error
	GetByID(id string) (*User, error)
	GetByEmail(tenantID, email string) (*User, error)
	Update(u *User) error
	Delete(id string) error
}
```

**NEW:**
```go
type UserRepository interface {
	Create(u *User) error
	GetByID(id string) (*User, error)
	GetByEmail(tenantID, email string) (*User, error)
	GetByEmailGlobal(email string) ([]*User, error)
	Update(u *User) error
	Delete(id string) error
}
```

- [ ] **Step 2: Implement GetByEmailGlobal in user_postgres.go**

In `internal/infrastructure/tenant/repo/user_postgres.go`, add the method after the existing `GetByEmail` method (before `Update`):

```go
func (r *userPostgresRepo) GetByEmailGlobal(email string) ([]*tenant.User, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, is_owner, created_at, updated_at
		FROM tenant_users WHERE email = $1
	`

	rows, err := r.pool.Query(context.Background(), query, email)
	if err != nil {
		return nil, fmt.Errorf("get user by email (global): %w", err)
	}
	defer rows.Close()

	var users []*tenant.User
	for rows.Next() {
		u := &tenant.User{}
		err := rows.Scan(
			&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.IsOwner,
			&u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return users, nil
}
```

**Verification:**
```bash
cd /home/hekemen/automata
go build ./internal/domain/tenant/
go build ./internal/infrastructure/tenant/repo/
```
### Task 2.2: Update AuthService interface and Login implementation

**Files:**
- Modify: `internal/domain/auth/auth.go`
- Modify: `internal/infrastructure/auth/service.go`

**Interfaces:**
- Consumes: `UserRepository.GetByEmailGlobal`
- Produces: `Login` with new signature returning tenant list

- [ ] **Step 1: Update AuthService interface in auth.go**

In `internal/domain/auth/auth.go`, change the `Login` method signature and add `VerifyTokenUserOnly`:

**OLD:**
```go
type AuthService interface {
	Login(email, password, tenantSlug string) (token string, userID string, err error)
	VerifyToken(token string) (userID string, tenantID string, err error)
	CreateAPIKey(userID, tenantID, name string, plaintextKey string, expiresAt *time.Time) (*APIKey, error)
	ValidateAPIKey(key string) (*APIKey, error)
	HashPassword(password string) (string, error)
	ComparePassword(hash, password string) error
}
```

**NEW:**
```go
type TenantInfo struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type AuthService interface {
	Login(email, password string) (token string, userID string, tenants []TenantInfo, err error)
	VerifyToken(token string) (userID string, tenantID string, err error)
	VerifyTokenUserOnly(token string) (userID string, err error)
	CreateAPIKey(userID, tenantID, name string, plaintextKey string, expiresAt *time.Time) (*APIKey, error)
	ValidateAPIKey(key string) (*APIKey, error)
	HashPassword(password string) (string, error)
	ComparePassword(hash, password string) error
}
```

- [ ] **Step 2: Update Login implementation in service.go**

In `internal/infrastructure/auth/service.go`, replace the entire `Login` method:

**OLD:**
```go
func (s *service) Login(email, password, tenantID string) (string, string, error) {
	secretKey := config.Get("auth.secret_key")
	if secretKey == "" {
		secretKey = "automata-dev-secret-key-change-in-production"
	}

	if s.userRepo == nil {
		return "", "", fmt.Errorf("user repository not initialized")
	}

	user, err := s.userRepo.GetByEmail(tenantID, email)
	if err != nil {
		return "", "", fmt.Errorf("invalid email or password")
	}

	if err := s.ComparePassword(user.PasswordHash, password); err != nil {
		return "", "", fmt.Errorf("invalid email or password")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":     user.ID,
		"tenant_slug": user.TenantID,
		"exp":         time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		log.Error().Err(err).Str("secret_key_prefix", string([]byte(secretKey)[:min(10, len(secretKey))])).Msg("token generation failed")
		return "", "", fmt.Errorf("generate token: %w", err)
	}

	return tokenString, user.ID, nil
}
```

**NEW:**
```go
func (s *service) Login(email, password string) (string, string, []TenantInfo, error) {
	secretKey := config.Get("auth.secret_key")
	if secretKey == "" {
		secretKey = "automata-dev-secret-key-change-in-production"
	}

	if s.userRepo == nil {
		return "", "", nil, fmt.Errorf("user repository not initialized")
	}

	users, err := s.userRepo.GetByEmailGlobal(email)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid email or password")
	}

	// Verify password against the first matching user
	if err := s.ComparePassword(users[0].PasswordHash, password); err != nil {
		return "", "", nil, fmt.Errorf("invalid email or password")
	}

	// Build tenant list for all matching users
	tenants := make([]TenantInfo, 0, len(users))
	for _, u := range users {
		tenants = append(tenants, TenantInfo{
			ID:   u.TenantID,
			Slug: "", // Will be filled by handler from tenantRepo
			Name: "", // Will be filled by handler from tenantRepo
		})
	}

	// Generate JWT without tenant_slug (tenant comes from header)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    users[0].ID,
		"user_email": email,
		"exp":        time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		log.Error().Err(err).Str("secret_key_prefix", string([]byte(secretKey)[:min(10, len(secretKey))])).Msg("token generation failed")
		return "", "", nil, fmt.Errorf("generate token: %w", err)
	}

	return tokenString, users[0].ID, tenants, nil
}
```

- [ ] **Step 3: Add VerifyTokenUserOnly method to service.go**

Add this method after the existing `VerifyToken` method in `internal/infrastructure/auth/service.go`:

```go
// VerifyTokenUserOnly verifies a JWT token and returns only the user ID.
// It does not require a tenant claim, making it suitable for the new global login flow.
func (s *service) VerifyTokenUserOnly(tokenStr string) (string, error) {
	secretKey := config.Get("auth.secret_key")
	if secretKey == "" {
		secretKey = "automata-dev-secret-key-change-in-production"
	}

	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", fmt.Errorf("missing user_id in token")
	}

	return userID, nil
}
```

**Verification:**
```bash
cd /home/hekemen/automata
go build ./internal/domain/auth/
go build ./internal/infrastructure/auth/
```
### Task 2.3: Update auth handler Login

**Files:**
- Modify: `internal/adapter/api/handler/auth_handler.go`

**Interfaces:**
- Consumes: `AuthService.Login(email, password)` returning `(token, userID, tenants, err)`
- Consumes: `TenantRepository.GetByID(id)` for filling tenant names
- Produces: Login response with `tenants` array, no `tenant_id` in response

- [ ] **Step 1: Update LoginRequest struct and Login handler**

In `internal/adapter/api/handler/auth_handler.go`, replace the `LoginRequest` struct and `Login` method:

**OLD LoginRequest:**
```go
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	Tenant   string `json:"tenant" binding:"required"`
}
```

**NEW LoginRequest:**
```go
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}
```

**OLD Login handler (lines 37-77):**
```go
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Info().Str("tenant_slug", req.Tenant).Msg("login attempt")

	t, err := h.tenantRepo.GetBySlug(req.Tenant)
	if err != nil {
		log.Error().Err(err).Str("tenant_slug", req.Tenant).Msg("tenant lookup failed")
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found", "debug": req.Tenant})
		return
	}

	u, err := h.userRepo.GetByEmail(t.ID, req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := h.authService.ComparePassword(u.PasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, _, err := h.authService.Login(req.Email, req.Password, t.ID)
	if err != nil {
		log.Error().Err(err).Msg("auth service login failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"user_id":   u.ID,
		"tenant_id": t.ID,
		"email":     u.Email,
	})
}
```

**NEW Login handler:**
```go
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Info().Str("email", req.Email).Msg("login attempt")

	token, userID, tenantInfos, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		log.Error().Err(err).Str("email", req.Email).Msg("auth service login failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Fill tenant details from tenantRepo
	tenants := make([]gin.H, 0, len(tenantInfos))
	for _, ti := range tenantInfos {
		t, err := h.tenantRepo.GetByID(ti.ID)
		if err != nil {
			continue
		}
		tenants = append(tenants, gin.H{
			"id":   t.ID,
			"slug": t.Slug,
			"name": t.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": userID,
		"email":   req.Email,
		"tenants": tenants,
	})
}
```

**Verification:**
```bash
cd /home/hekemen/automata
go build ./internal/adapter/api/handler/
```

### Task 2.4: Update auth middleware to use VerifyTokenUserOnly

**Files:**
- Modify: `internal/adapter/api/middleware/auth.go`

**Interfaces:**
- Consumes: `AuthService.VerifyTokenUserOnly(token)` for JWT validation
- Produces: Sets `user_id` in context; tenant comes from `X-Tenant-ID` header

- [ ] **Step 1: Replace AuthMiddleware implementation**

In `internal/adapter/api/middleware/auth.go`, replace the entire `AuthMiddleware` function:

**OLD:**
```go
func AuthMiddleware(authService auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/auth/login") || strings.HasPrefix(path, "/health") {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		userID, tenantSlug, err := authService.VerifyToken(token)
		if err == nil {
			c.Set("user_id", userID)
			c.Set("tenant_id", tenantSlug)
			c.Next()
			return
		}

		apiKey, err := authService.ValidateAPIKey(token)
		if err == nil {
			c.Set("user_id", apiKey.UserID)
			c.Set("tenant_id", apiKey.TenantID)
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token or API key"})
		c.Abort()
	}
}
```

**NEW:**
```go
func AuthMiddleware(authService auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/auth/login") || strings.HasPrefix(path, "/health") {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		userID, err := authService.VerifyTokenUserOnly(token)
		if err == nil {
			c.Set("user_id", userID)
			c.Next()
			return
		}

		apiKey, err := authService.ValidateAPIKey(token)
		if err == nil {
			c.Set("user_id", apiKey.UserID)
			c.Set("tenant_id", apiKey.TenantID)
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token or API key"})
		c.Abort()
	}
}
```

**Verification:**
```bash
cd /home/hekemen/automata
go build ./internal/adapter/api/middleware/
```
### Task 2.5: Update tenant_middleware.go to prefer X-Tenant-ID header

**Files:**
- Modify: `internal/adapter/api/tenant_middleware.go`

**Interfaces:**
- Consumes: `X-Tenant-ID` header from request
- Produces: Sets `tenant_id` in Gin context from header

- [ ] **Step 1: Add X-Tenant-ID header resolution to TenantResolver**

In `internal/adapter/api/tenant_middleware.go`, modify the `TenantResolver` function to check `X-Tenant-ID` header first. Replace the entire `TenantResolver` function (lines 23-65):

**OLD:**
```go
func TenantResolver(tenantRepo tenant.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip tenant resolution for admin API routes
		if strings.HasPrefix(c.Request.URL.Path, "/admin") ||
			strings.HasPrefix(c.Request.URL.Path, "/auth") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/auth") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/admin") ||
			c.Request.URL.Path == "/health" ||
			strings.HasPrefix(c.Request.URL.Path, "/api/health") {
			c.Next()
			return
		}

		// Try subdomain resolution first
		t, err := resolveBySubdomain(c, tenantRepo)
		if err != nil && err != ErrNoTenant {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		if t != nil {
			c.Set(string(TenantContextKey), t)
			c.Next()
			return
		}

		// Fall back to path-based resolution
		t, err = resolveByPath(c, tenantRepo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		if t != nil {
			c.Set(string(TenantContextKey), t)
			c.Next()
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found"})
		c.Abort()
	}
}
```

**NEW:**
```go
func TenantResolver(tenantRepo tenant.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip tenant resolution for admin API routes
		if strings.HasPrefix(c.Request.URL.Path, "/admin") ||
			strings.HasPrefix(c.Request.URL.Path, "/auth") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/auth") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/admin") ||
			c.Request.URL.Path == "/health" ||
			strings.HasPrefix(c.Request.URL.Path, "/api/health") {
			c.Next()
			return
		}

		// Try X-Tenant-ID header first (preferred)
		if tenantID := c.GetHeader("X-Tenant-ID"); tenantID != "" {
			t, err := tenantRepo.GetByID(tenantID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found"})
				c.Abort()
				return
			}
			c.Set(string(TenantContextKey), t)
			c.Set("tenant_id", tenantID)
			c.Next()
			return
		}

		// Try subdomain resolution
		t, err := resolveBySubdomain(c, tenantRepo)
		if err != nil && err != ErrNoTenant {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		if t != nil {
			c.Set(string(TenantContextKey), t)
			c.Set("tenant_id", t.ID)
			c.Next()
			return
		}

		// Fall back to path-based resolution
		t, err = resolveByPath(c, tenantRepo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		if t != nil {
			c.Set(string(TenantContextKey), t)
			c.Set("tenant_id", t.ID)
			c.Next()
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found"})
		c.Abort()
	}
}
```

**Verification:**
```bash
cd /home/hekemen/automata
go build ./internal/adapter/api/
```

### Task 2.6: Update all handlers that use c.GetString("tenant_id")

**Files:**
- Modify: `internal/adapter/api/handler/form_handler.go`
- Modify: `internal/adapter/api/handler/banner_handler.go`

**Interfaces:**
- Consumes: `c.GetHeader("X-Tenant-ID")` instead of `c.GetString("tenant_id")`
- Produces: Handlers that read tenant from header

- [ ] **Step 1: Update form_handler.go**

In `internal/adapter/api/handler/form_handler.go`, replace all occurrences of `c.GetString("tenant_id")` with `c.GetHeader("X-Tenant-ID")`. There are 7 occurrences (lines 43, 59, 81, 110, 143, 172, 197).

For each method (`List`, `Get`, `Create`, `Update`, `Delete`, `SubmitForm`, `ListSubmissions`), change:

**OLD:**
```go
tenantID := c.GetString("tenant_id")
if tenantID == "" {
    c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant not found"})
    return
}
```

**NEW:**
```go
tenantID := c.GetHeader("X-Tenant-ID")
if tenantID == "" {
    c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant not found"})
    return
}
```

- [ ] **Step 2: Update banner_handler.go**

In `internal/adapter/api/handler/banner_handler.go`, replace all occurrences of `c.GetString("tenant")` with `c.GetHeader("X-Tenant-ID")`. There are 8 occurrences across BannerHandler, PlacementHandler, and CampaignHandler methods.

**OLD:**
```go
tenant := c.GetString("tenant")
if tenant == "" {
    c.JSON(http.StatusBadRequest, gin.H{"error": "tenant required"})
    return
}
```

**NEW:**
```go
tenantID := c.GetHeader("X-Tenant-ID")
if tenantID == "" {
    c.JSON(http.StatusBadRequest, gin.H{"error": "tenant required"})
    return
}
```

Rename the variable from `tenant` to `tenantID` for consistency.

**Verification:**
```bash
cd /home/hekemen/automata
go build ./internal/adapter/api/handler/
```
---

## Phase 3: Admin Config Subsystem

### Task 3.1: Create domain/config/config.go

**Files:**
- Create: `internal/domain/config/config.go`

**Interfaces:**
- Produces: `ConfigRepository` interface with CRUD methods for admin configs

- [ ] **Step 1: Create the domain config file**

Create `internal/domain/config/config.go`:

```go
package config

import "time"

// ConfigKey represents a configuration key for a tenant.
type ConfigKey string

const (
	ConfigKeyCORS    ConfigKey = "cors"
	ConfigKeyDomain  ConfigKey = "domain"
	ConfigKeyDisplay ConfigKey = "display"
)

// CORSConfig holds CORS settings for a tenant.
type CORSConfig struct {
	Origins []string `json:"origins"`
}

// DomainConfig holds domain settings for a tenant.
type DomainConfig struct {
	Primary string   `json:"primary"`
	Aliases []string `json:"aliases"`
}

// DisplayConfig holds display settings for a tenant.
type DisplayConfig struct {
	Name    string `json:"name"`
	LogoURL string `json:"logo_url"`
}

// AdminConfig represents a stored configuration entry.
type AdminConfig struct {
	ID        string
	TenantID  string
	Key       ConfigKey
	Value     map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ConfigRepository defines the interface for admin config persistence.
type ConfigRepository interface {
	GetByTenant(tenantID string) (map[ConfigKey]map[string]interface{}, error)
	GetByKey(tenantID string, key ConfigKey) (map[string]interface{}, error)
	Upsert(tenantID string, key ConfigKey, value map[string]interface{}) error
	Delete(tenantID string, key ConfigKey) error
}
```

**Verification:**
```bash
cd /home/hekemen/automata
go build ./internal/domain/config/
```

### Task 3.2: Create infrastructure/config/repo/postgres.go

**Files:**
- Create: `internal/infrastructure/config/repo/postgres.go`

**Interfaces:**
- Consumes: `domain/config.ConfigRepository` interface
- Consumes: `pgxpool.Pool`
- Produces: PostgreSQL implementation of ConfigRepository

- [ ] **Step 1: Create the PostgreSQL config repository**

Create `internal/infrastructure/config/repo/postgres.go`:

```go
package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type configRepo struct {
	pool *pgxpool.Pool
}

// NewConfigRepo creates a new PostgreSQL admin config repository.
func NewConfigRepo(pool *pgxpool.Pool) config.ConfigRepository {
	return &configRepo{pool: pool}
}

func (r *configRepo) GetByTenant(tenantID string) (map[config.ConfigKey]map[string]interface{}, error) {
	ctx := context.Background()
	query := `SELECT key, value FROM admin_configs WHERE tenant_id = $1 ORDER BY key`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get configs by tenant: %w", err)
	}
	defer rows.Close()

	result := make(map[config.ConfigKey]map[string]interface{})
	for rows.Next() {
		var key string
		var valueJSON []byte
		if err := rows.Scan(&key, &valueJSON); err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		var value map[string]interface{}
		if err := json.Unmarshal(valueJSON, &value); err != nil {
			return nil, fmt.Errorf("unmarshal config value: %w", err)
		}
		result[config.ConfigKey(key)] = value
	}
	return result, nil
}

func (r *configRepo) GetByKey(tenantID string, key config.ConfigKey) (map[string]interface{}, error) {
	ctx := context.Background()
	query := `SELECT value FROM admin_configs WHERE tenant_id = $1 AND key = $2`

	var valueJSON []byte
	err := r.pool.QueryRow(ctx, query, tenantID, string(key)).Scan(&valueJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get config by key: %w", err)
	}

	var value map[string]interface{}
	if err := json.Unmarshal(valueJSON, &value); err != nil {
		return nil, fmt.Errorf("unmarshal config value: %w", err)
	}
	return value, nil
}

func (r *configRepo) Upsert(tenantID string, key config.ConfigKey, value map[string]interface{}) error {
	ctx := context.Background()
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal config value: %w", err)
	}

	query := `
		INSERT INTO admin_configs (id, tenant_id, key, value, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (tenant_id, key) DO UPDATE
			SET value = $4, updated_at = NOW()
		RETURNING id, tenant_id, key, value, created_at, updated_at
	`

	var id, storedKey string
	var storedValueJSON, createdAt, updatedAt []byte
	err = r.pool.QueryRow(ctx, query,
		uuid.New().String(), tenantID, string(key), valueJSON,
	).Scan(&id, &storedKey, &storedKey, &storedValueJSON, &createdAt, &updatedAt)
	if err != nil {
		return fmt.Errorf("upsert config: %w", err)
	}
	return nil
}

func (r *configRepo) Delete(tenantID string, key config.ConfigKey) error {
	ctx := context.Background()
	query := `DELETE FROM admin_configs WHERE tenant_id = $1 AND key = $2`

	result, err := r.pool.Exec(ctx, query, tenantID, string(key))
	if err != nil {
		return fmt.Errorf("delete config: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("config not found")
	}
	return nil
}
```

**Verification:**
```bash
cd /home/hekemen/automata
go build ./internal/infrastructure/config/repo/
```
### Task 3.3: Create adapter/api/handler/config_handler.go

**Files:**
- Create: `internal/adapter/api/handler/config_handler.go`

**Interfaces:**
- Consumes: `domain/config.ConfigRepository`
- Produces: HTTP handlers for config CRUD endpoints

- [ ] **Step 1: Create the config handler**

Create `internal/adapter/api/handler/config_handler.go`:

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/config"
)

type ConfigHandler struct {
	repo config.ConfigRepository
}

func NewConfigHandler(repo config.ConfigRepository) *ConfigHandler {
	return &ConfigHandler{repo: repo}
}

// GetConfig handles GET /api/admin/configs/:tenantId — returns all config for a tenant.
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenantId required"})
		return
	}

	configs, err := h.repo.GetByTenant(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"configs": configs})
}

// UpdateCORS handles PUT /api/admin/configs/:tenantId/cors — updates CORS origins.
func (h *ConfigHandler) UpdateCORS(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenantId required"})
		return
	}

	var req struct {
		Origins []string `json:"origins"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Upsert(tenantID, config.ConfigKeyCORS, map[string]interface{}{
		"origins": req.Origins,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "CORS config updated"})
}

// UpdateDomain handles PUT /api/admin/configs/:tenantId/domain — updates domain settings.
func (h *ConfigHandler) UpdateDomain(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenantId required"})
		return
	}

	var req struct {
		Primary string   `json:"primary"`
		Aliases []string `json:"aliases"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Upsert(tenantID, config.ConfigKeyDomain, map[string]interface{}{
		"primary": req.Primary,
		"aliases": req.Aliases,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Domain config updated"})
}

// UpdateDisplay handles PUT /api/admin/configs/:tenantId/display — updates display settings.
func (h *ConfigHandler) UpdateDisplay(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenantId required"})
		return
	}

	var req struct {
		Name    string `json:"name"`
		LogoURL string `json:"logo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Upsert(tenantID, config.ConfigKeyDisplay, map[string]interface{}{
		"name":    req.Name,
		"logo_url": req.LogoURL,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Display config updated"})
}
```

**Verification:**
```bash
cd /home/hekemen/automata
go build ./internal/adapter/api/handler/
```

### Task 3.4: Register config routes in server.go

**Files:**
- Modify: `internal/adapter/api/server.go`

**Interfaces:**
- Consumes: `config.ConfigRepository`
- Produces: Config routes under `/api/admin/configs/:tenantId/*`

- [ ] **Step 1: Add config handler initialization and route registration**

In `internal/adapter/api/server.go`, add the config handler initialization in `NewServer` (after the existing handler initializations):

**Add after existing handler init:**
```go
configRepo := repo.NewConfigRepo(server.dbPool)
configHandler := handler.NewConfigHandler(configRepo)
```

**Add config routes in the admin group registration:**
```go
// Config routes
admin.GET("/configs/:tenantId", configHandler.GetConfig)
admin.PUT("/configs/:tenantId/cors", configHandler.UpdateCORS)
admin.PUT("/configs/:tenantId/domain", configHandler.UpdateDomain)
admin.PUT("/configs/:tenantId/display", configHandler.UpdateDisplay)
```

**Verification:**
```bash
cd /home/hekemen/automata
go build ./internal/adapter/api/
```
---

## Phase 4: Dual-Server Architecture

### Task 4.1: Create tracking server setup

**Files:**
- Create: `internal/adapter/tracking/server.go`
- Modify: `cmd/automata/main.go`

**Interfaces:**
- Consumes: existing form, banner, tracking handlers
- Produces: Separate Gin server on port 8081

- [ ] **Step 1: Create tracking server.go**

Create `internal/adapter/tracking/server.go`:

```go
package tracking

import (
	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/adapter/api/handler"
	"github.com/hekemen/automata/internal/infrastructure/database"
)

// NewServer creates a new tracking server with public-facing endpoints.
func NewServer(dbPool *database.Pool, formHandler *handler.FormHandler, bannerHandler *handler.BannerHandler, trackingHandler *handler.TrackingHandler) *gin.Engine {
	r := gin.Default()

	// Tracking endpoints
	r.POST("/api/v1/track", trackingHandler.TrackEvent)
	r.GET("/api/v1/tracking.js", trackingHandler.ServeSnippet)

	// Banner endpoints
	r.GET("/api/v1/banners", bannerHandler.ListBanners)
	r.GET("/api/v1/banners/:id", bannerHandler.GetBanner)
	r.GET("/api/v1/placements/:id", bannerHandler.GetPlacement)
	r.GET("/api/v1/campaigns/:id", bannerHandler.GetCampaign)

	// Form endpoints
	r.GET("/api/v1/forms", formHandler.List)
	r.GET("/api/v1/forms/:id", formHandler.Get)
	r.POST("/api/v1/forms/:id/submit", formHandler.SubmitForm)
	r.GET("/api/v1/forms/:id/submissions", formHandler.ListSubmissions)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
```

- [ ] **Step 2: Update main.go to start dual servers**

In `cmd/automata/main.go`, replace the single server startup with dual server logic:

**OLD:**
```go
func main() {
	// ... config loading ...
	
	server := adapter.NewServer(dbPool, authService, tenantRepo, userRepo)
	
	addr := ":" + config.Get("server.port")
	log.Info().Str("addr", addr).Msg("starting server")
	if err := server.Run(addr); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}
```

**NEW:**
```go
func main() {
	// ... config loading ...

	// Initialize shared dependencies
	dbPool := database.NewPool(dbURL)
	authService := auth.NewService(userRepo, apiKeyRepo)
	tenantRepo := tenant.NewRepo(dbPool)
	userRepo := tenant.NewUserRepo(dbPool)

	// Create handlers
	authHandler := handler.NewAuthHandler(authService, tenantRepo, userRepo)
	formHandler := handler.NewFormHandler(formRepo, formSubmissionRepo, tenantRepo)
	bannerHandler := handler.NewBannerHandler(bannerRepo, placementRepo, campaignRepo)
	trackingHandler := handler.NewTrackingHandler(trackingRepo, snippetGenerator)
	configRepo := repo.NewConfigRepo(dbPool)
	configHandler := handler.NewConfigHandler(configRepo)

	// Admin server (port 8080)
	adminServer := adapter.NewServer(dbPool, authService, tenantRepo, userRepo, formHandler, bannerHandler, trackingHandler, configHandler)
	adminPort := config.Get("server.port")
	if adminPort == "" {
		adminPort = "8080"
	}
	go func() {
		log.Info().Str("addr", ":"+adminPort).Msg("starting admin server")
		if err := adminServer.Run(":" + adminPort); err != nil {
			log.Fatal().Err(err).Msg("admin server failed")
		}
	}()

	// Tracking server (port 8081)
	trackingServer := tracking.NewServer(dbPool, formHandler, bannerHandler, trackingHandler)
	trackingPort := config.Get("server.tracking_port")
	if trackingPort == "" {
		trackingPort = "8081"
	}
	log.Info().Str("addr", ":"+trackingPort).Msg("starting tracking server")
	if err := trackingServer.Run(":" + trackingPort); err != nil {
		log.Fatal().Err(err).Msg("tracking server failed")
	}
}
```

**Verification:**
```bash
cd /home/hekemen/automata
go build ./cmd/automata/
```
---

## Phase 5: Frontend Updates

### Task 5.1: Update LoginView.vue — remove tenant field

**Files:**
- Modify: `.worktrees/web-ui-phase1/webui/src/views/LoginView.vue`

**Interfaces:**
- Consumes: new login API (no `tenant` field in request)
- Produces: Login form without tenant selector

- [ ] **Step 1: Remove tenant field from login form**

In `.worktrees/web-ui-phase1/webui/src/views/LoginView.vue`:

**OLD (template):**
```vue
<template>
  <div class="login-container">
    <form @submit.prevent="handleLogin">
      <input v-model="email" placeholder="Email" required />
      <input v-model="password" type="password" placeholder="Password" required />
      <input v-model="tenant" placeholder="Tenant" required />
      <button type="submit">Login</button>
    </form>
  </div>
</template>
```

**NEW (template):**
```vue
<template>
  <div class="login-container">
    <form @submit.prevent="handleLogin">
      <input v-model="email" placeholder="Email" required />
      <input v-model="password" type="password" placeholder="Password" required />
      <button type="submit">Login</button>
    </form>
    <div v-if="tenants.length > 1" class="tenant-selector">
      <p>Select a tenant:</p>
      <select v-model="selectedTenantId">
        <option v-for="t in tenants" :key="t.id" :value="t.id">
          {{ t.name || t.slug }}
        </option>
      </select>
    </div>
  </div>
</template>
```

**OLD (script):**
```typescript
const email = ref('')
const password = ref('')
const tenant = ref('')
const { login } = useAuthStore()

const handleLogin = async () => {
  await login(email.value, password.value, tenant.value)
}
```

**NEW (script):**
```typescript
const email = ref('')
const password = ref('')
const selectedTenantId = ref('')
const tenants = ref<any[]>([])
const { login } = useAuthStore()

const handleLogin = async () => {
  const response: any = await api.post('/auth/login', {
    email: email.value,
    password: password.value,
  })
  
  tenants.value = response.data.tenants || []
  
  if (tenants.value.length === 1) {
    selectedTenantId.value = tenants.value[0].id
    await login(response.data.token, response.data.user_id, selectedTenantId.value)
  } else if (tenants.value.length > 1) {
    // Show tenant selector, wait for user selection
    return
  } else {
    throw new Error('No tenants found')
  }
}

// Watch for tenant selection
watch(selectedTenantId, async (newId) => {
  if (newId) {
    await login(tenants.value[0].token || response?.data?.token, response?.data?.user_id, newId)
  }
})
```

### Task 5.2: Update auth store — support tenant list

**Files:**
- Modify: `.worktrees/web-ui-phase1/webui/src/stores/auth.ts`

**Interfaces:**
- Consumes: login response with `tenants` array
- Produces: Store that manages current tenant selection

- [ ] **Step 1: Update auth store to handle tenant switching**

In `.worktrees/web-ui-phase1/webui/src/stores/auth.ts`:

**Add tenant state:**
```typescript
const tenants = ref<TenantInfo[]>([])
const currentTenantId = ref('')

export interface TenantInfo {
  id: string
  slug: string
  name: string
}
```

**Update login action:**
```typescript
const login = async (token: string, userId: string, tenantId: string) => {
  currentTenantId.value = tenantId
  token.value = token
  userId.value = userId
  localStorage.setItem('token', token)
  localStorage.setItem('tenant_id', tenantId)
}
```

**Add switchTenant action:**
```typescript
const switchTenant = async (tenantId: string) => {
  currentTenantId.value = tenantId
  localStorage.setItem('tenant_id', tenantId)
  // Optionally re-validate or refresh data
}
```

**Add getters:**
```typescript
const availableTenants = computed(() => tenants.value)
const hasMultipleTenants = computed(() => tenants.value.length > 1)
```

### Task 5.3: Update API client — add X-Tenant-ID header

**Files:**
- Modify: `.worktrees/web-ui-phase1/webui/src/api/client.ts`

**Interfaces:**
- Consumes: `currentTenantId` from auth store
- Produces: API client that includes `X-Tenant-ID` header on every request

- [ ] **Step 1: Add X-Tenant-ID header to API client**

In `.worktrees/web-ui-phase1/webui/src/api/client.ts`:

**OLD:**
```typescript
const client = createClient({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8080',
})
```

**NEW:**
```typescript
import { useAuthStore } from '@/stores/auth'

const client = createClient({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8080',
  headers: {
    'X-Tenant-ID': () => useAuthStore().currentTenantId || '',
  },
})

// Interceptor to ensure tenant header is set
client.interceptors.request.use((request) => {
  const authStore = useAuthStore()
  if (authStore.currentTenantId && !request.headers['X-Tenant-ID']) {
    request.headers['X-Tenant-ID'] = authStore.currentTenantId
  }
  return request
})
```

### Task 5.4: Add tenant switcher to Sidebar

**Files:**
- Modify: `.worktrees/web-ui-phase1/webui/src/components/layout/Sidebar.vue`

**Interfaces:**
- Consumes: `availableTenants`, `currentTenantId`, `switchTenant` from auth store
- Produces: Dropdown/selector in sidebar for switching tenants

- [ ] **Step 1: Add tenant switcher component to sidebar**

In `.worktrees/web-ui-phase1/webui/src/components/layout/Sidebar.vue`:

**Add tenant switcher template:**
```vue
<div v-if="hasMultipleTenants" class="tenant-switcher">
  <select :value="currentTenantId" @change="switchTenant($event.target.value)">
    <option v-for="t in availableTenants" :key="t.id" :value="t.id">
      {{ t.name || t.slug }}
    </option>
  </select>
</div>
```

**Add script imports:**
```typescript
import { useAuthStore } from '@/stores/auth'
const authStore = useAuthStore()
const hasMultipleTenants = computed(() => authStore.hasMultipleTenants)
const availableTenants = computed(() => authStore.availableTenants)
const currentTenantId = computed(() => authStore.currentTenantId)
const switchTenant = (tenantId: string) => authStore.switchTenant(tenantId)
```

### Task 5.5: Add new API modules for config and tenant endpoints

**Files:**
- Modify: `.worktrees/web-ui-phase1/webui/src/api/index.ts`
- Modify: `.worktrees/web-ui-phase1/webui/src/types/api.ts`

**Interfaces:**
- Produces: New API module functions for config CRUD
- Produces: New TypeScript interfaces for config types

- [ ] **Step 1: Add new types to api.ts**

In `.worktrees/web-ui-phase1/webui/src/types/api.ts`:

```typescript
export interface TenantInfo {
  id: string
  slug: string
  name: string
}

export interface LoginResponse {
  token: string
  user_id: string
  email: string
  tenants: TenantInfo[]
}

export interface ConfigValue {
  cors?: { origins: string[] }
  domain?: { primary: string; aliases: string[] }
  display?: { name: string; logo_url: string }
}

export interface ConfigResponse {
  configs: Record<string, Record<string, any>>
}
```

- [ ] **Step 2: Add new API functions to index.ts**

In `.worktrees/web-ui-phase1/webui/src/api/index.ts`:

```typescript
import type { ConfigValue, ConfigResponse } from '@/types/api'

export const configApi = {
  get: (tenantId: string) => 
    api.get(`/api/admin/configs/${tenantId}`).then(r => r.data as ConfigResponse),
  updateCors: (tenantId: string, origins: string[]) =>
    api.put(`/api/admin/configs/${tenantId}/cors`, { origins }),
  updateDomain: (tenantId: string, primary: string, aliases: string[]) =>
    api.put(`/api/admin/configs/${tenantId}/domain`, { primary, aliases }),
  updateDisplay: (tenantId: string, name: string, logoUrl: string) =>
    api.put(`/api/admin/configs/${tenantId}/display`, { name, logo_url: logoUrl }),
}
```

**Verification:**
```bash
cd /home/hekemen/automata/.worktrees/web-ui-phase1/webui
yarn lint
```
---

## Phase 6: Tracking Script Update

### Task 6.1: Update snippet generator to use tracking server URL

**Files:**
- Modify: `pkg/snippet/generator.go`

**Interfaces:**
- Consumes: tracking server URL from config
- Produces: Snippet with correct tracking endpoint URL

- [ ] **Step 1: Update snippet generator**

In `pkg/snippet/generator.go`, update the `Generate` function:

**OLD:**
```go
func Generate(siteID string) string {
    return `<script>
    (function() {
        var script = document.createElement('script');
        script.src = '/api/v1/tracking.js';
        document.head.appendChild(script);
    })();
    </script>`
}
```

**NEW:**
```go
func Generate(siteID string, trackingURL string) string {
    if trackingURL == "" {
        trackingURL = "http://localhost:8081"
    }
    return `<script>
    (function() {
        var script = document.createElement('script');
        script.src = "` + trackingURL + `/api/v1/tracking.js";
        document.head.appendChild(script);
    })();
    </script>`
}
```

**Verification:**
```bash
cd /home/hekemen/automata
go build ./pkg/snippet/
```

---

## Phase 7: Configuration & Deployment

### Task 7.1: Update config.example.yaml

**Files:**
- Modify: `config.example.yaml`

**Interfaces:**
- Produces: New config keys for tracking port

- [ ] **Step 1: Add tracking port to config example**

In `config.example.yaml`, add:

```yaml
server:
  port: "8080"
  tracking_port: "8081"
```

### Task 7.2: Update config loading logic

**Files:**
- Modify: `internal/infrastructure/config/config.go`

**Interfaces:**
- Consumes: new `server.tracking_port` config key
- Produces: Config loader that reads tracking port

- [ ] **Step 1: Ensure tracking port is loaded**

In `internal/infrastructure/config/config.go`, the existing `Load` function should already handle `server.tracking_port` via the dot-notation flattening. No code changes needed if the flattening logic is generic.

**Verification:**
```bash
cd /home/hekemen/automata
go build ./internal/infrastructure/config/
```

### Task 7.3: Update Dockerfile

**Files:**
- Modify: `Dockerfile`

**Interfaces:**
- Produces: Dockerfile that exposes both ports

- [ ] **Step 1: Add EXPOSE for tracking port**

In `Dockerfile`, add:

```dockerfile
EXPOSE 8080
EXPOSE 8081
```

### Task 7.4: Update docker-compose.yml

**Files:**
- Modify: `docker-compose.yml`

**Interfaces:**
- Produces: Service that exposes both ports

- [ ] **Step 1: Add port mappings for both servers**

In `docker-compose.yml`, update the automata service:

```yaml
services:
  automata:
    build: ..
    ports:
      - "8080:8080"
      - "8081:8081"
    environment:
      - AUTOMATA_SERVER_PORT=8080
      - AUTOMATA_TRACKING_PORT=8081
```

---

## Phase 8: E2E Tests

### Task 8.1: Update E2E tests for new login flow

**Files:**
- Modify: `tests/e2e/app.spec.ts`

**Interfaces:**
- Consumes: new login API (no tenant in request)
- Produces: Tests that verify global login and tenant switching

- [ ] **Step 1: Update login test**

In `tests/e2e/app.spec.ts`, update the login test:

**OLD:**
```typescript
test('login', async ({ page }) => {
  await page.goto('/login')
  await page.fill('input[name="email"]', 'admin@example.com')
  await page.fill('input[name="password"]', 'password')
  await page.fill('input[name="tenant"]', 'default')
  await page.click('button[type="submit"]')
  await page.waitForURL('/dashboard')
})
```

**NEW:**
```typescript
test('login with global auth', async ({ page }) => {
  await page.goto('/login')
  await page.fill('input[name="email"]', 'admin@example.com')
  await page.fill('input[name="password"]', 'password')
  await page.click('button[type="submit"]')
  
  // If multiple tenants, select one
  const tenantSelector = page.locator('.tenant-selector')
  if (await tenantSelector.isVisible()) {
    await tenantSelector.locator('select').selectOption({ label: 'default' })
  }
  
  await page.waitForURL('/dashboard')
})

test('switch tenant', async ({ page }) => {
  await page.goto('/dashboard')
  const tenantSwitcher = page.locator('.tenant-switcher select')
  await tenantSwitcher.selectOption({ label: 'tenant2' })
  await page.waitForLoadState('networkidle')
  // Verify tenant-specific data loads
})
```

**Verification:**
```bash
cd /home/hekemen/automata
yarn test:e2e
```
---

## Phase 9: Integration & Verification

### Task 9.1: Build and test all Go packages

- [ ] **Step 1: Build all packages**
```bash
cd /home/hekemen/automata
go build ./...
```

- [ ] **Step 2: Run unit tests**
```bash
cd /home/hekemen/automata
go test ./internal/...
```

### Task 9.2: Verify database migration

- [ ] **Step 1: Test migration SQL**
```bash
cd /home/hekemen/automata
# Connect to a test database and run migration.sql
psql -U test -d test -f internal/infrastructure/database/migration.sql
```

### Task 9.3: Verify dual-server startup

- [ ] **Step 1: Start servers and verify**
```bash
cd /home/hekemen/automata
go run ./cmd/automata/ &

# Verify admin server
curl http://localhost:8080/health

# Verify tracking server
curl http://localhost:8081/health
```

### Task 9.4: Verify API endpoints

- [ ] **Step 1: Test login endpoint**
```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password"}'
# Expected: {"token":"...","user_id":"...","email":"...","tenants":[{"id":"...","slug":"...","name":"..."}]}
```

- [ ] **Step 2: Test config endpoints**
```bash
curl -X GET http://localhost:8080/api/admin/configs/:tenantId \
  -H "Authorization: Bearer <token>"
```

- [ ] **Step 3: Test tracking endpoint**
```bash
curl -X POST http://localhost:8081/api/v1/track \
  -H "Content-Type: application/json" \
  -d '{"site_id":"test","event":"pageview"}'
```

---

## Summary of Files Changed

| File | Action | Phase |
|------|--------|-------|
| `internal/infrastructure/database/migration.sql` | Modify | 1 |
| `internal/domain/tenant/user_repository.go` | Modify | 2 |
| `internal/infrastructure/tenant/repo/user_postgres.go` | Modify | 2 |
| `internal/domain/auth/auth.go` | Modify | 2 |
| `internal/infrastructure/auth/service.go` | Modify | 2 |
| `internal/adapter/api/handler/auth_handler.go` | Modify | 2 |
| `internal/adapter/api/middleware/auth.go` | Modify | 2 |
| `internal/adapter/api/tenant_middleware.go` | Modify | 2 |
| `internal/adapter/api/handler/form_handler.go` | Modify | 2 |
| `internal/adapter/api/handler/banner_handler.go` | Modify | 2 |
| `internal/domain/config/config.go` | Create | 3 |
| `internal/infrastructure/config/repo/postgres.go` | Create | 3 |
| `internal/adapter/api/handler/config_handler.go` | Create | 3 |
| `internal/adapter/api/server.go` | Modify | 3 |
| `internal/adapter/tracking/server.go` | Create | 4 |
| `cmd/automata/main.go` | Modify | 4 |
| `.worktrees/web-ui-phase1/webui/src/views/LoginView.vue` | Modify | 5 |
| `.worktrees/web-ui-phase1/webui/src/stores/auth.ts` | Modify | 5 |
| `.worktrees/web-ui-phase1/webui/src/api/client.ts` | Modify | 5 |
| `.worktrees/web-ui-phase1/webui/src/components/layout/Sidebar.vue` | Modify | 5 |
| `.worktrees/web-ui-phase1/webui/src/api/index.ts` | Modify | 5 |
| `.worktrees/web-ui-phase1/webui/src/types/api.ts` | Modify | 5 |
| `pkg/snippet/generator.go` | Modify | 6 |
| `config.example.yaml` | Modify | 7 |
| `internal/infrastructure/config/config.go` | Verify | 7 |
| `Dockerfile` | Modify | 7 |
| `docker-compose.yml` | Modify | 7 |
| `tests/e2e/app.spec.ts` | Modify | 8 |

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Breaking existing API clients | Version tracking endpoints under `/api/v1/` |
| Tenant resolution conflicts | X-Tenant-ID header takes precedence, subdomain/path are fallbacks |
| Migration failures | Use `IF NOT EXISTS` in SQL, test on staging first |
| Dual-server port conflicts | Configurable ports with sensible defaults |
| Frontend tenant switching race conditions | Use Pinia store for single source of truth |
