# Dependency Injection

## Approach: Manual DI via Bootstrap

Dependencies are wired by hand in `internal/bootstrap/app.go` — the single composition root that knows about every feature module in the service:

```go
func Register(db *gorm.DB, rdb *redis.Client, tokenIssuer token.AccessIssuer, accessExpiry, refreshExpiry time.Duration, cookieSecure bool) *App {
    repo := user.NewUserRepository(db)
    tokenStore := auth.NewRedisTokenStore(rdb)

    authSvc := auth.NewAuthService(repo, tokenStore, tokenIssuer, accessExpiry, refreshExpiry)
    userSvc := user.NewUserService(repo)

    return &App{
        UserHandler:   user.NewUserHandler(userSvc),
        AuthHandler:   auth.NewAuthHandler(authSvc, cookieSecure),
        HealthHandler: health.NewHandler(db),
    }
}
```

`main.go` calls `bootstrap.Register(...)` and passes the resulting `App` to the router. Note that the same `user.Repository` instance is shared between the `user` and `auth` modules — `bootstrap` is the only place that wires a dependency *across* module boundaries; neither module imports the other's constructors.

## Why Manual DI?

- **Transparent**: every dependency is visible at the call site. No magic, no reflection.
- **Fast**: zero startup overhead; no scanning, no code generation.
- **Debuggable**: if a dependency is missing, it's a compile-time error (missing function argument), not a runtime panic.
- **No dependency**: no third-party library to learn, update, or debug.

At this scale (three modules, one shared repository), the bootstrap file is ~15 lines. It does not need to be more complex.

## What the Bootstrap Does

1. Takes raw infrastructure handles (`*gorm.DB`, `*redis.Client`, the token issuer) from `main.go`.
2. Constructs the `user` repository once (concrete struct implementing `user.Repository`).
3. Passes that repository to both `user.NewUserService` (as `user.Repository`) and `auth.NewAuthService` (as `auth.UserLookup`, a narrower interface that the same concrete repository satisfies implicitly) — this is the only cross-module wiring in the service.
4. Passes each service to its own module's handler constructor.
5. Returns all handlers so the router can register routes.

## Interface at the Repository Boundary

```
user.UserService  → user.Repository       (interface, owned by the user module, full CRUD surface)
auth.AuthService  → auth.UserLookup       (interface, owned by auth itself, 3 methods)
user.UserRepository → *gorm.DB            (concrete, satisfies both interfaces above)
```

The repository boundary is the one place an interface is used for dependency inversion. Handlers depend on their module's concrete `*Service` struct directly (e.g. `UserHandler.service *UserService`) — there's no handler-facing service interface today; see [internal-architecture.md](internal-architecture.md) for when that would be worth adding (Clean Architecture's Use Case boundary).

Depending on the repository **interface** rather than the concrete GORM struct means:
- Tests can inject a fake repository without touching the database.
- Swapping GORM for `pgx` only requires rewriting `user.UserRepository`.

## Alternatives Considered

- **Wire (Google)** — code-generation DI. Eliminates manual wiring. Adds a build step, requires learning Wire's provider/injector model. Overkill until the number of dependencies becomes unwieldy (>20 constructors).
- **uber-go/fx** — runtime DI with lifecycle management. Powerful, but significant learning curve. The lifecycle hooks duplicate what our graceful shutdown already does in `main.go`.
- **samber/do** — lightweight generic DI container. Simpler than fx. Still unnecessary at this scale; adds an import and a pattern to learn.
- **init() / global vars** — globals are hard to test, hide dependencies, and cause subtle ordering bugs. Explicitly rejected.
