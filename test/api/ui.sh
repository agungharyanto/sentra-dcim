#!/usr/bin/env bash
# Web UI edge tests (v0.2.1, v0.3.1): temporary basic_auth (ISS-026), SPA fallback, security & cache headers, app
# login (session cookie + CSRF), bootstrap and help API.
# Usage: test/api/ui.sh            (DOMAIN from .env; BASE_URL overrides, e.g. https://localhost)
# basic_auth credentials, never stored in the repo — first match wins:
#   1. SENTRA_WEB_USER + SENTRA_WEB_PASS environment variables
#   2. file $SENTRA_WEB_AUTH_FILE (default ~/.config/sentra/web-auth, mode 600, one line "user:password")
#   3. interactive prompt (TTY only)
# Application account for the session checks (an active account that already changed its first password), same
# order: SENTRA_APP_USER + SENTRA_APP_PASS, file $SENTRA_APP_AUTH_FILE (default ~/.config/sentra/app-auth), prompt.
# Without it the session checks are skipped (reported as SKIP, the run is then not a complete regression).
set -uo pipefail

# Whole body in main(): bash parses the entire file before running, so editing the script mid-run cannot make it
# re-execute or skip parts (CLAUDE.md rule 13; the 43-vs-41 checks incident of v0.2.5, ISS-035).
main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/../../scripts/lib/source-rev.sh"
source_rev_line "$0"

root="$(cd "$(dirname "$0")/../.." && pwd)"
if [[ -z "${BASE_URL:-}" ]]; then
  domain="$(sed -n 's/^DOMAIN=//p' "$root/.env" 2>/dev/null | tail -1)"
  BASE_URL="https://${domain:-localhost}"
fi
BASE="${BASE_URL%/}"
pass=0 fail=0 skipped=
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

curl_ca=()
if [[ -n "${SENTRA_CA_FILE:-}" ]]; then
  curl_ca=(--cacert "$SENTRA_CA_FILE")   # e.g. the internal CA of a separate test stack
elif [[ "$BASE" == https://localhost* ]]; then
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
# curl config strings treat \ as an escape: escape backslashes first, then double quotes.
curl_quote() { local v="${1//\\/\\\\}"; printf '%s' "${v//\"/\\\"}"; }
# Credentials go through a curl config file (mode 600), never the process list.
umask 077; printf 'user = "%s"\n' "$(curl_quote "$user:$pw")" >"$tmp/auth.cfg"

app_user="${SENTRA_APP_USER:-}" app_pw="${SENTRA_APP_PASS:-}"
app_file="${SENTRA_APP_AUTH_FILE:-$HOME/.config/sentra/app-auth}"
if [[ -z "$app_user" || -z "$app_pw" ]] && [[ -r "$app_file" ]]; then
  IFS= read -r line <"$app_file"; app_user="${line%%:*}" app_pw="${line#*:}"
fi
if [[ -z "$app_user" || -z "$app_pw" ]] && [[ -t 0 ]]; then
  read -rp "app user (Enter = skip session checks): " app_user
  [[ -n "$app_user" ]] && { read -rsp "app password: " app_pw; echo; }
fi

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

# Credential probe FIRST, exactly one authenticated request. Every request with wrong credentials counts towards
# the fail2ban jail sentra-web (10 per 10 min → ban of this server's public IP, ISS-034): stop immediately on 401.
# The probe uses the SPA (basic_auth only); bootstrap/help additionally need an app session since v0.3.1.
areq "$BASE/"
case "$code" in
  200) ;;
  401) echo "STOP: kredensial salah (401) untuk user '$user' — tidak ada percobaan lain dikirim (fail2ban, ISS-034)." >&2
       echo "      Cek login di browser (incognito) dulu; lihat docs/ops/runbooks/fail2ban.md." >&2
       exit 3 ;;
  000) # Distinguish a refused connection (fail2ban REJECT) from TLS/DNS problems.
       rc=0; curl -s "${curl_ca[@]}" -o /dev/null --max-time 5 "$BASE/api/v1/health" || rc=$?
       if [[ $rc == 7 ]]; then
         echo "STOP: koneksi ke $BASE ditolak — IP ini kemungkinan di-ban jail sentra-web (sudo fail2ban-client status sentra-web)." >&2
       else
         echo "STOP: $BASE tidak bisa dihubungi (curl exit $rc: TLS/DNS/timeout), bukan masalah kredensial." >&2
       fi
       exit 3 ;;
  *)   echo "STOP: probe kredensial mendapat HTTP $code (diharapkan 200)." >&2; exit 3 ;;
esac

# Without (or with wrong) credentials: 401 + Basic challenge.
for p in / /index.html /some/deep/link /api/v1/ui/bootstrap /api/v1/help/index /api/v1/help/openapi /api/v1/auth/me /api/v1/users; do
  req "$BASE$p"
  check "GET $p without auth = 401" "$([[ $code == 401 && $(hdr WWW-Authenticate) == Basic* ]]; echo $?)"
done
# The only intentionally wrong login of a run (1 of maxretry 10).
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

# App session API behind basic_auth (v0.3.1). Without a session: the app's own 401, no Basic challenge (the browser
# must not show its login dialog) and never counted by fail2ban.
for p in /api/v1/ui/bootstrap /api/v1/help/index /api/v1/auth/me /api/v1/users; do
  areq "$BASE$p"
  check "GET $p with basic_auth, no session = 401 unauthenticated" "$([[ $code == 401 && -z $(hdr WWW-Authenticate) ]] && jq -e '.error.code=="unauthenticated"' "$tmp/b" >/dev/null; echo $?)"
done
areq -X POST -H 'Content-Type: application/json' -d '{"username":"x","password":"y"}' "$BASE/api/v1/auth/login"
check "POST /auth/login without CSRF header = 403" "$([[ $code == 403 ]] && jq -e '.error.code=="csrf_failed"' "$tmp/b" >/dev/null; echo $?)"
areq -X POST -H 'Content-Type: application/json' -H 'Origin: https://evil.example' -H 'X-Sentra-CSRF: x' \
  -d '{"username":"x","password":"y"}' "$BASE/api/v1/auth/login"
check "POST /auth/login from foreign Origin = 403" "$([[ $code == 403 ]] && jq -e '.error.code=="csrf_failed"' "$tmp/b" >/dev/null; echo $?)"

if [[ -z "$app_user" || -z "$app_pw" ]]; then
  echo "  SKIP  session checks (no app account given; see header of $0)"
  skipped=1
else
  sreq() { areq -b "$tmp/jar" -c "$tmp/jar" -H "Origin: $BASE" "$@"; }
  # Body via a 600 file built from the environment: the password never appears in the process list.
  U="$app_user" P="$app_pw" jq -n '{username: env.U, password: env.P}' >"$tmp/login.json"
  sreq -X POST -H 'Content-Type: application/json' -H 'X-Sentra-CSRF: login' --data-binary @"$tmp/login.json" \
    "$BASE/api/v1/auth/login"
  rm -f "$tmp/login.json"
  # Exactly one app login per run: a wrong password counts towards the account lockout (5 → 15 min).
  case "$code" in
    200) ;;
    401) echo "STOP: login aplikasi gagal (401) untuk '$app_user' — tidak ada percobaan lain (lockout 5x)." >&2; exit 3 ;;
    429) echo "STOP: login aplikasi dibatasi (429, rate limit). Coba lagi setelah $(hdr Retry-After) detik." >&2; exit 3 ;;
    *)   echo "STOP: login aplikasi mendapat HTTP $code." >&2; exit 3 ;;
  esac
  sc="$(grep -i '^set-cookie: __Host-sentra_session=' "$tmp/h" | tr -d '\r')"
  check "session cookie __Host-, Secure, HttpOnly, SameSite=Strict, Path=/" "$([[ $sc == *"; Path=/"* && $sc == *"; HttpOnly"* && $sc == *"; Secure"* && $sc == *"; SameSite=Strict"* && $sc != *Domain=* ]]; echo $?)"
  if jq -e '.user.must_change_password' "$tmp/b" >/dev/null; then
    echo "STOP: akun '$app_user' masih wajib ganti password — ganti dulu di UI, lalu ulangi." >&2; exit 3
  fi
  csrf="$(awk '$6=="__Host-sentra_csrf"{print $7}' "$tmp/jar")"

  sreq "$BASE/api/v1/auth/me"
  check "GET /auth/me with session = 200, own user" "$([[ $code == 200 ]] && jq -e --arg u "$app_user" '(.user.username|ascii_downcase)==($u|ascii_downcase) and .session.current' "$tmp/b" >/dev/null; echo $?)"

  # Bootstrap API.
  sreq "$BASE/api/v1/ui/bootstrap"
  check "GET /api/v1/ui/bootstrap with session = 200" "$([[ $code == 200 ]]; echo $?)"
  check "bootstrap fields" "$(jq -e 'keys==["app_name","default_locale","menu_groups","menus","supported_locales","timezone","version"]' "$tmp/b" >/dev/null; echo $?)"
  check "bootstrap locale & timezone valid" "$(jq -e '(.default_locale as $l | .supported_locales|index($l))!=null and (.timezone|length>0)' "$tmp/b" >/dev/null; echo $?)"
  check "bootstrap menus: no labels, valid status, known group" "$(jq -e '(.menus|length>0) and all(.menus[]; (keys==["group","icon","key","order","path","permission","status"]) and (.status|IN("available","coming_soon","not_connected"))) and ([.menu_groups[].key] as $g | all(.menus[]; .group as $x | $g|index($x) != null))' "$tmp/b" >/dev/null; echo $?)"
  check "bootstrap Cache-Control no-store" "$([[ $(hdr Cache-Control) == no-store ]]; echo $?)"
  bver="$(jq -r .version "$tmp/b")"
  req "$BASE/api/v1/version"
  check "bootstrap version = public /version ($(jq -r .version "$tmp/b"))" "$([[ $(jq -r .version "$tmp/b") == "$bver" ]]; echo $?)"

  # Help Center API (v0.2.4).
  sreq "$BASE/api/v1/help/index?locale=en"
  check "GET /api/v1/help/index with session = 200, en, pages" "$([[ $code == 200 ]] && jq -e '.locale=="en" and (.pages|length>0) and all(.pages[]; has("markdown")|not)' "$tmp/b" >/dev/null; echo $?)"
  sreq "$BASE/api/v1/help/pages/dashboard?locale=id"
  check "GET /api/v1/help/pages/dashboard = 200 markdown" "$([[ $code == 200 ]] && jq -e '.slug=="dashboard" and (.markdown|length>0)' "$tmp/b" >/dev/null; echo $?)"
  sreq --path-as-is "$BASE/api/v1/help/pages/..%2f..%2fREADME"
  # Caddy cleans the path before matching (…/pages/../../README → /api/v1/README → 403); the app itself answers 404.
  check "help page traversal rejected (403 at proxy or 404)" "$([[ $code == 403 || $code == 404 ]]; echo $?)"
  sreq "$BASE/api/v1/help/index?locale=fr"
  check "help unknown locale → 400 invalid_locale" "$([[ $code == 400 ]] && jq -e '.error.code=="invalid_locale"' "$tmp/b" >/dev/null; echo $?)"
  sreq "$BASE/api/v1/help/openapi"
  check "GET /api/v1/help/openapi = 200 JSON spec" "$([[ $code == 200 ]] && jq -e '.openapi and .paths["/api/v1/auth/login"]' "$tmp/b" >/dev/null; echo $?)"
  sreq -X POST "$BASE/api/v1/help/index"
  check "POST /api/v1/help/index rejected (403)" "$([[ $code == 403 ]]; echo $?)"

  # RBAC (v0.3.2): answers follow the permissions of the test account (Admin in CI; any role works).
  sreq "$BASE/api/v1/auth/me"
  perms="$(jq -c '.user.permissions // []' "$tmp/b")"
  has() { jq -e --arg p "$1" 'index($p) != null' <<<"$perms" >/dev/null; }
  echo "  info  account permissions: $perms"
  if has users.read; then exp=200; else exp=403; fi
  sreq "$BASE/api/v1/users?limit=5"
  check "GET /api/v1/users = $exp (users.read)" "$([[ $code == "$exp" ]] && { [[ $exp == 403 ]] || jq -e '.users and (.total >= 1) and all(.users[]; has("password_hash")|not)' "$tmp/b" >/dev/null; }; echo $?)"
  if has roles.read; then exp=200; else exp=403; fi
  sreq "$BASE/api/v1/roles"
  check "GET /api/v1/roles = $exp (roles.read)" "$([[ $code == "$exp" ]] && { [[ $exp == 403 ]] || jq -e '[.roles[].key]|index("admin") != null' "$tmp/b" >/dev/null; }; echo $?)"
  # PATCH passes the proxy and CSRF; an unknown id proves it reached the app without changing anything.
  if has users.manage; then exp=404; else exp=403; fi
  sreq -X PATCH -H "X-Sentra-CSRF: $csrf" -H 'Content-Type: application/json' -d '{}' \
    "$BASE/api/v1/users/00000000-0000-4000-8000-000000000000"
  check "PATCH /api/v1/users/<unknown> = $exp (proxy + CSRF pass, users.manage)" "$([[ $code == "$exp" ]]; echo $?)"
  sreq -X DELETE -H "X-Sentra-CSRF: $csrf" "$BASE/api/v1/users/00000000-0000-4000-8000-000000000000"
  check "DELETE /api/v1/users/* rejected at proxy (403, users are disabled, never deleted)" "$([[ $code == 403 ]] && jq -e '.error.message=="not publicly available"' "$tmp/b" >/dev/null; echo $?)"
  sreq "$BASE/api/v1/version"
  if has system.version.read; then
    check "GET /version with Admin session = build details" "$(jq -e '.commit and .server_time_display' "$tmp/b" >/dev/null; echo $?)"
  else
    check "GET /version without system.version.read = {version} only" "$(jq -e 'keys==["version"]' "$tmp/b" >/dev/null; echo $?)"
  fi

  # Unsafe request with the session but without the CSRF token is refused; with it, logout works.
  sreq -X POST "$BASE/api/v1/auth/logout"
  check "POST /auth/logout without CSRF token = 403" "$([[ $code == 403 ]] && jq -e '.error.code=="csrf_failed"' "$tmp/b" >/dev/null; echo $?)"
  sreq -X POST -H "X-Sentra-CSRF: $csrf" "$BASE/api/v1/auth/logout"
  check "POST /auth/logout with CSRF token = 204" "$([[ $code == 204 ]]; echo $?)"
  sreq "$BASE/api/v1/auth/me"
  check "GET /auth/me after logout = 401" "$([[ $code == 401 ]]; echo $?)"
fi

echo "UI edge: $pass passed, $fail failed${skipped:+, session checks SKIPPED}"
[[ $fail -eq 0 ]]
}

main "$@"
