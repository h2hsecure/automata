# Design Spec: Auth Refresh — Dual-Token JWT Rotation

**Date:** 2026-09-25
**Status:** Draft
**Reference:** [Web UI PRD §8.2 Missing Endpoints](../webui-prd.md)
**Module:** `github.com/hekemen/automata`

---

## 1. Problem Statement

The current authentication flow issues a single JWT with a 24-hour expiry and stores it in an httpOnly cookie. When the cookie expires:

1. **Silent failures:** API requests with expired tokens receive 401 responses, causing pages to break.
2. **User friction:** Users must manually re-login every 24 hours, even after brief browser inactivity.
3. **No refresh path:** The PRD (§8.2) lists `POST /auth/refresh` as a missing endpoint. The PRD also says "BFF handles token refresh automatically" (§5.2).

A dual-token pattern (short-lived access token + longer-lived refresh token) solves both issues: the access token protects against token theft (short window), while the refresh token enables silent re-authentication without user interaction.

---

## 2. Goals & Non-Goals

### Goals

1. **Silent token renewal** — users stay logged in without re-entering credentials as long as they use the app within 7 days.
2. **Reduced exposure** — access tokens are valid only 24 hours, limiting the window of compromise.
3. **Backward compatible** — existing login flow continues to work; refresh is opt-in for the Web UI flow.
4. **Token rotation** — each refresh issues a new access token (rotation improves security: stolen refresh tokens can only be used once before the legitimate user invalidates them).
5. **No database changes** — refresh state is derived from the refresh token's own expiry claim.

### Non-Goals

1. **Refresh token revocation list** — no blacklist/allowlist table. Rotation is stateless; old refresh tokens become invalid only by expiry.
2. **Device-bound refresh tokens** — no device fingerprinting or binding in this iteration.
3. **Sliding expiry extension** — the 7-day refresh window is fixed from the initial login, not extended on each refresh.
4. **Multiple concurrent refresh tokens per session** — single refresh token per login; rotation replaces the old one.
5. **OAuth2-style refresh flow** — this is a simplified internal BFF pattern, not a full OAuth2 authorization server.

---

## 3. Architecture

### 3.1 Token Flow Diagram

```
┌──────────┐                                      ┌──────────────┐
│  Browser  │                                      │   Backend    │
│           │                                      │              │
│  Cookie:  │    1. POST /auth/login               │  1. Verify
│  refresh: │  ─────────────────────────────────►  │     creds
│  7d http  │                                      │     issue
│           │    { access_token, refresh_token }    │     tokens
│  Header:  │◄──────────────────────────────────  │
│  access:  │                                      │
│  24h JWT  │                                      │
└──────────┘                                      └──────────────┘
       │                                                  │
       │ 2. Normal requests: Authorization: Bearer <at>   │
       │ ───────────────────────────────────────────────► │
       │                                                  │
       │ 3. AT expired → 401                            │
       │ ───────────────────────────────────────────────► │
       │                                                  │
       │ 4. POST /auth/refresh (no auth header)          │
       │    Cookie: refresh_token                         │
       │ ───────────────────────────────────────────────► │
       │                                                  │
       │ 5. { new_access_token, new_refresh_token }      │
       │◄──────────────────────────────────────────────  │
       │    Cookie: new_refresh_token (rotated)          │
       │                                                  │
       │ 6. Retry original request with new AT           │
       │ ───────────────────────────────────────────────► │
       │                                                  │
       │ 7. Success 200                                  │
       │◄──────────────────────────────────────────────  │
       │                                                  │
       │ 8. RT expired / invalid                         │
       │ ───────────────────────────────────────────────► │
       │                                                  │
       │ 9. 401 → redirect to /login                     │
       │◄──────────────────────────────────────────────  │
       │                                                  │
```

### 3.2 Component Changes

```
internal/
├── domain/
│   └── auth/
│       └── auth.go          ← Add RefreshToken method to AuthService interface
│
├── infrastructure/
│   ├── auth/
│   │   └── service.go       ← Implement RefreshToken (generate new AT from valid RT)
│   └── webui/
│       └── cookie/
│           └── manager.go   ← Add RefreshTokenCookieName, RefreshCookieMaxAge constants;
│                              (no structural change to SessionData — RT is a standalone cookie)
│
└── adapter/
    └── webui/
        └── handler/
            ├── auth.go      ← Add HandleRefresh endpoint
            └── proxy.go     ← No changes needed
```

---

## 4. Data Model

**No new database tables are required.** The refresh token is a self-contained JWT — its expiry and claims are embedded in the token itself, not stored server-side.

### New Cookie

| Property | Value |
|----------|-------|
| Name | `automata_refresh` |
| Storage | Separate from `automata_session` (which holds the access token session data) |
| MaxAge | 604800 seconds (7 days) |
| HttpOnly | `true` |
| SameSite | `Lax` |
| Secure | `true` in production |
| Path | `/` |
| Value | Base64-encoded AES-GCM ciphertext of a simple JSON payload: `{ "token": "<jwt>", "created_at": "<iso8601>" }` |

### Why a separate cookie?

The existing `automata_session` cookie stores structured session data (user_id, email, is_admin, context_id) used for the BFF's middleware and server-rendered pages. The refresh token is a distinct credential used only by the SPA for silent renewal. Keeping them separate:

- Allows the refresh cookie to have a different expiry (7d) from the session cookie (24h).
- Simplifies rotation: replacing the refresh cookie does not affect session data.
- Reduces cookie size on every authenticated request (the RT cookie is not sent with normal API requests).

---

## 5. API Contracts

### 5.1 `POST /auth/login` (existing, modified)

**Request:**
```json
{
  "email": "admin@example.com",
  "password": "secret"
}
```

**Response (200 OK):**
```json
{
  "token": "<access_jwt>",
  "user_id": "uuid",
  "email": "admin@example.com",
  "is_admin": true,
  "redirect": "/",
  "refresh_token": "<refresh_jwt>"
}
```

**Cookie set:**
- `automata_refresh`: `<encrypted_json_with_refresh_jwt>`, MaxAge=604800, HttpOnly, SameSite=Lax

### 5.2 `POST /auth/refresh` (new)

**Request:**
```
(no body required; uses cookie for authentication)
Cookie: automata_refresh=<encrypted_refresh_token_payload>
```

**Response (200 OK):**
```json
{
  "token": "<new_access_jwt>",
  "expires_at": "2026-10-02T12:00:00Z",
  "refresh_token": "<new_refresh_jwt>"
}
```

**Cookie set:**
- `automata_refresh`: `<new_encrypted_payload>`, MaxAge=604800, HttpOnly, SameSite=Lax

**Response (401 Unauthorized):**
```json
{
  "error": "invalid or expired refresh token"
}
```

### 5.3 Token Payloads

**Access Token (existing format, unchanged):**
```json
{
  "user_id":    "uuid",
  "user_email": "admin@example.com",
  "is_admin":   true,
  "context_id": "uuid",
  "exp":        <unix_timestamp_24h_from_now>,
  "iat":        <unix_timestamp>,
  "jti":        "<uuid>"
}
```

**Refresh Token (new):**
```json
{
  "user_id":    "uuid",
  "type":       "refresh",
  "exp":        <unix_timestamp_7d_from_now>,
  "iat":        <unix_timestamp>,
  "jti":        "<uuid>"
}
```

Both tokens are signed with HS256 using the same `auth.secret_key`. The `jti` (JWT ID) claim enables future revocation if needed (rotation replaces the old jti).

---

## 6. Key Design Decisions

### 6.1 Refresh Token as a Standalone JWT (not embedded in session cookie)

**Decision:** Issue the refresh token as a separate JWT, stored in its own `automata_refresh` cookie.

**Rationale:**
- **Separation of concerns:** The session cookie (`automata_session`) holds display/session metadata used by the BFF for middleware and HTML rendering. The refresh token is an authentication credential used solely by the SPA.
- **Independent expiry:** The session cookie expires in 24h (matching the access token). The refresh token expires in 7d. A single cookie would force one expiry window.
- **Rotation simplicity:** On refresh, only the `automata_refresh` cookie needs updating. The session cookie is unaffected.
- **Smaller request payloads:** The refresh cookie is not sent with normal API requests (it's only used on the `/auth/refresh` endpoint), reducing bandwidth.

### 6.2 Rotation on Every Use (Not Sliding Expiry)

**Decision:** Each successful refresh issues a new refresh token (replaces the cookie). The 7-day window is fixed from initial login.

**Rationale:**
- **Stolen token mitigation:** If a refresh token is stolen, the legitimate user's next refresh replaces it. The attacker's copy becomes invalid.
- **Simpler than sliding expiry:** Sliding expiry requires server-side state (last-used timestamp) to avoid indefinite extension. A fixed window is stateless.
- **Predictable user experience:** Users always have exactly 7 days from login before they must re-authenticate.

### 6.3 Same Signing Key for Both Token Types

**Decision:** Access tokens and refresh tokens use the same `auth.secret_key` with HS256.

**Rationale:**
- **Simplicity:** The existing `VerifyToken` method in `service.go` already validates HS256 tokens. No new key management.
- **Differentiation by claim:** The refresh token carries `"type": "refresh"` to distinguish it from access tokens during validation.
- **Future-proof:** If separate keys are needed later (e.g., for key rotation), the JWT `kid` header can be extended.

### 6.4 Refresh Endpoint Does Not Require Authorization Header

**Decision:** `POST /auth/refresh` authenticates solely via the `automata_refresh` cookie. No `Authorization` header is required or accepted.

**Rationale:**
- **XSS isolation:** Access tokens in the `Authorization` header are accessible to JavaScript. Refresh tokens in httpOnly cookies are not. By separating the endpoints, a compromised JS environment can only trigger a refresh (getting a new short-lived AT), not access user data directly.
- **Matches PRD:** §5.2 says "BFF handles token refresh automatically" — the BFF should call `/auth/refresh` transparently without user interaction.
- **Practical:** The browser automatically sends cookies; the SPA doesn't need to read the refresh token from storage (which it can't, since it's httpOnly).

### 6.5 No Database Schema Changes

**Decision:** Refresh tokens are stateless JWTs. No refresh token table, blacklist, or revocation list.

**Rationale:**
- **Low overhead:** No DB queries needed for refresh validation — just JWT signature and expiry checks.
- **Acceptable risk:** The 24-hour access token expiry limits the damage window. Rotation on use means a stolen RT is valid for at most one refresh cycle.
- **Can be added later:** If revocation becomes a requirement (e.g., on password change), a simple `user_last_refresh_jti` column or a short-lived Redis blacklist can be added without restructuring.

### 6.6 Error Handling: 401 → Redirect to Login

**Decision:** When refresh fails (401), the SPA redirects to `/login`.

**Rationale:**
- **User experience:** A 7-day-expired refresh token means the user's session has lapsed. The only recovery is to re-authenticate.
- **No silent failure:** The SPA should not loop retrying a failed refresh. It should surface a "session expired, please log in" message.

---

## 7. Implementation Plan

### Phase 1: Domain & Service Layer

1. **Extend `AuthService` interface** (`internal/domain/auth/auth.go`):
   ```go
   // RefreshToken validates a refresh token and returns a new access token.
   // Returns (accessToken, newRefreshToken, error).
   // If the refresh token is expired, returns (nil, nil, ErrRefreshTokenExpired).
   // If the refresh token is invalid, returns (nil, nil, ErrRefreshTokenInvalid).
   RefreshToken(refreshToken string) (accessToken string, newRefreshToken string, err error)
   ```

2. **Implement `RefreshToken`** (`internal/infrastructure/auth/service.go`):
   - Parse and validate the refresh token (HS256 signature, `exp`, `type=refresh` claim).
   - Check that `exp > now()`. If not, return `ErrRefreshTokenExpired`.
   - Generate a new access token with the same claims (`user_id`, `user_email`, `is_admin`, `context_id` from the refresh token).
   - Generate a new refresh token with a fresh 7-day expiry and new `jti`.
   - Return both tokens.

### Phase 2: Cookie & Handler Layer

3. **Add refresh cookie constants** (`internal/infrastructure/webui/cookie/manager.go`):
   ```go
   const RefreshCookieName = "automata_refresh"
   const RefreshCookieMaxAge = 604800 // 7 days
   ```

4. **Add `HandleRefresh` endpoint** (`internal/adapter/webui/handler/auth.go`):
   ```go
   func (h *AuthHandler) HandleRefresh(c *gin.Context) {
       // 1. Extract automata_refresh cookie
       // 2. Decrypt session data (or use raw cookie value for the JWT)
       // 3. Call authService.RefreshToken(refreshJWT)
       // 4. On success: set new refresh cookie, return { token, expires_at }
       // 5. On failure: 401
   }
   ```

5. **Modify login response** (`HandleLoginPost`) to:
   - Generate a refresh token alongside the access token.
   - Set the `automata_refresh` cookie.
   - Return `refresh_token` in the JSON response for the SPA.

### Phase 3: Wire Up Routes

6. **Register the new route** in the Web UI router (likely in `internal/adapter/webui/router.go` or equivalent):
   ```go
   r.POST("/auth/refresh", authHandler.HandleRefresh)
   ```

### Phase 4: Frontend Integration (out of scope for this spec)

7. **SPA interceptor** (to be specified separately):
   - On 401 response, call `/auth/refresh`.
   - If refresh succeeds, retry the original request with the new token.
   - If refresh fails, redirect to `/login`.

---

## 8. Open Questions

1. **Should the refresh endpoint also return `is_admin`?** The SPA needs this for routing decisions. Currently, `is_admin` is derived from the JWT claims. The refresh response could include it as a convenience field, or the SPA should decode the access token JWT payload.

2. **What happens if the user's password changes during a 7-day refresh window?** The existing refresh token remains valid. This is acceptable for v1 (password change is a low-frequency operation), but a production system should invalidate all refresh tokens on password change.

3. **Should we add a `context_slug` claim to the access token?** The current `Login` method does not include it, but `VerifyToken` expects it. This is a pre-existing inconsistency that should be resolved separately.

4. **Should refresh token rotation be conditional?** Currently, the design rotates on every refresh. An alternative is to rotate only when suspicious activity is detected (e.g., IP change). For v1, unconditional rotation is simpler and more secure.

---

## 9. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Refresh token theft (XSS) | httpOnly cookie prevents JS access |
| Refresh token theft (MITM) | HTTPS required in production (`Secure=true`) |
| Token replay | Rotation on use; old RT becomes invalid |
| Token leakage in logs | Token values not logged; only user_email in login attempts |
| Brute-force on refresh | Rate-limiting should be added at the reverse proxy / API gateway layer |
| Secret key rotation | Same key for both token types; key rotation invalidates all sessions (acceptable for v1) |
