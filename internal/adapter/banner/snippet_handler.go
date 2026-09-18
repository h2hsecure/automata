package banner

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/pkg/snippet"
)

// SnippetHandler serves the JavaScript banner snippet.
type SnippetHandler struct{}

// ServeBannerSnippet handles GET /snippet/banner/:id.js — returns the banner JS for a tenant.
func (h *SnippetHandler) ServeBannerSnippet(c *gin.Context) {
	tenantID := c.Param("id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID required"})
		return
	}

	js, err := snippet.GenerateBannerSnippet(tenantID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/javascript; charset=utf-8", []byte(js))
}
