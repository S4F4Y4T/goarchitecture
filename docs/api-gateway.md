# API Gateway

## What It Is

[Kong](https://konghq.com/products/kong-gateway) runs as a reverse proxy in front of all services. Every external request hits Kong first; Kong routes it to the correct service and applies cross-cutting concerns (CORS, rate limiting, correlation ID) before the request reaches application code.

## Mode: DB-less (Declarative)

Kong runs with `KONG_DATABASE=off`. All configuration lives in `deploy/kong/kong.yml` and is loaded at startup. There is no external database to manage.

**Why DB-less?**
- Config is a file in version control — reviewed, diffed, and deployed like any other code.
- No extra Postgres to provision, back up, or migrate.
- One `docker compose up` gets everything running, including Kong.

The tradeoff: the Admin API (`:8001`) is read-only. You cannot add routes or consumers at runtime via the API. Any change requires editing `kong.yml` and restarting Kong (or hot-reloading via `POST /config`).

## Routing

| External path | Forwarded to | Strip prefix |
|---|---|---|
| `http://localhost:8000/v1/users/...` | `user_app:6969/v1/users/...` | no |
| `http://localhost:8000/v1/auth/...` | `user_app:6969/v1/auth/...` | no |
| `http://localhost:8000/v1/products/...` | `catalog_app:7070/v1/products/...` | no |

`strip_path: false` — paths are forwarded as-is. The frontend calls `/v1/users/` and the service receives `/v1/users/`. No prefix translation needed because the path prefixes (`/v1/users`, `/v1/auth`, `/v1/products`) are unique across services, so Kong can route on them directly.

## Service Access

The `user_app` and `catalog_app` containers use `expose` instead of `ports`. This makes their ports reachable within the Docker network (by Kong) but not from the host machine. All external traffic must go through Kong on port `8000`.

```yaml
expose:
  - "${USER_APP_PORT}"   # visible to Kong, not to the host
```

## Plugins

Four plugins are in play — three at the service level, one at the route level.

### JWT Verification (route-level)
Applied to `/v1/users` and `/v1/products` only — not `/v1/auth`. Three plugins run in sequence on each protected request:

1. **`jwt`** — reads the `iss` claim, finds the matching consumer credential (`key: go-microservice`), and verifies the RS256 signature using the embedded RSA public key. Requests with a missing, expired, or tampered token receive `401` before going further.

2. **`request-transformer`** — strips any incoming `X-User-ID` header from the client, preventing header forgery.

3. **`post-function`** (Lua) — reads `uid` from the verified JWT claims and injects it as `X-User-ID` on the forwarded request. The service reads this header to identify the caller — no JWT parsing needed in the service.

```lua
local token = kong.ctx.shared.authenticated_jwt_token
if token and token.claims and token.claims.uid then
  kong.service.request.set_header("X-User-ID", tostring(token.claims.uid))
end
```

The public key lives in the `consumers` block of `kong.yml`. The private key lives only in the user service. See [auth.md](auth.md) for the full token lifecycle.

### CORS
Handles preflight `OPTIONS` requests and sets `Access-Control-*` headers on all responses. Configured at the gateway so the services themselves do not need to set these headers.

Because Kong is the only layer sending CORS headers, there are no duplicate `Access-Control-Allow-Origin` values on the response (a problem that occurs when both the gateway and the service set the header independently).

The corresponding service-level `middleware.Cors` calls are commented out in each router.

### Rate Limiting
Fixed-window per-IP rate limit (100 req/min by default), enforced using Kong's `local` policy (in-memory counter per Kong node).

**Why Kong instead of the service-level middleware?**  
When a proxy sits in front, `r.RemoteAddr` inside the service is Kong's container IP — not the real client IP. The service-level Redis counter would key on Kong's IP and apply one shared quota to all clients. Kong sees the real client IP and can rate-limit per-client correctly.

The service-level `middleware.RateLimit` calls are commented out in each router.

### Correlation ID
Injects an `X-Request-ID` UUID on every request that does not already carry one. The header is forwarded upstream (so services log it) and echoed back in the response (so clients can trace requests).

The service-level `middleware.RequestID` middleware is still active — it reads the header Kong already set, stores it in the request context, and makes it available to logger and handlers via `middleware.GetRequestID(ctx)`. No UUID is generated at the service level when Kong is in front.

## Ports

| Port | What | Notes |
|---|---|---|
| `8000` | Kong proxy | External entry point for all API traffic |
| `8002` (host) → `8001` (container) | Kong Admin API | Read-only in DB-less mode |

## Declarative Config

`deploy/kong/kong.yml` is the single source of truth for Kong's routing and plugin configuration:

```yaml
_format_version: "3.0"

services:
  - name: user-service
    url: http://user_app:6969
    routes:
      - name: user-service-routes
        paths: [/user]
        strip_path: true
    plugins:
      - name: cors
      - name: rate-limiting
      - name: correlation-id
```

To add a new service: add a `services` entry pointing at the new container and define its routes and plugins. Restart Kong or call `POST /config` with the updated file.

## Alternatives Considered

- **Kong with Postgres (DB mode)** — stores config in a database; the Admin API can create/update routes at runtime; supports Kong Manager UI. Required when you need dynamic consumer management (per-tenant API keys, OAuth flows). Adds a database to the stack and a migration step to the deploy pipeline. Not needed here since config is static.
- **nginx as a reverse proxy** — simpler, lower overhead. Does not have a plugin system; CORS, rate limiting, and correlation ID would require custom `lua` blocks or separate modules. Kong is a superset of nginx for this use case.
- **Traefik** — YAML-driven, Docker-native, auto-discovers services via container labels. Less mature plugin ecosystem than Kong. A strong alternative for pure Docker/k8s routing with no custom plugins.
- **No gateway, services exposed directly** — each service handles its own CORS, rate limiting, and request IDs. Works for two services; becomes a maintenance problem as services multiply (logic duplicated N times, inconsistently).
