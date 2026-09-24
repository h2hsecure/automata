package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/auth"
	"github.com/hekemen/automata/internal/domain/context"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
	"github.com/rs/zerolog/log"
)

type AuthHandler struct {
	authService auth.AuthService
	userRepo    context.UserRepository
	contextRepo context.Repository
	apiKeyRepo  auth_repo.ApiKeyRepository
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type CreateAPIKeyRequest struct {
	Name      string `json:"name" binding:"required"`
	ExpiresIn *int   `json:"expires_in"`
}

func NewAuthHandler(authService auth.AuthService, userRepo context.UserRepository, contextRepo context.Repository, apiKeyRepo auth_repo.ApiKeyRepository) *AuthHandler {
	return &AuthHandler{authService: authService, userRepo: userRepo, contextRepo: contextRepo, apiKeyRepo: apiKeyRepo}
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
		"token":   token,
		"user_id": userID,
		"email":   req.Email,
		"contexts": contexts,
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
		"id":       user.ID,
		"email":    user.Email,
		"is_admin": user.IsAdmin,
		"contexts": contexts,
	})
}
