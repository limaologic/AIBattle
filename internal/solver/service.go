package solver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"reverse-challenge-system/pkg/auth"
	"reverse-challenge-system/pkg/config"
	"reverse-challenge-system/pkg/db"
	"reverse-challenge-system/pkg/logger"
	"reverse-challenge-system/pkg/models"

	"github.com/google/uuid"
	"github.com/pattonkan/sui-go/suisigner"
	"github.com/pattonkan/sui-go/suisigner/suicrypto"
)

type Service struct {
	config     *config.Config
	db         *db.SolverDB
	hmacAuth   *auth.HMACAuth
	client     *http.Client
	workerPool *WorkerPool
}

func NewService(cfg *config.Config, database *db.SolverDB, hmacAuth *auth.HMACAuth) *Service {
	service := &Service{
		config:   cfg,
		db:       database,
		hmacAuth: hmacAuth,
		client:   &http.Client{Timeout: 30 * time.Second},
	}

	// Initialize worker pool
	service.workerPool = NewWorkerPool(cfg.SolverWorkerCount, database, service)

	return service
}

func (s *Service) Start() {
	startupLogger := logger.NewCategoryLogger(s.config.LogLevel, logger.Solver, logger.Startup)
	startupLogger.Info().Msg("Starting solver service")
	s.workerPool.Start()
}

func (s *Service) Stop() {
	startupLogger := logger.NewCategoryLogger(s.config.LogLevel, logger.Solver, logger.Startup)
	startupLogger.Info().Msg("Stopping solver service")
	s.workerPool.Stop()
}

func (s *Service) HandleSolve(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// Create request-specific logger that writes to file
	requestLogger := logger.NewCategoryLogger(s.config.LogLevel, logger.Solver, logger.Request).
		With().
		Str("request_id", requestID).
		Logger()

	requestLogger.Info().
		Str("method", r.Method).
		Str("remote_addr", r.RemoteAddr).
		Str("user_agent", r.Header.Get("User-Agent")).
		Msg("Solve request received")

	// Decode solve request
	var solveReq models.SolveRequest
	if err := json.NewDecoder(r.Body).Decode(&solveReq); err != nil {
		requestLogger.Error().Err(err).Msg("Failed to decode solve request")
		s.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body", requestID)
		return
	}

	// Validate request
	if solveReq.APIVersion != "v2.1" {
		s.writeError(w, http.StatusBadRequest, "UNSUPPORTED_VERSION",
			"Unsupported API version", requestID)
		return
	}

	if solveReq.ChallengeID == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_CHALLENGE_ID",
			"Challenge ID is required", requestID)
		return
	}

	// Validate callback URL
	if err := s.validateCallbackURL(solveReq.CallbackURL); err != nil {
		requestLogger.Error().Err(err).Str("callback_url", solveReq.CallbackURL).Msg("Invalid callback URL")
		s.writeError(w, http.StatusBadRequest, "INVALID_CALLBACK_URL",
			"Invalid callback URL", requestID)
		return
	}

	// Check if we've already seen this challenge
	existingChallenge, err := s.db.GetChallenge(solveReq.ChallengeID)
	if err != nil {
		requestLogger.Error().Err(err).Msg("Failed to check existing challenge")
		s.writeError(w, http.StatusInternalServerError, "DB_ERROR",
			"Database error", requestID)
		return
	}

	if existingChallenge != nil {
		// Already have this challenge, return existing job ID
		jobID := fmt.Sprintf("solver_job_%s", solveReq.ChallengeID)
		response := models.SolveResponse{
			Message:     "Challenge already accepted",
			SolverJobID: jobID,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create pending challenge
	challenge := &models.PendingChallenge{
		ID:            solveReq.ChallengeID,
		Problem:       solveReq.Problem,
		OutputSpec:    solveReq.OutputSpec,
		CallbackURL:   solveReq.CallbackURL,
		ReceivedAt:    time.Now(),
		Status:        "pending",
		AttemptCount:  0,
		NextRetryTime: time.Now(),
	}

	// Save to database
	if err := s.db.SaveChallenge(challenge); err != nil {
		requestLogger.Error().Err(err).Msg("Failed to save challenge")
		s.writeError(w, http.StatusInternalServerError, "DB_ERROR",
			"Failed to save challenge", requestID)
		return
	}

	requestLogger.Info().
		Str("challenge_id", solveReq.ChallengeID).
		Str("callback_url", solveReq.CallbackURL).
		Msg("Challenge accepted and queued for processing")

	// Return success response
	jobID := fmt.Sprintf("solver_job_%s", solveReq.ChallengeID)
	response := models.SolveResponse{
		Message:     "Challenge accepted",
		SolverJobID: jobID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
}

func (s *Service) SendCallback(callbackURL string, callbackReq *models.CallbackRequest) (int, error) {
	logger := logger.WithChallengeID(callbackReq.ChallengeID)

	// Marshal request body
	body, err := json.Marshal(callbackReq)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal callback request: %w", err)
	}

	// Create HMAC signature
	nonce := uuid.New().String()

	// Extract path from callback URL
	// For URL like "https://example.com/callback/ch_001", we want "/callback/ch_001"
	callbackPath := "/callback/" + callbackReq.ChallengeID

	authHeader := s.hmacAuth.CreateAuthHeader("POST", callbackPath, body, s.config.ChalHMACKeyID, nonce)
	if authHeader == "" {
		return 0, fmt.Errorf("failed to create auth header")
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", callbackURL, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("X-Request-ID", uuid.New().String())
	signer, err := suisigner.NewSignerWithMnemonic(s.config.SUI.SolverMnemonic, suicrypto.KeySchemeFlagEd25519)
	if err != nil {
		panic(err)
	}
	logger.Info().Str("solver address", signer.Address.String())
	req.Header.Set("X-Solver-Address", signer.Address.String())

	// Send request
	logger.Info().Str("callback_url", callbackURL).Msg("Sending callback")
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.Info().
		Int("status_code", resp.StatusCode).
		Msg("Callback response received")

	return resp.StatusCode, nil
}

func (s *Service) validateCallbackURL(callbackURL string) error {
	// Basic validation - ensure it's not empty
	if callbackURL == "" {
		return fmt.Errorf("callback URL cannot be empty")
	}

	if len(callbackURL) > 2048 {
		return fmt.Errorf("callback URL too long")
	}

	// Parse URL for validation
	u, err := url.Parse(callbackURL)
	if err != nil {
		return fmt.Errorf("invalid callback URL: %w", err)
	}

	// If ngrok is enabled, require HTTPS
	if s.config.UseNgrok {
		if u.Scheme != "https" {
			return fmt.Errorf("callback URL must use HTTPS when USE_NGROK=true")
		}
		// Allow ngrok domains
		if strings.Contains(u.Host, "ngrok.io") || strings.Contains(u.Host, "ngrok-free.app") {
			return nil
		}
	} else {
		// For local development, allow HTTP on localhost/127.0.0.1
		if u.Scheme == "http" {
			if strings.Contains(u.Host, "localhost") || strings.Contains(u.Host, "127.0.0.1") {
				return nil
			}
			return fmt.Errorf("HTTP callback URLs only allowed for localhost when USE_NGROK=false")
		}
		if u.Scheme == "https" {
			// HTTPS is always allowed
			return nil
		}
		return fmt.Errorf("callback URL must use HTTP (localhost only) or HTTPS")
	}

	// Basic whitelist check
	allowedHosts := []string{
		"localhost",
		"127.0.0.1",
	}

	for _, host := range allowedHosts {
		if strings.Contains(u.Host, host) {
			return nil
		}
	}

	return fmt.Errorf("callback URL host not in whitelist: %s", u.Host)
}

func (s *Service) writeError(w http.ResponseWriter, statusCode int, code, message, requestID string) {
	errorResp := models.ErrorResponse{
		Error: models.ErrorDetails{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResp)
}

func (s *Service) GetStats() map[string]interface{} {
	// Get some basic stats from the database
	challenges, _ := s.db.GetPendingChallenges(1000) // Get up to 1000 for stats

	statusCounts := make(map[string]int)
	for _, challenge := range challenges {
		statusCounts[challenge.Status]++
	}

	return map[string]interface{}{
		"total_pending":    len(challenges),
		"status_breakdown": statusCounts,
		"worker_count":     s.config.SolverWorkerCount,
	}
}
