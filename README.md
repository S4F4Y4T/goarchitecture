# go-microservice

A production-grade Go microservice monorepo with four independent services — **auth**, **user**, **notification**, and **docs** — sharing a common `pkg` library, fronted by Kong as the single public API gateway. Built on the standard library `net/http` mux with GORM for persistence.

## Stack

- **Language:** Go 1.25 (workspace: `go.work`)
- **Router:** `net/http` (`http.ServeMux` with method+path patterns, Go 1.22+)
- **ORM:** GORM (PostgreSQL via `gorm.io/driver/postgres`)
- **API gateway:** Kong (DB-less) — JWT verification, rate limiting, CORS, correlation ID, identity header injection
- **East-west RPC:** gRPC (`auth` → `user`), contracts in `pkg/proto`, validated via `protovalidate`
- **Async messaging:** RabbitMQ — `user` publishes domain events, `notification` consumes them (see [docs/messaging.md](docs/messaging.md))
- **Email:** SMTP via `pkg/mailer` — Mailpit locally, any real provider in production (see [docs/email.md](docs/email.md))
- **Validation:** `go-playground/validator/v10` (HTTP), `protovalidate` (gRPC)
- **Docs:** a single combined OpenAPI 3 YAML served by `docs`, rendered with `swagger-ui-dist`
- **Hot reload:** `air` (per-service `.air.toml`)
- **Migrations:** `golang-migrate`

## Repository layout

```
go-microservice/
├── go.work                          # workspace root — links pkg + every service
├── go.work.sum
│
├── pkg/                             # shared module (github.com/s4f4y4t/go-microservice/pkg)
│   ├── apperror/                    # typed error codes + HTTP/gRPC status mapping
│   ├── config/                      # env helpers (GetEnvInt, GetEnvDuration, GetEnvBool)
│   ├── events/user/                 # wire contract for events the user service publishes
│   ├── grpcmiddleware/              # gRPC interceptors: RequestID, Logger, Recovery, Validation, Timeout
│   ├── logger/                      # slog JSON logger + request-scoped context helpers
│   ├── mailer/                      # Sender interface + SMTP implementation
│   ├── messaging/rabbitmq/          # connection, envelope, publisher, retry/DLQ topology, consumer
│   ├── middleware/                  # RequestID, Logger, Auth, PanicRecovery, Chain
│   ├── pagination/                  # Params + Meta for list endpoints
│   ├── proto/                       # .proto contracts + generated gRPC stubs (e.g. proto/user)
│   ├── query/                       # ORM-free allowlisted sort + filter parsing
│   │   └── gorm/                    # GORM scope adapters for query.Options
│   ├── request/                     # JSON decode with size cap + unknown-field rejection
│   ├── response/                    # ApiResponse envelope writer (success / error / meta)
│   ├── token/                       # JWT sign/verify
│   └── validation/                  # validator/v10 wrapper with JSON field names + messages
│
├── services/
│   ├── auth/                        # module: .../services/auth — JWT issuance, login/register/refresh
│   ├── user/                        # module: .../services/user — user CRUD, gRPC server, publishes user.created
│   ├── notification/                # module: .../services/notification — consumes events, sends email
│   └── docs/                        # module: .../services/docs — serves the combined OpenAPI spec
│       └── (each service: cmd/api/main.go, internal/{app,config,health,platform,router,<domain>}/, .air.toml, go.mod)
│
├── database/
│   └── migrations/
│       ├── user/
│       └── notification/
│
├── deploy/
│   ├── docker/                      # one Dockerfile.<service> per service
│   ├── kong/                        # declarative Kong config (kong.yml) + JWT keys
│   └── k8s/                         # placeholder, not yet implemented
│
├── scripts/
│   └── migrate.sh                   # wraps golang-migrate; reads per-service prefixed env vars
│
├── docker-compose.yml               # all services + per-service postgres + redis + rabbitmq + mailpit + kong + pgadmin
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
make migrate-up SVC=notification
```

All traffic goes through Kong on `http://localhost:8000`:

| Service | Internal port(s) | Route via Kong |
|---|---|---|
| auth | 6868 (HTTP) | `/v1/auth` |
| user | 6969 (HTTP), 6970 (gRPC, internal-only) | `/v1/users` |
| notification | 7171 (HTTP) | `/v1/notifications` (read-only) |
| docs | 9090 (HTTP) | `/docs` |

Other local-dev ports (not behind Kong): RabbitMQ management UI `http://localhost:15672`, Mailpit UI `http://localhost:8025`, pgAdmin `http://localhost:5050`.

Health probes (unversioned, no auth, checked directly against each service):
- `GET /healthz` — liveness (process alive, no dep checks)
- `GET /readyz` — readiness (DB ping, returns 503 when DB unreachable)

## Local development (hot reload)

```bash
# Run a service with live reload
make dev SVC=user
make dev SVC=notification
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
| `make tidy` | `go mod tidy` in pkg and every service module |
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
GET /v1/notifications/?filter[status]=failed&sort=-created_at
```

Allowlisted fields per resource:

| Resource | Sortable | Filterable (partial) | Filterable (exact) |
|---|---|---|---|
| users | `id`, `name`, `email`, `created_at`, `updated_at` | `name`, `email` | `id` |
| notifications | `id`, `type`, `recipient`, `status`, `created_at`, `updated_at` | `recipient` | `type`, `status` |

### HTTP semantics

- `PUT` does full replacement — all fields required; zero values are written unconditionally.
- `DELETE` returns `204 No Content` with an empty body.
- `POST` returns `201 Created`.

## Middleware chain (per service)

```
RequestID → Logger → PanicRecovery → router
```

CORS, rate limiting, and JWT verification moved to the Kong gateway (see [docs/api-gateway.md](docs/api-gateway.md)) — services no longer apply them themselves.

- **RequestID** — reuses inbound `X-Request-ID` or generates a UUID; echoed on the response; injected into context so all log lines carry it.
- **Logger** — structured JSON access log (method, path, status, duration_ms) via `slog`.
- **PanicRecovery** — catches panics, logs stack trace, returns 500 JSON.

## Environment variables

All variables are prefixed per service in the root `.env`. See `.env.example` for defaults.

| Prefix | Service |
|---|---|
| `AUTH_*` | auth service |
| `USER_*` | user service |
| `NOTIFICATION_*` | notification service |
| `DOCS_*` | docs service |
| `REDIS_*` | shared Redis |
| `RABBITMQ_*` | shared RabbitMQ |
| `KONG_*` | Kong gateway |
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

Tracked in [docs/next.md](docs/next.md) — a phased, dependency-ordered checklist (auth hardening → observability → API gateway → gRPC → async messaging → ... → Kubernetes) kept current as each phase lands. That file is the single source of truth for what's done vs. planned; this README doesn't duplicate it.

## Further reading

Per-topic docs live under `docs/`: [microservice.md](docs/microservice.md) (service boundaries), [grpc.md](docs/grpc.md) (auth↔user RPC), [messaging.md](docs/messaging.md) (RabbitMQ events), [email.md](docs/email.md) (SMTP + templates), [api-gateway.md](docs/api-gateway.md) (Kong), [auth.md](docs/auth.md) (JWT), and more.
