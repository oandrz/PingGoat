# Protecting Against Deleting In-Progress Jobs

## The Problem

When a job is being processed by the pipeline (status: `cloning`, `parsing`, `generating`), deleting it via `DELETE /jobs/{id}` will cascade-delete the job and its documents from the DB. The pipeline goroutine, still running, will then fail when it tries to update the now-gone job row.

## 3 Approaches

### Approach 1: Application-level check (Recommended for this project)

Fetch the job first with `GetJob`, check its status in Go code, return `409 Conflict` if the job is active.

- **Pros:** Simple, clear error messages, easy to reason about
- **Cons:** TOCTOU race condition (status could change between GET and DELETE)
- **Best for:** Low-concurrency systems like our single-worker pipeline

### Approach 2: SQL-level conditional DELETE

Modify the DELETE query to include `AND status NOT IN ('cloning', 'parsing', 'generating')`.

- **Pros:** Atomic, no race condition
- **Cons:** Can't distinguish "not found" vs "in-progress" when `rowsAffected == 0`
- **Best for:** High-concurrency systems where TOCTOU matters

### Approach 3: Hybrid (Fetch + conditional DELETE)

Fetch first for good error messages, then use a conditional DELETE as a safety net.

- **Pros:** Best UX + atomic safety
- **Cons:** Two DB round-trips
- **Best for:** When you want both correctness and good UX

## Implementation (Approach 1)

### Step 1: Create a status check helper

```go
func isActiveJobStatus(status string) bool {
    switch status {
    case "cloning", "parsing", "generating":
        return true
    default:
        return false
    }
}
```

### Step 2: In RemoveJobById, fetch the job before deleting

After UUID validation, before `DeleteJob`:

```go
job, err := h.queries.GetJob(r.Context(), database.GetJobParams{
    ID:     jobIdPgUUID,
    UserID: pgUUID,
})
```

### Step 3: Handle GetJob errors

Same pattern as `GetJobById` — check `pgx.ErrNoRows` for 404, other errors for 500.

### Step 4: Check status, return 409 if active

```go
if isActiveJobStatus(job.Status) {
    httputil.RespondWithError(w, http.StatusConflict, "Cannot delete job while it is being processed")
    return
}
```

### Step 5: Proceed with delete only if status is safe

The existing `DeleteJob` call stays, but now it only runs for `queued`, `completed`, or `failed` jobs.

## Why 409 Conflict?

HTTP 409 means "the request conflicts with the current state of the resource." The client wants to delete, but the resource's state doesn't allow it. The client can retry after the job completes or fails.

## TOCTOU Race Condition

"Time of Check to Time of Use" — between your `GetJob` (check) and `DeleteJob` (use), the status *could* theoretically change. In our single-worker pipeline this is practically impossible, but in a multi-worker system you'd want Approach 2 or 3.

## Final Request Flow

```
Parse user ID from JWT
Parse job ID from URL
→ Fetch the job (GetJob)
  → Not found? → 404
  → Active status? → 409 Conflict
→ Delete the job (DeleteJob)
  → rowsAffected == 0? → 404 (safety net)
→ 204 No Content (or 200 OK)
```
