package auth

import (
	"strings"
	"testing"
	"time"
)

func TestHMACAuth(t *testing.T) {
	secrets := map[string]string{
		"test-key-1": "test-secret-123",
	}

	auth := NewHMACAuth(secrets, 300*time.Second)

	t.Run("CreateAndVerifySignature", func(t *testing.T) {
		method := "POST"
		path := "/test"
		body := []byte(`{"test": "data"}`)
		keyID := "test-key-1"
		nonce := "test-nonce-123"

		// Create auth header
		authHeader := auth.CreateAuthHeader(method, path, body, keyID, nonce)
		if authHeader == "" {
			t.Fatal("Failed to create auth header")
		}

		// Parse auth header
		authInfo, err := ParseAuthHeader(authHeader)
		if err != nil {
			t.Fatalf("Failed to parse auth header: %v", err)
		}

		if authInfo.KeyID != keyID {
			t.Errorf("Expected keyID %s, got %s", keyID, authInfo.KeyID)
		}

		if authInfo.Nonce != nonce {
			t.Errorf("Expected nonce %s, got %s", nonce, authInfo.Nonce)
		}

		// Verify signature
		err = auth.VerifySignature(method, path, body, authInfo)
		if err != nil {
			t.Errorf("Signature verification failed: %v", err)
		}
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		method := "POST"
		path := "/test"
		body := []byte(`{"test": "data"}`)
		keyID := "test-key-1"
		nonce := "test-nonce-123"

		// Create auth header
		authHeader := auth.CreateAuthHeader(method, path, body, keyID, nonce)

		// Parse and modify signature
		authInfo, _ := ParseAuthHeader(authHeader)
		authInfo.Signature = "invalid-signature"

		// Verify signature should fail
		err := auth.VerifySignature(method, path, body, authInfo)
		if err == nil {
			t.Error("Expected signature verification to fail, but it passed")
		}
	})

	t.Run("UnknownKeyID", func(t *testing.T) {
		method := "POST"
		path := "/test"
		body := []byte(`{"test": "data"}`)
		keyID := "unknown-key"
		nonce := "test-nonce-123"

		// Create auth header should return empty string
		authHeader := auth.CreateAuthHeader(method, path, body, keyID, nonce)
		if authHeader != "" {
			t.Error("Expected empty auth header for unknown keyID")
		}
	})

	t.Run("ExpiredTimestamp", func(t *testing.T) {
		method := "POST"
		path := "/test"
		body := []byte(`{"test": "data"}`)
		keyID := "test-key-1"
		nonce := "test-nonce-123"

		// Create auth header
		authHeader := auth.CreateAuthHeader(method, path, body, keyID, nonce)
		authInfo, _ := ParseAuthHeader(authHeader)

		// Modify timestamp to be very old
		authInfo.Timestamp = "1000000000" // Jan 2001

		// Verify signature should fail due to expired timestamp
		err := auth.VerifySignature(method, path, body, authInfo)
		if err == nil {
			t.Error("Expected signature verification to fail for expired timestamp")
		}

		if !strings.Contains(err.Error(), "timestamp outside allowed skew") {
			t.Errorf("Expected timestamp skew error, got: %v", err)
		}
	})
}

func TestCanonicalString(t *testing.T) {
	method := "POST"
	path := "/callback/test-challenge-123"
	ts := "1640995200"
	nonce := "uuid-nonce-123"
	bodyHex := "abcdef123456"

	expected := "POST\n/callback/test-challenge-123\n1640995200\nuuid-nonce-123\nabcdef123456"
	actual := CanonicalString(method, path, ts, nonce, bodyHex)

	if actual != expected {
		t.Errorf("Canonical string mismatch.\nExpected: %q\nActual: %q", expected, actual)
	}
}

func TestBodySHA256Hex(t *testing.T) {
	body := []byte(`{"test": "data"}`)
	expected := "40b61fe1b15af0a4d5402735b26343e8cf8a045f4d81710e6108a21d91eaf366" // SHA256 of the JSON
	actual := BodySHA256Hex(body)

	if actual != expected {
		t.Errorf("Body SHA256 mismatch.\nExpected: %s\nActual: %s", expected, actual)
	}
}

func TestParseAuthHeader(t *testing.T) {
	t.Run("ValidHeader", func(t *testing.T) {
		header := "RCS-HMAC-SHA256 keyId=test-key,ts=1640995200,nonce=test-nonce,sig=abcd1234"

		authInfo, err := ParseAuthHeader(header)
		if err != nil {
			t.Fatalf("Failed to parse valid header: %v", err)
		}

		if authInfo.KeyID != "test-key" {
			t.Errorf("Expected keyId 'test-key', got '%s'", authInfo.KeyID)
		}

		if authInfo.Timestamp != "1640995200" {
			t.Errorf("Expected timestamp '1640995200', got '%s'", authInfo.Timestamp)
		}

		if authInfo.Nonce != "test-nonce" {
			t.Errorf("Expected nonce 'test-nonce', got '%s'", authInfo.Nonce)
		}

		if authInfo.Signature != "abcd1234" {
			t.Errorf("Expected signature 'abcd1234', got '%s'", authInfo.Signature)
		}
	})

	t.Run("InvalidPrefix", func(t *testing.T) {
		header := "Bearer token123"

		_, err := ParseAuthHeader(header)
		if err == nil {
			t.Error("Expected error for invalid prefix")
		}
	})

	t.Run("MissingFields", func(t *testing.T) {
		header := "RCS-HMAC-SHA256 keyId=test-key,ts=1640995200"

		_, err := ParseAuthHeader(header)
		if err == nil {
			t.Error("Expected error for missing fields")
		}
	})
}
