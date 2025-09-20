package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadDigestFromFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "verifier-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	digestFile := filepath.Join(tmpDir, "digest.txt")
	testDigest := "0x1234567890abcdef1234567890abcdef12345678901234567890abcdef123456789"

	// Write test digest to file
	err = os.WriteFile(digestFile, []byte(testDigest+"\n"), 0600)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Test reading digest
	digest, err := readDigestFromFile(digestFile)
	if err != nil {
		t.Fatalf("readDigestFromFile failed: %v", err)
	}

	if digest != testDigest {
		t.Errorf("expected digest %q, got %q", testDigest, digest)
	}
}

func TestReadDigestFromFileWithWhitespace(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "verifier-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	digestFile := filepath.Join(tmpDir, "digest.txt")
	testDigest := "0x1234567890abcdef1234567890abcdef12345678901234567890abcdef123456789"

	// Write test digest with various whitespace
	content := "  \t" + testDigest + "\n\r\n  "
	err = os.WriteFile(digestFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Test reading digest
	digest, err := readDigestFromFile(digestFile)
	if err != nil {
		t.Fatalf("readDigestFromFile failed: %v", err)
	}

	if digest != testDigest {
		t.Errorf("expected digest %q, got %q", testDigest, digest)
	}
}

func TestReadDigestFromFileNotExists(t *testing.T) {
	// Test reading from non-existent file
	nonExistentFile := "/path/that/does/not/exist/digest.txt"
	digest, err := readDigestFromFile(nonExistentFile)
	if err == nil {
		t.Fatal("expected error when reading non-existent file, got nil")
	}

	if digest != "" {
		t.Errorf("expected empty digest, got %q", digest)
	}

	expectedErr := "digest file does not exist"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
	}
}

func TestReadDigestFromFileEmpty(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "verifier-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	digestFile := filepath.Join(tmpDir, "empty_digest.txt")

	// Create empty file
	err = os.WriteFile(digestFile, []byte(""), 0600)
	if err != nil {
		t.Fatalf("failed to write empty test file: %v", err)
	}

	// Test reading from empty file
	digest, err := readDigestFromFile(digestFile)
	if err != nil {
		t.Fatalf("readDigestFromFile failed: %v", err)
	}

	if digest != "" {
		t.Errorf("expected empty digest from empty file, got %q", digest)
	}
}

func TestReadDigestFromFileEmptyPath(t *testing.T) {
	// Test reading with empty path
	digest, err := readDigestFromFile("")
	if err == nil {
		t.Fatal("expected error when path is empty, got nil")
	}

	if digest != "" {
		t.Errorf("expected empty digest, got %q", digest)
	}

	expectedErr := "digest file path is empty"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
	}
}

func TestReadDigestFromFileWhitespaceOnly(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "verifier-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	digestFile := filepath.Join(tmpDir, "whitespace_digest.txt")

	// Create file with only whitespace
	err = os.WriteFile(digestFile, []byte("  \t\n\r\n  "), 0600)
	if err != nil {
		t.Fatalf("failed to write whitespace test file: %v", err)
	}

	// Test reading from whitespace-only file
	digest, err := readDigestFromFile(digestFile)
	if err != nil {
		t.Fatalf("readDigestFromFile failed: %v", err)
	}

	if digest != "" {
		t.Errorf("expected empty digest from whitespace-only file, got %q", digest)
	}
}
