package config

import (
	"os"
	"testing"
	"time"
)

// Helper function to clear all environment variables used by the config
func clearConfigEnv() {
	envVars := []string{
		"CHALLENGER_HOST", "CHALLENGER_PORT", "USE_NGROK", "PUBLIC_CALLBACK_HOST",
		"CHALLENGER_CALLBACK_KEY", "CHAL_HMAC_KEY_ID", "CHAL_HMAC_SECRET",
		"SOLVER_HOST", "SOLVER_PORT", "SOLVER_API_KEY", "SOLVER_WORKER_COUNT",
		"SOLVER_HMAC_KEY_ID", "SOLVER_HMAC_SECRET", "SHARED_SECRET_KEY",
		"CHALLENGER_DB_PATH", "SOLVER_DB_PATH", "CLOCK_SKEW_SECONDS", "LOG_LEVEL",
		"SUI_MNEMONIC", "SUI_PACKAGE_ID", "SUI_TYPE_TREASURY_POS", "SUI_TYPE_TREASURY_NEG", "SUI_TYPE_COLLATERAL", // Add Sui related env vars for cleanup
	}
	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}

func TestConfig_Load_WithDefaults(t *testing.T) {
	clearConfigEnv()

	// Set required env vars
	os.Setenv("PUBLIC_CALLBACK_HOST", "https://example.com")
	os.Setenv("SHARED_SECRET_KEY", "test-secret")
	defer clearConfigEnv()

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test defaults
	if config.ChallengerHost != "0.0.0.0" {
		t.Errorf("Expected ChallengerHost '0.0.0.0', got '%s'", config.ChallengerHost)
	}

	if config.ChallengerPort != "8080" {
		t.Errorf("Expected ChallengerPort '8080', got '%s'", config.ChallengerPort)
	}

	if config.SolverHost != "0.0.0.0" {
		t.Errorf("Expected SolverHost '0.0.0.0', got '%s'", config.SolverHost)
	}

	if config.SolverPort != "8081" {
		t.Errorf("Expected SolverPort '8081', got '%s'", config.SolverPort)
	}

	if config.SolverWorkerCount != 4 {
		t.Errorf("Expected SolverWorkerCount 4, got %d", config.SolverWorkerCount)
	}

	if config.ClockSkewSeconds != 300 {
		t.Errorf("Expected ClockSkewSeconds 300, got %d", config.ClockSkewSeconds)
	}

	if config.LogLevel != "info" {
		t.Errorf("Expected LogLevel 'info', got '%s'", config.LogLevel)
	}

	if config.ChallengerDBPath != "challenger.db" {
		t.Errorf("Expected ChallengerDBPath 'challenger.db', got '%s'", config.ChallengerDBPath)
	}

	if config.SolverDBPath != "solver.db" {
		t.Errorf("Expected SolverDBPath 'solver.db', got '%s'", config.SolverDBPath)
	}

	if config.ChalHMACKeyID != "chal-kid-1" {
		t.Errorf("Expected ChalHMACKeyID 'chal-kid-1', got '%s'", config.ChalHMACKeyID)
	}

	if config.SolverHMACKeyID != "solver-kid-1" {
		t.Errorf("Expected SolverHMACKeyID 'solver-kid-1', got '%s'", config.SolverHMACKeyID)
	}
}

func TestConfig_Load_WithCustomValues(t *testing.T) {
	clearConfigEnv()

	// Set custom values
	os.Setenv("CHALLENGER_HOST", "127.0.0.1")
	os.Setenv("CHALLENGER_PORT", "9080")
	os.Setenv("PUBLIC_CALLBACK_HOST", "https://custom.example.com")
	os.Setenv("SOLVER_HOST", "192.168.1.10")
	os.Setenv("SOLVER_PORT", "9081")
	os.Setenv("SOLVER_WORKER_COUNT", "8")
	os.Setenv("SHARED_SECRET_KEY", "custom-secret")
	os.Setenv("CHALLENGER_DB_PATH", "/tmp/custom-challenger.db")
	os.Setenv("SOLVER_DB_PATH", "/tmp/custom-solver.db")
	os.Setenv("CLOCK_SKEW_SECONDS", "600")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("CHAL_HMAC_KEY_ID", "custom-chal-key")
	os.Setenv("SOLVER_HMAC_KEY_ID", "custom-solver-key")
	os.Setenv("SUI_MNEMONIC", "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
	os.Setenv("SUI_PACKAGE_ID", "0x1234567890abcdef1234567890abcdef12345678")
	os.Setenv("SUI_TYPE_TREASURY_POS", "0x2::custom::TOKEN")
	os.Setenv("SUI_TYPE_TREASURY_NEG", "0x2::custom::TOKEN2")
	os.Setenv("SUI_TYPE_COLLATERAL", "0x2::custom::COLLATERAL")
	defer clearConfigEnv()

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test custom values
	if config.ChallengerHost != "127.0.0.1" {
		t.Errorf("Expected ChallengerHost '127.0.0.1', got '%s'", config.ChallengerHost)
	}

	if config.ChallengerPort != "9080" {
		t.Errorf("Expected ChallengerPort '9080', got '%s'", config.ChallengerPort)
	}

	if config.PublicCallbackHost != "https://custom.example.com" {
		t.Errorf("Expected PublicCallbackHost 'https://custom.example.com', got '%s'", config.PublicCallbackHost)
	}

	if config.SolverHost != "192.168.1.10" {
		t.Errorf("Expected SolverHost '192.168.1.10', got '%s'", config.SolverHost)
	}

	if config.SolverPort != "9081" {
		t.Errorf("Expected SolverPort '9081', got '%s'", config.SolverPort)
	}

	if config.SolverWorkerCount != 8 {
		t.Errorf("Expected SolverWorkerCount 8, got %d", config.SolverWorkerCount)
	}

	if config.ClockSkewSeconds != 600 {
		t.Errorf("Expected ClockSkewSeconds 600, got %d", config.ClockSkewSeconds)
	}

	if config.LogLevel != "debug" {
		t.Errorf("Expected LogLevel 'debug', got '%s'", config.LogLevel)
	}

	if config.ChallengerDBPath != "/tmp/custom-challenger.db" {
		t.Errorf("Expected ChallengerDBPath '/tmp/custom-challenger.db', got '%s'", config.ChallengerDBPath)
	}

	if config.SolverDBPath != "/tmp/custom-solver.db" {
		t.Errorf("Expected SolverDBPath '/tmp/custom-solver.db', got '%s'", config.SolverDBPath)
	}

	if config.ChalHMACKeyID != "custom-chal-key" {
		t.Errorf("Expected ChalHMACKeyID 'custom-chal-key', got '%s'", config.ChalHMACKeyID)
	}

	if config.SolverHMACKeyID != "custom-solver-key" {
		t.Errorf("Expected SolverHMACKeyID 'custom-solver-key', got '%s'", config.SolverHMACKeyID)
	}

	if config.SUI.Mnemonic != "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about" {
		t.Errorf("Expected Mnemonic to match custom value, got '%s'", config.SUI.Mnemonic)
	}

	if config.SUI.PackageID != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("Expected SuiPackageID '0x1234567890abcdef1234567890abcdef12345678', got '%s'", config.SUI.PackageID)
	}

	if config.SUI.TreasuryPos != "0x2::custom::TOKEN" {
		t.Errorf("Expected SuiTreasuryPos '0x2::custom::TOKEN', got '%s'", config.SUI.TreasuryPos)
	}

	if config.SUI.TreasuryNeg != "0x2::custom::TOKEN2" {
		t.Errorf("Expected SuiTreasuryNeg '0x2::custom::TOKEN2', got '%s'", config.SUI.TreasuryNeg)
	}

	if config.SUI.Collateral != "0x2::custom::COLLATERAL" {
		t.Errorf("Expected SuiCollateral '0x2::custom::COLLATERAL', got '%s'", config.SUI.Collateral)
	}
}

func TestConfig_Validation_MissingPublicCallbackHost(t *testing.T) {
	clearConfigEnv()

	// Set USE_NGROK=true but not callback host - this should error
	os.Setenv("USE_NGROK", "true")
	os.Setenv("SHARED_SECRET_KEY", "test-secret")
	defer clearConfigEnv()

	_, err := Load()
	if err == nil {
		t.Error("Expected error when PUBLIC_CALLBACK_HOST is missing and USE_NGROK=true")
	}

	expectedError := "PUBLIC_CALLBACK_HOST must be set when USE_NGROK=true"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestConfig_Validation_MissingSecrets(t *testing.T) {
	clearConfigEnv()

	// Set callback host but no secrets
	os.Setenv("PUBLIC_CALLBACK_HOST", "https://example.com")
	defer clearConfigEnv()

	_, err := Load()
	if err == nil {
		t.Error("Expected error when no secrets are set")
	}

	expectedError := "either SHARED_SECRET_KEY or both CHAL_HMAC_SECRET and SOLVER_HMAC_SECRET must be set"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestConfig_Validation_OnlyChalSecret(t *testing.T) {
	clearConfigEnv()

	// Set callback host and only challenger secret
	os.Setenv("PUBLIC_CALLBACK_HOST", "https://example.com")
	os.Setenv("CHAL_HMAC_SECRET", "chal-secret")
	defer clearConfigEnv()

	_, err := Load()
	if err == nil {
		t.Error("Expected error when only challenger secret is set")
	}

	expectedError := "either SHARED_SECRET_KEY or both CHAL_HMAC_SECRET and SOLVER_HMAC_SECRET must be set"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestConfig_Validation_OnlySolverSecret(t *testing.T) {
	clearConfigEnv()

	// Set callback host and only solver secret
	os.Setenv("PUBLIC_CALLBACK_HOST", "https://example.com")
	os.Setenv("SOLVER_HMAC_SECRET", "solver-secret")
	defer clearConfigEnv()

	_, err := Load()
	if err == nil {
		t.Error("Expected error when only solver secret is set")
	}

	expectedError := "either SHARED_SECRET_KEY or both CHAL_HMAC_SECRET and SOLVER_HMAC_SECRET must be set"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestConfig_Validation_BothIndividualSecrets(t *testing.T) {
	clearConfigEnv()

	// Set callback host and both individual secrets
	os.Setenv("PUBLIC_CALLBACK_HOST", "https://example.com")
	os.Setenv("CHAL_HMAC_SECRET", "chal-secret")
	os.Setenv("SOLVER_HMAC_SECRET", "solver-secret")
	defer clearConfigEnv()

	_, err := Load()
	if err != nil {
		t.Fatalf("Expected no error when both individual secrets are set, got: %v", err)
	}
}

func TestConfig_GetChallengerAddr(t *testing.T) {
	clearConfigEnv()

	os.Setenv("PUBLIC_CALLBACK_HOST", "https://example.com")
	os.Setenv("SHARED_SECRET_KEY", "test-secret")
	os.Setenv("CHALLENGER_HOST", "192.168.1.5")
	os.Setenv("CHALLENGER_PORT", "9000")
	defer clearConfigEnv()

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	expectedAddr := "192.168.1.5:9000"
	actualAddr := config.GetChallengerAddr()

	if actualAddr != expectedAddr {
		t.Errorf("Expected challenger addr '%s', got '%s'", expectedAddr, actualAddr)
	}
}

func TestConfig_GetSolverAddr(t *testing.T) {
	clearConfigEnv()

	os.Setenv("PUBLIC_CALLBACK_HOST", "https://example.com")
	os.Setenv("SHARED_SECRET_KEY", "test-secret")
	os.Setenv("SOLVER_HOST", "10.0.0.1")
	os.Setenv("SOLVER_PORT", "8888")
	defer clearConfigEnv()

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	expectedAddr := "10.0.0.1:8888"
	actualAddr := config.GetSolverAddr()

	if actualAddr != expectedAddr {
		t.Errorf("Expected solver addr '%s', got '%s'", expectedAddr, actualAddr)
	}
}

func TestConfig_GetClockSkew(t *testing.T) {
	clearConfigEnv()

	os.Setenv("PUBLIC_CALLBACK_HOST", "https://example.com")
	os.Setenv("SHARED_SECRET_KEY", "test-secret")
	os.Setenv("CLOCK_SKEW_SECONDS", "900")
	defer clearConfigEnv()

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	expectedDuration := 900 * time.Second
	actualDuration := config.GetClockSkew()

	if actualDuration != expectedDuration {
		t.Errorf("Expected clock skew %v, got %v", expectedDuration, actualDuration)
	}
}

func TestConfig_GetChallengerSecrets_WithSharedSecret(t *testing.T) {
	clearConfigEnv()

	os.Setenv("PUBLIC_CALLBACK_HOST", "https://example.com")
	os.Setenv("SHARED_SECRET_KEY", "shared-secret-123")
	os.Setenv("CHAL_HMAC_KEY_ID", "custom-chal-key")
	os.Setenv("SOLVER_HMAC_KEY_ID", "custom-solver-key")
	defer clearConfigEnv()

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	secrets := config.GetChallengerSecrets()

	expectedSecrets := map[string]string{
		"custom-chal-key":   "shared-secret-123",
		"custom-solver-key": "shared-secret-123",
	}

	if len(secrets) != len(expectedSecrets) {
		t.Errorf("Expected %d secrets, got %d", len(expectedSecrets), len(secrets))
	}

	for keyID, expectedSecret := range expectedSecrets {
		if actualSecret, exists := secrets[keyID]; !exists {
			t.Errorf("Expected secret for key ID '%s' to exist", keyID)
		} else if actualSecret != expectedSecret {
			t.Errorf("Expected secret '%s' for key ID '%s', got '%s'", expectedSecret, keyID, actualSecret)
		}
	}
}

func TestConfig_GetChallengerSecrets_WithIndividualSecrets(t *testing.T) {
	clearConfigEnv()

	os.Setenv("PUBLIC_CALLBACK_HOST", "https://example.com")
	os.Setenv("CHAL_HMAC_SECRET", "chal-individual-secret")
	os.Setenv("SOLVER_HMAC_SECRET", "solver-individual-secret")
	os.Setenv("CHAL_HMAC_KEY_ID", "chal-key")
	os.Setenv("SOLVER_HMAC_KEY_ID", "solver-key")
	defer clearConfigEnv()

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	secrets := config.GetChallengerSecrets()

	expectedSecrets := map[string]string{
		"chal-key":   "chal-individual-secret",
		"solver-key": "solver-individual-secret",
	}

	if len(secrets) != len(expectedSecrets) {
		t.Errorf("Expected %d secrets, got %d", len(expectedSecrets), len(secrets))
	}

	for keyID, expectedSecret := range expectedSecrets {
		if actualSecret, exists := secrets[keyID]; !exists {
			t.Errorf("Expected secret for key ID '%s' to exist", keyID)
		} else if actualSecret != expectedSecret {
			t.Errorf("Expected secret '%s' for key ID '%s', got '%s'", expectedSecret, keyID, actualSecret)
		}
	}
}

func TestConfig_GetSolverSecrets_WithSharedSecret(t *testing.T) {
	clearConfigEnv()

	os.Setenv("PUBLIC_CALLBACK_HOST", "https://example.com")
	os.Setenv("SHARED_SECRET_KEY", "shared-secret-456")
	defer clearConfigEnv()

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	secrets := config.GetSolverSecrets()

	// Both challenger and solver secrets should return the same map when using shared secret
	challengerSecrets := config.GetChallengerSecrets()

	if len(secrets) != len(challengerSecrets) {
		t.Errorf("Expected solver secrets to have same length as challenger secrets")
	}

	for keyID, challengerSecret := range challengerSecrets {
		if solverSecret, exists := secrets[keyID]; !exists {
			t.Errorf("Expected solver secret for key ID '%s' to exist", keyID)
		} else if solverSecret != challengerSecret {
			t.Errorf("Expected solver and challenger secrets to match for key ID '%s'", keyID)
		}
	}
}

func TestGetEnvAsInt_ValidInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	result := getEnvAsInt("TEST_INT", 10)
	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}
}

func TestGetEnvAsInt_InvalidInt(t *testing.T) {
	os.Setenv("TEST_INT", "not_a_number")
	defer os.Unsetenv("TEST_INT")

	result := getEnvAsInt("TEST_INT", 10)
	if result != 10 {
		t.Errorf("Expected default value 10, got %d", result)
	}
}

func TestGetEnvAsInt_EmptyValue(t *testing.T) {
	os.Unsetenv("TEST_INT")

	result := getEnvAsInt("TEST_INT", 99)
	if result != 99 {
		t.Errorf("Expected default value 99, got %d", result)
	}
}

func TestGetEnv_ExistingValue(t *testing.T) {
	os.Setenv("TEST_STRING", "hello_world")
	defer os.Unsetenv("TEST_STRING")

	result := getEnv("TEST_STRING", "default")
	if result != "hello_world" {
		t.Errorf("Expected 'hello_world', got '%s'", result)
	}
}

func TestGetEnv_DefaultValue(t *testing.T) {
	os.Unsetenv("TEST_STRING")

	result := getEnv("TEST_STRING", "default_value")
	if result != "default_value" {
		t.Errorf("Expected 'default_value', got '%s'", result)
	}
}

func TestGetEnv_EmptyString(t *testing.T) {
	os.Setenv("TEST_STRING", "")
	defer os.Unsetenv("TEST_STRING")

	result := getEnv("TEST_STRING", "default")
	if result != "default" {
		t.Errorf("Expected 'default' for empty env var, got '%s'", result)
	}
}
