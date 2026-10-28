package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/settings"
)

type SettingsHandler struct {
	repo settings.Repository
}

func NewSettingsHandler(repo settings.Repository) *SettingsHandler {
	return &SettingsHandler{repo: repo}
}

// List handles GET /api/admin/settings — returns all global settings.
func (h *SettingsHandler) List(c *gin.Context) {
	m, err := h.repo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": m})
}

// Upsert handles PUT /api/admin/settings — creates or updates a single setting.
func (h *SettingsHandler) Upsert(c *gin.Context) {
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

	if err := h.repo.Upsert(req.Key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"key": req.Key, "value": req.Value})
}

// Delete handles DELETE /api/admin/settings/:key — removes a single setting.
func (h *SettingsHandler) Delete(c *gin.Context) {
	key := c.Param("key")
	if err := h.repo.Delete(key); err != nil {
		if errors.Is(err, settings.ErrNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
