#!/usr/bin/env bash
# Audit log retention (v0.3.4, D-073): archives the audit events older than AUDIT_RETENTION_DAYS (default 365,
# minimum 90) to $BACKUP_DIR/audit-archive/ and only then deletes them, leaving a checkpoint with the hash of the
# last pruned row so the chain stays verifiable. Runs in the one-shot ops container as the database owner (the
# application role cannot delete audit rows). Usage: ops/audit-prune.sh   (make audit-prune)
#   1. sentra audit prune-plan          → segment outside retention (nothing to do → exit 0)
#   2. sentra audit export --through N  → gzip JSON lines, stored 0600, gzip -t, line count checked
#   3. sentra audit prune ...           → re-exports in the same transaction and deletes only if the checksum of
#                                         the stored archive matches exactly; re-verifies the chain before commit
set -euo pipefail

main() {
  # shellcheck source=/dev/null
  source "$(dirname "$0")/../scripts/lib/source-rev.sh"
  source_rev_line "$0"
  export OPS_STEP=audit-prune
  # shellcheck source=lib.sh
  source "$(dirname "$0")/lib.sh"
  load_env
  command -v jq >/dev/null || die "jq is required"
  [[ -w "$BACKUP_DIR" ]] || die "$BACKUP_DIR not writable; run as its owner: sudo -u $(stat -c %U "$BACKUP_DIR") -H $0"
  umask 077
  local dir="$BACKUP_DIR/audit-archive"
  mkdir -p "$dir"

  local plan through from rows
  plan="$(ops_run audit prune-plan)"
  through="$(jq -er .through_seq <<<"$plan")"; from="$(jq -er .from_seq <<<"$plan")"; rows="$(jq -er .rows <<<"$plan")"
  if ((rows <= 0)); then
    log "nothing to prune (no audit events older than $(jq -r .cutoff <<<"$plan"))"
    return 0
  fi

  local name tmp sum
  name="audit-$(printf '%012d' "$from")-$(printf '%012d' "$through").jsonl.gz"
  [[ ! -e "$dir/$name" ]] || die "$dir/$name already exists (a previous run stopped after exporting?) — inspect it and remove it"
  tmp="$dir/.$name.partial"
  trap 'rm -f "$tmp"' EXIT
  log "archiving chain_seq $from..$through ($rows rows) to $dir/$name"
  ops_run audit export --through "$through" >"$tmp"
  gzip -t "$tmp" || die "archive is not a valid gzip file"
  [[ "$(gzip -dc "$tmp" | wc -l)" -eq $((rows + 1)) ]] || die "archive does not hold $rows rows"
  sum="$(sha256sum "$tmp" | cut -d' ' -f1)"
  mv "$tmp" "$dir/$name"
  printf '%s  %s\n' "$sum" "$name" >"$dir/$name.sha256"
  sync -f "$dir/$name"

  log "pruning chain_seq $from..$through"
  ops_run audit prune --through "$through" --archive-name "$name" --archive-sha256 "$sum"
  log "done; keep $dir with the backups (offsite)"
}

# The ops container carries the owner credentials (docker-compose.yml, profile ops); -T keeps stdout binary-clean.
ops_run() { compose run --rm --no-deps -T ops "$@"; }

main "$@"
