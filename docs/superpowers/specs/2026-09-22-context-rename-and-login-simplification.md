# Design: Rename Tenant to Context + Simplify Login

## Overview

Automata is a marketing automation tool where each "context" represents a website that uses the platform to manage contacts, forms, banners, and emails. This design renames "tenant" to "context" throughout the codebase, simplifies the login flow by removing the tenant field, and restructures the user model from tenant-scoped accounts to platform-level users with context memberships.

## Problem Statement

The current terminology ("tenant") and user model create confusion:
- "Tenant" implies a SaaS multi-tenant architecture where the platform operator manages multiple isolated customers
- In reality, Automata is self-hosted by each website owner — each website is a "context" managed within a single instance
- Users are currently scoped to tenants (`tenant_users` table), meaning the same email can have different accounts in different tenants
- Login requires a tenant slug field (even though the backend ignores it), adding unnecessary friction
- No clear admin bootstrap path for first-time setup

## Goals

1. Rename "tenant" to "context" everywhere (code, API, docs, UI)
2. Simplify login to email+password only — no context/tenant field
3. Restructure users as platform-level accounts with context memberships
4. Provide automatic admin bootstrap on first startup
5. Support SSO login as a future extension point

## Non-Goals

- Multi-tenant SaaS hosting (platform operator managing multiple customers)
- Cross-context data sharing (each context remains fully isolated)
- Role-based access control beyond owner/admin/member per context
- SSO implementation (only the data model and endpoint changes)

## Architecture

### Data Model

#### Users (platform-level)

```sql
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(254) NOT NULL UNIQUE,
    password_hash   TEXT,                    -- NULL if SSO-only user
    sso_provider    VARCHAR(64),             -- null = local password auth
    sso_id          VARCHAR(512),            -- SSO provider user ID
    is_admin        BOOLEAN DEFAULT false,   -- platform admin
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);
```

- `password_hash` is NULL for SSO-only users
- `sso_provider` and `sso_id` are NULL for local password users
- `is_admin` grants platform-wide admin access (can manage all contexts and users)

#### Contexts (renamed from tenants)

```sql
CREATE TABLE contexts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    domain          VARCHAR(256),            -- optional custom domain
    is_active       BOOLEAN DEFAULT true,
    settings        JSONB DEFAULT '{}',      -- { branding, snippet_customization, ... }
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);
```

#### User-Context Memberships

```sql
CREATE TABLE user_contexts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    context_id      UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    role            VARCHAR(32) DEFAULT 'member',  -- owner, admin, member
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, context_id)
);
```

- `role` determines access level within a context
- `owner` can manage the context and its users
- `admin` can manage contacts, forms, banners, emails
- `member` has read access (configurable per context)

#### API Keys (per-context, unchanged structure)

```sql
CREATE TABLE api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id  UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash    VARCHAR(64) NOT NULL,
    name        TEXT NOT NULL,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_api_keys_context_name ON api_keys(context_id, name);
```

### Migration from Old Schema

The migration script handles:

1. **Create new tables** (`users`, `user_contexts`, `contexts`)
2. **Migrate contexts data**: `tenants` → `contexts` (copy all rows, rename table)
3. **Migrate users**: Each `tenant_users` row becomes a `users` row + a `user_contexts` membership
   - Same email across different tenants becomes separate user accounts (different `user_id`)
   - This is intentional — each context had its own user base
4. **Migrate API keys**: `tenant_id` → `context_id` in `api_keys`
5. **Drop old tables**: `tenant_users`, `tenants`, old `api_keys` constraints

### Authentication Flow

#### Login

```
POST /auth/login
Body: { "email": "user@example.com", "password": "..." }
Returns: {
    "token": "<JWT>",
    "user": { "id": "...", "email": "...", "is_admin": false },
    "contexts": [
        { "id": "...", "slug": "mywebsite", "name": "My Website", "role": "owner" }
    ]
}
```

- No `tenant` field in request body
- JWT claims: `{ user_id, user_email, is_admin, exp }` — no context claim
- Response includes all contexts the user has access to

#### Logout

```
POST /auth/logout
Headers: Authorization: Bearer <token>
```

Unchanged.

#### API Key Auth

```
Headers: Authorization: Bearer ak_<key>
```

Unchanged — validates key, resolves `context_id` from the key record.

### Context Resolution

Context is resolved per-request via middleware, same as before but renamed:

| Context | Method | Source |
|---------|--------|--------|
| Admin API | `X-Context-ID` header | Frontend sends selected context UUID |
| Admin API | Subdomain | `Host` header first segment |
| Tracking/Banners | `X-Context-ID` header | JS snippet or API caller |
| Tracking/Banners | Subdomain | `Host` header first segment |

The middleware stores resolved context in Gin context:
- `c.Set("context", *context.Context)` — full context object
- `c.Set("context_id", context.ID)` — UUID string

### Admin Bootstrap

On first startup (no admin user exists):

1. Generate random email: `admin@automata.local` (configurable via `AUTOMATA_ADMIN_EMAIL`)
2. Generate random password (32 chars, alphanumeric + special)
3. Hash password with bcrypt
4. Create admin user in `users` table (`is_admin = true`)
5. Log to stdout:
   ```
   ╔══════════════════════════════════════════╗
   ║  ADMIN CREDENTIALS (first startup only)  ║
   ║  Email:    admin@automata.local          ║
   ║  Password: <generated-password>          ║
   ╚══════════════════════════════════════════╝
   ```
6. Write a marker file `.admin_initialized` to prevent re-generation on subsequent startups
7. If the marker file exists but no admin user in DB (edge case), regenerate

### API Endpoints

#### Auth

| Method | Path | Description |
|--------|------|-------------|
| POST | `/auth/login` | Platform login (email+password) |
| POST | `/auth/logout` | Invalidate session |
| POST | `/auth/api-keys` | Create API key (requires context) |
| GET | `/auth/api-keys` | List API keys (requires context) |
| DELETE | `/auth/api-keys/:id` | Revoke API key (requires context) |

#### Context Management (admin only)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin/contexts` | List all contexts |
| POST | `/admin/contexts` | Create context |
| GET | `/admin/contexts/:id` | Get context details |
| PUT | `/admin/contexts/:id` | Update context |
| DELETE | `/admin/contexts/:id` | Delete context (cascades) |
| GET | `/admin/contexts/:id/users` | List context users |
| POST | `/admin/contexts/:id/users` | Add user to context |
| DELETE | `/admin/contexts/:id/users/:userId` | Remove user from context |

#### User Management (admin only)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin/users` | List all platform users |
| POST | `/admin/users` | Create user |
| GET | `/admin/users/:id` | Get user details |
| PUT | `/admin/users/:id` | Update user |
| DELETE | `/admin/users/:id` | Delete user |

#### Context Selector (user endpoints)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/auth/me` | Get current user + their contexts |
| GET | `/auth/contexts` | List contexts accessible to current user |

### Frontend Changes

#### Login View

- Remove the tenant slug field
- Keep email and password fields only
- On success, redirect to dashboard with context selector visible

#### Dashboard Layout

- Add context selector in header or sidebar
- Shows all contexts the user has access to
- Switching context updates `X-Context-ID` header for all subsequent API calls
- Current context displayed prominently

#### Auth Store (Pinia)

- Store `user` object (id, email, is_admin)
- Store `contexts` array (id, slug, name, role)
- Store `activeContextId` (selected context)
- Token callback includes `X-Context-ID` header from active context

#### API Client

- Update all references from `X-Tenant-ID` to `X-Context-ID`
- Update all endpoint paths from `/admin/tenants/*` to `/admin/contexts/*`
- Update response field names from `tenant_id` to `context_id`

### File Renames (Backend)

| Old Path | New Path |
|----------|----------|
| `internal/domain/tenant/` | `internal/domain/context/` |
| `internal/infrastructure/tenant/repo/postgres.go` | `internal/infrastructure/context/repo/postgres.go` |
| `internal/infrastructure/tenant/repo/user_postgres.go` | `internal/infrastructure/context/repo/user_context_postgres.go` |
| `internal/adapter/api/tenant_middleware.go` | `internal/adapter/api/context_middleware.go` |
| `internal/adapter/api/handler/tenant_handler.go` | `internal/adapter/api/handler/context_handler.go` |
| `internal/adapter/api/handler/tenant_user_handler.go` | `internal/adapter/api/handler/context_user_handler.go` |

### Configuration Changes

```yaml
# automata.yaml
server.host: 0.0.0.0
server.port: 8080
database.host: localhost
database.port: 5432
database.name: automata
database.user: automata
database.password: changeme
database.ssl_mode: disable
context.resolution: subdomain       # renamed from tenant.resolution
context.default_path: /context      # renamed from tenant.default_path
mcp.enabled: true
mcp.port: 9000
storage.type: local
storage.local_path: ./uploads
auth.secret_key: your-secret-key
auth.admin_email: admin@automata.local   # new: admin bootstrap email
```

## Error Handling

- **Login with wrong credentials**: Return 401 with generic message ("Invalid email or password")
- **User not in any context**: Return 200 with empty contexts array, show "No contexts assigned" message
- **Context not found**: Return 404
- **User not a member of context**: Return 403
- **Admin bootstrap failure**: Log error to stderr, refuse to start (DB connection issue)

## Testing Strategy

### Unit Tests

- Context resolver middleware (header, subdomain, path extraction)
- Auth service (login, token verification, API key validation)
- User-context membership checks

### Integration Tests

- Login flow (email+password, SSO placeholder)
- Context CRUD operations
- User-context membership management
- API key creation and validation per context
- Admin bootstrap (first startup)

### E2E Tests

- Full login → context selection → data access flow
- Context switching updates API headers correctly
- Admin creates context, assigns user, user logs in

## Success Criteria

1. Login page has no tenant/context field — just email and password
2. All API endpoints use `context` terminology (paths, headers, responses)
3. Users are platform-level with context memberships
4. Admin credentials are logged on first startup
5. All existing integration tests pass with updated terminology
6. Frontend context selector works correctly
