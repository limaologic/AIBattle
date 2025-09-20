// Package main provides a CLI for initializing and deploying smart contracts.
// Supports both Sui Move and Ethereum contract deployments with idempotent operations.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"reverse-challenge-system/internal/initializer"
	"reverse-challenge-system/pkg/config"
)

// CLI flags
var (
	configFile     = flag.String("config", "", "Path to .env config file (optional)")
	contractPath   = flag.String("contracts", "", "Override default contracts path")
	force          = flag.Bool("force", false, "Force deployment even if contract already exists")
	verbose        = flag.Bool("verbose", false, "Enable verbose logging")
	network        = flag.String("network", "", "Override default network/chain ID")
	dryRun         = flag.Bool("dry-run", false, "Perform a dry run without deploying")
	fundFromFaucet = flag.Bool("fund", true, "Request funds from faucet before deployment")
)

func main() {
	flag.Parse()

	// Initialize logging
	if err := initLogging(); err != nil {
		fmt.Printf("Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Configure options
	options := buildOptions(cfg)

	if *dryRun {
		log.Info().Msg("Dry run mode - contract deployment will be simulated")
		// In a real implementation, you might want to validate contracts and configuration
		log.Info().
			Str("contracts_path", cfg.ContractsPath).
			Str("network", cfg.SUI.ChainID).
			Str("rpc_url", cfg.SUI.RPCUrl).
			Msg("Configuration validated successfully")
		return
	}

	// Run the initializer
	log.Info().Msg("Starting contract deployment initialization")
	if err := initializer.Run(ctx, cfg, options...); err != nil {
		log.Fatal().Err(err).Msg("Contract deployment failed")
	}

	log.Info().Msg("Contract deployment completed successfully")
}

// initLogging sets up structured logging with appropriate level
func initLogging() error {
	// Configure zerolog
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"})

	// Set log level based on verbose flag
	if *verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Debug().Msg("Verbose logging enabled")
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	return nil
}

// loadConfig loads configuration from environment with optional overrides
func loadConfig() (*config.Config, error) {
	// Load base configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load base config: %w", err)
	}

	// Apply CLI overrides
	if *contractPath != "" {
		cfg.ContractsPath = *contractPath
		log.Debug().Str("contracts_path", cfg.ContractsPath).Msg("Override contracts path from CLI")
	}

	if *network != "" {
		cfg.SUI.ChainID = *network
		log.Debug().Str("network", cfg.SUI.ChainID).Msg("Override network from CLI")
	}

	// Validate required configuration
	if err := validateDeploymentConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid deployment configuration: %w", err)
	}

	return cfg, nil
}

// validateDeploymentConfig ensures all required deployment settings are present
func validateDeploymentConfig(cfg *config.Config) error {
	if cfg.SUI.RPCUrl == "" {
		return fmt.Errorf("SUI_RPC_URL is required for deployment")
	}

	if cfg.SUI.InitializerMnemonic == "" {
		return fmt.Errorf("SUI_INITIALIZER_MNEMONIC is required for deployment")
	}

	if cfg.SUI.ChainID == "" {
		return fmt.Errorf("SUI_CHAIN_ID is required for deployment")
	}

	// Check if contracts path exists
	if _, err := os.Stat(cfg.ContractsPath); os.IsNotExist(err) {
		return fmt.Errorf("contracts path does not exist: %s", cfg.ContractsPath)
	}

	// Check for Move.toml in the contracts path or its subdirectories
	moveTomlPath := filepath.Join(cfg.ContractsPath, "Move.toml")
	if _, err := os.Stat(moveTomlPath); os.IsNotExist(err) {
		// Try looking for Move.toml in subdirectories
		found := false
		entries, err := os.ReadDir(cfg.ContractsPath)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					subMoveToml := filepath.Join(cfg.ContractsPath, entry.Name(), "Move.toml")
					if _, err := os.Stat(subMoveToml); err == nil {
						found = true
						break
					}
				}
			}
		}
		if !found {
			return fmt.Errorf("Move.toml not found in contracts path or its subdirectories: %s", cfg.ContractsPath)
		}
	}

	return nil
}

// buildOptions creates initializer options from CLI flags and configuration
func buildOptions(cfg *config.Config) []func(*initializer.Options) {
	var options []func(*initializer.Options)

	// Contract path override
	if *contractPath != "" {
		options = append(options, initializer.WithContractPath(*contractPath))
	}

	// Skip if exists behavior
	skipIfExists := !*force // If force is true, don't skip existing deployments
	options = append(options, initializer.WithSkipIfExists(skipIfExists))

	// Fund from faucet option
	if *fundFromFaucet {
		options = append(options, initializer.WithFundFromFaucet(true))
	}

	return options
}
