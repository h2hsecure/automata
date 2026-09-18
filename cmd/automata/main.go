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
	"github.com/hekemen/automata/internal/infrastructure/auth"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
	"github.com/hekemen/automata/internal/infrastructure/config"
	"github.com/hekemen/automata/internal/infrastructure/database"
	tenant_repo "github.com/hekemen/automata/internal/infrastructure/tenant/repo"
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

	// Initialize auth service
	authService := auth.NewService(userRepo)

	// Create Gin engine
	engine := gin.Default()

	// Register API routes under /api
	api.NewServer(engine, tenantRepo, userRepo, authService, apiKeyRepo, "/api")

	// Serve static UI files at root
	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "./web"
	}
	api.ServeStatic(engine, webDir)

	// Start server
	host := config.Get("server.host")
	port := config.Get("server.port")
	if host == "" {
		host = "0.0.0.0"
	}
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf("%s:%s", host, port)

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Info().Msg("shutting down...")
		cancel()
	}()

	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("starting server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("failed to start server")
		}
	}()

	<-ctx.Done()

	// Shutdown server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
	log.Info().Msg("server stopped")
}
