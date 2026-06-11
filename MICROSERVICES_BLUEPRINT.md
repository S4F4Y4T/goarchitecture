# Enterprise Go Microservices — Complete Blueprint

A topic-by-topic guide for converting this codebase (currently a layered monolith with Users + Products) into a **production-grade microservice system** with an **API Gateway** and **JWT authentication**. Read top to bottom: each section is a concept you must know, what the enterprise best practice is, and how it applies to *this* repo.

---

## 1. Target Architecture (Big Picture)

```
                                ┌──────────────────────────────┐
                                │          Clients             │
                                │  (web / mobile / curl)       │
                                └──────────────┬───────────────┘
                                               │  HTTPS :8080
                                               ▼
                            ┌──────────────────────────────────────┐
                            │             API GATEWAY              │
                            │  - single public entry point         │
                            │  - JWT verification (authn)          │
                            │  - rate limiting (per client IP)     │
                            │  - CORS, security headers            │
                            │  - request ID generation/propagation │
                            │  - reverse proxy + identity headers  │
                            │    (X-User-ID, X-User-Email, X-Role) │
                            └───────────┬──────────────┬───────────┘
                       /api/v1/auth/*   │              │   /api/v1/products/*
                       /api/v1/users/*  │              │
                                        ▼              ▼
                     ┌──────────────────────┐   ┌──────────────────────┐
                     │     USER SERVICE     │   │   PRODUCT SERVICE    │
                     │        :8081         │   │        :8082         │
                     │  - register/login    │   │  - product CRUD      │
                     │  - issues JWTs       │   │  - authorization via │
                     │  - bcrypt passwords  │   │    identity headers  │
                     │  - user CRUD (admin) │   │                      │
                     └──────────┬───────────┘   └──────────┬───────────┘
                                │                          │
                                ▼                          ▼
                     ┌──────────────────────┐   ┌──────────────────────┐
                     │      user-db         │   │     product-db       │
                     │   (Postgres :5432)   │   │   (Postgres :5432)   │
                     └──────────────────────┘   └──────────────────────┘

        Key rule: services are NEVER exposed publicly. Only the gateway
        publishes a port. Services live on a private Docker network.
```

**Topics this diagram encodes:**
- **API Gateway pattern** — one public entry point; cross-cutting concerns (auth, rate limit, CORS) live once, not in every service.
- **Database-per-service** — user-db and product-db are separate Postgres instances. Services never read each other's tables; they talk via APIs. This is THE defining microservice data rule.
- **Identity propagation** — gateway verifies the JWT once, then forwards *trusted headers* (`X-User-ID`, `X-User-Email`, `X-User-Role`) to services over the private network. Services don't re-parse JWTs.
- **Network segmentation** — only the gateway is on the public network; everything else is internal.

---

## 2. Repository Layout (Monorepo, Multi-Binary)

Enterprise Go shops use one of two layouts. **Recommended here: single module, multiple binaries** (simplest, still production-real — used at Google-scale monorepos):

```
microservice/
├── cmd/                          # one main package per deployable binary
│   ├── gateway/main.go
│   ├── user-service/main.go
│   └── product-service/main.go
├── internal/                     # ← rename from "internals"! Only the literal
│   │                             #   name "internal" gets compiler-enforced privacy
│   ├── gateway/
│   │   ├── config/               # gateway-specific env config
│   │   ├── middleware/           # auth.go (JWT verify), ratelimit.go
│   │   ├── proxy/                # reverse proxy w/ identity injection
│   │   └── router/
│   ├── user/
│   │   ├── bootstrap/            # dependency wiring (manual DI)
│   │   ├── config/
│   │   ├── dto/                  # request/response shapes + validation tags
│   │   ├── handler/              # HTTP layer (auth.go, user.go, health)
│   │   ├── model/                # domain entities + repository interfaces
│   │   ├── repository/           # GORM data access
│   │   ├── router/
│   │   └── service/              # business logic (auth.go: bcrypt+JWT, user.go)
│   └── product/
│       └── (same shape as user/)
├── pkg/                          # shared kit — importable by all services
│   ├── apperror/                 # ← rename from "appError" (Go: lowercase pkg names)
│   ├── config/                   # env helpers, DB config + pool, gorm setup
│   ├── identity/                 # identity ctx: read/write X-User-* headers
│   ├── logger/                   # slog JSON logger + context helpers (exists ✅)
│   ├── middleware/               # ← move from internals/middleweare (typo!)
│   ├── pagination/               # (exists ✅)
│   ├── query/  + query/gorm/     # allowlisted filter/sort (exists ✅)
│   ├── request/                  # safe JSON decoding (exists ✅)
│   ├── response/                 # response envelope (exists ✅)
│   ├── token/                    # JWT manager: sign/verify, access+refresh
│   └── validation/               # validator wrapper (exists ✅)
├── db/migrations/
│   ├── user/                     # 0001_create_users, 0002_seed_admin
│   └── product/                  # 0001_create_products
├── docs/openapi.yaml             # gateway-level API contract
├── deploy/  (or root)            # docker-compose.yml, .air/ configs
├── Dockerfile                    # multi-stage prod build, ARG SERVICE
├── Dockerfile.dev                # air hot-reload image
├── makefile
└── README.md
```

**Topics:**
- **`cmd/` pattern** — every deployable artifact gets `cmd/<name>/main.go`; main stays tiny (wire config → DB → router → server).
- **`internal/` enforcement** — the Go compiler forbids importing `internal/...` from outside the module. Your current `internals/` gets **no** protection — the rename is a real fix, not cosmetics.
- **Service boundaries inside a monorepo** — `internal/user` must never import `internal/product`. Shared code goes through `pkg/` only. (Lint rule: depguard / import-boundary checks.)
- **`pkg/` shared kit** — your `logger`, `response`, `apperror`, `request`, `validation`, `pagination`, `query` are already this. They stay ORM-free and HTTP-generic where possible (your `pkg/query` doc comment already states this — that's the right instinct).
- **Alternative layout** (know it for interviews): one Go module per service + `go.work` workspace. True independent versioning/deployability, but more toil (`replace` directives, `go work sync`). Choose it when teams own services separately.

---

## 3. API Gateway (Concepts & Responsibilities)

The gateway is a thin reverse proxy — **no business logic, ever**.

| Concern | Best practice |
|---|---|
| Routing | Declarative route table: path prefix → upstream service URL |
| Reverse proxy | `net/http/httputil.ReverseProxy` with the modern `Rewrite` hook (not the legacy `Director`) |
| Path mapping | Public `/api/v1/products/...` → strip `/api` → service sees `/v1/products/...` |
| AuthN | Verify `Authorization: Bearer <jwt>` once; reject 401 before traffic reaches services |
| Identity injection | **Delete inbound** `X-User-*` headers (spoofing defense!), then set them from verified claims |
| Public routes | Allowlist: `POST /auth/register`, `POST /auth/login`, `POST /auth/refresh`, `GET /products*`, health endpoints |
| Rate limiting | Token bucket per client IP (`golang.org/x/time/rate`), in-memory map + cleanup goroutine; Redis-backed in real multi-instance prod |
| Resilience | Upstream timeouts on the proxy `Transport`; `ErrorHandler` returns JSON 502/504 instead of hanging |
| Health | `/healthz` (self), `/readyz` (pings each upstream's health endpoint) |
| Headers | `X-Forwarded-For`, `X-Forwarded-Proto`, `X-Request-ID` propagation (your RequestID middleware already reuses inbound IDs — exactly for this) |

**Routing table for this app:**

```
PUBLIC    POST /api/v1/auth/register   → user-service
PUBLIC    POST /api/v1/auth/login      → user-service
PUBLIC    POST /api/v1/auth/refresh    → user-service
AUTH      GET  /api/v1/auth/me         → user-service
AUTH+ADMIN     /api/v1/users/**        → user-service   (CRUD)
PUBLIC    GET  /api/v1/products/**     → product-service (list/detail)
AUTH      POST/PUT/DELETE /api/v1/products/** → product-service
```

Go 1.22+ `http.ServeMux` method+wildcard patterns (`"POST /api/v1/auth/login"`, `"GET /api/v1/products/"`) handle this without a router dependency — consistent with the stdlib-first style you already use.

---

## 4. JWT Authentication (Full Topic List)

### 4.1 Token design
- **Access token**: short-lived (15 min), HS256-signed, carried on every request.
- **Refresh token**: long-lived (7 days), used only on `POST /auth/refresh` to mint a new pair. Distinguish with a `token_type` claim ("access" / "refresh") — verifying the type prevents a refresh token being used as an access token.
- **Claims**: registered (`sub`=user ID, `iss`, `exp`, `iat`, `jti`) + custom (`email`, `role`). Keep claims small — they ride on every request.
- **Library**: `github.com/golang-jwt/jwt/v5` (the maintained fork). Always pin the expected signing method when parsing (`jwt.WithValidMethods(["HS256"])`) — the classic `alg: none` attack.

### 4.2 Signing algorithm choice
- **HS256 (symmetric)** — one shared secret between user-service (signs) and gateway (verifies). Simple; fine when both sides are yours. Secret = 32+ random bytes from env/secret manager, never committed.
- **RS256/EdDSA (asymmetric)** — user-service holds the private key; gateway and any other service verify with the public key (distributed via JWKS endpoint). **The enterprise upgrade path** — verifiers can't mint tokens. Know why this matters.

### 4.3 Password handling
- `golang.org/x/crypto/bcrypt` (or argon2id) — never SHA/MD5, never reversible.
- Store only `password_hash`; struct tag `json:"-"` so it can never serialize out.
- Login failures return one generic message ("invalid credentials") for wrong-email AND wrong-password — no user enumeration. Same for register conflict timing if you're thorough.

### 4.4 Auth endpoints (user-service)
```
POST /v1/auth/register  {name, email, password}      → 201 + user (no token, or auto-login)
POST /v1/auth/login     {email, password}            → 200 + {access_token, refresh_token, expires_in, token_type:"Bearer"}
POST /v1/auth/refresh   {refresh_token}              → 200 + new token pair (rotate refresh)
GET  /v1/auth/me        (via gateway identity)       → 200 + current user
```

### 4.5 Authorization (RBAC)
- `role` column on users (`user` | `admin`); seeded admin via migration (documented dev-only credentials).
- Gateway does **authentication**; services do **authorization** (e.g., `RequireRole("admin")` middleware on `/v1/users`). Rationale: the resource owner knows its rules; the gateway shouldn't accumulate business policy.
- Ownership checks live in the service layer (e.g., later: "users can update only their own products" via `created_by` column vs. `X-User-ID`).

### 4.6 Identity propagation (the microservice-specific part)
- Gateway → services: `X-User-ID`, `X-User-Email`, `X-User-Role` headers.
- Services wrap them into a context value via a small `pkg/identity` middleware: `identity.FromContext(ctx) → (Identity, bool)`.
- **Threat model**: these headers are trusted only because services are unreachable except through the gateway (private network). The gateway must strip any client-supplied `X-User-*` headers. Document this assumption loudly; in zero-trust setups you'd forward the JWT itself or use mTLS between services.

### 4.7 Known tradeoffs to be able to discuss
- Stateless JWT logout problem → token denylist (Redis) or short TTL + refresh rotation.
- Refresh token storage on clients (httpOnly cookie vs body) and rotation/reuse detection.
- Clock skew (`jwt.WithLeeway`).

---

## 5. Per-Service Anatomy (the layered architecture you already have)

Keep your existing layering in each service — it's correct:

```
router → middleware → handler → service → repository → DB
              ↑           ↑         ↑          ↑
           pkg/midw    dto+valid  business   GORM + apperror
                                   rules      mapping
```

- **Handler**: HTTP only — decode (`pkg/request`), validate (`pkg/validation`), call service, write envelope (`pkg/response`). No business rules. ✅ you already do this.
- **Service**: business rules (uniqueness checks, password hashing, token issuing). Depends on **repository interfaces** declared in `model/` — this is dependency inversion, and it's what makes services unit-testable with fakes. ✅ exists.
- **Repository**: GORM; maps DB errors (e.g., SQLSTATE 23505 → Conflict) to `apperror`. ✅ exists — your `isUniqueViolation` is exactly the right pattern.
- **Bootstrap**: manual constructor wiring (repo → service → handler). Manual DI > frameworks at this scale; know `google/wire` exists for bigger graphs.
- **Models per service**: User gains `PasswordHash string \`json:"-"\``, `Role`, `CreatedAt`, `UpdatedAt`. Product gains timestamps (DB already has them) and later `created_by`.

**What changes in the split**: `internals/{handler,service,repository,model,dto}` duplicates into `internal/user/...` and `internal/product/...`. Each binary compiles only its own slice. Duplication between services is *accepted* in microservices — shared business code creates coupling; only generic infrastructure goes in `pkg/`.

---

## 6. Configuration Management

- **12-factor**: all config from env vars; `.env` only for local dev (`godotenv.Load` must *tolerate* a missing file — current code `os.Exit(1)`s, which breaks containers where env comes from the orchestrator. Fix this.)
- Fail fast on missing required vars at startup, with a list of everything missing at once (✅ your `loadDBConfig` does this — keep that pattern).
- Typed config structs per service: `Port`, `DB DBConfig`, plus user-service: `JWTSecret`, `JWTIssuer`, `AccessTTL`, `RefreshTTL`; gateway: `UserServiceURL`, `ProductServiceURL`, `RateLimitRPS`, `RateLimitBurst`.
- Shared helpers (`getEnvInt`, `getEnvDuration`, required-var collection) move to `pkg/config` (✅ already written, just relocate).
- Validate values, not just presence (e.g., JWT secret minimum length 32 bytes).
- Secrets: env in dev/compose; secret manager (Vault/AWS SM/K8s secrets) in prod — never in the image, never in git.

---

## 7. Database Topics

- **Database-per-service** — two Postgres containers (`user-db`, `product-db`), separate credentials, separate volumes. No cross-service JOINs ever; need product+owner data together? Call the other service's API or duplicate the needed fields (data denormalization across services is normal).
- **Migrations per service** — `db/migrations/user/`, `db/migrations/product/` with `golang-migrate` (versioned `NNNN_name.up.sql` / `.down.sql` — ✅ tool already in your makefile). Run as a one-shot job before the service starts (compose: `migrate/migrate` image + `depends_on: condition: service_completed_successfully`). Never `AutoMigrate` in prod.
- **Connection pooling** — `SetMaxOpenConns/MaxIdleConns/ConnMaxLifetime/ConnMaxIdleTime` (✅ done, with env tuning — keep).
- **Startup ping with timeout** to fail fast (✅ done).
- **Context everywhere** — `WithContext(ctx)` on every query so cancellation/timeouts propagate (✅ done).
- Distributed-data theory to know (even if not implemented here): saga pattern for cross-service transactions, eventual consistency, outbox pattern for reliable events. There is **no distributed transaction** between user-db and product-db — design so you don't need one.

---

## 8. Cross-Cutting Middleware (shared `pkg/middleware`)

Order matters — outermost first:

```
RequestID → Logger → Recovery → CORS → SecurityHeaders → (gateway only: RateLimit → Auth) → handler
```

- **RequestID** — reuse inbound `X-Request-ID` else generate UUID; echo on response; put in context. ✅ exists; in the gateway it becomes the *correlation ID across services* (gateway generates, services reuse — your comment already anticipates this).
- **Structured access log** — slog JSON, status + latency captured via `statusRecorder`. ✅ exists.
- **Panic recovery** — 500 JSON + stack to logs. ✅ exists.
- **CORS** — current `*` wildcard is dev-only; production: explicit origin allowlist from config, `Vary: Origin`, proper preflight (204, `Access-Control-Max-Age`). CORS belongs **only at the gateway** in the new world.
- **Security headers** — `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, HSTS (behind TLS).
- **Timeouts as middleware vs server** — keep server-level `ReadTimeout/WriteTimeout/IdleTimeout/MaxHeaderBytes` (✅ exists); add per-request `http.TimeoutHandler` or context deadline if handlers can be slow.
- **Body size limit** — ✅ `pkg/request.MaxBodyBytes` already does this; also `DisallowUnknownFields` + trailing-data rejection are genuinely above-average practice. Keep.
- Fix the package name typo: `middleweare` → `middleware`. Drop the throwaway `test.go`.

---

## 9. Error Handling Contract

✅ Mostly done — these are the topics it demonstrates; verify you can explain each:
- Central error type (`apperror.AppError`) with machine code, human message, optional field errors, wrapped cause (`Unwrap`), HTTP status mapping.
- Errors normalized at ONE place (`response.Error` → `apperror.From`) — handlers never `w.WriteHeader` for errors directly.
- Internal causes logged server-side, generic message to client (no DB error leakage). ✅
- Add codes for the auth world: you already have `UNAUTHORIZED`/`FORBIDDEN`; add `TOO_MANY_REQUESTS` (429) for the gateway rate limiter and `BAD_GATEWAY`/`UPSTREAM_TIMEOUT` (502/504) for proxy failures.
- Layering nit: `apperror.From` currently imports GORM to map `ErrRecordNotFound`. Since repositories already map it, remove the GORM import — keeps `pkg/apperror` dependency-free (gateway shouldn't pull GORM).
- Consistent envelope (`success`, `data`, `meta`, `error{code,message,fields}`) across ALL services and the gateway — clients see one API. ✅ `pkg/response`.

---

## 10. Observability

- **Structured logging** — slog JSON to stdout (✅), level from env (✅), request-scoped logger carrying `request_id` (✅). Add `service` attribute (`logger.Init` with `slog.With("service","user-service")`) so aggregated logs are filterable per service — essential once there are 3 binaries.
- **Log correlation across services** — same `X-Request-ID` flows gateway → service; searching one ID reconstructs the whole request path. (This is "poor-man's tracing" and a required interview topic.)
- **Health endpoints** — `/healthz` liveness (no deps), `/readyz` readiness (DB ping w/ 2s timeout, 503 when down). ✅ exists; extract to `pkg/health`, gateway's readyz fans out to upstreams.
- **Metrics (topic to know)** — Prometheus `/metrics`: request count/latency histograms per route+status, DB pool gauges, rate-limit rejections. `promhttp` + middleware.
- **Distributed tracing (topic to know)** — OpenTelemetry: gateway starts a span, `traceparent` header propagates, services add child spans; export to Jaeger/Tempo. The structural prep (context propagation everywhere) is already in place.

---

## 11. Resilience Patterns (gateway ↔ services)

- **Timeouts everywhere** — proxy `Transport` with `ResponseHeaderTimeout`; never wait forever on an upstream.
- **Graceful shutdown** — signal → stop accepting → drain in-flight → close. ✅ exists; raise the 1s shutdown timeout to 10–15s (1s will cut off in-flight requests; orchestrators allow ~30s).
- **Rate limiting** — token bucket per IP at the edge; 429 + `Retry-After`.
- Topics to know, even if not built day one: **circuit breaker** (sony/gobreaker) so a dead product-service fails fast instead of piling up goroutines; **retries with backoff + jitter** for idempotent GETs only; **bulkheading** (per-upstream connection caps); **load shedding**.

---

## 12. API Design & Versioning

- URL versioning `/v1/` so `/v2/` can ship breaking changes. ✅ exists; gateway adds the `/api` public prefix (`/api/v1/...` outside, `/v1/...` inside).
- REST conventions you already follow: nouns, plural resources, 201 on create, 204 on delete, PUT = full replace (✅ documented in your DTOs).
- Pagination envelope (`meta: {page, limit, total, total_pages}`) ✅; allowlisted sort/filter (`?sort=-price&filter[name]=x`) ✅ — your `query.Schema` approach (columns only from allowlist → SQL-injection-safe ORDER BY) is genuinely production-grade; extend a `ProductListSchema` the same way.
- **OpenAPI as the contract** — one gateway-level `openapi.yaml` covering auth + users + products with `bearerAuth` security scheme; Swagger UI served by the gateway only. Services may keep internal specs.
- Idempotency topic: PUT/DELETE idempotent by nature; know the `Idempotency-Key` pattern for POSTs.

---

## 13. Containerization & Local Orchestration

- **Production Dockerfile** — multi-stage: `golang:1.x` build stage (cache `go mod download` layer) → `CGO_ENABLED=0 go build ./cmd/$SERVICE` → final stage `gcr.io/distroless/static` or `scratch`, non-root user, single static binary. One Dockerfile, `ARG SERVICE=gateway|user-service|product-service`.
- **Dev hot-reload** — your air + volume-mount setup ✅; becomes three air configs (`.air/gateway.toml`, etc.) since each container builds a different `cmd/`.
- **docker-compose topology**:
  - `gateway` — the ONLY service with `ports:` published (8080).
  - `user-service`, `product-service` — `expose` only, on an internal network.
  - `user-db`, `product-db` — separate containers, separate volumes, healthchecks (✅ you already use `pg_isready` healthcheck + `depends_on: service_healthy` — extend to both DBs).
  - `user-migrate`, `product-migrate` — one-shot `migrate/migrate` jobs; services depend on `service_completed_successfully`.
  - Two networks: `edge` (gateway) and `backend` (everything); DBs only on `backend`.
- Container hygiene topics: `.dockerignore`, image scanning, pinned base image tags, read-only root FS, resource limits.
- **Beyond compose (know the names)**: Kubernetes Deployment/Service/Ingress, liveness/readiness probes wired to your `/healthz`+`/readyz`, HorizontalPodAutoscaler, ConfigMap/Secret.

---

## 14. Testing Strategy (enterprise expectations)

- **Unit tests, service layer** — fake the repository interface (this is why `model.UserRepository` is an interface ✅): register-with-existing-email → Conflict; login-wrong-password → Unauthorized; token type confusion → rejected.
- **Unit tests, pure pkg code** — `pkg/token` (sign → verify roundtrip, expired, wrong secret, wrong type), `pkg/query.Parse` (unknown fields dropped), `pkg/pagination` (clamping).
- **Handler tests** — `net/http/httptest` against the router: status codes, envelope shape, validation errors.
- **Integration tests** — testcontainers-go spins a real Postgres; run repository tests against it; build tag `//go:build integration`.
- **E2E smoke** — compose up → register → login → create product with token → 401 without token.
- **Gateway tests** — `httptest.NewServer` as a fake upstream; assert path rewrite, identity header injection, inbound `X-User-*` stripping, 401/429 behavior.
- Tooling: `go test -race -cover`, table-driven tests (the Go idiom), `golangci-lint` (errcheck, govet, staticcheck, depguard for import boundaries), `govulncheck`.

---

## 15. Security Checklist (beyond JWT)

- [x] SQL injection: parameterized queries (GORM) + allowlisted ORDER BY columns (`pkg/query`) ✅
- [x] Request body caps + unknown-field rejection ✅
- [ ] bcrypt password hashing, `json:"-"` on hash
- [ ] JWT: pinned alg, type claim, short access TTL, strong secret from env
- [ ] Gateway strips client-supplied identity headers (spoofing)
- [ ] Services unreachable except via gateway (network isolation)
- [ ] CORS allowlist (not `*`) in prod; security headers
- [ ] Rate limiting at the edge; login endpoint gets a stricter bucket (brute-force defense)
- [ ] No secrets in git/images; `.env` gitignored (✅ verify)
- [ ] TLS termination (at gateway or upstream LB); HSTS
- [ ] Audit logging for auth events (login success/failure with request_id, never with password)
- [ ] Dependency scanning: `govulncheck`, dependabot/renovate

---

## 16. CI/CD Pipeline (topics)

1. **CI per PR**: `go vet` → `golangci-lint` → `go test -race -cover` → `go build ./...` → `govulncheck`.
2. **Build**: docker build per service (matrix over `SERVICE` arg), tag with git SHA, push to registry.
3. **Deploy**: migrations job first, then rolling deploy per service (this is the microservice payoff — deploy product-service without touching user-service).
4. Topics to know: semantic versioning per service, canary/blue-green, rollback strategy, environment promotion (dev → staging → prod), GitOps (ArgoCD).

---

## 17. Topics Deliberately Deferred (know they exist, add later)

| Topic | What/why | Typical tool |
|---|---|---|
| Service discovery | Find upstreams dynamically instead of env URLs | Consul, K8s DNS |
| Async messaging / events | "UserDeleted" event → product-service cleans up; decouples services | Kafka, NATS, RabbitMQ |
| Outbox pattern | Atomically persist + publish events | DB table + relay |
| Saga pattern | Multi-service "transactions" via compensating actions | choreography/orchestration |
| gRPC for east-west traffic | Typed, fast service-to-service calls; REST stays north-south | protobuf, buf |
| API gateway products | Build vs buy | Kong, Traefik, Envoy |
| Centralized auth server | OIDC provider instead of hand-rolled | Keycloak, Auth0 |
| Config service / feature flags | Runtime config | Consul, LaunchDarkly |
| Caching layer | Hot reads, token denylist | Redis |
| CQRS | Separate read/write models when scale demands | — |

---

## 18. Suggested Build Order (when you do implement)

1. **Renames/moves** — `internals` → `internal`, `middleweare` → `pkg/middleware`, `appError` → `pkg/apperror`; everything still one binary; build passes.
2. **Shared kit additions** — `pkg/token` (JWT), `pkg/identity`, `pkg/config` (move config helpers + gorm setup), `pkg/health`.
3. **Split binaries** — `cmd/user-service`, `cmd/product-service` with their `internal/<svc>` trees and per-service migrations; two DBs in compose; both run standalone.
4. **Auth in user-service** — users table migration (password_hash, role, timestamps), register/login/refresh/me, bcrypt, token issuing, seeded admin.
5. **Gateway** — proxy + routing table; then auth middleware + identity injection; then rate limit, CORS, readyz fan-out.
6. **Service-side authorization** — identity middleware, `RequireRole("admin")` on users CRUD, auth on product writes.
7. **Compose/Docker/Make polish** — networks, migrate jobs, prod Dockerfile, make targets per service.
8. **Tests + OpenAPI + README** — lock the contract in.

Each step leaves the system buildable and runnable — never a big-bang rewrite.

---

## 19. Glossary Cheat-Sheet (say these correctly in interviews)

- **North-south traffic**: client ↔ gateway. **East-west**: service ↔ service.
- **AuthN vs AuthZ**: who you are (gateway) vs what you may do (service).
- **12-factor app**: config in env, stateless processes, logs to stdout, disposability (graceful shutdown).
- **Bounded context**: each service owns one business capability + its data.
- **Backing service**: DB/queue treated as attached resource, swappable via config.
- **Sidecar/ambassador/service mesh**: infra patterns that move retries/mTLS/observability out of app code (Istio, Linkerd).
- **Idempotency**: same request twice = same result; required for safe retries.
- **Correlation ID**: one request ID across all hops (you already built this).
