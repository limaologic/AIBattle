package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"reverse-challenge-system/pkg/models"
)

func createTestSolverDB(t *testing.T) (*SolverDB, func()) {
	// Create temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_solver.db")

	db, err := NewSolverDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}

	return db, cleanup
}

func createTestPendingChallenge() *models.PendingChallenge {
	problem := map[string]interface{}{
		"type": "test",
		"data": "test_data",
	}
	problemJSON, _ := json.Marshal(problem)

	outputSpec := map[string]interface{}{
		"content_type": "text/plain",
		"schema":       map[string]string{"type": "string"},
	}
	outputSpecJSON, _ := json.Marshal(outputSpec)

	return &models.PendingChallenge{
		ID:            "pending_challenge_123",
		Problem:       problemJSON,
		OutputSpec:    outputSpecJSON,
		CallbackURL:   "https://challenger.example.com/callback/123",
		ReceivedAt:    time.Now(),
		Status:        "pending",
		AttemptCount:  0,
		NextRetryTime: time.Now(),
	}
}

func TestSolverDB_SaveChallenge(t *testing.T) {
	db, cleanup := createTestSolverDB(t)
	defer cleanup()

	challenge := createTestPendingChallenge()

	err := db.SaveChallenge(challenge)
	if err != nil {
		t.Fatalf("Failed to save challenge: %v", err)
	}

	// Verify it was saved by trying to get it
	retrieved, err := db.GetChallenge(challenge.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve challenge: %v", err)
	}

	if retrieved.ID != challenge.ID {
		t.Errorf("Expected ID %s, got %s", challenge.ID, retrieved.ID)
	}

	if retrieved.Status != challenge.Status {
		t.Errorf("Expected status %s, got %s", challenge.Status, retrieved.Status)
	}

	if retrieved.CallbackURL != challenge.CallbackURL {
		t.Errorf("Expected callback URL %s, got %s", challenge.CallbackURL, retrieved.CallbackURL)
	}
}

func TestSolverDB_GetChallenge(t *testing.T) {
	db, cleanup := createTestSolverDB(t)
	defer cleanup()

	// Test getting non-existent challenge
	retrieved, err := db.GetChallenge("non_existent")
	if err != nil {
		t.Fatalf("Unexpected error when getting non-existent challenge: %v", err)
	}
	if retrieved != nil {
		t.Error("Expected nil result for non-existent challenge")
	}

	// Create challenge
	challenge := createTestPendingChallenge()
	err = db.SaveChallenge(challenge)
	if err != nil {
		t.Fatalf("Failed to save challenge: %v", err)
	}

	// Test getting existing challenge
	retrieved, err = db.GetChallenge(challenge.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve challenge: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected challenge, got nil")
	}

	// Verify all fields
	if retrieved.ID != challenge.ID {
		t.Errorf("Expected ID %s, got %s", challenge.ID, retrieved.ID)
	}

	if string(retrieved.Problem) != string(challenge.Problem) {
		t.Errorf("Problem mismatch")
	}

	if string(retrieved.OutputSpec) != string(challenge.OutputSpec) {
		t.Errorf("OutputSpec mismatch")
	}

	if retrieved.CallbackURL != challenge.CallbackURL {
		t.Errorf("Expected callback URL %s, got %s", challenge.CallbackURL, retrieved.CallbackURL)
	}

	if retrieved.Status != challenge.Status {
		t.Errorf("Expected status %s, got %s", challenge.Status, retrieved.Status)
	}

	if retrieved.AttemptCount != challenge.AttemptCount {
		t.Errorf("Expected attempt count %d, got %d", challenge.AttemptCount, retrieved.AttemptCount)
	}
}

func TestSolverDB_UpdateChallengeStatus(t *testing.T) {
	db, cleanup := createTestSolverDB(t)
	defer cleanup()

	// Create challenge
	challenge := createTestPendingChallenge()
	err := db.SaveChallenge(challenge)
	if err != nil {
		t.Fatalf("Failed to save challenge: %v", err)
	}

	// Update status
	newStatus := "processing"
	newAttemptCount := 1
	newNextRetryTime := time.Now().Add(1 * time.Hour)

	err = db.UpdateChallengeStatus(challenge.ID, newStatus, newAttemptCount, newNextRetryTime)
	if err != nil {
		t.Fatalf("Failed to update challenge status: %v", err)
	}

	// Verify update
	updated, err := db.GetChallenge(challenge.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated challenge: %v", err)
	}

	if updated.Status != newStatus {
		t.Errorf("Expected status %s, got %s", newStatus, updated.Status)
	}

	if updated.AttemptCount != newAttemptCount {
		t.Errorf("Expected attempt count %d, got %d", newAttemptCount, updated.AttemptCount)
	}

	// Check retry time (within reasonable margin due to precision)
	timeDiff := updated.NextRetryTime.Sub(newNextRetryTime)
	if timeDiff > time.Second || timeDiff < -time.Second {
		t.Errorf("NextRetryTime mismatch, expected around %v, got %v", newNextRetryTime, updated.NextRetryTime)
	}
}

func TestSolverDB_UpdateNonExistentChallenge(t *testing.T) {
	db, cleanup := createTestSolverDB(t)
	defer cleanup()

	// Try to update non-existent challenge - should not fail (UPDATE affects 0 rows)
	err := db.UpdateChallengeStatus("non_existent", "processing", 1, time.Now())
	if err != nil {
		t.Fatalf("Unexpected error when updating non-existent challenge: %v", err)
	}
}

func TestSolverDB_GetPendingChallenges(t *testing.T) {
	db, cleanup := createTestSolverDB(t)
	defer cleanup()

	now := time.Now()

	// Create several challenges with different statuses and retry times
	challenges := []*models.PendingChallenge{
		{
			ID:            "pending_1",
			Problem:       json.RawMessage(`{"type": "test1"}`),
			OutputSpec:    json.RawMessage(`{"type": "string"}`),
			CallbackURL:   "https://example.com/1",
			ReceivedAt:    now.Add(-10 * time.Minute),
			Status:        "pending",
			AttemptCount:  0,
			NextRetryTime: now.Add(-5 * time.Minute),
		},
		{
			ID:            "processing_ready_for_retry",
			Problem:       json.RawMessage(`{"type": "test2"}`),
			OutputSpec:    json.RawMessage(`{"type": "string"}`),
			CallbackURL:   "https://example.com/2",
			ReceivedAt:    now.Add(-20 * time.Minute),
			Status:        "processing",
			AttemptCount:  1,
			NextRetryTime: now.Add(-1 * time.Minute), // Ready for retry
		},
		{
			ID:            "processing_not_ready",
			Problem:       json.RawMessage(`{"type": "test3"}`),
			OutputSpec:    json.RawMessage(`{"type": "string"}`),
			CallbackURL:   "https://example.com/3",
			ReceivedAt:    now.Add(-5 * time.Minute),
			Status:        "processing",
			AttemptCount:  1,
			NextRetryTime: now.Add(10 * time.Minute), // Not ready yet
		},
		{
			ID:            "completed",
			Problem:       json.RawMessage(`{"type": "test4"}`),
			OutputSpec:    json.RawMessage(`{"type": "string"}`),
			CallbackURL:   "https://example.com/4",
			ReceivedAt:    now.Add(-30 * time.Minute),
			Status:        "completed",
			AttemptCount:  1,
			NextRetryTime: now.Add(-25 * time.Minute),
		},
	}

	// Save all challenges
	for _, challenge := range challenges {
		err := db.SaveChallenge(challenge)
		if err != nil {
			t.Fatalf("Failed to save challenge %s: %v", challenge.ID, err)
		}
	}

	// Get pending challenges
	pending, err := db.GetPendingChallenges(10)
	if err != nil {
		t.Fatalf("Failed to get pending challenges: %v", err)
	}

	// Should return pending_1 and processing_ready_for_retry (2 challenges)
	expectedIDs := map[string]bool{
		"pending_1":                  true,
		"processing_ready_for_retry": true,
	}

	if len(pending) != 2 {
		t.Errorf("Expected 2 pending challenges, got %d", len(pending))
	}

	for _, challenge := range pending {
		if !expectedIDs[challenge.ID] {
			t.Errorf("Unexpected challenge ID in results: %s", challenge.ID)
		}
		delete(expectedIDs, challenge.ID)
	}

	if len(expectedIDs) > 0 {
		t.Errorf("Expected challenges not found in results: %v", expectedIDs)
	}
}

func TestSolverDB_GetPendingChallenges_WithLimit(t *testing.T) {
	db, cleanup := createTestSolverDB(t)
	defer cleanup()

	now := time.Now()

	// Create 5 pending challenges
	for i := 0; i < 5; i++ {
		challenge := &models.PendingChallenge{
			ID:            fmt.Sprintf("pending_%d", i),
			Problem:       json.RawMessage(`{"type": "test"}`),
			OutputSpec:    json.RawMessage(`{"type": "string"}`),
			CallbackURL:   fmt.Sprintf("https://example.com/%d", i),
			ReceivedAt:    now.Add(-time.Duration(i) * time.Minute),
			Status:        "pending",
			AttemptCount:  0,
			NextRetryTime: now.Add(-1 * time.Minute),
		}

		err := db.SaveChallenge(challenge)
		if err != nil {
			t.Fatalf("Failed to save challenge %d: %v", i, err)
		}
	}

	// Get with limit of 3
	pending, err := db.GetPendingChallenges(3)
	if err != nil {
		t.Fatalf("Failed to get pending challenges: %v", err)
	}

	if len(pending) != 3 {
		t.Errorf("Expected 3 challenges due to limit, got %d", len(pending))
	}

	// Should be ordered by received_at ASC (oldest first)
	for i := 0; i < len(pending)-1; i++ {
		if pending[i].ReceivedAt.After(pending[i+1].ReceivedAt) {
			t.Error("Challenges not ordered by ReceivedAt ASC")
		}
	}
}

func TestSolverDB_DeleteChallenge(t *testing.T) {
	db, cleanup := createTestSolverDB(t)
	defer cleanup()

	// Create challenge
	challenge := createTestPendingChallenge()
	err := db.SaveChallenge(challenge)
	if err != nil {
		t.Fatalf("Failed to save challenge: %v", err)
	}

	// Verify it exists
	retrieved, err := db.GetChallenge(challenge.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve challenge: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected challenge to exist")
	}

	// Delete it
	err = db.DeleteChallenge(challenge.ID)
	if err != nil {
		t.Fatalf("Failed to delete challenge: %v", err)
	}

	// Verify it's gone
	retrieved, err = db.GetChallenge(challenge.ID)
	if err != nil {
		t.Fatalf("Unexpected error when getting deleted challenge: %v", err)
	}
	if retrieved != nil {
		t.Error("Expected challenge to be deleted")
	}
}

func TestSolverDB_DeleteNonExistentChallenge(t *testing.T) {
	db, cleanup := createTestSolverDB(t)
	defer cleanup()

	// Try to delete non-existent challenge - should not fail
	err := db.DeleteChallenge("non_existent")
	if err != nil {
		t.Fatalf("Unexpected error when deleting non-existent challenge: %v", err)
	}
}

func TestSolverDB_NonceOperations(t *testing.T) {
	db, cleanup := createTestSolverDB(t)
	defer cleanup()

	nonce := "solver_nonce_123"

	// Check non-existent nonce
	seen, err := db.HasSeenNonce(nonce)
	if err != nil {
		t.Fatalf("Failed to check nonce: %v", err)
	}
	if seen {
		t.Error("Expected nonce to not be seen initially")
	}

	// Save nonce
	err = db.SaveNonce(nonce)
	if err != nil {
		t.Fatalf("Failed to save nonce: %v", err)
	}

	// Check nonce again - should be seen now
	seen, err = db.HasSeenNonce(nonce)
	if err != nil {
		t.Fatalf("Failed to check nonce: %v", err)
	}
	if !seen {
		t.Error("Expected nonce to be seen after saving")
	}
}

func TestSolverDB_CleanupOldNonces(t *testing.T) {
	db, cleanup := createTestSolverDB(t)
	defer cleanup()

	// Save a nonce
	nonce := "old_solver_nonce"
	err := db.SaveNonce(nonce)
	if err != nil {
		t.Fatalf("Failed to save nonce: %v", err)
	}

	// Verify it exists
	seen, _ := db.HasSeenNonce(nonce)
	if !seen {
		t.Fatal("Expected nonce to exist")
	}

	// Clean up nonces older than future time (should clean all)
	futureTime := time.Now().Add(1 * time.Hour)
	err = db.CleanupOldNonces(futureTime)
	if err != nil {
		t.Fatalf("Failed to cleanup old nonces: %v", err)
	}

	// Verify it's gone
	seen, _ = db.HasSeenNonce(nonce)
	if seen {
		t.Error("Expected nonce to be cleaned up")
	}
}

func TestSolverDB_Close(t *testing.T) {
	db, cleanup := createTestSolverDB(t)
	defer cleanup()

	err := db.Close()
	if err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	// Trying to use closed database should fail
	err = db.SaveNonce("test")
	if err == nil {
		t.Error("Expected error when using closed database")
	}
}

func TestSolverDB_SaveDuplicateChallenge(t *testing.T) {
	db, cleanup := createTestSolverDB(t)
	defer cleanup()

	challenge := createTestPendingChallenge()

	// Save first time - should succeed
	err := db.SaveChallenge(challenge)
	if err != nil {
		t.Fatalf("Failed to save challenge first time: %v", err)
	}

	// Try to save again with same ID - should fail due to PRIMARY KEY constraint
	err = db.SaveChallenge(challenge)
	if err == nil {
		t.Error("Expected error when saving duplicate challenge")
	}
}
