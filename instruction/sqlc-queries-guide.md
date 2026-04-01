# sqlc Queries & Code Generation Guide

This guide walks you through writing SQL queries and using sqlc to generate type-safe Go code for DocGoat. It picks up where the [Goose Migrations Guide](goose-migrations-guide.md) left off — your schema tables should already exist.

---

## What is sqlc and Why Use It?

**sqlc** is a compiler that reads your SQL queries and generates type-safe Go code from them. Instead of writing boilerplate like `rows.Scan(&user.ID, &user.Email, ...)`, sqlc generates that for you.

**The workflow is:**
1. You write plain SQL queries in `.sql` files
2. You run `sqlc generate`
3. sqlc reads your schema + queries, then outputs Go structs and functions
4. You call those generated functions from your handler/service code

**Why sqlc instead of an ORM (like GORM)?**
- You write real SQL — no magic, no surprises, no "what query did the ORM actually run?"
- The generated code is plain Go — you can read it, debug it, step through it
- Compile-time safety: if your query references a column that doesn't exist, `sqlc generate` fails immediately (not at runtime)
- It works perfectly with Goose migrations — sqlc reads the same `.sql` schema files

---

## Step 1: Install sqlc

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

Verify it's installed:
```bash
sqlc version
```

> **If `sqlc` is not found:** Same fix as Goose — make sure `$GOPATH/bin` is in your `$PATH`:
> ```bash
> export PATH="$PATH:$(go env GOPATH)/bin"
> ```

---

## Step 2: Create the sqlc Configuration File

Create `sqlc.yaml` in the project root:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "sql/queries/"
    schema: "sql/schema/"
    gen:
      go:
        package: "database"
        out: "internal/database"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_empty_slices: true
```

**What each field means:**

| Field | Value | Why |
|-------|-------|-----|
| `engine` | `"postgresql"` | We're using Postgres |
| `queries` | `"sql/queries/"` | Where sqlc looks for your `.sql` query files |
| `schema` | `"sql/schema/"` | Where sqlc looks for your table definitions (your Goose migration files!) |
| `gen.go.package` | `"database"` | The Go package name for generated code |
| `gen.go.out` | `"internal/database"` | Where generated `.go` files land |
| `sql_package` | `"pgx/v5"` | Use `pgx` (the modern Postgres driver for Go) instead of `database/sql` |
| `emit_json_tags` | `true` | Adds `json:"field_name"` tags to generated structs — useful when you serialize to JSON in API responses |
| `emit_empty_slices` | `true` | Returns `[]` instead of `null` for empty lists in JSON — cleaner API responses |

**Why `pgx/v5` instead of `database/sql`?** `pgx` is the Go community's preferred PostgreSQL driver. It's faster, supports Postgres-specific types natively (like `UUID`, `TIMESTAMPTZ`, `JSONB`), and has better connection pooling. `database/sql` is the generic Go interface — it works, but you'd need extra conversion code for Postgres types.

---

## Step 3: Create the Queries Directory

```bash
mkdir -p sql/queries
```

Your project structure now looks like:
```
sql/
├── schema/          # Goose migration files (already done)
│   ├── 20260331..._create_users.sql
│   ├── 20260331..._create_jobs.sql
│   ├── 20260331..._create_documents.sql
│   └── 20260331..._create_doc_cache.sql
└── queries/         # sqlc query files (this step)
    ├── users.sql
    ├── jobs.sql
    ├── documents.sql
    └── doc_cache.sql
```

---

## Step 4: Understand Query Annotations

Every sqlc query needs a special comment that tells sqlc:
1. **The function name** to generate
2. **The return type** — how many rows to expect

The format is:
```sql
-- name: FunctionName :annotation
```

Here are the annotations you'll use:

| Annotation | When to use | Generated return type |
|------------|------------|----------------------|
| `:one` | SELECT/INSERT that returns exactly 1 row | `(Model, error)` |
| `:many` | SELECT that returns multiple rows | `([]Model, error)` |
| `:exec` | INSERT/UPDATE/DELETE that returns nothing | `error` |
| `:execrows` | DELETE/UPDATE where you need the affected row count | `(int64, error)` |

**The key rule:** Use `RETURNING *` with `:one` when you want the inserted/updated row back. This is common for CREATE operations where you need the generated `id` and timestamps.

---

## Step 5: Write the Query Files

### `sql/queries/users.sql` — User Authentication Queries

```sql
-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: UpdateGithubToken :exec
UPDATE users
SET github_token_enc = $2, updated_at = now()
WHERE id = $1;

-- name: DeleteGithubToken :exec
UPDATE users
SET github_token_enc = NULL, updated_at = now()
WHERE id = $1;
```

**Why `$1`, `$2` instead of `?`?** PostgreSQL uses numbered placeholders (`$1`, `$2`, `$3`). MySQL/SQLite use `?`. Since we set `engine: "postgresql"` in sqlc.yaml, we use the `$N` style.

**Why `RETURNING *` on CreateUser?** After inserting, we need the generated `id`, `created_at`, and `updated_at` back. Without `RETURNING`, we'd have to do a separate SELECT — wasteful and racy.

### `sql/queries/jobs.sql` — Job Management Queries

```sql
-- name: CreateJob :one
INSERT INTO jobs (user_id, repo_url, branch)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetJobByID :one
SELECT * FROM jobs
WHERE id = $1;

-- name: ListJobsByUser :many
SELECT * FROM jobs
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateJobStatus :exec
UPDATE jobs
SET status = $2, updated_at = now()
WHERE id = $1;

-- name: UpdateJobStarted :exec
UPDATE jobs
SET status = 'cloning', started_at = now(), updated_at = now()
WHERE id = $1;

-- name: UpdateJobCompleted :exec
UPDATE jobs
SET status = 'completed',
    completed_at = now(),
    file_count = $2,
    gemini_calls_used = $3,
    updated_at = now()
WHERE id = $1;

-- name: UpdateJobFailed :exec
UPDATE jobs
SET status = 'failed',
    error_message = $2,
    completed_at = now(),
    updated_at = now()
WHERE id = $1;

-- name: UpdateJobCommitSHA :exec
UPDATE jobs
SET commit_sha = $2, updated_at = now()
WHERE id = $1;

-- name: DeleteJob :execrows
DELETE FROM jobs
WHERE id = $1 AND user_id = $2;

-- name: GetNextQueuedJob :one
SELECT * FROM jobs
WHERE status = 'queued'
ORDER BY created_at ASC
LIMIT 1;
```

**Why `LIMIT $2 OFFSET $3` on ListJobsByUser?** This is pagination. The PRD specifies paginated job listing (`meta.page`, `meta.per_page`). LIMIT controls how many rows to return, OFFSET skips rows for previous pages.

**Why `DeleteJob` checks `user_id` too?** Security — a user should only be able to delete their own jobs. Without `AND user_id = $2`, any authenticated user could delete anyone's job by guessing the UUID. This is called an **authorization check at the query level**.

**Why `GetNextQueuedJob` uses `ORDER BY created_at ASC`?** FIFO (first in, first out) — the oldest queued job gets processed first. This is fair: users who submitted earlier get served first.

**Why separate `UpdateJobStarted`, `UpdateJobCompleted`, `UpdateJobFailed`?** Instead of one generic "update everything" query, these are purpose-built for each pipeline stage transition. This prevents bugs: you can't accidentally mark a job as "completed" without also setting `completed_at`.

### `sql/queries/documents.sql` — Generated Documentation Queries

```sql
-- name: CreateDocument :one
INSERT INTO documents (job_id, doc_type, content, prompt_tokens, completion_tokens)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetDocumentsByJobID :many
SELECT * FROM documents
WHERE job_id = $1;

-- name: GetDocumentByType :one
SELECT * FROM documents
WHERE job_id = $1 AND doc_type = $2;

-- name: DeleteDocumentsByJobID :exec
DELETE FROM documents
WHERE job_id = $1;
```

**Why no UPDATE query for documents?** Documents are immutable — once generated, they don't change. If you regenerate, the old documents get deleted (via `DeleteDocumentsByJobID`) and new ones are created. This is simpler than updating in-place and avoids partial update bugs.

### `sql/queries/doc_cache.sql` — Cache Lookup Queries

```sql
-- name: GetCachedDoc :one
SELECT * FROM doc_cache
WHERE repo_url = $1
  AND commit_sha = $2
  AND doc_type = $3
  AND expires_at > now();

-- name: UpsertCachedDoc :one
INSERT INTO doc_cache (repo_url, commit_sha, doc_type, content)
VALUES ($1, $2, $3, $4)
ON CONFLICT (repo_url, commit_sha, doc_type)
DO UPDATE SET content = EXCLUDED.content,
             created_at = now(),
             expires_at = now() + INTERVAL '7 days'
RETURNING *;

-- name: DeleteExpiredCache :execrows
DELETE FROM doc_cache
WHERE expires_at <= now();
```

**Why `AND expires_at > now()` on GetCachedDoc?** Cache entries expire after 7 days. Without this check, you'd return stale docs for repos that have been updated.

**Why `ON CONFLICT ... DO UPDATE` (upsert) on UpsertCachedDoc?** The unique index on `(repo_url, commit_sha, doc_type)` means the same cache entry might already exist. Instead of checking with a SELECT first and then deciding INSERT vs UPDATE (which has a race condition!), `ON CONFLICT` does it atomically in one query.

**What does `EXCLUDED.content` mean?** In an `ON CONFLICT DO UPDATE`, `EXCLUDED` refers to the row that *would have been* inserted. So `EXCLUDED.content` means "use the new content we were trying to insert."

**Why `DeleteExpiredCache` returns row count?** It's a housekeeping query — knowing how many stale entries got cleaned up is useful for logging/monitoring.

---

## Step 6: Generate the Go Code

Run from the project root:

```bash
sqlc generate
```

If everything is correct, this creates files in `internal/database/`:

```
internal/database/
├── db.go           # Database connection wrapper + Queries struct
├── models.go       # Go structs matching your tables (User, Job, Document, DocCache)
├── users.sql.go    # Generated functions for users queries
├── jobs.sql.go     # Generated functions for jobs queries
├── documents.sql.go    # Generated functions for documents queries
├── doc_cache.sql.go    # Generated functions for doc_cache queries
└── querier.go      # Interface for all generated methods (useful for testing/mocking)
```

**Do NOT edit these files.** They get overwritten every time you run `sqlc generate`. If you need to change a query, edit the `.sql` file and re-generate.

---

## Step 7: Install the pgx Driver

sqlc generated code that imports `pgx/v5`. You need to add it to your Go module:

```bash
go get github.com/jackc/pgx/v5
```

---

## Step 8: Use the Generated Code

Here's how you'd use the generated code in your handlers. This is a preview — you'll write the actual handler code later.

### Connect to the Database

```go
package main

import (
    "context"
    "log"

    "github.com/jackc/pgx/v5/pgxpool"
    "PingGoat/internal/database"
)

func main() {
    ctx := context.Background()

    // pgxpool gives you a connection pool — reuses connections instead of
    // opening a new one for every query (which would be slow)
    pool, err := pgxpool.New(ctx, "postgres://pinggoat:pinggoat@localhost:5432/pinggoat?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()

    // Create a Queries instance — this is your "database handle"
    queries := database.New(pool)

    // Now you can call any generated function!
    user, err := queries.CreateUser(ctx, database.CreateUserParams{
        Email:        "test@example.com",
        PasswordHash: "$2a$10$...", // bcrypt hash
    })
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Created user: %s (ID: %s)", user.Email, user.ID)
}
```

**Why `pgxpool` instead of just `pgx`?** A connection pool reuses database connections. Without it, every query opens a new TCP connection to Postgres, does the query, then closes it. That's slow and wasteful — like driving to the store for every single grocery item instead of making a list.

### Query Examples

```go
// Create a job — returns the full job row with generated ID + timestamps
job, err := queries.CreateJob(ctx, database.CreateJobParams{
    UserID:  userID,
    RepoUrl: "https://github.com/gorilla/mux",
    Branch:  "main",
})

// List user's jobs — paginated
jobs, err := queries.ListJobsByUser(ctx, database.ListJobsByUserParams{
    UserID: userID,
    Limit:  20,  // per_page
    Offset: 0,   // (page - 1) * per_page
})

// Check cache before calling Gemini
cached, err := queries.GetCachedDoc(ctx, database.GetCachedDocParams{
    RepoUrl:   "https://github.com/gorilla/mux",
    CommitSha: "abc1234",
    DocType:   "readme",
})
if err == pgx.ErrNoRows {
    // Cache miss — need to call Gemini
}
```

---

## Step 9: Verify with sqlc vet

Before generating, you can check your queries for errors:

```bash
sqlc vet
```

This catches:
- Queries referencing columns that don't exist in your schema
- Type mismatches (e.g., passing a string where an int is expected)
- Syntax errors in SQL

---

## Common sqlc Commands Reference

| Command | What it does |
|---------|-------------|
| `sqlc generate` | Read queries + schema, generate Go code |
| `sqlc vet` | Validate queries against schema without generating |
| `sqlc diff` | Show what would change if you re-generate (useful before committing) |
| `sqlc compile` | Check that queries parse correctly |

---

## The Workflow Going Forward

Every time you need a new database operation:

1. **Write the SQL query** in the appropriate `sql/queries/*.sql` file
2. **Run `sqlc generate`** to regenerate the Go code
3. **Use the generated function** in your handler/service code

```
Edit .sql file → sqlc generate → use in Go code
```

That's it. No writing `rows.Scan()`. No manual struct mapping. No runtime SQL errors.

---

## Troubleshooting

**"sqlc: command not found"**
-> Run `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` and check your `$PATH`.

**"relation 'users' does not exist" during sqlc generate**
-> sqlc reads your Goose migration files for schema info. Make sure your schema files are in `sql/schema/` and the `schema` path in `sqlc.yaml` points there.

**"column 'xyz' does not exist"**
-> Check your migration files — did you spell the column name correctly? sqlc reads the CREATE TABLE statements literally.

**"no queries found"**
-> Check that `queries` in `sqlc.yaml` points to the right directory and your `.sql` files have the `-- name: FuncName :annotation` comments.

**"pq: password authentication failed"**
-> This is a runtime error, not a sqlc error. Make sure your `DATABASE_URL` matches your docker-compose credentials.

---

## What's Next?

With the generated database code in hand, the next steps are:
1. **Config package** — load env vars (`DATABASE_URL`, `JWT_SECRET`, etc.) into a config struct
2. **Auth package** — JWT generation/validation + bcrypt password hashing
3. **HTTP handlers** — wire up the API routes using the generated `Queries` struct
