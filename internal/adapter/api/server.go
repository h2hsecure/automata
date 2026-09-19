package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/adapter/api/handler"
	"github.com/hekemen/automata/internal/adapter/api/middleware"
	"github.com/hekemen/automata/internal/adapter/proxy"
	"github.com/hekemen/automata/internal/domain/auth"
	"github.com/hekemen/automata/internal/domain/tenant"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
	config_repo "github.com/hekemen/automata/internal/infrastructure/config/repo"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewServer creates and configures the API server, registering all routes
// under the given prefix (e.g. "/api"). It returns the parent gin.Engine
// so the caller can attach additional handlers (static files, etc.).
func NewServer(
	parent *gin.Engine,
	pool *pgxpool.Pool,
	tenantRepo tenant.Repository,
	userRepo tenant.UserRepository,
	authService auth.AuthService,
	apiKeyRepo auth_repo.ApiKeyRepository,
	prefix string,
) *gin.Engine {
	if prefix == "" {
		prefix = "/api"
	}

	api := parent.Group(prefix)

	api.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	authHandler := handler.NewAuthHandler(authService, userRepo, tenantRepo, apiKeyRepo)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/logout", authHandler.Logout)

	admin := api.Group("/admin")
	admin.Use(middleware.AuthMiddleware(authService))
	{
		tenantHandler := handler.NewTenantHandler(tenantRepo)

		tenantRoutes := admin.Group("/tenants")
		{
			tenantRoutes.GET("", tenantHandler.List)
			tenantRoutes.POST("", tenantHandler.Create)
			tenantRoutes.GET("/:id", tenantHandler.Get)
			tenantRoutes.PUT("/:id", tenantHandler.Update)
			tenantRoutes.DELETE("/:id", tenantHandler.Delete)
		}

		admin.POST("/api-keys", authHandler.CreateAPIKey)

		// Config routes
		configRepo := config_repo.NewConfigRepo(pool)
		configHandler := handler.NewConfigHandler(configRepo)
		admin.GET("/configs/:tenantId", configHandler.GetConfig)
		admin.PUT("/configs/:tenantId/cors", configHandler.UpdateCORS)
		admin.PUT("/configs/:tenantId/domain", configHandler.UpdateDomain)
		admin.PUT("/configs/:tenantId/display", configHandler.UpdateDisplay)
	}

	// Tenant-resolved routes (forms, snippets, static assets)
	api.Use(TenantResolver(tenantRepo))
	proxy.NewProxy(tenantRepo, api)

	return parent
}

// ServeStatic serves static files from the given directory at the root path
// and falls back to index.html for SPA routing.
func ServeStatic(engine *gin.Engine, dir string) {
	// Serve static assets (JS, CSS, images) BEFORE the SPA fallback
	assets := engine.Group("/assets")
	assets.StaticFS("/", http.Dir(dir+"/assets"))
	engine.StaticFS("/favicon.svg", http.Dir(dir))

	engine.GET("/", func(c *gin.Context) {
		c.File(dir + "/index.html")
	})
	engine.GET("/index.html", func(c *gin.Context) {
		c.File(dir + "/index.html")
	})
	engine.NoRoute(func(c *gin.Context) {
		// Let API routes under /api/* be handled by the router group
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		// SPA fallback: serve index.html for all other routes
		c.File(dir + "/index.html")
	})
}
