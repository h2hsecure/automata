package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hekemen/automata/internal/domain/auth"
	"github.com/hekemen/automata/internal/domain/context"
	infraauth "github.com/hekemen/automata/internal/infrastructure/auth"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
	"github.com/hekemen/automata/internal/infrastructure/config"
	cookie "github.com/hekemen/automata/internal/infrastructure/webui/cookie"
	"github.com/rs/zerolog/log"
)

type AuthHandler struct {
	authService auth.AuthService
	userRepo    context.UserRepository
	contextRepo context.Repository
	apiKeyRepo  auth_repo.ApiKeyRepository
	cookieMgr   *cookie.Manager
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type CreateAPIKeyRequest struct {
	Name      string `json:"name" binding:"required"`
	ExpiresIn *int   `json:"expires_in"`
}

func NewAuthHandler(authService auth.AuthService, userRepo context.UserRepository, contextRepo context.Repository, apiKeyRepo auth_repo.ApiKeyRepository, cookieMgr *cookie.Manager) *AuthHandler {
	return &AuthHandler{authService: authService, userRepo: userRepo, contextRepo: contextRepo, apiKeyRepo: apiKeyRepo, cookieMgr: cookieMgr}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Info().Str("email", req.Email).Msg("login attempt")

	token, userID, tenantInfos, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		log.Error().Err(err).Str("email", req.Email).Msg("auth service login failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Generate refresh token
	secretKey := "automata-dev-secret-key-change-in-production"
	if sk := config.Get("auth.secret_key"); sk != "" {
		secretKey = sk
	}
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
		true,
		true,
	)

	// Fill context details from contextRepo
	contexts := make([]gin.H, 0, len(tenantInfos))
	for _, ti := range tenantInfos {
		t, err := h.contextRepo.GetByID(ti.ID)
		if err != nil {
			continue
		}
		contexts = append(contexts, gin.H{
			"id":   t.ID,
			"slug": t.Slug,
			"name": t.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         token,
		"refresh_token": refreshTokenStr,
		"user_id":       userID,
		"email":         req.Email,
		"contexts":      contexts,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

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
		if err == infraauth.ErrRefreshTokenExpired {
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

func (h *AuthHandler) CreateAPIKey(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plaintextKey := "ak_" + strconv.FormatInt(time.Now().UnixNano(), 36) + strconv.FormatInt(time.Now().UnixNano()/2, 36)

	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		exp := time.Now().AddDate(0, 0, *req.ExpiresIn)
		expiresAt = &exp
	}

	apiKey, err := h.authService.CreateAPIKey(userID, c.GetString("context_id"), req.Name, plaintextKey, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = h.apiKeyRepo.Create(apiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         apiKey.ID,
		"name":       apiKey.Name,
		"key":        plaintextKey,
		"expires_at": apiKey.ExpiresAt,
	})
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	// Get user from platform users table
	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Get context memberships
	memberships, _ := h.userRepo.ListByUser(userID)

	contexts := make([]gin.H, 0, len(memberships))
	for _, mc := range memberships {
		ctx, err := h.contextRepo.GetByID(mc.ContextID)
		if err != nil {
			continue
		}
		contexts = append(contexts, gin.H{
			"id":   ctx.ID,
			"slug": ctx.Slug,
			"name": ctx.Name,
			"role": mc.Role,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           user.ID,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"avatar_url":   user.AvatarURL,
		"is_admin":     user.IsAdmin,
		"contexts":     contexts,
	})
}

// UpdateProfileInput holds the input for updating a user's profile.
type UpdateProfileInput struct {
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}

// UpdateProfile updates the authenticated user's display name and/or avatar URL.
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
	memberships, _ := h.userRepo.ListByUser(userID)

	contexts := make([]gin.H, 0, len(memberships))
	for _, mc := range memberships {
		ctx, err := h.contextRepo.GetByID(mc.ContextID)
		if err != nil {
			continue
		}
		contexts = append(contexts, gin.H{
			"id":   ctx.ID,
			"slug": ctx.Slug,
			"name": ctx.Name,
			"role": mc.Role,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":      user.ID,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"avatar_url":   user.AvatarURL,
		"is_admin":     user.IsAdmin,
		"contexts":     contexts,
	})
}

// ChangePasswordInput holds the input for changing a user's password.
type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

// ChangePassword updates the authenticated user's password.
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

// LogoutAll terminates all active sessions for the authenticated user.
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
