package tracking

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/pkg/snippet"
)

// SnippetHandler serves the JavaScript tracking snippet.
type SnippetHandler struct{}

// ServeSnippet handles GET /snippet/:id.js — returns the tracking JS for a tenant.
func (h *SnippetHandler) ServeSnippet(c *gin.Context) {
	tenantID := c.Param("id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID required"})
		return
	}

	js, err := snippet.Generate(tenantID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/javascript; charset=utf-8", []byte(js))
}
