# Error Handling

## The Problem with Raw Errors

Go's `error` interface is untyped. Without a convention, every layer ends up with ad-hoc checks like `if err != nil && err.Error() == "record not found"` — brittle, untestable, and leaks implementation details to HTTP handlers.

## AppError: Typed Application Errors

`pkg/apperror` defines a small set of error codes that map cleanly to HTTP status codes:

| Code | HTTP Status | Meaning |
|---|---|---|
| `NOT_FOUND` | 404 | Resource does not exist |
| `INVALID_INPUT` | 400 | Malformed request or failed validation |
| `CONFLICT` | 409 | Uniqueness or state conflict |
| `UNAUTHORIZED` | 401 | Authentication required |
| `FORBIDDEN` | 403 | Authenticated but not allowed |
| `TOO_MANY_REQUESTS` | 429 | Rate limit exceeded |
| `INTERNAL` | 500 | Unexpected server error |

```go
type AppError struct {
    Code    string
    Message string
    Fields  map[string]string // field-level validation errors, optional
    Cause   error             // original error, for logging only
}
```

Helper constructors make creation ergonomic:
```go
apperror.NotFound("user not found")
apperror.Conflict("email already exists")
apperror.InvalidInput("name is required")
```

## The Normalization Layer: `apperror.From()`

Every handler calls `response.Error(w, r, err)`, which internally calls `apperror.From(err)`. This single function normalizes any error:

```
gorm.ErrRecordNotFound  →  AppError{NOT_FOUND}
*AppError (any code)    →  passes through unchanged
any other error         →  AppError{INTERNAL, cause preserved for logging}
```

**Why centralize normalization?**  
Repositories and services return raw errors from GORM. If handlers had to check `errors.Is(err, gorm.ErrRecordNotFound)` in every endpoint, GORM would leak into the HTTP layer. `apperror.From()` is the single place that knows about GORM sentinels.

## Logging Internal Errors

`response.Error()` logs the original `Cause` at ERROR level before responding to the client. The client receives a generic `"internal server error"` message; the operator gets the full stack in structured logs. This avoids leaking implementation details (table names, query strings) to API consumers.

## Field-Level Errors

Validation errors include per-field messages:

```json
{
  "success": false,
  "error": {
    "code": "INVALID_INPUT",
    "message": "Validation failed",
    "fields": {
      "email": "email must be a valid email address",
      "name":  "name is required"
    }
  }
}
```

The `Fields map[string]string` on `AppError` is populated by the validation layer and passed through untouched to the response.

## Alternatives Considered

- **errors.As / sentinel errors** — standard Go pattern. Works but requires every caller to do the type assertion. Centralized `From()` is cleaner for the HTTP layer.
- **pkg/errors with stack traces** — adds stack traces to every error. Useful for debugging but heavy. We capture the cause in `AppError.Cause` and log it; a stack trace at the logging call site is sufficient.
- **HTTP status codes as errors** — some frameworks return `(data, httpStatus, error)`. Makes the service/repository layers aware of HTTP, violating separation of concerns. Rejected.
- **gRPC status codes** — would work if we used gRPC. We use HTTP; gRPC codes are a worse fit than our custom set.
