# Repository Guidelines

## Project Structure & Module Organization

- `backend/cmd/server/` contains the Go application, HTTP handlers, workers, configuration, database setup, and embedded migrations in `migrations/`.
- `backend/internal/` contains reusable backend packages, including RSS crawlers, article extraction, and translation clients.
- `frontend/src/` contains the React/TypeScript Telegram Mini App; `frontend/public/` holds static assets.
- `frontend/dist/` is generated output. `data/` is local SQLite data and should not be committed.
- `Dockerfile`, `docker-compose.yml`, and `Makefile` define the containerized development and deployment workflow.

## Build, Test, and Development Commands

Backend commands should be run from `backend/`:

```bash
go test ./...                 # Run all Go tests
go run ./cmd/server           # Start the backend locally
```

Frontend commands should be run from `frontend/`:

```bash
pnpm install                  # Install locked dependencies
pnpm dev                      # Start Vite development server
pnpm build                    # Type-check and create a production build
```

For the full container workflow, use `make build`, `make up`, `make restart`, `make logs`, or `make down`. Copy `.env.example` to `.env` and keep secrets out of Git.

## Coding Style & Naming Conventions

Format Go changes with `gofmt`; use standard Go naming (exported `PascalCase`, local `camelCase`) and keep packages focused. Follow the existing TypeScript/React style: two-space indentation, `PascalCase` component names, `camelCase` functions/variables, and kebab-case CSS class names where appropriate. Keep API and database naming consistent with existing snake_case fields and migration conventions.

## Testing Guidelines

Go tests use the standard `testing` package and live beside implementation files with names such as `*_test.go`. Run `go test ./...` before submitting backend changes. Frontend currently has a build/type-check gate but no dedicated test suite; run `pnpm build` for frontend changes.

## Commit & Pull Request Guidelines

Use short, imperative commit subjects with the repository’s existing prefixes, such as `feat:`, `feat(api):`, or `fix:`. Keep unrelated changes separate. Pull requests should explain the behavior change, identify migrations or configuration changes, include test/build commands run, and attach screenshots for visible UI changes. Call out new environment variables and any operational or security considerations.

## Code Change Governance

For code changes, use the `typesafe-code` MCP server for change classification, risk review, test selection, dependency decisions, and merge gating.

Before proceeding automatically, inspect the `needsHumanReview` and `confidence` fields in each relevant response. Stop and request human review when `needsHumanReview` is `true` or when confidence is insufficient for the decision.
