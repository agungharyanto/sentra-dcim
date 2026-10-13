#!/usr/bin/env bash
# Public edge tests (Caddy + Let's Encrypt) against the real domain.
# Usage: DOMAIN=sentra.bajastu.id test/api/public.sh   (defaults to DOMAIN from .env)
# DOMAIN=localhost (local mode, DNS not pointed yet): same proxy/header checks against https://localhost, verified
# with Caddy's internal CA root instead of Let's Encrypt.
set -uo pipefail

if [[ -z "${DOMAIN:-}" && -f "$(dirname "$0")/../../.env" ]]; then
  DOMAIN="$(sed -n 's/^DOMAIN=//p' "$(dirname "$0")/../../.env" | tail -1)"
fi
[[ -n "${DOMAIN:-}" ]] || { echo "DOMAIN not set; skipping public tests"; exit 0; }
BASE="https://$DOMAIN"
pass=0 fail=0
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

curl_ca=() ossl_ca=()
if [[ "$DOMAIN" == localhost ]]; then
  docker compose -f "$(dirname "$0")/../../docker-compose.yml" exec -T caddy cat /data/caddy/pki/authorities/local/root.crt >"$tmp/ca.crt" ||
    { echo "cannot read Caddy internal CA root"; exit 1; }
  curl_ca=(--cacert "$tmp/ca.crt") ossl_ca=(-CAfile "$tmp/ca.crt" -verify_return_error)
fi

check() { if [[ "$2" == "0" ]]; then printf '  PASS  %s\n' "$1"; pass=$((pass+1)); else printf '  FAIL  %s\n' "$1"; fail=$((fail+1)); fi; }
req() { code="$(curl -sS "${curl_ca[@]}" -o "$tmp/b" -D "$tmp/h" -w '%{http_code}' "$@")"; }
hdr() { grep -i "^$1:" "$tmp/h" | head -1 | cut -d' ' -f2- | tr -d '\r'; }

echo "Public edge test → $BASE"

req -I "$BASE/api/v1/health"
check "HEAD /api/v1/health = 200 (curl -I)" "$([[ $code == 200 ]]; echo $?)"
req "$BASE/api/v1/health"
check "GET /api/v1/health = 200 ok" "$([[ $code == 200 ]] && jq -e '.status=="ok"' "$tmp/b" >/dev/null; echo $?)"
for h in Strict-Transport-Security Content-Security-Policy X-Frame-Options X-Content-Type-Options Referrer-Policy; do
  check "header $h" "$([[ -n $(hdr "$h") ]]; echo $?)"
done
check "HSTS max-age >= 1 year" "$([[ $(hdr Strict-Transport-Security) =~ max-age=([0-9]+) ]] && ((BASH_REMATCH[1] >= 31536000)); echo $?)"
check "no Server header" "$([[ -z $(hdr Server) ]]; echo $?)"
check "no Via header" "$([[ -z $(hdr Via) ]]; echo $?)"
check "request ID does not leak container ID" "$([[ $(hdr X-Request-Id) == sentra-app/* ]]; echo $?)"

req "$BASE/api/v1/version"
check "GET /api/v1/version = 200" "$([[ $code == 200 ]] && jq -e '.version' "$tmp/b" >/dev/null; echo $?)"
check "public /version exposes only {version}" "$(jq -e 'keys==["version"]' "$tmp/b" >/dev/null; echo $?)"
if [[ -n "${EXPECT_VERSION:-}" ]]; then
  check "public version = $EXPECT_VERSION" "$(jq -e --arg v "$EXPECT_VERSION" '.version==$v' "$tmp/b" >/dev/null; echo $?)"
fi

for p in /api/v1/ready /api/v1/does-not-exist /api/v2/health / /admin "/api/v1/health/../ready"; do
  req --path-as-is "$BASE$p"
  check "GET $p rejected at proxy (403)" "$([[ $code == 403 ]] && jq -e '.error.code=="forbidden"' "$tmp/b" >/dev/null; echo $?)"
done
req -X POST "$BASE/api/v1/health"
check "POST /api/v1/health rejected (403)" "$([[ $code == 403 ]]; echo $?)"

req "http://$DOMAIN/api/v1/health"
check "HTTP → HTTPS redirect (308)" "$([[ $code == 308 && $(hdr Location) == "https://$DOMAIN/api/v1/health" ]]; echo $?)"
check "redirect has no Server header" "$([[ -z $(hdr Server) ]]; echo $?)"

issuer="$(echo | openssl s_client "${ossl_ca[@]}" -connect "$DOMAIN:443" -servername "$DOMAIN" 2>/dev/null | openssl x509 -noout -issuer 2>/dev/null)"
end="$(date -d "$(echo | openssl s_client -connect "$DOMAIN:443" -servername "$DOMAIN" 2>/dev/null | openssl x509 -noout -enddate | cut -d= -f2)" +%s)"
if [[ "$DOMAIN" == localhost ]]; then
  # Caddy's internal leaf certificates live 12h and are renewed automatically.
  check "certificate issued by Caddy internal CA (local mode)" "$([[ $issuer == *"Caddy Local Authority"* ]]; echo $?)"
  check "certificate not expired ($(( (end - $(date +%s)) / 60 ))min left)" "$(( end > $(date +%s) ? 0 : 1 ))"
else
  check "certificate issued by Let's Encrypt" "$([[ $issuer == *"Let's Encrypt"* ]]; echo $?)"
  days="$(( (end - $(date +%s)) / 86400 ))"
  check "certificate valid > 14 days (${days}d left)" "$([[ $days -gt 14 ]]; echo $?)"
fi
check "TLS 1.1 refused" "$(! echo | openssl s_client -connect "$DOMAIN:443" -servername "$DOMAIN" -tls1_1 >/dev/null 2>&1; echo $?)"
check "TLS 1.3 accepted" "$(echo | openssl s_client -connect "$DOMAIN:443" -servername "$DOMAIN" -tls1_3 >/dev/null 2>&1; echo $?)"

echo "Result: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
