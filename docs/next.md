# Next: Large-Scale Roadmap

Step-by-step checklist of everything needed to take this project to production-grade, large-scale Go microservices. Items are ordered by dependency — complete earlier phases before later ones. Check off each item as it is implemented.

---

## Phase 1 — Foundation Hardening

These fix gaps that will cause real pain at any scale. Do these before adding new services.

### Authentication & Authorization
- [ ] JWT access token generation and validation (`golang-jwt/jwt`)
- [ ] Refresh token with rotation (store in Redis with TTL)
- [ ] Auth middleware in `pkg/middleware/auth.go` — extract and verify bearer token, inject claims into context
- [ ] Role-based access control (RBAC) — `admin`, `user` roles on JWT claims
- [ ] Auth service as a separate microservice (issues tokens, validates credentials)
- [ ] Inter-service auth — service-to-service calls use a shared secret or mTLS (not user JWTs)

### Testing
- [ ] Unit tests for all service methods (inject fake repository via interface)
- [ ] Integration tests for repository layer against a real test database (use `testcontainers-go` for ephemeral Postgres)
- [ ] HTTP handler tests using `httptest.NewRecorder` and `httptest.NewServer`
- [ ] Table-driven tests for validation, pagination, filter/sort edge cases
- [ ] CI pipeline runs all tests on every PR (GitHub Actions / GitLab CI)
- [ ] Test coverage gate (minimum 70% enforced in CI)

### Security Headers
- [ ] Add `pkg/middleware/security.go` — set `X-Content-Type-Options`, `X-Frame-Options`, `Strict-Transport-Security`, `Content-Security-Policy` on every response
- [ ] Remove `Server` header from all responses (leaks Go version)
- [ ] Enable `DB_SSLMODE=require` for all non-local environments (document in `.env.example`)

### Input Hardening
- [ ] Per-endpoint body size limits (override the global 1 MiB for file upload endpoints)
- [ ] Request timeout middleware — apply a per-request deadline context (e.g., 8s) to prevent slow handlers from holding connections indefinitely

### Pagination
- [ ] Add cursor-based pagination alongside offset — `?cursor=<opaque_token>` param; cursor encodes last-seen `id` + optional sort field
- [ ] Return `next_cursor` in meta response when more pages exist
- [ ] Keep offset pagination for admin/internal endpoints; default to cursor for user-facing endpoints

---

## Phase 2 — Observability

Must be in place before going to production. Cannot debug a distributed system without these.

### Distributed Tracing (OpenTelemetry)
- [ ] Add `go.opentelemetry.io/otel` to `pkg/`
- [ ] Trace context propagation middleware — read `traceparent` header (W3C format), create root span if absent, inject into request context
- [ ] Instrument GORM queries as child spans (`gorm.io/plugin/opentelemetry`)
- [ ] Instrument outbound HTTP calls with trace headers
- [ ] Export traces to Jaeger (local) / Tempo / Datadog (production)
- [ ] Attach `trace_id` and `span_id` to every structured log line

### Metrics (Prometheus)
- [ ] Add `prometheus/client_golang` to `pkg/`
- [ ] Expose `GET /metrics` endpoint on each service (on a separate internal port, not public)
- [ ] Instrument: request count, request latency histogram, error rate — all labelled by service, method, path, status
- [ ] DB connection pool metrics (open conns, idle conns, wait count)
- [ ] Redis hit/miss counters for rate limiting
- [ ] Grafana dashboard per service (latency p50/p95/p99, error rate, throughput)
- [ ] Alert rules: p99 latency > 500ms, error rate > 1%, DB pool saturation > 80%

### Structured Logging Improvements
- [ ] Add `trace_id` field to every log line automatically (read from context in `logger.FromContext`)
- [ ] Log sampling for high-volume INFO logs in production (e.g., access logs at >1000 req/s)
- [ ] Ship logs to centralized aggregator: Loki + Grafana, Datadog, or ELK stack
- [ ] Add `service_name` and `version` fields to every log line (set at startup from env)

### Health Check Improvements
- [ ] `/readyz` checks both DB and Redis (currently only DB)
- [ ] Add startup probe endpoint `/startupz` — returns 503 until migrations have been confirmed applied (prevents traffic before schema is ready)

---

## Phase 3 — API Gateway ✓

Centralizes cross-cutting concerns so individual services don't each implement them.

- [x] Choose gateway: **Kong DB-less mode** — config in `deploy/kong/kong.yml`, no external database (see [api-gateway.md](api-gateway.md))
- [x] Route all external traffic through the gateway — services use `expose` not `ports`, not directly reachable from host
- [x] Move rate limiting to gateway (Kong `rate-limiting` plugin, per-IP, 100 req/min) — service-level middleware commented out
- [x] Move CORS handling to gateway (Kong `cors` plugin) — service-level middleware commented out
- [x] Correlation ID injection at gateway (Kong `correlation-id` plugin → `X-Request-ID`)
- [x] Add gateway-level auth token verification — Kong `jwt` plugin with RS256; applied per-route (`/v1/users`, `/v1/products`); `/v1/auth` stays public (see [auth.md](auth.md))
- [ ] Load balancing across multiple service instances
- [ ] Circuit breaker at gateway for downstream service failures
- [ ] SSL termination at gateway — services communicate over plain HTTP internally

---

## Phase 4 — gRPC (East-West Communication)

Replace service-to-service HTTP calls with gRPC once multiple services need to talk to each other. Keep REST/HTTP for external (client-facing) APIs; use gRPC only for internal (service-to-service) communication.

### Toolchain & Contracts
- [x] Install protobuf toolchain: `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`, `protoc-gen-validate` — all four installed and in use
- [ ] Create `proto/` directory at monorepo root for shared `.proto` definitions — done differently: lives at `pkg/proto/user/`, inside the shared `pkg` Go module, rather than a separate root-level `proto/` dir
- [x] Define service contracts: `pkg/proto/user/user.proto` — see [grpc.md](grpc.md)
- [x] Declare field validation rules in `.proto` using `protoc-gen-validate` (PGV) — e.g., `(validate.rules).string.email = true`, `.string.min_len = 8`; enforced server-side by `pkg/grpcmiddleware.Validation`
- [x] Generate Go stubs into `pkg/proto/` via `make proto`

### Server & Client
- [x] Implement gRPC server alongside HTTP server — done for `user`, currently the only service that needs one (`:6970`, `expose` only, not `ports`); repeat this per future service that other services need to call
- [x] Implement gRPC client in services that need to call peers — done for `auth` → `user`
- [x] Add gRPC interceptors: request ID propagation (read `X-Request-ID` from metadata), structured logging, panic recovery — all three done (`pkg/grpcmiddleware.RequestID`/`Logger`/`Recovery`; client-side propagation via `pkg/grpcmiddleware.PropagateRequestID`)
- [x] Add gRPC health check protocol (`grpc_health_v1`) for load balancer / k8s probe integration — `user` registers the standard health service, static `SERVING` status set at startup (not wired to live DB health yet)
- [ ] Add gRPC reflection for dev tooling (`grpcurl`, `grpcui`) — confirmed missing; `grpcurl` currently requires passing `-proto` by hand instead of querying the server

### Auth
- [x] **Docker network (current)**: no gRPC auth required — internal ports are not reachable from outside the Docker network; trust the network boundary — implemented with `insecure.NewCredentials()`, decision documented in [grpc.md](grpc.md)
- [ ] **Kubernetes / multi-node**: add mTLS — each service gets a cert, peers verify identity; use cert-manager or SPIFFE/SPIRE; no application-level token logic needed, the sidecar (Envoy / Linkerd) handles it transparently

---

## Phase 5 — Async Messaging

Decouple services from each other for events that don't need an immediate response.

### RabbitMQ (Task Queues & Simple Events)
- [ ] Add RabbitMQ to `docker-compose.yml` (port 5672, management UI 15672)
- [ ] Create `pkg/messaging/rabbitmq/` — connection, channel pool, publisher, consumer
- [ ] Define message envelope struct: `{event_type, payload, trace_id, timestamp, version}`
- [ ] Implement publisher with confirm mode (guarantee delivery to broker)
- [ ] Implement consumer with manual ack — only ack after successful processing
- [ ] Dead-letter queue (DLQ) for messages that fail after N retries
- [ ] Example use case: user service publishes `user.created` → other services consume and react

### Kafka (Event Streaming & Audit Log)
- [ ] Add Kafka to `docker-compose.yml` (use Redpanda for local dev — same protocol, simpler setup)
- [ ] Create `pkg/messaging/kafka/` — producer, consumer group, offset management
- [ ] Use Kafka for: audit log (append-only record of all state changes), event sourcing, cross-service data replication
- [ ] Idempotent consumers — deduplicate messages using event ID stored in Redis/DB
- [ ] Schema registry (Confluent or Redpanda) — enforce message schema with Avro or Protobuf
- [ ] Partition strategy: partition by resource ID (e.g., user ID) to preserve per-resource ordering
- [ ] Kafka Connect or CDC (Debezium) for capturing DB changes as events without modifying application code

### Choosing Between RabbitMQ and Kafka
- RabbitMQ: use for task queues, work distribution, request/reply patterns, when message TTL matters
- Kafka: use for event streams, audit logs, replay, fan-out to many consumers, retention of history
- They are not mutually exclusive — use both for their respective strengths

---

## Phase 6 — Dependency Injection at Scale

- [ ] Adopt **Google Wire** — code-generation DI tool
- [ ] Create `internal/wire/wire.go` per service — declare providers, let Wire generate the wiring
- [ ] Run `wire gen ./...` as part of `make build`
- [ ] Wire eliminates manual bootstrap files; compile-time error if a dependency is missing
- [ ] Switch when any service's bootstrap function exceeds ~15 dependencies

---

## Phase 7 — Database Scaling

### Query Optimisation
- [ ] Add database indexes for all filtered/sorted columns (currently missing — only primary key indexed)
- [ ] `CREATE INDEX idx_users_email ON users(email)` — used by ExistsByEmail and filter queries
- [ ] `CREATE INDEX idx_users_name ON users(name)` — used by partial-match filter
- [ ] Analyse slow queries with `EXPLAIN ANALYSE` in Postgres; add missing indexes
- [ ] Replace `COUNT(*)` on large tables with estimated counts (`pg_class.reltuples`) for list endpoints

### Read Replicas
- [ ] Configure Postgres streaming replication (primary + 1 replica minimum)
- [ ] Add `DB_READ_DSN` env var — use replica for all SELECT queries, primary for writes
- [ ] Update repository to use a `readDB` for reads and `db` for writes
- [ ] pgBouncer connection pooler in front of Postgres to handle many short-lived connections from multiple service instances

### Migrations in Production
- [ ] Run migrations as a Kubernetes Job before the deployment rolls out (init container pattern)
- [ ] Add migration status check to `/startupz` — pod is not ready until its service's migrations are applied
- [ ] Never run migrations at service startup (race condition with multiple pods starting simultaneously)

### Move to sqlc for Complex Queries
- [ ] Install `sqlc` and define `sqlc.yaml` per service
- [ ] Write raw SQL queries in `internal/query/*.sql`
- [ ] Generate type-safe Go scan code via `make sqlc`
- [ ] Replace GORM query building with generated functions for complex queries (joins, CTEs, aggregations)
- [ ] Keep GORM only for simple CRUD or migrate fully

---

## Phase 8 — Kubernetes & Production Deployment

### Containerisation
- [ ] Multi-stage production Dockerfile per service: `golang:alpine` builder → `scratch` or `distroless/static` final image (~10 MB, no shell)
- [ ] Non-root user in final image (`USER nonroot:nonroot`)
- [ ] Image tagging strategy: `<service>:<git-sha>` for deployments, `<service>:latest` for local dev

### Kubernetes Manifests
- [ ] `deploy/k8s/<service>/` — Deployment, Service, HorizontalPodAutoscaler, PodDisruptionBudget
- [ ] ConfigMap for non-secret config, ExternalSecret / Sealed Secrets for credentials
- [ ] Resource requests and limits on every container (CPU, memory)
- [ ] `terminationGracePeriodSeconds: 30` on all pods (Go server timeout is 15s, giving k8s the remaining 15s buffer)
- [ ] Liveness probe → `/healthz`, readiness probe → `/readyz`, startup probe → `/startupz`
- [ ] HPA: scale on CPU utilisation (70% target) and/or custom Prometheus metrics (request queue depth)
- [ ] PodDisruptionBudget: `minAvailable: 1` so rolling deploys never take all pods down simultaneously
- [ ] Network policies: pods can only receive traffic from the API gateway, not directly from the internet

### CI/CD Pipeline
- [ ] GitHub Actions / GitLab CI pipeline: lint → test → build → push image → deploy
- [ ] Path-based CI: only build/test services whose files changed (reduces CI time)
- [ ] Semantic versioning and automated changelog generation
- [ ] Canary deployments — route 5% of traffic to new version, watch error rate, promote or rollback
- [ ] Automated rollback on error rate spike (Argo Rollouts or Flagger)

---

## Phase 9 — Advanced Resilience

### Circuit Breaker & Retry
- [ ] Add `pkg/httpclient/` — shared HTTP client for service-to-service calls (not needed yet — the only inter-service call today is `auth` → `user`, and it's gRPC, not HTTP)
- [ ] Circuit breaker with `sony/gobreaker` or `failsafe-go` — open after N consecutive failures, half-open probe after timeout
- [ ] Exponential backoff with jitter for retries (never retry non-idempotent requests blindly)
- [x] Timeout on every outbound call (context deadline, not just server-level timeouts) — done for the gRPC `auth` → `user` call via `pkg/grpcmiddleware.Timeout` (`USER_GRPC_TIMEOUT`, default `5s`); see [grpc.md](grpc.md)

### Idempotency
- [ ] Idempotency key header (`Idempotency-Key: <uuid>`) on POST/PUT endpoints
- [ ] Store key + response in Redis with TTL — return cached response on duplicate
- [ ] Prevents double-charges, double-creates on network retries

### Caching
- [ ] Cache-aside pattern in service layer for read-heavy resources
- [ ] Redis cache with TTL for individual resource lookups (e.g., `user:<id>`)
- [ ] Cache invalidation on write (delete cache key after successful update/delete)
- [ ] `Cache-Control` headers on GET responses for HTTP-level caching at CDN/proxy

---

## Phase 10 — Developer Experience at Scale

### Code Generation
- [ ] `make proto` — regenerate gRPC stubs from `.proto` files
- [ ] `make sqlc` — regenerate DB query code from `.sql` files
- [ ] `make wire` — regenerate DI wiring
- [ ] `make mock` — regenerate interface mocks for tests (`mockery` or `moq`)

### Service Template
- [ ] Create `services/_template/` — a skeleton new service with all boilerplate in place (model, repo, service, handler, router, bootstrap, config, migrations, Dockerfile, air.toml)
- [ ] `make new-service SVC=payment` — script that copies the template and renames all occurrences

### Documentation
- [ ] OpenAPI spec kept in sync with code (`swaggo/swag` annotations or hand-written YAML)
- [ ] Postman / Bruno collection for all endpoints, checked into the repo
- [ ] Architecture Decision Records (ADRs) in `docs/adr/` — one file per major decision, immutable history
- [ ] `docs/runbook.md` — on-call playbook: how to diagnose common failures, how to run migrations, how to roll back

### Local Development
- [ ] `docker compose --profile tools up` — optional profile for pgadmin, Redpanda console, Jaeger UI, Prometheus, Grafana so they don't run by default
- [ ] Seed data script (`make seed SVC=user`) — populate DB with realistic test data for local dev
- [ ] Local HTTPS via mkcert — test TLS behavior locally without self-signed cert warnings
