#!/usr/bin/env bash
# Restore a backup made by ops/backup.sh.
#
# Usage:
#   ops/restore.sh <backup-dir> [--yes] [--no-volumes] [--no-safety-backup]
#       Full restore into the live database ($POSTGRES_DB) and volumes. Stops app/caddy during restore.
#   ops/restore.sh <backup-dir> --target-db <name> [--yes]
#       Restore only the database into a NEW, separate database (verification / DR drill). Live stack untouched.
export OPS_STEP=restore
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"
load_env

src="" target_db="" assume_yes=0 with_volumes=1 safety_backup=1
while (($#)); do
  case "$1" in
    --yes) assume_yes=1 ;;
    --no-volumes) with_volumes=0 ;;
    --no-safety-backup) safety_backup=0 ;;
    --target-db) target_db="${2:?--target-db needs a name}"; shift ;;
    -*) die "unknown option $1" ;;
    *) src="$1" ;;
  esac
  shift
done
[[ -n "$src" ]] || die "usage: restore.sh <backup-dir> [--target-db name] [--yes] [--no-volumes] [--no-safety-backup]"
[[ -d "$src" ]] || die "backup dir not found: $src"
src="$(cd "$src" && pwd)"

log "verifying checksums in $src"
(cd "$src" && sha256sum --quiet -c SHA256SUMS) || die "checksum verification failed"
[[ -f "$src/db.dump" ]] || die "db.dump missing"
backup_schema="$(jq -r .schema_version "$src/manifest.json" 2>/dev/null || echo unknown)"
log "backup: $(jq -c '{created_at,app_version,schema_version}' "$src/manifest.json")"

isolated=0
if [[ -n "$target_db" && "$target_db" != "$POSTGRES_DB" ]]; then
  isolated=1
  [[ "$target_db" =~ ^[a-z_][a-z0-9_]{0,62}$ ]] || die "invalid database name '$target_db'"
  with_volumes=0 safety_backup=0
fi
db="${target_db:-$POSTGRES_DB}"

if ((!assume_yes)); then
  read -r -p "Restore $src into database '$db'$( ((with_volumes)) && echo ' and volumes'). Existing data will be REPLACED. Type 'restore' to continue: " ans
  [[ "$ans" == "restore" ]] || die "aborted"
fi

if ((safety_backup)); then
  log "taking safety backup of current state"
  "$OPS_ROOT/ops/backup.sh" pre-restore >/dev/null
fi

if ((!isolated)); then
  log "stopping app and caddy"
  compose stop app caddy >/dev/null
fi

log "recreating database $db"
db_psql -d postgres -v db="$db" <<'SQL'
SET client_min_messages = warning;
DROP DATABASE IF EXISTS :"db" WITH (FORCE);
CREATE DATABASE :"db";
SQL
db_psql -d "$db" -c "SET client_min_messages = warning; CREATE EXTENSION IF NOT EXISTS timescaledb; SELECT timescaledb_pre_restore();" >/dev/null

log "restoring dump"
errlog="$(mktemp)"
db_exec pg_restore -U "$POSTGRES_USER" -d "$db" --no-owner --role="$POSTGRES_USER" --exit-on-error <"$src/db.dump" 2>"$errlog" || {
  cat "$errlog" >&2
  rm -f "$errlog"
  die "pg_restore failed"
}
rm -f "$errlog"
db_psql -d "$db" -c "SELECT timescaledb_post_restore();" >/dev/null
db_psql -d "$db" -c "ANALYZE;" >/dev/null

restored_schema="$(schema_version "$db")"
[[ "$restored_schema" == "$backup_schema" ]] || die "schema version after restore ($restored_schema) != backup ($backup_schema)"
log "database restored (schema $restored_schema)"

if ((with_volumes)); then
  compose stop redis >/dev/null
  for vol in "${BACKUP_VOLUMES[@]}"; do
    f="$src/vol_${vol}.tar.gz" full="${COMPOSE_PROJECT}_${vol}"
    [[ -f "$f" ]] || { log "no archive for $vol, skipped"; continue; }
    docker volume inspect "$full" >/dev/null 2>&1 || docker volume create "$full" >/dev/null
    log "restoring volume $full"
    docker run --rm -i --network none -v "$full:/v" --entrypoint sh "$HELPER_IMAGE" \
      -c 'find /v -mindepth 1 -delete && tar -xzpf - -C /v' <"$f"
  done
fi

if ((isolated)); then
  log "isolated restore complete: database '$db' (drop with: DROP DATABASE $db)"
  exit 0
fi

log "starting stack"
compose up -d --wait >/dev/null
compose exec -T app /sentra migrate up >/dev/null
wait_ready 90 || die "stack not ready after restore (check: make logs SERVICE=app)"
record_run restore success "$(basename "$src")" "{\"schema_version\":\"$restored_schema\"}"
log "restore complete and stack is ready"
