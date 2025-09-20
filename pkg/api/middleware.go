// Package api provides HTTP middleware components for the Reverse Challenge System.
// Includes authentication, logging, CORS, request limiting, and health check functionality.
// Designed to work with both challenger and solver services.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"reverse-challenge-system/pkg/auth"
	"reverse-challenge-system/pkg/db"
	"reverse-challenge-system/pkg/logger"
	"reverse-challenge-system/pkg/models"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	MaxRequestSize = 5 * 1024 * 1024 // Maximum allowed request size: 5MB
)

// Middleware provides HTTP middleware functionality with HMAC authentication and request logging.
// Supports both challenger and solver database types through interface{} typing.
type Middleware struct {
	hmacAuth *auth.HMACAuth // HMAC authenticator for request verification
	db       interface{}    // Database instance - can be either ChallengerDB or SolverDB
}

// NewMiddleware creates a new middleware instance with HMAC authentication and database.
// The database parameter can be either *db.ChallengerDB or *db.SolverDB.
func NewMiddleware(hmacAuth *auth.HMACAuth, database interface{}) *Middleware {
	return &Middleware{
		hmacAuth: hmacAuth,
		db:       database,
	}
}

// RequestLogging middleware logs HTTP request start and completion with timing.
// Automatically generates request IDs and tracks response status codes.
func (m *Middleware) RequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create request ID if not present
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Add to context for later use
		r.Header.Set("X-Request-ID", requestID)

		// Create response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		log.Info().
			Str("request_id", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Msg("Request started")

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		log.Info().
			Str("request_id", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", wrapped.statusCode).
			Dur("duration", duration).
			Msg("Request completed")
	})
}

// SizeLimit middleware restricts request body size to prevent resource exhaustion.
// Rejects requests larger than MaxRequestSize (5MB) with appropriate error response.
func (m *Middleware) SizeLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, MaxRequestSize)
		next.ServeHTTP(w, r)
	})
}

// HMACAuth middleware validates HMAC-SHA256 signatures and prevents replay attacks.
// Checks authorization headers, verifies signatures, and tracks nonces.
func (m *Middleware) HMACAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		logger := logger.WithRequestID(requestID)

		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.writeError(w, http.StatusUnauthorized, "MISSING_AUTH", "Authorization header required", requestID)
			return
		}

		// Parse auth header
		authInfo, err := auth.ParseAuthHeader(authHeader)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to parse auth header")
			m.writeError(w, http.StatusUnauthorized, "INVALID_AUTH", "Invalid authorization header", requestID)
			return
		}

		// Read and buffer the body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to read request body")
			m.writeError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body", requestID)
			return
		}

		// Restore body for later use
		r.Body = io.NopCloser(bytes.NewReader(body))

		// Check for nonce replay
		if err := m.checkNonce(authInfo.Nonce); err != nil {
			logger.Error().Err(err).Str("nonce", authInfo.Nonce).Msg("Nonce replay detected")
			m.writeError(w, http.StatusUnauthorized, "REPLAY_ATTACK", "Nonce already seen", requestID)
			return
		}

		// Verify signature
		if err := m.hmacAuth.VerifySignature(r.Method, r.URL.EscapedPath(), body, authInfo); err != nil {
			logger.Error().Err(err).Str("key_id", authInfo.KeyID).Msg("Signature verification failed")
			m.writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "Signature verification failed", requestID)
			return
		}

		// Save nonce to prevent replay
		if err := m.saveNonce(authInfo.Nonce); err != nil {
			logger.Error().Err(err).Msg("Failed to save nonce")
			// Continue anyway - this is not critical
		}

		// Add auth info to headers for handlers to use
		r.Header.Set("X-Auth-KeyID", authInfo.KeyID)
		r.Header.Set("X-Auth-Timestamp", authInfo.Timestamp)
		r.Header.Set("X-Auth-Nonce", authInfo.Nonce)

		logger.Debug().Str("key_id", authInfo.KeyID).Msg("Authentication successful")
		next.ServeHTTP(w, r)
	})
}

// CORS middleware adds Cross-Origin Resource Sharing headers.
// Allows cross-origin requests for web-based challenger interfaces.
func (m *Middleware) CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// HTTPSOnly middleware redirects HTTP requests to HTTPS in production.
// Checks X-Forwarded-Proto header for proxy/load balancer scenarios.
func (m *Middleware) HTTPSOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In production, check if the request came through HTTPS
		// For development with ngrok, we can skip this check
		if r.Header.Get("X-Forwarded-Proto") == "http" {
			http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// checkNonce verifies if a nonce has been seen before to prevent replay attacks.
// Works with both challenger and solver databases through type assertion.
func (m *Middleware) checkNonce(nonce string) error {
	switch db := m.db.(type) {
	case *db.ChallengerDB:
		seen, err := db.HasSeenNonce(nonce)
		if err != nil {
			return err
		}
		if seen {
			return fmt.Errorf("nonce already seen")
		}
	case *db.SolverDB:
		seen, err := db.HasSeenNonce(nonce)
		if err != nil {
			return err
		}
		if seen {
			return fmt.Errorf("nonce already seen")
		}
	}
	return nil
}

// saveNonce stores a nonce in the database to prevent future replay attacks.
// Handles both challenger and solver database types.
func (m *Middleware) saveNonce(nonce string) error {
	switch db := m.db.(type) {
	case *db.ChallengerDB:
		return db.SaveNonce(nonce)
	case *db.SolverDB:
		return db.SaveNonce(nonce)
	}
	return nil
}

// writeError sends a standardized JSON error response to the client.
// Includes structured error details with request ID for tracing.
func (m *Middleware) writeError(w http.ResponseWriter, statusCode int, code, message, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResp := models.ErrorResponse{
		Error: models.ErrorDetails{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	}

	json.NewEncoder(w).Encode(errorResp)
}

// responseWriter wraps http.ResponseWriter to capture the status code.
// Used by request logging middleware to track response status.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code and delegates to the wrapped writer.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// HealthCheck provides a simple health status endpoint.
// Returns 200 OK with status message for load balancer health checks.
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ReadinessCheck provides a readiness probe that verifies database connectivity.
// Returns 503 Service Unavailable if database operations fail.
func ReadinessCheck(database interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Simple DB ping check
		status := "ok"
		statusCode := http.StatusOK

		switch dbInstance := database.(type) {
		case *db.ChallengerDB:
			// Try a simple nonce check to verify DB is working
			_, err := dbInstance.HasSeenNonce("readiness-check")
			if err != nil {
				status = "database connection failed"
				statusCode = http.StatusServiceUnavailable
				log.Error().Err(err).Msg("Database readiness check failed")
			}
		case *db.SolverDB:
			_, err := dbInstance.HasSeenNonce("readiness-check")
			if err != nil {
				status = "database connection failed"
				statusCode = http.StatusServiceUnavailable
				log.Error().Err(err).Msg("Database readiness check failed")
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]string{"status": status})
	}
}
