package sui

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/fardream/go-bcs/bcs"
	"github.com/pattonkan/sui-go/sui"
	"github.com/pattonkan/sui-go/sui/suiptb"
	"github.com/pattonkan/sui-go/suiclient"
	"github.com/pattonkan/sui-go/suisigner"
	"github.com/pattonkan/sui-go/suisigner/suicrypto"
	"github.com/rs/zerolog"
)

// TypeArgs represents the three generic types needed for upload_challenge_commitment
type TypeArgs struct {
	TreasuryCapPositive string
	TreasuryCapNegative string
	CoinTypeCollateral  string
}

// TransactionBuilder handles Sui blockchain interactions for the challenger service
type TransactionBuilder struct {
	client    *suiclient.ClientImpl
	packageID *sui.PackageId
	signer    *suisigner.Signer
	logger    zerolog.Logger
}

// NewTransactionBuilder creates a new TransactionBuilder instance
func NewTransactionBuilder(ctx context.Context, logger zerolog.Logger, rpcURL string, packageID string, mnemonic string) (*TransactionBuilder, error) {
	if rpcURL == "" {
		return nil, fmt.Errorf("rpcURL cannot be empty")
	}
	if packageID == "" {
		return nil, fmt.Errorf("packageID cannot be empty")
	}
	if mnemonic == "" {
		return nil, fmt.Errorf("mnemonic cannot be empty")
	}

	// Create Sui client
	client := suiclient.NewClient(rpcURL)

	// Derive keypair from mnemonic (using Ed25519)
	signer, err := suisigner.NewSignerWithMnemonic(mnemonic, suicrypto.KeySchemeFlagEd25519)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer from mnemonic: %w", err)
	}

	// Parse package ID
	pkgID, err := sui.PackageIdFromHex(packageID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse package ID: %w", err)
	}

	return &TransactionBuilder{
		client:    client,
		packageID: pkgID,
		signer:    signer,
		logger:    logger.With().Str("component", "sui_txbuilder").Logger(),
	}, nil
}

func (tb *TransactionBuilder) Signer() *suisigner.Signer {
	return tb.signer
}

func (tb *TransactionBuilder) PackageId() *sui.PackageId {
	return tb.packageID
}

// BuildUploadChallengeCommitment builds a Move call transaction for upload_challenge_commitment
func (tb *TransactionBuilder) BuildUploadChallengeCommitment(
	ctx context.Context,
	typeArgs TypeArgs,
	registryId string,
	commitment []byte,
	challengerAddr string,
	solverAddr string,
	score uint64,
	timestamp uint64,
) (*suiptb.ProgrammableTransaction, error) {
	// Parse addresses
	challengerAddress, err := sui.AddressFromHex(challengerAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid challenger address: %w", err)
	}

	solverAddress, err := sui.AddressFromHex(solverAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid solver address: %w", err)
	}

	// Parse registry object ID
	registryObjID, err := sui.ObjectIdFromHex(registryId)
	if err != nil {
		return nil, fmt.Errorf("invalid registry object ID: %w", err)
	}

	registryGetObject, err := tb.client.GetObject(ctx, &suiclient.GetObjectRequest{
		ObjectId: registryObjID,
		Options: &suiclient.SuiObjectDataOptions{
			ShowContent: true,
			ShowBcs:     true,
			ShowOwner:   true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get registry object object: %w", err)
	}

	registryRef := registryGetObject.Data.RefSharedObject()

	ptb := suiptb.NewTransactionDataTransactionBuilder()

	challengerArg := ptb.MustForceSeparatePure(challengerAddress)
	commitmentArg := ptb.Command(suiptb.Command{
		MoveCall: &suiptb.ProgrammableMoveCall{
			Package:  tb.packageID,
			Module:   "ctf_registry",
			Function: "upload_challenge_commitment",
			TypeArguments: []sui.TypeTag{
				*sui.MustNewTypeTag(typeArgs.TreasuryCapPositive),
				*sui.MustNewTypeTag(typeArgs.TreasuryCapNegative),
				*sui.MustNewTypeTag(typeArgs.CoinTypeCollateral),
			},
			Arguments: []suiptb.Argument{
				ptb.MustObj(suiptb.ObjectArg{SharedObject: &suiptb.SharedObjectArg{
					Id:                   registryRef.ObjectId,
					InitialSharedVersion: registryRef.Version,
					Mutable:              true,
				}}),
				ptb.MustPure(commitment),
				challengerArg,
				ptb.MustPure(solverAddress),
				ptb.MustPure(score),
				ptb.MustPure(timestamp),
			},
		},
	})

	ptb.Command(suiptb.Command{
		TransferObjects: &suiptb.ProgrammableTransferObjects{
			Objects: []suiptb.Argument{commitmentArg},
			Address: challengerArg,
		},
	})

	pt := ptb.Finish()
	return &pt, nil
}

// SelectGasObject selects the highest balance SUI coin for gas payment
func (tb *TransactionBuilder) SelectGasObject(ctx context.Context, owner string) (*sui.ObjectId, error) {
	ownerAddr, err := sui.AddressFromHex(owner)
	if err != nil {
		return nil, fmt.Errorf("invalid owner address: %w", err)
	}

	// Get owned SUI coins
	resp, err := tb.client.GetCoins(ctx, &suiclient.GetCoinsRequest{
		Owner:    ownerAddr,
		CoinType: nil, // SUI coins (default)
		Cursor:   nil,
		Limit:    50,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get SUI coins: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no SUI coins found for address %s", owner)
	}

	// Sort coins by balance (descending) and select the highest one
	coins := resp.Data
	sort.Slice(coins, func(i, j int) bool {
		return coins[i].Balance.Uint64() > coins[j].Balance.Uint64()
	})

	selectedCoin := coins[0]
	tb.logger.Debug().
		Str("coin_id", selectedCoin.CoinObjectId.String()).
		Str("balance", selectedCoin.Balance.String()).
		Msg("Selected gas coin")

	return selectedCoin.CoinObjectId, nil
}

// SignAndExecute signs and executes the transaction
func (tb *TransactionBuilder) SignAndExecute(ctx context.Context, txBytes *suiclient.TransactionBytes) (string, error) {
	// Sign the transaction
	signature, err := tb.signer.SignDigest(txBytes.TxBytes.Data(), suisigner.IntentTransaction())
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Execute the transaction
	resp, err := tb.client.ExecuteTransactionBlock(ctx, &suiclient.ExecuteTransactionBlockRequest{
		TxDataBytes: txBytes.TxBytes.Data(),
		Signatures:  []*suisigner.Signature{signature},
		Options: &suiclient.SuiTransactionBlockResponseOptions{
			ShowInput:          true,
			ShowEffects:        true,
			ShowEvents:         true,
			ShowObjectChanges:  true,
			ShowBalanceChanges: true,
		},
		RequestType: suiclient.TxnRequestTypeWaitForLocalExecution,
	})
	if err != nil {
		return "", fmt.Errorf("failed to execute transaction: %w", err)
	}

	digestStr := string(resp.Digest)
	tb.logger.Info().
		Str("digest", digestStr).
		Interface("status", resp.Effects.Data.V1.Status).
		Msg("Transaction executed")

	// Check if transaction was successful
	if resp.Effects.Data.V1.Status.Status != "success" {
		return digestStr, fmt.Errorf("transaction failed with status: %v", resp.Effects.Data.V1.Status)
	}

	return digestStr, nil
}

// UploadChallengeCommitment is a convenience method that builds, signs, and executes the transaction
func (tb *TransactionBuilder) UploadChallengeCommitment(
	ctx context.Context,
	typeArgs TypeArgs,
	registryId string,
	commitment []byte,
	challengerAddr string,
	solverAddr string,
	score uint64,
	timestamp uint64,
) (*sui.ObjectId, error) {
	tb.logger.Debug().
		Str("registry_id", registryId).
		Str("challenger_addr", challengerAddr).
		Str("solver_addr", solverAddr).
		Uint64("score", score).
		Uint64("timestamp", timestamp).
		Str("commitment_hex", hex.EncodeToString(commitment)).
		Msg("Building upload_challenge_commitment transaction")

	// Build the transaction
	pt, err := tb.BuildUploadChallengeCommitment(
		ctx, typeArgs, registryId, commitment,
		challengerAddr, solverAddr, score, timestamp,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build transaction: %w", err)
	}

	coinPage, err := tb.client.GetCoins(ctx, &suiclient.GetCoinsRequest{Owner: tb.signer.Address})
	if err != nil {
		return nil, fmt.Errorf("failed to get coins: %w", err)
	}

	tx := suiptb.NewTransactionData(
		tb.signer.Address,
		*pt,
		[]*sui.ObjectRef{coinPage.Data[1].Ref()},
		suiclient.DefaultGasBudget,
		suiclient.DefaultGasPrice,
	)

	txBytes, err := bcs.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction: %w", err)
	}

	txnResponse, err := tb.client.SignAndExecuteTransaction(
		ctx,
		tb.signer,
		txBytes,
		&suiclient.SuiTransactionBlockResponseOptions{
			ShowEffects:       true,
			ShowObjectChanges: true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign and execute transaction: %w", err)
	}

	if !txnResponse.Effects.Data.IsSuccess() {
		return nil, fmt.Errorf("transaction failed")
	}

	var objId *sui.ObjectId
	for _, change := range txnResponse.ObjectChanges {
		if change.Data.Created != nil {
			resource, err := sui.NewResourceType(change.Data.Created.ObjectType)
			if err != nil {
				return nil, fmt.Errorf("invalid resource string: %w", err)
			}

			if resource.Contains(tb.packageID, "ctf_registry", "ChallengeCommitment") {
				objId = &change.Data.Created.ObjectId
			}
		}
	}

	tb.logger.Info().
		Str("digest", txnResponse.Digest.String()).
		Str("objId", objId.String()).
		Msg("Successfully uploaded challenge commitment to Sui")

	return objId, nil
}

func (tb *TransactionBuilder) VaultAddBounty(
	ctx context.Context,
	vaultId string,
) error {
	tb.logger.Debug().
		Str("vault_id", vaultId).
		Msg("Building vault_add_bounty transaction")

	vaultGetObject, err := tb.client.GetObject(ctx, &suiclient.GetObjectRequest{
		ObjectId: sui.MustAddressFromHex(vaultId),
		Options: &suiclient.SuiObjectDataOptions{
			ShowContent: true,
			ShowBcs:     true,
			ShowOwner:   true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get vault object object: %w", err)
	}

	vaultRef := vaultGetObject.Data.RefSharedObject()

	coinPage, err := tb.client.GetCoins(ctx, &suiclient.GetCoinsRequest{Owner: tb.signer.Address})
	if err != nil {
		return fmt.Errorf("failed to get coins: %w", err)
	}
	coins := coinPage.Data

	ptb := suiptb.NewTransactionDataTransactionBuilder()

	ptb.Command(suiptb.Command{
		MoveCall: &suiptb.ProgrammableMoveCall{
			Package:       tb.packageID,
			Module:        "ctf_registry",
			Function:      "vault_add_bounty",
			TypeArguments: []sui.TypeTag{},
			Arguments: []suiptb.Argument{
				ptb.MustObj(suiptb.ObjectArg{SharedObject: &suiptb.SharedObjectArg{
					Id:                   vaultRef.ObjectId,
					InitialSharedVersion: vaultRef.Version,
					Mutable:              true,
				}}),
				ptb.MustObj(suiptb.ObjectArg{ImmOrOwnedObject: coins[0].Ref()}),
			},
		},
	})

	pt := ptb.Finish()

	tx := suiptb.NewTransactionData(
		tb.signer.Address,
		pt,
		[]*sui.ObjectRef{coinPage.Data[1].Ref()},
		suiclient.DefaultGasBudget,
		suiclient.DefaultGasPrice,
	)

	txBytes, err := bcs.Marshal(tx)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	txnResponse, err := tb.client.SignAndExecuteTransaction(
		ctx,
		tb.signer,
		txBytes,
		&suiclient.SuiTransactionBlockResponseOptions{
			ShowEffects:       true,
			ShowObjectChanges: true,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to sign and execute transaction: %w", err)
	}

	if !txnResponse.Effects.Data.IsSuccess() {
		return fmt.Errorf("transaction failed")
	}

	tb.logger.Info().
		Str("digest", txnResponse.Digest.String()).
		Msg("Successfully uploaded challenge commitment to Sui")

	return nil
}

func (tb *TransactionBuilder) VaultTransferBounty(
	ctx context.Context,
	vaultId string,
	vaultAdminCapId string,
	solverAddr string,
) error {
	tb.logger.Debug().
		Str("vault_id", vaultId).
		Str("vault_admin_cap_id", vaultAdminCapId).
		Msg("Building vault_transfer_bounty transaction")

	vaultGetObject, err := tb.client.GetObject(ctx, &suiclient.GetObjectRequest{
		ObjectId: sui.MustAddressFromHex(vaultId),
		Options: &suiclient.SuiObjectDataOptions{
			ShowContent: true,
			ShowBcs:     true,
			ShowOwner:   true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get vault object object: %w", err)
	}
	vaultRef := vaultGetObject.Data.RefSharedObject()

	vaultAdminCapGetObject, err := tb.client.GetObject(ctx, &suiclient.GetObjectRequest{
		ObjectId: sui.MustAddressFromHex(vaultAdminCapId),
		Options: &suiclient.SuiObjectDataOptions{
			ShowContent: true,
			ShowBcs:     true,
			ShowOwner:   true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get vault object object: %w", err)
	}
	vaultAdminCapRef := vaultAdminCapGetObject.Data.Ref()

	coinPage, err := tb.client.GetCoins(ctx, &suiclient.GetCoinsRequest{Owner: tb.signer.Address})
	if err != nil {
		return fmt.Errorf("failed to get coins: %w", err)
	}
	coins := coinPage.Data

	ptb := suiptb.NewTransactionDataTransactionBuilder()

	bountyArg := ptb.Command(suiptb.Command{
		MoveCall: &suiptb.ProgrammableMoveCall{
			Package:       tb.packageID,
			Module:        "ctf_registry",
			Function:      "vault_transfer_bounty",
			TypeArguments: []sui.TypeTag{},
			Arguments: []suiptb.Argument{
				ptb.MustObj(suiptb.ObjectArg{SharedObject: &suiptb.SharedObjectArg{
					Id:                   vaultRef.ObjectId,
					InitialSharedVersion: vaultRef.Version,
					Mutable:              true,
				}}),
				ptb.MustObj(suiptb.ObjectArg{ImmOrOwnedObject: vaultAdminCapRef}),
			},
		},
	})
	ptb.Command(suiptb.Command{
		TransferObjects: &suiptb.ProgrammableTransferObjects{
			Objects: []suiptb.Argument{bountyArg},
			Address: ptb.MustPure(sui.MustAddressFromHex(solverAddr)),
		},
	})

	pt := ptb.Finish()

	tx := suiptb.NewTransactionData(
		tb.signer.Address,
		pt,
		[]*sui.ObjectRef{coins[0].Ref()},
		suiclient.DefaultGasBudget,
		suiclient.DefaultGasPrice,
	)

	txBytes, err := bcs.Marshal(tx)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	txnResponse, err := tb.client.SignAndExecuteTransaction(
		ctx,
		tb.signer,
		txBytes,
		&suiclient.SuiTransactionBlockResponseOptions{
			ShowEffects:       true,
			ShowObjectChanges: true,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to sign and execute transaction: %w", err)
	}

	if !txnResponse.Effects.Data.IsSuccess() {
		return fmt.Errorf("transaction failed")
	}

	tb.logger.Info().
		Str("digest", txnResponse.Digest.String()).
		Msg("Successfully uploaded challenge commitment to Sui")

	return nil
}
