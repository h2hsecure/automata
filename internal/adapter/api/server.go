package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/adapter/api/handler"
	"github.com/hekemen/automata/internal/adapter/api/middleware"
	"github.com/hekemen/automata/internal/adapter/proxy"
	"github.com/hekemen/automata/internal/domain/auth"
	"github.com/hekemen/automata/internal/domain/context"
	"github.com/hekemen/automata/internal/infrastructure/config"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
	config_repo "github.com/hekemen/automata/internal/infrastructure/config/repo"
	"github.com/hekemen/automata/internal/infrastructure/tracking/repo"
	cookie "github.com/hekemen/automata/internal/infrastructure/webui/cookie"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewServer creates and configures the API server, registering all routes
// under the given prefix (e.g. "/api"). It returns the parent gin.Engine
// so the caller can attach additional handlers (static files, etc.).
func NewServer(
	parent *gin.Engine,
	pool *pgxpool.Pool,
	contextRepo context.Repository,
	userRepo context.UserRepository,
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

	cookieMgr := cookie.New(config.Get("auth.secret_key"))
	authHandler := handler.NewAuthHandler(authService, userRepo, contextRepo, apiKeyRepo, cookieMgr)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/logout", authHandler.Logout)
	api.POST("/auth/refresh", authHandler.HandleRefresh)

	api.GET("/auth/me", middleware.AuthMiddleware(authService), authHandler.GetMe)

	admin := api.Group("/admin")
	admin.Use(middleware.AuthMiddleware(authService))
	{
		contextHandler := handler.NewContextHandler(contextRepo)
		userHandler := handler.NewUserHandler(userRepo, contextRepo)
		contextUserHandler := handler.NewContextUserHandler(userRepo, contextRepo)

		contextRoutes := admin.Group("/contexts")
		{
			contextRoutes.GET("", contextHandler.List)
			contextRoutes.POST("", contextHandler.Create)
			contextRoutes.GET("/:id", contextHandler.Get)
			contextRoutes.PUT("/:id", contextHandler.Update)
			contextRoutes.DELETE("/:id", contextHandler.Delete)
		}

		usersRoutes := admin.Group("/users")
		{
			usersRoutes.GET("", userHandler.List)
			usersRoutes.POST("", userHandler.Create)
			usersRoutes.GET("/:id", userHandler.Get)
			usersRoutes.PUT("/:id", userHandler.Update)
			usersRoutes.DELETE("/:id", userHandler.Delete)
		}

		contextUserRoutes := admin.Group("/contexts/:id/users")
		{
			contextUserRoutes.GET("", contextUserHandler.List)
			contextUserRoutes.POST("", contextUserHandler.Create)
			contextUserRoutes.DELETE("/:userId", contextUserHandler.Delete)
		}

		admin.POST("/api-keys", authHandler.CreateAPIKey)

		// Config routes
		configRepo := config_repo.NewConfigRepo(pool)
		configHandler := handler.NewConfigHandler(configRepo)
		admin.GET("/configs/:contextId", configHandler.GetConfig)
		admin.PUT("/configs/:contextId/cors", configHandler.UpdateCORS)
		admin.PUT("/configs/:contextId/domain", configHandler.UpdateDomain)
		admin.PUT("/configs/:contextId/display", configHandler.UpdateDisplay)

		// Tracking visitor routes
		trackingRepo := repo.New(pool)
		trackingHandler := handler.NewTrackingHandler(trackingRepo)
		admin.GET("/tracking/visitors", trackingHandler.ListVisitors)
		admin.GET("/tracking/visitors/:id", trackingHandler.GetVisitorDetail)
		admin.GET("/tracking/visitors/:id/events", trackingHandler.GetVisitorEvents)
	}

	// Context-resolved routes (forms, snippets, static assets)
	api.Use(ContextResolver(contextRepo))
	proxy.NewProxy(contextRepo, api)

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
