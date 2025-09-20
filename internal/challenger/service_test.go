package challenger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"reverse-challenge-system/pkg/config"
	"reverse-challenge-system/pkg/logger"

	"github.com/rs/zerolog"
)

func TestWriteDigestToFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "challenger-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create service with test config
	cfg := &config.Config{
		TxDigestFile: filepath.Join(tmpDir, "digest.txt"),
	}
	service := &Service{
		config: cfg,
	}

	// Test logger
	testLogger := logger.NewCategoryLogger("info", logger.Challenger, logger.General)

	testDigest := "0x1234567890abcdef1234567890abcdef12345678901234567890abcdef123456789"

	// Test writing digest to file
	err = service.writeDigestToFile(testDigest, testLogger)
	if err != nil {
		t.Fatalf("writeDigestToFile failed: %v", err)
	}

	// Verify file exists and has correct content
	content, err := os.ReadFile(cfg.TxDigestFile)
	if err != nil {
		t.Fatalf("failed to read digest file: %v", err)
	}

	expectedContent := testDigest + "\n"
	if string(content) != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, string(content))
	}

	// Verify file permissions
	fileInfo, err := os.Stat(cfg.TxDigestFile)
	if err != nil {
		t.Fatalf("failed to stat digest file: %v", err)
	}

	expectedPerm := os.FileMode(0600)
	if fileInfo.Mode().Perm() != expectedPerm {
		t.Errorf("expected permissions %v, got %v", expectedPerm, fileInfo.Mode().Perm())
	}
}

func TestWriteDigestToFileCreatesDirs(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "challenger-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create service with test config pointing to nested directory
	nestedPath := filepath.Join(tmpDir, "data", "subdir", "digest.txt")
	cfg := &config.Config{
		TxDigestFile: nestedPath,
	}
	service := &Service{
		config: cfg,
	}

	// Test logger
	testLogger := logger.NewCategoryLogger("info", logger.Challenger, logger.General)

	testDigest := "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	// Test writing digest to nested path
	err = service.writeDigestToFile(testDigest, testLogger)
	if err != nil {
		t.Fatalf("writeDigestToFile failed: %v", err)
	}

	// Verify file exists and has correct content
	content, err := os.ReadFile(nestedPath)
	if err != nil {
		t.Fatalf("failed to read digest file: %v", err)
	}

	expectedContent := testDigest + "\n"
	if string(content) != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, string(content))
	}

	// Verify parent directories were created
	if _, err := os.Stat(filepath.Dir(nestedPath)); os.IsNotExist(err) {
		t.Error("parent directories were not created")
	}
}

func TestWriteDigestToFileEmptyConfig(t *testing.T) {
	// Create service with empty config
	cfg := &config.Config{
		TxDigestFile: "",
	}
	service := &Service{
		config: cfg,
	}

	// Test logger
	testLogger := logger.NewCategoryLogger("info", logger.Challenger, logger.General)

	testDigest := "0x1234567890abcdef"

	// Test writing digest with empty config should fail
	err := service.writeDigestToFile(testDigest, testLogger)
	if err == nil {
		t.Fatal("expected error when TX_DIGEST_FILE is empty, got nil")
	}

	expectedErr := "TX_DIGEST_FILE not configured"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
	}
}

func TestWriteDigestToFileOverwrite(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "challenger-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create service with test config
	cfg := &config.Config{
		TxDigestFile: filepath.Join(tmpDir, "digest.txt"),
	}
	service := &Service{
		config: cfg,
	}

	// Test logger
	testLogger := logger.NewCategoryLogger("info", logger.Challenger, logger.General)

	// Write first digest
	firstDigest := "0x1111111111111111111111111111111111111111111111111111111111111111"
	err = service.writeDigestToFile(firstDigest, testLogger)
	if err != nil {
		t.Fatalf("first writeDigestToFile failed: %v", err)
	}

	// Write second digest (should overwrite)
	secondDigest := "0x2222222222222222222222222222222222222222222222222222222222222222"
	err = service.writeDigestToFile(secondDigest, testLogger)
	if err != nil {
		t.Fatalf("second writeDigestToFile failed: %v", err)
	}

	// Verify file contains only the second digest
	content, err := os.ReadFile(cfg.TxDigestFile)
	if err != nil {
		t.Fatalf("failed to read digest file: %v", err)
	}

	expectedContent := secondDigest + "\n"
	if string(content) != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, string(content))
	}

	// Verify first digest is not in the file
	if strings.Contains(string(content), firstDigest) {
		t.Error("file should not contain the first digest after overwrite")
	}
}

// Helper function to create a test logger that doesn't output during tests
func createTestLogger() zerolog.Logger {
	return zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = os.Stderr
		w.TimeFormat = "15:04:05"
	})).With().Timestamp().Logger()
}
