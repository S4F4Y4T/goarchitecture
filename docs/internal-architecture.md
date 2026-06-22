# Per-Service Internal Architecture

## What We Use

This codebase uses a **Modular Monolith (package-by-feature)** layout inside every service.

Each service is a single deployable binary, but internally it is split into self-contained feature modules — `user`, `auth`, `health`, etc. Each module owns its own model, repository, service, handler, and DTOs. There is no cross-cutting `model/`, `repository/`, `service/`, `handler/` split — those concerns live *inside* the feature package, not as separate top-level layers.

```
┌─────────────────────────────────────────────────────────┐
│                      services/user                       │
│                  (one deployable binary)                 │
│                                                            │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐    │
│  │   user/      │  │   auth/      │  │   health/   │    │
│  │              │  │              │  │             │    │
│  │ model.go     │  │ dto.go       │  │ handler.go  │    │
│  │ repository.go│  │ handler.go   │  │             │    │
│  │ service.go   │  │ service.go   │  │             │    │
│  │ handler.go   │  │ token_store. │  │             │    │
│  │ dto.go       │  │ go           │  │             │    │
│  └──────────────┘  └──────────────┘  └─────────────┘    │
│         ▲                  │                              │
│         └──────────────────┘                              │
│         auth imports user.Repository directly              │
│                                                            │
│  bootstrap/  — composition root, wires modules together   │
│  router/     — maps HTTP paths to each module's handler   │
│  config/     — env loading, DB/Redis setup                │
└─────────────────────────────────────────────────────────┘
```

### Folder Structure

```
services/user/
├── cmd/api/main.go             # entry point — boot only, no logic
└── internal/
    ├── bootstrap/
    │   └── app.go              # composition root: wires repo → service → handler per module
    ├── config/
    │   ├── config.go           # Config struct + env loading
    │   ├── database.go         # GORM setup + connection pool
    │   └── redis.go            # Redis client setup
    ├── user/                   # feature module: user CRUD
    │   ├── model.go            # User struct + Repository interface + ListSchema
    │   ├── repository.go       # GORM impl of Repository
    │   ├── service.go          # CRUD orchestration, transactional update
    │   ├── handler.go          # GetAll, GetByID, Create, Update, Delete
    │   └── dto.go               # CreateUserRequest, UpdateUserRequest
    ├── auth/                   # feature module: authentication
    │   ├── dto.go               # RegisterDTO, LoginDTO
    │   ├── handler.go           # Register, Login, Refresh, Logout
    │   ├── service.go           # bcrypt hashing, token issuance/rotation
    │   └── token_store.go       # Redis impl of token.Store
    ├── health/                 # feature module: liveness/readiness
    │   └── handler.go
    └── router/
        ├── router.go           # root mux + middleware chain
        ├── auth.go              # /auth/* routes
        └── user.go              # /users/* routes + auth middleware
```

### Why Package-by-Feature and Not Package-by-Layer?

In the layer-based split this codebase used previously (`internal/model/`, `internal/repository/`, `internal/service/`, `internal/handler/`, `internal/dto/`), every change to one resource touched five different top-level packages, and the layer boundary was the only organizing principle — feature boundaries weren't visible in the folder tree at all.

With package-by-feature, everything about a feature is colocated. Adding or removing the `health` module never touches `user/` or `auth/`. The repository interface still lives in the package that owns the data (`user.Repository` in `user/model.go`), so the dependency-inversion benefit at the persistence boundary is preserved — it's just scoped per-module instead of globally.

```
user.Repository       ← interface (the user module owns it)
       ↑
user.UserRepository   ← GORM implementation (same package, conforms to the interface)
```

### What This Module Boundary Is — and Isn't

- **Modules share one process and one deploy.** This is what makes it a *monolith*, not a set of microservices, even though `auth` and `user` are conceptually separate domains.
- **Module boundaries are deliberately narrow at the one cross-module dependency.** `auth.Service` depends on `auth.UserLookup` (`internal/auth/service.go`) — a 3-method interface owned by `auth` itself — rather than the full `user.Repository`. `*user.UserRepository` satisfies it implicitly, so `bootstrap` wires them together with no extra glue. This is the seam that would need to move to a real network call before `auth` could be extracted into its own service, but the contract `auth` depends on is already minimal.
- **Not Vertical Slice Architecture** — true vertical slices cut per *use case* (one independent slice per `CreateUser`, `UpdateUser`, etc., each free to duplicate logic instead of sharing a layer). Here, every operation on a module shares one `Repository`, one `Service`, and one `Handler` for the whole domain (e.g. `user.Service.Create/Get/Update/Delete` all live in one `service.go`). The slice is the *module* (domain), not the *use case* — that's package-by-feature, and the internal handler→service→repository layering is preserved, just colocated per module instead of spread across top-level layer folders.
- **Not Clean Architecture** — there's no Use Cases/Interactors layer, and handlers call concrete `*Service` structs, not interfaces.
- **Not Hexagonal** — no named inbound/outbound ports; the only inverted boundary is the repository.
- **Not DDD** — `User` is an anemic struct (no behavior, no value objects); business rules live in the service, not the model.

---

## Alternatives

### 1. Layered Architecture (Handler → Service → Repository)

```
handler/ → service/ → repository/ → DB
```

One top-level package per layer, shared across all features in the service. This is what this codebase used before adopting the modular monolith.

**Adds:** Nothing over what we have now.

**Loses:** Feature boundaries aren't visible in the folder tree. Every new resource (e.g. adding a `product` feature to a service) adds a file to four or five existing packages instead of creating one new package. Harder to later extract a feature into its own microservice, since its code is scattered across layers instead of colocated.

**Choose when:** The service will only ever have one or two resources and isn't expected to grow more feature modules.

---

### 2. Clean Architecture (Uncle Bob)

Adds two things package-by-feature currently skips:

1. **Interfaces at every layer boundary** — handlers call a `UserServicePort` interface, not `*user.Service`.
2. **Use Cases / Interactors** — a dedicated layer between handlers and domain with one struct per business operation.

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

**Adds:** Every layer is mockable without a framework. New delivery mechanisms (gRPC, CLI, queue consumer) plug in without touching business logic. Compiler enforces every architectural boundary.

**Loses:** Significantly more files and interfaces per feature. Refactoring across boundaries is slower.

**Choose when:** The service has complex, branching business rules. Multiple delivery mechanisms exist (HTTP + gRPC + CLI). You have a team of 4+ developers who need the compiler to enforce boundaries, not code review.

---

### 3. Hexagonal Architecture (Ports and Adapters)

Same structural shape as Clean Architecture, different vocabulary: the domain sits at the center with named **inbound** (driving) and **outbound** (driven) ports; everything else is an adapter.

**Adds:** Domain code has zero framework imports. You can test the entire domain in-memory with no database at all.

**Loses:** Strict port/adapter naming discipline requires consistent team buy-in; in Go the file structure ends up very similar to Clean Architecture.

**Choose when:** You want the strictest possible isolation of business rules from frameworks, and infrastructure (DB engine, transport) is expected to change.

---

### 4. Domain-Driven Design (DDD — Tactical Patterns)

Enriches the domain layer with DDD building blocks (entities, value objects, aggregates, domain events). DDD is a modelling discipline, not a structural pattern — it can be layered on top of package-by-feature, Clean Architecture, or plain Layered.

| Building Block | What it is | Example here |
|---|---|---|
| **Entity** | Object with identity | `user.User` |
| **Value Object** | Immutable, identity-free | `Email`, `Password` (not currently present) |
| **Aggregate** | Cluster with one root | n/a — `User` has no owned children |
| **Domain Service** | Logic that doesn't fit one entity | n/a |
| **Repository** | Collection abstraction | Already present (`user.Repository`) |
| **Domain Event** | Something that happened | n/a — no event publishing |

`user.User` today is anemic — a plain struct with no methods that enforce invariants. Business rules (uniqueness checks, password hashing) live in `user.Service` / `auth.Service`, not on the model itself.

**Adds:** Models enforce their own invariants; rich value types (`Email`, `Password`) make invalid state unrepresentable. Domain events enable loose coupling between modules without direct imports — this is also the natural fix for the `auth → user.Repository` coupling noted above.

**Loses:** More types, more indirection. Most useful when the domain is genuinely complex with many invariants. Overkill for simple CRUD.

**Choose when:** A module's business rules grow rules-heavy — workflows, state machines, or multi-step processes — not just CRUD.

---

### 5. CQRS (Command Query Responsibility Segregation)

Split the request path into **Commands** (writes, go through the domain model + business rules) and **Queries** (reads, bypass the domain model and query the DB directly for a DTO projection). Applies within a feature module (e.g. `user/command/`, `user/query/`) rather than across the whole service.

**Adds:** Read paths become simple, fast SQL projections with no business logic overhead. Each side can be optimized independently.

**Loses:** Two parallel paths to maintain per module.

**Choose when:** Read traffic vastly outweighs writes and reads need different projections than the write model.

---

## Concept Categories

| Concept            | Category                         |
| ------------------ | --------------------------------- |
| Layered            | Structural Architecture          |
| Package-by-Feature  | Structural Architecture          |
| Clean               | Structural Architecture          |
| Hexagonal           | Structural Architecture          |
| Vertical Slice       | Structural Architecture          |
| DDD                  | Domain Modeling Discipline       |
| CQRS                 | Architectural Pattern            |
| Event Sourcing       | Persistence Pattern              |
| Event-Driven         | Architectural Style              |
| Saga                 | Distributed Transaction Pattern  |
| Microservices        | System Architecture              |
| Modular Monolith     | System Architecture (chosen, per-service) |
| Monolith             | System Architecture              |

---

## Decision Guide

| Situation | Recommended |
|---|---|
| One service, few resources, never expected to grow modules | **Plain Layered** |
| Multiple feature modules per service (current state) | **Modular Monolith / package-by-feature** ← current |
| Multiple delivery transports (HTTP + gRPC + CLI) | **Clean Architecture** |
| Strictest domain isolation, infrastructure expected to change | **Hexagonal** |
| Complex business rules, invariants, workflows in a module | **Package-by-feature + DDD tactical patterns** |
| Read-heavy, complex projections, or pre-Event Sourcing | **CQRS** (within a module) |

These are not mutually exclusive. The most likely evolution path for a growing module within a service:

```
Package-by-feature  →  add DDD tactical patterns to the module  →  add CQRS for its read paths
```

If a module eventually needs to scale or deploy independently of the rest of the service, the module boundary already drawn here (`internal/auth/`, `internal/user/`) is what gets lifted out into its own microservice — that's the whole point of choosing this layout per [[Monorepo Plan]].

---

## Where This Service Could Go Next

- **Add value objects** (`Email`, `Password`) to the `user` module — small DDD addition, high payoff for data integrity, no structural change required.
- **Separate read queries from write operations** in `user/repository.go` — a minimal CQRS split with no structural change; add a `GetAllView()` method that scans into a DTO instead of a model struct.
- **Add service interfaces** in each module alongside the repository interfaces — brings the handler→service boundary to the same level as the service→repository boundary.
