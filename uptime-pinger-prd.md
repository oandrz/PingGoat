# PingGoat 🐐 — Uptime Pinger Service

## Product Requirements Document (PRD)

**Project Type:** boot.dev Backend/Go Capstone Project
**Timeline:** Weekend (1–2 days)
**Author:** Squareen (PM hat on 🎩)

---

## 1. Overview

PingGoat is a lightweight uptime monitoring service built in Go. Users register HTTP endpoints they want to monitor, and PingGoat periodically pings them using a concurrent worker pool. It records response times, status codes, and uptime history — and exposes alerts when things go down.

The service is accessed via a REST API and a companion CLI client.

---

## 2. Goals & Success Criteria

### Learning Goals (Capstone Focus)
- **Concurrency:** Fan-out worker pool using goroutines, channels, and sync primitives
- **REST API Design:** Clean, RESTful resource modeling with proper status codes
- **Database Design:** Normalized schema with migrations (Goose + sqlc)
- **Auth:** JWT-based authentication with secure password hashing

### Definition of Done
- [ ] A user can register, log in, and manage monitored endpoints via the API
- [ ] A background worker pool pings all active endpoints at their configured intervals
- [ ] Check results (status, latency, timestamp) are stored in PostgreSQL
- [ ] Alerts are generated when an endpoint goes down or recovers
- [ ] A CLI client can perform all core operations
- [ ] The project runs locally with `docker-compose up` (API + PostgreSQL)

---

## 3. User Stories

### Authentication
| ID | Story | Priority |
|----|-------|----------|
| US-01 | As a new user, I can register with email and password | P0 |
| US-02 | As a registered user, I can log in and receive a JWT token | P0 |
| US-03 | As a logged-in user, my requests are authenticated via Bearer token | P0 |

### Endpoint Management
| ID | Story | Priority |
|----|-------|----------|
| US-04 | As a user, I can add a URL to monitor (with optional custom name and interval) | P0 |
| US-05 | As a user, I can list all my monitored endpoints with their current status | P0 |
| US-06 | As a user, I can update an endpoint's name, URL, or check interval | P0 |
| US-07 | As a user, I can pause/resume monitoring for an endpoint | P1 |
| US-08 | As a user, I can delete an endpoint | P0 |

### Check Results & Stats
| ID | Story | Priority |
|----|-------|----------|
| US-09 | As a user, I can view the check history for an endpoint (paginated) | P0 |
| US-10 | As a user, I can see uptime percentage for an endpoint (last 24h / 7d / 30d) | P1 |
| US-11 | As a user, I can see average response time for an endpoint | P1 |

### Alerts
| ID | Story | Priority |
|----|-------|----------|
| US-12 | As a user, I can view a list of all alerts (endpoint went DOWN or came back UP) | P0 |
| US-13 | As a user, I can filter alerts by endpoint or by status (down/recovered) | P1 |
| US-14 | As a user, I can mark an alert as acknowledged | P1 |

### CLI
| ID | Story | Priority |
|----|-------|----------|
| US-15 | As a user, I can log in via CLI and store my token locally | P0 |
| US-16 | As a user, I can add/list/delete endpoints from the CLI | P0 |
| US-17 | As a user, I can view endpoint status and recent checks from the CLI | P0 |

---

## 4. API Design

**Base URL:** `http://localhost:8080/api/v1`

### Auth
```
POST   /auth/register          Register a new user
POST   /auth/login             Log in, returns JWT
```

### Endpoints (protected)
```
POST   /endpoints              Create a monitored endpoint
GET    /endpoints              List all user's endpoints
GET    /endpoints/{id}         Get endpoint details + current status
PUT    /endpoints/{id}         Update endpoint config
DELETE /endpoints/{id}         Delete endpoint + its history
PATCH  /endpoints/{id}/pause   Pause monitoring
PATCH  /endpoints/{id}/resume  Resume monitoring
```

### Checks (protected)
```
GET    /endpoints/{id}/checks          Get check history (paginated)
GET    /endpoints/{id}/checks/stats    Get uptime % and avg response time
```

### Alerts (protected)
```
GET    /alerts                 List all alerts (filterable by endpoint_id, status)
PATCH  /alerts/{id}/ack       Acknowledge an alert
```

### System (public)
```
GET    /health                 Service health check
```

### Standard Response Envelope
```json
// Success
{
  "data": { ... },
  "meta": { "page": 1, "per_page": 20, "total": 42 }
}

// Error
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "url is required"
  }
}
```

---

## 5. Data Model

### `users`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| email | VARCHAR(255) | UNIQUE, NOT NULL |
| password_hash | VARCHAR(255) | bcrypt hash |
| created_at | TIMESTAMPTZ | DEFAULT now() |
| updated_at | TIMESTAMPTZ | DEFAULT now() |

### `endpoints`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| user_id | UUID | FK → users, NOT NULL |
| name | VARCHAR(255) | Optional friendly name |
| url | TEXT | NOT NULL, must be valid URL |
| interval_seconds | INT | DEFAULT 30, min 10, max 3600 |
| is_active | BOOLEAN | DEFAULT true (pause/resume) |
| last_status | VARCHAR(20) | 'unknown' / 'up' / 'down' |
| last_checked_at | TIMESTAMPTZ | NULL until first check |
| created_at | TIMESTAMPTZ | DEFAULT now() |
| updated_at | TIMESTAMPTZ | DEFAULT now() |

**Indexes:** `(user_id)`, `(is_active, last_checked_at)` for worker queries

### `checks`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| endpoint_id | UUID | FK → endpoints, NOT NULL |
| status_code | INT | NULL if request failed (timeout/DNS) |
| response_time_ms | INT | Latency in milliseconds |
| is_up | BOOLEAN | true if 2xx, false otherwise |
| error_message | TEXT | NULL if successful |
| created_at | TIMESTAMPTZ | DEFAULT now() |

**Indexes:** `(endpoint_id, created_at DESC)` for history queries

### `alerts`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| endpoint_id | UUID | FK → endpoints, NOT NULL |
| user_id | UUID | FK → users, NOT NULL (denormalized for query speed) |
| type | VARCHAR(20) | 'down' or 'recovered' |
| message | TEXT | e.g., "https://example.com returned 503" |
| is_acknowledged | BOOLEAN | DEFAULT false |
| created_at | TIMESTAMPTZ | DEFAULT now() |

**Indexes:** `(user_id, created_at DESC)`, `(endpoint_id)`

---

## 6. Concurrency Architecture

This is the beating heart of the project — the part that makes it a Go showcase.

### Worker Pool Design

```
┌─────────────┐
│   Scheduler  │  Runs on a tick (e.g., every 5s)
│   (1 goroutine)│  Queries DB for endpoints due for a check
└──────┬──────┘
       │ sends Endpoint structs
       ▼
   ┌────────┐
   │ Jobs   │  Buffered channel (capacity = N)
   │ Channel│
   └───┬────┘
       │ fan-out
       ▼
┌──────────┐ ┌──────────┐ ┌──────────┐
│ Worker 1 │ │ Worker 2 │ │ Worker N │   N configurable workers
│(goroutine)│ │(goroutine)│ │(goroutine)│   Each pings 1 endpoint
└─────┬────┘ └─────┬────┘ └─────┬────┘
      │            │            │  sends CheckResult structs
      ▼            ▼            ▼
   ┌─────────┐
   │ Results │  Buffered channel
   │ Channel │
   └────┬────┘
        │
        ▼
┌──────────────┐
│ Result Writer │  1 goroutine
│ (batch insert)│  Batches results, writes to DB
│               │  Generates alerts on status change
└──────────────┘
```

### Key Concurrency Patterns Used
1. **Fan-out/Fan-in:** Scheduler → Workers → Result Writer
2. **Buffered Channels:** Backpressure control between stages
3. **sync.WaitGroup:** Graceful shutdown — wait for in-flight checks to finish
4. **context.Context:** Timeout per HTTP check (default 10s), plus shutdown signal
5. **Ticker:** Scheduler runs on a `time.Ticker` to scan for due endpoints

### Scheduler Logic (Pseudocode)
```
every 5 seconds:
  query endpoints WHERE is_active = true
    AND (last_checked_at IS NULL
         OR last_checked_at + interval_seconds < now())
  for each endpoint:
    send to jobs channel (non-blocking, skip if full)
```

### Alert Generation Logic
```
on each check result:
  if endpoint.last_status == 'up' AND result.is_up == false:
    create alert(type='down')
    update endpoint.last_status = 'down'
  if endpoint.last_status == 'down' AND result.is_up == true:
    create alert(type='recovered')
    update endpoint.last_status = 'up'
```

---

## 7. CLI Client Spec

**Binary name:** `pinggoat`

**Config:** Stores API base URL and JWT token in `~/.pinggoat.yaml`

### Commands
```bash
# Auth
pinggoat login                          # Prompts for email/password, stores token
pinggoat register                       # Prompts for email/password

# Endpoints
pinggoat add <url> [--name "My API"] [--interval 60]
pinggoat list                           # Table: ID | Name | URL | Status | Last Check
pinggoat status <endpoint_id>           # Detailed view with recent checks
pinggoat delete <endpoint_id>
pinggoat pause <endpoint_id>
pinggoat resume <endpoint_id>

# Alerts
pinggoat alerts                         # List recent alerts
pinggoat alerts --endpoint <id>         # Filter by endpoint
pinggoat alerts ack <alert_id>          # Acknowledge alert
```

### Example Output
```
$ pinggoat list
┌──────────┬────────────┬─────────────────────────┬────────┬─────────────────┐
│ ID       │ Name       │ URL                     │ Status │ Last Check      │
├──────────┼────────────┼─────────────────────────┼────────┼─────────────────┤
│ a1b2c3   │ My API     │ https://api.example.com │ ✅ UP  │ 12 seconds ago  │
│ d4e5f6   │ Blog       │ https://blog.example.com│ 🔴 DOWN│ 8 seconds ago   │
│ g7h8i9   │ Docs       │ https://docs.example.com│ ✅ UP  │ 25 seconds ago  │
└──────────┴────────────┴─────────────────────────┴────────┴─────────────────┘
```

---

## 8. Tech Stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Language | Go 1.22+ | Capstone requirement |
| Router | chi or standard `net/http` mux | Lightweight, idiomatic |
| Database | PostgreSQL 16 | Relational data, good time-series query support |
| Migrations | Goose | Already familiar from boot.dev |
| Query Gen | sqlc | Already familiar, type-safe SQL |
| Auth | JWT (golang-jwt/jwt) | Stateless auth, industry standard |
| Password Hash | bcrypt | Secure, simple |
| HTTP Client | `net/http` (stdlib) | No external deps needed for pinging |
| CLI Framework | cobra | Industry standard Go CLI framework |
| Config | viper or envconfig | Env vars + config file |
| Containerization | Docker + docker-compose | API + Postgres in one command |

---

## 9. Project Structure

```
pinggoat/
├── cmd/
│   ├── api/                 # API server entrypoint
│   │   └── main.go
│   └── cli/                 # CLI client entrypoint
│       └── main.go
├── internal/
│   ├── auth/                # JWT generation/validation, password hashing
│   ├── database/            # sqlc generated code + connection
│   ├── handler/             # HTTP handlers (endpoints, checks, alerts, auth)
│   ├── middleware/           # Auth middleware, request logging
│   ├── model/               # Domain types (if needed beyond sqlc)
│   ├── pinger/              # ⭐ Worker pool, scheduler, HTTP checker
│   └── config/              # App configuration
├── sql/
│   ├── schema/              # Goose migration files
│   └── queries/             # sqlc query files
├── docker-compose.yml
├── Dockerfile
├── .env.example
├── go.mod
├── go.sum
├── Makefile                 # dev commands: migrate, generate, run, test
└── README.md
```

---

## 10. Weekend Build Plan

### Day 1 — Foundation + Core (Saturday)

| Block | Hours | Deliverable |
|-------|-------|-------------|
| Morning (1) | 1.5h | Project scaffold, Docker Compose (Postgres), DB schema + Goose migrations, sqlc queries |
| Morning (2) | 1.5h | Auth: register/login handlers, JWT middleware, bcrypt |
| Afternoon (1) | 1.5h | Endpoint CRUD handlers + tests |
| Afternoon (2) | 2h | ⭐ Pinger: worker pool, scheduler, result writer with channels |
| Evening | 1h | Integration test: spin up, add endpoint, verify checks are recorded |

**Day 1 Checkpoint:** API runs, user can register/login, add endpoints, and the worker pool is pinging them and writing results.

### Day 2 — Features + CLI (Sunday)

| Block | Hours | Deliverable |
|-------|-------|-------------|
| Morning (1) | 1.5h | Check history endpoint (paginated), stats endpoint (uptime %, avg latency) |
| Morning (2) | 1.5h | Alert generation logic (status change detection), alerts endpoint |
| Afternoon (1) | 2h | CLI client: login, add, list, status, alerts commands |
| Afternoon (2) | 1h | Pause/resume, acknowledge alerts |
| Evening | 1h | README, final cleanup, Makefile polish, manual end-to-end test |

**Day 2 Checkpoint:** Full feature set working. CLI can drive the whole flow. README documents setup and usage.

---

## 11. Configuration

```env
# .env.example
DATABASE_URL=postgres://pinggoat:pinggoat@localhost:5432/pinggoat?sslmode=disable
JWT_SECRET=change-me-to-a-real-secret
API_PORT=8080
PINGER_WORKERS=10
PINGER_SCAN_INTERVAL_SECONDS=5
PINGER_DEFAULT_CHECK_INTERVAL_SECONDS=30
PINGER_HTTP_TIMEOUT_SECONDS=10
```

---

## 12. Stretch Goals (Post-Weekend)

If you finish early or want to extend later:

- [ ] **Response body assertions** — Check that the response contains expected string/JSON key
- [ ] **Webhook notifications** — POST to a user-configured URL on status change
- [ ] **Dashboard** — Simple HTML/HTMX page showing live status
- [ ] **Multi-region pinging** — Simulate by running multiple pinger instances
- [ ] **SSL certificate expiry monitoring** — Check TLS cert expiry dates
- [ ] **Prometheus metrics** — Expose `/metrics` for Grafana dashboards

---

## 13. Evaluation Criteria Mapping

How this project maps to typical capstone rubrics:

| Criterion | How PingGoat Demonstrates It |
|-----------|------------------------------|
| **Concurrency** | Worker pool with goroutines + channels is the core feature |
| **REST API** | Clean resource modeling, proper HTTP verbs/status codes, pagination |
| **Database** | 4-table normalized schema, indexes, migrations, sqlc |
| **Auth** | JWT + bcrypt, middleware pattern |
| **Error Handling** | Graceful timeouts, failed checks don't crash workers |
| **Project Quality** | Docker Compose, Makefile, structured project layout, README |
