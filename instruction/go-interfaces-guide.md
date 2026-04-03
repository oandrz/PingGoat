# Go Interfaces Guide (with DocGoat examples)

## What is an interface?

An interface is a **contract** — it defines what methods something must have, not how they work.

Go checks interfaces **implicitly** — you never write `implements`. If your struct has the right methods with the right signatures, it satisfies the interface automatically.

---

## The pattern (3 pieces)

### 1. Interface — the contract

```go
type AuthHandler interface {
    Register(w http.ResponseWriter, r *http.Request)
    Login(w http.ResponseWriter, r *http.Request)
}
```

### 2. Struct — the implementation (unexported)

```go
type authHandler struct {
    queries   database.Queries
    jwtSecret string
}

func (h *authHandler) Register(w http.ResponseWriter, r *http.Request) {
    // actual logic
}

func (h *authHandler) Login(w http.ResponseWriter, r *http.Request) {
    // actual logic
}
```

### 3. Constructor — connects them

```go
func NewAuthHandler(queries database.Queries, jwtSecret string) AuthHandler {
    return &authHandler{
        queries:   queries,
        jwtSecret: jwtSecret,
    }
}
```

Return type is the **interface**, but you return a pointer to the **struct**. This forces consumers to use the interface, not the struct directly.

---

## Why use interfaces?

### Testability

Without interfaces, testing requires a real database. With interfaces, you create a mock:

```go
type mockAuthHandler struct{}

func (m *mockAuthHandler) Register(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusCreated)
}
```

The mock satisfies the same interface — pass it anywhere `AuthHandler` is expected.

### Decoupling

`main.go` only knows about `AuthHandler` (the interface). It doesn't know or care about the private `authHandler` struct or its fields. You can change the internals without touching `main.go`.

---

## Go's interface rules

1. **Implicit satisfaction** — no `implements` keyword. If the methods match, it works.
2. **Define where consumed, not where implemented** — if `pipeline/` needs to call Gemini, define the interface in `pipeline/`, not in `gemini/`.
3. **Keep interfaces small** — Go's best interfaces have 1-2 methods (`io.Reader`, `error`).
4. **Exported interface, unexported struct** — `AuthHandler` (exported) + `authHandler` (unexported) forces usage through the constructor.

---

## Where to define interfaces in DocGoat

| Consumer package | Needs to call... | Define interface in... |
|-----------------|------------------|----------------------|
| `pipeline/` | Gemini API | `pipeline/` (`LLMClient` interface) |
| `handler/` | Database queries | `handler/` |
| `main.go` | Handlers | `handler/` package (exported interfaces) |

**Why consumer-side?** Because the consumer owns the contract. If `pipeline/` defines `LLMClient`, you can swap Gemini for OpenAI without touching `pipeline/` — just make the new client satisfy the same interface.
