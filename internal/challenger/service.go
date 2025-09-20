package challenger

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"reverse-challenge-system/pkg/auth"
	"reverse-challenge-system/pkg/config"
	"reverse-challenge-system/pkg/db"
	"reverse-challenge-system/pkg/logger"
	"reverse-challenge-system/pkg/models"
	"reverse-challenge-system/pkg/sui"
	"reverse-challenge-system/pkg/validator"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	suigo "github.com/pattonkan/sui-go/sui"
	"github.com/rs/zerolog"
)

// logEntry represents the JSON payload for external log uploads
type logEntry struct {
	ID             string `json:"id"`
	Log            string `json:"log"`
	ChallengerAddr string `json:"challenger_addr,omitempty"`
	SolverAddr     string `json:"solver_addr,omitempty"`
	VerifierAddr   string `json:"verifier_addr,omitempty"`
}

type Service struct {
	config       *config.Config
	db           *db.ChallengerDB
	hmacAuth     *auth.HMACAuth
	validator    *validator.Validator
	client       *http.Client
	suiTxBuilder *sui.TransactionBuilder
}

func NewService(cfg *config.Config, database *db.ChallengerDB, hmacAuth *auth.HMACAuth, suiTxBuilder *sui.TransactionBuilder) *Service {
	return &Service{
		config:       cfg,
		db:           database,
		hmacAuth:     hmacAuth,
		validator:    validator.NewValidator(),
		client:       &http.Client{Timeout: 30 * time.Second},
		suiTxBuilder: suiTxBuilder,
	}
}

func (s *Service) CreateChallenge(challenge *models.Challenge) error {
	challenge.CreatedAt = time.Now()
	return s.db.CreateChallenge(challenge)
}

func (s *Service) SendChallenge(challengeID, solverURL string) error {
	// Create request-specific logger that writes to file
	requestLogger := logger.NewCategoryLogger(s.config.LogLevel, logger.Challenger, logger.Request).
		With().
		Str("challenge_id", challengeID).
		Str("solver_url", solverURL).
		Logger()

	// Get challenge from database
	challenge, err := s.db.GetChallenge(challengeID)
	if err != nil {
		return fmt.Errorf("failed to get challenge: %w", err)
	}

	// Construct callback URL
	callbackURL := fmt.Sprintf("%s/callback/%s", s.config.PublicCallbackHost, challengeID)

	// Create solve request
	solveReq := models.SolveRequest{
		APIVersion:  "v2.1",
		ChallengeID: challengeID,
		Problem:     challenge.Problem,
		OutputSpec:  challenge.OutputSpec,
		Constraints: models.Constraints{
			TimeoutMs:  30000,
			DeadlineTs: time.Now().Add(5 * time.Minute).Unix(),
		},
		CallbackURL: callbackURL,
	}

	// Marshal request body
	body, err := json.Marshal(solveReq)
	if err != nil {
		return fmt.Errorf("failed to marshal solve request: %w", err)
	}

	// Create HMAC signature
	nonce := uuid.New().String()
	authHeader := s.hmacAuth.CreateAuthHeader("POST", "/solve", body, s.config.SolverHMACKeyID, nonce)
	if authHeader == "" {
		return fmt.Errorf("failed to create auth header")
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", solverURL+"/solve", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("X-Request-ID", uuid.New().String())

	// Send request
	requestLogger.Info().Str("solver_url", solverURL).Msg("Sending challenge to solver")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("solver returned status %d", resp.StatusCode)
	}

	var solveResp models.SolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&solveResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	requestLogger.Info().
		Str("solver_job_id", solveResp.SolverJobID).
		Msg("Challenge sent successfully")

	return nil
}

func (s *Service) HandleCallback(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	challengeID := vars["challenge_id"]
	requestID := r.Header.Get("X-Request-ID")

	// Create callback-specific logger that writes to file
	callbackLogger := logger.NewCategoryLogger(s.config.LogLevel, logger.Challenger, logger.Callback).
		With().
		Str("request_id", requestID).
		Str("challenge_id", challengeID).
		Logger()

	callbackLogger.Info().
		Str("method", r.Method).
		Str("remote_addr", r.RemoteAddr).
		Str("user_agent", r.Header.Get("User-Agent")).
		Msg("Callback request received")

	// Read request body
	var callbackReq models.CallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&callbackReq); err != nil {
		callbackLogger.Error().Err(err).Msg("Failed to decode callback request")
		s.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body", requestID)
		return
	}

	// Validate request
	if callbackReq.ChallengeID != challengeID {
		callbackLogger.Error().
			Str("body_challenge_id", callbackReq.ChallengeID).
			Str("url_challenge_id", challengeID).
			Msg("Challenge ID mismatch")
		s.writeError(w, http.StatusBadRequest, "CHALLENGE_ID_MISMATCH",
			"Challenge ID in body does not match URL", requestID)
		return
	}

	// Removed the check-then-insert pattern to avoid race condition
	// We'll use INSERT OR IGNORE at the database level instead

	// Get challenge for validation
	challenge, err := s.db.GetChallenge(challengeID)
	if err != nil {
		callbackLogger.Error().Err(err).Msg("Failed to get challenge")
		s.writeError(w, http.StatusNotFound, "CHALLENGE_NOT_FOUND",
			"Challenge not found", requestID)
		return
	}

	// Validate answer if status is success
	isCorrect := false
	if callbackReq.Status == "success" && callbackReq.Answer != "" {
		correct, err := s.validator.ValidateAnswer(challenge.ValidationRule, callbackReq.Answer)
		if err != nil {
			callbackLogger.Error().Err(err).
				Str("answer", callbackReq.Answer).
				Msg("Failed to validate answer")
		} else {
			isCorrect = correct
		}
	}

	// Extract solver address from request headers or use a default
	solverAddress := r.Header.Get("X-Solver-Address")
	if solverAddress == "" {
		// For demo purposes, derive a deterministic address from remote IP
		// In production, this should come from authentication or be provided by solver
		solverAddress = "0x" + fmt.Sprintf("%040x", len(r.RemoteAddr))[:40] // Placeholder based on IP
	}

	// Create result record
	result := &models.Result{
		ChallengeID:    challengeID,
		RequestID:      requestID,
		SolverJobID:    callbackReq.SolverJobID,
		Status:         callbackReq.Status,
		ReceivedAnswer: callbackReq.Answer,
		IsCorrect:      isCorrect,
		SolverAddress:  solverAddress,
		ComputeTimeMs:  0, // Extract from metadata if available
		SolverMetadata: callbackReq.Metadata,
		CreatedAt:      time.Now(),
	}

	// Extract compute time from metadata if available
	if callbackReq.Metadata != nil {
		var metadata models.SolverMetadata
		if err := json.Unmarshal(callbackReq.Metadata, &metadata); err == nil {
			result.ComputeTimeMs = metadata.ComputeTimeMs
		}
	}

	// Save result with duplicate check for idempotency
	wasInserted, err := s.db.SaveResultWithDuplicateCheck(result)
	if err != nil {
		callbackLogger.Error().Err(err).Msg("Failed to save result")
		s.writeError(w, http.StatusInternalServerError, "DB_ERROR",
			"Failed to save result", requestID)
		return
	}

	// If the result wasn't inserted, it means this is a duplicate request
	isDuplicate := !wasInserted

	// Only save webhook audit for new (non-duplicate) requests to avoid unnecessary storage
	if !isDuplicate {
		bodyBytes, _ := json.Marshal(callbackReq)
		bodyHash := sha256.Sum256(bodyBytes)

		audit := &models.WebhookAudit{
			ChallengeID: challengeID,
			RequestID:   requestID,
			Headers:     s.serializeHeaders(r.Header),
			BodyHash:    hex.EncodeToString(bodyHash[:]),
			StatusCode:  http.StatusOK,
			CreatedAt:   time.Now(),
		}

		if err := s.db.SaveWebhookAudit(audit); err != nil {
			callbackLogger.Error().Err(err).Msg("Failed to save webhook audit")
			// Don't fail the request for audit errors
		}
	}

	callbackLogger.Info().
		Str("status", callbackReq.Status).
		Bool("is_correct", isCorrect).
		Bool("is_duplicate", isDuplicate).
		Str("solver_job_id", callbackReq.SolverJobID).
		Msg("Callback processed successfully")

	// Upload to Sui if enabled and this is a successful, non-duplicate result
	var commitmentID string
	if s.suiTxBuilder != nil && !isDuplicate && callbackReq.Status == "success" {
		objId, err := s.uploadToSuiSync(challengeID, result, callbackLogger)
		if err != nil {
			callbackLogger.Error().Err(err).Msg("Failed to upload to Sui")
			commitmentID = fmt.Sprintf("%s:%s", challengeID, requestID) // fallback to original format
		} else {
			commitmentID = objId.String()
		}
		// Add bounty to vault if vault ID is configured
		if err := s.VaultAddBounty(s.config.SUI.VaultID); err != nil {
			callbackLogger.Warn().Err(err).Msg("Failed to add bounty to vault")
		}
	} else {
		commitmentID = fmt.Sprintf("%s:%s", challengeID, requestID) // fallback to original format
	}

	// Build log entry
	challengerAddr := ""
	if s.suiTxBuilder != nil && s.suiTxBuilder.Signer() != nil {
		challengerAddr = s.suiTxBuilder.Signer().Address.String()
	}

	b, err := json.Marshal(result)
	if err != nil {
		callbackLogger.Error().Err(err).Msg("Failed to marshal result")
		s.writeError(w, http.StatusInternalServerError, "DB_ERROR",
			"Failed to marshal result", requestID)
		return
	}

	entry := logEntry{
		ID:             commitmentID,
		Log:            string(b),
		ChallengerAddr: challengerAddr,
		SolverAddr:     solverAddress,
		// VerifierAddr left empty for now
	}

	// Non-blocking upload with timeout
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.uploadCallbackLog(ctx, entry, callbackLogger)
	}()

	s.writeCallbackResponse(w, challengeID, isDuplicate)
}

func (s *Service) writeCallbackResponse(w http.ResponseWriter, challengeID string, duplicate bool) {
	response := models.CallbackResponse{
		Received:    true,
		ChallengeID: challengeID,
		Duplicate:   duplicate,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
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

func (s *Service) serializeHeaders(headers http.Header) string {
	var headerLines []string
	for key, values := range headers {
		for _, value := range values {
			headerLines = append(headerLines, fmt.Sprintf("%s: %s", key, value))
		}
	}
	return strings.Join(headerLines, "\n")
}

func (s *Service) ValidateCallbackURL(callbackURL string) error {
	u, err := url.Parse(callbackURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
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

	// Basic whitelist check - in production, you'd want a more comprehensive list
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

// uploadToSuiSync uploads challenge commitment to Sui blockchain synchronously and returns the object ID
func (s *Service) uploadToSuiSync(challengeID string, result *models.Result, callbackLogger zerolog.Logger) (*suigo.ObjectId, error) {
	// Get type arguments from config
	typeArgs := sui.TypeArgs{
		TreasuryCapPositive: fmt.Sprintf("%s::pos::POS", s.config.SUI.PosPackageID),
		TreasuryCapNegative: fmt.Sprintf("%s::neg::NEG", s.config.SUI.NegPackageID),
		CoinTypeCollateral:  "0x2::sui::SUI", // Using SUI as collateral for simplicity
	}

	registryID := s.config.SUI.RegistryID
	if registryID == "" {
		return nil, fmt.Errorf("SUI_REGISTRY_ID not configured")
	}

	// Create commitment hash from challenge data
	commitment := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", registryID, result.ReceivedAnswer)))

	// Get challenger address from environment (in production, this would come from authentication)
	challengerAddr := s.suiTxBuilder.Signer().Address.String()

	// Use solver address from the result data
	solverAddr := result.SolverAddress

	// Calculate score (simple example - in production this would be more sophisticated)
	score := uint64(0)
	if result.IsCorrect {
		// FIXME we need to calculate this
		score = 100
	}

	timestamp := uint64(result.CreatedAt.Unix())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	callbackLogger.Info().
		Str("registry_id", registryID).
		Str("challenger_addr", challengerAddr).
		Str("solver_addr", solverAddr).
		Uint64("score", score).
		Str("commitment_hex", hex.EncodeToString(commitment[:])).
		Msg("Building challenge commitment transaction")

	// Build the transaction first
	objId, err := s.suiTxBuilder.UploadChallengeCommitment(
		ctx,
		typeArgs,
		registryID,
		commitment[:],
		challengerAddr,
		solverAddr,
		score,
		timestamp,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build challenge commitment transaction: %w", err)
	}

	callbackLogger.Info().
		Str("objId", objId.String()).
		Msg("Challenge commitment successfully uploaded to Sui")

	// Write digest to file
	if err := s.writeDigestToFile(objId.String(), callbackLogger); err != nil {
		callbackLogger.Error().Err(err).
			Str("objId", objId.String()).
			Msg("Failed to write transaction digest to file")
		// Don't return error here as the upload was successful
	}

	return objId, nil
}

func (s *Service) VaultAddBounty(vaultId string) error {
	return s.suiTxBuilder.VaultAddBounty(context.Background(), vaultId)
}

// writeDigestToFile writes the transaction digest to the configured file path
func (s *Service) writeDigestToFile(digest string, logger zerolog.Logger) error {
	digestFile := s.config.TxDigestFile
	if digestFile == "" {
		return fmt.Errorf("TX_DIGEST_FILE not configured")
	}

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(digestFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write digest to file with 0600 permissions
	if err := os.WriteFile(digestFile, []byte(digest+"\n"), 0600); err != nil {
		return fmt.Errorf("failed to write digest to file %s: %w", digestFile, err)
	}

	logger.Info().
		Str("path", digestFile).
		Str("tx_digest", digest).
		Msg("Transaction digest written to file")

	return nil
}

// uploadCallbackLog uploads a callback log entry to the external log service
func (s *Service) uploadCallbackLog(ctx context.Context, entry logEntry, lg zerolog.Logger) {
	if s.config.LogServiceURL == "" || s.config.LogServiceAPIKey == "" {
		lg.Debug().Msg("Log service not configured; skipping upload")
		return
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		lg.Error().Err(err).Msg("Failed to marshal log entry")
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.config.LogServiceURL, bytes.NewBuffer(jsonData))
	if err != nil {
		lg.Error().Err(err).Msg("Failed to create log upload request")
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.config.LogServiceAPIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		lg.Error().Err(err).Msg("Log upload request failed")
		return
	}
	defer resp.Body.Close()

	lg.Info().
		Str("status", resp.Status).
		Int("status_code", resp.StatusCode).
		Msg("Log upload completed")
}
