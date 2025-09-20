// Package models defines data structures for the Reverse Challenge System.
// This package contains API request/response models, database models, and validation types
// used throughout the challenger and solver services.
package models

import (
	"encoding/json"
	"time"
)

// API Requests and Responses

// SolveRequest represents the request sent from challenger to solver to process a challenge.
// Contains all necessary information for the solver to understand and execute the challenge.
type SolveRequest struct {
	APIVersion  string          `json:"api_version"`  // API version for compatibility checking
	ChallengeID string          `json:"challenge_id"` // Unique identifier for the challenge
	Problem     json.RawMessage `json:"problem"`      // Challenge-specific problem data (JSON)
	OutputSpec  json.RawMessage `json:"output_spec"`  // Expected output format specification (JSON)
	Constraints Constraints     `json:"constraints"`  // Execution constraints for the solver
	CallbackURL string          `json:"callback_url"` // URL where solver should send results
}

// Constraints defines execution limits and deadlines for challenge processing.
type Constraints struct {
	TimeoutMs  int   `json:"timeout_ms"`            // Maximum processing time in milliseconds
	DeadlineTs int64 `json:"deadline_ts,omitempty"` // Unix timestamp deadline for completion
}

// SolveResponse is the immediate response from solver when accepting a challenge.
// Returns a job ID for tracking the asynchronous processing.
type SolveResponse struct {
	Message     string `json:"message"`       // Human-readable response message
	SolverJobID string `json:"solver_job_id"` // Unique job identifier for tracking
}

// CallbackRequest represents the asynchronous result sent from solver to challenger.
// Contains the solution or error information after challenge processing is complete.
type CallbackRequest struct {
	APIVersion   string          `json:"api_version"`             // API version for compatibility
	ChallengeID  string          `json:"challenge_id"`            // Original challenge identifier
	SolverJobID  string          `json:"solver_job_id"`           // Job identifier from SolveResponse
	Status       string          `json:"status"`                  // Processing status: "success" or "failed"
	Answer       string          `json:"answer,omitempty"`        // Solution answer (only if successful)
	ErrorCode    string          `json:"error_code,omitempty"`    // Error code (only if failed)
	ErrorMessage string          `json:"error_message,omitempty"` // Human-readable error (only if failed)
	Metadata     json.RawMessage `json:"metadata,omitempty"`      // Additional solver metadata (JSON)
}

// CallbackResponse is the challenger's response to a callback request.
// Confirms receipt and indicates if this was a duplicate submission.
type CallbackResponse struct {
	Received    bool   `json:"received"`     // Whether the callback was successfully received
	ChallengeID string `json:"challenge_id"` // Echo back the challenge ID for confirmation
	Duplicate   bool   `json:"duplicate"`    // True if this callback was already processed
}

// Database Models

// ValidationRule defines how to validate a solver's answer against the expected solution.
// Contains the validation type, parameters, and the correct answer (stored only on challenger).
type ValidationRule struct {
	Type   string          `json:"type"`             // Validation type: "ExactMatch", "NumericTolerance", or "Regex"
	Params json.RawMessage `json:"params,omitempty"` // Type-specific validation parameters (JSON)
	Answer string          `json:"answer"`           // Correct answer - stored locally, never sent to solver
}

// Challenge represents a complete challenge stored in the challenger database.
// Contains all information needed to validate solver responses.
type Challenge struct {
	ID             string          `json:"id" db:"id"`                           // Unique challenge identifier
	Type           string          `json:"type" db:"type"`                       // Challenge type (CAPTCHA, Math, Text)
	Problem        json.RawMessage `json:"problem" db:"problem"`                 // Problem data sent to solver (JSON)
	OutputSpec     json.RawMessage `json:"output_spec" db:"output_spec"`         // Expected output format (JSON)
	ValidationRule ValidationRule  `json:"validation_rule" db:"validation_rule"` // How to validate answers
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`           // Challenge creation timestamp
}

// Result stores the outcome of a challenge after receiving a solver's callback.
// Tracks validation results and solver performance metrics.
type Result struct {
	ID             int64           `json:"id" db:"id"`                           // Auto-increment primary key
	ChallengeID    string          `json:"challenge_id" db:"challenge_id"`       // Reference to original challenge
	RequestID      string          `json:"request_id" db:"request_id"`           // X-Request-ID for idempotency
	SolverJobID    string          `json:"solver_job_id" db:"solver_job_id"`     // Job ID from solver response
	Status         string          `json:"status" db:"status"`                   // Solver status: "success" or "failed"
	ReceivedAnswer string          `json:"received_answer" db:"received_answer"` // Answer provided by solver
	IsCorrect      bool            `json:"is_correct" db:"is_correct"`           // Whether answer passed validation
	SolverAddress  string          `json:"solver_address" db:"solver_address"`   // Sui address of the solver
	ComputeTimeMs  int             `json:"compute_time_ms" db:"compute_time_ms"` // Solver-reported processing time
	SolverMetadata json.RawMessage `json:"solver_metadata" db:"solver_metadata"` // Additional solver data (JSON)
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`           // Result creation timestamp
}

// WebhookAudit provides an audit trail of all callback requests received.
// Used for debugging, monitoring, and security analysis.
type WebhookAudit struct {
	ID          int64     `json:"id" db:"id"`                     // Auto-increment primary key
	ChallengeID string    `json:"challenge_id" db:"challenge_id"` // Related challenge identifier
	RequestID   string    `json:"request_id" db:"request_id"`     // Request ID for correlation
	Headers     string    `json:"headers" db:"headers"`           // JSON-encoded request headers
	BodyHash    string    `json:"body_hash" db:"body_hash"`       // SHA-256 hash of request body
	StatusCode  int       `json:"status_code" db:"status_code"`   // HTTP response status code
	CreatedAt   time.Time `json:"created_at" db:"created_at"`     // Audit record creation time
}

// PendingChallenge represents a challenge queued for processing in the solver database.
// Includes retry logic state and processing status tracking.
type PendingChallenge struct {
	ID            string          `json:"id" db:"id"`                           // Challenge identifier
	Problem       json.RawMessage `json:"problem" db:"problem"`                 // Problem data to process (JSON)
	OutputSpec    json.RawMessage `json:"output_spec" db:"output_spec"`         // Expected output format (JSON)
	CallbackURL   string          `json:"callback_url" db:"callback_url"`       // URL to send results to
	ReceivedAt    time.Time       `json:"received_at" db:"received_at"`         // When challenge was received
	Status        string          `json:"status" db:"status"`                   // Processing status: "pending", "processing", "completed", or "failed"
	AttemptCount  int             `json:"attempt_count" db:"attempt_count"`     // Number of processing attempts made
	NextRetryTime time.Time       `json:"next_retry_time" db:"next_retry_time"` // When to retry if processing failed
}

// SeenNonce tracks used nonces to prevent replay attacks in HMAC authentication.
// Each nonce can only be used once within the configured time window.
type SeenNonce struct {
	Nonce  string    `json:"nonce" db:"nonce"`     // Unique nonce string (UUID)
	SeenAt time.Time `json:"seen_at" db:"seen_at"` // When this nonce was first seen
}

// Contract represents a deployed smart contract in the database.
// Tracks deployment information and metadata for idempotent deployments.
type Contract struct {
	ID           int64                  `json:"id" db:"id"`                       // Auto-increment primary key
	Name         string                 `json:"name" db:"name"`                   // Contract name identifier
	Address      string                 `json:"address" db:"address"`             // Deployed contract address or package ID
	Network      string                 `json:"network" db:"network"`             // Network/chain identifier
	ChainID      string                 `json:"chain_id" db:"chain_id"`           // Chain ID for uniqueness
	TxHash       string                 `json:"tx_hash" db:"tx_hash"`             // Deployment transaction hash
	DeployedAt   time.Time              `json:"deployed_at" db:"deployed_at"`     // Deployment timestamp
	ContractType string                 `json:"contract_type" db:"contract_type"` // Contract type (sui_move_package, ethereum_contract)
	Metadata     map[string]interface{} `json:"metadata" db:"metadata"`           // Additional contract metadata (JSON)
}

// Error Response

// ErrorResponse represents a standardized error response structure.
// Used to return consistent error information to API clients.
type ErrorResponse struct {
	Error ErrorDetails `json:"error"` // Detailed error information
}

// ErrorDetails contains specific error information including codes and messages.
type ErrorDetails struct {
	Code      string `json:"code"`                 // Machine-readable error code
	Message   string `json:"message"`              // Human-readable error description
	RequestID string `json:"request_id,omitempty"` // Request ID for error correlation
}

// Validation Rule Parameters

// ExactMatchParams configures exact string matching validation.
// Used when answers must match precisely with optional case sensitivity.
type ExactMatchParams struct {
	CaseSensitive bool `json:"case_sensitive"` // Whether to perform case-sensitive comparison
}

// NumericToleranceParams configures numeric validation with tolerance.
// Used for mathematical answers that may have slight precision differences.
type NumericToleranceParams struct {
	Tolerance float64 `json:"tolerance"` // Maximum allowed absolute difference from correct answer
}

// RegexParams configures regular expression pattern matching validation.
// Used for flexible text pattern validation.
type RegexParams struct {
	Pattern string `json:"pattern"` // Regular expression pattern to match against
}

// Solver Metadata Examples

// SolverMetadata represents additional information provided by solvers in callbacks.
// This is a flexible structure that solvers can use to report processing details.
type SolverMetadata struct {
	ComputeTimeMs int                    `json:"compute_time_ms,omitempty"` // Processing time in milliseconds
	Algorithm     string                 `json:"algorithm,omitempty"`       // Algorithm or method used
	Confidence    float64                `json:"confidence,omitempty"`      // Solver's confidence in the answer (0.0-1.0)
	AttemptCount  int                    `json:"attempt_count,omitempty"`   // Number of attempts made
	Resource      map[string]interface{} `json:"resource,omitempty"`        // Resource usage information
}
