# Design Spec: Settings (Profile) Page

**Date:** 2026-09-25
**Status:** Draft
**PRD Reference:** Section 7.10 (Settings), Phase 4 (Weeks 9-10)

---

## 1. Problem Statement

The Automata Web UI currently has no self-service user profile management. Users have no way to:

- View or edit their own profile information (display name, avatar).
- Change their own password.
- Revoke all active sessions for security purposes.

PRD section 7.10 calls out these features as part of Phase 4 polish (Weeks 9-10). This spec defines the backend endpoints and frontend UI to deliver a complete Settings page with Profile and Security tabs.

---

## 2. Goals & Non-Goals

### Goals

1. **Profile management**: Users can view and edit their display name and avatar URL.
2. **Password management**: Users can change their password with current-password verification and strength validation.
3. **Session security**: Users can log out all sessions simultaneously.
4. **Read-only email**: Email address is displayed but not editable (managed via admin user management).
5. **Consistent UX**: Uses shadcn-vue `Tabs`, `Card`, `Input`, `Button`, `Alert` components matching the project design system (PRD section 10).

### Non-Goals

- **Avatar file upload**: Only URL-based avatars in v1 (e.g., Gravatar or external image URLs). File upload to storage is deferred.
- **2FA / MFA enrollment**: Not in scope for this phase.
- **Email change**: Email is managed exclusively via the admin user management interface (`/admin/users`).
- **OAuth/SSO provider management**: Not in scope; SSO users cannot change passwords set by their provider.
- **Audit log for password changes**: Not in scope for v1 (could be added later via existing `ws_audit_logs` or a new events table).

---

## 3. Architecture Overview

```
┌──────────────────────────────────────────────────────────┐
│  Browser: SettingsView.vue (Vue 3 + shadcn-vue)         │
│  Route: /settings                                         │
│  Tabs: Profile | Security                                 │
└────────────────────┬─────────────────────────────────────┘
                     │ HTTP/JSON (via BFF or direct to API)
                     ▼
┌──────────────────────────────────────────────────────────┐
│  API Backend (Gin server)                                │
│                                                          │
│  GET    /api/auth/profile  → ProfileHandler.GetProfile   │
│  PUT    /api/auth/profile  → ProfileHandler.UpdateProfile│
│  POST   /api/auth/password → PasswordHandler.Change      │
│  POST   /api/auth/logout-all → PasswordHandler.LogoutAll│
└────────────────────┬─────────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────────┐
│  Auth Service (internal/infrastructure/auth/service.go)  │
│  - HashPassword (bcrypt)                                 │
│  - ComparePassword (bcrypt)                              │
└────────────────────┬─────────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────────┐
│  PostgreSQL (users table + user_contexts table)          │
└──────────────────────────────────────────────────────────┘
```

### Key Architectural Decisions

1. **Two-layer API**: New endpoints live on the existing API server (`/api/auth/*`), not the BFF. The BFF's role is to serve the Vue SPA static files and proxy auth endpoints (current `/auth/login`, `/auth/logout` in `internal/adapter/webui/handler/auth.go`). Profile/password endpoints are RESTful API calls that use the existing `Authorization: Bearer <token>` auth pattern from the API server.

2. **Reuse existing auth service**: The `auth.AuthService` interface already provides `HashPassword` and `ComparePassword`. We add `ChangePassword` and `GetCurrentUser` methods rather than creating a new service.

3. **Session revocation strategy**: The BFF uses encrypted session cookies (`automata_session`). For logout-all, we use a `password_changed_at` versioning column on the `users` table — when a password changes (or logout-all is called), we update this timestamp. The auth middleware checks that the user's JWT `iat` (issued-at) claim is before the password's change timestamp. Old tokens become invalid automatically.

---

## 4. Data Model

### 4.1 Database Migration

Add two columns to the `users` table and update the `users` migration:

```sql
-- Migration: 202609250000 — Add profile and session tracking columns

ALTER TABLE users
  ADD COLUMN display_name VARCHAR(64),
  ADD COLUMN avatar_url TEXT,
  ADD COLUMN password_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_users_password_changed_at ON users(password_changed_at);
```

**Rationale for `password_changed_at`:**
- Provides a simple, effective session revocation mechanism without a separate session store.
- When password changes or logout-all is called, `password_changed_at` is updated to `NOW()`.
- JWT tokens issued before this timestamp will fail validation (middleware checks `claim.iat < password_changed_at`).

### 4.2 Domain Model Changes

**`internal/domain/context/user.go`** — Add fields to the `User` struct:

```go
type User struct {
    // ... existing fields ...
    DisplayName        *string     // nil means unset; display "email" as fallback in UI
    AvatarURL          *string     // nil means no avatar; use default avatar in UI
    PasswordChangedAt  time.Time   // used for session revocation
}
```

### 4.3 Repository Changes

**`internal/infrastructure/context/repo/user_postgres.go`** — Update:

- `Create()`: Include `display_name`, `avatar_url` in INSERT (nullable).
- `GetByID()`: Include `display_name`, `avatar_url`, `password_changed_at` in SELECT.
- `GetByUsername()`: Include new columns.
- `ListAll()`: Include new columns.
- Add new method: `UpdateProfile(userID string, displayName, avatarURL *string) error`
- Add new method: `ChangePassword(userID string, passwordHash string) error` — sets new password hash and updates `password_changed_at`.
- Add new method: `GetPasswordChangedAt(userID string) (time.Time, error)`

### 4.4 UserRepository Interface

**`internal/domain/context/user_repository.go`** — Add methods:

```go
type UserRepository interface {
    // ... existing methods ...

    // UpdateProfile updates user display name and/or avatar URL.
    UpdateProfile(userID string, displayName, avatarURL *string) error

    // ChangePassword hashes and stores a new password, updates password_changed_at.
    ChangePassword(userID string, passwordHash string) error

    // GetPasswordChangedAt returns the password_changed_at timestamp.
    GetPasswordChangedAt(userID string) (time.Time, error)
}
```

---

## 5. API Contracts

All endpoints require a valid JWT Bearer token via `Authorization: Bearer <token>`.

### 5.1 GET /api/auth/profile

**Description:** Returns the authenticated user's profile and their context memberships.

**Auth:** JWT required (via existing `AuthMiddleware`).

**Request:**
```
GET /api/auth/profile
Authorization: Bearer <token>
```

**Response (200 OK):**
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "admin@example.com",
  "display_name": "Admin User",
  "avatar_url": "https://example.com/avatar.png",
  "is_admin": true,
  "contexts": [
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "slug": "acme-corp",
      "name": "Acme Corp",
      "role": "owner"
    },
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "slug": "globex",
      "name": "Globex Inc",
      "role": "admin"
    }
  ]
}
```

**Notes:**
- `display_name` may be `null` in the response (render as email in UI if null).
- `avatar_url` may be `null` (render default avatar placeholder in UI).
- `contexts` array may be empty for users not assigned to any context.

**Error responses:**
- `401 Unauthorized` — missing or invalid token

**Implementation:** Extend existing `AuthHandler.GetMe()` to include `display_name` and `avatar_url` from the User model.

### 5.2 PUT /api/auth/profile

**Description:** Updates the authenticated user's display name and/or avatar URL.

**Auth:** JWT required.

**Request:**
```
PUT /api/auth/profile
Authorization: Bearer <token>
Content-Type: application/json

{
  "display_name": "John Doe",
  "avatar_url": "https://example.com/new-avatar.png"
}
```

**Body fields:**
| Field | Type | Required | Constraints |
|-------|------|----------|-------------|
| `display_name` | string | Optional | Max 64 chars; trimmed whitespace; if omitted, existing value is preserved |
| `avatar_url` | string | Optional | Must be a valid URL (http/https) if provided; if omitted, existing value is preserved |

**Response (200 OK):**
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "admin@example.com",
  "display_name": "John Doe",
  "avatar_url": "https://example.com/new-avatar.png",
  "is_admin": true,
  "contexts": [ ... ]
}
```

**Error responses:**
- `400 Bad Request` — display_name exceeds 64 chars, or avatar_url is not a valid URL
- `401 Unauthorized` — missing or invalid token
- `403 Forbidden` — SSO user trying to update profile (if SSO users are restricted)
- `500 Internal Server Error` — database error

**Notes:**
- Email is **not** included in this request/response — it is read-only.
- Partial updates are supported: sending only `display_name` leaves `avatar_url` unchanged, and vice versa.
- Empty string for either field clears the value (sets to NULL in DB).

**Implementation:** New handler method `ProfileHandler.UpdateProfile()` or extend `AuthHandler`.

### 5.3 POST /api/auth/password

**Description:** Changes the authenticated user's password after verifying the current password and validating the new password meets complexity requirements.

**Auth:** JWT required.

**Request:**
```
POST /api/auth/password
Authorization: Bearer <token>
Content-Type: application/json

{
  "current_password": "OldPass1!",
  "new_password": "NewPass2@"
}
```

**Body fields:**
| Field | Type | Required | Constraints |
|-------|------|----------|-------------|
| `current_password` | string | Yes | Must match current password (bcrypt verify) |
| `new_password` | string | Yes | Min 8 chars, at least 1 uppercase, 1 digit, 1 special character |

**Password strength rules (frontend + backend validation):**

| Rule | Regex / Check | Example |
|------|---------------|---------|
| Minimum length | `>= 8` chars | `short` ❌ |
| Uppercase letter | `.*[A-Z].*` | `passw0rd` ❌ |
| Digit | `.*\d.*` | `Password!` ❌ |
| Special character | `.*[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>/?].*` | `Password1` ❌ |

**Response (200 OK):**
```json
{
  "message": "password updated"
}
```

**Side effect:** Updates `password_changed_at` to `NOW()`, invalidating all existing JWT tokens for this user.

**Error responses:**
- `400 Bad Request` — new password fails strength validation (response includes `"errors": ["password must contain at least 1 uppercase letter"]`)
- `401 Unauthorized` — current password is incorrect
- `403 Forbidden` — SSO user (no local password to change)
- `500 Internal Server Error` — bcrypt/hash error

**Implementation:**
1. Verify `current_password` against stored hash using `auth.ComparePassword`.
2. Validate `new_password` against strength rules (both frontend and backend).
3. Hash `new_password` using `auth.HashPassword`.
4. Call `userRepo.ChangePassword(userID, newHash)` to update DB and `password_changed_at`.
5. Return success message.

### 5.4 POST /api/auth/logout-all

**Description:** Invalidates all active sessions for the authenticated user by updating `password_changed_at`. All existing JWT tokens become immediately invalid.

**Auth:** JWT required.

**Request:**
```
POST /api/auth/logout-all
Authorization: Bearer <token>
```

**Response (200 OK):**
```json
{
  "message": "all sessions terminated"
}
```

**Side effect:**
- Sets `password_changed_at = NOW()` in the database.
- All existing JWTs for this user (with `iat < password_changed_at`) are rejected by the auth middleware on next request.
- The frontend should redirect to `/login` after receiving this response.

**Error responses:**
- `401 Unauthorized` — missing or invalid token

**Notes:**
- This endpoint does **not** change the password; it only revokes sessions. The user keeps their current password.
- The frontend should treat this similarly to a password change: show a toast, redirect to login, clear local state.

---

## 6. UI Design

### 6.1 Route & Layout

- **Route:** `/settings`
- **Layout:** Nested under `AppLayout` (sidebar + header + main content area)
- **Sidebar entry:** "Settings" in the sidebar navigation (PRD section 6.2)

### 6.2 SettingsView.vue Structure

```vue
<template>
  <div class="max-w-2xl mx-auto py-8 px-4">
    <h1 class="text-2xl font-bold mb-6">Settings</h1>

    <Tabs v-model="activeTab" class="w-full">
      <TabsList class="grid w-full grid-cols-2">
        <TabsTrigger value="profile">Profile</TabsTrigger>
        <TabsTrigger value="security">Security</TabsTrigger>
      </TabsList>

      <!-- Profile Tab -->
      <TabsContent value="profile">
        <Card>
          <CardHeader>
            <CardTitle>Profile Information</CardTitle>
            <CardDescription>Manage your display name and avatar.</CardDescription>
          </CardHeader>
          <CardContent>
            <!-- Avatar -->
            <div class="flex items-center gap-4 mb-6">
              <Avatar>
                <AvatarImage :src="avatarUrl" />
                <AvatarFallback>{{ initialLetters }}</AvatarFallback>
              </Avatar>
              <div v-if="avatarUrl" class="text-sm">
                <Button variant="outline" size="sm" @click="clearAvatar">Remove</Button>
              </div>
            </div>

            <!-- Display Name -->
            <div class="space-y-2">
              <Label for="display-name">Display Name</Label>
              <Input
                id="display-name"
                v-model="displayName"
                placeholder="Your display name"
                :maxlength="64"
              />
              <p class="text-xs text-muted-foreground">{{ displayNameLength }}/64</p>
            </div>

            <!-- Email (read-only) -->
            <div class="space-y-2 mt-4">
              <Label for="email">Email</Label>
              <Input id="email" :value="email" disabled />
              <p class="text-xs text-muted-foreground">
                Email cannot be changed. Contact an administrator to update it.
              </p>
            </div>

            <!-- Save Button -->
            <div class="mt-6 flex justify-end">
              <Button :disabled="!isDirty" @click="saveProfile">
                Save Changes
              </Button>
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <!-- Security Tab -->
      <TabsContent value="security">
        <Card>
          <CardHeader>
            <CardTitle>Change Password</CardTitle>
            <CardDescription>Update your password and manage active sessions.</CardDescription>
          </CardHeader>
          <CardContent>
            <!-- Password Form -->
            <form @submit.prevent="changePassword" class="space-y-4">
              <!-- Current Password -->
              <div class="space-y-2">
                <Label for="current-password">Current Password</Label>
                <Input
                  id="current-password"
                  v-model="currentPassword"
                  type="password"
                  placeholder="Enter current password"
                  :disabled="changing"
                />
              </div>

              <!-- New Password -->
              <div class="space-y-2">
                <Label for="new-password">New Password</Label>
                <Input
                  id="new-password"
                  v-model="newPassword"
                  type="password"
                  placeholder="Enter new password"
                  :disabled="changing"
                />
                <!-- Strength indicator -->
                <div v-if="newPassword" class="mt-1">
                  <PasswordStrengthBar :password="newPassword" />
                  <ul class="text-xs text-muted-foreground mt-1 space-y-0.5">
                    <li :class="{ 'text-green-600': hasUpper, 'text-red-500': !hasUpper }">
                      ● At least 1 uppercase letter
                    </li>
                    <li :class="{ 'text-green-600': hasDigit, 'text-red-500': !hasDigit }">
                      ● At least 1 number
                    </li>
                    <li :class="{ 'text-green-600': hasSpecial, 'text-red-500': !hasSpecial }">
                      ● At least 1 special character
                    </li>
                    <li :class="{ 'text-green-600': isLongEnough, 'text-red-500': !isLongEnough }">
                      ● At least 8 characters
                    </li>
                  </ul>
                </div>
              </div>

              <!-- Confirm Password -->
              <div class="space-y-2">
                <Label for="confirm-password">Confirm New Password</Label>
                <Input
                  id="confirm-password"
                  v-model="confirmPassword"
                  type="password"
                  placeholder="Confirm new password"
                  :disabled="changing"
                />
                <p v-if="confirmPassword && newPassword !== confirmPassword" class="text-xs text-red-500">
                  Passwords do not match
                </p>
              </div>

              <!-- Submit -->
              <div class="flex justify-between items-center mt-6">
                <Button type="submit" :disabled="!canSubmitPassword">
                  {{ changing ? 'Updating...' : 'Update Password' }}
                </Button>
              </div>
            </form>

            <!-- Divider -->
            <Separator class="my-6" />

            <!-- Session Management -->
            <div>
              <h3 class="text-sm font-medium mb-2">Active Sessions</h3>
              <Alert variant="default" class="mb-4">
                <AlertDescription>
                  Logging out all sessions will sign you out on every device.
                  You will need to log in again.
                </AlertDescription>
              </Alert>
              <Button variant="destructive" :disabled="loggingOut" @click="logoutAll">
                {{ loggingOut ? 'Terminating...' : 'Log Out All Sessions' }}
              </Button>
            </div>

            <!-- Toast feedback -->
            <Toast v-if="toastMessage" :message="toastMessage" />
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  </div>
</template>
```

### 6.3 Component Breakdown

| Component | File | Responsibility |
|-----------|------|----------------|
| `SettingsView.vue` | `src/views/SettingsView.vue` | Root view, tab navigation, state orchestration |
| `PasswordStrengthBar.vue` | `src/components/settings/PasswordStrengthBar.vue` | Visual bar showing password strength (weak/fair/good/strong) |
| `AvatarUpload.vue` | `src/components/settings/AvatarUpload.vue` | Avatar URL input + preview (URL-only, no file upload) |

### 6.4 Composables

```typescript
// src/composables/useProfile.ts
// Manages profile data fetching, validation, and submission.
function useProfile() {
  const profile = ref<ProfileData | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // GET /api/auth/profile
  async function fetchProfile() { ... }

  // PUT /api/auth/profile
  async function updateProfile(displayName?: string, avatarUrl?: string) { ... }
}

// src/composables/usePassword.ts
// Manages password change flow.
function usePassword() {
  const changing = ref(false)
  const error = ref<string | null>(null)

  // POST /api/auth/password
  async function changePassword(current: string, newPass: string, confirm: string) { ... }

  // POST /api/auth/logout-all
  async function logoutAll() { ... }

  // Password strength validation (shared with UI)
  function validateStrength(password: string): ValidationErrors { ... }
}
```

### 6.5 Pinia Store Updates

The existing `AuthStore` should be extended to support:

```typescript
interface AuthState {
  // ... existing fields ...

  // Refresh profile data (call after profile/password changes)
  refreshProfile(): Promise<void>
}
```

After a successful password change or logout-all, the store should:
1. Clear the local token.
2. Redirect to `/login`.

### 6.6 Router Update

Add the settings route to `src/router/index.ts`:

```typescript
{
  path: 'settings',
  component: () => import('@/views/SettingsView.vue'),
  meta: { requiresAuth: true }
}
```

---

## 7. Backend Implementation Plan

### 7.1 Service Layer Changes

**`internal/infrastructure/auth/service.go`** — Add method:

```go
type service struct {
    userRepo   context.UserRepository
}

// ChangePassword verifies the current password and updates to a new one.
func (s *service) ChangePassword(userID string, currentPassword, newPassword string) error {
    // 1. Fetch user, compare current password
    // 2. Hash new password
    // 3. Update DB + password_changed_at
}

// LogoutAllSessions updates password_changed_at to invalidate all tokens.
func (s *service) LogoutAllSessions(userID string) error {
    // 1. Update users.password_changed_at = NOW()
}
```

**`internal/domain/auth/auth.go`** — Update interface:

```go
type AuthService interface {
    // ... existing methods ...
    ChangePassword(userID, currentPassword, newPassword string) error
    LogoutAllSessions(userID string) error
}
```

### 7.2 Handler Layer

**`internal/adapter/api/handler/auth_handler.go`** — Add methods:

```go
func (h *AuthHandler) GetProfile(c *gin.Context) {
    // Reuses existing GetMe logic but returns display_name, avatar_url
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
    // 1. Extract user_id from context (AuthMiddleware)
    // 2. Parse display_name, avatar_url from body
    // 3. Validate constraints
    // 4. Call userRepo.UpdateProfile
    // 5. Return updated profile
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
    // 1. Extract user_id from context
    // 2. Parse request body
    // 3. Call authService.ChangePassword
    // 4. Return success
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
    // 1. Extract user_id from context
    // 2. Call authService.LogoutAllSessions
    // 3. Return success
}
```

### 7.3 Route Registration

**`internal/adapter/api/server.go`** — Register new routes:

```go
// Existing line 44: api.GET("/auth/me", middleware.AuthMiddleware(authService), authHandler.GetMe)

// Add after line 44:
api.PUT("/auth/profile", middleware.AuthMiddleware(authService), authHandler.UpdateProfile)
api.POST("/auth/password", middleware.AuthMiddleware(authService), authHandler.ChangePassword)
api.POST("/auth/logout-all", middleware.AuthMiddleware(authService), authHandler.LogoutAll)
```

### 7.4 Auth Middleware Session Revocation

**`internal/adapter/api/middleware/auth.go`** — Add session revocation check:

```go
func AuthMiddleware(authService auth.AuthService) gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenStr := extractToken(c)
        userID, err := authService.VerifyTokenUserOnly(tokenStr)
        if err != nil {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
            return
        }

        // Session revocation: check password_changed_at against JWT iat
        user, err := authService.GetUserByID(userID)
        if err != nil {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
            return
        }

        claims := extractClaims(tokenStr)
        if claims.Iat < user.PasswordChangedAt.Unix() {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session revoked"})
            return
        }

        c.Set("user_id", userID)
        c.Next()
    }
}
```

---

## 8. Error Handling

| Scenario | HTTP Status | Response Body |
|----------|-------------|---------------|
| Missing auth token | 401 | `{"error": "unauthorized"}` |
| Invalid/expired token | 401 | `{"error": "unauthorized"}` |
| Session revoked (old token) | 401 | `{"error": "session revoked"}` |
| Invalid request body | 400 | `{"error": "<validation message>"}` |
| Display name > 64 chars | 400 | `{"error": "display_name must be 64 characters or less"}` |
| Invalid avatar URL | 400 | `{"error": "avatar_url must be a valid URL"}` |
| Wrong current password | 401 | `{"error": "current password is incorrect"}` |
| New password too weak | 400 | `{"error": "new password does not meet requirements", "details": ["too short", "missing uppercase"]}` |
| SSO user changing password | 403 | `{"error": "cannot change password for SSO accounts"}` |
| Database error | 500 | `{"error": "internal server error"}` |

---

## 9. Security Considerations

1. **Password hashing**: Always uses `bcrypt.DefaultCost` (already in use by existing `HashPassword` method).
2. **No plaintext passwords in logs**: `zerolog` should never log full passwords.
3. **Session revocation via `password_changed_at`**: Ensures that after a password change or logout-all, all existing JWTs become invalid immediately without a session store.
4. **CSRF protection**: Since the API uses JWT Bearer tokens (not cookie-based auth on the API layer), CSRF is mitigated by the SameSite cookie policy on the BFF cookie. The API endpoints should still validate the Origin header if cross-origin calls are possible.
5. **Rate limiting**: Password change endpoint should have rate limiting (e.g., max 5 attempts per 15 minutes) to prevent brute-force on the current password field. This can be added as middleware later.
6. **Display name sanitization**: Strip HTML/special characters from display name to prevent XSS in the UI. Use Vue's text interpolation (not `v-html`).

---

## 10. Testing Strategy

### 10.1 Backend (Go)

| Test | Type | Description |
|------|------|-------------|
| `TestUpdateProfile` | Unit | Validates display_name truncation, avatar URL validation |
| `TestChangePassword` | Unit | Verifies bcrypt hashing, password_changed_at update |
| `TestChangePasswordWrongCurrent` | Unit | Ensures 401 for wrong current password |
| `TestChangePasswordWeakNew` | Unit | Ensures 400 for weak new password |
| `TestLogoutAllSessions` | Unit | Verifies password_changed_at is updated |
| `TestPasswordRevokesOldTokens` | Integration | JWT issued before password change is rejected |
| `TestGetProfile` | Integration | Returns correct profile + context memberships |

### 10.2 Frontend (Vitest / Playwright)

| Test | Type | Description |
|------|------|-------------|
| `SettingsView displays correctly` | Unit | Renders tabs, profile fields, password form |
| `Password strength indicator updates` | Unit | Reacts to input changes |
| `Profile save sends correct payload` | Integration | PUT /api/auth/profile with correct body |
| `Password change validates constraints` | E2E | Submit with weak password shows errors |
| `Logout-all redirects to login` | E2E | Clicking log out all triggers redirect |
| `Email is read-only` | E2E | Email field is disabled and non-editable |

---

## 11. Open Questions

1. **Should SSO users be allowed to change passwords?** The current spec returns 403 for SSO users. If SSO users have a local password backup (stored in `password_hash`), they could be allowed to change it. Decision: default to **disallow** for safety; enable only if a specific SSO flow requires it.

2. **Should we display an "Active Sessions" list?** The current design only has a "Log out all" button. A more advanced feature would show each session's device, IP, and last active time. This is deferred to a future iteration and would require session tracking infrastructure.

3. **Should the password change endpoint return the new profile data?** Currently it returns only `{"message": "password updated"}`. The frontend will need to invalidate the local token and redirect to login, so returning the profile is unnecessary.

4. **Database migration timing**: The `display_name`, `avatar_url`, and `password_changed_at` columns should be added before the handler code is deployed. Since these are nullable/defaulted columns, the migration is backward-compatible.

---

## 12. Implementation Order

1. **Database migration** — Add `display_name`, `avatar_url`, `password_changed_at` columns to `users` table.
2. **Domain model** — Update `User` struct and `UserRepository` interface.
3. **Repository layer** — Implement `UpdateProfile`, `ChangePassword`, `GetPasswordChangedAt` in `user_postgres.go`.
4. **Auth service** — Add `ChangePassword`, `LogoutAllSessions` to `service.go`.
5. **Auth middleware** — Add `password_changed_at` revocation check.
6. **API handlers** — Implement `UpdateProfile`, `ChangePassword`, `LogoutAll` handlers.
7. **Route registration** — Wire up new routes in `server.go`.
8. **Frontend** — Create `SettingsView.vue`, composable hooks, password strength component.
9. **Tests** — Backend unit/integration tests, frontend Vitest tests.
10. **E2E tests** — Playwright tests for the full settings flow.
