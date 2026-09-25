# Design: Web UI Phase 2 (Tenants, Forms, API Keys + Full Contacts)

## Problem Statement

Phase 1 established the login/dashboard/contacts foundation. Phase 2 expands the UI to cover all platform resources: context (tenant) management, API key management, and complete form management with submission viewing. Phase 2 also wires the contacts page to use real API endpoints (previously using mock data) and adds the forms, banners, and API keys pages to complete the admin interface.

## Goals

1. Add Tenants (Contexts) CRUD page: list, create, edit, delete contexts
2. Add API Keys management page: create with expiry, list, delete keys
3. Add Forms CRUD page: create with field builder, list, delete forms
4. Add Form submissions viewer: list submissions per form with data display
5. Wire Contacts page to real API endpoints (previously using mock data)
6. Add Banners management page: CRUD for banners, placements, campaigns
7. Add analytics dashboard with real tracking metrics (top pages, referrers, device breakdown)
8. Implement backend repository infrastructure for contacts and forms
9. Add database tables for contacts, tags, forms, form_submissions, and activities

## Non-Goals

- Webhook configuration UI
- Email template editor
- User management beyond context members
- Role-based access control (owner/admin/member only)
- Mobile-responsive design (desktop-first)

## Architecture

### Component Diagram

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    Phase 2 Architecture                                    │
│                                                                            │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │                     Vue 3 SPA (Phase 2 Expanded)                   │   │
│  │                                                                     │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────┐  │   │
│  │  │ Dashboard│ │ Contacts │ │ Forms    │ │ Tenants  │ │API Keys│  │   │
│  │  │ (full)   │ │ (real)   │ │ (CRUD)   │ │ (CRUD)   │ │ (CRUD) │  │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └────────┘  │   │
│  │                                                                     │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐                           │   │
│  │  │ Banners  │ │Analytics │ │  Forms   │                           │   │
│  │  │ (CRUD)   │ │ (metrics)│ │(submiss.)│                           │   │
│  │  └──────────┘ └──────────┘ └──────────┘                           │   │
│  │                                                                     │   │
│  │  ┌────────────────────────────────────────────────────────────┐     │   │
│  │  │ AppLayout: Sidebar (7 nav items) + Header + RouterView     │     │   │
│  │  └────────────────────────────────────────────────────────────┘     │   │
│  └────────────────────────────────────┬─────────────────────────────────┘   │
│                                       │                                      │
│  ┌────────────────────────────────────┼─────────────────────────────────────┐│
│  │                      Go Backend (Enhanced)                                ││
│  │                                                                             ││
│  │  API Routes:                                                                ││
│  │    /api/contexts/*         → Context CRUD                                  ││
│  │    /api/api-keys           → API key CRUD                                  ││
│  │    /api/contacts/*         → Contact CRUD (real backend)                   ││
│  │    /api/forms/*            → Form CRUD                                     ││
│  │    /api/forms/:slug/submit → Public form submission                        ││
│  │    /api/banners/*          → Banner CRUD                                   ││
│  │    /api/tracking/*         → Tracking dashboard                            ││
│  │    /api/admin/configs/*    → Config management                             ││
│  │                                                                             ││
│  │  ServeStatic("web/dist/") → SPA at root /                                  ││
│  └─────────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │  Database (New Tables in Phase 2)                                  │     │
│  │  contacts, tags, contact_tags, forms, form_submissions, activities │     │
│  └────────────────────────────────────────────────────────────────────┘     │
└────────────────────────────────────────────────────────────────────────────┘
```

### Vue SPA Structure (Phase 2)

```
webui/src/
├── views/
│   ├── DashboardView.vue       # Enhanced: real metrics + analytics
│   ├── ContactsView.vue        # Enhanced: real API + merge + activity
│   ├── FormsView.vue           # NEW: form CRUD + field builder + submissions
│   ├── TenantsView.vue         # NEW: context CRUD
│   ├── ApiKeysView.vue         # NEW: API key CRUD
│   ├── BannersView.vue         # NEW: banner/placement/campaign CRUD
│   └── AnalyticsView.vue       # NEW: tracking metrics dashboard
├── components/
│   ├── ui/                     # shadcn-vue components (expanded)
│   │   ├── dialog.ts           # Modal dialogs
│   │   ├── table.ts            # Data tables
│   │   ├── form.ts             # Form field components
│   │   ├── select.ts           # Dropdown selects
│   │   ├── tabs.ts             # Tab navigation
│   │   ├── tooltip.ts          # Tooltip components
│   │   ├── toast.ts            # Toast notifications
│   │   └── ...
│   ├── forms/                  # NEW: form builder components
│   │   ├── FieldBuilder.vue    # Drag-and-drop field config
│   │   ├── FormPreview.vue     # Live form preview
│   │   └── SubmissionView.vue  # Submission data display
│   ├── banners/                # NEW: banner components
│   │   ├── BannerCard.vue      # Banner listing card
│   │   ├── PlacementForm.vue   # Placement create/edit form
│   │   └── CampaignCard.vue    # Campaign listing
│   └── contacts/               # Expanded
│       ├── ContactTable.vue    # Enhanced with merge
│       ├── ContactForm.vue     # Enhanced with activity timeline
│       ├── MergeDialog.vue     # Contact merge confirmation
│       └── ActivityTimeline.vue # Activity history
└── composables/
    └── usePagination.ts        # Shared pagination logic
```

## Data Model

### Database Schema

```sql
-- Contacts table
CREATE TABLE contacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id      UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
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

CREATE INDEX idx_contacts_context_id ON contacts(context_id);
CREATE INDEX idx_contacts_email ON contacts(context_id, email) WHERE email IS NOT NULL;
CREATE INDEX idx_contacts_name ON contacts(context_id, first_name, last_name);
CREATE INDEX idx_contacts_company ON contacts(context_id, company);

-- Tags table
CREATE TABLE tags (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    color       TEXT NOT NULL DEFAULT '#6366f1',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_tags_context_name ON tags(context_id, name);

-- Contact-tags junction
CREATE TABLE contact_tags (
    contact_id  UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    tag_id      UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (contact_id, tag_id)
);

-- Forms table
CREATE TABLE forms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    slug        TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    fields      JSONB NOT NULL DEFAULT '[]',
    settings    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_forms_context_slug ON forms(context_id, slug);

-- Form submissions
CREATE TABLE form_submissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id     UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    context_id   UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    data        JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_submissions_form_id ON form_submissions(form_id);
CREATE INDEX idx_submissions_context_id ON form_submissions(context_id);

-- Activities (for contact timeline)
CREATE TABLE activities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id  UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    context_id   UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,       -- form_submission, page_visit, banner_click
    data        JSONB DEFAULT '{}',
    source_id   TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_activities_contact_id ON activities(contact_id);
CREATE INDEX idx_activities_context_id ON activities(context_id);
```

## API Contracts

| Method | Endpoint | Page | Description |
|--------|----------|------|-------------|
| `GET/POST/PUT/DELETE` | `/api/contexts` | TenantsView | Context CRUD |
| `POST` | `/api/api-keys` | ApiKeysView | Create API key |
| `GET` | `/api/contacts` | ContactsView | List contacts |
| `POST/PUT/DELETE` | `/api/contacts/*` | ContactsView | Contact CRUD |
| `POST` | `/api/contacts/:id/merge` | ContactsView | Merge contacts |
| `GET` | `/api/forms` | FormsView | List forms |
| `POST/PUT/DELETE` | `/api/forms/*` | FormsView | Form CRUD |
| `GET` | `/api/forms/:id/submissions` | FormsView | List submissions |
| `POST` | `/form/:slug` | Public | Submit form |
| `GET/POST/PUT/DELETE` | `/api/banners/*` | BannersView | Banner CRUD |
| `GET` | `/api/tracking/dashboard` | AnalyticsView | Dashboard metrics |

### Request/Response Shapes

```json
// POST /api/forms (create with fields)
{
  "slug": "newsletter",
  "name": "Newsletter Signup",
  "fields": [
    { "slug": "email", "type": "email", "label": "Email", "required": true, "order": 1 },
    { "slug": "name", "type": "text", "label": "Full Name", "required": false, "order": 2 }
  ],
  "settings": {
    "webhook_url": "https://hooks.example.com/newsletter",
    "email_notifications": { "enabled": true, "to": ["admin@example.com"] }
  }
}
→ 201 { "form": { "id": "uuid", "slug": "newsletter", "name": "Newsletter Signup", ... } }

// GET /api/forms/:id/submissions
→ 200 {
    "submissions": [
      { "id": "uuid", "data": {"email": "a@b.com", "name": "John"}, "created_at": "2026-01-15T10:30:00Z" }
    ],
    "total": 50, "page": 1, "limit": 20
  }
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Context CRUD | TenantsView | Maps to `/api/contexts` endpoints |
| Forms as JSONB | JSONB fields + settings | Schema-free form builder; no DB migrations for new fields |
| Submissions viewer | GET /api/forms/:id/submissions | Hierarchical resource; relates to form |
| Tag management | In ContactsView | Tags are contact-specific; no standalone page needed |
| Analytics | Separate AnalyticsView | Dashboard is summary; analytics is detailed |
| Build pipeline | Vite → web/dist/ → Go ServeStatic | Same pattern as Phase 1; consistent deployment |
| Pagination | Shared usePagination composable | DRY; consistent UX across all list pages |
| Form field builder | Drag-and-drop UI | Intuitive for non-technical users |
| Contact merge | Dialog confirmation | Prevents accidental merges; shows preview of combined data |
| API keys with expiry | Optional expires_at field | Security best practice; automatic key rotation |
