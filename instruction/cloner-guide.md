# Pipeline Stage 1: Cloner Guide

> **Goal:** Take a repo URL → produce a folder of files on disk + the HEAD
> commit SHA → hand it to the next pipeline stage → clean up when done.
>
> **Time estimate:** 2–4 hours if you follow the steps in order.

---

## TL;DR — Where do I start?

**Start with Step 1** below. The steps are ordered so each one builds on the
previous and can be tested before moving on. Don't read the whole doc first —
jump in and flip back when you hit a step.

```
Step 1: Create the file + types            (5 min)   ← START HERE
Step 2: Write validateRepoURL + its test   (20 min)
Step 3: Create the temp dir + cleanup       (15 min)
Step 4: Build and run the git clone        (30 min)
Step 5: Resolve the commit SHA             (10 min)
Step 6: Add the redact helper + logging    (15 min)
Step 7: Write the integration test         (30 min)
Step 8: Wire into the worker               (30 min)
```

At each step you should have something that **compiles and runs**. Don't
batch-write everything and debug at the end.

---

## Before you start: the big picture

### What you're building

A function `Clone` that:

1. Takes a repo URL, optional branch, optional GitHub token
2. Runs `git clone --depth 1` into a temp directory
3. Runs `git rev-parse HEAD` to get the commit SHA
4. Returns a `Workspace` handle the caller uses + cleans up

### The function signature you're aiming for

```go
type CloneOptions struct {
    RepoURL     string
    Branch      string // empty = default branch
    GitHubToken string // empty = public repo
}

type Workspace struct {
    Dir       string
    CommitSHA string
    Cleanup   func() // caller MUST defer this
}

func Clone(ctx context.Context, opts CloneOptions) (*Workspace, error)
```

### Where the code lives

```
internal/pipeline/
├── cloner.go                    ← you'll create this
├── cloner_test.go               ← you'll create this
├── cloner_integration_test.go   ← you'll create this (//go:build integration)
├── job.go                         (existing)
├── worker.go                      (you'll edit this in Step 8)
├── recovery.go
└── status.go
```

All files use `package pipeline`. No sub-packages — stages live side by side
so they can share unexported types freely.

---

## Step 1 — Create the file and types (5 min)

Create `internal/pipeline/cloner.go`:

```go
package pipeline

import (
    "context"
    "errors"
)

var (
    ErrInvalidRepoURL = errors.New("invalid repository URL")
    ErrCloneTimeout   = errors.New("git clone timed out")
)

type CloneOptions struct {
    RepoURL     string
    Branch      string
    GitHubToken string
}

type Workspace struct {
    Dir       string
    CommitSHA string
    Cleanup   func()
}

func Clone(ctx context.Context, opts CloneOptions) (*Workspace, error) {
    return nil, errors.New("not implemented")
}
```

**Check:** `go build ./...` passes. You now have a skeleton to fill in.

---

## Step 2 — URL validation (20 min)

**Why first?** It's pure logic (no files, no network, no processes), so it's
the easiest to test-drive. You'll also use it on line 1 of `Clone` to fail
fast on bad input.

### The rules

- Scheme must be `https`
- Host must be `github.com`
- Path must look like `/owner/repo` or `/owner/repo.git`

### TDD approach (write `cloner_test.go` first)

Create `internal/pipeline/cloner_test.go`:

```go
package pipeline

import "testing"

func TestValidateRepoURL(t *testing.T) {
    tests := []struct {
        name    string
        url     string
        wantErr bool
    }{
        {"valid https", "https://github.com/octocat/Hello-World", false},
        {"valid with .git", "https://github.com/octocat/Hello-World.git", false},
        {"http not https", "http://github.com/a/b", true},
        {"wrong host", "https://gitlab.com/a/b", true},
        {"file scheme", "file:///etc/passwd", true},
        {"path traversal", "https://github.com/../../etc", true},
        {"empty", "", true},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            err := validateRepoURL(tc.url)
            if (err != nil) != tc.wantErr {
                t.Fatalf("got err=%v, wantErr=%v", err, tc.wantErr)
            }
        })
    }
}
```

Run: `go test ./internal/pipeline/...` → it fails (no `validateRepoURL` yet).
Good.

### Now implement it in `cloner.go`

Hints:

- Use `net/url.Parse`.
- Check `u.Scheme`, `u.Host`.
- Use a `regexp.MustCompile(^/[^/]+/[^/]+(\.git)?/?$)` on `u.Path`.
- Return `ErrInvalidRepoURL` (wrapped with context via `fmt.Errorf("%w: ...", ...)`).

Run tests again until green.

---

## Step 3 — Temp dir + cleanup closure (15 min)

**Why now?** Second-simplest piece. No network. Teaches the cleanup-closure
idiom you'll use everywhere.

Inside `Clone`, after validation:

```go
dir, err := os.MkdirTemp("", "docgoat-clone-*")
if err != nil {
    return nil, fmt.Errorf("create temp dir: %w", err)
}
cleanup := func() { _ = os.RemoveAll(dir) }
```

### Important rule

If anything fails **after** `MkdirTemp` but **before** you return a
`*Workspace` to the caller, YOU must call `cleanup()` yourself. Otherwise you
leak a directory. The caller only owns cleanup once they hold the handle.

Pattern:

```go
dir, err := os.MkdirTemp(...)
if err != nil { return nil, err }
cleanup := func() { _ = os.RemoveAll(dir) }

if err := someStep(); err != nil {
    cleanup()        // ← important
    return nil, err
}
```

### Unit test: cleanup is idempotent

Add to `cloner_test.go`:

```go
func TestWorkspaceCleanupIdempotent(t *testing.T) {
    dir, _ := os.MkdirTemp("", "docgoat-test-*")
    ws := &Workspace{Dir: dir, Cleanup: func() { os.RemoveAll(dir) }}
    ws.Cleanup()
    ws.Cleanup() // must not panic
}
```

---

## Step 4 — Run `git clone` (30 min)

This is the core of the function. Build it up in two sub-steps.

### 4a. Build the argument list

```go
args := []string{}
if opts.GitHubToken != "" {
    args = append(args,
        "-c", "http.extraHeader=Authorization: Bearer "+opts.GitHubToken)
}
args = append(args, "clone", "--depth", "1", "--no-tags", "--single-branch")
if opts.Branch != "" {
    args = append(args, "--branch", opts.Branch)
}
args = append(args, opts.RepoURL, dir)
```

**Why `-c http.extraHeader` instead of `https://TOKEN@github.com/...`?**
Embedding the token in the URL leaks it into git's reflog, config, and most
error messages. The `-c` flag keeps the URL clean; the token only lives in
`cmd.Args` (which you'll redact before logging in Step 6).

### 4b. Run with a timeout

```go
ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
defer cancel()

var stderr bytes.Buffer
cmd := exec.CommandContext(ctx, "git", args...)
cmd.Stderr = &stderr

if err := cmd.Run(); err != nil {
    cleanup()
    if errors.Is(ctx.Err(), context.DeadlineExceeded) {
        return nil, ErrCloneTimeout
    }
    return nil, fmt.Errorf("git clone failed: %w: %s", err, stderr.String())
}
```

### Why each part matters

| Piece | Why |
|---|---|
| `context.WithTimeout(ctx, 120s)` | Layered on worker's ctx; either can cancel |
| `exec.CommandContext` | Auto-SIGKILLs git when ctx cancels — free cancellation |
| `cmd.Stderr = &stderr` | Git writes useful errors there (404, auth, network) |
| `errors.Is(ctx.Err(), context.DeadlineExceeded)` | Distinguishes timeout from other failures |
| `cleanup()` on error | Prevents leaking the temp dir |

### Manual smoke test

Temporarily add a `main` somewhere or a quick test that calls
`Clone(ctx, CloneOptions{RepoURL: "https://github.com/octocat/Hello-World"})`
and prints the returned `dir`. Check the directory exists and has a README.
Then delete the smoke test — Step 7 replaces it with a real integration test.

---

## Step 5 — Resolve the commit SHA (10 min)

After a successful clone, run a second git command:

```go
var shaBuf bytes.Buffer
shaCmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")
shaCmd.Stdout = &shaBuf
if err := shaCmd.Run(); err != nil {
    cleanup()
    return nil, fmt.Errorf("resolve HEAD: %w", err)
}

sha := strings.TrimSpace(shaBuf.String())
if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(sha) {
    cleanup()
    return nil, fmt.Errorf("invalid SHA: %q", sha)
}

return &Workspace{Dir: dir, CommitSHA: sha, Cleanup: cleanup}, nil
```

The SHA validation regex protects you from corrupted output. This SHA is what
you'll store in `jobs.commit_sha` (PRD line 217).

---

## Step 6 — Redact helper for safe logging (15 min)

The token lives in `cmd.Args` as `Authorization: Bearer <TOKEN>`. If you ever
log `args` (for debugging), you leak it. Write a helper:

```go
func redact(args []string) []string {
    out := make([]string, len(args))
    for i, a := range args {
        if strings.HasPrefix(a, "http.extraHeader=Authorization:") {
            out[i] = "http.extraHeader=Authorization: ***"
        } else {
            out[i] = a
        }
    }
    return out
}
```

Unit-test it:

```go
func TestRedact(t *testing.T) {
    in := []string{"-c", "http.extraHeader=Authorization: Bearer sekret", "clone"}
    out := redact(in)
    for _, a := range out {
        if strings.Contains(a, "sekret") {
            t.Fatal("token leaked in redacted output")
        }
    }
}
```

Use `redact(args)` whenever you `log.Printf` the command.

---

## Step 7 — Integration test (30 min)

Create `internal/pipeline/cloner_integration_test.go`:

```go
//go:build integration

package pipeline

import (
    "context"
    "os"
    "testing"
    "time"
)

func TestClone_HappyPath(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    ws, err := Clone(ctx, CloneOptions{
        RepoURL: "https://github.com/octocat/Hello-World",
    })
    if err != nil {
        t.Fatal(err)
    }
    defer ws.Cleanup()

    if len(ws.CommitSHA) != 40 {
        t.Errorf("bad SHA: %q", ws.CommitSHA)
    }
    if _, err := os.Stat(ws.Dir + "/README"); err != nil {
        t.Errorf("README missing: %v", err)
    }
}

func TestClone_Timeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
    defer cancel()

    _, err := Clone(ctx, CloneOptions{
        RepoURL: "https://github.com/torvalds/linux",
    })
    if err == nil {
        t.Fatal("expected timeout error, got nil")
    }
}
```

Run with: `go test -tags=integration ./internal/pipeline/...`

The build tag keeps these out of normal `go test` runs so CI doesn't hit the
network on every commit.

---

## Step 8 — Wire into the worker (30 min)

Now edit `internal/pipeline/worker.go`. The key refactor: **extract a
per-job function** so `defer ws.Cleanup()` fires per iteration, not at
worker shutdown.

### Current shape

```go
func StartWorker(ctx context.Context, queries *database.Queries, id int, jobs <-chan JobMessage) {
    for msg := range jobs {
        // all logic inline here
    }
}
```

### New shape

```go
func StartWorker(ctx context.Context, queries *database.Queries, id int, jobs <-chan JobMessage) {
    for msg := range jobs {
        if err := processJob(ctx, queries, msg); err != nil {
            log.Printf("worker %d job %v failed: %v", id, msg.JobID, err)
        }
    }
}

func processJob(ctx context.Context, q *database.Queries, msg JobMessage) error {
    // 1. mark "cloning"
    // 2. Clone
    ws, err := Clone(ctx, CloneOptions{
        RepoURL: msg.RepoURL,
        Branch:  msg.Branch,
        // GitHubToken: decrypt from users.github_token_enc if private
    })
    if err != nil {
        // mark job "failed" with err message, then return
        return err
    }
    defer ws.Cleanup() // ✅ fires on every iteration

    // 3. persist ws.CommitSHA on jobs row
    // 4. TODO later: Scan(ws.Dir), Parse, Batch, Generate
    return nil
}
```

### Why the extraction matters

`defer` fires when the **function** returns. If you put `defer ws.Cleanup()`
directly inside the `for msg := range jobs` loop, temp dirs pile up until
`StartWorker` exits — disk leak. Moving the body into `processJob` makes
each job a clean function boundary.

### Don't call `Clone` from the HTTP handler

`POST /jobs` must return `202 Accepted` in milliseconds. Cloning is async —
the handler only enqueues the `JobMessage`, the worker does the actual work.

### Nice-to-have: fail fast at startup

In wherever you wire up workers (probably `cmd/api/`), add:

```go
if _, err := exec.LookPath("git"); err != nil {
    log.Fatal("git binary not found in PATH")
}
```

Better to crash on boot than to have every job fail mysteriously.

---

## Gotchas (read after you're done, or when stuck)

- **Don't `defer cleanup()` inside `Clone`** — the caller owns cleanup. Only
  call `cleanup()` on error paths inside `Clone`.
- **`exec.CommandContext` sends SIGKILL**, so a killed process surfaces as
  `signal: killed`. Detect timeout via `ctx.Err()`, not by parsing the error
  string.
- **`--depth 1` is shallow in history, not in files.** A 500 MB repo is still
  500 MB on disk. Consider a size guard later.
- **macOS temp path.** `$TMPDIR` on macOS points into `/var/folders/...`, not
  `/tmp`. Never hardcode `/tmp` in paths.
- **Git not in container.** Your `Dockerfile` must `apt-get install git` (or
  use a base image that has it). `exec.LookPath("git")` at startup catches
  this early.
- **Token in logs.** Always pass `redact(args)` to any logger, never raw
  `cmd.Args`.

---

## Reference: why shell out to `git`?

Three options were considered:

| Approach | Pros | Cons |
|---|---|---|
| `exec.Command("git", ...)` | Full protocol support, real `--depth 1`, fast | Needs `git` binary |
| `go-git` (pure Go) | No binary dep, static builds | Slower, heavier memory, protocol edge cases |
| GitHub tarball API | Super fast, native token auth | GitHub-only, extra API call, REST rate limits |

PRD picks shelling out (section on Pipeline/Clone, line ~282 + line 696).
It's also the most educational — you touch `context`, `exec`, `os`, and the
Go "cleanup closure" idiom.
