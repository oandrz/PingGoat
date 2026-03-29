# Docker & Docker Compose Guide for PingGoat

## Part 1: Core Concepts

### What is Docker?
Docker lets you package your app and everything it needs (Go runtime, dependencies, config) into a **container** — a lightweight, isolated environment that runs the same on any machine.

Think of it like shipping a package: instead of telling someone "install Go 1.22, set up Postgres 16, configure these env vars...", you hand them a container that already has everything inside.

**Key terms:**
- **Image** — A blueprint/recipe. Like a class in OOP. You build it once from a `Dockerfile`.
- **Container** — A running instance of an image. Like an object created from a class. You can run multiple containers from one image.
- **Dockerfile** — The recipe file that tells Docker how to build your image.

### What is Docker Compose?
PingGoat needs **two services** to run: your Go API server + a PostgreSQL database. Docker Compose lets you define and run **multiple containers together** with a single command (`docker-compose up`).

Without Compose, you'd have to:
1. Start Postgres manually
2. Figure out networking between containers
3. Start your Go app and point it at Postgres

With Compose, you write one `docker-compose.yml` file that describes both services and their relationship, and `docker-compose up` handles all of it.

---

## Part 2: The Dockerfile (for your Go API)

The Dockerfile goes in your project root. It tells Docker how to build your Go app.

### Structure of a Go Dockerfile

```dockerfile
# Stage 1: Build the Go binary
FROM golang:1.22-alpine AS builder

# Set working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum first (for caching dependencies)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary — CGO_ENABLED=0 makes a static binary (no C dependencies)
RUN CGO_ENABLED=0 GOOS=linux go build -o pinggoat ./cmd/api

# Stage 2: Run (tiny image, just the binary)
FROM alpine:latest

WORKDIR /app

# Copy ONLY the built binary from stage 1
COPY --from=builder /app/pinggoat .

# Tell Docker which port the app listens on
EXPOSE 8080

# Run the binary
CMD ["./pinggoat"]
```

### Why Two Stages? (Multi-stage build)
- **Stage 1 (builder):** Has the full Go toolchain (~300MB). Compiles your code.
- **Stage 2 (runtime):** Just Alpine Linux (~5MB) + your binary. No Go toolchain needed at runtime.
- Result: Your final image is ~10-15MB instead of ~300MB+.

### Key Commands Explained
| Command | What it does |
|---------|-------------|
| `FROM` | Base image to start from |
| `WORKDIR` | Sets the "current directory" inside the container |
| `COPY` | Copies files from your machine into the container |
| `RUN` | Executes a command during build (e.g., compile code) |
| `EXPOSE` | Documents which port the app uses (doesn't actually open it) |
| `CMD` | The command to run when the container starts |

### Why copy go.mod/go.sum before the rest?
Docker caches each step (layer). If your source code changes but `go.mod` hasn't, Docker skips the `go mod download` step and uses the cached dependencies. This makes rebuilds much faster.

---

## Part 3: The docker-compose.yml

This file goes in your project root alongside the Dockerfile.

### Structure

```yaml
services:
  # --- PostgreSQL Database ---
  db:
    image: postgres:16-alpine     # Use official Postgres image
    environment:
      POSTGRES_USER: pinggoat
      POSTGRES_PASSWORD: pinggoat
      POSTGRES_DB: pinggoat
    ports:
      - "5432:5432"               # host:container — lets you connect from your Mac
    volumes:
      - pgdata:/var/lib/postgresql/data   # Persist data across restarts
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U pinggoat"]
      interval: 2s
      timeout: 5s
      retries: 5

  # --- PingGoat API Server ---
  api:
    build: .                      # Build from the Dockerfile in current directory
    ports:
      - "8080:8080"               # host:container
    environment:
      DATABASE_URL: postgres://pinggoat:pinggoat@db:5432/pinggoat?sslmode=disable
      JWT_SECRET: dev-secret-change-in-prod
      API_PORT: "8080"
      PINGER_WORKERS: "4"
      PINGER_SCAN_INTERVAL_SECONDS: "5"
      PINGER_DEFAULT_CHECK_INTERVAL_SECONDS: "60"
      PINGER_HTTP_TIMEOUT_SECONDS: "10"
    depends_on:
      db:
        condition: service_healthy  # Wait for Postgres to be READY, not just started

# Named volume — Docker manages the storage location
volumes:
  pgdata:
```

### Key Concepts Explained

**services:** Each service = one container. You have two: `db` and `api`.

**image vs build:**
- `image: postgres:16-alpine` — Pull a pre-built image from Docker Hub.
- `build: .` — Build an image from your local Dockerfile.

**environment:** Sets environment variables inside the container. Your Go app reads these with `os.Getenv()`.

**ports: "8080:8080":** Maps `host_port:container_port`. The left side is what you use on your Mac (`localhost:8080`). The right side is what the app listens on inside the container.

**healthcheck + depends_on (why both matter):**

The problem: Postgres takes a few seconds to start up. `depends_on` alone only waits for the container to **start**, not for Postgres to be **ready**. Without a health check, this happens:

```
Without healthcheck:
  0.0s  db container starts
  0.1s  api container starts         <-- Postgres isn't ready yet!
  0.1s  api tries to connect to DB   <-- "connection refused" → crash
  2.0s  Postgres finally ready       <-- too late, api already crashed

With healthcheck + condition:
  0.0s  db container starts
  2.0s  health check: pg_isready → not ready yet
  4.0s  health check: pg_isready → ready!
  4.0s  api container starts         <-- Postgres is ready now
  4.1s  api connects to DB           <-- works!
```

`healthcheck` tells Docker **how** to check if a service is ready. `pg_isready` is a built-in Postgres tool that checks if Postgres can accept connections. The config means: check every 2s, allow up to 5s per check, try up to 5 times before giving up.

`depends_on` with `condition: service_healthy` tells Docker to **wait** until that health check passes before starting the dependent service.

**volumes (pgdata):** Without this, your database data disappears when you stop the container. Named volumes persist data on disk.

**DATABASE_URL uses `db` not `localhost`:** Inside Docker Compose, services talk to each other by service name. The `api` container reaches Postgres at `db:5432`, not `localhost:5432`.

---

## Part 4: Essential Commands

```bash
# Start everything (add -d for background/detached mode)
docker-compose up

# Start in background
docker-compose up -d

# Rebuild after code changes
docker-compose up --build

# Stop everything
docker-compose down

# Stop and DELETE all data (including database volumes)
docker-compose down -v

# View logs
docker-compose logs api
docker-compose logs db

# Follow logs in real-time
docker-compose logs -f api

# Run a one-off command in a service container
docker-compose exec db psql -U pinggoat -d pinggoat

# Check status of services
docker-compose ps
```

---

## Part 5: Common Gotchas

### 1. "Connection refused" on startup
Your API starts before Postgres is ready to accept connections. Solutions:
- Add retry logic in your Go code when connecting to the database (recommended)
- Use a wait script like `wait-for-it.sh`

### 2. Code changes not reflected
You need `docker-compose up --build` to rebuild the Go binary. Without `--build`, it reuses the old image.

### 3. Port already in use
If you have Postgres running locally on 5432, it conflicts. Either stop local Postgres or change the host port: `"5433:5432"`.

### 4. Can't connect to DB from your Mac
Use `localhost:5432` from your Mac (the host port). Use `db:5432` from the API container (Docker internal networking).

---

## Part 6: .dockerignore

Create a `.dockerignore` file in your project root to prevent unnecessary files from being copied into the container:

```
.git
.idea
*.iml
.env
docker-compose.yml
README.md
prd.md
instruction/
```

This keeps your Docker build context small and fast.

---

## Part 7: Running Migrations

Once both services are running, you can run Goose migrations:

```bash
# Option A: Run from your Mac (connecting to exposed port)
goose -dir sql/schema postgres "postgres://pinggoat:pinggoat@localhost:5432/pinggoat?sslmode=disable" up

# Option B: Add a migration step in docker-compose (more advanced)
# You'd create a separate service that runs migrations and exits
```

---

## FAQ

### Why does `CGO_ENABLED=0` matter?
By default (`CGO_ENABLED=1`), Go may link to C libraries on the system (e.g., for DNS or SSL). The binary then depends on those C libs being present at runtime.

With `CGO_ENABLED=0`, Go uses pure Go implementations for everything. The binary is completely self-contained — no external dependencies. This matters because the runtime stage (Alpine) doesn't have those C libraries installed, so without this flag your binary would crash.

`GOOS=linux` in the same line tells Go to compile for Linux, even if you're building on a Mac. Containers run Linux inside.

### Why do we need two stages in the Dockerfile? Can't we just build and run in one?
You can, but it's wasteful. Here's the difference:

**Single stage:**
```
golang:1.22-alpine (~300MB) + your source code + module cache + your binary
= ~300MB+ final image
```

**Multi-stage (what we use):**
```
Stage 1 (builder): Build the binary here, then throw this whole stage away.
Stage 2 (runtime):  alpine:latest (~5MB) + your binary (~15MB) = ~20MB final image
```

Think of it like building a house: Stage 1 is the construction site with all the tools and scaffolding. Stage 2 is the finished house — you don't leave the cement mixer inside when people move in.

**Why it matters:**
- **Faster deploys** — smaller images push/pull faster
- **Smaller attack surface** — no compiler or source code a hacker could exploit
- **Less disk usage** — especially when running many containers

---

## Implementation Order

1. Create `cmd/api/main.go` with a basic HTTP server first
2. Create the `Dockerfile`
3. Create `docker-compose.yml`
4. Create `.dockerignore`
5. Run `docker-compose up --build` and verify both services start
6. Add database connection logic to your Go app
7. Add migrations
