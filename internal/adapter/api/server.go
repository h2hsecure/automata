package api

import (
	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/adapter/api/handler"
	"github.com/hekemen/automata/internal/adapter/api/middleware"
	"github.com/hekemen/automata/internal/adapter/proxy"
	"github.com/hekemen/automata/internal/domain/auth"
	"github.com/hekemen/automata/internal/domain/tenant"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
)

// NewServer creates and configures the API server.
func NewServer(
	tenantRepo tenant.Repository,
	userRepo tenant.UserRepository,
	authService auth.AuthService,
	apiKeyRepo auth_repo.ApiKeyRepository,
) *gin.Engine {
	engine := gin.Default()

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	authHandler := handler.NewAuthHandler(authService, userRepo, tenantRepo, apiKeyRepo)
	engine.POST("/auth/login", authHandler.Login)
	engine.POST("/auth/logout", authHandler.Logout)

	admin := engine.Group("/admin")
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
	}

	engine.Use(TenantResolver(tenantRepo))

	proxy.NewProxy(tenantRepo, engine)

	return engine
}
