# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PingGoat is an uptime monitoring service built in Go (boot.dev capstone project). Users register HTTP endpoints to monitor, and a concurrent worker pool pings them on schedule, recording response times, status codes, and generating alerts on status changes. Accessed via REST API and a CLI client (`pinggoat`).

The PRD lives in `uptime-pinger-prd.md` — consult it for detailed API routes, data model, user stories, and CLI spec.

## Build & Run Commands

Go module: `PingGoat` (used in import paths, e.g., `PingGoat/internal/pinger`)

```bash
go build -o pinggoat ./cmd/api       # Build API server
go build -o pinggoat-cli ./cmd/cli   # Build CLI client
go run ./cmd/api                     # Run API server
go test ./...                        # Run all tests
go test ./internal/pinger/...        # Run tests for a single package
go test -run TestScheduler ./internal/pinger/...  # Run a single test
```

Once infrastructure is set up:
```bash
docker-compose up                    # API + PostgreSQL
goose -dir sql/schema postgres "$DATABASE_URL" up   # Run migrations
sqlc generate                        # Regenerate query code
```

## Architecture

**Two entrypoints:** `cmd/api/` (HTTP server) and `cmd/cli/` (Cobra CLI client).

**Core concurrency model** (the main Go showcase — in `internal/pinger/`):
- **Scheduler** (1 goroutine): ticks every 5s, queries DB for endpoints due for a check, sends them to a buffered jobs channel
- **Worker pool** (N goroutines): reads from jobs channel, makes HTTP requests with per-check context timeout (10s default)
- **Result writer** (1 goroutine): reads from results channel, batch-inserts check results, detects status changes to generate alerts

Fan-out/fan-in via buffered channels. Graceful shutdown uses `sync.WaitGroup` + `context.Context`.

**Key packages** (planned under `internal/`):
- `auth` — JWT generation/validation, bcrypt password hashing
- `database` — sqlc-generated code + DB connection
- `handler` — HTTP handlers for endpoints, checks, alerts, auth
- `middleware` — Auth middleware, request logging
- `pinger` — Worker pool, scheduler, HTTP checker
- `config` — App configuration from env vars

**Data layer:** PostgreSQL with Goose migrations (`sql/schema/`) and sqlc queries (`sql/queries/`). Four tables: `users`, `endpoints`, `checks`, `alerts`.

## Tech Stack

- **Go 1.22+**, chi or stdlib `net/http` for routing
- **PostgreSQL 16**, Goose for migrations, sqlc for type-safe queries
- **JWT** (golang-jwt/jwt) for auth, bcrypt for passwords
- **Cobra** for CLI, viper/envconfig for configuration
- **Docker Compose** for local dev (API + Postgres)

## Configuration

Env vars (see PRD section 11): `DATABASE_URL`, `JWT_SECRET`, `API_PORT`, `PINGER_WORKERS`, `PINGER_SCAN_INTERVAL_SECONDS`, `PINGER_DEFAULT_CHECK_INTERVAL_SECONDS`, `PINGER_HTTP_TIMEOUT_SECONDS`.
