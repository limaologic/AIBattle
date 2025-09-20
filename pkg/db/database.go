package db

import (
	"context"
	"reverse-challenge-system/pkg/models"
)

// Database interface for contract operations
type Database interface {
	SaveContract(ctx context.Context, contract *models.Contract) error
	GetContractByName(ctx context.Context, name, chainID string) (*models.Contract, error)
	ListContracts(ctx context.Context, chainID string) ([]*models.Contract, error)
	Close() error
}

// NewDatabase creates a new database instance based on the provided path
// Currently uses ChallengerDB as the default implementation
func NewDatabase(dbPath string) (Database, error) {
	return NewChallengerDB(dbPath)
}

// Ensure ChallengerDB implements Database interface
var _ Database = (*ChallengerDB)(nil)
