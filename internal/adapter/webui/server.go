package webui

import (
	"github.com/gin-gonic/gin"
	infraauth "github.com/hekemen/automata/internal/infrastructure/auth"
)

// RegisterRoutes registers BFF routes on the given Gin engine.
// This is a convenience function that can be used to set up the BFF server
// as a standalone component (not recommended — use cmd/bff/main.go for the
// integrated dual-server setup).
func RegisterRoutes(engine *gin.Engine) {
	_ = infraauth.GetCookieSecret() // ensures secrets are initialized

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Note: The full BFF server setup should be done in cmd/bff/main.go
	// which handles the dual-server architecture (admin + tracking + BFF).
}

// NewServer creates a standalone BFF server engine with basic routes.
// This is intended for testing and standalone use.
func NewServer() *gin.Engine {
	engine := gin.Default()
	RegisterRoutes(engine)
	return engine
}
