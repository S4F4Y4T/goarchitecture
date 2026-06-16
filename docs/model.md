# Model & Repository Interface

## Domain Model

The domain model lives inside its feature module (e.g. `internal/user/model.go`), not in a service-wide `model/` package — see [internal-architecture.md](internal-architecture.md) for why the codebase is organized by feature rather than by layer. The model is the canonical representation of a resource within a module. It is what the module's service operates on and what GORM maps to database rows.

```go
// internal/user/model.go
type User struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Password  string    `json:"-"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

Models carry:
- `gorm:` tags for ORM column mapping and constraints
- `json:` tags for HTTP response serialization

Models do **not** carry:
- `validate:` tags (those belong on DTOs)
- Business logic methods (those belong in the service)

## Repository Interface in the Same Module

The repository interface is defined **in the module's own package**, alongside the struct it describes — `internal/user/model.go` defines both `User` and `Repository`; `internal/user/repository.go` (same package) provides the GORM implementation:

```go
// internal/user/model.go
type Repository interface {
    GetByID(ctx context.Context, id int) (*User, error)
    GetAll(ctx context.Context, p pagination.Params, opts query.Options) ([]User, int64, error)
    Create(ctx context.Context, user *User) (*User, error)
    Update(ctx context.Context, id int, user *User) (*User, error)
    Delete(ctx context.Context, id int) error
    ExistsByEmail(ctx context.Context, email string) (bool, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
    WithTx(ctx context.Context, fn func(Repository) error) error
}
```

## Why the Interface Lives Next to the Struct?

This follows the **dependency inversion principle**: the module's service depends on `user.Repository` (an abstraction it owns), not on the concrete `UserRepository` GORM struct directly. Because both the interface and its implementation live in the same package, other modules (and `bootstrap`) only ever see `user.Repository` when they need it.

Dependency flow:
```
handler → service → user.Repository (interface)
                         ↑ implemented by
                    user.UserRepository (concrete GORM struct, same package)
```

`auth` consumes user data through its own narrower interface, `auth.UserLookup` (`ExistsByEmail`, `GetByEmail`, `Create`), rather than depending on the full `user.Repository`. `*user.UserRepository` satisfies `UserLookup` implicitly, so no adapter code is needed — see [internal-architecture.md](internal-architecture.md).

## Query Schema on the Model

The allowlist for filter/sort parameters is also defined on the model:

```go
// internal/user/model.go
var ListSchema = query.Schema{
    "id":         {Column: "id",    Sortable: true, Filterable: true},
    "name":       {Column: "name",  Sortable: true, Filterable: true, Partial: true},
    "email":      {Column: "email", Sortable: true, Filterable: true, Partial: true},
    "created_at": {Column: "created_at", Sortable: true},
    "updated_at": {Column: "updated_at", Sortable: true},
}
```

**Why on the model?** The schema describes what query capabilities the User resource exposes. It is a property of the domain concept, not of the HTTP handler or repository. Placing it on the model makes it co-located with the struct it describes.

## GORM Tags

| Tag | Purpose |
|---|---|
| `gorm:"primaryKey"` | marks the primary key column |
| `gorm:"uniqueIndex"` | ensures uniqueness + creates an index |
| `gorm:"not null"` | column-level NOT NULL constraint |
| `gorm:"column:x"` | explicit column name override |

GORM auto-handles `CreatedAt` and `UpdatedAt` if the fields are named exactly that — it sets them on create and updates `UpdatedAt` on save.

## Alternatives Considered

- **Interface in a separate `port/` package** — hexagonal architecture style (`port/repository.go`). More explicit about "this is a port." Adds a package for no practical benefit at this scale.
- **Interface in a service-wide `model/` package, separate from the implementation** — the layered-architecture approach this codebase used before adopting the modular monolith. Works, but spreads one feature across more top-level packages than necessary. Rejected in favor of package-by-feature — see [internal-architecture.md](internal-architecture.md).
- **No interface at all** — service holds a `*UserRepository` directly. Cannot mock for testing; tightly coupled to the GORM implementation. Rejected.
