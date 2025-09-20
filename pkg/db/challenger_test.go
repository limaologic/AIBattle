package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"reverse-challenge-system/pkg/models"
)

func createTestChallengerDB(t *testing.T) (*ChallengerDB, func()) {
	// Create temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_challenger.db")

	db, err := NewChallengerDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}

	return db, cleanup
}

func createTestChallenge() *models.Challenge {
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

	validation := models.ValidationRule{
		Type:   "ExactMatch",
		Answer: "expected_answer",
		Params: json.RawMessage(`{"case_sensitive": true}`),
	}

	return &models.Challenge{
		ID:             "test_challenge_123",
		Type:           "test",
		Problem:        problemJSON,
		OutputSpec:     outputSpecJSON,
		ValidationRule: validation,
		CreatedAt:      time.Now(),
	}
}

func TestChallengerDB_CreateChallenge(t *testing.T) {
	db, cleanup := createTestChallengerDB(t)
	defer cleanup()

	challenge := createTestChallenge()

	err := db.CreateChallenge(challenge)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	// Verify it was created by trying to get it
	retrieved, err := db.GetChallenge(challenge.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve challenge: %v", err)
	}

	if retrieved.ID != challenge.ID {
		t.Errorf("Expected ID %s, got %s", challenge.ID, retrieved.ID)
	}

	if retrieved.Type != challenge.Type {
		t.Errorf("Expected type %s, got %s", challenge.Type, retrieved.Type)
	}
}

func TestChallengerDB_GetChallenge(t *testing.T) {
	db, cleanup := createTestChallengerDB(t)
	defer cleanup()

	challenge := createTestChallenge()

	// Test getting non-existent challenge
	_, err := db.GetChallenge("non_existent")
	if err == nil {
		t.Error("Expected error when getting non-existent challenge")
	}

	// Create challenge
	err = db.CreateChallenge(challenge)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	// Test getting existing challenge
	retrieved, err := db.GetChallenge(challenge.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve challenge: %v", err)
	}

	// Verify all fields
	if retrieved.ID != challenge.ID {
		t.Errorf("Expected ID %s, got %s", challenge.ID, retrieved.ID)
	}

	if retrieved.Type != challenge.Type {
		t.Errorf("Expected type %s, got %s", challenge.Type, retrieved.Type)
	}

	if string(retrieved.Problem) != string(challenge.Problem) {
		t.Errorf("Problem mismatch")
	}

	if string(retrieved.OutputSpec) != string(challenge.OutputSpec) {
		t.Errorf("OutputSpec mismatch")
	}

	if retrieved.ValidationRule.Type != challenge.ValidationRule.Type {
		t.Errorf("ValidationRule type mismatch")
	}

	if retrieved.ValidationRule.Answer != challenge.ValidationRule.Answer {
		t.Errorf("ValidationRule answer mismatch")
	}
}

func TestChallengerDB_CreateDuplicateChallenge(t *testing.T) {
	db, cleanup := createTestChallengerDB(t)
	defer cleanup()

	challenge := createTestChallenge()

	// Create first time - should succeed
	err := db.CreateChallenge(challenge)
	if err != nil {
		t.Fatalf("Failed to create challenge first time: %v", err)
	}

	// Try to create again with same ID - should fail
	err = db.CreateChallenge(challenge)
	if err == nil {
		t.Error("Expected error when creating duplicate challenge")
	}
}

func TestChallengerDB_SaveResult(t *testing.T) {
	db, cleanup := createTestChallengerDB(t)
	defer cleanup()

	// Create challenge first
	challenge := createTestChallenge()
	err := db.CreateChallenge(challenge)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	result := &models.Result{
		ChallengeID:    challenge.ID,
		RequestID:      "req_123",
		SolverJobID:    "job_456",
		Status:         "success",
		ReceivedAnswer: "test_answer",
		IsCorrect:      true,
		ComputeTimeMs:  1500,
		SolverMetadata: json.RawMessage(`{"algorithm": "test"}`),
		CreatedAt:      time.Now(),
	}

	err = db.SaveResult(result)
	if err != nil {
		t.Fatalf("Failed to save result: %v", err)
	}
}

func TestChallengerDB_GetResult(t *testing.T) {
	db, cleanup := createTestChallengerDB(t)
	defer cleanup()

	// Create challenge first
	challenge := createTestChallenge()
	err := db.CreateChallenge(challenge)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	// Test getting non-existent result
	result, err := db.GetResult("non_existent", "req_123")
	if err != nil {
		t.Fatalf("Unexpected error when getting non-existent result: %v", err)
	}
	if result != nil {
		t.Error("Expected nil result for non-existent challenge")
	}

	// Create result
	testResult := &models.Result{
		ChallengeID:    challenge.ID,
		RequestID:      "req_123",
		SolverJobID:    "job_456",
		Status:         "success",
		ReceivedAnswer: "test_answer",
		IsCorrect:      true,
		ComputeTimeMs:  1500,
		SolverMetadata: json.RawMessage(`{"algorithm": "test"}`),
		CreatedAt:      time.Now(),
	}

	err = db.SaveResult(testResult)
	if err != nil {
		t.Fatalf("Failed to save result: %v", err)
	}

	// Get result
	retrieved, err := db.GetResult(challenge.ID, "req_123")
	if err != nil {
		t.Fatalf("Failed to get result: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected result, got nil")
	}

	// Verify fields
	if retrieved.ChallengeID != testResult.ChallengeID {
		t.Errorf("Expected ChallengeID %s, got %s", testResult.ChallengeID, retrieved.ChallengeID)
	}

	if retrieved.RequestID != testResult.RequestID {
		t.Errorf("Expected RequestID %s, got %s", testResult.RequestID, retrieved.RequestID)
	}

	if retrieved.Status != testResult.Status {
		t.Errorf("Expected Status %s, got %s", testResult.Status, retrieved.Status)
	}

	if retrieved.IsCorrect != testResult.IsCorrect {
		t.Errorf("Expected IsCorrect %v, got %v", testResult.IsCorrect, retrieved.IsCorrect)
	}
}

func TestChallengerDB_SaveWebhookAudit(t *testing.T) {
	db, cleanup := createTestChallengerDB(t)
	defer cleanup()

	audit := &models.WebhookAudit{
		ChallengeID: "test_challenge",
		RequestID:   "req_123",
		Headers:     `{"Content-Type": "application/json"}`,
		BodyHash:    "abcd1234",
		StatusCode:  200,
		CreatedAt:   time.Now(),
	}

	err := db.SaveWebhookAudit(audit)
	if err != nil {
		t.Fatalf("Failed to save webhook audit: %v", err)
	}
}

func TestChallengerDB_NonceOperations(t *testing.T) {
	db, cleanup := createTestChallengerDB(t)
	defer cleanup()

	nonce := "test_nonce_123"

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

	// Try to save the same nonce again - should not fail (multiple saves allowed)
	err = db.SaveNonce(nonce)
	if err != nil {
		t.Fatalf("Failed to save nonce again: %v", err)
	}
}

func TestChallengerDB_CleanupOldNonces(t *testing.T) {
	db, cleanup := createTestChallengerDB(t)
	defer cleanup()

	// Save a few nonces with different timestamps
	nonce1 := "old_nonce"
	nonce2 := "new_nonce"

	err := db.SaveNonce(nonce1)
	if err != nil {
		t.Fatalf("Failed to save nonce1: %v", err)
	}

	err = db.SaveNonce(nonce2)
	if err != nil {
		t.Fatalf("Failed to save nonce2: %v", err)
	}

	// Verify both exist
	seen1, _ := db.HasSeenNonce(nonce1)
	seen2, _ := db.HasSeenNonce(nonce2)

	if !seen1 || !seen2 {
		t.Fatal("Expected both nonces to exist")
	}

	// Clean up nonces older than future time (should clean all)
	futureTime := time.Now().Add(1 * time.Hour)
	err = db.CleanupOldNonces(futureTime)
	if err != nil {
		t.Fatalf("Failed to cleanup old nonces: %v", err)
	}

	// Verify they're gone
	seen1, _ = db.HasSeenNonce(nonce1)
	seen2, _ = db.HasSeenNonce(nonce2)

	if seen1 || seen2 {
		t.Error("Expected nonces to be cleaned up")
	}
}

func TestChallengerDB_Close(t *testing.T) {
	db, cleanup := createTestChallengerDB(t)
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

func TestChallengerDB_ResultUniqueness(t *testing.T) {
	db, cleanup := createTestChallengerDB(t)
	defer cleanup()

	// Create challenge first
	challenge := createTestChallenge()
	err := db.CreateChallenge(challenge)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	result := &models.Result{
		ChallengeID:    challenge.ID,
		RequestID:      "req_123",
		SolverJobID:    "job_456",
		Status:         "success",
		ReceivedAnswer: "test_answer",
		IsCorrect:      true,
		ComputeTimeMs:  1500,
		CreatedAt:      time.Now(),
	}

	// Save first time - should succeed
	err = db.SaveResult(result)
	if err != nil {
		t.Fatalf("Failed to save result first time: %v", err)
	}

	// Try to save again with same challenge_id and request_id - should fail due to UNIQUE constraint
	err = db.SaveResult(result)
	if err == nil {
		t.Error("Expected error when saving duplicate result")
	}
}

func TestChallengerDB_SaveResultWithDuplicateCheck(t *testing.T) {
	db, cleanup := createTestChallengerDB(t)
	defer cleanup()

	// Create challenge first
	challenge := createTestChallenge()
	err := db.CreateChallenge(challenge)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	result := &models.Result{
		ChallengeID:    challenge.ID,
		RequestID:      "req_123",
		SolverJobID:    "job_456",
		Status:         "success",
		ReceivedAnswer: "test_answer",
		IsCorrect:      true,
		ComputeTimeMs:  1500,
		CreatedAt:      time.Now(),
	}

	// Save first time - should succeed and return true (was inserted)
	wasInserted, err := db.SaveResultWithDuplicateCheck(result)
	if err != nil {
		t.Fatalf("Failed to save result first time: %v", err)
	}
	if !wasInserted {
		t.Error("Expected wasInserted=true for first insert")
	}

	// Try to save again with same challenge_id and request_id - should succeed but return false (was duplicate)
	wasInserted, err = db.SaveResultWithDuplicateCheck(result)
	if err != nil {
		t.Fatalf("Failed to save duplicate result: %v", err)
	}
	if wasInserted {
		t.Error("Expected wasInserted=false for duplicate insert")
	}

	// Verify the result still exists and can be retrieved
	retrieved, err := db.GetResult(challenge.ID, "req_123")
	if err != nil {
		t.Fatalf("Failed to get result: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected result to exist after duplicate insert attempt")
	}

	// Verify there's only one result (no duplicates were created)
	// We can verify this by checking the ID is the same
	if retrieved.ChallengeID != result.ChallengeID || retrieved.RequestID != result.RequestID {
		t.Error("Result data mismatch - possible duplicate was created")
	}
}
