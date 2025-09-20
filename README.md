# AIBattle — Sui-based Adversarial AI Prediction Market

**Abstract:** The pursuit of Artificial General Intelligence (AGI) is handicapped by a fundamental flaw: our benchmarks for measuring it are static, gameable, manipulated, and centrally controlled. AIBattle introduces a new paradigm: a decentralized, adversarial prediction market built on Sui. The system functions through crypto-economic proofs where Challengers post bounties and Solvers submit solutions. Outcomes are secured through on-chain logic and a finality-providing arbitration layer, creating a persistent, publicly auditable log of adversarial challenges. Layered on top, a prediction market transforms this log into a live economic signal, as traders price the probability of each task's success. It is the real-time aggregation of these price signals that forms the "AGI Clock"—a decentralized oracle tracking our trajectory toward the technological singularity.

## 🎯 Core Features

- **Answer Security**: Validation rules and correct answers never leave the Challenger's environment
- **Decentralized Compute**: Solvers use their own hardware (GPU, TPU, specialized equipment)
- **Technical Freedom**: Solvers can use any language/framework/hardware stack
- **Robust Authentication**: HMAC-SHA256 signatures with nonce-based replay protection
- **Retry Logic**: Exponential backoff with jitter for reliable callback delivery
- **Idempotency**: Duplicate-safe operations using X-Request-ID headers
- **Sui Integration**: On-chain commitments and smart contract deployment

## 🧩 System Overview

**Reverse Challenge System** - A Go-based system that reverses the traditional challenge-solving model. Instead of solvers uploading code to centralized platforms, **Challengers push problems to Solvers** who process them on their own compute resources and return results via authenticated callbacks.

### Three-Layer Market Design

1. **First-order market (tasks & verification)**
   - **Challengers** stake and post tasks (math, reasoning, CAPTCHAs, agentic missions)
   - **Solvers** (humans, LLMs, algorithms) submit answers
   - **Verifiers** validate results and escalate disputes to arbitration

2. **Second-order market (prediction on task outcomes)**
   - **Spectators** bet on whether tasks will be completed by specified deadlines

3. **AGI Clock** - Aggregated market signals form a real-time indicator of AI capability trends

## 🏗️ Technical Architecture


```
┌─────────────────┐          ┌─────────────────┐          ┌─────────────────┐
│   CHALLENGER    │          │      SOLVER     │          │    VERIFIER     │
│                 │          │                 │          │                 │
│ • Create tasks  │   (1)    │ • Process async │   (3)    │ • Read from Sui │
│ • Validate ans. │ ◄─────── │ • Worker pool   │          │ • Verify commit │
│ • Add bounty    │          │ • Retry logic   │          │ • Transfer fund │
│ • Upload commit │          │ • Submit answer │          │ • Independent   │
└─────────┬───────┘          └─────────┬───────┘          └─────────┬───────┘
          │                            │                            │
          │ (2) Upload                 │ (4) Submit                 │ (5) Read
          │     commitment             │     callback               │     data
          │                            │                            │
          ▼                            ▼                            ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                               SUI BLOCKCHAIN                                 │
│                                                                              │
│    ┌─────────────┐       ┌─────────────┐       ┌─────────────────────────┐  │
│    │   REGISTRY  │       │    VAULT    │       │      COMMITMENTS       │  │
│    │             │       │             │       │                         │  │
│    │ • Protocols │◄─(6)──│ • Bounties  │       │ • Challenge hashes      │  │
│    │ • Metadata  │       │ • Rewards   │       │ • Validation results    │  │
│    │ • Addresses │       │ • Transfer  │       │ • Solver submissions    │  │
│    └─────────────┘       └─────────────┘       └─────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────┘

Flow:
(1) HMAC signed challenge requests
(2) Upload commitment hash to blockchain
(3) Process challenge and generate answer
(4) HMAC signed callback with results
(5) Read and verify commitment data
(6) Transfer bounty to solver upon verification
```

## 🚀 Quick Start

## 🎮 Challenge Types

1. **CAPTCHA** - Image-to-text extraction tasks
2. **Puzzle Reasoning** - Logic puzzles and computational problems
3. **AI Agent Missions** - Real-world tasks like "fetch today's BBC headline" or "register on a forum"
4. **LLM-trap Tasks** - Easy for humans but challenging for current AI systems
5. **Formal Problems** - SAT/Lean proofs, NP-complete problems with machine-checkable solutions

### Prerequisites

- Go 1.21+
- SQLite3
- Sui CLI (for blockchain integration)
- ngrok (optional, for public callback URLs when USE_NGROK=true)

### 1. Clone and Setup

```bash
git clone https://github.com/limaologic/AIBattle.git
cd AIBattle
make setup  # Downloads deps and creates .env from .env.example
```

### 2. Development Commands

```bash
make build            # Build both challenger and solver services
make run-challenger   # Start challenger service (:8080)
make run-solver       # Start solver service (:8081)
make example          # Send test challenges
make test             # Run all tests
make clean            # Clean build artifacts
```

### 3. Blockchain Integration (Optional)

```bash
# Configure Sui in .env
SUI_MNEMONIC=your_mnemonic_here
SUI_RPC_URL=https://fullnode.testnet.sui.io:443
SUI_PACKAGE_ID=your_deployed_package_id

# Deploy contracts
make deploy-contracts
```

### 4. Local Testing

```bash
# Terminal 1
make run-challenger

# Terminal 2
make run-solver

# Terminal 3 - Send test challenges
make example
```

### 5. gRPC Bridge (External Solvers)

For Python/LLM integration:

```bash
make build-solver-grpc  # Build with gRPC support
make proto-gen-py       # Generate Python stubs

# Run Python solver client
python3 examples/grpc/client.py --challenge-id ch_test --answer "MOCK_ANSWER"
```

## 🔧 Technical Details

### API Flow

1. **Challenge Request**: Challenger → Solver via HMAC-signed POST `/solve`
2. **Processing**: Solver processes challenge asynchronously
3. **Callback**: Solver → Challenger with signed results via `/callback/{challenge-id}`
4. **Validation**: Challenger validates answer and stores result
5. **Blockchain**: Optional Sui commitment upload

### Authentication & Security

- **HMAC-SHA256** signatures with nonce-based replay protection
- **Time window validation** (±300 seconds configurable)
- **Answer isolation**: Validation rules never leave Challenger
- **Idempotent operations** with X-Request-ID headers

### Key Configuration

```bash
# .env file
USE_NGROK=false                    # Local dev (true for external access)
SHARED_SECRET_KEY=dev-shared-secret # HMAC signing key
SOLVER_WORKER_COUNT=4              # Concurrent workers
SUI_MNEMONIC=your_mnemonic         # Blockchain integration
```

## 🧪 Testing & Monitoring

```bash
# Run tests
make test

# Health checks
curl localhost:8080/healthz  # Challenger health
curl localhost:8081/healthz  # Solver health

# Database inspection
sqlite3 challenger.db "SELECT * FROM results;"
sqlite3 solver.db "SELECT * FROM pending_challenges;"
```

## 📁 Project Structure

```
├── cmd/                     # Service entry points
│   ├── challenger/          # Challenge creation service
│   ├── solver/              # Challenge processing service
│   └── initializer/         # Contract deployment CLI
├── internal/                # Core business logic
├── pkg/                     # Shared packages (auth, db, models)
├── proto/                   # gRPC definitions
└── examples/                # Demo scripts & gRPC clients
```

## 🤝 Contributing

AIBattle is an ambitious project to create decentralized AGI benchmarks. We welcome contributions in several areas:

### Development Priorities
- **Challenge Types**: Implement new types of AI/human-distinguishable tasks
- **Sui Integration**: Enhanced smart contract functionality and oracle feeds
- **Security**: Improved authentication, anti-gaming measures
- **Performance**: Optimizations for high-throughput challenge processing
- **UI/UX**: Web interface for challenge creation and market participation

### Getting Started
1. Fork the repository
2. Follow the Quick Start guide above
3. Check existing issues for good first contributions
4. Submit PRs with clear descriptions and tests

### Architecture Notes
- **Two-DB design**: Separate SQLite databases for Challenger (validation) and Solver (work queue)
- **HMAC security**: All inter-service communication uses signed requests with replay protection
- **Modular validation**: Easy to add new challenge types and validation rules
- **gRPC bridge**: Enables external solvers (Python/LLM) via `make build-solver-grpc`

For detailed technical documentation, see `CLAUDE.md` and `ARCHITECTURE.md`.

---

**Vision**: Transform AI capability measurement from static leaderboards to dynamic, adversarial markets that provide real-time signals on the path to AGI.

**Status**: MVP implementation with core challenge-solving infrastructure and Sui blockchain integration.
