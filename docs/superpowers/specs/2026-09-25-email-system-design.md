# Email System Design Spec

**Version:** 1.0
**Date:** 2026-09-25
**Status:** Draft
**Related PRD:** Sections 7.6 (Email), 14 (Email Template System)

---

## 1. Problem Statement

The Automata platform needs a templated email system to support automated communications such as welcome emails, contact notifications, form submission alerts, and password resets. The current codebase has:

- An **email queue worker** (`internal/infrastructure/queue/email_queue.go`) that processes jobs from a `email_jobs` table with retry logic and exponential backoff.
- No **template storage** — templates are hardcoded or absent.
- No **send email API endpoint** — the PRD marks `POST /api/tenant/:slug/email/send` as "to be implemented".
- No **admin email template management** — the PRD's Question #1 asks whether to add a `POST /admin/email-templates` endpoint.

This spec defines the missing infrastructure to make email sending a first-class feature: persistent template storage, template rendering with Go text/template syntax, a send-email API, template management endpoints, and a test-send capability.

---

## 2. Goals & Non-Goals

### Goals

1. **Store email templates persistently** in PostgreSQL, scoped to each tenant (context).
2. **Support Go text/template syntax** for subject and body with variable interpolation.
3. **Provide admin CRUD endpoints** for email templates (list, get, create, update, delete).
4. **Provide a tenant-scoped send email endpoint** that resolves a template, interpolates variables, and enqueues the result to the existing `email_jobs` queue.
5. **Provide a test-send endpoint** that validates template rendering and sends to a single address.
6. **Ship with 5 default templates** seeded on first startup.

### Non-Goals (v1)

- WYSIWYG template editor — plain text / HTML textareas only.
- Template versioning / history.
- Email delivery tracking (open rates, click-through) — out of scope for v1.
- BCC / CC / reply-to headers.
- HTML body sanitization or template sandboxing beyond standard Go template execution.
- Template sharing across tenants.
- Email scheduling (send-at future time) — enqueues immediately.

---

## 3. Architecture Overview

```
┌──────────────────────────────────────────────────────────────┐
│  HTTP Request                                                │
│  POST /api/admin/email-templates/:id/test                    │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               v
┌──────────────────────────────────────────────────────────────┐
│  email_handler.go (adapter/api/handler)                      │
│  - Validates request body                                    │
│  - Extracts context_id from header                           │
│  - Calls EmailUsecase                                        │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               v
┌──────────────────────────────────────────────────────────────┐
│  email.go (usecase/email)                                    │
│  - Resolves template by key                                  │
│  - Validates & renders Go text/template                      │
│  - Builds EmailJob (subject, body, html_body, to)            │
│  - Delegates to queue.Queue.Enqueue()                        │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               v
┌──────────────────────────────────────────────────────────────┐
│  email_queue.go (infrastructure/queue)                       │
│  - INSERT into email_jobs table                              │
│  - Background worker polls and sends                         │
└──────────────────────────────────────────────────────────────┘
```

### Layer Placement

| Layer | Location | Purpose |
|-------|----------|---------|
| Domain model | `internal/domain/email/` | EmailTemplate entity, request structs, Repository interface |
| Use case | `internal/usecase/email/` | Template CRUD, render + send orchestration |
| Repository | `internal/infrastructure/email/repo/` | PostgreSQL CRUD for email_templates table |
| Handler | `internal/adapter/api/handler/email_handler.go` | New file — HTTP endpoints |
| Migration | `internal/infrastructure/email/repo/migration.sql` | New file — CREATE TABLE + seed data |
| Bootstrap | Seed default templates | In `RunMigrations` or a dedicated bootstrap step |

The send-email flow reuses `queue.Queue` (the existing `emailQueue` struct) — the handler or usecase simply constructs an `EmailJob` and calls `Enqueue`.

### Route Structure

```
/admin/email-templates          → Admin-only (is_admin=true)
  GET                           → List all templates for admin's contexts
  POST                          → Create template
  GET /:id                      → Get template
  PUT /:id                      → Update template
  DELETE /:id                   → Delete template
  POST /:id/test                → Test send template

/api/tenant/:slug/email/send    → Tenant-scoped (context-resolved)
  POST                          → Resolve template + variables → enqueue
```

---

## 4. Data Model

### 4.1 Table: `email_templates`

```sql
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

**Design notes:**
- `context_id` (reusing the existing `contexts` table — the PRD calls them "tenants") scopes templates per tenant.
- `key` is a human-readable identifier (e.g., `welcome`, `contact_created`). Unique per context.
- `is_default` marks a template as the system default for its context; only one default per key allowed per context.
- `body_text` and `body_html` are separate columns — the send endpoint chooses which to use based on a flag.
- `ON DELETE CASCADE` ensures templates are cleaned up when their context is deleted.

### 4.2 Default Templates to Seed

Seeded once per context on first template creation (or via bootstrap). Stored with the `key` values:

| key | subject | body_text (plain) | body_html (html) |
|-----|---------|-------------------|------------------|
| `welcome` | `Welcome to {{ .ContextName }}` | Welcome to {{ .ContextName }}!\n\nHi {{ .ContactName }}, welcome aboard!\n\nLogin URL: {{ .LoginURL }} | `<h1>Welcome to {{ .ContextName }}</h1><p>Hi <strong>{{ .ContactName }}</strong>, welcome aboard!</p><p>Login URL: <a href="{{ .LoginURL }}">{{ .LoginURL }}</a></p>` |
| `contact_created` | `New contact: {{ .ContactName }}` | A new contact has been created:\nName: {{ .ContactName }}\nEmail: {{ .ContactEmail }}\nSource: {{ .Source }} | `<h2>New contact created</h2><p><strong>{{ .ContactName }}</strong> ({{ .ContactEmail }}) via {{ .Source }}</p>` |
| `form_submission` | `New form submission: {{ .FormName }}` | A new form has been submitted:\nForm: {{ .FormName }}\n{{ .SubmissionData }} | `<h2>New form submission</h2><p><strong>{{ .FormName }}</strong></p><pre>{{ .SubmissionData }}</pre>` |
| `password_reset` | `Reset your password` | Hi {{ .ContactName }},\n\nClick the link below to reset your password:\n{{ .ResetURL }}\n\nThis link expires in {{ .Expiry }}. | `<h2>Password Reset</h2><p>Hi <strong>{{ .ContactName }}</strong>,</p><p><a href="{{ .ResetURL }}">Reset your password</a></p><p>This link expires in {{ .Expiry }}.</p>` |
| `notification` | `{{ .Subject }}` | {{ .Body }} | `<h3>{{ .Subject }}</h3><p>{{ .Body }}</p>{{ if .CTA }}<p><a href="{{ .CTALink }}">{{ .CTAText }}</a></p>{{ end }}` |

### 4.3 Seed Data Strategy

**Option A (recommended):** Seed templates in `RunMigrations` using `UPSERT` (ON CONFLICT DO NOTHING) keyed on `(context_id, key)`. The bootstrap function iterates all contexts and inserts defaults only if they don't already exist.

**Option B:** Seed on first request to a context (lazy). Not recommended — first send would fail until a template exists.

---

## 5. API Contracts

### 5.1 Template CRUD (Admin-only)

All admin routes require authentication + `is_admin: true` in the JWT.

#### `GET /admin/email-templates`

List templates for the admin's accessible contexts.

**Query params:**
| Param | Type | Description |
|-------|------|-------------|
| `context_id` | UUID | Filter by context (required) |
| `key` | string | Filter by template key |

**Response 200:**
```json
{
  "templates": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "context_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
      "key": "welcome",
      "subject": "Welcome to {{ .ContextName }}",
      "body_text": "Welcome to {{ .ContextName }}!\n\nHi {{ .ContactName }}...",
      "body_html": "<h1>Welcome to {{ .ContextName }}</h1>...",
      "is_default": true,
      "created_at": "2026-09-25T10:00:00Z",
      "updated_at": "2026-09-25T10:00:00Z"
    }
  ],
  "count": 5
}
```

#### `POST /admin/email-templates`

Create a new email template.

**Request body:**
```json
{
  "context_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "key": "custom_welcome",
  "subject": "Welcome to {{ .ContextName }}!",
  "body_text": "Hello {{ .ContactName }}...",
  "body_html": "<h1>Hello {{ .ContactName }}</h1>",
  "is_default": false
}
```

**Validation:**
- `context_id` — required, must be a valid UUID.
- `key` — required, 1-64 chars, alphanumeric + underscores only. Must not conflict with existing `(context_id, key)` pair.
- `subject` — required, non-empty.
- `body_text` or `body_html` — at least one must be non-empty.

**Response 201:** The created template object.

**Response 409:** `{ "error": "template key already exists for this context" }`

#### `GET /admin/email-templates/:id`

Get a single template by ID.

**Response 200:** Template object (same as list item).

**Response 404:** `{ "error": "template not found" }`

#### `PUT /admin/email-templates/:id`

Update an existing template. All fields are optional; only provided fields are updated.

**Request body:**
```json
{
  "subject": "Updated subject with {{ .NewVar }}",
  "body_text": "Updated body text...",
  "body_html": "<p>Updated HTML body</p>",
  "is_default": true
}
```

**Response 200:** Updated template object.

**Response 404:** `{ "error": "template not found" }`

#### `DELETE /admin/email-templates/:id`

Delete a template.

**Response 200:** `{ "message": "template deleted" }`

**Response 404:** `{ "error": "template not found" }`

#### `POST /admin/email-templates/:id/test`

Test-send a template to a single email address. Renders the template with provided variables and enqueues a real send job.

**Request body:**
```json
{
  "to": "test@example.com",
  "variables": {
    "ContextName": "My Company",
    "ContactName": "Test User",
    "LoginURL": "https://app.example.com/login"
  }
}
```

**Validation:**
- `to` — required, valid email address.
- `variables` — optional object; all template variables must resolve or render fails.
- Template must exist and be accessible.

**Response 200:**
```json
{
  "job_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "rendered_subject": "Welcome to My Company",
  "rendered_body_preview": "Welcome to My Company!\n\nHi Test User..."
}
```

**Response 400:** `{ "error": "template variable undefined: ContextName" }`

---

### 5.2 Send Email (Tenant-scoped)

#### `POST /api/tenant/:slug/email/send`

Resolve a template, interpolate variables, and enqueue an email job.

**Request body:**
```json
{
  "to": ["user@example.com", "admin@example.com"],
  "template": "welcome",
  "variables": {
    "ContextName": "My Company",
    "ContactName": "Jane Doe",
    "LoginURL": "https://app.example.com/login"
  },
  "subject_override": null,
  "body_override": null,
  "html": true
}
```

**Fields:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `to` | string or string[] | Yes | Recipient(s). Accepts single string or array. |
| `template` | string | Yes | Template key (e.g., `welcome`). |
| `variables` | object | Yes | Key-value pairs for template variable substitution. |
| `subject_override` | string | No | If set, use this instead of the template's subject. |
| `body_override` | string | No | If set, use this as the entire body (ignores template body). |
| `html` | boolean | No | If true, use `body_html`; if false, use `body_text`. Default: `false`. |

**Response 202:**
```json
{
  "job_id": "b2c3d4e5-f6a7-8901-bcde-f12345678901"
}
```

**Response 400:**
```json
{
  "error": "template 'unknown_key' not found for context 'default'"
}
```
or
```json
{
  "error": "template variable undefined: ContactName"
}
```

**Response 404:** `{ "error": "context not found" }`

---

## 6. Key Design Decisions

### 6.1 Template Storage: Database vs Embedded Files

**Decision: Database with seeded defaults.**

**Rationale:**
- The PRD (Question #1) explicitly asks about DB vs embedded. Database storage is superior for this use case because:
  - Non-developers (marketing, support) need to edit templates without code changes.
  - Templates are tenant-scoped — each tenant may want different copy.
  - The existing `admin_configs` pattern already uses DB for key-value settings per context.
- Default templates are **seeded** into the database on context creation (or via bootstrap), so developers don't need to manually create them.

### 6.2 Template Syntax: Go text/template

**Decision: Go `text/template` package.**

**Rationale:**
- No new dependencies — Go stdlib includes `text/template`.
- Matches the Go ecosystem; consistent with the codebase.
- Supports conditionals (`{{ if .ShowCTA }}...{{ end }}`), which enables richer templates without external tooling.
- Sandboxed execution — no arbitrary code, only data access on the provided map.

**Variable naming convention:** Dot-notation with PascalCase (e.g., `.ContactName`, `.ContextName`). This matches the PRD's examples in section 14 and the existing email worker's `EmailJob` struct.

### 6.3 Admin vs Tenant Endpoint Split

**Decision: Two separate route groups.**

| Route | Access | Purpose |
|-------|--------|---------|
| `/admin/email-templates/*` | Admin JWT (`is_admin: true`) | CRUD management of templates across/all contexts |
| `/api/tenant/:slug/email/send` | Tenant JWT / API key | Send emails using templates |

**Rationale:**
- Template management is an admin concern — tenants shouldn't modify system templates or see other tenants' templates.
- Send email is a tenant operation — triggered by tenant events (form submission, contact creation) or tenant users.
- The existing codebase already follows this pattern: `/admin/contexts`, `/admin/users` vs `/api/tenant/:slug/forms`.

### 6.4 Context Isolation

**Decision: `context_id` foreign key with cascade delete.**

All template rows include `context_id` referencing `contexts(id)`. This means:
- Templates are scoped to a single context/tenant.
- When a context is deleted, all its templates are automatically cleaned up via `ON DELETE CASCADE`.
- Admin users can manage templates across contexts (they see all via `GET /admin/email-templates?context_id=...`).

### 6.5 Queue Integration: Reuse Existing EmailQueue

**Decision: Delegate to the existing `emailQueue.Enqueue()` method.**

The existing `EmailJob` struct in `email_queue.go` already has all the fields we need:

```go
type EmailJob struct {
    ID         string
    ContextID  string
    To         []string
    Subject    string
    Body       string
    HTMLBody   string
    Attempts   int
    MaxRetries int
    NextRetry  time.Time
    CreatedAt  time.Time
}
```

The usecase layer constructs an `EmailJob` from the rendered template and enqueues it. No changes to the queue or worker are needed.

### 6.6 Test-Send Endpoint: Enqueue, Don't Send Synchronously

**Decision: Test-send enqueues a job like a normal send.**

**Rationale:**
- Avoids coupling to the SMTP provider in the API layer.
- The test-send validates template rendering (variables resolve correctly) before enqueuing.
- The email worker delivers the test email asynchronously — same code path as production sends.
- If the SMTP provider is misconfigured, the test-send still succeeds (job is enqueued) and fails at worker time. This is acceptable for v1.

### 6.7 Default Template Seeding Strategy

**Decision: Upsert in `RunMigrations` during bootstrap.**

The `bootstrap/admin.go` pattern already runs on startup. We'll extend it:

```go
// In bootstrap.Run():
// After creating default context, seed email templates for it.
email_repo.SeedDefaults(pool, contextID)
```

The `SeedDefaults` function uses `INSERT ... ON CONFLICT (context_id, key) DO NOTHING` to safely seed the 5 default templates without errors on repeated startups.

### 6.8 Request Pattern: Object vs Map for Variables

**Decision: `map[string]string` for variables.**

Both `POST /admin/email-templates/:id/test` and `POST /api/tenant/:slug/email/send` accept `variables` as a flat key-value object. This is simpler than a typed struct because:
- Template variables are arbitrary per-template — different templates need different variables.
- Validation is done at render time (undefined variables produce errors).
- No need to maintain a schema per template.

---

## 7. Implementation Plan

### Phase 1: Domain + Repository

1. **Create `internal/domain/email/`**
   - `email.go`: `EmailTemplate` struct, request types (`CreateTemplateRequest`, `UpdateTemplateRequest`, `SendEmailRequest`, `TestSendRequest`)
   - `repository.go`: `Repository` interface with CRUD + `GetByKey(contextID, key)`, `List(contextID)`, `SeedDefaults(contextID)`

2. **Create `internal/infrastructure/email/repo/`**
   - `migration.sql`: CREATE TABLE + CREATE INDEX
   - `postgres.go`: `RunMigrations(pool)`, `New(pool) Repository`, all interface implementations
   - `seed.go` (or inline in `postgres.go`): `SeedDefaults` function with the 5 default templates

3. **Add to `cmd/automata/main.go`**
   - Call `email_repo.RunMigrations(pool)` after other migrations
   - Call `email_repo.SeedDefaults(pool, contextID)` during bootstrap for existing contexts

### Phase 2: Use Case + Handler

4. **Create `internal/usecase/email/email.go`**
   - `EmailUsecase` struct with `repo` dependency
   - `RenderTemplate(contextID, templateKey, variables) → (subject, body, htmlBody, err)`
   - `SendEmail(contextID, req) → (jobID, err)` — validates template, renders, enqueues
   - `TestSend(req) → (jobID, renderedSubject, preview, err)` — validates + renders + enqueues to single address

5. **Create `internal/adapter/api/handler/email_handler.go`**
   - `EmailHandler` struct with `usecase` dependency
   - Methods: `List`, `Create`, `Get`, `Update`, `Delete`, `TestSend` (admin routes)
   - Method: `Send` (tenant route)
   - Follow the banner_handler pattern: `c.ShouldBindJSON`, `c.GetHeader("X-Context-ID")`, `c.JSON(status, gin.H{...})`

6. **Register routes in `internal/adapter/api/server.go`**
   - Admin routes under `admin.Group("/email-templates")`
   - Tenant route under context-resolved group (after `api.Use(ContextResolver)`)

### Phase 3: Integration + Testing

7. **Wire everything together**
   - Pass `email_repo` → `EmailUsecase` → `EmailHandler` through the server constructor
   - The usecase receives `queue.Queue` (the existing `emailQueue`) to enqueue jobs

8. **Integration tests** (Ginkgo + testcontainers)
   - CRUD operations on templates
   - Template rendering with valid/invalid variables
   - Send email enqueues to `email_jobs` table
   - Test send renders and enqueues
   - Default template seeding

### File Summary

| File | Action |
|------|--------|
| `internal/domain/email/email.go` | **Create** — domain types + repository interface |
| `internal/domain/email/repository.go` | **Create** — Repository interface |
| `internal/infrastructure/email/repo/postgres.go` | **Create** — PostgreSQL implementation |
| `internal/infrastructure/email/repo/migration.sql` | **Create** — table + indexes |
| `internal/usecase/email/email.go` | **Create** — business logic |
| `internal/adapter/api/handler/email_handler.go` | **Create** — HTTP handlers |
| `internal/adapter/api/server.go` | **Modify** — register email routes |
| `cmd/automata/main.go` | **Modify** — run migrations + seed |
| `cmd/bff/main.go` | **Modify** — run migrations + seed (if BFF also needs templates) |
