package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"reverse-challenge-system/pkg/models"

	_ "github.com/mattn/go-sqlite3"
)

// SolverDB provides database operations for the solver service.
// Manages pending challenges, retry logic, and nonce tracking for HMAC authentication.
type SolverDB struct {
	db *sql.DB // SQLite database connection
}

// NewSolverDB creates and initializes a new solver database instance.
// Opens SQLite connection, enables WAL mode, and creates required tables.
func NewSolverDB(dbPath string) (*SolverDB, error) {
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

	sdb := &SolverDB{db: db}
	if err := sdb.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return sdb, nil
}

// createTables initializes all required database tables for solver operations.
// Creates tables for pending challenges, nonce tracking, and performance indexes.
func (s *SolverDB) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS pending_challenges (
			id TEXT PRIMARY KEY,
			problem TEXT NOT NULL,
			output_spec TEXT NOT NULL,
			callback_url TEXT NOT NULL,
			received_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			status TEXT NOT NULL DEFAULT 'pending',
			attempt_count INTEGER DEFAULT 0,
			next_retry_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS seen_nonces (
			nonce TEXT PRIMARY KEY,
			seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS ix_pending_status_retry ON pending_challenges(status, next_retry_time)`,
		`CREATE INDEX IF NOT EXISTS ix_seen_nonces_seen_at ON seen_nonces(seen_at)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", query, err)
		}
	}

	return nil
}

// SaveChallenge stores a new challenge for processing by the solver workers.
// Converts JSON fields to strings for database storage and sets initial status.
func (s *SolverDB) SaveChallenge(challenge *models.PendingChallenge) error {
	_, err := s.db.Exec(`
		INSERT INTO pending_challenges (id, problem, output_spec, callback_url, 
			received_at, status, attempt_count, next_retry_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		challenge.ID, string(challenge.Problem), string(challenge.OutputSpec),
		challenge.CallbackURL, challenge.ReceivedAt, challenge.Status,
		challenge.AttemptCount, challenge.NextRetryTime)

	if err != nil {
		return fmt.Errorf("failed to save challenge: %w", err)
	}

	return nil
}

// GetChallenge retrieves a specific challenge by ID from the solver database.
// Reconstructs the challenge with proper JSON field conversion from stored text.
func (s *SolverDB) GetChallenge(id string) (*models.PendingChallenge, error) {
	row := s.db.QueryRow(`
		SELECT id, problem, output_spec, callback_url, received_at, status, 
			attempt_count, next_retry_time
		FROM pending_challenges WHERE id = ?`, id)

	var challenge models.PendingChallenge
	var problemText, outputSpecText string

	err := row.Scan(&challenge.ID, &problemText, &outputSpecText,
		&challenge.CallbackURL, &challenge.ReceivedAt, &challenge.Status,
		&challenge.AttemptCount, &challenge.NextRetryTime)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get challenge: %w", err)
	}

	// Convert text back to json.RawMessage
	challenge.Problem = json.RawMessage(problemText)
	challenge.OutputSpec = json.RawMessage(outputSpecText)

	return &challenge, nil
}

func (s *SolverDB) UpdateChallengeStatus(id, status string, attemptCount int, nextRetryTime time.Time) error {
	_, err := s.db.Exec(`
		UPDATE pending_challenges 
		SET status = ?, attempt_count = ?, next_retry_time = ?
		WHERE id = ?`, status, attemptCount, nextRetryTime, id)

	if err != nil {
		return fmt.Errorf("failed to update challenge status: %w", err)
	}

	return nil
}

// GetPendingChallenges retrieves challenges ready for processing by worker threads.
// Returns challenges in pending status or failed challenges ready for retry.
func (s *SolverDB) GetPendingChallenges(limit int) ([]*models.PendingChallenge, error) {
	rows, err := s.db.Query(`
		SELECT id, problem, output_spec, callback_url, received_at, status, 
			attempt_count, next_retry_time
		FROM pending_challenges 
		WHERE (status = 'pending' OR (status = 'processing' AND next_retry_time <= ?))
		ORDER BY received_at ASC
		LIMIT ?`, time.Now(), limit)

	if err != nil {
		return nil, fmt.Errorf("failed to get pending challenges: %w", err)
	}
	defer rows.Close()

	var challenges []*models.PendingChallenge

	for rows.Next() {
		var challenge models.PendingChallenge
		var problemText, outputSpecText string
		err := rows.Scan(&challenge.ID, &problemText, &outputSpecText,
			&challenge.CallbackURL, &challenge.ReceivedAt, &challenge.Status,
			&challenge.AttemptCount, &challenge.NextRetryTime)

		if err != nil {
			return nil, fmt.Errorf("failed to scan challenge: %w", err)
		}

		// Convert text back to json.RawMessage
		challenge.Problem = json.RawMessage(problemText)
		challenge.OutputSpec = json.RawMessage(outputSpecText)

		challenges = append(challenges, &challenge)
	}

	return challenges, nil
}

func (s *SolverDB) DeleteChallenge(id string) error {
	_, err := s.db.Exec("DELETE FROM pending_challenges WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete challenge: %w", err)
	}
	return nil
}

func (s *SolverDB) HasSeenNonce(nonce string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM seen_nonces WHERE nonce = ?", nonce).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check nonce: %w", err)
	}
	return count > 0, nil
}

func (s *SolverDB) SaveNonce(nonce string) error {
	_, err := s.db.Exec("INSERT INTO seen_nonces (nonce, seen_at) VALUES (?, ?)",
		nonce, time.Now())
	if err != nil {
		return fmt.Errorf("failed to save nonce: %w", err)
	}
	return nil
}

func (s *SolverDB) CleanupOldNonces(olderThan time.Time) error {
	_, err := s.db.Exec("DELETE FROM seen_nonces WHERE seen_at < ?", olderThan)
	if err != nil {
		return fmt.Errorf("failed to cleanup old nonces: %w", err)
	}
	return nil
}

func (s *SolverDB) Close() error {
	return s.db.Close()
}
