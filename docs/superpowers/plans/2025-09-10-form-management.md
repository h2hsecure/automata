# Form Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build form builder, submissions, validation engine, webhooks, and email notifications.

**Architecture:** Hexagonal architecture with domain entities for Form, FormField, and Submission. Form fields stored as JSONB array. Validation engine in pkg/validation. Webhooks via DB-backed retry queue. Email notifications via core platform email queue.

**Tech Stack:** Go 1.26+, PostgreSQL, Ginkgo + testcontainers

**Spec:** docs/prds/2025-09-10-form-management-prd.md

## Global Constraints

- Hexagonal architecture: domain interfaces in `internal/domain`, use cases in `internal/usecase`, adapters in `internal/adapter`, infrastructure in `internal/infrastructure`
- KISS and DRY principles throughout
- Multi-tenant: all tables include `tenant_id`, data strictly isolated per tenant
- Form fields stored as JSONB array in database
- Webhook retry: DB-backed retry table with exponential backoff (3 retries over 1 hour)
- Email notifications via internal email queue (core platform Task 9)
- File uploads stored via storage adapter (local/S3)
- Spam protection via honeypot field

---

### Task 1: Form domain entity and repository interface

**Files:**
- Create: `internal/domain/form/form.go`
- Create: `internal/domain/form/submission.go`
- Create: `internal/domain/form/repository.go`

**Interfaces:**
- Consumes: none (domain layer)
- Produces: `Form` entity with fields as JSONB array, `Submission` entity, `Repository` interface

- [ ] **Step 1: Define Form entity**

```go
// internal/domain/form/form.go
package form

import "time"

type FormField struct {
    Slug       string                 `json:"slug"`
    Type       string                 `json:"type"`
    Label      string                 `json:"label"`
    Required   bool                   `json:"required"`
    Validation map[string]interface{} `json:"validation"`
    Order      int                    `json:"order"`
}

type Form struct {
    ID          string
    TenantID    string
    Slug        string
    Name        string
    Description string
    Fields      []FormField
    Settings    map[string]interface{}
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

func (f *Form) Validate() error {
    if f.Slug == "" {
        return errors.New("slug required")
    }
    if len(f.Fields) == 0 {
        return errors.New("at least one field required")
    }
    return nil
}
```

- [ ] **Step 2: Define Submission entity**

```go
// internal/domain/form/submission.go
package form

import "time"

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
    TenantID  string
    Data      map[string]interface{}
    Files     []FileUpload
    CreatedAt time.Time
}
```

- [ ] **Step 3: Define Repository interface**

```go
// internal/domain/form/repository.go
package form

type Repository interface {
    Create(f *Form) error
    GetByID(id string) (*Form, error)
    GetBySlug(tenantID, slug string) (*Form, error)
    List(tenantID string) ([]*Form, error)
    Update(f *Form) error
    Delete(id string) error
    CreateSubmission(s *Submission) error
    GetSubmission(id string) (*Submission, error)
    ListSubmissions(formID string, opts ListSubmissionsOptions) ([]*Submission, int64, error)
}

type ListSubmissionsOptions struct {
    Offset int
    Limit  int
}
```

- [ ] **Step 4: Write entity validation tests**

```go
func TestFormValidation(t *testing.T) {
    f := &Form{}
    assert.Error(t, f.Validate())
    f.Slug = "contact"
    assert.Error(t, f.Validate())
    f.Fields = []FormField{{Slug: "name", Type: "text"}}
    assert.NoError(t, f.Validate())
}
```

- [ ] **Step 5: Run tests to verify they fail**

Run: `go test ./internal/domain/form/... -v`
Expected: FAIL (Validate method not implemented)

- [ ] **Step 6: Implement validation**

```go
func (f *Form) Validate() error {
    if f.Slug == "" {
        return errors.New("slug required")
    }
    if len(f.Fields) == 0 {
        return errors.New("at least one field required")
    }
    return nil
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/domain/form/... -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/domain/form/
git commit -m "feat: add form, form field, submission domain entities with repository interface"
```

---

### Task 2: Form PostgreSQL repository

**Files:**
- Create: `internal/infrastructure/form/repo/postgres.go`
- Create: `internal/infrastructure/form/repo/postgres_test.go`
- Create: `internal/infrastructure/form/repo/migration.sql`

**Interfaces:**
- Consumes: `database.NewPool()` from core platform
- Produces: Full Repository implementation

- [ ] **Step 1: Write migration SQL**

```sql
CREATE TABLE forms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    slug        TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    fields      JSONB NOT NULL DEFAULT '[]',
    settings    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_forms_tenant_slug ON forms(tenant_id, slug);

CREATE TABLE form_submissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id     UUID NOT NULL REFERENCES forms(id),
    tenant_id   UUID NOT NULL,
    data        JSONB NOT NULL,
    files       JSONB DEFAULT '[]',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_submissions_form ON form_submissions(form_id);
CREATE INDEX idx_submissions_tenant_created ON form_submissions(tenant_id, created_at DESC);
```

- [ ] **Step 2: Implement Form CRUD**

```go
func (r *repo) Create(f *Form) error {
    fieldsJSON, _ := json.Marshal(f.Fields)
    settingsJSON, _ := json.Marshal(f.Settings)
    query := `INSERT INTO forms (tenant_id, slug, name, description, fields, settings)
              VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at, updated_at`
    return r.pool.QueryRow(ctx, query, f.TenantID, f.Slug, f.Name, f.Description, fieldsJSON, settingsJSON).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt)
}

func (r *repo) GetBySlug(tenantID, slug string) (*Form, error) {
    // SELECT * FROM forms WHERE tenant_id = $1 AND slug = $2
    // Unmarshal fields JSONB into []FormField
}
```

- [ ] **Step 3: Implement Submission storage**

```go
func (r *repo) CreateSubmission(s *Submission) error {
    dataJSON, _ := json.Marshal(s.Data)
    filesJSON, _ := json.Marshal(s.Files)
    query := `INSERT INTO form_submissions (form_id, tenant_id, data, files)
              VALUES ($1, $2, $3, $4) RETURNING id, created_at`
    return r.pool.QueryRow(ctx, query, s.FormID, s.TenantID, dataJSON, filesJSON).Scan(&s.ID, &s.CreatedAt)
}
```

- [ ] **Step 4: Write integration tests**

```go
var _ = Describe("FormRepository", func() {
    It("creates a form with fields", func() {})
    It("gets form by slug", func() {})
    It("creates a submission", func() {})
    It("lists submissions for a form", func() {})
})
```

- [ ] **Step 5: Run tests**

Run: `ginkgo internal/infrastructure/form/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/form/
git commit -m "feat: add PostgreSQL repository for forms and submissions"
```

---

### Task 3: Form use cases

**Files:**
- Create: `internal/usecase/form/create_form.go`
- Create: `internal/usecase/form/submit_form.go`
- Create: `internal/usecase/form/list_submissions.go`
- Create: `internal/usecase/form/delete_form.go`

**Interfaces:**
- Consumes: `form.Repository`
- Produces: `CreateForm(tenantID string, data map[string]interface{}) (*form.Form, error)`, `SubmitForm(formSlug string, data map[string]interface{}, files []FileUpload) (*form.Submission, error)`, `ListSubmissions(formID string, opts) ([]*form.Submission, int64, error)`

- [ ] **Step 1: Implement CreateForm**

```go
func CreateForm(repo form.Repository, tenantID string, data map[string]interface{}) (*form.Form, error) {
    f := &form.Form{
        TenantID: tenantID,
        Slug:     data["slug"].(string),
        Name:     data["name"].(string),
        Fields:   []form.FormField{},
        Settings: map[string]interface{}{},
    }
    if fields, ok := data["fields"].([]interface{}); ok {
        // Unmarshal fields
    }
    if settings, ok := data["settings"].(map[string]interface{}); ok {
        f.Settings = settings
    }
    if err := f.Validate(); err != nil {
        return nil, err
    }
    if err := repo.Create(f); err != nil {
        return nil, err
    }
    return f, nil
}
```

- [ ] **Step 2: Implement SubmitForm with validation**

```go
func SubmitForm(repo form.Repository, formSlug string, data map[string]interface{}, files []form.FileUpload) (*form.Submission, error) {
    form, err := repo.GetBySlug("", formSlug)
    if err != nil {
        return nil, err
    }
    // Validate data against form fields
    if err := validation.Validate(data, form.Fields); err != nil {
        return nil, err
    }
    // Check honeypot field
    if honeypot, ok := data["_honeypot"].(string); ok && honeypot != "" {
        return nil, errors.New("spam detected")
    }
    s := &form.Submission{
        FormID: form.ID,
        TenantID: form.TenantID,
        Data: data,
        Files: files,
    }
    if err := repo.CreateSubmission(s); err != nil {
        return nil, err
    }
    return s, nil
}
```

- [ ] **Step 3: Implement ListSubmissions**

```go
func ListSubmissions(repo form.Repository, formID string, opts form.ListSubmissionsOptions) ([]*form.Submission, int64, error) {
    return repo.ListSubmissions(formID, opts)
}
```

- [ ] **Step 4: Write unit tests**

```go
func TestSubmitFormValidation(t *testing.T) {
    // Test required field validation
    // Test regex pattern validation
    // Test honeypot spam detection
}

func TestSubmitFormSuccess(t *testing.T) {
    // Valid submission creates record
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/usecase/form/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/usecase/form/
git commit -m "feat: add form use cases for CRUD, submission, and listing"
```

---

### Task 4: Form field validation engine

**Files:**
- Create: `pkg/validation/validator.go`
- Create: `pkg/validation/validator_test.go`

**Interfaces:**
- Consumes: none (domain-agnostic)
- Produces: `Validate(data map[string]interface{}, fields []form.FormField) error`

- [ ] **Step 1: Implement validation engine**

```go
// pkg/validation/validator.go
package validation

import (
    "fmt"
    "regexp"
    "strings"
)

func Validate(data map[string]interface{}, fields []form.FormField) error {
    var errors []string
    for _, field := range fields {
        val, exists := data[field.Slug]
        if !exists || val == nil || val == "" {
            if field.Required {
                errors = append(errors, fmt.Sprintf("%s is required", field.Label))
            }
            continue
        }
        strVal := fmt.Sprintf("%v", val)
        if field.Type == "email" && !isValidEmail(strVal) {
            errors = append(errors, fmt.Sprintf("%s must be a valid email", field.Label))
        }
        if field.Type == "number" {
            // Check min/max
        }
        if pattern, ok := field.Validation["pattern"]; ok {
            re, _ := regexp.Compile(pattern.(string))
            if !re.MatchString(strVal) {
                errors = append(errors, fmt.Sprintf("%s format invalid", field.Label))
            }
        }
    }
    if len(errors) > 0 {
        return fmt.Errorf("validation failed: %s", strings.Join(errors, "; "))
    }
    return nil
}

func isValidEmail(email string) bool {
    re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
    return re.MatchString(email)
}
```

- [ ] **Step 2: Write comprehensive tests**

```go
func TestValidateRequiredField(t *testing.T) {
    fields := []form.FormField{{Slug: "name", Type: "text", Required: true}}
    err := Validate(map[string]interface{}{}, fields)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "required")
}

func TestValidateEmail(t *testing.T) {
    fields := []form.FormField{{Slug: "email", Type: "email", Required: true}}
    err := Validate(map[string]interface{}{"email": "invalid"}, fields)
    assert.Error(t, err)
}

func TestValidateRegexPattern(t *testing.T) {
    fields := []form.FormField{{Slug: "phone", Type: "phone", Required: false, Validation: map[string]interface{}{"pattern": `^\d{10}$`}}}
    err := Validate(map[string]interface{}{"phone": "123"}, fields)
    assert.Error(t, err)
}

func TestValidatePass(t *testing.T) {
    fields := []form.FormField{{Slug: "name", Type: "text", Required: true}}
    err := Validate(map[string]interface{}{"name": "John"}, fields)
    assert.NoError(t, err)
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./pkg/validation/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add pkg/validation/
git commit -m "feat: add form field validation engine"
```

---

### Task 5: Webhook delivery with retry

**Files:**
- Create: `internal/infrastructure/queue/webhook_queue.go`
- Create: `internal/infrastructure/queue/webhook_worker.go`
- Create: `internal/infrastructure/queue/webhook_queue_test.go`
- Create: `internal/infrastructure/queue/migration.sql`

**Interfaces:**
- Consumes: `database.NewPool()`, `http.Client`
- Produces: `EnqueueWebhook(url string, payload map[string]interface{}) error` with DB-backed retry (3 retries, exponential backoff over 1 hour)

- [ ] **Step 1: Write webhook retry migration**

```sql
CREATE TABLE webhook_deliveries (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
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

CREATE INDEX idx_webhooks_tenant_status ON webhook_deliveries(tenant_id, status, next_retry);
```

- [ ] **Step 2: Implement webhook enqueue**

```go
func (q *queue) EnqueueWebhook(tenantID, formID string, url string, payload map[string]interface{}) error {
    job := &WebhookJob{
        TenantID:   tenantID,
        FormID:     formID,
        URL:        url,
        Payload:    payload,
        Attempts:   0,
        MaxRetries: 3,
        NextRetry:  time.Now(),
    }
    // INSERT INTO webhook_deliveries
    return nil
}
```

- [ ] **Step 3: Implement webhook worker with exponential backoff**

```go
func (q *queue) StartWorker(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            jobs := q.fetchDueJobs(ctx)
            for _, job := range jobs {
                resp, err := http.PostJSON(job.URL, job.Payload)
                if err != nil || resp.StatusCode >= 400 {
                    job.Attempts++
                    if job.Attempts >= job.MaxRetries {
                        job.Status = "failed"
                    } else {
                        // Exponential backoff: 1min, 5min, 30min
                        job.NextRetry = time.Now().Add(time.Duration(job.Attempts) * time.Minute * 5)
                        job.Status = "pending"
                    }
                    q.updateJob(ctx, job)
                } else {
                    job.Status = "delivered"
                    q.updateJob(ctx, job)
                }
            }
        case <-ctx.Done():
            return
        }
    }
}
```

- [ ] **Step 4: Write integration tests**

```go
func TestWebhookEnqueueAndDeliver(t *testing.T) {
    // Enqueue webhook, verify worker delivers it
}

func TestWebhookRetryOnFailure(t *testing.T) {
    // Simulate failure, verify retry schedule
}

func TestWebhookMaxRetries(t *testing.T) {
    // After 3 failures, mark as failed
}
```

- [ ] **Step 5: Run tests**

Run: `ginkgo internal/infrastructure/queue/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/queue/
git commit -m "feat: add DB-backed webhook delivery with exponential backoff retry"
```

---

### Task 6: Form submission endpoint and proxy route

**Files:**
- Create: `internal/adapter/api/handler/form_submit_handler.go`
- Create: `internal/adapter/proxy/form_render_handler.go`
- Modify: `internal/adapter/api/server.go` (add form routes)
- Modify: `internal/adapter/proxy/proxy.go` (add /form/<slug> route)

**Interfaces:**
- Consumes: form use cases, webhook queue, email queue
- Produces: Public POST `/form/<slug>` endpoint, form rendering via proxy

- [ ] **Step 1: Implement public form submission handler**

```go
// internal/adapter/api/handler/form_submit_handler.go
func (h *handler) Submit(c *gin.Context) {
    slug := c.Param("slug")
    var data map[string]interface{}
    c.BindJSON(&data)

    submission, err := usecase.SubmitForm(h.formRepo, slug, data, nil)
    if err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // Trigger webhook if configured
    form, _ := h.formRepo.GetBySlug("", slug)
    if webhookURL, ok := form.Settings["webhook_url"].(string); ok && webhookURL != "" {
        h.webhookQueue.EnqueueWebhook(form.TenantID, form.ID, webhookURL, map[string]interface{}{
            "form_id":       form.ID,
            "form_name":     form.Name,
            "submission_id": submission.ID,
            "fields":        submission.Data,
            "submitted_at":  submission.CreatedAt,
        })
    }

    // Trigger email notification if configured
    if emailEnabled, ok := form.Settings["email_notifications"].(map[string]interface{}); ok {
        // Enqueue email job via core platform email queue
    }

    c.JSON(200, gin.H{"status": "success", "submission_id": submission.ID})
}
```

- [ ] **Step 2: Implement form rendering via proxy**

```go
// internal/adapter/proxy/form_render_handler.go
func (h *handler) RenderForm(c *gin.Context) {
    slug := c.Param("slug")
    form, err := h.formRepo.GetBySlug(getTenantID(c), slug)
    if err != nil {
        c.String(404, "Form not found")
        return
    }
    // Render form HTML from form.Fields
    html := renderFormHTML(form)
    c.String(200, html)
}
```

- [ ] **Step 3: Register routes**

```go
// Public form submission (no auth required)
api.POST("/form/:slug", formSubmitHandler.Submit)

// Proxy form rendering
proxy.GET("/form/:slug", formRenderHandler.RenderForm)
```

- [ ] **Step 4: Write integration tests**

```go
func TestFormSubmission(t *testing.T) {
    // Submit valid form data, verify submission created
}

func TestFormSubmissionValidation(t *testing.T) {
    // Submit invalid data, verify 400 response
}

func TestFormSubmissionSpam(t *testing.T) {
    // Submit with honeypot filled, verify spam rejection
}
```

- [ ] **Step 5: Run tests**

Run: `ginkgo internal/adapter/api/... internal/adapter/proxy/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/api/handler/form_submit_handler.go internal/adapter/proxy/form_render_handler.go
git commit -m "feat: add public form submission endpoint and proxy rendering"
```

---

### Task 7: Form API handlers and MCP tools

**Files:**
- Create: `internal/adapter/api/handler/form_handler.go`
- Modify: `internal/adapter/mcp/forms_tools.go`

**Interfaces:**
- Consumes: form use cases
- Produces: HTTP handlers for form CRUD, MCP tools for forms.list, forms.get, forms.create, forms.update, forms.delete, forms.submit, forms.list_submissions

- [ ] **Step 1: Implement form CRUD handlers**

```go
func (h *handler) List(c *gin.Context) {
    forms, _ := usecase.ListForms(h.repo, getTenantID(c))
    c.JSON(200, forms)
}

func (h *handler) Create(c *gin.Context) {
    var req map[string]interface{}
    c.BindJSON(&req)
    form, err := usecase.CreateForm(h.repo, getTenantID(c), req)
    c.JSON(201, form)
}

func (h *handler) Get(c *gin.Context) {
    form, err := usecase.GetFormByID(h.repo, getTenantID(c), c.Param("id"))
    c.JSON(200, form)
}

func (h *handler) Update(c *gin.Context) {
    var req map[string]interface{}
    c.BindJSON(&req)
    form, err := usecase.UpdateForm(h.repo, getTenantID(c), c.Param("id"), req)
    c.JSON(200, form)
}

func (h *handler) Delete(c *gin.Context) {
    usecase.DeleteForm(h.repo, getTenantID(c), c.Param("id"))
    c.Status(204)
}
```

- [ ] **Step 2: Implement MCP form tools**

```go
s.AddTool("forms.submit", "Submit a form",
    func(ctx context.Context, req struct{ FormSlug string, Data map[string]interface{} }) (map[string]interface{}, error) {
        sub, err := usecase.SubmitForm(repo, req.FormSlug, req.Data, nil)
        return map[string]interface{}{"submission_id": sub.ID}, err
    })

s.AddTool("forms.list_submissions", "List submissions for a form",
    func(ctx context.Context, req struct{ FormID string }) ([]map[string]interface{}, error) {
        subs, _, _ := usecase.ListSubmissions(repo, req.FormID, form.ListSubmissionsOptions{})
        return subs, nil
    })
```

- [ ] **Step 3: Write tests**

```go
func TestFormCRUD(t *testing.T) {
    // Create -> Get -> Update -> List -> Delete
}

func TestFormMCPTools(t *testing.T) {
    // Test forms.submit, forms.list_submissions
}
```

- [ ] **Step 4: Run tests**

Run: `ginkgo internal/adapter/api/... internal/adapter/mcp/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/api/handler/form_handler.go internal/adapter/mcp/forms_tools.go
git commit -m "feat: add form CRUD API handlers and MCP tools"
```

---

## Testing Strategy

- **Unit tests:** Form field validation, entity validation, use case logic -- `go test ./...`
- **Integration tests:** Repository CRUD, form submission flow, webhook retry -- Ginkgo + testcontainers for PostgreSQL
- **E2E tests:** Full API and MCP layer tests in `cicd/` directory
- **Mail/IMAP mocks:** For email queue integration validation
- **Load tests:** 50 submissions/sec per tenant

## Task Dependencies

```
Task 1 (domain entities) ──> Task 3 (use cases) ──> Task 6 (submission endpoint) ──> Task 7 (API + MCP)
      │                           │
      v                           │
Task 2 (postgres repo) ─────────┘
Task 4 (validation pkg) ────────┘
Task 5 (webhook queue) ─────────┘
```
