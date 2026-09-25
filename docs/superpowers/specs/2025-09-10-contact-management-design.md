# Design: Contact Management

## Problem Statement

Marketing automation platforms need a robust contact database to store, search, segment, and manage leads and customers. Without a structured contact system, organizations cannot effectively track their audience, personalize communications, or measure campaign results. Contact data must support deduplication, custom fields for flexible data modeling, tag-based segmentation, CSV import/export for data migration, and an activity timeline that aggregates events from forms, web tracking, and banner interactions.

## Goals

1. Store contact records with structured fields and JSONB custom fields for extensibility
2. Deduplicate contacts by email address (nil email = unverified)
3. Support tag-based segmentation with tag membership junction table
4. Provide CSV import with field mapping and CSV export
5. Aggregate activity timeline from multiple sources (form submissions, page visits, banner clicks)
6. Enable contact merging to combine duplicate records
7. Expose contacts via REST API and MCP tools for AI integration
8. All data strictly isolated per context (tenant)

## Non-Goals

- Email nurturing campaigns or drip sequences
- Advanced lead scoring algorithms
- Social media profile enrichment
- Contact deduplication via fuzzy matching (exact email only)
- Real-time contact synchronization with external CRMs

## Architecture

### Component Diagram

```
┌──────────────────────────────────────────────────────────────┐
│                      API / MCP Layer                         │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────┐  │
│  │ ContactHandler│  │ContactTools  │  │ FormHandler       │  │
│  │ (REST API)    │  │ (MCP tools)  │  │ (form→contact)    │  │
│  └──────┬───────┘  └──────┬───────┘  └────────┬──────────┘  │
│         └─────────────────┼───────────────────┘             │
│                   ┌───────┴───────┐                         │
│                   │ Use Cases     │                         │
│                   │ CRUD, merge,  │                         │
│                   │ segment, CSV  │                         │
│                   └───────┬───────┘                         │
│                           │                                 │
│  ┌────────────────────────┼────────────────────────┐       │
│  │ Domain Layer         │ Infrastructure          │       │
│  │ ┌──────────────┐     │ ┌──────────────────┐    │       │
│  │ │ Contact      │     │ │ PostgreSQL Repo  │    │       │
│  │ │ Entity       │     │ │ (migration.sql)  │    │       │
│  │ │ + Tag        │     │ │ (contact/, repo/)│    │       │
│  │ │ + Activity   │     │ └──────────────────┘    │       │
│  │ └──────────────┘     │                          │       │
│  │ ┌──────────────┐     │ ┌──────────────────┐    │       │
│  │ │ Repository   │     │ │ pkg/import/      │    │       │
│  │ │ Interface    │     │ │ CSV Importer     │    │       │
│  │ └──────────────┘     │ └──────────────────┘    │       │
│  └──────────────────────────────────────────────────┘       │
│  ┌──────────────────────────────────────────────────┐       │
│  │ pkg/export/ CSV Exporter                          │       │
│  └──────────────────────────────────────────────────┘       │
└──────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
internal/
  domain/contact/
    contact.go         # Contact entity with custom fields as JSONB
    tag.go             # Tag entity (name, color)
    activity.go        # Activity entity (form_submission, page_visit, banner_click)
    repository.go      # Repository interface with CRUD, merge, activity, filters
  usecase/contact/
    create_contact.go  # Create with dedup check
    list_contacts.go   # List with pagination and filters
    update_contact.go  # Update with email conflict check
    delete_contact.go  # Soft/hard delete
    merge_contacts.go  # Merge two contacts (JSONB merge, tag merge)
    get_activity.go    # Activity timeline aggregation
    segment_contacts.go # Segment by filters
    import_contacts.go # CSV import orchestration
    export_contacts.go # CSV export orchestration
    tags.go            # Tag management
  infrastructure/contact/
    migration.sql      # contacts, contact_tags, contact_tag_memberships, contact_field_definitions
    repo/postgres.go   # PostgreSQL implementation
  adapter/api/handler/
    contact_handler.go # REST API endpoints
  adapter/mcp/
    contacts_tools.go  # MCP tools for AI integration
pkg/
  import/csv_importer.go    # CSV parsing with field mapping
  export/csv_exporter.go    # CSV generation
```

### Multi-Context Isolation

All feature tables include a `context_id` column with a foreign key to `contexts(id)`. Queries always filter by `context_id` to ensure strict data isolation. The context is extracted from the JWT token and set in the Gin context by `ContextResolver` middleware.

### Contact Merge Strategy

When merging two contacts:
1. JSONB merge of `custom_fields`: keep non-null values from the kept contact
2. UNION of tags from both contacts
3. Reassign all activity records from the merged contact to the kept contact
4. Delete the merged contact record

### Deduplication

- Contacts with an email address are deduplicated: `CREATE UNIQUE INDEX ... WHERE email IS NOT NULL`
- Contacts without an email are created but marked as "unverified" (email = NULL)
- No fuzzy matching — only exact email match

## Data Model

### Tables

```sql
-- Contacts: core contact record
CREATE TABLE contacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id      UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    email           VARCHAR(254),
    first_name      VARCHAR(128),
    last_name       VARCHAR(128),
    phone           VARCHAR(32),
    company         VARCHAR(256),
    custom_fields   JSONB DEFAULT '{}',
    source          VARCHAR(64),       -- form, tracking, manual, import
    source_id       VARCHAR(128),      -- originating form submission or visitor ID
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_contacts_context_email
    ON contacts(context_id, email) WHERE email IS NOT NULL;
CREATE INDEX idx_contacts_context_created
    ON contacts(context_id, created_at DESC);
CREATE INDEX idx_contacts_context_source
    ON contacts(context_id, source);

-- Tags: organizational labels for contacts
CREATE TABLE contact_tags (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    color       VARCHAR(7) DEFAULT '#6366f1',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_tags_context_name
    ON contact_tags(context_id, name);

-- Contact-Tag junction
CREATE TABLE contact_tag_memberships (
    contact_id  UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    tag_id      UUID NOT NULL REFERENCES contact_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (contact_id, tag_id)
);

-- Custom fields definition (per context)
CREATE TABLE contact_field_definitions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL,
    key         TEXT NOT NULL,
    label       TEXT NOT NULL,
    type        VARCHAR(32) NOT NULL,
    options     JSONB,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_field_defs_context_key
    ON contact_field_definitions(context_id, key);
```

### Domain Entities

```go
// Contact represents a contact/lead/customer record
type Contact struct {
    ID           string
    ContextID    string
    Email        *string  // nil = unverified
    FirstName    string
    LastName     string
    Phone        string
    Company      string
    CustomFields map[string]interface{}  // JSONB storage
    Source       string  // form, tracking, manual, import
    SourceID     string
    Tags         []Tag
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// Tag is an organizational label for contacts
type Tag struct {
    ID        string
    ContextID string
    Name      string
    Color     string
    CreatedAt time.Time
}

// Activity represents a tracked interaction
type Activity struct {
    ID        string
    ContactID string
    ContextID string
    Type      ActivityType  // form_submission, page_visit, banner_click, contact_created, contact_updated, contact_merged
    Data      map[string]interface{}
    SourceID  string
    CreatedAt time.Time
}
```

## API Contracts

### REST API

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `GET` | `/api/contacts` | List contacts (paginated, filterable) | Bearer token |
| `GET` | `/api/contacts/:id` | Get contact details with tags | Bearer token |
| `POST` | `/api/contacts` | Create contact (deduplicates by email) | Bearer token |
| `PUT` | `/api/contacts/:id` | Update contact | Bearer token |
| `DELETE` | `/api/contacts/:id` | Delete contact | Bearer token |
| `POST` | `/api/contacts/:id/merge` | Merge two contacts | Bearer token |
| `GET` | `/api/contacts/:id/activity` | Get activity timeline | Bearer token |
| `POST` | `/api/contacts/import` | Import contacts from CSV (multipart) | Bearer token |
| `GET` | `/api/contacts/export` | Export contacts to CSV | Bearer token |
| `GET` | `/api/contacts/tags` | List tags for context | Bearer token |
| `POST` | `/api/contacts/tags` | Create tag | Bearer token |
| `DELETE` | `/api/contacts/tags/:id` | Delete tag | Bearer token |

### Request/Response Shapes

```json
// GET /api/contacts?tags=spring-sale&source=form&page=1&limit=20
{
  "contacts": [
    {
      "id": "uuid",
      "email": "john@example.com",
      "first_name": "John",
      "last_name": "Doe",
      "company": "Acme Corp",
      "custom_fields": {"loyalty_tier": "gold"},
      "source": "form",
      "tags": [{"id": "uuid", "name": "spring-sale", "color": "#6366f1"}],
      "created_at": "2026-01-15T10:30:00Z"
    }
  ],
  "total": 150,
  "page": 1,
  "limit": 20
}

// POST /api/contacts
{
  "email": "new@example.com",
  "first_name": "Jane",
  "last_name": "Smith",
  "company": "Widget Inc",
  "custom_fields": {"referral_source": "google"},
  "tags": ["q4-leads"]
}
→ 200 { "contact": { ... }, "created": false }  // returned existing if dedup

// POST /api/contacts/:id/merge
{ "merge_with": "uuid-of-other-contact" }
→ 200 { "contact": { ... }, "merged_from": "uuid" }
```

### MCP Tools

```
contacts.list(context_id, tags[], source, page, limit) → Contact[]
contacts.get(context_id, id) → Contact
contacts.create(context_id, data) → Contact
contacts.update(context_id, id, data) → Contact
contacts.delete(context_id, id) → bool
contacts.merge(context_id, keep_id, merge_into_id) → bool
contacts.import(context_id, csv_data, mapping) → ImportResult
contacts.export(context_id, tags[], fields[]) → string (base64 CSV)
contacts.get_activity(context_id, contact_id, offset, limit) → Activity[]
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Email as primary identifier | Email field with nullable unique constraint | Natural dedup key; supports unverified contacts with nil email |
| Custom fields storage | JSONB column | Flexible schema without migrations; type-checked at application level |
| Tag junction table | Separate `contact_tag_memberships` with PK(contact_id, tag_id) | Standard N:M pattern; allows efficient tag queries |
| Merge strategy | JSONB merge + tag UNION + activity reassignment | Preserves all data from both contacts; no data loss |
| CSV import | Field mapping + dedup per email row | Handles varied CSV formats; safe duplicate handling |
| Activity source | Source field (form/tracking/manual/import) | Enables timeline filtering; traces origin of contact |
| Context isolation | FK to contexts(id) with CASCADE | Strict multi-tenancy; automatic cleanup on context deletion |
