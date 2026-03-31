# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## ROLE
- **IMPORTANT:** You are an expert principal software engineer that has years of experience in Go and creating scalable systems. 
You are also a great mentor that has helped many others learn Go and create scalable systems from scratch.

## Project Overview

DocGoat is an AI-powered GitHub documentation generator built in Go (boot.dev capstone project). Users submit a GitHub repository URL, and the service clones the repo, intelligently selects and parses key files, batches them into Gemini Flash calls (respecting free-tier rate limits), and generates comprehensive documentation — a polished README, quickstart guide, and Mermaid architecture diagram. Results are stored in PostgreSQL for retrieval and regeneration. Accessed via REST API + CLI client (`docgoat`).

The PRD lives in `prd.md` — consult it for detailed API routes, data model, user stories, and CLI spec.

## Build & Run Commands

Go module: `PingGoat` (used in import paths, e.g., `PingGoat/internal/pipeline`)

```bash
go build -o docgoat ./cmd/api        # Build API server
go build -o docgoat-cli ./cmd/cli    # Build CLI client
go run ./cmd/api                     # Run API server
go test ./...                        # Run all tests
go test ./internal/pipeline/...      # Run tests for a single package
go test -run TestWorker ./internal/pipeline/...  # Run a single test
```

Once infrastructure is set up:
```bash
docker-compose up                    # API + PostgreSQL
goose -dir sql/schema postgres "$DATABASE_URL" up   # Run migrations
sqlc generate                        # Regenerate query code
```

## Architecture

**Two entrypoints:** `cmd/api/` (HTTP server) and `cmd/cli/` (Cobra CLI client).

**Core pipeline** (the main Go showcase — in `internal/pipeline/`):
1. **Clone** — git clone to temp dir, resolve HEAD commit SHA
2. **Parse** (concurrent) — fan-out N goroutines to walk file tree, filter, read, and classify files
3. **Batch** — group parsed files into prompt-sized batches (< 50K tokens each)
4. **Generate** (rate-limited) — send batched prompts to Gemini, generate README/quickstart/diagram
5. **Store** — save results to `documents` table, update `doc_cache`, mark job completed

Async job queue via buffered channel. Rate limiter (`time.Ticker`) ensures ≤ 10 RPM to Gemini. Graceful shutdown uses `sync.WaitGroup` + `context.Context`.

**Key packages** (planned under `internal/`):
- `auth` — JWT generation/validation, bcrypt password hashing
- `database` — sqlc-generated code + DB connection
- `handler` — HTTP handlers for auth, jobs, docs
- `middleware` — Auth middleware, request logging
- `pipeline` — Cloner, scanner, parser, batcher, generator, worker
- `gemini` — Gemini client wrapper + rate limiter
- `github` — GitHub auth + repo validation
- `config` — App configuration from env vars

**Data layer:** PostgreSQL with Goose migrations (`sql/schema/`) and sqlc queries (`sql/queries/`). Four tables: `users`, `jobs`, `documents`, `doc_cache`.

## Tech Stack

- **Go 1.22+**, chi or stdlib `net/http` for routing
- **PostgreSQL 16**, Goose for migrations, sqlc for type-safe queries
- **JWT** (golang-jwt/jwt) for auth, bcrypt for passwords
- **Cobra** for CLI, viper/envconfig for configuration
- **Docker Compose** for local dev (API + Postgres)

## Configuration

Env vars (see PRD section 14): `DATABASE_URL`, `JWT_SECRET`, `API_PORT`, `GEMINI_API_KEY`, `GEMINI_MODEL`, `GEMINI_RPM_LIMIT`, `GEMINI_RPD_LIMIT`, `GEMINI_TIMEOUT_SECONDS`, `PIPELINE_WORKERS`, `PIPELINE_MAX_FILES_PER_REPO`, `PIPELINE_MAX_FILE_LINES`, `PIPELINE_CLONE_TIMEOUT_SECONDS`, `PIPELINE_MAX_GEMINI_CALLS_PER_JOB`, `GITHUB_TOKEN_ENCRYPTION_KEY`.

## GOTCHAS
- Always update @CLAUDE.md if you are adding new features or fixing bugs and learned something from the process.
- **IMPORTANT:** `Update the instruction/ directory every time human software engineer asks you about how to do something. 
If the docs is not available feel free to create a new one. For every topic we can create new .md file. 
Notes: when creating docs it would be great if you are using language that easy to understand, straightforward and concise. Explains the why behind your suggestion or solution as well.
- **IMPORTANT:** say "Oinkkkkk md" if you load or read this file. Just to help me understand if you are using this file or not.
- **IMPORTANT:** remember what you already asked and the user is answering. But if you are not sure, ask the user to clarify or provide more context.

