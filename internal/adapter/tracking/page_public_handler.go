package tracking

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	pagedomain "github.com/h2hsecure/automata/internal/domain/page"
	"github.com/h2hsecure/automata/pkg/snippet"
)

// ipHash returns the hex-encoded SHA-256 of an IP address.
func ipHash(ip string) string {
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:])
}

// PagePublicHandler serves the public page config and records visitor
// decisions for positioned pages. Trust model matches the existing
// /track/* and /form/* endpoints (anonymous visitor_id, hashed IP, no PII).
type PagePublicHandler struct {
	repo pagedomain.Repository
}

// NewPagePublicHandler creates a PagePublicHandler.
func NewPagePublicHandler(repo pagedomain.Repository) *PagePublicHandler {
	return &PagePublicHandler{repo: repo}
}

// RegisterPagePublicRoutes registers the public page endpoints.
func RegisterPagePublicRoutes(r *gin.Engine, h *PagePublicHandler) {
	rl := publicRateLimit()
	r.GET("/page/config/:contextID", rl, h.Config)
	r.HEAD("/page/config/:contextID", rl, h.Config)
	r.POST("/page/decision/:contextID", rl, h.Decision)
	r.GET("/page/snippet/:contextID.js", rl, h.ServeSnippet)
	r.HEAD("/page/snippet/:contextID.js", rl, h.ServeSnippet)
}

// Config responds with the active pages for a context, each with its
// position, buttons, and the inlined page content referenced by the first
// button whose Href is a page slug.
func (h *PagePublicHandler) Config(c *gin.Context) {
	contextID := c.Param("contextID")
	pages, err := h.repo.ListActive(contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load pages"})
		return
	}

	visitorID := c.Query("visitor_id")
	out := make([]gin.H, 0, len(pages))
	for _, p := range pages {
		if visitorID != "" {
			_ = h.repo.RecordImpression(contextID, p.ID, visitorID)
		}
		out = append(out, gin.H{
			"id":       p.ID,
			"title":    p.Title,
			"slug":     p.Slug,
			"position": p.Position,
			"content":  p.Content,
			"buttons":  p.Buttons,
			"page":     h.inlinePage(contextID, p.Buttons),
		})
	}
	c.JSON(http.StatusOK, gin.H{"pages": out})
}

// inlinePage returns the content of the first page referenced by a button
// whose Href is a page slug (starts with "/"), or nil.
func (h *PagePublicHandler) inlinePage(contextID string, buttons []pagedomain.Button) *string {
	for _, btn := range buttons {
		if strings.HasPrefix(btn.Href, "/") {
			slug := strings.TrimPrefix(btn.Href, "/")
			if p, err := h.repo.GetBySlug(contextID, slug); err == nil && p != nil && p.IsActive {
				content := p.Content
				return &content
			}
		}
	}
	return nil
}

// Decision records a visitor's action on a page button. The snippet
// posts JSON in a text/plain body to avoid a CORS preflight.
func (h *PagePublicHandler) Decision(c *gin.Context) {
	var body struct {
		PageID    string `json:"page_id"`
		Decision  string `json:"decision"`
		VisitorID string `json:"visitor_id"`
	}
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if body.PageID == "" || body.Decision == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page_id and decision are required"})
		return
	}

	p, err := h.repo.GetByID(body.PageID)
	if err != nil || p == nil || p.ContextID != c.Param("contextID") {
		c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
		return
	}

	if err := h.repo.RecordDecision(&pagedomain.Decision{
		ContextID: c.Param("contextID"),
		PageID:    body.PageID,
		VisitorID: body.VisitorID,
		Action:    body.Decision,
		IPHash:    ipHash(c.ClientIP()),
		UserAgent: c.GetHeader("User-Agent"),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record decision"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ServeSnippet responds with the self-contained page snippet JavaScript
// for a context, for embedding in customer sites as
// <script src="https://tracking.../page/snippet/<ctx>.js"></script>.
// The :contextID.js route captures the whole segment, so the context ID is
// sliced from the path (same pattern as snippet_handler.go).
func (h *PagePublicHandler) ServeSnippet(c *gin.Context) {
	path := c.Request.URL.Path
	const prefix = "/page/snippet/"
	if len(path) <= len(prefix)+3 || path[:len(prefix)] != prefix || !strings.HasSuffix(path, ".js") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	contextID := path[len(prefix) : len(path)-3]
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID required"})
		return
	}

	js, err := snippet.GeneratePageConfigSnippet(contextID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", []byte(js))
}
