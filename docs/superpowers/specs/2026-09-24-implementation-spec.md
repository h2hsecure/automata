# Implementation Spec: Context Rename + Login Simplification (Phase 2)

> **Purpose:** Implement the remaining pieces of the context-rename / login-simplification design so the running application matches `docs/superpowers/specs/2026-09-22-context-rename-and-login-simplification.md`.
>
> **Prerequisite:** The existing schema rename (tenant→context in table names, columns, code variable names) is already committed. This spec covers what was *not* yet implemented.

---

## 1. New Schema: `users` + `user_contexts`

### 1.1 `users` table (platform-level accounts)

```sql
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
```

- `password_hash` is NULL for SSO-only users
- `is_admin` grants platform-wide admin access

### 1.2 `user_contexts` table (role-based memberships)

```sql
CREATE TABLE IF NOT EXISTS user_contexts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    context_id      UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    role            VARCHAR(32) NOT NULL DEFAULT 'member',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, context_id)
);
```

- Roles: `owner`, `admin`, `member`
- `owner` can manage the context and its users
- `admin` can manage contacts, forms, banners, emails
- `member` has read access (configurable per context)

### 1.3 Migration from `context_users`

- Existing `context_users` rows have `is_owner` boolean.
- Migrate: for each `context_users` row with `is_owner = true`, create a `user_contexts` entry with role `owner`; otherwise `admin`.
- Migrate `context_users` → `users`: for each unique email, create a `users` row. If the same email existed in multiple contexts, create one `users` row with multiple `user_contexts` rows.

### 1.4 Migration file

Create: `internal/infrastructure/database/migration_user_contexts.sql`

Steps:
1. `CREATE TABLE users …`
2. `CREATE TABLE user_contexts …`
3. Migrate data from `context_users` → `users` + `user_contexts`
4. Drop index `idx_context_users_context_email`
5. Add index `idx_user_contexts_user` and `idx_user_contexts_context`
6. Mark migration version `202609240000`

---

## 2. Admin Bootstrap

On first startup (no `users` exist and `context_users` is empty):

1. Read config `auth.admin_email` (default `admin@automata.local`)
2. Generate a random 16-character password
3. Hash with bcrypt (cost 10)
4. Insert into `users` with `is_admin = true`
5. Insert into `user_contexts` for every existing context with role `owner`; if no contexts exist, create a default context first
6. Log credentials to stdout:

```
╔══════════════════════════════════════════════════╗
║  ADMIN CREDENTIALS (first startup only)          ║
║  Email:    admin@automata.local                  ║
║  Password: <generated-password>                  ║
╚══════════════════════════════════════════════════╝
```

7. Write `.admin_initialized` marker file to the working directory to prevent re-generation.
8. If marker exists but no admin in DB, regenerate and log a warning.

**Implementation:** New function `internal/infrastructure/bootstrap/admin.go` with `func Bootstrap(pool *pgxpool.Pool) error`. Called from `cmd/automata/main.go` after DB connection is established.

---

## 3. Domain Layer Updates

### 3.1 `internal/domain/context/user.go`

Add to `User` struct:
```go
type User struct {
    ID           string
    Email        string
    PasswordHash string   // empty for SSO users
    SSOProvider  *string  // nil for local auth
    SSOID        *string  // nil for local auth
    IsAdmin      bool
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

Add `UserContext` struct:
```go
type UserContext struct {
    ID        string
    UserID    string
    ContextID string
    Role      string // owner, admin, member
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### 3.2 `internal/domain/context/user_repository.go`

Add methods:
```go
type UserRepository interface {
    // Existing
    Create(u *User) error
    GetByEmail(ctx context.Context, contextID, email string) (*User, error)
    GetByID(ctx context.Context, id string) (*User, error)
    Update(u *User) error

    // New
    GetByUsername(ctx context.Context, email string) (*User, error)          // platform-wide lookup
    GetByUserIDContext(ctx context.Context, userID, contextID string) (*UserContext, error)
    ListByUser(ctx context.Context, userID string) ([]UserContext, error)
    CreateMembership(ctx context.Context, uc *UserContext) error
    UpdateMembership(ctx context.Context, uc *UserContext) error
    DeleteMembership(ctx context.Context, userID, contextID string) error
    ListMembers(ctx context.Context, contextID string) ([]UserContext, error)
    ExistsAnyUser(ctx context.Context) (bool, error)  // for bootstrap check
}
```

### 3.3 `internal/domain/auth/auth.go` (interface)

Update `AuthService` interface:
```go
type AuthService interface {
    Login(email, password string) (token string, userID string, contexts []ContextInfo, err error)
    Logout(token string) error
    HashPassword(password string) (string, error)
    VerifyToken(token string) (*jwt.Token, error)
    CreateAPIKey(userID, contextID, name, plaintextKey string, expiresAt *time.Time) (*APIKey, error)
    GetCurrentUser(ctx context.Context) (*User, error)  // new
}

type ContextInfo struct {
    ID   string
    Name string
    Slug string
    Role string // owner, admin, member
}
```

---

## 4. Infrastructure Layer

### 4.1 `internal/infrastructure/context/repo/user_postgres.go`

Add implementations for all new repository methods:
- `GetByUsername` — query `users` table directly (no WHERE context_id)
- `ExistsAnyUser` — `SELECT EXISTS(SELECT 1 FROM users LIMIT 1)`
- `GetByUserIDContext` — query `user_contexts` with user_id + context_id
- `ListByUser`, `CreateMembership`, `UpdateMembership`, `DeleteMembership`, `ListMembers`

### 4.2 `internal/infrastructure/context/repo/user_context_postgres.go` (new file)

Dedicated file for user_contexts repository with the same interface methods, focused on membership operations.

### 4.3 `internal/infrastructure/auth/service.go`

Update `Login`:
- Query `users` table for email (not `context_users`)
- Verify password via bcrypt
- Query `user_contexts` for all memberships
- Build JWT with `is_admin` claim
- Return list of contexts with roles

---

## 5. API Layer

### 5.1 New routes in `internal/adapter/api/server.go`

Under `admin` group (requires auth middleware):

```go
// User management
usersRoutes := admin.Group("/users")
{
    usersRoutes.GET("", userHandler.List)       // list all platform users
    usersRoutes.POST("", userHandler.Create)    // create user
    usersRoutes.GET("/:id", userHandler.Get)    // get user
    usersRoutes.PUT("/:id", userHandler.Update) // update user
    usersRoutes.DELETE("/:id", userHandler.Delete) // delete user
}

// Context memberships under /admin/contexts/:id/users
contextUserRoutes := admin.Group("/contexts/:id/users")
{
    contextUserRoutes.GET("", contextUserHandler.List)
    contextUserRoutes.POST("", contextUserHandler.Create)
    contextUserRoutes.DELETE("/:userId", contextUserHandler.Delete)
}
```

Under `api` group (no admin restriction for /auth endpoints):

```go
// Auth endpoints
api.GET("/auth/me", authHandler.GetMe)        // current user + their contexts
```

### 5.2 `internal/adapter/api/handler/user_handler.go` (new file)

```go
type UserHandler struct {
    userRepo    context.UserRepository
    contextRepo context.Repository
}

func (h *UserHandler) List(c *gin.Context)      // GET /admin/users — admin only
func (h *UserHandler) Create(c *gin.Context)    // POST /admin/users
func (h *UserHandler) Get(c *gin.Context)       // GET /admin/users/:id
func (h *UserHandler) Update(c *gin.Context)    // PUT /admin/users/:id
func (h *UserHandler) Delete(c *gin.Context)    // DELETE /admin/users/:id
```

### 5.3 `internal/adapter/api/handler/context_user_handler.go` (new file)

```go
type ContextUserHandler struct {
    userRepo    context.UserRepository
    contextRepo context.Repository
}

func (h *ContextUserHandler) List(c *gin.Context)    // GET /admin/contexts/:id/users
func (ContextUserHandler) Create(c *gin.Context)    // POST /admin/contexts/:id/users
func (ContextUserHandler) Delete(c *gin.Context)    // DELETE /admin/contexts/:id/users/:userId
```

### 5.4 `internal/adapter/api/handler/auth_handler.go` — update

- `Login` response already returns `contexts` array. Add `role` to each context.
- Add `GetMe` handler for `GET /auth/me`.

---

## 6. Frontend / Middleware

### 6.1 Context key renaming

No changes needed — middleware already uses `c.Set("context_id", ...)` and `c.Get("context_id")`.

### 6.2 Auth middleware

`AuthMiddleware` in `internal/adapter/api/middleware/` — update to work with platform-level users:
- Extract `user_id` from JWT
- Query `users` table (not `context_users`)
- Set `c.Set("user_id", userID)` and `c.Set("user_email", email)`
- Set `c.Set("is_admin", isAdmin)` from JWT claim

---

## 7. Test Updates

### 7.1 Migration tests
- Add test for `migration_user_contexts.sql` idempotency

### 7.2 `cicd/global_login_test.go`
- Update to create users in `users` table instead of `context_users`
- Update login flow to verify new schema

### 7.3 New tests
- `cicd/user_handler_test.go` — user CRUD
- `cicd/context_user_handler_test.go` — membership management
- `cicd/bootstrap_test.go` — admin bootstrap

### 7.4 Existing test fixes
- Update any remaining `tenantID` variable names to `contextID` in test files
- Update any hardcoded queries referencing old table names

---

## 8. Config Changes

### `config.example.yaml`

```yaml
auth:
  secret_key: change-this-in-production
  admin_email: admin@automata.local
```

No new config keys needed — admin email already exists.

---

## Files Summary

| New files |
|-----------|
| `internal/infrastructure/database/migration_user_contexts.sql` |
| `internal/infrastructure/bootstrap/admin.go` |
| `internal/adapter/api/handler/user_handler.go` |
| `internal/adapter/api/handler/context_user_handler.go` |

| Modified files |
|----------------|
| `internal/domain/context/user.go` |
| `internal/domain/context/user_repository.go` |
| `internal/domain/auth/auth.go` |
| `internal/infrastructure/context/repo/user_postgres.go` |
| `internal/infrastructure/auth/service.go` |
| `internal/adapter/api/handler/auth_handler.go` |
| `internal/adapter/api/server.go` |
| `internal/adapter/api/middleware/auth.go` |
| `cmd/automata/main.go` |
| `config.example.yaml` (minor) |
| Test files (multiple) |
