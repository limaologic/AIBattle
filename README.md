# Reverse Challenge System (MVP)

A Go-based system that reverses the traditional challenge-solving model. Instead of solvers uploading their code to a centralized platform, **Challengers push problems to Solvers**, who run them on their own computing resources and send back results.

## 🎯 Core Features

- **Answer Security**: Validation rules and correct answers never leave the Challenger's environment
- **Decentralized Compute**: Solvers use their own hardware (GPU, TPU, specialized equipment)  
- **Technical Freedom**: Solvers can use any language/framework/hardware stack
- **Robust Authentication**: HMAC-SHA256 signatures with nonce-based replay protection
- **Retry Logic**: Exponential backoff with jitter for reliable callback delivery
- **Idempotency**: Duplicate-safe operations using X-Request-ID headers

## 🏗️ Architecture

```
┌─────────────────┐                    ┌─────────────────┐
│   CHALLENGER    │────── HMAC ────────│     SOLVER      │
│                 │   signed requests   │                 │
│ • Create tasks  │                     │ • Process async │
│ • Validate ans. │◄──── callbacks ────│ • Worker pool   │
│ • Store results │                     │ • Retry logic   │
└─────────────────┘                    └─────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- SQLite3
- ngrok (optional, for public callback URLs when USE_NGROK=true)

### 1. Clone and Setup

```bash
git clone <repository>
cd reverse-challenge-system
go mod tidy
```

### 2. Configuration

```bash
cp .env.example .env
# Edit .env with your settings
```

Key configuration values:
- `USE_NGROK`: Set to `true` for production/external testing, `false` for local development (default: false)
- `PUBLIC_CALLBACK_HOST`: Your public callback URL (auto-set to localhost when USE_NGROK=false)
- `SHARED_SECRET_KEY`: HMAC signing key (same for both services in MVP)
- `SOLVER_WORKER_COUNT`: Number of concurrent workers

### 3. Local Development (Default)

For local testing, no additional setup needed:

```bash
# .env already configured with USE_NGROK=false and localhost callback
make run-challenger  # Terminal 1
make run-solver      # Terminal 2 
make example         # Terminal 3 - sends test challenge
```

### 3a. Production/External Testing (Optional)

Only if you need external access, set `USE_NGROK=true` and start ngrok:

```bash
# Install ngrok: https://ngrok.com/
ngrok http 8080
# Copy the https://xxx.ngrok.io URL to PUBLIC_CALLBACK_HOST in .env
# Set USE_NGROK=true in .env
```

### 4. Run Services (if not using make commands)

Terminal 1 - Challenger:
```bash
go run cmd/challenger/main.go
# Starts on :8080
```

Terminal 2 - Solver:  
```bash
# Without gRPC bridge:
go run cmd/solver/main.go cmd/solver/grpc_bridge_disabled.go
# OR use the makefile:
make run-solver

# With gRPC bridge:
go run -tags=grpcbridge cmd/solver/main.go
# OR build first: make build-solver-grpc && ./bin/solver
```

### 5. Send Test Challenge

```bash
go run examples/send_challenge.go
```

This creates and sends sample challenges (CAPTCHA, Math, Text processing).

## 📡 API Overview

### Challenger → Solver: Send Challenge

```http
POST https://solver.example.com/solve
Authorization: RCS-HMAC-SHA256 keyId=solver-kid-1,ts=1735200000,nonce=uuid,sig=hex
Content-Type: application/json

{
  "api_version": "v2.1",
  "challenge_id": "ch_20241226_001", 
  "problem": {
    "type": "captcha",
    "data": "base64-encoded-image"
  },
  "output_spec": {
    "content_type": "text/plain",
    "schema": {"type": "string"}
  },
  "constraints": {
    "timeout_ms": 30000
  },
  "callback_url": "https://challenger.ngrok.io/callback/ch_20241226_001"
}
```

### Solver → Challenger: Return Result

```http  
POST https://challenger.ngrok.io/callback/ch_20241226_001
Authorization: RCS-HMAC-SHA256 keyId=chal-kid-1,ts=1735200001,nonce=uuid,sig=hex
Content-Type: application/json

{
  "api_version": "v2.1",
  "challenge_id": "ch_20241226_001",
  "solver_job_id": "solver_job_abc123",
  "status": "success",
  "answer": "identified_text",
  "metadata": {
    "compute_time_ms": 1500,
    "algorithm": "cnn_v2", 
    "confidence": 0.95
  }
}
```

## 🔐 Security Features

### HMAC Authentication
- **Algorithm**: HMAC-SHA256
- **Canonical String**: `METHOD\nPATH\nTIMESTAMP\nNONCE\nSHA256(body)`
- **Time Window**: ±300 seconds (configurable)
- **Nonce Protection**: Single-use UUIDs prevent replay attacks

### Request Validation
- Content-Length ≤ 5MB (configurable)
- HTTPS-only callbacks
- Host whitelist support
- Signature constant-time comparison

## 🛠️ Implementation Details

### Challenge Types (MVP Examples)

1. **CAPTCHA**: Image → text extraction
2. **Math**: Numerical computations with tolerance validation  
3. **Text**: String processing operations

### Validation Rules

- **ExactMatch**: String equality (case-sensitive/insensitive)
- **NumericTolerance**: Float comparison within epsilon
- **Regex**: Pattern matching validation

### Retry Strategy

- **Base Delay**: 500ms
- **Max Delay**: 30s  
- **Backoff**: Exponential with jitter (0.85-1.15x)
- **Max Attempts**: 6
- **Retry On**: 429, 5xx, network errors
- **No Retry**: 4xx (except 429)

## 📊 Monitoring & Health

### Health Endpoints

- `GET /healthz` - Service liveness
- `GET /readyz` - Database connectivity
- `GET /stats` (Solver only) - Worker pool status

### Database Schema

**Challenger Tables**:
- `challenges` - Problem definitions (with answers)
- `results` - Solver responses and validation results
- `webhooks` - Audit trail of callbacks
- `seen_nonces` - Replay attack prevention

**Solver Tables**:
- `pending_challenges` - Work queue with retry state
- `seen_nonces` - Replay attack prevention

## 🧪 Testing

### Unit Tests
```bash
go test ./pkg/auth -v
go test ./pkg/validator -v
```

### Integration Testing
```bash
# Start both services, then:
go run examples/send_challenge.go

# Check databases:
sqlite3 challenger.db "SELECT * FROM results;"
sqlite3 solver.db "SELECT * FROM pending_challenges;"
```

## 📁 Project Structure

```
├── cmd/
│   ├── challenger/main.go    # Challenger service entry point
│   └── solver/main.go        # Solver service entry point
├── internal/
│   ├── challenger/service.go # Challenge creation & callback handling
│   └── solver/
│       ├── service.go        # HTTP handlers & callback logic
│       └── worker.go         # Worker pool & retry logic
├── pkg/
│   ├── api/middleware.go     # HMAC auth, logging, CORS
│   ├── auth/hmac.go         # HMAC signature implementation  
│   ├── config/config.go     # Environment configuration
│   ├── db/                  # Database layers (SQLite)
│   ├── logger/logger.go     # Structured logging (zerolog)
│   ├── models/models.go     # Data structures & API contracts
│   └── validator/           # Answer validation engine
└── examples/
    └── send_challenge.go    # Demo challenge creation
```

## 🔌 gRPC Bridge Quick Start (External Python Solver)

Use a Python process (mocking an LLM) to submit answers to the Solver via gRPC, which then forwards results to the Challenger using existing HMAC/HTTP callbacks.

1) Verify tooling and generate stubs
- `make proto-verify`  # checks protoc, Go plugins, Python grpc tooling
- `make proto-gen-go`  # generate Go stubs into `proto/solverbridge`
- `make proto-gen-py`  # generate Python stubs into `examples/grpc`

2) Add Go gRPC dependencies and build Solver with bridge
- `make grpc-deps`           # adds grpc/protobuf to go.mod (requires network)
- `make build-solver-grpc`   # builds `bin/solver` with `-tags=grpcbridge`

3) Run services
- Terminal A: `go run cmd/challenger/main.go`
- Terminal B: `./bin/solver` (optionally set `SOLVER_GRPC_BRIDGE_ADDR=:9090`)

4) Run Python example client
- `python3 examples/grpc/client.py --challenge-id ch_123 --job-id solver_job_ch_123 --answer "MOCK_ANSWER" --target localhost:9090`

Notes
- The Python client returns a mock LLM answer if no `OPENAI_API_KEY` is set.
- The bridge only runs in builds with the `grpcbridge` tag and does not affect normal builds/tests.

### Identifiers: `challenge-id` vs `job-id`

This repo uses two related identifiers when submitting answers through the gRPC bridge and when delivering HTTP callbacks back to the Challenger. Understanding these will help you run and troubleshoot the examples confidently.

- `challenge-id`
  - What it is: A stable, unique ID for the problem itself. It represents “the thing to solve”.
  - Who creates it: The Challenger service when a challenge is created (e.g., via its own API or our example seeder). In local E2E shortcuts, we manually pick a value like `ch_dbg_1` and seed it into both databases.
  - Where it lives:
    - Challenger DB table `challenges.id`
    - Solver DB table `pending_challenges.id` (only while waiting to be answered)
  - How it’s used:
    - In the callback URL path: `http://<challenger-host>/callback/{challenge-id}`
    - As the lookup key so the Challenger can validate and persist the result.
  - Why it matters: If the Challenger doesn’t have a matching `challenge-id` in its DB, it returns `404` on callback because it cannot find “the thing to solve”.

- `job-id` (a.k.a. `solver_job_id`)
  - What it is: A work-tracking ID on the Solver side for “this attempt to solve the challenge”. Think of it as a queue/job identifier for observability.
  - Who creates it: The Solver when a challenge is accepted via `/solve` (HTTP). In this codebase we format it as `solver_job_<challenge-id>`. In the gRPC example, you pass it from the client and it’s forwarded unchanged.
  - Where it shows up:
    - Returned by the Solver in `SolveResponse.solver_job_id` (HTTP flow)
    - Included in the HTTP callback body as `solver_job_id` so the Challenger can log/audit it.
  - How the Challenger uses it:
    - For tracing in logs and stored results. It’s not used to route or locate the challenge; the Challenger still looks up by `challenge-id`.

Putting it together with the Python example:

```
python3 examples/grpc/client.py \
  --challenge-id ch_dbg_1 \
  --job-id solver_job_ch_dbg_1 \
  --answer "MOCK_ANSWER" \
  --target localhost:9090
```

- `--challenge-id ch_dbg_1`:
  - Must match a row in both DBs for the local shortcut flow:
    - `challenger.db` → table `challenges` (represents the known problem)
    - `solver.db` → table `pending_challenges` (holds the callback URL)
  - The Solver reads `solver.db` to find the callback URL for `ch_dbg_1`, signs a request, and POSTs to `http://127.0.0.1:8080/callback/ch_dbg_1`.

- `--job-id solver_job_ch_dbg_1`:
  - A label for this solving attempt. It’s forwarded to the Challenger in the callback body as `solver_job_id` for traceability.
  - Helpful for debugging and metrics; not used to locate the challenge on the Challenger side.

- `--answer "MOCK_ANSWER"`:
  - The content the Solver is “returning”. For this local flow, the Challenger has a validation rule (e.g., ExactMatch) and will mark it correct/incorrect accordingly.

- `--target localhost:9090`:
  - The gRPC bridge address. The Python client calls `SubmitAnswer` on the Solver bridge. The bridge then performs the signed HTTP callback to the Challenger.

Common pitfalls and how to fix them:
- 404 on callback: The Challenger cannot find `challenge-id` in its own DB. Seed the same `challenge-id` into `challenger.db` (and ensure both services use the same `.env` DB paths).
- Auth errors: Ensure both services share the same HMAC config (e.g., `SHARED_SECRET_KEY=dev-shared-secret`) and clocks are within allowed skew.
- Worker race: When testing the gRPC path, set `SOLVER_WORKER_COUNT=0` to prevent built‑in workers from consuming the pending challenge before your gRPC answer arrives.

### Local E2E Walkthrough (no ngrok, mock answer)

This flow seeds a pending challenge directly into the solver DB, then reports an answer via gRPC; the solver forwards a signed callback to the challenger.

1) Start services
- Terminal A: `go run cmd/challenger/main.go`
- Terminal B: `SOLVER_GRPC_BRIDGE_ADDR=:9090 ./bin/solver` (built with `make build-solver-grpc`)

2) Ensure shared secret (for callback HMAC)
- In `.env`, set `SHARED_SECRET_KEY=dev-shared-secret` (or individual keys).

3) Seed a pending challenge into solver DB
- `go run examples/grpc/seed_pending.go --challenge-id ch_local_e2e`
  - Uses callback `http://127.0.0.1:8080/callback/ch_local_e2e` by default

4) Submit an answer via gRPC (Python)
- `python3 examples/grpc/client.py --challenge-id ch_local_e2e --answer "MOCK_ANSWER" --target localhost:9090`

5) Verify
- Challenger logs show callback processed and result stored.

## 🔧 Configuration Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `CHALLENGER_HOST` | Challenger bind address | `0.0.0.0` |
| `CHALLENGER_PORT` | Challenger port | `8080` |
| `USE_NGROK` | Enable ngrok mode (HTTPS required) | `false` |
| `PUBLIC_CALLBACK_HOST` | Public callback URL | *auto-set when USE_NGROK=false* |
| `SOLVER_HOST` | Solver bind address | `0.0.0.0` |
| `SOLVER_PORT` | Solver port | `8081` |
| `SOLVER_WORKER_COUNT` | Concurrent workers | `4` |
| `SHARED_SECRET_KEY` | HMAC key (MVP) | *required* |
| `CLOCK_SKEW_SECONDS` | Auth time window | `300` |
| `LOG_LEVEL` | Logging level | `info` |

## 🚨 Production Considerations

**Security**:
- Use individual HMAC keys per service (not shared)
- Implement proper host whitelisting for callbacks
- Enable request rate limiting
- Use HTTPS certificates (set USE_NGROK=true for external access)

**Scalability**:
- Consider Redis for nonce storage in multi-instance deployments
- Implement database connection pooling
- Add metrics collection (Prometheus)

**Reliability**:
- Configure reverse proxy timeouts appropriately
- Implement circuit breakers for external calls
- Add comprehensive logging and alerting

## 📝 License

This project is part of a technical specification implementation. See project documentation for licensing terms.

## 🤝 Contributing

This is an MVP implementation. For production deployment, consider:
- Comprehensive error handling
- Performance optimization  
- Additional challenge types
- Advanced retry policies
- Monitoring integration
- Multi-tenancy support

## 📦 Data Storage: Challenger DB vs Solver DB

This system intentionally keeps two small, separate SQLite databases — one per service — to reflect their distinct responsibilities and to minimize cross‑service coupling. Below is a detailed description of what each DB stores, the exact table structures, and why each piece exists.

### Challenger DB (`challenger.db`)

Authoritative store for challenges (what needs to be solved), validation rules, results, webhook audits, and replay‑protection nonces. It is the source of truth for verification and auditing.

Tables

1) `challenges`

Schema:

```
id TEXT PRIMARY KEY,
type TEXT NOT NULL,
problem TEXT NOT NULL,
output_spec TEXT NOT NULL,
validation_rule TEXT NOT NULL,
created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
```

Purpose and field rationale:
- `id`: Stable unique identifier of the challenge (e.g., `ch_text_xxx`). Used everywhere as the logical problem key and as a path segment in the callback URL.
- `type`: High‑level category (e.g., text, math, captcha). Helps downstream processing and UI.
- `problem`: JSON payload describing the problem (stored as text). This is what the Solver receives to compute an answer.
- `output_spec`: JSON describing expected output shape (e.g., content_type, schema). Aids validation and interoperability.
- `validation_rule`: JSON encoding of the rule used to decide if an answer is correct (e.g., ExactMatch, Regex, NumericTolerance). Kept on the Challenger and never exposed to solvers.
- `created_at`: Auditability and ordering.

Why this table exists: The Challenger must validate answers independently; storing the full problem and its validation rule allows reproducible checks and auditing.

2) `results`

Schema:

```
id INTEGER PRIMARY KEY AUTOINCREMENT,
challenge_id TEXT NOT NULL,
request_id TEXT NOT NULL,
solver_job_id TEXT,
status TEXT NOT NULL,
received_answer TEXT,
is_correct BOOLEAN,
compute_time_ms INTEGER,
solver_metadata TEXT,
created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
FOREIGN KEY (challenge_id) REFERENCES challenges(id),
UNIQUE (challenge_id, request_id)
```

Indexes:
- `ix_results_cid_created` on `(challenge_id, created_at)`

Purpose and field rationale:
- `challenge_id`: Associates the result to a known challenge definition.
- `request_id`: The HTTP `X-Request-ID` from the callback; used for idempotency (`UNIQUE (challenge_id, request_id)`) so a retried callback won’t duplicate rows.
- `solver_job_id`: Traceability back to the Solver’s unit of work.
- `status`: `success` or `failed` outcome of the solve attempt.
- `received_answer`: Raw answer string for audit and later analysis.
- `is_correct`: Result of applying the `validation_rule` to `received_answer`.
- `compute_time_ms` and `solver_metadata`: Optional performance and method details, stored as integers/JSON.
- `created_at`: When Challenger stored the result.

Why this table exists: Durable, queryable history of solver outcomes, with strict idempotency for safe retries.

3) `webhooks`

Schema:

```
id INTEGER PRIMARY KEY AUTOINCREMENT,
challenge_id TEXT NOT NULL,
request_id TEXT NOT NULL,
headers TEXT,
body_hash TEXT,
status_code INTEGER,
created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
```

Purpose and field rationale:
- Stores a minimal audit trail of incoming callback requests (headers snapshot, body hash) for security forensics and debugging (e.g., mismatched signatures, load‑balancer behavior).
- `request_id` links back to `results` rows; both are written per‑callback.

Why this table exists: Operational visibility into the webhook layer, which can be a common point of failure.

4) `seen_nonces`

Schema:

```
nonce TEXT PRIMARY KEY,
seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
```

Indexes:
- `ix_seen_nonces_seen_at` on `(seen_at)`

Purpose and field rationale:
- Nonce storage for HMAC replay protection. Each authenticated request carries a unique `nonce` which is rejected if seen before.

Why this table exists: Prevents replay attacks on signed HTTP requests.

### Solver DB (`solver.db`)

Operational queue and state for Solver workers plus HMAC nonce storage. It intentionally does not store validation rules (those belong to the Challenger). In the gRPC example, we also store the callback URL here for the Solver to forward results.

Tables

1) `pending_challenges`

Schema:

```
id TEXT PRIMARY KEY,
problem TEXT NOT NULL,
output_spec TEXT NOT NULL,
callback_url TEXT NOT NULL,
received_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
status TEXT NOT NULL DEFAULT 'pending',
attempt_count INTEGER DEFAULT 0,
next_retry_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
```

Indexes:
- `ix_pending_status_retry` on `(status, next_retry_time)`

Purpose and field rationale:
- `id`: Same `challenge-id` as on the Challenger; ties queue items back to the original problem.
- `problem` and `output_spec`: Copy of the data needed to execute the job without re‑calling the Challenger.
- `callback_url`: Where the Solver should POST results (Challenger’s `/callback/{challenge-id}` endpoint). The Solver signs this callback with HMAC.
- `status`: Lightweight state machine for workers (e.g., `pending`, `processing`, `failed`).
- `attempt_count` and `next_retry_time`: Backoff/retry scheduling fields for robust delivery in real deployments.
- `received_at`: Ingestion timestamp for monitoring and ordering.

Why this table exists: Provides a decoupled work queue for the Solver so it can independently scale and retry without blocking the Challenger.

2) `seen_nonces`

Same schema and purpose as in the Challenger DB, but used for the Solver’s inbound authenticated endpoints.

### Why two databases?

- Clear ownership: The Challenger owns problem definitions, validation, and final truth. The Solver owns execution state and retries.
- Failure isolation: A failure or lock in one service does not corrupt or stall the other’s critical state.
- Simpler operational model: Each service can be developed, tested, and scaled independently with its own persistence lifecycle.
- Security posture: Validation rules never leave the Challenger; solvers see only what they need to compute answers.

### How the data flows (end‑to‑end)

1) Challenge creation (Challenger)
- Row inserted into `challenger.db.challenges` with the problem JSON, output spec, and `validation_rule`.

2) Challenge acceptance (Solver)
- Via HTTP `/solve`, the Solver persists a row into `solver.db.pending_challenges` with the callback URL; or in local shortcuts, you seed it directly.

3) Solve + callback (Solver → Challenger)
- Solver (or gRPC bridge) computes/receives an answer, then sends a signed HTTP POST to `callback_url` (e.g., `/callback/{challenge-id}`) with `solver_job_id`, `status`, and `answer`.

4) Validation + persistence (Challenger)
- Challenger verifies HMAC signature and replay (`seen_nonces`), checks `challenge-id` exists in `challenges`, validates `answer` using `validation_rule`, then writes a row into `results` (idempotent on `(challenge_id, request_id)`).
- A webhook audit row may be stored in `webhooks` with a header snapshot and body hash.

5) Cleanup (Solver)
- On a 2xx callback response, the Solver removes the entry from `pending_challenges`.

This split keeps correctness logic and auditability on the Challenger while giving the Solver a robust, retry‑friendly queue to manage external computation and delivery.
