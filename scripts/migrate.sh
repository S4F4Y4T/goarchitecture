#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 <service> <up|down> [steps]"
  echo "  service  name of the service (e.g. user, catalog)"
  echo "  up|down  migration direction"
  echo "  steps    number of steps for down (default: 1)"
  exit 1
}

[ $# -lt 2 ] && usage

SERVICE=$1
DIRECTION=$2
STEPS=${3:-1}

MIGRATIONS_DIR="$(cd "$(dirname "$0")/.." && pwd)/database/migrations/${SERVICE}"

if [ ! -d "$MIGRATIONS_DIR" ]; then
  echo "Error: migrations directory not found: $MIGRATIONS_DIR"
  exit 1
fi

if [ -z "${DATABASE_URL:-}" ]; then
  source "$(dirname "$0")/../.env" 2>/dev/null || true

  # Root .env uses per-service prefixed vars (USER_DB_HOST, CATALOG_DB_HOST …).
  # Derive the prefix from the service name (uppercased).
  PREFIX="${SERVICE^^}"

  DB_USER="${!PREFIX}_DB_USER"
  DB_PASSWORD="${!PREFIX}_DB_PASSWORD"
  DB_HOST="${!PREFIX}_DB_HOST"
  DB_PORT="${!PREFIX}_DB_PORT"
  DB_NAME="${!PREFIX}_DB_NAME"
  DB_SSLMODE="${!PREFIX}_DB_SSLMODE"

  # Resolve each variable through indirection, falling back to empty string.
  DB_USER="${!DB_USER:-}"
  DB_PASSWORD="${!DB_PASSWORD:-}"
  DB_HOST="${!DB_HOST:-localhost}"
  DB_PORT="${!DB_PORT:-5432}"
  DB_NAME="${!DB_NAME:-}"
  DB_SSLMODE="${!DB_SSLMODE:-disable}"

  DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"
fi

case "$DIRECTION" in
  up)
    migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" up
    ;;
  down)
    migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" down "$STEPS"
    ;;
  *)
    usage
    ;;
esac
