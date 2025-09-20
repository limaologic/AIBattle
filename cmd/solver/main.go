package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"reverse-challenge-system/internal/solver"
	"reverse-challenge-system/pkg/api"
	"reverse-challenge-system/pkg/auth"
	"reverse-challenge-system/pkg/config"
	"reverse-challenge-system/pkg/db"
	"reverse-challenge-system/pkg/logger"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Initialize logger with file output for solver service
	logger.InitWithFileLogging(cfg.LogLevel, logger.Solver)

	// Create startup logger
	startupLogger := logger.NewCategoryLogger(cfg.LogLevel, logger.Solver, logger.Startup)
	startupLogger.Info().Msg("Starting Reverse Challenge System - Solver")

	// Initialize database
	database, err := db.NewSolverDB(cfg.SolverDBPath)
	if err != nil {
		startupLogger.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer database.Close()
	startupLogger.Info().Str("db_path", cfg.SolverDBPath).Msg("Database initialized successfully")

	// Initialize HMAC authentication
	secrets := cfg.GetSolverSecrets()
	hmacAuth := auth.NewHMACAuth(secrets, cfg.GetClockSkew())
	startupLogger.Info().Int("secret_count", len(secrets)).Msg("HMAC authentication initialized")

	// Initialize service
	service := solver.NewService(cfg, database, hmacAuth)
	startupLogger.Info().Msg("Solver service initialized")

	// Start the worker pool
	service.Start()
	defer service.Stop()
	startupLogger.Info().Int("worker_count", cfg.SolverWorkerCount).Msg("Worker pool started")

	// Optionally start gRPC bridge (only active when built with -tags=grpcbridge)
	if stopBridge := startBridgeIfEnabled(service); stopBridge != nil {
		defer stopBridge(context.Background())
		startupLogger.Info().Msg("gRPC bridge started")
	}

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

	// Stats endpoint (no auth required for development)
	router.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := service.GetStats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}).Methods("GET")

	// Solve endpoint (requires HMAC auth)
	solveRouter := router.PathPrefix("/solve").Subrouter()
	solveRouter.Use(middleware.HMACAuth)
	solveRouter.HandleFunc("", service.HandleSolve).Methods("POST")

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.GetSolverAddr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		startupLogger.Info().
			Str("address", cfg.GetSolverAddr()).
			Int("workers", cfg.SolverWorkerCount).
			Msg("Solver server starting")

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

	startupLogger.Info().Msg("Solver server stopped")

	// Clean up old log files (keep last 7 days)
	if err := logger.CleanupOldLogs(7); err != nil {
		startupLogger.Warn().Err(err).Msg("Failed to cleanup old log files")
	}
}

func cleanupNonces(database *db.SolverDB, cfg *config.Config) {
	// Create a general category logger for background tasks
	cleanupLogger := logger.NewCategoryLogger(cfg.LogLevel, logger.Solver, logger.General)

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
