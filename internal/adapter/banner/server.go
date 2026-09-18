package banner

import (
	"github.com/gin-gonic/gin"
	bdomain "github.com/hekemen/automata/internal/domain/banner"
)

// NewBannerServer creates a Gin engine with banner serving routes registered.
func NewBannerServer(repo Repository) *gin.Engine {
	engine := gin.Default()

	servingHandler := &ServingHandler{repo: repo}
	snippetHandler := &SnippetHandler{}

	engine.GET("/api/banners/placements", servingHandler.GetPlacements)
	engine.GET("/track/banner", servingHandler.TrackBanner)
	engine.GET("/snippet/banner/:id.js", snippetHandler.ServeBannerSnippet)

	return engine
}

// NewBannerServerWithRepo creates a Gin engine with the domain banner.Repository.
func NewBannerServerWithRepo(repo bdomain.Repository) *gin.Engine {
	return NewBannerServer(repo)
}
