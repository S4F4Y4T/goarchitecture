# Enterprise Go Microservices — Blueprint

A topic-by-topic guide for evolving this codebase into a **production-grade microservice system** with an **API Gateway** and **JWT authentication**. Sections marked ✅ are already implemented; sections marked 🔲 are planned next steps.

---

## 1. Target Architecture

```
                            ┌──────────────────────────────────────┐
                            │             API GATEWAY              │
                            │  - single public entry point         │
                            │  - JWT verification (authn)          │
                            │  - rate limiting (per client IP)     │
                            │  - CORS, security headers            │
                            │  - request ID generation/propagation │
                            │  - identity header injection         │
                            │    (X-User-ID, X-User-Email, X-Role) │
                            └───────────┬──────────────┬───────────┘
                    /api/v1/auth/*       │              │  /api/v1/products/*
                    /api/v1/users/*      │              │
                                         ▼              ▼
                     ┌──────────────────────┐   ┌──────────────────────┐
                     │     USER SERVICE     │   │   CATALOG SERVICE    │
                     │        :8081         │   │        :8082         │
                     │  - register/login    │   │  - product CRUD      │
                     │  - issues JWTs       │   │  - authz via         │
                     │  - bcrypt passwords  │   │    identity headers  │
                     │  - user CRUD (admin) │   │                      │
                     └──────────┬───────────┘   └──────────┬───────────┘
                                │                          │
                                ▼                          ▼
                     ┌──────────────────────┐   ┌──────────────────────┐
                     │      user-db         │   │    catalog-db        │
                     │   (Postgres :5432)   │   │   (Postgres :5432)   │
                     └──────────────────────┘   └──────────────────────┘

        Key rule: services are NEVER exposed publicly. Only the gateway
        publishes a port. Services live on a private Docker network.
```

**Core principles encoded:**
- **API Gateway pattern** — cross-cutting concerns (auth, rate limit, CORS) live once at the edge, not duplicated in every service.
- **Database-per-service** ✅ — each service owns its own Postgres instance; services never read each other's tables.
- **Identity propagation** 🔲 — gateway verifies the JWT once, forwards trusted headers (`X-User-ID`, `X-User-Email`, `X-User-Role`) over the private network.
- **Network segmentation** 🔲 — only the gateway is on a public-facing network; services are internal.

---

## 2. Repository Layout ✅

The repo uses a **Go workspace (multi-module)** layout — one module per service plus one shared `pkg` module, tied together by a root `go.work` file.

```
go-microservice/               ← workspace root
├── go.work                    ← links pkg, services/user, services/catalog
├── pkg/                       ← shared module (ORM-free, HTTP-generic)
│   ├── apperror/
│   ├── config/
│   ├── logger/
│   ├── middleware/
│   ├── pagination/
│   ├── query/ + query/gorm/
│   ├── request/
│   ├── response/
│   └── validation/
├── services/
│   ├── user/                  ← independent Go module
│   │   ├── cmd/api/main.go
│   │   └── internal/          ← compiler-enforced privacy
│   │       ├── bootstrap/
│   │       ├── config/
│   │       ├── dto/
│   │       ├── handler/
│   │       ├── model/         ← domain types + repository interfaces
│   │       ├── repository/
│   │       ├── router/
│   │       └── service/
│   └── catalog/               ← independent Go module (same shape)
├── database/migrations/
│   ├── user/
│   └── catalog/
└── deploy/
    ├── docker/
    └── k8s/                   ← placeholder; see section 13
```

**Why this layout:**
- `internal/` is the Go-compiler-enforced privacy boundary — no external module can import it.
- Each service is a separate Go module — teams can evolve dependencies independently.
- `pkg/` is shared infrastructure (logger, errors, middleware) but never shared business logic. Duplication of business code between services is accepted — shared business code creates coupling.
- `go.work` + `replace` directives allow `go mod tidy` to resolve the local `pkg` module without a published tag.

---

## 3. API Gateway 🔲

The gateway is a thin reverse proxy — **no business logic, ever**.

| Concern | Approach |
|---|---|
| Routing | Declarative route table: path prefix → upstream URL |
| Reverse proxy | `net/http/httputil.ReverseProxy` with the `Rewrite` hook |
| Path mapping | Public `/api/v1/products/...` → strip `/api` → service sees `/v1/products/...` |
| AuthN | Verify `Authorization: Bearer <jwt>` once; 401 before traffic reaches services |
| Identity injection | **Delete inbound** `X-User-*` headers (spoofing defense), then set from verified claims |
| Public routes | `POST /auth/register`, `POST /auth/login`, `POST /auth/refresh`, `GET /products*`, health probes |
| Rate limiting | Token bucket per IP at the edge; 429 + `Retry-After`; stricter bucket on auth endpoints |
| Resilience | `ResponseHeaderTimeout` on proxy transport; `ErrorHandler` returns JSON 502/504 |
| Health | `/healthz` (self), `/readyz` (pings each upstream's `/readyz`) |

**Routing table:**

```
PUBLIC    POST /api/v1/auth/register   → user-service
PUBLIC    POST /api/v1/auth/login      → user-service
PUBLIC    POST /api/v1/auth/refresh    → user-service
AUTH      GET  /api/v1/auth/me         → user-service
AUTH+ADMIN     /api/v1/users/**        → user-service
PUBLIC    GET  /api/v1/products/**     → catalog-service
AUTH      POST|PUT|DELETE /api/v1/products/** → catalog-service
```

Go 1.22+ `http.ServeMux` method+wildcard patterns handle this without an external router — consistent with the stdlib-first approach used throughout.

---

## 4. JWT Authentication 🔲

### Token design

- **Access token**: short-lived (15 min), HS256-signed, carried on every request.
- **Refresh token**: long-lived (7 days), used only on `POST /auth/refresh`. Distinguished by a `token_type` claim (`"access"` / `"refresh"`) — prevents a refresh token from being accepted as an access token.
- **Claims**: `sub` (user ID), `iss`, `exp`, `iat`, `jti`, `email`, `role`. Keep small — they ride on every request.
- **Library**: `github.com/golang-jwt/jwt/v5`. Always pin the expected signing method (`jwt.WithValidMethods(["HS256"])`) — guards against the `alg: none` attack.

### Signing algorithm

- **HS256 (symmetric)** — shared secret between user-service (signs) and gateway (verifies). Simple; appropriate when both sides are yours. Secret = 32+ random bytes from env/secret manager, never committed.
- **RS256/EdDSA (asymmetric)** — user-service holds private key; gateway verifies with public key (distributed via JWKS endpoint). Enterprise upgrade path: verifiers can't mint tokens.

### Password handling

- `golang.org/x/crypto/bcrypt` (or argon2id) — never SHA/MD5.
- Store only `password_hash`; struct tag `json:"-"` prevents it from ever serializing.
- Login failures return one generic message ("invalid credentials") for wrong email AND wrong password — prevents user enumeration.

### Auth endpoints (user-service)

```
POST /v1/auth/register  {name, email, password}   → 201
POST /v1/auth/login     {email, password}          → 200 {access_token, refresh_token, expires_in}
POST /v1/auth/refresh   {refresh_token}            → 200 {access_token, refresh_token}  (rotate)
GET  /v1/auth/me        (via gateway identity)     → 200 current user
```

### Authorization (RBAC)

- `role` column on users (`user` | `admin`); seeded admin via migration.
- Gateway does **authentication**; services do **authorization** — the resource owner knows its rules.
- `RequireRole("admin")` middleware on `/v1/users` CRUD.
- Ownership checks in the service layer (e.g., `created_by` vs `X-User-ID`).

### Identity propagation

- Gateway → services: `X-User-ID`, `X-User-Email`, `X-User-Role` headers on the private network.
- Services read them via a small `pkg/identity` middleware: `identity.FromContext(ctx) → (Identity, bool)`.
- **Threat model**: these headers are trusted only because services are unreachable except through the gateway. The gateway must strip any client-supplied `X-User-*` headers before injecting its own.

### Known tradeoffs

- Stateless JWT logout → token denylist in Redis (store `jti`, TTL = token expiry) or short access TTL + refresh rotation.
- Refresh token storage: httpOnly cookie vs response body — cookie is harder to steal via XSS.
- Clock skew between services: `jwt.WithLeeway(5 * time.Second)`.

---

## 5. Per-Service Layer Architecture ✅

```
router → middleware → handler → service → repository → DB
              ↑           ↑         ↑          ↑
           pkg/midw    dto+valid  business   GORM +
                                   rules    apperror mapping
```

- **Handler** ✅ — HTTP only: decode (`pkg/request`), validate (`pkg/validation`), call service, write envelope (`pkg/response`). No business rules.
- **Service** ✅ — business rules (uniqueness checks, password hashing, token issuing). Depends on repository interfaces declared in `model/` — this is dependency inversion and what makes services unit-testable with fakes.
- **Repository** ✅ — GORM; maps DB errors (SQLSTATE 23505 → Conflict) to `apperror`. `isUniqueViolation` is the right pattern — keep it.
- **Bootstrap** ✅ — manual constructor wiring. Manual DI > frameworks at this scale.
- **Model** ✅ — domain struct + repository interface + list schema in one file. Interface in model package means the service layer doesn't import the repository package (clean dependency direction).

**What changes with auth**: `User` gains `PasswordHash string \`json:"-"\``, `Role string`. User-service gains an `internal/handler/auth.go` + `internal/service/auth.go` (bcrypt, JWT issuing).

---

## 6. Configuration Management ✅

- **12-factor**: all config from env vars; `godotenv.Load()` is best-effort (`_ = godotenv.Load()`) so containers that inject env directly still work. ✅
- Fail fast on missing required vars at startup, listing all missing vars at once. ✅ (`loadDBConfig` collects all missing keys)
- Shared env helpers (`GetEnvInt`, `GetEnvDuration`) in `pkg/config`. ✅
- Typed config structs per service with clear zero-value defaults. ✅
- Future: validate values not just presence — e.g., JWT secret minimum 32 bytes, valid URL format for upstream URLs.
- Secrets: env vars in dev/compose; secret manager (Vault / AWS Secrets Manager / K8s Secrets) in production — never in the image, never in git.

---

## 7. Database Topics ✅

- **Database-per-service** ✅ — two Postgres containers, separate credentials, separate volumes. No cross-service JOINs; need data from another service? Call its API.
- **Migrations per service** ✅ — `database/migrations/user/` and `database/migrations/catalog/` with `golang-migrate` versioned SQL files.
- **Connection pool tuning** ✅ — `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`, `ConnMaxIdleTime` all configurable via env.
- **Startup ping with timeout** ✅ — fails fast with a clear error instead of discovering DB is unreachable on first request.
- **Context everywhere** ✅ — `db.WithContext(ctx)` on every query; cancellation propagates.

**Distributed-data theory to know**: saga pattern for cross-service transactions, eventual consistency, outbox pattern for reliable event publishing. There is **no distributed transaction** between databases — design to avoid needing one.

**Future**: run migrations as a one-shot job in compose (`depends_on: condition: service_completed_successfully`) instead of a manual `make migrate-up` step.

---

## 8. Cross-Cutting Middleware ✅

Current chain (per service, innermost last):

```
RequestID → Logger → CORS → RateLimit(Redis) → PanicRecovery → router
```

**What each does:**

| Middleware | Status | Notes |
|---|---|---|
| `RequestID` | ✅ | Reuses inbound `X-Request-ID` or generates UUID; echoes on response; in context. Becomes the correlation ID across services once a gateway exists. |
| `Logger` | ✅ | JSON access log: method, path, status, duration_ms via `slog`. |
| `CORS` | ✅ | Configurable allowed origins; `Vary: Origin` when not wildcard; handles OPTIONS preflight. |
| `RateLimit` | ✅ | Fixed-window per IP, Redis-backed, per-service namespaced key (`rl:user:<ip>`), fails open. |
| `PanicRecovery` | ✅ | Catches panics, logs stack, returns 500 JSON. |
| `SecurityHeaders` | 🔲 | `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, HSTS. |
| `ReadHeaderTimeout` | 🔲 | Add to `http.Server` to defend against Slowloris — currently only `ReadTimeout` is set. |

**CORS note**: once a gateway exists, CORS moves entirely to the gateway. Services on the private network don't need it.

---

## 9. Error Handling Contract ✅

- Central error type (`apperror.AppError`) with machine code, human message, optional field errors, wrapped cause (`Unwrap`), HTTP status mapping. ✅
- Errors normalized at one place (`response.Error` → `apperror.From`). ✅
- Internal causes logged server-side only; generic message to client — no DB error leakage. ✅
- Error codes: `NOT_FOUND`, `INVALID_INPUT`, `CONFLICT`, `UNAUTHORIZED`, `FORBIDDEN`, `TOO_MANY_REQUESTS`, `INTERNAL`. ✅
- Consistent envelope (`success`, `status_code`, `data`, `meta`, `error{code,message,fields}`) across all services. ✅

**One open item**: `apperror.From` imports GORM to map `ErrRecordNotFound`. Since repositories already do this mapping, removing the GORM import from `pkg/apperror` would keep it dependency-free — important when the gateway (which has no DB) imports `pkg/apperror` for its own error responses.

---

## 10. Observability

| Concern | Status | Notes |
|---|---|---|
| Structured logging | ✅ | slog JSON to stdout, level from `LOG_LEVEL` env, request-scoped logger with `request_id` |
| Log correlation | ✅ | Same `X-Request-ID` flows gateway → service; one ID reconstructs the full request path |
| Health probes | ✅ | `/healthz` (liveness, no deps) and `/readyz` (DB ping, 503 on failure) |
| Service label in logs | 🔲 | Add `slog.With("service", "user")` in `logger.Init` so aggregated logs are filterable |
| Metrics | 🔲 | Prometheus `/metrics`: request count/latency histograms, DB pool gauges, rate-limit rejections |
| Distributed tracing | 🔲 | OpenTelemetry: gateway starts span, `traceparent` header propagates, services add child spans |

Context propagation is already in place throughout — the structural prep for tracing is done.

---

## 11. Resilience

| Pattern | Status | Notes |
|---|---|---|
| Timeouts (server) | ✅ | `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes` on `http.Server` |
| Graceful shutdown | ✅ | SIGINT/SIGTERM → `srv.Shutdown(ctx)`. Raise the 1s timeout to 10–15s — 1s cuts in-flight requests; orchestrators allow ~30s. |
| Rate limiting | ✅ | Fixed-window per IP at service level. Move to gateway edge once gateway exists. |
| `ReadHeaderTimeout` | 🔲 | Slowloris defense — add to `http.Server` |
| Upstream timeouts | 🔲 | Gateway `Transport.ResponseHeaderTimeout` — never wait forever on a service |
| Circuit breaker | 🔲 | `sony/gobreaker` — fail fast when a service is down instead of piling goroutines |
| Retries | 🔲 | Idempotent GETs only; exponential backoff with jitter |

---

## 12. API Design ✅

- URL versioning `/v1/` ✅ — `/v2/` can ship breaking changes without breaking existing clients.
- REST conventions: plural resource nouns, 201 on create, 204 on delete, PUT = full replacement. ✅
- Allowlisted sort/filter (`?sort=-price&filter[name]=widget`) with `query.Schema` — column names reach SQL only from the allowlist, values always parameterized. ✅ Both services.
- Pagination envelope (`meta: {page, limit, total, total_pages}`). ✅
- OpenAPI 3 per service, embedded YAML, Swagger UI at `/swagger/`. ✅

**Planned:**
- `PATCH` endpoints for partial updates alongside `PUT`.
- Gateway-level unified `openapi.yaml` covering all services with `bearerAuth` security scheme.
- Idempotency-Key header support for POST operations.

---

## 13. Containerization ✅ / 🔲

| Concern | Status | Notes |
|---|---|---|
| Dev hot-reload | ✅ | Air + volume-mount; per-service `.air.toml` watching workspace root |
| Docker Compose | ✅ | 2×app + 2×postgres + redis + pgadmin; healthchecks; `depends_on: condition: service_healthy` |
| Layer caching | ✅ | `go.mod`/`go.sum` copied before source; dep layer only busts on dependency changes |
| Go version pinned | ✅ | `golang:1.25-alpine` in both Dockerfiles |
| Network segmentation | 🔲 | Services should be on a private network; only gateway publishes a port |
| Migration job | 🔲 | One-shot `migrate/migrate` container; services `depends_on: condition: service_completed_successfully` |
| Production Dockerfile | 🔲 | Multi-stage: build in `golang:1.25` → final in `gcr.io/distroless/static`; non-root user; `CGO_ENABLED=0` |
| K8s manifests | 🔲 | Deployment, Service, Ingress, HPA; liveness/readiness probes wired to `/healthz`/`/readyz` |

---

## 14. Testing Strategy 🔲

Nothing tested yet. Priority order:

1. **Pure `pkg/` unit tests** — `pkg/query.Parse` (unknown fields dropped, sort direction), `pkg/pagination` (clamping), `pkg/apperror` (HTTP status mapping).
2. **Service unit tests** — fake the repository interface (already defined in `model/`) to test business rules: create with existing email → Conflict, update own email unchanged → no conflict check, delete non-existent → NotFound.
3. **Handler tests** — `httptest.NewRecorder` against the router: correct status codes, envelope shape, validation errors with field details.
4. **Repository integration tests** — `testcontainers-go` spins a real Postgres; tests run actual SQL. Build tag `//go:build integration`.
5. **Gateway tests** (once built) — `httptest.NewServer` as fake upstream; assert path rewriting, identity header injection, inbound `X-User-*` stripping, 401 on missing token, 429 on rate limit.
6. **E2E smoke** — `docker compose up` → register → login → create product with token → 401 without token.

Tooling: `go test -race -cover`, table-driven tests, `golangci-lint` (errcheck, govet, staticcheck), `govulncheck`.

---

## 15. Security Checklist

- [x] SQL injection — parameterized queries (GORM) + allowlisted ORDER BY columns (`pkg/query`)
- [x] Request body caps + unknown-field rejection + trailing-data rejection
- [x] Rate limiting — Redis-backed, per-service namespaced key, fails open
- [x] CORS — configurable origin list; `Vary: Origin` when not wildcard
- [x] Panic recovery — 500 JSON, stack to logs, cause never to client
- [x] DB credentials from env — never hardcoded
- [x] `.env` gitignored
- [ ] `ReadHeaderTimeout` on `http.Server` — Slowloris defense
- [ ] bcrypt password hashing; `json:"-"` on hash field
- [ ] JWT: pinned alg, type claim, short access TTL (15 min), strong secret (32+ bytes) from env
- [ ] Gateway strips inbound `X-User-*` headers before injecting from verified claims
- [ ] Services unreachable except via gateway (private Docker/K8s network)
- [ ] CORS locked to explicit origin list in production (not `*`)
- [ ] Security headers: `X-Content-Type-Options`, `X-Frame-Options`, HSTS
- [ ] Login endpoint gets a stricter rate limit bucket than general API
- [ ] Audit logging for auth events (login success/failure with `request_id`, never with password)
- [ ] Dependency scanning: `govulncheck`, Dependabot/Renovate
- [ ] TLS termination at gateway or upstream load balancer; HSTS

---

## 16. Build Order (Remaining Work)

Each step leaves the system buildable and runnable — no big-bang rewrites.

1. **`pkg/` additions** — `pkg/token` (JWT sign/verify, access+refresh), `pkg/identity` (read/write `X-User-*` context), `pkg/health` (reusable health handler).
2. **Auth in user-service** — add `password_hash` + `role` columns (migration), `POST /auth/register|login|refresh`, `GET /auth/me`, bcrypt, seeded admin.
3. **Gateway service** — `cmd/gateway/main.go`; routing table; JWT middleware; identity header injection (strip inbound, inject from claims); Docker Compose wiring with private network.
4. **Service-side authorization** — identity middleware, `RequireRole("admin")` on user CRUD, token-gated product writes.
5. **Production Dockerfile** — multi-stage build, distroless final image, `ARG SERVICE`.
6. **K8s manifests** — Deployment/Service/Ingress per service; probe wiring.
7. **Tests** — service unit tests first, then handler tests, then integration tests.
8. **CI/CD** — `go vet` + lint + test + build + `govulncheck` per PR; image build + push on merge.

---

## 17. Topics to Know (Deferred)

| Topic | What/Why | Typical Tool |
|---|---|---|
| Service discovery | Find upstreams dynamically | Consul, K8s DNS |
| Async messaging / events | "UserDeleted" → catalog cleans up; decouples services | Kafka, NATS, RabbitMQ |
| Outbox pattern | Atomically persist + publish events | DB table + relay |
| Saga pattern | Multi-service "transactions" via compensating actions | Choreography / orchestration |
| gRPC for east-west | Typed, fast inter-service calls; REST stays north-south | Protobuf, `buf` |
| API gateway products | Build vs buy | Kong, Traefik, Envoy |
| Centralized auth server | OIDC provider instead of hand-rolled JWT | Keycloak, Auth0 |
| CQRS | Separate read/write models when scale demands | — |
| Feature flags | Runtime config without redeploy | LaunchDarkly, Unleash |
| Service mesh | mTLS, retries, observability out of app code | Istio, Linkerd |

---

## 18. Glossary

- **North-south traffic** — client ↔ gateway. **East-west** — service ↔ service.
- **AuthN vs AuthZ** — who you are (authentication, gateway) vs what you may do (authorization, service).
- **12-factor app** — config in env, stateless processes, logs to stdout, graceful shutdown.
- **Bounded context** — each service owns one business capability and its data.
- **Backing service** — DB/queue/cache treated as an attached resource, swappable via config.
- **Idempotency** — same request twice = same result; required for safe retries.
- **Correlation ID** — one request ID across all hops; already implemented via `X-Request-ID`.
- **Sidecar / service mesh** — infra patterns that move retries, mTLS, and observability out of app code.
- **Database-per-service** — the defining microservice data rule; no shared schema, no cross-service JOINs.
