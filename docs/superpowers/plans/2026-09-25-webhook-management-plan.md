# Webhook Management — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add admin-only REST API for full CRUD of webhook endpoints, HMAC payload signing, URL validation, event whitelisting, and test-delivery capability to the Automata platform.

**Architecture:** New `webhook` domain package with Repository interface, bcrypt-hashed secret management in infrastructure service layer, PostgreSQL implementation in infrastructure layer, HTTP handlers in adapter layer. Reuses the existing `queue.WebhookQueue` for delivery enqueueing. No changes to the existing worker.

**Tech Stack:** Go 1.26.2, Gin v1.12.0, PostgreSQL (pgx v5), zerolog

**Spec:** `docs/superpowers/specs/2026-09-25-webhook-management-design.md`

## Global Constraints

- Module path: `github.com/hekemen/automata`
- Go version: `1.26.2`
- All UUIDs must be valid UUID format strings (use `gen_random_uuid()`)
- Config loading: `config.Get("key")` from `internal/infrastructure/config/config.go`
- Database migrations: use `CREATE TABLE IF NOT EXISTS` and `ON CONFLICT` for idempotency
- Hexagonal architecture: domain interfaces in `internal/domain/`, infrastructure in `internal/infrastructure/`, adapters in `internal/adapter/`
- All new files go in existing package directories

---

### Task 1: Domain Layer — WebhookEndpoint struct and Repository interface

**Files:**
- Create: `internal/domain/webhook/webhook_endpoint.go`
- Create: `internal/domain/webhook/repository.go`

**Interfaces:**
- Produces: `Repository` interface from `domain/webhook` package

Steps:
1. **Create `internal/domain/webhook/webhook_endpoint.go`**
   - Define `WebhookEndpoint` struct:
     ```go
     type WebhookEndpoint struct {
         ID         string
         ContextID  string
         Name       string
         URL        string
         Events     []string
         SecretHash *string  // nil if no secret configured
         Active     bool
         CreatedAt  time.Time
         UpdatedAt  time.Time
     }
     ```
   - Define request structs:
     ```go
     type CreateWebhookInput struct {
         ContextID string   `json:"context_id" binding:"required,uuid"`
         Name      string   `json:"name" binding:"required,min=1,max=128"`
         URL       string   `json:"url" binding:"required"`
         Events    []string `json:"events" binding:"required,min=1"`
         Secret    string   `json:"secret"`
         Active    *bool    `json:"active"`
     }
     ```
     ```go
     type UpdateWebhookInput struct {
         Name      string   `json:"name"`
         URL       string   `json:"url"`
         Events    []string `json:"events"`
         Secret    string   `json:"secret"`
         Active    *bool    `json:"active"`
     }
     ```
   - Define response struct:
     ```go
     type WebhookResponse struct {
         ID         string   `json:"id"`
         Name       string   `json:"name"`
         URL        string   `json:"url"`
         Events     []string `json:"events"`
         SecretSet  bool     `json:"secret_set"`
         Active     bool     `json:"active"`
         ContextID  string   `json:"context_id"`
         CreatedAt  time.Time `json:"created_at"`
         UpdatedAt  time.Time `json:"updated_at,omitempty"`
     }
     ```
     ```go
     type WebhookListResponse struct {
         Webhooks []*WebhookResponse `json:"webhooks"`
         Total    int                `json:"total"`
     }
     ```
   - Define allowed events whitelist constant:
     ```go
     var AllowedEvents = map[string]bool{
         "contact.created":   true,
         "contact.updated":   true,
         "contact.deleted":   true,
         "form.submitted":    true,
         "banner.impressed":  true,
         "banner.clicked":    true,
     }
     ```
   - Define `ValidateCreate` method:
     - Validate URL format: `^https://|^http://(localhost|127\.0\.0\.1)`
     - Validate each event in Events is in `AllowedEvents`
     - Validate Secret length: if provided, must be 8-256 characters
   - Define `ValidateUpdate` method (all fields optional, only validate if provided)

2. **Create `internal/domain/webhook/repository.go`**
   ```go
   type Repository interface {
       Create(e *WebhookEndpoint) error
       GetByID(id string) (*WebhookEndpoint, error)
       ListByContext(contextID string) ([]*WebhookEndpoint, error)
       Update(e *WebhookEndpoint) error
       Delete(id string) error
       ListActiveByContextAndEvent(contextID, event string) ([]*WebhookEndpoint, error)
   }
   ```
   - Add `ErrNotFound = fmt.Errorf("webhook endpoint not found")`

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 2: Infrastructure — PostgreSQL Repository, Migration, and Service

**Files:**
- Create: `internal/infrastructure/webhook/repo/migration.sql`
- Create: `internal/infrastructure/webhook/repo/migration.go`
- Create: `internal/infrastructure/webhook/repo/postgres.go`
- Create: `internal/infrastructure/webhook/service.go`

**Interfaces:**
- Consumes: `Repository` from `domain/webhook`
- Produces: `webhookRepo` implementing `Repository`
- Produces: `Service` struct with business logic (bcrypt hashing, HMAC signing)

Steps:
1. **Create `internal/infrastructure/webhook/repo/migration.sql`**
   ```sql
   -- Webhook endpoints table
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

2. **Create `internal/infrastructure/webhook/repo/migration.go`**
   ```go
   package repo
   ```
   - Same pattern as `contact/migration.go`:
     ```go
     import (
         "context"
         "embed"
         "fmt"
         "github.com/jackc/pgx/v5/pgxpool"
     )

     //go:embed migration.sql
     var migrationSQL embed.FS

     func RunMigrations(pool *pgxpool.Pool) error {
         migrationBytes, err := migrationSQL.ReadFile("migration.sql")
         if err != nil {
             return fmt.Errorf("read migration.sql: %w", err)
         }
         _, err = pool.Exec(context.Background(), string(migrationBytes))
         if err != nil {
             return fmt.Errorf("execute migration: %w", err)
         }
         return nil
     }
     ```

3. **Create `internal/infrastructure/webhook/repo/postgres.go`**
   ```go
   package repo
   ```
   - Import: `context`, `database/sql`, `fmt`, `strings`, `time`, `github.com/google/uuid`, `github.com/jackc/pgx/v5/pgxpool`, `github.com/hekemen/automata/internal/domain/webhook`, `github.com/rs/zerolog/log`
   - Implement `New(pool *pgxpool.Pool) Repository` — returns `*webhookRepo`
   - Implement all `Repository` methods:
     ```go
     type webhookRepo struct {
         pool *pgxpool.Pool
     }
     ```
   - `Create`: INSERT webhook_endpoint, storing events as comma-separated string
     ```sql
     INSERT INTO webhook_endpoints (id, context_id, name, url, events, secret_hash, active, created_at, updated_at)
     VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
     ```
     Events stored as: `strings.Join(events, ",")`
   - `GetByID`: `SELECT * FROM webhook_endpoints WHERE id = $1`
     - Parse events: `strings.Split(events, ",")` (filter out empty strings)
   - `ListByContext`: `SELECT * FROM webhook_endpoints WHERE context_id = $1 ORDER BY name ASC`
   - `Update`: Dynamic UPDATE — only update fields that are non-zero/non-empty
     ```sql
     UPDATE webhook_endpoints SET
         name = COALESCE(COALESCE($2, name), name),
         url = COALESCE(COALESCE($3, url), url),
         events = COALESCE(COALESCE($4, events), events),
         secret_hash = COALESCE($5, secret_hash),
         active = COALESCE($6, active),
         updated_at = NOW()
     WHERE id = $1
     ```
     (Actually, better to build the query dynamically based on which fields are provided, similar to the email handler pattern)
   - `Delete`: `DELETE FROM webhook_endpoints WHERE id = $1`
   - `ListActiveByContextAndEvent`: Query all active endpoints, then filter in Go by event match:
     ```sql
     SELECT * FROM webhook_endpoints WHERE context_id = $1 AND active = true
     ```
     Then filter results where event is in the endpoint's events list (parsed from comma-separated TEXT).

4. **Create `internal/infrastructure/webhook/service.go`**
   ```go
   package webhook

   import (
       "context"
       "crypto/hmac"
       "crypto/sha256"
       "encoding/hex"
       "encoding/json"
       "fmt"
       "time"

       "github.com/google/uuid"
       "github.com/jackc/pgx/v5/pgxpool"
       "github.com/hekemen/automata/internal/domain/webhook"
       "github.com/rs/zerolog/log"
       "golang.org/x/crypto/bcrypt"
   )

   type Service struct {
       repo       Repository
       queue      queue.WebhookQueue
       dbPool     *pgxpool.Pool
   }

   func NewService(repo Repository, q queue.WebhookQueue, pool *pgxpool.Pool) *Service {
       return &Service{repo: repo, queue: q, dbPool: pool}
   }
   ```
   - **`CreateEndpoint(ctx context.Context, input webhook.CreateWebhookInput) (*webhook.WebhookEndpoint, error)`**
     - Validate input using `input.ValidateCreate()`
     - Hash secret with bcrypt if provided: `bcrypt.GenerateFromPassword([]byte(input.Secret), bcrypt.DefaultCost)`
     - Create `WebhookEndpoint` with hashed secret
     - Call `repo.Create(endpoint)`
     - Return endpoint with `SecretHash` set (for internal use)
   - **`UpdateEndpoint(ctx context.Context, id string, input webhook.UpdateWebhookInput) (*webhook.WebhookEndpoint, error)`**
     - Get existing endpoint, return 404 if not found
     - Validate input using `input.ValidateUpdate()`
     - Hash new secret if provided, otherwise keep existing
     - Update non-zero fields on endpoint
     - Call `repo.Update(endpoint)`
   - **`SignPayload(endpoint *webhook.WebhookEndpoint, payload map[string]interface{}) (map[string]interface{}, error)`**
     - If `endpoint.SecretHash == nil`, return payload unchanged (or error depending on spec)
     - Marshal payload to JSON
     - Compute HMAC-SHA256: `hmac.New(sha256.New, []byte(secret))`
     - Wait — we can't un-hmac bcrypt. The secret_hash is bcrypt, so we need to store the plaintext secret separately or use a different approach.
     - **Correction:** Per the spec (Section 6.2), secrets are bcrypt-hashed and cannot be retrieved. For signing, we need the plaintext secret. The spec acknowledges this trade-off. For v1, we'll add a separate column `secret_encrypted` (AES-GCM with a master key from config) OR we'll store the plaintext secret in an encrypted form.
     - **Revised approach for v1:** Store the plaintext secret in a separate column `signing_secret` encrypted with AES-GCM using a master key from config (`config.Get("webhook.master_key")`). Use bcrypt only for a separate `secret_hash` column that is used for verification display only.
     - Actually, the simpler v1 approach per the spec: The `secret_hash` column is bcrypt, but for signing we need the raw secret. The spec (6.2) acknowledges this. **Decision:** Store the raw secret in a separate `signing_secret` column encrypted with a master key, OR use a simpler approach: store the raw secret in `secret_hash` but hash it for API responses. Let me re-read the spec.
     - The spec says `secret_hash` is bcrypt-hashed and **never retrieved**. For signing, the spec's `SignPayload` takes the endpoint and computes HMAC. This means we need the raw secret at signing time. The spec's design is internally inconsistent for v1.
     - **Resolution for implementation:** Use a two-column approach:
       - `secret_hash` (TEXT) — bcrypt hash for display/verification
       - `signing_secret` (TEXT, encrypted) — for HMAC signing
       - For v1 simplicity, skip encryption on `signing_secret` but document this as a v2 improvement. Use `config.Get("webhook.master_key")` as a simple XOR or AES-GCM key if provided, otherwise store plaintext (dev mode).
     - **Final v1 decision:** Store plaintext secret in `secret_hash` column. Add a `secret_encrypted` column only if a master key is configured. For now, the `SignPayload` method will read from a new `signing_secret` column that stores the raw secret (to be encrypted in v2).
     - Actually, the cleanest v1 approach: Use `secret_hash` to store the raw secret (not bcrypt-hashed) for signing purposes, and add a `secret_display_hash` column for bcrypt. No — this is overly complex.
     - **Simpler approach matching the spec:** The spec says `secret_hash` is bcrypt. For signing, the caller (e.g., form handler) stores the raw secret and passes it to `SignPayload` alongside the endpoint. But the spec's `SignPayload(endpoint, payload)` doesn't take the raw secret.
     - **Resolution:** Change the table to use a single `signing_secret` column (TEXT) that stores the raw secret for HMAC signing. Remove `secret_hash`. The API response includes `secret_set: true/false` boolean. This is simpler and functionally correct. If bcrypt hashing is required later, it can be added as a separate column.
     - **Revised final approach:** Add `signing_secret TEXT` column (stores raw secret, optionally encrypted). Use `secret_set BOOLEAN` derived from `signing_secret IS NOT NULL`. The `secret_hash` column from the spec is removed in favor of simpler storage.
   - **Revised `SignPayload(endpoint *webhook.WebhookEndpoint, payload map[string]interface{})`**
     - If `endpoint.SigningSecret == ""`, return error
     - Marshal payload to JSON bytes
     - Compute HMAC:
       ```go
       mac := hmac.New(sha256.New, []byte(endpoint.SigningSecret))
       mac.Write(payloadBytes)
       signature := mac.Sum(nil)
       payload["signature"] = hex.EncodeToString(signature)
       return payload, nil
       ```
   - **`DeliverTest(ctx context.Context, endpointID string) (deliveryID string, err error)`**
     - Get endpoint by ID via `repo.GetByID(endpointID)`
     - Build sample payload:
       ```json
       {
           "event": "test.ping",
           "context_id": "<endpoint.ContextID>",
           "data": {"test": true, "note": "webhook test delivery from Automata"},
           "timestamp": "<now>",
           "signature": "<HMAC-SHA256 of payload>"
       }
       ```
     - Sign the payload using `SignPayload` (returns error if no secret)
     - Enqueue via `queue.Enqueue(endpoint.URL, signedPayload, endpoint.ContextID, "")`
     - Return the delivery ID

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 3: HTTP Handler — Webhook endpoints

**Files:**
- Create: `internal/adapter/api/handler/webhook_handler.go`

**Interfaces:**
- Consumes: `webhook.Service` from `infrastructure/webhook`
- Produces: `WebhookHandler` struct with Gin handler methods

Steps:
1. **Create `internal/adapter/api/handler/webhook_handler.go`**
   ```go
   type WebhookHandler struct {
       service *webhook.Service
       log     *zerolog.Logger
   }

   func NewWebhookHandler(svc *webhook.Service) *WebhookHandler {
       return &WebhookHandler{
           service: svc,
           log: zerolog.New(os.Stdout).With().Str("handler", "webhook").Logger(),
       }
   }
   ```
   - **`List(c *gin.Context)`**
     - Extract optional `context_id` query param
     - If `context_id` present, call `service.repo.ListByContext(contextID)`
     - If absent, call `service.repo.ListAll()` (for super-admin) — or call `ListByContext` for all known contexts
     - Build `WebhookListResponse` with `secret_set` boolean for each endpoint
     - Return `200`
   - **`Create(c *gin.Context)`**
     - Bind JSON to `CreateWebhookInput`
     - Call `service.CreateEndpoint(ctx, input)`
     - Return `201` with `WebhookResponse`
     - Handle validation errors → `400`
   - **`Get(c *gin.Context)`**
     - Extract `:id` param
     - Call `service.repo.GetByID(id)`
     - Return `WebhookResponse` or `404`
   - **`Update(c *gin.Context)`**
     - Extract `:id` param
     - Bind `UpdateWebhookInput`
     - Call `service.UpdateEndpoint(ctx, id, input)`
     - Return `200` with `WebhookResponse` or `404`
   - **`Delete(c *gin.Context)`**
     - Extract `:id` param
     - Call `service.repo.Delete(id)`
     - Return `200` with `{"message": "webhook endpoint deleted"}` or `404`
   - **`TestDeliver(c *gin.Context)`**
     - Extract `:id` param
     - Call `service.DeliverTest(ctx, id)`
     - Return `200` with `{"delivery_id": id, "message": "test delivery enqueued"}`
     - Return `400` if no signing secret configured
     - Return `404` if endpoint not found

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 4: Route Registration in Server

**Files:**
- Modify: `internal/adapter/api/server.go`

**Steps:**
1. Add imports for `webhook_service`, `webhook_repo`, `webhookhandler`
2. In the `admin` group (after email-templates routes, before `}`):
   ```go
   webhookRepo := webhook_repo.New(pool)
   webhookQueue := queue.NewWebhookQueue(pool)
   webhookService := webhook_service.NewService(webhookRepo, webhookQueue, pool)
   webhookHandler := webhookhandler.NewWebhookHandler(webhookService)

   webhookRoutes := admin.Group("/webhooks")
   {
       webhookRoutes.GET("", webhookHandler.List)
       webhookRoutes.POST("", webhookHandler.Create)
       webhookRoutes.GET("/:id", webhookHandler.Get)
       webhookRoutes.PUT("/:id", webhookHandler.Update)
       webhookRoutes.DELETE("/:id", webhookHandler.Delete)
       webhookRoutes.POST("/:id/test", webhookHandler.TestDeliver)
   }
   ```

**Verification:**
- `go build ./...` compiles clean

---

### Task 5: Migration Registration

**Files:**
- Modify: `cmd/automata/main.go`
- Modify: `cmd/bff/main.go`

**Steps:**
1. **In `cmd/automata/main.go`:**
   - Add import: `webhook_repo "github.com/hekemen/automata/internal/infrastructure/webhook/repo"`
   - After `banner_repo.RunMigrations(pool)`, add:
     ```go
     if err := webhook_repo.RunMigrations(pool); err != nil {
         log.Fatal().Err(err).Msg("failed to run webhook migrations")
     }
     ```
2. **In `cmd/bff/main.go`:**
   - Add import: `webhook_repo "github.com/hekemen/automata/internal/infrastructure/webhook/repo"`
   - After `tracking_repo.RunMigrations(dbPool)`, add:
     ```go
     _ = webhook_repo.RunMigrations(dbPool)
     ```

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 6: Integration Tests

**Files:**
- Create: `cicd/webhook_endpoint_test.go`

**Interfaces:**
- Uses: `cicd/support/` test infrastructure (TestDB, Ginkgo suite)

Steps:
1. **Create `cicd/webhook_endpoint_test.go`**
   - Suite setup: use existing Ginkgo `BeforeSuite` pattern
   - **Describe("Webhook CRUD")**:
     - `It("creates a webhook with valid HTTPS URL")` — 201
     - `It("rejects HTTP URLs on non-localhost hosts")` — 400
     - `It("accepts localhost URLs for development")` — 201
     - `It("rejects invalid event names")` — 400
     - `It("accepts multiple events")` — 201
     - `It("gets a webhook by ID")` — 200
     - `It("updates webhook fields partially")` — 200
     - `It("toggles active flag")` — 200, subsequent list excludes inactive
     - `It("deletes a webhook")` — 200, subsequent Get returns 404
   - **Describe("Secret Management")**:
     - `It("stores secret hashed, never returns plaintext")` — response has `secret_set: true` but no secret value
     - `It("updates secret by replacing hash")`
     - `It("removes secret when empty string provided")`
   - **Describe("Test Delivery")**:
     - `It("enqueues test payload with HMAC signature")` — delivery_id returned
     - `It("returns error when no signing secret configured")` — 400
   - **Describe("Event Filtering")**:
     - `It("returns only matching endpoints for a given event")`
     - `It("returns active endpoints only")`

**Verification:**
- `go test ./cicd/...` passes (requires Docker for testcontainers)

---

### File Summary

| File | Action |
|------|--------|
| `internal/domain/webhook/webhook_endpoint.go` | **Create** — domain model, request/response structs, validation |
| `internal/domain/webhook/repository.go` | **Create** — Repository interface |
| `internal/infrastructure/webhook/repo/migration.sql` | **Create** — DDL for webhook_endpoints |
| `internal/infrastructure/webhook/repo/migration.go` | **Create** — RunMigrations |
| `internal/infrastructure/webhook/repo/postgres.go` | **Create** — PostgreSQL implementation |
| `internal/infrastructure/webhook/service.go` | **Create** — business logic (bcrypt, HMAC signing, test delivery) |
| `internal/adapter/api/handler/webhook_handler.go` | **Create** — HTTP handlers |
| `internal/adapter/api/server.go` | **Modify** — register webhook routes under `/admin/webhooks` |
| `cmd/automata/main.go` | **Modify** — run webhook migrations |
| `cmd/bff/main.go` | **Modify** — run webhook migrations |
| `cicd/webhook_endpoint_test.go` | **Create** — integration tests |
