#!/usr/bin/env bash
# Merge the current feature branch into main with --no-ff, create an annotated tag, push and redeploy the
# running stack so /api/v1/version reports the tag.
# Usage: scripts/release.sh vX.Y.Z "Title"
# Set NO_PUSH=1 to skip pushing (e.g. remote temporarily unreachable); push later with:
#   git push origin main --follow-tags
set -euo pipefail

version="${1:?usage: release.sh vX.Y.Z \"Title\"}"
title="${2:?usage: release.sh vX.Y.Z \"Title\"}"

[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "invalid version: $version" >&2; exit 1; }
[[ -z "$(git status --porcelain)" ]] || { echo "working tree not clean" >&2; exit 1; }
git rev-parse -q --verify "refs/tags/$version" >/dev/null && { echo "tag $version already exists" >&2; exit 1; }

branch="$(git rev-parse --abbrev-ref HEAD)"
[[ "$branch" == feat/"$version"-* ]] || { echo "current branch '$branch' must be feat/$version-<slug>" >&2; exit 1; }

git checkout main
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
git branch -d "$branch"

# The running build must report the new tag (`/api/v1/version` == git describe), so redeploy if the stack is up.
if docker compose ps --status running --services 2>/dev/null | grep -qx app; then
  make --no-print-directory up
fi

echo "released $version \"$title\""
git status --short --branch
