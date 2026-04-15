# How to Build a "Submit Job" Handler in Go (Async Pattern)

This guide walks through creating an async job-submission endpoint: `POST /jobs` that creates a DB row and returns `202 Accepted` immediately.

## Why 202 Accepted (not 201 Created)?

`201 Created` means "I made the resource and it's done."  
`202 Accepted` means "I received your request and queued it for processing."  

Since doc generation is async (clone → parse → generate), the job isn't "done" at submission time — it's just queued. The client polls `GET /jobs/{id}` to watch it progress.

## Step-by-Step

### 1. Write the sqlc Query

Create `sql/queries/jobs.sql`:

```sql
-- name: CreateJob :one
INSERT INTO jobs (user_id, repo_url, branch)
VALUES ($1, $2, $3)
RETURNING *;
```

You only insert what the user provides. Everything else uses schema defaults:
- `id` → `gen_random_uuid()`
- `status` → `'queued'`
- `created_at` / `updated_at` → `now()`
- `commit_sha`, `file_count`, etc. → `NULL` (set later by the pipeline)

Then run `sqlc generate` to get Go code.

### 2. Handler Pattern

Follow the same flow as your auth handlers:

```
MaxBytesReader → Decode JSON → Validate → Get userID from ctx → DB call → Respond
```

### 3. Getting userID from JWT Context

Your `middleware.JWTAuth` stores the user ID (a UUID string) in the request context:

```go
userID, ok := r.Context().Value(middleware.UserIDKey).(string)
if !ok {
    // respond 401
}
```

### 4. Converting string UUID to pgtype.UUID

sqlc generates params with `pgtype.UUID`. To convert:

```go
import "github.com/google/uuid"

parsed, err := uuid.Parse(userIDString)
if err != nil {
    // respond 401 — means the JWT had a bad subject
}

pgUUID := pgtype.UUID{Bytes: parsed, Valid: true}
```

This works because `uuid.UUID` is `[16]byte` and `pgtype.UUID.Bytes` is also `[16]byte`.

### 5. Validating the Request

At minimum:
- `repo_url` must not be empty
- `repo_url` should look like a GitHub URL (`strings.HasPrefix(url, "https://github.com/")`)
- If `branch` is empty, default it to `"main"`

Per the PRD error matrix: invalid URL → 400, repo not found → 404.

### 6. Response Envelope

The PRD uses a `"data"` wrapper:

```json
{
  "data": {
    "id": "uuid-here",
    "repo_url": "https://github.com/user/repo",
    "branch": "main",
    "status": "queued",
    "created_at": "2026-03-28T10:00:00Z"
  }
}
```

Use `http.StatusAccepted` (202).

### 7. pgtype.Text for Optional Fields

`branch` is `pgtype.Text` in the generated model. To set it:

```go
pgtype.Text{String: "main", Valid: true}
```

## Common Gotchas

| Issue | Fix |
|-------|-----|
| `pgtype.UUID` vs `string` | Parse with `uuid.Parse()`, then `pgtype.UUID{Bytes: parsed, Valid: true}` |
| `pgtype.Text` for branch | `pgtype.Text{String: value, Valid: true}` |
| Forgetting to default branch | Check if empty and set to `"main"` before DB insert |
| Using 201 instead of 202 | 202 signals async — the resource exists but work is pending |
