package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	webui_cookie "github.com/h2hsecure/automata/internal/infrastructure/webui/cookie"
)

// AuthHandler handles logout and session inspection for the Web UI.
// Login is handled by the proxy (token custodian), not by a form handler.
type AuthHandler struct {
	cookieMgr *webui_cookie.Manager
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(cookieMgr *webui_cookie.Manager) *AuthHandler {
	return &AuthHandler{
		cookieMgr: cookieMgr,
	}
}

// HandleLogout clears the session cookie.
func (h *AuthHandler) HandleLogout(c *gin.Context) {
	http.SetCookie(c.Writer, webui_cookie.ClearSessionCookie(secureCookies()))
	c.Redirect(http.StatusFound, "/")
}
