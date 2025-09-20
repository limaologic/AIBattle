//go:build !grpcbridge

package main

import (
	"context"
	"reverse-challenge-system/internal/solver"
)

// startBridgeIfEnabled is a no-op when the grpcbridge build tag is not set.
// It returns nil to indicate the bridge is not running.
func startBridgeIfEnabled(_ *solver.Service) func(context.Context) error {
	return nil
}
