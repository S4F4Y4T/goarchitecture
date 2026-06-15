# Per-Service Internal Architecture

## What We Use

This codebase uses **Layered Architecture with Dependency Inversion at the repository boundary** — a pragmatic middle ground between plain layers and full Clean Architecture.

### The Four Layers

```
┌──────────────────────────────────────────────────────┐
│  Delivery Layer                                      │
│  handler/ · router/ · middleware/                    │
│  Decodes HTTP input, calls service, writes response  │
└───────────────────┬──────────────────────────────────┘
                    │ calls concrete struct
┌───────────────────▼──────────────────────────────────┐
│  Business Logic Layer                                │
│  service/                                            │
│  Orchestrates domain rules, transactions, bcrypt     │
└───────────────────┬──────────────────────────────────┘
                    │ calls interface defined in model/
┌───────────────────▼──────────────────────────────────┐
│  Domain Layer                                        │
│  model/                                              │
│  Structs + repository interfaces + query schemas     │
└───────────────────┬──────────────────────────────────┘
                    │ implemented by
┌───────────────────▼──────────────────────────────────┐
│  Infrastructure Layer                                │
│  repository/ · config/                               │
│  GORM, Redis, Postgres, env-var loading              │
└──────────────────────────────────────────────────────┘
```

### Folder Structure

```
services/user/
├── cmd/api/main.go             # entry point — boot only, no logic
└── internal/
    ├── bootstrap/
    │   └── app.go              # manual DI: wire repo → service → handler
    ├── config/
    │   ├── config.go           # Config struct + env loading
    │   ├── database.go         # GORM setup + connection pool
    │   └── redis.go            # Redis client setup
    ├── dto/
    │   └── user.go             # CreateUserRequest, RegisterDTO, LoginDTO …
    ├── handler/
    │   ├── auth.go             # Register, Login, Refresh, Logout
    │   ├── user.go             # GetAll, GetByID, Create, Update, Delete
    │   └── health.go           # /healthz, /readyz
    ├── middleware/
    │   └── auth.go             # reads X-User-ID injected by Kong → ctx
    ├── model/
    │   └── user.go             # User struct + UserRepository interface
    ├── repository/
    │   ├── user.go             # GORM impl of model.UserRepository
    │   └── token.go            # Redis impl of token.Store
    ├── router/
    │   ├── router.go           # root mux + middleware chain
    │   ├── auth.go             # /auth/* routes
    │   └── user.go             # /users/* routes + auth middleware
    └── service/
        ├── auth.go             # Register (bcrypt hash), Login (bcrypt compare)
        └── user.go             # CRUD + UpdateUser transaction
```

### Why "Layered with DI" and not plain Layered?

In classic Layered Architecture the repository interface belongs to the data-access layer — handlers and services import downward into the database layer.

Here the `model/` package defines `UserRepository`. The `repository/` package imports `model/` and implements it. The domain owns the contract; infrastructure conforms to it. Services never import GORM or Redis directly.

```
model.UserRepository  ← interface (domain owns it)
       ↑
repository.UserRepository  ← GORM implementation (infrastructure conforms)
```

### What it is not

- Not full **Clean Architecture** — handlers call `*service.UserService` concretely, not through an interface. There is no Use Cases / Interactors boundary layer.
- Not **Hexagonal** — there are no named input/output ports; the boundary exists only at the repository layer.
- Not **Vertical Slice** — code is organized by layer, not by feature.

---

## Alternatives

### 1. Plain Layered Architecture

```
handler/ → service/ → repository/ → DB
```

Repository interfaces live in `repository/`, not in `model/`. Services import the data layer directly.

### Folder Structure

```
services/user/
├── cmd/api/main.go
└── internal/
    ├── config/
    │   ├── config.go
    │   ├── database.go
    │   └── redis.go
    ├── handler/
    │   ├── auth.go
    │   ├── user.go
    │   └── health.go
    ├── middleware/
    │   └── auth.go
    ├── model/
    │   └── user.go             # plain struct only — no interface here
    ├── repository/
    │   ├── user.go             # interface + GORM impl in the same package
    │   └── token.go
    ├── router/
    │   ├── router.go
    │   ├── auth.go
    │   └── user.go
    └── service/
        ├── auth.go             # imports repository.UserRepository directly
        └── user.go
```

The key difference from the current approach: `model/user.go` is just a struct file. The `UserRepository` interface moves into `repository/user.go` next to its implementation. Services import `repository` instead of `model` for the interface.

**Adds:** Nothing over what we have. Slightly simpler — one fewer package to navigate.

**Loses:** The data layer can now leak ORM types upward. Harder to test services without real GORM structs.

**Choose when:** The service is very small (< 5 endpoints), has no complex business rules, and you want the absolute minimum structure.

---

### 2. Clean Architecture (Uncle Bob)

Adds two things we currently skip:

1. **Interfaces at every layer boundary** — handlers call a `UserServicePort` interface, not `*service.UserService`. Every boundary is invertible.
2. **Use Cases / Interactors** — a dedicated layer between handlers and domain that contains one struct per business operation (`RegisterUseCase`, `LoginUseCase`, etc.).

```
┌────────────────────────────────────┐
│  Delivery (handler, router)        │  ← depends on Use Case interfaces
└───────────────┬────────────────────┘
                │ via interface
┌───────────────▼────────────────────┐
│  Use Cases / Interactors           │  ← one struct per operation
└───────────────┬────────────────────┘
                │ via interface
┌───────────────▼────────────────────┐
│  Entities / Domain                 │  ← pure business rules, no framework
└───────────────┬────────────────────┘
                │ interface defined here, implemented outward
┌───────────────▼────────────────────┐
│  Infrastructure (DB, Redis, HTTP)  │
└────────────────────────────────────┘
```

### Folder Structure

```
services/user/
├── cmd/api/main.go
└── internal/
    ├── domain/
    │   ├── entity/
    │   │   └── user.go             # User struct, pure — no framework imports
    │   └── port/
    │       ├── user_repository.go  # UserRepository interface
    │       └── token_store.go      # TokenStore interface
    │
    ├── usecase/
    │   ├── port/
    │   │   ├── auth_usecase.go     # AuthUseCase interface (handlers depend on this)
    │   │   └── user_usecase.go     # UserUseCase interface
    │   ├── auth/
    │   │   ├── register.go         # RegisterUseCase struct + Execute()
    │   │   ├── login.go            # LoginUseCase struct + Execute()
    │   │   ├── refresh.go
    │   │   └── logout.go
    │   └── user/
    │       ├── get_all.go
    │       ├── get_by_id.go
    │       ├── create.go
    │       ├── update.go
    │       └── delete.go
    │
    ├── delivery/
    │   └── http/
    │       ├── handler/
    │       │   ├── auth.go         # calls usecase.AuthUseCase interface
    │       │   ├── user.go         # calls usecase.UserUseCase interface
    │       │   └── health.go
    │       ├── middleware/
    │       │   └── auth.go
    │       └── router/
    │           ├── router.go
    │           ├── auth.go
    │           └── user.go
    │
    └── infrastructure/
        ├── persistence/
        │   └── user_repository.go  # GORM impl of domain/port.UserRepository
        ├── cache/
        │   └── token_store.go      # Redis impl of domain/port.TokenStore
        └── config/
            ├── config.go
            ├── database.go
            └── redis.go
```

**Adds:** Every layer is mockable without a framework. New delivery mechanisms (gRPC, CLI, queue consumer) plug in without touching business logic. Compiler enforces every architectural boundary.

**Loses:** Significantly more files and interfaces. A `CreateProduct` operation that today spans ~40 lines of handler + service code becomes a handler, a use case interface, a use case struct, a request/response object, and a presenter. Refactoring across boundaries is slower.

**Choose when:** The service has complex, branching business rules. Multiple delivery mechanisms exist (HTTP + gRPC + CLI). You have a team of 4+ developers who need the compiler to enforce boundaries, not code review.

---

### 3. Hexagonal Architecture (Ports and Adapters)

The framing differs from Clean Architecture even though the structure is similar. The domain is at the center. Everything outside is an adapter connecting through named ports (interfaces). The key terminology: adapters that drive the application are **inbound** (e.g., HTTP handler); adapters the application drives are **outbound** (e.g., Postgres, Redis).

```
         ┌─────────────────────────────────────┐
         │              Domain                  │
         │  (pure Go, zero imports, no HTTP)    │
         │                                      │
         │  ┌──────────┐    ┌──────────────┐   │
         │  │ inbound  │    │  outbound    │   │
         │  │ ports    │    │  ports       │   │
         │  │(driving) │    │ (driven)     │   │
         │  └────┬─────┘    └──────┬───────┘   │
         └───────┼─────────────────┼───────────┘
                 │                 │
        ┌────────▼──────┐ ┌────────▼──────┐
        │ HTTP Adapter  │ │  DB Adapter   │
        │  (inbound)    │ │  (outbound)   │
        └───────────────┘ └───────────────┘
```

### Folder Structure

```
services/user/
├── cmd/api/main.go
└── internal/
    ├── domain/
    │   ├── user.go                 # User entity — pure Go, no imports
    │   ├── email.go                # Email value object with validation method
    │   ├── password.go             # Password value object with bcrypt methods
    │   └── port/
    │       ├── inbound/
    │       │   ├── auth_service.go # interface: Register, Login, Refresh, Logout
    │       │   └── user_service.go # interface: GetAll, GetByID, Create, Update, Delete
    │       └── outbound/
    │           ├── user_repo.go    # interface: persistence operations
    │           └── token_store.go  # interface: token save/lookup/delete
    │
    ├── adapter/
    │   ├── inbound/
    │   │   └── http/
    │   │       ├── handler/
    │   │       │   ├── auth.go     # implements nothing; calls inbound port
    │   │       │   ├── user.go
    │   │       │   └── health.go
    │   │       ├── middleware/
    │   │       │   └── auth.go
    │   │       └── router/
    │   │           └── router.go
    │   └── outbound/
    │       ├── postgres/
    │       │   └── user_repo.go    # implements outbound/user_repo.go port
    │       └── redis/
    │           └── token_store.go  # implements outbound/token_store.go port
    │
    ├── app/
    │   ├── auth_service.go         # implements inbound/auth_service.go port
    │   └── user_service.go         # implements inbound/user_service.go port
    │
    └── config/
        ├── config.go
        ├── database.go
        └── redis.go
```

**Adds:** Domain code has zero framework imports. You can test the entire domain in-memory with no database at all. Swap Postgres for SQLite for tests; swap HTTP for gRPC without touching the domain.

**Loses:** Explicit port/adapter naming discipline requires consistent team buy-in. In Go the file structure is very similar to Clean Architecture — the main difference is the vocabulary (ports/adapters vs use-cases/entities) and the stricter zero-import rule on the domain.

**Choose when:** You want the strictest possible isolation of business rules from all frameworks. Good when the domain logic is the most valuable and long-lived part of the system, and infrastructure is expected to change (e.g., migrating DB engines, adding new transport protocols).

---

### 4. Vertical Slice Architecture

Abandon layer-based folders entirely. Organize by **feature** instead. Each slice is a self-contained mini-stack from HTTP handler to DB query.

### Folder Structure

```
services/user/
├── cmd/api/main.go
└── internal/
    ├── register/
    │   ├── handler.go          # HTTP handler for POST /auth/register
    │   ├── service.go          # bcrypt hash + create user
    │   ├── repository.go       # ExistsByEmail + CreateUser queries
    │   └── dto.go              # RegisterRequest, RegisterResponse
    │
    ├── login/
    │   ├── handler.go          # HTTP handler for POST /auth/login
    │   ├── service.go          # bcrypt compare + issue token pair
    │   └── dto.go              # LoginRequest, LoginResponse
    │
    ├── refresh/
    │   ├── handler.go          # POST /auth/refresh
    │   └── service.go          # Redis lookup + token rotation
    │
    ├── logout/
    │   ├── handler.go          # POST /auth/logout
    │   └── service.go          # Redis delete + clear cookie
    │
    ├── getuser/
    │   ├── handler.go          # GET /users/{id}
    │   ├── repository.go       # SELECT by id
    │   └── dto.go
    │
    ├── listusers/
    │   ├── handler.go          # GET /users
    │   ├── repository.go       # SELECT with filter/sort/page
    │   └── dto.go
    │
    ├── createuser/
    │   ├── handler.go          # POST /users
    │   ├── service.go          # duplicate email check + insert
    │   ├── repository.go
    │   └── dto.go
    │
    ├── updateuser/
    │   ├── handler.go          # PUT /users/{id}
    │   ├── service.go          # tx: fetch → check email → update
    │   ├── repository.go
    │   └── dto.go
    │
    ├── deleteuser/
    │   ├── handler.go          # DELETE /users/{id}
    │   └── repository.go
    │
    └── shared/                 # only extract here when 3+ slices need it
        ├── model/
        │   └── user.go         # shared User struct
        ├── middleware/
        │   └── auth.go         # X-User-ID → ctx
        └── router/
            └── router.go       # registers all slice handlers
```

Each slice owns its handler, business logic, and DB query. Slices import from `pkg/` for cross-cutting concerns (pagination, response, apperror) but not from each other.

**Adds:** Adding or deleting a feature is self-contained — one folder, no ripple across layers. Cognitive load per change is low. Avoids the "which layer does this belong to?" question. Scales well when features are numerous and largely independent.

**Loses:** Shared logic (e.g., `GetUserByID` used by both `updateuser` and `getuser`) must either be duplicated or extracted into `shared/`, which recreates a partial layer anyway. Harder to enforce consistent patterns across slices. Go's package-per-directory model means each slice becomes a separate package, which is verbose.

**Choose when:** The service has many loosely-related features (e.g., a BFF or admin dashboard). Features are added and removed frequently. You find yourself saying "I just need to change this one thing" but layers force you to touch five files.

---

### 5. Domain-Driven Design (DDD — Tactical Patterns)

Enriches the domain layer with DDD building blocks layered on top of any structural style. Applied on top of Layered or Clean Architecture — it is not a structural pattern on its own, it is a domain-modelling discipline.

| Building Block | What it is | Example here |
|---|---|---|
| **Entity** | Object with identity | `User`, `Product` |
| **Value Object** | Immutable, identity-free | `Email`, `Price`, `Money` |
| **Aggregate** | Cluster with one root | `Order` owns `OrderLine[]` |
| **Domain Service** | Logic that doesn't fit one entity | `PricingService` |
| **Repository** | Collection abstraction | Already present |
| **Domain Event** | Something that happened | `UserRegistered`, `OrderPlaced` |

In the current codebase `User` and `Product` are anemic — plain structs with no methods that enforce invariants. Business rules (e.g., "email must be unique") live in `service/`, not on the model itself.

### Folder Structure

```
services/user/
├── cmd/api/main.go
└── internal/
    ├── domain/
    │   ├── user/
    │   │   ├── user.go             # User aggregate root with behaviour methods
    │   │   ├── email.go            # Email value object — NewEmail() validates format
    │   │   ├── password.go         # Password value object — Hash(), Matches()
    │   │   ├── event.go            # UserRegistered, UserDeleted domain events
    │   │   └── repository.go       # UserRepository interface
    │   └── token/
    │       ├── token.go            # RefreshToken value object
    │       └── store.go            # TokenStore interface
    │
    ├── application/                # thin orchestration — no business rules here
    │   ├── auth_service.go         # Register(), Login(), Refresh(), Logout()
    │   └── user_service.go         # GetAll(), GetByID(), Create(), Update(), Delete()
    │
    ├── infrastructure/
    │   ├── persistence/
    │   │   └── user_repository.go  # GORM impl of domain/user.UserRepository
    │   └── cache/
    │       └── token_store.go      # Redis impl of domain/token.TokenStore
    │
    ├── delivery/
    │   └── http/
    │       ├── handler/
    │       │   ├── auth.go
    │       │   ├── user.go
    │       │   └── health.go
    │       ├── middleware/
    │       │   └── auth.go
    │       └── router/
    │           └── router.go
    │
    └── config/
        ├── config.go
        ├── database.go
        └── redis.go
```

What changes in `domain/user/user.go` vs the current `model/user.go`:

```go
// Current (anemic): plain struct, rules live in service/
type User struct {
    ID    int
    Email string
}

// DDD: aggregate enforces its own rules
type User struct {
    id       int
    email    Email     // value object — already validated
    password Password  // value object — hashed
}

func NewUser(name string, email Email, password Password) (*User, error) {
    // invariants enforced here, not in service/
}

func (u *User) ChangeEmail(email Email) error { ... }
```

**Adds:** Models enforce their own invariants. Rich value types (`Email`, `Password`) make invalid state unrepresentable at compile time. Domain events enable loose coupling between aggregates without direct imports.

**Loses:** More types, more indirection. Most useful when the domain is genuinely complex with many invariants. Overkill for simple CRUD.

**Choose when:** The service has a rich, rules-heavy domain — not just CRUD but workflows, state machines, or multi-step business processes (e.g., an orders or billing service).

---

### 6. CQRS (Command Query Responsibility Segregation)

Split the request path into two: **Commands** (writes, go through domain model + business rules) and **Queries** (reads, bypass the domain model and query the DB directly for a DTO projection).

```
┌──────────────────────┐          ┌───────────────────────┐
│    Command Side       │          │     Query Side         │
│                       │          │                        │
│  RegisterUser         │  write   │  GetAllUsers           │
│  UpdateUser    ───────┼─────────▶│  GetUserByID           │
│  DeleteUser           │  sync /  │                        │
│                       │  event   │  Direct SQL → DTO      │
│  domain model used    │          │  no domain model       │
│  full business rules  │          │  no ORM overhead       │
└──────────────────────┘          └───────────────────────┘
```

In its simplest form (single DB, no event bus): commands go through `service/ → repository/`, queries go through a dedicated read repository that returns DTOs directly from SQL — no `User` struct instantiated, no GORM model scanning.

### Folder Structure

```
services/user/
├── cmd/api/main.go
└── internal/
    ├── command/                    # write side — full domain model + rules
    │   ├── handler/
    │   │   └── auth.go             # POST /auth/register, /login, /refresh, /logout
    │   ├── service/
    │   │   ├── auth.go             # Register (bcrypt), Login (bcrypt compare)
    │   │   └── user.go             # Create, Update (tx), Delete
    │   └── repository/
    │       ├── user.go             # write operations: Insert, Update, Delete
    │       └── token.go            # Redis token store
    │
    ├── query/                      # read side — direct SQL, returns DTOs
    │   ├── handler/
    │   │   └── user.go             # GET /users, GET /users/{id}
    │   ├── dto/
    │   │   └── user.go             # UserView, UserListItem — read-optimised shapes
    │   └── repository/
    │       └── user.go             # GetByID, GetAll — raw SQL or GORM scan into DTOs
    │
    ├── domain/
    │   └── user.go                 # User struct + UserRepository interface (write side)
    │
    ├── delivery/
    │   └── http/
    │       ├── middleware/
    │       │   └── auth.go
    │       └── router/
    │           └── router.go       # wires both command and query handlers
    │
    └── config/
        ├── config.go
        ├── database.go
        └── redis.go
```

The query repository returns a flat DTO directly — no domain model is loaded:

```go
// query/repository/user.go
func (r *UserReadRepo) GetAll(ctx context.Context, p pagination.Params) ([]dto.UserView, int64, error) {
    // Raw scan into DTO — no model.User instantiated, no business logic
    var views []dto.UserView
    r.db.Raw("SELECT id, name, email, created_at FROM users LIMIT ? OFFSET ?",
        p.Limit, p.Offset()).Scan(&views)
    ...
}
```

**Adds:** Read paths become simple, fast SQL projections with no business logic overhead. Write paths stay protected by business rules. Each side can be optimised (and eventually scaled) independently. Natural stepping stone toward Event Sourcing.

**Loses:** Two parallel paths to maintain. More indirection for what are simple lookups. Full CQRS with separate read/write databases adds eventual consistency complexity.

**Choose when:** Read traffic vastly outweighs writes, and reads need different projections than the write model (dashboard aggregates, search results). Or when you want to add Event Sourcing later and need the write model fully separated.

---

## Decision Guide

| Situation | Recommended |
|---|---|
| Simple CRUD, small team, fast iteration | **Plain Layered** |
| CRUD with moderate rules, single transport | **Layered + DI at repo** ← current |
| Multiple delivery transports (HTTP + gRPC + CLI) | **Clean Architecture** |
| Strictest domain isolation, infrastructure expected to change | **Hexagonal** |
| Many loosely-related features, frequent add/remove | **Vertical Slice** |
| Complex business rules, invariants, workflows | **Layered + DDD tactical patterns** |
| Read-heavy, complex projections, or pre-Event Sourcing | **CQRS** |

These are not mutually exclusive. The most common evolution path for a growing microservice:

```
Layered + DI  →  add DDD tactical patterns  →  add CQRS for read paths
```

Clean Architecture and Hexagonal Architecture are structural overlays — they can sit on top of any of the above.

---

## Where This Service Could Go Next

The current catalog and user services are CRUD-heavy with thin business logic. The most useful next steps if complexity grows:

- **Add value objects** (`Email`, `Price`) to the model layer — small DDD addition, high payoff for data integrity, no structural change required.
- **Separate read queries from write operations** in the repository — a minimal CQRS split with no structural change; just add a `GetAllView()` method that scans into a DTO instead of a model struct.
- **Add service interfaces** in `model/` alongside the repository interfaces — brings the handler→service boundary to the same level as the service→repository boundary, completing the Clean Architecture dependency rule.
