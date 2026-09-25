# Auth Refresh — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement dual-token JWT rotation (short-lived access token + 7-day refresh token) so users stay logged in silently without re-entering credentials within 7 days.

**Architecture:** Add `RefreshToken` method to the auth domain interface, implement it as a stateless JWT rotation service, introduce a separate `automata_refresh` encrypted cookie (AES-GCM), create a `POST /auth/refresh` handler that extracts and validates the refresh cookie, and modify the existing login flow to issue and store a refresh token alongside the access token.

**Tech Stack:** Go 1.26.2, Gin v1.12.0, PostgreSQL (pgx v5), golang-jwt/v5, zerolog

**Spec:** `docs/superpowers/specs/2026-09-25-auth-refresh-design.md`

## Global Constraints

- Module path: `github.com/hekemen/automata`
- Go version: `1.26.2`
- All UUIDs must be valid UUID format strings (use `gen_random_uuid()`)
- JWT HS256 signing
- Config loading: `config.Get("key")` from `internal/infrastructure/config/config.go`
- Database migrations: use `CREATE TABLE IF NOT EXISTS` and `ON CONFLICT` for idempotency
- Hexagonal architecture: domain interfaces in `internal/domain/`, infrastructure in `internal/infrastructure/`, adapters in `internal/adapter/`
- All new files go in existing package directories
- Refresh token is a standalone JWT with `"type": "refresh"` claim
- Same signing key (`auth.secret_key`) for both token types
- Rotation on every use: old refresh token invalidated immediately

---

### Task 1: Add `RefreshToken` method to `AuthService` interface

**Files:**
- Modify: `internal/domain/auth/auth.go`

**Interfaces:**
- Consumes: none
- Produces: `RefreshToken(refreshToken string) (accessToken string, newRefreshToken string, err error)` on `AuthService`

Steps:
1. **Add the `RefreshToken` method signature to the `AuthService` interface**
   - Append to `internal/domain/auth/auth.go` after `GetCurrentUser`:
     ```go
     // RefreshToken validates a refresh token and returns a new access token pair.
     // Returns (accessToken, newRefreshToken, error).
     // If the refresh token is expired, returns (nil, nil, ErrRefreshTokenExpired).
     // If the refresh token is invalid, returns (nil, nil, ErrRefreshTokenInvalid).
     RefreshToken(refreshToken string) (accessToken string, newRefreshToken string, err error)
     ```

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 2: Implement `RefreshToken` in auth service layer

**Files:**
- Create: `internal/infrastructure/auth/errors.go`
- Modify: `internal/infrastructure/auth/service.go`

**Interfaces:**
- Consumes: `config.Get("auth.secret_key")`, `jwt.Parse` from golang-jwt/v5
- Produces: Implements `RefreshToken` on `domainauth.AuthService`

Steps:
1. **Create error definitions** in `internal/infrastructure/auth/errors.go`:
   ```go
   package auth

   import "errors"

   var (
       ErrRefreshTokenExpired  = errors.New("refresh token expired")
       ErrRefreshTokenInvalid  = errors.New("invalid refresh token")
   )
   ```

2. **Implement `RefreshToken` method** at the end of `internal/infrastructure/auth/service.go`:
   ```go
   // RefreshToken validates a refresh token JWT and returns a new access + refresh token pair.
   // The refresh token must have "type": "refresh" claim and not be expired.
   // Rotation: always issues new tokens (old refresh token is invalidated).
   func (s *service) RefreshToken(refreshToken string) (string, string, error) {
       secretKey := config.Get("auth.secret_key")
       if secretKey == "" {
           secretKey = "automata-dev-secret-key-change-in-production"
       }

       // Parse and validate the refresh token
       refreshToken = strings.TrimPrefix(refreshToken, "Bearer ")
       token, err := jwt.Parse(refreshToken, func(t *jwt.Token) (interface{}, error) {
           if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
               return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
           }
           return []byte(secretKey), nil
       })

       if err != nil || !token.Valid {
           return "", "", ErrRefreshTokenInvalid
       }

       claims, ok := token.Claims.(jwt.MapClaims)
       if !ok {
           return "", "", ErrRefreshTokenInvalid
       }

       // Verify type claim is "refresh"
       if typ, ok := claims["type"].(string); !ok || typ != "refresh" {
           return "", "", ErrRefreshTokenInvalid
       }

       // Check expiry
       if !token.Valid {
           return "", "", ErrRefreshTokenExpired
       }

       // Extract user claims
       userID, ok := claims["user_id"].(string)
       if !ok || userID == "" {
           return "", "", ErrRefreshTokenInvalid
       }

       userEmail, _ := claims["user_email"].(string)
       isAdmin, _ := claims["is_admin"].(bool)

       // Issue a new access token (24h expiry) with same claims
       accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
           "user_id":    userID,
           "user_email": userEmail,
           "is_admin":   isAdmin,
           "exp":        time.Now().Add(24 * time.Hour).Unix(),
       })

       accessTokenStr, err := accessToken.SignedString([]byte(secretKey))
       if err != nil {
           log.Error().Err(err).Msg("access token generation failed")
           return "", "", fmt.Errorf("generate access token: %w", err)
       }

       // Issue a new refresh token (7-day expiry, fresh jti)
       newRefreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
           "user_id":    userID,
           "type":       "refresh",
           "exp":        time.Now().Add(7 * 24 * time.Hour).Unix(),
           "iat":        time.Now().Unix(),
       })

       newRefreshTokenStr, err := newRefreshToken.SignedString([]byte(secretKey))
       if err != nil {
           log.Error().Err(err).Msg("refresh token generation failed")
           return "", "", fmt.Errorf("generate refresh token: %w", err)
       }

       return accessTokenStr, newRefreshTokenStr, nil
   }
   ```

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 3: Add refresh cookie constants and encrypt/decrypt helpers

**Files:**
- Modify: `internal/infrastructure/webui/cookie/manager.go`

**Interfaces:**
- Consumes: existing AES-GCM encrypt/decrypt in `Manager`
- Produces: `RefreshCookieName`, `RefreshCookieMaxAge` constants; `EncryptRefreshToken`, `DecryptRefreshToken` methods on `Manager`

Steps:
1. **Add a new struct type** for the refresh cookie payload, and constants, after the existing `CookieMaxAge` constant:
   ```go
   // RefreshCookieName is the name used for the refresh token cookie.
   const RefreshCookieName = "automata_refresh"

   // RefreshCookieMaxAge is the cookie expiration in seconds (7 days).
   const RefreshCookieMaxAge = 604800

   // RefreshPayload is the JSON payload stored in the encrypted refresh cookie.
   type RefreshPayload struct {
       Token      string `json:"token"`
       CreatedAt  string `json:"created_at"`
   }
   ```

2. **Add `EncryptRefreshToken` method** to `Manager`:
   ```go
   // EncryptRefreshToken encrypts a refresh token JWT and returns a base64-encoded ciphertext.
   func (m *Manager) EncryptRefreshToken(jwtToken string) (string, error) {
       payload := RefreshPayload{
           Token:     jwtToken,
           CreatedAt: time.Now().UTC().Format(time.RFC3339),
       }
       plaintext, err := json.Marshal(payload)
       if err != nil {
           return "", fmt.Errorf("marshal refresh payload: %w", err)
       }

       block, err := aes.NewCipher([]byte(m.secret))
       if err != nil {
           return "", fmt.Errorf("create cipher: %w", err)
       }

       aesGCM, err := cipher.NewGCM(block)
       if err != nil {
           return "", fmt.Errorf("create GCM: %w", err)
       }

       nonce := make([]byte, aesGCM.NonceSize())
       if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
           return "", fmt.Errorf("generate nonce: %w", err)
       }

       ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
       return base64.URLEncoding.EncodeToString(ciphertext), nil
   }
   ```

3. **Add `DecryptRefreshToken` method** to `Manager`:
   ```go
   // DecryptRefreshToken decrypts a base64-encoded refresh cookie value and returns the JWT token.
   func (m *Manager) DecryptRefreshToken(cookieValue string) (string, error) {
       ciphertext, err := base64.URLEncoding.DecodeString(cookieValue)
       if err != nil {
           return "", fmt.Errorf("decode refresh cookie: %w", err)
       }

       block, err := aes.NewCipher([]byte(m.secret))
       if err != nil {
           return "", fmt.Errorf("create cipher: %w", err)
       }

       aesGCM, err := cipher.NewGCM(block)
       if err != nil {
           return "", fmt.Errorf("create GCM: %w", err)
       }

       nonceSize := aesGCM.NonceSize()
       if len(ciphertext) < nonceSize {
           return "", fmt.Errorf("ciphertext too short")
       }

       nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

       plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
       if err != nil {
           return "", fmt.Errorf("decrypt refresh cookie: %w", err)
       }

       var payload RefreshPayload
       if err := json.Unmarshal(plaintext, &payload); err != nil {
           return "", fmt.Errorf("unmarshal refresh payload: %w", err)
       }

       return payload.Token, nil
   }
   ```

4. **Add `time` import** to the import block (it's needed for `time.Now()` and `time.RFC3339`).

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 4: Modify login handler to issue and store refresh token

**Files:**
- Modify: `internal/adapter/api/handler/auth_handler.go`

**Interfaces:**
- Consumes: `auth.AuthService.Login()` returns `(token, userID, contexts, err)`
- Produces: Modified login response includes `refresh_token` and sets `automata_refresh` cookie

Steps:
1. **Add imports** — ensure `cookie` package is imported:
   ```go
   cookie "github.com/hekemen/automata/internal/infrastructure/webui/cookie"
   ```

2. **Add a field to AuthHandler** to hold the cookie manager:
   ```go
   type AuthHandler struct {
       authService auth.AuthService
       userRepo    context.UserRepository
       contextRepo context.Repository
       apiKeyRepo  auth_repo.ApiKeyRepository
       cookieMgr   *cookie.Manager
   }
   ```

3. **Update `NewAuthHandler`** to accept and store the cookie manager:
   ```go
   func NewAuthHandler(authService auth.AuthService, userRepo context.UserRepository,
       contextRepo context.Repository, apiKeyRepo auth_repo.ApiKeyRepository, cookieMgr *cookie.Manager) *AuthHandler {
       return &AuthHandler{
           authService: authService, userRepo: userRepo,
           contextRepo: contextRepo, apiKeyRepo: apiKeyRepo, cookieMgr: cookieMgr,
       }
   }
   ```

4. **Modify `Login` handler** to generate and set the refresh token:
   - After `token, userID, tenantInfos, err := h.authService.Login(...)` succeeds, add:
     ```go
     // Generate refresh token
     refreshClaims := jwt.MapClaims{
         "user_id": userID,
         "type":    "refresh",
         "exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
         "iat":     time.Now().Unix(),
     }
     refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
     refreshTokenStr, err := refresh.SignedString([]byte(secretKey))
     if err != nil {
         log.Error().Err(err).Msg("refresh token generation failed")
         c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
         return
     }

     // Encrypt and set refresh cookie
     refreshCookieVal, err := h.cookieMgr.EncryptRefreshToken(refreshTokenStr)
     if err != nil {
         log.Error().Err(err).Msg("encrypt refresh token failed")
         c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
         return
     }
     c.SetSameSite(http.SameSiteLaxMode)
     c.SetCookie(
         cookie.RefreshCookieName,
         refreshCookieVal,
         cookie.RefreshCookieMaxAge,
         "/",
         "",
         true, // Secure: true in production, controlled by caller
         true, // HttpOnly
     )
     ```

5. **Modify the login response JSON** to include `refresh_token`:
   ```go
   c.JSON(http.StatusOK, gin.H{
       "token":          token,
       "refresh_token":  refreshTokenStr,
       "user_id":        userID,
       "email":          req.Email,
       "is_admin":       false, // Will be set from user context
       "contexts":       contexts,
   })
   ```

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 5: Add `HandleRefresh` endpoint handler

**Files:**
- Modify: `internal/adapter/api/handler/auth_handler.go`

**Interfaces:**
- Consumes: `auth.AuthService.RefreshToken(refreshJWT) -> (accessToken, newRefreshToken, err)`
- Produces: JSON response `{ token, expires_at, refresh_token }` or 401

Steps:
1. **Add the `HandleRefresh` method** to `AuthHandler`:
   ```go
   // HandleRefresh handles POST /auth/refresh — validates the refresh cookie
   // and returns a new access token pair.
   func (h *AuthHandler) HandleRefresh(c *gin.Context) {
       // Extract the refresh cookie
       refreshCookie, err := c.Cookie(cookie.RefreshCookieName)
       if err != nil || refreshCookie == "" {
           c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh token"})
           return
       }

       // Decrypt the refresh cookie to get the JWT
       refreshJWT, err := h.cookieMgr.DecryptRefreshToken(refreshCookie)
       if err != nil {
           log.Error().Err(err).Msg("decrypt refresh token failed")
           c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
           return
       }

       // Call the auth service to rotate tokens
       accessToken, newRefreshToken, err := h.authService.RefreshToken(refreshJWT)
       if err != nil {
           if err == auth.ErrRefreshTokenExpired {
               c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token expired"})
               return
           }
           c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
           return
       }

       // Encrypt and set the new refresh cookie
       newRefreshCookieVal, err := h.cookieMgr.EncryptRefreshToken(newRefreshToken)
       if err != nil {
           log.Error().Err(err).Msg("encrypt new refresh token failed")
           c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
           return
       }
       c.SetSameSite(http.SameSiteLaxMode)
       c.SetCookie(
           cookie.RefreshCookieName,
           newRefreshCookieVal,
           cookie.RefreshCookieMaxAge,
           "/",
           "",
           true,
           true,
       )

       // Return the new access token
       c.JSON(http.StatusOK, gin.H{
           "token":         accessToken,
           "refresh_token": newRefreshToken,
       })
   }
   ```

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 6: Register the refresh route in the API server

**Files:**
- Modify: `internal/adapter/api/server.go`

**Interfaces:**
- Consumes: `NewAuthHandler(..., cookieMgr)` now requires the cookie manager
- Produces: `POST /auth/refresh` route registered under the API group

Steps:
1. **Add import** for the cookie package:
   ```go
   cookie "github.com/hekemen/automata/internal/infrastructure/webui/cookie"
   ```

2. **Create the cookie manager** near the top of `NewServer` (after `api := parent.Group(prefix)` and before the auth handler):
   ```go
   cookieMgr := cookie.New(config.Get("auth.secret_key"))
   ```

3. **Pass `cookieMgr` to `NewAuthHandler`**:
   Change:
   ```go
   authHandler := handler.NewAuthHandler(authService, userRepo, contextRepo, apiKeyRepo)
   ```
   To:
   ```go
   authHandler := handler.NewAuthHandler(authService, userRepo, contextRepo, apiKeyRepo, cookieMgr)
   ```

4. **Register the refresh route** after the existing auth routes:
   ```go
   api.POST("/auth/refresh", authHandler.HandleRefresh)
   ```

**Verification:**
- `go build ./...` compiles clean
- `go vet ./...` reports no issues

---

### Task 7: Add unit tests for `RefreshToken` and cookie operations

**Files:**
- Create: `internal/infrastructure/auth/service_test.go`
- Create: `internal/infrastructure/webui/cookie/manager_test.go`

**Interfaces:**
- Consumes: existing domain interfaces
- Produces: Test coverage for `RefreshToken`, login rotation, cookie encrypt/decrypt

Steps:
1. **Test `RefreshToken` happy path** — valid refresh token produces new access + refresh tokens
2. **Test `RefreshToken` expired** — token with past `exp` returns `ErrRefreshTokenExpired`
3. **Test `RefreshToken` invalid type** — access token (no `type=refresh` claim) returns `ErrRefreshTokenInvalid`
4. **Test cookie encrypt/decrypt round-trip** — encrypt then decrypt returns the original JWT
5. **Test login sets refresh cookie** — verify the refresh token appears in response and cookie

**Verification:**
- `go test ./internal/infrastructure/auth/...` passes
- `go test ./internal/infrastructure/webui/cookie/...` passes
