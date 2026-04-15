# Go Enum Pattern Guide

## Why Go Doesn't Have Enums

Go has no `enum` keyword. Instead, you combine a **custom type** with **`const` blocks**.

## The Pattern: Custom Type + String Constants

```go
type JobStatus string

const (
    StatusQueued     JobStatus = "queued"
    StatusCloning    JobStatus = "cloning"
    StatusCompleted  JobStatus = "completed"
)
```

### Why `string` type instead of `iota` (int)?

Go's `iota` gives you auto-incrementing integers:

```go
const (
    StatusQueued JobStatus = iota  // 0
    StatusCloning                   // 1
)
```

Use `iota` when the value doesn't matter (flags, internal state).
Use `string` when the value must match external systems (DB columns, JSON, APIs).

Our job statuses must match `varchar(20)` in PostgreSQL, so `string` is the right choice.

### Why a custom type instead of raw `string`?

Type safety. Compare:

```go
// Raw strings — any string is accepted, typos compile fine
func UpdateStatus(id string, status string) { ... }
UpdateStatus(id, "queud")  // typo — compiles, breaks at runtime

// Custom type — wrong values are compile errors
func UpdateStatus(id string, status pipeline.JobStatus) { ... }
UpdateStatus(id, "queud")  // compile error: cannot use "queud" as JobStatus
```

### Adding Methods

You can attach methods to enum types:

```go
func (s JobStatus) IsActive() bool {
    switch s {
    case StatusCloning, StatusParsing, StatusGenerating:
        return true
    default:
        return false
    }
}
```

### Converting Between Types

Since the underlying type is `string`, conversion is straightforward:

```go
// Go type → DB string
dbStatus := string(pipeline.StatusQueued)  // "queued"

// DB string → Go type
goStatus := pipeline.JobStatus(job.Status)  // JobStatus("queued")

// Then use methods on it
if goStatus.IsActive() { ... }
```

## Where to Put Enums

Put them in the package that **owns the concept**:
- Job statuses → `internal/pipeline/` (pipeline owns the lifecycle)
- User roles → `internal/auth/`
- HTTP error codes → `internal/httputil/`

Avoid putting all enums in one `models` or `types` package — that creates a grab-bag that everything imports.
