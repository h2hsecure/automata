package tracking

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	pagedomain "github.com/h2hsecure/automata/internal/domain/page"
)

// PageServeHandler serves per-context pages publicly at
// GET /page/:contextID/:slug.
type PageServeHandler struct {
	repo pagedomain.Repository
}

// NewPageServeHandler creates a PageServeHandler.
func NewPageServeHandler(repo pagedomain.Repository) *PageServeHandler {
	return &PageServeHandler{repo: repo}
}

// RegisterPageRoutes registers the public page endpoint on the tracking engine.
func RegisterPageRoutes(r *gin.Engine, h *PageServeHandler) {
	rl := publicRateLimit()
	r.GET("/page/:contextID/:slug", rl, h.Serve)
	r.HEAD("/page/:contextID/:slug", rl, h.Serve)
}

// Serve renders the page content inside a minimal HTML document.
func (h *PageServeHandler) Serve(c *gin.Context) {
	p, err := h.repo.GetBySlug(c.Param("contextID"), c.Param("slug"))
	if err != nil || p == nil || !p.IsActive {
		c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
		return
	}
	doc := fmt.Sprintf(
		"<!DOCTYPE html><html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>%s</title></head><body>%s</body></html>",
		p.Title, p.Content,
	)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(doc))
}
