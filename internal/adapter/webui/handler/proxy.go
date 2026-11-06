package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/infrastructure/config"
	webui_cookie "github.com/h2hsecure/automata/internal/infrastructure/webui/cookie"
	"github.com/rs/zerolog/log"
)

// secureCookies reports whether the session cookie should carry the Secure
// flag. It is true by default; the `insecure_cookie=true` config value is an
// escape hatch for local HTTP development (browsers reject Secure cookies
// over plain HTTP).
func secureCookies() bool {
	return config.Get("insecure_cookie") != "true"
}

// ProxyHandler forwards /api/* requests to the internal API backend.
// It acts as the JWT custodian for the Web UI: the browser only ever sees
// an encrypted session cookie, and the API JWT lives server-side inside
// it. On successful login/refresh the proxy captures the token from the
// response, stores it in the session cookie, and strips it from the body
// the client receives.
type ProxyHandler struct {
	proxy         *httputil.ReverseProxy
	cookieManager *webui_cookie.Manager
}

// NewProxyHandler creates a new ProxyHandler.
func NewProxyHandler(apiBackendURL string, cookieManager *webui_cookie.Manager) *ProxyHandler {
	backendURL, err := url.Parse(apiBackendURL)
	if err != nil {
		log.Fatal().Err(err).Str("url", apiBackendURL).Msg("invalid API backend URL for proxy")
	}

	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error().Err(err).Msg("proxy error")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"proxy error"}`))
	}

	return &ProxyHandler{
		proxy:         proxy,
		cookieManager: cookieManager,
	}
}

// isAuthPath reports whether the path is served through the capturing
// writer so tokens can be stripped from the response and stored in the
// session cookie.
func isAuthPath(path string) bool {
	return path == "/api/auth/login" || path == "/api/auth/refresh"
}

// bufferWriter fully captures a proxied response (status, headers, body)
// without sending anything to the client. Used on auth paths so the proxy
// can rewrite the JSON before the client sees it.
type bufferWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (b *bufferWriter) Header() http.Header { return b.header }
func (b *bufferWriter) WriteHeader(code int) {
	if b.status == 0 {
		b.status = code
	}
}
func (b *bufferWriter) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(p)
}
func (b *bufferWriter) Flush() {}

// HandleProxy forwards the request to the backend with session-based auth.
func (h *ProxyHandler) HandleProxy(c *gin.Context) {
	// Cap the request body so the proxy never forwards an unbounded upload
	// to the backend (max_body_bytes, 10 MB default).
	limit := int64(config.GetInt("max_body_bytes", 10*1024*1024))
	if cl := c.Request.ContentLength; cl > limit {
		c.AbortWithStatus(http.StatusRequestEntityTooLarge)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)

	// Strip any client-supplied Authorization: the SPA no longer sends it,
	// and the session cookie is the only credential the proxy honors.
	c.Request.Header.Del("Authorization")

	var session *webui_cookie.SessionData
	if cookie, err := c.Cookie(webui_cookie.CookieName); err == nil && cookie != "" {
		if s, err := h.cookieManager.Decrypt(cookie); err == nil {
			session = &s
		}
	}
	if session != nil && session.Token != "" {
		c.Request.Header.Set("Authorization", "Bearer "+session.Token)
	}

	if isAuthPath(c.Request.URL.Path) {
		// The capturing writer parses the response as JSON. A client
		// Accept-Encoding: gzip would make the backend (gzip middleware)
		// compress the body, and the transport only transparently
		// decompresses gzip it requested itself — so the capture would see
		// raw gzip bytes, fail to parse, and leak the tokens to the browser
		// with no session cookie set. Drop the header so the proxy's own
		// transport requests gzip and decompresses it.
		c.Request.Header.Del("Accept-Encoding")
		h.proxyAuth(c, session)
		return
	}
	h.proxy.ServeHTTP(c.Writer, c.Request)
}

// proxyAuth serves an auth-path request through the capturing writer and,
// on a 200 response carrying tokens, stores them in the session cookie
// and strips them from the client-visible body.
func (h *ProxyHandler) proxyAuth(c *gin.Context, session *webui_cookie.SessionData) {
	bw := &bufferWriter{header: http.Header{}}
	h.proxy.ServeHTTP(bw, c.Request)

	status := bw.status
	if status == 0 {
		status = http.StatusOK
	}

	body := bw.body.Bytes()

	if status == http.StatusOK {
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err == nil {
			if tok, _ := payload["token"].(string); tok != "" {
				// Build the new session: preserve identity when we already
				// have one, otherwise take it from the response.
				var next webui_cookie.SessionData
				if session != nil {
					next = *session
				} else {
					if uid, ok := payload["user_id"].(string); ok {
						next.UserID = uid
					}
					if email, ok := payload["email"].(string); ok {
						next.Email = email
					}
				}
				next.Token = tok
				if rt, ok := payload["refresh_token"].(string); ok {
					next.RefreshToken = rt
				}

				if encrypted, err := h.cookieManager.Encrypt(next); err == nil {
					http.SetCookie(c.Writer, webui_cookie.SessionCookie(encrypted, secureCookies()))
				} else {
					log.Error().Err(err).Msg("failed to encrypt session cookie")
				}

				// The client never sees the tokens.
				delete(payload, "token")
				delete(payload, "refresh_token")
				if newBody, err := json.Marshal(payload); err == nil {
					body = newBody
				}
			}
		}
	}

	// Copy backend headers (except Content-Length, which we recompute).
	// Set-Cookie must be APPENDED: http.SetCookie above already added the
	// BFF's session cookie, and c.Header uses Set semantics, which would
	// silently replace it with the backend's refresh cookie — dropping the
	// session and turning every post-login API call into a 401.
	for k, vs := range bw.header {
		if k == "Content-Length" {
			continue
		}
		for _, v := range vs {
			if k == "Set-Cookie" {
				c.Writer.Header().Add(k, v)
			} else {
				c.Header(k, v)
			}
		}
	}
	c.Header("Content-Length", strconv.Itoa(len(body)))
	c.Status(status)
	if _, err := c.Writer.Write(body); err != nil {
		log.Error().Err(err).Msg("failed to write proxied auth response")
	}
}