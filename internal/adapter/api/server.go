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
	formRepo "github.com/hekemen/automata/internal/infrastructure/form/repo"
	bannerRepo "github.com/hekemen/automata/internal/infrastructure/banner/repo"
	trackingRepo "github.com/hekemen/automata/internal/infrastructure/tracking/repo"
	emailusecase "github.com/hekemen/automata/internal/usecase/email"
	"github.com/hekemen/automata/internal/infrastructure/config"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
	config_repo "github.com/hekemen/automata/internal/infrastructure/config/repo"
	crepo "github.com/hekemen/automata/internal/infrastructure/contact/repo"
	email_repo "github.com/hekemen/automata/internal/infrastructure/email/repo"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	webhook_svc "github.com/hekemen/automata/internal/infrastructure/webhook"
	webhook_repo "github.com/hekemen/automata/internal/infrastructure/webhook/repo"
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

	cookieSecret := config.Get("auth.cookie_secret")
	if cookieSecret == "" {
		cookieSecret = "automata-cookie-secret-32bytes!!" // 32 bytes for AES-256
	}
	cookieMgr := cookie.New(cookieSecret)
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
		trackingRepo := trackingRepo.New(pool)
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

	// Form routes (context-resolved)
	formRepo := formRepo.New(pool)
	formHandler := handler.NewFormHandler(formRepo)
	{
		api.GET("/context/forms", middleware.AuthMiddleware(authService), formHandler.List)
		api.GET("/context/forms/:id", middleware.AuthMiddleware(authService), formHandler.Get)
		api.POST("/context/forms", middleware.AuthMiddleware(authService), formHandler.Create)
		api.PUT("/context/forms/:id", middleware.AuthMiddleware(authService), formHandler.Update)
		api.DELETE("/context/forms/:id", middleware.AuthMiddleware(authService), formHandler.Delete)
		api.POST("/context/forms/:id/submit", formHandler.SubmitForm)
		api.GET("/context/forms/:id/submissions", middleware.AuthMiddleware(authService), formHandler.ListSubmissions)
		api.GET("/context/forms/:id/submissions/:sid", middleware.AuthMiddleware(authService), formHandler.GetSubmission)
	}

	// Banner routes (context-resolved)
	bannerRepo := bannerRepo.New(pool)
	bannerHandler := handler.NewBannerHandler(bannerRepo)
	placementHandler := handler.NewPlacementHandler(bannerRepo)
	campaignHandler := handler.NewCampaignHandler(bannerRepo)
	{
		api.GET("/context/banners", middleware.AuthMiddleware(authService), bannerHandler.ListBanners)
		api.GET("/context/banners/:id", middleware.AuthMiddleware(authService), bannerHandler.GetBanner)
		api.POST("/context/banners", middleware.AuthMiddleware(authService), bannerHandler.CreateBanner)
		api.PUT("/context/banners/:id", middleware.AuthMiddleware(authService), bannerHandler.UpdateBanner)
		api.DELETE("/context/banners/:id", middleware.AuthMiddleware(authService), bannerHandler.DeleteBanner)
		api.GET("/context/placements", middleware.AuthMiddleware(authService), placementHandler.ListPlacements)
		api.POST("/context/placements", middleware.AuthMiddleware(authService), placementHandler.CreatePlacement)
		api.GET("/context/placements/:id", middleware.AuthMiddleware(authService), placementHandler.GetPlacement)
		api.PUT("/context/placements/:id", middleware.AuthMiddleware(authService), placementHandler.UpdatePlacement)
		api.DELETE("/context/placements/:id", middleware.AuthMiddleware(authService), placementHandler.DeletePlacement)
		api.GET("/context/campaigns", middleware.AuthMiddleware(authService), campaignHandler.ListCampaigns)
		api.POST("/context/campaigns", middleware.AuthMiddleware(authService), campaignHandler.CreateCampaign)
		api.GET("/context/campaigns/:id", middleware.AuthMiddleware(authService), campaignHandler.GetCampaign)
		api.PUT("/context/campaigns/:id", middleware.AuthMiddleware(authService), campaignHandler.UpdateCampaign)
		api.DELETE("/context/campaigns/:id", middleware.AuthMiddleware(authService), campaignHandler.DeleteCampaign)
	}

	// Tracking dashboard routes (context-resolved)
	trackingHandler := handler.NewTrackingHandler(trackingRepo.New(pool))
	{
		api.GET("/context/tracking/dashboard", middleware.AuthMiddleware(authService), trackingHandler.GetDashboard)
		api.GET("/context/tracking/events", middleware.AuthMiddleware(authService), trackingHandler.GetEvents)
	}

	// API keys list/revoke (admin routes)
	{
		api.GET("/admin/api-keys", middleware.AuthMiddleware(authService), authHandler.ListAPIKeys)
		api.DELETE("/admin/api-keys/:id", middleware.AuthMiddleware(authService), authHandler.RevokeAPIKey)
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
