# Repository Guidelines

## Project Structure & Module Organization
- `cmd/`: Service entry points (`challenger/`, `solver/`).
- `internal/`: Service logic (`challenger/`, `solver/` workers, handlers).
- `pkg/`: Reusable libraries (`api/`, `auth/`, `config/`, `db/`, `logger/`, `models/`, `validator/`).
- `examples/`: Demo scripts (e.g., `send_challenge.go`).
- `bin/`: Built binaries (`challenger`, `solver`).
- Datastores: `challenger.db`, `solver.db` (SQLite). Clean with `make clean`.

## Build, Test, and Development Commands
- `make setup`: Download deps, scaffold `.env` from `.env.example`.
- `make build`: Build both services to `bin/`.
- `make test`: Run package tests (`./pkg/...`).
- `make run-challenger` / `make run-solver`: Run services locally (ports 8080/8081).
- `make example`: Send sample challenges after services are running.
- `make dev`: Clean → setup → build → test; prints next steps (ngrok, env).

## Coding Style & Naming Conventions
- Language: Go 1.21+. Format with `go fmt ./...`; verify with `go vet ./...`.
- Indentation: tabs (Go default). Keep files `gofmt`-clean before pushing.
- Packages: lower_snake for dirs; exported names are `CamelCase`; private are `camelCase`.
- Files: tests end with `_test.go`; HTTP handlers/middleware in `pkg/api/`.
- Logging: use `pkg/logger` (zerolog) and structured fields; avoid logging secrets.

## Testing Guidelines
- Unit tests live beside code in `pkg/**/**/*_test.go`; prefer table-driven tests.
- Run: `make test` or `go test ./pkg/... -v`; optional coverage: `go test -cover ./pkg/...`.
- Integration: start both services, then `go run examples/send_challenge.go`; inspect SQLite as needed.

## Commit & Pull Request Guidelines
- Commits: short imperative subject; include scope when helpful (e.g., `pkg/db: fix JSON scan`).
- PRs must include:
  - Purpose, summary of changes, and any schema/API/config updates.
  - Repro steps and test evidence (logs, sample curl, or `make example` output).
  - Linked issue (if applicable). Keep PRs focused and reasonably small.

## Security & Configuration Tips
- Secrets: never commit real values; use `.env` and update `.env.example` when adding keys.
- HMAC key (`SHARED_SECRET_KEY`) must match services in MVP; rotate for production.
- Callbacks must be HTTPS in real deployments; ngrok for local testing.

## Agent-Specific Instructions
- Keep changes minimal and scoped; do not alter public APIs without coordination.
- Follow this guide and existing patterns; update docs when behavior/config changes.
