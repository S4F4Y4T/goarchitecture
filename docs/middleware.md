# Middleware

## Chain Order

```
Request →
  RequestID
  → Logger
  → CORS
  → RateLimit
  → PanicRecovery
  → handler
← Response
```

Each middleware wraps the next. The outermost middleware (RequestID) is the first to run on the way in and the last to run on the way out.

## `pkg/middleware` Chain Builder

```go
chain := middleware.Chain(
    middleware.RequestID,
    middleware.Logger,
    middleware.Cors(allowedOrigins),
    middleware.RateLimit(rdb, "global", requests, window),
    middleware.PanicRecovery,
)
handler := chain(router)
```

`Chain(m1, m2, m3)` applies in declaration order: m1 is outermost, m3 is innermost. Internally it reverses the slice before nesting.

## Each Middleware

### RequestID
Generates or reuses the `X-Request-ID` header. Sets it on both the request context and the response header.

**Position**: must be first so that all subsequent middleware and handlers can read the request ID from context.

### Logger
Creates a request-scoped logger with the request ID, stores it in context, then logs the access line after the handler returns.

**Position**: after RequestID (needs the ID) but before CORS and business logic (so all subsequent log calls have the request ID).

### CORS
Handles preflight `OPTIONS` requests and adds `Access-Control-*` headers to all responses.

**Position**: early, before rate limiting. Preflight requests should be fast and cheap. Rate-limiting a preflight would block browser CORS discovery.

### RateLimit
Checks the per-IP counter in Redis. Returns 429 if exceeded.

**Position**: after CORS (don't block preflights) but before the handler (reject abusive clients before they consume any business logic resources). A no-op if Redis is not configured.

### PanicRecovery
Wraps `recover()` around the rest of the chain. Logs the panic with stack trace and returns a 500 JSON error.

**Position**: innermost middleware (closest to the handler). Only needs to catch panics from the handler and inner middleware. Putting it outermost would also catch panics in RequestID/Logger, but those should never panic.

## Route-Specific Middleware

For middleware that applies to only one route (e.g., a stricter rate limit on `/login`):

```go
middleware.With(loginHandler, strictRateLimit)
```

`With(h, middlewares...)` wraps a single `http.HandlerFunc` without affecting other routes.

## Why Not a Framework's Middleware?

We use `net/http` directly. Framework middleware (Gin, Echo, Fiber) would work but lock us into the framework's request/response types. Standard library `http.Handler` middleware is portable and has no dependencies.

## Alternatives Considered

- **alice** — a popular middleware chain builder. Functionally identical to our `Chain()` helper. Not worth the import.
- **chi** — provides a `Use()` method for middleware chaining. Would require adopting chi as the router (see [router.md](router.md)).
- **Middleware as decorators on each route** — registering middleware per route explicitly. More verbose; better handled with `With()` for exceptional cases.
