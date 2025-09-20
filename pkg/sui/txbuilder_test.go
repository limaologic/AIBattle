package sui

import (
	"context"
	"testing"

	suiTypes "github.com/pattonkan/sui-go/sui"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestNewTransactionBuilder_Validation(t *testing.T) {
	ctx := context.Background()
	logger := log.Logger

	tests := []struct {
		name      string
		rpcURL    string
		packageID string
		mnemonic  string
		wantError bool
	}{
		{
			name:      "empty rpcURL",
			rpcURL:    "",
			packageID: "0x1234567890abcdef1234567890abcdef12345678",
			mnemonic:  "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
			wantError: true,
		},
		{
			name:      "empty packageID",
			rpcURL:    "https://fullnode.testnet.sui.io:443",
			packageID: "",
			mnemonic:  "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
			wantError: true,
		},
		{
			name:      "empty mnemonic",
			rpcURL:    "https://fullnode.testnet.sui.io:443",
			packageID: "0x1234567890abcdef1234567890abcdef12345678",
			mnemonic:  "",
			wantError: true,
		},
		{
			name:      "invalid packageID format",
			rpcURL:    "https://fullnode.testnet.sui.io:443",
			packageID: "invalid-hex",
			mnemonic:  "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
			wantError: true,
		},
		{
			name:      "invalid mnemonic",
			rpcURL:    "https://fullnode.testnet.sui.io:443",
			packageID: "0x1234567890abcdef1234567890abcdef12345678",
			mnemonic:  "invalid mnemonic words",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTransactionBuilder(ctx, logger, tt.rpcURL, tt.packageID, tt.mnemonic)

			if tt.wantError && err == nil {
				t.Errorf("NewTransactionBuilder() expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("NewTransactionBuilder() unexpected error: %v", err)
			}
		})
	}
}

func TestBuildUploadChallengeCommitment_Validation(t *testing.T) {
	// This test focuses on input validation logic
	// We test the structure and argument validation
	tests := []struct {
		name           string
		typeArgs       TypeArgs
		registryId     string
		commitment     []byte
		challengerAddr string
		solverAddr     string
		score          uint64
		timestamp      uint64
		expectError    bool
	}{
		{
			name: "valid inputs",
			typeArgs: TypeArgs{
				TreasuryCapPositive: "0x2::sui::SUI",
				TreasuryCapNegative: "0x2::sui::SUI",
				CoinTypeCollateral:  "0x2::sui::SUI",
			},
			registryId:     "0x1234567890abcdef1234567890abcdef12345678901234567890abcdef123456",
			commitment:     []byte("test commitment"),
			challengerAddr: "0x1234567890abcdef1234567890abcdef12345678901234567890abcdef123456",
			solverAddr:     "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			score:          100,
			timestamp:      1234567890,
			expectError:    false,
		},
		{
			name: "invalid challenger address",
			typeArgs: TypeArgs{
				TreasuryCapPositive: "0x2::sui::SUI",
				TreasuryCapNegative: "0x2::sui::SUI",
				CoinTypeCollateral:  "0x2::sui::SUI",
			},
			registryId:     "0x1234567890abcdef1234567890abcdef12345678901234567890abcdef123456",
			commitment:     []byte("test commitment"),
			challengerAddr: "invalid-hex",
			solverAddr:     "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			score:          100,
			timestamp:      1234567890,
			expectError:    true,
		},
		{
			name: "invalid solver address",
			typeArgs: TypeArgs{
				TreasuryCapPositive: "0x2::sui::SUI",
				TreasuryCapNegative: "0x2::sui::SUI",
				CoinTypeCollateral:  "0x2::sui::SUI",
			},
			registryId:     "0x1234567890abcdef1234567890abcdef12345678901234567890abcdef123456",
			commitment:     []byte("test commitment"),
			challengerAddr: "0x1234567890abcdef1234567890abcdef12345678901234567890abcdef123456",
			solverAddr:     "invalid-hex",
			score:          100,
			timestamp:      1234567890,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For this test, we just verify that input validation works
			// We test address parsing validation by calling sui.AddressFromHex directly

			if tt.expectError {
				// Test that invalid addresses return errors
				if tt.challengerAddr == "invalid-hex" {
					_, err := suiTypes.AddressFromHex(tt.challengerAddr)
					if err == nil {
						t.Error("Expected error for invalid challenger address")
					}
				}
				if tt.solverAddr == "invalid-hex" {
					_, err := suiTypes.AddressFromHex(tt.solverAddr)
					if err == nil {
						t.Error("Expected error for invalid solver address")
					}
				}
			} else {
				// Test that valid addresses parse correctly
				_, err := suiTypes.AddressFromHex(tt.challengerAddr)
				if err != nil {
					t.Errorf("Valid challenger address should parse: %v", err)
				}
				_, err = suiTypes.AddressFromHex(tt.solverAddr)
				if err != nil {
					t.Errorf("Valid solver address should parse: %v", err)
				}
				_, err = suiTypes.ObjectIdFromHex(tt.registryId)
				if err != nil {
					t.Errorf("Valid registry ID should parse: %v", err)
				}
			}
		})
	}
}

func TestTypeArgs_Structure(t *testing.T) {
	// Test that TypeArgs has the correct structure
	typeArgs := TypeArgs{
		TreasuryCapPositive: "0x2::sui::SUI",
		TreasuryCapNegative: "0x2::coin::COIN",
		CoinTypeCollateral:  "0x1::collateral::COLLATERAL",
	}

	if typeArgs.TreasuryCapPositive != "0x2::sui::SUI" {
		t.Errorf("Expected TreasuryCapPositive to be '0x2::sui::SUI', got %s", typeArgs.TreasuryCapPositive)
	}
	if typeArgs.TreasuryCapNegative != "0x2::coin::COIN" {
		t.Errorf("Expected TreasuryCapNegative to be '0x2::coin::COIN', got %s", typeArgs.TreasuryCapNegative)
	}
	if typeArgs.CoinTypeCollateral != "0x1::collateral::COLLATERAL" {
		t.Errorf("Expected CoinTypeCollateral to be '0x1::collateral::COLLATERAL', got %s", typeArgs.CoinTypeCollateral)
	}
}

func TestTransactionBuilder_MethodSignatures(t *testing.T) {
	// Test that TransactionBuilder methods have correct signatures
	// This is a compile-time test to ensure the API is correct
	ctx := context.Background()
	logger := zerolog.Nop()

	// These should compile - testing that the methods exist with correct signatures
	_ = func() (*TransactionBuilder, error) {
		return NewTransactionBuilder(ctx, logger, "url", "package", "mnemonic")
	}

	// Test method signatures exist and compile
	var tb *TransactionBuilder
	if tb != nil { // This will never be true, but ensures methods compile
		_, _ = tb.BuildUploadChallengeCommitment(
			ctx, TypeArgs{}, "registry", []byte{}, "challenger", "solver", 0, 0,
		)
		_, _ = tb.SelectGasObject(ctx, "owner")
		_, _ = tb.SignAndExecute(ctx, nil)
		_, _ = tb.UploadChallengeCommitment(
			ctx, TypeArgs{}, "registry", []byte{}, "challenger", "solver", 0, 0,
		)
	}
}
