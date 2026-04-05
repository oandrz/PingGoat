# Middleware in Go (with Chi)

## What is middleware?

A function that wraps an HTTP handler to run logic **before** and/or **after** the actual handler. Common uses: authentication, logging, CORS, rate limiting.

## The Pattern

```go
func MyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Before: runs before the handler
        log.Println("request received")

        next.ServeHTTP(w, r)  // call the next handler in the chain

        // After: runs after the handler (e.g., logging response time)
    })
}
```

**Why this signature?** Go's `http.Handler` interface has one method: `ServeHTTP(w, r)`. Middleware takes a handler, returns a handler — this makes them composable. You can stack them like layers.

## Applying Middleware in Chi

```go
// Apply to ALL routes
r.Use(loggingMiddleware)

// Apply to a GROUP of routes only
r.Group(func(r chi.Router) {
    r.Use(authMiddleware)       // only routes inside this group are protected
    r.Get("/profile", getProfile)
    r.Post("/jobs", createJob)
})
```

**Why `r.Group`?** It creates a sub-router that inherits the parent's middleware but lets you add more without affecting other routes. This is how you protect some routes while leaving others public.

## Passing Data via Context

Middleware often needs to pass data to handlers (e.g., the authenticated user ID). Go uses `context.Context` for this:

```go
// In middleware — store value
ctx := context.WithValue(r.Context(), userIDKey, "some-uuid")
next.ServeHTTP(w, r.WithContext(ctx))

// In handler — retrieve value
userID := r.Context().Value(userIDKey).(string)
```

**Why a custom context key type?** Using a raw string like `"userID"` risks collision if another package uses the same key. Define an unexported type:

```go
type contextKey string
const userIDKey contextKey = "userID"
```

This guarantees uniqueness — only your package can create values of type `contextKey`.

## JWT Auth Middleware Specifically

The flow:
1. Extract `Authorization: Bearer <token>` header
2. Parse the JWT and validate signature + expiry
3. Extract claims (user ID from `Subject`)
4. Store in context for downstream handlers
5. If anything fails → 401 Unauthorized (don't leak details about *why* it failed)

**Security note:** Always return the same generic error message for all auth failures. Don't say "token expired" vs "invalid signature" — that tells attackers what they're dealing with.

## Helper: Extract User ID in Handlers

Create a helper so handlers don't need to know about context key internals:

```go
func UserIDFromContext(ctx context.Context) string {
    id, _ := ctx.Value(UserIDKey).(string)
    return id
}
```

Handlers just call `middleware.UserIDFromContext(r.Context())`.
