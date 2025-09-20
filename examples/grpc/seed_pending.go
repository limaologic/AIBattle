package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"reverse-challenge-system/pkg/db"
	"reverse-challenge-system/pkg/models"
)

func main() {
	var (
		challengeID string
		callbackURL string
		solverDB    string
		problemType string
		text        string
	)

	flag.StringVar(&challengeID, "challenge-id", "ch_local_e2e", "Challenge ID to seed")
	flag.StringVar(&callbackURL, "callback-url", "", "Callback URL (default http://127.0.0.1:8080/callback/{challenge-id})")
	flag.StringVar(&solverDB, "db", "", "Path to solver.db (default from SOLVER_DB_PATH or ./solver.db)")
	flag.StringVar(&problemType, "type", "text", "Problem type: text|math|captcha (mock)")
	flag.StringVar(&text, "text", "hello world", "Text content for text problem")
	flag.Parse()

	if solverDB == "" {
		solverDB = os.Getenv("SOLVER_DB_PATH")
		if solverDB == "" {
			solverDB = "solver.db"
		}
	}

	if callbackURL == "" {
		callbackURL = fmt.Sprintf("http://127.0.0.1:8080/callback/%s", challengeID)
	}

	database, err := db.NewSolverDB(solverDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open solver db: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// If challenge already exists, skip to keep seeding idempotent
	if existing, _ := database.GetChallenge(challengeID); existing != nil {
		fmt.Printf("Pending challenge %q already exists in %s; skipping.\n", challengeID, solverDB)
		return
	}

	// Minimal mock problem according to worker's expectations
	problemObj := map[string]any{"type": problemType, "text": text}
	problemJSON, _ := json.Marshal(problemObj)
	outputSpec := json.RawMessage(`{"type":"string"}`)

	challenge := &models.PendingChallenge{
		ID:            challengeID,
		Problem:       problemJSON,
		OutputSpec:    outputSpec,
		CallbackURL:   callbackURL,
		ReceivedAt:    time.Now(),
		Status:        "pending",
		AttemptCount:  0,
		NextRetryTime: time.Now(),
	}

	if err := database.SaveChallenge(challenge); err != nil {
		fmt.Fprintf(os.Stderr, "failed to save pending challenge: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Seeded pending challenge %q with callback %q into %s\n", challengeID, callbackURL, solverDB)
}
