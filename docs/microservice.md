# Microservice Architecture

## Core Principle: Database Per Service

Each service owns its own Postgres instance. No shared tables, no cross-service joins.

| Service | Database | Port (host) |
|---|---|---|
| user | user_db | 5433 |
| catalog | catalog_db | 5434 |

Services never reach into another service's database. If the user service needs product data, it will call the catalog service's HTTP API (or consume an event — future work).

**Why?**
- Independent deployability: upgrading catalog's schema does not require coordinating with the user service.
- Failure isolation: if catalog's DB goes down, the user service continues serving.
- Clear ownership: the team responsible for catalog owns its schema migrations, indices, and tuning decisions.

**Tradeoff**: No cross-service SQL joins. Aggregation across services requires either service-to-service calls or an event-driven read model. Accepted — this is the fundamental microservice tradeoff.

## Service Structure

Every service is internally a **Modular Monolith** — package-by-feature, not package-by-layer. See [internal-architecture.md](internal-architecture.md) for the full rationale.

```
cmd/api/main.go      entry point
internal/
  bootstrap/         composition root — wires each feature module together
  config/            env-var loading, DB + Redis setup
  <feature>/          one package per feature (e.g. user/, auth/, health/)
                      each owns its own model, repository, service, handler, dto
  router/            route registration + middleware chain
```

This identical top-level shape (`bootstrap/`, `config/`, one package per feature, `router/`) means moving between services has zero context-switch overhead, even though the specific feature modules inside differ per service.

## Service Boundaries

Services are separated by **domain** (bounded context), not by technical layer:
- **user service**: everything about users — registration, profile, lookup
- **catalog service**: everything about products — name, description, price

Each service exposes a REST API over HTTP for clients. Service-to-service calls use:
- **Synchronous**: gRPC — currently `auth` → `user` only, see [grpc.md](grpc.md)
- **Asynchronous**: message queue (NATS / RabbitMQ / Kafka) for events like "user created" — not yet implemented

## Health Endpoints

Every service exposes two health endpoints:

| Path | Purpose | Returns |
|---|---|---|
| `GET /healthz` | Liveness — is the process alive? | Always 200 |
| `GET /readyz` | Readiness — can it serve traffic? | 200 if DB responds, 503 otherwise |

**Why two endpoints?**  
Kubernetes (and other orchestrators) distinguish between:
- **Liveness**: should I restart this pod? (Process alive, but maybe deadlocked) → `/healthz`
- **Readiness**: should I send traffic to this pod? (DB available, migrations done) → `/readyz`

A pod failing readiness is removed from the load balancer rotation without being restarted. A pod failing liveness is killed and restarted.

## API Versioning

All routes are under `/v1/`:
```
GET  /v1/users/
POST /v1/users/
GET  /v1/users/{id}
```

**Why prefix with version?**  
Breaking API changes require a new major version. `/v2/users/` can be added without removing `/v1/users/`. Clients upgrade on their schedule.

**Why not header-based versioning (`Accept: application/vnd.api+json;version=1`)?**  
URL versioning is more explicit, easier to test in a browser, simpler to proxy, and the standard in most public APIs.

## Alternatives Considered

- **Monolithic service** — everything in one binary. Simpler initially. Harder to scale individual domains. Rejected because the explicit goal is microservice patterns.
- **HTTP+JSON between services instead of gRPC** — simpler, no protobuf toolchain. Used for the `auth` → `user` call specifically because it's the highest-frequency internal hop and benefits from a generated, typed contract; see [grpc.md](grpc.md).
- **Shared database** — single Postgres with separate schemas per service. Easier joins, harder to isolate failures and deployments. Rejected.
