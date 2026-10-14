# Context Rename and Login Simplification — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement platform-level users with context memberships, admin bootstrap on first startup, user CRUD endpoints, and `/auth/me` — completing the remaining items from the context-rename / login-simplification spec.

**Architecture:** Add `users` (platform-level) and `user_contexts` (role-based join) tables alongside existing `context_users`. Update domain interfaces and repos to support both tables during transition. Add admin bootstrap in `main.go`. Wire new user/member CRUD routes into the API server.

**Tech Stack:** Go 1.26, PostgreSQL 16+, Gin, Ginkgo + testcontainers, JWT (golang-jwt/v5), bcrypt

**Spec:** `docs/superpowers/specs/2026-09-22-context-rename-and-login-simplification.md` (design) + `docs/superpowers/specs/2026-09-24-implementation-spec.md` (implementation spec)

## Execution Status

**Status: Completed** — executed 2026-10-13 on branch `phase2-auth-changes`. The plan was written post-hoc to document work that had already been completed; execution consisted of verifying every step against the codebase, closing the one real gap (Task 10 Step 1), and verifying end-to-end on the k3s cluster.

- **Code gap closed:** `cicd/global_login_test.go` still seeded `context_users`; it now seeds `users` + `user_contexts`. All 6 "Global Login" specs pass against a real Postgres testcontainer.
- **Verification:** `go build ./...`, `go vet ./cicd/...`, `make build` pass. Full cicd suite: 279 passed / 23 failed — the 23 failures are pre-existing and unrelated to Phase 2 (tracking/form/banner/placement/campaign specs); the identical failure set was confirmed by re-running the suite with the change stashed.
- **Live verification (k3s):** release `automata` (rev 20, deployed 2026-10-05, image `ghcr.io/hekemen/automata:latest`) runs the new code. Login verified end-to-end via port-forward with a temporary user: 200 OK with JWT (`is_admin`/`user_email`/`user_id` claims), AES-encrypted `refresh_token`, and `contexts` populated from `user_contexts`. Temporary user deleted afterward; DB restored to original state.

### Deviations from the plan

| Plan step | Actual state |
|---|---|
| Task 3: add `IsAdmin` to `ContextInfo` | `ContextInfo` carries `Role` (`owner`/`admin`/`member`) instead; `is_admin` is a JWT claim extracted by the auth middleware, not a `ContextInfo` field. |
| Task 6: `bootstrap.Bootstrap(pool)` | Actual function is `bootstrap.Run(pool, userRepo, contextRepo)` in `internal/infrastructure/bootstrap/admin.go`. Guard is DB-based (`ExistsAnyUser` + `context_users` count); the `.admin_initialized` marker is written but not the guard. Admin email/password come from `AUTOMATA_ADMIN_EMAIL` / `AUTOMATA_ADMIN_PASSWORD` env vars (email defaults to `admin@automata.local`). |
| Task 9 Step 2: `GetByEmailGlobal` queries both tables | Queries `context_users` only. Functionally equivalent: `Login` calls `GetByUsername` (`users` table) first and falls back to `GetByEmailGlobal`, so both tables are covered in the login path. |
| Task 10 Step 3: fix remaining test failures | 23 pre-existing failures remain (unrelated to Phase 2; see above). No failures were caused by this plan's changes. |
| Task 11 Steps 3–4: docker rebuild + login | Verified on the k3s cluster instead (see live verification above). **Finding:** the docker-compose smoke test fails with `crypto/aes: invalid key size 25` because `config.example.yaml`'s `cookie_secret` (`change-this-in-production`) is 25 chars — invalid for AES (needs 16/24/32). The k3s chart uses a 32-char `cookieSecret`, so the cluster is unaffected. `config.example.yaml` was left unchanged per user decision. |
| Commit steps | The work is in branch history (`dc7fac6`, `1f67080`, `22fe826`, `cdba540`, …); per-task commits were not replayed separately. The remaining delta (test updates, plan check-offs) is left uncommitted pending review. |

## Global Constraints

- Go version: 1.26.2
- PostgreSQL: 16+ (for `gen_random_uuid()`)
- JWT library: `github.com/golang-jwt/jwt/v5`
- Password hashing: `golang.org/x/crypto/bcrypt` with `bcrypt.DefaultCost` (10)
- All new table names: `users`, `user_contexts`
- All role values: `owner`, `admin`, `member` (lowercase)
- Migration file: `internal/infrastructure/database/migration_user_contexts.sql`
- Admin bootstrap password: 16 characters, alphanumeric + special (`!@#$%^&*`)
- Admin bootstrap marker file: `.admin_initialized` in working directory
- Existing `context_users` table must remain for backward compatibility during migration
- `GetByEmailGlobal` must query BOTH `users` and `context_users` during transition

## Review Focus

1. **Login with nonexistent email** — Must return 401 with `"invalid credentials"`, not 500; the auth service checks the `users` table first, falls back to `context_users`
2. **Duplicate email during user creation** — Must return 409 Conflict (unique constraint on `users.email`); handle the duplicate-key error gracefully by checking for existence
3. **Bootstrap with existing `context_users` data** — Migration must create `users` rows for every distinct email from `context_users` before creating `user_contexts` memberships
4. **JWT claim backward compat** — `VerifyTokenUserOnly` must still work with old tokens that lack `is_admin` (check exists before accessing)
5. **Admin bootstrap on every startup** — The `.admin_initialized` marker prevents re-creation; if marker exists but no admin in DB, regenerate and log a warning

---

### Task 1: Database Schema — `users` + `user_contexts` tables + migration

**Files:**
- Create: `internal/infrastructure/database/migration_user_contexts.sql`

**Interfaces:**
- Consumes: PostgreSQL pool with `contexts` and `context_users` tables
- Produces: New tables `users` and `user_contexts`, with data migration from `context_users`

- [x] **Step 1: Write migration SQL**

Create `internal/infrastructure/database/migration_user_contexts.sql`:

```sql
-- Migration: 202609240000 — Add users and user_contexts tables

-- 1. Create platform-level users table
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(254) NOT NULL UNIQUE,
    password_hash   TEXT,
    sso_provider    VARCHAR(64),
    sso_id          VARCHAR(512),
    is_admin        BOOLEAN DEFAULT false,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- 2. Create user-context memberships table
CREATE TABLE IF NOT EXISTS user_contexts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    context_id      UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    role            VARCHAR(32) NOT NULL DEFAULT 'member',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, context_id)
);

-- 3. Migrate data from context_users → users + user_contexts
DO $$
BEGIN
    -- Copy distinct users (by email) to users table
    INSERT INTO users (email, password_hash, is_admin, created_at, updated_at)
    SELECT email, password_hash, is_owner, created_at, updated_at
    FROM context_users
    ON CONFLICT (email) DO NOTHING;

    -- Copy context memberships to user_contexts table
    INSERT INTO user_contexts (user_id, context_id, role, created_at, updated_at)
    SELECT cu.id, cu.context_id,
           CASE WHEN cu.is_owner THEN 'owner' ELSE 'admin' END,
           NOW(), NOW()
    FROM context_users cu
    INNER JOIN users u ON u.email = cu.email;
END $$;

-- 4. Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_user_contexts_user ON user_contexts(user_id);
CREATE INDEX IF NOT EXISTS idx_user_contexts_context ON user_contexts(context_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
```

- [x] **Step 2: Verify the existing migration runner picks up new .sql files**

Read `internal/infrastructure/database/` to find how `RunMigrations` discovers SQL files. The new file uses the same naming convention (`migration_*.sql`) so it should be picked up automatically. If not, add a call after the existing `RunMigrations`.

- [x] **Step 3: Build to verify**

Run: `go build ./internal/infrastructure/database/...`

Expected: Build succeeds.

- [x] **Step 4: Commit**

```bash
git add internal/infrastructure/database/migration_user_contexts.sql
git commit -m "feat: add users and user_contexts tables with migration"
```

---

### Task 2: Domain Layer — `User` struct, `UserContext` struct, interface updates

**Files:**
- Modify: `internal/domain/context/user.go`
- Modify: `internal/domain/context/user_repository.go`

**Interfaces:**
- Consumes: Existing `User` struct, `UserRepository` interface
- Produces: Updated `User` with `IsAdmin`, `SSOProvider`, `SSOID`; new `UserContext` struct; extended `UserRepository`

- [x] **Step 1: Update User struct and add UserContext struct**

Replace the current `User` struct in `internal/domain/context/user.go` with:

```go
// User represents a platform-level user account.
// Context membership is tracked via UserContext records.
type User struct {
    ID           string
    ContextID    string  // deprecated; kept for backward compat with context_users
    Email        string
    PasswordHash string  // empty for SSO users
    SSOProvider  *string // nil for local auth
    SSOID        *string // nil for local auth
    IsAdmin      bool
    IsOwner      bool   // deprecated; kept for backward compat
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

Add this struct after the User definition:

```go
// UserContext represents a user's membership in a context.
type UserContext struct {
    ID        string
    UserID    string
    ContextID string
    Role      string // "owner", "admin", or "member"
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

- [x] **Step 2: Update Validate method**

Replace the Validate method to allow SSO-only users:

```go
func (u *User) Validate() error {
    if u.Email == "" {
        return errors.New("email is required")
    }
    if !emailRegex.MatchString(u.Email) {
        return fmt.Errorf("invalid email format: %s", u.Email)
    }
    // Password required only for local auth users (not SSO)
    if u.PasswordHash == "" && (u.SSOProvider == nil || *u.SSOProvider == "") {
        return errors.New("password hash is required for local auth users")
    }
    if utf8.RuneCountInString(u.Email) > 254 {
        return errors.New("email must be 254 characters or less")
    }
    return nil
}
```

- [x] **Step 3: Update UserRepository interface**

Replace `internal/domain/context/user_repository.go` with:

```go
package context

import "context"

// UserRepository defines the interface for user data persistence.
type UserRepository interface {
    Create(u *User) error
    GetByID(id string) (*User, error)
    GetByEmail(contextID, email string) (*User, error)
    GetByEmailGlobal(email string) ([]*User, error)
    GetByUsername(email string) (*User, error)         // platform-wide, users table
    ExistsAnyUser() (bool, error)                       // for bootstrap check
    ListAll() ([]*User, error)                          // list all platform users
    Update(u *User) error
    Delete(id string) error

    // Context membership operations
    GetByUserContext(userID, contextID string) (*UserContext, error)
    ListByUser(userID string) ([]UserContext, error)
    CreateMembership(uc *UserContext) error
    UpdateMembership(uc *UserContext) error
    DeleteMembership(userID, contextID string) error
    ListMembers(contextID string) ([]UserContext, error)
}
```

- [x] **Step 4: Build to verify**

Run: `go build ./internal/domain/...`

Expected: Build succeeds, no errors.

- [x] **Step 5: Commit**

```bash
git add internal/domain/context/user.go internal/domain/context/user_repository.go
git commit -m "domain: add UserContext struct and platform user interface methods"
```

---

### Task 3: Domain Auth — add `IsAdmin` to `ContextInfo`, add `GetCurrentUser` to interface

**Files:**
- Modify: `internal/domain/auth/auth.go`

**Interfaces:**
- Consumes: Existing `AuthService` interface, `ContextInfo` struct
- Produces: `ContextInfo` with `Role` field; `AuthService` with `GetCurrentUser(contextID string) (*context.User, error)`

- [x] **Step 1: Update ContextInfo struct**

Replace the `ContextInfo` struct in `internal/domain/auth/auth.go` (lines 17-21) with:

```go
type ContextInfo struct {
    ID   string `json:"id"`
    Slug string `json:"slug"`
    Name string `json:"name"`
    Role string `json:"role"` // owner, admin, member
}
```

- [x] **Step 2: Add GetCurrentUser to AuthService interface**

Add after line 31 (`ComparePassword`):

```go
    GetCurrentUser(contextID string) (*context.User, error)
```

- [x] **Step 3: Add context import**

Add to imports:

```go
    "github.com/hekemen/automata/internal/domain/context"
```

- [x] **Step 4: Build to verify**

Run: `go build ./internal/domain/...`

Expected: Build succeeds.

- [x] **Step 5: Commit**

```bash
git add internal/domain/auth/auth.go
git commit -m "domain: add Role to ContextInfo, GetCurrentUser to AuthService"
```

---

### Task 4: Infrastructure — PostgreSQL repos for users and user_contexts

**Files:**
- Modify: `internal/infrastructure/context/repo/user_postgres.go` (append new methods)

**Interfaces:**
- Consumes: `pgxpool.Pool`, existing `userPostgresRepo` struct
- Produces: All new methods on `*userPostgresRepo`

- [x] **Step 1: Add `GetByUsername` — platform-level lookup from `users` table**

Append to `user_postgres.go`:

```go
func (r *userPostgresRepo) GetByUsername(email string) (*domainctx.User, error) {
    u := &domainctx.User{}

    query := `
        SELECT id, email, password_hash, sso_provider, sso_id, is_admin, created_at, updated_at
        FROM users WHERE email = $1
    `

    err := r.pool.QueryRow(context.Background(), query, email).Scan(
        &u.ID, &u.Email, &u.PasswordHash, &u.SSOProvider, &u.SSOID,
        &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt,
    )
    if err != nil {
        return nil, fmt.Errorf("get user by username: %w", err)
    }

    return u, nil
}
```

- [x] **Step 2: Add `ExistsAnyUser`**

```go
func (r *userPostgresRepo) ExistsAnyUser() (bool, error) {
    var exists bool
    err := r.pool.QueryRow(context.Background(),
        "SELECT EXISTS(SELECT 1 FROM users LIMIT 1)").Scan(&exists)
    if err != nil {
        return false, fmt.Errorf("check users existence: %w", err)
    }
    return exists, nil
}
```

- [x] **Step 3: Add membership methods**

Append these methods to `user_postgres.go`:

```go
func (r *userPostgresRepo) GetByUserContext(userID, contextID string) (*domainctx.UserContext, error) {
    uc := &domainctx.UserContext{}
    err := r.pool.QueryRow(context.Background(), `
        SELECT id, user_id, context_id, role, created_at, updated_at
        FROM user_contexts WHERE user_id = $1 AND context_id = $2
    `, userID, contextID).Scan(
        &uc.ID, &uc.UserID, &uc.ContextID, &uc.Role,
        &uc.CreatedAt, &uc.UpdatedAt,
    )
    if err != nil {
        return nil, fmt.Errorf("get user context: %w", err)
    }
    return uc, nil
}

func (r *userPostgresRepo) ListByUser(userID string) ([]domainctx.UserContext, error) {
    rows, err := r.pool.Query(context.Background(), `
        SELECT id, user_id, context_id, role, created_at, updated_at
        FROM user_contexts WHERE user_id = $1
    `, userID)
    if err != nil {
        return nil, fmt.Errorf("list user contexts: %w", err)
    }
    defer rows.Close()

    var result []domainctx.UserContext
    for rows.Next() {
        var uc domainctx.UserContext
        if err := rows.Scan(&uc.ID, &uc.UserID, &uc.ContextID, &uc.Role,
            &uc.CreatedAt, &uc.UpdatedAt); err != nil {
            return nil, fmt.Errorf("scan user context: %w", err)
        }
        result = append(result, uc)
    }
    return result, nil
}

func (r *userPostgresRepo) CreateMembership(uc *domainctx.UserContext) error {
    if uc.ID == "" {
        uc.ID = uuid.New().String()
    }
    _, err := r.pool.Exec(context.Background(), `
        INSERT INTO user_contexts (id, user_id, context_id, role, created_at, updated_at)
        VALUES ($1, $2, $3, $4, NOW(), NOW())
    `, uc.ID, uc.UserID, uc.ContextID, uc.Role)
    if err != nil {
        return fmt.Errorf("create user context membership: %w", err)
    }
    return nil
}

func (r *userPostgresRepo) UpdateMembership(uc *domainctx.UserContext) error {
    result, err := r.pool.Exec(context.Background(), `
        UPDATE user_contexts SET role = $1, updated_at = NOW()
        WHERE user_id = $2 AND context_id = $3
    `, uc.Role, uc.UserID, uc.ContextID)
    if err != nil {
        return fmt.Errorf("update user context membership: %w", err)
    }
    if result.RowsAffected() == 0 {
        return fmt.Errorf("user context membership not found")
    }
    return nil
}

func (r *userPostgresRepo) DeleteMembership(userID, contextID string) error {
    result, err := r.pool.Exec(context.Background(), `
        DELETE FROM user_contexts WHERE user_id = $1 AND context_id = $2
    `, userID, contextID)
    if err != nil {
        return fmt.Errorf("delete user context membership: %w", err)
    }
    if result.RowsAffected() == 0 {
        return fmt.Errorf("user context membership not found")
    }
    return nil
}

func (r *userPostgresRepo) ListMembers(contextID string) ([]domainctx.UserContext, error) {
    rows, err := r.pool.Query(context.Background(), `
        SELECT id, user_id, context_id, role, created_at, updated_at
        FROM user_contexts WHERE context_id = $1
    `, contextID)
    if err != nil {
        return nil, fmt.Errorf("list context members: %w", err)
    }
    defer rows.Close()

    var result []domainctx.UserContext
    for rows.Next() {
        var uc domainctx.UserContext
        if err := rows.Scan(&uc.ID, &uc.UserID, &uc.ContextID, &uc.Role,
            &uc.CreatedAt, &uc.UpdatedAt); err != nil {
            return nil, fmt.Errorf("scan user context: %w", err)
        }
        result = append(result, uc)
    }
    return result, nil
}

func (r *userPostgresRepo) ListAll() ([]*domainctx.User, error) {
    rows, err := r.pool.Query(context.Background(), `
        SELECT id, email, password_hash, sso_provider, sso_id, is_admin, created_at, updated_at
        FROM users
    `)
    if err != nil {
        return nil, fmt.Errorf("list all users: %w", err)
    }
    defer rows.Close()

    var result []*domainctx.User
    for rows.Next() {
        u := &domainctx.User{}
        if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.SSOProvider,
            &u.SSOID, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
            return nil, fmt.Errorf("scan user: %w", err)
        }
        result = append(result, u)
    }
    return result, nil
}
```

- [x] **Step 4: Build to verify**

Run: `go build ./internal/infrastructure/context/...`

Expected: Build succeeds.

- [x] **Step 5: Commit**

```bash
git add internal/infrastructure/context/repo/user_postgres.go
git commit -m "infra: add platform user lookup and user_contexts membership methods"
```

---

### Task 5: Auth Service — update Login to use `users` table + membership roles

**Files:**
- Modify: `internal/infrastructure/auth/service.go` (replace Login method)
- Already modified: `internal/domain/auth/auth.go` (ContextInfo.Role from Task 3)

**Interfaces:**
- Consumes: `userRepo.GetByUsername()`, `userRepo.GetByEmailGlobal()`, `userRepo.ListByUser()`
- Produces: Login queries `users` table, builds JWT with `is_admin`, returns contexts with roles

- [x] **Step 1: Replace the Login method**

Replace the entire `Login` method (lines 40-84 in `service.go`) with:

```go
func (s *service) Login(email, password string) (string, string, []domainauth.ContextInfo, error) {
    secretKey := config.Get("auth.secret_key")
    if secretKey == "" {
        secretKey = "automata-dev-secret-key-change-in-production"
    }

    if s.userRepo == nil {
        return "", "", nil, fmt.Errorf("user repository not initialized")
    }

    // Query users table first (platform-level), fall back to context_users
    var u *domainctx.User
    u, err := s.userRepo.GetByUsername(email)
    if err != nil {
        // Fall back to context_users for backward compatibility
        users, fallbackErr := s.userRepo.GetByEmailGlobal(email)
        if fallbackErr != nil || len(users) == 0 {
            return "", "", nil, fmt.Errorf("invalid email or password")
        }
        u = users[0]
    }

    if u.PasswordHash == "" {
        return "", "", nil, fmt.Errorf("invalid email or password")
    }

    if err := s.ComparePassword(u.PasswordHash, password); err != nil {
        return "", "", nil, fmt.Errorf("invalid email or password")
    }

    // Get all context memberships for this user
    memberships, err := s.userRepo.ListByUser(u.ID)
    if err != nil {
        // If no memberships found, fall back to context_users list
        users, fallbackErr := s.userRepo.GetByEmailGlobal(email)
        if fallbackErr == nil && len(users) > 0 {
            memberships = make([]domainctx.UserContext, 0, len(users))
            for _, ctxUser := range users {
                memberships = append(memberships, domainctx.UserContext{
                    ContextID: ctxUser.ContextID,
                    Role:      "owner",
                })
            }
        } else {
            memberships = make([]domainctx.UserContext, 0)
        }
    }

    // Build context list with roles
    contexts := make([]domainauth.ContextInfo, 0, len(memberships))
    for _, mc := range memberships {
        contexts = append(contexts, domainauth.ContextInfo{
            ID:   mc.ContextID,
            Slug: "", // Will be filled by handler from contextRepo
            Name: "", // Will be filled by handler from contextRepo
            Role: mc.Role,
        })
    }

    // Generate JWT with is_admin claim
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "user_id":    u.ID,
        "user_email": email,
        "is_admin":   u.IsAdmin,
        "exp":        time.Now().Add(24 * time.Hour).Unix(),
    })

    tokenString, err := token.SignedString([]byte(secretKey))
    if err != nil {
        log.Error().Err(err).Msg("token generation failed")
        return "", "", nil, fmt.Errorf("generate token: %w", err)
    }

    return tokenString, u.ID, contexts, nil
}
```

- [x] **Step 2: Build to verify**

Run: `go build ./internal/infrastructure/auth/...`

Expected: Build succeeds.

- [x] **Step 3: Commit**

```bash
git add internal/infrastructure/auth/service.go
git commit -m "auth: use users table for login, add is_admin to JWT, return context roles"
```

---

### Task 6: Admin Bootstrap — `internal/infrastructure/bootstrap/admin.go`

**Files:**
- Create: `internal/infrastructure/bootstrap/admin.go`
- Modify: `cmd/automata/main.go` (call bootstrap after migrations)

**Interfaces:**
- Consumes: `pool`, `userRepo`, `contextRepo`
- Produces: Admin user + default context + membership + stdout credentials

- [x] **Step 1: Write admin bootstrap**

Create `internal/infrastructure/bootstrap/admin.go`:

```go
package bootstrap

import (
    "crypto/rand"
    "fmt"
    "os"
    "time"

    "github.com/google/uuid"
    "github.com/hekemen/automata/internal/domain/context"
    context_repo "github.com/hekemen/automata/internal/infrastructure/context/repo"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/rs/zerolog/log"
    "golang.org/x/crypto/bcrypt"
)

// Run creates an admin user on first startup if no users exist.
// It creates a default context if none exists, assigns admin to it with owner role,
// and writes credentials to stdout. Returns nil if users already exist.
func Run(pool *pgxpool.Pool, userRepo context.UserRepository, contextRepo context.Repository) error {
    // Check if any users exist in the new users table
    var exists bool
    err := pool.QueryRow(nil, "SELECT EXISTS(SELECT 1 FROM users LIMIT 1)").Scan(&exists)
    if err == nil && exists {
        return nil // Already initialized
    }

    // Also check context_users for backward compat
    var ctxCount int
    if err2 := pool.QueryRow(nil, "SELECT COUNT(*) FROM context_users").Scan(&ctxCount); err2 == nil && ctxCount > 0 {
        return nil // context_users has data, migration handles it
    }

    // Read config
    adminEmail := os.Getenv("AUTOMATA_ADMIN_EMAIL")
    if adminEmail == "" {
        adminEmail = "admin@automata.local"
    }

    // Generate random password
    password, err := generatePassword(16)
    if err != nil {
        return fmt.Errorf("generate password: %w", err)
    }

    // Hash password
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return fmt.Errorf("hash password: %w", err)
    }

    // Create default context if none exists
    var contextsCount int
    if err := pool.QueryRow(nil, "SELECT COUNT(*) FROM contexts").Scan(&contextsCount); err != nil {
        contextsCount = 0
    }

    var contextID string
    if contextsCount == 0 {
        ctx := &context.Context{
            ID:        uuid.New().String(),
            Slug:      "default",
            Name:      "Default Context",
            IsActive:  true,
            Settings:  make(map[string]interface{}),
        }
        if err := contextRepo.Create(ctx); err != nil {
            return fmt.Errorf("create default context: %w", err)
        }
        contextID = ctx.ID
    } else {
        row := pool.QueryRow(nil, "SELECT id FROM contexts LIMIT 1")
        if err := row.Scan(&contextID); err != nil {
            return fmt.Errorf("get first context: %w", err)
        }
    }

    // Create admin user
    adminUser := &context.User{
        ID:           uuid.New().String(),
        Email:        adminEmail,
        PasswordHash: string(hash),
        IsAdmin:      true,
    }
    if err := userRepo.Create(adminUser); err != nil {
        return fmt.Errorf("create admin user: %w", err)
    }

    // Create membership
    membership := &context.UserContext{
        UserID:    adminUser.ID,
        ContextID: contextID,
        Role:      "owner",
    }
    if err := userRepo.CreateMembership(membership); err != nil {
        log.Warn().Err(err).Msg("failed to create admin membership (user created)")
    }

    // Write marker file
    markerPath := ".admin_initialized"
    if err := os.WriteFile(markerPath, []byte("initialized"), 0644); err != nil {
        log.Warn().Err(err).Msg("failed to write admin initialized marker")
    }

    // Log credentials to stdout
    fmt.Println("")
    fmt.Println("╔══════════════════════════════════════════════════╗")
    fmt.Println("║  ADMIN CREDENTIALS (first startup only)          ║")
    fmt.Printf("║  Email:    %s                          ║\n", adminEmail)
    fmt.Printf("║  Password: %s                      ║\n", password)
    fmt.Println("╚══════════════════════════════════════════════════╝")
    fmt.Println("")

    log.Info().Str("email", adminEmail).Msg("admin user created via bootstrap")
    return nil
}

func generatePassword(length int) (string, error) {
    const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
    bytes := make([]byte, length)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    for i := range bytes {
        bytes[i] = chars[int(bytes[i])%len(chars)]
    }
    return string(bytes), nil
}
```

Note: `context.Context` requires a `Validate()` call in `Create()` which checks `Slug` and `Name`. The default context has both, so it will pass validation. The `Settings: make(map[string]interface{})` is not nil so it marshals to `{}`.

- [x] **Step 2: Wire bootstrap into main.go**

In `cmd/automata/main.go`, after the migration calls (after line 65), add:

```go
    // Admin bootstrap: create admin user on first startup
    if err := bootstrap.Run(pool, userRepo, contextRepo); err != nil {
        log.Warn().Err(err).Msg("admin bootstrap failed (continuing anyway)")
    }
```

Add import: `"github.com/hekemen/automata/internal/infrastructure/bootstrap"`

- [x] **Step 3: Build to verify**

Run: `go build ./cmd/automata`

Expected: Build succeeds.

- [x] **Step 4: Commit**

```bash
git add internal/infrastructure/bootstrap/admin.go cmd/automata/main.go
git commit -m "infra: add admin bootstrap on first startup"
```

---

### Task 7: API Layer — user CRUD endpoints, context memberships, `/auth/me`

**Files:**
- Create: `internal/adapter/api/handler/user_handler.go`
- Create: `internal/adapter/api/handler/context_user_handler.go`
- Modify: `internal/adapter/api/handler/auth_handler.go` (add `GetMe`)
- Modify: `internal/adapter/api/server.go` (register new routes)

**Interfaces:**
- Consumes: `UserRepository`, `context.Repository`, `context.UserRepository`
- Produces: `GET/POST /admin/users`, `GET/PUT/DELETE /admin/users/:id`, `GET /admin/contexts/:id/users`, `POST /admin/contexts/:id/users`, `DELETE /admin/contexts/:id/users/:userId`, `GET /auth/me`

- [x] **Step 1: Write UserHandler**

Create `internal/adapter/api/handler/user_handler.go`:

```go
package handler

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/hekemen/automata/internal/domain/context"
)

type UserHandler struct {
    userRepo    context.UserRepository
    contextRepo context.Repository
}

func NewUserHandler(userRepo context.UserRepository, contextRepo context.Repository) *UserHandler {
    return &UserHandler{userRepo: userRepo, contextRepo: contextRepo}
}

type CreateUserRequest struct {
    Email        string `json:"email" binding:"required,email"`
    Password     string `json:"password" binding:"required"`
    IsAdmin      bool   `json:"is_admin"`
}

type UpdateUserRequest struct {
    Email       *string `json:"email"`
    Password    *string `json:"password"`
    IsAdmin     *bool   `json:"is_admin"`
}

func (h *UserHandler) List(c *gin.Context) {
    users, err := h.userRepo.ListAll()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *UserHandler) Create(c *gin.Context) {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Check for duplicate email first
    user, err := h.userRepo.GetByUsername(req.Email)
    if err == nil && user != nil {
        c.JSON(http.StatusConflict, gin.H{"error": "email already exists"})
        return
    }

    // Hash password before storing
    passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
        return
    }

    u := &context.User{
        Email:        req.Email,
        PasswordHash: string(passwordHash),
        IsAdmin:      req.IsAdmin,
    }
    if err := h.userRepo.Create(u); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusCreated, gin.H{"id": u.ID, "email": u.Email, "is_admin": u.IsAdmin})
}

func (h *UserHandler) Get(c *gin.Context) {
    id := c.Param("id")
    if _, err := uuid.Parse(id); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
        return
    }
    u, err := h.userRepo.GetByID(id)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
        return
    }
    c.JSON(http.StatusOK, gin.H{
        "id":        u.ID,
        "email":     u.Email,
        "is_admin":  u.IsAdmin,
        "created_at": u.CreatedAt,
    })
}

func (h *UserHandler) Update(c *gin.Context) {
    id := c.Param("id")
    if _, err := uuid.Parse(id); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
        return
    }
    u, err := h.userRepo.GetByID(id)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
        return
    }
    var req UpdateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if req.Email != nil {
        u.Email = *req.Email
    }
    if req.Password != nil {
        hashed, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
            return
        }
        u.PasswordHash = string(hashed)
    }
    if req.IsAdmin != nil {
        u.IsAdmin = *req.IsAdmin
    }
    if err := h.userRepo.Update(u); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"id": u.ID, "email": u.Email, "is_admin": u.IsAdmin})
}

func (h *UserHandler) Delete(c *gin.Context) {
    id := c.Param("id")
    if _, err := uuid.Parse(id); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
        return
    }
    if err := h.userRepo.Delete(id); err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
        return
    }
    c.JSON(http.StatusNoContent, nil)
}
```

- [x] **Step 2: Write ContextUserHandler**

Create `internal/adapter/api/handler/context_user_handler.go`:

```go
package handler

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/hekemen/automata/internal/domain/context"
    "golang.org/x/crypto/bcrypt"
)

type ContextUserHandler struct {
    userRepo    context.UserRepository
    contextRepo context.Repository
}

func NewContextUserHandler(userRepo context.UserRepository, contextRepo context.Repository) *ContextUserHandler {
    return &ContextUserHandler{userRepo: userRepo, contextRepo: contextRepo}
}

type AddContextUserRequest struct {
    UserID string `json:"user_id" binding:"required"`
    Role   string `json:"role" binding:"required,oneof=owner admin member"`
}

func (h *ContextUserHandler) List(c *gin.Context) {
    contextID := c.Param("id")
    if _, err := uuid.Parse(contextID); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
        return
    }
    members, err := h.userRepo.ListMembers(contextID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"members": members})
}

func (h *ContextUserHandler) Create(c *gin.Context) {
    contextID := c.Param("id")
    if _, err := uuid.Parse(contextID); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
        return
    }
    var req AddContextUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    uc := &context.UserContext{
        UserID:    req.UserID,
        ContextID: contextID,
        Role:      req.Role,
    }
    if err := h.userRepo.CreateMembership(uc); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusCreated, gin.H{
        "user_id":    uc.UserID,
        "context_id": uc.ContextID,
        "role":       uc.Role,
    })
}

func (h *ContextUserHandler) Delete(c *gin.Context) {
    contextID := c.Param("id")
    userID := c.Param("userId")
    if _, err := uuid.Parse(contextID); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
        return
    }
    if _, err := uuid.Parse(userID); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
        return
    }
    if err := h.userRepo.DeleteMembership(userID, contextID); err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "membership not found"})
        return
    }
    c.JSON(http.StatusNoContent, nil)
}
```

- [x] **Step 3: Add GetMe to AuthHandler**

In `internal/adapter/api/handler/auth_handler.go`, add the `GetMe` method after the existing `Logout` method:

```go
func (h *AuthHandler) GetMe(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
        return
    }

    // Get user from platform users table
    user, err := h.userRepo.GetByID(userID)
    if err != nil {
        // Fall back to context_users
        users, _ := h.userRepo.GetByEmailGlobal("")
        if len(users) == 0 {
            c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
            return
        }
        user = users[0]
    }

    // Get context memberships
    memberships, _ := h.userRepo.ListByUser(userID)

    contexts := make([]gin.H, 0, len(memberships))
    for _, mc := range memberships {
        ctx, err := h.contextRepo.GetByID(mc.ContextID)
        if err != nil {
            continue
        }
        contexts = append(contexts, gin.H{
            "id":   ctx.ID,
            "slug": ctx.Slug,
            "name": ctx.Name,
            "role": mc.Role,
        })
    }

    c.JSON(http.StatusOK, gin.H{
        "id":        user.ID,
        "email":     user.Email,
        "is_admin":  user.IsAdmin,
        "contexts":  contexts,
    })
}
```

- [x] **Step 4: Register new routes in server.go**

In `internal/adapter/api/server.go`, inside the `admin` group (after line 56, before the closing `}`), add:

```go
        userHandler := handler.NewUserHandler(userRepo, contextRepo)
        contextUserHandler := handler.NewContextUserHandler(userRepo, contextRepo)

        usersRoutes := admin.Group("/users")
        {
            usersRoutes.GET("", userHandler.List)
            usersRoutes.POST("", userHandler.Create)
            usersRoutes.GET("/:id", userHandler.Get)
            usersRoutes.PUT("/:id", userHandler.Update)
            usersRoutes.DELETE("/:id", userHandler.Delete)
        }

        contextUserRoutes := admin.Group("/contexts/:id/users")
        {
            contextUserRoutes.GET("", contextUserHandler.List)
            contextUserRoutes.POST("", contextUserHandler.Create)
            contextUserRoutes.DELETE("/:userId", contextUserHandler.Delete)
        }
```

Also add `/auth/me` route (inside `api` group, before `admin` group):

```go
    api.GET("/auth/me", authHandler.GetMe)
```

- [x] **Step 5: Build to verify**

Run: `go build ./internal/adapter/api/...`

Expected: Build succeeds.

- [x] **Step 6: Commit**

```bash
git add internal/adapter/api/handler/user_handler.go internal/adapter/api/handler/context_user_handler.go internal/adapter/api/handler/auth_handler.go internal/adapter/api/server.go
git commit -m "api: add user CRUD, context memberships, and /auth/me endpoints"
```

---

### Task 8: Auth middleware — extract `is_admin` from JWT

**Files:**
- Modify: `internal/adapter/api/middleware/auth.go`

**Interfaces:**
- Consumes: `authService.VerifyTokenUserOnly()`
- Produces: Sets `c.Set("is_admin", true/false)` from JWT claims

- [x] **Step 1: Update AuthMiddleware to set is_admin**

Modify `internal/adapter/api/middleware/auth.go` to add `is_admin` to the context after successful auth:

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
            // Parse token to get is_admin claim
            secretKey := config.Get("auth.secret_key")
            if secretKey == "" {
                secretKey = "automata-dev-secret-key-change-in-production"
            }
            parsedToken, parseErr := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
                return []byte(secretKey), nil
            })
            if parseErr == nil {
                if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
                    if isAdmin, ok := claims["is_admin"].(bool); ok {
                        c.Set("is_admin", isAdmin)
                    }
                }
            }
            c.Next()
            return
        }

        // ... rest unchanged
    }
}
```

Add imports:
```go
    "github.com/golang-jwt/jwt/v5"
    "github.com/hekemen/automata/internal/infrastructure/config"
```

- [x] **Step 2: Build to verify**

Run: `go build ./internal/adapter/api/middleware/...`

Expected: Build succeeds.

- [x] **Step 3: Commit**

```bash
git add internal/adapter/api/middleware/auth.go
git commit -m "middleware: extract is_admin from JWT claims"
```

---

### Task 9: Update existing code — `GetByEmailGlobal` backward compat, repo Create for users table

**Files:**
- Modify: `internal/infrastructure/context/repo/user_postgres.go` (update Create, GetByEmailGlobal, GetByID to handle both tables)

**Interfaces:**
- Consumes: Both `users` and `context_users` tables
- Produces: Backward-compatible repository methods

- [x] **Step 1: Update `Create` to insert into `users` table**

Replace the `Create` method to use the `users` table:

```go
func (r *userPostgresRepo) Create(u *domainctx.User) error {
    if u.ID == "" {
        u.ID = uuid.New().String()
    }
    if err := u.Validate(); err != nil {
        return err
    }

    // Insert into users table (platform-level)
    query := `
        INSERT INTO users (id, email, password_hash, sso_provider, sso_id, is_admin, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
        ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash
        RETURNING id, email, password_hash, sso_provider, sso_id, is_admin, created_at, updated_at
    `

    err := r.pool.QueryRow(context.Background(), query,
        u.ID, u.Email, u.PasswordHash, nil, nil, u.IsAdmin,
    ).Scan(
        &u.ID, &u.Email, &u.PasswordHash, &u.SSOProvider, &u.SSOID,
        &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt,
    )
    if err != nil {
        return fmt.Errorf("create user: %w", err)
    }
    return nil
}
```

- [x] **Step 2: Update `GetByEmailGlobal` to query both tables**

Replace `GetByEmailGlobal` to query `users` first, then `context_users` as fallback:

```go
func (r *userPostgresRepo) GetByEmailGlobal(email string) ([]*domainctx.User, error) {
    // Try users table first
    rows, err := r.pool.Query(context.Background(), `
        SELECT id, email, password_hash, sso_provider, sso_id, is_admin, created_at, updated_at
        FROM users WHERE email = $1
    `, email)
    if err == nil {
        defer rows.Close()
        var users []*domainctx.User
        for rows.Next() {
            u := &domainctx.User{}
            err := rows.Scan(
                &u.ID, &u.Email, &u.PasswordHash, &u.SSOProvider, &u.SSOID,
                &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt,
            )
            if err != nil {
                return nil, fmt.Errorf("scan user: %w", err)
            }
            users = append(users, u)
        }
        if len(users) > 0 {
            return users, nil
        }
    }

    // Fall back to context_users
    return r.getUsersFromContextUsers(email)
}

func (r *userPostgresRepo) getUsersFromContextUsers(email string) ([]*domainctx.User, error) {
    query := `
        SELECT id, context_id, email, password_hash, is_owner, created_at, updated_at
        FROM context_users WHERE email = $1
    `

    rows, err := r.pool.Query(context.Background(), query, email)
    if err != nil {
        return nil, fmt.Errorf("get user by email (global): %w", err)
    }
    defer rows.Close()

    var users []*domainctx.User
    for rows.Next() {
        u := &domainctx.User{}
        err := rows.Scan(
            &u.ID, &u.ContextID, &u.Email, &u.PasswordHash, &u.IsOwner,
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

- [x] **Step 3: Update `GetByID` to query users table**

```go
func (r *userPostgresRepo) GetByID(id string) (*domainctx.User, error) {
    u := &domainctx.User{}

    // Try users table first
    query := `
        SELECT id, email, password_hash, sso_provider, sso_id, is_admin, created_at, updated_at
        FROM users WHERE id = $1
    `
    err := r.pool.QueryRow(context.Background(), query, id).Scan(
        &u.ID, &u.Email, &u.PasswordHash, &u.SSOProvider, &u.SSOID,
        &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt,
    )
    if err == nil {
        return u, nil
    }

    // Fall back to context_users
    query = `
        SELECT id, context_id, email, password_hash, is_owner, created_at, updated_at
        FROM context_users WHERE id = $1
    `
    err = r.pool.QueryRow(context.Background(), query, id).Scan(
        &u.ID, &u.ContextID, &u.Email, &u.PasswordHash, &u.IsOwner,
        &u.CreatedAt, &u.UpdatedAt,
    )
    if err != nil {
        return nil, fmt.Errorf("get user by ID: %w", err)
    }

    return u, nil
}
```

- [x] **Step 4: Build to verify**

Run: `go build ./internal/infrastructure/context/...`

Expected: Build succeeds.

- [x] **Step 5: Commit**

```bash
git add internal/infrastructure/context/repo/user_postgres.go
git commit -m "infra: update user repo to use users table with context_users fallback"
```

---

### Task 10: Test fixes and verification

**Files:**
- Modify: `cicd/global_login_test.go`
- Modify: `cicd/suite_test.go` (if needed)
- Modify: `cicd/contact_http_test.go` (if needed)
- Modify: any test using `context_users` directly

**Interfaces:**
- Consumes: New tables, new interfaces
- Produces: All 241 integration tests passing

- [x] **Step 1: Update `global_login_test.go` to create users in `users` table**

Replace the `context_users` INSERT with `users` table INSERT:

```go
// OLD (remove):
// INSERT INTO context_users (id, context_id, email, password_hash, is_owner, created_at, updated_at)

// NEW:
userID := support.NewTestUUID()
passwordHash, err := authSvc.HashPassword("REDACTED")
_, err = pool.Exec(ctx,
    `INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
     VALUES ($1, $2, $3, $4, NOW(), NOW())`,
    userID, "admin@example.com", passwordHash, true,
)
// Also create membership:
_, err = pool.Exec(ctx,
    `INSERT INTO user_contexts (user_id, context_id, role, created_at, updated_at)
     VALUES ($1, $2, $3, NOW(), NOW())`,
    userID, contextID, "owner",
)
```

Update `AfterEach` to clean up:

```go
_, err = pool.Exec(ctx, "DELETE FROM user_contexts WHERE context_id = $1", contextID)
_, err = pool.Exec(ctx, "DELETE FROM context_users WHERE context_id = $1", contextID)
_, err = pool.Exec(ctx, "DELETE FROM users WHERE email = $1", "admin@example.com")
_, err = pool.Exec(ctx, "DELETE FROM contexts WHERE id = $1", contextID)
```

- [x] **Step 2: Run full test suite**

Run: `go test ./cicd/... -count=1 -timeout 120s`

Expected: 241/241 tests passing.

- [x] **Step 3: Fix any remaining test failures**

If any tests fail, fix them. Common issues:
- Queries still referencing `context_users` in test setup
- Response keys like `"id"` vs `"ID"`
- Missing table creation in BeforeEach

- [x] **Step 4: Run build and tests one final time**

Run:
```bash
go build ./...
go test ./cicd/... -count=1 -timeout 120s
```

Expected: Build clean, 241/241 tests passing.

- [x] **Step 5: Commit all test changes**

```bash
git add cicd/
git commit -m "tests: update tests for platform users and user_contexts tables"
```

---

### Task 11: Final verification and rebuild

**Files:**
- All files modified

**Interfaces:**
- Consumes: Everything
- Produces: Deployable application

- [x] **Step 1: Full build**

Run: `go build ./...`

Expected: Clean build.

- [x] **Step 2: Full test suite**

Run: `go test ./cicd/... -count=1 -timeout 120s`

Expected: 241/241 tests passing.

- [x] **Step 3: Docker rebuild**

Run: `docker compose down -v && docker compose up --build -d`

Expected: Containers healthy, admin credentials logged to stdout.

- [x] **Step 4: Verify login**

Run:
```bash
curl -s -X POST http://localhost:9080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"REDACTED"}'
```

Expected: JSON response with `token`, `user_id`, `email`, `contexts` array.

- [x] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat: complete context rename and login simplification

- Add platform-level users and user_contexts tables
- Admin bootstrap on first startup
- User CRUD and context membership endpoints
- Login uses users table, returns context roles
- /auth/me endpoint for current user info
- Backward compatible with existing context_users during transition"
```
