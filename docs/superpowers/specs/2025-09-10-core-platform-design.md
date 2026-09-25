# Design: Core Platform

## Problem Statement

Automata needs a foundation layer that handles authentication, multi-tenant isolation, configuration management, and background job processing. Without these primitives, no feature-level functionality can be built reliably.

## Goals

1. Provide a secure, multi-tenant authentication system with JWT tokens
2. Support configuration via YAML file with environment variable overrides
3. Isolate all feature data per tenant via `tenant_id` (now `context_id`) foreign keys
4. Provide a reverse proxy for routing tenant traffic
5. Expose MCP (Model Context Protocol) tools for AI integration
6. Implement a background email queue for transactional messages

## Non-Goals

- Email delivery (relies on external SMTP provider)
- Advanced RBAC beyond owner/admin/member per tenant
- SSO/SAML authentication
- Multi-region deployment

## Architecture

### Directory Structure

```
internal/
  domain/
    auth/          # Auth interfaces (AuthService, APIKeyService)
    config/        # Config domain types
  infrastructure/
    auth/          # JWT signing, bcrypt password hashing
    config/        # YAML loader + env var override
    database/      # PostgreSQL pool + embedded migration runner
    email/         # SMTP email sender with retry queue
  adapter/
    api/           # Gin HTTP handlers (auth, admin, proxy)
    proxy/         # Reverse proxy for tenant traffic
```

### Multi-Tenant Isolation

All feature tables include a `context_id` column with a foreign key to `contexts(id)`. Queries always filter by `context_id` to ensure strict data isolation. The context slug is extracted from the JWT token or URL path and mapped to a context ID.

### Authentication Flow

```
Client → POST /api/auth/login { email, password, context_slug }
        → AuthService.Login() → bcrypt comparison
        → JWT signed with HS256: { user_id, context_slug, is_owner, exp }
        → Returns { token, contexts: [{ id, slug, name }] }

Client → Authorization: Bearer <token>
        → AuthMiddleware.VerifyToken() → extracts user_id + context_slug
        → c.Set("user_id", userID), c.Set("context_id", contextID)
        → Downstream handlers read from context
```

### API Key System

API keys are bcrypt-hashed values stored in `api_keys` table. Each key is scoped to a single context. Keys can have optional TTL expiration. The plaintext key is returned only once at creation time.

### Configuration System

- YAML file loaded at startup (path: `config.yaml` or `$CONFIG_FILE`)
- Flattened to dot-notation: `auth.secret_key`, `database.host`
- Environment variables override YAML: `AUTOMATA_AUTH_SECRET_KEY`
- In-memory cache with mutex for concurrent reads

### Email Queue

- DB-backed retry table: `email_queue` (id, context_id, to, subject, body, status, retries, next_retry)
- Background worker polls every 30s for pending emails
- Exponential backoff: 1min, 5min, 30min
- Status transitions: `pending → sending → sent` or `pending → failed`

### Database Schema

```sql
CREATE TABLE contexts (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug      TEXT NOT NULL UNIQUE,
    name      TEXT NOT NULL,
    domain    TEXT,
    is_active BOOLEAN DEFAULT true,
    settings  JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE context_users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    email      VARCHAR(254) NOT NULL,
    password_hash TEXT NOT NULL,
    is_owner   BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(context_id, email)
);

CREATE TABLE api_keys (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL,
    key_hash   TEXT NOT NULL,
    name       VARCHAR(128) NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE admin_configs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    key        VARCHAR(128) NOT NULL,
    value      JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(context_id, key)
);

CREATE TABLE email_queue (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    to_addr    VARCHAR(254) NOT NULL,
    subject    TEXT NOT NULL,
    body       TEXT NOT NULL,
    status     VARCHAR(16) DEFAULT 'pending',
    retries    INTEGER DEFAULT 0,
    next_retry TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| JWT algorithm | HS256 | Simple, no external dependencies, sufficient for single-instance deployment |
| Password hashing | bcrypt with DefaultCost (10) | Battle-tested, Go standard library via golang.org/x/crypto |
| Config format | YAML + env override | Human-readable file + easy CI/CD override |
| Email queue | DB-backed with worker | No external message broker needed; simple retry logic |
| Multi-tenancy | context_id FK | Strict isolation; query layer enforces filtering |
