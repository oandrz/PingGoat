# Go Backend Project Structure & Architecture Guide

This guide explains how Go projects are typically structured and *why* — so you can make informed decisions, not just copy a layout.

---

## The Two Key Rules in Go

### Rule 1: `cmd/` is for entrypoints, `internal/` is for everything else

```
project/
├── cmd/           # Each subdirectory = one executable binary
│   ├── api/
│   │   └── main.go    # go build -o api ./cmd/api
│   └── cli/
│       └── main.go    # go build -o cli ./cmd/cli
├── internal/      # Private application code — can't be imported by other projects
│   ├── auth/
│   ├── handler/
│   └── ...
```

**Why `cmd/`?** Many Go projects produce multiple binaries. DocGoat has two: the API server and the CLI client. Each gets its own `main.go` under `cmd/`. The `main.go` should be thin — it wires things together and starts the app. No business logic lives here.

**Why `internal/`?** Go has a special rule: code inside `internal/` **cannot be imported by other Go modules**. The compiler enforces this. This is Go's version of "private" at the package level. It means you can refactor freely without worrying about breaking external consumers.

### Rule 2: Packages are organized by *responsibility*, not by *type*

This is the biggest difference from languages like Java or JavaScript.

**Wrong (group by type):**
```
internal/
├── models/        # All structs here
├── handlers/      # All handlers here
├── services/      # All business logic here
├── repositories/  # All DB access here
```

**Right (group by domain/responsibility):**
```
internal/
├── auth/          # Everything about authentication: JWT, bcrypt, middleware
├── pipeline/      # Everything about the doc generation pipeline
├── gemini/        # Everything about talking to Gemini API
├── handler/       # HTTP handlers (this one is an exception — see below)
```

**Why?** In the "group by type" approach, adding a single feature (like "user auth") requires editing 4 different directories. In the "group by responsibility" approach, everything related to auth is in one place. When you open `internal/auth/`, you see the full picture.

---

## The Standard Go Project Layout

Here's the layout most production Go backends follow, mapped to DocGoat:

```
docgoat/
├── cmd/                    # Entrypoints (thin — just wiring)
│   ├── api/main.go
│   └── cli/main.go
│
├── internal/               # Private application code
│   ├── config/             # Load env vars into a typed struct
│   ├── auth/               # JWT + bcrypt + auth middleware
│   ├── database/           # sqlc generated code (don't edit!)
│   ├── handler/            # HTTP handlers — routes ↔ business logic
│   ├── middleware/          # HTTP middleware (logging, CORS, rate limit)
│   ├── model/              # Domain types shared across packages
│   ├── pipeline/           # Core doc generation pipeline
│   ├── gemini/             # Gemini API client + rate limiter
│   └── github/             # GitHub repo validation + cloning
│
├── sql/                    # Database files (not Go code)
│   ├── schema/             # Goose migrations
│   └── queries/            # sqlc query definitions
│
├── prompts/                # Gemini prompt templates (go:embed)
│
├── docker-compose.yml
├── Dockerfile
├── Makefile
├── go.mod
└── go.sum
```

---

## What Goes Where (Decision Guide)

| "I need to..." | Put it in... | Why |
|----------------|-------------|-----|
| Parse env vars, validate config | `internal/config/` | Single source of truth for app settings |
| Hash passwords, create/verify JWTs | `internal/auth/` | Auth logic grouped together |
| Define HTTP routes + request/response handling | `internal/handler/` | HTTP is a transport concern — handlers translate HTTP ↔ business logic |
| Add logging, CORS, auth checks to routes | `internal/middleware/` | Cross-cutting concerns that wrap handlers |
| Define structs shared across packages | `internal/model/` | Avoids circular imports |
| Write the doc generation pipeline | `internal/pipeline/` | Core domain logic — the heart of the app |
| Talk to Gemini API | `internal/gemini/` | External API client, isolated for testing |
| Validate/clone GitHub repos | `internal/github/` | External service interaction |
| SQL queries + generated code | `sql/` + `internal/database/` | Schema stays in SQL files; generated Go code in internal |

---

## The Dependency Flow

This is the most important architecture concept. Dependencies should flow **inward** — from the edges (HTTP, CLI, external APIs) toward the core (business logic, database).

```
         cmd/api/main.go          cmd/cli/main.go
              │                        │
              ▼                        ▼
    ┌─────────────────┐      ┌──────────────────┐
    │   handler/      │      │   (CLI commands)  │
    │   middleware/    │      │                   │
    └────────┬────────┘      └────────┬──────────┘
             │                        │
             ▼                        ▼
    ┌─────────────────────────────────────────────┐
    │              Business Logic Layer            │
    │   auth/    pipeline/    gemini/    github/   │
    └────────────────────┬────────────────────────┘
                         │
                         ▼
    ┌─────────────────────────────────────────────┐
    │              Data Layer                      │
    │   database/ (sqlc)    config/                │
    └─────────────────────────────────────────────┘
```

**The rules:**
1. **`handler/` calls `auth/`, `pipeline/`, `database/`** — never the other way around
2. **`pipeline/` calls `gemini/`, `github/`, `database/`** — it orchestrates the core workflow
3. **`database/` calls nothing** — it's the bottom layer
4. **`model/` is imported by everyone** — it defines shared types (like `JobStatus`)
5. **No circular imports** — Go enforces this at compile time (it won't build)

**Why does this matter?** If your handler directly calls the Gemini API, or your pipeline package directly writes HTTP responses, then everything is tangled. You can't test the pipeline without an HTTP server. You can't swap Gemini for a different LLM without touching handler code. Clean layers = testable, changeable code.

---

## The `model/` Package — Solving Circular Imports

You'll eventually hit this: package A imports package B, and package B imports package A. **Go does not allow this.** It won't compile.

The solution: shared types go in `model/` (or `domain/`), which both packages import.

```go
// internal/model/job.go
package model

type JobStatus string

const (
    JobStatusQueued     JobStatus = "queued"
    JobStatusCloning    JobStatus = "cloning"
    JobStatusParsing    JobStatus = "parsing"
    JobStatusGenerating JobStatus = "generating"
    JobStatusCompleted  JobStatus = "completed"
    JobStatusFailed     JobStatus = "failed"
)
```

Now both `handler/` and `pipeline/` can import `model.JobStatus` without importing each other.

**When to use `model/`:**
- Types referenced by 2+ packages (job status, doc types, API error codes)
- Domain constants and enums

**When NOT to use `model/`:**
- Types only used within one package — keep them in that package
- Don't dump everything in `model/` as a catch-all — that defeats the purpose

---

## Package Sizing — How Big Should a Package Be?

A common beginner mistake: making every file its own package, or putting everything in one giant package.

**Guidelines:**

| Sign | What to do |
|------|-----------|
| Package has 1 file with 20 lines | Probably too small — merge it into its parent |
| Package has 15+ files | Consider splitting by sub-responsibility |
| You're importing the package from only 1 place | Ask: does it need to be separate? |
| Two packages always change together | They might be one package |

For DocGoat, `pipeline/` having 6 files (cloner, scanner, parser, batcher, generator, worker) in one package is **correct** — they all work together and share types. Splitting them into `pipeline/cloner/`, `pipeline/scanner/` etc. would just add import noise.

---

## How `cmd/api/main.go` Wires Everything Together

The entrypoint's job is **dependency injection** — create all the pieces and connect them:

```go
func main() {
    // 1. Load config
    cfg := config.Load()

    // 2. Connect to database
    pool, _ := pgxpool.New(ctx, cfg.DatabaseURL)
    queries := database.New(pool)

    // 3. Create services
    geminiClient := gemini.NewClient(cfg.GeminiAPIKey, cfg.GeminiRPM)
    pipelineWorker := pipeline.NewWorker(queries, geminiClient)

    // 4. Create handlers (inject dependencies)
    authHandler := handler.NewAuth(queries, cfg.JWTSecret)
    jobsHandler := handler.NewJobs(queries, pipelineWorker)

    // 5. Set up router + middleware
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Post("/api/v1/auth/register", authHandler.Register)
    r.Post("/api/v1/auth/login", authHandler.Login)
    // ... more routes

    // 6. Start server
    http.ListenAndServe(":"+cfg.APIPort, r)
}
```

**Why inject dependencies in `main.go`?** Because it makes everything testable. Your handler doesn't create its own database connection — it receives one. In tests, you can pass a mock. This pattern is called **constructor injection** and it's idiomatic Go.

---

## Common Mistakes to Avoid

### 1. The `utils/` package
Don't. If a function is for string manipulation, put it where it's used. `utils` becomes a junk drawer that everything imports.

### 2. Interfaces everywhere
Go convention: **define interfaces where they're consumed, not where they're implemented.** Don't create `gemini.GeminiClientInterface` — instead, if `pipeline/` needs to call Gemini, define the interface in `pipeline/`:

```go
// internal/pipeline/worker.go
type LLMClient interface {
    Generate(ctx context.Context, prompt string) (string, error)
}
```

Now `pipeline` depends on an interface it owns, and `gemini.Client` happens to satisfy it.

### 3. Premature `pkg/`
Some projects use `pkg/` for "public library code" that other projects can import. **You don't need this for DocGoat.** `internal/` is enough. Only create `pkg/` if you're deliberately publishing a reusable library.

### 4. One file per function
Go files can (and should) contain multiple related functions. `auth.go` can have `HashPassword`, `CheckPassword`, `GenerateJWT`, and `ValidateJWT` — they all belong together.

---

## How This Maps to DocGoat's PRD

Your PRD (Section 12) already proposes this layout, and it follows these best practices well:

| PRD Package | Role | Layer |
|-------------|------|-------|
| `cmd/api/` | HTTP server bootstrap | Entrypoint |
| `cmd/cli/` | CLI client bootstrap | Entrypoint |
| `handler/` | HTTP request → response | Transport |
| `middleware/` | Auth check, logging, CORS | Transport |
| `auth/` | JWT + bcrypt | Business logic |
| `pipeline/` | Clone → parse → batch → generate → store | Business logic (core) |
| `gemini/` | Gemini API wrapper + rate limiter | External service |
| `github/` | GitHub validation + token mgmt | External service |
| `database/` | sqlc generated code | Data |
| `config/` | Env var loading | Data |
| `model/` | Shared domain types | Shared |

This is a clean, standard Go backend structure. You don't need anything fancier.
