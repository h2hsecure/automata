# Contact Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build contact database with CRUD, deduplication, segmentation, CSV import/export, activity timeline, and contact merge.

**Architecture:** Hexagonal architecture with domain entities for Contact, Tag, and Activity. Contacts stored with JSONB custom fields. Deduplication by email address. Activity timeline aggregates data from forms, tracking, and banner events. CSV import/export uses domain-agnostic pkg/import and pkg/export packages.

**Tech Stack:** Go 1.26+, PostgreSQL, Ginkgo + testcontainers

**Spec:** docs/prds/2025-09-10-contact-management-prd.md

## Global Constraints

- Hexagonal architecture: domain interfaces in `internal/domain`, use cases in `internal/usecase`, adapters in `internal/adapter`, infrastructure in `internal/infrastructure`
- KISS and DRY principles throughout
- Multi-tenant: all tables include `tenant_id`, data strictly isolated per tenant
- Email is the primary contact identifier; deduplication by email
- Custom fields stored as JSONB column in PostgreSQL
- Custom field definitions stored in key-value config (`contact.fields.<key>.label`, `contact.fields.<key>.type`)
- Activity timeline aggregates from forms, tracking, and banner events
- Import: CSV with field mapping, duplicates handled by email match
- Contacts without email are created but marked as "unverified" and not deduplicated

---

### Task 1: Contact domain entity and repository interface

**Files:**
- Create: `internal/domain/contact/contact.go`
- Create: `internal/domain/contact/tag.go`
- Create: `internal/domain/contact/activity.go`
- Create: `internal/domain/contact/repository.go`

**Interfaces:**
- Consumes: none (domain layer)
- Produces: `Contact` entity with custom fields as JSONB, `Tag` entity, `Activity` entity, `Repository` interface with `Create`, `GetByID`, `List`, `Update`, `Delete`, `FindByEmail`, `Merge`, `GetActivity`

- [ ] **Step 1: Define Contact entity with validation**

```go
// internal/domain/contact/contact.go
package contact

import (
    "errors"
    "time"
)

type Contact struct {
    ID           string
    TenantID     string
    Email        *string  // nil = unverified
    FirstName    string
    LastName     string
    Phone        string
    Company      string
    CustomFields map[string]interface{}
    Source       string  // form, tracking, manual, import
    SourceID     string
    Tags         []Tag
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

func (c *Contact) Validate() error {
    if c.TenantID == "" {
        return errors.New("tenant_id required")
    }
    if c.Email != nil && *c.Email != "" {
        if !isValidEmail(*c.Email) {
            return errors.New("invalid email format")
        }
    }
    return nil
}

func isValidEmail(email string) bool {
    // Basic email validation
    return len(email) > 0 && len(email) <= 254
}

func (c *Contact) IsVerified() bool {
    return c.Email != nil && *c.Email != ""
}
```

- [ ] **Step 2: Define Tag entity**

```go
// internal/domain/contact/tag.go
package contact

type Tag struct {
    ID        string
    TenantID  string
    Name      string
    Color     string
    CreatedAt time.Time
}

func (t *Tag) Validate() error {
    if t.Name == "" {
        return errors.New("tag name required")
    }
    return nil
}
```

- [ ] **Step 3: Define Activity entity**

```go
// internal/domain/contact/activity.go
package contact

import "time"

type ActivityType string

const (
    ActivityFormSubmission ActivityType = "form_submission"
    ActivityPageVisit      ActivityType = "page_visit"
    ActivityBannerClick    ActivityType = "banner_click"
    ActivityContactCreated ActivityType = "contact_created"
    ActivityContactUpdated ActivityType = "contact_updated"
    ActivityContactMerged  ActivityType = "contact_merged"
)

type Activity struct {
    ID        string
    ContactID string
    TenantID  string
    Type      ActivityType
    Data      map[string]interface{} // event details
    SourceID  string                 // originating form submission or visitor ID
    CreatedAt time.Time
}
```

- [ ] **Step 4: Define Repository interface**

```go
// internal/domain/contact/repository.go
package contact

import "time"

type Repository interface {
    Create(c *Contact) error
    GetByID(id string) (*Contact, error)
    List(offset, limit int, filters FilterOptions) ([]*Contact, error)
    Update(c *Contact) error
    Delete(id string) error
    FindByEmail(tenantID, email string) (*Contact, error)
    Merge(keepID, mergeIntoID string) error
    GetActivity(contactID string, offset, limit int) ([]Activity, error)
    CountByTenant(tenantID string) (int64, error)
}

type FilterOptions struct {
    Tags            []string
    Source          string
    MinSubmissions  int
    Company         string
    Search          string // full-text search on name/email
}
```

- [ ] **Step 5: Write unit tests for entity validation**

```go
// internal/domain/contact/contact_test.go
func TestContactValidation(t *testing.T) {
    // Valid contact passes
    // Missing tenant_id fails
    // Invalid email format fails
    // Nil email (unverified) passes
}

func TestContactIsVerified(t *testing.T) {
    // Email set -> true
    // Email nil -> false
    // Email empty string -> false
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `go test ./internal/domain/contact/... -v`
Expected: FAIL (implementation not yet written)

- [ ] **Step 7: Implement validation logic**

Implement `Validate()`, `IsVerified()`, `isValidEmail()`.

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/domain/contact/... -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/domain/contact/
git commit -m "feat: add contact, tag, activity domain entities and repository interface"
```

---

### Task 2: Database schema and migration

**Files:**
- Create: `internal/infrastructure/contact/migration.sql`
- Create: `internal/infrastructure/contact/migration.go`
- Create: `internal/infrastructure/contact/migration_test.go`

**Interfaces:**
- Consumes: `database.NewPool()`
- Produces: Tables `contacts`, `contact_tags`, `contact_tag_memberships`, `contact_field_definitions`

- [ ] **Step 1: Write migration SQL**

```sql
-- internal/infrastructure/contact/migration.sql

-- Contacts table
CREATE TABLE IF NOT EXISTS contacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    email           VARCHAR(254),
    first_name      VARCHAR(128),
    last_name       VARCHAR(128),
    phone           VARCHAR(32),
    company         VARCHAR(256),
    custom_fields   JSONB DEFAULT '{}',
    source          VARCHAR(64),
    source_id       VARCHAR(128),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_tenant_email
    ON contacts(tenant_id, email) WHERE email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_contacts_tenant_created
    ON contacts(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_contacts_tenant_source
    ON contacts(tenant_id, source);
CREATE INDEX IF NOT EXISTS idx_contacts_tenant_company
    ON contacts(tenant_id, company) WHERE company != '';
CREATE INDEX IF NOT EXISTS idx_contacts_tenant_email_gin
    ON contacts USING gin(to_jsonb(custom_fields)) WHERE email IS NOT NULL;

-- Tags table
CREATE TABLE IF NOT EXISTS contact_tags (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    name        TEXT NOT NULL,
    color       VARCHAR(7) DEFAULT '#6366f1',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_tenant_name
    ON contact_tags(tenant_id, name);

-- Contact-tag junction
CREATE TABLE IF NOT EXISTS contact_tag_memberships (
    contact_id  UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    tag_id      UUID NOT NULL REFERENCES contact_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (contact_id, tag_id)
);

-- Custom fields definition (per tenant)
CREATE TABLE IF NOT EXISTS contact_field_definitions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    key         TEXT NOT NULL,
    label       TEXT NOT NULL,
    type        VARCHAR(32) NOT NULL,
    options     JSONB,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_field_defs_tenant_key
    ON contact_field_definitions(tenant_id, key);
```

- [ ] **Step 2: Write migration runner**

```go
// internal/infrastructure/contact/migration.go
package contact

func RunMigrations(db *pgxpool.Pool) error {
    // Read and execute migration.sql
}
```

- [ ] **Step 3: Write integration tests with testcontainers**

```go
// internal/infrastructure/contact/migration_test.go
var _ = Describe("Contact Migration", func() {
    It("creates all tables", func() {})
    It("creates all indexes", func() {})
    It("handles idempotent re-runs", func() {})
    It("creates unique constraint on tenant+email", func() {})
})
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `ginkgo internal/infrastructure/contact/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/contact/migration.sql internal/infrastructure/contact/migration.go
git commit -m "feat: add contact management database schema and migrations"
```

---

### Task 3: PostgreSQL repository implementation

**Files:**
- Create: `internal/infrastructure/contact/repo/postgres.go`
- Create: `internal/infrastructure/contact/repo/postgres_test.go`

**Interfaces:**
- Consumes: `database.NewPool()`, migration SQL
- Produces: `Repository` interface implementation with all CRUD operations

- [ ] **Step 1: Implement Contact repository**

```go
// internal/infrastructure/contact/repo/postgres.go
package contact

func NewPostgresRepo(pool *pgxpool.Pool) Repository {
    return &repo{pool: pool}
}

func (r *repo) Create(c *Contact) error {
    // INSERT INTO contacts ... RETURNING id, created_at, updated_at
    // Handle unique constraint violation for email dedup
}

func (r *repo) GetByID(id string) (*Contact, error) {
    // SELECT * FROM contacts WHERE id = $1
    // JOIN contact_tag_memberships + contact_tags for tags
}

func (r *repo) List(offset, limit int, filters FilterOptions) ([]*Contact, error) {
    // SELECT * FROM contacts WHERE tenant_id = $1
    // Apply filters: tags (JOIN), source, company, search (full-text on name||email)
    // ORDER BY created_at DESC
}

func (r *repo) Update(c *Contact) error {
    // UPDATE contacts SET ... WHERE id = $1
}

func (r *repo) Delete(id string) error {
    // DELETE FROM contacts WHERE id = $1 (tags cascade)
}

func (r *repo) FindByEmail(tenantID, email string) (*Contact, error) {
    // SELECT * FROM contacts WHERE tenant_id = $1 AND email = $2
}

func (r *repo) Merge(keepID, mergeIntoID string) error {
    // BEGIN transaction
    // 1. Copy custom_fields from keep to mergeInto (merge JSONB)
    // 2. Copy tags from keep to mergeInto
    // 3. Update activity records: set contact_id = mergeIntoID where contact_id = keepID
    // 4. DELETE contact with keepID
    // COMMIT
}

func (r *repo) GetActivity(contactID string, offset, limit int) ([]Activity, error) {
    // SELECT * FROM activities WHERE contact_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3
    // UNION of form submissions, page visits, banner clicks
}

func (r *repo) CountByTenant(tenantID string) (int64, error) {
    // SELECT COUNT(*) FROM contacts WHERE tenant_id = $1
}
```

- [ ] **Step 2: Implement merge logic with transaction**

The merge operation must:
1. Preserve all custom fields (JSONB merge, keep non-null values)
2. Preserve all tags
3. Reassign all activity records to the surviving contact
4. Delete the merged contact
5. Log a merge activity record

- [ ] **Step 3: Write integration tests**

```go
// internal/infrastructure/contact/repo/postgres_test.go
var _ = Describe("Contact Repository", func() {
    var repo Repository

    BeforeEach(func() {
        repo = NewPostgresRepo(testDBPool)
    })

    Describe("Create", func() {
        It("creates a contact", func() {})
        It("rejects duplicate email", func() {})
        It("creates unverified contact without email", func() {})
    })

    Describe("FindByEmail", func() {
        It("finds contact by email", func() {})
        It("returns error for non-existent email", func() {})
    })

    Describe("List", func() {
        It("lists all contacts for tenant", func() {})
        It("filters by tags", func() {})
        It("filters by source", func() {})
        It("paginates results", func() {})
        It("searches by name and email", func() {})
    })

    Describe("Merge", func() {
        It("merges custom fields", func() {})
        It("merges tags", func() {})
        It("reassigns activity records", func() {})
        It("deletes the merged contact", func() {})
    })

    Describe("GetActivity", func() {
        It("returns activities for contact", func() {})
        It("orders by created_at DESC", func() {})
    })
})
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `ginkgo internal/infrastructure/contact/repo/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/contact/repo/
git commit -m "feat: add PostgreSQL repository for contacts, tags, and activities"
```

---

### Task 4: Contact use cases

**Files:**
- Create: `internal/usecase/contact/list_contacts.go`
- Create: `internal/usecase/contact/create_contact.go`
- Create: `internal/usecase/contact/update_contact.go`
- Create: `internal/usecase/contact/delete_contact.go`
- Create: `internal/usecase/contact/merge_contacts.go`
- Create: `internal/usecase/contact/find_by_email.go`
- Create: `internal/usecase/contact/get_activity.go`
- Create: `internal/usecase/contact/segment_contacts.go`

**Interfaces:**
- Consumes: `contact.Repository`
- Produces: Use case functions for all contact operations

- [ ] **Step 1: Implement ListContacts**

```go
// internal/usecase/contact/list_contacts.go
package contact

func ListContacts(repo Repository, tenantID string, opts ListOptions) ([]*domain.Contact, int64, error) {
    contacts, err := repo.List(opts.Offset, opts.Limit, opts.Filters)
    if err != nil {
        return nil, 0, err
    }
    count, err := repo.CountByTenant(tenantID)
    return contacts, count, err
}
```

- [ ] **Step 2: Implement CreateContact with deduplication**

```go
// internal/usecase/contact/create_contact.go
func CreateContact(repo Repository, input CreateInput) (*domain.Contact, error) {
    // 1. Validate input
    // 2. If email present, check for existing contact
    // 3. If found, return existing (deduplication)
    // 4. If not found, create new contact
    // 5. Apply tags if provided
    // 6. Return created/contact
}
```

- [ ] **Step 3: Implement UpdateContact**

```go
// internal/usecase/contact/update_contact.go
func UpdateContact(repo Repository, id string, input UpdateInput) (*domain.Contact, error) {
    // 1. Get existing contact
    // 2. Validate new email if changed
    // 3. Check for email conflicts with other contacts
    // 4. Update contact
    // 5. Update tags (replace all)
    // 6. Return updated contact
}
```

- [ ] **Step 4: Implement DeleteContact**

```go
// internal/usecase/contact/delete_contact.go
func DeleteContact(repo Repository, id string) error {
    // 1. Verify contact exists and belongs to tenant
    // 2. Delete (tags cascade)
}
```

- [ ] **Step 5: Implement MergeContacts**

```go
// internal/usecase/contact/merge_contacts.go
func MergeContacts(repo Repository, keepID, mergeIntoID string) error {
    // 1. Verify both contacts exist and belong to same tenant
    // 2. Verify they are different contacts
    // 3. Call repo.Merge(keepID, mergeIntoID)
    // 4. Log merge activity
}
```

- [ ] **Step 6: Implement GetActivity**

```go
// internal/usecase/contact/get_activity.go
func GetActivity(repo Repository, contactID string, offset, limit int) ([]domain.Activity, error) {
    // 1. Verify contact exists
    // 2. Call repo.GetActivity(contactID, offset, limit)
}
```

- [ ] **Step 7: Implement SegmentContacts**

```go
// internal/usecase/contact/segment_contacts.go
func SegmentContacts(repo Repository, tenantID string, filters contact.FilterOptions) ([]*domain.Contact, error) {
    // Delegate to repo.List with filters
    // Used for segmentation queries
}
```

- [ ] **Step 8: Write unit tests for use cases**

```go
// internal/usecase/contact/create_contact_test.go
func TestCreateContact_Deduplication(t *testing.T) {
    // Given existing contact with email
    // When creating contact with same email
    // Then return existing contact (no duplicate)
}

func TestCreateContact_Unverified(t *testing.T) {
    // Given no email provided
    // When creating contact
    // Then contact is created with nil email
}

func TestMergeContacts_SameTenant(t *testing.T) {
    // Given two contacts in same tenant
    // When merging
    // Then custom fields and tags are preserved
}

func TestMergeContacts_DifferentTenants(t *testing.T) {
    // Given contacts in different tenants
    // When merging
    // Then error is returned
}
```

- [ ] **Step 9: Run tests to verify they pass**

Run: `ginkgo internal/usecase/contact/...`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add internal/usecase/contact/
git commit -m "feat: add contact use cases: CRUD, merge, segmentation, activity"
```

---

### Task 5: CSV import and export

**Files:**
- Create: `pkg/import/csv_importer.go`
- Create: `pkg/import/csv_importer_test.go`
- Create: `pkg/export/csv_exporter.go`
- Create: `pkg/export/csv_exporter_test.go`
- Create: `internal/usecase/contact/import_contacts.go`
- Create: `internal/usecase/contact/export_contacts.go`

**Interfaces:**
- Consumes: `contact.Repository`, `contact.FilterOptions`
- Produces: CSV parsing with field mapping, CSV generation

- [ ] **Step 1: Implement CSV importer**

```go
// pkg/import/csv_importer.go
package import

type FieldMapping map[string]string // field name -> CSV column index

type ImportResult struct {
    Created   int
    Updated   int
    Skipped   int
    Errors    []ImportError
}

type ImportError struct {
    Row    int
    Field  string
    Reason string
}

func ImportCSV(r io.Reader, mapping FieldMapping, tenantID string) (*ImportResult, error) {
    // 1. Parse CSV header
    // 2. Validate mapping
    // 3. For each row:
    //    - Extract fields by mapping
    //    - Validate email if present
    //    - Call contact.CreateContact (handles dedup)
    // 4. Return result with counts
}
```

- [ ] **Step 2: Implement CSV exporter**

```go
// pkg/export/csv_exporter.go
package export

func ExportCSV(contacts []*domain.Contact, fields []string) ([]byte, error) {
    // 1. Write CSV header
    // 2. For each contact, write row with specified fields
    // 3. Include custom fields as separate columns
    // 4. Return CSV bytes
}
```

- [ ] **Step 3: Implement import use case**

```go
// internal/usecase/contact/import_contacts.go
func ImportContacts(repo Repository, tenantID string, csvData io.Reader, mapping FieldMapping) (*ImportResult, error) {
    // 1. Validate mapping
    // 2. Call pkg/import.ImportCSV
    // 3. Log import activities
}
```

- [ ] **Step 4: Implement export use case**

```go
// internal/usecase/contact/export_contacts.go
func ExportContacts(repo Repository, tenantID string, filters contact.FilterOptions, fields []string) ([]byte, error) {
    // 1. List contacts with filters
    // 2. Call pkg/export.ExportCSV
}
```

- [ ] **Step 5: Write integration tests**

```go
// pkg/import/csv_importer_test.go
func TestImportCSV_Basic(t *testing.T) {
    // Given CSV with headers and rows
    // When importing with field mapping
    // Then contacts are created
}

func TestImportCSV_Deduplication(t *testing.T) {
    // Given CSV with duplicate emails
    // When importing
    // Then only one contact per email is created
}

func TestImportCSV_FieldMapping(t *testing.T) {
    // Given CSV with custom column order
    // When importing with mapping
    // Then fields are correctly assigned
}

// pkg/export/csv_exporter_test.go
func TestExportCSV_Basic(t *testing.T) {
    // Given contacts
    // When exporting
    // Then CSV contains all fields
}

func TestExportCSV_CustomFields(t *testing.T) {
    // Given contacts with custom fields
    // When exporting
    // Then custom fields are included as columns
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `ginkgo pkg/import/... pkg/export/... internal/usecase/contact/import_contacts.go internal/usecase/contact/export_contacts.go`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add pkg/import/ pkg/export/ internal/usecase/contact/import_contacts.go internal/usecase/contact/export_contacts.go
git commit -m "feat: add CSV import/export for contacts with field mapping"
```

---

### Task 6: API handlers and MCP tools

**Files:**
- Create: `internal/adapter/api/handler/contact_handler.go`
- Create: `internal/adapter/api/handler/contact_handler_test.go`
- Create: `internal/adapter/mcp/contact_tools.go`
- Create: `internal/adapter/mcp/contact_tools_test.go`

**Interfaces:**
- Consumes: `contact.Repository`, use case functions
- Produces: HTTP API routes and MCP tools for contact operations

- [ ] **Step 1: Implement API handler**

```go
// internal/adapter/api/handler/contact_handler.go
package handler

func (h *Handler) List(c *gin.Context) {
    // GET /api/contacts
    // Parse query params: tags, source, min_submissions, page, limit
    // Call usecase.ListContacts
    // Return paginated JSON response
}

func (h *Handler) Get(c *gin.Context) {
    // GET /api/contacts/:id
    // Call usecase.GetByID
    // Return contact with tags
}

func (h *Handler) Create(c *gin.Context) {
    // POST /api/contacts
    // Bind JSON, call usecase.CreateContact
    // Return created contact (or existing if dedup)
}

func (h *Handler) Update(c *gin.Context) {
    // PUT /api/contacts/:id
    // Bind JSON, call usecase.UpdateContact
    // Return updated contact
}

func (h *Handler) Delete(c *gin.Context) {
    // DELETE /api/contacts/:id
    // Call usecase.DeleteContact
    // Return 204
}

func (h *Handler) Merge(c *gin.Context) {
    // POST /api/contacts/:id/merge
    // Bind JSON with "merge_with" field
    // Call usecase.MergeContacts
    // Return merged contact
}

func (h *Handler) GetActivity(c *gin.Context) {
    // GET /api/contacts/:id/activity
    // Parse query params: offset, limit
    // Call usecase.GetActivity
    // Return activity timeline
}

func (h *Handler) Import(c *gin.Context) {
    // POST /api/contacts/import
    // Parse multipart form (CSV file + mapping JSON)
    // Call usecase.ImportContacts
    // Return import result
}

func (h *Handler) Export(c *gin.Context) {
    // GET /api/contacts/export
    // Parse query params: tags, format, fields
    // Call usecase.ExportContacts
    // Return CSV file download
}
```

- [ ] **Step 2: Register API routes**

```go
// In internal/adapter/api/server.go
contacts := engine.Group("/api/contacts")
{
    contacts.GET("", contactHandler.List)
    contacts.GET("/:id", contactHandler.Get)
    contacts.POST("", contactHandler.Create)
    contacts.PUT("/:id", contactHandler.Update)
    contacts.DELETE("/:id", contactHandler.Delete)
    contacts.POST("/:id/merge", contactHandler.Merge)
    contacts.GET("/:id/activity", contactHandler.GetActivity)
}
engine.POST("/api/contacts/import", contactHandler.Import)
engine.GET("/api/contacts/export", contactHandler.Export)
```

- [ ] **Step 3: Implement MCP tools**

```go
// internal/adapter/mcp/contact_tools.go
func (s *Server) registerContactTools() {
    s.AddTool("contacts.list", "List contacts with filters",
        func(ctx context.Context, req struct{ TenantID string, Tags []string, Source string, Page int, Limit int }) ([]Contact, error) {
            // Call usecase.ListContacts
        }),
    s.AddTool("contacts.get", "Get contact details",
        func(ctx context.Context, req struct{ TenantID string, ID string }) (Contact, error) {
            // Call usecase.GetByID
        }),
    s.AddTool("contacts.create", "Create a contact",
        func(ctx context.Context, req struct{ TenantID string, Data map[string]interface{} }) (Contact, error) {
            // Call usecase.CreateContact
        }),
    s.AddTool("contacts.update", "Update a contact",
        func(ctx context.Context, req struct{ TenantID string, ID string, Data map[string]interface{} }) (Contact, error) {
            // Call usecase.UpdateContact
        }),
    s.AddTool("contacts.delete", "Delete a contact",
        func(ctx context.Context, req struct{ TenantID string, ID string }) (bool, error) {
            // Call usecase.DeleteContact
        }),
    s.AddTool("contacts.merge", "Merge two contacts",
        func(ctx context.Context, req struct{ TenantID string, KeepID string, MergeIntoID string }) (bool, error) {
            // Call usecase.MergeContacts
        }),
    s.AddTool("contacts.import", "Import contacts from CSV",
        func(ctx context.Context, req struct{ TenantID string, CSVData string, Mapping map[string]string }) (ImportResult, error) {
            // Call usecase.ImportContacts
        }),
    s.AddTool("contacts.export", "Export contacts to CSV",
        func(ctx context.Context, req struct{ TenantID string, Tags []string, Fields []string }) (string, error) {
            // Call usecase.ExportContacts, return base64-encoded CSV
        }),
    s.AddTool("contacts.get_activity", "Get contact activity timeline",
        func(ctx context.Context, req struct{ TenantID string, ContactID string, Offset int, Limit int }) ([]Activity, error) {
            // Call usecase.GetActivity
        }),
}
```

- [ ] **Step 4: Write API handler tests**

```go
// internal/adapter/api/handler/contact_handler_test.go
var _ = Describe("Contact Handler", func() {
    It("lists contacts with pagination", func() {})
    It("creates a contact", func() {})
    It("deduplicates on create", func() {})
    It("updates a contact", func() {})
    It("deletes a contact", func() {})
    It("merges two contacts", func() {})
    It("returns activity timeline", func() {})
    It("imports contacts from CSV", func() {})
    It("exports contacts to CSV", func() {})
})
```

- [ ] **Step 5: Write MCP tool tests**

```go
// internal/adapter/mcp/contact_tools_test.go
var _ = Describe("Contact MCP Tools", func() {
    It("contacts.list returns contacts", func() {})
    It("contacts.create creates a contact", func() {})
    It("contacts.merge merges contacts", func() {})
    It("contacts.import imports CSV", func() {})
})
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `ginkgo internal/adapter/api/handler/... internal/adapter/mcp/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/api/handler/contact_handler.go internal/adapter/mcp/contact_tools.go
git commit -m "feat: add contact API handlers and MCP tools"
```

---

## Testing Strategy

- **Unit tests:** Contact entity validation, tag validation, CSV import/export parsing — `go test ./internal/domain/contact/... ./pkg/import/... ./pkg/export/...`
- **Integration tests:** Repository CRUD, merge with transactions, deduplication, segmentation queries — Ginkgo + testcontainers for PostgreSQL
- **Activity timeline test:** Verify aggregation from forms, tracking, and banner events
- **E2E tests:** Full API and MCP layer tests in `cicd/` directory
- **Load tests:** 10k contacts per tenant, list query < 500ms; CSV import of 5000 contacts < 30 seconds; segmentation queries < 2 seconds for 50k contacts

## Task Dependencies

```
Task 1 (domain entities) ──> Task 2 (migration) ──> Task 3 (repository) ──> Task 4 (use cases)
                                                                              │
                                                                              ├──> Task 6 (API + MCP)
                                                                              │
Task 5 (CSV import/export) ───────────────────────────────────────────────────┘
```

**Prerequisites from Core Platform:**
- Task 2 (database migration infrastructure) must be complete
- Task 3 (tenant repository) must be complete for tenant-scoped queries
- Task 8 (MCP server base) must be complete for MCP tool registration
- Task 9 (email queue) not required for contacts
