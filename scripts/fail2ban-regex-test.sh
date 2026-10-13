#!/usr/bin/env bash
# Tests the sentra-caddy-auth fail2ban filter against the synthetic sample log (deploy/fail2ban/tests).
# Uses the host's fail2ban-regex when installed (production), otherwise Ubuntu 24.04 in Docker (CI) — same
# fail2ban 1.0.x as production. Never touches the host's fail2ban configuration.
# Expected: exactly 203.0.113.10, .11 and .12 match (3 lines); everything else is missed.
set -euo pipefail

# Whole body in main(): bash parses the entire file before running, so editing the script mid-run cannot make it
# re-execute or skip parts (CLAUDE.md rule 13; the 43-vs-41 checks incident of v0.2.5, ISS-035).
main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/lib/source-rev.sh"
source_rev_line "$0"

root="$(cd "$(dirname "$0")/.." && pwd)"
sample="deploy/fail2ban/tests/caddy-access.sample.log"
host_re='sentra\.example\.go\.id'

run() { # $1 = fail2ban config dir containing filter.d/sentra-caddy-auth.conf
  fail2ban-regex --config "$1" --print-all-matched "$root/$sample" "sentra-caddy-auth[sentra_host=\"$host_re\"]"
}

if command -v fail2ban-regex >/dev/null && [[ -z "${FAIL2BAN_TEST_DOCKER:-}" ]]; then
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
  cp -r /etc/fail2ban/. "$tmp/" 2>/dev/null || true
  [[ -f "$tmp/filter.d/common.conf" ]] || { echo "cannot read /etc/fail2ban (try sudo)"; exit 1; }
  cp "$root/deploy/fail2ban/filter.d/sentra-caddy-auth.conf" "$tmp/filter.d/"
  out="$(run "$tmp")"
else
  out="$(docker run --rm -v "$root:/repo:ro" ubuntu:24.04 bash -euo pipefail -c "
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq >/dev/null && apt-get install -y -qq --no-install-recommends fail2ban >/dev/null
    cp /repo/deploy/fail2ban/filter.d/sentra-caddy-auth.conf /etc/fail2ban/filter.d/
    fail2ban-regex --version | head -1
    fail2ban-regex --print-all-matched /repo/$sample 'sentra-caddy-auth[sentra_host=\"$host_re\"]'")"
fi

echo "$out" | grep -E "^Lines:|fail2ban-regex [0-9]|^\|  \[[0-9]+\] \"ts\"" || true
matched="$(echo "$out" | sed -n '/Matched line(s):/,/^`-/p' | grep -oE '"client_ip":\s*"[0-9.]+"' |
  grep -oE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' | sort -u | tr '\n' ' ')"
dates="$(echo "$out" | sed -nE 's/^\|  \[([0-9]+)\] "ts".*/\1/p')"
lines="$(echo "$out" | sed -nE 's/^Lines: .* ([0-9]+) ignored, ([0-9]+) matched, ([0-9]+) missed.*/\1 \2 \3/p')"
echo "matched IPs: $matched"
if [[ "$lines" == "1 4 6" && "$matched" == "203.0.113.10 203.0.113.11 203.0.113.12 203.0.113.13 " && "$dates" == "11" ]]; then
  echo "fail2ban-regex: OK (4 matched, 1 ignored app 401, 6 missed, timestamp recognised on all 11 lines)"
else
  echo "fail2ban-regex: FAILED (ignored/matched/missed = ${lines:-?}, IPs = ${matched:-none}, dated lines = ${dates:-0})" >&2
  echo "$out" >&2
  exit 1
fi
}

main "$@"
