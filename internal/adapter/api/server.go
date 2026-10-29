package api

import (
	"net/http"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/adapter/api/handler"
	"github.com/h2hsecure/automata/internal/adapter/api/middleware"
	"github.com/h2hsecure/automata/internal/adapter/api/swagger"
	mcpadapter "github.com/h2hsecure/automata/internal/adapter/mcp"
	"github.com/h2hsecure/automata/internal/adapter/proxy"
	"github.com/h2hsecure/automata/internal/domain/auth"
	"github.com/h2hsecure/automata/internal/domain/context"
	infraauth "github.com/h2hsecure/automata/internal/infrastructure/auth"
	auth_repo "github.com/h2hsecure/automata/internal/infrastructure/auth/repo"
	bannerRepo "github.com/h2hsecure/automata/internal/infrastructure/banner/repo"
	config_repo "github.com/h2hsecure/automata/internal/infrastructure/config/repo"
	crepo "github.com/h2hsecure/automata/internal/infrastructure/contact/repo"
	email_repo "github.com/h2hsecure/automata/internal/infrastructure/email/repo"
	formRepo "github.com/h2hsecure/automata/internal/infrastructure/form/repo"
	"github.com/h2hsecure/automata/internal/infrastructure/queue"
	settingsrepo "github.com/h2hsecure/automata/internal/infrastructure/settings/repo"
	trackingRepo "github.com/h2hsecure/automata/internal/infrastructure/tracking/repo"
	webhook_svc "github.com/h2hsecure/automata/internal/infrastructure/webhook"
	webhook_repo "github.com/h2hsecure/automata/internal/infrastructure/webhook/repo"
	cookie "github.com/h2hsecure/automata/internal/infrastructure/webui/cookie"
	emailusecase "github.com/h2hsecure/automata/internal/usecase/email"
	bannerusecase "github.com/h2hsecure/automata/internal/usecase/banner"
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

	// Compress responses for clients that accept gzip (JSON, JS, HTML).
	parent.Use(gzip.Gzip(gzip.DefaultCompression))

	api := parent.Group(prefix)

	api.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// OpenAPI docs (hand-written, embedded; auth: JWT or API key).
	// Registered before ContextResolver (below) so no context is required.
	{
		api.GET("/swagger.yaml", middleware.AuthMiddleware(authService), func(c *gin.Context) {
			c.Data(http.StatusOK, "application/yaml", swagger.OpenAPIMain)
		})
		api.GET("/swagger-public.yaml", middleware.AuthMiddleware(authService), func(c *gin.Context) {
			c.Data(http.StatusOK, "application/yaml", swagger.OpenAPIPublic)
		})
	}

	cookieMgr := cookie.New(infraauth.GetCookieSecret())
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
		contextHandler := handler.NewContextHandler(contextRepo, userRepo)
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

		// Global settings routes
		settingsRepo := settingsrepo.New(pool)
		settingsHandler := handler.NewSettingsHandler(settingsRepo)
		admin.GET("/settings", settingsHandler.List)
		admin.PUT("/settings", settingsHandler.Upsert)
		admin.DELETE("/settings/:key", settingsHandler.Delete)

		// Per-context settings routes
		contextSettingsRepo := settingsrepo.NewContext(pool)
		contextSettingsHandler := handler.NewContextSettingsHandler(contextSettingsRepo)
		admin.GET("/contexts/:id/settings", contextSettingsHandler.List)
		admin.PUT("/contexts/:id/settings", contextSettingsHandler.Upsert)
		admin.DELETE("/contexts/:id/settings/:key", contextSettingsHandler.Delete)

		// Per-context email queue routes
		emailQueueHandler := handler.NewEmailQueueHandler(queue.NewEmailQueue(pool))
		admin.GET("/contexts/:id/email-queue", emailQueueHandler.List)
		admin.POST("/contexts/:id/email-queue/:jobId/retry", emailQueueHandler.Retry)
		admin.DELETE("/contexts/:id/email-queue/:jobId", emailQueueHandler.Cancel)
		admin.POST("/contexts/:id/email-queue/test", emailQueueHandler.TestSend)

		// Tracking visitor routes
		trackingRepo := trackingRepo.New(pool)
		trackingHandler := handler.NewTrackingHandler(trackingRepo)
		admin.GET("/tracking/visitors", trackingHandler.ListVisitors)
		admin.GET("/tracking/visitors/:id", trackingHandler.GetVisitorDetail)
		admin.GET("/tracking/visitors/:id/events", trackingHandler.GetVisitorEvents)
	}

	// Email template routes (declared outside admin block for context route access)
	contactRepo := crepo.New(pool)
	emailRepo := email_repo.New(pool)
	emailQueue := queue.NewEmailQueue(pool)
	emailUsecase := emailusecase.NewEmailUsecaseWithContact(emailRepo, emailQueue, contactRepo)
	emailHandler := handler.NewEmailHandlerWithInbox(emailUsecase, email_repo.NewInboxRepo(pool))

	// Form, tracking, and banner repos (shared by context routes and MCP).
	formRepo := formRepo.New(pool)
	trackingRepo := trackingRepo.New(pool)
	bannerRepo := bannerRepo.New(pool)

	admin.Group("/email-templates").Use(middleware.AuthMiddleware(authService)).GET("", emailHandler.List)
	admin.Group("/email-templates").Use(middleware.AuthMiddleware(authService)).POST("", emailHandler.Create)
	admin.Group("/email-templates").Use(middleware.AuthMiddleware(authService)).GET("/:id", emailHandler.Get)
	admin.Group("/email-templates").Use(middleware.AuthMiddleware(authService)).PUT("/:id", emailHandler.Update)
	admin.Group("/email-templates").Use(middleware.AuthMiddleware(authService)).DELETE("/:id", emailHandler.Delete)
	admin.Group("/email-templates").Use(middleware.AuthMiddleware(authService)).POST("/:id/test", emailHandler.TestSend)
	admin.POST("/contexts/:id/email-templates/:tid/send", middleware.AuthMiddleware(authService), emailHandler.SendToContact)
	admin.GET("/contexts/:id/inbox", middleware.AuthMiddleware(authService), emailHandler.ListInbox)
	admin.POST("/contexts/:id/inbox/:mid/read", middleware.AuthMiddleware(authService), emailHandler.MarkInboxRead)

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

	// MCP server (streamable HTTP, stateless). Registered before
	// ContextResolver so it never requires a context header.
	mcpDeps := mcpadapter.Deps{
		AuthService:   authService,
		ContactRepo:   contactRepo,
		FormRepo:      formRepo,
		BannerUsecase: bannerusecase.NewBannerUsecase(bannerRepo),
		TrackingRepo:  trackingRepo,
		EmailUsecase:  emailUsecase,
		WebhookSvc:    webhookSvc,
	}
	api.POST("/mcp", gin.WrapH(mcpadapter.NewHandler(mcpDeps)))
	api.GET("/mcp-skill.md", middleware.AuthMiddleware(authService), func(c *gin.Context) {
		c.Data(http.StatusOK, "text/markdown", []byte(mcpadapter.SkillMD))
	})

	// Context-resolved routes (forms, snippets, static assets)
	api.Use(ContextResolver(contextRepo))
	contextEmailGroup := api.Group("/context/:slug/email")
	{
		contextEmailGroup.POST("/send", emailHandler.Send)
	}
	api.GET("/context/email-stats", middleware.AuthMiddleware(authService), emailHandler.GetEmailStats)

	contactHandler := handler.NewContactHandler(contactRepo)
	{
		api.GET("/context/contacts", middleware.AuthMiddleware(authService), contactHandler.List)
		api.GET("/context/contacts/export", middleware.AuthMiddleware(authService), contactHandler.Export)
		api.POST("/context/contacts/import", middleware.AuthMiddleware(authService), contactHandler.Import)
		api.GET("/context/contacts/tags", middleware.AuthMiddleware(authService), contactHandler.ListTags)
		api.POST("/context/contacts/tags", middleware.AuthMiddleware(authService), contactHandler.CreateTag)
		api.DELETE("/context/contacts/tags/:id", middleware.AuthMiddleware(authService), contactHandler.DeleteTag)
		api.GET("/context/contacts/:id", middleware.AuthMiddleware(authService), contactHandler.Get)
		api.POST("/context/contacts", middleware.AuthMiddleware(authService), contactHandler.Create)
		api.PUT("/context/contacts/:id", middleware.AuthMiddleware(authService), contactHandler.Update)
		api.DELETE("/context/contacts/:id", middleware.AuthMiddleware(authService), contactHandler.Delete)
		api.POST("/context/contacts/:id/merge", middleware.AuthMiddleware(authService), contactHandler.Merge)
		api.GET("/context/contacts/:id/activity", middleware.AuthMiddleware(authService), contactHandler.GetActivity)
	}

	// Form routes (context-resolved)
	formHandler := handler.NewFormHandlerWithEmailAndWebhooks(formRepo, contactRepo, trackingRepo, emailUsecase, webhookSvc)
	{
		api.GET("/context/forms", middleware.AuthMiddleware(authService), formHandler.List)
		api.GET("/context/forms/:id", middleware.AuthMiddleware(authService), formHandler.Get)
		api.POST("/context/forms", middleware.AuthMiddleware(authService), formHandler.Create)
		api.PUT("/context/forms/:id", middleware.AuthMiddleware(authService), formHandler.Update)
		api.DELETE("/context/forms/:id", middleware.AuthMiddleware(authService), formHandler.Delete)
		api.GET("/context/forms/:id/submissions", middleware.AuthMiddleware(authService), formHandler.ListSubmissions)
		api.GET("/context/forms/:id/submissions/:sid", middleware.AuthMiddleware(authService), formHandler.GetSubmission)
		api.DELETE("/context/forms/:id/submissions/:sid", middleware.AuthMiddleware(authService), formHandler.DeleteSubmission)
	}

	// Banner routes (context-resolved)
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
	trackingHandler := handler.NewTrackingHandler(trackingRepo)
	{
		api.GET("/context/tracking/dashboard", middleware.AuthMiddleware(authService), trackingHandler.GetDashboard)
		api.GET("/context/tracking/events", middleware.AuthMiddleware(authService), trackingHandler.GetEvents)
		api.GET("/context/tracking/visitors", middleware.AuthMiddleware(authService), trackingHandler.ListVisitors)
		api.GET("/context/tracking/visitors/:id/events", middleware.AuthMiddleware(authService), trackingHandler.GetVisitorEvents)
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
