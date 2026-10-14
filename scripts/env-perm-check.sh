#!/usr/bin/env bash
# .env permission check (v0.3.9, ISS-046): the file holds every secret, so it must be owner-only — mode 0600 and, on
# a server with a separate admin user (ops/bootstrap-new-server.sh --admin-user), one extra read-only ACL entry for
# that admin. Called by `make preflight` (so `make up`) and scripts/release.sh; exits 1 on any deviation.
#   allowed: user::rw-, group::---, other::---, optional named users with r-- (mask r--), no named groups,
#            no default ACL; if the caller is not the owner (admin running make), the caller needs user:<caller>:r--.
# Usage: scripts/env-perm-check.sh [path/to/.env]   (default: .env in the repository root; symlinks are followed)
set -uo pipefail

main() {
local env="${1:-$(cd "$(dirname "$0")/.." && pwd)/.env}"
local f me owner fix
f="$(readlink -f "$env")" || f="$env"
[[ -f "$f" ]] || fail "$env: not a regular file"
me="$(id -un)"
owner="$(stat -c %U "$f")"
fix="sudo setfacl -b $f && sudo chmod 0600 $f"
[[ "$owner" != "$me" && "$me" != root ]] && fix+=" && sudo setfacl -m u:$me:r $f"

if ! command -v getfacl >/dev/null 2>&1; then
  [[ "$(stat -c %a "$f")" == 600 ]] || fail "$f: mode $(stat -c %a "$f"), want 0600 (fix: chmod 0600 $f)"
  [[ "$owner" == "$me" || "$me" == root ]] || fail "$f: owned by $owner, getfacl missing to verify read access for $me"
  return 0
fi

local acl bad=()
acl="$(getfacl -cpE "$f" 2>/dev/null)" || fail "$f: cannot read the ACL (run as the owner or the admin user)"
grep -qx 'user::rw-' <<<"$acl" || grep -qx 'user::r--' <<<"$acl" || bad+=("owner $(grep '^user::' <<<"$acl")")
grep -qx 'group::---' <<<"$acl" || bad+=("$(grep '^group::' <<<"$acl") (want group::---)")
grep -qx 'other::---' <<<"$acl" || bad+=("$(grep '^other::' <<<"$acl") (want other::---)")
local e
while read -r e; do bad+=("$e (named users may only read)"); done < <(grep -E '^user:[^:]+:' <<<"$acl" | grep -v ':r--$')
while read -r e; do bad+=("$e (no named groups)"); done < <(grep -E '^group:[^:]+:' <<<"$acl")
while read -r e; do bad+=("$e (mask must be r--)"); done < <(grep '^mask::' <<<"$acl" | grep -vx 'mask::r--')
while read -r e; do bad+=("$e (no default ACL on a file)"); done < <(grep '^default:' <<<"$acl")
if [[ "$owner" != "$me" && "$me" != root ]] && ! grep -qx "user:$me:r--" <<<"$acl"; then
  bad+=("no user:$me:r-- entry (the admin running make must be able to read it)")
fi
if ((${#bad[@]})); then
  printf 'env-perm-check: %s is not owner-only (ISS-046):\n' "$f" >&2
  printf '  - %s\n' "${bad[@]}" >&2
  echo "  fix: $fix" >&2
  exit 1
fi
}

fail() { echo "env-perm-check: $*" >&2; exit 1; }

main "$@"
