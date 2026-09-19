package tracking

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/adapter/api/handler"
)

// NewServer creates a new tracking server with public-facing endpoints.
func NewServer(formHandler *handler.FormHandler, bannerHandler *handler.BannerHandler, placementHandler *handler.PlacementHandler, campaignHandler *handler.CampaignHandler, trackingRepo Repository) *gin.Engine {
	r := gin.Default()

	trackHandler := &TrackHandler{repo: trackingRepo}
	snippetHandler := &SnippetHandler{}

	// Tracking endpoints
	r.POST("/track", trackHandler.Track)
	r.POST("/track/batch", trackHandler.TrackBatch)
	r.GET("/snippet/:id.js", snippetHandler.ServeSnippet)

	// Banner endpoints
	r.GET("/track/banner/list", bannerHandler.ListBanners)
	r.GET("/track/banner/:id", bannerHandler.GetBanner)
	r.GET("/track/placement/:id", placementHandler.GetPlacement)
	r.GET("/track/campaign/:id", campaignHandler.GetCampaign)

	// Form endpoints
	r.GET("/form/list", formHandler.List)
	r.GET("/form/:id", formHandler.Get)
	r.POST("/form/submit/:slug", formHandler.SubmitForm)
	r.GET("/form/:id/submissions", formHandler.ListSubmissions)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}
