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
	emailusecase "github.com/hekemen/automata/internal/usecase/email"
	"github.com/hekemen/automata/internal/infrastructure/config"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
	config_repo "github.com/hekemen/automata/internal/infrastructure/config/repo"
	crepo "github.com/hekemen/automata/internal/infrastructure/contact/repo"
	email_repo "github.com/hekemen/automata/internal/infrastructure/email/repo"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	webhook_svc "github.com/hekemen/automata/internal/infrastructure/webhook"
	webhook_repo "github.com/hekemen/automata/internal/infrastructure/webhook/repo"
	tracking_repo "github.com/hekemen/automata/internal/infrastructure/tracking/repo"
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
	api.PUT("/auth/profile", middleware.AuthMiddleware(authService), authHandler.UpdateProfile)
	api.POST("/auth/password", middleware.AuthMiddleware(authService), authHandler.ChangePassword)
	api.POST("/auth/logout-all", middleware.AuthMiddleware(authService), authHandler.LogoutAll)

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
		trackingRepo := tracking_repo.New(pool)
		trackingHandler := handler.NewTrackingHandler(trackingRepo)
		admin.GET("/tracking/visitors", trackingHandler.ListVisitors)
		admin.GET("/tracking/visitors/:id", trackingHandler.GetVisitorDetail)
		admin.GET("/tracking/visitors/:id/events", trackingHandler.GetVisitorEvents)
	}

	// Email template routes (declared outside admin block for tenant route access)
	emailRepo := email_repo.New(pool)
	emailQueue := queue.NewEmailQueue(pool)
	emailUsecase := emailusecase.NewEmailUsecase(emailRepo, emailQueue)
	emailHandler := handler.NewEmailHandler(emailUsecase)

	admin.Group("/email-templates").Use(middleware.AuthMiddleware(authService)).GET("", emailHandler.List)
	admin.Group("/email-templates").Use(middleware.AuthMiddleware(authService)).POST("", emailHandler.Create)
	admin.Group("/email-templates").Use(middleware.AuthMiddleware(authService)).GET("/:id", emailHandler.Get)
	admin.Group("/email-templates").Use(middleware.AuthMiddleware(authService)).PUT("/:id", emailHandler.Update)
	admin.Group("/email-templates").Use(middleware.AuthMiddleware(authService)).DELETE("/:id", emailHandler.Delete)
	admin.Group("/email-templates").Use(middleware.AuthMiddleware(authService)).POST("/:id/test", emailHandler.TestSend)

	// Tracking visitor admin routes (already above)

	// Webhook management routes
	webhookSvc := webhook_svc.NewService(webhook_repo.New(pool), queue.NewWebhookQueue(pool))
	webhookHandler := handler.NewWebhookHandler(webhookSvc)
	admin.GET("/webhooks", webhookHandler.List)
	admin.POST("/webhooks", webhookHandler.Create)
	admin.GET("/webhooks/:id", webhookHandler.Get)
	admin.PUT("/webhooks/:id", webhookHandler.Update)
	admin.DELETE("/webhooks/:id", webhookHandler.Delete)
	admin.POST("/webhooks/:id/test", webhookHandler.TestDeliver)

	// Context-resolved routes (forms, snippets, static assets)
	api.Use(ContextResolver(contextRepo))
	tenantEmailGroup := api.Group("/tenant/:slug/email")
	{
		tenantEmailGroup.POST("/send", emailHandler.Send)
	}

	contactRepo := crepo.New(pool)
	contactHandler := handler.NewContactHandler(contactRepo)
	{
		api.GET("/context/contacts", middleware.AuthMiddleware(authService), contactHandler.List)
		api.GET("/context/contacts/:id", middleware.AuthMiddleware(authService), contactHandler.Get)
		api.POST("/context/contacts", middleware.AuthMiddleware(authService), contactHandler.Create)
		api.PUT("/context/contacts/:id", middleware.AuthMiddleware(authService), contactHandler.Update)
		api.DELETE("/context/contacts/:id", middleware.AuthMiddleware(authService), contactHandler.Delete)
		api.POST("/context/contacts/:id/merge", middleware.AuthMiddleware(authService), contactHandler.Merge)
		api.GET("/context/contacts/:id/activity", middleware.AuthMiddleware(authService), contactHandler.GetActivity)
	}
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
