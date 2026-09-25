# Design: Web UI Phase 1 (Login + Dashboard + Contacts)

## Problem Statement

The initial web UI must establish the foundation for the admin interface: user authentication, a navigable app layout, and the core contact management workflow. Phase 1 delivers a working Vue 3 SPA that users can log into, see dashboard metrics, and manage contacts. Without Phase 1, the platform has no graphical interface for its most-used features (contacts).

## Goals

1. Scaffold a Vue 3 project with TypeScript, Vite, Tailwind CSS, and shadcn-vue
2. Implement login with email/password/context selection and redirect to dashboard
3. Build sidebar navigation layout (Dashboard, Contacts nav items) with header
4. Create dashboard page with metric cards (contacts count, forms count, banners count, visitors)
5. Build contacts list with pagination, search, and create/edit/delete functionality
6. Fix backend auth middleware to set `tenant_id` (context_id) from JWT claims
7. Integrate Vue build into Go binary build pipeline (builds to `web/dist/`)

## Non-Goals

- Forms management UI
- Banners management UI
- Context/tenant management UI
- API key management UI
- Dark mode toggle
- Mobile-responsive sidebar collapse
- Analytics charts or graphs

## Architecture

### Directory Structure

```
webui/                              # Vue project source
├── package.json                    # Vue 3, Vite, TypeScript, shadcn-vue
├── vite.config.ts                  # Build to ../web/dist/
├── tsconfig.json                   # TypeScript config
├── tailwind.config.ts              # Tailwind with shadcn-vue theme
├── postcss.config.js               # Tailwind + Autoprefixer
├── index.html                      # SPA entry point
├── components.json                 # shadcn-vue config
├── public/
│   └── favicon.svg                 # Purple "A" icon
└── src/
    ├── main.ts                     # createApp → Pinia → Router → mount
    ├── App.vue                     # RouterView
    ├── style.css                   # Tailwind directives + CSS variables
    ├── lib/
    │   └── utils.ts                # cn() utility (clsx + tailwind-merge)
    ├── router/
    │   └── index.ts                # /login, /dashboard, /contacts + guards
    ├── stores/
    │   └── auth.ts                 # Pinia store: login, logout, token, user
    ├── api/
    │   └── client.ts               # Fetch wrapper with Bearer injection
    ├── types/
    │   └── api.ts                  # LoginResponse, Contact, PaginatedResponse
    ├── views/
    │   ├── LoginView.vue           # Email, password, tenant fields
    │   ├── DashboardView.vue       # Metric cards grid
    │   └── ContactsView.vue        # Contact list + table + form
    └── components/
        ├── layout/
        │   ├── Sidebar.vue         # Collapsible sidebar
        │   └── Header.vue          # User email + logout
        ├── ui/
        │   ├── input.ts            # shadcn Input
        │   ├── button.ts           # shadcn Button
        │   ├── card.ts             # shadcn Card
        │   └── badge.ts            # shadcn Badge
        └── contacts/
            ├── ContactTable.vue    # Paginated table
            └── ContactForm.vue     # Create/edit modal
```

### Backend Auth Middleware Fix

The auth middleware (`internal/adapter/api/middleware/auth.go`) must set `tenant_id` from the JWT token. Previously, only API key auth set the context; JWT auth did not. This is critical because the contact handler reads `c.GetString("tenant_id")` to filter queries.

```go
// Before (broken):
userID, _, err := authService.VerifyToken(token)
if err == nil {
    c.Set("user_id", userID)
    // tenant_id NOT set → all API calls get "tenant not found"
}

// After (fixed):
userID, tenantSlug, err := authService.VerifyToken(token)
if err == nil {
    c.Set("user_id", userID)
    c.Set("tenant_id", tenantSlug)  // ← This line was missing
}
```

### Auth Flow

```
User → /login
    → Enter: email, password, context slug
    → POST /api/auth/login { email, password }
    → Backend: VerifyToken → JWT signed with HS256
       Claims: user_id, tenant_slug (context_slug), exp
    → Returns: { token, user_id, email, contexts: [{id, slug, name}] }
    → Store token in localStorage
    → Redirect to /dashboard
    → All API requests: Authorization: Bearer <token>
```

### API Client

```typescript
// api/client.ts
const API_BASE = '/api'

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const token = authStore.token
  const headers: HeadersInit = { 'Content-Type': 'application/json' }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(`${API_BASE}${path}`, { method, headers, body: body ? JSON.stringify(body) : undefined })
  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(error.error || res.statusText)
  }
  return res.json() as Promise<T>
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  delete: <T>(path: string) => request<T>('DELETE', path),
}
```

### Build Pipeline

```
webui/ (Vite dev)  →  http://localhost:5173     # Development
webui/ (Vite build) →  web/dist/                 # Production

Go backend:
  ServeStatic(engine, "web/dist/")
    → serves web/dist/index.html at /
    → serves web/dist/assets/* at /assets/
    → SPA fallback at NoRoute → index.html
```

## Data Model

No new tables. Phase 1 consumes:

| Table | Used By |
|-------|---------|
| contexts | LoginView (context selection), DashboardView |
| context_users | LoginView (auth), DashboardView |
| contacts | ContactsView |
| contact_tags | ContactsView |

## API Contracts

| Method | Endpoint | Page | Purpose |
|--------|----------|------|---------|
| `POST` | `/api/auth/login` | LoginView | Authenticate, get token + contexts |
| `GET` | `/api/contacts` | ContactsView | List with pagination + search |
| `POST` | `/api/contacts` | ContactsView | Create contact |
| `PUT` | `/api/contacts/:id` | ContactsView | Update contact |
| `DELETE` | `/api/contacts/:id` | ContactsView | Delete contact |
| `GET` | `/api/contacts/tags` | ContactsView | List available tags |

### Request/Response Shapes

```json
// POST /api/auth/login
{ "email": "admin@example.com", "password": "secret" }
→ 200 {
    "token": "jwt.here",
    "user_id": "uuid",
    "email": "admin@example.com",
    "contexts": [{"id": "uuid", "slug": "acme", "name": "Acme Corp"}]
  }

// GET /api/contacts?search=john&page=1&limit=20
→ 200 {
    "contacts": [
      {"id": "uuid", "email": "john@example.com", "first_name": "John",
       "last_name": "Doe", "company": "Acme", "source": "form",
       "tags": [{"id": "uuid", "name": "lead"}],
       "created_at": "2026-01-15T10:30:00Z"}
    ],
    "total": 150, "page": 1, "limit": 20
  }
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| JWT storage | localStorage | Same-origin SPA; simpler than httpOnly cookie flow |
| Auth middleware fix | Set tenant_id from JWT claims | Required for all authenticated API calls to work |
| Build output | `../web/dist/` | Go serves from single directory; Docker COPY trivial |
| API client | Fetch wrapper with Bearer injection | Simple; no external HTTP library dependency |
| State management | Pinia store | Official Vue 3 recommendation; simple API |
| Component library | shadcn-vue | Headless, accessible, copy-editable components |
| CSS framework | Tailwind CSS 3 | Utility-first; rapid UI development |
| Layout | Sidebar + Header + RouterView | Standard admin dashboard pattern |
| Contacts page | Table + form modal | Single-page interaction; no sub-pages needed |
| No BFF | Direct Go static serving | Eliminates auth cookie complexity; same-origin = no CORS |
