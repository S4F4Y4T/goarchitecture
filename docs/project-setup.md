# Project Setup & Start

## What We Built

A `makefile` as the single entry point for every development and operational task. Air handles hot-reload during development. Environment variables (`.env`) drive all runtime config.

## Makefile Design

The `makefile` exposes a `SVC` variable (default: `user`) so the same targets work for every service:

```bash
make run SVC=user        # go run the user service
make dev SVC=user        # hot-reload the user service with air
make build SVC=user      # compile binary → ./bin/user
make migrate-up SVC=user # apply pending DB migrations
make migrate-create SVC=user name=add_phone  # scaffold new migration pair
make tidy                # go mod tidy across all modules
make test                # run all tests
make lint                # run golangci-lint
make clean               # remove bin/ and tmp/
```

**Why a Makefile?**  
Go projects don't have a universal `npm run dev` equivalent. A Makefile gives every developer (and CI) one documented interface regardless of their shell or IDE. Targets are short, readable, and composable.

**Alternatives considered:**
- **Task / Taskfile** — YAML-based, cross-platform. Rejected: adds a binary dependency; Makefile is universally available on Linux/macOS without install.
- **Shell scripts in `scripts/`** — already used for migration wrapping, but a Makefile aggregates all scripts under one roof with tab-completion.
- **Mage** — Go-native build tool. Rejected: too heavy for this stage; requires Go to compile the build tool itself.

## Air (Hot Reload)

Air watches for file changes and rebuilds/restarts the service automatically during development.

Each service has its own `.air.toml`:
- Watches the workspace root (`root = "../.."`) so changes to `pkg/` trigger a rebuild of the service.
- Rebuilds into `./bin/<service>`.
- Excludes test files, `vendor/`, `tmp/`, `bin/`.

**Why Air?**
- The most widely used live-reload tool in the Go ecosystem.
- Supports workspace-level watching — critical here because `pkg/` changes must also trigger rebuilds.
- Integrates cleanly with Docker (services run `air` in their container).

**Alternatives:**
- **`go run` with entr/watchexec** — filesystem watchers piped to `go run`. Works but requires extra tooling and produces less useful output.
- **Manual restart** — viable for small changes, but painful for inner-loop development.
- **Reflex** — similar to air. Less maintained, smaller community.

## Environment Configuration

All config is read from environment variables. `godotenv` loads a `.env` file at startup (best-effort: if the file is missing, env vars from the shell are used). All variables for every service and shared infrastructure live in the single root `.env` (docker-compose reads it directly). Variables are prefixed per service to avoid collisions:

```
USER_APP_PORT=6969
USER_DB_PASSWORD=<change-me>
AUTH_APP_PORT=6868
REDIS_PORT=6380
REDIS_PASSWORD=<change-me>
JWT_PRIVATE_KEY_PATH=/app/deploy/kong/jwt.key
COOKIE_SECURE=true
```

Copy `.env.example` to `.env` and fill in every `<change-me>` before running `docker compose up`. The docker-compose `environment:` blocks translate prefixed root vars into the unprefixed names the service processes actually read (e.g. `DB_PASSWORD: ${USER_DB_PASSWORD}`).

**Why env vars?**
- 12-factor app compliance: config is injected, not embedded.
- Works identically in Docker Compose (environment section), Kubernetes (ConfigMap/Secret), and local shell.
- `.env.example` in version control documents every variable without leaking secrets.

**Why not YAML/TOML config files?**
- Env vars are simpler to inject in containers — no need to mount config files or template them.
- Secrets (DB passwords, Redis passwords) are always env vars anyway; mixing sources increases complexity.

**Env var parsing: fail-graceful for optional values, fail-fast for required ones.**  
`pkg/config/env.go` provides `GetEnvInt(key, default)` and `GetEnvDuration(key, default)`. If the value is set but cannot be parsed (e.g., `RATE_LIMIT_REQUESTS=abc`), they log a warning and fall back to the default rather than crashing. This is intentional for optional tuning values — a misconfigured pool size or rate-limit window should not take down the service. Required values (`PORT`, `DB_HOST`, etc.) are validated in `LoadConfig()` with a hard error and `os.Exit(1)` on startup.

## Startup Flow

```
main() →
  1. Init logger (structured JSON, level from LOG_LEVEL env)
  2. LoadConfig() — read env vars, validate required fields
  3. SetupDatabase() — open GORM connection, tune pool, ping (5s timeout)
  4. SetupRedis() — connect if REDIS_ADDR set; nil if absent
  5. Bootstrap() — wire repo → service → handler
  6. NewRouter() — register routes + middleware chain
  7. http.Server{} — with timeouts (Read 10s, Write 10s, Idle 60s)
  8. ListenAndServe in goroutine
  9. Wait for SIGINT/SIGTERM
  10. Graceful shutdown (1s drain timeout)
```

**Why validate config at startup?**  
Fail fast. A missing `DB_PASSWORD` should be a startup panic with a clear message, not a nil-pointer crash at the first request.

**Why graceful shutdown?**  
In-flight requests (especially long DB queries) should complete before the process exits. The 1-second timeout is a short window sufficient for health-check-based load balancers to drain traffic before the OS reclaims the port.
