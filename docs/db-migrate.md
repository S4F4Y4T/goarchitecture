# Database Migrations

## Tool: golang-migrate

We use [`golang-migrate`](https://github.com/golang-migrate/migrate) — a standalone CLI that applies numbered SQL migration files up or down.

Migration files live at:
```
database/migrations/
└── user/
    ├── 000001_create_users.up.sql
    ├── 000001_create_users.down.sql
    ├── 000003_add_timestamps_to_users.up.sql
    └── 000003_add_timestamps_to_users.down.sql
```

## Migration Script

`scripts/migrate.sh` wraps the CLI so developers don't have to construct the DATABASE_URL manually:

```bash
./scripts/migrate.sh user up          # apply all pending migrations for user service
./scripts/migrate.sh user down 1      # roll back one migration
```

The script reads the root `.env` file and builds the connection string from prefixed vars (`USER_DB_*`).

Makefile targets delegate to this script:
```bash
make migrate-up SVC=user
make migrate-down SVC=user
make migrate-create SVC=user name=add_phone_column
```

## File Naming Convention

```
<version>_<description>.<up|down>.sql
```

- Version is zero-padded to 6 digits (000001, 000002, …) so lexicographic sort matches execution order.
- `up.sql` — forward change (CREATE TABLE, ALTER TABLE ADD COLUMN).
- `down.sql` — reversal of that change (DROP TABLE, ALTER TABLE DROP COLUMN). Should be idempotent where possible (`IF EXISTS`).

## Why migrations live at the repo root, not inside each service?

Migrations are operated by CI/CD pipelines and ops scripts, not by the service process itself. The service does not run migrations at startup (avoids race conditions in multi-instance deploys). Centralizing them in `database/migrations/` makes it obvious where all schema changes live.

## Why golang-migrate and not embedded migrations?

- **`golang-migrate` CLI**: language-agnostic, runs without the Go binary, easy to use in CI before the service starts.
- **Embedded (`database/sql/migrate` or GORM `AutoMigrate`)**: convenient but loses fine-grained control over exactly what SQL runs. `AutoMigrate` in particular can silently fail to drop columns or change constraints.

The service binary does **not** run migrations. Migrations are a deliberate operational action, not an automatic side effect of starting the service.

## Alternatives Considered

- **GORM AutoMigrate** — calls `db.AutoMigrate(&User{})` at startup. Easy for prototypes. Cannot drop columns, cannot change column types reliably, no rollback. Rejected for production use.
- **Atlas / Flyway / Liquibase** — more powerful migration tools. golang-migrate is simpler and Go-native; added complexity not yet justified.
- **Raw `psql` scripts** — no versioning tracking. golang-migrate maintains a `schema_migrations` table to know which version is applied.

## Known Issue

Migration numbering for the user service jumps from 000001 to 000003 (missing 000002). This is a historical artifact from development and does not affect correctness — golang-migrate applies files in numeric order and records each applied version individually.
