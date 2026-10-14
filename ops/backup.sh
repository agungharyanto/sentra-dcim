#!/usr/bin/env bash
# Backup: compressed pg_dump + roles + important volumes + manifest + SHA256 checksums, with retention.
# Usage: ops/backup.sh [label]
# Output: prints the backup directory path on stdout (last line).
# Env: BACKUP_DIR (default /var/backups/sentra), BACKUP_RETENTION_DAYS (default 14), BACKUP_OFFSITE_CMD (optional).
export OPS_STEP=backup
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"
load_env

label="${1:-manual}"
[[ "$label" =~ ^[a-z0-9-]{1,32}$ ]] || die "invalid label '$label' (a-z, 0-9, -)"

# Create the directory on first use only; never re-own an existing one (backups belong to the app user).
if [[ ! -d "$BACKUP_DIR" ]]; then
  sudo install -d -o "$(id -u)" -g "$(id -g)" -m 0700 "$BACKUP_DIR"
elif [[ ! -w "$BACKUP_DIR" ]]; then
  die "$BACKUP_DIR is not writable by $(id -un); run as its owner: sudo -u $(stat -c %U "$BACKUP_DIR") -H $0"
fi

# Everything created from here on (lock file, dump, archives) is owner-only: files 0600, directories 0700.
umask 077
exec 9>"$BACKUP_DIR/.lock"
chmod 0600 "$BACKUP_DIR/.lock"   # also tightens a lock file created by older versions (was 0664)
flock -n 9 || die "another backup is running"

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
dest="$BACKUP_DIR/${stamp}_${label}"
work="$dest.partial"
mkdir -p "$work"
done=0
on_exit() {
  if ((!done)); then
    rm -rf "$work"
    record_run backup failed "$label"
    log "backup FAILED, partial files removed"
  fi
}
trap on_exit EXIT

compose ps --status running --services | grep -qx db || die "db container is not running"

# External anchor of the audit hash chain (D-073), taken BEFORE the dump so the anchored row is always inside it:
# a later rewrite of the whole chain by the database owner no longer matches this file (`make audit-verify`).
echo '{}' >"$work/audit-anchor.json"
if [[ "$(db_psql -d "$POSTGRES_DB" -At -c "SELECT to_regclass('audit_checkpoints') IS NOT NULL")" == t ]]; then
  db_psql -d "$POSTGRES_DB" -At >"$work/audit-anchor.json" <<'SQL'
SELECT coalesce((SELECT json_build_object('seq', seq, 'hash', encode(h, 'hex'),
                                          'at', to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'))::text
                 FROM (SELECT chain_seq, hash FROM audit_events
                       UNION ALL SELECT through_seq, last_hash FROM audit_checkpoints) head(seq, h)
                 ORDER BY seq DESC LIMIT 1), '{}');
SQL
fi

log "dumping database $POSTGRES_DB"
db_exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --compress=6 --no-password >"$work/db.dump"
db_exec pg_dumpall -U "$POSTGRES_USER" --globals-only --no-password >"$work/globals.sql"
# Validate the dump is readable and keep its table of contents.
db_exec pg_restore --list <"$work/db.dump" >"$work/db.toc" || die "dump is not readable"

for vol in "${BACKUP_VOLUMES[@]}"; do
  full="${COMPOSE_PROJECT}_${vol}"
  if docker volume inspect "$full" >/dev/null 2>&1; then
    log "archiving volume $full"
    docker run --rm --network none -v "$full:/v:ro" --entrypoint tar "$HELPER_IMAGE" -czf - -C /v . >"$work/vol_${vol}.tar.gz"
  else
    log "volume $full not found, skipped"
  fi
done

schema="$(schema_version "$POSTGRES_DB")"
cat >"$work/manifest.json" <<JSON
{
  "created_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "label": "$label",
  "app_version": "$(app_version)",
  "git_commit": "$(git -C "$OPS_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)",
  "schema_version": "$schema",
  "database": "$POSTGRES_DB",
  "volumes": [$(printf '"%s",' "${BACKUP_VOLUMES[@]}" | sed 's/,$//')],
  "host": "$(hostname)"
}
JSON

(cd "$work" && sha256sum -- * >SHA256SUMS && sha256sum --quiet -c SHA256SUMS)
mv "$work" "$dest"
done=1
size="$(du -sh "$dest" | cut -f1)"
log "backup complete: $dest ($size, schema $schema)"
record_run backup success "$(basename "$dest")" "{\"size\":\"$size\",\"schema_version\":\"$schema\"}"

# Remove partial directories left by crashed runs (older than 1 day).
find "$BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -name '*.partial' -mtime +0 -exec rm -rf {} +

if ((BACKUP_RETENTION_DAYS > 0)); then
  find "$BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -name '20*Z_*' ! -name '*.partial' -mtime "+$BACKUP_RETENTION_DAYS" -print -exec rm -rf {} + |
    while read -r old; do log "retention: removed $old"; done
fi

if [[ -n "${BACKUP_OFFSITE_CMD:-}" ]]; then
  log "running offsite command"
  bash -c "$BACKUP_OFFSITE_CMD \"\$1\"" _ "$dest" || log "WARNING: offsite command failed"
fi

echo "$dest"
