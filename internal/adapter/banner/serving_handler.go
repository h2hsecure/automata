package banner

import (
	"net/http"

	"github.com/gin-gonic/gin"
	bdomain "github.com/hekemen/automata/internal/domain/banner"
	bannerUsecase "github.com/hekemen/automata/internal/usecase/banner"
)

// ServingHandler handles banner serving endpoints.
type ServingHandler struct {
	repo Repository
}

// Repository is the interface for banner data persistence.
type Repository interface {
	bdomain.Repository
}

// PlacementBanner represents a placement with its banners.
type PlacementBanner struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	CSSSelector string                  `json:"css_selector"`
	Banners     []*bdomain.Banner       `json:"banners"`
}

// PlacementsResponse represents the full placements response.
type PlacementsResponse struct {
	Placements []PlacementBanner `json:"placements"`
}

// GetPlacements handles GET /api/banners/placements — returns placements with active banners.
func (h *ServingHandler) GetPlacements(c *gin.Context) {
	contextID := ResolveContext(c)
	if contextID == "" {
		return
	}

	placements, err := bannerUsecase.NewBannerUsecase(h.repo).ListPlacements(contextID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := PlacementsResponse{
		Placements: make([]PlacementBanner, 0, len(placements)),
	}

	for _, p := range placements {
		opts := bdomain.ListOptions{
			IsActive: true,
		}
		banners, _, err := bannerUsecase.NewBannerUsecase(h.repo).ListBanners(contextID, opts)
		if err != nil {
			continue
		}

		response.Placements = append(response.Placements, PlacementBanner{
			ID:          p.ID,
			Name:        p.Name,
			CSSSelector: p.CSSSelector,
			Banners:     banners,
		})
	}

	c.JSON(http.StatusOK, response)
}

// TrackBanner handles GET /track/banner — tracks banner impressions and clicks.
func (h *ServingHandler) TrackBanner(c *gin.Context) {
	contextID := ResolveContext(c)
	if contextID == "" {
		return
	}

	bannerID := c.Query("banner_id")
	visitorID := c.Query("visitor_id")
	btype := c.Query("type")

	if bannerID == "" || visitorID == "" || btype == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "banner_id, visitor_id, and type are required"})
		return
	}

	switch btype {
	case "impression":
		if err := bannerUsecase.NewBannerUsecase(h.repo).RecordImpression(bannerID, visitorID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	case "click":
		if err := bannerUsecase.NewBannerUsecase(h.repo).RecordClick(bannerID, visitorID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'impression' or 'click'"})
		return
	}

	// Return 1x1 transparent PNG pixel
	pixel := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
		0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}

	c.Data(http.StatusOK, "image/png", pixel)
}
