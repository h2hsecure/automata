# Web UI Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Tenants, Forms, API Keys pages to the Vue SPA and wire the Contacts page to real API endpoints, including backend infrastructure for contacts and forms.

**Architecture:** Vue 3 SPA builds to `web/dist/`, served by Go at root `/`. API at `/api/*`. Contacts and forms use PostgreSQL with new tables. Tenants and API Keys use existing backend endpoints.

**Tech Stack:** Go 1.26.5, Gin v1.12.0, PostgreSQL (pgx v5), Vue 3, Vite 5, TypeScript, shadcn-vue, Tailwind CSS 3, Pinia, Vue Router 4, Lucide Vue

**Spec:** `docs/webui-prd.md` (Phase 2 scope: Contacts CRUD, Forms CRUD, Tenants CRUD, API Keys)

## Global Constraints

- Go 1.26.5, Gin v1.12.0, PostgreSQL (pgx v5)
- JWT HS256 signing, 24h expiry, claims: `user_id`, `tenant_slug`, `exp`
- API base URL: `/api/*` (same origin, no CORS)
- Auth: `Authorization: Bearer <token>` header on all authenticated requests
- Pagination pattern: `{ data, total, page, limit }`
- Error responses: `{ "error": "message" }` with HTTP status code
- Docker Compose for local development, Alpine-based multi-stage builds
- Hexagonal architecture: domain in `internal/domain`, infrastructure in `internal/infrastructure`, adapters in `internal/adapter`
- All new database tables must use UUID primary keys with `gen_random_uuid()`
- Timestamps: ISO 8601 strings in API responses

---

### Task 1: Database Schema for Contacts, Tags, Forms, Submissions, Activities

**Files:**
- Modify: `internal/infrastructure/database/migration.sql`

**Interfaces:**
- Produces: 5 new tables (`contacts`, `tags`, `forms`, `form_submissions`, `activities`) with proper indexes and foreign keys

- [ ] **Step 1: Append migration SQL for new tables**

Append to end of `internal/infrastructure/database/migration.sql`:

```sql
-- Contacts table
CREATE TABLE IF NOT EXISTS contacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email           VARCHAR(254),
    first_name      TEXT NOT NULL DEFAULT '',
    last_name       TEXT NOT NULL DEFAULT '',
    phone           TEXT NOT NULL DEFAULT '',
    company         TEXT NOT NULL DEFAULT '',
    custom_fields   JSONB DEFAULT '{}',
    source          TEXT NOT NULL DEFAULT '',
    source_id       TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_contacts_tenant_id ON contacts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_contacts_email ON contacts(tenant_id, email) WHERE email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_contacts_name ON contacts(tenant_id, first_name, last_name);
CREATE INDEX IF NOT EXISTS idx_contacts_company ON contacts(tenant_id, company);

-- Tags table
CREATE TABLE IF NOT EXISTS tags (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    color       TEXT NOT NULL DEFAULT '#6366f1',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_tenant_name ON tags(tenant_id, name);

-- Contact-tags junction table
CREATE TABLE IF NOT EXISTS contact_tags (
    contact_id  UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    tag_id      UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (contact_id, tag_id)
);

-- Forms table
CREATE TABLE IF NOT EXISTS forms (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    slug            TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    fields          JSONB NOT NULL DEFAULT '[]',
    settings        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_forms_tenant_slug ON forms(tenant_id, slug);

-- Form submissions table
CREATE TABLE IF NOT EXISTS form_submissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id     UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    data        JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_submissions_form_id ON form_submissions(form_id);
CREATE INDEX IF NOT EXISTS idx_submissions_tenant_id ON form_submissions(tenant_id);

-- Activities table
CREATE TABLE IF NOT EXISTS activities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id  UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,
    data        JSONB DEFAULT '{}',
    source_id   TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_activities_contact_id ON activities(contact_id);
CREATE INDEX IF NOT EXISTS idx_activities_tenant_id ON activities(tenant_id);
```

- [ ] **Step 2: Verify SQL syntax**
  Run: `psql -c "\dt"` against a test DB to confirm tables create without errors

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/database/migration.sql
git commit -m "feat: add database schema for contacts, tags, forms, submissions, activities"
```

---

### Task 2: Contact Infrastructure Repository (PostgreSQL)

**Files:**
- Create: `internal/infrastructure/contact/repo/postgres.go`
- Create: `internal/infrastructure/contact/repo/postgres_test.go`

**Interfaces:**
- Consumes: `github.com/jackc/pgx/v5/pgxpool.Pool`, `github.com/hekemen/automata/internal/domain/contact` (Contact, Tag, Activity, FilterOptions, ContactWithCount)
- Produces: `*ContactPostgresRepo` implementing `contact.Repository`

- [ ] **Step 1: Create postgres.go**

Create `internal/infrastructure/contact/repo/postgres.go` with the following content:

```go
package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/contact"
	"github.com/jackc/pgx/v5/pgxpool"
)

type contactPostgresRepo struct {
	pool *pgxpool.Pool
}

// NewContactPostgresRepo creates a new PostgreSQL contact repository.
func NewContactPostgresRepo(pool *pgxpool.Pool) contact.Repository {
	return &contactPostgresRepo{pool: pool}
}

func (r *contactPostgresRepo) Create(c *contact.Contact) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	if err := c.Validate(); err != nil {
		return err
	}

	cfBytes, err := json.Marshal(c.CustomFields)
	if err != nil {
		return fmt.Errorf("marshal custom fields: %w", err)
	}
	if cfBytes == nil {
		cfBytes = []byte("{}")
	}

	var emailStr sql.NullString
	if c.Email != nil && *c.Email != "" {
		emailStr = sql.NullString{String: *c.Email, Valid: true}
	}

	query := `
		INSERT INTO contacts (id, tenant_id, email, first_name, last_name, phone, company, custom_fields, source, source_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err = r.pool.QueryRow(context.Background(), query,
		c.ID, c.TenantID, emailStr, c.FirstName, c.LastName, c.Phone, c.Company,
		cfBytes, c.Source, c.SourceID,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create contact: %w", err)
	}

	// Insert tags
	for _, tag := range c.Tags {
		_, err := r.pool.Exec(context.Background(),
			`INSERT INTO tags (id, tenant_id, name, color, created_at) VALUES ($1, $2, $3, $4, NOW()) ON CONFLICT (tenant_id, name) DO UPDATE SET color = EXCLUDED.color`,
			tag.ID, c.TenantID, tag.Name, tag.Color,
		)
		if err != nil {
			return fmt.Errorf("create tag: %w", err)
		}
	}

	return nil
}

func (r *contactPostgresRepo) GetByID(id string) (*contact.Contact, error) {
	c := &contact.Contact{}
	var cfBytes []byte
	var emailStr sql.NullString

	query := `
		SELECT id, tenant_id, email, first_name, last_name, phone, company, custom_fields, source, source_id, created_at, updated_at
		FROM contacts WHERE id = $1
	`

	err := r.pool.QueryRow(context.Background(), query, id).Scan(
		&c.ID, &c.TenantID, &emailStr, &c.FirstName, &c.LastName,
		&c.Phone, &c.Company, &cfBytes, &c.Source, &c.SourceID,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get contact by ID: %w", err)
	}

	if emailStr.Valid {
		c.Email = &emailStr.String
	}
	if len(cfBytes) > 0 {
		_ = json.Unmarshal(cfBytes, &c.CustomFields)
	}

	// Load tags
	tags, err := r.getTagsForContact(id)
	if err != nil {
		return nil, fmt.Errorf("get contact tags: %w", err)
	}
	c.Tags = tags

	return c, nil
}

func (r *contactPostgresRepo) getTagsForContact(contactID string) ([]contact.Tag, error) {
	query := `
		SELECT t.id, t.tenant_id, t.name, t.color, t.created_at
		FROM tags t
		INNER JOIN contact_tags ct ON t.id = ct.tag_id
		WHERE ct.contact_id = $1
	`
	rows, err := r.pool.Query(context.Background(), query, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []contact.Tag
	for rows.Next() {
		var t contact.Tag
		err := rows.Scan(&t.ID, &t.TenantID, &t.Name, &t.Color, &t.CreatedAt)
		if err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *contactPostgresRepo) List(offset, limit int, filters contact.FilterOptions) ([]*contact.Contact, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	whereClauses := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if filters.Search != "" {
		searchPattern := "%" + filters.Search + "%"
		whereClauses = append(whereClauses, fmt.Sprintf("((c.first_name || ' ' || c.last_name) ILIKE $%d OR c.email ILIKE $%d)", argIdx, argIdx+1))
		args = append(args, searchPattern, searchPattern)
		argIdx += 2
	}

	if filters.Source != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("c.source = $%d", argIdx))
		args = append(args, filters.Source)
		argIdx++
	}

	if filters.Company != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("c.company ILIKE $%d", argIdx))
		args = append(args, "%"+filters.Company+"%")
		argIdx++
	}

	if len(filters.Tags) > 0 {
		placeholders := make([]string, len(filters.Tags))
		tagArgs := make([]interface{}, len(filters.Tags))
		for i, tag := range filters.Tags {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			tagArgs[i] = tag
			argIdx++
		}
		phStr := strings.Join(placeholders, ", ")
		whereClauses = append(whereClauses, fmt.Sprintf("c.id IN (SELECT ct.contact_id FROM contact_tags ct WHERE ct.tag_id IN (%s))", phStr))
		args = append(args, tagArgs...)
	}

	whereClause := "WHERE " + strings.Join(whereClauses[1:], " AND ")

	query := fmt.Sprintf(`
		SELECT c.id, c.tenant_id, c.email, c.first_name, c.last_name, c.phone, c.company, c.custom_fields, c.source, c.source_id, c.created_at, c.updated_at
		FROM contacts c %s
		ORDER BY c.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	var contacts []*contact.Contact
	for rows.Next() {
		c := &contact.Contact{}
		var cfBytes []byte
		var emailStr sql.NullString
		err := rows.Scan(
			&c.ID, &c.TenantID, &emailStr, &c.FirstName, &c.LastName,
			&c.Phone, &c.Company, &cfBytes, &c.Source, &c.SourceID,
			&c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		if emailStr.Valid {
			c.Email = &emailStr.String
		}
		if len(cfBytes) > 0 {
			_ = json.Unmarshal(cfBytes, &c.CustomFields)
		}
		contacts = append(contacts, c)
	}

	return contacts, rows.Err()
}

func (r *contactPostgresRepo) Update(c *contact.Contact) error {
	cfBytes, err := json.Marshal(c.CustomFields)
	if err != nil {
		return fmt.Errorf("marshal custom fields: %w", err)
	}
	if cfBytes == nil {
		cfBytes = []byte("{}")
	}

	var emailStr sql.NullString
	if c.Email != nil && *c.Email != "" {
		emailStr = sql.NullString{String: *c.Email, Valid: true}
	}

	query := `
		UPDATE contacts SET email = $1, first_name = $2, last_name = $3, phone = $4, company = $5,
			custom_fields = $6, source = $7, source_id = $8, updated_at = NOW()
		WHERE id = $9
	`

	result, err := r.pool.Exec(context.Background(), query,
		emailStr, c.FirstName, c.LastName, c.Phone, c.Company,
		cfBytes, c.Source, c.SourceID, c.ID,
	)
	if err != nil {
		return fmt.Errorf("update contact: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("contact not found: %s", c.ID)
	}

	return nil
}

func (r *contactPostgresRepo) Delete(id string) error {
	query := `DELETE FROM contacts WHERE id = $1`
	result, err := r.pool.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("contact not found: %s", id)
	}

	return nil
}

func (r *contactPostgresRepo) FindByEmail(tenantID, email string) (*contact.Contact, error) {
	c := &contact.Contact{}
	var cfBytes []byte
	var emailStr sql.NullString

	query := `
		SELECT id, tenant_id, email, first_name, last_name, phone, company, custom_fields, source, source_id, created_at, updated_at
		FROM contacts WHERE tenant_id = $1 AND email = $2
	`

	err := r.pool.QueryRow(context.Background(), query, tenantID, email).Scan(
		&c.ID, &c.TenantID, &emailStr, &c.FirstName, &c.LastName,
		&c.Phone, &c.Company, &cfBytes, &c.Source, &c.SourceID,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("find contact by email: %w", err)
	}

	if emailStr.Valid {
		c.Email = &emailStr.String
	}
	if len(cfBytes) > 0 {
		_ = json.Unmarshal(cfBytes, &c.CustomFields)
	}

	return c, nil
}

func (r *contactPostgresRepo) Merge(keepID, mergeIntoID string) error {
	return fmt.Errorf("merge contacts not yet implemented")
}

func (r *contactPostgresRepo) GetActivity(contactID string, offset, limit int) ([]contact.Activity, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, contact_id, tenant_id, type, data, source_id, created_at
		FROM activities WHERE contact_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(context.Background(), query, contactID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get activity: %w", err)
	}
	defer rows.Close()

	var activities []contact.Activity
	for rows.Next() {
		var a contact.Activity
		var dataBytes []byte
		err := rows.Scan(&a.ID, &a.ContactID, &a.TenantID, &a.Type, &dataBytes, &a.SourceID, &a.CreatedAt)
		if err != nil {
			return nil, err
		}
		if len(dataBytes) > 0 {
			_ = json.Unmarshal(dataBytes, &a.Data)
		}
		activities = append(activities, a)
	}

	return activities, rows.Err()
}

func (r *contactPostgresRepo) CountByTenant(tenantID string) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM contacts WHERE tenant_id = $1`
	err := r.pool.QueryRow(context.Background(), query, tenantID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count contacts: %w", err)
	}
	return count, nil
}
```

- [ ] **Step 2: Create postgres_test.go** (basic integration tests using Ginkgo + testcontainers, same pattern as `tenant/repo/postgres_test.go`)

- [ ] **Step 3: Verify Go compiles**
  Run: `go build ./internal/infrastructure/contact/repo/`
  Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/contact/repo/
git commit -m "feat: add contact PostgreSQL repository with CRUD operations"
```

---

### Task 3: Form Infrastructure Repository (PostgreSQL)

**Files:**
- Create: `internal/infrastructure/form/repo/postgres.go`
- Create: `internal/infrastructure/form/repo/postgres_test.go`

**Interfaces:**
- Consumes: `pgxpool.Pool`, `github.com/hekemen/automata/internal/domain/form` (Form, FormField, Submission, FileUpload)
- Produces: `*FormPostgresRepo` implementing `form.Repository`

- [ ] **Step 1: Create postgres.go**

Create `internal/infrastructure/form/repo/postgres.go` with the following content:

```go
package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/form"
	"github.com/jackc/pgx/v5/pgxpool"
)

type formPostgresRepo struct {
	pool *pgxpool.Pool
}

// NewFormPostgresRepo creates a new PostgreSQL form repository.
func NewFormPostgresRepo(pool *pgxpool.Pool) form.Repository {
	return &formPostgresRepo{pool: pool}
}

func (r *formPostgresRepo) Create(f *form.Form) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	if err := f.Validate(); err != nil {
		return err
	}

	fieldsBytes, err := json.Marshal(f.Fields)
	if err != nil {
		return fmt.Errorf("marshal fields: %w", err)
	}

	settingsBytes, err := json.Marshal(f.Settings)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if settingsBytes == nil {
		settingsBytes = []byte("{}")
	}

	query := `
		INSERT INTO forms (id, tenant_id, slug, name, description, fields, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err = r.pool.QueryRow(context.Background(), query,
		f.ID, f.TenantID, f.Slug, f.Name, f.Description, fieldsBytes, settingsBytes,
	).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create form: %w", err)
	}
	return nil
}

func (r *formPostgresRepo) GetByID(id string) (*form.Form, error) {
	f := &form.Form{}
	var fieldsBytes, settingsBytes []byte

	query := `
		SELECT id, tenant_id, slug, name, description, fields, settings, created_at, updated_at
		FROM forms WHERE id = $1
	`

	err := r.pool.QueryRow(context.Background(), query, id).Scan(
		&f.ID, &f.TenantID, &f.Slug, &f.Name, &f.Description,
		&fieldsBytes, &settingsBytes, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get form by ID: %w", err)
	}

	if len(fieldsBytes) > 0 {
		_ = json.Unmarshal(fieldsBytes, &f.Fields)
	}
	if len(settingsBytes) > 0 {
		_ = json.Unmarshal(settingsBytes, &f.Settings)
	}

	return f, nil
}

func (r *formPostgresRepo) GetBySlug(tenantID, slug string) (*form.Form, error) {
	f := &form.Form{}
	var fieldsBytes, settingsBytes []byte

	query := `
		SELECT id, tenant_id, slug, name, description, fields, settings, created_at, updated_at
		FROM forms WHERE tenant_id = $1 AND slug = $2
	`

	err := r.pool.QueryRow(context.Background(), query, tenantID, slug).Scan(
		&f.ID, &f.TenantID, &f.Slug, &f.Name, &f.Description,
		&fieldsBytes, &settingsBytes, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get form by slug: %w", err)
	}

	if len(fieldsBytes) > 0 {
		_ = json.Unmarshal(fieldsBytes, &f.Fields)
	}
	if len(settingsBytes) > 0 {
		_ = json.Unmarshal(settingsBytes, &f.Settings)
	}

	return f, nil
}

func (r *formPostgresRepo) List(tenantID string) ([]*form.Form, error) {
	query := `
		SELECT id, tenant_id, slug, name, description, fields, settings, created_at, updated_at
		FROM forms WHERE tenant_id = $1 ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(context.Background(), query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list forms: %w", err)
	}
	defer rows.Close()

	var forms []*form.Form
	for rows.Next() {
		f := &form.Form{}
		var fieldsBytes, settingsBytes []byte
		err := rows.Scan(
			&f.ID, &f.TenantID, &f.Slug, &f.Name, &f.Description,
			&fieldsBytes, &settingsBytes, &f.CreatedAt, &f.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan form: %w", err)
		}
		if len(fieldsBytes) > 0 {
			_ = json.Unmarshal(fieldsBytes, &f.Fields)
		}
		if len(settingsBytes) > 0 {
			_ = json.Unmarshal(settingsBytes, &f.Settings)
		}
		forms = append(forms, f)
	}

	return forms, rows.Err()
}

func (r *formPostgresRepo) Update(f *form.Form) error {
	fieldsBytes, err := json.Marshal(f.Fields)
	if err != nil {
		return fmt.Errorf("marshal fields: %w", err)
	}

	settingsBytes, err := json.Marshal(f.Settings)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if settingsBytes == nil {
		settingsBytes = []byte("{}")
	}

	query := `
		UPDATE forms SET slug = $1, name = $2, description = $3, fields = $4, settings = $5, updated_at = NOW()
		WHERE id = $6
	`

	result, err := r.pool.Exec(context.Background(), query,
		f.Slug, f.Name, f.Description, fieldsBytes, settingsBytes, f.ID,
	)
	if err != nil {
		return fmt.Errorf("update form: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("form not found: %s", f.ID)
	}

	return nil
}

func (r *formPostgresRepo) Delete(id string) error {
	query := `DELETE FROM forms WHERE id = $1`
	result, err := r.pool.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("delete form: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("form not found: %s", id)
	}

	return nil
}

func (r *formPostgresRepo) CreateSubmission(s *form.Submission) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}

	dataBytes, err := json.Marshal(s.Data)
	if err != nil {
		return fmt.Errorf("marshal submission data: %w", err)
	}

	query := `
		INSERT INTO form_submissions (id, form_id, tenant_id, data, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, created_at
	`

	err = r.pool.QueryRow(context.Background(), query,
		s.ID, s.FormID, s.TenantID, dataBytes,
	).Scan(&s.ID, &s.CreatedAt)

	if err != nil {
		return fmt.Errorf("create submission: %w", err)
	}
	return nil
}

func (r *formPostgresRepo) GetSubmission(id string) (*form.Submission, error) {
	s := &form.Submission{}
	var dataBytes []byte

	query := `
		SELECT id, form_id, tenant_id, data, created_at
		FROM form_submissions WHERE id = $1
	`

	err := r.pool.QueryRow(context.Background(), query, id).Scan(
		&s.ID, &s.FormID, &s.TenantID, &dataBytes, &s.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get submission: %w", err)
	}

	if len(dataBytes) > 0 {
		_ = json.Unmarshal(dataBytes, &s.Data)
	}

	return s, nil
}

func (r *formPostgresRepo) ListSubmissions(formID string, opts form.ListSubmissionsOptions) ([]*form.Submission, int64, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}

	// Get total count
	var total int64
	countQuery := `SELECT COUNT(*) FROM form_submissions WHERE form_id = $1`
	err := r.pool.QueryRow(context.Background(), countQuery, formID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count submissions: %w", err)
	}

	query := `
		SELECT id, form_id, tenant_id, data, created_at
		FROM form_submissions WHERE form_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(context.Background(), query, formID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list submissions: %w", err)
	}
	defer rows.Close()

	var submissions []*form.Submission
	for rows.Next() {
		s := &form.Submission{}
		var dataBytes []byte
		err := rows.Scan(&s.ID, &s.FormID, &s.TenantID, &dataBytes, &s.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		if len(dataBytes) > 0 {
			_ = json.Unmarshal(dataBytes, &s.Data)
		}
		submissions = append(submissions, s)
	}

	return submissions, total, rows.Err()
}
```

- [ ] **Step 2: Verify Go compiles**
  Run: `go build ./internal/infrastructure/form/repo/`
  Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/form/repo/
git commit -m "feat: add form PostgreSQL repository with CRUD and submissions"
```

---

### Task 4: Contact API Handler

**Files:**
- Create: `internal/adapter/api/handler/contact_handler.go`

**Interfaces:**
- Consumes: `contact.Repository`, `tenant.Repository` (for tenant_id from context)
- Produces: `*ContactHandler` with methods: `List`, `Create`, `Get`, `Update`, `Delete`, `GetActivity`
- Routes: `GET/POST /api/tenant/:slug/contacts`, `GET/PUT/DELETE /api/tenant/:slug/contacts/:id`, `GET /api/tenant/:slug/contacts/:id/activity`

- [ ] **Step 1: Create contact_handler.go**

Create `internal/adapter/api/handler/contact_handler.go`:

```go
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/contact"
	"github.com/hekemen/automata/internal/domain/tenant"
	"github.com/hekemen/automata/internal/adapter/api"
)

type ContactHandler struct {
	repo       contact.Repository
	tenantRepo tenant.Repository
}

func NewContactHandler(repo contact.Repository, tenantRepo tenant.Repository) *ContactHandler {
	return &ContactHandler{repo: repo, tenantRepo: tenantRepo}
}

func (h *ContactHandler) List(c *gin.Context) {
	t, exists := c.Get(string(api.TenantContextKey))
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found"})
		return
	}
	tenant := t.(*tenant.Tenant)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	filters := contact.FilterOptions{
		Search:  c.Query("search"),
		Source:  c.Query("source"),
		Company: c.Query("company"),
		Tags:    c.QueryArray("tags"),
	}

	contacts, err := h.repo.List(offset, limit, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	total, _ := h.repo.CountByTenant(tenant.ID)

	c.JSON(http.StatusOK, gin.H{
		"data":  contacts,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *ContactHandler) Create(c *gin.Context) {
	t, exists := c.Get(string(api.TenantContextKey))
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found"})
		return
	}
	tenant := t.(*tenant.Tenant)

	var req struct {
		FirstName    string                 `json:"first_name" binding:"required"`
		LastName     string                 `json:"last_name" binding:"required"`
		Email        string                 `json:"email"`
		Phone        string                 `json:"phone"`
		Company      string                 `json:"company"`
		Source       string                 `json:"source"`
		SourceID     string                 `json:"source_id"`
		CustomFields map[string]interface{} `json:"custom_fields"`
		Tags         []struct {
			Name  string `json:"name" binding:"required"`
			Color string `json:"color"`
		} `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var emailPtr *string
	if req.Email != "" {
		emailPtr = &req.Email
	}

	var tags []contact.Tag
	for _, t := range req.Tags {
		tags = append(tags, contact.Tag{
			Name:  t.Name,
			Color: t.Color,
		})
	}

	contact := &contact.Contact{
		TenantID:     tenant.ID,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Email:        emailPtr,
		Phone:        req.Phone,
		Company:      req.Company,
		Source:       req.Source,
		SourceID:     req.SourceID,
		CustomFields: req.CustomFields,
		Tags:         tags,
	}

	if err := h.repo.Create(contact); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, contact)
}

func (h *ContactHandler) Get(c *gin.Context) {
	id := c.Param("id")

	contact, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
		return
	}

	c.JSON(http.StatusOK, contact)
}

func (h *ContactHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		FirstName    string                 `json:"first_name"`
		LastName     string                 `json:"last_name"`
		Email        string                 `json:"email"`
		Phone        string                 `json:"phone"`
		Company      string                 `json:"company"`
		Source       string                 `json:"source"`
		SourceID     string                 `json:"source_id"`
		CustomFields map[string]interface{} `json:"custom_fields"`
		Tags         []contact.Tag          `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contact, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
		return
	}

	if req.FirstName != "" {
		contact.FirstName = req.FirstName
	}
	if req.LastName != "" {
		contact.LastName = req.LastName
	}
	if req.Email != "" {
		contact.Email = &req.Email
	}
	if req.Phone != "" {
		contact.Phone = req.Phone
	}
	if req.Company != "" {
		contact.Company = req.Company
	}
	if req.Source != "" {
		contact.Source = req.Source
	}
	if req.SourceID != "" {
		contact.SourceID = req.SourceID
	}
	if req.CustomFields != nil {
		contact.CustomFields = req.CustomFields
	}
	if len(req.Tags) > 0 {
		contact.Tags = req.Tags
	}

	if err := h.repo.Update(contact); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, contact)
}

func (h *ContactHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "contact deleted"})
}

func (h *ContactHandler) GetActivity(c *gin.Context) {
	id := c.Param("id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	activities, err := h.repo.GetActivity(id, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": activities,
		"page": page,
		"limit": limit,
	})
}
```

- [ ] **Step 2: Register routes in server.go**
  Add to `internal/adapter/api/server.go` under tenant routes:

```go
// Contact routes
contacts := tenantGroup.Group("/contacts")
{
	contacts.GET("", contactHandler.List)
	contacts.POST("", contactHandler.Create)
	contacts.GET("/:id", contactHandler.Get)
	contacts.PUT("/:id", contactHandler.Update)
	contacts.DELETE("/:id", contactHandler.Delete)
	contacts.GET("/:id/activity", contactHandler.GetActivity)
}
```

- [ ] **Step 3: Verify Go compiles**
  Run: `go build ./internal/adapter/api/...`
  Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/api/handler/contact_handler.go
git add internal/adapter/api/server.go
git commit -m "feat: add contact API handler with CRUD endpoints"
```

---

### Task 5: Form API Handler

**Files:**
- Create: `internal/adapter/api/handler/form_handler.go`

**Interfaces:**
- Consumes: `form.Repository`, `tenant.Repository` (for tenant_id from context)
- Produces: `*FormHandler` with methods: `List`, `Create`, `Get`, `Update`, `Delete`, `GetBySlug`, `Submit`, `ListSubmissions`
- Routes: `GET/POST /api/tenant/:slug/forms`, `GET/PUT/DELETE /api/tenant/:slug/forms/:id`, `POST /api/tenant/:slug/forms/slug/:slug/submit`, `GET /api/tenant/:slug/forms/:id/submissions`

- [ ] **Step 1: Create form_handler.go**

Create `internal/adapter/api/handler/form_handler.go`:

```go
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/form"
	"github.com/hekemen/automata/internal/domain/tenant"
	"github.com/hekemen/automata/internal/adapter/api"
)

type FormHandler struct {
	repo       form.Repository
	tenantRepo tenant.Repository
}

func NewFormHandler(repo form.Repository, tenantRepo tenant.Repository) *FormHandler {
	return &FormHandler{repo: repo, tenantRepo: tenantRepo}
}

func (h *FormHandler) List(c *gin.Context) {
	t, exists := c.Get(string(api.TenantContextKey))
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found"})
		return
	}
	tenant := t.(*tenant.Tenant)

	forms, err := h.repo.List(tenant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": forms,
	})
}

func (h *FormHandler) Create(c *gin.Context) {
	t, exists := c.Get(string(api.TenantContextKey))
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found"})
		return
	}
	tenant := t.(*tenant.Tenant)

	var req struct {
		Slug        string                 `json:"slug" binding:"required"`
		Name        string                 `json:"name" binding:"required"`
		Description string                 `json:"description"`
		Fields      []form.FormField       `json:"fields"`
		Settings    map[string]interface{} `json:"settings"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	f := &form.Form{
		TenantID:    tenant.ID,
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		Fields:      req.Fields,
		Settings:    req.Settings,
	}

	if err := h.repo.Create(f); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, f)
}

func (h *FormHandler) Get(c *gin.Context) {
	id := c.Param("id")

	f, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}

	c.JSON(http.StatusOK, f)
}

func (h *FormHandler) GetBySlug(c *gin.Context) {
	t, exists := c.Get(string(api.TenantContextKey))
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found"})
		return
	}
	tenant := t.(*tenant.Tenant)

	slug := c.Param("slug")

	f, err := h.repo.GetBySlug(tenant.ID, slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}

	c.JSON(http.StatusOK, f)
}

func (h *FormHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Slug        string                 `json:"slug"`
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Fields      []form.FormField       `json:"fields"`
		Settings    map[string]interface{} `json:"settings"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	f, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}

	if req.Slug != "" {
		f.Slug = req.Slug
	}
	if req.Name != "" {
		f.Name = req.Name
	}
	if req.Description != "" {
		f.Description = req.Description
	}
	if len(req.Fields) > 0 {
		f.Fields = req.Fields
	}
	if req.Settings != nil {
		f.Settings = req.Settings
	}

	if err := h.repo.Update(f); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, f)
}

func (h *FormHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "form deleted"})
}

func (h *FormHandler) Submit(c *gin.Context) {
	slug := c.Param("slug")

	t, exists := c.Get(string(api.TenantContextKey))
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found"})
		return
	}
	tenant := t.(*tenant.Tenant)

	var req struct {
		Data map[string]interface{} `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	f, err := h.repo.GetBySlug(tenant.ID, slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}

	s := &form.Submission{
		FormID:   f.ID,
		TenantID: tenant.ID,
		Data:     req.Data,
	}

	if err := h.repo.CreateSubmission(s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, s)
}

func (h *FormHandler) ListSubmissions(c *gin.Context) {
	id := c.Param("id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	opts := form.ListSubmissionsOptions{
		Limit:  limit,
		Offset: offset,
	}

	submissions, total, err := h.repo.ListSubmissions(id, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  submissions,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}
```

- [ ] **Step 2: Register routes in server.go**
  Add to `internal/adapter/api/server.go` under tenant routes:

```go
// Form routes
forms := tenantGroup.Group("/forms")
{
	forms.GET("", formHandler.List)
	forms.POST("", formHandler.Create)
	forms.GET("/:id", formHandler.Get)
	forms.PUT("/:id", formHandler.Update)
	forms.DELETE("/:id", formHandler.Delete)
	forms.POST("/slug/:slug/submit", formHandler.Submit)
	forms.GET("/:id/submissions", formHandler.ListSubmissions)
}
```

- [ ] **Step 3: Verify Go compiles**
  Run: `go build ./internal/adapter/api/...`
  Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/api/handler/form_handler.go
git add internal/adapter/api/server.go
git commit -m "feat: add form API handler with CRUD and submission endpoints"
```

---

### Task 6: Vue Frontend - Tenants Page

**Files:**
- Create: `webui/src/views/Tenants.vue`
- Create: `webui/src/components/tenants/TenantForm.vue`
- Create: `webui/src/api/tenants.ts`

**Interfaces:**
- Consumes: Existing auth store, API client
- Produces: Tenants list view with CRUD operations

- [ ] **Step 1: Create tenants API module**

Create `webui/src/api/tenants.ts`:

```typescript
import { get, post, put, del } from './client'

export interface Tenant {
  id: string
  slug: string
  name: string
  created_at: string
  updated_at: string
}

export const tenantsApi = {
  list: () => get<Tenant[]>('/api/tenants'),
  create: (data: Partial<Tenant>) => post<Tenant>('/api/tenants', data),
  update: (id: string, data: Partial<Tenant>) => put<Tenant>(`/api/tenants/${id}`, data),
  delete: (id: string) => del(`/api/tenants/${id}`),
}
```

- [ ] **Step 2: Create Tenants.vue page**

Create `webui/src/views/Tenants.vue` with:
- Table listing all tenants (id, slug, name, created_at)
- "Create Tenant" button opening a dialog
- Edit and Delete actions per row
- Uses `TenantForm` component for create/edit

- [ ] **Step 3: Create TenantForm.vue component**

Create `webui/src/components/tenants/TenantForm.vue` with:
- Form fields: slug, name
- Validation: slug must be unique, non-empty
- Submit calls `tenantsApi.create` or `tenantsApi.update`

- [ ] **Step 4: Commit**

```bash
git add webui/src/views/Tenants.vue
git add webui/src/components/tenants/TenantForm.vue
git add webui/src/api/tenants.ts
git commit -m "feat: add Tenants page with CRUD UI"
```

---

### Task 7: Vue Frontend - Contacts Page (Wired to API)

**Files:**
- Modify: `webui/src/views/Contacts.vue`
- Create: `webui/src/api/contacts.ts`
- Create: `webui/src/components/contacts/ContactForm.vue`

**Interfaces:**
- Consumes: `contactsApi` from `webui/src/api/contacts.ts`
- Produces: Contacts list view with real API integration

- [ ] **Step 1: Create contacts API module**

Create `webui/src/api/contacts.ts`:

```typescript
import { get, post, put, del } from './client'

export interface Contact {
  id: string
  tenant_id: string
  email: string | null
  first_name: string
  last_name: string
  phone: string
  company: string
  custom_fields: Record<string, unknown>
  source: string
  source_id: string
  tags: Tag[]
  created_at: string
  updated_at: string
}

export interface Tag {
  id: string
  tenant_id: string
  name: string
  color: string
  created_at: string
}

export interface Activity {
  id: string
  contact_id: string
  tenant_id: string
  type: string
  data: Record<string, unknown>
  source_id: string
  created_at: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  limit: number
}

export const contactsApi = {
  list: (params: { page?: number; limit?: number; search?: string; source?: string; company?: string; tags?: string[] }) =>
    get<PaginatedResponse<Contact>>('/api/contacts', { params }),
  create: (data: Partial<Contact>) => post<Contact>('/api/contacts', data),
  update: (id: string, data: Partial<Contact>) => put<Contact>(`/api/contacts/${id}`, data),
  delete: (id: string) => del(`/api/contacts/${id}`),
  getActivity: (id: string, params?: { page?: number; limit?: number }) =>
    get<PaginatedResponse<Activity>>(`/api/contacts/${id}/activity`, { params }),
}
```

- [ ] **Step 2: Update Contacts.vue to use API**

Modify `webui/src/views/Contacts.vue` to:
- Replace mock data with `contactsApi.list()` calls
- Add search, filter, and pagination controls
- Wire "Create Contact" button to open `ContactForm` dialog
- Wire edit/delete actions to API calls

- [ ] **Step 3: Create ContactForm.vue component**

Create `webui/src/components/contacts/ContactForm.vue` with:
- Form fields: first_name, last_name, email, phone, company, source, custom_fields, tags
- Validation: email format, required fields
- Submit calls `contactsApi.create` or `contactsApi.update`

- [ ] **Step 4: Commit**

```bash
git add webui/src/views/Contacts.vue
git add webui/src/api/contacts.ts
git add webui/src/components/contacts/ContactForm.vue
git commit -m "feat: wire Contacts page to real API endpoints"
```

---

### Task 8: Vue Frontend - Forms Page

**Files:**
- Create: `webui/src/views/Forms.vue`
- Create: `webui/src/components/forms/FormBuilder.vue`
- Create: `webui/src/api/forms.ts`

**Interfaces:**
- Consumes: `formsApi` from `webui/src/api/forms.ts`
- Produces: Forms list view with CRUD and form builder

- [ ] **Step 1: Create forms API module**

Create `webui/src/api/forms.ts`:

```typescript
import { get, post, put, del } from './client'

export interface FormField {
  id: string
  name: string
  label: string
  type: 'text' | 'email' | 'number' | 'textarea' | 'select' | 'checkbox' | 'date'
  required: boolean
  options?: string[] // For select type
}

export interface Form {
  id: string
  tenant_id: string
  slug: string
  name: string
  description: string
  fields: FormField[]
  settings: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface Submission {
  id: string
  form_id: string
  tenant_id: string
  data: Record<string, unknown>
  created_at: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  limit: number
}

export const formsApi = {
  list: () => get<PaginatedResponse<Form>>('/api/forms'),
  create: (data: Partial<Form>) => post<Form>('/api/forms', data),
  update: (id: string, data: Partial<Form>) => put<Form>(`/api/forms/${id}`, data),
  delete: (id: string) => del(`/api/forms/${id}`),
  getBySlug: (slug: string) => get<Form>(`/api/forms/slug/${slug}`),
  submit: (slug: string, data: Record<string, unknown>) =>
    post<Submission>(`/api/forms/slug/${slug}/submit`, data),
  listSubmissions: (id: string, params?: { page?: number; limit?: number }) =>
    get<PaginatedResponse<Submission>>(`/api/forms/${id}/submissions`, { params }),
}
```

- [ ] **Step 2: Create Forms.vue page**

Create `webui/src/views/Forms.vue` with:
- Table listing all forms (slug, name, description, created_at)
- "Create Form" button opening a dialog
- Edit and Delete actions per row
- "View Submissions" action to see form responses

- [ ] **Step 3: Create FormBuilder.vue component**

Create `webui/src/components/forms/FormBuilder.vue` with:
- Dynamic form field builder (add/remove fields)
- Field type selector (text, email, number, textarea, select, checkbox, date)
- Field configuration (name, label, required, options)
- Preview mode to test form
- Submit calls `formsApi.create` or `formsApi.update`

- [ ] **Step 4: Commit**

```bash
git add webui/src/views/Forms.vue
git add webui/src/components/forms/FormBuilder.vue
git add webui/src/api/forms.ts
git commit -m "feat: add Forms page with form builder UI"
```

---

### Task 9: Vue Frontend - API Keys Page

**Files:**
- Create: `webui/src/views/APIKeys.vue`
- Create: `webui/src/components/apikeys/ApiKeyForm.vue`
- Create: `webui/src/api/apikeys.ts`

**Interfaces:**
- Consumes: Existing API keys endpoints (if available) or creates new ones
- Produces: API Keys list view with CRUD

- [ ] **Step 1: Create API keys API module**

Create `webui/src/api/apikeys.ts`:

```typescript
import { get, post, put, del } from './client'

export interface APIKey {
  id: string
  key: string
  name: string
  permissions: string[]
  created_at: string
  expires_at: string | null
  last_used_at: string | null
}

export const apiKeysApi = {
  list: () => get<APIKey[]>('/api/api-keys'),
  create: (data: { name: string; permissions: string[]; expires_in_days?: number }) =>
    post<APIKey>('/api/api-keys', data),
  revoke: (id: string) => del(`/api/api-keys/${id}`),
}
```

- [ ] **Step 2: Create APIKeys.vue page**

Create `webui/src/views/APIKeys.vue` with:
- Table listing all API keys (name, permissions, created_at, last_used_at)
- "Create API Key" button opening a dialog
- Revoke action per key
- Display full key only on creation (copy to clipboard)

- [ ] **Step 3: Create ApiKeyForm.vue component**

Create `webui/src/components/apikeys/ApiKeyForm.vue` with:
- Form fields: name, permissions (multi-select), expiry (optional)
- Submit calls `apiKeysApi.create`

- [ ] **Step 4: Commit**

```bash
git add webui/src/views/APIKeys.vue
git add webui/src/components/apikeys/ApiKeyForm.vue
git add webui/src/api/apikeys.ts
git commit -m "feat: add API Keys page with CRUD UI"
```

---

### Task 10: Update Sidebar Navigation

**Files:**
- Modify: `webui/src/components/layout/Sidebar.vue`

- [ ] **Step 1: Add new navigation items**

Update `webui/src/components/layout/Sidebar.vue` to include:
- Tenants (if not already present)
- Contacts (already present, verify link)
- Forms
- API Keys

- [ ] **Step 2: Commit**

```bash
git add webui/src/components/layout/Sidebar.vue
git commit -m "feat: update sidebar navigation with Tenants, Forms, API Keys"
```

---

### Task 11: Integration Testing

**Files:**
- Create: `webui/tests/e2e/tenants.spec.ts`
- Create: `webui/tests/e2e/contacts.spec.ts`
- Create: `webui/tests/e2e/forms.spec.ts`
- Create: `webui/tests/e2e/apikeys.spec.ts`

- [ ] **Step 1: Write E2E tests for Tenants**

Create `webui/tests/e2e/tenants.spec.ts`:
- Login as admin
- Navigate to Tenants page
- Verify tenant list loads
- Create a new tenant
- Verify tenant appears in list
- Edit tenant
- Delete tenant

- [ ] **Step 2: Write E2E tests for Contacts**

Create `webui/tests/e2e/contacts.spec.ts`:
- Login as admin
- Navigate to Contacts page
- Verify contacts list loads
- Create a new contact
- Search/filter contacts
- Edit contact
- Delete contact

- [ ] **Step 3: Write E2E tests for Forms**

Create `webui/tests/e2e/forms.spec.ts`:
- Login as admin
- Navigate to Forms page
- Create a new form with fields
- Submit a test response
- View submissions

- [ ] **Step 4: Write E2E tests for API Keys**

Create `webui/tests/e2e/apikeys.spec.ts`:
- Login as admin
- Navigate to API Keys page
- Create a new API key
- Revoke API key

- [ ] **Step 5: Run tests**

```bash
cd webui && yarn test:e2e
```

- [ ] **Step 6: Commit**

```bash
git add webui/tests/e2e/
git commit -m "feat: add E2E tests for Tenants, Contacts, Forms, API Keys"
```

---

## Summary of Deliverables

### Backend (Go)
1. Database schema for contacts, tags, forms, submissions, activities
2. Contact PostgreSQL repository with full CRUD
3. Form PostgreSQL repository with CRUD and submissions
4. Contact API handler with REST endpoints
5. Form API handler with REST endpoints
6. Route registration in server.go

### Frontend (Vue)
1. Tenants page with CRUD
2. Contacts page wired to real API
3. Forms page with form builder
4. API Keys page with CRUD
5. Updated sidebar navigation
6. E2E tests for all pages

### API Endpoints
- `GET/POST /api/tenant/:slug/contacts`
- `GET/PUT/DELETE /api/tenant/:slug/contacts/:id`
- `GET /api/tenant/:slug/contacts/:id/activity`
- `GET/POST /api/tenant/:slug/forms`
- `GET/PUT/DELETE /api/tenant/:slug/forms/:id`
- `POST /api/tenant/:slug/forms/slug/:slug/submit`
- `GET /api/tenant/:slug/forms/:id/submissions`

---

## Rollback Plan

If any task fails or causes issues:

1. **Database migrations**: Keep migration SQL as additive-only (CREATE TABLE IF NOT EXISTS). To rollback, run DROP TABLE in reverse order.
2. **Go code**: Each task is committed separately. Use `git revert <commit-hash>` to undo individual tasks.
3. **Vue code**: Each page is a separate commit. Use `git revert` to undo.
4. **Docker Compose**: No changes to docker-compose.yml required. Services run as-is.

## Verification Checklist

- [ ] All Go code compiles: `go build ./...`
- [ ] All Go tests pass: `go test ./...`
- [ ] Vue builds successfully: `cd webui && yarn build`
- [ ] Vue type checking passes: `cd webui && yarn lint`
- [ ] E2E tests pass: `cd webui && yarn test:e2e`
- [ ] API endpoints respond correctly (manual or automated testing)
- [ ] Database migrations apply cleanly
- [ ] All new files follow existing code style

