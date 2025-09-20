package main

import (
	"context"
	"encoding/json"
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

	"github.com/google/uuid"
	"github.com/pattonkan/sui-go/suiclient/conn"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	database, err := db.NewChallengerDB(cfg.ChallengerDBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

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

	// Initialize HMAC authentication
	secrets := cfg.GetChallengerSecrets()
	hmacAuth := auth.NewHMACAuth(secrets, cfg.GetClockSkew())

	// Initialize service
	service := challenger.NewService(cfg, database, hmacAuth, suiTxBuilder)

	// Create example challenges
	challenges := []struct {
		name      string
		challenge *models.Challenge
	}{
		// Temporarily disabled - CAPTCHA challenges always fail due to random mock answers
		// {
		// 	"CAPTCHA Challenge",
		// 	createCAPTCHAChallenge(),
		// },
		{
			"Math Challenge",
			createMathChallenge(),
		},
		// Temporarily disabled - focusing on math challenges only for now
		// {
		// 	"Text Challenge",
		// 	createTextChallenge(),
		// },
	}

	// Create and send each challenge
	for _, example := range challenges {
		fmt.Printf("Creating %s...\n", example.name)

		// Save challenge to database
		if err := service.CreateChallenge(example.challenge); err != nil {
			log.Printf("Failed to create challenge %s: %v", example.name, err)
			continue
		}

		fmt.Printf("Challenge ID: %s\n", example.challenge.ID)

		// Send challenge to solver (adjust URL as needed)
		solverURL := fmt.Sprintf("http://localhost:%s", cfg.SolverPort)
		if err := service.SendChallenge(example.challenge.ID, solverURL); err != nil {
			log.Printf("Failed to send challenge %s: %v", example.name, err)
		} else {
			fmt.Printf("Challenge sent successfully!\n")
		}

		fmt.Printf("---\n")

		// Small delay between challenges
		time.Sleep(3 * time.Second)
	}

	fmt.Println("All challenges created and sent.")
	fmt.Printf("Check the databases at:\n")
	fmt.Printf("- Challenger DB: %s\n", cfg.ChallengerDBPath)
	fmt.Printf("- Solver DB: %s\n", cfg.SolverDBPath)
}

func createCAPTCHAChallenge() *models.Challenge {
	// Create problem data
	problem := map[string]interface{}{
		"type":        "captcha",
		"format":      "base64",
		"data":        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==", // 1x1 pixel PNG
		"description": "Solve this CAPTCHA",
	}
	problemJSON, _ := json.Marshal(problem)

	// Create output spec
	outputSpec := map[string]interface{}{
		"content_type": "text/plain",
		"schema": map[string]interface{}{
			"type":      "string",
			"minLength": 1,
			"maxLength": 10,
		},
	}
	outputSpecJSON, _ := json.Marshal(outputSpec)

	// Create validation rule (exact match, case insensitive)
	validationRule := validator.CreateExactMatchRule("abc123", false)

	return &models.Challenge{
		ID:             fmt.Sprintf("ch_captcha_%s", uuid.New().String()[:8]),
		Type:           "captcha",
		Problem:        problemJSON,
		OutputSpec:     outputSpecJSON,
		ValidationRule: validationRule,
		CreatedAt:      time.Now(),
	}
}

func createMathChallenge() *models.Challenge {
	// Create problem data
	problem := map[string]interface{}{
		"type":        "math",
		"operation":   "add",
		"a":           15.5,
		"b":           24.3,
		"description": "Calculate the result of the given mathematical operation",
	}
	problemJSON, _ := json.Marshal(problem)

	// Create output spec
	outputSpec := map[string]interface{}{
		"content_type": "text/plain",
		"schema": map[string]interface{}{
			"type":    "string",
			"pattern": "^[0-9]+(\\.[0-9]+)?$",
		},
	}
	outputSpecJSON, _ := json.Marshal(outputSpec)

	// Create validation rule (numeric tolerance)
	validationRule := validator.CreateNumericToleranceRule("39.80", 0.01)

	return &models.Challenge{
		ID:             fmt.Sprintf("ch_math_%s", uuid.New().String()[:8]),
		Type:           "math",
		Problem:        problemJSON,
		OutputSpec:     outputSpecJSON,
		ValidationRule: validationRule,
		CreatedAt:      time.Now(),
	}
}

func createTextChallenge() *models.Challenge {
	// Create problem data
	problem := map[string]interface{}{
		"type":        "text",
		"text":        "hello world",
		"description": "Convert the given text to uppercase",
	}
	problemJSON, _ := json.Marshal(problem)

	// Create output spec
	outputSpec := map[string]interface{}{
		"content_type": "text/plain",
		"schema": map[string]interface{}{
			"type": "string",
		},
	}
	outputSpecJSON, _ := json.Marshal(outputSpec)

	// Create validation rule (exact match, case sensitive)
	validationRule := validator.CreateExactMatchRule("HELLO WORLD", true)

	return &models.Challenge{
		ID:             fmt.Sprintf("ch_text_%s", uuid.New().String()[:8]),
		Type:           "text",
		Problem:        problemJSON,
		OutputSpec:     outputSpecJSON,
		ValidationRule: validationRule,
		CreatedAt:      time.Now(),
	}
}
