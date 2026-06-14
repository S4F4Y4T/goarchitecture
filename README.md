# go-microservice

A production-grade Go microservice monorepo with two independent services — **user** and **catalog** — sharing a common `pkg` library. Built on the standard library `net/http` mux with GORM for persistence.

## Stack

- **Language:** Go 1.25 (workspace: `go.work`)
- **Router:** `net/http` (`http.ServeMux` with method+path patterns, Go 1.22+)
- **ORM:** GORM (PostgreSQL via `gorm.io/driver/postgres`)
- **Validation:** `go-playground/validator/v10`
- **Docs:** OpenAPI 3 embedded YAML + Swagger UI (`swaggo/http-swagger`) — per service
- **Hot reload:** `air` (per-service `.air.toml`)
- **Migrations:** `golang-migrate`
- **Rate limiting:** Redis-backed fixed-window, per-service namespaced key

## Repository layout

```
go-microservice/
├── go.work                          # workspace root — links pkg + both services
├── go.work.sum
│
├── pkg/                             # shared module (github.com/s4f4y4t/go-microservice/pkg)
│   ├── apperror/                    # typed error codes + HTTP status mapping
│   ├── config/                      # env helpers (GetEnvInt, GetEnvDuration)
│   ├── logger/                      # slog JSON logger + request-scoped context helpers
│   ├── middleware/                  # RequestID, Logger, CORS, RateLimit, PanicRecovery, Chain
│   ├── pagination/                  # Params + Meta for list endpoints
│   ├── query/                       # ORM-free allowlisted sort + filter parsing
│   │   └── gorm/                    # GORM scope adapters for query.Options
│   ├── request/                     # JSON decode with size cap + unknown-field rejection
│   ├── response/                    # ApiResponse envelope writer (success / error / meta)
│   └── validation/                  # validator/v10 wrapper with JSON field names + messages
│
├── services/
│   ├── user/                        # module: github.com/s4f4y4t/go-microservice/services/user
│   │   ├── cmd/api/main.go          # entrypoint — config → DB → Redis → router → server
│   │   ├── docs/                    # embedded openapi.yaml + Swagger UI
│   │   ├── .air.toml                # hot-reload config (root = "../..")
│   │   ├── .env.example
│   │   ├── go.mod / go.sum
│   │   └── internal/
│   │       ├── bootstrap/           # manual DI: repo → service → handler
│   │       ├── config/              # LoadConfig, SetupDatabase, SetupRedis
│   │       ├── dto/                 # request structs + validation tags
│   │       ├── handler/             # HTTP handlers (user.go, health.go)
│   │       ├── model/               # domain types + UserRepository interface + UserListSchema
│   │       ├── repository/          # GORM implementation of UserRepository
│   │       ├── router/              # mux wiring + middleware chain
│   │       └── service/             # business logic (email uniqueness, etc.)
│   │
│   └── catalog/                     # module: github.com/s4f4y4t/go-microservice/services/catalog
│       └── (same structure as user/)
│
├── database/
│   └── migrations/
│       ├── user/                    # 000001_create_users, 000003_add_timestamps_to_users
│       └── catalog/                 # 000001_create_products
│
├── deploy/
│   ├── docker/
│   │   ├── Dockerfile.user
│   │   └── Dockerfile.catalog
│   └── k8s/
│       ├── user/                    # (placeholder)
│       └── catalog/                 # (placeholder)
│
├── scripts/
│   └── migrate.sh                   # wraps golang-migrate; reads per-service prefixed env vars
│
├── docker-compose.yml               # user_app + catalog_app + 2×postgres + redis + pgadmin
├── .env.example                     # all services in one file with prefixed var names
└── makefile
```

## Quickstart

```bash
# 1. Copy env
cp .env.example .env

# 2. Start all infrastructure + services
docker compose up -d

# 3. Run migrations (requires golang-migrate CLI)
make migrate-up SVC=user
make migrate-up SVC=catalog
```

Endpoints:

| Service | Port | API base | Swagger UI |
|---|---|---|---|
| user | 6969 | `/v1/users/` | `http://localhost:6969/swagger/` |
| catalog | 7070 | `/v1/products/` | `http://localhost:7070/swagger/` |

Health probes (unversioned, no auth):
- `GET /healthz` — liveness (process alive, no dep checks)
- `GET /readyz` — readiness (DB ping, returns 503 when DB unreachable)

## Local development (hot reload)

```bash
# Run user service with live reload
make dev SVC=user

# Run catalog service with live reload
make dev SVC=catalog
```

Air watches the workspace root and rebuilds only the target service binary on Go file changes.

## Make targets

All targets that operate on a single service accept `SVC=<name>` (default: `user`).

| Target | What it does |
|---|---|
| `make run SVC=user` | `go run` the service (no build step) |
| `make build SVC=user` | Build → `./bin/user` |
| `make dev SVC=user` | Live reload via `air -c services/user/.air.toml` |
| `make test` | `go test ./...` across all modules |
| `make lint` | `golangci-lint run ./...` |
| `make tidy` | `go mod tidy` in pkg, services/user, services/catalog |
| `make clean` | Remove `bin/` and `tmp/` |
| `make migrate-up SVC=user` | Apply all pending migrations |
| `make migrate-down SVC=user` | Roll back 1 migration |
| `make migrate-create SVC=user name=foo` | Create new migration file pair |

## API conventions

### Response envelope

All endpoints return a consistent envelope:

```json
{
  "success": true,
  "status_code": 200,
  "message": "Users retrieved successfully",
  "data": [...],
  "meta": { "page": 1, "limit": 10, "total": 42, "total_pages": 5 }
}
```

`meta` is only present on list endpoints. `error` replaces `data` on failures.

### Error format

```json
{
  "success": false,
  "status_code": 400,
  "error": {
    "code": "INVALID_INPUT",
    "message": "validation failed",
    "fields": [
      { "field": "email", "message": "Email must be a valid email address" }
    ]
  }
}
```

Error codes: `NOT_FOUND`, `INVALID_INPUT`, `CONFLICT`, `UNAUTHORIZED`, `FORBIDDEN`, `TOO_MANY_REQUESTS`, `INTERNAL`.

### Pagination

List endpoints accept `?page=` and `?limit=`.

| Param | Default | Min | Max |
|---|---|---|---|
| `page` | 1 | 1 | — |
| `limit` | 10 | 1 | 100 |

Out-of-range values are clamped, not rejected.

### Filtering & sorting

List endpoints accept `?sort=` and `?filter[field]=`. Unknown or disallowed fields are silently ignored.

**Sorting** — comma-separated fields; a leading `-` sorts descending:

```
GET /v1/users/?sort=name          # name ASC
GET /v1/users/?sort=-id           # id DESC
GET /v1/users/?sort=-name,id      # name DESC, then id ASC
```

**Filtering** — string fields match case-insensitively as a substring (`ILIKE %value%`); other fields match exactly:

```
GET /v1/users/?filter[name]=alice
GET /v1/users/?filter[email]=@example.com
GET /v1/products/?filter[name]=widget&sort=-price
```

Allowlisted fields per resource:

| Resource | Sortable | Filterable (partial) | Filterable (exact) |
|---|---|---|---|
| users | `id`, `name`, `email`, `created_at`, `updated_at` | `name`, `email` | `id` |
| products | `id`, `name`, `price`, `created_at`, `updated_at` | `name`, `description` | `id`, `price` |

### HTTP semantics

- `PUT` does full replacement — all fields required; zero values are written unconditionally.
- `DELETE` returns `204 No Content` with an empty body.
- `POST` returns `201 Created`.

## Middleware chain (per service)

```
RequestID → Logger → CORS → RateLimit(Redis) → PanicRecovery → router
```

- **RequestID** — reuses inbound `X-Request-ID` or generates a UUID; echoed on the response; injected into context so all log lines carry it.
- **Logger** — structured JSON access log (method, path, status, duration_ms) via `slog`.
- **CORS** — configurable allowed origins via `CORS_ALLOWED_ORIGINS` env var (comma-separated list or `*`).
- **RateLimit** — fixed-window per-IP, backed by Redis. Namespaced per service (`rl:user:<ip>`, `rl:catalog:<ip>`). Fails open when Redis is unavailable.
- **PanicRecovery** — catches panics, logs stack trace, returns 500 JSON.

## Environment variables

All variables are prefixed per service in the root `.env`. See `.env.example` for defaults.

| Prefix | Service |
|---|---|
| `USER_*` | user service |
| `CATALOG_*` | catalog service |
| `REDIS_*` | shared Redis |
| `PGADMIN_*` | pgadmin tool |

Key variables per service (using `USER_` prefix as example):

| Variable | Description | Default |
|---|---|---|
| `USER_APP_PORT` | HTTP listen port | `6969` |
| `USER_LOG_LEVEL` | Log level (`debug`/`info`/`warn`/`error`) | `info` |
| `USER_DB_HOST` | Postgres host | — |
| `USER_DB_PORT` | Postgres port (host-side) | `5433` |
| `USER_DB_USER` | Postgres user | `postgres` |
| `USER_DB_PASSWORD` | Postgres password | — |
| `USER_DB_NAME` | Postgres database | `user_db` |
| `USER_DB_SSLMODE` | SSL mode | `disable` |
| `USER_DB_MAX_OPEN_CONNS` | Connection pool max open | `25` |
| `USER_DB_MAX_IDLE_CONNS` | Connection pool max idle | `25` |
| `USER_DB_CONN_MAX_LIFETIME` | Connection max lifetime | `5m` |
| `USER_DB_CONN_MAX_IDLE_TIME` | Connection max idle time | `5m` |
| `USER_CORS_ALLOWED_ORIGINS` | Allowed CORS origins | `*` |
| `USER_RATE_LIMIT_REQUESTS` | Max requests per window | `100` |
| `USER_RATE_LIMIT_WINDOW` | Rate limit window duration | `1m` |

## Roadmap

### Done

- [x] Layered architecture — handler → service → repository with interface-based DI
- [x] Go workspace (multi-module) — `go.work` + per-service `go.mod` + shared `pkg` module
- [x] Body size limit + strict JSON decoding (`DisallowUnknownFields`, trailing-data rejection)
- [x] PUT full replacement semantics — every field required, zero values written
- [x] Request ID middleware — UUID per request, in context + response header, log correlation
- [x] Health endpoints — `/healthz` (liveness) and `/readyz` (readiness, DB ping)
- [x] Server timeouts — `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes`
- [x] API versioning — `/v1/` prefix
- [x] DELETE returns 204 No Content
- [x] Structured logging — `log/slog` JSON, request-scoped logger via context
- [x] Filtering & sorting — allowlisted `?sort=` and `?filter[field]=` on list endpoints (both services)
- [x] Timestamps — `created_at` / `updated_at` on User and Product models
- [x] DB connection pool tuning — configurable via env vars
- [x] CORS — configurable allowed origins
- [x] Rate limiting — Redis-backed fixed-window, per-service namespaced key, fails open
- [x] Panic recovery middleware
- [x] Graceful shutdown — SIGINT/SIGTERM with context timeout
- [x] OpenAPI 3 — embedded YAML + Swagger UI per service
- [x] Per-service Postgres databases (database-per-service)
- [x] Docker Compose — healthchecks, `depends_on: condition: service_healthy`
- [x] Hot reload — per-service `air` configs watching workspace root
- [x] Migrations — versioned SQL with up/down, per-service migrate script

### Planned

- [ ] Authentication & authorization — JWT access/refresh tokens, bcrypt passwords, RBAC
- [ ] API gateway — single public entry point, JWT verification, identity header propagation
- [ ] `PATCH` endpoints — partial updates alongside existing `PUT`
- [ ] `ReadHeaderTimeout` on HTTP server — Slowloris defense
- [ ] Optimistic concurrency — `ETag` / `If-Match` or `version` column on updates
- [ ] Soft delete — `deleted_at` column for recoverability
- [ ] Metrics — `/metrics` Prometheus endpoint with request duration histograms
- [ ] Tests — handler integration tests + service unit tests with repository fakes
- [ ] K8s manifests — Deployment, Service, Ingress, liveness/readiness probe wiring
- [ ] Production Dockerfile — multi-stage, distroless/scratch final image, non-root user
