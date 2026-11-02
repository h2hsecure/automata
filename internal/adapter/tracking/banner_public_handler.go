package tracking

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	bdomain "github.com/h2hsecure/automata/internal/domain/banner"
	pagedomain "github.com/h2hsecure/automata/internal/domain/page"
	"github.com/h2hsecure/automata/pkg/snippet"
)

// BannerPublicHandler serves the public banner config and records visitor
// decisions. Trust model matches the existing /track/banner/* and /form/*
// endpoints (anonymous visitor_id, hashed IP, no PII).
type BannerPublicHandler struct {
	bannerRepo bdomain.Repository
	pageRepo   pagedomain.Repository
}

// NewBannerPublicHandler creates a BannerPublicHandler.
func NewBannerPublicHandler(bannerRepo bdomain.Repository, pageRepo pagedomain.Repository) *BannerPublicHandler {
	return &BannerPublicHandler{bannerRepo: bannerRepo, pageRepo: pageRepo}
}

// RegisterBannerPublicRoutes registers the public banner endpoints.
func RegisterBannerPublicRoutes(r *gin.Engine, h *BannerPublicHandler) {
	r.GET("/banner/config/:contextID", h.Config)
	r.HEAD("/banner/config/:contextID", h.Config)
	r.POST("/banner/decision/:contextID", h.Decision)
	r.GET("/banner/snippet/:contextID.js", h.ServeSnippet)
	r.HEAD("/banner/snippet/:contextID.js", h.ServeSnippet)
}

// Config responds with the active banners for a context, each with its
// position, buttons, and the inlined page content referenced by the first
// button whose LinkURL is a page slug.
func (h *BannerPublicHandler) Config(c *gin.Context) {
	contextID := c.Param("contextID")
	banners, err := h.bannerRepo.ListActiveBannersForPlacement(contextID, "", time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load banners"})
		return
	}

	visitorID := c.Query("visitor_id")
	out := make([]gin.H, 0, len(banners))
	for _, b := range banners {
		if visitorID != "" {
			_ = h.bannerRepo.RecordImpression(contextID, b.ID, visitorID)
		}
		out = append(out, gin.H{
			"id":       b.ID,
			"name":     b.Name,
			"position": b.Position,
			"content":  b.Content,
			"buttons":  b.Buttons,
			"page":     h.inlinePage(contextID, b.Buttons),
		})
	}
	c.JSON(http.StatusOK, gin.H{"banners": out})
}

// inlinePage returns the content of the first page referenced by a button
// whose LinkURL is a page slug (starts with "/"), or nil.
func (h *BannerPublicHandler) inlinePage(contextID string, buttons []bdomain.Button) *string {
	for _, btn := range buttons {
		if strings.HasPrefix(btn.LinkURL, "/") {
			slug := strings.TrimPrefix(btn.LinkURL, "/")
			if p, err := h.pageRepo.GetBySlug(contextID, slug); err == nil && p != nil && p.IsActive {
				content := p.Content
				return &content
			}
		}
	}
	return nil
}

// Decision records a visitor's action on a banner button. The snippet
// posts JSON in a text/plain body to avoid a CORS preflight.
func (h *BannerPublicHandler) Decision(c *gin.Context) {
	var body struct {
		BannerID  string `json:"banner_id"`
		Action    string `json:"action"`
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
	if body.BannerID == "" || body.Action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "banner_id and action are required"})
		return
	}

	b, err := h.bannerRepo.GetBanner(body.BannerID)
	if err != nil || b == nil || b.ContextID != c.Param("contextID") {
		c.JSON(http.StatusNotFound, gin.H{"error": "banner not found"})
		return
	}

	if err := h.bannerRepo.RecordDecision(&bdomain.Decision{
		ContextID: c.Param("contextID"),
		BannerID:  body.BannerID,
		VisitorID: body.VisitorID,
		Action:    body.Action,
		IPHash:    ipHash(c.ClientIP()),
		UserAgent: c.GetHeader("User-Agent"),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record decision"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ServeSnippet responds with the self-contained banner snippet JavaScript
// for a context, for embedding in customer sites as
// <script src="https://tracking.../banner/snippet/<ctx>.js"></script>.
// The :contextID.js route captures the whole segment, so the context ID is
// sliced from the path (same pattern as snippet_handler.go).
func (h *BannerPublicHandler) ServeSnippet(c *gin.Context) {
	path := c.Request.URL.Path
	const prefix = "/banner/snippet/"
	if len(path) <= len(prefix)+3 || path[:len(prefix)] != prefix || !strings.HasSuffix(path, ".js") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	contextID := path[len(prefix) : len(path)-3]
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID required"})
		return
	}

	js, err := snippet.GenerateBannerConfigSnippet(contextID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", []byte(js))
}

// ipHash returns the hex-encoded SHA-256 of an IP address.
func ipHash(ip string) string {
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:])
}
