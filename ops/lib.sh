#!/usr/bin/env bash
# shellcheck disable=SC2034 # variables below are consumed by scripts sourcing this file
# Shared helpers for ops scripts. Source only.
set -euo pipefail

OPS_ROOT="${OPS_ROOT_OVERRIDE:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
export OPS_ROOT

log() { printf '[%s] [%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${OPS_STEP:-ops}" "$*" >&2; }
die() { log "ERROR: $*"; exit 1; }

load_env() {
  [[ -f "$OPS_ROOT/.env" ]] || die ".env not found in $OPS_ROOT"
  set -a
  # shellcheck source=/dev/null
  source "$OPS_ROOT/.env"
  set +a
  BACKUP_DIR="${BACKUP_DIR:-/var/backups/sentra}"
  BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
}

compose() { (cd "$OPS_ROOT" && docker compose "$@"); }

# Run a PostgreSQL client tool inside the db container. The password is read from the container's own
# environment so it never appears in host process arguments.
# shellcheck disable=SC2016 # expanded inside the container on purpose
db_exec() { compose exec -T db sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" exec "$@"' sh "$@"; }

# psql inside the db container. Extra args are passed to psql.
db_psql() { db_exec psql -v ON_ERROR_STOP=1 -X -q -U "$POSTGRES_USER" "$@"; }

app_version() { git -C "$OPS_ROOT" describe --tags --always --dirty 2>/dev/null || echo unknown; }

# Current schema version ("none" when no migration applied).
schema_version() {
  db_psql -d "${1:-$POSTGRES_DB}" -tA -c "SELECT CASE WHEN to_regclass('schema_migrations') IS NULL THEN 'none' ELSE (SELECT COALESCE(max(version)::text,'none') FROM schema_migrations) END" 2>/dev/null | tr -d '[:space:]'
}

# Record an ops run when the ops_runs table exists (migration 000002+). Never fails the caller.
record_run() { # kind status ref [details_json]
  local details="${4:-}"
  [[ -n "$details" ]] || details='{}'
  db_psql -d "$POSTGRES_DB" \
    -v kind="$1" -v status="$2" -v ref="$3" -v ver="$(app_version)" -v details="$details" <<'SQL' >/dev/null 2>&1 || true
SELECT to_regclass('ops_runs') IS NOT NULL AS has_table \gset
\if :has_table
INSERT INTO ops_runs (kind, status, ref, app_version, details) VALUES (:'kind', :'status', :'ref', :'ver', :'details'::jsonb);
\endif
SQL
}

wait_ready() { # timeout_seconds
  local url="http://${CADDY_HTTP_BIND:-127.0.0.1:8080}/api/v1/ready" deadline=$((SECONDS + ${1:-90}))
  until curl -fsS -o /dev/null "$url"; do
    ((SECONDS < deadline)) || return 1
    sleep 2
  done
}

# Named volumes (besides the database) included in backups. Used by scripts sourcing this file.
BACKUP_VOLUMES=(caddy_data caddy_config redis_data)
COMPOSE_PROJECT="sentra"
HELPER_IMAGE="redis:8.10.2-alpine" # any small image with busybox tar; already pulled by the stack
