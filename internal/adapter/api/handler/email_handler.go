package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ewemail "github.com/hekemen/automata/internal/domain/email"
	"github.com/hekemen/automata/internal/usecase/email"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// EmailHandler handles HTTP requests for email templates and sending.
type EmailHandler struct {
	usecase *email.EmailUsecase
	log     zerolog.Logger
}

// NewEmailHandler creates a new EmailHandler.
func NewEmailHandler(uc *email.EmailUsecase) *EmailHandler {
	return &EmailHandler{
		usecase: uc,
		log:     log.With().Str("handler", "email").Logger(),
	}
}

// List handles GET /admin/email-templates — list templates for a context.
func (h *EmailHandler) List(c *gin.Context) {
	contextID := c.Query("context_id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id query parameter is required"})
		return
	}

	if _, err := uuid.Parse(contextID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_id format"})
		return
	}

	templates, err := h.usecase.Repo().List(contextID)
	if err != nil {
		log.Error().Err(err).Msg("failed to list email templates")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list templates"})
		return
	}

	resp := ewemail.TemplateListResponse{
		Templates: make([]*ewemail.TemplateResponse, 0, len(templates)),
		Count:     len(templates),
	}

	for _, t := range templates {
		resp.Templates = append(resp.Templates, toTemplateResponse(t))
	}

	c.JSON(http.StatusOK, resp)
}

// Create handles POST /admin/email-templates — create a new template.
func (h *EmailHandler) Create(c *gin.Context) {
	var req ewemail.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate key format
	if err := ewemail.ValidateKey(req.Key); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate at least one body is non-empty
	if err := ewemail.ValidateCreateRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tmpl := &ewemail.EmailTemplate{
		ContextID: req.ContextID,
		Key:       req.Key,
		Subject:   req.Subject,
		BodyText:  req.BodyText,
		BodyHTML:  req.BodyHTML,
		IsDefault: req.IsDefault,
	}

	if err := h.usecase.Repo().Create(tmpl); err != nil {
		if err == ewemail.ErrDuplicateKey {
			c.JSON(http.StatusConflict, gin.H{"error": "template key already exists for this context"})
			return
		}
		log.Error().Err(err).Msg("failed to create email template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create template"})
		return
	}

	c.JSON(http.StatusCreated, toTemplateResponse(tmpl))
}

// Get handles GET /admin/email-templates/:id — get a template by ID.
func (h *EmailHandler) Get(c *gin.Context) {
	id := c.Param("id")

	tmpl, err := h.usecase.Repo().GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	c.JSON(http.StatusOK, toTemplateResponse(tmpl))
}

// Update handles PUT /admin/email-templates/:id — update a template.
func (h *EmailHandler) Update(c *gin.Context) {
	id := c.Param("id")

	existing, err := h.usecase.Repo().GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	var req ewemail.UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Apply non-zero/non-empty fields
	if req.Subject != "" {
		existing.Subject = req.Subject
	}
	if req.BodyText != "" {
		existing.BodyText = req.BodyText
	}
	if req.BodyHTML != "" {
		existing.BodyHTML = req.BodyHTML
	}
	if req.IsDefault != nil {
		existing.IsDefault = *req.IsDefault
	}

	if err := h.usecase.Repo().Update(existing); err != nil {
		log.Error().Err(err).Msg("failed to update email template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update template"})
		return
	}

	c.JSON(http.StatusOK, toTemplateResponse(existing))
}

// Delete handles DELETE /admin/email-templates/:id — delete a template.
func (h *EmailHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.usecase.Repo().Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "template deleted"})
}

// TestSend handles POST /admin/email-templates/:id/test — test-render and enqueue.
func (h *EmailHandler) TestSend(c *gin.Context) {
	id := c.Param("id")

	// Get template to determine context_id
	tmpl, err := h.usecase.Repo().GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	var req ewemail.TestSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jobID, subject, preview, err := h.usecase.TestSend(tmpl.ContextID, req)
	if err != nil {
		log.Error().Err(err).Msg("test send failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ewemail.TestSendResponse{
		JobID:               jobID,
		RenderedSubject:     subject,
		RenderedBodyPreview: preview,
	})
}

// Send handles POST /api/tenant/:slug/email/send — tenant-scoped send.
func (h *EmailHandler) Send(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	var req ewemail.SendEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jobID, err := h.usecase.SendEmail(contextID, req)
	if err != nil {
		log.Error().Err(err).Msg("send email failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, ewemail.SendEmailResponse{JobID: jobID})
}

// toTemplateResponse converts a domain model to API response.
func toTemplateResponse(t *ewemail.EmailTemplate) *ewemail.TemplateResponse {
	return &ewemail.TemplateResponse{
		ID:        t.ID,
		ContextID: t.ContextID,
		Key:       t.Key,
		Subject:   t.Subject,
		BodyText:  t.BodyText,
		BodyHTML:  t.BodyHTML,
		IsDefault: t.IsDefault,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

// sanitizeInput trims spaces from a string.
func sanitizeInput(s string) string {
	return strings.TrimSpace(s)
}
