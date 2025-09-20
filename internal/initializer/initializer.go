package initializer

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pattonkan/sui-go/sui"
	"github.com/pattonkan/sui-go/suiclient"
	"github.com/pattonkan/sui-go/suisigner"
	"github.com/pattonkan/sui-go/suisigner/suicrypto"
	"github.com/rs/zerolog/log"

	"reverse-challenge-system/pkg/config"
	"reverse-challenge-system/pkg/db"
	"reverse-challenge-system/pkg/models"
)

// Options configures the initializer behavior
type Options struct {
	ContractPath   string // Override default contract path
	SkipIfExists   bool   // Skip deployment if contract already exists
	FundFromFaucet bool   // Request funds from faucet before deployment
}

// Result contains deployment results
type Result struct {
	PackageId     *sui.PackageId
	RegistryId    *sui.ObjectId
	TransactionId string
	Network       string
	Metadata      map[string]interface{}
}

// Run executes the contract initialization process
func Run(ctx context.Context, cfg *config.Config, opts ...func(*Options)) error {
	// Apply options
	options := &Options{
		ContractPath: filepath.Join(cfg.ContractsPath, "sources"),
		SkipIfExists: true,
	}
	for _, opt := range opts {
		opt(options)
	}

	log.Info().Msg("Starting contract deployment initialization")

	// Validate required configuration
	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize database connection
	database, err := db.NewDatabase(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer database.Close()

	// // Check for existing deployment
	// if options.SkipIfExists {
	// 	existing, err := database.GetContractByName(ctx, "conditional_tokens_framework", cfg.SUI.ChainID)
	// 	if err != nil && err.Error() != "contract not found" {
	// 		return fmt.Errorf("failed to check existing contracts: %w", err)
	// 	}
	// 	if existing != nil {
	// 		log.Info().
	// 			Str("package_id", existing.Address).
	// 			Str("network", existing.Network).
	// 			Msg("Contract already deployed, skipping deployment")
	// 		return nil
	// 	}
	// }

	// Initialize Sui client and signer
	client, signer, err := initializeSuiClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize Sui client: %w", err)
	}

	// Request funds from faucet if requested
	if options.FundFromFaucet {
		if err := fundFromFaucet(client, signer, cfg); err != nil {
			return fmt.Errorf("failed to fund from faucet: %w", err)
		}
	}

	// Deploy contracts
	deployer := NewSuiDeployer(client, signer, options.ContractPath)
	result, err := deployer.Deploy(ctx, cfg)
	if err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	// Store deployment result in database
	contract := &models.Contract{
		Name:         "conditional_tokens_framework",
		Address:      result.PackageId.String(),
		Network:      cfg.SUI.ChainID,
		TxHash:       result.TransactionId,
		DeployedAt:   time.Now(),
		ChainID:      cfg.SUI.ChainID,
		ContractType: "sui_move_package",
		Metadata: map[string]interface{}{
			"registry_id":        result.RegistryId.String(),
			"pos_package_id":     result.Metadata["pos_package_id"],
			"neg_package_id":     result.Metadata["neg_package_id"],
			"vault_id":           result.Metadata["vault_id"],
			"vault_admin_cap_id": result.Metadata["vault_admin_cap_id"],
		},
	}

	if err := database.SaveContract(ctx, contract); err != nil {
		return fmt.Errorf("failed to save contract deployment: %w", err)
	}

	// Update .env file with deployment results
	if err := updateEnvFile(result.PackageId.String(), result.RegistryId.String(), result.Metadata); err != nil {
		log.Warn().Err(err).Msg("Failed to update .env file with deployment results")
		// Don't fail the deployment if .env update fails
	}

	log.Info().
		Str("package_id", result.PackageId.String()).
		Str("registry_id", result.RegistryId.String()).
		Str("transaction_id", result.TransactionId).
		Str("network", cfg.SUI.ChainID).
		Msg("Successfully deployed contracts and saved to database")

	return nil
}

// updateEnvFile updates the .env file with the deployed package and registry IDs
func updateEnvFile(packageId, registryId string, metadata map[string]interface{}) error {
	envPath := ".env"

	// Extract all relevant IDs from metadata
	var posPackageId, negPackageId, vaultId, vaultAdminCapId string
	if metadata != nil {
		if pos, ok := metadata["pos_package_id"].(string); ok {
			posPackageId = pos
		}
		if neg, ok := metadata["neg_package_id"].(string); ok {
			negPackageId = neg
		}
		if vault, ok := metadata["vault_id"].(string); ok {
			vaultId = vault
		}
		if vaultCap, ok := metadata["vault_admin_cap_id"].(string); ok {
			vaultAdminCapId = vaultCap
		}
	}

	log.Info().
		Str("package_id", packageId).
		Str("registry_id", registryId).
		Str("pos_package_id", posPackageId).
		Str("neg_package_id", negPackageId).
		Str("vault_id", vaultId).
		Str("vault_admin_cap_id", vaultAdminCapId).
		Msg("Updating .env file with deployment results")

	// Read the existing .env file
	file, err := os.Open(envPath)
	if err != nil {
		return fmt.Errorf("failed to open .env file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)

	packageIdPattern := regexp.MustCompile(`^SUI_PACKAGE_ID=.*`)
	registryIdPattern := regexp.MustCompile(`^SUI_REGISTRY_ID=.*`)
	posPackageIdPattern := regexp.MustCompile(`^SUI_POS_PACKAGE_ID=.*`)
	negPackageIdPattern := regexp.MustCompile(`^SUI_NEG_PACKAGE_ID=.*`)
	vaultIdPattern := regexp.MustCompile(`^SUI_VAULT_ID=.*`)
	vaultAdminCapIdPattern := regexp.MustCompile(`^SUI_VAULT_ADMIN_CAP_ID=.*`)

	packageIdUpdated := false
	registryIdUpdated := false
	posPackageIdUpdated := false
	negPackageIdUpdated := false
	vaultIdUpdated := false
	vaultAdminCapIdUpdated := false

	// Read and update existing lines
	for scanner.Scan() {
		line := scanner.Text()

		if packageIdPattern.MatchString(line) {
			lines = append(lines, fmt.Sprintf("SUI_PACKAGE_ID=%s", packageId))
			packageIdUpdated = true
		} else if registryIdPattern.MatchString(line) {
			lines = append(lines, fmt.Sprintf("SUI_REGISTRY_ID=%s", registryId))
			registryIdUpdated = true
		} else if posPackageIdPattern.MatchString(line) && posPackageId != "" {
			lines = append(lines, fmt.Sprintf("SUI_POS_PACKAGE_ID=%s", posPackageId))
			posPackageIdUpdated = true
		} else if negPackageIdPattern.MatchString(line) && negPackageId != "" {
			lines = append(lines, fmt.Sprintf("SUI_NEG_PACKAGE_ID=%s", negPackageId))
			negPackageIdUpdated = true
		} else if vaultIdPattern.MatchString(line) && vaultId != "" {
			lines = append(lines, fmt.Sprintf("SUI_VAULT_ID=%s", vaultId))
			vaultIdUpdated = true
		} else if vaultAdminCapIdPattern.MatchString(line) && vaultAdminCapId != "" {
			lines = append(lines, fmt.Sprintf("SUI_VAULT_ADMIN_CAP_ID=%s", vaultAdminCapId))
			vaultAdminCapIdUpdated = true
		} else {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading .env file: %w", err)
	}

	// Add missing entries if they weren't found
	if !packageIdUpdated {
		lines = append(lines, fmt.Sprintf("SUI_PACKAGE_ID=%s", packageId))
	}
	if !registryIdUpdated {
		lines = append(lines, fmt.Sprintf("SUI_REGISTRY_ID=%s", registryId))
	}
	if !posPackageIdUpdated && posPackageId != "" {
		lines = append(lines, fmt.Sprintf("SUI_POS_PACKAGE_ID=%s", posPackageId))
	}
	if !negPackageIdUpdated && negPackageId != "" {
		lines = append(lines, fmt.Sprintf("SUI_NEG_PACKAGE_ID=%s", negPackageId))
	}
	if !vaultIdUpdated && vaultId != "" {
		lines = append(lines, fmt.Sprintf("SUI_VAULT_ID=%s", vaultId))
	}
	if !vaultAdminCapIdUpdated && vaultAdminCapId != "" {
		lines = append(lines, fmt.Sprintf("SUI_VAULT_ADMIN_CAP_ID=%s", vaultAdminCapId))
	}

	// Write the updated content back to the file
	content := strings.Join(lines, "\n") + "\n"
	err = os.WriteFile(envPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}

	log.Info().
		Str("env_path", envPath).
		Msg("Successfully updated .env file with deployment results")

	return nil
}

// validateConfig ensures all required configuration is present
func validateConfig(cfg *config.Config) error {
	if cfg.SUI.RPCUrl == "" {
		return fmt.Errorf("SUI_RPC_URL is required")
	}
	if cfg.SUI.InitializerMnemonic == "" {
		return fmt.Errorf("SUI_INITIALIZER_MNEMONIC is required")
	}
	if cfg.SUI.ChainID == "" {
		return fmt.Errorf("SUI_CHAIN_ID is required")
	}
	return nil
}

// initializeSuiClient creates a Sui client and signer from configuration
func initializeSuiClient(cfg *config.Config) (*suiclient.ClientImpl, *suisigner.Signer, error) {
	// Create Sui client
	client := suiclient.NewClient(cfg.SUI.RPCUrl)

	// Create signer from mnemonic
	signer, err := suisigner.NewSignerWithMnemonic(cfg.SUI.InitializerMnemonic, suicrypto.KeySchemeFlagEd25519)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create signer: %w", err)
	}

	return client, signer, nil
}

// fundFromFaucet requests SUI tokens from the faucet for the deployer address
func fundFromFaucet(client *suiclient.ClientImpl, signer *suisigner.Signer, cfg *config.Config) error {
	log.Info().
		Str("address", signer.Address.String()).
		Str("network", cfg.SUI.ChainID).
		Msg("Requesting funds from faucet")

	// Determine faucet URL based on network
	var faucetUrl string
	fmt.Println("cfg.SUI.ChainID: ", cfg.SUI.ChainID)
	switch cfg.SUI.ChainID {
	case "testnet":
		faucetUrl = "https://faucet.testnet.sui.io/gas"
	case "devnet":
		faucetUrl = "https://faucet.devnet.sui.io/gas"
	case "localnet":
		faucetUrl = "http://127.0.0.1:9123/gas" // Default localnet faucet
	default:
		return fmt.Errorf("faucet not available for network: %s (use testnet, devnet, or localnet)", cfg.SUI.ChainID)
	}

	err := suiclient.RequestFundFromFaucet(signer.Address, faucetUrl)
	if err != nil {
		return fmt.Errorf("failed to request funds from faucet %s: %w", faucetUrl, err)
	}

	log.Info().
		Str("address", signer.Address.String()).
		Str("faucet_url", faucetUrl).
		Msg("Successfully requested funds from faucet")

	return nil
}

// WithContractPath sets a custom contract path
func WithContractPath(path string) func(*Options) {
	return func(o *Options) {
		o.ContractPath = path
	}
}

// WithSkipIfExists controls whether to skip deployment if contract exists
func WithSkipIfExists(skip bool) func(*Options) {
	return func(o *Options) {
		o.SkipIfExists = skip
	}
}

// WithFundFromFaucet controls whether to request funds from faucet before deployment
func WithFundFromFaucet(fund bool) func(*Options) {
	return func(o *Options) {
		o.FundFromFaucet = fund
	}
}
