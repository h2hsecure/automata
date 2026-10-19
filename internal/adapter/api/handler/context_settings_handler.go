package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/settings"
)

type ContextSettingsHandler struct {
	repo settings.ContextRepository
}

func NewContextSettingsHandler(repo settings.ContextRepository) *ContextSettingsHandler {
	return &ContextSettingsHandler{repo: repo}
}

// List handles GET /api/admin/contexts/:id/settings — returns all settings for a context.
func (h *ContextSettingsHandler) List(c *gin.Context) {
	contextID := c.Param("id")
	if _, err := uuid.Parse(contextID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
		return
	}
	m, err := h.repo.List(contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": m})
}

// Upsert handles PUT /api/admin/contexts/:id/settings — creates or updates a single setting.
func (h *ContextSettingsHandler) Upsert(c *gin.Context) {
	contextID := c.Param("id")
	if _, err := uuid.Parse(contextID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
		return
	}
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if strings.TrimSpace(req.Key) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
		return
	}

	if err := h.repo.Upsert(contextID, req.Key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"key": req.Key, "value": req.Value})
}

// Delete handles DELETE /api/admin/contexts/:id/settings/:key — removes a single setting.
func (h *ContextSettingsHandler) Delete(c *gin.Context) {
	contextID := c.Param("id")
	if _, err := uuid.Parse(contextID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
		return
	}
	key := c.Param("key")
	if err := h.repo.Delete(contextID, key); err != nil {
		if errors.Is(err, settings.ErrNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
