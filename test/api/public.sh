#!/usr/bin/env bash
# Public edge tests (Caddy + Let's Encrypt) against the real domain.
# Usage: DOMAIN=sentra.bajastu.id test/api/public.sh   (defaults to DOMAIN from .env)
# DOMAIN=localhost (local mode, DNS not pointed yet): same proxy/header checks against https://localhost, verified
# with Caddy's internal CA root instead of Let's Encrypt.
set -uo pipefail

# Whole body in main(): bash parses the entire file before running, so editing the script mid-run cannot make it
# re-execute or skip parts (CLAUDE.md rule 13; the 43-vs-41 checks incident of v0.2.5, ISS-035).
main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/../../scripts/lib/source-rev.sh"
source_rev_line "$0"

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
check "exactly one Content-Security-Policy" "$([[ $(grep -ci '^Content-Security-Policy:' "$tmp/h") == 1 ]]; echo $?)"
check "no Via header" "$([[ -z $(hdr Via) ]]; echo $?)"
check "request ID does not leak container ID" "$([[ $(hdr X-Request-Id) == sentra-app/* ]]; echo $?)"

req "$BASE/api/v1/version"
check "GET /api/v1/version = 200" "$([[ $code == 200 ]] && jq -e '.version' "$tmp/b" >/dev/null; echo $?)"
check "public /version exposes only {version}" "$(jq -e 'keys==["version"]' "$tmp/b" >/dev/null; echo $?)"
if [[ -n "${EXPECT_VERSION:-}" ]]; then
  check "public version = $EXPECT_VERSION" "$(jq -e --arg v "$EXPECT_VERSION" '.version==$v' "$tmp/b" >/dev/null; echo $?)"
fi

for p in /api/v1/ready /api/v1/does-not-exist /api/v2/health "/api/v1/health/../ready"; do
  req --path-as-is "$BASE$p"
  check "GET $p rejected at proxy (403)" "$([[ $code == 403 ]] && jq -e '.error.code=="forbidden"' "$tmp/b" >/dev/null; echo $?)"
done
# ---- Without a session (basic_auth removed in v0.3.3, ISS-026, D-066) --------------------------------------------
# Allowlist: the SPA shell (index.html for every UI route) and the assets it loads, health, the short version, and
# reaching auth/login. Everything else — help content and images, OpenAPI, data APIs, MFA steps — needs a session.
nononce() { sed -E 's/(name="csp-nonce" content=")[^"]*/\1/' "$1"; }
req "$BASE/login"
cp "$tmp/b" "$tmp/shell.html"
check "GET /login = 200 SPA shell, no basic_auth challenge" "$([[ $code == 200 && -z $(hdr WWW-Authenticate) && $(hdr Content-Type) == text/html* ]] && grep -q '<div id="root">' "$tmp/shell.html"; echo $?)"
check "SPA shell is small and holds no data (no JSON, < 4 KiB)" "$([[ $(wc -c <"$tmp/shell.html") -lt 4096 ]] && ! grep -q '"version"' "$tmp/shell.html"; echo $?)"
for p in / /dashboard /mfa /change-password /design-system /users /help /help/auth/mfa /account/security /openapi.yaml /api.yaml /.env /.git/config /docs/help/id/auth/mfa.md /assets/; do
  req --path-as-is "$BASE$p"
  check "GET $p without session = SPA shell only (no file, no data)" "$([[ $code == 200 || $code == 404 ]] && { [[ $code == 404 ]] || cmp -s <(nononce "$tmp/b") <(nononce "$tmp/shell.html"); }; echo $?)"
done
assets="$(grep -oE '/assets/[^"]+\.(js|css|svg)' "$tmp/shell.html" | sort -u)"
for a in $assets; do
  req "$BASE$a"
  check "asset $a loaded by the login page = 200" "$([[ $code == 200 ]]; echo $?)"
  if [[ $a == *.js || $a == *.css ]]; then
    check "asset $a has no source map reference" "$(! grep -q 'sourceMappingURL' "$tmp/b"; echo $?)"
    req "$BASE$a.map"
    check "source map $a.map not served (404)" "$([[ $code == 404 ]]; echo $?)"
  fi
done
for p in /api/v1/ui/bootstrap /api/v1/help/index /api/v1/help/pages/dashboard /api/v1/help/pages/auth/mfa /api/v1/help/openapi \
  /api/v1/help/assets/help-mfa-qr-id.png /api/v1/help/assets/mfa-phone-scan.svg /api/v1/auth/me /api/v1/auth/sessions \
  /api/v1/auth/mfa /api/v1/users /api/v1/roles; do
  req "$BASE$p"
  check "GET $p without session = 401 unauthenticated (no Basic challenge)" "$([[ $code == 401 && -z $(hdr WWW-Authenticate) ]] && jq -e '.error.code=="unauthenticated"' "$tmp/b" >/dev/null; echo $?)"
done
for p in /api/v1/auth/mfa/enrol /api/v1/auth/mfa/verify /api/v1/auth/mfa/enrol/confirm /api/v1/auth/mfa/recovery-codes; do
  req -X POST -H "Origin: $BASE" -H 'X-Sentra-CSRF: x' -H 'Content-Type: application/json' -d '{"code":"000000"}' "$BASE$p"
  check "POST $p without session = 401 unauthenticated" "$([[ $code == 401 ]] && jq -e '.error.code=="unauthenticated"' "$tmp/b" >/dev/null; echo $?)"
done
# Login is reachable without a session; CSRF is checked before credentials, so no failed login is counted.
req -X POST -H 'Content-Type: application/json' -d '{"username":"edge.check","password":"x"}' "$BASE/api/v1/auth/login"
check "POST /api/v1/auth/login reachable without session (403 csrf_failed, nothing counted)" "$([[ $code == 403 ]] && jq -e '.error.code=="csrf_failed"' "$tmp/b" >/dev/null; echo $?)"

# Access log file for the fail2ban login jail: grows per request and records the client IP.
root="$(cd "$(dirname "$0")/../.." && pwd)"
logsize() { docker compose -f "$root/docker-compose.yml" exec -T caddy stat -c %s /var/log/caddy/access.log 2>/dev/null || echo -1; }
size0="$(logsize)"
req "$BASE/api/v1/auth/me?logprobe=$RANDOM"
sleep 1
size1="$(logsize)"
check "access.log exists and grows after a request ($size0 → $size1 bytes)" "$(( size0 >= 0 && size1 > size0 ? 0 : 1 ))"
last="$(docker compose -f "$root/docker-compose.yml" exec -T caddy tail -50 /var/log/caddy/access.log)"
check "access.log records method, uri, status and client_ip" "$(grep -F 'logprobe=' <<<"$last" | tail -1 | grep -q '"method":"GET".*"client_ip":"\|"client_ip":".*"method":"GET"' && grep -F 'logprobe=' <<<"$last" | tail -1 | grep -q '"status":401'; echo $?)"

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
}

main "$@"
