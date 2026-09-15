# PRD: Contact Management

## Overview

Store and manage contacts (leads/customers) collected via forms, tracking events, and manual entry. Target user: small-to-mid business owners who need a simple CRM to track their leads and customers.

## Goals

- Provide a central contact database with rich profiles
- Auto-populate contacts from form submissions and tracking events
- Segment contacts by tags, source, and behavior
- Import/export contacts via CSV
- Show activity timeline per contact

## Scope

### In Scope

- Contact CRUD: name, email, phone, company, custom fields, tags
- Contact deduplication by email address
- Segmentation: filter contacts by tags, source, form submissions, tracking behavior
- Import contacts via CSV (with field mapping)
- Export contacts to CSV
- Activity timeline: aggregate form submissions, page visits, banner clicks per contact
- Manual contact creation and editing in admin dashboard
- Contact merge for duplicates

### Out of Scope

- Email outreach / campaign tools (Phase 2+)
- Lead scoring (deferred)
- Pipeline / deal tracking (deferred)
- Team collaboration / roles (deferred)
- API access for contacts (deferred)
- Social profile enrichment (deferred)

## Architecture

Clean architecture with hexagonal layout. KISS and DRY principles.

### Layer Structure

```
internal/
  domain/contact       — Contact, ContactField, Tag, Activity (entities, value objects, domain interfaces)
  usecase/contact      — ListContacts, CreateContact, MergeContacts, SegmentContacts
internal/adapter/mcp   — MCP server exposing contact tools
internal/infrastructure/
  contact/repo         — PostgreSQL repository implementation
  config/kv            — Key-value config for custom field definitions
pkg/import             — CSV import handler (domain-agnostic)
pkg/export             — CSV export generator (domain-agnostic)
```

### Data Flow

```
Form submission / tracking event → Contact created or matched → Activity logged → Dashboard shows contact profile + timeline
Tenant manually creates contact → Stored → Visible in dashboard
Tenant imports CSV → Contacts created/updated → Activity logged
```

### Key Decisions

- **Deduplication:** Automatic deduplication by email address. If a form submission or tracking event matches an existing contact email, the activity is appended to that contact instead of creating a duplicate.
- **Custom fields:** Stored as JSONB column in PostgreSQL for flexibility. Tenant defines custom fields via UI, stored in key-value config (`contact.fields.<key>.label`, `contact.fields.<key>.type`).
- **Activity timeline:** Aggregates data from forms, tracking, and banner events. Shows a chronological list of all interactions with a contact.
- **Import:** CSV with field mapping UI. Maps CSV columns to contact fields (name, email, phone, custom fields). Duplicates handled by email match.
- **Contact identification:** Email is the primary identifier. If email is missing, a contact is still created but marked as "unverified" and not deduplicated.

## API Design

### Contact CRUD

```
GET    /api/contacts                       # List contacts (tenant-scoped, paginated)
GET    /api/contacts/<id>                  # Get contact details
POST   /api/contacts                       # Create contact
PUT    /api/contacts/<id>                  # Update contact
DELETE /api/contacts/<id>                  # Delete contact
POST   /api/contacts/<id>/merge            # Merge two contacts
GET    /api/contacts/<id>/activity         # Get activity timeline
```

### Segmentation

```
GET /api/contacts?tags=support,paid&source=form&min_submissions=3
```

### Import

```
POST /api/contacts/import
Content-Type: multipart/form-data
Body: {
  "file": <csv>,
  "mapping": {
    "first_name": "0",    // CSV column index
    "last_name": "1",
    "email": "2",
    "phone": "3",
    "company": "4"
  }
}
```

### Export

```
GET /api/contacts/export?tags=support&format=csv
Returns: CSV file download
```

## Database Schema (tentative)

```sql
-- Contacts table
CREATE TABLE contacts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    email       VARCHAR(254),                -- nullable, primary dedup key when present
    first_name  VARCHAR(128),
    last_name   VARCHAR(128),
    phone       VARCHAR(32),
    company     VARCHAR(256),
    custom_fields JSONB DEFAULT '{}',
    source      VARCHAR(64),                 -- form, tracking, manual, import
    source_id   VARCHAR(128),                -- id of originating form submission or visitor
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_contacts_tenant_email ON contacts(tenant_id, email) WHERE email IS NOT NULL;
CREATE INDEX idx_contacts_tenant_created ON contacts(tenant_id, created_at DESC);
CREATE INDEX idx_contacts_tenant_source ON contacts(tenant_id, source);

-- Tags table
CREATE TABLE contact_tags (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    name        TEXT NOT NULL,
    color       VARCHAR(7) DEFAULT '#6366f1',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_tags_tenant_name ON contact_tags(tenant_id, name);

-- Contact-tag junction
CREATE TABLE contact_tag_memberships (
    contact_id  UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    tag_id      UUID NOT NULL REFERENCES contact_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (contact_id, tag_id)
);

-- Custom fields definition (per tenant)
CREATE TABLE contact_field_definitions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    key         TEXT NOT NULL,
    label       TEXT NOT NULL,
    type        VARCHAR(32) NOT NULL,  -- text, number, select, date
    options     JSONB,                 -- for select type: ["option1", "option2"]
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_field_defs_tenant_key ON contact_field_definitions(tenant_id, key);
```

## Testing

- Unit tests for contact deduplication logic (email match, merge)
- Integration tests for CSV import with various field mappings
- Segmentation query tests (filter by tags, source, behavior)
- Activity timeline test: verify aggregation from forms, tracking, banners
- E2E tests for API and MCP layers using Ginkgo + testcontainers, executed via `cicd/` directory
- Load test: 10k contacts per tenant, list query < 500ms

## Success Criteria

- Contact deduplication by email works correctly (no duplicates for same email)
- CSV import handles 5000 contacts in < 30 seconds
- Segmentation queries return results in < 2 seconds for up to 50k contacts
- Activity timeline shows all interactions within 1 minute of the triggering event
- Contact merge preserves all activity history and custom fields
