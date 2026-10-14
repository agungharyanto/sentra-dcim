#!/usr/bin/env bash
# Web UI edge tests (v0.2.1–v0.3.3): SPA fallback, security & cache headers, CSP nonce, app login (session cookie +
# CSRF + MFA), bootstrap, help and users API. No basic_auth since v0.3.3 (ISS-026); the no-session inventory is in
# test/api/public.sh.
# Usage: test/api/ui.sh            (DOMAIN from .env; BASE_URL overrides, e.g. https://localhost)
# Application account for the session checks (an active account with MFA set up), never stored in the repo — first
# match wins: SENTRA_APP_USER + SENTRA_APP_PASS, file $SENTRA_APP_AUTH_FILE (default ~/.config/sentra/app-auth, mode
# 600, one line "user:password"), prompt. The MFA code is asked for (or SENTRA_APP_TOTP).
# The session checks are required (v0.3.6): without an account, or when sign-in/MFA fails, the run FAILS (exit 2/3)
# and says why — it is never silently skipped.
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
pass=0 fail=0
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

curl_ca=()
if [[ -n "${SENTRA_CA_FILE:-}" ]]; then
  curl_ca=(--cacert "$SENTRA_CA_FILE")   # e.g. the internal CA of a separate test stack
elif [[ "$BASE" == https://localhost* ]]; then
  docker compose -f "$root/docker-compose.yml" exec -T caddy cat /data/caddy/pki/authorities/local/root.crt >"$tmp/ca.crt" ||
    { echo "cannot read Caddy internal CA root"; exit 1; }
  curl_ca=(--cacert "$tmp/ca.crt")
fi

app_user="${SENTRA_APP_USER:-}" app_pw="${SENTRA_APP_PASS:-}" app_src=env
app_file="${SENTRA_APP_AUTH_FILE:-$HOME/.config/sentra/app-auth}"
# Why no account was found, per source (printed only on failure; never the values).
why=()
[[ -n "$app_user" && -n "$app_pw" ]] || why+=("env: SENTRA_APP_USER ${SENTRA_APP_USER:+set}${SENTRA_APP_USER:-not set}, SENTRA_APP_PASS ${SENTRA_APP_PASS:+set}${SENTRA_APP_PASS:-not set}")
if [[ -z "$app_user" || -z "$app_pw" ]]; then
  if [[ ! -e "$app_file" ]]; then
    why+=("file: $app_file does not exist")
  elif [[ ! -r "$app_file" ]]; then
    why+=("file: $app_file not readable by $(id -un)")
  else
    IFS= read -r line <"$app_file" || true
    app_user="${line%%:*}" app_pw="${line#*:}" app_src="file $app_file"
    [[ "$line" == *:* && -n "$app_user" && -n "$app_pw" ]] || { why+=("file: $app_file is not one line \"user:password\""); app_user="" app_pw=""; }
    [[ "$(stat -c %a "$app_file")" == 600 ]] || echo "  WARN  $app_file should be mode 600" >&2
  fi
fi
if [[ -z "$app_user" || -z "$app_pw" ]]; then
  if [[ -t 0 ]]; then
    read -rp "app user: " app_user
    [[ -n "$app_user" ]] && { read -rsp "app password: " app_pw; echo; }
    app_src=prompt
    [[ -n "$app_user" && -n "$app_pw" ]] || why+=("prompt: empty user or password entered")
  else
    why+=("prompt: stdin is not a terminal")
  fi
fi
if [[ -z "$app_user" || -z "$app_pw" ]]; then
  echo "  FAIL  session checks: no application account — the run is NOT a complete regression" >&2
  for w in "${why[@]}"; do echo "        - $w" >&2; done
  echo "UI edge: FAILED before the checks (no app account; see header of $0)" >&2
  exit 2
fi
echo "  info  app account: $app_user (source: $app_src)"

check() { if [[ "$2" == "0" ]]; then printf '  PASS  %s\n' "$1"; pass=$((pass+1)); else printf '  FAIL  %s\n' "$1"; fail=$((fail+1)); fi; }
req() { code="$(curl -sS "${curl_ca[@]}" -o "$tmp/b" -D "$tmp/h" -w '%{http_code}' "$@")"; }
areq() { req "$@"; }   # was: with basic_auth credentials (removed in v0.3.3)
hdr() { grep -i "^$1:" "$tmp/h" | head -1 | cut -d' ' -f2- | tr -d '\r'; }

echo "UI edge test → $BASE"

# Public endpoints stay open.
for p in /api/v1/health /api/v1/version; do
  req "$BASE$p"
  check "GET $p without auth = 200" "$([[ $code == 200 ]]; echo $?)"
done

# Reachability first: a refused connection usually means this IP is banned by the jail sentra-web.
rc=0; curl -s "${curl_ca[@]}" -o /dev/null --max-time 5 "$BASE/api/v1/health" || rc=$?
if [[ $rc != 0 ]]; then
  if [[ $rc == 7 ]]; then
    echo "STOP: koneksi ke $BASE ditolak — IP ini kemungkinan di-ban jail sentra-web (sudo fail2ban-client status sentra-web)." >&2
  else
    echo "STOP: $BASE tidak bisa dihubungi (curl exit $rc: TLS/DNS/timeout)." >&2
  fi
  exit 3
fi

# Other API paths stay rejected at the proxy.
for p in /api/v1/ready /api/v1/ui/does-not-exist /api/v2/ui/bootstrap; do
  areq "$BASE$p"
  check "GET $p rejected at proxy (403)" "$([[ $code == 403 ]] && jq -e '.error.code=="forbidden"' "$tmp/b" >/dev/null; echo $?)"
done

# SPA (public shell).
areq "$BASE/"
check "GET / = 200 html" "$([[ $code == 200 && $(hdr Content-Type) == text/html* ]] && grep -q '<div id="root">' "$tmp/b"; echo $?)"
cp "$tmp/b" "$tmp/index.html"
check "index.html has no inline <script>" "$(! grep -Eq '<script( type="module")?( crossorigin)?>' "$tmp/index.html"; echo $?)"
# index.html carries a per-request CSP nonce (ISS-030): never cached or revalidated.
check "index.html Cache-Control no-store, no ETag/Last-Modified" "$([[ $(hdr Cache-Control) == no-store && -z $(hdr ETag) && -z $(hdr Last-Modified) ]]; echo $?)"
csp="$(hdr Content-Security-Policy)"
check "CSP script-src exactly 'self' (no unsafe-inline/eval)" "$([[ $csp == *"script-src 'self';"* && $csp != *unsafe-eval* ]]; echo $?)"
check "CSP has no 'unsafe-inline' at all (nonce, ISS-030)" "$([[ $csp != *unsafe-inline* ]]; echo $?)"
nonce="$(grep -o "style-src 'self' 'nonce-[A-Za-z0-9+/_=-]\{16,\}'" <<<"$csp" | sed -E "s/.*'nonce-([^']+)'/\1/")"
meta="$(grep -o '<meta name="csp-nonce" content="[^"]*"' "$tmp/index.html" | sed -E 's/.*content="([^"]*)"/\1/')"
check "style-src nonce = <meta name=csp-nonce> of the same response" "$([[ -n $nonce && $nonce == "$meta" ]]; echo $?)"
areq "$BASE/some/deep/link"
check "nonce differs per request (deep link)" "$([[ $(hdr Content-Security-Policy) != *"$nonce"* ]] && grep -q 'name="csp-nonce"' "$tmp/b"; echo $?)"
check "CSP frame-ancestors 'none'" "$([[ $csp == *"frame-ancestors 'none'"* ]]; echo $?)"
check "CSP object-src 'none', base-uri 'none'" "$([[ $csp == *"object-src 'none'"* && $csp == *"base-uri 'none'"* ]]; echo $?)"
check "X-Content-Type-Options nosniff" "$([[ $(hdr X-Content-Type-Options) == nosniff ]]; echo $?)"
check "Referrer-Policy set" "$([[ -n $(hdr Referrer-Policy) ]]; echo $?)"
check "HSTS set" "$([[ $(hdr Strict-Transport-Security) == max-age=* ]]; echo $?)"
check "no Server header" "$([[ -z $(hdr Server) ]]; echo $?)"
check "exactly one Content-Security-Policy" "$([[ $(grep -ci '^Content-Security-Policy:' "$tmp/h") == 1 ]]; echo $?)"

areq "$BASE/apa-saja/deep/link?x=1"
# Same document apart from the per-request CSP nonce (ISS-030).
nononce() { sed -E 's/(name="csp-nonce" content=")[^"]*/\1/' "$1"; }
check "deep link /apa-saja/... returns index.html (nonce aside)" "$([[ $code == 200 ]] && cmp -s <(nononce "$tmp/b") <(nononce "$tmp/index.html"); echo $?)"

asset="$(grep -oE '/assets/[^"]+\.js' "$tmp/index.html" | head -1)"
areq "$BASE$asset"
check "hashed asset $asset = 200 immutable" "$([[ $code == 200 && $(hdr Cache-Control) == *immutable* ]]; echo $?)"
check "asset Content-Type javascript" "$([[ $(hdr Content-Type) == text/javascript* || $(hdr Content-Type) == application/javascript* ]]; echo $?)"
areq "$BASE/assets/missing-0000.js"
check "missing asset = 404 (no SPA fallback, not immutable)" "$([[ $code == 404 && $(hdr Cache-Control) != *immutable* ]]; echo $?)"

areq -X POST "$BASE/"
check "POST / rejected (405)" "$([[ $code == 405 ]]; echo $?)"

# App session API without a session: the app's own 401 (no Basic challenge since basic_auth is gone).
for p in /api/v1/ui/bootstrap /api/v1/help/index /api/v1/auth/me /api/v1/users; do
  areq "$BASE$p"
  check "GET $p without session = 401 unauthenticated" "$([[ $code == 401 && -z $(hdr WWW-Authenticate) ]] && jq -e '.error.code=="unauthenticated"' "$tmp/b" >/dev/null; echo $?)"
done
areq -X POST -H 'Content-Type: application/json' -d '{"username":"x","password":"y"}' "$BASE/api/v1/auth/login"
check "POST /auth/login without CSRF header = 403" "$([[ $code == 403 ]] && jq -e '.error.code=="csrf_failed"' "$tmp/b" >/dev/null; echo $?)"
areq -X POST -H 'Content-Type: application/json' -H 'Origin: https://evil.example' -H 'X-Sentra-CSRF: x' \
  -d '{"username":"x","password":"y"}' "$BASE/api/v1/auth/login"
check "POST /auth/login from foreign Origin = 403" "$([[ $code == 403 ]] && jq -e '.error.code=="csrf_failed"' "$tmp/b" >/dev/null; echo $?)"

{
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
  if ! jq -e '.user.mfa_enabled' "$tmp/b" >/dev/null; then
    echo "STOP: akun '$app_user' belum memasang MFA — masuk sekali di browser dan pasang authenticator, lalu ulangi." >&2; exit 3
  fi
  if jq -e '.user.must_change_password' "$tmp/b" >/dev/null; then
    echo "STOP: akun '$app_user' masih wajib ganti password — ganti dulu di UI, lalu ulangi." >&2; exit 3
  fi
  csrf="$(awk '$6=="__Host-sentra_csrf"{print $7}' "$tmp/jar")"

  # MFA (v0.3.3): the session is mfa_pending until a code is entered.
  sreq "$BASE/api/v1/ui/bootstrap"
  check "GET /api/v1/ui/bootstrap before MFA = 403 mfa_required" "$([[ $code == 403 ]] && jq -e '.error.code=="mfa_required"' "$tmp/b" >/dev/null; echo $?)"
  # Code source: SENTRA_APP_TOTP_SECRET (CI throwaway account only), SENTRA_APP_TOTP (one code), or a prompt. A
  # production account's secret never leaves the user's phone.
  totp="${SENTRA_APP_TOTP:-}"
  if [[ -z "$totp" && -n "${SENTRA_APP_TOTP_SECRET:-}" ]]; then
    # The current 30 s step may already be used by an earlier sign-in (anti-replay): wait for a fresh one.
    sleep "$((30 - $(date +%s) % 30))"
    totp="$(printf '%s' "$SENTRA_APP_TOTP_SECRET" | "$(dirname "$0")/../lib/totp.py")"
  fi
  if [[ -z "$totp" && -t 0 ]]; then read -rp "kode authenticator (6 digit) untuk $app_user: " totp; fi
  [[ -n "$totp" ]] || { echo "STOP: tidak ada kode MFA (SENTRA_APP_TOTP atau terminal interaktif)." >&2; exit 3; }
  C="$totp" jq -n '{code: env.C}' >"$tmp/code.json"
  sreq -X POST -H 'Content-Type: application/json' -H "X-Sentra-CSRF: $csrf" --data-binary @"$tmp/code.json" "$BASE/api/v1/auth/mfa/verify"
  rm -f "$tmp/code.json"
  case "$code" in
    204) ;;
    401) echo "STOP: kode MFA ditolak (salah, kedaluwarsa, atau sudah dipakai — satu kode hanya sah sekali: bila baru" \
           "dipakai test-e2e/login lain, tunggu kode berikutnya). Tidak ada percobaan lain (lockout 5x)." >&2; exit 3 ;;
    *)   echo "STOP: verifikasi MFA mendapat HTTP $code ($(jq -r .error.code "$tmp/b" 2>/dev/null))." >&2; exit 3 ;;
  esac
  check "POST /auth/mfa/verify = 204 (session passed MFA)" 0

  sreq "$BASE/api/v1/auth/me"
  check "GET /auth/me with session = 200, own user, MFA passed" "$([[ $code == 200 ]] && jq -e --arg u "$app_user" '(.user.username|ascii_downcase)==($u|ascii_downcase) and .session.current and .session.mfa_verified' "$tmp/b" >/dev/null; echo $?)"

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
  # Help Center filtered by permission (ISS-032): the API reference and admin pages only with help.admin.
  sreq "$BASE/api/v1/help/openapi"
  if has help.admin; then
    check "GET /api/v1/help/openapi = 200 JSON spec (help.admin)" "$([[ $code == 200 ]] && jq -e '.openapi and .paths["/api/v1/audit/events"]' "$tmp/b" >/dev/null; echo $?)"
  else
    check "GET /api/v1/help/openapi = 404 without help.admin" "$([[ $code == 404 ]]; echo $?)"
  fi
  if has help.admin; then exp=200; else exp=404; fi
  sreq "$BASE/api/v1/help/pages/users/roles"
  check "GET /api/v1/help/pages/users/roles = $exp (audience admin)" "$([[ $code == "$exp" ]]; echo $?)"

  # Audit log (v0.3.4): viewer, integrity, CSV export (an empty future window: no rows, but audited).
  if has audit.read; then exp=200; else exp=403; fi
  sreq "$BASE/api/v1/audit/events?limit=5"
  check "GET /api/v1/audit/events = $exp (audit.read)" "$([[ $code == "$exp" ]] && { [[ $exp == 403 ]] || jq -e '(.events|length >= 1) and all(.events[]; .seq and .hash)' "$tmp/b" >/dev/null; }; echo $?)"
  sreq "$BASE/api/v1/audit/integrity"
  check "GET /api/v1/audit/integrity = $exp (audit.read)" "$([[ $code == "$exp" ]] && { [[ $exp == 403 ]] || jq -e 'has("last") and has("running")' "$tmp/b" >/dev/null; }; echo $?)"
  sreq -X POST -H "X-Sentra-CSRF: $csrf" "$BASE/api/v1/audit/integrity/verify"
  check "POST /api/v1/audit/integrity/verify = $exp, chain intact (audit.read)" "$([[ $code == "$exp" ]] && { [[ $exp == 403 ]] || jq -e '.ok == true' "$tmp/b" >/dev/null; }; echo $?)"
  if has audit.export; then exp=200; else exp=403; fi
  sreq "$BASE/api/v1/audit/events/export.csv?since=2099-01-01T00:00:00Z"
  check "GET /api/v1/audit/events/export.csv = $exp (audit.export)" "$([[ $code == "$exp" ]] && { [[ $exp == 403 ]] || head -1 "$tmp/b" | grep -q '^seq,occurred_at_utc,'; }; echo $?)"

  # Settings – Regional (v0.3.5): effective values in /me, own preferences, system settings with settings.manage.
  sreq "$BASE/api/v1/auth/me"
  check "GET /auth/me carries effective regional values" "$(jq -e '.regional | (.language and .timezone and .time_format and .date_format)' "$tmp/b" >/dev/null; echo $?)"
  sreq "$BASE/api/v1/me/preferences"
  check "GET /api/v1/me/preferences = 200 (every user)" "$([[ $code == 200 ]] && jq -e '.preferences and .effective.timezone and .system.timezone' "$tmp/b" >/dev/null; echo $?)"
  if has settings.manage; then exp=200; else exp=403; fi
  sreq "$BASE/api/v1/settings/regional"
  check "GET /api/v1/settings/regional = $exp (settings.manage)" "$([[ $code == "$exp" ]] && { [[ $exp == 403 ]] || jq -e '[.items[].name] == ["language","timezone","time_format","date_format"]' "$tmp/b" >/dev/null; }; echo $?)"
  # Invalid value: validated before anything is written (no change in production).
  if has settings.manage; then exp=422; else exp=403; fi
  sreq -X PUT -H "X-Sentra-CSRF: $csrf" -H 'Content-Type: application/json' -d '{"values":{"timezone":"Mars/Base"}}' "$BASE/api/v1/settings/regional"
  check "PUT /api/v1/settings/regional unknown zone = $exp (nothing written)" "$([[ $code == "$exp" ]]; echo $?)"

  # Allowed IP (v0.3.6): state with the client IP as the server sees it; the decision endpoint is internal only.
  if has settings.manage; then exp=200; else exp=403; fi
  sreq "$BASE/api/v1/settings/allowed-ips"
  check "GET /api/v1/settings/allowed-ips = $exp (settings.manage)" "$([[ $code == "$exp" ]] && { [[ $exp == 403 ]] || jq -e '(.mode|test("^(off|monitor|enforce)$")) and (.your_ip|length > 0) and (.always_public|index("/api/v1/health") != null)' "$tmp/b" >/dev/null; }; echo $?)"
  sreq "$BASE/api/v1/allowlist/check"
  check "GET /api/v1/allowlist/check not reachable on the public site (403 at proxy)" "$([[ $code == 403 ]] && jq -e '.error.message=="not publicly available"' "$tmp/b" >/dev/null; echo $?)"

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
}

echo "UI edge: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
}

main "$@"
