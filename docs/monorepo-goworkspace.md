# Monorepo & Go Workspace

## What We Did

Three separate Go modules in one Git repository, linked by a `go.work` file:

```
go.work
go 1.25
use (
    ./pkg
    ./services/user
    ./services/catalog
)
```

Each service imports the shared library locally:
```go
import "github.com/s4f4y4t/go-microservice/pkg/apperror"
```

During development, Go resolves this to the local `./pkg` directory via the workspace. When built in CI (without `go.work`), the published module version is used.

## Why Monorepo?

- **Single source of truth**: schema changes, shared lib changes, and all services live in one PR. No "which version of pkg does user service use?" confusion.
- **Atomic refactoring**: rename a function in `pkg/` and fix all callers in both services in one commit.
- **Shared tooling**: one `makefile`, one `docker-compose.yml`, one `.env.example`.
- **Discoverability**: a new developer checks out one repo and understands the whole system.

**Tradeoff**: As the number of services grows, monorepo CI becomes slower (all services rebuild on any change). Accepted at this scale; can add path-based CI filters later.

## Why Go Workspace (`go.work`) Specifically?

Before `go.work` (Go 1.18+), sharing local modules required `replace` directives in each service's `go.mod`:

```go
// go.mod (old way, before workspace)
replace github.com/s4f4y4t/go-microservice/pkg => ../../pkg
```

This was error-prone (forget to remove before tagging a release) and made CI inconsistent.

`go.work` is a workspace-level overlay that:
- Lives **outside** individual `go.mod` files (should be gitignored in some conventions, but we commit it for reproducibility)
- Requires no changes to `go.mod`
- Is transparent to CI if `go.work` is not present (services resolve via their published module path)

## Module Boundaries

| Module | Import path | What it contains |
|---|---|---|
| `pkg` | `github.com/s4f4y4t/go-microservice/pkg` | All shared, domain-agnostic code |
| `services/user` | `github.com/s4f4y4t/go-microservice/services/user` | User domain — models, repo, service, handler |
| `services/catalog` | `github.com/s4f4y4t/go-microservice/services/catalog` | Catalog domain |

Services **never import each other**. All cross-service communication will go through HTTP or message queues (future).

## Alternatives Considered

- **Single module at root** — simpler `go.mod`, no workspace needed. All packages importable by all other packages (no `internal` isolation across services). Rejected: two services sharing one module means they must be released together and can accidentally import each other's internal code.
- **Separate Git repos** — maximum isolation. Every `pkg` change requires a tagged release + version bump in each service. High overhead for a small team. Viable at large scale; deferred.
- **Git submodules** — `pkg` as a submodule in each service repo. Notoriously complex to manage, especially for developers unfamiliar with submodule workflows. Rejected.
