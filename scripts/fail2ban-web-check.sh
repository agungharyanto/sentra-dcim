#!/usr/bin/env bash
# Release drift check (no root): the jail sentra-web files installed on this host must be byte-identical
# (sha256) to what scripts/fail2ban-web-install.sh would install from this checkout. Used by scripts/release.sh
# before the merge and after the redeploy; release.sh never gets sudo, so on drift it stops and asks for
# `make fail2ban-web` (D-057).
#   scripts/fail2ban-web-check.sh   exit 0 = in sync, 1 = drift or not installed, 2 = cannot render
set -euo pipefail

main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/lib/source-rev.sh"
source_rev_line "$0"

root="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/fail2ban-web.sh
source "$root/scripts/lib/fail2ban-web.sh"
die() { echo "fail2ban-check: ERROR: $*" >&2; exit 2; }
f2b_dests

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
f2b_render "$root" "$tmp"

local drift=0 name want have
for pair in "sentra-caddy-auth.conf:$filter_dst" "sentra-web.local:$jail_dst" "sentra-docker.conf:$dropin_dst"; do
  name="${pair%%:*}"; dst="${pair#*:}"
  want="$(sha256sum < "$tmp/$name" | cut -c1-64)"
  if [[ ! -r "$dst" ]]; then
    echo "fail2ban-check: MISSING $dst"; drift=1; continue
  fi
  have="$(sha256sum < "$dst" | cut -c1-64)"
  if [[ "$want" == "$have" ]]; then
    echo "fail2ban-check: ok      $dst (${want:0:12})"
  else
    echo "fail2ban-check: DRIFT   $dst (installed ${have:0:12}, repo ${want:0:12})"; drift=1
  fi
done

if [[ $drift -ne 0 ]]; then
  echo "fail2ban-check: FAIL — installed jail sentra-web differs from the repository; run: make fail2ban-web" >&2
  exit 1
fi
echo "fail2ban-check: OK — jail sentra-web files match the repository"
}

main "$@"
