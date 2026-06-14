#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 <service> <up|down> [steps]"
  echo "  service  name of the service (e.g. user, order)"
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
  DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE:-disable}"
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
