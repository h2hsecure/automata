# Email System — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add persistent email template storage, template rendering with Go text/template, admin CRUD endpoints, tenant-scoped send-email endpoint, and test-send capability to the Automata platform.

**Architecture:** New `email` domain package with Repository interface, PostgreSQL implementation in infrastructure layer, use case layer for rendering+send orchestration, HTTP handlers in adapter layer. Reuses the existing `queue.Queue` (EmailQueue) for enqueueing. Default templates are seeded on context creation via upsert in `RunMigrations`.

**Tech Stack:** Go 1.26.2, Gin v1.12.0, PostgreSQL (pgx v5), zerolog

**Spec:** `docs/superpowers/specs/2026-09-25-email-system-design.md`

## Global Constraints

- Module path: `github.com/hekemen/automata`
- Go version: `1.26.2`
- All UUIDs must be valid UUID format strings (use `gen_random_uuid()`)
- Config loading: `config.Get("key")` from `internal/infrastructure/config/config.go`
- Database migrations: use `CREATE TABLE IF NOT EXISTS` and `ON CONFLICT` for idempotency
- Hexagonal architecture: domain interfaces in `internal/domain/`, infrastructure in `internal/infrastructure/`, adapters in `internal/adapter/`
- All new files go in existing package directories

---

### Task 1: Domain Layer — EmailTemplate struct and Repository interface

**Files:**
- Create: `internal/domain/email/email.go`
- Create: `internal/domain/email/repository.go`

**Interfaces:**
- Produces: `Repository` interface from `domain/email` package

Steps:
1. **Create `internal/domain/email/email.go`**
   - Define `EmailTemplate` struct:
     ```go
     type EmailTemplate struct {
         ID        string
         ContextID string
         Key       string
         Subject   string
         BodyText  string
         BodyHTML  string
         IsDefault bool
         CreatedAt time.Time
         UpdatedAt time.Time
     }
     ```
   - Define request structs matching the API contract:
     ```go
     type CreateTemplateRequest struct {
         ContextID string `json:"context_id" binding:"required,uuid"`
         Key       string `json:"key" binding:"required,min=1,max=64"`
         Subject   string `json:"subject" binding:"required"`
         BodyText  string `json:"body_text"`
         BodyHTML  string `json:"body_html"`
         IsDefault bool  `json:"is_default"`
     }
     ```
     ```go
     type UpdateTemplateRequest struct {
         Subject   string `json:"subject"`
         BodyText  string `json:"body_text"`
         BodyHTML  string `json:"body_html"`
         IsDefault *bool  `json:"is_default"`
     }
     ```
     ```go
     type SendEmailRequest struct {
         To             string              `json:"to"`
         ToArray        []string            `json:"to_array"`
         Template       string              `json:"template" binding:"required"`
         Variables      map[string]string   `json:"variables"`
         SubjectOverride *string            `json:"subject_override"`
         BodyOverride    *string            `json:"body_override"`
         HTML           *bool               `json:"html"`
     }
     ```
     ```go
     type TestSendRequest struct {
         To        string            `json:"to" binding:"required,email"`
         Variables map[string]string `json:"variables"`
     }
     ```
   - Define response struct:
     ```go
     type TemplateResponse struct {
         ID        string    `json:"id"`
         ContextID string    `json:"context_id"`
         Key       string    `json:"key"`
         Subject   string    `json:"subject"`
         BodyText  string    `json:"body_text"`
         BodyHTML  string    `json:"body_html"`
         IsDefault bool      `json:"is_default"`
         CreatedAt time.Time `json:"created_at"`
         UpdatedAt time.Time `json:"updated_at"`
     }
     ```
   - Define `TemplateListResponse` with `Templates` slice and `Count`
   - Define `SendEmailResponse` with `JobID` field
   - Define `TestSendResponse` with `JobID`, `RenderedSubject`, `RenderedBodyPreview`
   - Define `ValidateKey` function to enforce alphanumeric + underscore pattern
   - Define `ValidateCreateRequest` helper to check at least body_text or body_html is non-empty

2. **Create `internal/domain/email/repository.go`**
   ```go
   type Repository interface {
       Create(t *EmailTemplate) error
       GetByID(id string) (*EmailTemplate, error)
       GetByKey(contextID, key string) (*EmailTemplate, error)
       List(contextID string) ([]*EmailTemplate, error)
       Update(t *EmailTemplate) error
       Delete(id string) error
       SeedDefaults(contextID string) error
   }
   ```
   - Add `ErrNotFound = fmt.Errorf("email template not found")`
   - Add `ErrDuplicateKey = fmt.Errorf("template key already exists for this context")`

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 2: Infrastructure — PostgreSQL Repository and Migration

**Files:**
- Create: `internal/infrastructure/email/repo/migration.sql`
- Create: `internal/infrastructure/email/repo/postgres.go`

**Interfaces:**
- Consumes: `Repository` from `domain/email`
- Produces: `emailRepo` struct implementing `Repository`

Steps:
1. **Create `internal/infrastructure/email/repo/migration.sql`**
   ```sql
   -- Email templates table
   CREATE TABLE IF NOT EXISTS email_templates (
       id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
       context_id  UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
       key         VARCHAR(64) NOT NULL,
       subject     TEXT NOT NULL DEFAULT '',
       body_text   TEXT NOT NULL DEFAULT '',
       body_html   TEXT NOT NULL DEFAULT '',
       is_default  BOOLEAN DEFAULT false,
       created_at  TIMESTAMPTZ DEFAULT NOW(),
       updated_at  TIMESTAMPTZ DEFAULT NOW(),

       CONSTRAINT uq_email_templates_context_key UNIQUE (context_id, key)
   );

   CREATE INDEX IF NOT EXISTS idx_email_templates_context ON email_templates(context_id);
   ```

2. **Create `internal/infrastructure/email/repo/postgres.go`**
   - Import: `context`, `database/sql`, `fmt`, `time`, `github.com/google/uuid`, `github.com/jackc/pgx/v5/pgxpool`, `github.com/hekemen/automata/internal/domain/email`, `github.com/rs/zerolog/log`
   - Add `//go:embed migration.sql` directive
   - Implement `RunMigrations(pool *pgxpool.Pool) error` — read embedded SQL, execute
   - Implement `New(pool *pgxpool.Pool) Repository` — returns `*emailRepo`
   - Implement all `Repository` methods:
     ```go
     type emailRepo struct {
         pool *pgxpool.Pool
     }
     ```
   - `Create`: INSERT with `ON CONFLICT (context_id, key) DO NOTHING`. Check if row was actually inserted; if `rowsAffected == 0`, return `ErrDuplicateKey`
   - `GetByID`: `SELECT * FROM email_templates WHERE id = $1`
   - `GetByKey`: `SELECT * FROM email_templates WHERE context_id = $1 AND key = $2`
   - `List`: `SELECT * FROM email_templates WHERE context_id = $1 ORDER BY key ASC`
   - `Update`: `UPDATE email_templates SET subject=$2, body_text=$3, body_html=$4, is_default=$5, updated_at=NOW() WHERE id=$1` (use dynamic query — only update non-empty fields)
   - `Delete`: `DELETE FROM email_templates WHERE id = $1`
   - `SeedDefaults`: INSERT the 5 default templates with `ON CONFLICT (context_id, key) DO NOTHING`:
     ```go
     var defaultTemplates = []struct {
         key, subject, bodyText, bodyHTML string
     }{
         {
             key: "welcome",
             subject: "Welcome to {{ .ContextName }}",
             bodyText: "Welcome to {{ .ContextName }}!\n\nHi {{ .ContactName }}, welcome aboard!\n\nLogin URL: {{ .LoginURL }}",
             bodyHTML: "<h1>Welcome to {{ .ContextName }}</h1><p>Hi <strong>{{ .ContactName }}</strong>, welcome aboard!</p><p>Login URL: <a href=\"{{ .LoginURL }}\">{{ .LoginURL }}</a></p>",
         },
         {
             key: "contact_created",
             subject: "New contact: {{ .ContactName }}",
             bodyText: "A new contact has been created:\nName: {{ .ContactName }}\nEmail: {{ .ContactEmail }}\nSource: {{ .Source }}",
             bodyHTML: "<h2>New contact created</h2><p><strong>{{ .ContactName }}</strong> ({{ .ContactEmail }}) via {{ .Source }}</p>",
         },
         {
             key: "form_submission",
             subject: "New form submission: {{ .FormName }}",
             bodyText: "A new form has been submitted:\nForm: {{ .FormName }}\n{{ .SubmissionData }}",
             bodyHTML: "<h2>New form submission</h2><p><strong>{{ .FormName }}</strong></p><pre>{{ .SubmissionData }}</pre>",
         },
         {
             key: "password_reset",
             subject: "Reset your password",
             bodyText: "Hi {{ .ContactName }},\n\nClick the link below to reset your password:\n{{ .ResetURL }}\n\nThis link expires in {{ .Expiry }}.",
             bodyHTML: "<h2>Password Reset</h2><p>Hi <strong>{{ .ContactName }}</strong>,</p><p><a href=\"{{ .ResetURL }}\">Reset your password</a></p><p>This link expires in {{ .Expiry }}.</p>",
         },
         {
             key: "notification",
             subject: "{{ .Subject }}",
             bodyText: "{{ .Body }}",
             bodyHTML: "<h3>{{ .Subject }}</h3><p>{{ .Body }}</p>{{ if .CTA }}<p><a href=\"{{ .CTALink }}\">{{ .CTAText }}</a></p>{{ end }}",
         },
     }
     ```
     Query per template:
     ```sql
     INSERT INTO email_templates (id, context_id, key, subject, body_text, body_html, is_default, created_at, updated_at)
     VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
     ON CONFLICT (context_id, key) DO NOTHING
     ```

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues
- `go test ./internal/domain/email/...` — placeholder tests

---

### Task 3: Use Case Layer — Template rendering and send orchestration

**Files:**
- Create: `internal/usecase/email/email.go`

**Interfaces:**
- Consumes: `email.Repository` from `domain/email`
- Consumes: `queue.Queue` from `infrastructure/queue` (the existing `EmailQueue`)
- Produces: `EmailUsecase` struct with render/send methods

Steps:
1. **Create `internal/usecase/email/email.go`**
   ```go
   type EmailUsecase struct {
       repo email.Repository
       queue queue.Queue
       log *zerolog.Logger
   }

   func NewEmailUsecase(repo email.Repository, q queue.Queue) *EmailUsecase {
       return &EmailUsecase{
           repo: repo,
           queue: q,
           log: zerolog.New(os.Stdout).With().Str("service", "email_usecase").Logger(),
       }
   }
   ```
   - **`RenderTemplate(contextID, templateKey, variables map[string]string) (subject, body, htmlBody string, err error)`**
     - Call `repo.GetByKey(contextID, templateKey)`
     - Parse template with `template.New("email").Parse(subject)` and `template.New("body").Parse(bodyText)`
     - Execute templates with `variables` as the data map
     - Return rendered subject and body; if body rendering fails, return error with undefined variable info
     - Handle `body_override` case: if override is set, use it as both body and htmlBody
   - **`SendEmail(contextID string, req domain.SendEmailRequest) (jobID string, err error)`**
     - Normalize `To` field (accept single string or array)
     - Resolve template via `RenderTemplate`
     - Apply `subject_override` and `body_override` if set
     - Choose `body_text` or `body_html` based on `req.HTML` (default: `false`)
     - Build `queue.EmailJob`:
       ```go
       job := &queue.EmailJob{
           ContextID: contextID,
           To:        toAddresses,
           Subject:   renderedSubject,
           Body:      renderedBody,
           HTMLBody:  renderedHTML,
           MaxRetries: 3,
       }
       ```
     - Call `q.Enqueue(job)`, return the job's ID
   - **`TestSend(req domain.TestSendRequest) (jobID, renderedSubject, preview string, err error)`**
     - Validate `req.To` email format (reuse existing `isValidEmail` logic)
     - Resolve template: `repo.GetByKey(contextID, req.Template)` — contextID comes from gin.Context via middleware
     - Render subject and body_text with `req.Variables`
     - Create preview (first 200 chars of body)
     - Enqueue job with `To: []string{req.To}`
     - Return jobID, renderedSubject, preview

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 4: HTTP Handler — Email endpoints

**Files:**
- Create: `internal/adapter/api/handler/email_handler.go`

**Interfaces:**
- Consumes: `EmailUsecase` from `usecase/email`
- Produces: `EmailHandler` struct with Gin handler methods

Steps:
1. **Create `internal/adapter/api/handler/email_handler.go`**
   ```go
   type EmailHandler struct {
       usecase *emailusecase.EmailUsecase
       log     *zerolog.Logger
   }

   func NewEmailHandler(uc *emailusecase.EmailUsecase) *EmailHandler {
       return &EmailHandler{
           usecase: uc,
           log: zerolog.New(os.Stdout).With().Str("handler", "email").Logger(),
       }
   }
   ```
   - **`List(c *gin.Context)`**
     - Extract `context_id` from query param
     - Validate UUID format
     - Call `uc.Repo.List(contextID)`
     - Return `{ "templates": [...], "count": N }`
   - **`Create(c *gin.Context)`**
     - Bind JSON to `CreateTemplateRequest`
     - Validate `key` with `ValidateKey` (alphanumeric + underscore only)
     - Check at least `body_text` or `body_html` non-empty
     - Call `uc.Repo.Create(template)`
     - Return `201` with created template
     - Handle `ErrDuplicateKey` → `409`
   - **`Get(c *gin.Context)`**
     - Extract `:id` param
     - Call `uc.Repo.GetByID(id)`
     - Return template or `404`
   - **`Update(c *gin.Context)`**
     - Extract `:id` param, get existing template
     - Bind JSON to `UpdateTemplateRequest` (all fields optional)
     - Apply only non-zero/non-empty fields
     - Call `uc.Repo.Update(template)`
     - Return updated template or `404`
   - **`Delete(c *gin.Context)`**
     - Extract `:id` param
     - Call `uc.Repo.Delete(id)`
     - Return `200` with `{"message": "template deleted"}` or `404`
   - **`TestSend(c *gin.Context)`**
     - Extract `:id` param, get template to resolve `context_id`
     - Bind `TestSendRequest`
     - Call `uc.TestSend(req)` (usecase needs `contextID` from context or template)
     - Return jobID, renderedSubject, preview

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 5: Route Registration in Server

**Files:**
- Modify: `internal/adapter/api/server.go`

**Steps:**
1. Add imports for `emailusecase`, `email_repo`, `emailhandler`
2. In the `admin` group (after the config routes section, before `}`):
   ```go
   emailRepo := email_repo.New(pool)
   emailQueue := queue.NewEmailQueue(pool)
   emailUsecase := emailusecase.NewEmailUsecase(emailRepo, emailQueue)
   emailHandler := emailhandler.NewEmailHandler(emailUsecase)

   emailRoutes := admin.Group("/email-templates")
   {
       emailRoutes.GET("", emailHandler.List)
       emailRoutes.POST("", emailHandler.Create)
       emailRoutes.GET("/:id", emailHandler.Get)
       emailRoutes.PUT("/:id", emailHandler.Update)
       emailRoutes.DELETE("/:id", emailHandler.Delete)
       emailRoutes.POST("/:id/test", emailHandler.TestSend)
   }
   ```
3. After the `api.Use(ContextResolver(contextRepo))` line, add tenant-scoped send endpoint:
   ```go
   tenantEmailGroup := api.Group("/tenant/:slug/email/send")
   tenantEmailGroup.Use(ContextResolver(contextRepo)) // ensure context resolved
   tenantEmailGroup.POST("", emailHandler.Send)
   ```

**Verification:**
- `go build ./...` compiles clean

---

### Task 6: Migration Registration + Default Template Seeding

**Files:**
- Modify: `cmd/automata/main.go`
- Modify: `cmd/bff/main.go`

**Steps:**
1. **In `cmd/automata/main.go`:**
   - Add import: `email_repo "github.com/hekemen/automata/internal/infrastructure/email/repo"`
   - After `banner_repo.RunMigrations(pool)`, add:
     ```go
     if err := email_repo.RunMigrations(pool); err != nil {
         log.Fatal().Err(err).Msg("failed to run email migrations")
     }
     ```
   - After the `bootstrap.Run(pool, ...)` call, add seeding for all existing contexts:
     ```go
     // Seed default email templates for all existing contexts
     contexts, _ := contextRepo.List(0, 1000)
     for _, ctx := range contexts {
         _ = email_repo.SeedDefaultsForContext(pool, ctx.ID)
     }
     ```
2. **In `cmd/bff/main.go`:**
   - Add import: `email_repo "github.com/hekemen/automata/internal/infrastructure/email/repo"`
   - After `tracking_repo.RunMigrations(dbPool)`, add:
     ```go
     _ = email_repo.RunMigrations(dbPool)
     ```

**Note on `SeedDefaults`:** The `RunMigrations` approach seeds templates per context. Since `RunMigrations` runs on every startup (idempotent), we need a separate `SeedDefaultsForContext(pool, contextID string) error` function that can be called from `main.go` after context resolution. This function calls `repo.SeedDefaults(contextID)` internally.

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 7: Integration Tests

**Files:**
- Create: `cicd/email_template_test.go`

**Interfaces:**
- Uses: `cicd/support/` test infrastructure (TestDB, Ginkgo suite)

Steps:
1. **Create `cicd/email_template_test.go`**
   - Suite setup: use existing Ginkgo `BeforeSuite` pattern from `cicd/suite_test.go`
   - **Describe("Email Template CRUD")**:
     - `It("creates a template with valid request")` — CreateTemplateRequest → 201
     - `It("rejects duplicate key for same context")` — 409
     - `It("gets a template by ID")` — 200
     - `It("updates a template with partial fields")` — 200, only updated fields changed
     - `It("deletes a template")` — 200, subsequent Get returns 404
     - `It("lists templates filtered by context")` — returns only matching
   - **Describe("Template Rendering")**:
     - `It("renders Go text/template variables correctly")` — verify interpolated values
     - `It("returns error for undefined variables")` — 400
     - `It("renders HTML and plain text variants")` — both body_text and body_html render
   - **Describe("Send Email")**:
     - `It("enqueues email job after rendering")` — verify email_jobs table has new row
     - `It("accepts single string or array for To field")` — both formats work
     - `It("applies subject_override and body_override")` — overrides are used instead of template
   - **Describe("Test Send")**:
     - `It("renders template and enqueues for single address")` — 200 with job_id
     - `It("returns rendered subject preview")`
   - **Describe("Default Template Seeding")**:
     - `It("seeds 5 default templates on first call")`
     - `It("skips seeding on repeated call (idempotent)")`

**Verification:**
- `go test ./cicd/...` passes (requires Docker for testcontainers)

---

### File Summary

| File | Action |
|------|--------|
| `internal/domain/email/email.go` | **Create** — domain types, request/response structs |
| `internal/domain/email/repository.go` | **Create** — Repository interface |
| `internal/infrastructure/email/repo/postgres.go` | **Create** — PostgreSQL implementation + SeedDefaults |
| `internal/infrastructure/email/repo/migration.sql` | **Create** — DDL for email_templates |
| `internal/usecase/email/email.go` | **Create** — business logic (render, send, test) |
| `internal/adapter/api/handler/email_handler.go` | **Create** — HTTP handlers |
| `internal/adapter/api/server.go` | **Modify** — register email routes (admin + tenant) |
| `cmd/automata/main.go` | **Modify** — run migrations + seed defaults |
| `cmd/bff/main.go` | **Modify** — run email migrations |
| `cicd/email_template_test.go` | **Create** — integration tests |
