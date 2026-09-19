package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/form"
	uc "github.com/hekemen/automata/internal/usecase/form"
)

type FormHandler struct {
	repo form.Repository
}

func NewFormHandler(repo form.Repository) *FormHandler {
	return &FormHandler{repo: repo}
}

type CreateFormInput struct {
	Slug        string                 `json:"slug" binding:"required"`
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	Fields      []form.FormField       `json:"fields" binding:"required"`
	Settings    map[string]interface{} `json:"settings"`
}

type UpdateFormInput struct {
	Slug        string                 `json:"slug"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Fields      []form.FormField       `json:"fields"`
	Settings    map[string]interface{} `json:"settings"`
}

type SubmitFormInput struct {
	Slug string `json:"slug" form:"slug" binding:"required"`
	Data map[string]interface{} `json:"data" form:"data"`
}

func (h *FormHandler) List(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant not found"})
		return
	}

	forms, err := uc.ListForms(h.repo, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"forms": forms})
}

func (h *FormHandler) Get(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant not found"})
		return
	}

	id := c.Param("id")
	f, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}

	if f.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}

	c.JSON(http.StatusOK, f)
}

func (h *FormHandler) Create(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant not found"})
		return
	}

	var input CreateFormInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createInput := uc.CreateFormInput{
		Slug:        input.Slug,
		Name:        input.Name,
		Description: input.Description,
		Fields:      input.Fields,
		Settings:    input.Settings,
	}

	created, err := uc.CreateForm(h.repo, tenantID, createInput)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *FormHandler) Update(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant not found"})
		return
	}

	id := c.Param("id")

	var input UpdateFormInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updateInput := uc.CreateFormInput{
		Slug:        input.Slug,
		Name:        input.Name,
		Description: input.Description,
		Fields:      input.Fields,
		Settings:    input.Settings,
	}

	updated, err := uc.UpdateForm(h.repo, tenantID, id, updateInput)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *FormHandler) Delete(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant not found"})
		return
	}

	id := c.Param("id")

	if err := uc.DeleteForm(h.repo, tenantID, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *FormHandler) SubmitForm(c *gin.Context) {
	var input SubmitFormInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.ShouldBindQuery(&input)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
			return
		}
	}

	if input.Data == nil {
		input.Data = make(map[string]interface{})
	}

	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant not found"})
		return
	}

	submitInput := uc.SubmitFormInput{
		Slug: input.Slug,
		Data: input.Data,
	}

	_, err := uc.SubmitForm(h.repo, tenantID, submitInput)
	if err != nil {
		if err.Error() == "spam detected" {
			c.JSON(http.StatusOK, gin.H{"status": "submitted"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": "submission_id"})
}

func (h *FormHandler) ListSubmissions(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant not found"})
		return
	}

	formID := c.Param("form_id")

	page := 1
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
			if limit > 100 {
				limit = 100
			}
		}
	}

	offset := (page - 1) * limit

	opts := form.ListSubmissionsOptions{
		Offset: offset,
		Limit:  limit,
	}

	submissions, total, err := uc.ListSubmissions(h.repo, formID, opts)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"submissions": submissions,
		"total":       total,
		"page":        page,
		"limit":       limit,
	})
}

func (h *FormHandler) GetSubmission(c *gin.Context) {
	id := c.Param("id")

	submission, err := uc.GetSubmission(h.repo, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
		return
	}

	c.JSON(http.StatusOK, submission)
}
