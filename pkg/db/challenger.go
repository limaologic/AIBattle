// Package db provides database access layer for the Reverse Challenge System.
// Implements SQLite-based storage for challenges, results, and audit data.
// This file contains the challenger-specific database operations.
package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"reverse-challenge-system/pkg/models"

	_ "github.com/mattn/go-sqlite3"
)

// ChallengerDB provides database operations for the challenger service.
// Manages challenges, results, webhook audits, and nonce tracking for replay protection.
type ChallengerDB struct {
	db *sql.DB // SQLite database connection
}

// NewChallengerDB creates and initializes a new challenger database instance.
// Opens SQLite connection, enables WAL mode for better concurrency, and creates required tables.
// Returns configured database ready for challenger operations.
func NewChallengerDB(dbPath string) (*ChallengerDB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent access and set busy timeout
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	cdb := &ChallengerDB{db: db}
	if err := cdb.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return cdb, nil
}

// createTables initializes all required database tables for challenger operations.
// Creates tables for challenges, results, webhook audits, and nonce tracking.
func (c *ChallengerDB) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS challenges (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			problem TEXT NOT NULL,
			output_spec TEXT NOT NULL,
			validation_rule TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			challenge_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			solver_job_id TEXT,
			status TEXT NOT NULL,
			received_answer TEXT,
			is_correct BOOLEAN,
			solver_address TEXT,
			compute_time_ms INTEGER,
			solver_metadata TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (challenge_id) REFERENCES challenges(id),
			UNIQUE (challenge_id, request_id)
		)`,
		`CREATE TABLE IF NOT EXISTS webhooks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			challenge_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			headers TEXT,
			body_hash TEXT,
			status_code INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS seen_nonces (
			nonce TEXT PRIMARY KEY,
			seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS contracts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			address TEXT NOT NULL,
			network TEXT NOT NULL,
			chain_id TEXT NOT NULL,
			tx_hash TEXT NOT NULL,
			deployed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			contract_type TEXT NOT NULL,
			metadata TEXT,
			UNIQUE (name, chain_id)
		)`,
		`CREATE INDEX IF NOT EXISTS ix_results_cid_created ON results(challenge_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS ix_seen_nonces_seen_at ON seen_nonces(seen_at)`,
		`CREATE INDEX IF NOT EXISTS ix_contracts_name_chain ON contracts(name, chain_id)`,
	}

	for _, query := range queries {
		if _, err := c.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", query, err)
		}
	}

	return nil
}

// CreateChallenge stores a new challenge in the database.
// Serializes validation rules to JSON and handles the complete challenge lifecycle.
func (c *ChallengerDB) CreateChallenge(challenge *models.Challenge) error {
	validationRuleJSON, err := json.Marshal(challenge.ValidationRule)
	if err != nil {
		return fmt.Errorf("failed to marshal validation rule: %w", err)
	}

	_, err = c.db.Exec(`
		INSERT INTO challenges (id, type, problem, output_spec, validation_rule, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		challenge.ID, challenge.Type, string(challenge.Problem),
		string(challenge.OutputSpec), string(validationRuleJSON), challenge.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert challenge: %w", err)
	}

	return nil
}

// GetChallenge retrieves a challenge by its ID from the database.
// Reconstructs the challenge object with proper JSON deserialization of validation rules.
func (c *ChallengerDB) GetChallenge(id string) (*models.Challenge, error) {
	row := c.db.QueryRow(`
		SELECT id, type, problem, output_spec, validation_rule, created_at
		FROM challenges WHERE id = ?`, id)

	var challenge models.Challenge
	var problemText, outputSpecText, validationRuleJSON string

	err := row.Scan(&challenge.ID, &challenge.Type, &problemText,
		&outputSpecText, &validationRuleJSON, &challenge.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get challenge: %w", err)
	}

	// Convert text back to json.RawMessage
	challenge.Problem = json.RawMessage(problemText)
	challenge.OutputSpec = json.RawMessage(outputSpecText)

	if err := json.Unmarshal([]byte(validationRuleJSON), &challenge.ValidationRule); err != nil {
		return nil, fmt.Errorf("failed to unmarshal validation rule: %w", err)
	}

	return &challenge, nil
}

// SaveResult stores a challenge result in the database.
// Handles serialization of solver metadata and prevents duplicate insertions.
func (c *ChallengerDB) SaveResult(result *models.Result) error {
	metadataJSON := ""
	if result.SolverMetadata != nil {
		metadataJSON = string(result.SolverMetadata)
	}

	_, err := c.db.Exec(`
		INSERT INTO results (challenge_id, request_id, solver_job_id, status,
			received_answer, is_correct, solver_address, compute_time_ms, solver_metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		result.ChallengeID, result.RequestID, result.SolverJobID, result.Status,
		result.ReceivedAnswer, result.IsCorrect, result.SolverAddress, result.ComputeTimeMs,
		metadataJSON, result.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to save result: %w", err)
	}

	return nil
}

// SaveResultWithDuplicateCheck saves a result and returns insertion status.
// Returns true if the result was newly inserted, false if it was a duplicate.
// Implements idempotent result storage for reliable callback handling.
func (c *ChallengerDB) SaveResultWithDuplicateCheck(result *models.Result) (bool, error) {
	metadataJSON := ""
	if result.SolverMetadata != nil {
		metadataJSON = string(result.SolverMetadata)
	}

	res, err := c.db.Exec(`
		INSERT OR IGNORE INTO results (challenge_id, request_id, solver_job_id, status,
			received_answer, is_correct, solver_address, compute_time_ms, solver_metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		result.ChallengeID, result.RequestID, result.SolverJobID, result.Status,
		result.ReceivedAnswer, result.IsCorrect, result.SolverAddress, result.ComputeTimeMs,
		metadataJSON, result.CreatedAt)

	if err != nil {
		return false, fmt.Errorf("failed to save result: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to get rows affected: %w", err)
	}

	// If no rows were affected, it means the record already existed and was ignored
	return rowsAffected > 0, nil
}

// GetResult retrieves a stored result by challenge ID and request ID.
// Returns nil if no result is found, otherwise returns the complete result with metadata.
func (c *ChallengerDB) GetResult(challengeID, requestID string) (*models.Result, error) {
	row := c.db.QueryRow(`
		SELECT id, challenge_id, request_id, solver_job_id, status, received_answer,
			is_correct, solver_address, compute_time_ms, solver_metadata, created_at
		FROM results WHERE challenge_id = ? AND request_id = ?`, challengeID, requestID)

	var result models.Result
	var metadataJSON sql.NullString
	var solverAddress sql.NullString

	err := row.Scan(&result.ID, &result.ChallengeID, &result.RequestID, &result.SolverJobID,
		&result.Status, &result.ReceivedAnswer, &result.IsCorrect, &solverAddress, &result.ComputeTimeMs,
		&metadataJSON, &result.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get result: %w", err)
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		result.SolverMetadata = json.RawMessage(metadataJSON.String)
	}

	if solverAddress.Valid {
		result.SolverAddress = solverAddress.String
	}

	return &result, nil
}

// SaveWebhookAudit stores audit information for webhook callbacks.
// Used for debugging, monitoring, and security analysis of incoming callbacks.
func (c *ChallengerDB) SaveWebhookAudit(audit *models.WebhookAudit) error {
	_, err := c.db.Exec(`
		INSERT INTO webhooks (challenge_id, request_id, headers, body_hash, status_code, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		audit.ChallengeID, audit.RequestID, audit.Headers, audit.BodyHash,
		audit.StatusCode, audit.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to save webhook audit: %w", err)
	}

	return nil
}

func (c *ChallengerDB) HasSeenNonce(nonce string) (bool, error) {
	var count int
	err := c.db.QueryRow("SELECT COUNT(*) FROM seen_nonces WHERE nonce = ?", nonce).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check nonce: %w", err)
	}
	return count > 0, nil
}

func (c *ChallengerDB) SaveNonce(nonce string) error {
	_, err := c.db.Exec("INSERT OR IGNORE INTO seen_nonces (nonce, seen_at) VALUES (?, ?)",
		nonce, time.Now())
	if err != nil {
		return fmt.Errorf("failed to save nonce: %w", err)
	}
	return nil
}

func (c *ChallengerDB) CleanupOldNonces(olderThan time.Time) error {
	_, err := c.db.Exec("DELETE FROM seen_nonces WHERE seen_at < ?", olderThan)
	if err != nil {
		return fmt.Errorf("failed to cleanup old nonces: %w", err)
	}
	return nil
}

// SaveContract stores a contract deployment record in the database.
// Handles JSON serialization of metadata and enforces uniqueness by (name, chain_id).
func (c *ChallengerDB) SaveContract(ctx context.Context, contract *models.Contract) error {
	metadataJSON, err := json.Marshal(contract.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal contract metadata: %w", err)
	}

	_, err = c.db.Exec(`
		INSERT OR REPLACE INTO contracts (name, address, network, chain_id, tx_hash, deployed_at, contract_type, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		contract.Name, contract.Address, contract.Network, contract.ChainID,
		contract.TxHash, contract.DeployedAt, contract.ContractType, string(metadataJSON))

	if err != nil {
		return fmt.Errorf("failed to save contract: %w", err)
	}

	return nil
}

// GetContractByName retrieves a contract by name and chain ID from the database.
// Returns the contract with deserialized metadata or an error if not found.
func (c *ChallengerDB) GetContractByName(ctx context.Context, name, chainID string) (*models.Contract, error) {
	row := c.db.QueryRow(`
		SELECT id, name, address, network, chain_id, tx_hash, deployed_at, contract_type, metadata
		FROM contracts WHERE name = ? AND chain_id = ?`, name, chainID)

	var contract models.Contract
	var metadataJSON string

	err := row.Scan(&contract.ID, &contract.Name, &contract.Address, &contract.Network,
		&contract.ChainID, &contract.TxHash, &contract.DeployedAt, &contract.ContractType, &metadataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("contract not found")
		}
		return nil, fmt.Errorf("failed to get contract: %w", err)
	}

	if err := json.Unmarshal([]byte(metadataJSON), &contract.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract metadata: %w", err)
	}

	return &contract, nil
}

// ListContracts retrieves all contracts for a given chain ID.
// Returns a slice of contracts with deserialized metadata.
func (c *ChallengerDB) ListContracts(ctx context.Context, chainID string) ([]*models.Contract, error) {
	rows, err := c.db.Query(`
		SELECT id, name, address, network, chain_id, tx_hash, deployed_at, contract_type, metadata
		FROM contracts WHERE chain_id = ? ORDER BY deployed_at DESC`, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to query contracts: %w", err)
	}
	defer rows.Close()

	var contracts []*models.Contract
	for rows.Next() {
		var contract models.Contract
		var metadataJSON string

		err := rows.Scan(&contract.ID, &contract.Name, &contract.Address, &contract.Network,
			&contract.ChainID, &contract.TxHash, &contract.DeployedAt, &contract.ContractType, &metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contract: %w", err)
		}

		if err := json.Unmarshal([]byte(metadataJSON), &contract.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal contract metadata: %w", err)
		}

		contracts = append(contracts, &contract)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating contracts: %w", err)
	}

	return contracts, nil
}

func (c *ChallengerDB) Close() error {
	return c.db.Close()
}
