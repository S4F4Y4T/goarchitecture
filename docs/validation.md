# Validation

## Tool: go-playground/validator

`pkg/validation` wraps `go-playground/validator/v10` — the de-facto standard Go struct validator.

```go
type CreateUserRequest struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
}
```

Call site in handlers:
```go
if err := validation.Validate(&req); err != nil {
    response.Error(w, r, err)
    return
}
```

`Validate()` returns `nil` on success or an `*apperror.AppError` with `INVALID_INPUT` code and per-field errors on failure.

## JSON Field Names in Error Messages

By default, `validator` reports errors using the **struct field name** (`Name`, `Email`). We register a custom tag name function that reads the `json:` tag instead:

```go
v.RegisterTagNameFunc(func(fld reflect.StructField) string {
    name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
    if name == "-" {
        return ""
    }
    return name
})
```

This means error messages reference `"email"` (the API field name) rather than `"Email"` (the Go field name). API consumers see field names that match what they sent.

## Human-Readable Error Messages

The validator returns tag names (`"required"`, `"email"`, `"min"`). We map them to readable strings:

| Tag | Message |
|---|---|
| `required` | `"Field is required"` |
| `email` | `"Field must be a valid email address"` |
| `min` | `"Field must be at least N characters long"` |
| `max` | `"Field must be at most N characters long"` |
| `gte` / `lte` | `"Field must be greater/less than or equal to N"` |
| (unknown tag) | `"Field is invalid"` |

## Where Validation Happens

Validation runs **only in the handler layer**, on DTOs. Domain models are never validated with struct tags — the domain layer trusts that the data it receives has already been validated at the boundary.

This means:
- Service methods don't call `Validate()`.
- Repository methods don't call `Validate()`.
- Only the HTTP entry point validates.

## What We Don't Validate with Struct Tags

- **Business rules** (e.g., "email must not already exist") — handled in the service layer with explicit checks.
- **Database constraints** (unique, foreign key) — caught by the repository and translated to `AppError{CONFLICT}`.
- **Query parameters** (pagination, filters) — clamped to valid ranges rather than rejected (see [pagination.md](pagination.md) and [filter-sort.md](filter-sort.md)).

## Alternatives Considered

- **Manual validation** — check each field with `if req.Email == ""`. Verbose, error-prone, inconsistent messages. Rejected.
- **ozzo-validation** — fluent API, no struct tags, more flexible. Slightly more code per struct. `go-playground/validator` is more widely known in the Go ecosystem.
- **Protobuf validation (protovalidate)** — only relevant if using protobuf. We use JSON/REST.
- **JSON Schema** — language-agnostic, good for polyglot systems. Requires a separate schema file and a JSON Schema library. Overhead not justified here.
