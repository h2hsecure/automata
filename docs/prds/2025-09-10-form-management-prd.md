# PRD: Form Management

## Overview

Let tenants create forms, collect submissions, and trigger actions on submission. Target user: small-to-mid business owners who need to collect leads, feedback, and other data from their websites.

## Goals

- Provide a visual form builder for creating forms with various field types
- Render forms on tenant websites via reverse proxy (no self-hosted form page needed)
- Store submissions with validation and optional file uploads
- Notify tenants of new submissions via webhook and/or email

## Scope

### In Scope

- Form builder UI with drag-and-drop field placement
- Field types: text, email, phone, textarea, number, select, checkbox, radio, file upload
- Form validation rules (required, min/max, regex patterns)
- Form rendering via reverse proxy: `tenant.example.com/form/<slug>`
- Submission storage with tenant isolation
- Webhook notifications on submission (fire-and-forget with retry)
- Email notifications on submission (configurable per form)
- Submission list view in admin dashboard with search/filter
- Basic spam protection (honeypot field)

### Out of Scope

- Conditional logic / conditional field display (Phase 2+)
- Payment integration (deferred)
- Form embedding via iframe (reverse proxy is primary method)
- Multi-language form support (deferred)
- Advanced analytics per form (deferred)

## Architecture

Clean architecture with hexagonal layout. KISS and DRY principles.

### Layer Structure

```
internal/
  domain/form          — Form, FormField, Submission (entities, value objects, domain interfaces)
  usecase/form         — CreateForm, SubmitForm, ListSubmissions
internal/adapter/mcp   — MCP server exposing form tools
internal/adapter/api   — Form submission endpoint (public, no auth)
internal/infrastructure/
  form/repo            — PostgreSQL repository implementation
  queue/email          — Internal email queue (DB-backed job queue)
  storage              — File storage adapter (local/S3)
pkg/validation         — Form field validation engine
```

### Data Flow

```
Tenant creates form → Form stored as JSON schema → Rendered via proxy → Visitor submits → Validation → Storage → Email queue + Webhook
```

### Key Decisions

- **Form fields as JSON schema:** Fields stored as a JSON array in the database. Each field has a type, label, validation rules, and order.
- **Reverse proxy rendering:** Forms served at `tenant.example.com/form/<slug>`. Tenant's reverse proxy routes `/form/*` to Automata.
- **Webhook retry:** Simple DB-backed retry table with exponential backoff (3 retries over 1 hour).
- **Email notifications:** Sent via internal email queue mechanism. Jobs stored in DB, processed by background worker. Configurable per form via key-value config (`form.email.<form_id>.enabled`, `form.email.<form_id>.recipients`).
- **File uploads:** Stored locally on tenant's filesystem or S3-compatible storage. File metadata (name, size, type, URL) stored in submissions table.
- **Spam protection:** Honeypot field (invisible field that bots fill out). CAPTCHA deferred.

## API Design

### Form CRUD

```
GET    /api/forms                          # List forms (tenant-scoped)
POST   /api/forms                          # Create form
GET    /api/forms/<id>                     # Get form details
PUT    /api/forms/<id>                     # Update form
DELETE /api/forms/<id>                     # Delete form
```

### Form Submission

```
POST /form/<slug>
Body: {
  "field_slug_1": "value",
  "field_slug_2": "value",
  "_ honeypot": ""   // spam protection, must be empty
}
```

### Webhook

```
POST <tenant-configured-url>
Headers:
  X-Automata-Signature: <HMAC-SHA256 signature>
  X-Automata-Event: form_submission
Body: {
  "form_id": "uuid",
  "form_name": "Contact Us",
  "submission_id": "uuid",
  "fields": {
    "name": "John Doe",
    "email": "john@example.com"
  },
  "submitted_at": "2025-01-01T00:00:00Z"
}
```

## Database Schema (tentative)

```sql
-- Forms table
CREATE TABLE forms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    slug        TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    fields      JSONB NOT NULL DEFAULT '[]',
    settings    JSONB DEFAULT '{}',  -- { webhook_url, email_notifications, success_message }
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_forms_tenant_slug ON forms(tenant_id, slug);

-- Submissions table
CREATE TABLE form_submissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id     UUID NOT NULL REFERENCES forms(id),
    tenant_id   UUID NOT NULL,
    data        JSONB NOT NULL,
    files       JSONB DEFAULT '[]',  -- [{ field, name, size, type, url }]
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_submissions_form ON form_submissions(form_id);
CREATE INDEX idx_submissions_tenant_created ON form_submissions(tenant_id, created_at DESC);
```

## Testing

- Unit tests for form field validation (required, min/max, regex)
- Integration tests for form submission via proxy route
- Webhook retry tests (simulate failure, verify retry schedule)
- File upload tests (local storage and S3 backend)
- E2E tests for API and MCP layers using Ginkgo + testcontainers, executed via `cicd/` directory
- Mail and IMAP mocks for email queue integration validation
- Load test: 50 submissions/sec per tenant

## Success Criteria

- Form renders correctly within 200ms of page request
- Submission validation catches all invalid inputs with clear error messages
- Webhook delivery succeeds on first attempt > 99% of the time
- File uploads up to 10MB per file supported
