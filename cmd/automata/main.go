package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/adapter/api"
	"github.com/hekemen/automata/internal/adapter/api/handler"
	"github.com/hekemen/automata/internal/adapter/tracking"
	infraauth "github.com/hekemen/automata/internal/infrastructure/auth"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
	banner_repo "github.com/hekemen/automata/internal/infrastructure/banner/repo"
	"github.com/hekemen/automata/internal/infrastructure/config"
	"github.com/hekemen/automata/internal/infrastructure/bootstrap"
	"github.com/hekemen/automata/internal/infrastructure/contact"
	"github.com/hekemen/automata/internal/infrastructure/database"
	context_repo "github.com/hekemen/automata/internal/infrastructure/context/repo"
	email_repo "github.com/hekemen/automata/internal/infrastructure/email/repo"
	form_repo "github.com/hekemen/automata/internal/infrastructure/form/repo"
	webhook_repo "github.com/hekemen/automata/internal/infrastructure/webhook/repo"
	tracking_repo "github.com/hekemen/automata/internal/infrastructure/tracking/repo"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config.yaml"
	}
	if err := config.Load(configPath); err != nil {
		log.Warn().Err(err).Str("path", configPath).Msg("failed to load config, using defaults")
	}

	// Initialize and validate secret keys (panics if missing)
	infraauth.Init()

	log.Info().Str("db_host", config.Get("database.host")).Msg("loaded config")

	// Initialize database
	pool, err := database.NewPool()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer database.Close(pool)

	if err := database.RunMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run core migrations")
	}

	if err := tracking_repo.RunMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run tracking migrations")
	}

	if err := form_repo.RunMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run form migrations")
	}

	if err := contact.RunMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run contact migrations")
	}

	if err := banner_repo.RunMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run banner migrations")
	}

	if err := email_repo.RunMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run email migrations")
	}

	if err := webhook_repo.RunMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run webhook migrations")
	}

	if err := database.RunUserContextMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run user-context migrations")
	}

	// Verify database connection
	var contextCount int
	pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM contexts").Scan(&contextCount)
	log.Info().Int("contexts", contextCount).Msg("database connected")

	// Initialize repositories
	contextRepo := context_repo.NewPostgresRepo(pool)
	userRepo := context_repo.NewUserPostgresRepo(pool)
	apiKeyRepo := auth_repo.NewApiKeyPostgresRepo(pool)
	formRepo := form_repo.New(pool)
	bannerRepo := banner_repo.New(pool)
	trackingRepo := tracking_repo.New(pool)

	// Admin bootstrap: create admin user on first startup
	if err := bootstrap.Run(pool, userRepo, contextRepo); err != nil {
		log.Warn().Err(err).Msg("admin bootstrap failed (continuing anyway)")
	}

	// Seed default email templates for all existing contexts
	contexts, _ := contextRepo.List(0, 1000)
	for _, ctx := range contexts {
		if err := email_repo.SeedDefaultsForContext(pool, ctx.ID); err != nil {
			log.Warn().Err(err).Str("context_id", ctx.ID).Msg("failed to seed email templates")
		}
	}

	// Initialize auth service
	authService := infraauth.NewService(userRepo)

	// Create handlers for tracking server
	formHandler := handler.NewFormHandler(formRepo)
	bannerHandler := handler.NewBannerHandler(bannerRepo)
	placementHandler := handler.NewPlacementHandler(bannerRepo)
	campaignHandler := handler.NewCampaignHandler(bannerRepo)

	// Admin server (port 8080)
	adminEngine := gin.Default()

	// Health check at root level (before SPA fallback)
	adminEngine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api.NewServer(adminEngine, pool, contextRepo, userRepo, authService, apiKeyRepo, "/api")

	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "./web"
	}
	api.ServeStatic(adminEngine, webDir)

	adminPort := config.Get("server.port")
	if adminPort == "" {
		adminPort = "8080"
	}

	adminHost := config.Get("server.host")
	if adminHost == "" {
		adminHost = "0.0.0.0"
	}

	adminAddr := fmt.Sprintf("%s:%s", adminHost, adminPort)

	adminSrv := &http.Server{
		Addr:    adminAddr,
		Handler: adminEngine,
	}

	go func() {
		log.Info().Str("addr", adminAddr).Msg("starting admin server")
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("admin server failed")
		}
	}()

	// Tracking server (port 8081)
	trackingEngine := tracking.NewServer(formHandler, bannerHandler, placementHandler, campaignHandler, trackingRepo)

	trackingPort := config.Get("server.tracking_port")
	if trackingPort == "" {
		trackingPort = "8081"
	}

	trackingAddr := fmt.Sprintf(":%s", trackingPort)

	trackingSrv := &http.Server{
		Addr:    trackingAddr,
		Handler: trackingEngine,
	}

	go func() {
		log.Info().Str("addr", trackingAddr).Msg("starting tracking server")
		if err := trackingSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("tracking server failed")
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Info().Msg("shutting down...")
		cancel()
	}()

	<-ctx.Done()

	// Shutdown admin server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	log.Info().Msg("shutting down admin server...")
	adminSrv.Shutdown(shutdownCtx)

	log.Info().Msg("shutting down tracking server...")
	trackingSrv.Shutdown(shutdownCtx)

	log.Info().Msg("all servers stopped")
}
