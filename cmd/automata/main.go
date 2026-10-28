package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/adapter/api"
	"github.com/h2hsecure/automata/internal/adapter/api/handler"
	"github.com/h2hsecure/automata/internal/adapter/tracking"
	infraauth "github.com/h2hsecure/automata/internal/infrastructure/auth"
	auth_repo "github.com/h2hsecure/automata/internal/infrastructure/auth/repo"
	banner_repo "github.com/h2hsecure/automata/internal/infrastructure/banner/repo"
	"github.com/h2hsecure/automata/internal/infrastructure/bootstrap"
	"github.com/h2hsecure/automata/internal/infrastructure/config"
	"github.com/h2hsecure/automata/internal/infrastructure/contact"
	contact_repo "github.com/h2hsecure/automata/internal/infrastructure/contact/repo"
	context_repo "github.com/h2hsecure/automata/internal/infrastructure/context/repo"
	"github.com/h2hsecure/automata/internal/infrastructure/database"
	imap "github.com/h2hsecure/automata/internal/infrastructure/email/imap"
	email_repo "github.com/h2hsecure/automata/internal/infrastructure/email/repo"
	mailsmtp "github.com/h2hsecure/automata/internal/infrastructure/email/smtp"
	form_repo "github.com/h2hsecure/automata/internal/infrastructure/form/repo"
	geo_infra "github.com/h2hsecure/automata/internal/infrastructure/geo"
	"github.com/h2hsecure/automata/internal/infrastructure/queue"
	settings_repo "github.com/h2hsecure/automata/internal/infrastructure/settings/repo"
	staticfile_repo "github.com/h2hsecure/automata/internal/infrastructure/staticfile/repo"
	"github.com/h2hsecure/automata/internal/infrastructure/storage"
	tracking_repo "github.com/h2hsecure/automata/internal/infrastructure/tracking/repo"
	webhook_repo "github.com/h2hsecure/automata/internal/infrastructure/webhook/repo"
	webhook_svc "github.com/h2hsecure/automata/internal/infrastructure/webhook"
	emailusecase "github.com/h2hsecure/automata/internal/usecase/email"
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

	if err := queue.RunMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run queue migrations")
	}

	if err := database.RunUserContextMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run user-context migrations")
	}

	if err := settings_repo.RunMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("settings migration failed")
	}

	if err := staticfile_repo.RunMigrations(pool); err != nil {
		log.Fatal().Err(err).Msg("static file migration failed")
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
	contactRepo := contact_repo.New(pool)
	geoLookup := geo_infra.New()

	// File upload storage (optional; enabled when uploads.dir is configured).
	var fileStore *storage.FileStore
	if dir := config.Get("uploads.dir"); dir != "" {
		s, err := storage.New(dir)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to init file store")
		}
		fileStore = s
		log.Info().Str("dir", dir).Msg("file upload storage enabled")
	}
	staticFileRepo := staticfile_repo.New(pool)

	// Admin bootstrap: create admin user on first startup
	if err := bootstrap.Run(pool, userRepo); err != nil {
		log.Warn().Err(err).Msg("admin bootstrap failed (continuing anyway)")
	}

	// Seed default email templates for all existing contexts
	contexts, _ := contextRepo.List(0, 1000)
	for _, ctx := range contexts {
		if err := email_repo.SeedDefaultsForContext(pool, ctx.ID); err != nil {
			log.Warn().Err(err).Str("context_id", ctx.ID).Msg("failed to seed email templates")
		}
	}

	// Initialize auth service (with API key validation backed by the DB)
	authService := infraauth.NewServiceWithAPIKeys(userRepo, apiKeyRepo)

	// Email templates + queue (shared by admin API and form-submission hook)
	emailRepo := email_repo.New(pool)
	emailQueue := queue.NewEmailQueue(pool)
	emailUsecase := emailusecase.NewEmailUsecaseWithContact(emailRepo, emailQueue, contactRepo)

	// Webhook queue + service (shared by admin API, form-submission hook, and worker)
	webhookQueue := queue.NewWebhookQueue(pool)
	webhookSvc := webhook_svc.NewService(webhook_repo.New(pool), webhookQueue)

	// Create handlers for tracking server
	formHandler := handler.NewFormHandlerWithEmailAndWebhooks(formRepo, contactRepo, trackingRepo, emailUsecase, webhookSvc).
		WithFileResolver(uploadFileResolver{store: fileStore})
	bannerHandler := handler.NewBannerHandler(bannerRepo)
	placementHandler := handler.NewPlacementHandler(bannerRepo)
	campaignHandler := handler.NewCampaignHandler(bannerRepo)

	// Context for graceful shutdown and background workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Admin server (port 8080)
	adminEngine := gin.Default()

	// Health check at root level (before SPA fallback)
	adminEngine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api.NewServer(adminEngine, pool, contextRepo, userRepo, authService, apiKeyRepo, "/api")
	api.RegisterStaticFileRoutes(adminEngine, contextRepo, authService, staticFileRepo, fileStore, "/api")
	api.RegisterSubmissionFileRoutes(adminEngine, contextRepo, authService, formRepo, fileStore, "/api")

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
	trackingEngine := tracking.NewServer(formHandler, bannerHandler, placementHandler, campaignHandler, trackingRepo, geoLookup, contactRepo)
	tracking.RegisterUploadRoutes(trackingEngine, tracking.NewUploadHandler(fileStore, formRepo))
	tracking.RegisterStaticRoutes(trackingEngine, tracking.NewStaticHandler(staticFileRepo, fileStore))

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

	// Email delivery worker (port 8080 API enqueues jobs; worker delivers via SMTP)
	portStr := config.Get("smtp.port")
	smtpPort, _ := strconv.Atoi(portStr)
	if smtpPort == 0 {
		smtpPort = 587
	}
	smtpCfg := mailsmtp.Config{
		Host:     config.Get("smtp.host"),
		Port:     smtpPort,
		Username: config.Get("smtp.username"),
		Password: config.Get("smtp.password"),
		From:     config.Get("smtp.from"),
		StartTLS: config.Get("smtp.starttls") != "false",
	}
	settingsRepo := settings_repo.New(pool)
	contextSettingsRepo := settings_repo.NewContext(pool)
	configFor := func(ctxID string) mailsmtp.Config {
		m, _ := settingsRepo.List()
		cm, _ := contextSettingsRepo.List(ctxID)
		return mailsmtp.MergeConfig(mailsmtp.MergeConfig(smtpCfg, m), cm)
	}
	mailer := mailsmtp.NewMailerWithProvider(configFor, smtpCfg, mailsmtp.DefaultDialer)
	if !mailer.IsConfigured() {
		log.Warn().Msg("smtp not configured: email jobs will be discarded after max retries")
	}
	if smtpCfg.Username != "" && !smtpCfg.StartTLS {
		log.Warn().Msg("smtp: username set with starttls disabled; credentials will be sent in cleartext")
	}
	emailQueue.StartWorker(ctx, mailer)

	// IMAP inbox poller (fetches new INBOX messages per context)
	imapPoller := imap.NewPoller(email_repo.NewInboxRepo(pool), contextRepo, configFor)
	go imapPoller.Start(ctx)

	// Webhook delivery worker (API enqueues deliveries; worker POSTs them)
	go queue.NewWebhookWorker(webhookQueue).Start(ctx)

	// Graceful shutdown on SIGINT/SIGTERM
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
