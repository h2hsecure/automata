package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/contact"
	uc "github.com/hekemen/automata/internal/usecase/contact"
)

type ContactHandler struct {
	repo contact.Repository
}

type CreateContactInput struct {
	Email        *string                `json:"email"`
	FirstName    string                 `json:"first_name"`
	LastName     string                 `json:"last_name"`
	Phone        string                 `json:"phone"`
	Company      string                 `json:"company"`
	CustomFields map[string]interface{} `json:"custom_fields"`
	Source       string                 `json:"source"`
	Tags         []string               `json:"tags"`
}

type UpdateContactInput struct {
	Email        *string                `json:"email"`
	FirstName    string                 `json:"first_name"`
	LastName     string                 `json:"last_name"`
	Phone        string                 `json:"phone"`
	Company      string                 `json:"company"`
	CustomFields map[string]interface{} `json:"custom_fields"`
	Tags         []string               `json:"tags"`
}

type MergeContactInput struct {
	MergeWith string `json:"merge_with" binding:"required"`
}

func NewContactHandler(repo contact.Repository) *ContactHandler {
	return &ContactHandler{repo: repo}
}

func (h *ContactHandler) List(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	var tags []string
	if tagsStr := c.Query("tags"); tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	source := c.Query("source")
	company := c.Query("company")
	search := c.Query("search")

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

	opts := uc.ListOptions{
		Offset: offset,
		Limit:  limit,
		Filters: contact.FilterOptions{
			Tags:     tags,
			Source:   source,
			Company:  company,
			Search:   search,
		},
	}

	result, err := uc.ListContacts(h.repo, contextID, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	contacts := make([]interface{}, 0, len(result.Contacts))
	for _, c := range result.Contacts {
		contacts = append(contacts, c)
	}

	c.JSON(http.StatusOK, gin.H{
		"contacts": contacts,
		"total":    result.Total,
		"page":     page,
		"limit":    limit,
	})
}

func (h *ContactHandler) Get(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	id := c.Param("id")
	contact, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
		return
	}

	if contact.ContextID != contextID {
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
		return
	}

	c.JSON(http.StatusOK, contact)
}

func (h *ContactHandler) Create(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	var input CreateContactInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createInput := uc.CreateInput{
		Email:        input.Email,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Phone:        input.Phone,
		Company:      input.Company,
		CustomFields: input.CustomFields,
		Source:       input.Source,
		Tags:         input.Tags,
	}

	created, err := uc.CreateContact(h.repo, contextID, createInput)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *ContactHandler) Update(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	id := c.Param("id")

	var input UpdateContactInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updateInput := uc.UpdateInput{
		Email:        input.Email,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Phone:        input.Phone,
		Company:      input.Company,
		CustomFields: input.CustomFields,
		Tags:         input.Tags,
	}

	updated, err := uc.UpdateContact(h.repo, id, contextID, updateInput)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *ContactHandler) Delete(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	id := c.Param("id")

	if err := uc.DeleteContact(h.repo, id, contextID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ContactHandler) Merge(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	keepID := c.Param("id")

	var input MergeContactInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := uc.MergeContacts(h.repo, contextID, keepID, input.MergeWith); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	merged, err := h.repo.GetByID(keepID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve merged contact"})
		return
	}

	c.JSON(http.StatusOK, merged)
}

func (h *ContactHandler) GetActivity(c *gin.Context) {
	contactID := c.Param("id")

	offset := 0
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
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

	activities, err := uc.GetActivity(h.repo, contactID, offset, limit)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"activities": activities,
		"offset":     offset,
		"limit":      limit,
	})
}
