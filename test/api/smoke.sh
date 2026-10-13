#!/usr/bin/env bash
# API smoke tests against a running stack (through Caddy).
# Usage: BASE_URL=http://127.0.0.1:8080 EXPECT_VERSION=v0.1.3 EXPECT_TZ=Asia/Jakarta test/api/smoke.sh
# BASE_URL is the internal listener (full /version details). EXPECT_TZ defaults to DISPLAY_TIMEZONE or Asia/Jakarta.
set -uo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
EXPECT_VERSION="${EXPECT_VERSION:-}"
EXPECT_TZ="${EXPECT_TZ:-${DISPLAY_TIMEZONE:-Asia/Jakarta}}"
EXPECT_OFFSET="$(TZ="$EXPECT_TZ" date +%:z)"
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
check "version fields" "$(jq -e '.version and .commit and .go_version and .server_time_utc and .server_time_display' "$tmp/b" >/dev/null; echo $?)"
check "Cache-Control no-store" "$([[ $(hdr Cache-Control) == no-store ]]; echo $?)"
check "display_timezone = $EXPECT_TZ" "$(jq -e --arg tz "$EXPECT_TZ" '.display_timezone==$tz' "$tmp/b" >/dev/null; echo $?)"
check "server_time_display offset $EXPECT_OFFSET" "$(jq -e --arg o "$EXPECT_OFFSET" '.server_time_display|endswith($o)' "$tmp/b" >/dev/null; echo $?)"
check "server_time_utc is UTC (Z)" "$(jq -e '.server_time_utc|endswith("Z")' "$tmp/b" >/dev/null; echo $?)"
if [[ -n "$EXPECT_VERSION" ]]; then
  check "version = $EXPECT_VERSION" "$(jq -e --arg v "$EXPECT_VERSION" '.version==$v' "$tmp/b" >/dev/null; echo $?)"
fi
echo "  info  version=$(jq -r .version "$tmp/b") commit=$(jq -r .commit "$tmp/b")"

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
