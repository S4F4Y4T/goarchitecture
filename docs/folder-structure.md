# Folder Structure

## Top-Level Layout

```
/var/www/microservice/
├── database/
│   └── migrations/
│       ├── user/          # SQL migration files for the user service
│       └── catalog/       # SQL migration files for the catalog service
├── deploy/
│   ├── docker/
│   │   ├── Dockerfile.auth
│   │   ├── Dockerfile.user
│   │   ├── Dockerfile.catalog
│   │   └── Dockerfile.docs
│   └── kong/
│       ├── kong.yml           # Kong declarative config (routes, plugins, consumers)
│       └── jwt.key.pub        # RSA public key — embedded in kong.yml consumers block
├── docs/                  # design decision documentation (this dir)
├── pkg/                   # shared Go module (github.com/s4f4y4t/go-microservice/pkg)
│   ├── apperror/          # typed application errors
│   ├── config/            # env-var helpers
│   ├── logger/            # structured JSON logger
│   ├── middleware/        # reusable HTTP middleware
│   ├── pagination/        # page/limit params + meta
│   ├── query/             # filter/sort parser + GORM bridge
│   ├── request/           # JSON decoding helpers
│   ├── response/          # JSON response helpers
│   └── validation/        # struct validation
├── scripts/
│   └── migrate.sh         # migration wrapper CLI
├── services/
│   ├── auth/              # auth service (register, login, refresh, logout)
│   │   ├── cmd/api/main.go
│   │   ├── internal/
│   │   │   ├── app/       # composition root
│   │   │   ├── config/    # service-specific config (DB, Redis, JWT)
│   │   │   ├── auth/      # feature module: dto, handler, service, token store
│   │   │   ├── user/      # minimal user model + repo (read/create only; owns no schema)
│   │   │   ├── health/    # liveness/readiness handler
│   │   │   ├── platform/  # DB + Redis connection helpers
│   │   │   └── router/    # route registration
│   │   ├── .air.toml
│   │   └── go.mod
│   ├── user/              # user service (CRUD for users, JWT-protected by Kong)
│   │   ├── cmd/api/main.go
│   │   ├── internal/
│   │   │   ├── app/       # composition root
│   │   │   ├── config/    # service-specific config (DB only)
│   │   │   ├── user/      # feature module: model, repository, service, handler, dto
│   │   │   ├── health/    # liveness/readiness handler
│   │   │   ├── platform/  # DB connection helper
│   │   │   └── router/    # route registration
│   │   ├── .air.toml
│   │   └── go.mod
│   ├── catalog/           # catalog service (identical structure to user)
│   └── docs/              # API docs service (serves Swagger UI + openapi.yaml)
│       ├── cmd/api/main.go
│       ├── static/
│       │   ├── index.html     # Swagger UI (CDN-loaded)
│       │   └── openapi.yaml   # Combined OpenAPI spec for all services
│       ├── docs.go            # embed.FS for static/
│       ├── .air.toml
│       └── go.mod
├── docker-compose.yml
├── go.work
├── go.work.sum
└── makefile
```

## What Lives Where and Why

### `pkg/` — Shared Library
Code that is **domain-agnostic** and safe to import by any service. No service-specific business logic. No database schema awareness.

Examples of what belongs here: error types, logger, pagination math, JSON helpers, middleware primitives.  
Examples of what does **not** belong here: user model, product repository, auth tokens.

### `services/<name>/internal/` — Private Service Code
Go's `internal` package rule enforces that **no other module can import this code**. This is the architectural boundary: a service's business logic, models, and repositories are its private concern.

### `services/<name>/cmd/api/main.go` — Entry Point
A thin `main.go` that only wires things together: read config, open connections, run DI bootstrap, start server. No business logic.

### `database/migrations/` — Migration Files at Root
Migrations are **not** inside the service directory because they are run by an external tool (`golang-migrate`) and often by ops/CI rather than the service process itself. Centralizing them also makes it easy to see all schema changes in one place.

### `deploy/` — Deployment Artifacts
Dockerfiles and (future) Kubernetes manifests. Kept separate from service code so building images doesn't require understanding the Go source tree.

### `scripts/` — Operational Scripts
Shell scripts that wrap CLI tools (e.g., `golang-migrate`). Not Go code.

## Why Not `cmd/` at the Root?

A common Go layout puts `cmd/<service>/main.go` at the repo root. We chose `services/<name>/cmd/api/main.go` because each service is a separate Go module with its own `go.mod`. Having the module root at `services/<name>/` keeps `go build ./...` scoped to that service without leaking internal packages.

## Alternatives Considered

- **Flat single module** — one `go.mod` for everything. Simpler to set up but means all services share the same dependency versions and must be built together. Rejected because services need to evolve independently.
- **Separate repos per service** — full isolation. Harder to share `pkg/` (requires publishing tags). More ops overhead for a small team. Rejected for now; the monorepo with workspace gives 80% of the isolation without separate repos.
- **`pkg/` inside each service** — no shared library. Code duplication across services. Rejected.
