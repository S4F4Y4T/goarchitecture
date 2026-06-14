# Router

## Implementation: `net/http` ServeMux

Routes are registered on the standard `http.ServeMux` introduced in Go 1.22+, which added path parameters and method-based routing:

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /v1/users/", handler.GetAllUsers)
mux.HandleFunc("POST /v1/users/", handler.CreateUser)
mux.HandleFunc("GET /v1/users/{id}", handler.GetUserByID)
mux.HandleFunc("PUT /v1/users/{id}", handler.UpdateUser)
mux.HandleFunc("DELETE /v1/users/{id}", handler.DeleteUser)
```

Path parameters are extracted with `r.PathValue("id")`.

## Route Structure

```
/healthz           GET  — liveness probe
/readyz            GET  — readiness probe
/swagger/          GET  — Swagger UI
/swagger/openapi.yaml GET

/v1/users/         GET  — list users (paginated, filterable, sortable)
/v1/users/         POST — create user
/v1/users/{id}     GET  — get user by ID
/v1/users/{id}     PUT  — update user (full replacement)
/v1/users/{id}     DELETE — delete user
```

## Versioning

All business routes are under `/v1/`. This allows adding `/v2/` routes without removing `/v1/`.

Health and Swagger endpoints are not versioned — they are infrastructure endpoints, not API contract endpoints.

## Sub-Mux for Versioned Routes

```go
v1Mux := http.NewServeMux()
RegisterUsersRoute(v1Mux, userHandler)
mux.Handle("/v1/", http.StripPrefix("/v1", v1Mux))
```

Routes are registered on a sub-mux that is then mounted at `/v1/`. Adding a new service group (e.g., `/v1/products/`) is a one-line `mux.Handle` call.

## Route Registration: Separate Function

`RegisterUsersRoute(mux, handler)` lives in `internal/router/user.go` (or `product.go`). The main `NewRouter()` function calls all registration functions. This keeps the router file short regardless of how many resource types exist.

## Why Standard Library Mux (Not chi, Gorilla, Gin)?

Go 1.22's enhanced `ServeMux` supports:
- Method-specific routing (`GET /users/`)
- Path parameters (`/users/{id}`)

This covers 100% of our routing needs without a third-party dependency.

**Alternatives considered:**
- **chi** — lightweight, idiomatic. Adds `chi.Router.Use()` for middleware chaining and more expressive middleware handling. The main reason to adopt chi would be if route-level middleware becomes complex. Currently our middleware chain is global, so chi's advantage doesn't apply.
- **Gin** — high performance, popular. Brings its own `Context` type that wraps `*http.Request`. This means middleware from the `net/http` ecosystem (including our `pkg/middleware`) wouldn't compose directly. Rejected to avoid lock-in.
- **Echo** — similar to Gin. Same concern about custom context types.
- **Fiber** — built on fasthttp rather than `net/http`. Incompatible with standard library middleware. Rejected.

## Why No Router Middleware via the Framework?

All middleware is applied at the `http.Server` level via `middleware.Chain()`, not via a router's `.Use()` method. This means middleware is independent of the router choice — we could swap `ServeMux` for chi without touching middleware code.
