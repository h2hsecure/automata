# Web UI Implementation Plan

> **For agentic workers:** Use subagent-driven-development or executing-plans to implement task-by-task.

**Goal:** Build a Vue 3 SPA with login, dashboard, and contacts management, baked into the Go binary and served at root `/`.

**Architecture:** Vue 3 + Vite + TypeScript + shadcn-vue builds to `web/dist/`. Go serves the static files at `/` and the API at `/api/*`. JWT stored in localStorage, sent via `Authorization: Bearer <token>` header. No BFF - same-origin communication.

**Tech Stack:** Vue 3 (Composition API), Vite 5, TypeScript, shadcn-vue, Tailwind CSS 3, Pinia, Vue Router 4, Lucide Vue

**Spec:** `docs/webui-prd.md` (Phase 1 scope: Login + Dashboard + Contacts)

## Global Constraints

- Go 1.26.5, Gin v1.12.0, PostgreSQL (pgx v5)
- JWT HS256 signing, 24h expiry, claims: `user_id`, `tenant_slug`, `exp`
- API base URL: `/api/*` (same origin, no CORS)
- Auth: `Authorization: Bearer <token>` header on all authenticated requests
- Pagination pattern: `{ contacts, total, page, limit }`
- Error responses: `{ "error": "message" }` with HTTP status code
- Docker Compose for local development, Alpine-based multi-stage builds

---

### Task 1: Scaffold Vue 3 Project

**Files:**
- Create: `webui/package.json`, `webui/tsconfig.json`, `webui/vite.config.ts`, `webui/tailwind.config.ts`, `webui/postcss.config.js`, `webui/index.html`, `webui/components.json`
- Create: `webui/src/main.ts`, `webui/src/App.vue`, `webui/src/style.css`, `webui/src/lib/utils.ts`, `webui/public/favicon.svg`

**Interfaces:** Produces a working Vite + TypeScript + Vue 3 project that compiles and runs with `pnpm dev`

- [ ] **Step 1: Create package.json**

```json
{
  "name": "automata-webui",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "vue": "^3.4.0",
    "vue-router": "^4.3.0",
    "pinia": "^2.2.0",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "tailwind-merge": "^2.4.0",
    "tailwindcss-animate": "^1.0.7",
    "lucide-vue-next": "^0.400.0",
    "@tanstack/vue-table": "^8.19.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.1.0",
    "typescript": "^5.5.0",
    "vue-tsc": "^2.0.0",
    "vite": "^5.4.0",
    "tailwindcss": "^3.4.0",
    "autoprefixer": "^10.4.0",
    "postcss": "^8.4.0",
    "@types/node": "^20.14.0"
  }
}
```

- [ ] **Step 2: Create tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "module": "ESNext",
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "preserve",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src/**/*.ts", "src/**/*.tsx", "src/**/*.vue"]
}
```

- [ ] **Step 3: Create vite.config.ts**

```ts
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  }
})
```

- [ ] **Step 4: Create tailwind.config.ts**

```ts
import type { Config } from 'tailwindcss'
import animate from 'tailwindcss-animate'

export default {
  darkMode: ['class'],
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        border: 'hsl(var(--border))',
        input: 'hsl(var(--input))',
        ring: 'hsl(var(--ring))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: { DEFAULT: 'hsl(var(--primary))', foreground: 'hsl(var(--primary-foreground))' },
        secondary: { DEFAULT: 'hsl(var(--secondary))', foreground: 'hsl(var(--secondary-foreground))' },
        destructive: { DEFAULT: 'hsl(var(--destructive))', foreground: 'hsl(var(--destructive-foreground))' },
        muted: { DEFAULT: 'hsl(var(--muted))', foreground: 'hsl(var(--muted-foreground))' },
        accent: { DEFAULT: 'hsl(var(--accent))', foreground: 'hsl(var(--accent-foreground))' },
        card: { DEFAULT: 'hsl(var(--card))', foreground: 'hsl(var(--card-foreground))' }
      },
      borderRadius: { lg: 'var(--radius)', md: 'calc(var(--radius) - 2px)', sm: 'calc(var(--radius) - 4px)' }
    }
  },
  plugins: [animate]
} satisfies Config
```

- [ ] **Step 5: Create postcss.config.js**

```js
export default {
  plugins: { tailwindcss: {}, autoprefixer: {} }
}
```

- [ ] **Step 6: Create index.html**

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Automata</title>
    <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
  </head>
  <body class="bg-background text-foreground">
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **Step 7: Create components.json**

```json
{
  "$schema": "https://shadcn-vue.com/schema.json",
  "style": "new-york",
  "typescript": true,
  "tailwind": { "config": "", "css": "src/style.css", "baseColor": "zinc", "cssVariables": true },
  "aliases": { "components": "@/components", "utils": "@/lib/utils", "ui": "@/components/ui", "lib": "@/lib", "composables": "@/composables" },
  "icon": "lucide"
}
```

- [ ] **Step 8: Create src/style.css** (Tailwind directives + CSS variables for light/dark themes)

- [ ] **Step 9: Create src/lib/utils.ts** (cn() utility using clsx + tailwind-merge)

- [ ] **Step 10: Create src/main.ts** (createApp, createPinia, mount)

- [ ] **Step 11: Create src/App.vue** (placeholder "Loading...")

- [ ] **Step 12: Create public/favicon.svg** (purple "A" icon)

- [ ] **Step 13: Install dependencies and verify dev server starts**
  Run: `cd webui && pnpm install && pnpm dev`
  Expected: Vite dev server on `http://localhost:5173` showing "Loading..."

- [ ] **Step 14: Commit**

```bash
git add webui/
git commit -m "feat: scaffold Vue 3 + shadcn-vue project"
```

---

### Task 2: Set Up Routing, Auth Store, and API Client

**Files:**
- Create: `webui/src/router/index.ts`, `webui/src/stores/auth.ts`, `webui/src/api/client.ts`, `webui/src/types/api.ts`
- Modify: `webui/src/App.vue`, `webui/src/main.ts`

**Interfaces:**
- Consumes: Vue Router, Pinia, TypeScript
- Produces: `authStore` with `login(email, password, tenant)`, `logout()`, `isLoggedIn`, `token`, `user`; API client with `get(path)`, `post(path, body)`, `delete(path)` that auto-injects `Authorization: Bearer <token>`

- [ ] **Step 1: Create src/types/api.ts** (LoginResponse, LoginRequest, Contact, Tag, PaginatedResponse<T>, DashboardMetrics)

- [ ] **Step 2: Create src/api/client.ts** (fetch wrapper with auth header injection, login/logout functions)

- [ ] **Step 3: Create src/stores/auth.ts** (Pinia store with login, logout, init from localStorage)

- [ ] **Step 4: Create src/router/index.ts** (routes for /login and /dashboard, /contacts with requiresAuth guard)

- [ ] **Step 5: Modify src/main.ts** (add router to createApp)

- [ ] **Step 6: Modify src/App.vue** (RouterView only)

- [ ] **Step 7: Verify router works**
  Run: `cd webui && pnpm dev`
  Expected: `/login` shows placeholder, `/dashboard` redirects to login

- [ ] **Step 8: Commit**

```bash
git add webui/src/
git commit -m "feat: add routing, auth store, and API client"
```

---

### Task 3: Build Login Page

**Files:**
- Create: `webui/src/views/LoginView.vue`, `webui/src/components/ui/input.ts`, `webui/src/components/ui/button.ts`, `webui/src/components/ui/card.ts`

**Interfaces:**
- Consumes: `authStore.login(email, password, tenant)`, API client
- Produces: Login page with email, password, tenant slug fields; shows error messages; redirects to `/dashboard` on success

- [ ] **Step 1: Create shadcn Input component** (`webui/src/components/ui/input.ts`)

- [ ] **Step 2: Create shadcn Button component** (`webui/src/components/ui/button.ts`)

- [ ] **Step 3: Create shadcn Card component** (`webui/src/components/ui/card.ts`)

- [ ] **Step 4: Create LoginView.vue** (form with email, password, tenant; error display; redirect on success)

- [ ] **Step 5: Verify login page renders**
  Run: `cd webui && pnpm dev`
  Expected: Login page shows with all fields and "Sign in" button

- [ ] **Step 6: Commit**

```bash
git add webui/src/views/LoginView.vue webui/src/components/ui/
git commit -m "feat: add login page with email, password, tenant fields"
```

---

### Task 4: Fix Backend Auth Middleware (tenant_id from JWT)

**Files:**
- Modify: `internal/adapter/api/middleware/auth.go:29-34`

**Interfaces:**
- Consumes: `authService.VerifyToken(token)` which returns `(userID, tenantSlug, error)`
- Produces: `tenant_id` set in Gin context when JWT auth succeeds

**Why this is needed:** The contact handler checks `c.GetString("tenant_id")` but the JWT auth path in the middleware ignores the tenant slug returned by `VerifyToken`. Without this fix, all authenticated API calls from the UI will get "tenant not found".

- [ ] **Step 1: Fix auth middleware to set tenant_id from JWT**

Change lines 29-34 in `internal/adapter/api/middleware/auth.go`:

```go
// Before:
userID, _, err := authService.VerifyToken(token)
if err == nil {
    c.Set("user_id", userID)
    c.Next()
    return
}

// After:
userID, tenantSlug, err := authService.VerifyToken(token)
if err == nil {
    c.Set("user_id", userID)
    c.Set("tenant_id", tenantSlug)
    c.Next()
    return
}
```

- [ ] **Step 2: Verify Go compiles**
  Run: `go build ./cmd/automata/`
  Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/api/middleware/auth.go
git commit -m "fix: set tenant_id in context for JWT auth (was only set for API key auth)"
```

---

### Task 5: Build App Layout (Sidebar + Header)

**Files:**
- Create: `webui/src/layouts/AppLayout.vue`, `webui/src/components/layout/Sidebar.vue`, `webui/src/components/layout/Header.vue`

**Interfaces:**
- Consumes: `authStore.logout`, Vue Router
- Produces: Layout with collapsible sidebar (Dashboard, Contacts nav items), header with user email + logout button

- [ ] **Step 1: Create Sidebar.vue** (collapsible, nav items with icons, active route highlighting)

- [ ] **Step 2: Create Header.vue** (user email display, logout button)

- [ ] **Step 3: Create AppLayout.vue** (sidebar + header + main content area with RouterView)

- [ ] **Step 4: Verify layout renders**
  Run: `cd webui && pnpm dev`
  Expected: After login, shows sidebar with Dashboard/Contacts, header with email + logout

- [ ] **Step 5: Commit**

```bash
git add webui/src/layouts/ webui/src/components/layout/
git commit -m "feat: add app layout with sidebar navigation and header"
```

---

### Task 6: Build Dashboard Page

**Files:**
- Create: `webui/src/views/DashboardView.vue`, `webui/src/components/ui/badge.ts`

**Interfaces:**
- Consumes: API endpoints for metrics (uses mock data initially, wired to real API in Task 8)
- Produces: Dashboard with metric cards showing contacts count, forms count, banners count, visitors

- [ ] **Step 1: Create Badge component** (`webui/src/components/ui/badge.ts`)

- [ ] **Step 2: Create DashboardView.vue** (metric cards grid, top referrers list, device breakdown)

- [ ] **Step 3: Wire to real API** (fetch dashboard metrics endpoint)

- [ ] **Step 4: Verify dashboard renders**
  Run: `cd webui && pnpm dev`
  Expected: Dashboard shows metric cards with data from API

- [ ] **Step 5: Commit**

```bash
git add webui/src/views/DashboardView.vue webui/src/components/ui/badge.ts
git commit -m "feat: add dashboard page with metrics cards"
```

---

### Task 7: Build Contacts Page

**Files:**
- Create: `webui/src/views/ContactsView.vue`, `webui/src/components/contacts/ContactTable.vue`, `webui/src/components/contacts/ContactForm.vue`

**Interfaces:**
- Consumes: `api.get<PaginatedResponse<Contact>>('/contacts')`, `api.post('/contacts')`, `api.put('/contacts/:id')`, `api.delete('/contacts/:id')`
- Produces: Contacts list with pagination, search, create/edit/delete functionality

- [ ] **Step 1: Create ContactTable.vue** (sortable columns, pagination, search input)

- [ ] **Step 2: Create ContactForm.vue** (modal/form for create and edit, validation)

- [ ] **Step 3: Create ContactsView.vue** (table + form integration, fetch contacts on mount)

- [ ] **Step 4: Wire to real API** (all CRUD operations)

- [ ] **Step 5: Verify contacts page works**
  Run: `cd webui && pnpm dev`
  Expected: Can list, create, edit, delete contacts

- [ ] **Step 6: Commit**

```bash
git add webui/src/views/ContactsView.vue webui/src/components/contacts/
git commit -m "feat: add contacts page with CRUD operations"
```

---

### Task 8: Build and Integrate with Go Backend

**Files:**
- Modify: `webui/vite.config.ts` (build output to `web/dist/`)
- Modify: `Makefile` (add `webui` build target)
- Modify: `Dockerfile` (copy `web/dist/` instead of `web/`)

**Interfaces:**
- Consumes: Go static file serving (already implemented in Task 0)
- Produces: Vue app built to `web/dist/`, served by Go at root `/`

- [ ] **Step 1: Update vite.config.ts** to output to `../web/dist/`

- [ ] **Step 2: Add Makefile target** for building webui

- [ ] **Step 3: Update Dockerfile** to copy `web/dist/`

- [ ] **Step 4: Build and test end-to-end**
  Run: `make build` (Go) + `cd webui && pnpm build`
  Run: `make up` (Docker Compose)
  Expected: Full app loads at `http://localhost:8080` with login, dashboard, contacts

- [ ] **Step 5: Commit**

```bash
git add webui/vite.config.ts Makefile Dockerfile
git commit -m "feat: integrate Vue SPA with Go backend build pipeline"
```

---

## Post-Implementation

After all tasks are complete:
1. Run Playwright E2E tests: `cd tests/e2e && npx playwright test`
2. Run Go tests: `make cicd-tests`
3. Update `docs/webui-prd.md` with completion status
4. Document API endpoints used by the SPA in `docs/api-webui.md`
