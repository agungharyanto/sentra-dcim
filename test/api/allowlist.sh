#!/usr/bin/env bash
# Allowed IP edge test (v0.3.6, D-078) through the real Caddy: modes off/monitor/enforce, client inside/outside the
# list, spoofed X-Forwarded-For / X-Real-IP, health and ACME paths open while enforcing. It CHANGES the allowlist
# (via the emergency CLI) and therefore refuses to run against anything but a local throwaway stack (CI).
# Usage: BASE_URL=https://localhost test/api/allowlist.sh
set -uo pipefail

main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/../../scripts/lib/source-rev.sh"
source_rev_line "$0"
BASE="${BASE_URL:-https://localhost}"
BASE="${BASE%/}"
host="${BASE#*://}"; host="${host%%[:/]*}"
if [[ "$host" != localhost && "$host" != 127.0.0.1 ]]; then
  echo "REFUSED: allowlist.sh changes the allowlist; it only runs against a local throwaway stack (got $host)" >&2
  exit 2
fi
root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root" || exit 2
# Second guard: docker compose here drives the stack of this checkout — on the server that is production.
env_domain="$(sed -n 's/^DOMAIN=//p' .env 2>/dev/null | tail -1)"
if [[ -n "$env_domain" && "$env_domain" != localhost ]]; then
  echo "REFUSED: .env has DOMAIN=$env_domain (a real deployment); allowlist.sh only runs on a throwaway stack" >&2
  exit 2
fi
pass=0 fail=0
tmp="$(mktemp -d)"
check() { if [[ "$2" == "0" ]]; then printf '  PASS  %s\n' "$1"; pass=$((pass+1)); else printf '  FAIL  %s\n' "$1"; fail=$((fail+1)); fi; }
req() { code="$(curl -sk -o "$tmp/b" -w '%{http_code}' "$@")"; }
cli() { docker compose exec -T app /sentra allowlist "$@"; }
# shellcheck disable=SC2016 # expanded inside the db container
sql() { docker compose exec -T db sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" psql -X -qAt -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$1"' sh "$1"; }
apply() { sleep 6; } # the app reloads the allowlist every 5 s
cleanup() { cli mode off >/dev/null 2>&1; rm -rf "$tmp"; }
trap cleanup EXIT

check "starts in mode off" "$(cli list | grep -qx 'mode: off'; echo $?)"
req "$BASE/"
check "off: GET / = 200" "$([[ $code == 200 ]]; echo $?)"

cli add 192.0.2.10 ci-other >/dev/null
cli mode monitor >/dev/null; apply
before="$(sql "SELECT count(*) FROM audit_events WHERE action = 'security.allowlist_outside'")"
req "$BASE/dashboard"
check "monitor: outside client still served (200)" "$([[ $code == 200 ]]; echo $?)"
after="$(sql "SELECT count(*) FROM audit_events WHERE action = 'security.allowlist_outside'")"
check "monitor: outside request audited (security.allowlist_outside)" "$([[ $after -gt $before ]]; echo $?)"

cli mode enforce --force >/dev/null; apply
req "$BASE/"
check "enforce: outside client gets 403 ip_not_allowed on the SPA" "$([[ $code == 403 ]] && jq -e '.error.code=="ip_not_allowed"' "$tmp/b" >/dev/null; echo $?)"
req "$BASE/api/v1/version"
check "enforce: outside client gets 403 on the API" "$([[ $code == 403 ]]; echo $?)"
req -H 'X-Forwarded-For: 192.0.2.10' -H 'X-Real-IP: 192.0.2.10' -H 'True-Client-IP: 192.0.2.10' -H 'Forwarded: for=192.0.2.10' "$BASE/"
check "enforce: spoofed X-Forwarded-For / X-Real-IP of a listed IP is ignored (403)" "$([[ $code == 403 ]]; echo $?)"
req "$BASE/api/v1/health"
check "enforce: /api/v1/health stays public (200)" "$([[ $code == 200 ]]; echo $?)"
req "$BASE/.well-known/acme-challenge/sentra-test"
check "enforce: HTTPS /.well-known/acme-challenge/* not blocked by the allowlist" "$([[ $code != 403 ]] || ! grep -q ip_not_allowed "$tmp/b"; echo $?)"
req "http://${BASE#*://}/.well-known/acme-challenge/sentra-test"
check "enforce: HTTP (port 80) ACME path not blocked by the allowlist" "$([[ $code != 403 ]]; echo $?)"
blocked_ip="$(sql "SELECT host(ip) FROM audit_events WHERE action = 'security.allowlist_blocked' ORDER BY chain_seq DESC LIMIT 1")"
check "enforce: blocked request audited with the real client IP ($blocked_ip, not 192.0.2.10)" "$([[ -n $blocked_ip && $blocked_ip != 192.0.2.10 ]]; echo $?)"

cli add "$blocked_ip" ci-runner >/dev/null; apply
req "$BASE/"
check "enforce: listed client served (200)" "$([[ $code == 200 ]]; echo $?)"
req "$BASE/api/v1/version"
check "enforce: listed client reaches the API (200)" "$([[ $code == 200 ]]; echo $?)"

cli mode off >/dev/null
cli remove "$blocked_ip/32" >/dev/null; cli remove 192.0.2.10/32 >/dev/null; apply
req "$BASE/"
check "emergency door: mode off via CLI restores access (200)" "$([[ $code == 200 ]]; echo $?)"

echo "Allowlist edge: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
}

main "$@"
