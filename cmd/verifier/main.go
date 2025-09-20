package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"reverse-challenge-system/pkg/config"
	"reverse-challenge-system/pkg/logger"
	"reverse-challenge-system/pkg/models"
	localsui "reverse-challenge-system/pkg/sui"

	"github.com/fardream/go-bcs/bcs"
	"github.com/pattonkan/sui-go/sui"
	"github.com/pattonkan/sui-go/suiclient"
	"github.com/pattonkan/sui-go/suisigner"
	"github.com/pattonkan/sui-go/suisigner/suicrypto"
	"github.com/rs/zerolog"
)

func main() {
	var (
		rpcURL     = flag.String("rpc-url", "", "Sui RPC URL (overrides config)")
		digestFile = flag.String("digest-file", "", "Path to digest file (overrides config)")
		help       = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		showUsage()
		os.Exit(0)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	appLogger := logger.NewCategoryLogger(cfg.LogLevel, logger.Challenger, logger.General)

	// Override config with command line flags if provided
	if *rpcURL != "" {
		cfg.SUI.RPCUrl = *rpcURL
	}
	if *digestFile != "" {
		cfg.TxDigestFile = *digestFile
	}

	// Validate required config
	if cfg.SUI.RPCUrl == "" {
		fmt.Fprintf(os.Stderr, "Error: SUI_RPC_URL not configured and --rpc-url not provided\n")
		os.Exit(1)
	}

	// Initialize Sui TransactionBuilder if mnemonic is provided
	var suiTxBuilder *localsui.TransactionBuilder
	suiTxBuilder, err = localsui.NewTransactionBuilder(
		context.Background(),
		appLogger,
		cfg.SUI.RPCUrl,
		cfg.SUI.PackageID,
		cfg.SUI.InitializerMnemonic,
	)
	if err != nil {
		appLogger.Error().Err(err).Msg("Failed to initialize Sui TransactionBuilder, continuing without Sui integration")
	} else {
		appLogger.Info().
			Str("rpc_url", cfg.SUI.RPCUrl).
			Str("package_id", cfg.SUI.PackageID).
			Msg("Sui TransactionBuilder initialized successfully")
	}

	if cfg.TxDigestFile == "" {
		fmt.Fprintf(os.Stderr, "Error: TX_DIGEST_FILE not configured and --digest-file not provided\n")
		os.Exit(1)
	}

	// Read digest from file
	digest, err := readDigestFromFile(cfg.TxDigestFile)
	if err != nil {
		appLogger.Error().Err(err).
			Str("digest_file", cfg.TxDigestFile).
			Msg("Failed to read digest from file")
		fmt.Fprintf(os.Stderr, "Error reading digest file: %v\n", err)
		os.Exit(1)
	}

	if digest == "" {
		appLogger.Error().
			Str("digest_file", cfg.TxDigestFile).
			Msg("Digest file is empty")
		fmt.Fprintf(os.Stderr, "Error: Digest file is empty: %s\n", cfg.TxDigestFile)
		os.Exit(1)
	}

	// Fetch transaction using the official Sui client
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	appLogger.Info().
		Str("digest", digest).
		Str("rpc_url", cfg.SUI.RPCUrl).
		Msg("Fetching transaction from Sui")

	objRes, err := localsui.GetObject(ctx, cfg.SUI.RPCUrl, digest, appLogger)
	if err != nil {
		appLogger.Error().Err(err).
			Str("digest", digest).
			Str("rpc_url", cfg.SUI.RPCUrl).
			Msg("Failed to fetch transaction")
		fmt.Fprintf(os.Stderr, "Error fetching transaction: %v\n", err)
		os.Exit(1)
	}

	// Parse and extract commitment payload
	commitmentPayload, err := extractCommitmentPayload(objRes)
	if err != nil {
		appLogger.Warn().Err(err).
			Str("digest", digest).
			Msg("Failed to extract commitment payload, printing raw transaction")

		// Print raw JSON if parsing fails
		fmt.Printf("Transaction Digest: %s\n", digest)
		// fmt.Printf("Raw Transaction Data:\n%s\n", string(txData))
	} else {
		// Print structured output
		fmt.Printf("Transaction Digest: %s\n", digest)
		fmt.Printf("Commitment Payload: %v\n", commitmentPayload)
	}

	var result models.Result
	// Fetch log entry using commitment payload ID
	if commitmentPayload != nil && commitmentPayload.Id != nil {
		logID := commitmentPayload.Id.String()
		appLogger.Info().
			Str("logID", logID).
			Msg("Fetching log entry from API")

		entry, err := fetchLogEntry(logID, cfg, appLogger)
		if err != nil {
			appLogger.Error().Err(err).
				Str("logID", logID).
				Msg("Failed to fetch log entry")
			fmt.Fprintf(os.Stderr, "Error fetching log entry: %v\n", err)
			os.Exit(1)
		}

		if err := json.Unmarshal([]byte(entry.Log), &result); err != nil {
			appLogger.Error().Err(err).
				Str("logID", logID).
				Str("log", entry.Log).
				Msg("Failed to unmarshal log entry")
			fmt.Fprintf(os.Stderr, "Error unmarshaling log entry: %v\n", err)
			os.Exit(1)
		}

		appLogger.Info().
			Str("logID", logID).
			Str("receivedAnswer", result.ReceivedAnswer).
			Str("status", result.Status).
			Msg("Successfully populated result from log entry")
	} else {
		appLogger.Warn().Msg("No commitment payload ID available for log lookup")
	}

	commitment := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", cfg.SUI.RegistryID, result.ReceivedAnswer)))

	if !bytes.Equal(commitment[:], commitmentPayload.Commitment) {
		appLogger.Error().
			Str("registry id", cfg.SUI.RegistryID).
			Msg("Challenge verification failed")
		fmt.Fprintf(os.Stderr, "Error: Challenge verification failed\n")
		os.Exit(1)
	}

	appLogger.Info().
		Str("digest", digest).
		Msg("Challenge verification completed successfully")

	solverSinger, err := suisigner.NewSignerWithMnemonic(cfg.SUI.SolverMnemonic, suicrypto.KeySchemeFlagEd25519)
	if err != nil {
		appLogger.Error().Err(err).Msg("Failed to get solver address")
		os.Exit(1)
	}
	err = suiTxBuilder.VaultTransferBounty(
		ctx,
		cfg.SUI.VaultID,
		cfg.SUI.VaultAdminCapID,
		solverSinger.Address.String(),
	)
	if err != nil {
		appLogger.Error().Err(err).
			Str("VaultID", cfg.SUI.VaultID).
			Str("VaultAdminCapID,", cfg.SUI.VaultAdminCapID).
			Str("solver address", solverSinger.Address.String()).
			Msg("Failed to transfer bounty to solver")
		fmt.Fprintf(os.Stderr, "Error transfer bounty to solver: %v\n", err)
		os.Exit(1)
	}
}

// readDigestFromFile reads the transaction digest from the specified file
func readDigestFromFile(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("digest file path is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("digest file does not exist: %s", path)
		}
		return "", fmt.Errorf("failed to read digest file: %w", err)
	}

	digest := strings.TrimSpace(string(data))
	return digest, nil
}

type MoveCommitmentPayload struct {
	Id             *sui.ObjectId
	RegistryId     *sui.ObjectId
	ChallengerAddr *sui.Address
	SolverAddr     *sui.Address
	Score          uint64
	Timestamp      uint64
	Commitment     []byte
}

// LogEntry represents the response from the logs API
type LogEntry struct {
	ID             string `json:"id"`
	Log            string `json:"log"`
	ChallengerAddr string `json:"challenger_addr,omitempty"`
	SolverAddr     string `json:"solver_addr,omitempty"`
	VerifierAddr   string `json:"verifier_addr,omitempty"`
}

// Result represents the verification result structure
type Result struct {
	ReceivedAnswer string
	Status         string
}

// fetchLogEntry fetches log entry from the logs API by ID
func fetchLogEntry(logID string, cfg *config.Config, appLogger zerolog.Logger) (*LogEntry, error) {
	if cfg.LogsAPIBaseURL == "" {
		return nil, fmt.Errorf("LOGS_API_BASE_URL not configured")
	}
	if cfg.LogsAPIKey == "" {
		return nil, fmt.Errorf("LOGS_API_KEY not configured")
	}

	url := fmt.Sprintf("%s/api/logs/%s", cfg.LogsAPIBaseURL, logID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", cfg.LogsAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		appLogger.Error().
			Str("logID", logID).
			Int("httpStatus", resp.StatusCode).
			Msg("Non-200 response from logs API")
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var entry LogEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	appLogger.Info().
		Str("logID", logID).
		// Str("receivedAnswer", entry.ReceivedAnswer).
		// Str("status", entry.Status).
		Msg("Successfully fetched log entry")

	return &entry, nil
}

// extractCommitmentPayload attempts to extract commitment-related data from the transaction
func extractCommitmentPayload(objRes *suiclient.SuiObjectResponse) (*MoveCommitmentPayload, error) {
	var moveCommitmentPayload MoveCommitmentPayload
	_, err := bcs.Unmarshal(objRes.Data.Bcs.Data.MoveObject.BcsBytes, &moveCommitmentPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal to MoveCommitmentPayload: %w", err)
	}
	return &moveCommitmentPayload, nil
}

// showUsage displays help information
func showUsage() {
	fmt.Printf(`verifier - Sui transaction verifier for challenge commitments

Usage:
  verifier [options]

Options:
  --rpc-url string      Sui RPC URL (overrides SUI_RPC_URL config)
  --digest-file string  Path to digest file (overrides TX_DIGEST_FILE config)
  --help               Show this help message

Environment Variables:
  SUI_RPC_URL              Sui RPC endpoint URL
  TX_DIGEST_FILE           Path to file containing transaction digest
  SUI_INITIALIZER_MNEMONIC Mnemonic for transaction signing (enables TransactionBuilder)
  SUI_PACKAGE_ID           Package ID (required for TransactionBuilder)
  SUI_REGISTRY_ID          Registry ID (required for verification)

Examples:
  # Use config from environment to verify a transaction
  verifier

  # Override RPC URL
  verifier --rpc-url https://fullnode.testnet.sui.io:443

  # Override digest file
  verifier --digest-file ./custom_digest.txt

  # Override both
  verifier --rpc-url https://fullnode.testnet.sui.io:443 --digest-file ./custom_digest.txt
`)
}
