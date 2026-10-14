#!/usr/bin/env bash
# Maintenance page test (v0.3.7): stops the app container and expects Caddy to answer 503 + Retry-After with the
# static maintenance page (ID/EN, no version or stack details) for pages and a JSON error for /api/*; then starts
# the app again and expects normal service. It STOPS the app, so it refuses to run against a real deployment (CI).
# Usage: BASE_URL=https://localhost test/api/maintenance.sh
set -uo pipefail

main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/../../scripts/lib/source-rev.sh"
source_rev_line "$0"
BASE="${BASE_URL:-https://localhost}"
BASE="${BASE%/}"
host="${BASE#*://}"; host="${host%%[:/]*}"
if [[ "$host" != localhost && "$host" != 127.0.0.1 ]]; then
  echo "REFUSED: maintenance.sh stops the app; it only runs against a local throwaway stack (got $host)" >&2
  exit 2
fi
root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root" || exit 2
env_domain="$(sed -n 's/^DOMAIN=//p' .env 2>/dev/null | tail -1)"
if [[ -n "$env_domain" && "$env_domain" != localhost ]]; then
  echo "REFUSED: .env has DOMAIN=$env_domain (a real deployment); maintenance.sh only runs on a throwaway stack" >&2
  exit 2
fi
pass=0 fail=0
tmp="$(mktemp -d)"
check() { if [[ "$2" == "0" ]]; then printf '  PASS  %s\n' "$1"; pass=$((pass+1)); else printf '  FAIL  %s\n' "$1"; fail=$((fail+1)); fi; }
req() { code="$(curl -sk -o "$tmp/b" -D "$tmp/h" -w '%{http_code}' "$@")"; }
hdr() { grep -i "^$1:" "$tmp/h" | head -1 | cut -d' ' -f2- | tr -d '\r'; }
start_app() {
  docker compose start app >/dev/null 2>&1
  for _ in $(seq 1 60); do
    [[ "$(docker inspect -f '{{.State.Health.Status}}' "$(docker compose ps -q app)" 2>/dev/null)" == healthy ]] && return 0
    sleep 1
  done
  return 1
}
restore() { start_app; rm -rf "$tmp"; }
trap restore EXIT

style_hash="$(python3 -c "
import re,hashlib,base64
s=open('deploy/caddy/maintenance/index.html').read()
print(base64.b64encode(hashlib.sha256(re.search(r'<style>(.*?)</style>',s,re.S).group(1).encode()).digest()).decode())")"

docker compose stop app >/dev/null 2>&1
sleep 2
for p in / /dashboard /audit?x=1; do
  req "$BASE$p"
  check "app down: GET $p = 503 maintenance page (ID + EN)" "$([[ $code == 503 ]] && grep -q 'Sedang pemeliharaan' "$tmp/b" && grep -q 'Under maintenance' "$tmp/b"; echo $?)"
done
req "$BASE/"
check "app down: Retry-After set, no-store" "$([[ -n $(hdr Retry-After) && $(hdr Cache-Control) == *no-store* ]]; echo $?)"
check "app down: page is HTML" "$([[ $(hdr Content-Type) == text/html* ]]; echo $?)"
check "app down: CSP pins the inline style by hash ($style_hash)" "$([[ $(hdr Content-Security-Policy) == *"'sha256-$style_hash'"* && $(hdr Content-Security-Policy) != *unsafe-inline* ]]; echo $?)"
check "app down: no version, stack or upstream details in the page or headers" "$(! grep -qiE 'v0\.[0-9]+|caddy|golang|502|bad gateway|dial tcp|app:808' "$tmp/b" "$tmp/h"; echo $?)"
for p in /api/v1/version /api/v1/health /api/v1/auth/me; do
  req "$BASE$p"
  check "app down: GET $p = 503 JSON maintenance" "$([[ $code == 503 && $(hdr Content-Type) == application/json* ]] && jq -e '.error.code=="maintenance"' "$tmp/b" >/dev/null; echo $?)"
done

check "app restarted and healthy" "$(start_app; echo $?)"
# Caddy's next active health check (every 15 s) marks the upstream healthy again. Wait for the API too: the SPA
# coming back first and /api/v1/health still 503 a moment later failed CI once (v0.3.9, ISS-047).
for _ in $(seq 1 45); do
  req "$BASE/api/v1/health"; [[ $code == 200 ]] && { req "$BASE/"; [[ $code == 200 ]] && break; }; sleep 1
done
check "app back: GET / = 200 (SPA)" "$([[ $code == 200 ]] && ! grep -q 'Sedang pemeliharaan' "$tmp/b"; echo $?)"
req "$BASE/api/v1/health"
check "app back: /api/v1/health = 200" "$([[ $code == 200 ]]; echo $?)"

echo "Maintenance page: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
}

main "$@"
