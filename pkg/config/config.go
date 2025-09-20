// Package config provides configuration management for the Reverse Challenge System.
// Loads settings from environment variables and .env files with validation and defaults.
// Supports both shared secret and individual key configurations for HMAC authentication.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// SuiConfig contains Sui blockchain-specific configuration
type SuiConfig struct {
	RPCUrl              string // Sui RPC endpoint URL
	ChallengerMnemonic  string // Challenger service wallet mnemonic for signing transactions
	SolverMnemonic      string // Solver service wallet mnemonic for signing transactions
	InitializerMnemonic string // Initializer/Verifier wallet mnemonic for contract deployment and verification
	ChainID             string // Network identifier (mainnet, testnet, devnet)
	PackageID           string // Deployed package ID (optional)
	RegistryID          string // Registry object ID (optional)
	PosPackageID        string // Positive token package ID (optional)
	NegPackageID        string // Negative token package ID (optional)
	VaultID             string // Vault object ID (optional)
	VaultAdminCapID     string // Vault admin capability ID (optional)
	TreasuryPos         string // Treasury positive type argument
	TreasuryNeg         string // Treasury negative type argument
	Collateral          string // Collateral type argument
}

// Config holds all configuration settings for both challenger and solver services.
// Provides centralized configuration management with validation and helper methods.
type Config struct {
	// Challenger Configuration
	ChallengerHost        string // Challenger service bind host address
	ChallengerPort        string // Challenger service bind port
	UseNgrok              bool   // Whether to use ngrok for callbacks (requires HTTPS)
	PublicCallbackHost    string // Public URL for callbacks (e.g., ngrok URL or localhost)
	ChallengerCallbackKey string // API key for challenger callback validation
	ChalHMACKeyID         string // Key identifier for challenger HMAC signing
	ChalHMACSecret        string // Secret for challenger HMAC signing

	// Sui Configuration
	SUI SuiConfig // Sui blockchain configuration

	// Contract Configuration
	ContractsPath string // Path to contract sources directory
	DatabasePath  string // Primary database path for initializer

	// Solver Configuration
	SolverHost        string // Solver service bind host address
	SolverPort        string // Solver service bind port
	SolverAPIKey      string // API key for solver authentication
	SolverWorkerCount int    // Number of concurrent worker processes
	SolverHMACKeyID   string // Key identifier for solver HMAC signing
	SolverHMACSecret  string // Secret for solver HMAC signing

	// Shared Configuration
	SharedSecretKey string // Shared secret for simplified HMAC setup (overrides individual secrets)

	// Database
	ChallengerDBPath string // File path for challenger SQLite database
	SolverDBPath     string // File path for solver SQLite database

	// Security
	ClockSkewSeconds int // Maximum allowed time difference for HMAC timestamp validation

	// Logging
	LogLevel string // Log level (debug, info, warn, error)

	// Verifier Configuration
	TxDigestFile string // File path for storing last transaction digest

	// Log Service Configuration
	LogServiceURL    string // External log collector endpoint
	LogServiceAPIKey string // API key for log service
	LogsAPIBaseURL   string // Base URL for logs API endpoint
	LogsAPIKey       string // API key for logs API access
}

// Load reads configuration from environment variables and .env file.
// Returns a validated configuration instance with all required settings.
// Automatically loads .env file if present, with environment variables taking precedence.
func Load() (*Config, error) {
	// Try to load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	config := &Config{
		// Challenger Configuration
		ChallengerHost:        getEnv("CHALLENGER_HOST", "0.0.0.0"),
		ChallengerPort:        getEnv("CHALLENGER_PORT", "8080"),
		UseNgrok:              getEnvAsBool("USE_NGROK", false),
		PublicCallbackHost:    getEnv("PUBLIC_CALLBACK_HOST", ""),
		ChallengerCallbackKey: getEnv("CHALLENGER_CALLBACK_KEY", ""),
		ChalHMACKeyID:         getEnv("CHAL_HMAC_KEY_ID", "chal-kid-1"),
		ChalHMACSecret:        getEnv("CHAL_HMAC_SECRET", ""),

		// Sui Configuration
		SUI: SuiConfig{
			RPCUrl:              getEnv("SUI_RPC_URL", "https://fullnode.testnet.sui.io:443"),
			ChallengerMnemonic:  getEnv("SUI_CHALLENGER_MNEMONIC", ""),
			SolverMnemonic:      getEnv("SUI_SOLVER_MNEMONIC", ""),
			InitializerMnemonic: getEnv("SUI_INITIALIZER_MNEMONIC", ""),
			ChainID:             getEnv("SUI_CHAIN_ID", "testnet"),
			PackageID:           getEnv("SUI_PACKAGE_ID", ""),
			RegistryID:          getEnv("SUI_REGISTRY_ID", ""),
			PosPackageID:        getEnv("SUI_POS_PACKAGE_ID", ""),
			NegPackageID:        getEnv("SUI_NEG_PACKAGE_ID", ""),
			VaultID:             getEnv("SUI_VAULT_ID", ""),
			VaultAdminCapID:     getEnv("SUI_VAULT_ADMIN_CAP_ID", ""),
			TreasuryPos:         getEnv("SUI_TYPE_TREASURY_POS", "0x2::sui::SUI"),
			TreasuryNeg:         getEnv("SUI_TYPE_TREASURY_NEG", "0x2::sui::SUI"),
			Collateral:          getEnv("SUI_TYPE_COLLATERAL", "0x2::sui::SUI"),
		},

		// Contract Configuration
		ContractsPath: getEnv("CONTRACTS_PATH", "/Users/hauyang/Work/deepbattle"),
		DatabasePath:  getEnv("DATABASE_PATH", "challenger.db"),

		// Solver Configuration
		SolverHost:        getEnv("SOLVER_HOST", "0.0.0.0"),
		SolverPort:        getEnv("SOLVER_PORT", "8081"),
		SolverAPIKey:      getEnv("SOLVER_API_KEY", ""),
		SolverWorkerCount: getEnvAsInt("SOLVER_WORKER_COUNT", 4),
		SolverHMACKeyID:   getEnv("SOLVER_HMAC_KEY_ID", "solver-kid-1"),
		SolverHMACSecret:  getEnv("SOLVER_HMAC_SECRET", ""),

		// Shared Configuration
		SharedSecretKey: getEnv("SHARED_SECRET_KEY", ""),

		// Database
		ChallengerDBPath: getEnv("CHALLENGER_DB_PATH", "challenger.db"),
		SolverDBPath:     getEnv("SOLVER_DB_PATH", "solver.db"),

		// Security
		ClockSkewSeconds: getEnvAsInt("CLOCK_SKEW_SECONDS", 300),

		// Logging
		LogLevel: getEnv("LOG_LEVEL", "info"),

		// Verifier Configuration
		TxDigestFile: getEnv("TX_DIGEST_FILE", "./data/last_tx_digest.txt"),

		// Log Service Configuration
		LogServiceURL:    getEnv("LOG_SERVICE_URL", ""),
		LogServiceAPIKey: getEnv("LOG_SERVICE_API_KEY", ""),
		LogsAPIBaseURL:   getEnv("LOGS_API_BASE_URL", ""),
		LogsAPIKey:       getEnv("LOGS_API_KEY", ""),
	}

	return config, config.validate()
}

// validate ensures all required configuration values are present and valid.
// Checks that either shared secret or individual HMAC secrets are configured.
// Validates that the public callback host is set for proper callback routing.
func (c *Config) validate() error {
	if c.SharedSecretKey == "" && (c.ChalHMACSecret == "" || c.SolverHMACSecret == "") {
		return fmt.Errorf("either SHARED_SECRET_KEY or both CHAL_HMAC_SECRET and SOLVER_HMAC_SECRET must be set")
	}

	if c.PublicCallbackHost == "" {
		// Provide default based on USE_NGROK setting
		if c.UseNgrok {
			return fmt.Errorf("PUBLIC_CALLBACK_HOST must be set when USE_NGROK=true")
		} else {
			// Auto-set to localhost for local development
			c.PublicCallbackHost = fmt.Sprintf("http://localhost:%s", c.ChallengerPort)
		}
	}

	return nil
}

// GetChallengerAddr returns the complete address for the challenger service.
// Combines host and port into a format suitable for server binding.
func (c *Config) GetChallengerAddr() string {
	return fmt.Sprintf("%s:%s", c.ChallengerHost, c.ChallengerPort)
}

// GetSolverAddr returns the complete address for the solver service.
// Combines host and port into a format suitable for server binding.
func (c *Config) GetSolverAddr() string {
	return fmt.Sprintf("%s:%s", c.SolverHost, c.SolverPort)
}

// GetClockSkew returns the clock skew tolerance as a time.Duration.
// Converts the configured seconds value to a duration for HMAC validation.
func (c *Config) GetClockSkew() time.Duration {
	return time.Duration(c.ClockSkewSeconds) * time.Second
}

// GetChallengerSecrets returns the HMAC secrets map for challenger service.
// Includes secrets for validating requests from both challenger and solver keys.
// Prefers shared secret configuration over individual secrets if available.
func (c *Config) GetChallengerSecrets() map[string]string {
	secrets := make(map[string]string)

	// If using shared secret, map both key IDs to same secret
	if c.SharedSecretKey != "" {
		secrets[c.ChalHMACKeyID] = c.SharedSecretKey
		secrets[c.SolverHMACKeyID] = c.SharedSecretKey
	} else {
		// Use individual secrets
		if c.ChalHMACSecret != "" {
			secrets[c.ChalHMACKeyID] = c.ChalHMACSecret
		}
		if c.SolverHMACSecret != "" {
			secrets[c.SolverHMACKeyID] = c.SolverHMACSecret
		}
	}

	return secrets
}

// GetSolverSecrets returns the HMAC secrets map for solver service.
// Includes secrets for validating requests from both challenger and solver keys.
// Prefers shared secret configuration over individual secrets if available.
func (c *Config) GetSolverSecrets() map[string]string {
	secrets := make(map[string]string)

	// If using shared secret, map both key IDs to same secret
	if c.SharedSecretKey != "" {
		secrets[c.ChalHMACKeyID] = c.SharedSecretKey
		secrets[c.SolverHMACKeyID] = c.SharedSecretKey
	} else {
		// Use individual secrets
		if c.ChalHMACSecret != "" {
			secrets[c.ChalHMACKeyID] = c.ChalHMACSecret
		}
		if c.SolverHMACSecret != "" {
			secrets[c.SolverHMACKeyID] = c.SolverHMACSecret
		}
	}

	return secrets
}

// getEnv retrieves an environment variable or returns a default value.
// Helper function for loading configuration with fallback defaults.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as integer or returns a default.
// Safely converts string environment variables to integers with error handling.
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsBool retrieves an environment variable as boolean or returns a default.
// Safely converts string environment variables to booleans with error handling.
func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
