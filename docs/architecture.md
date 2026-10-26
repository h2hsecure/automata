# Architecture

## Tech Stack

- **Language:** Go
- **Database:** PostgreSQL
- **Logging:** zerolog
- **Testing:** Ginkgo + testcontainers

## Architecture Style

Hexagonal (ports and adapters) with Clean Architecture principles. KISS and DRY applied throughout.

### Layer Structure

```
internal/
  domain/<feature>     — entities, value objects, domain interfaces (ports)
  usecase/<feature>    — application use cases, orchestration
internal/adapter/      — driver adapters (inbound)
  api/                 — HTTP REST API server
  mcp/                 — MCP protocol servers (one per feature domain)
  proxy/               — reverse proxy router
internal/infrastructure/ — driving adapters (outbound)
  <feature>/repo       — database/repository implementations
  queue/               — internal job queues (email, webhooks)
  config/              — key-value configuration store
  storage/             — file storage (local/S3)
pkg/                   — shared, domain-agnostic utilities
```

### Driver Adapters

Four separate driver adapters handle different traffic patterns:

| Adapter | Location | Purpose |
|---------|----------|---------|
| API Server | `internal/adapter/api` | Admin dashboard API, tenant management, CRUD operations |
| MCP Servers | `internal/adapter/mcp` | One MCP server per feature domain. Exposes tools for AI integration. |
| Tracking | `internal/adapter/tracking` | High-throughput event ingestion endpoint. Independent from API server. |
| BFF / WebUI | `internal/adapter/webui` | Backend-for-frontend: serves the built Web UI and WebUI API handlers. Runs as a separate binary (`cmd/bff`) on port 8082, proxying to the API server. |

All adapters share tenant resolution middleware and auth.

### BFF (Backend-for-Frontend)

The `bff` binary (`cmd/bff`) serves the static Web UI (built by the Dockerfile into `/root/web`) plus WebUI handlers, and proxies API calls to the main API server (`bff.api_backend_url`). It shares the same database and auth secrets as the API server, so session cookies and JWTs are compatible across both.

Configuration keys (see `config.example.yaml`):
- `bff.port` — HTTP port (default `8082`)
- `bff.api_backend_url` — URL of the API server (default `http://localhost:8080`)
- `bff.web_dir` — Web UI build directory; empty falls back to `<exe_dir>/web`

In `docker-compose.yml`, the `bff` service uses the same image as `automata` with `command: ["./bff"]` and `AUTOMATA_BFF_API_BACKEND_URL=http://automata:8080`.

### Configuration

Key-value configuration store. Backed by YAML file at startup, overridden by environment variables.

```
config.Get("server.host")
config.Set("database.ssl_mode", "require")
```

Nested keys supported: `server.host`, `database.pool.size`, `mcp.port`.

### Email Queue

Internal DB-backed job queue for sending emails. Background worker processes jobs with retry logic. Used by Form Management for submission notifications.

### Multi-Tenancy

One Automata instance manages multiple websites (tenants). Each tenant has:
- Isolated data (all tables include `tenant_id`)
- Independent configuration (key-value store scoped to tenant)
- Own contacts, forms, tracking data, banners
- Subdomain-based (`tenant.example.com`) or path-based (`example.com/tenant`) routing

### Testing

- **Unit/Integration:** Ginkgo + testcontainers for PostgreSQL
- **E2E:** API and MCP layer tests in `cicd/` directory
- **Mocks:** Mail and IMAP mocks for email/IMAP integration validation
