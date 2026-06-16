# User Service — Clean Architecture & DDD

## Is It Clean Architecture?

**Yes.** The user service fully implements Clean Architecture with an additional DDD domain modelling layer on top. This is not the "Layered + DI at repo boundary" pattern described in `internal-architecture.md` as the baseline — it goes further.

The key upgrade: **every layer boundary is protected by an interface**, not just the repository boundary.

```
┌────────────────────────────────────────────────────────────┐
│  Delivery  (internal/delivery/http/)                       │
│  handler/ · router/ · middleware/                          │
│  Depends on: port.AuthUseCase, port.UserUseCase            │
└────────────────────────┬───────────────────────────────────┘
                         │ via interface (usecase/port/)
┌────────────────────────▼───────────────────────────────────┐
│  Use Case  (internal/usecase/)                             │
│  AuthService · UserService                                 │
│  Depends on: domain.Repository, token.Store (interfaces)  │
└────────────────────────┬───────────────────────────────────┘
                         │ via interface (domain/user/repository.go)
┌────────────────────────▼───────────────────────────────────┐
│  Domain  (internal/domain/user/)                           │
│  User entity · Email VO · Password VO · Repository iface  │
│  Zero framework imports                                    │
└────────────────────────┬───────────────────────────────────┘
                         │ implemented by
┌────────────────────────▼───────────────────────────────────┐
│  Infrastructure  (internal/infrastructure/)                │
│  persistence/ (GORM) · cache/ (Redis) · config/           │
└────────────────────────────────────────────────────────────┘
```

### The Dependency Rule

Every arrow points inward. No inner layer imports from an outer layer.

| Layer | Imports | Never imports |
|---|---|---|
| Domain | `pkg/query`, `pkg/pagination` only | usecase, delivery, infrastructure |
| Use Case | domain interfaces, `pkg/*` | GORM, Redis, `net/http` |
| Delivery | `usecase/port` interfaces, `pkg/*` | concrete use case structs, GORM |
| Infrastructure | domain interfaces, GORM, Redis | delivery, usecase |

---

## Folder Map

```
services/user/
├── cmd/api/main.go                        # entry point — boot only
└── internal/
    ├── bootstrap/
    │   └── app.go                         # manual DI: wires all layers
    ├── domain/
    │   └── user/
    │       ├── user.go                    # User entity + New() constructor
    │       ├── email.go                   # Email value object (validates on construction)
    │       ├── password.go                # Password value object (bcrypt on construction)
    │       └── repository.go             # Repository interface (owned by domain)
    ├── usecase/
    │   ├── port/
    │   │   ├── auth.go                    # AuthUseCase interface + input/output types
    │   │   └── user.go                    # UserUseCase interface + input/output types
    │   ├── auth.go                        # AuthService: Register, Login, Refresh, Logout
    │   └── user.go                        # UserService: CRUD + transactional update
    ├── delivery/
    │   └── http/
    │       ├── handler/
    │       │   ├── auth.go                # decodes HTTP → calls AuthUseCase interface
    │       │   ├── user.go                # decodes HTTP → calls UserUseCase interface
    │       │   └── health.go
    │       ├── middleware/
    │       │   └── auth.go                # reads X-User-ID from Kong → context
    │       └── router/
    │           ├── router.go
    │           ├── auth.go
    │           └── user.go
    ├── dto/
    │   └── user.go                        # HTTP request shapes (separate from domain)
    └── infrastructure/
        ├── persistence/
        │   └── user_repository.go         # GORM impl of domain.Repository
        ├── cache/
        │   └── token_store.go            # Redis impl of token.Store
        └── config/
            ├── config.go
            ├── database.go
            └── redis.go
```

---

## How DDD Contributes

DDD tactical patterns are applied at the domain layer. This is what makes the user service richer than plain Clean Architecture.

### Value Objects

Both `Email` and `Password` are typed strings that enforce their own invariants on construction. Invalid state is impossible to represent once past the constructor.

**Email** — validates RFC 5322 format via `mail.ParseAddress`. Implements `driver.Value` / `Scan` for transparent DB round-tripping and custom JSON marshalling.

```go
// Invalid email cannot exist as a domain value
email, err := user.NewEmail("not-an-email")  // returns error immediately
// If err == nil, email is guaranteed valid for its entire lifetime
```

**Password** — runs bcrypt on construction. The raw string never survives past `NewPassword`. JSON marshalling always emits `null`, preventing accidental leakage in API responses.

```go
password, err := user.NewPassword("plaintext")  // hashed immediately, plain discarded
password.Matches("plaintext")                   // compare without ever exposing the hash
```

These are DDD value objects in the strictest sense: identity-free, immutable after construction, carry their own invariants.

### Entity

`User` is the entity. It has identity (`ID`), can change over time, and is compared by identity rather than value. The `New()` constructor is the only way to create a valid user — it requires both an `Email` and `Password` value object, so a user cannot be constructed with an unvalidated email or a plaintext password.

```go
func New(name string, email Email, password Password) *User
```

### Repository Interface in Domain

`domain/user/repository.go` defines the `Repository` interface. The domain owns the contract; infrastructure conforms to it. This is the Repository pattern as described by Evans — a collection-like abstraction that makes the domain ignorant of persistence mechanics.

```go
// Domain defines what it needs
type Repository interface {
    GetByID(ctx context.Context, id int) (*User, error)
    Create(ctx context.Context, user *User) (*User, error)
    WithTx(ctx context.Context, fn func(Repository) error) error
    // ...
}
```

`infrastructure/persistence/user_repository.go` implements this with GORM. The domain never sees GORM.

### Use Case Ports

`usecase/port/` defines the inbound interfaces (`AuthUseCase`, `UserUseCase`) that handlers depend on. Handlers never import the concrete `AuthService` or `UserService` structs. This is the Ports and Adapters pattern applied to the use case boundary.

```go
// handler/auth.go depends only on this interface
type AuthUseCase interface {
    Register(ctx context.Context, input RegisterInput) (*user.User, error)
    Login(ctx context.Context, input LoginInput) (TokenPair, error)
    Refresh(ctx context.Context, refreshToken string) (TokenPair, error)
    Logout(ctx context.Context, refreshToken string) error
}
```

### What the Use Cases Own

Use cases orchestrate domain objects but contain no business rules of their own — rules live in the domain. The split is:

| Concern | Where it lives |
|---|---|
| Email format is valid | `domain/user/email.go` — `NewEmail()` |
| Password must be hashed | `domain/user/password.go` — `NewPassword()` |
| Password matches stored hash | `domain/user/password.go` — `Password.Matches()` |
| Email uniqueness | `usecase/auth.go` — checks via repository before creating |
| Token generation and rotation | `usecase/auth.go` — orchestrates `token.Generate` + `tokenStore` |
| Transactional read-modify-write | `usecase/user.go` — `Update()` uses `repo.WithTx` |

---

## What Is NOT DDD (and Why That Is Fine)

### No Domain Events

There are no `UserRegistered` or `UserDeleted` events. This is intentional for the current scope — no other service needs to react to user lifecycle changes yet. Adding events when there is no subscriber would be speculative design.

### No Aggregate Beyond User

`User` is both entity and aggregate root. There are no child entities owned by `User`. This is correct for the current domain — a user does not own orders, products, or any other domain objects in this service.

### No Domain Service

Uniqueness checking (`ExistsByEmail`) happens in the use case, not a domain service. A domain service makes sense when logic spans multiple aggregates or requires cross-cutting domain knowledge. Single-entity uniqueness checks in the use case are idiomatic in Go microservices and do not represent an architectural flaw.

---

## Suggestions

### 1. Remove `TableName()` from the Domain Entity

`user.go:18` has `func (User) TableName() string { return "users" }`. This is a GORM lifecycle hook leaking into the domain layer — the domain should have no knowledge of how it is stored.

Move it to the persistence layer:

```go
// infrastructure/persistence/user_repository.go
// Tell GORM the table name without touching the domain struct
db.NamingStrategy = schema.NamingStrategy{TablePrefix: ""}

// Or use a GORM tabler adapter in the repo, not the entity:
type userRecord struct {
    user.User
}
func (userRecord) TableName() string { return "users" }
```

Alternatively, since `users` is the conventional GORM plural of `User`, you can simply delete `TableName()` entirely and let GORM's default naming convention handle it.

### 2. Abstract Token Signing Behind an Interface

`usecase/auth.go:17` holds `privateKey *rsa.PrivateKey` directly. This means the use case is aware of RSA and the `token` package internals. Introduce a `TokenIssuer` interface:

```go
// usecase/port/auth.go (or domain/token/)
type TokenIssuer interface {
    Issue(userID int, expiry time.Duration) (string, error)
}
```

The `AuthService` depends on `TokenIssuer`, and the infrastructure provides an RSA implementation. This makes `AuthService` testable with a mock issuer and decoupled from the crypto library.

### 3. `UserService.Create` Creates a User With an Empty Password

`usecase/user.go:45`:
```go
return s.repo.Create(ctx, userDomain.New(input.Name, email, ""))
```

`New()` accepts `Password` typed as a plain `string` alias, so `""` passes through as a zero-value `Password`. This creates a user who can never log in (no password set), but the domain model does not express this intent.

Options in order of preference:

**a)** Add a `NewUserWithoutPassword` constructor that makes the passwordless state explicit:
```go
func NewWithoutPassword(name string, email Email) *User {
    return &User{Name: name, Email: email}
}
```

**b)** Make `Password` a pointer in the `User` struct so `nil` means "no password set", distinguishing it from an empty hash.

**c)** Add a domain method `(u *User) SetPassword(p Password)` used only by the admin flow.

### 4. `ListSchema` Belongs in the Layer That Uses It

`ListSchema` is a `query.Schema` that configures which columns can be filtered and sorted. It is only consumed by `delivery/http/handler/user.go` — it is not a domain rule. It now lives as a package-level var in the handler (`userListSchema`) and the domain has no dependency on `pkg/query`.

### 5. Consider a `UserID` Value Type

IDs are `int` throughout. Passing a bare `int` as a user ID can silently mix with other integer IDs (product IDs, order IDs). A named type costs nothing:

```go
// domain/user/user.go
type ID int

type User struct {
    ID        ID
    // ...
}
```

This makes `repo.GetByID(ctx, productID)` a compile error rather than a silent bug.

---

## Summary

| Question | Answer |
|---|---|
| Is this Clean Architecture? | Yes — every layer boundary is enforced by an interface |
| Is DDD applied? | Partially — value objects and repository pattern are correct DDD; no domain events or aggregate hierarchies |
| Is it appropriate for the service's complexity? | Yes — the current domain is CRUD-heavy; full tactical DDD would add indirection without payoff |
| What should change? | Remove `TableName()` from domain; fix passwordless user construction; move `ListSchema` out of domain |
