# Design: Form Management

## Problem Statement

Marketing automation requires a form builder that lets users create custom forms for lead capture, feedback collection, and data collection. Without a form system, organizations cannot capture visitor information beyond manual data entry. Forms must support configurable fields with validation, file uploads, webhook notifications on submission, email notifications, and spam protection. Submitted form data should flow into the contact management system, linking submissions to contacts and activities.

## Goals

1. Create forms with configurable fields (text, email, textarea, select, checkbox, radio, file upload) stored as JSONB
2. Validate submissions against form field definitions (required, pattern, email)
3. Deliver submissions via webhooks with exponential backoff retry (3 retries, 1h window)
4. Send email notifications to configured addresses on form submission
5. Protect forms from spam via honeypot field technique
6. Support file uploads via storage adapter (local/S3)
7. Provide public form submission endpoint (no auth required)
8. Provide private form management API for CRUD operations
9. All data strictly isolated per context (tenant)

## Non-Goals

- Conditional logic between form fields
- reCAPTCHA or hCaptcha integration
- Payment processing via forms
- Multi-step / wizard forms
- Form embedding via iframe (server-rendered HTML only)
- Form A/B testing

## Architecture

### Component Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                      Form Management Architecture                    │
│                                                                      │
│  ┌──────────────────┐   ┌───────────────────┐   ┌────────────────┐  │
│  │ Management API    │   │ Public API        │   │ Email Queue    │  │
│  │ (authenticated)   │   │ (public)          │   │ (worker)       │  │
│  │                   │   │                   │   │                │  │
│  │ POST /forms       │   │ POST /form/:slug  │   │ SMTP mock      │  │
│  │ GET  /forms       │   │ GET  /form/:slug  │   │ Exponential    │  │
│  │ PUT  /forms/:id   │   │                   │   │ backoff retry  │  │
│  │ DELETE /forms/:id │   └────────┬──────────┘   └────────────────┘  │
│  └────────┬──────────┘            │                                   │
│           │                       │                                   │
│  ┌────────┴───────────────────────┼─────────────────────────┐        │
│  │ Use Cases                       │ Domain Layer             │        │
│  │ CreateForm, SubmitForm          │ Form entity              │        │
│  │ ListForms, UpdateForm           │ FormField struct         │        │
│  │ DeleteForm                      │ Submission entity        │        │
│  │ ListSubmissions                 │ Repository interface     │        │
│  └────────┬────────────────────────┴─────────────────────────┘        │
│           │                                                            │
│  ┌────────┴──────────────────────────────────────────────────────────┐│
│  │ Infrastructure                                                      ││
│  │ migration.sql (forms, form_submissions)                             ││
│  │ queue/migration.sql (webhook_deliveries)                            ││
│  │ form/repo/postgres.go                                                ││
│  │ queue/webhook_worker.go                                              ││
│  │ queue/email_worker.go                                                ││
│  └────────────────────────────────────────────────────────────────────┘│
│  ┌────────────────────────────────────────────────────────────────────┐│
│  │ pkg/validation/validator.go                                          ││
│  └────────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────┘
```

### Form Field Definition

Form fields are stored as a JSONB array on the forms table. Each field definition includes:

```json
{
  "slug": "email",
  "type": "email",
  "label": "Email Address",
  "required": true,
  "validation": {
    "pattern": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
  },
  "order": 1
}
```

Supported field types: `text`, `email`, `textarea`, `select`, `checkbox`, `radio`, `file`.

### Validation Engine

The validation engine (`pkg/validation/validator.go`) processes submitted data against form field definitions:

1. **Required check**: Missing required fields produce validation errors
2. **Type validation**: Email fields validated against regex pattern
3. **Pattern validation**: Fields with `validation.pattern` validated via regex
4. **Honeypot check**: Hidden `_honeypot` field must be empty (bots fill it)
5. **Error aggregation**: All validation errors collected and returned as a single message

### Webhook Delivery

Webhook delivery is DB-backed with retry logic:

```
Form submitted → Webhook enqueued (status: pending)
       │
       ▼
Worker polls every 10s for due jobs
       │
       ▼
POST payload to webhook URL
       │
       ├── 200 OK → status: delivered
       │
       └── Error/4xx+ → attempts++, if < max_retries → status: retrying
            │
            ▼ Exponential backoff (1min, 5min, 30min)
            │
            ▼ After 3 failures → status: failed
```

### Public Form Submission

Form submission is public (no authentication required):

```
POST /form/:slug { "field_slug": "value", ... }
       │
       ├── Validation → 400 { "error": "field is required" }
       │
       ├── Honeypot check → 400 { "error": "spam detected" }
       │
       ├── Create submission record → 200 { "submission_id": "uuid" }
       │
       ├── Trigger webhook (if configured)
       │
       └── Send email notification (if configured)
```

## Data Model

### Tables

```sql
-- Forms: form definitions with fields
CREATE TABLE forms (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id      UUID NOT NULL,
    slug            TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    fields          JSONB NOT NULL DEFAULT '[]',   -- []FormField
    settings        JSONB DEFAULT '{}',            -- {webhook_url, email_notifications}
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_forms_context_slug ON forms(context_id, slug);
CREATE INDEX idx_forms_context_created ON forms(context_id, created_at DESC);

-- Form submissions
CREATE TABLE form_submissions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id         UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    context_id      UUID NOT NULL,
    data            JSONB NOT NULL,                -- field values
    files           JSONB DEFAULT '[]',            -- []FileUpload
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_submissions_form ON form_submissions(form_id);
CREATE INDEX idx_submissions_context_created ON form_submissions(context_id, created_at DESC);

-- Webhook delivery queue
CREATE TABLE webhook_deliveries (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL,
    form_id     UUID NOT NULL,
    url         TEXT NOT NULL,
    payload     JSONB NOT NULL,
    status      VARCHAR(16) DEFAULT 'pending',
    attempts    INT DEFAULT 0,
    max_retries INT DEFAULT 3,
    next_retry  TIMESTAMPTZ,
    error_msg   TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_webhooks_status ON webhook_deliveries(status) WHERE status IN ('pending', 'retrying');
CREATE INDEX idx_webhooks_next_retry ON webhook_deliveries(next_retry) WHERE status = 'pending';
```

### Domain Entities

```go
type FormField struct {
    Slug       string                 `json:"slug"`
    Type       string                 `json:"type"`    // text, email, textarea, select, checkbox, radio, file
    Label      string                 `json:"label"`
    Required   bool                   `json:"required"`
    Validation map[string]interface{} `json:"validation"`
    Order      int                    `json:"order"`
}

type Form struct {
    ID          string
    ContextID   string
    Slug        string
    Name        string
    Description string
    Fields      []FormField          // unmarshaled from JSONB
    Settings    map[string]interface{}  // {webhook_url, email_notifications}
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type FileUpload struct {
    Field string `json:"field"`
    Name  string `json:"name"`
    Size  int64  `json:"size"`
    Type  string `json:"type"`
    URL   string `json:"url"`
}

type Submission struct {
    ID        string
    FormID    string
    ContextID string
    Data      map[string]interface{}  // field values
    Files     []FileUpload
    CreatedAt time.Time
}

type WebhookJob struct {
    ID          string
    ContextID   string
    FormID      string
    URL         string
    Payload     map[string]interface{}
    Status      string   // pending, retrying, delivered, failed
    Attempts    int
    MaxRetries  int
    NextRetry   time.Time
    ErrorMsg    *string
    CreatedAt   time.Time
}
```

## API Contracts

### Management API (Authenticated)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/forms` | Create form with fields |
| `GET` | `/api/forms` | List forms for context |
| `GET` | `/api/forms/:id` | Get form by ID |
| `PUT` | `/api/forms/:id` | Update form |
| `DELETE` | `/api/forms/:id` | Delete form (cascades to submissions) |
| `GET` | `/api/forms/:id/submissions` | List submissions (paginated) |
| `GET` | `/api/forms/:id/submissions/:subId` | Get submission details |

### Public API (No Auth)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/form/:slug` | Submit form data |
| `GET` | `/form/:slug` | Render form HTML |

### Request/Response Shapes

```json
// POST /api/forms (create form)
{
  "slug": "contact-us",
  "name": "Contact Us Form",
  "description": "General inquiries",
  "fields": [
    { "slug": "name", "type": "text", "label": "Full Name", "required": true, "order": 1 },
    { "slug": "email", "type": "email", "label": "Email", "required": true, "order": 2 },
    { "slug": "message", "type": "textarea", "label": "Message", "required": true, "order": 3 }
  ],
  "settings": {
    "webhook_url": "https://hooks.example.com/form",
    "email_notifications": {
      "enabled": true,
      "to": ["admin@example.com"],
      "subject": "New form submission"
    }
  }
}
→ 201 { "form": { ... } }

// POST /form/contact-us (public submission)
{
  "name": "John Doe",
  "email": "john@example.com",
  "message": "I have a question...",
  "_honeypot": ""
}
→ 200 { "status": "success", "submission_id": "uuid" }

// POST /form/contact-us with honeypot filled
{ "_honeypot": "filled" }
→ 400 { "error": "spam detected" }
```

### MCP Tools

```
forms.list(context_id) → Form[]
forms.get(context_id, id) → Form
forms.create(context_id, data) → Form
forms.update(context_id, id, data) → Form
forms.delete(context_id, id) → void
forms.submit(form_slug, data) → { submission_id: string }
forms.list_submissions(form_id) → Submission[]
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Form fields as JSONB | JSONB array of field definitions | Schema-free form builder; no migrations needed for new fields |
| Public submission endpoint | No auth on `/form/:slug` | Forms are meant for public-facing visitor capture |
| Validation engine | pkg/validation shared library | Reusable; testable independently; supports required/pattern/email |
| Honeypot spam protection | Hidden field technique | Simple, effective, no external dependencies (vs reCAPTCHA) |
| Webhook delivery | DB-backed retry queue | Reliable delivery; observable retry status; no external message broker |
| Email notifications | Core platform email queue | Shared infrastructure; automatic retry; SMTP mock for testing |
| Submission data | JSONB column | Flexible; supports any field combination per form |
| Context isolation | context_id on all tables | Strict multi-tenancy; FK to contexts(id) with CASCADE |
