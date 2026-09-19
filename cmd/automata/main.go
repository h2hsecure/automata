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
	auth "github.com/hekemen/automata/internal/infrastructure/auth"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
	banner_repo "github.com/hekemen/automata/internal/infrastructure/banner/repo"
	"github.com/hekemen/automata/internal/infrastructure/config"
	"github.com/hekemen/automata/internal/infrastructure/database"
	form_repo "github.com/hekemen/automata/internal/infrastructure/form/repo"
	tenant_repo "github.com/hekemen/automata/internal/infrastructure/tenant/repo"
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

	log.Info().Str("db_host", config.Get("database.host")).Msg("loaded config")

	// Initialize database
	pool, err := database.NewPool()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer database.Close(pool)

	if err := database.RunMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run migrations")
	}

	// Verify database connection
	var tenantCount int
	pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM tenants").Scan(&tenantCount)
	log.Info().Int("tenants", tenantCount).Msg("database connected")

	// Initialize repositories
	tenantRepo := tenant_repo.NewPostgresRepo(pool)
	userRepo := tenant_repo.NewUserPostgresRepo(pool)
	apiKeyRepo := auth_repo.NewApiKeyPostgresRepo(pool)
	formRepo := form_repo.New(pool)
	bannerRepo := banner_repo.New(pool)
	trackingRepo := tracking_repo.New(pool)

	// Initialize auth service
	authService := auth.NewService(userRepo)

	// Create handlers for tracking server
	formHandler := handler.NewFormHandler(formRepo)
	bannerHandler := handler.NewBannerHandler(bannerRepo)
	placementHandler := handler.NewPlacementHandler(bannerRepo)
	campaignHandler := handler.NewCampaignHandler(bannerRepo)

	// Admin server (port 8080)
	adminEngine := gin.Default()
	api.NewServer(adminEngine, pool, tenantRepo, userRepo, authService, apiKeyRepo, "/api")

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
