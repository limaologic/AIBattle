//go:build grpcbridge

package main

import (
	"context"
	"os"

	gbridge "reverse-challenge-system/internal/solver"

	"github.com/rs/zerolog/log"
)

// startBridgeIfEnabled starts the gRPC bridge when built with -tags=grpcbridge.
// Address is controlled via SOLVER_GRPC_BRIDGE_ADDR (default ":9090").
func startBridgeIfEnabled(svc *gbridge.Service) func(context.Context) error {
	addr := os.Getenv("SOLVER_GRPC_BRIDGE_ADDR")
	if addr == "" {
		addr = ":9090"
	}
	stop, err := gbridge.StartGRPCBridge(svc, addr)
	if err != nil {
		log.Error().Err(err).Str("addr", addr).Msg("Failed to start gRPC bridge")
		return nil
	}
	log.Info().Str("addr", addr).Msg("gRPC bridge started")
	return stop
}
