# Docker & Containerization

## Strategy: Dev-Only Images

The Dockerfiles do **not** use multi-stage builds. They are development images — the container runs `air` for hot-reload, not a compiled binary:

```dockerfile
FROM golang:1.25-alpine
RUN apk add --no-cache make
RUN go install github.com/air-verse/air@latest
COPY go.work go.work.sum ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/user/go.mod services/user/go.sum ./services/user/
COPY services/docs/go.mod ./services/docs/
RUN go mod download
CMD ["air", "-c", "services/user/.air.toml"]
```

Each service has its own Dockerfile (`Dockerfile.user`, `Dockerfile.auth`, `Dockerfile.docs`, `Dockerfile.notification`) following this same pattern, with the `CMD` pointing at the appropriate `.air.toml`. Every Dockerfile copies *every* workspace module's `go.mod`/`go.sum` (not just its own) — `go mod download` inside a Go workspace needs all of them present to resolve, so adding a new service module means adding one `COPY` line to all the existing Dockerfiles too, or every other service's build breaks. The `docs` service has no external Go dependencies so it has no `go.sum`.

The source code is **not** copied into the image at build time. It is mounted as a volume at runtime via `docker-compose.yml`. This means `docker compose up` reflects code changes immediately (via air) without rebuilding the image.

**Why no multi-stage production build yet?**  
The project is in active development. A production image (Go builder stage → distroless/scratch final stage) will be added before deploying. For now, the dev image keeps the workflow simple: one `docker compose up` gets everything running.

## Docker Compose Services

```
auth_app                — auth service (exposed to Docker network only)
user_app                — user service (exposed to Docker network only)
user_postgres           — postgres:17-alpine (port 5433 on host)
notification_app        — notification service (exposed to Docker network only)
notification_postgres   — postgres:17-alpine (port 5435 on host)
docs_app                — API docs service / Swagger UI (exposed to Docker network only)
redis                   — redis:7-alpine (port 6380 on host)
rabbitmq                — rabbitmq:3.13-management-alpine (port 5672 broker, 15672 management UI on host)
mailpit                 — local-dev SMTP catcher for notification_app (port 8025 web UI on host)
kong                    — Kong gateway (port 8000 proxy, port 8002→8001 admin on host)
pgadmin                 — pgadmin4 (port 5050 on host)
```

### Port Mapping Choices

Standard ports (5432, 6379) are deliberately avoided on the host side:

| Service | Container port | Host port | Why offset |
|---|---|---|---|
| user_postgres | 5432 | 5433 | Avoids collision with any local Postgres |
| notification_postgres | 5432 | 5435 | Avoids collision with any local Postgres |
| redis | 6379 | 6380 | Avoids collision with any local Redis |

A developer can run `docker compose up` even if they have a local Postgres or Redis already running on the standard ports.

### Database Per Service

Each service that needs Postgres gets its own container with its own volume (`user_postgres_data`, `notification_postgres_data`). Services never share a container or a volume. This mirrors the production architecture (database-per-service) so local dev matches prod behavior.

### Redis

Currently only `auth_app` connects to Redis, for refresh-token storage (`refresh:<token>` keys). The container is provisioned as shared infrastructure so any other service that needs Redis (rate limiting, caching) can connect to the same instance without adding a new container; keys would be namespaced per service to avoid collisions.

### RabbitMQ & Mailpit

`rabbitmq` is shared infrastructure the same way `redis` is: `user_app` publishes to it, `notification_app` consumes from it — see [messaging.md](messaging.md) for the topology. Its username is deliberately not `guest` (RabbitMQ hardcodes that user to loopback-only connections, which would block every other container).

`mailpit` is a local-dev-only SMTP catcher — `notification_app` sends real SMTP traffic to it instead of a live provider, viewable at its web UI (`http://localhost:8025`). It has no role in production; `NOTIFICATION_SMTP_*` env vars point at a real provider there instead, with no code change — see [email.md](email.md).

### Health Checks

Every infrastructure container has a health check:
- Postgres: `pg_isready`
- Redis: `redis-cli ping`

App containers use `depends_on: condition: service_healthy` so the service process doesn't start until its database is accepting connections. This eliminates a class of startup-order race conditions that would otherwise require retry loops in `main.go`.

### pgAdmin

pgAdmin runs at port 5050 for database inspection during development. It's a convenience tool — not part of the application stack. Credentials come from `PGADMIN_EMAIL` and `PGADMIN_PASSWORD` env vars.

## Why Alpine Base Images?

`golang:1.25-alpine` and `postgres:17-alpine` use Alpine Linux (~5 MB) rather than Debian/Ubuntu (~100 MB). Smaller images:
- Faster to pull in CI and on developer machines
- Smaller attack surface (fewer packages)

The tradeoff: Alpine uses musl libc instead of glibc. This is irrelevant for Go (static binaries) and Postgres (Alpine-specific packages are well-tested upstream).

## Alternatives Considered

- **Multi-stage production Dockerfile now** — build Go binary in builder stage, copy to scratch/distroless. Produces a ~10 MB final image with zero shell or OS. Deferred until deployment is needed; adds complexity before it provides value.
- **Docker Compose with pre-built binaries** — `make build` + copy binary into container, no air. Eliminates hot-reload. Not suitable for inner-loop development.
- **Dev containers (VS Code devcontainer.json)** — entire development environment in a container. Useful for teams with heterogeneous setups. Added complexity; deferred.
- **Podman** — Docker-compatible, rootless. Drop-in replacement; no reason to switch yet.
