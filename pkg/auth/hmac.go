// Package auth provides HMAC-SHA256 authentication for the Reverse Challenge System.
// Implements request signing and verification with nonce-based replay protection
// and configurable clock skew tolerance for distributed systems.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	AuthHeaderPrefix = "RCS-HMAC-SHA256" // HTTP Authorization header prefix for this auth scheme
	DefaultClockSkew = 300               // Default clock skew tolerance: 300 seconds = 5 minutes
)

// HMACAuth manages HMAC-SHA256 authentication with multiple key support.
// Handles both signing outbound requests and verifying inbound requests.
type HMACAuth struct {
	secrets   map[string]string // Map of keyId to secret for multi-key support
	clockSkew time.Duration     // Maximum allowed time difference between request and verification
}

// AuthHeader represents the parsed components of an HMAC authentication header.
// Contains all fields required for signature verification.
type AuthHeader struct {
	KeyID     string // Identifier for the signing key
	Timestamp string // Unix timestamp when request was signed
	Nonce     string // Unique identifier to prevent replay attacks
	Signature string // HMAC-SHA256 signature of the canonical request string
}

// NewHMACAuth creates a new HMAC authenticator with the provided secrets and clock skew.
// If clockSkew is 0, uses the default 5-minute tolerance.
func NewHMACAuth(secrets map[string]string, clockSkew time.Duration) *HMACAuth {
	if clockSkew == 0 {
		clockSkew = DefaultClockSkew * time.Second
	}
	return &HMACAuth{
		secrets:   secrets,
		clockSkew: clockSkew,
	}
}

// AddSecret adds or updates a signing secret for the given key ID.
// Allows dynamic key management without recreating the authenticator.
func (h *HMACAuth) AddSecret(keyID, secret string) {
	h.secrets[keyID] = secret
}

// BodySHA256Hex computes the SHA-256 hash of the request body and returns it as a hex string.
// Used as part of the canonical string to prevent body tampering.
func BodySHA256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// CanonicalString creates the standardized string representation of a request for signing.
// Combines HTTP method, path, timestamp, nonce, and body hash in a specific format.
// This ensures consistent signature generation across different implementations.
func CanonicalString(method, path, ts, nonce, bodyHex string) string {
	return strings.Join([]string{
		strings.ToUpper(method),
		path, // EscapedPath only, no querystring in MVP
		ts,
		nonce,
		bodyHex,
	}, "\n")
}

// ComputeSignature generates an HMAC-SHA256 signature for the given request parameters.
// Creates a canonical string from the request and signs it with the provided secret.
func ComputeSignature(method, path string, body []byte, ts, nonce, secret string) string {
	canonical := CanonicalString(method, path, ts, nonce, BodySHA256Hex(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

// CreateAuthHeader generates a complete Authorization header for the given request.
// Uses the current timestamp and provided nonce to create a signed header.
// Returns empty string if the keyID is not found in the secrets map.
func (h *HMACAuth) CreateAuthHeader(method, path string, body []byte, keyID, nonce string) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	secret, exists := h.secrets[keyID]
	if !exists {
		return ""
	}

	sig := ComputeSignature(method, path, body, ts, nonce, secret)

	return fmt.Sprintf("%s keyId=%s,ts=%s,nonce=%s,sig=%s",
		AuthHeaderPrefix, keyID, ts, nonce, sig)
}

// ParseAuthHeader parses an Authorization header into its component parts.
// Validates the header format and extracts keyId, timestamp, nonce, and signature.
// Returns an error if the header format is invalid or required fields are missing.
func ParseAuthHeader(authHeader string) (*AuthHeader, error) {
	if !strings.HasPrefix(authHeader, AuthHeaderPrefix) {
		return nil, fmt.Errorf("invalid auth header prefix")
	}

	parts := strings.TrimPrefix(authHeader, AuthHeaderPrefix+" ")
	pairs := strings.Split(parts, ",")

	auth := &AuthHeader{}

	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		switch key {
		case "keyId":
			auth.KeyID = value
		case "ts":
			auth.Timestamp = value
		case "nonce":
			auth.Nonce = value
		case "sig":
			auth.Signature = value
		}
	}

	if auth.KeyID == "" || auth.Timestamp == "" || auth.Nonce == "" || auth.Signature == "" {
		return nil, fmt.Errorf("missing required auth header fields")
	}

	return auth, nil
}

// VerifySignature validates an incoming request's HMAC signature.
// Performs comprehensive validation including key existence, timestamp freshness,
// and signature verification using constant-time comparison to prevent timing attacks.
func (h *HMACAuth) VerifySignature(method, path string, body []byte, auth *AuthHeader) error {
	// Check if keyID exists
	secret, exists := h.secrets[auth.KeyID]
	if !exists {
		return fmt.Errorf("unknown keyId: %s", auth.KeyID)
	}

	// Verify timestamp is within clock skew
	ts, err := strconv.ParseInt(auth.Timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %s", auth.Timestamp)
	}

	now := time.Now().Unix()
	if abs(now-ts) > int64(h.clockSkew.Seconds()) {
		return fmt.Errorf("timestamp outside allowed skew: %d vs %d", ts, now)
	}

	// Compute expected signature
	expectedSig := ComputeSignature(method, path, body, auth.Timestamp, auth.Nonce, secret)

	// Constant time comparison to prevent timing attacks
	if !hmac.Equal([]byte(expectedSig), []byte(auth.Signature)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// abs returns the absolute value of an int64.
// Helper function for timestamp difference calculations.
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// GetSecret retrieves the secret for a given key ID.
// Returns the secret and a boolean indicating whether the key exists.
// Used for debugging and testing purposes.
func (h *HMACAuth) GetSecret(keyID string) (string, bool) {
	secret, exists := h.secrets[keyID]
	return secret, exists
}
