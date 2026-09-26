# Context Rename + Login Simplification — Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add platform-level users and context memberships, admin bootstrap on first startup, user CRUD and membership endpoints, and `/auth/me` — completing the remaining pieces from the context-rename / login-simplification design.

**Architecture:** Create `users` (platform-level accounts) and `user_contexts` (role-based memberships) tables via migration. Extend domain interfaces with membership methods and a platform-level user struct. Implement repository methods for both tables, update login to query the users table and build JWTs with `is_admin`. Wire admin bootstrap into main.go, then add user CRUD and context membership API handlers with corresponding routes.

**Tech Stack:** Go 1.26.2, Gin v1.12.0, PostgreSQL 16+ (pgx v5), golang-jwt/v5, bcrypt, zerolog

**Spec:** `docs/superpowers/specs/2026-09-24-implementation-spec.md`

## Global Constraints

- Module path: `github.com/hekemen/automata`
- Go version: `1.26.2`
- PostgreSQL: `16+` (for `gen_random_uuid()`)
- JWT library: `github.com/golang-jwt/jwt/v5` with HS256 signing
- Password hashing: `golang.org/x/crypto/bcrypt` with `bcrypt.DefaultCost` (10)
- All new table names: `users`, `user_contexts`
- All role values: `owner`, `admin`, `member` (lowercase strings)
- Migration file: `internal/infrastructure/database/migration_user_contexts.sql`
- Migration version: `202609240000`
- Admin bootstrap password: 16 characters, alphanumeric + special (`!@#$%^&*`)
- Admin bootstrap marker file: `.admin_initialized` in working directory
- Existing `context_users` table remains for migration data copy (dropped by migration)
- Hexagonal architecture: domain interfaces in `internal/domain/`, infrastructure in `internal/infrastructure/`, adapters in `internal/adapter/`
- All new files go in existing package directories

## Review Focus

1. **Login with nonexistent email** — Must return 401 `"invalid credentials"`, not 500; auth service checks `users` table first
2. **Duplicate email during user creation** — Must return 409 Conflict (unique constraint on `users.email`); check for existence before insert
3. **Bootstrap with existing `context_users` data** — Migration must create `users` rows for every distinct email from `context_users` before creating `user_contexts` memberships
4. **JWT claim backward compat** — Old tokens lack `is_admin`; middleware must check claim existence before accessing
5. **Admin bootstrap on every startup** — `.admin_initialized` marker prevents re-creation; if marker exists but no admin in DB, regenerate with a warning

---

### Task 1: Database Schema — `users` + `user_contexts` tables + migration

**Files:**
- Create: `internal/infrastructure/database/migration_user_contexts.sql`

**Interfaces:**
- Consumes: PostgreSQL pool with `contexts` and `context_users` tables
- Produces: New tables `users` and `user_contexts` with migrated data from `context_users`

Steps:
1. **Step 1: Write migration SQL**

Create `internal/infrastructure/database/migration_user_contexts.sql`:

```sql
-- Migration: 202609240000 — Add users and user_contexts tables

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

CREATE TABLE IF NOT EXISTS user_contexts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    context_id      UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    role            VARCHAR(32) NOT NULL DEFAULT 'member',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, context_id)
);

-- Migrate existing context_users data
DO $$
BEGIN
    INSERT INTO users (email, password_hash, is_admin, created_at, updated_at)
    SELECT email, password_hash, is_owner, created_at, updated_at
    FROM context_users
    ON CONFLICT (email) DO NOTHING;

    INSERT INTO user_contexts (user_id, context_id, role, created_at, updated_at)
    SELECT cu.id, cu.context_id,
           CASE WHEN cu.is_owner THEN 'owner' ELSE 'admin' END,
           NOW(), NOW()
    FROM context_users cu
    INNER JOIN users u ON u.email = cu.email;
END $$;

CREATE INDEX IF NOT EXISTS idx_user_contexts_user ON user_contexts(user_id);
CREATE INDEX IF NOT EXISTS idx_user_contexts_context ON user_contexts(context_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
```

2. **Step 2: Verify the migration runner discovers new `.sql` files**

Read `internal/infrastructure/database/` to find how `RunMigrations` discovers SQL files. The new file uses the `migration_*.sql` naming convention — confirm it's picked up automatically. If not, add a call after the existing `RunMigrations`.

**Verification:**
- `go build ./internal/infrastructure/database/...` compiles clean

---

### Task 2: Domain Layer — User struct, UserContext struct, interface updates

**Files:**
- Modify: `internal/domain/context/user.go`
- Modify: `internal/domain/context/user_repository.go`
- Modify: `internal/domain/auth/auth.go`

**Interfaces:**
- Consumes: Existing `User` struct, `UserRepository` interface, `AuthService` interface
- Produces: Updated `User` with `IsAdmin`, `SSOProvider`, `SSOID`; new `UserContext` struct; extended `UserRepository`; `ContextInfo` with `Role`; `AuthService.GetCurrentUser`

Steps:
1. **Step 1: Extend User struct and add UserContext struct**

In `internal/domain/context/user.go`, add platform-level fields to the existing `User` struct: `IsAdmin bool`, `SSOProvider *string`, `SSOID *string`. Then add the `UserContext` struct:

```go
type UserContext struct {
    ID        string
    UserID    string
    ContextID string
    Role      string // "owner", "admin", or "member"
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

2. **Step 2: Extend UserRepository interface**

In `internal/domain/context/user_repository.go`, add the following methods to the existing `UserRepository` interface (keep existing methods for backward compat):

```go
GetByUsername(ctx context.Context, email string) (*User, error)
ExistsAnyUser(ctx context.Context) (bool, error)
GetByUserIDContext(ctx context.Context, userID, contextID string) (*UserContext, error)
ListByUser(ctx context.Context, userID string) ([]UserContext, error)
CreateMembership(ctx context.Context, uc *UserContext) error
UpdateMembership(ctx context.Context, uc *UserContext) error
DeleteMembership(ctx context.Context, userID, contextID string) error
ListMembers(ctx context.Context, contextID string) ([]UserContext, error)
```

3. **Step 3: Update AuthService interface and ContextInfo**

In `internal/domain/auth/auth.go`, add `Role string` to the `ContextInfo` struct and add `GetCurrentUser(ctx context.Context) (*context.User, error)` to the `AuthService` interface. Add the `context` package import.

**Verification:**
- `go build ./internal/domain/...` compiles clean

---

### Task 3: Infrastructure — New repository methods for users and user_contexts

**Files:**
- Modify: `internal/infrastructure/context/repo/user_postgres.go`
- Create: `internal/infrastructure/context/repo/user_context_postgres.go`

**Interfaces:**
- Consumes: `pgxpool.Pool`, domain `User` and `UserContext` structs
- Produces: Implementations of all new `UserRepository` methods on `*userPostgresRepo`; new `*userContextPostgresRepo` for membership operations (can embed shared query logic from user_postgres.go)

Steps:
1. **Step 1: Add platform-level lookup methods to user_postgres.go**

Append these methods to `user_postgres.go`:

```go
func (r *userPostgresRepo) GetByUsername(ctx context.Context, email string) (*domainctx.User, error)
func (r *userPostgresRepo) ExistsAnyUser(ctx context.Context) (bool, error)
```

`GetByUsername` queries the `users` table directly (no context scoping). `ExistsAnyUser` runs `SELECT EXISTS(SELECT 1 FROM users LIMIT 1)`.

2. **Step 2: Add membership methods to user_context_postgres.go (new file)**

Create a new file `user_context_postgres.go` with a `*userContextPostgresRepo` struct holding the pool. Implement:

```go
func (r *userContextPostgresRepo) GetByUserIDContext(ctx context.Context, userID, contextID string) (*domainctx.UserContext, error)
func (r *userContextPostgresRepo) ListByUser(ctx context.Context, userID string) ([]domainctx.UserContext, error)
func (r *userContextPostgresRepo) CreateMembership(ctx context.Context, uc *domainctx.UserContext) error
func (r *userContextPostgresRepo) UpdateMembership(ctx context.Context, uc *domainctx.UserContext) error
func (r *userContextPostgresRepo) DeleteMembership(ctx context.Context, userID, contextID string) error
func (r *userContextPostgresRepo) ListMembers(ctx context.Context, contextID string) ([]domainctx.UserContext, error)
```

Each method queries the `user_contexts` table. `CreateMembership` generates a UUID for `uc.ID` if empty. `DeleteMembership` and `UpdateMembership` verify `RowsAffected() > 0`.

3. **Step 3: Wire the userContextPostgresRepo into the UserRepository implementation**

In the existing user repository constructor, either embed the userContextPostgresRepo or delegate membership calls. The `UserRepository` interface is in `user_postgres.go`, so `userPostgresRepo` must satisfy it — delegate membership method calls to the userContextPostgresRepo instance.

**Verification:**
- `go build ./internal/infrastructure/context/...` compiles clean

---

### Task 4: Auth service — update Login to use `users` table, build JWT with `is_admin`, return context roles

**Files:**
- Modify: `internal/infrastructure/auth/service.go`

**Interfaces:**
- Consumes: `userRepo.GetByUsername()`, `userRepo.ListByUser()`, bcrypt, JWT library
- Produces: `Login` that queries `users` table, builds JWT with `is_admin` claim, returns `[]ContextInfo` with `Role` populated

Steps:
1. **Step 1: Rewrite the Login method**

Replace the existing `Login` method in `service.go`. New flow:
- Look up user via `userRepo.GetByUsername(email)` (queries `users` table)
- Check `PasswordHash` is not empty (SSO-only users have none)
- Verify password with `ComparePassword`
- Query memberships via `userRepo.ListByUser(userID)`
- Build `[]ContextInfo` with `Role` from each membership
- Generate JWT with `user_id`, `user_email`, `is_admin`, and `exp` claims
- Return `(token, userID, contexts, nil)`

On not-found or wrong password, return `(zero, zero, nil, "invalid email or password")`.

**Verification:**
- `go build ./internal/infrastructure/auth/...` compiles clean

---

### Task 5: Admin bootstrap — `internal/infrastructure/bootstrap/admin.go`

**Files:**
- Create: `internal/infrastructure/bootstrap/admin.go`
- Modify: `cmd/automata/main.go` (wire bootstrap call after DB connection)

**Interfaces:**
- Consumes: `pool *pgxpool.Pool`, `userRepo context.UserRepository`, `contextRepo context.Repository`
- Produces: `Bootstrap(pool, userRepo, contextRepo) error` — creates admin user + default context + membership on first startup; writes credentials to stdout

Steps:
1. **Step 1: Write admin bootstrap function**

Create `internal/infrastructure/bootstrap/admin.go` with `func Bootstrap(pool *pgxpool.Pool, userRepo context.UserRepository, contextRepo context.Repository) error`:

- Check `users` table count — if > 0, return nil (already initialized)
- Read `auth.admin_email` from config (env `AUTOMATA_ADMIN_EMAIL` or default `admin@automata.local`)
- Generate 16-char random password using `crypto/rand` from chars `abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*`
- Hash with bcrypt (DefaultCost)
- If no contexts exist, create a default context with slug `"default"` and name `"Default Context"`
- Create user in `users` table with `is_admin = true`
- Create membership in `user_contexts` with role `"owner"`
- Write `.admin_initialized` marker file
- Print credentials block to stdout (exact ASCII art box per spec §2)

2. **Step 2: Wire into main.go**

In `cmd/automata/main.go`, after DB pool creation and migrations, call `bootstrap.Run(pool, userRepo, contextRepo)`. Log any error with warning level but don't abort.

**Verification:**
- `go build ./cmd/automata` compiles clean

---

### Task 6: API layer — UserHandler, ContextUserHandler, AuthHandler.GetMe

**Files:**
- Create: `internal/adapter/api/handler/user_handler.go`
- Create: `internal/adapter/api/handler/context_user_handler.go`
- Modify: `internal/adapter/api/handler/auth_handler.go` (add `GetMe` method, update `Login` response)

**Interfaces:**
- Consumes: `context.UserRepository`, `context.Repository`, `auth.AuthService`
- Produces: `GET/POST /admin/users`, `GET/PUT/DELETE /admin/users/:id`, `GET/POST /admin/contexts/:id/users`, `DELETE /admin/contexts/:id/users/:userId`, `GET /auth/me`

Steps:
1. **Step 1: Write UserHandler**

Create `internal/adapter/api/handler/user_handler.go` with:

```go
type UserHandler struct { userRepo context.UserRepository; contextRepo context.Repository }
type CreateUserRequest struct { Email string `json:"email" binding:"required,email"`; Password string `json:"password" binding:"required"`; IsAdmin bool `json:"is_admin"` }
type UpdateUserRequest struct { Email *string `json:"email"`; Password *string `json:"password"`; IsAdmin *bool `json:"is_admin"` }
func (h *UserHandler) List(c *gin.Context)      // GET /admin/users — returns all platform users
func (h *UserHandler) Create(c *gin.Context)    // POST /admin/users — check duplicate email (409), hash password
func (h *UserHandler) Get(c *gin.Context)       // GET /admin/users/:id — return user by UUID
func (h *UserHandler) Update(c *gin.Context)    // PUT /admin/users/:id — partial update with pointers
func (h *UserHandler) Delete(c *gin.Context)    // DELETE /admin/users/:id
```

2. **Step 2: Write ContextUserHandler**

Create `internal/adapter/api/handler/context_user_handler.go` with:

```go
type ContextUserHandler struct { userRepo context.UserRepository; contextRepo context.Repository }
type AddContextUserRequest struct { UserID string `json:"user_id" binding:"required"`; Role string `json:"role" binding:"required,oneof=owner admin member"` }
func (h *ContextUserHandler) List(c *gin.Context)    // GET /admin/contexts/:id/users
func (h *ContextUserHandler) Create(c *gin.Context)  // POST /admin/contexts/:id/users
func (h *ContextUserHandler) Delete(c *gin.Context)  // DELETE /admin/contexts/:id/users/:userId
```

3. **Step 3: Add GetMe to AuthHandler**

In `internal/adapter/api/handler/auth_handler.go`, add `GetMe` method:
- Extract `user_id` from context (set by auth middleware)
- Look up user via `userRepo.GetByID(userID)`
- List memberships via `userRepo.ListByUser(userID)`
- For each membership, fetch context via `contextRepo.GetByID(mc.ContextID)`
- Return `{id, email, is_admin, contexts: [{id, slug, name, role}]}`

4. **Step 4: Update Login handler response**

Ensure the login response handler includes the `role` field from each `ContextInfo` in the JSON output (the auth service already returns ContextInfo with Role from Task 4).

**Verification:**
- `go build ./internal/adapter/api/...` compiles clean

---

### Task 7: Register routes and update auth middleware

**Files:**
- Modify: `internal/adapter/api/server.go` (register new routes, create handlers)
- Modify: `internal/adapter/api/middleware/auth.go` (extract `is_admin` from JWT claims)

**Interfaces:**
- Consumes: UserHandler, ContextUserHandler, AuthHandler instances from Task 6
- Produces: New API routes registered; middleware sets `c.Set("is_admin", ...)` on successful auth

Steps:
1. **Step 1: Register routes in server.go**

In `internal/adapter/api/server.go`, inside the `admin` group:

```go
userHandler := handler.NewUserHandler(userRepo, contextRepo)
contextUserHandler := handler.NewContextUserHandler(userRepo, contextRepo)

usersRoutes := admin.Group("/users")
usersRoutes.GET("", userHandler.List)
usersRoutes.POST("", userHandler.Create)
usersRoutes.GET("/:id", userHandler.Get)
usersRoutes.PUT("/:id", userHandler.Update)
usersRoutes.DELETE("/:id", userHandler.Delete)

contextUserRoutes := admin.Group("/contexts/:id/users")
contextUserRoutes.GET("", contextUserHandler.List)
contextUserRoutes.POST("", contextUserHandler.Create)
contextUserRoutes.DELETE("/:userId", contextUserHandler.Delete)
```

Also add under the `api` group: `api.GET("/auth/me", authHandler.GetMe)`.

2. **Step 2: Update auth middleware**

In `internal/adapter/api/middleware/auth.go`, after successful token validation, parse the JWT token and extract the `is_admin` claim. Set `c.Set("is_admin", isAdmin)` where `isAdmin` is the bool claim value (default false if claim missing — handles backward compat per Review Focus #4).

Add import: `"github.com/golang-jwt/jwt/v5"`.

**Verification:**
- `go build ./internal/adapter/api/...` compiles clean

---

### Task 8: Test updates — login flow, user CRUD, bootstrap

**Files:**
- Modify: `cicd/global_login_test.go` (update to use `users` table)
- Create: `cicd/user_handler_test.go`
- Create: `cicd/context_user_handler_test.go`
- Create: `cicd/bootstrap_test.go`

**Interfaces:**
- Consumes: New tables, new endpoints, new domain models
- Produces: All integration tests passing — login works with `users` table, CRUD endpoints return correct status codes, bootstrap creates admin on first startup

Steps:
1. **Step 1: Update `global_login_test.go`**

Replace the `context_users` INSERT with a `users` table INSERT + `user_contexts` membership INSERT. Verify the login endpoint returns a token and `contexts` array with `role` field.

2. **Step 2: Write user handler tests**

In `cicd/user_handler_test.go`:
- `POST /admin/users` with valid body returns 201
- `POST /admin/users` with duplicate email returns 409
- `GET /admin/users/:id` returns user or 404
- `PUT /admin/users/:id` updates fields
- `DELETE /admin/users/:id` returns 204
- `GET /admin/users` returns user list

3. **Step 3: Write context user handler tests**

In `cicd/context_user_handler_test.go`:
- `GET /admin/contexts/:id/users` returns members list
- `POST /admin/contexts/:id/users` with valid role creates membership
- `POST /admin/contexts/:id/users` with invalid role returns 400
- `DELETE /admin/contexts/:id/users/:userId` removes membership

4. **Step 4: Write bootstrap test**

In `cicd/bootstrap_test.go`:
- When `users` table is empty, `Bootstrap` creates admin user with default email
- Admin has `is_admin = true` and membership with role `"owner"`
- `.admin_initialized` marker file exists after bootstrap
- When `users` table already has entries, `Bootstrap` returns nil (no-op)

**Verification:**
- `go test ./cicd/... -count=1` — all tests pass

---

### Task 9: Final verification and rebuild

**Files:**
- All files modified/created

**Interfaces:**
- Consumes: Everything

Steps:
1. **Step 1: Full build**

Run: `go build ./...`
Expected: Clean build, no errors.

2. **Step 2: Full test suite**

Run: `go test ./cicd/... -count=1`
Expected: All tests passing.

3. **Step 3: Verify login endpoint manually**

Run:
```bash
curl -s -X POST http://localhost:9080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@automata.local","password":"<bootstrap-password>"}'
```
Expected: JSON with `token`, `user_id`, `email`, and `contexts` array containing `role` field.

4. **Step 4: Verify /auth/me endpoint**

Use the token from Step 3:
```bash
curl -s http://localhost:9080/api/auth/me \
  -H "Authorization: Bearer <token>"
```
Expected: JSON with `id`, `email`, `is_admin: true`, and `contexts` array.

**Verification:**
- All manual endpoint checks return expected responses
- `go build ./...` still clean
