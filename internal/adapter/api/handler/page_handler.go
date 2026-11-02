package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/page"
	pageuc "github.com/h2hsecure/automata/internal/usecase/page"
)

// PageHandler handles per-context page CRUD on the admin API.
type PageHandler struct {
	repo page.Repository
}

// NewPageHandler creates a PageHandler.
func NewPageHandler(repo page.Repository) *PageHandler {
	return &PageHandler{repo: repo}
}

type pageInput struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	IsActive *bool  `json:"is_active"`
}

func (h *PageHandler) List(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	pages, err := pageuc.ListPages(h.repo, contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pages": pages, "total": len(pages)})
}

func (h *PageHandler) Get(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	p, err := pageuc.GetPage(h.repo, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
		return
	}

	if p.ContextID != contextID {
		c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *PageHandler) Create(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	var input pageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err := pageuc.CreatePage(h.repo, contextID, pageuc.CreatePageInput{
		Slug:     input.Slug,
		Title:    input.Title,
		Content:  input.Content,
		IsActive: input.IsActive,
	})
	if err != nil {
		if isDuplicateError(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "a page with this slug already exists"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *PageHandler) Update(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	id := c.Param("id")

	var input pageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := pageuc.UpdatePage(h.repo, contextID, id, pageuc.CreatePageInput{
		Slug:     input.Slug,
		Title:    input.Title,
		Content:  input.Content,
		IsActive: input.IsActive,
	})
	if err != nil {
		if isDuplicateError(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "a page with this slug already exists"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *PageHandler) Delete(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	id := c.Param("id")

	if err := pageuc.DeletePage(h.repo, contextID, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "page deleted"})
}

// isDuplicateError reports whether err is a unique-constraint violation.
func isDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "duplicate") ||
		strings.Contains(err.Error(), "unique") ||
		strings.Contains(err.Error(), "already exists")
}
