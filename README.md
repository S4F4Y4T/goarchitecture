# Microservice

A small Go REST API for managing users, built on the standard library `net/http` mux with GORM for persistence.

## Stack

- **Language:** Go 1.24+
- **Router:** `net/http` (`http.ServeMux` with method+path patterns)
- **ORM:** GORM (PostgreSQL)
- **Validation:** `go-playground/validator/v10`
- **Docs:** OpenAPI 3 served via Swagger UI (`swaggo/http-swagger`)
- **Hot reload:** `air`
- **Migrations:** `golang-migrate`

## Project layout

```
cmd/api/                 entrypoint
config/                  config + DB setup
database/migration/      SQL migrations
docs/                    embedded OpenAPI spec
internals/
  bootstrap/             wires repo -> service -> handler
  handler/               HTTP handlers
  service/               business logic
  repository/            GORM data access
  model/                 domain types + repo interfaces
  dto/                   request/response shapes
  middleweare/           logger, cors, panic recovery
pkg/
  appError/              typed errors + HTTP status mapping
  pagination/            page-based pagination helpers
  response/              ApiResponse envelope (success/error/meta)
  validation/            struct validation + field-level errors
router/                  route registration
```

## Quickstart

```bash
# 1. Start Postgres
docker compose up -d

# 2. Run migrations
make migrate-up

# 3. Run the server (hot reload via air)
make dev
# or without hot reload
make run
```

Default server: `http://localhost:6969`
API base path: `/v1` (e.g. `http://localhost:6969/v1/users/`)
Swagger UI: `http://localhost:6969/swagger/`
Health probes: `GET /healthz` (liveness), `GET /readyz` (readiness, DB ping) — unversioned.

## Conventions

### Response envelope

All endpoints return:

```json
{
  "success": true,
  "message": "Users retrieved successfully",
  "data": [...],
  "meta": { "page": 1, "limit": 10, "total": 42, "total_pages": 5 },
  "error": null
}
```

`meta` is only present on list endpoints. `error` is only present on failures.

### Pagination

List endpoints accept `?page=` and `?limit=` query params.

| Param  | Default | Min | Max |
|--------|---------|-----|-----|
| page   | 1       | 1   | —   |
| limit  | 10      | 1   | 100 |

Out-of-range values are clamped, not rejected.

### Error format

```json
{
  "success": false,
  "error": {
    "code": "INVALID_INPUT",
    "message": "validation failed",
    "fields": [{ "field": "email", "message": "Email must be a valid email address" }]
  }
}
```

Codes: `NOT_FOUND`, `INVALID_INPUT`, `CONFLICT`, `UNAUTHORIZED`, `FORBIDDEN`, `INTERNAL`.

## Make targets

| Target                         | What it does                            |
|--------------------------------|-----------------------------------------|
| `make run`                     | Run the server                          |
| `make dev`                     | Run with hot reload via `air`           |
| `make build`                   | Build to `./bin/api`                    |
| `make test`                    | Run tests                               |
| `make lint`                    | Run `golangci-lint`                     |
| `make tidy`                    | `go mod tidy`                           |
| `make migrate-up`              | Apply migrations                        |
| `make migrate-down`            | Roll back migrations                    |
| `make migrate-create name=foo` | Create new migration files              |

## Roadmap

Planned REST best-practice improvements, ordered by impact.

### High value

- [x] **Body size limit** — wrap `r.Body` with `http.MaxBytesReader` before decoding to prevent unbounded uploads.
- [x] **Strict JSON decoding** — call `dec.DisallowUnknownFields()` so typos in client payloads fail loudly instead of silently dropping fields.
- [x] **PUT vs PATCH semantics** — `PUT` now does full replacement: every field is required and is written unconditionally (zero values included).
- [x] **Request ID middleware** — generate a UUID per request, inject into context + `X-Request-ID` header, thread through all logs.
- [x] **Health endpoints** — `GET /healthz` (liveness) and `GET /readyz` (readiness with DB ping) for container orchestrators.
- [x] **`IdleTimeout` on `http.Server`** — add `IdleTimeout: 60 * time.Second` so keep-alive connections don't pile up.
- [x] **API versioning** — prefix routes with `/v1/` so breaking changes can ship without breaking existing clients.

### Medium value

- [ ] **`DELETE` returns 204** — currently returns 200 with `{"data": null}`; convention is 204 No Content.
- [ ] **Structured logging** — replace `log.Printf` / `fmt.Printf` mix with `log/slog`; pairs with the request-ID middleware.
- [ ] **Filtering & sorting** — extend list endpoints with `?sort=`, `?filter[name]=`, etc.
- [ ] **Timestamps** — add `created_at` / `updated_at` to the User model (most UIs eventually need them).
- [ ] **Optimistic concurrency** — `ETag` / `If-Match` or a `version` column on update.

### Polish / future

- [ ] **Rate limiting** — middleware-level token bucket.
- [ ] **Authentication & authorization** — currently no auth on any endpoint.
- [ ] **Metrics** — `/metrics` Prometheus endpoint with request duration histograms.
- [ ] **Tests** — no `_test.go` files yet; start with handler-level integration tests against a test DB.
- [ ] **CORS allowed origins from config** — avoid hardcoded values in middleware.
- [ ] **Soft delete** — `deleted_at` column instead of hard delete, for recoverability.
- [ ] **Trailing-slash canonicalization** — `/users/` works but `/users` 404s; redirect or document.
