#!/usr/bin/env bash
# API smoke tests against a running stack (through Caddy).
# Usage: BASE_URL=http://127.0.0.1:8080 EXPECT_VERSION=v0.1.3 test/api/smoke.sh
# BASE_URL is the internal listener. No account is used: /version build details (commit, Go, server clock and
# display timezone) need system.version.read since v0.3.2 and are checked by test/api/ui.sh and the Go tests.
set -uo pipefail

# Whole body in main(): bash parses the entire file before running, so editing the script mid-run cannot make it
# re-execute or skip parts (CLAUDE.md rule 13; the 43-vs-41 checks incident of v0.2.5, ISS-035).
main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/../../scripts/lib/source-rev.sh"
source_rev_line "$0"

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
EXPECT_VERSION="${EXPECT_VERSION:-}"
pass=0 fail=0
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

check() { # name, condition-result
  if [[ "$2" == "0" ]]; then printf '  PASS  %s\n' "$1"; pass=$((pass+1)); else printf '  FAIL  %s\n' "$1"; fail=$((fail+1)); fi
}
req() { # method path -> sets code, body file $tmp/b, headers $tmp/h
  code="$(curl -sS -o "$tmp/b" -D "$tmp/h" -w '%{http_code}' -X "$1" "$BASE_URL$2")"
}
hdr() { grep -i "^$1:" "$tmp/h" | head -1 | cut -d' ' -f2- | tr -d '\r'; }

echo "API smoke test → $BASE_URL"

req GET /api/v1/health
check "GET /health = 200" "$([[ $code == 200 ]]; echo $?)"
check "health.status = ok" "$(jq -e '.status=="ok"' "$tmp/b" >/dev/null; echo $?)"
check "Content-Type JSON" "$([[ $(hdr Content-Type) == application/json* ]]; echo $?)"
check "X-Request-Id present" "$([[ -n $(hdr X-Request-Id) ]]; echo $?)"
for h in X-Content-Type-Options X-Frame-Options Content-Security-Policy Referrer-Policy Strict-Transport-Security; do
  check "header $h" "$([[ -n $(hdr "$h") ]]; echo $?)"
done
check "Server header hidden" "$([[ -z $(hdr Server) ]]; echo $?)"

req GET /api/v1/ready
check "GET /ready = 200" "$([[ $code == 200 ]]; echo $?)"
check "ready.status = ready" "$(jq -e '.status=="ready"' "$tmp/b" >/dev/null; echo $?)"
for c in database redis migrations; do
  check "ready.checks.$c = ok" "$(jq -e ".checks.$c.status==\"ok\"" "$tmp/b" >/dev/null; echo $?)"
done

req GET /api/v1/version
check "GET /version = 200" "$([[ $code == 200 ]]; echo $?)"
check "/version without session = {version} only (details need system.version.read)" "$(jq -e 'keys==["version"]' "$tmp/b" >/dev/null; echo $?)"
check "Cache-Control no-store" "$([[ $(hdr Cache-Control) == no-store ]]; echo $?)"
if [[ -n "$EXPECT_VERSION" ]]; then
  check "version = $EXPECT_VERSION" "$(jq -e --arg v "$EXPECT_VERSION" '.version==$v' "$tmp/b" >/dev/null; echo $?)"
fi
echo "  info  version=$(jq -r .version "$tmp/b")"

# Default-deny (v0.3.1): everything except health/ready/version/login needs a session.
for p in /api/v1/ui/bootstrap /api/v1/help/index /api/v1/auth/me /api/v1/auth/sessions /api/v1/users /api/v1/roles; do
  req GET "$p"
  check "GET $p without session = 401 unauthenticated" "$([[ $code == 401 ]] && jq -e '.error.code=="unauthenticated"' "$tmp/b" >/dev/null; echo $?)"
done
# CSRF is checked before credentials, so these never reach the login logic (no audit/rate-limit noise).
code="$(curl -sS -o "$tmp/b" -w '%{http_code}' -X POST -H 'Content-Type: application/json' \
  -d '{"username":"smoke.test","password":"x"}' "$BASE_URL/api/v1/auth/login")"
check "POST /auth/login without CSRF header = 403 csrf_failed" "$([[ $code == 403 ]] && jq -e '.error.code=="csrf_failed"' "$tmp/b" >/dev/null; echo $?)"
code="$(curl -sS -o "$tmp/b" -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H 'Origin: https://evil.example' \
  -H 'X-Sentra-CSRF: x' -d '{"username":"smoke.test","password":"x"}' "$BASE_URL/api/v1/auth/login")"
check "POST /auth/login from foreign Origin = 403 csrf_failed" "$([[ $code == 403 ]] && jq -e '.error.code=="csrf_failed"' "$tmp/b" >/dev/null; echo $?)"

req GET /api/v1/does-not-exist
check "unknown path = 404 JSON error" "$([[ $code == 404 ]] && jq -e '.error.code=="not_found" and (.error.request_id|length>0)' "$tmp/b" >/dev/null; echo $?)"

req POST /api/v1/health
check "POST /health = 405" "$([[ $code == 405 ]]; echo $?)"

# Spoofed client IP headers must not bypass rate limiting.
codes="$(for _ in $(seq 1 80); do curl -s -o /dev/null -w '%{http_code}\n' -H "X-Real-IP: 10.9.9.$RANDOM" -H "True-Client-IP: 1.2.3.$RANDOM" -H "X-Forwarded-For: 5.6.7.$RANDOM" "$BASE_URL/api/v1/health"; done | sort | uniq -c)"
check "rate limit 429 (spoofed IP headers ignored)" "$(grep -q ' 429$' <<<"$codes"; echo $?)"
sleep 3

echo "Result: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
}

main "$@"
