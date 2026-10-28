package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/context"
	"github.com/rs/zerolog/log"
)

type ContextHandler struct {
	contextRepo context.Repository
	userRepo    context.UserRepository
}

func NewContextHandler(contextRepo context.Repository, userRepo context.UserRepository) *ContextHandler {
	return &ContextHandler{contextRepo: contextRepo, userRepo: userRepo}
}

func (h *ContextHandler) List(c *gin.Context) {
	offset := 0
	limit := 20

	if o := c.Query("offset"); o != "" {
		if p, err := strconv.Atoi(o); err == nil {
			offset = p
		}
	}
	if l := c.Query("limit"); l != "" {
		if p, err := strconv.Atoi(l); err == nil {
			limit = p
		}
	}

	contexts, err := h.contextRepo.List(offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"contexts": contexts, "count": len(contexts)})
}

func (h *ContextHandler) Create(c *gin.Context) {
	var req struct {
		Slug     string                 `json:"slug" binding:"required"`
		Name     string                 `json:"name" binding:"required"`
		Domain   string                 `json:"domain"`
		Settings map[string]interface{} `json:"settings"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t := &context.Context{
		Slug:     strings.ToLower(req.Slug),
		Name:     req.Name,
		Domain:   &req.Domain,
		Settings: req.Settings,
	}

	if err := h.contextRepo.Create(t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Grant the creator an owner membership so the new context appears in
	// their context list (this is how onboarding gets the first context).
	if userID := c.GetString("user_id"); userID != "" {
		membership := &context.UserContext{
			UserID:    userID,
			ContextID: t.ID,
			Role:      "owner",
		}
		if err := h.userRepo.CreateMembership(membership); err != nil {
			log.Warn().Err(err).Str("context_id", t.ID).Str("user_id", userID).Msg("failed to grant creator owner membership")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "context created but membership grant failed"})
			return
		}
	}

	c.JSON(http.StatusCreated, t)
}

func (h *ContextHandler) Get(c *gin.Context) {
	id := c.Param("id")
	t, err := h.contextRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "context not found"})
		return
	}

	c.JSON(http.StatusOK, t)
}

func (h *ContextHandler) Update(c *gin.Context) {
	id := c.Param("id")

	existing, err := h.contextRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "context not found"})
		return
	}

	var req struct {
		Slug     string                 `json:"slug"`
		Name     string                 `json:"name"`
		Domain   string                 `json:"domain"`
		IsActive *bool                  `json:"is_active"`
		Settings map[string]interface{} `json:"settings"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Slug != "" {
		existing.Slug = strings.ToLower(req.Slug)
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Domain != "" {
		existing.Domain = &req.Domain
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}
	if req.Settings != nil {
		existing.Settings = req.Settings
	}

	if err := h.contextRepo.Update(existing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, existing)
}

func (h *ContextHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.contextRepo.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "context not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "context deleted"})
}
