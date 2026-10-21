package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/form"
	"github.com/hekemen/automata/internal/domain/tracking"
	contactuc "github.com/hekemen/automata/internal/usecase/contact"
	emailusecase "github.com/hekemen/automata/internal/usecase/email"
	uc "github.com/hekemen/automata/internal/usecase/form"
	"github.com/rs/zerolog/log"
)

type FormHandler struct {
	repo         form.Repository
	contactRepo  contactuc.Repository
	trackingRepo tracking.Repository
	emailUC      *emailusecase.EmailUsecase
}

func NewFormHandler(repo form.Repository) *FormHandler {
	return &FormHandler{repo: repo}
}

// NewFormHandlerWithIdentifiers builds a form handler that also identifies
// the submitting visitor as a contact on form submission.
func NewFormHandlerWithIdentifiers(repo form.Repository, contactRepo contactuc.Repository, trackingRepo tracking.Repository) *FormHandler {
	return &FormHandler{repo: repo, contactRepo: contactRepo, trackingRepo: trackingRepo}
}

// NewFormHandlerWithEmail builds a form handler that additionally sends
// form-integrated email templates to the submitter on form submission.
func NewFormHandlerWithEmail(repo form.Repository, contactRepo contactuc.Repository, trackingRepo tracking.Repository, emailUC *emailusecase.EmailUsecase) *FormHandler {
	return &FormHandler{repo: repo, contactRepo: contactRepo, trackingRepo: trackingRepo, emailUC: emailUC}
}

// Repo exposes the form repository for adapters that need direct access.
func (h *FormHandler) Repo() form.Repository {
	return h.repo
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

type SubmitFormBody struct {
	Data      map[string]interface{} `json:"data" form:"data"`
	VisitorID string                 `json:"visitor_id"`
}

func (h *FormHandler) List(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	forms, err := uc.ListForms(h.repo, contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"forms": forms})
}

func (h *FormHandler) Get(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	id := c.Param("id")
	f, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}

	if f.ContextID != contextID {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}

	c.JSON(http.StatusOK, f)
}

func (h *FormHandler) Create(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
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

	created, err := uc.CreateForm(h.repo, contextID, createInput)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *FormHandler) Update(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
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

	updated, err := uc.UpdateForm(h.repo, contextID, id, updateInput)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *FormHandler) Delete(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	id := c.Param("id")

	if err := uc.DeleteForm(h.repo, contextID, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *FormHandler) SubmitForm(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	var body SubmitFormBody
	if err := c.ShouldBindJSON(&body); err != nil {
		_ = c.ShouldBindQuery(&body)
	}

	var data map[string]interface{}
	if body.Data != nil {
		data = body.Data
	} else {
		data = make(map[string]interface{})
	}

	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	submitInput := uc.SubmitFormInput{
		Slug:      slug,
		Data:      data,
		VisitorID: body.VisitorID,
	}

	sub, err := uc.SubmitForm(h.repo, contextID, submitInput)
	if err != nil {
		if err.Error() == "spam detected" {
			c.JSON(http.StatusOK, gin.H{"status": "submitted"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if body.VisitorID != "" && h.contactRepo != nil && h.trackingRepo != nil {
		if _, err := uc.IdentifyVisitor(h.contactRepo, h.trackingRepo, contextID, slug, body.VisitorID, data); err != nil {
			log.Warn().Err(err).Msg("identify visitor from form submission")
		}
	}

	if h.emailUC != nil {
		if f, err := h.repo.GetBySlug(contextID, slug); err == nil {
			h.emailUC.SendForSubmission(f, sub)
		} else {
			log.Warn().Err(err).Msg("load form for email send")
		}
	}

	c.JSON(http.StatusCreated, gin.H{"ID": sub.ID})
}

func (h *FormHandler) ListSubmissions(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	formID := c.Param("id")

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

	submissions, total, err := uc.ListSubmissions(h.repo, contextID, formID, opts)
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
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	id := c.Param("id")

	submission, err := uc.GetSubmission(h.repo, contextID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
		return
	}

	c.JSON(http.StatusOK, submission)
}
