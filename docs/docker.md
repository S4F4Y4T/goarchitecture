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
COPY services/catalog/go.mod services/catalog/go.sum ./services/catalog/
COPY services/docs/go.mod ./services/docs/
RUN go mod download
CMD ["air", "-c", "services/user/.air.toml"]
```

Each service has its own Dockerfile (`Dockerfile.user`, `Dockerfile.catalog`, `Dockerfile.docs`) following this same pattern, with the `CMD` pointing at the appropriate `.air.toml`. The `docs` service has no external Go dependencies so it has no `go.sum`.

The source code is **not** copied into the image at build time. It is mounted as a volume at runtime via `docker-compose.yml`. This means `docker compose up` reflects code changes immediately (via air) without rebuilding the image.

**Why no multi-stage production build yet?**  
The project is in active development. A production image (Go builder stage → distroless/scratch final stage) will be added before deploying. For now, the dev image keeps the workflow simple: one `docker compose up` gets everything running.

## Docker Compose Services

```
user_app         — user service (exposed to Docker network only)
user_postgres    — postgres:17-alpine (port 5433 on host)
catalog_app      — catalog service (exposed to Docker network only)
catalog_postgres — postgres:17-alpine (port 5434 on host)
docs_app         — API docs service / Swagger UI (exposed to Docker network only)
redis            — redis:7-alpine (port 6380 on host)
kong             — Kong gateway (port 8100→8000 proxy, port 8101→8001 admin on host)
pgadmin          — pgadmin4 (port 5050 on host)
```

### Port Mapping Choices

Standard ports (5432, 6379) are deliberately avoided on the host side:

| Service | Container port | Host port | Why offset |
|---|---|---|---|
| user_postgres | 5432 | 5433 | Avoids collision with any local Postgres |
| catalog_postgres | 5432 | 5434 | Separate host port per service DB |
| redis | 6379 | 6380 | Avoids collision with any local Redis |

A developer can run `docker compose up` even if they have a local Postgres or Redis already running on the standard ports.

### Database Per Service

Each service gets its own Postgres container with its own volume (`user_postgres_data`, `catalog_postgres_data`). They never share a container or a volume. This mirrors the production architecture (database-per-service) so local dev matches prod behavior.

### Shared Redis

Both services connect to the same Redis container. Rate-limit counters are namespaced per service (`rl:user:<ip>` vs `rl:catalog:<ip>`) so there's no collision. A single Redis is simpler to run locally; in production each service can have its own Redis cluster.

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
