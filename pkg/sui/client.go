package sui

import (
	"context"
	"fmt"

	"github.com/pattonkan/sui-go/sui"
	"github.com/pattonkan/sui-go/suiclient"
	"github.com/rs/zerolog"
)

// GetTransactionBlock fetches transaction details by digest using the official Sui client
func GetObject(ctx context.Context, rpcURL string, objIdRaw string, logger zerolog.Logger) (*suiclient.SuiObjectResponse, error) {
	if objIdRaw == "" {
		return nil, fmt.Errorf("objId cannot be empty")
	}

	if rpcURL == "" {
		return nil, fmt.Errorf("rpcURL cannot be empty")
	}

	logger.Debug().
		Str("objId", objIdRaw).
		Str("rpc_url", rpcURL).
		Msg("Fetching transaction from Sui")

	// Create Sui client
	client := suiclient.NewClient(rpcURL)

	// Convert string objId to proper TransactionDigest type
	objId := sui.MustObjectIdFromHex(objIdRaw)

	// Fetch transaction using the official client
	return client.GetObject(
		ctx,
		&suiclient.GetObjectRequest{
			ObjectId: objId,
			Options: &suiclient.SuiObjectDataOptions{
				ShowType:                true,
				ShowContent:             true,
				ShowBcs:                 true,
				ShowOwner:               true,
				ShowPreviousTransaction: true,
				ShowStorageRebate:       true,
				ShowDisplay:             true,
			},
		},
	)
}
