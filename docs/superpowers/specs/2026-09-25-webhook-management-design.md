# Design Spec: Webhook Management

**Date:** 2026-09-25
**Status:** Draft
**Source:** PRD §17 Open Question 7 — Webhook management
**Module:** `github.com/hekemen/automata`

---

## 1. Problem Statement

The platform currently has a webhook delivery queue and worker (`webhook_queue.go`, `webhook_worker.go`) that can deliver payloads to URLs. However, there is no management surface for users or admins to **configure, discover, or administer webhook endpoints**.

This gap means:
- Webhook URLs and event subscriptions are hardcoded or managed externally.
- There is no way to list, create, update, or delete webhook endpoints from the admin UI.
- There is no mechanism for verifying that a configured endpoint is reachable and correctly receiving payloads.
- Secrets for HMAC payload signing are not managed.

This feature adds a full CRUD API (admin-only) and a test-delivery endpoint for webhook endpoints, along with the database schema and domain layer to persist and manage them.

---

## 2. Goals & Non-Goals

### Goals

1. Provide admin-only REST endpoints for full CRUD of webhook endpoints scoped to a context (tenant).
2. Persist webhook endpoints in a new `webhook_endpoints` table with HMAC signing secret (bcrypt-hashed).
3. Validate URLs (HTTPS or localhost), event names (whitelisted subset), and secret format on create/update.
4. Enrich the existing delivery pipeline: the queue worker reads the URL and secret from `webhook_endpoints` at enqueue time (or a new delivery-creation path computes the signed payload before enqueueing).
5. Provide a `POST /api/admin/webhooks/:id/test` endpoint that sends a sample payload to the configured URL and logs the result.
6. Ensure all operations are scoped per `context_id` — tenants can only manage their own webhook endpoints.

### Non-Goals (v1)

- Webhook delivery log / audit trail UI (though the `webhook_deliveries` table already tracks delivery attempts).
- Webhook retry dashboard or manual retry of individual deliveries.
- Event-driven auto-creation of deliveries from domain events — that is handled by existing callers (form handler, banner handler, etc.). This spec only adds the *management* layer.
- Webhook signature verification in the UI (the UI does not receive payloads; the remote endpoint does).
- Rate limiting or throttling of deliveries.
- Bulk operations (import/export, bulk enable/disable).
- Event delivery retry in the UI.

---

## 3. Architecture Overview

### 3.1 High-Level Data Flow

```
+-----------+        +------------------+        +-------------------+        +-------------+
| Admin UI  |------->|  API Server      |------->|  PostgreSQL       |--------| webhooks    |
| (Vue SPA) |  REST  |  /api/admin/     |  ORM   |  webhook_endpoints|  config| endpoints   |
+-----------+        +------------------+        +-------------------+        +-------------+
                                                                  |
                                                                  v
                                                          +---------------+
                                                          | Webhook Queue | (existing)
                                                          +---------------+
                                                                  |
                                                                  v
                                                          +---------------+
                                                          | Webhook Worker| (existing)
                                                          +---------------+
                                                                  |
                                                                  v
                                                          +---------------+
                                                          | Remote URL    |
                                                          +---------------+
```

### 3.2 Hexagonal Architecture Layers

The feature follows the existing hexagonal (ports & adapters) pattern used throughout the codebase:

```
internal/
├── domain/
│   └── webhook/                      [NEW] Domain layer
│       ├── webhook_endpoint.go       -- WebhookEndpoint model + validation
│       └── repository.go             -- Repository interface (ports)
│
├── infrastructure/
│   ├── webhook/                      [NEW] Infrastructure layer
│   │   ├── repo/
│   │   │   ├── postgres.go           -- PostgreSQL implementation
│   │   │   ├── migration.go          -- Migration runner
│   │   │   └── migration.sql         -- DDL
│   │   └── service.go                -- Business logic service (orchestrates repo + bcrypt)
│   │
│   └── queue/                        [EXISTING - no changes needed]
│       ├── webhook_queue.go          -- WebhookDelivery queue operations
│       └── webhook_worker.go         -- Background worker
│
└── adapter/
    └── api/
        ├── handler/
        │   └── webhook_handler.go    [NEW] HTTP handlers (gin.Context)
        └── server.go                 -- Extend with webhook routes
```

### 3.3 Integration with Existing Delivery Pipeline

**Current flow (existing):**
```
Event occurs (e.g., form submitted)
  → caller calls queue.Enqueue(url, payload, contextID, formID)
    (url is hardcoded or passed from a config/settings source)
  → WebhookDelivery inserted into webhook_deliveries table
  → WebhookWorker picks it up and POSTs to the URL
```

**New flow:**
```
Event occurs
  → WebhookService.ListActive(contextID, event) retrieves matching webhook_endpoints
  → For each match:
      → WebhookService.SignPayload(secretHash, payload) → computes HMAC-SHA256 signature
      → queue.Enqueue(url, signedPayload, contextID, formID)
        (same interface; the signed payload is already computed)
  → WebhookWorker POSTs the signed payload
```

The key change is that the **caller** (form handler, banner handler, etc.) queries `webhook_endpoints` at trigger time to discover which endpoints to fire, computes the HMAC signature, and enqueues per-endpoint deliveries. The existing `webhook_queue.go` interface (`Enqueue(url, payload, contextID, formID)`) is **unchanged** — the signing happens before the call.

### 3.4 Test Delivery Flow

```
POST /api/admin/webhooks/:id/test (Admin Auth)
  → WebhookService.GetByID(id) → fetches endpoint + context_id
  → WebhookService.SignPayload(secretHash, samplePayload) → HMAC
  → queue.Enqueue(endpoint.URL, signedSamplePayload, contextID, formID)
    → WebhookWorker delivers
  → Response returns delivery ID + HTTP status of test
```

---

## 4. Data Model

### 4.1 New Table: `webhook_endpoints`

```sql
CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id  UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    name        VARCHAR(128) NOT NULL,
    url         TEXT NOT NULL,
    events      TEXT NOT NULL DEFAULT '',
    secret_hash TEXT,
    active      BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_context ON webhook_endpoints(context_id);
CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_active ON webhook_endpoints(context_id, active) WHERE active = true;
```

**Column notes:**

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID | Primary key, auto-generated |
| `context_id` | UUID | FK to `contexts(id)`, cascade delete. Scopes webhooks to tenant |
| `name` | VARCHAR(128) | Human-readable label |
| `url` | TEXT | HTTPS or localhost only (validated in handler) |
| `events` | TEXT | Comma-separated event list stored as a single string (e.g., `"contact.created,form.submitted"`) |
| `secret_hash` | TEXT | bcrypt-hashed signing secret. NULL when no secret is set |
| `active` | BOOLEAN | Soft-delete toggle; inactive endpoints are excluded from delivery |
| `created_at` | TIMESTAMPTZ | Auto-set on creation |
| `updated_at` | TIMESTAMPTZ | Updated on each modification |

**Rationale for `events` as TEXT:**
- The events list is always queried as a whole (split at read time in Go), so storing as TEXT (comma-separated) is simpler than a separate junction table.
- This matches the existing convention of comma-delimited fields in the codebase (e.g., `to_addresses` in `email_jobs` uses TEXT[]).
- Indexing on individual events is not required for v1.

### 4.2 Migration Strategy

A new migration file at `internal/infrastructure/webhook/repo/migration.sql` following the existing pattern (`migration.sql` + `migration.go` with `RunMigrations()`). The migration is called from `cmd/automata/main.go` and `cmd/bff/main.go` alongside other feature migrations.

### 4.3 Domain Model

```go
// internal/domain/webhook/webhook_endpoint.go
package webhook

import (
    "time"
)

// WebhookEndpoint represents a configurable webhook endpoint.
type WebhookEndpoint struct {
    ID        string
    ContextID string
    Name      string
    URL       string
    Events    []string   // parsed from the stored comma-separated string
    SecretHash string    // bcrypt hash; nil if no secret
    Active    bool
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### 4.4 Repository Interface

```go
// internal/domain/webhook/repository.go
package webhook

// Repository defines the persistence interface for webhook endpoints.
type Repository interface {
    Create(e *WebhookEndpoint) error
    GetByID(id string) (*WebhookEndpoint, error)
    ListByContext(contextID string) ([]*WebhookEndpoint, error)
    Update(e *WebhookEndpoint) error
    Delete(id string) error
    ListActiveByContextAndEvent(contextID string, event string) ([]*WebhookEndpoint, error)
}
```

### 4.5 Service Layer

```go
// internal/infrastructure/webhook/service.go
package webhook

type Service struct {
    repo Repository
}

func NewService(repo Repository) *Service

// CreateEndpoint validates and creates a new endpoint.
func (s *Service) CreateEndpoint(ctx context.Context, input CreateInput) (*WebhookEndpoint, error)

// UpdateEndpoint validates and updates an endpoint.
func (s *Service) UpdateEndpoint(ctx context.Context, id string, input UpdateInput) (*WebhookEndpoint, error)

// SignPayload computes HMAC-SHA256 signature of the payload using the endpoint's secret.
func (s *Service) SignPayload(endpoint *WebhookEndpoint, payload map[string]interface{}) (map[string]interface{}, error)

// DeliverTest sends a test payload to the endpoint and returns the HTTP status.
func (s *Service) DeliverTest(ctx context.Context, endpointID string) (int, error)
```

---

## 5. API Contracts

All routes are registered under `/api/admin/webhooks` with `AuthMiddleware` (admin JWT or API key required).

### 5.1 List Webhook Endpoints

```
GET /api/admin/webhooks?context_id=<uuid>
```

**Auth:** Admin JWT / API key  
**Query params:**
- `context_id` (optional) — if omitted, returns all endpoints across all contexts (super-admin only)

**Response — 200 OK:**
```json
{
  "webhooks": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Production Webhook",
      "url": "https://hooks.example.com/automata",
      "events": ["contact.created", "form.submitted"],
      "secret_set": true,
      "active": true,
      "context_id": "7b3e2d1a-...",
      "created_at": "2026-09-25T10:00:00Z"
    }
  ],
  "total": 1
}
```

**Note:** The raw `secret` is never returned. `secret_set` is a boolean indicating whether a signing secret has been configured.

### 5.2 Create Webhook Endpoint

```
POST /api/admin/webhooks
Content-Type: application/json
{
  "context_id": "7b3e2d1a-...",
  "name": "Production Webhook",
  "url": "https://hooks.example.com/automata",
  "events": ["contact.created", "form.submitted"],
  "secret": "super-secret-signing-key"
}
```

**Auth:** Admin JWT / API key  
**Validation:**
- `context_id`: required, valid UUID
- `name`: required, 1–128 characters
- `url`: required, must match `^https://` or `^http://localhost` / `^http://127\.0\.0\.1`
- `events`: required, non-empty array; each element must be in the allowed set
- `secret`: optional; if provided, 8–256 characters, hashed with bcrypt before storage

**Allowed events (whitelist):**
| Event | Description |
|-------|-------------|
| `contact.created` | New contact added |
| `contact.updated` | Contact modified |
| `contact.deleted` | Contact removed |
| `form.submitted` | Form submission received |
| `banner.impressed` | Banner impression tracked |
| `banner.clicked` | Banner click tracked |

**Response — 201 Created:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Production Webhook",
  "url": "https://hooks.example.com/automata",
  "events": ["contact.created", "form.submitted"],
  "secret_set": true,
  "active": true,
  "context_id": "7b3e2d1a-...",
  "created_at": "2026-09-25T10:00:00Z"
}
```

### 5.3 Get Webhook Detail

```
GET /api/admin/webhooks/:id
```

**Auth:** Admin JWT / API key  
**Path params:**
- `id`: webhook endpoint UUID

**Response — 200 OK:**
Same structure as create response.

**Response — 404 Not Found:**
```json
{ "error": "webhook endpoint not found" }
```

### 5.4 Update Webhook Endpoint

```
PUT /api/admin/webhooks/:id
Content-Type: application/json
{
  "name": "Updated Name",
  "url": "https://hooks.example.com/new-endpoint",
  "events": ["contact.created"],
  "secret": "new-secret-key",
  "active": false
}
```

**Auth:** Admin JWT / API key  
**Validation:** Same as create (all fields optional; only provided fields are updated).

**Response — 200 OK:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Updated Name",
  "url": "https://hooks.example.com/new-endpoint",
  "events": ["contact.created"],
  "secret_set": true,
  "active": false,
  "context_id": "7b3e2d1a-...",
  "created_at": "2026-09-25T10:00:00Z",
  "updated_at": "2026-09-25T11:00:00Z"
}
```

### 5.5 Delete Webhook Endpoint

```
DELETE /api/admin/webhooks/:id
```

**Auth:** Admin JWT / API key  
**Path params:**
- `id`: webhook endpoint UUID

**Response — 200 OK:**
```json
{ "message": "webhook endpoint deleted" }
```

### 5.6 Test Webhook Delivery

```
POST /api/admin/webhooks/:id/test
```

**Auth:** Admin JWT / API key  
**Path params:**
- `id`: webhook endpoint UUID

**Behavior:**
1. Fetches the webhook endpoint by ID.
2. Constructs a sample payload:
   ```json
   {
     "event": "test.ping",
     "context_id": "<endpoint.context_id>",
     "data": { "test": true, "note": "webhook test delivery from Automata" },
     "timestamp": "2026-09-25T10:00:00Z",
     "signature": "<HMAC-SHA256 of above payload>"
   }
   ```
3. Signs the payload using the endpoint's secret (if `secret_hash` is set).
4. Enqueues the payload via the existing `queue.Enqueue()`.
5. Returns the delivery ID.

**Response — 200 OK:**
```json
{
  "delivery_id": "a1b2c3d4-...",
  "message": "test delivery enqueued"
}
```

**Response — 400 Bad Request (no secret configured):**
```json
{ "error": "webhook has no signing secret configured" }
```

---

## 6. Key Design Decisions

### 6.1 Admin-Only Access

Webhook management is **admin-only** (registered under the `/admin` group with `AuthMiddleware`). This is consistent with other admin resources (tenants, API keys, configs) and prevents tenant users from modifying webhook configurations. The `context_id` scope ensures a tenant admin can only manage their own endpoints.

### 6.2 Secret Hashing with bcrypt

Secrets are hashed with bcrypt before storage in the database. This ensures that:
- A database compromise does not immediately expose signing secrets.
- The secret can be verified (bcrypt compare) during payload signing but never read back.
- When updating the secret, the new value is hashed before replacing the old hash.

**Trade-off:** The plaintext secret is never available after initial creation, which means it cannot be displayed in the UI for reference. If display is needed later, a separate encrypted-field approach (AES-GCM with a master key) could be used instead. For v1, `secret_set: true/false` is sufficient.

### 6.3 URL Validation

Only HTTPS URLs or localhost/127.0.0.1 are accepted:
- **HTTPS** ensures production webhooks use TLS.
- **localhost/127.0.0.1** enables local development and testing.
- HTTP on other hosts is rejected to prevent accidental plaintext delivery.

Validation regex:
```go
var urlRegex = regexp.MustCompile(`^https://|^http://(localhost|127\.0\.0\.1)`)
```

### 6.4 Events as Comma-Separated TEXT

Events are stored as a single comma-separated TEXT column rather than a JSON array or junction table:
- **Simple:** no extra table or JOIN needed.
- **Compact:** single column, easy to query.
- **Split on read:** parsed in Go with `strings.Split(events, ",")`.
- **Indexed:** `idx_webhook_endpoints_active` supports the common query pattern (list active endpoints for a context).

**Trade-off:** Individual event queries require string splitting in Go rather than a SQL `ANY()` or junction table join. This is acceptable for v1 since the number of webhooks per context is expected to be small (< 50).

### 6.5 Delivery Enrichment Without Queue Changes

The existing `WebhookQueue.Enqueue(url, payload, contextID, formID)` interface is **not modified**. Instead, the enrichment happens at the call site:

1. When a domain event fires (e.g., form submitted), the caller calls `WebhookService.ListActiveByContextAndEvent(contextID, event)` to get matching endpoints.
2. For each endpoint, the service computes the signed payload and calls `queue.Enqueue()` individually.
3. This approach means the worker has no awareness of which events triggered the delivery — it just POSTs whatever payload it receives, which is the correct design (the worker is transport-agnostic).

### 6.6 Test Delivery Enqueues Rather Than Sends Inline

The test endpoint enqueues the payload through `queue.Enqueue()` rather than sending it directly. This:
- Uses the same code path as real deliveries (same signing, same HTTP client).
- Records the test delivery in `webhook_deliveries` for visibility.
- Avoids race conditions if the worker is mid-processing.
- The test response returns the `delivery_id` so the caller can track it.

### 6.7 Soft-Delete via `active` Flag

The `active` boolean acts as a soft-delete toggle rather than hard-deleting from the database:
- Inactive endpoints are excluded from `ListActiveByContextAndEvent()` queries.
- Hard-delete (`DELETE /api/admin/webhooks/:id`) still exists for permanent removal.
- This allows auditing and recovery if a webhook was accidentally deactivated.

### 6.8 On-Delete Cascade

`webhook_endpoints` uses `ON DELETE CASCADE` on `context_id`. When a context (tenant) is deleted, all its webhook endpoints are automatically removed, keeping the database consistent.

---

## 7. Implementation Checklist

### Domain Layer
- [ ] `internal/domain/webhook/webhook_endpoint.go` — WebhookEndpoint struct, validation methods
- [ ] `internal/domain/webhook/repository.go` — Repository interface

### Infrastructure Layer
- [ ] `internal/infrastructure/webhook/repo/migration.sql` — DDL
- [ ] `internal/infrastructure/webhook/repo/migration.go` — RunMigrations
- [ ] `internal/infrastructure/webhook/repo/postgres.go` — PostgreSQL implementation
- [ ] `internal/infrastructure/webhook/service.go` — Service with signing, validation, test delivery

### Adapter Layer
- [ ] `internal/adapter/api/handler/webhook_handler.go` — CRUD + test handlers
- [ ] `internal/adapter/api/server.go` — Register routes under `/api/admin/webhooks`

### Main Entry Points
- [ ] `cmd/automata/main.go` — Call `webhook_repo.RunMigrations(pool)`
- [ ] `cmd/bff/main.go` — Call `webhook_repo.RunMigrations(pool)` (if BFF serves webhook admin)

### Event Integration
- [ ] Form handler: call `WebhookService` on form submission
- [ ] Contact handler: call `WebhookService` on contact CRUD
- [ ] Banner handler: call `WebhookService` on impression/click

### Testing
- [ ] Unit tests for WebhookEndpoint validation
- [ ] Unit tests for SignPayload (HMAC-SHA256)
- [ ] Integration tests for Repository (CRUD operations)
- [ ] Integration tests for webhook_handler (using httptest)
- [ ] Integration test for test delivery endpoint

---

## 8. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Secret exposure in API responses | `secret` is never returned; only `secret_set: boolean` |
| Secret exposure in database | bcrypt-hashed storage |
| Unauthenticated webhook config | Admin-only middleware on all routes |
| Insecure URLs (HTTP) | Regex validation allows only HTTPS or localhost |
| Event injection | Whitelist validation on create/update |
| SQL injection | All queries use parameterized statements (`$1`, `$2`, etc.) |
| Context isolation | `context_id` foreign key + FK constraint; queries scoped by `context_id` |

---

## 9. File Reference

| File | Purpose |
|------|---------|
| `internal/domain/webhook/webhook_endpoint.go` | Domain model |
| `internal/domain/webhook/repository.go` | Repository port interface |
| `internal/infrastructure/webhook/repo/migration.sql` | DDL for `webhook_endpoints` |
| `internal/infrastructure/webhook/repo/migration.go` | Migration runner |
| `internal/infrastructure/webhook/repo/postgres.go` | PostgreSQL repository implementation |
| `internal/infrastructure/webhook/service.go` | Business logic service |
| `internal/adapter/api/handler/webhook_handler.go` | HTTP handlers |
| `internal/adapter/api/server.go` | Route registration (existing, extended) |
| `internal/infrastructure/queue/webhook_queue.go` | Existing delivery queue (unchanged) |
| `internal/infrastructure/queue/webhook_worker.go` | Existing delivery worker (unchanged) |
| `docs/superpowers/specs/2026-09-25-webhook-management-design.md` | This spec |
