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

### Why "Layered with DI" and not plain Layered?

In classic Layered Architecture the repository interface belongs to the data-access layer — handlers and services import downward into the database layer.

Here the `model/` package defines the `UserRepository` and `ProductRepository` interfaces. The `repository/` package imports `model/` and implements those interfaces. This inverts the dependency: the domain layer owns the contract, and infrastructure conforms to it. Services never import GORM or Redis directly.

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

**Adds:** Nothing over what we have. Slightly simpler — one fewer package to navigate.

**Loses:** The data layer can now leak ORM types upward. Harder to test services without real GORM structs.

**Choose when:** The service is very small (< 5 endpoints), has no complex business rules, and you want the absolute minimum structure.

---

### 2. Clean Architecture (Uncle Bob)

Adds two things we currently skip:

1. **Interfaces at every layer boundary** — handlers call a `UserServicePort` interface, not `*service.UserService`. Every boundary is invertible.
2. **Use Cases / Interactors** — a dedicated layer between handlers and domain that contains one struct per business operation (`RegisterUserUseCase`, `LoginUseCase`, etc.).

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

**Loses:** Significantly more files and interfaces. A `CreateProduct` operation that today spans ~40 lines of handler + service code becomes a handler, a use case interface, a use case struct, a request/response object, and a presenter. Refactoring across boundaries is slower.

**Choose when:** The service has complex, branching business rules. Multiple delivery mechanisms exist (HTTP + gRPC + CLI). You have a team of 4+ developers who need the compiler to enforce boundaries, not code review.

---

### 3. Hexagonal Architecture (Ports and Adapters)

The framing differs from Clean Architecture even though the structure is similar. The domain is at the center. Everything outside is an adapter. Adapters connect through named ports (interfaces).

```
         ┌─────────────────────────────────────┐
         │              Domain                  │
         │  (pure Go, zero imports, no HTTP)    │
         │                                      │
         │  ┌──────────┐    ┌──────────────┐   │
         │  │ driving  │    │  driven      │   │
         │  │ ports    │    │  ports       │   │
         │  │(inbound) │    │ (outbound)   │   │
         │  └────┬─────┘    └──────┬───────┘   │
         └───────┼─────────────────┼───────────┘
                 │                 │
        ┌────────▼──────┐ ┌────────▼──────┐
        │ HTTP Adapter  │ │  DB Adapter   │
        │ (handler)     │ │ (repository)  │
        └───────────────┘ └───────────────┘
```

**Adds:** Domain code has zero framework imports. You can test the entire domain in-memory with no database at all. Swap Postgres for SQLite for tests; swap HTTP for gRPC without touching the domain.

**Loses:** Explicit port/adapter naming discipline. In Go this is mostly the same code as Clean Architecture — the difference is conceptual framing, not file structure.

**Choose when:** You want the strictest possible isolation of business rules from all frameworks. Good when the domain logic is the most valuable and long-lived part of the system, and infrastructure is expected to change (e.g., migrating DB engines, adding new transport protocols).

---

### 4. Vertical Slice Architecture

Abandon layer-based folders entirely. Organize by **feature** instead.

```
services/user/
└── internal/
    ├── register/
    │   ├── handler.go
    │   ├── service.go
    │   ├── repository.go
    │   └── dto.go
    ├── login/
    │   ├── handler.go
    │   ├── service.go
    │   └── dto.go
    └── getuser/
        ├── handler.go
        ├── repository.go
        └── dto.go
```

Each slice owns its full stack from HTTP to DB. Slices share nothing by default; they import from `pkg/` for cross-cutting concerns.

**Adds:** Adding or deleting a feature is self-contained — one folder, no ripple across layers. Cognitive load per change is low. Avoids the "which layer does this belong to?" question. Scales well when features are numerous and largely independent.

**Loses:** Shared logic (e.g., `GetUserByID` used by both `UpdateUser` and `Login`) must either be duplicated or extracted into a shared internal package, which recreates a partial layer anyway. Harder to enforce consistent patterns across slices.

**Choose when:** The service has many loosely-related features (e.g., a BFF or admin dashboard). Features are added and removed frequently. You find yourself saying "I just need to change this one thing" but layers force you to touch five files.

---

### 5. Domain-Driven Design (DDD — Tactical Patterns)

Enriches the domain layer with DDD building blocks on top of any structural style above.

| Building Block | What it is | Example here |
|---|---|---|
| **Entity** | Object with identity | `User`, `Product` |
| **Value Object** | Immutable, identity-free | `Email`, `Price`, `Money` |
| **Aggregate** | Cluster with one root | `Order` owns `OrderLine[]` |
| **Domain Service** | Logic that doesn't fit one entity | `PricingService` |
| **Repository** | Collection abstraction | Already present |
| **Domain Event** | Something that happened | `UserRegistered`, `OrderPlaced` |

In the current codebase `User` and `Product` are anemic — they are plain structs with no methods that enforce invariants. Business rules (e.g., "email must be unique") live in `service/`, not on the model itself.

**Adds:** Models enforce their own invariants. Rich types (`Email` value object) make invalid state unrepresentable. Domain events enable loose coupling between aggregates.

**Loses:** More types, more indirection. Most useful when the domain is genuinely complex with many invariants. Overkill for simple CRUD.

**Choose when:** The service has a rich, rules-heavy domain — not just CRUD but workflows, state machines, or multi-step business processes (e.g., an orders or billing service).

---

### 6. CQRS (Command Query Responsibility Segregation)

Split the model into two: one for writes (Commands), one for reads (Queries).

```
┌──────────────────┐         ┌──────────────────────┐
│  Command Side    │         │  Query Side           │
│                  │         │                       │
│  CreateProduct   │         │  GetAllProducts       │
│  UpdateProduct   │ ──────▶ │  GetProductByID       │
│  DeleteProduct   │  event  │                       │
│                  │  / sync │  Read model           │
│  Write DB        │         │  (optimized for reads)│
└──────────────────┘         └──────────────────────┘
```

In its simplest form (no separate DB): commands go through service → repository, queries bypass the service and query the DB directly with a DTO projection. No domain model loaded, no ORM overhead.

**Adds:** Read paths become simple, fast SQL queries with no business logic overhead. Write paths stay protected by business rules. Scales independently.

**Loses:** Two paths to maintain. More indirection. Full CQRS with separate read/write databases adds eventual consistency complexity.

**Choose when:** Read traffic vastly outweighs writes, and reads need different projections than the write model (e.g., dashboard aggregates, search results). Or when you want to add Event Sourcing later.

---

## Decision Guide

| Situation | Recommended |
|---|---|
| Simple CRUD, small team, fast iteration | **Plain Layered** |
| What this codebase currently is — CRUD with moderate rules | **Layered + DI at repo** (current) |
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

The current catalog and user services are CRUD-heavy with thin business logic. The most useful next step if complexity grows:

- **Add value objects** (`Email`, `Price`) to the model layer — small DDD addition, high payoff for data integrity.
- **Separate read queries from write operations** in the repository — a minimal CQRS split with no structural change.
- **Add service interfaces** in `model/` alongside the repository interfaces — brings the handler→service boundary to the same level as the service→repository boundary, completing the Clean Architecture dependency rule.
