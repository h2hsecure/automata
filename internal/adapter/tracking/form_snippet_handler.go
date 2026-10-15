package tracking

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/form"
	"github.com/hekemen/automata/pkg/snippet"
)

// FormSnippetHandler serves public form definition and self-rendering snippet endpoints.
type FormSnippetHandler struct {
	repo form.Repository
}

// ServeDefinition handles GET /form/definition/:contextID/:slug — returns the form definition as JSON.
func (h *FormSnippetHandler) ServeDefinition(c *gin.Context) {
	f, err := h.repo.GetBySlug(c.Param("contextID"), c.Param("slug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}
	// Emit an explicit public contract (lowercase keys) rather than the domain
	// struct, whose untagged fields serialize capitalized and whose shape is
	// the main API's internal contract.
	c.JSON(http.StatusOK, gin.H{
		"id":          f.ID,
		"contextId":   f.ContextID,
		"slug":        f.Slug,
		"name":        f.Name,
		"description": f.Description,
		"fields":      f.Fields,
	})
}

// ServeFormSnippet handles GET /form/snippet/:contextID/:slug — returns a
// self-rendering form snippet. A trailing ".js" is tolerated (the :slug param
// captures the whole segment; gin v1.10.0 does not split ":slug.js" params).
func (h *FormSnippetHandler) ServeFormSnippet(c *gin.Context) {
	slug := strings.TrimSuffix(c.Param("slug"), ".js")
	if _, err := h.repo.GetBySlug(c.Param("contextID"), slug); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}
	js, err := snippet.GenerateForm(c.Param("contextID"), slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", []byte(js))
}