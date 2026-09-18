package tracking

import (
	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/tracking"
)

// NewTrackingServer creates a Gin engine with tracking routes registered.
func NewTrackingServer(repo Repository) *gin.Engine {
	engine := gin.Default()

	trackHandler := &TrackHandler{repo: repo}
	snippetHandler := &SnippetHandler{}

	engine.POST("/track", trackHandler.Track)
	engine.POST("/track/batch", trackHandler.TrackBatch)
	engine.GET("/snippet/:id.js", snippetHandler.ServeSnippet)

	return engine
}

// NewTrackingServerWithRepo creates a Gin engine with the domain tracking.Repository.
func NewTrackingServerWithRepo(repo tracking.Repository) *gin.Engine {
	return NewTrackingServer(repo)
}
