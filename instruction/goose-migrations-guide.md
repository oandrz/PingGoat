# Database Schema & Goose Migrations Guide

This guide walks you through setting up PostgreSQL migrations for DocGoat using Goose.

---

## What is Goose and Why Use It?

**Goose** is a database migration tool for Go. It lets you version-control your database schema changes as individual SQL files, so you can:

- Apply changes in order (`goose up`)
- Roll back changes (`goose down`)
- Track which migrations have been applied
- Share the same schema across your team and environments

**Why not just write SQL directly against the DB?** Because then there's no history. If you add a column locally but forget what you did, or your teammate needs the same change — you're stuck. Migrations are like "git commits for your database."

---

## Step 1: Install Goose

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Verify it's installed:
```bash
goose --version
```

> **If `goose` is not found:** Make sure `$GOPATH/bin` is in your `$PATH`. Add this to your `~/.zshrc`:
> ```bash
> export PATH="$PATH:$(go env GOPATH)/bin"
> ```
> Then run `source ~/.zshrc`.

---

## Step 2: Create the Migration Directory

```bash
mkdir -p sql/schema
```

This is where all your `.sql` migration files will live. Goose reads them in order based on the number prefix.

Your project structure should look like:
```
sql/
├── schema/          # Goose migration files (this step)
└── queries/         # sqlc query files (later step)
```

---

## Step 3: Create Migration Files

Goose can generate migration files for you with timestamps:

```bash
goose -dir sql/schema create create_users sql
goose -dir sql/schema create create_jobs sql
goose -dir sql/schema create create_documents sql
goose -dir sql/schema create create_doc_cache sql
```

This creates files like:
```
sql/schema/
├── 20260401120000_create_users.sql
├── 20260401120001_create_jobs.sql
├── 20260401120002_create_documents.sql
└── 20260401120003_create_doc_cache.sql
```

Each file has two sections separated by comments:
- `-- +goose Up` — SQL to apply the migration
- `-- +goose Down` — SQL to undo the migration (rollback)

**Why separate files per table?** So you can roll back one table at a time if something goes wrong. If everything were in one file, rolling back means losing ALL tables.

---

## Step 4: Write the Migration SQL

Here's the DocGoat schema from the PRD. Write each migration file.

### Migration 1: `create_users.sql`

```sql
-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    github_token_enc TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS users;
```

**Why UUID instead of auto-increment?** UUIDs are globally unique — they're safe to generate anywhere (API server, tests, scripts) without hitting the DB first. They also don't leak information about how many users you have.

**Why `gen_random_uuid()`?** It's a built-in PostgreSQL function (v13+). No extensions needed.

### Migration 2: `create_jobs.sql`

```sql
-- +goose Up
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    repo_url TEXT NOT NULL,
    branch VARCHAR(255) NOT NULL DEFAULT 'main',
    commit_sha VARCHAR(40),
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    error_message TEXT,
    file_count INT,
    gemini_calls_used INT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes for common query patterns
CREATE INDEX idx_jobs_user_created ON jobs (user_id, created_at DESC);
CREATE INDEX idx_jobs_status ON jobs (status);

-- +goose Down
DROP TABLE IF EXISTS jobs;
```

**Why these indexes?**
- `idx_jobs_user_created` — "list all my jobs, newest first" is the most common user query
- `idx_jobs_status` — the worker polls for `queued` jobs; without this index, it scans the entire table every time

**Why `ON DELETE CASCADE`?** When a user is deleted, their jobs should go with them. Without CASCADE, deleting a user would fail if they have any jobs.

### Migration 3: `create_documents.sql`

```sql
-- +goose Up
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    doc_type VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    prompt_tokens INT,
    completion_tokens INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One document per type per job
CREATE UNIQUE INDEX idx_documents_job_type ON documents (job_id, doc_type);

-- +goose Down
DROP TABLE IF EXISTS documents;
```

**Why the UNIQUE index on `(job_id, doc_type)`?** Each job should only produce one README, one quickstart, and one diagram. This constraint prevents duplicate documents at the database level — even if a bug in your code tries to insert two READMEs for the same job, Postgres will reject it.

### Migration 4: `create_doc_cache.sql`

```sql
-- +goose Up
CREATE TABLE doc_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_url TEXT NOT NULL,
    commit_sha VARCHAR(40) NOT NULL,
    doc_type VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '7 days'
);

-- Fast cache lookups: "do we already have a README for this repo at this commit?"
CREATE UNIQUE INDEX idx_doc_cache_lookup ON doc_cache (repo_url, commit_sha, doc_type);

-- +goose Down
DROP TABLE IF EXISTS doc_cache;
```

**Why a separate cache table instead of just reusing `documents`?** The cache is keyed by `(repo_url, commit_sha, doc_type)` — it's repo-level, not job-level. Multiple users submitting the same repo at the same commit can share cached results. This saves Gemini API calls (remember: 250/day limit!).

---

## Step 5: Start Postgres and Run Migrations

Make sure your Docker Postgres is running:

```bash
docker-compose up db -d
```

Set your database URL (matches docker-compose.yml):

```bash
export DATABASE_URL="postgres://pinggoat:pinggoat@localhost:5432/pinggoat?sslmode=disable"
```

Run all migrations:

```bash
goose -dir sql/schema postgres "$DATABASE_URL" up
```

You should see output like:
```
OK   20260401120000_create_users.sql (Xms)
OK   20260401120001_create_jobs.sql (Xms)
OK   20260401120002_create_documents.sql (Xms)
OK   20260401120003_create_doc_cache.sql (Xms)
```

---

## Step 6: Verify Your Schema

Connect to the database and check:

```bash
docker exec -it pinggoat-db-1 psql -U pinggoat -d pinggoat
```

Inside `psql`:
```sql
\dt                          -- list all tables
\d users                     -- describe users table
\d jobs                      -- describe jobs table
\d documents                 -- describe documents table
\d doc_cache                 -- describe doc_cache table
```

You should see all 4 tables plus the `goose_db_version` table (Goose's internal tracking).

Type `\q` to exit psql.

---

## Common Goose Commands Reference

| Command | What it does |
|---------|-------------|
| `goose -dir sql/schema postgres "$DATABASE_URL" up` | Apply all pending migrations |
| `goose -dir sql/schema postgres "$DATABASE_URL" down` | Roll back the last migration |
| `goose -dir sql/schema postgres "$DATABASE_URL" status` | Show which migrations have been applied |
| `goose -dir sql/schema postgres "$DATABASE_URL" reset` | Roll back ALL migrations (careful!) |
| `goose -dir sql/schema create add_some_column sql` | Create a new empty migration file |

---

## What's Next?

After your schema is in place, the next step is **sqlc** — it reads your SQL queries and generates type-safe Go code to interact with these tables. That's a separate guide.

---

## Troubleshooting

**"goose: command not found"**
→ Run `go install github.com/pressly/goose/v3/cmd/goose@latest` and make sure `$GOPATH/bin` is in your `$PATH`.

**"connection refused" when running migrations**
→ Make sure Postgres is running: `docker-compose up db -d`. Wait a few seconds for it to start.

**"relation already exists"**
→ You've already run this migration. Check status with `goose status`. If you need to re-run, either `goose down` first or `goose reset`.

**"role 'pinggoat' does not exist"**
→ You're connecting to a Postgres that wasn't started with `docker-compose`. The docker-compose.yml creates the `pinggoat` user automatically.
