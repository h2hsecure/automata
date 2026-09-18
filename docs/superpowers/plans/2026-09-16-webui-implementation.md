# Web UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Automata Web UI — a Vue 3 single-page application with a Go BFF (Backend-for-Frontend) that provides a complete admin interface for managing contacts, forms, banners, email templates, tenants, and tracking analytics.

**Architecture:** Vue 3 SPA + Go BFF + existing Go API backend. The BFF sits between the browser and the API, handling auth sessions and serving the Vue dist.

**Spec:** docs/webui-prd.md

## Global Constraints

- **Hexagonal architecture** for BFF: domain interfaces in `internal/domain`, use cases in `internal/usecase`, adapters in `internal/adapter`, infrastructure in `internal/infrastructure`
- **KISS and DRY** principles throughout
- **Multi-tenant**: all feature tables include `tenant_id`, data strictly isolated per tenant
- **Vue 3 SPA**: Composition API, `<script setup>`, TypeScript, Vite 5, shadcn-vue, Tailwind CSS 4, Pinia, Vue Router 4
- **BFF**: Go (Gin), httpOnly cookies for JWT, serves Vue dist as static files
- **API conventions**: `Authorization: Bearer <token>`, error responses `{ "error": "message" }`, pagination `{ data, total, page, limit }`, timestamps ISO 8601
- **Performance targets**: < 2s initial page load, < 200KB gzipped initial bundle, lazy-loaded routes
- **No SSR/Nuxt** — SPA only for v1
- **No WebSocket** — polling acceptable for v1
- **All tests**: Vitest (unit), Playwright (E2E)

## Assumptions (from PRD open questions)

| Question | Assumption |
|----------|-----------|
| Email template storage | Stored in DB; backend adds `POST /admin/email-templates` endpoint |
| Admin vs tenant user | Auth response includes `is_owner` boolean claim |
| Real-time updates | Dashboard polls every 30s; no WebSocket for v1 |
| Form builder complexity | Basic fields only (text, email, textarea, select, checkbox, radio, file upload); no conditional logic, no reCAPTCHA |
| BFF technology | Go (same codebase, same deployment patterns) |
| File uploads | Out of scope for v1 |
| Webhook management | Out of scope for v1 |
| Activity timeline | Included; backend `GetActivity` handler wired into server routing |

---

### Task 1: Vue project scaffolding

**Files:**
- Create: `ui/package.json`
- Create: `ui/vite.config.ts`
- Create: `ui/tsconfig.json`
- Create: `ui/tsconfig.node.json`
- Create: `ui/tailwind.config.ts`
- Create: `ui/index.html`
- Create: `ui/components.json` (shadcn-vue config)
- Create: `ui/src/main.ts`
- Create: `ui/src/App.vue`
- Create: `ui/src/styles.css` (Tailwind imports + base styles)
- Create: `ui/.gitignore`
- Create: `ui/public/vite.svg`

**Interfaces:**
- Consumes: none (foundation layer)
- Produces: Vue 3 project with TypeScript, Vite, Tailwind CSS, shadcn-vue ready

**Steps:**

- [ ] **Step 1: Initialize project structure**

Create `ui/` directory with the following structure:

```
ui/
+-- public/
|   +-- vite.svg
+-- src/
|   +-- App.vue
|   +-- main.ts
|   +-- styles.css
+-- components.json
+-- index.html
+-- package.json
+-- tailwind.config.ts
+-- tsconfig.json
+-- tsconfig.node.json
+-- vite.config.ts
```

- [ ] **Step 2: Write package.json**

```json
{
  "name": "automata-webui",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc -b && vite build",
    "preview": "vite preview",
    "lint": "vue-tsc -b",
    "test": "vitest",
    "test:e2e": "playwright test",
    "shadcn": "shadcn-vue init"
  },
  "dependencies": {
    "@hey-api/client-fetch": "^0.8.0",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "lucide-vue-next": "^0.460.0",
    "pinia": "^2.2.0",
    "tailwind-merge": "^2.5.0",
    "tailwindcss-animate": "^1.0.7",
    "vee-validate": "^4.13.0",
    "vue": "^3.5.0",
    "vue-router": "^4.4.0",
    "zod": "^3.23.0"
  },
  "devDependencies": {
    "@hey-api/openapi-ts": "^0.57.0",
    "@playwright/test": "^1.48.0",
    "@vitejs/plugin-vue": "^5.1.0",
    "autoprefixer": "^10.4.20",
    "jsdom": "^25.0.0",
    "postcss": "^8.4.47",
    "tailwindcss": "^3.4.13",
    "typescript": "~5.6.0",
    "vite": "^5.4.0",
    "vitest": "^2.1.0",
    "vue-tsc": "^2.1.0"
  }
}
```

- [ ] **Step 3: Write vite.config.ts**

```typescript
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
      '/admin': 'http://localhost:8080'
    }
  }
})
```

- [ ] **Step 4: Write tsconfig.json**

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

- [ ] **Step 5: Write tailwind.config.ts**

```typescript
import type { Config } from 'tailwindcss'
import animate from 'tailwindcss-animate'

export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        border: 'hsl(var(--border))',
        input: 'hsl(var(--input))',
        ring: 'hsl(var(--ring))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))'
        },
        secondary: {
          DEFAULT: 'hsl(var(--secondary))',
          foreground: 'hsl(var(--secondary-foreground))'
        },
        destructive: {
          DEFAULT: 'hsl(var(--destructive))',
          foreground: 'hsl(var(--destructive-foreground))'
        },
        muted: {
          DEFAULT: 'hsl(var(--muted))',
          foreground: 'hsl(var(--muted-foreground))'
        },
        accent: {
          DEFAULT: 'hsl(var(--accent))',
          foreground: 'hsl(var(--accent-foreground))'
        },
        card: {
          DEFAULT: 'hsl(var(--card))',
          foreground: 'hsl(var(--card-foreground))'
        }
      },
      borderRadius: {
        lg: 'var(--radius)',
        md: 'calc(var(--radius) - 2px)',
        sm: 'calc(var(--radius) - 4px)'
      }
    }
  },
  plugins: [animate]
} satisfies Config
```

- [ ] **Step 6: Write styles.css**

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 222.2 84% 4.9%;
    --primary: 238 77% 63%;
    --primary-foreground: 210 40% 98%;
    --card: 0 0% 100%;
    --card-foreground: 222.2 84% 4.9%;
    --radius: 0.5rem;
  }
  .dark {
    --background: 222.2 84% 4.9%;
    --foreground: 210 40% 98%;
    --primary: 238 77% 63%;
    --primary-foreground: 210 40% 98%;
    --card: 222.2 84% 4.9%;
    --card-foreground: 210 40% 98%;
  }
}

@layer base {
  * {
    @apply border-border;
  }
  body {
    @apply bg-background text-foreground;
    font-family: 'Inter', system-ui, -apple-system, sans-serif;
  }
}
```

- [ ] **Step 7: Write index.html**

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Automata — Marketing Automation</title>
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet" />
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **Step 8: Write components.json (shadcn-vue)**

```json
{
  "$schema": "https://shadcn-vue.com/schema.json",
  "style": "new-york",
  "typescript": true,
  "tailwind": {
    "config": "tailwind.config.ts",
    "css": "src/styles.css"
  },
  "aliases": {
    "components": "@/components",
    "utils": "@/lib/utils",
    "ui": "@/components/ui",
    "lib": "@/lib",
    "composables": "@/composables"
  },
  "icon": "lucide"
}
```

- [ ] **Step 9: Install dependencies**

Run: `cd ui && npm install`

- [ ] **Step 10: Install shadcn-vue components**

Run: `cd ui && npx shadcn-vue@latest add button card input table dialog dropdown-menu skeleton badge avatar tabs select checkbox radio-group switch combobox command popover tooltip alert progress accordion separator sheet drawer label`

---

### Task 2: BFF server scaffolding

**Files:**
- Create: `cmd/webui/main.go`
- Create: `internal/domain/webui/session.go`
- Create: `internal/adapter/webui/server.go`
- Create: `internal/adapter/webui/handler/auth.go`
- Create: `internal/adapter/webui/middleware/cookie.go`
- Create: `internal/infrastructure/webui/cookie/manager.go`
- Create: `ui/Dockerfile`
- Modify: `deployment/docker-compose.yml` (add webui service)

**Interfaces:**
- Consumes: none (foundation layer)
- Produces: `webui.Server` with `/login`, `/logout`, `/auth/refresh`, static file serving

**Steps:**

- [ ] **Step 1: Create BFF project structure**

```
cmd/webui/
+-- main.go
internal/
+-- domain/webui/
|   +-- session.go
+-- adapter/webui/
|   +-- server.go
|   +-- handler/
|   |   +-- auth.go
|   +-- middleware/
|   |   +-- cookie.go
+-- infrastructure/webui/
    +-- cookie/
        +-- manager.go
```

- [ ] **Step 2: Write cmd/webui/main.go**

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hekemen/automata/internal/adapter/webui"
	"github.com/hekemen/automata/internal/adapter/webui/handler"
)

func main() {
	port := os.Getenv("WEBUI_PORT")
	if port == "" {
		port = "3000"
	}

	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	cookieSecret := os.Getenv("COOKIE_SECRET")
	if cookieSecret == "" {
		cookieSecret = "dev-secret-change-in-production"
	}

	srv := webui.NewServer(apiURL, cookieSecret)
	authHandler := handler.NewAuthHandler(srv.HTTPClient(), apiURL, cookieSecret)
	srv.RegisterRoutes(authHandler)

	distDir := os.Getenv("VUE_DIST_DIR")
	if distDir == "" {
		distDir = "./ui/dist"
	}
	srv.ServeStatic(distDir)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("WebUI server starting on %s (API: %s)", addr, apiURL)
	if err := srv.Listen(addr); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 3: Write internal/domain/webui/session.go**

```go
package webui

import "time"

// Session represents an authenticated user session.
type Session struct {
	UserID     string    `json:"user_id"`
	Email      string    `json:"email"`
	TenantID   string    `json:"tenant_id"`
	TenantSlug string    `json:"tenant_slug"`
	IsOwner    bool      `json:"is_owner"`
	CreatedAt  time.Time `json:"created_at"`
}

// CookieConfig holds cookie settings.
type CookieConfig struct {
	Secret   string
	HTTPOnly bool
	Secure   bool
	SameSite string
	MaxAge   int
	Domain   string
}
```

- [ ] **Step 4: Write internal/adapter/webui/server.go**

```go
package webui

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/adapter/webui/handler"
)

// Server is the BFF HTTP server.
type Server struct {
	engine     *gin.Engine
	apiURL     string
	cookieCfg  CookieConfig
	httpClient *http.Client
}

// NewServer creates a new BFF server.
func NewServer(apiURL, cookieSecret string) *Server {
	return &Server{
		engine:     gin.Default(),
		apiURL:     apiURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cookieCfg: CookieConfig{
			Secret:   cookieSecret,
			HTTPOnly: true,
			Secure:   false, // set true in production
			SameSite: "lax",
			MaxAge:   86400, // 24 hours
		},
	}
}

// RegisterRoutes sets up all BFF routes.
func (s *Server) RegisterRoutes(authHandler *handler.AuthHandler) {
	s.engine.POST("/auth/login", authHandler.Login)
	s.engine.POST("/auth/logout", authHandler.Logout)
	s.engine.POST("/auth/refresh", authHandler.Refresh)
}

// ServeStatic serves the Vue SPA dist.
func (s *Server) ServeStatic(distDir string) {
	s.engine.StaticFS("/", http.Dir(distDir))
	// SPA fallback: serve index.html for all non-file routes
	s.engine.NoRoute(func(c *gin.Context) {
		if !s.serveStatic(c) {
			c.File(distDir + "/index.html")
		}
	})
}

func (s *Server) serveStatic(c *gin.Context) bool {
	if c.Request.URL.Path == "/" || c.Request.URL.Path == "/login" {
		return false
	}
	if _, err := http.Dir(".").Open(c.Request.URL.Path); err == nil {
		http.FileServer(http.Dir(".")).ServeHTTP(c.Writer, c.Request)
		return true
	}
	return false
}

// Listen starts the HTTP server.
func (s *Server) Listen(addr string) error {
	return s.engine.Run(addr)
}

// APIURL returns the backend API URL for proxying.
func (s *Server) APIURL() string {
	return s.apiURL
}

// HTTPClient returns the shared HTTP client.
func (s *Server) HTTPClient() *http.Client {
	return s.httpClient
}

// CookieConfig returns the cookie configuration.
func (s *Server) CookieConfig() CookieConfig {
	return s.cookieCfg
}
```

- [ ] **Step 5: Write internal/adapter/webui/handler/auth.go**

```go
package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/adapter/webui"
)

// AuthHandler handles authentication proxying.
type AuthHandler struct {
	client    *http.Client
	apiURL    string
	cookieCfg webui.CookieConfig
}

func NewAuthHandler(client *http.Client, apiURL, cookieSecret string) *AuthHandler {
	return &AuthHandler{
		client:    client,
		apiURL:    apiURL,
		cookieCfg: webui.CookieConfig{Secret: cookieSecret},
	}
}

// LoginRequest is the login form body.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	Tenant   string `json:"tenant" binding:"required"`
}

// Login proxies to the backend auth endpoint and sets the session cookie.
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	resp, err := h.proxyRequest(c, "POST", "/auth/login", req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "auth service unavailable"})
		return
	}
	defer resp.Body.Close()

	var result struct {
		Token      string `json:"token"`
		UserID     string `json:"user_id"`
		TenantID   string `json:"tenant_id"`
		Email      string `json:"email"`
		TenantSlug string `json:"tenant_slug"`
		IsOwner    bool   `json:"is_owner"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid response"})
		return
	}

	session := webui.Session{
		UserID:     result.UserID,
		Email:      result.Email,
		TenantID:   result.TenantID,
		TenantSlug: result.TenantSlug,
		IsOwner:    result.IsOwner,
	}
	cookie := h.buildSessionCookie(session)
	http.SetCookie(c.Writer, cookie)

	c.JSON(http.StatusOK, gin.H{
		"token":     result.Token,
		"user_id":   result.UserID,
		"tenant_id": result.TenantID,
		"email":     result.Email,
		"is_owner":  result.IsOwner,
	})
}

// Logout clears the session cookie.
func (h *AuthHandler) Logout(c *gin.Context) {
	cookie := h.buildExpiredCookie()
	http.SetCookie(c.Writer, cookie)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// Refresh extends the session cookie.
func (h *AuthHandler) Refresh(c *gin.Context) {
	cookie, err := c.Request.Cookie("automata_session")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	resp, err := h.proxyRequest(c, "POST", "/auth/refresh", nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "refresh failed"})
		return
	}
	defer resp.Body.Close()

	var result struct {
		UserID     string `json:"user_id"`
		Email      string `json:"email"`
		TenantID   string `json:"tenant_id"`
		TenantSlug string `json:"tenant_slug"`
		IsOwner    bool   `json:"is_owner"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	session := webui.Session{
		UserID:     result.UserID,
		Email:      result.Email,
		TenantID:   result.TenantID,
		TenantSlug: result.TenantSlug,
		IsOwner:    result.IsOwner,
	}
	http.SetCookie(c.Writer, h.buildSessionCookie(session))
	c.JSON(http.StatusOK, gin.H{"message": "refreshed"})
}

func (h *AuthHandler) proxyRequest(c *gin.Context, method, path string, body interface{}) (*http.Response, error) {
	url := h.apiURL + path
	var req *http.Request
	var err error

	if body != nil {
		data, _ := json.Marshal(body)
		req, _ = http.NewRequest(method, url, strings.NewReader(string(data)))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, url, nil)
	}

	resp, err := h.client.Do(req)
	return resp, err
}

func (h *AuthHandler) buildSessionCookie(session webui.Session) *http.Cookie {
	return &http.Cookie{
		Name:     "automata_session",
		Value:    session.UserID,
		Path:     "/",
		HttpOnly: h.cookieCfg.HTTPOnly,
		Secure:   h.cookieCfg.Secure,
		MaxAge:   h.cookieCfg.MaxAge,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *AuthHandler) buildExpiredCookie() *http.Cookie {
	return &http.Cookie{
		Name:     "automata_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	}
}
```

- [ ] **Step 6: Write internal/infrastructure/webui/cookie/manager.go**

```go
package cookie

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/hekemen/automata/internal/adapter/webui"
)

// Manager handles cookie creation and validation.
type Manager struct {
	secret string
}

// New creates a new cookie manager.
func New(secret string) *Manager {
	return &Manager{secret: secret}
}

// CreateSessionCookie creates a signed session cookie.
func (m *Manager) CreateSessionCookie(session webui.Session) *http.Cookie {
	return &http.Cookie{
		Name:     "automata_session",
		Value:    session.UserID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		MaxAge:   86400,
		SameSite: http.SameSiteLaxMode,
	}
}

// GenerateCSRFToken generates a CSRF token.
func (m *Manager) GenerateCSRFToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
```

- [ ] **Step 7: Write ui/Dockerfile**

```dockerfile
FROM node:20-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

- [ ] **Step 8: Add webui service to docker-compose.yml**

Add to `deployment/docker-compose.yml`:

```yaml
  webui:
    build:
      context: ../ui
      dockerfile: Dockerfile
    ports:
      - "3000:80"
    environment:
      - API_URL=http://automata:8080
    depends_on:
      - automata
    restart: unless-stopped
```

---

### Task 3: Auth flow — login, logout, token refresh

**Files:**
- Create: `ui/src/stores/auth.ts`
- Create: `ui/src/composables/useAuth.ts`
- Create: `ui/src/api/client.ts`
- Create: `ui/src/router/guards.ts`
- Create: `ui/src/views/LoginView.vue`
- Create: `ui/src/components/ui/login-form.vue`

**Interfaces:**
- Consumes: BFF `/auth/login`, `/auth/logout`, `/auth/refresh` endpoints
- Produces: `useAuth()` composable with `login()`, `logout()`, `refresh()`, `isAuthenticated`

**Steps:**

- [ ] **Step 1: Write API client with auth interceptor**

```typescript
// ui/src/api/client.ts
import { ofetch } from 'ofetch'

const apiClient = ofetch.create({
  baseURL: '/', // BFF proxies to backend
  credentials: 'include', // send httpOnly cookies
  headers: {
    'Content-Type': 'application/json',
  },
  onResponseError(context) {
    if (context.response.status === 401) {
      window.location.href = '/login'
    }
  },
})

export { apiClient }
```

- [ ] **Step 2: Write auth store**

```typescript
// ui/src/stores/auth.ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { apiClient } from '@/api/client'

export interface User {
  id: string
  email: string
  tenant_id: string
  tenant_slug: string
  is_owner: boolean
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(null)
  const user = ref<User | null>(null)
  const loading = ref(false)

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.is_owner ?? false)

  async function login(email: string, password: string, tenant: string) {
    loading.value = true
    try {
      const result = await apiClient('/auth/login', {
        method: 'POST',
        body: { email, password, tenant },
      })
      token.value = result.token
      user.value = {
        id: result.user_id,
        email: result.email,
        tenant_id: result.tenant_id,
        tenant_slug: result.tenant_slug,
        is_owner: result.is_owner,
      }
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    await apiClient('/auth/logout', { method: 'POST' })
    token.value = null
    user.value = null
  }

  async function refresh() {
    try {
      await apiClient('/auth/refresh', { method: 'POST' })
    } catch {
      await logout()
    }
  }

  return { token, user, loading, isAuthenticated, isAdmin, login, logout, refresh }
})
```

- [ ] **Step 3: Write useAuth composable**

```typescript
// ui/src/composables/useAuth.ts
import { useAuthStore } from '@/stores/auth'

export function useAuth() {
  const auth = useAuthStore()
  return auth
}
```

- [ ] **Step 4: Write router guards**

```typescript
// ui/src/router/guards.ts
import { useAuthStore } from '@/stores/auth'

export function setupRouterGuards(router: any) {
  router.beforeEach((to: any) => {
    const auth = useAuthStore()

    if (to.meta.requiresAuth && !auth.isAuthenticated) {
      return { path: '/login', query: { redirect: to.fullPath } }
    }

    if (to.meta.guest && auth.isAuthenticated) {
      return { path: '/dashboard' }
    }

    if (to.meta.requiresAdmin && !auth.isAdmin) {
      return { path: '/dashboard' }
    }
  })
}
```

- [ ] **Step 5: Write LoginView**

```vue
<!-- ui/src/views/LoginView.vue -->
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { LoginForm } from '@/components/ui/login-form'

const router = useRouter()
const route = useRoute()
const auth = useAuth()

const email = ref('')
const password = ref('')
const tenant = ref('')
const error = ref('')

async function handleLogin() {
  try {
    error.value = ''
    await auth.login(email.value, password.value, tenant.value)
    const redirect = (route.query.redirect as string) || '/dashboard'
    router.push(redirect)
  } catch (e: any) {
    error.value = e.response?.data?.error || 'Login failed'
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-50">
    <div class="w-full max-w-md space-y-8 p-8">
      <div class="text-center">
        <h1 class="text-3xl font-bold text-gray-900">Automata</h1>
        <p class="mt-2 text-gray-600">Sign in to your account</p>
      </div>

      <LoginForm
        v-model:email="email"
        v-model:password="password"
        v-model:tenant="tenant"
        @submit="handleLogin"
        :loading="auth.loading"
      />

      <div v-if="error" class="text-red-600 text-sm text-center">
        {{ error }}
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 6: Write LoginForm component**

```vue
<!-- ui/src/components/ui/login-form.vue -->
<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface Props {
  email: string
  password: string
  tenant: string
  loading: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  submit: []
  'update:email': [value: string]
  'update:password': [value: string]
  'update:tenant': [value: string]
}>()
</script>

<template>
  <form @submit.prevent="emit('submit')" class="space-y-4">
    <div class="space-y-2">
      <Label for="tenant">Tenant Slug</Label>
      <Input
        id="tenant"
        v-model="props.tenant"
        placeholder="mycompany"
        required
        @input="emit('update:tenant', ($event.target as HTMLInputElement).value)"
      />
    </div>

    <div class="space-y-2">
      <Label for="email">Email</Label>
      <Input
        id="email"
        type="email"
        v-model="props.email"
        placeholder="you@company.com"
        required
        @input="emit('update:email', ($event.target as HTMLInputElement).value)"
      />
    </div>

    <div class="space-y-2">
      <Label for="password">Password</Label>
      <Input
        id="password"
        type="password"
        v-model="props.password"
        required
        @input="emit('update:password', ($event.target as HTMLInputElement).value)"
      />
    </div>

    <Button type="submit" class="w-full" :disabled="props.loading">
      {{ props.loading ? 'Signing in...' : 'Sign in' }}
    </Button>
  </form>
</template>
```

---

### Task 4: Layout shell — sidebar, header, main content area

**Files:**
- Create: `ui/src/App.vue`
- Create: `ui/src/layouts/MainLayout.vue`
- Create: `ui/src/components/layout/Sidebar.vue`
- Create: `ui/src/components/layout/Header.vue`
- Create: `ui/src/components/layout/Breadcrumb.vue`
- Create: `ui/src/router/index.ts`

**Interfaces:**
- Consumes: `useAuth()` composable
- Produces: App shell with responsive sidebar navigation, user menu header, breadcrumb trail

**Steps:**

- [ ] **Step 1: Write router index with lazy-loaded routes**

```typescript
// ui/src/router/index.ts
import { createRouter, createWebHistory } from 'vue-router'
import { setupRouterGuards } from './guards'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/LoginView.vue'),
      meta: { guest: true },
    },
    {
      path: '/',
      component: () => import('@/layouts/MainLayout.vue'),
      children: [
        {
          path: '',
          name: 'Dashboard',
          component: () => import('@/views/DashboardView.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'contacts',
          name: 'Contacts',
          component: () => import('@/views/ContactsView.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'forms',
          name: 'Forms',
          component: () => import('@/views/FormsView.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'banners',
          name: 'Banners',
          component: () => import('@/views/BannersView.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'email-templates',
          name: 'EmailTemplates',
          component: () => import('@/views/EmailTemplatesView.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'tenants',
          name: 'Tenants',
          component: () => import('@/views/TenantsView.vue'),
          meta: { requiresAuth: true, requiresAdmin: true },
        },
        {
          path: 'analytics',
          name: 'Analytics',
          component: () => import('@/views/AnalyticsView.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'settings',
          name: 'Settings',
          component: () => import('@/views/SettingsView.vue'),
          meta: { requiresAuth: true },
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/dashboard',
    },
  ],
})

setupRouterGuards(router)
export default router
```

- [ ] **Step 2: Write App.vue**

```vue
<!-- ui/src/App.vue -->
<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>

<template>
  <RouterView />
</template>
```

- [ ] **Step 3: Write main.ts**

```typescript
// ui/src/main.ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './styles.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
```

- [ ] **Step 4: Write MainLayout**

```vue
<!-- ui/src/layouts/MainLayout.vue -->
<script setup lang="ts">
import { Sidebar } from '@/components/layout/Sidebar'
import { Header } from '@/components/layout/Header'
</script>

<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <Sidebar />
    <div class="lg:pl-64">
      <Header />
      <main class="p-6">
        <RouterView />
      </main>
    </div>
  </div>
</template>
```

- [ ] **Step 5: Write Sidebar**

```vue
<!-- ui/src/components/layout/Sidebar.vue -->
<script setup lang="ts">
import { useAuth } from '@/composables/useAuth'
import {
  LayoutDashboard,
  Users,
  FileText,
  Image,
  Mail,
  Building2,
  BarChart3,
  Settings,
} from 'lucide-vue-next'

const { user } = useAuth()

const navItems = [
  { name: 'Dashboard', path: '/dashboard', icon: LayoutDashboard },
  { name: 'Contacts', path: '/contacts', icon: Users },
  { name: 'Forms', path: '/forms', icon: FileText },
  { name: 'Banners', path: '/banners', icon: Image },
  { name: 'Email Templates', path: '/email-templates', icon: Mail },
  { name: 'Analytics', path: '/analytics', icon: BarChart3 },
  { name: 'Settings', path: '/settings', icon: Settings },
]

const adminItems = [
  { name: 'Tenants', path: '/tenants', icon: Building2 },
]
</script>

<template>
  <aside class="fixed inset-y-0 left-0 w-64 bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 hidden lg:block">
    <div class="flex items-center gap-2 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
      <h1 class="text-lg font-bold text-gray-900 dark:text-white">Automata</h1>
    </div>
    <nav class="p-4 space-y-1">
      <template v-for="item in navItems" :key="item.path">
        <RouterLink
          :to="item.path"
          class="flex items-center gap-3 px-3 py-2 text-sm rounded-md hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300"
        >
          <component :is="item.icon" class="w-4 h-4" />
          {{ item.name }}
        </RouterLink>
      </template>
      <template v-if="user?.is_owner">
        <div class="pt-4 pb-2 px-3 text-xs font-semibold text-gray-400 uppercase">Admin</div>
        <template v-for="item in adminItems" :key="item.path">
          <RouterLink
            :to="item.path"
            class="flex items-center gap-3 px-3 py-2 text-sm rounded-md hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300"
          >
            <component :is="item.icon" class="w-4 h-4" />
            {{ item.name }}
          </RouterLink>
        </template>
      </template>
    </nav>
  </aside>
</template>
```

- [ ] **Step 6: Write Header**

```vue
<!-- ui/src/components/layout/Header.vue -->
<script setup lang="ts">
import { useAuth } from '@/composables/useAuth'
import { Bell, Menu } from 'lucide-vue-next'
import { Breadcrumb } from '@/components/layout/Breadcrumb'

const { user, logout } = useAuth()
</script>

<template>
  <header class="sticky top-0 z-10 flex items-center justify-between px-6 py-3 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
    <div class="flex items-center gap-4">
      <button class="lg:hidden p-2 rounded-md hover:bg-gray-100 dark:hover:bg-gray-700">
        <Menu class="w-5 h-5" />
      </button>
      <Breadcrumb />
    </div>
    <div class="flex items-center gap-4">
      <button class="p-2 rounded-md hover:bg-gray-100 dark:hover:bg-gray-700">
        <Bell class="w-5 h-5 text-gray-600 dark:text-gray-400" />
      </button>
      <DropdownMenu>
        <DropdownMenuTrigger>
          <Avatar>
            <AvatarFallback>{{ user?.email?.[0]?.toUpperCase() }}</AvatarFallback>
          </Avatar>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem>
            <span>{{ user?.email }}</span>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem @click="logout">
            Sign out
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  </header>
</template>
```

- [ ] **Step 7: Write Breadcrumb**

```vue
<!-- ui/src/components/layout/Breadcrumb.vue -->
<script setup lang="ts">
import { useRoute } from 'vue-router'
import { Breadcrumb as UIBreadcrumb, BreadcrumbList, BreadcrumbItem, BreadcrumbLink, BreadcrumbPage, BreadcrumbSeparator } from '@/components/ui/breadcrumb'

const route = useRoute()

const breadcrumbItems: Record<string, { label: string; href?: string }> = {
  dashboard: { label: 'Dashboard' },
  contacts: { label: 'Contacts' },
  forms: { label: 'Forms' },
  banners: { label: 'Banners' },
  'email-templates': { label: 'Email Templates' },
  tenants: { label: 'Tenants' },
  analytics: { label: 'Analytics' },
  settings: { label: 'Settings' },
}
</script>

<template>
  <UIBreadcrumb>
    <BreadcrumbList>
      <BreadcrumbItem>
        <BreadcrumbLink :href="'/'">Home</BreadcrumbLink>
      </BreadcrumbItem>
      <BreadcrumbSeparator />
      <BreadcrumbItem v-if="route.name">
        <BreadcrumbPage>{{ breadcrumbItems[route.name as string]?.label || route.name }}</BreadcrumbPage>
      </BreadcrumbItem>
    </BreadcrumbList>
  </UIBreadcrumb>
</template>
```

---

### Task 5: Dashboard view — stats cards, recent activity, charts

**Files:**
- Create: `ui/src/views/DashboardView.vue`
- Create: `ui/src/components/dashboard/StatsCard.vue`
- Create: `ui/src/components/dashboard/RecentActivity.vue`
- Create: `ui/src/components/dashboard/ContactChart.vue`
- Create: `ui/src/api/types.ts`
- Create: `ui/src/stores/dashboard.ts`

**Interfaces:**
- Consumes: `GET /api/dashboard/stats`, `GET /api/activity?limit=10`
- Produces: Dashboard with 4 stat cards, activity timeline, contact trend chart

**Steps:**

- [ ] **Step 1: Define API types**

```typescript
// ui/src/api/types.ts
export interface DashboardStats {
  total_contacts: number
  total_forms: number
  total_submissions: number
  total_banners: number
}

export interface ActivityEntry {
  id: string
  action: string
  entity_type: string
  entity_id: string
  user_email: string
  created_at: string
}

export interface ContactTrend {
  date: string
  count: number
}
```

- [ ] **Step 2: Write dashboard store**

```typescript
// ui/src/stores/dashboard.ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/api/client'
import type { DashboardStats, ActivityEntry, ContactTrend } from '@/api/types'

export const useDashboardStore = defineStore('dashboard', () => {
  const stats = ref<DashboardStats | null>(null)
  const activity = ref<ActivityEntry[]>([])
  const trend = ref<ContactTrend[]>([])
  const loading = ref(false)

  async function fetchStats() {
    loading.value = true
    try {
      stats.value = await apiClient('/api/dashboard/stats')
    } finally {
      loading.value = false
    }
  }

  async function fetchActivity(limit = 10) {
    activity.value = await apiClient('/api/activity', {
      params: { limit },
    })
  }

  async function fetchTrend(days = 30) {
    trend.value = await apiClient('/api/dashboard/trend', {
      params: { days },
    })
  }

  return { stats, activity, trend, loading, fetchStats, fetchActivity, fetchTrend }
})
```

- [ ] **Step 3: Write DashboardView**

```vue
<!-- ui/src/views/DashboardView.vue -->
<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'
import { StatsCard } from '@/components/dashboard/StatsCard'
import { RecentActivity } from '@/components/dashboard/RecentActivity'
import { ContactChart } from '@/components/dashboard/ContactChart'

const dashboard = useDashboardStore()

let interval: number

onMounted(() => {
  dashboard.fetchStats()
  dashboard.fetchActivity()
  dashboard.fetchTrend()
  interval = setInterval(() => dashboard.fetchStats(), 30000)
})

onUnmounted(() => clearInterval(interval))
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Dashboard</h1>

    <div v-if="dashboard.loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      <div v-for="i in 4" :key="i" class="h-24 bg-gray-200 dark:bg-gray-700 rounded-lg animate-pulse" />
    </div>

    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      <StatsCard
        v-if="dashboard.stats"
        title="Total Contacts"
        :value="dashboard.stats.total_contacts"
        icon="users"
      />
      <StatsCard
        v-if="dashboard.stats"
        title="Total Forms"
        :value="dashboard.stats.total_forms"
        icon="file-text"
      />
      <StatsCard
        v-if="dashboard.stats"
        title="Submissions"
        :value="dashboard.stats.total_submissions"
        icon="check-circle"
      />
      <StatsCard
        v-if="dashboard.stats"
        title="Active Banners"
        :value="dashboard.stats.total_banners"
        icon="image"
      />
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <ContactChart :data="dashboard.trend" />
      <RecentActivity :activities="dashboard.activity" />
    </div>
  </div>
</template>
```

- [ ] **Step 4: Write StatsCard component**

```vue
<!-- ui/src/components/dashboard/StatsCard.vue -->
<script setup lang="ts">
import { Users, FileText, CheckCircle, Image } from 'lucide-vue-next'

interface Props {
  title: string
  value: number
  icon: string
}

const props = defineProps<Props>()

const iconMap: Record<string, any> = {
  users: Users,
  'file-text': FileText,
  'check-circle': CheckCircle,
  image: Image,
}
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
    <div class="flex items-center justify-between">
      <div>
        <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ props.title }}</p>
        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ props.value.toLocaleString() }}</p>
      </div>
      <component :is="iconMap[props.icon]" class="w-8 h-8 text-gray-400" />
    </div>
  </div>
</template>
```

- [ ] **Step 5: Write RecentActivity component**

```vue
<!-- ui/src/components/dashboard/RecentActivity.vue -->
<script setup lang="ts">
import type { ActivityEntry } from '@/api/types'
import { formatDistanceToNow } from 'date-fns'

interface Props {
  activities: ActivityEntry[]
}

const props = defineProps<Props>()
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
    <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">Recent Activity</h3>
    <div v-if="props.activities.length === 0" class="text-gray-500 text-sm">No recent activity</div>
    <ul v-else class="space-y-3">
      <li v-for="entry in props.activities" :key="entry.id" class="flex items-start gap-3 text-sm">
        <div class="w-2 h-2 mt-1.5 rounded-full bg-primary flex-shrink-0" />
        <div>
          <p class="text-gray-900 dark:text-white">
            <strong>{{ entry.user_email }}</strong> {{ entry.action }} {{ entry.entity_type }}
          </p>
          <p class="text-gray-500 text-xs">{{ formatDistanceToNow(new Date(entry.created_at)) }} ago</p>
        </div>
      </li>
    </ul>
  </div>
</template>
```

- [ ] **Step 6: Write ContactChart component**

```vue
<!-- ui/src/components/dashboard/ContactChart.vue -->
<script setup lang="ts">
import type { ContactTrend } from '@/api/types'
import { BarChart } from 'lucide-vue-next'

interface Props {
  data: ContactTrend[]
}

const props = defineProps<Props>()
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
    <div class="flex items-center justify-between mb-4">
      <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Contact Growth</h3>
      <BarChart class="w-5 h-5 text-gray-400" />
    </div>
    <div v-if="props.data.length === 0" class="h-48 flex items-center justify-center text-gray-500">
      No data yet
    </div>
    <div v-else class="h-48 flex items-end gap-1">
      <div
        v-for="(point, i) in props.data"
        :key="i"
        class="flex-1 bg-primary/60 rounded-t hover:bg-primary transition-colors"
        :style="{ height: `${(point.count / (props.data[i - 1]?.count || 1)) * 100}%`, minHeight: '4px' }"
        :title="`${point.date}: ${point.count}`"
      />
    </div>
  </div>
</template>
```

---

### Task 6: Contacts management — list, search, create, edit, delete

**Files:**
- Create: `ui/src/views/ContactsView.vue`
- Create: `ui/src/components/contacts/ContactTable.vue`
- Create: `ui/src/components/contacts/ContactForm.vue`
- Create: `ui/src/components/contacts/ContactFilters.vue`
- Create: `ui/src/api/types.ts` (extend)
- Create: `ui/src/stores/contacts.ts`

**Interfaces:**
- Consumes: `GET/POST /api/contacts`, `GET/PUT/DELETE /api/contacts/:id`, `GET /api/contacts/search?q=`
- Produces: Contacts CRUD with pagination, search, filter by tag/source

**Steps:**

- [ ] **Step 1: Extend API types**

```typescript
// ui/src/api/types.ts (add)
export interface Contact {
  id: string
  first_name: string
  last_name: string
  email: string
  phone?: string
  company?: string
  tags: string[]
  source: string
  custom_fields: Record<string, string>
  created_at: string
  updated_at: string
}

export interface ContactListResponse {
  data: Contact[]
  total: number
  page: number
  limit: number
}

export interface ContactFilter {
  search?: string
  tag?: string
  source?: string
  page?: number
  limit?: number
}
```

- [ ] **Step 2: Write contacts store**

```typescript
// ui/src/stores/contacts.ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/api/client'
import type { Contact, ContactListResponse, ContactFilter } from '@/api/types'

export const useContactsStore = defineStore('contacts', () => {
  const contacts = ref<Contact[]>([])
  const total = ref(0)
  const page = ref(1)
  const limit = ref(20)
  const loading = ref(false)

  async function fetchList(filters: ContactFilter = {}) {
    loading.value = true
    try {
      const res = await apiClient<ContactListResponse>('/api/contacts', {
        params: { ...filters, page: filters.page ?? page.value, limit: filters.limit ?? limit.value },
      })
      contacts.value = res.data
      total.value = res.total
      page.value = res.page
      limit.value = res.limit
    } finally {
      loading.value = false
    }
  }

  async function create(contact: Omit<Contact, 'id' | 'created_at' | 'updated_at'>) {
    return apiClient('/api/contacts', { method: 'POST', body: contact })
  }

  async function update(id: string, contact: Partial<Contact>) {
    return apiClient(`/api/contacts/${id}`, { method: 'PUT', body: contact })
  }

  async function remove(id: string) {
    return apiClient(`/api/contacts/${id}`, { method: 'DELETE' })
  }

  async function search(query: string) {
    return apiClient('/api/contacts/search', { params: { q: query } })
  }

  return { contacts, total, page, limit, loading, fetchList, create, update, remove, search }
})
```

- [ ] **Step 3: Write ContactsView**

```vue
<!-- ui/src/views/ContactsView.vue -->
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useContactsStore } from '@/stores/contacts'
import { ContactTable } from '@/components/contacts/ContactTable'
import { ContactForm } from '@/components/contacts/ContactForm'
import { ContactFilters } from '@/components/contacts/ContactFilters'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

const contacts = useContactsStore()
const showForm = ref(false)
const editingContact = ref<string | null>(null)
const filters = ref({ search: '', tag: '', source: '', page: 1 })

onMounted(() => contacts.fetchList(filters.value))

async function handleEdit(id: string) {
  editingContact.value = id
  showForm.value = true
}

async function handleDelete(id: string) {
  if (confirm('Delete this contact?')) {
    await contacts.remove(id)
    contacts.fetchList(filters.value)
  }
}

async function handleSave(contact: any) {
  if (editingContact.value) {
    await contacts.update(editingContact.value, contact)
  } else {
    await contacts.create(contact)
  }
  showForm.value = false
  editingContact.value = null
  contacts.fetchList(filters.value)
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Contacts</h1>
      <Button @click="showForm = true; editingContact = null">Add Contact</Button>
    </div>

    <ContactFilters v-model="filters" @search="contacts.fetchList(filters)" />

    <ContactTable
      :contacts="contacts.contacts"
      :total="contacts.total"
      :page="contacts.page"
      :loading="contacts.loading"
      @edit="handleEdit"
      @delete="handleDelete"
      @page-change="(p: number) => { filters.page = p; contacts.fetchList(filters) }"
    />

    <Dialog v-model:open="showForm">
      <DialogContent class="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{{ editingContact ? 'Edit Contact' : 'New Contact' }}</DialogTitle>
        </DialogHeader>
        <ContactForm
          :contact="editingContact ? contacts.contacts.find(c => c.id === editingContact) : null"
          @submit="handleSave"
          @cancel="showForm = false"
        />
      </DialogContent>
    </Dialog>
  </div>
</template>
```

- [ ] **Step 4: Write ContactTable**

```vue
<!-- ui/src/components/contacts/ContactTable.vue -->
<script setup lang="ts">
import type { Contact } from '@/api/types'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Pencil, Trash2 } from 'lucide-vue-next'

interface Props {
  contacts: Contact[]
  total: number
  page: number
  loading: boolean
}

defineProps<Props>()
defineEmits<{
  edit: [id: string]
  delete: [id: string]
  'page-change': [page: number]
}>()
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>Email</TableHead>
          <TableHead>Company</TableHead>
          <TableHead>Tags</TableHead>
          <TableHead>Source</TableHead>
          <TableHead class="w-24">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-if="loading">
          <TableCell :col-span="6" class="text-center py-8">Loading...</TableCell>
        </TableRow>
        <TableRow v-for="contact in contacts" :key="contact.id">
          <TableCell>{{ contact.first_name }} {{ contact.last_name }}</TableCell>
          <TableCell>{{ contact.email }}</TableCell>
          <TableCell>{{ contact.company || '-' }}</TableCell>
          <TableCell>
            <span v-for="tag in contact.tags" :key="tag" class="inline-block px-2 py-0.5 text-xs bg-gray-100 dark:bg-gray-700 rounded mr-1">
              {{ tag }}
            </span>
          </TableCell>
          <TableCell>{{ contact.source || '-' }}</TableCell>
          <TableCell>
            <div class="flex gap-2">
              <Button variant="ghost" size="sm" @click="$emit('edit', contact.id)">
                <Pencil class="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="sm" @click="$emit('delete', contact.id)">
                <Trash2 class="w-4 h-4" />
              </Button>
            </div>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <div class="flex items-center justify-between p-4 border-t border-gray-200 dark:border-gray-700">
      <span class="text-sm text-gray-500">Showing {{ contacts.length }} of {{ total }}</span>
      <div class="flex gap-2">
        <Button variant="outline" size="sm" :disabled="page <= 1" @click="$emit('page-change', page - 1)">Previous</Button>
        <Button variant="outline" size="sm" :disabled="page * 20 >= total" @click="$emit('page-change', page + 1)">Next</Button>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 5: Write ContactForm**

```vue
<!-- ui/src/components/contacts/ContactForm.vue -->
<script setup lang="ts">
import { ref, watch } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Contact } from '@/api/types'

interface Props {
  contact: Contact | null
}

const props = defineProps<Props>()

const form = ref({
  first_name: '',
  last_name: '',
  email: '',
  phone: '',
  company: '',
  tags: [] as string[],
  source: '',
  custom_fields: {},
})

watch(() => props.contact, (c) => {
  if (c) form.value = { ...c, tags: [...c.tags] }
}, { immediate: true })

const emit = defineEmits<{
  submit: [data: any]
  cancel: []
}>()
</script>

<template>
  <form @submit.prevent="emit('submit', form)" class="space-y-4">
    <div class="grid grid-cols-2 gap-4">
      <div class="space-y-2">
        <Label for="first_name">First Name</Label>
        <Input id="first_name" v-model="form.first_name" required />
      </div>
      <div class="space-y-2">
        <Label for="last_name">Last Name</Label>
        <Input id="last_name" v-model="form.last_name" required />
      </div>
    </div>
    <div class="space-y-2">
      <Label for="email">Email</Label>
      <Input id="email" type="email" v-model="form.email" required />
    </div>
    <div class="grid grid-cols-2 gap-4">
      <div class="space-y-2">
        <Label for="phone">Phone</Label>
        <Input id="phone" v-model="form.phone" />
      </div>
      <div class="space-y-2">
        <Label for="company">Company</Label>
        <Input id="company" v-model="form.company" />
      </div>
    </div>
    <div class="space-y-2">
      <Label for="source">Source</Label>
      <Input id="source" v-model="form.source" placeholder="website, referral, etc." />
    </div>
    <div class="flex justify-end gap-2">
      <Button type="button" variant="outline" @click="$emit('cancel')">Cancel</Button>
      <Button type="submit">{{ contact ? 'Update' : 'Create' }}</Button>
    </div>
  </form>
</template>
```

- [ ] **Step 6: Write ContactFilters**

```vue
<!-- ui/src/components/contacts/ContactFilters.vue -->
<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Search } from 'lucide-vue-next'

interface Props {
  search: string
  tag: string
  source: string
}

const model = defineModel<Props>()
const emit = defineEmits<{ search: [] }>()
</script>

<template>
  <div class="flex gap-3">
    <div class="relative flex-1">
      <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
      <Input
        v-model="model?.search"
        placeholder="Search contacts..."
        class="pl-9"
        @keyup.enter="emit('search')"
      />
    </div>
    <Input v-model="model?.tag" placeholder="Filter by tag" class="w-32" />
    <Input v-model="model?.source" placeholder="Filter by source" class="w-32" />
    <Button @click="emit('search')">Search</Button>
  </div>
</template>
```

---

### Task 7: Forms management — list, create, edit, preview, submissions

**Files:**
- Create: `ui/src/views/FormsView.vue`
- Create: `ui/src/components/forms/FormBuilder.vue`
- Create: `ui/src/components/forms/FormFieldPicker.vue`
- Create: `ui/src/components/forms/FormPreview.vue`
- Create: `ui/src/stores/forms.ts`
- Create: `ui/src/api/types.ts` (extend)

**Interfaces:**
- Consumes: `GET/POST /api/forms`, `GET/PUT/DELETE /api/forms/:id`, `GET /api/forms/:id/submissions`
- Produces: Form CRUD with drag-and-drop field builder, live preview, submission viewer

**Steps:**

- [ ] **Step 1: Extend API types**

```typescript
// ui/src/api/types.ts (add)
export type FieldType = 'text' | 'email' | 'textarea' | 'select' | 'checkbox' | 'radio' | 'file'

export interface FormField {
  id: string
  type: FieldType
  label: string
  required: boolean
  placeholder?: string
  options?: string[] // for select/radio
}

export interface Form {
  id: string
  name: string
  description: string
  fields: FormField[]
  is_active: boolean
  embed_code?: string
  created_at: string
  updated_at: string
}

export interface FormSubmission {
  id: string
  form_id: string
  data: Record<string, string>
  created_at: string
}

export interface FormListResponse {
  data: Form[]
  total: number
  page: number
  limit: number
}
```

- [ ] **Step 2: Write forms store**

```typescript
// ui/src/stores/forms.ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/api/client'
import type { Form, FormSubmission, FormListResponse } from '@/api/types'

export const useFormsStore = defineStore('forms', () => {
  const forms = ref<Form[]>([])
  const total = ref(0)
  const loading = ref(false)

  async function fetchList(page = 1, limit = 20) {
    loading.value = true
    try {
      const res = await apiClient<FormListResponse>('/api/forms', {
        params: { page, limit },
      })
      forms.value = res.data
      total.value = res.total
    } finally {
      loading.value = false
    }
  }

  async function create(form: Omit<Form, 'id' | 'created_at' | 'updated_at'>) {
    return apiClient('/api/forms', { method: 'POST', body: form })
  }

  async function update(id: string, form: Partial<Form>) {
    return apiClient(`/api/forms/${id}`, { method: 'PUT', body: form })
  }

  async function remove(id: string) {
    return apiClient(`/api/forms/${id}`, { method: 'DELETE' })
  }

  async function getSubmissions(formId: string) {
    return apiClient(`/api/forms/${formId}/submissions`)
  }

  return { forms, total, loading, fetchList, create, update, remove, getSubmissions }
})
```

- [ ] **Step 3: Write FormsView**

```vue
<!-- ui/src/views/FormsView.vue -->
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useFormsStore } from '@/stores/forms'
import { FormBuilder } from '@/components/forms/FormBuilder'
import { FormPreview } from '@/components/forms/FormPreview'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

const forms = useFormsStore()
const showBuilder = ref(false)
const editingForm = ref<string | null>(null)
const previewForm = ref<string | null>(null)

onMounted(() => forms.fetchList())

async function handleDelete(id: string) {
  if (confirm('Delete this form?')) {
    await forms.remove(id)
    forms.fetchList()
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Forms</h1>
      <Button @click="showBuilder = true; editingForm = null">Create Form</Button>
    </div>

    <div v-if="forms.loading" class="text-center py-8 text-gray-500">Loading...</div>

    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <div
        v-for="form in forms.forms"
        :key="form.id"
        class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6"
      >
        <div class="flex items-start justify-between">
          <div>
            <h3 class="font-semibold text-gray-900 dark:text-white">{{ form.name }}</h3>
            <p class="text-sm text-gray-500 mt-1">{{ form.description }}</p>
            <p class="text-xs text-gray-400 mt-2">{{ form.fields.length }} fields</p>
          </div>
          <div class="flex gap-2">
            <Button variant="ghost" size="sm" @click="previewForm = form.id">
              Preview
            </Button>
            <Button variant="ghost" size="sm" @click="showBuilder = true; editingForm = form.id">
              Edit
            </Button>
            <Button variant="ghost" size="sm" @click="handleDelete(form.id)">
              Delete
            </Button>
          </div>
        </div>
        <div class="mt-4 flex items-center gap-2">
          <span
            class="inline-flex items-center px-2 py-0.5 rounded text-xs"
            :class="form.is_active ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'"
          >
            {{ form.is_active ? 'Active' : 'Inactive' }}
          </span>
          <Button
            v-if="form.embed_code"
            variant="ghost"
            size="sm"
            class="text-xs"
            @click="navigator.clipboard.writeText(form.embed_code!)"
          >
            Copy Embed
          </Button>
        </div>
      </div>
    </div>

    <!-- FormBuilder Dialog -->
    <Dialog v-model:open="showBuilder">
      <DialogContent class="max-w-4xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{{ editingForm ? 'Edit Form' : 'Create Form' }}</DialogTitle>
        </DialogHeader>
        <FormBuilder :form-id="editingForm" @save="() => { showBuilder = false; forms.fetchList() }" @cancel="showBuilder = false" />
      </DialogContent>
    </Dialog>

    <!-- Preview Dialog -->
    <Dialog v-model:open="!!previewForm">
      <DialogContent class="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Form Preview</DialogTitle>
        </DialogHeader>
        <FormPreview v-if="previewForm" :form-id="previewForm" />
      </DialogContent>
    </Dialog>
  </div>
</template>
```

- [ ] **Step 4: Write FormBuilder**

```vue
<!-- ui/src/components/forms/FormBuilder.vue -->
<script setup lang="ts">
import { ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { FormField } from '@/components/forms/FormFieldPicker'
import { Plus, Trash2, GripVertical } from 'lucide-vue-next'

interface Props {
  formId: string | null
}

const props = defineProps<Props>()

const name = ref('')
const description = ref('')
const isActive = ref(true)
const fields = ref<FormField[]>([])

const emit = defineEmits<{
  save: []
  cancel: []
}>()

const fieldTypes = [
  { type: 'text', label: 'Text' },
  { type: 'email', label: 'Email' },
  { type: 'textarea', label: 'Textarea' },
  { type: 'select', label: 'Select' },
  { type: 'checkbox', label: 'Checkbox' },
  { type: 'radio', label: 'Radio' },
] as const

function addField(type: string) {
  fields.value.push({
    id: crypto.randomUUID(),
    type: type as any,
    label: `New ${type}`,
    required: false,
    placeholder: '',
    options: type === 'select' || type === 'radio' ? ['Option 1', 'Option 2'] : undefined,
  })
}

function removeField(id: string) {
  fields.value = fields.value.filter(f => f.id !== id)
}

function moveField(index: number, direction: 'up' | 'down') {
  const newIdx = direction === 'up' ? index - 1 : index + 1
  if (newIdx < 0 || newIdx >= fields.value.length) return
  const temp = fields.value[index]
  fields.value[index] = fields.value[newIdx]
  fields.value[newIdx] = temp
}
</script>

<template>
  <div class="space-y-6">
    <div class="grid grid-cols-2 gap-4">
      <div class="space-y-2">
        <Label for="form-name">Form Name</Label>
        <Input id="form-name" v-model="name" placeholder="Contact Us" />
      </div>
      <div class="space-y-2">
        <Label for="form-desc">Description</Label>
        <Input id="form-desc" v-model="description" placeholder="Brief description" />
      </div>
    </div>

    <div class="flex items-center justify-between">
      <h3 class="font-semibold">Fields</h3>
      <div class="flex gap-2">
        <Button v-for="ft in fieldTypes" :key="ft.type" variant="outline" size="sm" @click="addField(ft.type)">
          <Plus class="w-3 h-3 mr-1" /> {{ ft.label }}
        </Button>
      </div>
    </div>

    <div class="space-y-3">
      <div
        v-for="(field, index) in fields"
        :key="field.id"
        class="flex items-start gap-3 p-3 border border-gray-200 dark:border-gray-700 rounded-lg"
      >
        <GripVertical class="w-4 h-4 mt-4 text-gray-400 cursor-grab" />
        <div class="flex-1 space-y-2">
          <div class="flex items-center gap-2">
            <Label class="text-sm font-medium">Label</Label>
            <Input v-model="field.label" class="flex-1" />
          </div>
          <div class="flex items-center gap-4 text-sm">
            <label class="flex items-center gap-2">
              <Switch :checked="field.required" @update:checked="field.required = !field.required" />
              Required
            </label>
            <span class="text-gray-400">Type: {{ field.type }}</span>
          </div>
          <div v-if="field.options" class="flex gap-2">
            <Input
              v-for="(opt, oi) in field.options"
              :key="oi"
              v-model="field.options![oi]"
              class="w-32"
              placeholder="Option"
            />
            <Button variant="ghost" size="sm" @click="field.options?.push('New option')">Add</Button>
          </div>
        </div>
        <div class="flex flex-col gap-1">
          <Button variant="ghost" size="sm" @click="moveField(index, 'up')" :disabled="index === 0">Up</Button>
          <Button variant="ghost" size="sm" @click="moveField(index, 'down')" :disabled="index === fields.length - 1">Down</Button>
          <Button variant="ghost" size="sm" class="text-red-500" @click="removeField(field.id)">
            <Trash2 class="w-4 h-4" />
          </Button>
        </div>
      </div>
    </div>

    <div class="flex justify-end gap-2">
      <Button variant="outline" @click="$emit('cancel')">Cancel</Button>
      <Button @click="$emit('save')">Save Form</Button>
    </div>
  </div>
</template>
```

- [ ] **Step 5: Write FormPreview**

```vue
<!-- ui/src/components/forms/FormPreview.vue -->
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiClient } from '@/api/client'
import type { Form } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'

interface Props {
  formId: string
}

const props = defineProps<Props>()
const form = ref<Form | null>(null)
const submitting = ref(false)

onMounted(async () => {
  form.value = await apiClient(`/api/forms/${props.formId}`)
})
</script>

<template>
  <div v-if="form" class="space-y-4">
    <h3 class="font-semibold">{{ form.name }}</h3>
    <p class="text-sm text-gray-500">{{ form.description }}</p>

    <form @submit.prevent="submitting = true" class="space-y-4">
      <div v-for="field in form.fields" :key="field.id" class="space-y-2">
        <Label v-if="field.type !== 'checkbox'">{{ field.label }}{{ field.required ? ' *' : '' }}</Label>

        <Input
          v-if="field.type === 'text' || field.type === 'email'"
          :type="field.type"
          :placeholder="field.placeholder"
          :required="field.required"
        />

        <Textarea
          v-else-if="field.type === 'textarea'"
          :placeholder="field.placeholder"
          :required="field.required"
        />

        <select
          v-else-if="field.type === 'select'"
          :required="field.required"
          class="w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
        >
          <option value="">Select...</option>
          <option v-for="opt in (field.options || [])" :key="opt" :value="opt">{{ opt }}</option>
        </select>

        <div v-else-if="field.type === 'radio'" class="flex gap-4">
          <label v-for="opt in (field.options || [])" :key="opt" class="flex items-center gap-2">
            <input type="radio" :name="field.id" :value="opt" :required="field.required" /> {{ opt }}
          </label>
        </div>

        <label v-else-if="field.type === 'checkbox'" class="flex items-center gap-2">
          <Checkbox :required="field.required" /> {{ field.label }}
        </label>
      </div>

      <Button type="submit" :disabled="submitting">Submit</Button>
    </form>
  </div>
</template>
```

---

### Task 8: Banners management — list, create, edit, toggle active

**Files:**
- Create: `ui/src/views/BannersView.vue`
- Create: `ui/src/components/banners/BannerTable.vue`
- Create: `ui/src/components/banners/BannerForm.vue`
- Create: `ui/src/stores/banners.ts`
- Create: `ui/src/api/types.ts` (extend)

**Interfaces:**
- Consumes: `GET/POST /api/banners`, `GET/PUT/DELETE /api/banners/:id`
- Produces: Banners CRUD with image upload placeholder, active/inactive toggle

**Steps:**

- [ ] **Step 1: Extend API types**

```typescript
// ui/src/api/types.ts (add)
export interface Banner {
  id: string
  title: string
  description: string
  image_url: string
  link_url: string
  is_active: boolean
  priority: number
  created_at: string
  updated_at: string
}

export interface BannerListResponse {
  data: Banner[]
  total: number
  page: number
  limit: number
}
```

- [ ] **Step 2: Write banners store**

```typescript
// ui/src/stores/banners.ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/api/client'
import type { Banner, BannerListResponse } from '@/api/types'

export const useBannersStore = defineStore('banners', () => {
  const banners = ref<Banner[]>([])
  const total = ref(0)
  const loading = ref(false)

  async function fetchList(page = 1, limit = 20) {
    loading.value = true
    try {
      const res = await apiClient<BannerListResponse>('/api/banners', {
        params: { page, limit },
      })
      banners.value = res.data
      total.value = res.total
    } finally {
      loading.value = false
    }
  }

  async function create(banner: Omit<Banner, 'id' | 'created_at' | 'updated_at'>) {
    return apiClient('/api/banners', { method: 'POST', body: banner })
  }

  async function update(id: string, banner: Partial<Banner>) {
    return apiClient(`/api/banners/${id}`, { method: 'PUT', body: banner })
  }

  async function remove(id: string) {
    return apiClient(`/api/banners/${id}`, { method: 'DELETE' })
  }

  return { banners, total, loading, fetchList, create, update, remove }
})
```

- [ ] **Step 3: Write BannersView**

```vue
<!-- ui/src/views/BannersView.vue -->
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useBannersStore } from '@/stores/banners'
import { BannerTable } from '@/components/banners/BannerTable'
import { BannerForm } from '@/components/banners/BannerForm'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

const banners = useBannersStore()
const showForm = ref(false)
const editingBanner = ref<string | null>(null)

onMounted(() => banners.fetchList())

async function handleDelete(id: string) {
  if (confirm('Delete this banner?')) {
    await banners.remove(id)
    banners.fetchList()
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Banners</h1>
      <Button @click="showForm = true; editingBanner = null">Create Banner</Button>
    </div>

    <BannerTable
      :banners="banners.banners"
      :total="banners.total"
      :loading="banners.loading"
      @edit="(id: string) => { editingBanner = id; showForm = true }"
      @delete="handleDelete"
      @toggle="(id: string) => { banners.fetchList() }"
    />

    <Dialog v-model:open="showForm">
      <DialogContent class="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{{ editingBanner ? 'Edit Banner' : 'New Banner' }}</DialogTitle>
        </DialogHeader>
        <BannerForm
          :banner-id="editingBanner"
          @save="() => { showForm = false; banners.fetchList() }"
          @cancel="showForm = false"
        />
      </DialogContent>
    </Dialog>
  </div>
</template>
```

- [ ] **Step 4: Write BannerTable**

```vue
<!-- ui/src/components/banners/BannerTable.vue -->
<script setup lang="ts">
import type { Banner } from '@/api/types'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Pencil, Trash2 } from 'lucide-vue-next'

interface Props {
  banners: Banner[]
  total: number
  loading: boolean
}

defineProps<Props>()
defineEmits<{
  edit: [id: string]
  delete: [id: string]
  toggle: [id: string]
}>()
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Title</TableHead>
          <TableHead>Image</TableHead>
          <TableHead>Link</TableHead>
          <TableHead>Priority</TableHead>
          <TableHead>Status</TableHead>
          <TableHead class="w-24">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-if="loading">
          <TableCell :col-span="6" class="text-center py-8">Loading...</TableCell>
        </TableRow>
        <TableRow v-for="banner in banners" :key="banner.id">
          <TableCell>{{ banner.title }}</TableCell>
          <TableCell>
            <img v-if="banner.image_url" :src="banner.image_url" class="w-12 h-8 object-cover rounded" alt="" />
            <span v-else class="text-gray-400 text-sm">No image</span>
          </TableCell>
          <TableCell class="truncate max-w-[200px]">{{ banner.link_url }}</TableCell>
          <TableCell>{{ banner.priority }}</TableCell>
          <TableCell>
            <Switch
              :checked="banner.is_active"
              @update:checked="$emit('toggle', banner.id)"
            />
          </TableCell>
          <TableCell>
            <div class="flex gap-2">
              <Button variant="ghost" size="sm" @click="$emit('edit', banner.id)">
                <Pencil class="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="sm" @click="$emit('delete', banner.id)">
                <Trash2 class="w-4 h-4" />
              </Button>
            </div>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </div>
</template>
```

- [ ] **Step 5: Write BannerForm**

```vue
<!-- ui/src/components/banners/BannerForm.vue -->
<script setup lang="ts">
import { ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface Props {
  bannerId: string | null
}

const props = defineProps<Props>()

const title = ref('')
const description = ref('')
const imageUrl = ref('')
const linkUrl = ref('')
const priority = ref(0)

const emit = defineEmits<{
  save: []
  cancel: []
}>()
</script>

<template>
  <form @submit.prevent="emit('save')" class="space-y-4">
    <div class="space-y-2">
      <Label for="title">Title</Label>
      <Input id="title" v-model="title" required />
    </div>
    <div class="space-y-2">
      <Label for="description">Description</Label>
      <Input id="description" v-model="description" />
    </div>
    <div class="space-y-2">
      <Label for="image_url">Image URL</Label>
      <Input id="image_url" v-model="imageUrl" placeholder="/images/banner.jpg" />
    </div>
    <div class="space-y-2">
      <Label for="link_url">Link URL</Label>
      <Input id="link_url" v-model="linkUrl" placeholder="https://example.com" />
    </div>
    <div class="space-y-2">
      <Label for="priority">Priority</Label>
      <Input id="priority" type="number" v-model.number="priority" />
    </div>
    <div class="flex justify-end gap-2">
      <Button type="button" variant="outline" @click="$emit('cancel')">Cancel</Button>
      <Button type="submit">{{ bannerId ? 'Update' : 'Create' }}</Button>
    </div>
  </form>
</template>
```

---

### Task 9: Email templates management — list, create, edit, preview, test send

**Files:**
- Create: `ui/src/views/EmailTemplatesView.vue`
- Create: `ui/src/components/email/TemplateList.vue`
- Create: `ui/src/components/email/TemplateEditor.vue`
- Create: `ui/src/components/email/TemplatePreview.vue`
- Create: `ui/src/stores/emailTemplates.ts`
- Create: `ui/src/api/types.ts` (extend)

**Interfaces:**
- Consumes: `GET/POST /admin/email-templates`, `GET/PUT/DELETE /admin/email-templates/:id`, `POST /admin/email-templates/:id/test`
- Produces: Email template CRUD with HTML editor, variable preview, test send

**Steps:**

- [ ] **Step 1: Extend API types**

```typescript
// ui/src/api/types.ts (add)
export interface EmailTemplate {
  id: string
  name: string
  subject: string
  body_html: string
  body_text?: string
  variables: string[]
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface EmailTemplateListResponse {
  data: EmailTemplate[]
  total: number
  page: number
  limit: number
}

export interface TestSendRequest {
  template_id: string
  to: string
  variables: Record<string, string>
}
```

- [ ] **Step 2: Write emailTemplates store**

```typescript
// ui/src/stores/emailTemplates.ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/api/client'
import type { EmailTemplate, EmailTemplateListResponse, TestSendRequest } from '@/api/types'

export const useEmailTemplatesStore = defineStore('emailTemplates', () => {
  const templates = ref<EmailTemplate[]>([])
  const total = ref(0)
  const loading = ref(false)

  async function fetchList(page = 1, limit = 20) {
    loading.value = true
    try {
      const res = await apiClient<EmailTemplateListResponse>('/admin/email-templates', {
        params: { page, limit },
      })
      templates.value = res.data
      total.value = res.total
    } finally {
      loading.value = false
    }
  }

  async function create(template: Omit<EmailTemplate, 'id' | 'created_at' | 'updated_at'>) {
    return apiClient('/admin/email-templates', { method: 'POST', body: template })
  }

  async function update(id: string, template: Partial<EmailTemplate>) {
    return apiClient(`/admin/email-templates/${id}`, { method: 'PUT', body: template })
  }

  async function remove(id: string) {
    return apiClient(`/admin/email-templates/${id}`, { method: 'DELETE' })
  }

  async function testSend(req: TestSendRequest) {
    return apiClient('/admin/email-templates/test', { method: 'POST', body: req })
  }

  return { templates, total, loading, fetchList, create, update, remove, testSend }
})
```

- [ ] **Step 3: Write EmailTemplatesView**

```vue
<!-- ui/src/views/EmailTemplatesView.vue -->
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useEmailTemplatesStore } from '@/stores/emailTemplates'
import { TemplateList } from '@/components/email/TemplateList'
import { TemplateEditor } from '@/components/email/TemplateEditor'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

const templates = useEmailTemplatesStore()
const showEditor = ref(false)
const editingTemplate = ref<string | null>(null)
const previewTemplate = ref<string | null>(null)

onMounted(() => templates.fetchList())

async function handleDelete(id: string) {
  if (confirm('Delete this template?')) {
    await templates.remove(id)
    templates.fetchList()
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Email Templates</h1>
      <Button @click="showEditor = true; editingTemplate = null">Create Template</Button>
    </div>

    <TemplateList
      :templates="templates.templates"
      :total="templates.total"
      :loading="templates.loading"
      @edit="(id: string) => { editingTemplate = id; showEditor = true }"
      @delete="handleDelete"
      @preview="(id: string) => { previewTemplate = id }"
      @test="(id: string) => { /* trigger test send */ }"
    />

    <Dialog v-model:open="showEditor">
      <DialogContent class="max-w-4xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{{ editingTemplate ? 'Edit Template' : 'New Template' }}</DialogTitle>
        </DialogHeader>
        <TemplateEditor
          :template-id="editingTemplate"
          @save="() => { showEditor = false; templates.fetchList() }"
          @cancel="showEditor = false"
        />
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="!!previewTemplate">
      <DialogContent class="max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Template Preview</DialogTitle>
        </DialogHeader>
        <TemplatePreview v-if="previewTemplate" :template-id="previewTemplate" />
      </DialogContent>
    </Dialog>
  </div>
</template>
```

- [ ] **Step 4: Write TemplateList**

```vue
<!-- ui/src/components/email/TemplateList.vue -->
<script setup lang="ts">
import type { EmailTemplate } from '@/api/types'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Eye, Mail, Pencil, Trash2 } from 'lucide-vue-next'

interface Props {
  templates: EmailTemplate[]
  total: number
  loading: boolean
}

defineProps<Props>()
defineEmits<{
  edit: [id: string]
  delete: [id: string]
  preview: [id: string]
  test: [id: string]
}>()
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>Subject</TableHead>
          <TableHead>Variables</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Updated</TableHead>
          <TableHead class="w-32">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-if="loading">
          <TableCell :col-span="6" class="text-center py-8">Loading...</TableCell>
        </TableRow>
        <TableRow v-for="t in templates" :key="t.id">
          <TableCell class="font-medium">{{ t.name }}</TableCell>
          <TableCell class="truncate max-w-[200px]">{{ t.subject }}</TableCell>
          <TableCell>
            <span v-for="v in t.variables" :key="v" class="inline-block px-2 py-0.5 text-xs bg-blue-100 dark:bg-blue-900 rounded mr-1">
              {{ v }}
            </span>
          </TableCell>
          <TableCell>
            <span class="inline-flex items-center px-2 py-0.5 rounded text-xs"
              :class="t.is_active ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'">
              {{ t.is_active ? 'Active' : 'Inactive' }}
            </span>
          </TableCell>
          <TableCell class="text-sm text-gray-500">{{ new Date(t.updated_at).toLocaleDateString() }}</TableCell>
          <TableCell>
            <div class="flex gap-1">
              <Button variant="ghost" size="sm" @click="$emit('preview', t.id)">
                <Eye class="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="sm" @click="$emit('test', t.id)">
                <Mail class="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="sm" @click="$emit('edit', t.id)">
                <Pencil class="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="sm" @click="$emit('delete', t.id)">
                <Trash2 class="w-4 h-4" />
              </Button>
            </div>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </div>
</template>
```

- [ ] **Step 5: Write TemplateEditor**

```vue
<!-- ui/src/components/email/TemplateEditor.vue -->
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiClient } from '@/api/client'
import type { EmailTemplate } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

interface Props {
  templateId: string | null
}

const props = defineProps<Props>()

const name = ref('')
const subject = ref('')
const bodyHtml = ref('')
const bodyText = ref('')
const variables = ref<string[]>([])

onMounted(async () => {
  if (props.templateId) {
    const t = await apiClient<EmailTemplate>(`/admin/email-templates/${props.templateId}`)
    name.value = t.name
    subject.value = t.subject
    bodyHtml.value = t.body_html
    bodyText.value = t.body_text || ''
    variables.value = [...t.variables]
  }
})

const emit = defineEmits<{
  save: []
  cancel: []
}>()
</script>

<template>
  <div class="space-y-4">
    <div class="grid grid-cols-2 gap-4">
      <div class="space-y-2">
        <Label for="name">Template Name</Label>
        <Input id="name" v-model="name" required />
      </div>
      <div class="space-y-2">
        <Label for="subject">Email Subject</Label>
        <Input id="subject" v-model="subject" required />
      </div>
    </div>

    <div class="space-y-2">
      <Label>Variables</Label>
      <p class="text-sm text-gray-500">Use {{ variable_name }} syntax in your HTML</p>
      <div class="flex gap-2 flex-wrap">
        <span v-for="v in variables" :key="v" class="px-2 py-1 text-sm bg-blue-100 dark:bg-blue-900 rounded">
          {{ v }}
        </span>
      </div>
    </div>

    <Tabs default-value="html">
      <TabsList>
        <TabsTrigger value="html">HTML</TabsTrigger>
        <TabsTrigger value="text">Plain Text</TabsTrigger>
        <TabsTrigger value="preview">Preview</TabsTrigger>
      </TabsList>
      <TabsContent value="html">
        <textarea
          v-model="bodyHtml"
          class="w-full h-96 p-4 font-mono text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900"
          placeholder="<h1>Hello {{ name }},</h1>..."
        />
      </TabsContent>
      <TabsContent value="text">
        <textarea
          v-model="bodyText"
          class="w-full h-96 p-4 font-mono text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900"
        />
      </TabsContent>
      <TabsContent value="preview">
        <div class="border border-gray-300 dark:border-gray-600 rounded-lg p-4 min-h-96" v-html="bodyHtml" />
      </TabsContent>
    </Tabs>

    <div class="flex justify-end gap-2">
      <Button variant="outline" @click="$emit('cancel')">Cancel</Button>
      <Button @click="$emit('save')">Save Template</Button>
    </div>
  </div>
</template>
```

- [ ] **Step 6: Write TemplatePreview**

```vue
<!-- ui/src/components/email/TemplatePreview.vue -->
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiClient } from '@/api/client'
import type { EmailTemplate } from '@/api/types'

interface Props {
  templateId: string
}

const props = defineProps<Props>()
const template = ref<EmailTemplate | null>(null)

onMounted(async () => {
  template.value = await apiClient<EmailTemplate>(`/admin/email-templates/${props.templateId}`)
})
</script>

<template>
  <div v-if="template">
    <h3 class="font-semibold mb-2">Subject: {{ template.subject }}</h3>
    <div class="border border-gray-300 dark:border-gray-600 rounded-lg p-4 min-h-96" v-html="template.body_html" />
  </div>
</template>
```

---

### Task 10: Tenants management — list, create, edit, manage members

**Files:**
- Create: `ui/src/views/TenantsView.vue`
- Create: `ui/src/components/tenants/TenantTable.vue`
- Create: `ui/src/components/tenants/TenantForm.vue`
- Create: `ui/src/components/tenants/TenantMembers.vue`
- Create: `ui/src/stores/tenants.ts`
- Create: `ui/src/api/types.ts` (extend)

**Interfaces:**
- Consumes: `GET/POST /admin/tenants`, `GET/PUT/DELETE /admin/tenants/:id`, `GET/POST /admin/tenants/:id/members`
- Produces: Admin-only tenant CRUD with member management

**Steps:**

- [ ] **Step 1: Extend API types**

```typescript
// ui/src/api/types.ts (add)
export interface Tenant {
  id: string
  name: string
  slug: string
  is_active: boolean
  plan: string
  created_at: string
  updated_at: string
}

export interface TenantMember {
  id: string
  user_id: string
  email: string
  role: string // 'owner', 'admin', 'member'
  created_at: string
}

export interface TenantListResponse {
  data: Tenant[]
  total: number
  page: number
  limit: number
}
```

- [ ] **Step 2: Write tenants store**

```typescript
// ui/src/stores/tenants.ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/api/client'
import type { Tenant, TenantMember, TenantListResponse } from '@/api/types'

export const useTenantsStore = defineStore('tenants', () => {
  const tenants = ref<Tenant[]>([])
  const total = ref(0)
  const loading = ref(false)

  async function fetchList(page = 1, limit = 20) {
    loading.value = true
    try {
      const res = await apiClient<TenantListResponse>('/admin/tenants', {
        params: { page, limit },
      })
      tenants.value = res.data
      total.value = res.total
    } finally {
      loading.value = false
    }
  }

  async function create(tenant: Omit<Tenant, 'id' | 'created_at' | 'updated_at'>) {
    return apiClient('/admin/tenants', { method: 'POST', body: tenant })
  }

  async function update(id: string, tenant: Partial<Tenant>) {
    return apiClient(`/admin/tenants/${id}`, { method: 'PUT', body: tenant })
  }

  async function remove(id: string) {
    return apiClient(`/admin/tenants/${id}`, { method: 'DELETE' })
  }

  async function getMembers(tenantId: string) {
    return apiClient<TenantMember[]>(`/admin/tenants/${tenantId}/members`)
  }

  async function addMember(tenantId: string, member: { user_id: string; role: string }) {
    return apiClient(`/admin/tenants/${tenantId}/members`, { method: 'POST', body: member })
  }

  async function removeMember(tenantId: string, memberId: string) {
    return apiClient(`/admin/tenants/${tenantId}/members/${memberId}`, { method: 'DELETE' })
  }

  return { tenants, total, loading, fetchList, create, update, remove, getMembers, addMember, removeMember }
})
```

- [ ] **Step 3: Write TenantsView**

```vue
<!-- ui/src/views/TenantsView.vue -->
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useTenantsStore } from '@/stores/tenants'
import { TenantTable } from '@/components/tenants/TenantTable'
import { TenantForm } from '@/components/tenants/TenantForm'
import { TenantMembers } from '@/components/tenants/TenantMembers'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

const tenants = useTenantsStore()
const showForm = ref(false)
const editingTenant = ref<string | null>(null)
const selectedTenant = ref<string | null>(null)

onMounted(() => tenants.fetchList())

async function handleDelete(id: string) {
  if (confirm('Delete this tenant? This action cannot be undone.')) {
    await tenants.remove(id)
    tenants.fetchList()
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Tenants</h1>
      <Button @click="showForm = true; editingTenant = null">Create Tenant</Button>
    </div>

    <TenantTable
      :tenants="tenants.tenants"
      :total="tenants.total"
      :loading="tenants.loading"
      @edit="(id: string) => { editingTenant = id; showForm = true }"
      @delete="handleDelete"
      @view-members="(id: string) => { selectedTenant = id }"
    />

    <Dialog v-model:open="showForm">
      <DialogContent class="max-w-xl">
        <DialogHeader>
          <DialogTitle>{{ editingTenant ? 'Edit Tenant' : 'New Tenant' }}</DialogTitle>
        </DialogHeader>
        <TenantForm
          :tenant-id="editingTenant"
          @save="() => { showForm = false; tenants.fetchList() }"
          @cancel="showForm = false"
        />
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="!!selectedTenant">
      <DialogContent class="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Tenant Members</DialogTitle>
        </DialogHeader>
        <TenantMembers v-if="selectedTenant" :tenant-id="selectedTenant" />
      </DialogContent>
    </Dialog>
  </div>
</template>
```

- [ ] **Step 4: Write TenantTable**

```vue
<!-- ui/src/components/tenants/TenantTable.vue -->
<script setup lang="ts">
import type { Tenant } from '@/api/types'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Users, Pencil, Trash2 } from 'lucide-vue-next'

interface Props {
  tenants: Tenant[]
  total: number
  loading: boolean
}

defineProps<Props>()
defineEmits<{
  edit: [id: string]
  delete: [id: string]
  'view-members': [id: string]
}>()
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>Slug</TableHead>
          <TableHead>Plan</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Created</TableHead>
          <TableHead class="w-32">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-if="loading">
          <TableCell :col-span="6" class="text-center py-8">Loading...</TableCell>
        </TableRow>
        <TableRow v-for="tenant in tenants" :key="tenant.id">
          <TableCell class="font-medium">{{ tenant.name }}</TableCell>
          <TableCell class="text-gray-500">{{ tenant.slug }}</TableCell>
          <TableCell>{{ tenant.plan }}</TableCell>
          <TableCell>
            <span class="inline-flex items-center px-2 py-0.5 rounded text-xs"
              :class="tenant.is_active ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'">
              {{ tenant.is_active ? 'Active' : 'Inactive' }}
            </span>
          </TableCell>
          <TableCell class="text-sm text-gray-500">{{ new Date(tenant.created_at).toLocaleDateString() }}</TableCell>
          <TableCell>
            <div class="flex gap-1">
              <Button variant="ghost" size="sm" @click="$emit('view-members', tenant.id)">
                <Users class="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="sm" @click="$emit('edit', tenant.id)">
                <Pencil class="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="sm" @click="$emit('delete', tenant.id)">
                <Trash2 class="w-4 h-4" />
              </Button>
            </div>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </div>
</template>
```

- [ ] **Step 5: Write TenantForm**

```vue
<!-- ui/src/components/tenants/TenantForm.vue -->
<script setup lang="ts">
import { ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

interface Props {
  tenantId: string | null
}

const props = defineProps<Props>()

const name = ref('')
const slug = ref('')
const plan = ref('free')

const emit = defineEmits<{
  save: []
  cancel: []
}>()
</script>

<template>
  <form @submit.prevent="emit('save')" class="space-y-4">
    <div class="space-y-2">
      <Label for="name">Tenant Name</Label>
      <Input id="name" v-model="name" required />
    </div>
    <div class="space-y-2">
      <Label for="slug">Slug</Label>
      <Input id="slug" v-model="slug" required placeholder="mycompany" />
    </div>
    <div class="space-y-2">
      <Label for="plan">Plan</Label>
      <Select v-model="plan">
        <SelectTrigger>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="free">Free</SelectItem>
          <SelectItem value="pro">Pro</SelectItem>
          <SelectItem value="enterprise">Enterprise</SelectItem>
        </SelectContent>
      </Select>
    </div>
    <div class="flex justify-end gap-2">
      <Button type="button" variant="outline" @click="$emit('cancel')">Cancel</Button>
      <Button type="submit">{{ tenantId ? 'Update' : 'Create' }}</Button>
    </div>
  </form>
</template>
```

- [ ] **Step 6: Write TenantMembers**

```vue
<!-- ui/src/components/tenants/TenantMembers.vue -->
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useTenantsStore } from '@/stores/tenants'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Plus, Trash2 } from 'lucide-vue-next'

interface Props {
  tenantId: string
}

const props = defineProps<Props>()
const store = useTenantsStore()
const members = ref<any[]>([])
const newUserId = ref('')
const newRole = ref('member')

onMounted(async () => {
  members.value = await store.getMembers(props.tenantId)
})

async function addMember() {
  await store.addMember(props.tenantId, { user_id: newUserId.value, role: newRole.value })
  members.value = await store.getMembers(props.tenantId)
  newUserId.value = ''
}

async function removeMember(memberId: string) {
  await store.removeMember(props.tenantId, memberId)
  members.value = await store.getMembers(props.tenantId)
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex gap-2">
      <Input v-model="newUserId" placeholder="User ID" class="flex-1" />
      <Select v-model="newRole">
        <SelectTrigger class="w-32">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="member">Member</SelectItem>
          <SelectItem value="admin">Admin</SelectItem>
        </SelectContent>
      </Select>
      <Button @click="addMember"><Plus class="w-4 h-4 mr-1" /> Add</Button>
    </div>

    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Email</TableHead>
          <TableHead>Role</TableHead>
          <TableHead class="w-12"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="m in members" :key="m.id">
          <TableCell>{{ m.email }}</TableCell>
          <TableCell>{{ m.role }}</TableCell>
          <TableCell>
            <Button variant="ghost" size="sm" @click="removeMember(m.id)">
              <Trash2 class="w-4 h-4 text-red-500" />
            </Button>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </div>
</template>
```

---

### Task 11: Analytics view — submissions chart, conversion funnel, export

**Files:**
- Create: `ui/src/views/AnalyticsView.vue`
- Create: `ui/src/components/analytics/SubmissionsChart.vue`
- Create: `ui/src/components/analytics/ConversionFunnel.vue`
- Create: `ui/src/components/analytics/ExportButton.vue`
- Create: `ui/src/stores/analytics.ts`
- Create: `ui/src/api/types.ts` (extend)

**Interfaces:**
- Consumes: `GET /api/analytics/submissions?from=&to=`, `GET /api/analytics/funnel`, `GET /api/analytics/export?format=csv`
- Produces: Analytics dashboard with submission trends, form conversion rates, CSV export

**Steps:**

- [ ] **Step 1: Extend API types**

```typescript
// ui/src/api/types.ts (add)
export interface SubmissionTrend {
  date: string
  count: number
}

export interface FunnelStep {
  form_id: string
  form_name: string
  views: number
  submissions: number
  conversion_rate: number
}

export interface AnalyticsResponse {
  total_submissions: number
  total_views: number
  overall_conversion_rate: number
  trends: SubmissionTrend[]
  funnel: FunnelStep[]
}
```

- [ ] **Step 2: Write analytics store**

```typescript
// ui/src/stores/analytics.ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/api/client'
import type { AnalyticsResponse } from '@/api/types'

export const useAnalyticsStore = defineStore('analytics', () => {
  const data = ref<AnalyticsResponse | null>(null)
  const loading = ref(false)
  const from = ref('')
  const to = ref('')

  async function fetchAnalytics() {
    loading.value = true
    try {
      data.value = await apiClient('/api/analytics', {
        params: { from: from.value, to: to.value },
      })
    } finally {
      loading.value = false
    }
  }

  function exportCSV() {
    const url = `/api/analytics/export?format=csv&from=${from.value}&to=${to.value}`
    window.open(url, '_blank')
  }

  return { data, loading, from, to, fetchAnalytics, exportCSV }
})
```

- [ ] **Step 3: Write AnalyticsView**

```vue
<!-- ui/src/views/AnalyticsView.vue -->
<script setup lang="ts">
import { onMounted } from 'vue'
import { useAnalyticsStore } from '@/stores/analytics'
import { SubmissionsChart } from '@/components/analytics/SubmissionsChart'
import { ConversionFunnel } from '@/components/analytics/ConversionFunnel'
import { ExportButton } from '@/components/analytics/ExportButton'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const analytics = useAnalyticsStore()

onMounted(() => analytics.fetchAnalytics())
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Analytics</h1>
      <ExportButton />
    </div>

    <div class="flex gap-4 items-end">
      <div class="space-y-2">
        <Label for="from">From</Label>
        <Input id="from" type="date" v-model="analytics.from" @change="analytics.fetchAnalytics" />
      </div>
      <div class="space-y-2">
        <Label for="to">To</Label>
        <Input id="to" type="date" v-model="analytics.to" @change="analytics.fetchAnalytics" />
      </div>
    </div>

    <div v-if="analytics.loading" class="text-center py-8 text-gray-500">Loading...</div>

    <div v-else-if="analytics.data" class="space-y-6">
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div class="bg-white dark:bg-gray-800 rounded-lg border p-6">
          <p class="text-sm text-gray-500">Total Submissions</p>
          <p class="text-3xl font-bold">{{ analytics.data.total_submissions.toLocaleString() }}</p>
        </div>
        <div class="bg-white dark:bg-gray-800 rounded-lg border p-6">
          <p class="text-sm text-gray-500">Total Views</p>
          <p class="text-3xl font-bold">{{ analytics.data.total_views.toLocaleString() }}</p>
        </div>
        <div class="bg-white dark:bg-gray-800 rounded-lg border p-6">
          <p class="text-sm text-gray-500">Conversion Rate</p>
          <p class="text-3xl font-bold">{{ (analytics.data.overall_conversion_rate * 100).toFixed(1) }}%</p>
        </div>
      </div>

      <SubmissionsChart :data="analytics.data.trends" />
      <ConversionFunnel :funnel="analytics.data.funnel" />
    </div>
  </div>
</template>
```

- [ ] **Step 4: Write SubmissionsChart**

```vue
<!-- ui/src/components/analytics/SubmissionsChart.vue -->
<script setup lang="ts">
import type { SubmissionTrend } from '@/api/types'

interface Props {
  data: SubmissionTrend[]
}

const props = defineProps<Props>()
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
    <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">Submission Trends</h3>
    <div v-if="data.length === 0" class="h-48 flex items-center justify-center text-gray-500">No data</div>
    <div v-else class="h-48 flex items-end gap-1">
      <div
        v-for="(point, i) in data"
        :key="i"
        class="flex-1 bg-primary/60 rounded-t hover:bg-primary transition-colors relative group"
        :style="{ height: `${Math.max(4, (point.count / (data.reduce((m, p) => Math.max(m, p.count), 0))) * 100)}%` }"
      >
        <div class="absolute bottom-full left-1/2 -translate-x-1/2 mb-1 px-2 py-1 text-xs bg-gray-900 text-white rounded opacity-0 group-hover:opacity-100 whitespace-nowrap">
          {{ point.date }}: {{ point.count }}
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 5: Write ConversionFunnel**

```vue
<!-- ui/src/components/analytics/ConversionFunnel.vue -->
<script setup lang="ts">
import type { FunnelStep } from '@/api/types'

interface Props {
  funnel: FunnelStep[]
}

const props = defineProps<Props>()
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
    <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">Conversion by Form</h3>
    <div v-if="funnel.length === 0" class="text-gray-500">No data</div>
    <div v-else class="space-y-3">
      <div v-for="step in funnel" :key="step.form_id" class="space-y-1">
        <div class="flex justify-between text-sm">
          <span class="font-medium">{{ step.form_name }}</span>
          <span>{{ (step.conversion_rate * 100).toFixed(1) }}%</span>
        </div>
        <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-4 overflow-hidden">
          <div
            class="bg-primary h-full rounded-full transition-all"
            :style="{ width: `${step.conversion_rate * 100}%` }"
          />
        </div>
        <div class="flex justify-between text-xs text-gray-500">
          <span>{{ step.views }} views</span>
          <span>{{ step.submissions }} submissions</span>
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 6: Write ExportButton**

```vue
<!-- ui/src/components/analytics/ExportButton.vue -->
<script setup lang="ts">
import { useAnalyticsStore } from '@/stores/analytics'
import { Button } from '@/components/ui/button'
import { Download } from 'lucide-vue-next'

const analytics = useAnalyticsStore()
</script>

<template>
  <Button variant="outline" @click="analytics.exportCSV">
    <Download class="w-4 h-4 mr-1" /> Export CSV
  </Button>
</template>
```

---

### Task 12: Settings view — profile, tenant config, notification prefs

**Files:**
- Create: `ui/src/views/SettingsView.vue`
- Create: `ui/src/components/settings/ProfileForm.vue`
- Create: `ui/src/components/settings/TenantConfigForm.vue`
- Create: `ui/src/components/settings/NotificationPrefs.vue`
- Create: `ui/src/stores/settings.ts`

**Interfaces:**
- Consumes: `GET/PUT /api/settings/profile`, `GET/PUT /api/settings/tenant`, `GET/PUT /api/settings/notifications`
- Produces: Settings page with profile edit, tenant config, notification preferences

**Steps:**

- [ ] **Step 1: Write settings store**

```typescript
// ui/src/stores/settings.ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/api/client'

export const useSettingsStore = defineStore('settings', () => {
  const profile = ref<any>(null)
  const tenantConfig = ref<any>(null)
  const notifications = ref<any>(null)
  const loading = ref(false)

  async function fetchAll() {
    loading.value = true
    try {
      const [p, t, n] = await Promise.all([
        apiClient('/api/settings/profile'),
        apiClient('/api/settings/tenant'),
        apiClient('/api/settings/notifications'),
      ])
      profile.value = p
      tenantConfig.value = t
      notifications.value = n
    } finally {
      loading.value = false
    }
  }

  async function updateProfile(data: any) {
    profile.value = await apiClient('/api/settings/profile', { method: 'PUT', body: data })
  }

  async function updateTenantConfig(data: any) {
    tenantConfig.value = await apiClient('/api/settings/tenant', { method: 'PUT', body: data })
  }

  async function updateNotifications(data: any) {
    notifications.value = await apiClient('/api/settings/notifications', { method: 'PUT', body: data })
  }

  return { profile, tenantConfig, notifications, loading, fetchAll, updateProfile, updateTenantConfig, updateNotifications }
})
```

- [ ] **Step 2: Write SettingsView**

```vue
<!-- ui/src/views/SettingsView.vue -->
<script setup lang="ts">
import { onMounted } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import { ProfileForm } from '@/components/settings/ProfileForm'
import { TenantConfigForm } from '@/components/settings/TenantConfigForm'
import { NotificationPrefs } from '@/components/settings/NotificationPrefs'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

const settings = useSettingsStore()

onMounted(() => settings.fetchAll())
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Settings</h1>

    <Tabs default-value="profile">
      <TabsList>
        <TabsTrigger value="profile">Profile</TabsTrigger>
        <TabsTrigger value="tenant">Tenant Config</TabsTrigger>
        <TabsTrigger value="notifications">Notifications</TabsTrigger>
      </TabsList>

      <TabsContent value="profile">
        <ProfileForm />
      </TabsContent>

      <TabsContent value="tenant">
        <TenantConfigForm />
      </TabsContent>

      <TabsContent value="notifications">
        <NotificationPrefs />
      </TabsContent>
    </Tabs>
  </div>
</template>
```

- [ ] **Step 3: Write ProfileForm**

```vue
<!-- ui/src/components/settings/ProfileForm.vue -->
<script setup lang="ts">
import { useSettingsStore } from '@/stores/settings'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const settings = useSettingsStore()

async function handleSave() {
  await settings.updateProfile({
    first_name: settings.profile?.first_name,
    last_name: settings.profile?.last_name,
  })
}
</script>

<template>
  <div class="max-w-xl space-y-4">
    <div class="space-y-2">
      <Label for="email">Email</Label>
      <Input id="email" :value="settings.profile?.email" disabled />
    </div>
    <div class="grid grid-cols-2 gap-4">
      <div class="space-y-2">
        <Label for="first_name">First Name</Label>
        <Input id="first_name" v-model="settings.profile?.first_name" />
      </div>
      <div class="space-y-2">
        <Label for="last_name">Last Name</Label>
        <Input id="last_name" v-model="settings.profile?.last_name" />
      </div>
    </div>
    <Button @click="handleSave">Save Changes</Button>
  </div>
</template>
```

- [ ] **Step 4: Write TenantConfigForm**

```vue
<!-- ui/src/components/settings/TenantConfigForm.vue -->
<script setup lang="ts">
import { useSettingsStore } from '@/stores/settings'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const settings = useSettingsStore()

async function handleSave() {
  await settings.updateTenantConfig({
    company_name: settings.tenantConfig?.company_name,
    timezone: settings.tenantConfig?.timezone,
    locale: settings.tenantConfig?.locale,
  })
}
</script>

<template>
  <div class="max-w-xl space-y-4">
    <div class="space-y-2">
      <Label for="company_name">Company Name</Label>
      <Input id="company_name" v-model="settings.tenantConfig?.company_name" />
    </div>
    <div class="space-y-2">
      <Label for="timezone">Timezone</Label>
      <Input id="timezone" v-model="settings.tenantConfig?.timezone" />
    </div>
    <div class="space-y-2">
      <Label for="locale">Locale</Label>
      <Input id="locale" v-model="settings.tenantConfig?.locale" />
    </div>
    <Button @click="handleSave">Save Changes</Button>
  </div>
</template>
```

- [ ] **Step 5: Write NotificationPrefs**

```vue
<!-- ui/src/components/settings/NotificationPrefs.vue -->
<script setup lang="ts">
import { useSettingsStore } from '@/stores/settings'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'

const settings = useSettingsStore()

async function handleSave() {
  await settings.updateNotifications({
    new_submission: settings.notifications?.new_submission ?? true,
    weekly_digest: settings.notifications?.weekly_digest ?? false,
  })
}
</script>

<template>
  <div class="max-w-xl space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <Label>New Submission Alerts</Label>
        <p class="text-sm text-gray-500">Get notified when someone submits a form</p>
      </div>
      <Switch
        :checked="settings.notifications?.new_submission ?? true"
        @update:checked="settings.notifications!.new_submission = !settings.notifications!.new_submission"
      />
    </div>
    <div class="flex items-center justify-between">
      <div>
        <Label>Weekly Digest</Label>
        <p class="text-sm text-gray-500">Receive a weekly summary email</p>
      </div>
      <Switch
        :checked="settings.notifications?.weekly_digest ?? false"
        @update:checked="settings.notifications!.weekly_digest = !settings.notifications!.weekly_digest"
      />
    </div>
    <Button @click="handleSave">Save Preferences</Button>
  </div>
</template>
```

---

### Task 13: Testing — unit tests, E2E tests, BFF tests

**Files:**
- Create: `ui/src/__tests__/stores/auth.test.ts`
- Create: `ui/src/__tests__/stores/contacts.test.ts`
- Create: `ui/src/__tests__/components/dashboard/StatsCard.test.ts`
- Create: `ui/playwright.config.ts`
- Create: `ui/playwright/tests/auth.spec.ts`
- Create: `ui/playwright/tests/dashboard.spec.ts`
- Create: `cmd/webui/main_test.go`
- Create: `internal/adapter/webui/handler/auth_test.go`

**Interfaces:**
- Consumes: Vitest, Playwright, Go testing
- Produces: Test coverage for auth flow, dashboard, contacts CRUD

**Steps:**

- [ ] **Step 1: Write vitest config**

Add to `ui/vite.config.ts`:

```typescript
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  test: {
    globals: true,
    environment: 'jsdom',
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
    }
  }
})
```

- [ ] **Step 2: Write auth store test**

```typescript
// ui/src/__tests__/stores/auth.test.ts
import { describe, it, expect, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'
import { apiClient } from '@/api/client'

vi.mock('@/api/client', () => ({
  apiClient: vi.fn(),
}))

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('starts unauthenticated', () => {
    const auth = useAuthStore()
    expect(auth.isAuthenticated).toBe(false)
  })

  it('sets user on login', async () => {
    const auth = useAuthStore()
    vi.mocked(apiClient).mockResolvedValue({
      token: 'test-token',
      user_id: '1',
      email: 'test@example.com',
      tenant_id: 't1',
      tenant_slug: 'test',
      is_owner: false,
    })

    await auth.login('test@example.com', 'password', 'test')

    expect(auth.isAuthenticated).toBe(true)
    expect(auth.user?.email).toBe('test@example.com')
  })

  it('clears user on logout', async () => {
    const auth = useAuthStore()
    vi.mocked(apiClient).mockResolvedValue({})

    await auth.logout()
    expect(auth.isAuthenticated).toBe(false)
  })
})
```

- [ ] **Step 3: Write StatsCard test**

```typescript
// ui/src/__tests__/components/dashboard/StatsCard.test.ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { StatsCard } from '@/components/dashboard/StatsCard'

describe('StatsCard', () => {
  it('renders title and value', () => {
    const wrapper = mount(StatsCard, {
      props: { title: 'Total Contacts', value: 42, icon: 'users' },
    })
    expect(wrapper.text()).toContain('Total Contacts')
    expect(wrapper.text()).toContain('42')
  })
})
```

- [ ] **Step 4: Write Playwright auth test**

```typescript
// ui/playwright/tests/auth.spec.ts
import { test, expect } from '@playwright/test'

test('user can log in', async ({ page }) => {
  await page.goto('/login')
  await page.fill('input[name="email"]', 'admin@example.com')
  await page.fill('input[name="password"]', 'password')
  await page.fill('input[name="tenant"]', 'demo')
  await page.click('button[type="submit"]')
  await expect(page).toHaveURL('/dashboard')
})

test('redirects to login when unauthenticated', async ({ page }) => {
  await page.goto('/contacts')
  await expect(page).toHaveURL('/login')
})
```

- [ ] **Step 5: Write Playwright dashboard test**

```typescript
// ui/playwright/tests/dashboard.spec.ts
import { test, expect } from '@playwright/test'

test('dashboard loads with stats', async ({ page }) => {
  // Assumes login first
  await page.goto('/login')
  await page.fill('input[name="email"]', 'admin@example.com')
  await page.fill('input[name="password"]', 'password')
  await page.fill('input[name="tenant"]', 'demo')
  await page.click('button[type="submit"]')

  await page.waitForURL('/dashboard')
  await expect(page.locator('text=Total Contacts')).toBeVisible()
  await expect(page.locator('text=Recent Activity')).toBeVisible()
})
```

- [ ] **Step 6: Write BFF auth handler test**

```go
// internal/adapter/webui/handler/auth_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestHandler() (*AuthHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{
		client:  http.DefaultClient,
		apiURL:  "http://localhost:8080",
		cookieCfg: CookieConfig{Secret: "test-secret"},
	}
	r := gin.Default()
	return h, r
}

func TestLogin_BadRequest(t *testing.T) {
	h, r := setupTestHandler()

	w := httptest.NewRecorder()
	c, _ := r.CreateTestRequest("POST", "/auth/login", strings.NewReader(`{"email": "invalid"}`))
	h.Login(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

---

### Task 14: Build, deploy, CI/CD pipeline

**Files:**
- Create: `cmd/webui/Dockerfile`
- Create: `ui/Dockerfile` (already in Task 2)
- Create: `deployment/docker-compose.yml` (already in Task 2)
- Create: `.github/workflows/webui.yml`
- Create: `Makefile` (webui targets)
- Modify: `deployment/docker-compose.yml` (update webui service)

**Interfaces:**
- Consumes: Docker, GitHub Actions, npm
- Produces: Container images, CI pipeline, deployment configuration

**Steps:**

- [ ] **Step 1: Write BFF Dockerfile**

```dockerfile
# cmd/webui/Dockerfile
FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /webui ./cmd/webui

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=build /webui /webui
EXPOSE 3000
CMD ["/webui"]
```

- [ ] **Step 2: Write GitHub Actions workflow**

```yaml
# .github/workflows/webui.yml
name: WebUI

on:
  push:
    branches: [main]
    paths:
      - 'ui/**'
      - 'cmd/webui/**'
  pull_request:
    branches: [main]
    paths:
      - 'ui/**'
      - 'cmd/webui/**'

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Node
        uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: 'npm'
          cache-dependency-path: ui/package-lock.json

      - name: Install deps
        run: cd ui && npm ci

      - name: Type check
        run: cd ui && npm run lint

      - name: Unit tests
        run: cd ui && npm test

      - name: Build
        run: cd ui && npm run build

  go-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Run Go tests
        run: cd cmd/webui && go test ./... -v

  e2e:
    runs-on: ubuntu-latest
    needs: test
    steps:
      - uses: actions/checkout@v4

      - name: Setup Node
        uses: actions/setup-node@v4
        with:
          node-version: 20

      - name: Install deps
        run: cd ui && npm ci

      - name: Install Playwright
        run: cd ui && npx playwright install --with-deps

      - name: Run E2E tests
        run: cd ui && npm run test:e2e
```

- [ ] **Step 3: Write Makefile targets**

```makefile
# Add to root Makefile

.PHONY: webui-dev webui-build webui-test webui-lint

webui-dev:
	cd ui && npm run dev

webui-build:
	cd ui && npm run build

webui-test:
	cd ui && npm test

webui-lint:
	cd ui && npm run lint

webui-e2e:
	cd ui && npx playwright test
```

---

## Implementation Order

Execute tasks in this sequence for optimal dependency resolution:

1. **Task 1** — Vue scaffolding (foundation)
2. **Task 2** — BFF scaffolding (foundation)
3. **Task 3** — Auth flow (required by all subsequent tasks)
4. **Task 4** — Layout shell (required by all view tasks)
5. **Task 5** — Dashboard view (first feature, low dependency)
6. **Task 6** — Contacts management (core feature)
7. **Task 7** — Forms management (core feature)
8. **Task 8** — Banners management (independent feature)
9. **Task 9** — Email templates (independent feature)
10. **Task 10** — Tenants management (admin-only, independent)
11. **Task 11** — Analytics (depends on types from previous tasks)
12. **Task 12** — Settings (independent, low priority)
13. **Task 13** — Testing (parallel with feature tasks)
14. **Task 14** — Build/deploy/CI (final step)

## Risk & Mitigation

| Risk | Mitigation |
|------|-----------|
| Backend API endpoints don't exist | BFF proxies to existing endpoints; add stub endpoints if needed |
| shadcn-vue component API changes | Pin versions in package.json; use stable components first |
| Large bundle size | Use Vite code splitting; lazy-load route components |
| Auth cookie cross-origin issues | Configure `SameSite` and `Domain` correctly in production |
| Form builder drag-and-drop complexity | Start with simple reorder buttons; add dnd library later |

## Out of Scope (v2+)

- Real-time collaboration on forms
- Advanced form builder (conditional logic, calculations, reCAPTCHA)
- File upload handling
- Webhook management UI
- Multi-language / i18n
- Dark mode toggle (CSS variables prepared, toggle not included)
- Mobile responsive sidebar (collapsible hamburger menu)
- Role-based access control beyond is_owner
