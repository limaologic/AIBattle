# gRPC Bridge Example (External Python → Solver)

See also
- Root README → "Data Storage: Challenger DB vs Solver DB" for full table schemas and rationale.

This example shows how an external Python process (e.g., an LLM/ML worker) can send an answer to the Solver via gRPC. The Python example mocks the OpenAI call and submits a static answer.

## Overview
- Proto: `proto/solver_bridge.proto`
- Solver gRPC server (optional, behind build tag): `internal/solver/grpc_bridge_server.go`
- Python client: `examples/grpc/client.py`

## Generate Stubs
- Verify tooling first (recommended):
  - `make proto-verify` (checks protoc, Go plugins, Python grpc tooling)

- Generate Go stubs (for the bridge server):
  - `make proto-gen-go`

- Generate Python stubs (for the example client):
  - `make proto-gen-py`

## Run Solver with gRPC Bridge
- The bridge runs only when built with the `grpcbridge` build tag.
- Steps:
  - Add gRPC deps (optional convenience): `make grpc-deps`
  - Build solver with bridge: `make build-solver-grpc`
  - Start services:
    - Terminal A: `go run cmd/challenger/main.go`
    - Terminal B: `SOLVER_GRPC_BRIDGE_ADDR=:9090 ./bin/solver`
  - Note: You can omit `SOLVER_GRPC_BRIDGE_ADDR` to use default `:9090`.

## Local E2E (seed both DBs, then gRPC)

This flow seeds a pending challenge into `solver.db` and a matching challenge into `challenger.db`, then submits an answer via gRPC; the Solver forwards a signed HTTP/HMAC callback to the Challenger.

1) Pre-checks
- Ensure `.env` has consistent HMAC config (simplest): `SHARED_SECRET_KEY=dev-shared-secret`

2) Start services
- Terminal A: `go run cmd/challenger/main.go`
- Terminal B: `SOLVER_WORKER_COUNT=0 SOLVER_GRPC_BRIDGE_ADDR=:9090 ./bin/solver`
  - `SOLVER_WORKER_COUNT=0` prevents internal workers from racing the gRPC result.

3) Seed both databases with the same challenge ID (e.g., `ch_123`)
- Seed solver DB with a callback URL pointing at the Challenger:
  - `go run examples/grpc/seed_pending.go --challenge-id ch_123 --callback-url http://127.0.0.1:8080/callback/ch_123`
- Seed challenger DB with validation rule expecting the same answer you will submit:
  - `go run examples/grpc/seed_challenger.go --challenge-id ch_123 --answer "MOCK_ANSWER"`

4) Submit the answer via Python gRPC client
- `python3 examples/grpc/client.py --challenge-id ch_123 --job-id solver_job_ch_123 --answer "MOCK_ANSWER" --target localhost:9090`

5) Expected output
- `{'accepted': True, 'message': 'callback accepted'}`
- Challenger logs show callback processed and result stored; Solver removes the pending challenge.

## Identifiers: `challenge-id` vs `job-id`

This example uses two identifiers that serve different purposes.

- `challenge-id`
  - What it is: A stable, unique ID that represents the problem to solve.
  - Who creates it: The Challenger (or our seed helper for local tests).
  - Where it lives:
    - Challenger DB → table `challenges.id`
    - Solver DB → table `pending_challenges.id` (while pending)
  - How it’s used:
    - In the callback path: `http://<challenger>/callback/{challenge-id}`
    - As the lookup key for the Challenger to validate and store results.
  - Why it matters: If the Challenger lacks this ID, the callback returns 404.

- `job-id` (forwarded as `solver_job_id`)
  - What it is: A work-tracking ID for the solving attempt (observability/trace).
  - Who creates it: Solver when accepting via HTTP `/solve` (or you provide it in this gRPC example).
  - Where it shows up:
    - Returned by SolveResponse in the HTTP flow
    - Included in the callback body to the Challenger
  - How the Challenger uses it: For logs/audits only. Routing still uses `challenge-id`.

Example command breakdown

```
python3 examples/grpc/client.py \
  --challenge-id ch_dbg_1 \
  --job-id solver_job_ch_dbg_1 \
  --answer "MOCK_ANSWER" \
  --target localhost:9090
```

- `--challenge-id ch_dbg_1`: Must exist in both DBs locally (Challenger: `challenges`, Solver: `pending_challenges`). It determines the callback URL and what the Challenger loads.
- `--job-id solver_job_ch_dbg_1`: A label for this attempt, forwarded as `solver_job_id` for traceability.
- `--answer "MOCK_ANSWER"`: The returned content; the Challenger validates it per the stored rule (e.g., ExactMatch).
- `--target localhost:9090`: gRPC bridge endpoint; the bridge will perform the signed HTTP callback to the Challenger.

## Data Storage: Challenger DB vs Solver DB (Quick Reference)

This example interacts with two SQLite databases managed by the services. Below is a concise reference to what each stores and why it matters for the gRPC flow.

### Challenger DB (authoritative truth and validation)

- `challenges`
  - Schema:
    - `id TEXT PRIMARY KEY`
    - `type TEXT NOT NULL`
    - `problem TEXT NOT NULL` (JSON string)
    - `output_spec TEXT NOT NULL` (JSON string)
    - `validation_rule TEXT NOT NULL` (JSON string)
    - `created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`
  - Purpose: Defines the problem and how to validate answers. Must contain the `challenge-id` or callbacks will 404.

- `results`
  - Schema (key columns):
    - `challenge_id TEXT NOT NULL`
    - `request_id TEXT NOT NULL` (idempotency key via `UNIQUE (challenge_id, request_id)`)
    - `solver_job_id TEXT`, `status TEXT NOT NULL`, `received_answer TEXT`, `is_correct BOOLEAN`
    - `compute_time_ms INTEGER`, `solver_metadata TEXT`, `created_at TIMESTAMP`
  - Purpose: Durable history of callbacks and validation outcomes. Prevents duplicate inserts on retries.

- `webhooks`
  - Schema (key columns):
    - `challenge_id TEXT NOT NULL`, `request_id TEXT NOT NULL`
    - `headers TEXT`, `body_hash TEXT`, `status_code INTEGER`, `created_at TIMESTAMP`
  - Purpose: Minimal audit trail of incoming callbacks for debugging/security.

- `seen_nonces`
  - Schema:
    - `nonce TEXT PRIMARY KEY`, `seen_at TIMESTAMP`
  - Purpose: HMAC replay protection for Challenger’s authenticated endpoints.

### Solver DB (operational queue and delivery state)

- `pending_challenges`
  - Schema:
    - `id TEXT PRIMARY KEY`
    - `problem TEXT NOT NULL`, `output_spec TEXT NOT NULL` (JSON strings)
    - `callback_url TEXT NOT NULL`
    - `received_at TIMESTAMP`, `status TEXT`, `attempt_count INTEGER`, `next_retry_time TIMESTAMP`
  - Purpose: Work queue item for the Solver. The gRPC bridge reads `callback_url` here and forwards a signed HTTP callback to the Challenger.

- `seen_nonces`
  - Same purpose as on the Challenger, but for the Solver’s inbound authenticated endpoints.

Why two DBs?
- Ownership separation: Challenger owns truth/validation; Solver owns execution state and retries.
- Failure isolation and simpler scaling per service.
- Validation rules never leave the Challenger.

For the full, deeply detailed explanation (table-by-table schemas and rationale), see the root README section:
- README.md → "Data Storage: Challenger DB vs Solver DB"

Troubleshooting
- If you see `callback status 404`, make sure the challenge was seeded into `challenger.db` for the same ID.
- If you see auth errors, confirm both services loaded the same `.env` and the HMAC key(s) match.
