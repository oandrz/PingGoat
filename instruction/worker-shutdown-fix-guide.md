# Fix: Worker Shutdown Race (Dropping Buffered Jobs)

## The Problem

Your current worker uses `select` with both `ctx.Done()` and channel read:

```go
// worker.go (current — broken)
for {
    select {
    case <-ctx.Done():        // ← SIGTERM fires, worker exits HERE
        return
    case msg, ok := <-jobs:   // ← buffered jobs never read
        // process
    }
}
```

When SIGTERM arrives, Go's `select` picks a random ready case. Since `ctx.Done()` is immediately ready, workers often exit **before** draining the channel buffer. Jobs are silently lost.

## The Key Insight

**Two different things control two different lifecycles:**

| What | Controls | Mechanism |
|------|----------|-----------|
| **Channel close** | Worker lifecycle (when to stop the loop) | `close(jobCh)` in main |
| **Context cancel** | Individual job work (when to abort a clone/API call) | `ctx` passed into pipeline stages |

Right now you're using context for both. That's the bug.

## The Fix (2 files)

### 1. `internal/pipeline/worker.go`

Replace the entire `select` loop with `range`:

```go
func StartWorker(ctx context.Context, id int, jobs <-chan JobMessage) {
    for msg := range jobs {
        log.Printf("worker %d processing job: %v", id, msg)
        // When you implement real pipeline stages, pass ctx here:
        // err := runPipeline(ctx, msg)
        // ctx lets you cancel long-running work (git clone, Gemini call)
        // but the worker loop itself only stops when the channel closes.
    }
    log.Printf("worker %d: channel closed, exiting", id)
}
```

**Why `range` works:** `for msg := range jobs` blocks until a message arrives. When someone calls `close(jobCh)`, the range loop exits naturally — but only **after** all buffered messages have been read. No jobs lost.

**Why you still need `ctx`:** You keep the `ctx` parameter because later, inside the loop, you'll call things like `cloneRepo(ctx, msg.RepoURL)`. If SIGTERM fires mid-clone, the context cancellation aborts that specific operation — but the worker still processes the remaining buffered jobs.

### 2. `cmd/api/main.go` — shutdown sequence

Your shutdown sequence is already correct:

```go
<-ctx.Done()                      // 1. SIGTERM received
log.Println("Shutting down...")
srv.Shutdown(context.Background()) // 2. Stop accepting HTTP requests
close(jobCh)                       // 3. Signal workers to drain & exit
wg.Wait()                          // 4. Wait for all workers to finish
log.Println("Shutdown complete")
```

No changes needed here. The `close(jobCh)` is what tells the `range` loop to stop.

## What Happens at Shutdown (After Fix)

```
Timeline:
─────────────────────────────────────────────────────
SIGTERM arrives
  ↓
ctx cancelled → recovery sweep exits (good, stop enqueuing)
  ↓
srv.Shutdown() → no new HTTP requests (no new jobs submitted)
  ↓
close(jobCh) → workers' `range` drains remaining buffered jobs
  ↓
range exits → workers return → wg.Done()
  ↓
wg.Wait() returns
  ↓
"Shutdown complete"
```

## Checklist

- [ ] Replace `select` loop in `worker.go` with `for msg := range jobs`
- [ ] Keep `ctx` parameter (you'll need it for pipeline stages later)
- [ ] No changes needed in `main.go` — shutdown order is already correct
- [ ] Test: run server, submit a job, send SIGTERM, verify log shows job was processed before exit
