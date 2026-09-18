package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/tenant"
)

type TenantHandler struct {
	tenantRepo tenant.Repository
}

func NewTenantHandler(tenantRepo tenant.Repository) *TenantHandler {
	return &TenantHandler{tenantRepo: tenantRepo}
}

func (h *TenantHandler) List(c *gin.Context) {
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

	tenants, err := h.tenantRepo.List(offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tenants": tenants, "count": len(tenants)})
}

func (h *TenantHandler) Create(c *gin.Context) {
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

	t := &tenant.Tenant{
		Slug:     strings.ToLower(req.Slug),
		Name:     req.Name,
		Domain:   &req.Domain,
		Settings: req.Settings,
	}

	if err := h.tenantRepo.Create(t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, t)
}

func (h *TenantHandler) Get(c *gin.Context) {
	id := c.Param("id")
	t, err := h.tenantRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	c.JSON(http.StatusOK, t)
}

func (h *TenantHandler) Update(c *gin.Context) {
	id := c.Param("id")

	existing, err := h.tenantRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
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

	if err := h.tenantRepo.Update(existing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, existing)
}

func (h *TenantHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.tenantRepo.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tenant deleted"})
}
