package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/config"
)

type ConfigHandler struct {
	repo config.ConfigRepository
}

func NewConfigHandler(repo config.ConfigRepository) *ConfigHandler {
	return &ConfigHandler{repo: repo}
}

// GetConfig handles GET /api/admin/configs/:contextId — returns all config for a context.
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	contextID := c.Param("contextId")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contextId required"})
		return
	}

	configs, err := h.repo.GetByContext(contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"configs": configs})
}

// UpdateCORS handles PUT /api/admin/configs/:contextId/cors — updates CORS origins.
func (h *ConfigHandler) UpdateCORS(c *gin.Context) {
	contextID := c.Param("contextId")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contextId required"})
		return
	}

	var req struct {
		Origins []string `json:"origins"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Upsert(contextID, config.ConfigKeyCORS, map[string]interface{}{
		"origins": req.Origins,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "CORS config updated"})
}

// UpdateDomain handles PUT /api/admin/configs/:contextId/domain — updates domain settings.
func (h *ConfigHandler) UpdateDomain(c *gin.Context) {
	contextID := c.Param("contextId")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contextId required"})
		return
	}

	var req struct {
		Primary string   `json:"primary"`
		Aliases []string `json:"aliases"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Upsert(contextID, config.ConfigKeyDomain, map[string]interface{}{
		"primary": req.Primary,
		"aliases": req.Aliases,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Domain config updated"})
}

// UpdateDisplay handles PUT /api/admin/configs/:contextId/display — updates display settings.
func (h *ConfigHandler) UpdateDisplay(c *gin.Context) {
	contextID := c.Param("contextId")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contextId required"})
		return
	}

	var req struct {
		Name    string `json:"name"`
		LogoURL string `json:"logo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Upsert(contextID, config.ConfigKeyDisplay, map[string]interface{}{
		"name":     req.Name,
		"logo_url": req.LogoURL,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Display config updated"})
}
