package banner

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/pkg/snippet"
)

// SnippetHandler serves the JavaScript banner snippet.
type SnippetHandler struct{}

// ServeBannerSnippet handles GET /snippet/banner/:id.js — returns the banner JS for a context.
func (h *SnippetHandler) ServeBannerSnippet(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID required"})
		return
	}

	js, err := snippet.GenerateBannerSnippet(contextID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/javascript; charset=utf-8", []byte(js))
}
