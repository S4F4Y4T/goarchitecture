# Repository

## Purpose

The repository is the **only place in the codebase that talks to the database**. It implements the `UserRepository` interface (defined in the model package) using GORM.

```go
type UserRepository struct {
    db *gorm.DB
}

func NewUserRepository(db *gorm.DB) model.UserRepository {
    return &UserRepository{db: db}
}
```

The return type is `model.UserRepository` (the interface), not `*UserRepository` (the concrete type). This ensures the caller can only use the repository through the interface.

## ORM: GORM

GORM is used for:
- Struct-to-column mapping via tags
- Connection pooling (via the underlying `database/sql`)
- Query building (scopes for pagination, filter, sort)
- Automatic `created_at` / `updated_at` management
- Error normalization (`gorm.ErrRecordNotFound`)

GORM is **not** used for:
- Migrations (golang-migrate handles that)
- Schema management (explicit SQL files own the schema)

## Key Implementation Decisions

### Explicit Column Selection on Update

```go
db.Model(&user).Select("name", "email").Updates(&input)
```

Without `Select()`, GORM skips zero-value fields by default (a `""` name would not be written). With `Select()`, all listed columns are always written, even if zero. This is required for full-replacement PUT semantics where the client explicitly sets a field to empty.

### Conflict Detection via Postgres Error Code

```go
func isUniqueViolation(err error) bool {
    var pgErr *pgconn.PgError
    return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
```

Postgres error code `23505` is the standard unique_violation code. Checking this specifically (rather than `err.Error()` string matching) is robust against Postgres version changes and locale differences.

### DeleteUser: Rows Affected Check

```go
result := db.Delete(&model.User{}, id)
if result.RowsAffected == 0 {
    return apperror.NotFound("user not found")
}
```

GORM's `Delete` does not return `gorm.ErrRecordNotFound` — it succeeds with zero rows affected if the ID doesn't exist. We check `RowsAffected` explicitly to distinguish "deleted" from "not found."

### GetAllUsers: Two-Phase Query

```go
db.Model(&model.User{}).Scopes(filters).Count(&total)
db.Model(&model.User{}).Scopes(filters, sorts).Limit(limit).Offset(offset).Find(&users)
```

Two queries:
1. Count with filters applied (no limit/offset) — to compute `total_pages`
2. Fetch with filters + sorts + pagination — to get the actual rows

The count query must apply the same filters as the fetch query so that `total` reflects the filtered result set, not all rows.

## Context Propagation

All repository methods accept `ctx context.Context` and pass it to GORM via `db.WithContext(ctx)`. This enables:
- Request cancellation (client disconnects → query is cancelled)
- Tracing (future: attach spans to the context)
- Query timeouts (set a deadline on the context before calling the repository)

## What the Repository Does NOT Do

- Business logic (email uniqueness checks are in the service)
- Validation (validated at the HTTP boundary)
- Error wrapping beyond `apperror.From()` — internal errors are returned as `INTERNAL` and logged by the response layer

## Alternatives Considered

- **sqlc** — generates type-safe Go code from SQL queries. Zero ORM magic, full SQL control. More verbose; requires maintaining SQL files for every query. Excellent choice if GORM's magic becomes a maintenance burden.
- **pgx directly** — lowest-level Postgres driver. Maximum control, maximum code to write. No query building, no struct scanning without a helper.
- **Raw `database/sql`** — standard library, no ORM. Same tradeoff as pgx; more boilerplate.
- **ent** — code-generation ORM. Strongly typed schemas, powerful query builder. Higher upfront investment (schema DSL). Good for complex domain models with many relationships.
