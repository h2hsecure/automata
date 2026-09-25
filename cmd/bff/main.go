package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	auth "github.com/hekemen/automata/internal/infrastructure/auth"
	banner_repo "github.com/hekemen/automata/internal/infrastructure/banner/repo"
	"github.com/hekemen/automata/internal/infrastructure/bootstrap"
	"github.com/hekemen/automata/internal/infrastructure/config"
	"github.com/hekemen/automata/internal/infrastructure/contact"
	"github.com/hekemen/automata/internal/infrastructure/database"
	ctx_repo "github.com/hekemen/automata/internal/infrastructure/context/repo"
	email_repo "github.com/hekemen/automata/internal/infrastructure/email/repo"
	form_repo "github.com/hekemen/automata/internal/infrastructure/form/repo"
	webhook_repo "github.com/hekemen/automata/internal/infrastructure/webhook/repo"
	tracking_repo "github.com/hekemen/automata/internal/infrastructure/tracking/repo"
	webui_handler "github.com/hekemen/automata/internal/adapter/webui/handler"
	webui_cookie "github.com/hekemen/automata/internal/infrastructure/webui/cookie"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	configPath := config.Get("config.file")
	if configPath == "" {
		configPath = "config.yaml"
	}
	if err := config.Load(configPath); err != nil {
		log.Warn().Err(err).Str("path", configPath).Msg("failed to load config, using defaults")
	}

	port := config.Get("bff.port")
	if port == "" {
		port = "8082"
	}
	cookieSecret := config.Get("bff.cookie_secret")
	if cookieSecret == "" {
		cookieSecret = "change-me-in-production"
	}
	apiBackendURL := config.Get("bff.api_backend_url")
	if apiBackendURL == "" {
		apiBackendURL = "http://localhost:8080"
	}
	webDir := config.Get("bff.web_dir")
	if webDir == "" {
		// Try relative to executable, then current directory
		execPath, _ := os.Executable()
		execDir := filepath.Dir(execPath)
		candidates := []string{
			filepath.Join(execDir, "web"),
			"web",
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				webDir = candidate
				break
			}
		}
		if webDir == "" {
			log.Fatal().Msg("web directory not found")
		}
	}

	// Initialize database connection
	dbPool, err := database.NewPool()
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer dbPool.Close()

	// Run migrations
	if err := database.RunMigrations(dbPool); err != nil {
		log.Fatal().Err(err).Msg("migration failed")
	}
	if err := database.RunUserContextMigrations(dbPool); err != nil {
		log.Fatal().Err(err).Msg("user context migration failed")
	}

	// Run feature-specific migrations
	contact.RunMigrations(dbPool)
	form_repo.RunMigrations(dbPool)
	banner_repo.RunMigrations(dbPool)
	tracking_repo.RunMigrations(dbPool)
	email_repo.RunMigrations(dbPool)
	webhook_repo.RunMigrations(dbPool)

	// Initialize repositories
	userRepo := ctx_repo.NewUserPostgresRepo(dbPool)
	contextRepo := ctx_repo.NewPostgresRepo(dbPool)

	// Admin bootstrap
	bootstrap.Run(dbPool, userRepo, contextRepo)

	// Initialize services
	authService := auth.NewService(userRepo)

	// Create session manager
	sessionManager := webui_cookie.New(cookieSecret)

	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health"},
	}))

	// Frontend handler
	frontendHandler := webui_handler.NewFrontendHandler(webDir)

	// Auth handler
	authHandler := webui_handler.NewAuthHandler(authService, userRepo, sessionManager)

	// Proxy handler
	proxyHandler := webui_handler.NewProxyHandler(apiBackendURL, sessionManager)

	// Register routes
	r.GET("/", frontendHandler.HandleIndex)
	r.GET("/login", frontendHandler.HandleLogin)
	r.POST("/login", authHandler.HandleLoginPost)
	r.POST("/logout", authHandler.HandleLogout)
	r.GET("/dashboard", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/")
	})
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			proxyHandler.HandleProxy(c)
		} else {
			frontendHandler.HandleSPA(c)
		}
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Start server
	log.Info().Str("port", port).Msg("starting BFF server")
	if err := r.Run(":" + port); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}
