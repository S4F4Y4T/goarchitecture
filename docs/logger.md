# Logger

## Implementation: `log/slog` with JSON Output

`pkg/logger` wraps Go's standard `log/slog` (introduced in Go 1.21) with a JSON handler.

```go
logger.Init(os.Stdout, slog.LevelInfo)
log := logger.L()
log.Info("server started", "port", 6969)
// {"time":"...","level":"INFO","msg":"server started","port":6969}
```

Output is newline-delimited JSON — parseable by any log aggregator (Datadog, Loki, CloudWatch).

## Process-Level vs. Request-Scoped Logger

Two separate concepts:

**Process logger** (`logger.L()`): used for startup, shutdown, background tasks. One instance per process, initialized in `main.go`.

**Request-scoped logger** (`logger.FromContext(ctx)`): carries the request ID so every log line from a single request shares the same `request_id` field. Set up by the `Logger` middleware:

```go
// middleware/logger.go
reqLogger := logger.L().With("request_id", requestID)
ctx = logger.WithContext(r.Context(), reqLogger)
next.ServeHTTP(w, r.WithContext(ctx))
```

Handlers and services log via `logger.FromContext(ctx)` so their logs are automatically tagged with the request ID, without passing a logger argument down every call stack.

## Log Levels

| Level | Use |
|---|---|
| DEBUG | Verbose diagnostic info (disabled in production) |
| INFO | Normal operations (requests, startup/shutdown) |
| WARN | Unexpected but handled conditions (Redis down, unknown field in query) |
| ERROR | Errors that should trigger investigation (internal errors, panics) |

Level is configured via `LOG_LEVEL` env var (`"debug"`, `"info"`, `"warn"`, `"error"`). Default: `"info"`.

## Access Log

The Logger middleware emits one access log line per request after the handler returns:

```json
{
  "time": "...",
  "level": "INFO",
  "msg": "request completed",
  "request_id": "abc-123",
  "method": "GET",
  "path": "/v1/users/",
  "status": 200,
  "duration_ms": 14
}
```

Fields:
- `request_id` — for correlation across services
- `method` + `path` — what was requested
- `status` — HTTP status code written to the client
- `duration_ms` — handler latency

## Context Propagation

`logger.WithContext(ctx, l)` stores a `*slog.Logger` in the context under a package-private key. `logger.FromContext(ctx)` retrieves it, falling back to the process logger if not set. This means:

- Tests that don't set up a request context get the default logger.
- Production requests always have the request-scoped logger with `request_id`.
- No logger argument threads through every function signature.

## Alternatives Considered

- **zerolog** — very fast, zero-allocation JSON logger. `slog` is now part of the standard library and sufficient for our throughput. Avoids a third-party dependency.
- **zap (uber-go)** — high-performance, widely used. Same tradeoff: `slog` covers the use case without an import.
- **logrus** — older, reflection-based, slower. Being replaced by `slog` in the ecosystem.
- **`log.Printf`** — unstructured text logs. Not parseable by log aggregators. Rejected for production.
- **Passing logger via function arguments** — `service.CreateUser(ctx, logger, req)`. Makes every function signature messier and doesn't compose well. Context propagation is cleaner.
