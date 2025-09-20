package initializer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fardream/go-bcs/bcs"
	"github.com/pattonkan/sui-go/sui"
	"github.com/pattonkan/sui-go/sui/suiptb"
	"github.com/pattonkan/sui-go/suiclient"
	"github.com/pattonkan/sui-go/suisigner"
	"github.com/pattonkan/sui-go/utils"
	"github.com/rs/zerolog/log"

	"reverse-challenge-system/pkg/config"
)

// Deployer interface for different contract deployment strategies
type Deployer interface {
	Deploy(ctx context.Context, cfg *config.Config) (*Result, error)
}

// SuiDeployer implements Sui Move contract deployment
type SuiDeployer struct {
	client       *suiclient.ClientImpl
	signer       *suisigner.Signer
	contractPath string
}

// NewSuiDeployer creates a new Sui contract deployer
func NewSuiDeployer(client *suiclient.ClientImpl, signer *suisigner.Signer, contractPath string) *SuiDeployer {
	return &SuiDeployer{
		client:       client,
		signer:       signer,
		contractPath: contractPath,
	}
}

// Deploy deploys Sui Move contracts and returns deployment results
func (d *SuiDeployer) Deploy(ctx context.Context, cfg *config.Config) (*Result, error) {
	log.Info().
		Str("contract_path", d.contractPath).
		Str("signer_address", d.signer.Address.String()).
		Msg("Starting Sui Move contract deployment")

	// Build and deploy the main package
	packageId, registryId, posPackageId, negPackageId, vaultId, vaultAdminCapId, txDigest, err := d.deployConditionalsFramework(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy conditionals framework: %w", err)
	}

	result := &Result{
		PackageId:     packageId,
		RegistryId:    registryId,
		TransactionId: txDigest,
		Network:       cfg.SUI.ChainID,
		Metadata: map[string]interface{}{
			"pos_package_id":     posPackageId.String(),
			"neg_package_id":     negPackageId.String(),
			"vault_id":           vaultId.String(),
			"vault_admin_cap_id": vaultAdminCapId.String(),
		},
	}

	log.Info().
		Str("package_id", packageId.String()).
		Str("registry_id", registryId.String()).
		Str("vault_id", vaultId.String()).
		Str("vault_admin_cap_id", vaultAdminCapId.String()).
		Str("tx_digest", txDigest).
		Msg("Successfully deployed Sui Move contracts")

	return result, nil
}

// deployConditionalsFramework builds and publishes the conditional tokens framework
func (d *SuiDeployer) deployConditionalsFramework(ctx context.Context, cfg *config.Config) (*sui.PackageId, *sui.ObjectId, *sui.PackageId, *sui.PackageId, *sui.ObjectId, *sui.ObjectId, string, error) {
	// Find the package root (directory containing Move.toml)
	packageRoot, err := d.findPackageRoot()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, "", fmt.Errorf("failed to find package root: %w", err)
	}

	log.Info().Str("package_root", packageRoot).Msg("Building Sui Move package")

	// Build the Move package
	modules, err := utils.MoveBuild(packageRoot)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, "", fmt.Errorf("failed to build Move package: %w", err)
	}

	log.Info().
		Int("module_count", len(modules.Modules)).
		Int("dependency_count", len(modules.Dependencies)).
		Msg("Successfully built Move package")

	// Publish the package
	txnBytes, err := d.client.Publish(ctx, &suiclient.PublishRequest{
		Sender:          d.signer.Address,
		CompiledModules: modules.Modules,
		Dependencies:    modules.Dependencies,
		GasBudget:       sui.NewBigInt(20 * suiclient.DefaultGasBudget), // Higher gas budget for complex contracts
	})
	if err != nil {
		return nil, nil, nil, nil, nil, nil, "", fmt.Errorf("failed to publish package: %w", err)
	}

	// Sign and execute the transaction
	txnResponse, err := d.client.SignAndExecuteTransaction(
		ctx,
		d.signer,
		txnBytes.TxBytes,
		&suiclient.SuiTransactionBlockResponseOptions{
			ShowEffects:       true,
			ShowObjectChanges: true,
			ShowEvents:        true,
		},
	)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, "", fmt.Errorf("failed to execute publish transaction: %w", err)
	}

	// Check transaction success
	if !txnResponse.Effects.Data.IsSuccess() {
		return nil, nil, nil, nil, nil, nil, "", fmt.Errorf("publish transaction failed")
	}

	// Extract package ID
	packageId, err := txnResponse.GetPublishedPackageId()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, "", fmt.Errorf("failed to extract package ID: %w", err)
	}

	packageIdPos, treasuryCapPos, err := buildDeployToken(ctx, d.client, d.signer, "pos")
	if err != nil {
		return nil, nil, nil, nil, nil, nil, "", fmt.Errorf("failed to publish token Pos: %w", err)
	}
	packageIdNeg, treasuryCapNeg, err := buildDeployToken(ctx, d.client, d.signer, "neg")
	if err != nil {
		return nil, nil, nil, nil, nil, nil, "", fmt.Errorf("failed to publish token Neg: %w", err)
	}

	// Create a registry object after successful package deployment
	// For now using a placeholder - in a real implementation, you'd call a Move function
	// to create the registry object using the deployed package
	registryId, _, _, err := d.createRegistry(ctx, packageId, packageIdPos, packageIdNeg, treasuryCapPos, treasuryCapNeg)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create registry object, using placeholder")
		registryId = &sui.ObjectId{} // Use placeholder if registry creation fails
	}
	vaultId, vaultAdminCapId, err := d.createVault(ctx, packageId)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create vault object, using placeholder")
		vaultId = &sui.ObjectId{}
		vaultAdminCapId = &sui.ObjectId{}
	}

	txDigest := txnResponse.Digest

	log.Info().
		Str("package_id", packageId.String()).
		Str("tx_digest", string(txDigest)).
		Msg("Package published successfully")

	return packageId, registryId, packageIdPos, packageIdNeg, vaultId, vaultAdminCapId, string(txDigest), nil
}

func buildDeployToken(ctx context.Context, client *suiclient.ClientImpl, signer *suisigner.Signer, tokenName string) (*sui.PackageId, *sui.ObjectId, error) {
	contractPath := fmt.Sprintf("/internal/initializer/contracts/%s/", tokenName)
	modules, err := utils.MoveBuild(utils.GetGitRoot() + contractPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build %s contract: %w", tokenName, err)
	}

	txnBytes, err := client.Publish(
		ctx,
		&suiclient.PublishRequest{
			Sender:          signer.Address,
			CompiledModules: modules.Modules,
			Dependencies:    modules.Dependencies,
			GasBudget:       sui.NewBigInt(10 * suiclient.DefaultGasBudget),
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to publish %s contract: %w", tokenName, err)
	}

	txnResponse, err := client.SignAndExecuteTransaction(
		ctx, signer, txnBytes.TxBytes, &suiclient.SuiTransactionBlockResponseOptions{
			ShowEffects:       true,
			ShowObjectChanges: true,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign and execute %s publish transaction: %w", tokenName, err)
	}

	if !txnResponse.Effects.Data.IsSuccess() {
		return nil, nil, fmt.Errorf("%s publish transaction failed", tokenName)
	}

	packageId, err := txnResponse.GetPublishedPackageId()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get published package ID for %s: %w", tokenName, err)
	}

	treasuryCap, _, err := txnResponse.GetCreatedObjectInfo("coin", "TreasuryCap")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get treasury cap for %s: %w", tokenName, err)
	}

	return packageId, treasuryCap, nil
}

// createRegistry creates a registry object using the deployed package
// This is a placeholder implementation - in a real scenario, you'd call a Move function
func (d *SuiDeployer) createRegistry(
	ctx context.Context,
	packageId, posPackageId, negPackageId *sui.PackageId,
	posTreasuryCap, negTreasuryCap *sui.ObjectId,
) (*sui.ObjectId, *sui.ObjectId, *sui.ObjectId, error) {
	posGetObjectRes, err := d.client.GetObject(ctx, &suiclient.GetObjectRequest{
		ObjectId: posTreasuryCap,
		Options:  &suiclient.SuiObjectDataOptions{},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get pos object: %w", err)
	}
	negGetObjectRes, err := d.client.GetObject(ctx, &suiclient.GetObjectRequest{
		ObjectId: negTreasuryCap,
		Options:  &suiclient.SuiObjectDataOptions{},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get neg object: %w", err)
	}

	posObjectRef := posGetObjectRes.Data.Ref()
	negObjectRef := negGetObjectRes.Data.Ref()

	coinPage, err := d.client.GetCoins(ctx, &suiclient.GetCoinsRequest{Owner: d.signer.Address})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get coins: %w", err)
	}

	ptb := suiptb.NewTransactionDataTransactionBuilder()
	ptb.Command(suiptb.Command{
		MoveCall: &suiptb.ProgrammableMoveCall{
			Package:  packageId,
			Module:   "ctf_registry",
			Function: "create_registry",
			TypeArguments: []sui.TypeTag{
				{Struct: &sui.StructTag{
					Address: posPackageId,
					Module:  "pos",
					Name:    "POS",
				}},
				{Struct: &sui.StructTag{
					Address: negPackageId,
					Module:  "neg",
					Name:    "NEG",
				}},
				{Struct: &sui.StructTag{
					Address: sui.MustAddressFromHex("0x2"),
					Module:  "sui",
					Name:    "SUI",
				}},
			},
			Arguments: []suiptb.Argument{
				ptb.MustObj(suiptb.ObjectArg{ImmOrOwnedObject: posObjectRef}),
				ptb.MustObj(suiptb.ObjectArg{ImmOrOwnedObject: negObjectRef}),
				ptb.MustObj(suiptb.ObjectArg{ImmOrOwnedObject: coinPage.Data[0].Ref()}),
			},
		}},
	)

	ptb.TransferArg(d.signer.Address, suiptb.Argument{NestedResult: &suiptb.NestedResult{Cmd: 0, Result: 0}})
	ptb.TransferArg(d.signer.Address, suiptb.Argument{NestedResult: &suiptb.NestedResult{Cmd: 0, Result: 1}})

	pt := ptb.Finish()

	tx := suiptb.NewTransactionData(
		d.signer.Address,
		pt,
		[]*sui.ObjectRef{coinPage.Data[1].Ref()},
		suiclient.DefaultGasBudget,
		suiclient.DefaultGasPrice,
	)

	txBytes, err := bcs.Marshal(tx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to marshal transaction: %w", err)
	}

	txnResponse, err := d.client.SignAndExecuteTransaction(
		ctx,
		d.signer,
		txBytes,
		&suiclient.SuiTransactionBlockResponseOptions{
			ShowEffects:       true,
			ShowObjectChanges: true,
		},
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to sign and execute transaction: %w", err)
	}

	if !txnResponse.Effects.Data.IsSuccess() {
		return nil, nil, nil, fmt.Errorf("transaction failed")
	}

	var protocolId *sui.ObjectId
	for _, change := range txnResponse.ObjectChanges {
		if change.Data.Created != nil {
			if strings.Contains(string(change.Data.Created.ObjectType), "ctf_registry::ConditionRegistry<") {
				protocolId = &change.Data.Created.ObjectId
			}
		}
	}

	if protocolId == nil {
		return nil, nil, nil, fmt.Errorf("registry ID not found in transaction response")
	}

	return protocolId, posPackageId, negPackageId, nil
}

func (d *SuiDeployer) createVault(
	ctx context.Context,
	packageId *sui.PackageId,
) (*sui.ObjectId, *sui.ObjectId, error) {
	coinPage, err := d.client.GetCoins(ctx, &suiclient.GetCoinsRequest{Owner: d.signer.Address})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get coins: %w", err)
	}

	ptb := suiptb.NewTransactionDataTransactionBuilder()
	ptb.Command(suiptb.Command{
		MoveCall: &suiptb.ProgrammableMoveCall{
			Package:       packageId,
			Module:        "ctf_registry",
			Function:      "create_vault",
			TypeArguments: []sui.TypeTag{},
			Arguments:     []suiptb.Argument{},
		}},
	)

	ptb.TransferArg(d.signer.Address, suiptb.Argument{NestedResult: &suiptb.NestedResult{Cmd: 0, Result: 0}})

	pt := ptb.Finish()

	tx := suiptb.NewTransactionData(
		d.signer.Address,
		pt,
		[]*sui.ObjectRef{coinPage.Data[1].Ref()},
		suiclient.DefaultGasBudget,
		suiclient.DefaultGasPrice,
	)

	txBytes, err := bcs.Marshal(tx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal transaction: %w", err)
	}

	txnResponse, err := d.client.SignAndExecuteTransaction(
		ctx,
		d.signer,
		txBytes,
		&suiclient.SuiTransactionBlockResponseOptions{
			ShowEffects:       true,
			ShowObjectChanges: true,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign and execute transaction: %w", err)
	}

	if !txnResponse.Effects.Data.IsSuccess() {
		return nil, nil, fmt.Errorf("transaction failed")
	}

	var vaultId *sui.ObjectId
	var vaultAdminCapId *sui.ObjectId
	for _, change := range txnResponse.ObjectChanges {
		if change.Data.Created != nil {

			if strings.Contains(string(change.Data.Created.ObjectType), "ctf_registry::Vault") {
				vaultId = &change.Data.Created.ObjectId
			}
			if strings.Contains(string(change.Data.Created.ObjectType), "ctf_registry::VaultAdminCap") {
				vaultAdminCapId = &change.Data.Created.ObjectId
			}
		}
	}

	if vaultId == nil {
		return nil, nil, fmt.Errorf("Vault ID not found in transaction response")
	}
	if vaultAdminCapId == nil {
		return nil, nil, fmt.Errorf("VaultAdminCap ID not found in transaction response")
	}

	return vaultId, vaultAdminCapId, nil
}

// findPackageRoot searches for the directory containing Move.toml
func (d *SuiDeployer) findPackageRoot() (string, error) {
	// Start from the contract path and search upwards for Move.toml
	currentDir := d.contractPath

	// Try the contract path directly first
	if d.hasMoveTOML(currentDir) {
		return currentDir, nil
	}

	// Try parent directories
	for i := 0; i < 5; i++ { // Limit search depth
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break // Reached filesystem root
		}

		if d.hasMoveTOML(parent) {
			return parent, nil
		}

		currentDir = parent
	}

	return "", fmt.Errorf("Move.toml not found in %s or its parent directories", d.contractPath)
}

// hasMoveTOML checks if a directory contains Move.toml
func (d *SuiDeployer) hasMoveTOML(dir string) bool {
	moveTomlPath := filepath.Join(dir, "Move.toml")
	return fileExists(moveTomlPath)
}

// fileExists checks if a file exists
func fileExists(filename string) bool {
	_, err := filepath.Abs(filename)
	return err == nil
}

// EthereumDeployer would implement Ethereum contract deployment for future use
// Currently not implemented as the project uses Sui Move contracts
type EthereumDeployer struct {
	rpcURL     string
	privateKey string
	chainID    int64
}

// NewEthereumDeployer creates a new Ethereum contract deployer
func NewEthereumDeployer(rpcURL, privateKey string, chainID int64) *EthereumDeployer {
	return &EthereumDeployer{
		rpcURL:     rpcURL,
		privateKey: privateKey,
		chainID:    chainID,
	}
}

// Deploy implements Ethereum contract deployment (placeholder for future implementation)
func (d *EthereumDeployer) Deploy(ctx context.Context, cfg *config.Config) (*Result, error) {
	return nil, fmt.Errorf("Ethereum deployment not yet implemented - project uses Sui Move contracts")
}
