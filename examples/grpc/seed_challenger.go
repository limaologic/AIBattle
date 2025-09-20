package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"reverse-challenge-system/internal/challenger"
	"reverse-challenge-system/pkg/auth"
	"reverse-challenge-system/pkg/config"
	"reverse-challenge-system/pkg/db"
	"reverse-challenge-system/pkg/logger"
	"reverse-challenge-system/pkg/models"
	"reverse-challenge-system/pkg/sui"
	"reverse-challenge-system/pkg/validator"

	"github.com/pattonkan/sui-go/suiclient/conn"
)

// seed_challenger inserts a single challenge row into challenger.db
// so that the Challenger can accept callbacks for that ID.
//
// Usage examples:
//
//	go run examples/grpc/seed_challenger.go --challenge-id ch_123 --answer "MOCK_ANSWER"
//	go run examples/grpc/seed_challenger.go --challenge-id ch_abc --type text --text "hello world" --answer "HELLO WORLD"
func main() {
	var (
		challengeID string
		ctype       string
		text        string
		answer      string
	)

	flag.StringVar(&challengeID, "challenge-id", "ch_local_e2e", "Challenge ID to seed into challenger.db")
	flag.StringVar(&ctype, "type", "text", "Challenge type (text|math|captcha)")
	flag.StringVar(&text, "text", "demo", "Problem text (for type=text)")
	flag.StringVar(&answer, "answer", "MOCK_ANSWER", "Expected answer (used for ExactMatch validation)")
	flag.Parse()

	// Load configuration (.env) to get DB paths and secrets
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Open challenger DB
	cdb, err := db.NewChallengerDB(cfg.ChallengerDBPath)
	if err != nil {
		log.Fatalf("failed to open challenger db: %v", err)
	}
	defer cdb.Close()

	startupLogger := logger.NewCategoryLogger(cfg.LogLevel, logger.Challenger, logger.Startup)
	var suiTxBuilder *sui.TransactionBuilder
	suiRPCURL := conn.LocalnetEndpointUrl
	suiTxBuilder, err = sui.NewTransactionBuilder(context.Background(), startupLogger, suiRPCURL, cfg.SUI.PackageID, cfg.SUI.ChallengerMnemonic)
	if err != nil {
		startupLogger.Error().Err(err).Msg("Failed to initialize Sui TransactionBuilder, continuing without Sui integration")
	} else {
		startupLogger.Info().
			Str("rpc_url", suiRPCURL).
			Str("package_id", cfg.SUI.PackageID).
			Msg("Sui TransactionBuilder initialized successfully")
	}

	// Initialize HMAC and service (required by CreateChallenge signature path)
	hmacAuth := auth.NewHMACAuth(cfg.GetChallengerSecrets(), cfg.GetClockSkew())
	svc := challenger.NewService(cfg, cdb, hmacAuth, suiTxBuilder)

	// If challenge already exists, skip to keep seeding idempotent
	if existing, _ := cdb.GetChallenge(challengeID); existing != nil {
		fmt.Printf("Challenge %q already exists in %s; skipping.\n", challengeID, cfg.ChallengerDBPath)
		return
	}

	// Minimal problem and output spec
	problem := map[string]any{"type": ctype, "text": text}
	problemJSON, _ := json.Marshal(problem)
	outputSpec := map[string]any{
		"content_type": "text/plain",
		"schema": map[string]any{
			"type": "string",
		},
	}
	outputSpecJSON, _ := json.Marshal(outputSpec)

	// ExactMatch rule with case-insensitive compare by default
	rule := validator.CreateExactMatchRule(answer, false)

	ch := &models.Challenge{
		ID:             challengeID,
		Type:           ctype,
		Problem:        problemJSON,
		OutputSpec:     outputSpecJSON,
		ValidationRule: rule,
		CreatedAt:      time.Now(),
	}

	if err := svc.CreateChallenge(ch); err != nil {
		log.Fatalf("failed to create challenge: %v", err)
	}

	fmt.Printf("Seeded challenge %q into %s (type=%s, expected=%q)\n", challengeID, cfg.ChallengerDBPath, ctype, answer)
}
