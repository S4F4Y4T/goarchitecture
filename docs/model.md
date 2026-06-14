# Model & Repository Interface

## Domain Model

The domain model (`internal/model/`) is the canonical representation of a resource within a service. It is what the service layer operates on and what GORM maps to database rows.

```go
// internal/model/user.go
type User struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
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

## Repository Interface on the Model Layer

The repository interface is defined **in the model package**, not in the repository package:

```go
// internal/model/user.go
type UserRepository interface {
    GetUserByID(ctx context.Context, id uint) (*User, error)
    GetAllUsers(ctx context.Context, p pagination.Params, q query.Options) ([]User, int64, error)
    CreateUser(ctx context.Context, user *User) (*User, error)
    UpdateUser(ctx context.Context, id uint, user *User) (*User, error)
    DeleteUser(ctx context.Context, id uint) error
    ExistsByEmail(ctx context.Context, email string) (bool, error)
}
```

## Why the Interface Lives in the Model Package?

This follows the **dependency inversion principle**: the service layer imports `model.UserRepository` (an abstraction), not `repository.UserRepository` (a concrete type). The repository package imports the model package to implement the interface — not the other way around.

Dependency flow:
```
handler → service → model.UserRepository (interface)
                         ↑ implemented by
                    repository.UserRepository (concrete, imports model)
```

If the interface were in the repository package, the service would import the repository package, creating a tight coupling (and potentially a circular import).

## Query Schema on the Model

The allowlist for filter/sort parameters is also defined on the model:

```go
var UserListSchema = query.Schema{
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
- **Interface in the service package** — service defines what it needs. Valid but less discoverable; the handler needs to know the interface too (to pass it via DI).
- **No interface at all** — service holds a `*repository.UserRepository` directly. Cannot mock for testing; tightly coupled to the GORM implementation. Rejected.
