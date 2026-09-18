package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/auth"
	"github.com/hekemen/automata/internal/domain/tenant"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
	"github.com/rs/zerolog/log"
)

type AuthHandler struct {
	authService auth.AuthService
	userRepo    tenant.UserRepository
	tenantRepo  tenant.Repository
	apiKeyRepo  auth_repo.ApiKeyRepository
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	Tenant   string `json:"tenant" binding:"required"`
}

type CreateAPIKeyRequest struct {
	Name      string `json:"name" binding:"required"`
	ExpiresIn *int   `json:"expires_in"`
}

func NewAuthHandler(authService auth.AuthService, userRepo tenant.UserRepository, tenantRepo tenant.Repository, apiKeyRepo auth_repo.ApiKeyRepository) *AuthHandler {
	return &AuthHandler{authService: authService, userRepo: userRepo, tenantRepo: tenantRepo, apiKeyRepo: apiKeyRepo}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Info().Str("tenant_slug", req.Tenant).Msg("login attempt")

	t, err := h.tenantRepo.GetBySlug(req.Tenant)
	if err != nil {
		log.Error().Err(err).Str("tenant_slug", req.Tenant).Msg("tenant lookup failed")
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found", "debug": req.Tenant})
		return
	}

	u, err := h.userRepo.GetByEmail(t.ID, req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := h.authService.ComparePassword(u.PasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, _, err := h.authService.Login(req.Email, req.Password, t.ID)
	if err != nil {
		log.Error().Err(err).Msg("auth service login failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"user_id":   u.ID,
		"tenant_id": t.ID,
		"email":     u.Email,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
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

	apiKey, err := h.authService.CreateAPIKey(userID, c.GetString("tenant_id"), req.Name, plaintextKey, expiresAt)
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
