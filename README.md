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

#### Security
- [ ] **Authentication** — JWT access token (15 min) + refresh token (7 days); `pkg/token` with `golang-jwt/jwt/v5`; pin signing method to prevent `alg:none` attack
- [ ] **Password hashing** — `bcrypt` (or `argon2id`); store only `password_hash`; `json:"-"` tag so it never serializes; generic "invalid credentials" message on failure (prevents user enumeration)
- [ ] **RBAC** — `role` column on users (`user` | `admin`); `RequireRole` middleware; seeded admin via migration
- [ ] **`ReadHeaderTimeout`** — add to `http.Server` to defend against Slowloris attacks (currently only `ReadTimeout` is set)
- [ ] **Security headers** — `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Strict-Transport-Security` (HSTS behind TLS)
- [ ] **CORS tightened** — lock `CORS_ALLOWED_ORIGINS` default to empty (deny all) instead of `*`; require explicit opt-in in production
- [ ] **Audit logging** — structured log entries for auth events (login success/failure with `request_id`, never including the password)
- [ ] **Dependency scanning** — `govulncheck` in CI; Dependabot or Renovate for automated dep updates

#### API Gateway
- [ ] **Gateway service** — `services/gateway/` (or `cmd/gateway/`); single public entry point on port 8080; all other services on a private Docker network with no published ports
- [ ] **JWT verification at the edge** — gateway verifies `Authorization: Bearer <token>` once; 401 before traffic reaches services
- [ ] **Identity header injection** — gateway strips inbound `X-User-*` headers (spoofing defense), then injects `X-User-ID`, `X-User-Email`, `X-User-Role` from verified claims; services trust these over the private network
- [ ] **`pkg/identity`** — small middleware + context helper: `identity.FromContext(ctx) → (Identity, bool)`; used by services to read injected identity
- [ ] **Gateway readyz fan-out** — `/readyz` on the gateway pings each upstream's `/readyz` and aggregates results
- [ ] **Proxy resilience** — `Transport.ResponseHeaderTimeout` so a slow/dead upstream doesn't hold connections; `ErrorHandler` returns JSON 502/504 instead of default HTML

#### API & Data
- [ ] **`PATCH` endpoints** — partial updates alongside existing `PUT` (full replacement); clients can update a single field without re-sending the whole resource
- [ ] **`Price` as integer cents** — change `Product.Price` from `float64` to `int64` (store cents, e.g. `999` = $9.99); update migration column from `DECIMAL` to `BIGINT`; float64 causes rounding errors on monetary arithmetic
- [ ] **User and Product IDs as `int64`** — current `int` type truncates on 32-bit builds; `BIGSERIAL` / `SERIAL` map to 64-bit in Go
- [ ] **Optimistic concurrency** — `ETag` / `If-Match` header pair, or a `version INTEGER` column incremented on every update; prevents lost-update race conditions
- [ ] **Soft delete** — `deleted_at TIMESTAMPTZ` column; GORM `DeletedAt` field; hard deletes replaced with a `WHERE deleted_at IS NULL` filter; allows data recovery
- [ ] **Fix migration gap** — user migrations jump from `000001` to `000003`; a future `000002` will not auto-apply because `000003` is already recorded; renumber or insert a no-op `000002`
- [ ] **Fix OpenAPI specs** — split the single shared YAML into two: user spec lists only `/v1/users/*` paths; catalog spec lists only `/v1/products/*` paths; fix catalog server URL (`6969` → `7070`); add `created_at`/`updated_at` to both schemas; document `sort`/`filter` params on catalog list endpoint; correct DELETE response from `200` to `204`

#### Observability
- [ ] **Service label in logs** — `logger.Init` should attach `slog.With("service", "user")` so logs from multiple services are filterable when aggregated
- [ ] **Metrics** — `/metrics` Prometheus endpoint; request count + latency histograms per route and status code; DB connection pool gauges; rate-limit rejection counter
- [ ] **Distributed tracing** — OpenTelemetry: gateway starts a root span, `traceparent` header propagates to services, services create child spans; export to Jaeger or Tempo

#### Resilience
- [ ] **Graceful shutdown timeout** — raise from `1s` to `10–15s`; 1 second cuts in-flight requests under load; Kubernetes allows up to 30s before force-kill
- [ ] **Circuit breaker** — `sony/gobreaker` on gateway → service calls; fail fast and return 503 when an upstream is down instead of queuing goroutines
- [ ] **Sliding-window rate limit** — replace the current fixed-window algorithm; fixed-window allows `2×N` requests in `2×1ms` at the window boundary; sliding window eliminates the burst

#### Testing
- [ ] **`pkg/` unit tests** — `pkg/query.Parse` (unknown fields dropped, sort direction), `pkg/pagination` (clamping), `pkg/apperror` (HTTP status mapping)
- [ ] **Service unit tests** — fake the repository interface (already defined in `model/`) to test business rules without a DB; e.g., create with duplicate email → Conflict
- [ ] **Handler tests** — `httptest.NewRecorder` against the real router; assert status codes, envelope shape, field-level validation errors
- [ ] **Repository integration tests** — `testcontainers-go` spins a real Postgres; build tag `//go:build integration`
- [ ] **E2E smoke test** — `docker compose up` → register → login → create product with token → assert 401 without token

#### Infrastructure
- [ ] **Production Dockerfile** — multi-stage: build in `golang:1.25` → final in `gcr.io/distroless/static`; `CGO_ENABLED=0`; non-root user; single static binary; one `Dockerfile` with `ARG SERVICE`
- [ ] **Network segmentation in Compose** — `edge` network (gateway only, published port) + `backend` network (services + DBs, internal); services have no `ports:` exposed
- [ ] **Migration job in Compose** — one-shot `migrate/migrate` container per service; app service `depends_on: condition: service_completed_successfully` instead of manual `make migrate-up`
- [ ] **K8s manifests** — Deployment + Service + Ingress per service; liveness/readiness probes wired to `/healthz`/`/readyz`; HorizontalPodAutoscaler; ConfigMap/Secret for env
- [ ] **`pgadmin` behind a Compose profile** — move pgadmin to a `tools` profile (`docker compose --profile tools up`) so it doesn't start on every `docker compose up`

#### Inter-service Communication
- [ ] **gRPC for east-west traffic** — service-to-service calls (e.g. catalog fetching user details) use gRPC instead of REST; strongly typed contracts via protobuf; faster than JSON over HTTP; `buf` for schema management and linting; REST stays north-south (client ↔ gateway)
- [ ] **RabbitMQ / NATS for async events** — decouple services via a message broker; e.g. `UserDeleted` event published by user-service, consumed by catalog-service to clean up owned products; services no longer need to call each other synchronously for side effects
- [ ] **Outbox pattern** — guarantee events are published even if the broker is temporarily down; write the event to an `outbox` DB table in the same transaction as the business write, then a relay process publishes and deletes it; prevents dual-write inconsistency
- [ ] **Transactional email** — dedicated email service (or `pkg/mailer`) backed by SMTP / SendGrid / AWS SES; triggered by domain events (welcome email on register, password-reset link, order confirmation); never call the mail provider synchronously in the request path — publish a job to a queue and send in the background so a slow/failing mail provider doesn't degrade the API
- [ ] **Background job queue** — Redis-backed worker queue (e.g. `asynq`) or broker-backed (RabbitMQ); handles tasks that must not block HTTP responses: sending emails, generating reports, resizing images, expiring stale records; each service registers its own workers; jobs are retried with exponential backoff on failure; dead-letter queue for permanently failed jobs
