package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	if err := config.Load("config.yaml"); err != nil {
		log.Warn().Err(err).Msg("failed to load config, using defaults")
	}

	// Initialize database
	pool, err := database.NewPool()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer database.Close(pool)

	if err := database.RunMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run migrations")
	}

	// Initialize repositories
	tenantRepo := tenant_repo.NewPostgresRepo(pool)
	userRepo := tenant_repo.NewUserPostgresRepo(pool)
	apiKeyRepo := auth_repo.NewApiKeyPostgresRepo(pool)

	// Initialize auth service
	authService := auth.NewService(userRepo)

	// Initialize API server
	server := api.NewServer(tenantRepo, userRepo, authService, apiKeyRepo)

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
		Handler: server,
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
