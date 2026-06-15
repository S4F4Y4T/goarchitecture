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
| `http://localhost:8100/v1/users/...` | `user_app:6969/v1/users/...` | no |
| `http://localhost:8100/v1/auth/...` | `user_app:6969/v1/auth/...` | no |
| `http://localhost:8100/v1/products/...` | `catalog_app:7070/v1/products/...` | no |
| `http://localhost:8100/docs` | `docs_app:9090/` | yes (`/docs` stripped) |

`strip_path: false` — API paths are forwarded as-is. The path prefixes (`/v1/users`, `/v1/auth`, `/v1/products`) are unique across services so Kong can route on them without translation. The `/docs` route uses `strip_path: true` — the docs service serves from `/`, so the prefix must be stripped.

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
local token_str = kong.ctx.shared.authenticated_jwt_token
if token_str then
  local b64 = token_str:match("^[^.]+%.([^.]+)%.")
  if b64 then
    -- pad + un-URL-safe the base64, then decode
    local mod = #b64 % 4
    if mod == 2 then b64 = b64 .. "=="
    elseif mod == 3 then b64 = b64 .. "=" end
    b64 = b64:gsub("%-", "+"):gsub("_", "/")
    local payload = ngx.decode_base64(b64)
    if payload then
      local uid = payload:match('"uid"%s*:%s*(%d+)')
      if uid then
        kong.service.request.set_header("X-User-ID", uid)
      end
    end
  end
end
```

The token string (not a parsed table) is extracted from shared context, the payload section is base64-decoded manually, and the `uid` field is pulled out with a pattern match. This avoids a JSON library dependency in Lua.

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
| `8100` (host) → `8000` (container) | Kong proxy | External entry point for all API traffic |
| `8101` (host) → `8001` (container) | Kong Admin API | Read-only in DB-less mode |

## Declarative Config

`deploy/kong/kong.yml` is the single source of truth for Kong's routing and plugin configuration. Abbreviated structure:

```yaml
_format_version: "3.0"

consumers:
  - username: app-users
    jwt_secrets:
      - algorithm: RS256
        key: go-microservice   # must match the "iss" claim in every JWT
        rsa_public_key: |
          -----BEGIN PUBLIC KEY-----
          ...
          -----END PUBLIC KEY-----

services:
  - name: user-service
    url: http://user_app:6969
    routes:
      - name: user-service-users     # protected
        paths: [/v1/users]
        strip_path: false
        plugins: [jwt, request-transformer, post-function]
      - name: user-service-auth      # public — no jwt plugin
        paths: [/v1/auth]
        strip_path: false
    plugins: [cors, rate-limiting, correlation-id]

  - name: catalog-service
    url: http://catalog_app:7070
    routes:
      - name: catalog-service-routes  # protected
        paths: [/v1/products]
        strip_path: false
        plugins: [jwt, request-transformer, post-function]
    plugins: [cors, rate-limiting, correlation-id]

  - name: docs-service              # public — no JWT
    url: http://docs_app:9090
    routes:
      - name: docs-route
        paths: [/docs]
        strip_path: true            # /docs stripped; docs_app serves from /
```

To add a new service: add a `services` entry pointing at the new container and define its routes and plugins. Restart Kong or hot-reload via `POST /config` with the updated file.

## Alternatives Considered

- **Kong with Postgres (DB mode)** — stores config in a database; the Admin API can create/update routes at runtime; supports Kong Manager UI. Required when you need dynamic consumer management (per-tenant API keys, OAuth flows). Adds a database to the stack and a migration step to the deploy pipeline. Not needed here since config is static.
- **nginx as a reverse proxy** — simpler, lower overhead. Does not have a plugin system; CORS, rate limiting, and correlation ID would require custom `lua` blocks or separate modules. Kong is a superset of nginx for this use case.
- **Traefik** — YAML-driven, Docker-native, auto-discovers services via container labels. Less mature plugin ecosystem than Kong. A strong alternative for pure Docker/k8s routing with no custom plugins.
- **No gateway, services exposed directly** — each service handles its own CORS, rate limiting, and request IDs. Works for two services; becomes a maintenance problem as services multiply (logic duplicated N times, inconsistently).
