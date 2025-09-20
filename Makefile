# Reverse Challenge System - Makefile

.PHONY: help build test clean run-challenger run-solver run-verifier example deps
.PHONY: proto-gen-go proto-gen-py build-solver-grpc grpc-deps proto-verify
.PHONY: build-initializer deploy-contracts build-verifier

# Default target
help:
	@echo "Reverse Challenge System - Available commands:"
	@echo "  deps           - Download dependencies"
	@echo "  build          - Build all services"
	@echo "  test           - Run tests"
	@echo "  run-challenger - Start challenger service"
	@echo "  run-solver     - Start solver service"
	@echo "  run-verifier   - Run transaction verifier"
	@echo "  example        - Send example challenges"
	@echo "  clean          - Clean build artifacts and databases"
	@echo "  setup          - Initial setup (deps + env)"
	@echo "  deploy-contracts - Deploy smart contracts"

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build all services

build:
	@echo "Building Challenger service..."
	mkdir -p .gocache
	GOCACHE=$(PWD)/.gocache go build -o bin/challenger ./cmd/challenger
	@echo "Building Solver service..."
	GOCACHE=$(PWD)/.gocache go build -o bin/solver ./cmd/solver
	@echo "Building Verifier CLI..."
	GOCACHE=$(PWD)/.gocache go build -o bin/verifier ./cmd/verifier
	@echo "Build complete!"

# Build individual services
build-initializer:
	@echo "Building Initializer CLI..."
	mkdir -p .gocache
	GOCACHE=$(PWD)/.gocache go build -o bin/initializer ./cmd/initializer
	@echo "Initializer CLI built!"

build-verifier:
	@echo "Building Verifier CLI..."
	mkdir -p .gocache
	GOCACHE=$(PWD)/.gocache go build -o bin/verifier ./cmd/verifier
	@echo "Verifier CLI built!"

# Run tests
test:
	@echo "Running tests..."
	mkdir -p .gocache
	GOCACHE=$(PWD)/.gocache go test ./pkg/... -v
	@echo "Tests complete!"

# Start challenger service
run-challenger:
	@echo "Starting Challenger service on :8080..."
	go run cmd/challenger/main.go

# Start solver service
run-solver:
	@echo "Starting Solver service on :8081..."
	go run cmd/solver/main.go cmd/solver/grpc_bridge_disabled.go

# Run verifier CLI
run-verifier:
	@echo "Running Verifier CLI..."
	go run cmd/verifier/main.go

# Send example challenges
example:
	@echo "Sending example challenges..."
	go run examples/send_challenge.go

# Clean up
clean:
	@echo "Cleaning up..."
	rm -f bin/challenger bin/solver bin/verifier bin/initializer
	rm -f challenger.db solver.db
	rm -f challenger.db-wal challenger.db-shm
	rm -f solver.db-wal solver.db-shm
	@echo "Clean complete!"

# Initial setup
setup: deps
	@echo "Setting up environment..."
	@if [ ! -f .env ]; then \
		echo "Copying .env.example to .env..."; \
		cp .env.example .env; \
		echo "Please edit .env with your configuration!"; \
	else \
		echo ".env already exists"; \
	fi
	@echo "Setup complete!"

# Development workflow
dev: clean setup build test
	@echo "Development setup complete!"
	@echo "1. Edit .env with your ngrok URL"
	@echo "2. Start ngrok: ngrok http 8080"
	@echo "3. Update PUBLIC_CALLBACK_HOST in .env"
	@echo "4. Run: make run-challenger"
	@echo "5. In another terminal: make run-solver"
	@echo "6. Test with: make example"

# Deploy smart contracts
deploy-contracts: build-initializer
	@echo "Deploying smart contracts..."
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found. Run 'make setup' first."; \
		exit 1; \
	fi
	./bin/initializer
	@echo "Contract deployment complete!"

# --- gRPC / Protobuf utilities ---

# Generate Go stubs for the solver bridge gRPC service.
# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc in PATH.
proto-gen-go:
	@echo "Generating Go gRPC stubs..."
	PATH="$$PATH:$$(go env GOPATH)/bin:$$HOME/go/bin" \
	protoc -I proto \
		--go_out=. \
		--go-grpc_out=. \
		proto/solver_bridge.proto
	@echo "Go stubs generated under proto/solverbridge/*.pb.go"

# Generate Python stubs for the solver bridge gRPC service.
# Requires: pip install grpcio grpcio-tools
proto-gen-py:
	@echo "Generating Python gRPC stubs..."
	python3 -m grpc_tools.protoc -I proto \
		--python_out=examples/grpc \
		--grpc_python_out=examples/grpc \
		proto/solver_bridge.proto
	@echo "Python stubs generated under examples/grpc/"

# Add gRPC dependencies (optional convenience target).
grpc-deps:
	@echo "Adding gRPC deps to go.mod (requires network access)..."
	go get google.golang.org/grpc@v1.64.0
	go get google.golang.org/protobuf@v1.34.0
	@echo "Done."

# Build solver with gRPC bridge enabled via build tag.
build-solver-grpc:
	@echo "Building Solver with gRPC bridge (tag grpcbridge)..."
	mkdir -p .gocache
	GOCACHE=$(PWD)/.gocache go build -tags=grpcbridge -o bin/solver ./cmd/solver
	@echo "Built bin/solver with gRPC bridge. Configure SOLVER_GRPC_BRIDGE_ADDR if needed."

# Verify required tools for proto generation are installed and discoverable.
proto-verify:
	@PATH="$$PATH:$$(go env GOPATH)/bin:$$HOME/go/bin"; \
	  echo "Checking protoc..."; \
	  command -v protoc >/dev/null 2>&1 || { echo "Missing protoc. Install from https://grpc.io/docs/protoc-installation/"; exit 1; }; \
	  protoc --version; \
	  echo "Checking protoc-gen-go..."; \
	  command -v protoc-gen-go >/dev/null 2>&1 || { echo "Missing protoc-gen-go. Install: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; exit 1; }; \
	  echo "protoc-gen-go: $$(command -v protoc-gen-go)"; \
	  echo "Checking protoc-gen-go-grpc..."; \
	  command -v protoc-gen-go-grpc >/dev/null 2>&1 || { echo "Missing protoc-gen-go-grpc. Install: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"; exit 1; }; \
	  echo "protoc-gen-go-grpc: $$(command -v protoc-gen-go-grpc)"; \
	  echo "Checking Python grpcio and grpcio-tools..."; \
	  python3 -c "import grpc" 2>/dev/null || { echo "Missing Python package: grpcio (pip install grpcio)"; exit 1; }; \
	  python3 -c "import grpc_tools" 2>/dev/null || { echo "Missing Python package: grpcio-tools (pip install grpcio-tools)"; exit 1; }; \
	  echo "All gRPC tooling present."
