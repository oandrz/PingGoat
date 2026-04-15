# Guide: List User's Jobs (Paginated)

`GET /api/v1/jobs` — returns a paginated list of the authenticated user's jobs.

## Files to Touch

| # | File | What to do |
|---|------|------------|
| 1 | `sql/queries/jobs.sql` | Add `ListJobsByUser` (`:many`) and `CountJobsByUser` (`:one`) queries |
| 2 | Run `sqlc generate` | Regenerate `internal/database/jobs.sql.go` |
| 3 | `internal/handler/jobsHandler.go` | Add `ListJobs` method + update `JobsHandler` interface |
| 4 | `cmd/api/main.go` | Register `r.Get("/api/v1/jobs", ...)` in the auth group |

## Step 1: SQL Queries

Add two queries to `sql/queries/jobs.sql`:

**ListJobsByUser** — paginated select filtered by user_id, ordered by `created_at DESC`, with `LIMIT` and `OFFSET`. Use `:many`.

**CountJobsByUser** — `SELECT COUNT(*) FROM jobs WHERE user_id = $1`. Use `:one`.

### Why two queries instead of a window function?

sqlc handles simple queries cleanly. A `COUNT(*) OVER()` window function works but makes the generated struct messy — every row carries the total count redundantly. Two queries is more idiomatic with sqlc.

### Why offset-based instead of cursor-based?

Offset-based matches the PRD's `page/per_page/total` response envelope. Cursor-based is better for massive datasets, but per-user job lists are small enough that offset is fine and simpler.

### Index Coverage

The existing index `idx_user_created_at ON jobs (user_id, created_at DESC)` covers the `WHERE user_id = $1 ORDER BY created_at DESC` pattern perfectly.

## Step 2: Regenerate sqlc

```bash
sqlc generate
```

This updates `internal/database/jobs.sql.go` with the new functions.

## Step 3: Handler Logic

The `ListJobs` handler should:

1. **Parse query params** — `page` and `per_page` from `r.URL.Query()`. Default to page=1, per_page=20. Cap per_page at 100.
2. **Extract user ID** — Same pattern as `SubmitJob` (from context via `middleware.UserIDKey`).
3. **Calculate offset** — `offset = (page - 1) * perPage`.
4. **Call both queries** — `CountJobsByUser` for total, `ListJobsByUser` for the page.
5. **Build response envelope** — Per the PRD:

```json
{
  "data": [ { "id": "...", "repo_url": "...", "status": "...", ... } ],
  "meta": { "page": 1, "per_page": 20, "total": 42 }
}
```

6. **Respond** — `httputil.RespondWithJSON(w, 200, response)`

### Tips

- Use `strconv.Atoi` for query param parsing; fall back to defaults on error.
- Map `database.Job` to a simpler response struct (ID as string, timestamps as RFC3339).
- Return empty `data: []` (not null) when no jobs exist.

## Step 4: Route Registration

In `cmd/api/main.go`, inside the authenticated group:

```go
r.Get("/api/v1/jobs", jobsHandler.ListJobs)
```

## Verification

```bash
# Default pagination
curl -H "Authorization: Bearer <token>" "http://localhost:8080/api/v1/jobs"

# Custom page/size
curl -H "Authorization: Bearer <token>" "http://localhost:8080/api/v1/jobs?page=2&per_page=5"
```

Check:
- Empty list returns `data: []` with `total: 0`
- Only the authenticated user's jobs are returned
- Offset math is correct
- Defaults work when params are omitted
