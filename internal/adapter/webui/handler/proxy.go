package handler

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	webui_cookie "github.com/hekemen/automata/internal/infrastructure/webui/cookie"
	"github.com/rs/zerolog/log"
)

// ProxyHandler forwards /api/* requests to the internal API backend,
// injecting the authenticated user's Bearer token from the session.
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

// HandleProxy forwards the request to the backend with session-based auth.
func (h *ProxyHandler) HandleProxy(c *gin.Context) {
	// Get session from cookie
	cookie, err := c.Cookie("automata_session")
	if err == nil && cookie != "" {
		session, err := h.cookieManager.Decrypt(cookie)
		if err == nil {
			// Use the UserID as a simple auth token for the backend
			if session.UserID != "" {
				c.Header("Authorization", "Bearer "+session.UserID)
			}
		}
	}

	h.proxy.ServeHTTP(c.Writer, c.Request)
}
