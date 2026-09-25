# Settings / Profile — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable per-user self-service profile management (display name, avatar), password change with strength validation, and session revocation via `password_changed_at` column versioning on the users table.

**Architecture:** Add `display_name`, `avatar_url`, and `password_changed_at` columns to the `users` table via migration. Extend `User` domain model and `UserRepository` interface. Implement `UpdateProfile`, `ChangePassword`, `LogoutAllSessions` in the PostgreSQL repository and auth service layer. Add session revocation check in auth middleware. Register new REST endpoints under `/api/auth/*` on the existing `AuthHandler`. Frontend is out of scope.

**Tech Stack:** Go 1.26.2, Gin v1.12.0, PostgreSQL (pgx v5), zerolog

**Spec:** `docs/superpowers/specs/2026-09-25-settings-profile-design.md`

## Global Constraints

- Module path: `github.com/hekemen/automata`
- Go version: `1.26.2`
- All UUIDs must be valid UUID format strings (use `gen_random_uuid()`)
- JWT HS256 signing
- Config loading: `config.Get("key")` from `internal/infrastructure/config/config.go`
- Database migrations: use `CREATE TABLE IF NOT EXISTS` and `ON CONFLICT` for idempotency
- Hexagonal architecture: domain interfaces in `internal/domain/`, infrastructure in `internal/infrastructure/`, adapters in `internal/adapter/`
- All new files go in existing package directories

---

### Task 1: Database migration — add profile and session tracking columns

**Files:**
- Create: `internal/infrastructure/database/migration_profile.sql`

**Interfaces:**
- Consumes: existing `users` table from `migration_users.sql`
- Produces: `display_name`, `avatar_url`, `password_changed_at` columns with index

Steps:
1. **Step 1: Create migration SQL file**
   ```sql
   -- Migration: 202609250000 — Add profile and session tracking columns

   ALTER TABLE users
     ADD COLUMN display_name VARCHAR(64),
     ADD COLUMN avatar_url TEXT,
     ADD COLUMN password_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

   CREATE INDEX IF NOT EXISTS idx_users_password_changed_at ON users(password_changed_at);
   ```

2. **Step 2: Ensure migration runner picks it up**
   - The `cicd/support/testdb.go` `RunMigrations` method reads `.sql` files from each directory
   - Verify that `internal/infrastructure/database/` is in the migration dirs list
   - If needed, add `internal/infrastructure/database` to the migration scan list

**Verification:**
- SQL is syntactically valid (test with `psql --no-password -f migration_profile.sql`)

---

### Task 2: Extend User domain model and UserRepository interface

**Files:**
- Modify: `internal/domain/context/user.go`
- Modify: `internal/domain/context/user_repository.go`

**Interfaces:**
- Consumes: `context.User` from `domain/context`
- Produces: Extended `User` struct with profile columns and extended `UserRepository` interface

Steps:
1. **Step 1: Add fields to User struct**
   ```go
   type User struct {
       // ... existing fields ...
       DisplayName        *string     // nil means unset; display "email" as fallback in UI
       AvatarURL          *string     // nil means no avatar; use default avatar in UI
       PasswordChangedAt  time.Time   // used for session revocation
   }
   ```

2. **Step 2: Add methods to UserRepository interface**
   ```go
   // UpdateProfile updates user display name and/or avatar URL.
   UpdateProfile(userID string, displayName, avatarURL *string) error

   // ChangePassword hashes and stores a new password, updates password_changed_at.
   ChangePassword(userID string, passwordHash string) error

   // GetPasswordChangedAt returns the password_changed_at timestamp.
   GetPasswordChangedAt(userID string) (time.Time, error)
   ```

**Verification:**
- `go build ./...` compiles clean

---

### Task 3: Implement repository methods in user_postgres.go

**Files:**
- Modify: `internal/infrastructure/context/repo/user_postgres.go`

**Interfaces:**
- Consumes: `context.UserRepository` interface
- Produces: PostgreSQL implementation of new methods

Steps:
1. **Step 1: Update existing queries to include new columns**
   - `Create()`: Add `display_name`, `avatar_url`, `password_changed_at` to INSERT and RETURNING
   - `GetByID()`: Add `display_name`, `avatar_url`, `password_changed_at` to SELECT and Scan
   - `GetByUsername()`: Add new columns to SELECT and Scan
   - `GetByEmail()`: Add new columns
   - `GetByEmailGlobal()`: Add new columns
   - `ListAll()`: Add new columns
   - `Update()`: Add `display_name`, `avatar_url` to SET clause

2. **Step 2: Implement UpdateProfile**
   ```go
   func (r *userPostgresRepo) UpdateProfile(userID string, displayName, avatarURL *string) error {
       ctx := context.Background()
       query := "UPDATE users SET display_name = COALESCE($2, display_name), avatar_url = COALESCE($3, avatar_url), updated_at = NOW() WHERE id = $1"
       _, err := r.pool.Exec(ctx, query, userID, displayName, avatarURL)
       if err != nil {
           return fmt.Errorf("update profile: %w", err)
       }
       return nil
   }
   ```

3. **Step 3: Implement ChangePassword**
   ```go
   func (r *userPostgresRepo) ChangePassword(userID string, passwordHash string) error {
       ctx := context.Background()
       query := "UPDATE users SET password_hash = $2, password_changed_at = NOW(), updated_at = NOW() WHERE id = $1"
       _, err := r.pool.Exec(ctx, query, userID, passwordHash)
       if err != nil {
           return fmt.Errorf("change password: %w", err)
       }
       return nil
   }
   ```

4. **Step 4: Implement GetPasswordChangedAt**
   ```go
   func (r *userPostgresRepo) GetPasswordChangedAt(userID string) (time.Time, error) {
       ctx := context.Background()
       var pwt time.Time
       query := "SELECT password_changed_at FROM users WHERE id = $1"
       err := r.pool.QueryRow(ctx, query, userID).Scan(&pwt)
       if err != nil {
           return time.Time{}, fmt.Errorf("get password changed at: %w", err)
       }
       return pwt, nil
   }
   ```

**Verification:**
- `go build ./...` compiles clean

---

### Task 4: Extend AuthService interface and service implementation

**Files:**
- Modify: `internal/domain/auth/auth.go`
- Modify: `internal/infrastructure/auth/service.go`

**Interfaces:**
- Consumes: `auth.AuthService` interface
- Produces: Extended interface with `ChangePassword`, `LogoutAllSessions`, `GetCurrentUser`

Steps:
1. **Step 1: Update AuthService interface in `auth.go`**
   ```go
   type AuthService interface {
       // ... existing methods ...
       ChangePassword(userID, currentPassword, newPassword string) error
       LogoutAllSessions(userID string) error
       GetCurrentUser(userID string) (*context.User, error)
   }
   ```

2. **Step 2: Implement ChangePassword in service.go**
   ```go
   func (s *service) ChangePassword(userID, currentPassword, newPassword string) error {
       // 1. Fetch user to get current password hash
       user, err := s.userRepo.GetByID(userID)
       if err != nil {
           return fmt.Errorf("get user: %w", err)
       }
       // 2. Verify current password
       if err := s.ComparePassword(user.PasswordHash, currentPassword); err != nil {
           return fmt.Errorf("invalid current password")
       }
       // 3. Validate new password strength
       if err := ValidatePasswordStrength(newPassword); err != nil {
           return fmt.Errorf("password does not meet requirements: %w", err)
       }
       // 4. Hash new password
       newHash, err := s.HashPassword(newPassword)
       if err != nil {
           return fmt.Errorf("hash new password: %w", err)
       }
       // 5. Update DB
       if err := s.userRepo.ChangePassword(userID, newHash); err != nil {
           return fmt.Errorf("update password: %w", err)
       }
       return nil
   }
   ```

3. **Step 3: Implement LogoutAllSessions in service.go**
   ```go
   func (s *service) LogoutAllSessions(userID string) error {
       if err := s.userRepo.UpdatePasswordChangedAt(userID); err != nil {
           return fmt.Errorf("logout all sessions: %w", err)
       }
       return nil
   }
   ```

4. **Step 4: Implement GetCurrentUser in service.go** (extend existing stub)
   ```go
   func (s *service) GetCurrentUser(userID string) (*context.User, error) {
       return s.userRepo.GetByID(userID)
   }
   ```

5. **Step 5: Implement password strength validator**
   ```go
   func ValidatePasswordStrength(password string) error {
       if len(password) < 8 {
           return fmt.Errorf("password must be at least 8 characters")
       }
       hasUpper := regexp.MustCompile(`.*[A-Z].*`).MatchString(password)
       hasDigit := regexp.MustCompile(`.*\d.*`).MatchString(password)
       hasSpecial := regexp.MustCompile(`.*[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>/?].*`).MatchString(password)
       if !hasUpper {
           return fmt.Errorf("missing uppercase letter")
       }
       if !hasDigit {
           return fmt.Errorf("missing digit")
       }
       if !hasSpecial {
           return fmt.Errorf("missing special character")
       }
       return nil
   }
   ```

**Verification:**
- `go build ./...` compiles clean

---

### Task 5: Add session revocation check in auth middleware

**Files:**
- Modify: `internal/adapter/api/middleware/auth.go`

**Interfaces:**
- Consumes: `auth.AuthService` (now with `VerifyTokenUserOnly` + `GetUserByID`)
- Produces: Middleware that rejects revoked tokens

Steps:
1. **Step 1: Add `GetUserByID` method to AuthService interface** (if not already via `GetCurrentUser`)
   - The existing `VerifyTokenUserOnly` already returns `userID`. We need a way to fetch the user.
   - Add `GetUserByID(userID string) (*context.User, error)` to the `AuthService` interface
   - Implement it in `service.go` by delegating to `userRepo.GetByID`

2. **Step 2: Extend AuthMiddleware to check password_changed_at**
   - After `VerifyTokenUserOnly` succeeds and before setting `user_id`, parse the JWT to get `iat` claim
   - Fetch user via `authService.GetUserByID(userID)`
   - Compare `claims.Iat < user.PasswordChangedAt.Unix()`
   - If revoked, abort with 401 and `"error": "session revoked"`
   - Add `is_admin` parsing from the JWT (already present)

   ```go
   // In the auth middleware, after verifying the token:
   parsedToken, parseErr := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
       return []byte(secretKey), nil
   })
   if parseErr == nil {
       if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
           if isAdmin, ok := claims["is_admin"].(bool); ok {
               c.Set("is_admin", isAdmin)
           }
           // Session revocation check
           user, userErr := authService.GetUserByID(userID)
           if userErr == nil {
               if iat, ok := claims["iat"].(float64); ok {
                   if int64(iat) < user.PasswordChangedAt.Unix() {
                       c.JSON(http.StatusUnauthorized, gin.H{"error": "session revoked"})
                       c.Abort()
                       return
                   }
               }
           }
       }
   }
   ```

**Verification:**
- `go build ./...` compiles clean

---

### Task 6: Implement API handlers — UpdateProfile, ChangePassword, LogoutAll

**Files:**
- Modify: `internal/adapter/api/handler/auth_handler.go`

**Interfaces:**
- Consumes: `auth.AuthService`, `context.UserRepository`
- Produces: REST handlers for profile and password endpoints

Steps:
1. **Step 1: Update GetMe handler to include profile fields**
   - Add `display_name` and `avatar_url` to the JSON response

2. **Step 2: Implement UpdateProfile handler**
   - Parse request body: optional `display_name` (max 64 chars, trimmed), optional `avatar_url` (must be valid URL)
   - Validate constraints
   - Call `userRepo.UpdateProfile(userID, displayName, avatarURL)`
   - Return updated profile with contexts

   ```go
   type UpdateProfileInput struct {
       DisplayName *string `json:"display_name"`
       AvatarURL   *string `json:"avatar_url"`
   }

   func (h *AuthHandler) UpdateProfile(c *gin.Context) {
       userID := c.GetString("user_id")
       if userID == "" {
           c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
           return
       }

       var input UpdateProfileInput
       if err := c.ShouldBindJSON(&input); err != nil {
           c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
           return
       }

       // Validate display_name length
       if input.DisplayName != nil && len(*input.DisplayName) > 64 {
           c.JSON(http.StatusBadRequest, gin.H{"error": "display_name must be 64 characters or less"})
           return
       }

       // Validate avatar_url if provided
       if input.AvatarURL != nil && *input.AvatarURL != "" {
           if _, err := url.Parse(*input.AvatarURL); err != nil {
               c.JSON(http.StatusBadRequest, gin.H{"error": "avatar_url must be a valid URL"})
               return
           }
       }

       // Update profile
       if err := h.userRepo.UpdateProfile(userID, input.DisplayName, input.AvatarURL); err != nil {
           log.Error().Err(err).Str("user_id", userID).Msg("update profile failed")
           c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
           return
       }

       // Fetch and return updated profile
       user, _ := h.userRepo.GetByID(userID)
       // ... build response with contexts ...
       c.JSON(http.StatusOK, gin.H{
           "user_id":     user.ID,
           "email":       user.Email,
           "display_name": user.DisplayName,
           "avatar_url":  user.AvatarURL,
           "is_admin":    user.IsAdmin,
           "contexts":    buildContextsResponse(h.userRepo, h.contextRepo, userID),
       })
   }
   ```

3. **Step 3: Implement ChangePassword handler**
   - Parse request: `current_password`, `new_password`
   - Validate new password strength (call `ValidatePasswordStrength`)
   - Call `authService.ChangePassword(userID, current, new)`
   - Return `{"message": "password updated"}`
   - On 401: wrong current password; on 400: weak password; on 403: SSO user

   ```go
   type ChangePasswordInput struct {
       CurrentPassword string `json:"current_password" binding:"required"`
       NewPassword     string `json:"new_password" binding:"required"`
   }

   func (h *AuthHandler) ChangePassword(c *gin.Context) {
       userID := c.GetString("user_id")
       if userID == "" {
           c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
           return
       }

       var input ChangePasswordInput
       if err := c.ShouldBindJSON(&input); err != nil {
           c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
           return
       }

       user, err := h.userRepo.GetByID(userID)
       if err != nil {
           c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
           return
       }

       // Check if SSO user
       if user.SSOProvider != nil && *user.SSOProvider != "" {
           c.JSON(http.StatusForbidden, gin.H{"error": "cannot change password for SSO accounts"})
           return
       }

       if err := h.authService.ChangePassword(userID, input.CurrentPassword, input.NewPassword); err != nil {
           if strings.Contains(err.Error(), "invalid current password") {
               c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
               return
           }
           if strings.Contains(err.Error(), "requirements") {
               c.JSON(http.StatusBadRequest, gin.H{"error": "new password does not meet requirements"})
               return
           }
           c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
           return
       }

       log.Info().Str("user_id", userID).Msg("password changed")
       c.JSON(http.StatusOK, gin.H{"message": "password updated"})
   }
   ```

4. **Step 4: Implement LogoutAll handler**
   - Call `authService.LogoutAllSessions(userID)`
   - Return `{"message": "all sessions terminated"}`

   ```go
   func (h *AuthHandler) LogoutAll(c *gin.Context) {
       userID := c.GetString("user_id")
       if userID == "" {
           c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
           return
       }

       if err := h.authService.LogoutAllSessions(userID); err != nil {
           log.Error().Err(err).Str("user_id", userID).Msg("logout all failed")
           c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
           return
       }

       log.Info().Str("user_id", userID).Msg("all sessions terminated")
       c.JSON(http.StatusOK, gin.H{"message": "all sessions terminated"})
   }
   ```

**Verification:**
- `go build ./...` compiles clean

---

### Task 7: Register new routes in server.go

**Files:**
- Modify: `internal/adapter/api/server.go`

**Interfaces:**
- Consumes: existing `authHandler`, `authService`
- Produces: Registered routes under `/api/auth/*`

Steps:
1. **Step 1: Add routes after existing `/api/auth/me` line (line 44)**
   ```go
   api.PUT("/auth/profile", middleware.AuthMiddleware(authService), authHandler.UpdateProfile)
   api.POST("/auth/password", middleware.AuthMiddleware(authService), authHandler.ChangePassword)
   api.POST("/auth/logout-all", middleware.AuthMiddleware(authService), authHandler.LogoutAll)
   ```

**Verification:**
- `go build ./...` compiles clean

---

### Task 8: Testing — backend unit and integration tests

**Files:**
- Create: `cicd/profile_handler_test.go`
- Create: `cicd/password_handler_test.go`
- Create: `cicd/session_revocation_test.go`

**Interfaces:**
- Consumes: `support.TestDB`, `support` fixtures
- Produces: Test coverage for profile update, password change, session revocation

Steps:
1. **Step 1: Unit tests for password validation**
   - `TestValidatePasswordStrength` — valid password passes
   - `TestValidatePasswordStrengthToShort` — 7 chars fails
   - `TestValidatePasswordStrengthNoUpper` — lowercase only fails
   - `TestValidatePasswordStrengthNoDigit` — no number fails
   - `TestValidatePasswordStrengthNoSpecial` — no special char fails

2. **Step 2: Integration tests for profile endpoints**
   - `TestUpdateProfile` — PUT /api/auth/profile with valid display_name and avatar_url
   - `TestUpdateProfileInvalidURL` — 400 for invalid avatar URL
   - `TestUpdateProfileLongName` — 400 for >64 char display_name
   - `TestGetProfile` — returns display_name, avatar_url, contexts

3. **Step 3: Integration tests for password change**
   - `TestChangePassword` — correct current, valid new → 200
   - `TestChangePasswordWrongCurrent` — 401 for wrong current password
   - `TestChangePasswordWeakNew` — 400 for weak new password
   - `TestChangePasswordSSOUser` — 403 for SSO user

4. **Step 4: Integration tests for session revocation**
   - `TestLogoutAllSessions` — POST /api/auth/logout-all → password_changed_at updated
   - `TestPasswordRevokesOldTokens` — JWT issued before password change is rejected (401 "session revoked")

**Verification:**
- `go build ./...` compiles clean
- Integration tests pass with `go test ./cicd/...`

---

## Frontend (out of scope for this spec)

Per the user decision: **Per-user self-service** (any logged-in user can update their own profile). The backend endpoints support this:

- `/api/auth/profile` — GET/PUT for profile management
- `/api/auth/password` — POST for password change
- `/api/auth/logout-all` — POST for session revocation
- All use `AuthMiddleware` which sets `user_id` from JWT
- Users can only modify their own profile (enforced by `userID` from the authenticated token)
