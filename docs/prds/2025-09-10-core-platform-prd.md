# PRD: Core Platform

## Overview

The foundation for Automata: multi-tenant authentication, reverse proxy, MCP server, and shared infrastructure. Target user: small-to-mid business owners who self-host Automata on their own server.

## Goals

- Provide a secure, multi-tenant authentication system for tenant admins
- Route tenant traffic via subdomain or path-based reverse proxy
- Expose all feature modules via MCP protocol for AI tool integration
- Support self-hosted deployment as a single binary or Docker container

## Scope

### In Scope

- Multi-tenant auth: tenant registration, admin user management, session-based auth, API keys
- Tenant resolution: subdomain-based (`tenant.example.com`) and path-based (`example.com/tenant`)
- Reverse proxy: routes tenant traffic to tenant-specific content (forms, snippets, static assets)
- MCP server: exposes tools for contacts, forms, submissions, tracking, and banners
- Admin dashboard: tenant management, user management, system configuration
- Backend API only — no UI in Phase 1
- Shared infrastructure: logging (zerolog), key-value config store, health checks
- Internal email queue: background worker processes email send jobs from a DB-backed queue

### Out of Scope

- SaaS hosting / managed service (same codebase, different deployment config)
- Custom domain management per tenant (Phase 2+)
- OAuth / SSO for tenant users (deferred)
- Rate limiting / throttling (deferred)
- Audit logging (deferred)

## Architecture

Clean architecture with hexagonal layout. KISS and DRY principles.

### Layer Structure

```
internal/
  domain/          — domain entities, value objects, domain interfaces (ports)
  usecase/         — application use cases, orchestration logic
internal/adapter/  — driver adapters (inbound)
  api/             — HTTP API server (Gin or similar)
  mcp/             — MCP protocol server(s), one per feature domain
  proxy/           — reverse proxy router
internal/infrastructure/ — driving adapters (outbound)
  queue/           — internal email queue mechanism
  config/          — key-value configuration store
  storage/         — file storage adapter (local/S3)
pkg/               — shared utilities, domain-agnostic packages
```

### Driver Adapters

- **API Server** (`internal/adapter/api`): HTTP REST API for admin dashboard and programmatic access. Handles auth, tenant resolution, and routes requests to use cases.
- **MCP Servers** (`internal/adapter/mcp`): Separate MCP server per feature domain (contacts, forms, tracking, banners). Each server exposes tools for its domain. Decoupled from API server — can run independently or together.
- **Reverse Proxy** (`internal/adapter/proxy`): Routes tenant traffic to tenant-specific content (forms, snippets, static assets).

### Data Flow

```
Request → Tenant resolver → Auth middleware → Use case → Domain service → DB/Storage
AI Tool → MCP server → Use case → Domain service → DB/Storage
```

### Key Decisions

- **Subdomain-based tenant resolution:** Primary method. `tenant.example.com` resolves to the tenant by extracting the subdomain. Requires wildcard DNS or CNAME setup.
- **Path-based fallback:** Secondary method. `example.com/tenant/<slug>` used when subdomain resolution isn't available (e.g., localhost, non-wildcard setups).
- **PostgreSQL with tenant_id:** All feature tables include a `tenant_id` column. Simpler than schema-per-tenant for self-hosted deployments.
- **MCP servers as separate adapters:** Each feature domain has its own MCP server. Shared infrastructure (auth, tenant resolution) is factored into a base MCP server.
- **Key-value configuration:** System and tenant configuration stored as key-value pairs. Supports nested keys (e.g., `server.host`, `database.ssl_mode`). Loaded at startup, reloaded on change.
- **Self-hosted deployment:** Single Go binary or Docker container. Configuration via key-value store backed by YAML file or environment variables.

## API Design

### Auth

```
POST /auth/login
Body: { "email": "admin@example.com", "password": "..." }
Returns: { "token": "...", "tenant_id": "..." }

POST /auth/logout
Headers: Authorization: Bearer <token>

POST /auth/api-keys                    # Generate API key
GET    /auth/api-keys                  # List API keys
DELETE /auth/api-keys/<id>            # Revoke API key
```

### Tenant Management (admin only)

```
GET    /admin/tenants                  # List tenants
POST   /admin/tenants                  # Create tenant
GET    /admin/tenants/<id>             # Get tenant details
PUT    /admin/tenants/<id>             # Update tenant
DELETE /admin/tenants/<id>             # Delete tenant (cascades)
GET    /admin/tenants/<id>/users       # List tenant users
POST   /admin/tenants/<id>/users       # Add user to tenant
```

### MCP Tools (via MCP protocol)

```
# Contact tools
contacts.list(tenant_id, filters)
contacts.get(tenant_id, contact_id)
contacts.create(tenant_id, data)
contacts.update(tenant_id, contact_id, data)
contacts.delete(tenant_id, contact_id)
contacts.merge(tenant_id, contact_id_a, contact_id_b)
contacts.import(tenant_id, csv_file, mapping)
contacts.export(tenant_id, filters)

# Form tools
forms.list(tenant_id)
forms.get(tenant_id, form_id)
forms.create(tenant_id, data)
forms.update(tenant_id, form_id, data)
forms.delete(tenant_id, form_id)
forms.submit(form_slug, data)
forms.list_submissions(tenant_id, form_id)

# Tracking tools
tracking.get_dashboard(tenant_id, date_range)
tracking.get_events(tenant_id, filters)
tracking.track(tenant_id, event_data)

# Banner tools
banners.list(tenant_id)
banners.get(tenant_id, banner_id)
banners.create(tenant_id, data)
banners.update(tenant_id, banner_id, data)
banners.delete(tenant_id, banner_id)
banners.set_placements(tenant_id, banner_id, placements)
banners.list_variants(tenant_id, banner_id)
banners.create_variant(tenant_id, banner_id, data)
```

## Database Schema (tentative)

```sql
-- Tenants table
CREATE TABLE tenants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    domain          VARCHAR(256),        -- optional custom domain
    is_active       BOOLEAN DEFAULT true,
    settings        JSONB DEFAULT '{}',  -- { branding, snippet_customization, ... }
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Tenant users (admin users per tenant)
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

-- API keys
CREATE TABLE api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES tenant_users(id) ON DELETE CASCADE,
    key_hash    VARCHAR(64) NOT NULL,    -- hashed, never stored raw
    name        TEXT NOT NULL,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_api_keys_tenant_name ON api_keys(tenant_id, name);
```

## Configuration

Key-value configuration store. Backed by YAML file at startup, environment variables override file values.

```yaml
# automata.yaml
server.host: 0.0.0.0
server.port: 8080
server.tls.enabled: false
server.tls.cert: /path/to/cert.pem
server.tls.key: /path/to/key.pem
database.host: localhost
database.port: 5432
database.name: automata
database.user: automata
database.password: changeme
database.ssl_mode: disable
tenant.resolution: subdomain
tenant.default_path: /tenant
mcp.enabled: true
mcp.port: 9000
storage.type: local
storage.local_path: ./uploads
storage.s3.bucket: automata-uploads
storage.s3.region: us-east-1
```

Access via `config.Get("server.host")`, `config.Set("server.host", "0.0.0.0")`.

## Testing

- Unit tests for tenant resolution (subdomain and path extraction)
- Integration tests for auth flow (login, token validation, API key auth)
- Proxy routing tests (verify correct tenant content served per subdomain/path)
- MCP tool tests (verify each tool returns correct data, respects tenant isolation)
- E2E tests for API and MCP layers using Ginkgo + testcontainers, executed via `cicd/` directory
- Mail and IMAP mocks for email queue integration validation
- Load test: 100 concurrent auth requests, 50 concurrent proxy requests

## Success Criteria

- Tenant resolution works for both subdomain and path-based routing
- Auth response time < 100ms (excluding password hashing)
- MCP tools respond within 500ms for standard queries
- Single binary deployment works on Linux (amd64, arm64)
- Docker image < 200MB (alpine-based)
