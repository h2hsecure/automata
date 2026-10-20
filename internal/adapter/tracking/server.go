package tracking

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/adapter/api/handler"
)

// NewServer creates a new tracking server with public-facing endpoints.
// The optional contactRepo enables hidden visitor contacts and page_visit
// activity logging; servers built without it (e.g. test harnesses) skip them.
func NewServer(formHandler *handler.FormHandler, bannerHandler *handler.BannerHandler, placementHandler *handler.PlacementHandler, campaignHandler *handler.CampaignHandler, trackingRepo Repository, contactRepo ...ContactRepository) *gin.Engine {
	r := gin.Default()

	trackHandler := &TrackHandler{repo: trackingRepo}
	if len(contactRepo) > 0 && contactRepo[0] != nil {
		trackHandler.contactRepo = contactRepo[0]
	}
	snippetHandler := &SnippetHandler{}

	// Tracking endpoints
	r.POST("/track", trackHandler.Track)
	r.POST("/track/batch", trackHandler.TrackBatch)
	r.GET("/snippet/:id.js", snippetHandler.ServeSnippet)

	// Banner endpoints
	r.GET("/track/banner/list", ContextMiddleware(), bannerHandler.ListBanners)
	r.GET("/track/banner/:id", bannerHandler.GetBanner)
	r.GET("/track/placement/:id", placementHandler.GetPlacement)
	r.GET("/track/campaign/:id", campaignHandler.GetCampaign)

	// Form endpoints (context via X-Context-ID header or context_id query)
	formGroup := r.Group("/form")
	formGroup.Use(ContextMiddleware())
	formGroup.GET("/list", formHandler.List)
	formGroup.GET("/:id", formHandler.Get)
	formGroup.POST("/submit/:slug", formHandler.SubmitForm)
	formGroup.GET("/:id/submissions", formHandler.ListSubmissions)

	// Public form definition + self-rendering snippet (context in path, no auth).
	// Registered only when a form handler is provided — some test harnesses
	// construct the tracking server with a nil form handler.
	if formHandler != nil {
		formSnippetHandler := &FormSnippetHandler{repo: formHandler.Repo()}
		r.GET("/form/definition/:contextID/:slug", formSnippetHandler.ServeDefinition)
		r.GET("/form/snippet/:contextID/:slug", formSnippetHandler.ServeFormSnippet)
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}
