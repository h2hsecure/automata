package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/infrastructure/config"
	webui_cookie "github.com/h2hsecure/automata/internal/infrastructure/webui/cookie"
)

const testCookieSecret = "proxy-test-secret-1234567890"

// newTestProxy builds a ProxyHandler pointing at a fake backend. The
// returned manager lets tests mint/decrypt session cookies.
func newTestProxy(t *testing.T, backend http.Handler) (*ProxyHandler, *webui_cookie.Manager) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mgr := webui_cookie.New(testCookieSecret)
	ts := httptest.NewServer(backend)
	t.Cleanup(ts.Close)
	return NewProxyHandler(ts.URL, mgr), mgr
}

// newTestEngine mirrors the BFF wiring: everything that is not a
// registered route is proxied (NoRoute catch-all, like cmd/bff/main.go).
func newTestEngine(ph *ProxyHandler) *gin.Engine {
	engine := gin.New()
	engine.NoRoute(ph.HandleProxy)
	return engine
}

// extractSession pulls the raw cookie value out of a Set-Cookie header.
// The value runs up to the first ";" (attributes like "; Path=/" follow
// with no separating space), so we split on ";" before trimming.
func extractSession(setCookie string) string {
	attrs := strings.SplitN(setCookie, ";", 2)
	return strings.TrimSpace(strings.TrimPrefix(attrs[0], "automata_session="))
}

// closeNotifyRecorder wraps httptest.ResponseRecorder so it satisfies
// http.CloseNotifier. gin's responseWriter implements CloseNotifier and
// delegates to the underlying writer; a bare ResponseRecorder would make
// gin panic when the ReverseProxy consults it (production writers always
// implement CloseNotifier, so this is a test-harness concern only).
type closeNotifyRecorder struct {
	*httptest.ResponseRecorder
	notify chan bool
}

func (c *closeNotifyRecorder) CloseNotify() <-chan bool { return c.notify }

func doRequest(t *testing.T, engine *gin.Engine, method, path, cookie, auth string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if cookie != "" {
		req.Header.Set("Cookie", "automata_session="+cookie)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	w := &closeNotifyRecorder{ResponseRecorder: httptest.NewRecorder(), notify: make(chan bool)}
	engine.ServeHTTP(w, req)
	return w.ResponseRecorder
}

// TestProxy_LoginSetsSessionCookie: a successful login response is
// intercepted; the JWT + refresh token are stored in the encrypted
// session cookie.
func TestProxy_LoginSetsSessionCookie(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"token":"jwt-token-1","refresh_token":"refresh-1","user_id":"u1","email":"a@b.c"}`))
	})
	ph, mgr := newTestProxy(t, mux)
	engine := newTestEngine(ph)

	w := doRequest(t, engine, http.MethodPost, "/api/auth/login", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	cookieVal := w.Header().Get("Set-Cookie")
	if !strings.Contains(cookieVal, "automata_session=") {
		t.Fatalf("expected Set-Cookie automata_session, got %q", cookieVal)
	}
	if !strings.Contains(cookieVal, "HttpOnly") || !strings.Contains(cookieVal, "SameSite=Lax") {
		t.Errorf("cookie must be HttpOnly + SameSite=Lax, got %q", cookieVal)
	}
	// Extract the cookie value (first field).
	session := extractSession(cookieVal)
	data, err := mgr.Decrypt(session)
	if err != nil {
		t.Fatalf("decrypt session cookie: %v", err)
	}
	if data.Token != "jwt-token-1" {
		t.Errorf("session.Token = %q, want jwt-token-1", data.Token)
	}
	if data.RefreshToken != "refresh-1" {
		t.Errorf("session.RefreshToken = %q, want refresh-1", data.RefreshToken)
	}
	if data.UserID != "u1" {
		t.Errorf("session.UserID = %q, want u1", data.UserID)
	}
}

// TestProxy_LoginResponseStripsTokens: the client never sees the tokens
// in the login/refresh response body.
func TestProxy_LoginResponseStripsTokens(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"token":"jwt-token-1","refresh_token":"refresh-1","user_id":"u1","email":"a@b.c"}`))
	})
	ph, _ := newTestProxy(t, mux)
	engine := newTestEngine(ph)

	w := doRequest(t, engine, http.MethodPost, "/api/auth/login", "", "")
	body := w.Body.String()
	if strings.Contains(body, "jwt-token-1") || strings.Contains(body, "refresh-1") {
		t.Fatalf("client response must not contain tokens, got %s", body)
	}
	if !strings.Contains(body, `"user_id":"u1"`) {
		t.Errorf("other response fields must be preserved, got %s", body)
	}
}

// TestProxy_StripsClientAuthorization: any client-supplied Authorization
// header is replaced by the session JWT (defense in depth; the SPA stops
// sending it, but the proxy must not trust it).
func TestProxy_StripsClientAuthorization(t *testing.T) {
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/context/contacts", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"contacts":[]}`))
	})
	ph, mgr := newTestProxy(t, mux)
	engine := newTestEngine(ph)

	session, err := mgr.Encrypt(webui_cookie.SessionData{UserID: "u1", Email: "a@b.c", Token: "session-jwt"})
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	w := doRequest(t, engine, http.MethodGet, "/api/context/contacts", session, "Bearer evil")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotAuth != "Bearer session-jwt" {
		t.Fatalf("backend Authorization = %q, want %q", gotAuth, "Bearer session-jwt")
	}
}

// TestProxy_NoSessionNoAuthHeader: no cookie, non-login path → the
// backend must not receive an Authorization header.
func TestProxy_NoSessionNoAuthHeader(t *testing.T) {
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/context/contacts", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"contacts":[]}`))
	})
	ph, _ := newTestProxy(t, mux)
	engine := newTestEngine(ph)

	w := doRequest(t, engine, http.MethodGet, "/api/context/contacts", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotAuth != "" {
		t.Fatalf("backend Authorization = %q, want empty", gotAuth)
	}
}

// TestProxy_RefreshUpdatesCookie: a successful refresh rotates the token
// in the session cookie.
func TestProxy_RefreshUpdatesCookie(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"token":"rotated-jwt-2"}`))
	})
	ph, mgr := newTestProxy(t, mux)
	engine := newTestEngine(ph)

	old, err := mgr.Encrypt(webui_cookie.SessionData{UserID: "u1", Email: "a@b.c", Token: "old-jwt"})
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	w := doRequest(t, engine, http.MethodPost, "/api/auth/refresh", old, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	cookieVal := w.Header().Get("Set-Cookie")
	if !strings.Contains(cookieVal, "automata_session=") {
		t.Fatalf("expected Set-Cookie on refresh, got %q", cookieVal)
	}
	session := extractSession(cookieVal)
	data, err := mgr.Decrypt(session)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if data.Token != "rotated-jwt-2" {
		t.Fatalf("session.Token = %q, want rotated-jwt-2", data.Token)
	}
	if data.UserID != "u1" || data.Email != "a@b.c" {
		t.Errorf("session identity must be preserved: %+v", data)
	}
}

// TestProxy_LoginFailurePassesThrough: non-200 auth responses are not
// intercepted (no cookie, body untouched).
func TestProxy_LoginFailurePassesThrough(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid credentials"}`))
	})
	ph, _ := newTestProxy(t, mux)
	engine := newTestEngine(ph)

	w := doRequest(t, engine, http.MethodPost, "/api/auth/login", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w.Header().Get("Set-Cookie") != "" {
		t.Errorf("no Set-Cookie on failed login, got %q", w.Header().Get("Set-Cookie"))
	}
	if w.Body.String() != `{"error":"invalid credentials"}` {
		t.Errorf("body must pass through untouched, got %s", w.Body.String())
	}
}

// TestProxy_CookieHasSecureFlag: the session cookie must be flagged Secure
// (and HttpOnly + SameSite=Lax) by default, and the logout cookie must be
// too. The `insecure_cookie=true` escape hatch (local HTTP dev) omits
// Secure from both.
func TestProxy_CookieHasSecureFlag(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"token":"jwt-token-1","refresh_token":"refresh-1","user_id":"u1","email":"a@b.c"}`))
	})
	ph, mgr := newTestProxy(t, mux)
	engine := gin.New()
	engine.POST("/logout", NewAuthHandler(mgr).HandleLogout)
	engine.NoRoute(ph.HandleProxy)

	check := func(t *testing.T, wantSecure bool) {
		t.Helper()
		w := doRequest(t, engine, http.MethodPost, "/api/auth/login", "", "")
		if w.Code != http.StatusOK {
			t.Fatalf("login: expected 200, got %d", w.Code)
		}
		sc := w.Header().Get("Set-Cookie")
		if !strings.Contains(sc, "automata_session=") {
			t.Fatalf("expected Set-Cookie automata_session, got %q", sc)
		}
		if !strings.Contains(sc, "HttpOnly") || !strings.Contains(sc, "SameSite=Lax") {
			t.Errorf("login cookie must be HttpOnly + SameSite=Lax, got %q", sc)
		}
		if strings.Contains(sc, "Secure") != wantSecure {
			t.Errorf("login cookie Secure flag = %v, want %v (got %q)", !wantSecure, wantSecure, sc)
		}

		w = doRequest(t, engine, http.MethodPost, "/logout", "", "")
		if w.Code != http.StatusFound {
			t.Fatalf("logout: expected 302, got %d", w.Code)
		}
		sc = w.Header().Get("Set-Cookie")
		if !strings.Contains(sc, "automata_session=") {
			t.Fatalf("logout: expected Set-Cookie automata_session, got %q", sc)
		}
		if !strings.Contains(sc, "HttpOnly") || !strings.Contains(sc, "SameSite=Lax") {
			t.Errorf("logout cookie must be HttpOnly + SameSite=Lax, got %q", sc)
		}
		if strings.Contains(sc, "Secure") != wantSecure {
			t.Errorf("logout cookie Secure flag = %v, want %v (got %q)", !wantSecure, wantSecure, sc)
		}
	}

	// Default: Secure on both cookies.
	check(t, true)

	// Escape hatch: local HTTP dev omits Secure.
	config.Set("insecure_cookie", "true")
	defer config.Set("insecure_cookie", "")
	check(t, false)
}