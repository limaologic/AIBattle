package solver

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"reverse-challenge-system/pkg/db"
	"reverse-challenge-system/pkg/logger"
	"reverse-challenge-system/pkg/models"

	"github.com/rs/zerolog"
)

const (
	MaxRetryAttempts = 6
	BaseDelay        = 500 * time.Millisecond
	MaxDelay         = 30 * time.Second
	JitterMin        = 0.85
	JitterMax        = 1.15
)

type WorkerPool struct {
	workers    int
	db         *db.SolverDB
	service    *Service
	jobQueue   chan *models.PendingChallenge
	quit       chan struct{}
	workerQuit []chan struct{}
}

func NewWorkerPool(workers int, database *db.SolverDB, service *Service) *WorkerPool {
	return &WorkerPool{
		workers:    workers,
		db:         database,
		service:    service,
		jobQueue:   make(chan *models.PendingChallenge, workers*2),
		quit:       make(chan struct{}),
		workerQuit: make([]chan struct{}, workers),
	}
}

func (wp *WorkerPool) Start() {
	workerLogger := logger.NewCategoryLogger(wp.service.config.LogLevel, logger.Solver, logger.Worker)
	workerLogger.Info().Int("workers", wp.workers).Msg("Starting worker pool")

	// Start workers
	for i := 0; i < wp.workers; i++ {
		wp.workerQuit[i] = make(chan struct{})
		go wp.worker(i, wp.workerQuit[i])
	}

	// Start job dispatcher
	go wp.dispatcher()
}

func (wp *WorkerPool) Stop() {
	workerLogger := logger.NewCategoryLogger(wp.service.config.LogLevel, logger.Solver, logger.Worker)
	workerLogger.Info().Msg("Stopping worker pool")

	// Signal stop to dispatcher
	close(wp.quit)

	// Signal stop to all workers
	for i := 0; i < wp.workers; i++ {
		close(wp.workerQuit[i])
	}
}

func (wp *WorkerPool) dispatcher() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-wp.quit:
			workerLogger := logger.NewCategoryLogger(wp.service.config.LogLevel, logger.Solver, logger.Worker)
			workerLogger.Info().Msg("Worker pool dispatcher stopping")
			return

		case <-ticker.C:
			// Get pending challenges from database
			challenges, err := wp.db.GetPendingChallenges(wp.workers * 2)
			if err != nil {
				workerLogger := logger.NewCategoryLogger(wp.service.config.LogLevel, logger.Solver, logger.Worker)
				workerLogger.Error().Err(err).Msg("Failed to get pending challenges")
				continue
			}

			// Dispatch challenges to workers
			for _, challenge := range challenges {
				// Check if it's time to retry
				if challenge.Status == "processing" && time.Now().Before(challenge.NextRetryTime) {
					continue
				}

				select {
				case wp.jobQueue <- challenge:
					// Job queued successfully
				default:
					// Queue is full, skip this challenge for now
				}
			}
		}
	}
}

func (wp *WorkerPool) worker(id int, quit chan struct{}) {
	workerLogger := logger.NewCategoryLogger(wp.service.config.LogLevel, logger.Solver, logger.Worker).
		With().
		Int("worker_id", id).
		Logger()
	workerLogger.Info().Msg("Worker started")

	defer workerLogger.Info().Msg("Worker stopped")

	for {
		select {
		case <-quit:
			return

		case challenge := <-wp.jobQueue:
			wp.processChallenge(workerLogger, challenge)
		}
	}
}

func (wp *WorkerPool) processChallenge(workerLogger zerolog.Logger, challenge *models.PendingChallenge) {
	challengeLogger := workerLogger.With().Str("challenge_id", challenge.ID).Logger()
	challengeLogger.Info().Msg("Processing challenge")

	// Update status to processing
	if err := wp.db.UpdateChallengeStatus(challenge.ID, "processing", challenge.AttemptCount, time.Now()); err != nil {
		challengeLogger.Error().Err(err).Msg("Failed to update challenge status")
		return
	}

	// Solve the challenge
	answer, metadata, err := wp.solveChallenge(challenge)

	// Prepare callback request
	var callbackReq models.CallbackRequest

	if err != nil {
		challengeLogger.Error().Err(err).Msg("Failed to solve challenge")
		callbackReq = models.CallbackRequest{
			APIVersion:   "v2.1",
			ChallengeID:  challenge.ID,
			SolverJobID:  fmt.Sprintf("solver_job_%s", challenge.ID),
			Status:       "failed",
			ErrorCode:    "SOLVER_ERROR",
			ErrorMessage: err.Error(),
			Metadata:     metadata,
		}
	} else {
		callbackReq = models.CallbackRequest{
			APIVersion:  "v2.1",
			ChallengeID: challenge.ID,
			SolverJobID: fmt.Sprintf("solver_job_%s", challenge.ID),
			Status:      "success",
			Answer:      answer,
			Metadata:    metadata,
		}
	}

	// Send callback with retry
	if err := wp.sendCallbackWithRetry(challenge, &callbackReq); err != nil {
		challengeLogger.Error().Err(err).Msg("Failed to send callback after all retries")
		// Mark as failed
		wp.db.UpdateChallengeStatus(challenge.ID, "failed", MaxRetryAttempts, time.Now())
	} else {
		challengeLogger.Info().Msg("Challenge completed successfully")
		// Remove from pending challenges
		wp.db.DeleteChallenge(challenge.ID)
	}
}

func (wp *WorkerPool) solveChallenge(challenge *models.PendingChallenge) (string, json.RawMessage, error) {
	// This is where the actual solving logic would go
	// For this MVP, we'll implement a simple mock solver

	// Parse the problem to determine challenge type
	var problem map[string]interface{}
	if err := json.Unmarshal(challenge.Problem, &problem); err != nil {
		return "", nil, fmt.Errorf("failed to parse problem: %w", err)
	}

	challengeType, ok := problem["type"].(string)
	if !ok {
		return "", nil, fmt.Errorf("missing or invalid challenge type")
	}

	startTime := time.Now()

	var answer string

	switch challengeType {
	case "captcha":
		// Mock CAPTCHA solving - in reality, this would use ML models
		answer = wp.solveMockCAPTCHA(problem)

	case "math":
		// Mock math problem solving
		var err error
		answer, err = wp.solveMockMath(problem)
		if err != nil {
			return "", nil, err
		}

	case "text":
		// Mock text processing
		answer = wp.solveMockText(problem)

	default:
		return "", nil, fmt.Errorf("unsupported challenge type: %s", challengeType)
	}

	// Add some random delay to simulate processing time
	time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)

	computeTime := time.Since(startTime)

	// Create metadata
	metadata := models.SolverMetadata{
		ComputeTimeMs: int(computeTime.Milliseconds()),
		Algorithm:     fmt.Sprintf("mock_%s_solver", challengeType),
		Confidence:    0.85 + rand.Float64()*0.15, // 0.85-1.0
		AttemptCount:  1,
		Resource: map[string]interface{}{
			"cpu":    "4 cores",
			"mem_gb": 8,
			"gpu":    "mock-gpu",
		},
	}

	metadataJSON, _ := json.Marshal(metadata)

	return answer, metadataJSON, nil
}

func (wp *WorkerPool) solveMockCAPTCHA(problem map[string]interface{}) string {
	// Mock CAPTCHA solving - return random alphanumeric string
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	length := 5
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

func (wp *WorkerPool) solveMockMath(problem map[string]interface{}) (string, error) {
	// Mock math problem - expect format like {"operation": "add", "a": 5, "b": 3}
	operation, ok := problem["operation"].(string)
	if !ok {
		return "", fmt.Errorf("missing operation")
	}

	a, ok := problem["a"].(float64)
	if !ok {
		return "", fmt.Errorf("missing operand a")
	}

	b, ok := problem["b"].(float64)
	if !ok {
		return "", fmt.Errorf("missing operand b")
	}

	var result float64
	switch operation {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b == 0 {
			return "", fmt.Errorf("division by zero")
		}
		result = a / b
	default:
		return "", fmt.Errorf("unsupported operation: %s", operation)
	}

	return fmt.Sprintf("%.2f", result), nil
}

func (wp *WorkerPool) solveMockText(problem map[string]interface{}) string {
	// Mock text processing - just return uppercased version
	text, ok := problem["text"].(string)
	if !ok {
		return "PROCESSED_TEXT"
	}
	return strings.ToUpper(text)
}

func (wp *WorkerPool) sendCallbackWithRetry(challenge *models.PendingChallenge, callbackReq *models.CallbackRequest) error {
	challengeLogger := logger.NewCategoryLogger(wp.service.config.LogLevel, logger.Solver, logger.Worker).
		With().
		Str("challenge_id", challenge.ID).
		Logger()

	for attempt := 0; attempt < MaxRetryAttempts; attempt++ {
		attemptLogger := challengeLogger.With().Int("attempt", attempt+1).Logger()

		// Send callback
		statusCode, err := wp.service.SendCallback(challenge.CallbackURL, callbackReq)

		if err == nil && statusCode >= 200 && statusCode < 300 {
			// Success
			attemptLogger.Info().Int("status_code", statusCode).Msg("Callback sent successfully")
			return nil
		}

		// Check if we should retry
		shouldRetry := wp.shouldRetry(statusCode, err)

		attemptLogger.Error().
			Err(err).
			Int("status_code", statusCode).
			Bool("will_retry", shouldRetry && attempt < MaxRetryAttempts-1).
			Msg("Callback failed")

		if !shouldRetry {
			if err != nil {
				return fmt.Errorf("callback failed with non-retryable error: %w", err)
			}
			return fmt.Errorf("callback failed with non-retryable status code: %d", statusCode)
		}

		// Last attempt?
		if attempt == MaxRetryAttempts-1 {
			return fmt.Errorf("callback failed after %d attempts", MaxRetryAttempts)
		}

		// Calculate backoff delay with jitter
		delay := wp.calculateBackoffDelay(attempt)
		nextRetryTime := time.Now().Add(delay)

		// Update database with retry info
		if err := wp.db.UpdateChallengeStatus(challenge.ID, "processing", attempt+1, nextRetryTime); err != nil {
			attemptLogger.Error().Err(err).Msg("Failed to update retry status")
		}

		attemptLogger.Info().Dur("delay", delay).Msg("Waiting before retry")
		time.Sleep(delay)
	}

	return fmt.Errorf("callback failed after %d attempts", MaxRetryAttempts)
}

func (wp *WorkerPool) shouldRetry(statusCode int, err error) bool {
	// Network errors - retry
	if err != nil {
		return true
	}

	// 429 Too Many Requests - retry
	if statusCode == 429 {
		return true
	}

	// 5xx server errors - retry
	if statusCode >= 500 {
		return true
	}

	// 4xx client errors (except 429) - don't retry
	if statusCode >= 400 && statusCode < 500 {
		return false
	}

	// Other status codes - don't retry
	return false
}

func (wp *WorkerPool) calculateBackoffDelay(attempt int) time.Duration {
	// Exponential backoff: delay = min(max, base * 2^attempt)
	delay := BaseDelay * time.Duration(math.Pow(2, float64(attempt)))

	if delay > MaxDelay {
		delay = MaxDelay
	}

	// Add jitter: delay * random(0.85, 1.15)
	jitter := JitterMin + rand.Float64()*(JitterMax-JitterMin)
	delay = time.Duration(float64(delay) * jitter)

	return delay
}
