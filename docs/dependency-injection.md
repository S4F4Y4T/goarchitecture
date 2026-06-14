# Dependency Injection

## Approach: Manual DI via Bootstrap

Dependencies are wired by hand in `internal/bootstrap/app.go`:

```go
func New(db *gorm.DB) *App {
    repo    := repository.NewUserRepository(db)
    service := service.NewUserService(repo)
    handler := handler.NewUserHandler(service)
    health  := handler.NewHealthHandler(db)
    return &App{UserHandler: handler, HealthHandler: health}
}
```

`main.go` calls `bootstrap.New(db, rdb)` and passes the resulting `App` to the router.

## Why Manual DI?

- **Transparent**: every dependency is visible at the call site. No magic, no reflection.
- **Fast**: zero startup overhead; no scanning, no code generation.
- **Debuggable**: if a dependency is missing, it's a compile-time error (missing function argument), not a runtime panic.
- **No dependency**: no third-party library to learn, update, or debug.

At this scale (one repository, one service handler), the bootstrap file is ~10 lines. It does not need to be more complex.

## What the Bootstrap Does

1. Takes raw infrastructure handles (`*gorm.DB`, `*redis.Client`) from `main.go`.
2. Constructs the repository (concrete struct implementing the domain interface).
3. Passes the repository **interface** to the service (not the concrete type).
4. Passes the service to the handler.
5. Returns all handlers so the router can register routes.

## Interface at Every Layer

```
handler → UserService (interface)
service → UserRepository (interface)
repository → *gorm.DB (concrete)
```

Each layer depends on the **interface** of the layer below, not its concrete implementation. This means:
- Tests can inject a fake repository without touching the database.
- Swapping GORM for `pgx` only requires rewriting the repository struct.

## Alternatives Considered

- **Wire (Google)** — code-generation DI. Eliminates manual wiring. Adds a build step, requires learning Wire's provider/injector model. Overkill until the number of dependencies becomes unwieldy (>20 constructors).
- **uber-go/fx** — runtime DI with lifecycle management. Powerful, but significant learning curve. The lifecycle hooks duplicate what our graceful shutdown already does in `main.go`.
- **samber/do** — lightweight generic DI container. Simpler than fx. Still unnecessary at this scale; adds an import and a pattern to learn.
- **init() / global vars** — globals are hard to test, hide dependencies, and cause subtle ordering bugs. Explicitly rejected.
