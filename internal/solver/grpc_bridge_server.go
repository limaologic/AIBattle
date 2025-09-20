//go:build grpcbridge

package solver

import (
	"context"
	"fmt"
	"net"

	"reverse-challenge-system/pkg/models"
	solverbridge "reverse-challenge-system/proto/solverbridge"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

// grpcServer implements the SolverBridge gRPC service.
type grpcServer struct {
	solverbridge.UnimplementedSolverBridgeServer
	svc *Service
}

func (g *grpcServer) SubmitAnswer(ctx context.Context, req *solverbridge.SubmitAnswerRequest) (*solverbridge.SubmitAnswerResponse, error) {
	if req.GetChallengeId() == "" {
		return &solverbridge.SubmitAnswerResponse{Accepted: false, Message: "missing challenge_id"}, nil
	}

	// Fetch pending challenge to obtain callback URL
	ch, err := g.svc.db.GetChallenge(req.GetChallengeId())
	if err != nil {
		log.Error().Err(err).Str("challenge_id", req.GetChallengeId()).Msg("gRPC: failed to load pending challenge")
		return &solverbridge.SubmitAnswerResponse{Accepted: false, Message: "database error"}, nil
	}
	if ch == nil {
		return &solverbridge.SubmitAnswerResponse{Accepted: false, Message: "challenge not found"}, nil
	}

	status := req.GetStatus()
	if status == "" {
		status = "success"
	}

	cb := &models.CallbackRequest{
		APIVersion:   "v2.1",
		ChallengeID:  req.GetChallengeId(),
		SolverJobID:  req.GetSolverJobId(),
		Status:       status,
		Answer:       req.GetAnswer(),
		ErrorCode:    req.GetErrorCode(),
		ErrorMessage: req.GetErrorMessage(),
	}

	// Send callback to Challenger using existing HTTP/HMAC path
	statusCode, sendErr := g.svc.SendCallback(ch.CallbackURL, cb)
	if sendErr != nil {
		log.Error().Err(sendErr).Str("challenge_id", req.GetChallengeId()).Msg("gRPC: callback send failed")
		return &solverbridge.SubmitAnswerResponse{Accepted: false, Message: fmt.Sprintf("callback send failed: %v", sendErr)}, nil
	}

	if statusCode >= 200 && statusCode < 300 {
		// On success, remove the pending challenge
		_ = g.svc.db.DeleteChallenge(req.GetChallengeId())
		return &solverbridge.SubmitAnswerResponse{Accepted: true, Message: "callback accepted"}, nil
	}

	return &solverbridge.SubmitAnswerResponse{Accepted: false, Message: fmt.Sprintf("callback status %d", statusCode)}, nil
}

// StartGRPCBridge starts a gRPC server for external solvers to submit answers.
// Example addr: ":9090". Returns a shutdown function.
func StartGRPCBridge(s *Service, addr string) (func(context.Context) error, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	srv := grpc.NewServer()
	solverbridge.RegisterSolverBridgeServer(srv, &grpcServer{svc: s})

	go func() {
		log.Info().Str("addr", addr).Msg("Solver gRPC bridge listening")
		if err := srv.Serve(lis); err != nil {
			log.Error().Err(err).Msg("gRPC server stopped")
		}
	}()

	stop := func(ctx context.Context) error {
		done := make(chan struct{})
		go func() {
			srv.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			srv.Stop()
			return ctx.Err()
		}
	}
	return stop, nil
}
