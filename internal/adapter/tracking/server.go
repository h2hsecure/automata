package tracking

import (
	"net/http"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/adapter/api/handler"
	"github.com/h2hsecure/automata/internal/adapter/api/middleware"
	geodomain "github.com/h2hsecure/automata/internal/domain/geo"
	"github.com/h2hsecure/automata/internal/infrastructure/config"
)

// trackLimiter is the shared per-IP rate limiter for all public tracking
// endpoints on one engine (in-process, per-pod — spec 4.2, §9).
var trackLimiter = middleware.NewLimiter()

// publicRateLimit builds the shared per-IP rate-limit middleware for public
// tracking endpoints (rate_tracking_per_min, 300/min default).
func publicRateLimit() gin.HandlerFunc {
	return middleware.RateLimit(trackLimiter, config.GetInt("rate_tracking_per_min", 300), time.Minute, middleware.IPKeyFn)
}

// NewServer creates a new tracking server with public-facing endpoints.
// The geoLookup resolves visitor locations offline (nil disables
// geolocation). The optional contactRepo enables hidden visitor contacts and
// page_visit activity logging; servers built without it (e.g. test harnesses)
// skip them.
func NewServer(formHandler *handler.FormHandler, trackingRepo Repository, geoLookup geodomain.Lookup, contactRepo ...ContactRepository) *gin.Engine {
	r := gin.Default()

	// Compress responses for clients that accept gzip (JSON, JS, HTML).
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	// Cap request bodies on public endpoints (max_body_bytes, 10 MB default).
	r.Use(middleware.LimitBody(int64(config.GetInt("max_body_bytes", 10*1024*1024))))

	trackHandler := &TrackHandler{repo: trackingRepo, geoLookup: geoLookup}
	if len(contactRepo) > 0 && contactRepo[0] != nil {
		trackHandler.contactRepo = contactRepo[0]
	}
	snippetHandler := &SnippetHandler{}

	// Public GET routes are also registered for HEAD (curl -I, health checks,
	// proxies): gin >= 1.12 no longer auto-maps HEAD to GET, so a GET-only
	// route 404s HEAD requests.
	// Tracking endpoints (public, per-IP rate limited: rate_tracking_per_min)
	r.POST("/track", publicRateLimit(), trackHandler.Track)
	r.POST("/track/:cid", publicRateLimit(), trackHandler.TrackScoped)
	r.POST("/track/batch", publicRateLimit(), trackHandler.TrackBatch)
	r.GET("/snippet/:id.js", snippetHandler.ServeSnippet)
	r.HEAD("/snippet/:id.js", snippetHandler.ServeSnippet)

	// Form endpoints (context via X-Context-ID header or context_id query)
	formGroup := r.Group("/form")
	formGroup.Use(ContextMiddleware())
	formGroup.GET("/list", formHandler.List)
	formGroup.HEAD("/list", formHandler.List)
	formGroup.GET("/:id", formHandler.Get)
	formGroup.HEAD("/:id", formHandler.Get)
	formGroup.POST("/submit/:slug", formHandler.SubmitForm)
	formGroup.GET("/:id/submissions", formHandler.ListSubmissions)
	formGroup.HEAD("/:id/submissions", formHandler.ListSubmissions)

	// Public form definition + self-rendering snippet (context in path, no auth).
	// Registered only when a form handler is provided — some test harnesses
	// construct the tracking server with a nil form handler.
	if formHandler != nil {
		formSnippetHandler := &FormSnippetHandler{repo: formHandler.Repo()}
		r.GET("/form/definition/:contextID/:slug", formSnippetHandler.ServeDefinition)
		r.HEAD("/form/definition/:contextID/:slug", formSnippetHandler.ServeDefinition)
		r.GET("/form/snippet/:contextID/:slug", formSnippetHandler.ServeFormSnippet)
		r.HEAD("/form/snippet/:contextID/:slug", formSnippetHandler.ServeFormSnippet)
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.HEAD("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}
