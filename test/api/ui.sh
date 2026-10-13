#!/usr/bin/env bash
# Web UI edge tests (v0.2.1): temporary basic_auth (ISS-026), SPA fallback, security & cache headers, bootstrap API.
# Usage: test/api/ui.sh            (DOMAIN from .env; BASE_URL overrides, e.g. https://localhost)
# Credentials for the authenticated checks, never stored in the repo — first match wins:
#   1. SENTRA_WEB_USER + SENTRA_WEB_PASS environment variables
#   2. file $SENTRA_WEB_AUTH_FILE (default ~/.config/sentra/web-auth, mode 600, one line "user:password")
#   3. interactive prompt (TTY only)
set -uo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
if [[ -z "${BASE_URL:-}" ]]; then
  domain="$(sed -n 's/^DOMAIN=//p' "$root/.env" 2>/dev/null | tail -1)"
  BASE_URL="https://${domain:-localhost}"
fi
BASE="${BASE_URL%/}"
pass=0 fail=0
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

curl_ca=()
if [[ "$BASE" == https://localhost* ]]; then
  docker compose -f "$root/docker-compose.yml" exec -T caddy cat /data/caddy/pki/authorities/local/root.crt >"$tmp/ca.crt" ||
    { echo "cannot read Caddy internal CA root"; exit 1; }
  curl_ca=(--cacert "$tmp/ca.crt")
fi

user="${SENTRA_WEB_USER:-}" pw="${SENTRA_WEB_PASS:-}"
auth_file="${SENTRA_WEB_AUTH_FILE:-$HOME/.config/sentra/web-auth}"
if [[ -z "$user" || -z "$pw" ]] && [[ -r "$auth_file" ]]; then
  IFS= read -r line <"$auth_file"; user="${line%%:*}" pw="${line#*:}"
fi
if [[ -z "$user" || -z "$pw" ]] && [[ -t 0 ]]; then
  read -rp "basic_auth user: " user; read -rsp "basic_auth password: " pw; echo
fi
[[ -n "$user" && -n "$pw" ]] || { echo "no basic_auth credentials (see header of $0)"; exit 2; }
# Credentials go through a curl config file (mode 600), never the process list.
umask 077; printf 'user = "%s:%s"\n' "${user//\"/\\\"}" "${pw//\"/\\\"}" >"$tmp/auth.cfg"

check() { if [[ "$2" == "0" ]]; then printf '  PASS  %s\n' "$1"; pass=$((pass+1)); else printf '  FAIL  %s\n' "$1"; fail=$((fail+1)); fi; }
req() { code="$(curl -sS "${curl_ca[@]}" -o "$tmp/b" -D "$tmp/h" -w '%{http_code}' "$@")"; }
areq() { req -K "$tmp/auth.cfg" "$@"; }
hdr() { grep -i "^$1:" "$tmp/h" | head -1 | cut -d' ' -f2- | tr -d '\r'; }

echo "UI edge test → $BASE"

# Public endpoints stay open.
for p in /api/v1/health /api/v1/version; do
  req "$BASE$p"
  check "GET $p without auth = 200" "$([[ $code == 200 ]]; echo $?)"
done

# Without (or with wrong) credentials: 401 + Basic challenge.
for p in / /index.html /some/deep/link /api/v1/ui/bootstrap /api/v1/help/index /api/v1/help/openapi; do
  req "$BASE$p"
  check "GET $p without auth = 401" "$([[ $code == 401 && $(hdr WWW-Authenticate) == Basic* ]]; echo $?)"
done
req -u "$user:wrong-$RANDOM" "$BASE/"
check "GET / with wrong password = 401" "$([[ $code == 401 ]]; echo $?)"

# Other API paths stay rejected, even with credentials.
for p in /api/v1/ready /api/v1/ui/does-not-exist /api/v2/ui/bootstrap; do
  areq "$BASE$p"
  check "GET $p with auth still 403" "$([[ $code == 403 ]] && jq -e '.error.code=="forbidden"' "$tmp/b" >/dev/null; echo $?)"
done

# SPA with credentials.
areq "$BASE/"
check "GET / with auth = 200 html" "$([[ $code == 200 && $(hdr Content-Type) == text/html* ]] && grep -q '<div id="root">' "$tmp/b"; echo $?)"
cp "$tmp/b" "$tmp/index.html"
check "index.html has no inline <script>" "$(! grep -Eq '<script( type="module")?( crossorigin)?>' "$tmp/index.html"; echo $?)"
check "index.html Cache-Control no-cache" "$([[ $(hdr Cache-Control) == no-cache ]]; echo $?)"
csp="$(hdr Content-Security-Policy)"
check "CSP script-src exactly 'self' (no unsafe-inline/eval)" "$([[ $csp == *"script-src 'self';"* && $csp != *unsafe-eval* ]]; echo $?)"
check "CSP 'unsafe-inline' only in style-src (ADR-002)" "$([[ $(grep -o "[a-z-]*-src[^;]*'unsafe-inline'" <<<"$csp") == "style-src 'self' 'unsafe-inline'" ]]; echo $?)"
check "CSP frame-ancestors 'none'" "$([[ $csp == *"frame-ancestors 'none'"* ]]; echo $?)"
check "CSP object-src 'none', base-uri 'none'" "$([[ $csp == *"object-src 'none'"* && $csp == *"base-uri 'none'"* ]]; echo $?)"
check "X-Content-Type-Options nosniff" "$([[ $(hdr X-Content-Type-Options) == nosniff ]]; echo $?)"
check "Referrer-Policy set" "$([[ -n $(hdr Referrer-Policy) ]]; echo $?)"
check "HSTS set" "$([[ $(hdr Strict-Transport-Security) == max-age=* ]]; echo $?)"
check "no Server header" "$([[ -z $(hdr Server) ]]; echo $?)"
check "exactly one Content-Security-Policy" "$([[ $(grep -ci '^Content-Security-Policy:' "$tmp/h") == 1 ]]; echo $?)"

areq "$BASE/apa-saja/deep/link?x=1"
check "deep link /apa-saja/... returns index.html" "$([[ $code == 200 ]] && cmp -s "$tmp/b" "$tmp/index.html"; echo $?)"

asset="$(grep -oE '/assets/[^"]+\.js' "$tmp/index.html" | head -1)"
areq "$BASE$asset"
check "hashed asset $asset = 200 immutable" "$([[ $code == 200 && $(hdr Cache-Control) == *immutable* ]]; echo $?)"
check "asset Content-Type javascript" "$([[ $(hdr Content-Type) == text/javascript* || $(hdr Content-Type) == application/javascript* ]]; echo $?)"
areq "$BASE/assets/missing-0000.js"
check "missing asset = 404 (no SPA fallback, not immutable)" "$([[ $code == 404 && $(hdr Cache-Control) != *immutable* ]]; echo $?)"

areq -X POST "$BASE/"
check "POST / rejected (405)" "$([[ $code == 405 ]]; echo $?)"

# Bootstrap API.
areq "$BASE/api/v1/ui/bootstrap"
check "GET /api/v1/ui/bootstrap with auth = 200" "$([[ $code == 200 ]]; echo $?)"
check "bootstrap fields" "$(jq -e 'keys==["app_name","default_locale","menu_groups","menus","supported_locales","timezone","version"]' "$tmp/b" >/dev/null; echo $?)"
check "bootstrap locale & timezone valid" "$(jq -e '(.default_locale as $l | .supported_locales|index($l))!=null and (.timezone|length>0)' "$tmp/b" >/dev/null; echo $?)"
check "bootstrap menus: no labels, valid status, known group" "$(jq -e '(.menus|length>0) and all(.menus[]; (keys==["group","icon","key","order","path","status"]) and (.status|IN("available","coming_soon","not_connected"))) and ([.menu_groups[].key] as $g | all(.menus[]; .group as $x | $g|index($x) != null))' "$tmp/b" >/dev/null; echo $?)"
check "bootstrap Cache-Control no-store" "$([[ $(hdr Cache-Control) == no-store ]]; echo $?)"
req "$BASE/api/v1/version"
ver="$(jq -r .version "$tmp/b")"
areq "$BASE/api/v1/ui/bootstrap"
check "bootstrap version = public /version ($ver)" "$(jq -e --arg v "$ver" '.version==$v' "$tmp/b" >/dev/null; echo $?)"

# Help Center API (v0.2.4).
areq "$BASE/api/v1/help/index?locale=en"
check "GET /api/v1/help/index with auth = 200, en, pages" "$([[ $code == 200 ]] && jq -e '.locale=="en" and (.pages|length>0) and all(.pages[]; has("markdown")|not)' "$tmp/b" >/dev/null; echo $?)"
areq "$BASE/api/v1/help/pages/dashboard?locale=id"
check "GET /api/v1/help/pages/dashboard = 200 markdown" "$([[ $code == 200 ]] && jq -e '.slug=="dashboard" and (.markdown|length>0)' "$tmp/b" >/dev/null; echo $?)"
areq --path-as-is "$BASE/api/v1/help/pages/..%2f..%2fREADME"
# Caddy cleans the path before matching (…/pages/../../README → /api/v1/README → 403); the app itself answers 404.
check "help page traversal rejected (403 at proxy or 404)" "$([[ $code == 403 || $code == 404 ]]; echo $?)"
areq "$BASE/api/v1/help/index?locale=fr"
check "help unknown locale → 400 invalid_locale" "$([[ $code == 400 ]] && jq -e '.error.code=="invalid_locale"' "$tmp/b" >/dev/null; echo $?)"
areq "$BASE/api/v1/help/openapi"
check "GET /api/v1/help/openapi = 200 JSON spec" "$([[ $code == 200 ]] && jq -e '.openapi and .paths["/api/v1/help/index"]' "$tmp/b" >/dev/null; echo $?)"
areq -X POST "$BASE/api/v1/help/index"
check "POST /api/v1/help/index rejected (403)" "$([[ $code == 403 ]]; echo $?)"

echo "UI edge: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
