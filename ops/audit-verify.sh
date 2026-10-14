#!/usr/bin/env bash
# Verifies the tamper-evident audit chain (D-073) of the running stack against the chain heads anchored in backups
# ($BACKUP_DIR/*/audit-anchor.json), so even a rewrite of the whole chain by the database owner is detected.
# Usage: ops/audit-verify.sh            all backups
#        UNTIL=<backup dir name> ...    only anchors up to that backup (after restoring it, later backups hold a
#                                       history that was deliberately rolled back)
# Exit 0 = chain intact; 1 = broken or error (the JSON result on stdout names the first bad chain_seq).
set -euo pipefail

main() {
  # shellcheck source=/dev/null
  source "$(dirname "$0")/../scripts/lib/source-rev.sh"
  source_rev_line "$0"
  export OPS_STEP=audit-verify
  # shellcheck source=lib.sh
  source "$(dirname "$0")/lib.sh"
  load_env

  [[ -r "$BACKUP_DIR" ]] || die "$BACKUP_DIR not readable; run as its owner: sudo -u $(stat -c %U "$BACKUP_DIR") -H $0"
  local anchors="" n=0 dir name
  for dir in "$BACKUP_DIR"/*/; do
    dir="${dir%/}"; name="$(basename "$dir")"
    [[ "$name" == *.partial || ! -f "$dir/audit-anchor.json" ]] && continue
    [[ -n "${UNTIL:-}" && "$name" > "$UNTIL" ]] && continue
    if ! (cd "$dir" && grep -E '  audit-anchor\.json$' SHA256SUMS | sha256sum --quiet -c -) 2>/dev/null; then
      log "WARNING: $name/audit-anchor.json does not match its SHA256SUMS — skipped"
      continue
    fi
    anchors+="$(cat "$dir/audit-anchor.json")"$'\n'; n=$((n + 1))
  done
  log "verifying the audit chain against $n backup anchor(s)"
  printf '%s' "$anchors" | compose exec -T app /sentra audit verify --anchors -
}

main "$@"
