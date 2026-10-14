#!/usr/bin/env bash
# Merge a feature branch into main with --no-ff, create an annotated tag, push and redeploy the running stack so
# /api/v1/version reports the tag. Runs in the server working tree, which stays on a clean `main`; the feature
# branch lives in its own worktree (CLAUDE.md rule 13) and is found by name.
# Usage: scripts/release.sh vX.Y.Z "Title"
# Set NO_PUSH=1 to skip pushing (e.g. remote temporarily unreachable); push later with:
#   git push origin main --follow-tags
#
# fail2ban (D-057): release.sh never uses sudo. The installed jail sentra-web must match the repository
# (scripts/fail2ban-web-check.sh, sha256) before the merge and again after the redeploy; on drift the release stops
# with exit 1 and asks for `make fail2ban-web`. SKIP_FAIL2BAN_CHECK=1 only for hosts without the jail (dev).
set -euo pipefail

main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/lib/source-rev.sh"
source_rev_line "$0"

local version="${1:?usage: release.sh vX.Y.Z \"Title\"}"
local title="${2:?usage: release.sh vX.Y.Z \"Title\"}"
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "invalid version: $version"
[[ "$(git rev-parse --abbrev-ref HEAD)" == main ]] || die "run in the main working tree (checked out on main)"
[[ -z "$(git status --porcelain)" ]] || die "working tree not clean"
git rev-parse -q --verify "refs/tags/$version" >/dev/null && die "tag $version already exists"

local branches branch
branches="$(git for-each-ref --format='%(refname:short)' "refs/heads/feat/$version-*")"
[[ -n "$branches" ]] || die "no local branch feat/$version-<slug>"
[[ "$(wc -l <<<"$branches")" -eq 1 ]] || die "more than one branch feat/$version-*: $(tr '\n' ' ' <<<"$branches")"
branch="$branches"

fail2ban_check "nothing merged yet; re-run the release afterwards"

if git ls-remote --exit-code origin main >/dev/null 2>&1; then
  git pull --ff-only origin main
fi
git merge --no-ff "$branch" -m "merge: $version $title"
git tag -a "$version" -m "$title"
if [[ "${NO_PUSH:-0}" == "1" ]]; then
  echo "WARNING: NO_PUSH=1, $version not pushed. Run: git push origin main --follow-tags" >&2
else
  git push origin main "$version"
fi

# The feature worktree (if any) is removed only when clean; then the merged branch is deleted.
local wt
wt="$(git worktree list --porcelain | awk -v b="refs/heads/$branch" '/^worktree /{w=$2} $0=="branch "b{print w}')"
if [[ -n "$wt" ]]; then git worktree remove "$wt"; fi
git branch -d "$branch"

# The running build must report the new tag (`/api/v1/version` == git describe), so redeploy if the stack is up.
if docker compose ps --status running --services 2>/dev/null | grep -qx app; then
  make --no-print-directory up
fi

fail2ban_check "$version is already tagged and pushed; only the jail needs updating"
echo "released $version \"$title\""
git status --short --branch
}

die() { echo "release: ERROR: $*" >&2; exit 1; }

fail2ban_check() { # $1 = what the operator should know about the release state
  if [[ "${SKIP_FAIL2BAN_CHECK:-0}" == "1" ]]; then
    echo "release: WARNING: SKIP_FAIL2BAN_CHECK=1 — jail sentra-web not compared" >&2
    return 0
  fi
  "$root/scripts/fail2ban-web-check.sh" ||
    die "fail2ban jail sentra-web is out of date — run 'make fail2ban-web' ($1)"
}

main "$@"
