# Design: Web UI - Full Implementation

## Problem Statement

The Automata platform requires a comprehensive web-based admin interface for managing contacts, forms, banners, contexts (tenants), API keys, and viewing analytics. Without a UI, administrators must use the REST API or MCP tools, which are not accessible to non-technical users. The interface must provide CRUD operations for all managed resources, form builder capabilities, contact management with search and filtering, and a dashboard with analytics metrics. The frontend is a Vue 3 single-page application (SPA) built to static files and served by the Go backend, eliminating CORS concerns and simplifying deployment.

## Goals

1. Build a Vue 3 SPA with shadcn-vue components and Tailwind CSS for the admin interface
2. Implement login with email/password and context selection
3. Provide sidebar navigation with Dashboard, Contacts, Forms, Banners, Contexts, API Keys
4. Build contact management: list with search/filter, create/edit form, delete, merge
5. Build form management: create with field builder, list, delete, view submissions
6. Build context management: CRUD for contexts (tenants) and context users
7. Build API key management: create, list, delete API keys
8. Build dashboard with metrics cards (contacts, forms, banners, visitors)
9. Build analytics dashboard (tracking metrics, top pages, referrers, device breakdown)
10. Serve the Vue SPA from Go backend at root `/` with SPA fallback routing
11. Integrate build pipeline: Vite builds to `web/dist/`, Go embeds and serves static files

## Non-Goals

- Server-side rendering (SSR) or Nuxt.js
- WebSocket-based real-time updates
- File upload handling in UI (out of scope)
- Webhook management UI (out of scope)
- Email template editor (out of scope)
- Dark mode toggle (CSS variables support exists but not implemented)

## Architecture

### Component Diagram

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                      Web UI Architecture                                      │
│                                                                                │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │                          Browser (Vue 3 SPA)                              │ │
│  │                                                                           │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────────────┐ │ │
│  │  │ LoginView │  │Dashboard │  │Contacts  │  │Forms / Banners /        │ │ │
│  │  │          │  │View      │  │View      │  │Contexts / API Keys     │ │ │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────────────────────┘ │ │
│  │         │          │           │                                            │ │
│  │         └──────────┴───────────┴───────────────────────────────────────────┘ │ │
│  │                          │                                                    │ │
│  │  ┌────────────────────────────────────────────────────────────────────────┐  │ │
│  │  │ Router (vue-router) + Auth Store (Pinia) + API Client                  │  │ │
│  │  │ - Route guards for /login, /dashboard                                 │  │ │
│  │  │ - JWT in localStorage, Bearer header on all requests                   │  │ │
│  │  └────────────────────────────────────────────────────────────────────────┘  │ │
│  └──────────────────────────────┬───────────────────────────────────────────────┘ │
│                                 │ (same origin, no CORS)                          │
│  ┌──────────────────────────────┼───────────────────────────────────────────────┐ │
│  │                          Go Backend                                           │ │
│  │                                                                                │ │
│  │  ServeStatic("/home/.../web/dist/")                                           │ │
│  │    ├── /assets/*       → serve JS/CSS bundles                                  │ │
│  │    ├── /favicon.svg    → favicon                                              │ │
│  │    ├── / → index.html (SPA fallback)                                          │ │
│  │    └── /api/*          → API handlers (auth, contacts, forms, ...)            │ │
│  │                                                                                │ │
│  │  ┌────────────────────────────────────────────────────────────────────────┐   │ │
│  │  │ API Server (Gin)                                                      │   │ │
│  │  │ /api/auth/login, /api/auth/logout, /api/auth/me                      │   │ │
│  │  │ /api/contexts/*, /api/users/*, /api/api-keys                           │   │ │
│  │  │ /api/contacts/*, /api/forms/*, /api/banners/*                           │   │ │
│  │  │ /api/tracking/*, /api/admin/configs/*                                  │   │ │
│  │  └────────────────────────────────────────────────────────────────────────┘   │ │
│  └────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │  Frontend Build Pipeline                                                 │  │
│  │  webui/ (Vue + Vite + TypeScript) → web/dist/                          │  │
│  │  package.json: "build": "vue-tsc --noEmit && vite build"               │  │
│  │  vite.config.ts: outDir: '../web/dist/'                                │  │
│  └──────────────────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────────────┘
```

### Vue SPA Structure

```
web/                           # Compiled output (gitignored source)
├── index.html                 # Entry point
├── favicon.svg                # Logo
└── assets/                    # Bundled JS/CSS
    ├── index-Vckk0faO.js      # App bundle
    ├── index-CWQEow6C.css     # Styles
    ├── DashboardView-*.js     # Dashboard route chunk
    ├── ContactsView-*.js      # Contacts route chunk
    ├── FormsView-*.js         # Forms route chunk
    ├── TenantsView-*.js       # Contexts route chunk
    ├── ApiKeysView-*.js       # API Keys route chunk
    ├── LoginView-*.js         # Login route chunk
    ├── AppLayout-*.js         # Layout component
    └── ...                    # UI component chunks

Source (webui/, git-tracked):
webui/
├── package.json               # Vue 3, Vite, TypeScript, shadcn-vue
├── vite.config.ts             # Build to ../web/dist/
├── tsconfig.json
├── tailwind.config.ts
├── postcss.config.js
├── index.html
├── components.json            # shadcn-vue config
├── public/
│   └── favicon.svg
└── src/
    ├── main.ts                # createApp, Pinia, Router, mount
    ├── App.vue                # RouterView only
    ├── style.css              # Tailwind directives + CSS variables
    ├── lib/
    │   └── utils.ts           # cn() utility (clsx + tailwind-merge)
    ├── router/
    │   └── index.ts           # Routes + auth guards
    ├── stores/
    │   └── auth.ts            # Pinia auth store (login/logout/token)
    ├── api/
    │   └── client.ts          # Fetch wrapper with Bearer injection
    ├── types/
    │   └── api.ts             # TypeScript types (Contact, Form, etc.)
    ├── views/
    │   ├── LoginView.vue      # Email, password, context selection
    │   ├── DashboardView.vue  # Metric cards + analytics
    │   ├── ContactsView.vue   # Contact list + form
    │   ├── FormsView.vue      # Form list + builder
    │   ├── TenantsView.vue    # Context CRUD
    │   └── ApiKeysView.vue    # API key management
    ├── layouts/
    │   └── AppLayout.vue      # Sidebar + Header + main area
    ├── components/
    │   ├── layout/
    │   │   ├── Sidebar.vue    # Collapsible nav (Dashboard, Contacts, etc.)
    │   │   └── Header.vue     # User email + logout button
    │   ├── ui/                # shadcn-vue components
    │   │   ├── button.ts
    │   │   ├── input.ts
    │   │   ├── card.ts
    │   │   └── badge.ts
    │   └── contacts/
    │       ├── ContactTable.vue  # Paginated table with search
    │       └── ContactForm.vue   # Create/edit form modal
    └── composables/           # Vue composables (if any)
```

### Routing

| Route | Component | Auth Required |
|-------|-----------|---------------|
| `/login` | LoginView | No (redirect to /dashboard if logged in) |
| `/dashboard` | DashboardView | Yes (redirect to /login if not) |
| `/contacts` | ContactsView | Yes |
| `/forms` | FormsView | Yes |
| `/banners` | BannersView | Yes |
| `/tenants` | TenantsView | Yes |
| `/api-keys` | ApiKeysView | Yes |
| `/*` | SPA fallback → index.html | Yes (handled by Go server) |

### Auth Flow

```
User visits /login
    │
    ▼
Enters email, password, selects context
    │
    ▼
POST /api/auth/login { email, password }
    │
    ▼
Backend returns { token, user_id, email, contexts[] }
    │
    ▼
Store token in localStorage + Pinia auth store
    │
    ▼
Redirect to /dashboard
    │
    ▼
All subsequent requests: Authorization: Bearer <token>
```

### Static File Serving (Go)

The Go backend serves the compiled Vue SPA:

```go
// ServeStatic registers static file handlers
func ServeStatic(engine *gin.Engine, dir string) {
    // Serve assets before SPA fallback
    assets := engine.Group("/assets")
    assets.StaticFS("/", http.Dir(dir+"/assets"))
    engine.StaticFS("/favicon.svg", http.Dir(dir))

    // SPA fallback
    engine.GET("/", func(c *gin.Context) {
        c.File(dir + "/index.html")
    })
    engine.GET("/index.html", func(c *gin.Context) {
        c.File(dir + "/index.html")
    })
    engine.NoRoute(func(c *gin.Context) {
        if strings.HasPrefix(c.Request.URL.Path, "/api/") {
            c.AbortWithStatus(http.StatusNotFound)
            return
        }
        c.File(dir + "/index.html")  // SPA fallback
    })
}
```

## Data Model

No new database tables. The UI consumes existing tables:

| Table | UI Pages |
|-------|----------|
| contexts | TenantsView |
| context_users | TenantsView, LoginView |
| api_keys | ApiKeysView |
| contacts | ContactsView |
| contact_tags | ContactsView |
| contact_tag_memberships | ContactsView |
| forms | FormsView |
| form_submissions | FormsView |
| banner_banners | BannersView |
| banner_placements | BannersView |
| banner_campaigns | BannersView |
| tracking_events | DashboardView |
| admin_configs | DashboardView (settings) |

## API Contracts (UI-Consumed)

| Method | Endpoint | UI Page |
|--------|----------|---------|
| `POST` | `/api/auth/login` | LoginView |
| `GET` | `/api/auth/me` | All pages (user info) |
| `POST` | `/api/api-keys` | ApiKeysView |
| `GET` | `/api/contexts` | TenantsView |
| `POST` | `/api/contexts` | TenantsView |
| `PUT` | `/api/contexts/:id` | TenantsView |
| `DELETE` | `/api/contexts/:id` | TenantsView |
| `GET` | `/api/contacts` | ContactsView |
| `POST` | `/api/contacts` | ContactsView |
| `PUT` | `/api/contacts/:id` | ContactsView |
| `DELETE` | `/api/contacts/:id` | ContactsView |
| `POST` | `/api/contacts/:id/merge` | ContactsView |
| `GET` | `/api/forms` | FormsView |
| `POST` | `/api/forms` | FormsView |
| `PUT` | `/api/forms/:id` | FormsView |
| `DELETE` | `/api/forms/:id` | FormsView |
| `GET` | `/api/banners` | BannersView |
| `GET` | `/api/tracking/dashboard` | DashboardView |

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| JWT storage | localStorage (not cookie) | Same-origin SPA; simpler client-side flow |
| Build output | `web/dist/` | Go serves from single directory; Docker COPY simple |
| No BFF | Go serves static files directly | Eliminates auth cookie complexity; same-origin = no CORS |
| Component library | shadcn-vue | Headless, accessible, Tailwind-native, copy-editable |
| State management | Pinia | Official Vue 3 store; simple API; TypeScript support |
| Routing | vue-router | Standard Vue routing; supports guards |
| CSS | Tailwind CSS 3 + CSS variables | Utility-first; dark mode ready via variables |
| Build tool | Vite 5 | Fast HMR; ES modules; TypeScript support |
| Component chunks | Vite code splitting | Lazy-loaded routes; small initial bundle |
| No SSR | SPA only | Simpler deployment; Go serves index.html |
