# API Docs (Swagger UI)

## What It Is

A dedicated Go microservice (`services/docs`) that serves a single Swagger UI covering all API routes. It is not part of any business service — it is a cross-cutting concern with its own container, its own `go.mod`, and its own lifecycle.

All "Try it out" calls in the UI go through Kong at `http://localhost:8100`, not directly to any service.

## Access

```
http://localhost:8100/docs
```

Kong proxies the `/docs` path to `docs_app:9090` with `strip_path: true`. The docs service serves from `/`, so the prefix is stripped before forwarding.

## Why a Separate Service

Serving docs from a business service (e.g., user service) creates two problems:

1. **Wrong failure domain** — if the user service is down, docs go down too, even though docs are unrelated to user auth.
2. **Wrong responsibility** — a service that owns user accounts should not also own the deployment of API documentation for other services.

A dedicated service means docs can be deployed, restarted, or updated independently of any business service.

## File Layout

```
services/docs/
├── cmd/api/main.go        # entry point — reads PORT, serves embedded files
├── docs.go                # embeds the static/ directory via go:embed
├── static/
│   ├── index.html         # Swagger UI (CDN-loaded)
│   └── openapi.yaml       # combined OpenAPI 3.0 spec for all services
├── .air.toml              # hot-reload config (watches .go, .yaml, .html)
└── go.mod                 # own module, no external dependencies
```

## How Serving Works

`docs.go` embeds the `static/` directory into the binary at compile time:

```go
//go:embed static
var FS embed.FS
```

`main.go` creates a sub-filesystem rooted at `static/` and hands it to `http.FileServer`:

```go
sub, _ := fs.Sub(docs.FS, "static")
http.Handle("/", http.FileServer(http.FS(sub)))
```

This means the compiled binary carries the HTML and YAML inside it — no runtime filesystem access needed. In development, air rebuilds when any `.go`, `.yaml`, or `.html` file changes.

## OpenAPI Spec

`services/docs/static/openapi.yaml` is the single source of truth for the public API. It covers all services in one file:

| Tag | Paths |
|---|---|
| `health` | `/healthz`, `/readyz` |
| `auth` | `/v1/auth/register`, `/v1/auth/login`, `/v1/auth/refresh`, `/v1/auth/logout` |
| `users` | `/v1/users/me`, `/v1/users/{id}`, ... |
| `products` | `/v1/products`, `/v1/products/{id}`, ... |

The `servers` block points to Kong, not to any individual service:

```yaml
servers:
  - url: http://localhost:8100
    description: API Gateway
```

This is why "Try it out" requests in Swagger pass through Kong and get JWT verification, CORS handling, and rate limiting applied — exactly the same path as a real client.

## Updating the Spec

Edit `services/docs/static/openapi.yaml` directly. Air detects the change and rebuilds; the browser refresh picks up the new spec.

When adding a new service or route:

1. Add the path(s) to `openapi.yaml` under the correct tag.
2. Add request/response schemas to `components/schemas` if new shapes are introduced.
3. Verify the new route is also wired in `deploy/kong/kong.yml` — the spec and the gateway config must stay in sync.

## Environment Variable

| Variable | Default | Where used |
|---|---|---|
| `DOCS_APP_PORT` | — | `.env`, `docker-compose.yml` |
| `PORT` | `8080` | inside the container (set from `DOCS_APP_PORT` by docker-compose) |

`main.go` reads only `PORT`. Docker Compose translates `DOCS_APP_PORT` → `PORT` when starting the container, following the same pattern as all other services.

## Alternatives Considered

- **Per-service Swagger** — each service hosts its own `/swagger` route. Every service becomes a docs deployment target; docs go down with the service; no single place to test the full API. Rejected.
- **Swagger hosted on user service** — user service already existed and it was expedient. Architecturally wrong (cross-cutting concern in the wrong place; wrong failure domain). Removed.
- **Kong developer portal** — Kong Enterprise feature that auto-generates docs from declarative config. Requires Kong Enterprise license. Overkill for this project.
- **Redoc instead of Swagger UI** — read-only, cleaner layout. Lacks the "Try it out" interactive testing that makes Swagger useful during development. Not chosen, but `openapi.yaml` is compatible with Redoc if preferred.
