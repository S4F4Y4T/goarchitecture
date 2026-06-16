# User Service — Modular Monolith

## What This Service Actually Is

The user service is a **Modular Monolith**: one deployable binary containing two feature modules — `user` (CRUD) and `auth` (registration/login/token lifecycle) — plus a small `health` module, wired together by a single composition root (`internal/bootstrap/app.go`).

```
┌──────────────────────────────────────────────────────┐
│  router/  (net/http ServeMux + middleware chain)      │
└───────┬───────────────────┬────────────────┬──────────┘
        │                   │                │
┌───────▼──────┐   ┌────────▼──────┐  ┌──────▼──────┐
│  user/        │   │  auth/        │  │  health/    │
│  handler.go   │   │  handler.go   │  │  handler.go │
│  service.go   │   │  service.go   │  │             │
│  repository.go│   │  token_store.go│ │             │
│  model.go     │   │  dto.go        │  │             │
│  dto.go       │   │               │  │             │
└──────┬────────┘   └──────┬────────┘  └─────────────┘
       │ Repository iface  │ auth.UserLookup iface
       │                   │ (satisfied by user.Repository)
┌──────▼───────────────────▼─────┐
│   Postgres (GORM)  ·  Redis     │
└──────────────────────────────────┘
```

Each module owns its own model, repository, service, and handler — there is no service-wide `model/`, `repository/`, `service/`, `handler/` split. `bootstrap.Register` is the only place that knows about every module at once; it constructs the shared `user.Repository` and hands it to both `user.NewUserService` and `auth.NewAuthService`.

---

## Folder Map

```
services/user/
├── cmd/api/main.go             # entry point — boot only
└── internal/
    ├── bootstrap/
    │   └── app.go              # composition root: builds repo, services, handlers
    ├── config/
    │   ├── config.go
    │   ├── database.go
    │   └── redis.go
    ├── user/
    │   ├── model.go            # User struct + Repository interface + ListSchema
    │   ├── repository.go       # GORM impl of Repository
    │   ├── service.go          # CRUD + transactional Update
    │   ├── handler.go          # GetAll, GetByID, Create, Update, Delete
    │   └── dto.go               # CreateUserRequest, UpdateUserRequest
    ├── auth/
    │   ├── dto.go               # RegisterDTO, LoginDTO
    │   ├── handler.go           # Register, Login, Refresh, Logout
    │   ├── service.go           # bcrypt hash/compare, token issue/rotate
    │   └── token_store.go       # Redis impl of token.Store (refresh tokens)
    ├── health/
    │   └── handler.go           # Live, Ready
    └── router/
        ├── router.go
        ├── auth.go
        └── user.go
```

---

## The `user` Module

`internal/user/model.go` defines both the `User` struct and the `Repository` interface in the same package:

```go
type User struct {
    ID        int
    Name      string
    Email     string
    Password  string `json:"-"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

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

`internal/user/repository.go` implements `Repository` with GORM (`UserRepository`). The module owns the contract; the implementation just has to conform to it — this is the one place dependency-inversion is applied.

`User` is an **anemic** struct: no methods, no invariants, no value objects. Uniqueness checks and update orchestration live in `internal/user/service.go`, not on the model. That's a deliberate simplification for CRUD-heavy code, not an oversight — see "What's Missing for DDD" below.

---

## The `auth` Module

`internal/auth/service.go` depends on `user.User` (the type) and a small interface it owns itself, `UserLookup`, rather than the full `user.Repository`:

```go
type UserLookup interface {
    ExistsByEmail(ctx context.Context, email string) (bool, error)
    GetByEmail(ctx context.Context, email string) (*user.User, error)
    Create(ctx context.Context, u *user.User) (*user.User, error)
}

type AuthService struct {
    repo          UserLookup
    tokenStore    token.Store
    tokenIssuer   token.AccessIssuer
    accessExpiry  time.Duration
    refreshExpiry time.Duration
}
```

`*user.UserRepository` already implements `ExistsByEmail`, `GetByEmail`, and `Create`, so it satisfies `UserLookup` implicitly — `bootstrap` passes the same repository instance to both `user.NewUserService` and `auth.NewAuthService` with no extra adapter. This keeps `auth`'s coupling to `user` limited to exactly the three operations it needs, instead of the entire CRUD surface. It's still an in-process Go interface, not a network boundary — splitting `auth` into its own service would still mean replacing this with an HTTP/gRPC call to `user` — but the contract is already minimal, so that future change touches one interface, not every call site.

`internal/auth/token_store.go` (`RedisTokenStore`) implements `token.Store` from `pkg/token` — refresh tokens are stored in Redis as `refresh:<token> → userID`, independent of the `user` module's Postgres-backed repository.

---

## What Is and Isn't DDD Here

| Pattern | Present? |
|---|---|
| Entity with identity | Yes — `User`, compared by `ID` |
| Value Objects (`Email`, `Password`) | **No** — both are plain `string` fields |
| Aggregate beyond the entity itself | No — `User` owns no child entities |
| Domain events (`UserRegistered`, etc.) | No — nothing currently needs to react to user lifecycle changes |
| Repository pattern | Yes — `user.Repository`, owned by the domain, implemented by infrastructure |
| Domain service (logic spanning multiple aggregates) | No — single-entity checks live in the module's service |

This service is intentionally **DDD-lite**: it gets the repository pattern's testability/inversion benefit without paying for value objects, aggregates, or events that have no current use case. If business rules around `User` grow (e.g. password policy, email verification workflows), reintroducing `Email`/`Password` as value objects is the natural next step — see [[internal-architecture.md]] alternative #4.

---

## Suggestions

### 1. Consider a `UserID` Value Type

IDs are bare `int` throughout. A named type (`type ID int`) costs nothing and turns "passed a product ID where a user ID was expected" into a compile error instead of a silent bug.

### 2. Value Objects Only If Rules Grow

Don't add `Email`/`Password` value objects speculatively — the current validation (`validate:"required,email"` on DTOs, bcrypt in the service) is adequate for CRUD-level rules. Revisit if password policy or email-verification logic grows beyond what fits comfortably in `auth.Service`.

---

## Summary

| Question | Answer |
|---|---|
| What architecture is this? | Modular Monolith — package-by-feature, one binary, modules wired by `bootstrap` |
| Is it Clean Architecture? | No — handlers call concrete `*Service` structs, no Use Case interfaces |
| Is DDD applied? | Partially — repository pattern only; no value objects, aggregates, or events |
| Is it appropriate for the service's complexity? | Yes — CRUD + auth, no workflows or invariants complex enough to need more |
| What should change next? | Optional: a `UserID` value type; value objects only if password/email rules grow |
