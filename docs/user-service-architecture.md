# User Service — Fat Microservice (Modular Monolith)

## What This Service Actually Is

The user service is a **fat microservice**: a single Go module, deployed as several binaries (`api`, `grpc`, `worker`, `kafkaconsumer`, ...), all of which share one set of feature modules and one DI composition root. "Fat" here means the service is not split into one-module-per-service — `user` (CRUD), `auth` (registration/login/token lifecycle), and any future module (e.g. `notification`, `audit`) all live inside this one service and are reused across every entry point, instead of each transport getting its own copy of the wiring.

Today only `cmd/api` exists. The sections below describe the target shape once `grpc`, `worker`, and `kafkaconsumer` entry points are added — they reuse the exact same modules and the exact same composition root, just exposing them over a different transport or trigger.

```
                         ┌────────────────────────────────┐
                         │       internal/app/app.go        │
                         │  global composition root —       │
                         │  builds shared infra once,        │
                         │  calls each module's New(...)      │
                         └────────────────┬────────────────┘
                                           │ injects plain deps (DB, Redis client,
                                           │ Kafka producer, config) into constructors
        ┌──────────────────┬──────────────┼──────────────┬───────────────────┐
        │                  │              │               │                   │
┌───────▼───────┐ ┌────────▼───────┐ ┌────▼────────┐ ┌────▼─────────┐ ┌───────▼───────┐
│  user/          │ │  auth/          │ │  health/    │ │  (future)     │ │  (future)      │
│  entity.go       │ │  dto.go          │ │  handler.go │ │  notification/│ │  module        │
│  dto.go          │ │  repository.go   │ │             │ │               │ │                │
│  repository.go   │ │  service.go      │ │             │ │               │ │                │
│  service.go      │ │  http_handler.go │ │             │ │               │ │                │
│  http_handler.go │ │  grpc_handler.go │ │             │ │               │ │                │
│  grpc_handler.go │ │  consumer.go     │ │             │ │               │ │                │
│  consumer.go     │ │  worker.go       │ │             │ │               │ │                │
│  worker.go       │ │                  │ │             │ │               │ │                │
└────────┬─────────┘ └────────┬─────────┘ └─────────────┘ └───────────────┘ └────────────────┘
         │ user.Repository iface       │ auth.Repository iface
         │ (GORM impl)                 │ (Redis impl — refresh tokens)
┌────────▼─────────────────────────────▼──────────────────────────────────────────────────┐
│        internal/platform/  →  database/  ·  redis/  ·  kafka/  ·  middleware/  ·  ...      │
│        (the ONLY place connections to Postgres, Redis, Kafka are opened)                    │
└──────────────────────────────────────────────────────────────────────────────────────────┘
                                           ▲
        ┌──────────────────┬──────────────┼──────────────┬───────────────────┐
        │                  │              │               │                   │
┌───────▼───────┐ ┌────────▼───────┐ ┌────▼────────┐ ┌────▼─────────────┐
│ cmd/api        │ │ cmd/grpc        │ │ cmd/worker  │ │ cmd/kafkaconsumer  │
│ (HTTP REST)    │ │ (internal RPC)  │ │ (cron/queue)│ │ (Kafka consumer    │
│                │ │                 │ │             │ │  group)            │
└────────────────┘ └─────────────────┘ └─────────────┘ └────────────────────┘
```

Each module owns its own entity, DTO, repository, service, and every transport adapter it needs (`http_handler.go`, `grpc_handler.go`, `consumer.go`, `worker.go`) — there is no service-wide `model/`, `repository/`, `handler/` split, and no separate top-level `internal/grpc/`, `internal/worker/`, `internal/kafka/` dirs holding business logic. Those concerns live *inside* the module that owns the behavior; only the generic transport machinery (the gRPC server, the consumer-group runner, the cron scheduler) is shared, and that machinery lives in `internal/app`, not in the modules.

---

## DI Principle: Global Root for Infra, Per-Module Constructors for Internals

This is the rule that keeps a fat microservice from collapsing into a tangle, and it has two halves:

**1. Global composition root → shared infrastructure only.**
`internal/app/app.go` is the *only* place that opens a Postgres connection, a Redis client, or a Kafka producer/consumer-group connection. It builds these once per process (driven by `internal/platform/*`) and passes the already-constructed client objects down. No module ever calls `sql.Open`, `redis.NewClient`, or constructs a Kafka writer itself — they receive a `*gorm.DB` / `*redis.Client` / `*kafka.Producer` as a constructor argument and trust the caller to have wired it correctly. This is what "modules don't create their own DB/Redis/Kafka connections" means in practice.

**2. Per-module constructors → module internals.**
Each module exposes plain Go constructors (`user.NewRepository(db)`, `user.NewService(repo)`, `auth.NewService(repo, tokenRepo, issuer, ...)`) that `app.go` calls explicitly, in dependency order. `app.go` does NOT reach into a module's internals or know *how* a module is built beyond calling its constructor — it only knows the constructor's signature. The module decides its own internal wiring (e.g. whether `auth.Service` needs a cache in front of its repository); `app.go` just supplies the raw ingredients (DB handle, Redis handle) and receives back a ready-to-use service/handler.

**Avoid:**
- A giant `App` struct that holds every object transitively (every repo, every service, every handler, every infra client) reachable from every other field — this turns into an untyped global container. `App` should hold the *outputs* each `cmd/*` entry point needs (handlers, servers, job lists), not every intermediate object.
- Modules importing `gorm.io/gorm` or `github.com/redis/go-redis` to open their own connections "for convenience" — this is exactly the coupling that makes a module impossible to extract into its own service later, and it duplicates connection-pool/lifecycle config that should exist once.

---

## Folder Map (target shape)

```
services/user/
├── cmd/
│   ├── api/main.go              # HTTP entry point — boot only
│   ├── grpc/main.go             # internal gRPC server — boot only
│   ├── worker/main.go           # background job runner — boot only
│   └── kafkaconsumer/main.go    # Kafka consumer group — boot only
├── proto/
│   └── user.proto               # hand-written gRPC contract source
├── gen/
│   └── userpb/                  # generated *.pb.go / *_grpc.pb.go — never hand-edited
└── internal/
    ├── app/
    │   └── app.go                # global composition root for ALL entry points
    ├── platform/                 # the only code that touches infra directly
    │   ├── database/             # Postgres/GORM connection setup
    │   ├── redis/                # Redis client setup
    │   ├── kafka/                # Kafka producer/consumer-group client setup
    │   ├── logger/                # thin wrapper around pkg/logger for this service's config
    │   ├── middleware/            # thin wrapper around pkg/middleware for this service's config
    │   ├── metrics/                # Prometheus/metrics client setup
    │   └── tracing/                # OpenTelemetry tracer setup
    ├── user/
    │   ├── entity.go               # User struct + Repository interface + ListSchema
    │   ├── dto.go                   # CreateUserRequest, UpdateUserRequest
    │   ├── repository.go            # GORM impl of Repository — takes *gorm.DB, opens nothing itself
    │   ├── service.go                # CRUD + transactional Update
    │   ├── http_handler.go           # HTTP handlers: GetAll, GetByID, Create, Update, Delete
    │   ├── grpc_handler.go            # gRPC handlers, same Service underneath
    │   ├── consumer.go                 # reacts to Kafka events relevant to user (if any)
    │   └── worker.go                   # background jobs owned by this module (if any)
    ├── auth/
    │   ├── dto.go                       # RegisterDTO, LoginDTO
    │   ├── repository.go                 # Redis impl of token storage — replaces old token_store.go
    │   ├── service.go                     # bcrypt hash/compare, token issue/rotate
    │   ├── http_handler.go                 # Register, Login, Refresh, Logout
    │   ├── grpc_handler.go                  # internal RPC equivalents, if needed
    │   ├── consumer.go                       # e.g. react to UserDeleted to revoke tokens
    │   └── worker.go                          # e.g. PurgeExpiredTokens job
    ├── health/
    │   └── handler.go             # Live, Ready
    └── router/
        ├── router.go
        ├── auth.go
        └── user.go
```

A module gains a `grpc_handler.go`, `consumer.go`, or `worker.go` only when it actually needs that transport — `health` has none of them. Every transport-specific file still calls the same `*Service` struct the HTTP handler calls; only the request/response marshaling and trigger differ. `internal/platform/` is service-local infra wiring (this service's DB config, this service's Kafka topic config) — it is not a replacement for the shared `pkg/logger`, `pkg/middleware`, etc. at the repo root; `platform/logger` and `platform/middleware` simply configure and re-export those shared packages for this service's own needs.

---

## The `user` Module

`internal/user/entity.go` defines both the `User` struct and the `Repository` interface in the same package:

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

`internal/user/repository.go` implements `Repository` with GORM (`UserRepository`). It takes an already-open `*gorm.DB` in its constructor (`NewRepository(db *gorm.DB)`) — it never opens a connection itself, that's `internal/platform/database`'s job, invoked once from `app.go`. The module owns the contract; the implementation just has to conform to it — this is the one place dependency-inversion is applied.

`User` is an **anemic** struct: no methods, no invariants, no value objects. Uniqueness checks and update orchestration live in `internal/user/service.go`, not on the model. That's a deliberate simplification for CRUD-heavy code, not an oversight — see "What Is and Isn't DDD Here" below.

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
    tokenRepo     token.Store
    tokenIssuer   token.AccessIssuer
    accessExpiry  time.Duration
    refreshExpiry time.Duration
}
```

`*user.UserRepository` already implements `ExistsByEmail`, `GetByEmail`, and `Create`, so it satisfies `UserLookup` implicitly — `app.go` passes the same repository instance to both `user.NewService` and `auth.NewService` with no extra adapter. This keeps `auth`'s coupling to `user` limited to exactly the three operations it needs, instead of the entire CRUD surface. It's still an in-process Go interface, not a network boundary — splitting `auth` into its own service would still mean replacing this with an HTTP/gRPC call to `user` — but the contract is already minimal, so that future change touches one interface, not every call site.

`internal/auth/repository.go` (`RedisTokenRepository`, replacing the old `token_store.go`) implements `token.Store` from `pkg/token` — refresh tokens are stored in Redis as `refresh:<token> → userID`, independent of the `user` module's Postgres-backed repository. Like `user.Repository`, its constructor takes an already-constructed `*redis.Client` — the connection itself comes from `internal/platform/redis`, built once in `app.go`.

---

## `internal/app/app.go` — the Global Composition Root

`app.go` is the single place that knows every module exists. Its job, in order:

1. Call `internal/platform/*` to build shared infra clients (`*gorm.DB`, `*redis.Client`, Kafka producer/consumer-group client) — once, regardless of how many `cmd/*` binaries end up using them.
2. Call each module's constructors in dependency order, passing those infra clients straight through — `user.NewRepository(db)`, `auth.NewRepository(rdb)`, `user.NewService(userRepo)`, `auth.NewService(userRepo, authRepo, issuer, ...)`.
3. Build the transport adapters each module needs — `user.NewHTTPHandler(userSvc)`, `user.NewGRPCHandler(userSvc)`, `auth.NewHTTPHandler(authSvc, cookieSecure)`.
4. Return an `App` struct holding only what each `cmd/*` entry point actually needs — not every intermediate repo/service.

```go
type App struct {
    UserHTTPHandler *user.HTTPHandler // cmd/api
    AuthHTTPHandler *auth.HTTPHandler // cmd/api
    HealthHandler   *health.Handler  // cmd/api
    UserGRPCHandler *user.GRPCHandler // cmd/grpc
    WorkerJobs      []worker.Job      // cmd/worker — collected from each module's worker.go
    ConsumerRoutes  []kafka.Route     // cmd/kafkaconsumer — collected from each module's consumer.go
}

func Build(cfg *config.Config) (*App, func(), error) {
    db := platformdb.MustOpen(cfg.DB)
    rdb := platformredis.MustOpen(cfg.Redis)
    producer := platformkafka.MustNewProducer(cfg.Kafka)

    userRepo := user.NewRepository(db)
    authRepo := auth.NewRepository(rdb)

    userSvc := user.NewService(userRepo)
    authSvc := auth.NewService(userRepo, authRepo, cfg.TokenIssuer, cfg.JWT.AccessExpiry, cfg.JWT.RefreshExpiry, producer)

    app := &App{
        UserHTTPHandler: user.NewHTTPHandler(userSvc),
        AuthHTTPHandler: auth.NewHTTPHandler(authSvc, cfg.JWT.CookieSecure),
        HealthHandler:   health.NewHandler(db),
        UserGRPCHandler: user.NewGRPCHandler(userSvc),
        WorkerJobs:      append(user.Jobs(userRepo), auth.Jobs(authRepo)...),
        ConsumerRoutes:  append(user.ConsumerRoutes(userSvc), auth.ConsumerRoutes(authSvc)...),
    }
    cleanup := func() { rdb.Close(); producer.Close() }
    return app, cleanup, nil
}
```

Each `cmd/*/main.go` calls `app.Build` once, then only touches the fields it needs:

- `cmd/api/main.go` — `app.UserHTTPHandler`, `app.AuthHTTPHandler`, `app.HealthHandler` → passed to `router.Register`, served over HTTP.
- `cmd/grpc/main.go` — `app.UserGRPCHandler` → registered on a `grpc.Server`, served over a separate port.
- `cmd/worker/main.go` — `app.WorkerJobs` → handed to a cron/queue runner, no HTTP/gRPC server started at all.
- `cmd/kafkaconsumer/main.go` — `app.ConsumerRoutes` → handed to a consumer-group runner that dispatches incoming events to the right module's `consumer.go`.

**The key invariant:** modules don't know which entry point is running them. `user.Service` and `auth.Service` take plain Go arguments and return plain Go values/errors — they have no idea whether they were invoked from an HTTP handler, a gRPC handler, a Kafka consumer, or a worker job. Only the thin per-module adapter files (`http_handler.go`, `grpc_handler.go`, `consumer.go`, `worker.go`) are transport-aware, and only `internal/app` and `internal/platform` know about real infrastructure.

---

## gRPC

gRPC is used here for **internal service-to-service calls** (e.g. another service in the monorepo looking up a user by ID without going through the public HTTP API) — it is not a replacement for the public REST API. The contract is hand-written in `proto/user.proto`; running the generator produces `gen/userpb/*.pb.go`, which is never edited by hand and is safe to regenerate or gitignore-and-rebuild in CI. `cmd/grpc/main.go` builds a `*grpc.Server`, applies gRPC-specific middleware (auth interceptor, logging interceptor), and registers each module's `grpc_handler.go` (`userpb.RegisterUserServiceServer(srv, app.UserGRPCHandler)`) — that registration loop lives in `cmd/grpc/main.go` itself (or a thin helper in `internal/app`), not inside any module.

## Kafka

Each module that needs to publish events does so through the shared `*kafka.Producer` built once in `internal/platform/kafka` and injected via the module's service constructor — e.g. `auth.NewService(..., producer)` lets `auth.Service` call `producer.Publish(ctx, UserRegistered{...})` without knowing about Kafka's wire format. Each module that needs to *react* to events owns a `consumer.go` exposing the routes/handlers it cares about; `app.go` collects those into `ConsumerRoutes`, and `cmd/kafkaconsumer/main.go` starts one consumer-group runner (built from `internal/platform/kafka`) that dispatches incoming messages to the right module handler — e.g. consuming a `catalog.ProductPurchased` event from another service in the monorepo and updating a user's purchase history via `user.Service`. The consumer-group connection itself is opened once in `platform/kafka`, never inside a module.

## Worker

Each module that has background work (token cleanup, scheduled emails, retries) exposes a `worker.go` with `Jobs(...) []worker.Job` — a job is a small struct implementing `Run(ctx) error`, constructed with whatever the module's own repository/service already is (no separate infra access). `app.go` concatenates every module's job list into `App.WorkerJobs`; `cmd/worker/main.go` hands that slice to a scheduler loop (cron-style ticker or queue-backed runner) that has no knowledge of what each job does internally.

---

## What Is and Isn't DDD Here

| Pattern | Present? |
|---|---|
| Entity with identity | Yes — `User`, compared by `ID` |
| Value Objects (`Email`, `Password`) | **No** — both are plain `string` fields |
| Aggregate beyond the entity itself | No — `User` owns no child entities |
| Domain events (`UserRegistered`, etc.) | No today — the Kafka producer hook exists structurally but no module emits events yet |
| Repository pattern | Yes — `user.Repository`, owned by the domain, implemented by infrastructure |
| Domain service (logic spanning multiple aggregates) | No — single-entity checks live in the module's service |

This service is intentionally **DDD-lite**: it gets the repository pattern's testability/inversion benefit without paying for value objects, aggregates, or events that have no current use case. If business rules around `User` grow (e.g. password policy, email verification workflows), reintroducing `Email`/`Password` as value objects is the natural next step.

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
| What architecture is this? | Fat microservice / Modular Monolith — package-by-feature modules, multiple `cmd/*` entry points (api, grpc, worker, kafkaconsumer), one global composition root in `internal/app` |
| Is it Clean Architecture? | No — handlers call concrete `*Service` structs, no Use Case interfaces |
| Is DDD applied? | Partially — repository pattern only; no value objects, aggregates, or events |
| Is it appropriate for the service's complexity? | Yes — CRUD + auth, no workflows or invariants complex enough to need more |
| Why multiple entry points instead of separate services? | grpc/worker/kafkaconsumer all need the same modules and DB/Redis/Kafka connections as the api; splitting them into separate services now would mean duplicating wiring with no isolation benefit yet |
| Where do DB/Redis/Kafka connections get created? | Only in `internal/platform/*`, called once from `internal/app/app.go` — never inside a module |
| Where does module-internal wiring happen? | Inside each module's own constructors (`NewRepository`, `NewService`, `NewHTTPHandler`, ...), called by `app.go` but decided by the module |
| What should change next? | Optional: a `UserID` value type; value objects only if password/email rules grow; add `cmd/grpc`/`cmd/worker`/`cmd/kafkaconsumer`, `internal/app`, `internal/platform`, `proto/`, `gen/userpb/` when those transports are actually needed |
