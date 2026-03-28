# DocGoat 🐐 — AI-Powered GitHub Documentation Generator

## Product Requirements Document (PRD)

**Project Type:** boot.dev Backend/Go Capstone Project
**Timeline:** Weekend (1–2 days)
**Author:** Squareen (PM hat on 🎩)
**LLM Backend:** Google Gemini 2.5 Flash (Free Tier)

---

## 1. Overview

DocGoat takes a GitHub repository URL and generates comprehensive documentation for it — a polished README, usage examples with a quickstart guide, and a Mermaid architecture diagram showing how the codebase works. It supports any programming language and both public and private repos (via GitHub personal access token).

The service clones the repo, intelligently selects and parses key files, batches them into Gemini Flash calls (respecting free-tier rate limits), and assembles the generated documentation. Results are stored in PostgreSQL for retrieval and regeneration.

Accessed via REST API + CLI client.

---

## 2. Goals & Success Criteria

### Learning Goals (Capstone Focus)
- **Concurrency:** Parallel file parsing, rate-limited AI worker pipeline with goroutines and channels
- **REST API Design:** Clean resource modeling, async job processing with polling
- **Database Design:** Normalized schema with migrations (Goose + sqlc)
- **Auth:** JWT-based authentication with secure password hashing
- **External API Integration:** GitHub API + Gemini API with rate limiting

### Definition of Done
- [ ] A user can register, log in, and submit a GitHub repo URL via the API
- [ ] The system clones the repo, parses files concurrently, and sends batched prompts to Gemini
- [ ] Generated documentation (README, quickstart, diagram) is stored in PostgreSQL
- [ ] Users can retrieve, list, and regenerate documentation for their repos
- [ ] A CLI client can perform all core operations
- [ ] Rate limiting ensures the service stays within Gemini free-tier limits
- [ ] The project runs locally with `docker-compose up`

---

## 3. Gemini Free Tier Constraints & Design Implications

### Hard Limits (Gemini 2.5 Flash Free Tier)
| Dimension | Limit |
|-----------|-------|
| Requests per Minute (RPM) | 10 |
| Requests per Day (RPD) | 250 |
| Tokens per Minute (TPM) | 250,000 |
| Context Window | 1,000,000 tokens |

### Design Decisions Driven by Limits

1. **Batch files into fewer, larger prompts** — instead of 1 file = 1 API call, group related files into a single prompt (e.g., all files from one package). This keeps RPD low.

2. **Async job processing** — documentation generation is not instant. The API returns a job ID immediately, and the client polls for completion. This decouples the user experience from the LLM processing time.

3. **Rate limiter** — a `time.Ticker`-based rate limiter ensures we never exceed 10 RPM. The worker sleeps between calls if needed.

4. **Caching** — generated docs are stored in the DB. Re-requesting docs for the same repo + commit SHA returns cached results without burning RPD.

5. **Smart file selection** — not every file in a repo needs to go to the LLM. We prioritize: entry points, exported functions, config files, existing docs. Skip: vendor, node_modules, test fixtures, binary files, lock files.

6. **Token budget per repo** — cap at ~3-5 Gemini calls per repo. With an average of 50K tokens per call, that's 150-250K tokens, well within TPM.

---

## 4. User Stories

### Authentication
| ID | Story | Priority |
|----|-------|----------|
| US-01 | As a new user, I can register with email and password | P0 |
| US-02 | As a registered user, I can log in and receive a JWT token | P0 |
| US-03 | As a logged-in user, my requests are authenticated via Bearer token | P0 |

### GitHub Configuration
| ID | Story | Priority |
|----|-------|----------|
| US-04 | As a user, I can optionally store a GitHub personal access token for private repos | P0 |
| US-05 | As a user, I can update or delete my stored GitHub token | P1 |

### Documentation Generation
| ID | Story | Priority |
|----|-------|----------|
| US-06 | As a user, I can submit a GitHub repo URL to generate documentation | P0 |
| US-07 | As a user, I receive a job ID immediately and can poll for status | P0 |
| US-08 | As a user, I can see the progress of my documentation job (cloning → parsing → generating → done) | P1 |
| US-09 | As a user, I can retrieve the generated documentation (README, quickstart, diagram) once complete | P0 |
| US-10 | As a user, I can list all my documentation jobs with their status | P0 |
| US-11 | As a user, I can regenerate documentation for a repo (force refresh, ignore cache) | P1 |
| US-12 | As a user, I can delete a documentation job and its results | P0 |

### Documentation Outputs
| ID | Story | Priority |
|----|-------|----------|
| US-13 | As a user, the generated README includes: project title, description, features, tech stack, installation, and project structure | P0 |
| US-14 | As a user, the generated quickstart includes: prerequisites, setup steps, and runnable code examples | P0 |
| US-15 | As a user, the generated diagram is a valid Mermaid diagram showing architecture/module relationships | P0 |
| US-16 | As a user, I can download each output as a standalone file (README.md, QUICKSTART.md, ARCHITECTURE.mermaid) | P1 |

### CLI
| ID | Story | Priority |
|----|-------|----------|
| US-17 | As a user, I can log in via CLI and store my token locally | P0 |
| US-18 | As a user, I can submit a repo URL and watch the job progress from the CLI | P0 |
| US-19 | As a user, I can list my jobs and view generated docs from the CLI | P0 |
| US-20 | As a user, I can configure my GitHub token via the CLI | P0 |

---

## 5. API Design

**Base URL:** `http://localhost:8080/api/v1`

### Auth
```
POST   /auth/register                     Register a new user
POST   /auth/login                        Log in, returns JWT
```

### GitHub Config (protected)
```
PUT    /settings/github-token             Store/update GitHub PAT (encrypted at rest)
DELETE /settings/github-token             Remove stored GitHub PAT
```

### Jobs (protected)
```
POST   /jobs                              Submit a repo URL for doc generation
GET    /jobs                              List all user's jobs (paginated)
GET    /jobs/{id}                         Get job details + status
DELETE /jobs/{id}                         Delete a job and its outputs
POST   /jobs/{id}/regenerate              Re-run generation (ignore cache)
```

### Documents (protected)
```
GET    /jobs/{id}/docs                    Get all generated docs for a job
GET    /jobs/{id}/docs/readme             Get the generated README
GET    /jobs/{id}/docs/quickstart         Get the generated quickstart guide
GET    /jobs/{id}/docs/diagram            Get the generated Mermaid diagram
GET    /jobs/{id}/docs/{type}/raw         Download as raw file (.md / .mermaid)
```

### System (public)
```
GET    /health                            Service health check
```

### Request/Response Formats

**Submit a Job:**
```json
// POST /jobs
{
  "repo_url": "https://github.com/user/repo",
  "branch": "main"                          // optional, defaults to default branch
}

// Response 202 Accepted
{
  "data": {
    "id": "job_abc123",
    "repo_url": "https://github.com/user/repo",
    "branch": "main",
    "status": "queued",
    "created_at": "2026-03-28T10:00:00Z"
  }
}
```

**Job Status Progression:**
```
queued → cloning → parsing → generating → completed
                                        → failed (with error message)
```

**Standard Envelope:**
```json
// Success
{
  "data": { ... },
  "meta": { "page": 1, "per_page": 20, "total": 5 }
}

// Error
{
  "error": {
    "code": "REPO_NOT_FOUND",
    "message": "Could not access repository. Check the URL and permissions."
  }
}
```

---

## 6. Data Model

### `users`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| email | VARCHAR(255) | UNIQUE, NOT NULL |
| password_hash | VARCHAR(255) | bcrypt |
| github_token_enc | TEXT | Encrypted GitHub PAT, nullable |
| created_at | TIMESTAMPTZ | DEFAULT now() |
| updated_at | TIMESTAMPTZ | DEFAULT now() |

### `jobs`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| user_id | UUID | FK → users, NOT NULL |
| repo_url | TEXT | NOT NULL |
| branch | VARCHAR(255) | DEFAULT 'main' |
| commit_sha | VARCHAR(40) | Resolved after clone |
| status | VARCHAR(20) | queued/cloning/parsing/generating/completed/failed |
| error_message | TEXT | NULL unless failed |
| file_count | INT | Number of files analyzed |
| gemini_calls_used | INT | Track API usage per job |
| started_at | TIMESTAMPTZ | NULL until processing begins |
| completed_at | TIMESTAMPTZ | NULL until done |
| created_at | TIMESTAMPTZ | DEFAULT now() |
| updated_at | TIMESTAMPTZ | DEFAULT now() |

**Indexes:** `(user_id, created_at DESC)`, `(status)` for worker polling

### `documents`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| job_id | UUID | FK → jobs, NOT NULL |
| doc_type | VARCHAR(20) | 'readme' / 'quickstart' / 'diagram' |
| content | TEXT | Generated markdown/mermaid content |
| prompt_tokens | INT | Tokens used in prompt |
| completion_tokens | INT | Tokens in response |
| created_at | TIMESTAMPTZ | DEFAULT now() |

**Indexes:** `(job_id, doc_type)` UNIQUE

### `doc_cache`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| repo_url | TEXT | NOT NULL |
| commit_sha | VARCHAR(40) | NOT NULL |
| doc_type | VARCHAR(20) | NOT NULL |
| content | TEXT | Cached content |
| created_at | TIMESTAMPTZ | DEFAULT now() |
| expires_at | TIMESTAMPTZ | DEFAULT now() + 7 days |

**Indexes:** `(repo_url, commit_sha, doc_type)` UNIQUE — for cache lookups

---

## 7. Core Pipeline Architecture

```
User submits repo URL
        │
        ▼
  ┌───────────┐
  │  REST API  │  Returns job_id immediately (202 Accepted)
  └─────┬─────┘
        │ sends Job to channel
        ▼
  ┌───────────┐
  │ Job Queue │  Buffered channel
  │ (channel) │
  └─────┬─────┘
        │
        ▼
  ┌─────────────┐
  │ Job Worker   │  Single goroutine (respects rate limits)
  │ (goroutine)  │  Processes jobs sequentially
  └─────┬───────┘
        │
        ▼
  ┌─── Pipeline Stages ───────────────────────────────────┐
  │                                                        │
  │  Stage 1: CLONE                                        │
  │  ┌──────────────┐                                      │
  │  │ Git Clone    │  Clone to temp dir                   │
  │  │ (exec.Command)│  Resolve HEAD commit SHA            │
  │  └──────┬───────┘                                      │
  │         │                                              │
  │  Stage 2: PARSE (concurrent)                           │
  │  ┌──────┴───────┐                                      │
  │  │ File Scanner │  Walk tree, filter relevant files    │
  │  └──────┬───────┘                                      │
  │         │ fan-out to N parser goroutines                │
  │  ┌──────┼──────┬──────────┐                            │
  │  ▼      ▼      ▼          ▼                            │
  │  📄     📄     📄         📄   Read + classify files   │
  │  Parser Parser Parser  Parser  (entry point, config,   │
  │  │      │      │        │       lib code, etc.)        │
  │  └──────┴──────┴────────┘                              │
  │         │ fan-in                                        │
  │         ▼                                              │
  │  ┌──────────────┐                                      │
  │  │ File Batcher │  Group files into prompt-sized       │
  │  │              │  batches (< 50K tokens each)         │
  │  └──────┬───────┘                                      │
  │         │                                              │
  │  Stage 3: GENERATE (rate-limited, sequential)          │
  │  ┌──────┴───────┐                                      │
  │  │ Gemini Caller│  Sends batched prompts               │
  │  │ + Rate Limiter│  Waits between calls (time.Ticker)  │
  │  │              │  Generates README, Quickstart, Diagram│
  │  └──────┬───────┘                                      │
  │         │                                              │
  │  Stage 4: STORE                                        │
  │  ┌──────┴───────┐                                      │
  │  │ Result Writer│  Saves to `documents` table          │
  │  │              │  Updates `doc_cache`                  │
  │  │              │  Updates job status → completed       │
  │  └──────────────┘                                      │
  └────────────────────────────────────────────────────────┘
```

### Key Concurrency Patterns

1. **Async Job Queue** — buffered channel decouples API requests from processing
2. **Fan-out File Parsing** — N goroutines parse files concurrently, fan-in via channel
3. **Rate-Limited Sequential AI Calls** — `time.Ticker` ensures ≤ 10 RPM to Gemini
4. **Context + WaitGroup** — graceful shutdown waits for in-flight jobs
5. **Cache Check** — before calling Gemini, check `doc_cache` for same repo+SHA

### Rate Limiter Design

```go
type RateLimiter struct {
    ticker *time.Ticker     // fires every 6 seconds (10 RPM)
    daily  atomic.Int32     // tracks daily usage
    limit  int32            // 250 RPD
}

func (r *RateLimiter) Wait(ctx context.Context) error {
    if r.daily.Load() >= r.limit {
        return ErrDailyLimitExceeded
    }
    select {
    case <-r.ticker.C:
        r.daily.Add(1)
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

---

## 8. File Selection & Batching Strategy

### Smart File Selection

Not every file in a repo should go to the LLM. DocGoat uses a priority-based file selector:

**Always Include (Tier 1 — high signal):**
- Entry points: `main.go`, `main.py`, `index.ts`, `app.py`, `server.go`, `cmd/*/main.go`
- Config: `Makefile`, `Dockerfile`, `docker-compose.yml`, `package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml`
- Existing docs: `README.md`, `CONTRIBUTING.md`, `docs/*.md`
- API definitions: `openapi.yaml`, `swagger.json`, `*.proto`

**Include if Budget Allows (Tier 2 — medium signal):**
- Exported package files (Go: files with exported functions; JS: index files)
- Route/handler files (common patterns: `routes.go`, `handler.go`, `controller.py`)
- Core domain/model files
- Database migrations (for schema understanding)

**Always Skip:**
- `vendor/`, `node_modules/`, `.git/`, `dist/`, `build/`
- Lock files: `go.sum`, `package-lock.json`, `yarn.lock`, `Cargo.lock`
- Binary files, images, fonts
- Test fixtures and snapshots
- Files > 500 lines (truncate to first 200 lines + last 50 lines)
- Generated code (e.g., `*.pb.go`, `*.generated.ts`)

### Batching Strategy

```
Total token budget per repo: ~200K tokens (across 3-5 Gemini calls)

Call 1: "Overview" prompt
  - go.mod / package.json / Cargo.toml (dependencies + language detection)
  - Entry points (main files)
  - Existing README (if any)
  - Config files (Dockerfile, Makefile)
  → Output: README.md

Call 2: "Deep dive" prompt
  - Handler / route files
  - Core domain / model files
  - Key library code
  → Output: Quickstart guide with usage examples

Call 3: "Architecture" prompt
  - All Tier 1 + Tier 2 file NAMES and first-line summaries
  - Package/module dependency graph (parsed from imports)
  - Directory structure
  → Output: Mermaid diagram (flowchart or C4-style)
```

---

## 9. Gemini Prompt Templates

### README Generation Prompt
```
You are a technical writer. Given the following source files from a GitHub
repository, generate a professional README.md.

Repository: {repo_url}
Language: {detected_language}
Framework: {detected_framework}

Include these sections:
1. Project Title + one-line description
2. Features (bullet list of key capabilities)
3. Tech Stack
4. Prerequisites
5. Installation
6. Configuration (environment variables, etc.)
7. Project Structure (directory tree with explanations)
8. Contributing (brief)
9. License (if detectable)

Source files:
---
{batched_file_contents}
---

Output ONLY valid Markdown. Do not include ```markdown fences.
```

### Quickstart Generation Prompt
```
You are a developer advocate writing a quickstart guide. Given the following
source files, create a practical guide that gets a new developer from zero
to running the project with a working example.

Repository: {repo_url}
Language: {detected_language}

Include:
1. Prerequisites (runtime versions, tools needed)
2. Step-by-step setup (clone, install deps, configure, run)
3. A working code example showing the primary use case
4. Common first tasks or API calls
5. Troubleshooting tips for common setup issues

Source files:
---
{batched_file_contents}
---

Output ONLY valid Markdown. Do not include ```markdown fences.
Use real code blocks with correct language tags.
```

### Architecture Diagram Prompt
```
You are a software architect. Given the following repository structure and
key source files, generate a Mermaid diagram showing how the system works.

Repository: {repo_url}
Language: {detected_language}

Directory structure:
{directory_tree}

File summaries:
{file_name_and_first_lines}

Key source files:
---
{batched_file_contents}
---

Generate a Mermaid diagram that shows:
- Main components/modules and their responsibilities
- Data flow between components
- External dependencies (databases, APIs, etc.)
- Entry points

Use `graph TD` or `flowchart TD` syntax.
Output ONLY the Mermaid diagram code. Do not include ```mermaid fences.
Keep it readable (max 20 nodes).
```

---

## 10. CLI Client Spec

**Binary name:** `docgoat`

**Config:** Stores API base URL, JWT token, and GitHub PAT in `~/.docgoat.yaml`

### Commands
```bash
# Auth
docgoat login                             # Prompts for email/password, stores JWT
docgoat register                          # Prompts for email/password

# GitHub
docgoat config github-token               # Prompts for PAT, sends to API
docgoat config github-token --remove      # Removes stored PAT

# Generate docs
docgoat generate <repo_url> [--branch main]  # Submits job, shows progress spinner
docgoat generate <repo_url> --force          # Regenerate (ignore cache)

# View results
docgoat jobs                              # List all jobs: ID | Repo | Status | Created
docgoat jobs <job_id>                     # Show job details
docgoat jobs <job_id> --readme            # Print generated README to stdout
docgoat jobs <job_id> --quickstart        # Print generated quickstart
docgoat jobs <job_id> --diagram           # Print generated Mermaid diagram
docgoat jobs <job_id> --save ./output/    # Save all docs to directory
docgoat jobs <job_id> --delete            # Delete job

# Status
docgoat status                            # Show API usage: daily Gemini calls remaining
```

### Example Session
```
$ docgoat generate https://github.com/gorilla/mux

🐐 DocGoat — Generating documentation...
  Repository: gorilla/mux
  Branch:     main
  Commit:     abc1234

  [1/4] Cloning repository...        ✅
  [2/4] Parsing 23 files...          ✅  (selected 12 key files)
  [3/4] Generating documentation...  ✅  (3 Gemini calls used)
  [4/4] Saving results...            ✅

✨ Documentation ready! Job ID: job_7f8a9b

  docgoat jobs job_7f8a9b --readme       View README
  docgoat jobs job_7f8a9b --quickstart   View quickstart
  docgoat jobs job_7f8a9b --diagram      View architecture diagram
  docgoat jobs job_7f8a9b --save ./docs  Save all files

$ docgoat jobs job_7f8a9b --diagram

flowchart TD
    A[HTTP Request] --> B[Router / mux.Router]
    B --> C{Route Matching}
    C --> D[Path Variables]
    C --> E[Method Matching]
    C --> F[Host Matching]
    D --> G[Handler Function]
    E --> G
    F --> G
    G --> H[HTTP Response]
    B --> I[Middleware Chain]
    I --> G
```

---

## 11. Tech Stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Language | Go 1.22+ | Capstone requirement |
| Router | chi or net/http mux | Lightweight, idiomatic |
| Database | PostgreSQL 16 | Relational, great text storage |
| Migrations | Goose | Familiar from boot.dev |
| Query Gen | sqlc | Familiar, type-safe |
| Auth | JWT (golang-jwt/jwt) | Stateless |
| Password Hash | bcrypt | Secure |
| LLM | Gemini 2.5 Flash | Free tier, fast, Go SDK |
| Gemini SDK | google.golang.org/genai | Official Go SDK (new) |
| Git Operations | exec.Command("git") | Simple, no extra deps |
| CLI Framework | cobra | Industry standard |
| Config | viper or envconfig | Env vars + config |
| Containerization | Docker + docker-compose | API + Postgres |

---

## 12. Project Structure

```
docgoat/
├── cmd/
│   ├── api/                    # API server entrypoint
│   │   └── main.go
│   └── cli/                    # CLI client entrypoint
│       └── main.go
├── internal/
│   ├── auth/                   # JWT, bcrypt, middleware
│   ├── config/                 # App configuration
│   ├── database/               # sqlc generated code + connection
│   ├── handler/                # HTTP handlers
│   │   ├── auth.go
│   │   ├── jobs.go
│   │   └── docs.go
│   ├── middleware/              # Auth, logging, CORS
│   ├── model/                  # Domain types
│   ├── pipeline/               # ⭐ Core pipeline
│   │   ├── cloner.go           # Git clone + SHA resolution
│   │   ├── scanner.go          # File walker + filter
│   │   ├── parser.go           # Concurrent file reader + classifier
│   │   ├── batcher.go          # Groups files into prompt batches
│   │   ├── generator.go        # Gemini caller + prompt builder
│   │   └── worker.go           # Job queue + orchestrator
│   ├── gemini/                 # Gemini client wrapper + rate limiter
│   │   ├── client.go
│   │   └── ratelimiter.go
│   └── github/                 # GitHub auth + repo validation
├── sql/
│   ├── schema/                 # Goose migration files
│   └── queries/                # sqlc query files
├── prompts/                    # Prompt templates (embedded via go:embed)
│   ├── readme.tmpl
│   ├── quickstart.tmpl
│   └── diagram.tmpl
├── docker-compose.yml
├── Dockerfile
├── .env.example
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## 13. Weekend Build Plan

### Day 1 — Foundation + Pipeline (Saturday)

| Block | Hours | Deliverable |
|-------|-------|-------------|
| Morning (1) | 1.5h | Project scaffold, Docker Compose, DB schema + Goose migrations, sqlc queries |
| Morning (2) | 1.5h | Auth: register/login handlers, JWT middleware, bcrypt |
| Afternoon (1) | 1.5h | Job CRUD handlers (submit, list, get, delete) + async job channel |
| Afternoon (2) | 2h | ⭐ Pipeline: cloner → scanner → parser (concurrent) → batcher |
| Evening | 1.5h | ⭐ Gemini client + rate limiter + generator (README prompt working end-to-end) |

**Day 1 Checkpoint:** User can register, submit a repo URL, and get a generated README back. The pipeline clones, parses concurrently, and calls Gemini with rate limiting.

### Day 2 — Complete Pipeline + CLI (Sunday)

| Block | Hours | Deliverable |
|-------|-------|-------------|
| Morning (1) | 1.5h | Add quickstart + diagram prompts, test with 2-3 different repos |
| Morning (2) | 1.5h | Cache layer (doc_cache table, skip Gemini if same SHA exists) |
| Afternoon (1) | 2h | CLI client: login, generate (with progress), view docs, save to files |
| Afternoon (2) | 1h | GitHub token support (encrypted storage, private repo cloning) |
| Evening | 1h | README, Makefile, cleanup, end-to-end manual test |

**Day 2 Checkpoint:** Full feature set. CLI can drive the entire flow. Caching works. Private repos supported.

---

## 14. Configuration

```env
# .env.example
DATABASE_URL=postgres://docgoat:docgoat@localhost:5432/docgoat?sslmode=disable
JWT_SECRET=change-me-to-a-real-secret
API_PORT=8080

# Gemini
GEMINI_API_KEY=your-gemini-api-key
GEMINI_MODEL=gemini-2.5-flash
GEMINI_RPM_LIMIT=10
GEMINI_RPD_LIMIT=250
GEMINI_TIMEOUT_SECONDS=60

# Pipeline
PIPELINE_WORKERS=1
PIPELINE_MAX_FILES_PER_REPO=50
PIPELINE_MAX_FILE_LINES=500
PIPELINE_CLONE_TIMEOUT_SECONDS=120
PIPELINE_MAX_GEMINI_CALLS_PER_JOB=5

# Encryption
GITHUB_TOKEN_ENCRYPTION_KEY=32-byte-hex-key-for-aes
```

---

## 15. Security Considerations

| Concern | Mitigation |
|---------|------------|
| GitHub PAT storage | AES-256-GCM encryption at rest in DB |
| Repo cloning | Clone to temp dir, auto-cleanup after processing, timeout limit |
| Malicious repos | Max file count (50), max file size, skip binaries, sandboxed temp dir |
| Prompt injection | Files are included as data in prompt, not as instructions. System prompt is fixed |
| Rate abuse | Per-user job limits (max 10 active jobs), auth required |
| Gemini API key | Server-side only, never exposed to client |

---

## 16. Error Handling Matrix

| Error | Status Code | User Message | Behavior |
|-------|-------------|-------------|----------|
| Invalid repo URL | 400 | "Invalid GitHub repository URL" | Reject immediately |
| Repo not found / no access | 404 | "Could not access repository" | Job marked failed |
| Clone timeout | 500 | "Repository too large or slow" | Job marked failed, temp dir cleaned |
| Gemini rate limit (429) | — | "Generation queued, retrying..." | Worker backs off, retries |
| Gemini daily limit | 429 | "Daily AI limit reached, try tomorrow" | Job stays queued, processed next day |
| No parseable files | 400 | "No supported source files found" | Job marked failed |
| Gemini returns garbage | — | "Generation failed, please retry" | Job marked failed after 2 retries |

---

## 17. Stretch Goals (Post-Weekend)

If you finish early or want to extend later:

- [ ] **Webhook on completion** — POST to a user-configured URL when docs are ready
- [ ] **GitHub Action** — auto-generate docs on push to main
- [ ] **Multiple output formats** — generate HTML docs, not just Markdown
- [ ] **Diff-aware regeneration** — only regenerate sections affected by changed files
- [ ] **Multi-language support** — detect framework-specific patterns (Express routes, FastAPI endpoints, etc.)
- [ ] **Web dashboard** — simple HTMX page to submit repos and view generated docs
- [ ] **Mermaid → SVG rendering** — convert diagram to SVG image for embedding in README
- [ ] **Cost tracking dashboard** — show daily Gemini usage with charts

---

## 18. Evaluation Criteria Mapping

| Criterion | How DocGoat Demonstrates It |
|-----------|------------------------------|
| **Concurrency** | Fan-out file parsing, rate-limited job queue, async processing pipeline |
| **REST API** | Async job submission (202), polling pattern, clean resource modeling |
| **Database** | 4-table schema with caching layer, indexes, migrations, sqlc |
| **Auth** | JWT + bcrypt, middleware, encrypted GitHub token storage |
| **External APIs** | GitHub (cloning, validation) + Gemini (rate-limited LLM calls) |
| **Error Handling** | Graceful degradation, retry logic, timeout management |
| **Project Quality** | Docker Compose, Makefile, CLI client, prompt templates, README |

---

## 19. Test Plan (Manual Weekend Checklist)

Test with these diverse repos to validate language detection and doc quality:

| Repo | Language | Why Test This |
|------|----------|---------------|
| `gorilla/mux` | Go | Well-structured Go library |
| `expressjs/express` | JS | Large, well-known Node framework |
| `tiangolo/fastapi` | Python | Python + framework detection |
| A small personal repo | Any | Edge case: few files, no docs |
| A monorepo (if time) | Mixed | Stress test file selection |