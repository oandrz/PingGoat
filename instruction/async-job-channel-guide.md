# Wiring Up the Async Job Channel

## What You're Building

Right now your `SubmitJob` handler does this:
1. Validates the request
2. Creates the job row in Postgres (status = "pending")
3. Returns 202 Accepted with the job ID

What's missing: **nobody picks up that job to actually process it.** You need a buffered channel that acts as an in-memory job queue. The POST handler sends jobs into it, and worker goroutines read from it on the other end.

```
POST /jobs → DB insert → send to channel → return 202
                                ↓
                        worker goroutine(s)
                        reads from channel
                        runs pipeline stages
```

## Why a Buffered Channel?

- **Unbuffered channel** (`make(chan T)`) — the sender blocks until a receiver is ready. If your worker is busy, your HTTP handler would hang. Bad for an API.
- **Buffered channel** (`make(chan T, size)`) — the sender only blocks when the buffer is full. Your HTTP handler can fire-and-forget (up to the buffer size) and return 202 immediately.

The buffer size should match your expected concurrency. From your PRD config: `PIPELINE_WORKERS` controls how many concurrent workers you run. A good buffer size is something like `PIPELINE_WORKERS * 2` or a fixed number like 100.

## Step-by-Step TODO

### Step 1: Define a Job Message Type

Create a new file `internal/pipeline/job.go`. You need a struct that carries enough info for the worker to process a job. Think about what the worker needs:

- The job ID (to update status in the DB)
- The repo URL (to clone)
- The branch name

You could use `database.Job` directly, or define a lightweight struct like:

```go
package pipeline

import "github.com/jackc/pgx/v5/pgtype"

type JobMessage struct {
    ID      pgtype.UUID
    RepoURL string
    Branch  string
}
```

**Why a separate type instead of reusing `database.Job`?** The pipeline shouldn't depend on every DB field. This keeps coupling low. The handler converts `database.Job` → `pipeline.JobMessage` before sending.

### Step 2: Inject the Channel Into Your Handler

Your current `jobsHandler` struct:

```go
type jobsHandler struct {
    queries *database.Queries
}
```

Add a channel field:

```go
type jobsHandler struct {
    queries *database.Queries
    jobCh   chan<- pipeline.JobMessage  // send-only direction
}
```

**Why `chan<-` (send-only)?** The handler should only *send* to the channel, never *read* from it. Go's channel direction types enforce this at compile time. This is a great Go idiom — restrict capabilities to what's actually needed.

Update `NewJobsHandler` to accept and store the channel.

### Step 3: Send the Job After DB Insert (in `SubmitJob`)

After the `h.queries.CreateJob(...)` call succeeds (line ~84-91 in your current code), add a non-blocking send:

```go
select {
case h.jobCh <- pipeline.JobMessage{
    ID:      job.ID,
    RepoURL: job.RepoUrl,
    Branch:  job.Branch.String,
}:
default:
    log.Printf("job channel full, job %s will be picked up by recovery sweep", job.ID)
}
```

**Why `select` with `default` instead of just `h.jobCh <- msg`?** A plain send blocks if the buffer is full. With `select`/`default`, you get a non-blocking send. If the buffer is full, you hit the `default` case instead of hanging your HTTP handler.

**Why always return 202 (even when the channel is full)?** The job is already saved in Postgres with status "pending" — it's not lost. From the user's perspective, the job *was* created successfully. The channel is just an internal optimization for fast dispatch. A recovery sweep (Step 8) will pick up any jobs that didn't make it into the channel.

### Step 4: Create the Channel in `main.go`

In your `main()` function, before creating the handler:

```go
jobCh := make(chan pipeline.JobMessage, bufferSize)
```

Then pass it to `NewJobsHandler`:

```go
jobsHandler := handler.NewJobsHandler(dbQueries, jobCh)
```

**Where does `bufferSize` come from?** Add a `PipelineWorkers` field to your `Config` struct (from `PIPELINE_WORKERS` env var). Use that to size the buffer.

### Step 5: Create a Stub Worker

Create `internal/pipeline/worker.go`. For now, just a stub that reads from the channel and logs:

```go
func StartWorker(ctx context.Context, id int, jobs <-chan JobMessage) {
    for {
        select {
        case <-ctx.Done():
            log.Printf("worker %d: shutting down", id)
            return
        case msg, ok := <-jobs:
            if !ok {
                return // channel closed
            }
            log.Printf("worker %d: picked up job %s for %s", id, msg.ID, msg.RepoURL)
            // TODO: run pipeline stages here
        }
    }
}
```

**Key patterns here:**
- `<-chan JobMessage` — receive-only channel direction (workers only read)
- `ctx.Done()` in the select — allows graceful shutdown when context is cancelled
- `msg, ok := <-jobs` — the `ok` check detects when the channel is closed

### Step 6: Launch Workers in `main.go`

After creating the channel, spin up N workers:

```go
var wg sync.WaitGroup
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

for i := 0; i < cfg.PipelineWorkers; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        pipeline.StartWorker(ctx, id, jobCh)
    }(i)
}
```

**Why `signal.NotifyContext`?** It creates a context that gets cancelled when the process receives SIGINT (Ctrl+C) or SIGTERM. This triggers `ctx.Done()` in your workers, letting them finish gracefully.

**Why `sync.WaitGroup`?** So your `main()` can wait for all workers to finish before exiting. Without it, the process exits immediately after cancel, and workers might be mid-processing.

### Step 7: Recovery Sweep (Pick Up Orphaned Jobs)

Since we always return 202 even when the channel is full, we need a mechanism to pick up "pending" jobs that didn't make it into the channel. This also handles crash recovery — if the server restarts, any in-flight jobs that were in the channel are lost, but they're still "pending" in the DB.

Create a function in `internal/pipeline/recovery.go` that:

1. Runs on a `time.Ticker` (e.g., every 30 seconds)
2. Queries `SELECT * FROM jobs WHERE status = 'pending'`
3. Tries a non-blocking send for each one into the channel

```go
func StartRecoverySweep(ctx context.Context, queries *database.Queries, jobCh chan<- JobMessage, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Query pending jobs from DB
            // For each job, try non-blocking send into jobCh
            // If channel is full, stop — next tick will retry
        }
    }
}
```

**Key details:**
- Use the same non-blocking `select`/`default` send pattern — don't block the sweep if the channel is full
- You'll need a new sqlc query: `ListPendingJobs` — add it to `sql/queries/jobs.sql`
- Launch this as another goroutine in `main.go` (add it to your WaitGroup)
- The sweep also runs once at startup to catch jobs orphaned by a previous crash

**Latency trade-off:** If the channel is full and the ticker runs every 30s, a job might wait up to 30s. But given Gemini's 10 RPM rate limit, if the channel is full, workers are already busy — the job would be waiting anyway.

### Step 8: Graceful Shutdown

After `http.ListenAndServe`, add shutdown logic:

```go
// Start server in a goroutine so we can handle shutdown
srv := &http.Server{Addr: ":" + cfg.APIPort, Handler: r}
go func() {
    if err := srv.ListenAndServe(); err != http.ErrServerClosed {
        log.Fatalf("server error: %v", err)
    }
}()

<-ctx.Done()           // wait for interrupt signal
log.Println("shutting down...")
srv.Shutdown(context.Background())  // stop accepting new requests
close(jobCh)           // signal workers no more jobs
wg.Wait()              // wait for workers to finish
log.Println("shutdown complete")
```

**Order matters:**
1. Cancel context (done by `signal.NotifyContext`)
2. Shut down HTTP server (stop accepting requests)
3. Close the channel (workers exit their `for` loop)
4. Wait on WaitGroup (workers finish in-flight work)

## The Big Picture After All Steps

```
main.go
├── creates jobCh (buffered channel)
├── passes jobCh to NewJobsHandler()
├── launches N worker goroutines (reading from jobCh)
├── launches recovery sweep goroutine (re-enqueues orphaned "pending" jobs)
├── starts HTTP server
└── on shutdown: cancel ctx → close(jobCh) → wg.Wait()

POST /jobs handler
├── validates + inserts DB row
├── sends JobMessage into jobCh (non-blocking)
├── if channel full: logs warning, job stays "pending" in DB
└── always returns 202

Recovery sweep (every 30s)
├── queries DB for "pending" jobs
├── tries non-blocking send into jobCh
└── also runs at startup for crash recovery

Worker goroutines
├── read from jobCh
├── (later) run clone → parse → batch → generate → store
└── exit when ctx cancelled or channel closed
```

## Checklist

- [ ] Create `internal/pipeline/job.go` with `JobMessage` struct
- [ ] Create `internal/pipeline/worker.go` with `StartWorker` stub
- [ ] Add `chan<- pipeline.JobMessage` to `jobsHandler` struct
- [ ] Update `NewJobsHandler` to accept the channel
- [ ] Add non-blocking send in `SubmitJob` after DB insert (always return 202)
- [ ] Add `PipelineWorkers` to `Config`
- [ ] Create the buffered channel in `main.go`
- [ ] Launch worker goroutines with WaitGroup
- [ ] Add `ListPendingJobs` sqlc query
- [ ] Create `internal/pipeline/recovery.go` with ticker-based sweep
- [ ] Launch recovery sweep goroutine in `main.go`
- [ ] Set up graceful shutdown with `signal.NotifyContext`

## Common Mistakes to Watch For

1. **Forgetting channel direction** — use `chan<-` in handler (send-only), `<-chan` in worker (receive-only). Don't use bidirectional `chan` everywhere.
2. **Blocking send in HTTP handler** — always use `select`/`default` for non-blocking sends, or your API will hang when the buffer fills up.
3. **Not closing the channel** — if you don't `close(jobCh)`, workers will block forever on read, and `wg.Wait()` will never return.
4. **Closing from the wrong side** — only the sender (main) should close the channel, never the workers (receivers). Closing a closed channel panics.
5. **Launching goroutines without WaitGroup** — your process will exit before workers finish their in-flight jobs.
