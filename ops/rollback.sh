#!/usr/bin/env bash
# Roll the deployment back (or forward) to a git ref: backup -> migrate schema -> checkout -> build & up -> verify.
#
# Usage: ops/rollback.sh <vX.Y.Z|main> [--yes]
#
# Order matters: when the target has fewer migrations, the schema is migrated DOWN with the CURRENT binary
# (older binaries do not contain the newer *.down.sql files). When the target has more migrations, `make up`
# of the target migrates UP. Minimum supported target: v0.1.3 (first release with `sentra migrate`).
export OPS_STEP=rollback
set -euo pipefail

# git checkout rewrites this file while bash is still reading it: re-exec from a private copy.
if [[ -z "${SENTRA_ROLLBACK_REEXEC:-}" ]]; then
  here="$(cd "$(dirname "$0")" && pwd)"
  tmp="$(mktemp -d)"
  cp "$here/rollback.sh" "$here/lib.sh" "$here/backup.sh" "$tmp/"
  export SENTRA_ROLLBACK_REEXEC="$tmp" OPS_ROOT_OVERRIDE="${OPS_ROOT_OVERRIDE:-$(cd "$here/.." && pwd)}"
  exec bash "$tmp/rollback.sh" "$@"
fi
trap 'rm -rf "$SENTRA_ROLLBACK_REEXEC"' EXIT

# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"
load_env

ref="" assume_yes=0
for a in "$@"; do
  case "$a" in
    --yes) assume_yes=1 ;;
    -*) die "unknown option $a" ;;
    *) ref="$a" ;;
  esac
done
[[ -n "$ref" ]] || die "usage: rollback.sh <vX.Y.Z|main> [--yes]"

cd "$OPS_ROOT"
[[ -z "$(git status --porcelain)" ]] || die "working tree not clean"
git rev-parse -q --verify "$ref^{commit}" >/dev/null || die "unknown git ref '$ref'"
git show "$ref:cmd/sentra/main.go" 2>/dev/null | grep -q 'runMigrate' || die "'$ref' predates migration support (minimum v0.1.3)"

from="$(curl -fsS "http://${CADDY_HTTP_BIND:-127.0.0.1:8080}/api/v1/version" 2>/dev/null | jq -r .version 2>/dev/null || true)"
from="${from:-$(app_version)}"
current_schema="$(schema_version "$POSTGRES_DB")"
[[ "$current_schema" == "none" ]] && current_schema=0
target_schema="$(git ls-tree --name-only "$ref" migrations/ | sed -nE 's#^migrations/0*([0-9]+)_.*\.up\.sql$#\1#p' | sort -n | tail -1)"
target_schema="${target_schema:-0}"
target_desc="$(git describe --tags --always "$ref")"

log "from $from (schema $current_schema) -> $ref [$target_desc] (schema $target_schema)"
if ((!assume_yes)); then
  read -r -p "Proceed with rollback to $ref? A backup is taken first. Type 'rollback' to continue: " ans
  [[ "$ans" == "rollback" ]] || die "aborted"
fi

backup_path="$(bash "$(dirname "$0")/backup.sh" rollback)"
log "backup: $backup_path"

if ((target_schema < current_schema)); then
  log "migrating schema down to $target_schema with current binary"
  compose exec -T app /sentra migrate goto "$target_schema"
fi

if [[ "$ref" == "main" ]]; then
  git checkout -q main
else
  git checkout -q --detach "$ref"
fi
log "checked out $(git describe --tags --always)"

log "building and starting target version"
make --no-print-directory up >/dev/null

wait_ready 120 || die "target not ready; restore with: ops/restore.sh $backup_path"
got="$(curl -fsS "http://${CADDY_HTTP_BIND:-127.0.0.1:8080}/api/v1/version" | jq -r .version)"
want="$(app_version)"
[[ "$got" == "$want" ]] || die "running version '$got' != expected '$want'"
record_run rollback success "$ref" "{\"from\":\"$from\",\"to\":\"$got\",\"schema\":\"$(schema_version "$POSTGRES_DB")\",\"backup\":\"$(basename "$backup_path")\"}"
log "rollback complete: now running $got (schema $(schema_version "$POSTGRES_DB"))"
