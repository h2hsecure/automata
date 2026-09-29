package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	domainauth "github.com/hekemen/automata/internal/domain/auth"
	domainwebui "github.com/hekemen/automata/internal/domain/webui"
	webui_cookie "github.com/hekemen/automata/internal/infrastructure/webui/cookie"
	"github.com/rs/zerolog/log"
)

// AuthHandler handles login and logout for the Web UI.
type AuthHandler struct {
	authService domainauth.AuthService
	userRepo    domainwebui.UserRepository
	cookieMgr   *webui_cookie.Manager
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authService domainauth.AuthService, userRepo domainwebui.UserRepository, cookieMgr *webui_cookie.Manager) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		userRepo:    userRepo,
		cookieMgr:   cookieMgr,
	}
}

// HandleLogin renders the login page (redirects to SPA).
func (h *AuthHandler) HandleLogin(c *gin.Context) {
	c.Redirect(http.StatusFound, "/")
}

// HandleLoginPost processes the login form submission.
func (h *AuthHandler) HandleLoginPost(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.Redirect(http.StatusFound, "/?error=invalid_request")
		return
	}

	// Authenticate with the auth service (returns userID, email, contexts, error)
	userID, email, contexts, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		log.Warn().Err(err).Str("email", req.Email).Msg("login failed")
		c.Redirect(http.StatusFound, "/?error=invalid_credentials")
		return
	}

	// Create session data
	contextID := ""
	if len(contexts) > 0 {
		contextID = contexts[0].ID
	}

	sessionData := webui_cookie.SessionData{
		UserID:    userID,
		Email:     email,
		IsAdmin:   false, // Will be set from user repo
		ContextID: contextID,
	}

	// Encrypt and set session cookie
	encrypted, err := h.cookieMgr.Encrypt(sessionData)
	if err != nil {
		log.Error().Err(err).Msg("failed to create session")
		c.Redirect(http.StatusFound, "/login?error=session_error")
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "automata_session",
		Value:    encrypted,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})

	// Also set the token in a JSON response for SPA hydration
	c.JSON(http.StatusOK, gin.H{
		"token":    userID,
		"user_id":  userID,
		"email":    email,
		"is_admin": false,
		"redirect": "/",
	})
}

// HandleLogout clears the session cookie.
func (h *AuthHandler) HandleLogout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "automata_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	c.Redirect(http.StatusFound, "/")
}

// GetSessionData extracts session data from the cookie.
func (h *AuthHandler) GetSessionData(c *gin.Context) *webui_cookie.SessionData {
	cookie, err := c.Cookie("automata_session")
	if err != nil || cookie == "" {
		return nil
	}

	sessionData, err := h.cookieMgr.Decrypt(cookie)
	if err != nil {
		log.Warn().Err(err).Msg("failed to parse session")
		return nil
	}

	return &sessionData
}

// RequireAuth middleware checks for a valid session.
func (h *AuthHandler) RequireAuth(c *gin.Context) {
	session := h.GetSessionData(c)
	if session == nil {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}
	c.Set("session", session)
	c.Next()
}
