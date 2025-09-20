package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"reverse-challenge-system/internal/challenger"
	"reverse-challenge-system/pkg/api"
	"reverse-challenge-system/pkg/auth"
	"reverse-challenge-system/pkg/config"
	"reverse-challenge-system/pkg/db"
	"reverse-challenge-system/pkg/logger"
	"reverse-challenge-system/pkg/sui"

	"github.com/gorilla/mux"
	"github.com/pattonkan/sui-go/suiclient"
	"github.com/pattonkan/sui-go/suiclient/conn"
	"github.com/pattonkan/sui-go/suisigner"
	"github.com/pattonkan/sui-go/suisigner/suicrypto"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Initialize logger with file output for challenger service
	logger.InitWithFileLogging(cfg.LogLevel, logger.Challenger)

	// Create startup logger
	startupLogger := logger.NewCategoryLogger(cfg.LogLevel, logger.Challenger, logger.Startup)
	startupLogger.Info().Msg("Starting Reverse Challenge System - Challenger")

	// Initialize database
	database, err := db.NewChallengerDB(cfg.ChallengerDBPath)
	if err != nil {
		startupLogger.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer database.Close()
	startupLogger.Info().Str("db_path", cfg.ChallengerDBPath).Msg("Database initialized successfully")

	// Initialize HMAC authentication
	secrets := cfg.GetChallengerSecrets()
	hmacAuth := auth.NewHMACAuth(secrets, cfg.GetClockSkew())
	startupLogger.Info().Int("secret_count", len(secrets)).Msg("HMAC authentication initialized")

	singer, err := suisigner.NewSignerWithMnemonic(cfg.SUI.ChallengerMnemonic, suicrypto.KeySchemeFlagEd25519)
	if err != nil {
		panic(err)
	}

	err = suiclient.RequestFundFromFaucet(singer.Address, cfg.SUI.FaucetRPCUrl)

	// Initialize Sui TransactionBuilder if mnemonic is provided
	var suiTxBuilder *sui.TransactionBuilder
	suiRPCURL := conn.LocalnetEndpointUrl
	suiTxBuilder, err = sui.NewTransactionBuilder(context.Background(), startupLogger, suiRPCURL, cfg.SUI.PackageID, cfg.SUI.ChallengerMnemonic)
	if err != nil {
		startupLogger.Error().Err(err).Msg("Failed to initialize Sui TransactionBuilder, continuing without Sui integration")
	} else {
		startupLogger.Info().
			Str("rpc_url", suiRPCURL).
			Str("package_id", cfg.SUI.PackageID).
			Msg("Sui TransactionBuilder initialized successfully")
	}

	// Initialize service
	service := challenger.NewService(cfg, database, hmacAuth, suiTxBuilder)
	startupLogger.Info().Msg("Challenger service initialized")

	// Initialize middleware
	middleware := api.NewMiddleware(hmacAuth, database)

	// Create router
	router := mux.NewRouter()

	// Add middleware
	router.Use(middleware.RequestLogging)
	router.Use(middleware.SizeLimit)
	router.Use(middleware.CORS)

	// Health endpoints (no auth required)
	router.HandleFunc("/healthz", api.HealthCheck).Methods("GET")
	router.HandleFunc("/readyz", api.ReadinessCheck(database)).Methods("GET")

	// Callback endpoint (requires HMAC auth)
	callbackRouter := router.PathPrefix("/callback").Subrouter()
	callbackRouter.Use(middleware.HMACAuth)
	callbackRouter.HandleFunc("/{challenge_id}", service.HandleCallback).Methods("POST")

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.GetChallengerAddr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		startupLogger.Info().
			Str("address", cfg.GetChallengerAddr()).
			Str("public_callback_host", cfg.PublicCallbackHost).
			Msg("Challenger server starting")

		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			startupLogger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// Start background cleanup goroutine
	go cleanupNonces(database, cfg)
	startupLogger.Info().Msg("Background nonce cleanup routine started")

	// Wait for interrupt signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	<-interrupt
	startupLogger.Info().Msg("Shutdown signal received")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		startupLogger.Error().Err(err).Msg("Server shutdown error")
	}

	startupLogger.Info().Msg("Challenger server stopped")

	// Clean up old log files (keep last 7 days)
	if err := logger.CleanupOldLogs(7); err != nil {
		startupLogger.Warn().Err(err).Msg("Failed to cleanup old log files")
	}
}

func cleanupNonces(database *db.ChallengerDB, cfg *config.Config) {
	// Create a general category logger for background tasks
	cleanupLogger := logger.NewCategoryLogger(cfg.LogLevel, logger.Challenger, logger.General)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Clean up nonces older than 2x clock skew
			olderThan := time.Now().Add(-2 * cfg.GetClockSkew())
			if err := database.CleanupOldNonces(olderThan); err != nil {
				cleanupLogger.Error().Err(err).Msg("Failed to cleanup old nonces")
			} else {
				cleanupLogger.Debug().Msg("Cleaned up old nonces")
			}
		}
	}
}
