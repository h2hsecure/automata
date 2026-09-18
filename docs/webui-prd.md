# Product Requirements Document — Automata Web UI

**Version:** 1.0
**Date:** 2026-09-16
**Status:** Draft
**Source:** `docs/webui.md` + codebase analysis

---

## 1. Overview

Automata is a marketing automation platform with a Go backend (Gin + PostgreSQL) providing multi-tenant management of contacts, forms, banners, email queues, webhooks, and visitor tracking. This PRD defines the **Web UI** — a Vue 3 single-page application that gives administrators a visual interface to manage all platform resources.

The Web UI runs as a **separate server** (BFF pattern) proxied through the same domain, communicating with the existing Go API backend over HTTP.

---

## 2. Goals

1. Provide a complete admin interface for all backend entities (contacts, forms, banners, email templates, tenants, tracking analytics).
2. Enable the admin user to see and manage **all tenants** and their data (super-admin view).
3. Ship fast — minimal bundle size, lazy-loaded routes, server-side rendering not required for v1.
4. Use shadcn-vue components for a consistent, accessible UI.
5. Implement a BFF (Backend-for-Frontend) layer that handles auth token management, session persistence, and API aggregation.

---

## 3. Non-Goals (v1)

- Server-side rendering (SSR) / Nuxt — not required; SPA is sufficient.
- Real-time WebSocket updates — polling is acceptable for v1.
- Email template editor with WYSIWYG — pre-configured templates with basic editing.
- Multi-language / i18n — English only for v1.
- Mobile app — responsive web is the target.

---

## 4. Architecture

### 4.1 High-Level

```
+-----------------------------------------------------+
|                    Browser                           |
|  +-----------------------------------------------+  |
|  |  Vue 3 SPA (Vite + TypeScript + shadcn-vue)  |  |
|  +-----------------------------------------------+  |
+-----------------------------+-----------------------+
                              | HTTP / JSON
                              v
+-----------------------------------------------------+
|              BFF Server (Go)                         |
|  +-----------------------------------------------+  |
|  |  Auth proxy (JWT refresh, session)            |  |
|  |  API aggregation (optional)                   |  |
|  |  Static file serving (Vue dist)               |  |
|  +-----------------------------------------------+  |
+-----------------------------+-----------------------+
                              | HTTP / JSON
                              v
+-----------------------------------------------------+
|              Go API Backend (existing)               |
|  Gin server on :8080                                |
|  /auth/* /admin/* /api/tenant/:slug/* /api/track/*  |
+-----------------------------------------------------+
```

### 4.2 BFF Responsibilities

The BFF is a lightweight Go service (or Node/Go) that:

1. **Serves the Vue SPA** — static files from `dist/`.
2. **Handles auth flow** — proxies `/auth/login`, `/auth/logout`, `/auth/refresh` to the backend, manages JWT in httpOnly cookies.
3. **Strips tenant context** — the backend expects `tenant` in context; the BFF injects it from the authenticated user's session.
4. **CORS** — if the UI and API run on different origins, the BFF eliminates CORS complexity.

**Deployment:** The BFF + Vue dist run on a separate port (e.g., `:3000`), behind the same reverse proxy (Envoy/Nginx) as the API.

### 4.3 Tech Stack

| Layer | Technology |
|-------|-----------|
| Framework | Vue 3 + Composition API + `<script setup>` |
| Language | TypeScript |
| Build | Vite 5 |
| UI Components | shadcn-vue (Radix Vue primitives + Tailwind) |
| Styling | Tailwind CSS 4 |
| State Management | Pinia |
| Routing | Vue Router 4 |
| HTTP Client | Built-in `fetch` or `$fetch` (ofetch) |
| Forms | VeeValidate + Zod |
| Charts | Apache ECharts or Chart.js (for analytics) |
| Icons | Lucide Vue |
| Testing | Vitest + Playwright |

---

## 5. Authentication & Authorization

### 5.1 Login Flow

The backend auth endpoint is `POST /auth/login` with body `{ email, password, tenant }`.

**UI Flow:**
1. User lands on `/login` page.
2. Enters email, password, and tenant slug.
3. BFF proxies to backend, receives `{ token, user_id, tenant_id, email }`.
4. BFF stores JWT in **httpOnly cookie** (not localStorage) with 24h expiry.
5. Redirect to `/dashboard`.

### 5.2 Session Management

- JWT stored in httpOnly cookie, `SameSite=Lax`, `Secure=true` in production.
- BFF handles token refresh automatically via a `/auth/refresh` endpoint (to be added to backend if not present).
- Token expiry: 24 hours (matches backend JWT config).
- On expiry, BFF redirects to `/login`.

### 5.3 Admin vs Tenant User

| Role | Access |
|------|--------|
| **Admin** (owner) | Sees all tenants, all contacts, all forms, all banners across all tenants. `/admin/*` routes. |
| **Tenant User** | Sees only their own tenant's data. `/api/tenant/:slug/*` routes. |

The backend determines scope via the `tenant` claim in the JWT. The UI must respect this:
- Admin users get a **tenant switcher** in the sidebar to view data per tenant.
- Non-admin users see only their assigned tenant.

---

## 6. Information Architecture

### 6.1 Navigation Structure

```
Automata Web UI
+-- Login (/login)
+-- Dashboard (/dashboard)
|   +-- Overview metrics (visitors, contacts, forms, banners)
+-- Tenants (/tenants)
|   +-- List all tenants (admin only)
|   +-- Create / Edit tenant
|   +-- Tenant settings
+-- Contacts (/contacts)
|   +-- Contact list (search, filter, paginate)
|   +-- Contact detail (profile, activities, tags)
|   +-- Create / Edit contact
|   +-- Merge contacts
|   +-- Tag management
+-- Forms (/forms)
|   +-- Form list
|   +-- Form builder (field editor)
|   +-- Form submissions
|   +-- Submission detail
+-- Banners (/banners)
|   +-- Banner list (with CTR stats)
|   +-- Create / Edit banner
|   +-- Placements management
|   +-- Campaigns management
+-- Email (/email)
|   +-- Email templates (pre-configured)
|   +-- Send email (compose)
|   +-- Email queue status
+-- Tracking (/tracking)
|   +-- Dashboard metrics (visitors, page views, referrers)
|   +-- Events log
|   +-- Visitors list
|   +-- Top pages / referrers
+-- API Keys (/settings/api-keys)
|   +-- Create / Revoke API keys
+-- Settings (/settings)
    +-- Profile, password change
```

### 6.2 Layout

```
+----------------------------------------------------------+
|  Header: Logo | Tenant Switcher | User Menu              |
+----------------+-----------------------------------------+
|                |                                         |
| Sidebar        |  Main Content Area                      |
|                |  +-----------------------------------+  |
| - Dashboard    |  |  Page Content (table, form,       |  |
| - Contacts     |  |   charts, cards)                  |  |
| - Forms        |  |                                   |  |
| - Banners      |  |                                   |  |
| - Email        |  |                                   |  |
| - Tracking     |  |                                   |  |
| - API Keys     |  |                                   |  |
| - Settings     |  |                                   |  |
|                |  |                                   |  |
+----------------+-----------------------------------------+
```

---

## 7. Page Specifications

### 7.1 Login Page (`/login`)

**Requirements:**
- Email field (validated, max 254 chars)
- Password field
- Tenant slug field
- "Sign in" button
- Error display for invalid credentials / tenant not found

**API:** `POST /auth/login` -> `{ token, user_id, tenant_id, email }`

### 7.2 Dashboard (`/dashboard`)

**Metrics displayed:**
- Total visitors (from tracking)
- Active visitors (last 30 min)
- Total page views
- Total contacts count
- Total forms count
- Total banners count
- Recent activity feed

**Charts:**
- Page views over time (line chart, last 7/30 days)
- Top pages (bar chart)
- Top referrers (bar chart)
- Device breakdown (pie chart)
- Browser breakdown (pie chart)

**API:** `GET /api/tracking/dashboard?start=&end=` -> `{ total_visitors, active_visitors, page_views, top_pages, top_referrers, device_breakdown, browser_breakdown }`

### 7.3 Contacts

#### Contact List
- Data table with columns: Name, Email, Company, Tags, Source, Created At
- Search bar (full-text on name/email)
- Filter by: tags (multi-select), source, company
- Pagination: 20/50/100 per page
- Sortable columns
- "Create Contact" button

**API:** `GET /api/tenant/:slug/contacts?search=&tags=&source=&company=&page=&limit=` -> `{ contacts, total, page, limit }`

#### Contact Detail
- Profile card: name, email, phone, company, tags, custom fields
- Activity timeline (form submissions, page visits, banner clicks)
- Edit button -> inline edit or modal
- Merge button -> merge with another contact

**API:**
- `GET /api/tenant/:slug/contacts/:id` -> contact object
- `GET /api/tenant/:slug/contacts/:id/activity?offset=&limit=` -> `{ activities, offset, limit }`

#### Create / Edit Contact
- Form with fields: first name, last name, email (optional), phone, company, custom fields (dynamic key-value), tags (multi-select)
- Validation: email format if provided
- Source field (text)

**API:** `POST /api/tenant/:slug/contacts` / `PUT /api/tenant/:slug/contacts/:id`

#### Merge Contacts
- Modal: select two contacts to merge
- Confirm merge action

**API:** `POST /api/tenant/:slug/contacts/:id/merge` -> `{ merge_with: "<contact_id>" }`

### 7.4 Forms

#### Form List
- Table: Name, Slug, Fields count, Submissions count, Created At
- "Create Form" button

**API:** `GET /api/tenant/:slug/forms` -> `{ forms }`

#### Form Builder
- Visual form builder with draggable fields:
  - Text input
  - Email input
  - Textarea
  - Select / dropdown
  - Checkbox
  - Radio buttons
  - File upload
- Field configuration panel: label, slug, required, validation rules, order
- Preview mode
- Save / Publish toggle

**API:**
- `POST /api/tenant/:slug/forms` -> form object
- `PUT /api/tenant/:slug/forms/:id` -> form object
- `DELETE /api/tenant/:slug/forms/:id`

**Form field structure:**
```json
{
  "slug": "email",
  "type": "email",
  "label": "Email Address",
  "required": true,
  "validation": { "max": 254 },
  "order": 1
}
```

#### Form Submissions
- Select a form -> see its submissions in a table
- Columns: submission data (expandable row), submitted at
- Pagination

**API:** `GET /api/tenant/:slug/forms/:form_id/submissions?page=&limit=` -> `{ submissions, total, page, limit }`

#### Public Form Embed
- Generated embed code (JavaScript snippet) for each form
- Copy-to-clipboard button
- Preview iframe

**API:** `POST /api/tenant/:slug/forms/submit` -> `{ id: "submission_id" }`

### 7.5 Banners

#### Banner List
- Table: Name, Type (html/image/video/popup/sticky), Campaign, Placements, Status (active/inactive), Impressions, Clicks, CTR
- Filter by: campaign, active status
- "Create Banner" button

**API:** `GET /api/tenant/:slug/banners?campaign_id=&is_active=&offset=&limit=` -> `{ banners, total }`

#### Create / Edit Banner
- Banner name
- Type selector (html, image, video, popup, sticky)
- Content editor:
  - HTML type -> rich text / code editor
  - Image type -> image URL + alt text
  - Video type -> video URL
- Link URL (destination)
- Campaign selector (dropdown)
- Placement multi-select
- Priority (number)
- Start date / End date (date pickers)
- A/B test toggle -> dynamic variant editor
- Active/inactive toggle

**API:**
- `POST /api/tenant/:slug/banners` -> banner object
- `PUT /api/tenant/:slug/banners/:id` -> banner object
- `DELETE /api/tenant/:slug/banners/:id`

#### Placements
- Table: Name, Location, CSS Selector, Max Banners, Status
- Create / Edit / Delete placement

**API:**
- `POST /api/tenant/:slug/placements` -> placement object
- `PUT /api/tenant/:slug/placements/:id` -> placement object
- `DELETE /api/tenant/:slug/placements/:id`

#### Campaigns
- Table: Name, Description, Start Date, End Date, Status, Impressions, Clicks, Conversion Rate
- Create / Edit / Delete campaign

**API:**
- `POST /api/tenant/:slug/campaigns` -> campaign object
- `PUT /api/tenant/:slug/campaigns/:id` -> campaign object
- `DELETE /api/tenant/:slug/campaigns/:id`

### 7.6 Email

#### Email Templates
- Pre-configured templates stored in the backend (to be defined).
- Template list: name, subject, body preview, created at.
- Create / Edit template with:
  - Subject line
  - Plain text body
  - HTML body
  - Variable placeholders (e.g., `{{ .ContactFirstName }}`)

#### Send Email
- Compose form:
  - To (multi-recipient, comma-separated)
  - Subject
  - Template selector (dropdown of pre-configured templates)
  - Body override (optional, merges with template)
  - HTML body toggle
- Preview before send
- Enqueue -> backend email worker picks up and sends

**API:** `POST /api/tenant/:slug/email/send` (to be implemented) -> `{ id: "job_id" }`

#### Email Queue Status
- Pending jobs count
- Failed jobs (after max retries)
- Recent send log

### 7.7 Tracking & Analytics

#### Dashboard Metrics
Same as the main dashboard but with more detail:
- Real-time visitor count
- Page views (last 24h / 7d / 30d)
- Top pages with view counts
- Top referrers
- Device breakdown (desktop/mobile/tablet)
- Browser breakdown (Chrome, Firefox, Safari, etc.)

**API:** `GET /api/tracking/dashboard?start=&end=`

#### Events Log
- Paginated table of tracking events
- Columns: Type (pageview/event), URL, Event Name, Referrer, Visitor, Created At
- Filter by: type, event name, URL
- Pagination

**API:** `GET /api/tracking/events?type=&event_name=&url=&offset=&limit=` -> `{ events, total }`

#### Visitors
- List of tracked visitors
- Columns: Visitor ID, Cookie Value, Fingerprint, First Seen, Last Seen, Page Views
- Click into visitor detail -> their event history

**API:** `GET /api/tracking/visitors` (to be implemented)

### 7.8 Tenants (Admin Only)

#### Tenant List
- Table: Slug, Name, Domain, Status (active/inactive), Created At
- "Create Tenant" button
- Admin-only page

**API:**
- `GET /admin/tenants?offset=&limit=` -> `{ tenants, count }`
- `POST /admin/tenants` -> tenant object
- `GET /admin/tenants/:id` -> tenant object
- `PUT /admin/tenants/:id` -> tenant object
- `DELETE /admin/tenants/:id`

#### Create / Edit Tenant
- Slug (unique, max 63 chars, lowercase alphanumeric + hyphens)
- Name (max 255 chars)
- Domain (optional)
- Settings (key-value JSON editor)
- Active/inactive toggle

### 7.9 API Keys

**API:**
- `POST /admin/api-keys` -> `{ id, name, key, expires_at }`
- List existing keys (without exposing the full key value)
- Revoke key

### 7.10 Settings

- Profile: display name, email
- Change password
- API key management (per user)

---

## 8. API Integration

### 8.1 Backend API Endpoints (Existing)

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/auth/login` | None | Login, returns JWT |
| POST | `/auth/logout` | None | Logout |
| GET | `/health` | None | Health check |
| GET | `/admin/tenants` | Admin JWT | List all tenants |
| POST | `/admin/tenants` | Admin JWT | Create tenant |
| GET | `/admin/tenants/:id` | Admin JWT | Get tenant |
| PUT | `/admin/tenants/:id` | Admin JWT | Update tenant |
| DELETE | `/admin/tenants/:id` | Admin JWT | Delete tenant |
| POST | `/admin/api-keys` | Admin JWT | Create API key |
| GET | `/api/tenant/:slug/contacts` | JWT/API key | List contacts |
| POST | `/api/tenant/:slug/contacts` | JWT/API key | Create contact |
| GET | `/api/tenant/:slug/contacts/:id` | JWT/API key | Get contact |
| PUT | `/api/tenant/:slug/contacts/:id` | JWT/API key | Update contact |
| DELETE | `/api/tenant/:slug/contacts/:id` | JWT/API key | Delete contact |
| POST | `/api/tenant/:slug/contacts/:id/merge` | JWT/API key | Merge contacts |
| GET | `/api/tenant/:slug/contacts/:id/activity` | JWT/API key | Get contact activity |
| GET | `/api/tenant/:slug/forms` | JWT/API key | List forms |
| POST | `/api/tenant/:slug/forms` | JWT/API key | Create form |
| GET | `/api/tenant/:slug/forms/:id` | JWT/API key | Get form |
| PUT | `/api/tenant/:slug/forms/:id` | JWT/API key | Update form |
| DELETE | `/api/tenant/:slug/forms/:id` | JWT/API key | Delete form |
| POST | `/api/tenant/:slug/forms/submit` | JWT/API key | Submit form |
| GET | `/api/tenant/:slug/forms/:form_id/submissions` | JWT/API key | List submissions |
| GET | `/api/tracking/dashboard` | JWT/API key | Dashboard metrics |
| GET | `/api/tracking/events` | JWT/API key | Events log |
| GET | `/api/tenant/:slug/banners` | JWT/API key | List banners |
| POST | `/api/tenant/:slug/banners` | JWT/API key | Create banner |
| PUT | `/api/tenant/:slug/banners/:id` | JWT/API key | Update banner |
| DELETE | `/api/tenant/:slug/banners/:id` | JWT/API key | Delete banner |
| GET | `/api/tenant/:slug/placements` | JWT/API key | List placements |
| POST | `/api/tenant/:slug/placements` | JWT/API key | Create placement |
| PUT | `/api/tenant/:slug/placements/:id` | JWT/API key | Update placement |
| DELETE | `/api/tenant/:slug/placements/:id` | JWT/API key | Delete placement |
| GET | `/api/tenant/:slug/campaigns` | JWT/API key | List campaigns |
| POST | `/api/tenant/:slug/campaigns` | JWT/API key | Create campaign |
| PUT | `/api/tenant/:slug/campaigns/:id` | JWT/API key | Update campaign |
| DELETE | `/api/tenant/:slug/campaigns/:id` | JWT/API key | Delete campaign |
| GET | `/form/*path` | JWT/API key | Form rendering |
| GET | `/snippet/*path` | None | Tracking snippet |
| GET | `/static/*path` | None | Static assets |

### 8.2 Missing Endpoints (to be implemented in backend)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/auth/refresh` | Refresh JWT token |
| POST | `/api/tenant/:slug/email/send` | Send email via queue |
| GET | `/api/tracking/visitors` | List visitors |

### 8.3 Request/Response Conventions

- All authenticated requests: `Authorization: Bearer <token>` header.
- Error responses: `{ "error": "message" }` with HTTP status code.
- Success responses: JSON body with resource or `{ "message": "..." }`.
- Pagination: `{ data, total, page, limit }` pattern.
- Timestamps: ISO 8601 strings.

---

## 9. Performance Requirements

| Metric | Target |
|--------|--------|
| Initial page load | < 2s on 4G |
| LCP (Largest Contentful Paint) | < 2.5s |
| Bundle size (gzipped) | < 200KB initial, < 500KB total |
| Route lazy loading | All routes lazy-loaded except login/dashboard |
| API response time | < 200ms for list endpoints, < 500ms for complex queries |

**Strategies:**
- Vite code splitting per route.
- Lazy-load heavy components (charts, form builder).
- Debounced search inputs (contacts, forms).
- Virtualized tables for large datasets (> 1000 rows).
- Cached API responses with stale-while-revalidate.

---

## 10. Design System

### 10.1 Component Library

**shadcn-vue** components to use:

| Category | Components |
|----------|-----------|
| Navigation | Sidebar, NavigationMenu, Breadcrumb |
| Data Display | Table, Card, Badge, Avatar, Tabs, Accordion, Chart (via ECharts) |
| Forms | Input, Textarea, Select, Checkbox, RadioGroup, Switch, DatePicker, Combobox |
| Feedback | Dialog, Toast, Alert, Skeleton, Progress, Tooltip, DropdownMenu |
| Overlays | Sheet, Drawer, Popover, Command (command palette) |

### 10.2 Theme

- Default: light mode with dark mode toggle.
- Tailwind CSS with CSS variables for theming.
- Primary color: indigo (`#6366f1` — matches existing tag color in codebase).
- Font: Inter (Google Fonts) or system font stack.

### 10.3 Icons

Lucide icons (consistent with shadcn-vue ecosystem).

---

## 11. Project Structure

```
pdavui/                          # Vue 3 SPA
+-- src/
|   +-- api/                     # API client layer
|   |   +-- client.ts            # $fetch instance with auth interceptor
|   |   +-- endpoints.ts         # Typed API endpoint definitions
|   |   +-- types.ts             # TypeScript types from backend
|   +-- components/
|   |   +-- ui/                  # shadcn-vue components (auto-generated)
|   |   +-- contacts/
|   |   |   +-- ContactTable.vue
|   |   |   +-- ContactForm.vue
|   |   |   +-- ContactDetail.vue
|   |   |   +-- MergeDialog.vue
|   |   +-- forms/
|   |   |   +-- FormBuilder.vue
|   |   |   +-- FormFieldEditor.vue
|   |   |   +-- SubmissionTable.vue
|   |   |   +-- EmbedCode.vue
|   |   +-- banners/
|   |   |   +-- BannerForm.vue
|   |   |   +-- PlacementForm.vue
|   |   |   +-- CampaignForm.vue
|   |   +-- email/
|   |   |   +-- TemplateList.vue
|   |   |   +-- EmailComposer.vue
|   |   |   +-- QueueStatus.vue
|   |   +-- tracking/
|   |   |   +-- DashboardCharts.vue
|   |   |   +-- EventsTable.vue
|   |   |   +-- VisitorsTable.vue
|   |   +-- layout/
|   |   |   +-- AppLayout.vue
|   |   |   +-- Sidebar.vue
|   |   |   +-- Header.vue
|   |   |   +-- TenantSwitcher.vue
|   |   +-- shared/
|   |       +-- SearchInput.vue
|   |       +-- Pagination.vue
|   |       +-- ConfirmDialog.vue
|   +-- composables/
|   |   +-- useAuth.ts           # Auth state, login, logout
|   |   +-- useApi.ts            # API call wrapper with error handling
|   |   +-- usePagination.ts     # Pagination state
|   |   +-- useDebounce.ts       # Debounced search
|   +-- stores/
|   |   +-- auth.ts              # Pinia auth store
|   |   +-- tenant.ts            # Pinia tenant store (current tenant, switcher)
|   |   +-- ui.ts                # Pinia UI store (sidebar, theme, modals)
|   +-- views/
|   |   +-- LoginView.vue
|   |   +-- DashboardView.vue
|   |   +-- ContactsView.vue
|   |   +-- ContactDetailView.vue
|   |   +-- FormsView.vue
|   |   +-- FormBuilderView.vue
|   |   +-- FormSubmissionsView.vue
|   |   +-- BannersView.vue
|   |   +-- PlacementsView.vue
|   |   +-- CampaignsView.vue
|   |   +-- EmailView.vue
|   |   +-- TrackingView.vue
|   |   +-- TenantsView.vue
|   |   +-- ApiKeysView.vue
|   |   +-- SettingsView.vue
|   +-- router/
|   |   +-- index.ts             # Vue Router with guards
|   +-- App.vue
|   +-- main.ts
+-- public/
+-- index.html
+-- vite.config.ts
+-- tailwind.config.ts
+-- tsconfig.json
+-- package.json
+-- components.json               # shadcn-vue config
```

---

## 12. Routing & Navigation Guards

```typescript
// Pseudo-code for router guards
const routes = [
  { path: '/login', component: LoginView, meta: { guest: true } },
  {
    path: '/',
    component: AppLayout,
    meta: { requiresAuth: true },
    children: [
      { path: 'dashboard', component: DashboardView },
      { path: 'contacts', component: ContactsView },
      { path: 'contacts/:id', component: ContactDetailView },
      { path: 'forms', component: FormsView },
      { path: 'forms/new', component: FormBuilderView },
      { path: 'forms/:id/edit', component: FormBuilderView },
      { path: 'forms/:formId/submissions', component: FormSubmissionsView },
      { path: 'banners', component: BannersView },
      { path: 'placements', component: PlacementsView },
      { path: 'campaigns', component: CampaignsView },
      { path: 'email', component: EmailView },
      { path: 'tracking', component: TrackingView },
      { path: 'tenants', component: TenantsView, meta: { requiresAdmin: true } },
      { path: 'settings/api-keys', component: ApiKeysView },
      { path: 'settings', component: SettingsView },
    ],
  },
]
```

---

## 13. State Management (Pinia Stores)

### Auth Store
```typescript
interface AuthState {
  token: string | null
  user: { id: string; email: string; tenant_id: string; is_owner: boolean } | null
  login(email: string, password: string, tenant: string): Promise<void>
  logout(): Promise<void>
  refresh(): Promise<void>
}
```

### Tenant Store
```typescript
interface TenantState {
  currentTenant: { id: string; slug: string; name: string } | null
  allTenants: { id: string; slug: string; name: string }[]
  switchTenant(slug: string): Promise<void>
  loadTenants(): Promise<void>
}
```

### UI Store
```typescript
interface UIState {
  sidebarCollapsed: boolean
  theme: 'light' | 'dark'
  toggleSidebar(): void
  setTheme(theme: 'light' | 'dark'): void
}
```

---

## 14. Email Template System

### Pre-configured Templates

The backend should ship with these default email templates (stored as data in the DB or as Go embed):

| Template | Subject | Body Variables |
|----------|---------|---------------|
| Welcome | `Welcome to {{ .TenantName }}` | TenantName, ContactFirstName, LoginURL |
| Contact Created | `New contact: {{ .ContactName }}` | ContactName, ContactEmail, Source |
| Form Submission | `New form submission: {{ .FormName }}` | FormName, SubmissionData |
| Password Reset | `Reset your password` | ResetURL, Expiry |
| Notification | `{{ .Subject }}` | Subject, Body, CTA |

### Template Editor

- List view of all templates with subject preview.
- Edit view with:
  - Template name (identifier)
  - Subject line with variable picker
  - Plain text body editor (with variable autocomplete)
  - HTML body editor (with variable autocomplete)
  - Test send form (enter email to preview)

---

## 15. Deployment

### 15.1 Docker Compose (Development)

```yaml
services:
  # Existing backend (unchanged)
  api:
    build: ../
    ports:
      - "8080:8080"

  # New BFF + UI
  webui:
    build: ./pdavui
    ports:
      - "3000:3000"
    environment:
      - API_URL=http://api:8080
    depends_on:
      - api
```

### 15.2 Production

- BFF serves Vue dist as static files.
- Both API (:8080) and BFF (:3000) behind Envoy ingress.
- Envoy routes:
  - `/api/*`, `/auth/*`, `/admin/*`, `/form/*`, `/snippet/*`, `/track/*` -> API
  - `/*` -> BFF (Vue SPA)
- TLS termination at Envoy.

---

## 16. Testing Strategy

| Type | Tool | Coverage |
|------|------|----------|
| Unit | Vitest | Components, composables, stores |
| Integration | Vitest | API client layer |
| E2E | Playwright | Login flow, CRUD operations, form builder |
| Visual | Chromatic (optional) | Component regression |

---

## 17. Open Questions

1. **Email template storage**: Are email templates stored in the DB or as Go-embedded files? The backend currently has an email queue system but no template endpoint. Should we add a `POST /admin/email-templates` endpoint?

2. **Admin vs tenant user distinction**: The `User` struct has an `IsOwner` field. Does the backend currently distinguish admin-level access (all tenants) from tenant-level access? The `/admin/*` routes exist but the JWT doesn't seem to carry an `is_owner` claim. Should the auth response include this?

3. **Real-time updates**: Should the dashboard show live visitor counts? This would require WebSocket support in the backend, or at minimum, a polling interval config.

4. **Form builder complexity**: Should the form builder support conditional logic (show/hide fields based on answers), file uploads, and reCAPTCHA? The backend `FormField` struct has a `validation` map but no conditional logic field.

5. **BFF technology**: Should the BFF be Go (reusing the existing codebase) or Node.js (simpler for a frontend-focused service)? Go would share the same deployment patterns; Node would have a larger ecosystem for auth/session libraries.

6. **File uploads**: The `FileUpload` struct exists in the form submission domain. Should the UI support file upload to a storage backend (S3, local disk)? The backend doesn't currently have a file upload endpoint.

7. **Webhook management**: The backend has a webhook queue system. Should the UI allow users to configure webhook endpoints per tenant?

8. **Activity timeline**: The `Activity` domain exists with types like `form_submission`, `page_visit`, `banner_click`. Should the contact detail page show this timeline? The backend has a `GetActivity` handler but it's not wired into the server routing.

---

## 18. Implementation Phases

### Phase 1: Foundation (Weeks 1-2)
- [] Scaffold Vue 3 + Vite + TypeScript + shadcn-vue project
- [ ] Set up Tailwind CSS, Pinia, Vue Router
- [ ] Implement BFF server (Go) with auth proxy
- [ ] Login page + auth flow (httpOnly cookie)
- [ ] App layout (sidebar, header, tenant switcher)
- [ ] Dashboard with basic metrics

### Phase 2: Core CRUD (Weeks 3-5)
- [ ] Contacts: list, create, edit, delete, search, filter, pagination
- [ ] Contacts: detail view with activity timeline
- [ ] Forms: list, create, edit, delete
- [ ] Forms: submissions list
- [ ] Tenants: admin CRUD (list, create, edit, delete)

### Phase 3: Advanced Features (Weeks 6-8)
- [ ] Form builder (visual drag-and-drop)
- [ ] Banners: CRUD + placements + campaigns
- [ ] Tracking: dashboard charts, events log, visitors list
- [ ] Email: template list, compose/send, queue status

### Phase 4: Polish (Weeks 9-10)
- [ ] Dark mode
- [ ] API keys management
- [ ] Settings (profile, password change)
- [ ] E2E tests (Playwright)
- [ ] Performance optimization (bundle size, lazy loading)
