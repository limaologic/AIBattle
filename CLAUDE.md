# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Reverse Challenge System (MVP) - A Go-based system that reverses the traditional challenge-solving model. Instead of solvers uploading code to centralized platforms, Challengers push problems to Solvers who process them on their own compute resources and return results via authenticated callbacks.

## Development Commands

### Build Commands
```bash
make build            # Build both challenger and solver services to bin/
make build-solver-grpc # Build solver with gRPC bridge support
make deps             # Download and tidy Go dependencies
make grpc-deps        # Add gRPC/protobuf dependencies to go.mod
make clean            # Remove build artifacts and databases
```

### Running Services
```bash
make run-challenger # Start challenger service on :8080
make run-solver     # Start solver service on :8081
make example        # Send test challenges to demonstrate the system
```

### Testing
```bash
make test           # Run all tests in ./pkg/... with verbose output
go test ./pkg/auth -v      # Test HMAC authentication
go test ./pkg/validator -v # Test answer validation engine
```

### Development Setup
```bash
make setup          # Initial setup: deps + create .env from .env.example
make dev            # Full dev setup: clean + setup + build + test
make deploy-contracts # Deploy smart contracts using initializer
```

### Contract Deployment
```bash
make build-initializer  # Build the contract deployment CLI
make deploy-contracts   # Deploy contracts (builds initializer if needed)
./bin/initializer --help  # Show CLI options
```

### gRPC Bridge Development
```bash
make proto-verify   # Check protoc and tooling availability
make proto-gen-go   # Generate Go protobuf stubs
make proto-gen-py   # Generate Python protobuf stubs
```

## Architecture Overview

### Core Components

**Services:**
- `cmd/challenger/` - Challenge creation service that pushes problems to solvers
- `cmd/solver/` - Processing service that runs challenges and returns results via callbacks
  - Optional gRPC bridge server (enabled with `-tags=grpcbridge`)

**Key Packages:**
- `pkg/auth/hmac.go` - HMAC-SHA256 authentication with nonce-based replay protection
- `pkg/models/` - Data structures and API contracts (v2.1)
- `pkg/validator/` - Answer validation engine (exact match, numeric tolerance, regex)
- `pkg/db/` - SQLite database layers for challenges and results storage
- `internal/solver/worker.go` - Worker pool with exponential backoff retry logic
- `internal/solver/grpc_bridge_server.go` - gRPC bridge for external solvers (Python/LLM)

**Protocol Buffers:**
- `proto/solver_bridge.proto` - gRPC service definition for external solver integration
- Generated Go stubs in `proto/solverbridge/`
- Generated Python stubs in `examples/grpc/`

### Security Model

The system implements a zero-trust model where:
- Challengers store answers locally (never transmitted)
- Solvers receive only problem data and output specifications  
- All communication uses HMAC-SHA256 signatures with timestamped nonces
- Callback authentication prevents replay attacks

### Database Design

**Challenger DB (`challenger.db`):**
- `challenges` - Problems with validation rules and answers (local only)
- `results` - Solver responses and validation outcomes
- `webhooks` - Callback audit trail
- `seen_nonces` - Replay attack prevention

**Solver DB (`solver.db`):**
- `pending_challenges` - Work queue with retry state management
- `seen_nonces` - Replay attack prevention

## Configuration

Environment variables are loaded from `.env` (copy from `.env.example`):

**Required for Development:**
- `USE_NGROK` - Enable ngrok mode for external access (default: false)
- `PUBLIC_CALLBACK_HOST` - Callback URL (auto-set to localhost when USE_NGROK=false)
- `SHARED_SECRET_KEY` - HMAC signing key (MVP uses shared secret)
- `SOLVER_WORKER_COUNT` - Number of concurrent workers (default: 4, set to 0 for gRPC-only mode)
- `SOLVER_GRPC_BRIDGE_ADDR` - gRPC bridge address (default: :9090)

**Local Development (Default - No ngrok needed):**
```bash
# .env already configured with USE_NGROK=false
# PUBLIC_CALLBACK_HOST automatically set to http://localhost:8080
make run-challenger && make run-solver && make example
```

**External Access (Optional - ngrok setup):**
```bash
# Only needed for external testing
# Set USE_NGROK=true in .env first
ngrok http 8080  # Copy https URL to PUBLIC_CALLBACK_HOST in .env
```

**Additional Configuration:**
- `CLOCK_SKEW_SECONDS` - HMAC auth time window (default: 300)
- `LOG_LEVEL` - Logging level (info, debug, error)
- Database files: `challenger.db`, `solver.db` (SQLite)

## Challenge Types

The system supports extensible challenge types with mock implementations:
- **CAPTCHA**: Base64 image → text extraction
- **Math**: Numerical computations with tolerance validation
- **Text**: String processing operations

Add new types by implementing handlers in `internal/solver/service.go` and validation rules in `pkg/validator/`.

## Testing Integration

### Standard HTTP Flow
1. Start both services: `make run-challenger` and `make run-solver`
2. Send test challenges: `make example`
3. Verify results: `sqlite3 challenger.db "SELECT * FROM results;"`

### gRPC Bridge Testing
1. Build solver with gRPC support: `make build-solver-grpc`
2. Start services:
   - Terminal A: `go run cmd/challenger/main.go`
   - Terminal B: `SOLVER_GRPC_BRIDGE_ADDR=:9090 ./bin/solver`
3. Seed test data:
   - `go run examples/grpc/seed_challenger.go --challenge-id ch_test`
   - `go run examples/grpc/seed_pending.go --challenge-id ch_test`
4. Test Python client: `python3 examples/grpc/client.py --challenge-id ch_test --answer "MOCK_ANSWER" --target localhost:9090`
5. Verify callback processing in challenger logs

## Worker Pool Architecture

The solver uses a concurrent worker pool (`internal/solver/worker.go`):
- Dispatcher polls database every 5 seconds for pending challenges
- Buffered job queue distributes work to N workers
- Each worker handles challenge processing and callback delivery
- Exponential backoff retry (500ms base, 30s max, 6 attempts)
- Failures on 4xx (except 429) are not retried
- Set `SOLVER_WORKER_COUNT=0` to disable workers when using gRPC bridge only

## API Authentication

All requests use custom HMAC authentication:
```
Authorization: RCS-HMAC-SHA256 keyId=xxx,ts=timestamp,nonce=uuid,sig=hex
```
Canonical string: `METHOD\nPATH\nTIMESTAMP\nNONCE\nSHA256(body)`

Time window validation: ±300 seconds (configurable via `CLOCK_SKEW_SECONDS`)

## gRPC Bridge Architecture

External solvers (Python, LLMs, etc.) can integrate via gRPC:
- Build tag: `-tags=grpcbridge` enables the bridge server
- gRPC service: `SolverBridge` with `SubmitAnswer` method
- Python client example: `examples/grpc/client.py`
- Protocol: `proto/solver_bridge.proto`

**Key Identifiers:**
- `challenge-id`: Stable problem identifier (shared across both DBs)
- `job-id`: Solver-side work tracking ID (for tracing and observability)

**Database Seeding for Local Testing:**
- `examples/grpc/seed_challenger.go` - Seeds challenge into challenger.db
- `examples/grpc/seed_pending.go` - Seeds pending work into solver.db

## Sui Blockchain Integration

The challenger service can optionally upload challenge commitments to the Sui blockchain when solver results are processed successfully.

**Configuration:**
```bash
# Required for Sui integration
SUI_MNEMONIC=abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about
SUI_RPC_URL=https://fullnode.testnet.sui.io:443
SUI_PACKAGE_ID=0x1234567890abcdef1234567890abcdef12345678
SUI_REGISTRY_ID=0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890

# Optional addresses (defaults to placeholder addresses if not set)
SUI_CHALLENGER_ADDR=0x1234567890abcdef1234567890abcdef12345678901234567890abcdef123456
SUI_SOLVER_ADDR=0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890

SUI_TYPE_TREASURY_POS=
SUI_TYPE_TREASURY_NEG=
SUI_TYPE_COLLATERAL=0x2::sui::SUI
```

**Architecture:**
- `pkg/sui/txbuilder.go` - TransactionBuilder handles Sui blockchain interactions
- Automatically uploads successful challenge results as commitments
- Uses Ed25519 signing with mnemonic-derived keypair
- Calls `upload_challenge_commitment` Move function with challenge metadata

**Usage:**
When a solver successfully completes a challenge, the challenger automatically:
1. Creates a SHA256 commitment hash from challenge data
2. Builds a Move call transaction to `upload_challenge_commitment`
3. Signs and executes the transaction on Sui blockchain
4. Logs the transaction digest for verification

**Example Log Output:**
```
INFO Successfully uploaded challenge commitment to Sui tx_digest=ABC123... component=sui_txbuilder
```

## Contract Initializer

The system includes a contract deployment initializer that mirrors the structure and behavior of leafsii/webservice/backend/internal/initializer. It provides idempotent deployment of Sui Move contracts.

**Key Features:**
- Detects and compiles contracts from ~/Work/deepbattle/sources using Sui Move toolchain
- Deploys contracts to configured Sui network with idempotent operations
- Stores deployment metadata in database with (contract_name, chain_id) uniqueness
- Provides CLI and Makefile integration for deployment automation
- Automatic faucet funding for testnet/devnet deployments (optional --fund flag)
- **Automatic .env file updates** - Updates SUI_PACKAGE_ID and SUI_REGISTRY_ID after successful deployment
- Comprehensive validation and error handling with structured logging

**Configuration:**
```bash
# Required for contract deployment
SUI_MNEMONIC=abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about
SUI_RPC_URL=https://fullnode.testnet.sui.io:443
SUI_CHAIN_ID=testnet
CONTRACTS_PATH=./contract
DATABASE_PATH=challenger.db
```

**Architecture:**
- `internal/initializer/initializer.go` - Main deployment orchestrator with Run(ctx, cfg) entry point
- `internal/initializer/deployer.go` - Sui Move contract deployment abstraction
- `pkg/models/contracts.go` - Contract metadata model for deployment tracking
- `pkg/db/challenger.go` - Database methods for contract storage and retrieval
- `cmd/initializer/main.go` - CLI interface with validation and logging

**Usage:**
```bash
# Deploy contracts (idempotent - skips if already deployed)
make deploy-contracts

# CLI options
./bin/initializer --help
./bin/initializer --verbose --force              # Force redeploy
./bin/initializer --contracts /custom/path       # Override contracts path
./bin/initializer --network mainnet              # Override network
./bin/initializer --dry-run                      # Validate without deploying
./bin/initializer --fund                         # Request faucet funds before deployment
./bin/initializer --fund --verbose --network testnet  # Fund and deploy on testnet
```

**Automatic .env Updates:**
After successful deployment, the initializer automatically updates your `.env` file:
- `SUI_PACKAGE_ID` - Set to the deployed package ID
- `SUI_REGISTRY_ID` - Set to the created registry object ID

This eliminates manual copy-paste of deployment results and ensures your configuration stays in sync with deployed contracts.

**Database Schema:**
The contracts table tracks deployment metadata:
```sql
CREATE TABLE contracts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,                    -- Contract identifier
    address TEXT NOT NULL,                 -- Package ID on Sui
    network TEXT NOT NULL,                 -- Network name
    chain_id TEXT NOT NULL,                -- Chain identifier
    tx_hash TEXT NOT NULL,                 -- Deployment transaction
    deployed_at TIMESTAMP,                 -- Deployment timestamp
    contract_type TEXT NOT NULL,           -- "sui_move_package"
    metadata TEXT,                         -- JSON metadata
    UNIQUE (name, chain_id)
);
```

## Project Structure

```
├── cmd/
│   ├── challenger/main.go          # Challenger service entry
│   ├── initializer/main.go         # Contract deployment CLI
│   └── solver/
│       ├── main.go                 # Solver service entry
│       ├── grpc_bridge_enabled.go  # gRPC build tag conditional
│       └── grpc_bridge_disabled.go # No-op when gRPC disabled
├── internal/
│   ├── challenger/service.go       # Challenge creation & callbacks
│   ├── initializer/                # Contract deployment system
│   │   ├── initializer.go          # Main deployment orchestrator
│   │   └── deployer.go             # Sui Move deployment abstraction
│   └── solver/
│       ├── service.go              # HTTP handlers & logic
│       ├── worker.go               # Worker pool & retry logic
│       └── grpc_bridge_server.go   # gRPC bridge implementation
├── pkg/
│   ├── api/middleware.go           # HMAC auth, logging, CORS
│   ├── auth/hmac.go               # HMAC signature implementation
│   ├── config/config.go           # Environment configuration
│   ├── db/                        # Database layers (challenger/solver)
│   ├── logger/logger.go           # Structured logging (zerolog)
│   ├── models/models.go           # Data structures & API contracts
│   └── validator/                 # Answer validation engine
├── proto/
│   ├── solver_bridge.proto        # gRPC service definition
│   └── solverbridge/              # Generated Go stubs
├── examples/
│   ├── grpc/
│   │   ├── client.py              # Python gRPC client
│   │   ├── seed_challenger.go     # Challenger DB seeder
│   │   ├── seed_pending.go        # Solver DB seeder
│   │   └── README.md              # gRPC examples documentation
│   └── send_challenge.go          # HTTP integration demo
└── AICodingJournal/               # Development logs and notes
```