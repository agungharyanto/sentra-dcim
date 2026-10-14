#!/usr/bin/env bash
# Browser smoke test (Playwright, Chromium) in mcr.microsoft.com/playwright — no Node or browser on the host.
# Usage: test/e2e/run.sh   (BASE_URL defaults to https://$DOMAIN from .env)
# Credentials: same sources as test/api/ui.sh (env SENTRA_WEB_USER/SENTRA_WEB_PASS, ~/.config/sentra/web-auth, prompt)
# plus the application account (SENTRA_APP_USER/SENTRA_APP_PASS, ~/.config/sentra/app-auth, prompt).
set -euo pipefail

# Whole body in main(): bash parses the entire file before running, so editing the script mid-run cannot make it
# re-execute or skip parts (CLAUDE.md rule 13; the 43-vs-41 checks incident of v0.2.5, ISS-035).
main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/../../scripts/lib/source-rev.sh"
source_rev_line "$0"

PLAYWRIGHT_IMAGE="mcr.microsoft.com/playwright:v1.64.0-noble" # multi-arch (amd64 + arm64)
root="$(cd "$(dirname "$0")/../.." && pwd)"
if [[ -z "${BASE_URL:-}" ]]; then
  domain="$(sed -n 's/^DOMAIN=//p' "$root/.env" 2>/dev/null | tail -1)"
  BASE_URL="https://${domain:-localhost}"
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

# Application account (v0.3.1): active, first password already changed. Same sources as test/api/ui.sh.
app_user="${SENTRA_APP_USER:-}" app_pw="${SENTRA_APP_PASS:-}"
app_file="${SENTRA_APP_AUTH_FILE:-$HOME/.config/sentra/app-auth}"
if [[ -z "$app_user" || -z "$app_pw" ]] && [[ -r "$app_file" ]]; then
  IFS= read -r line <"$app_file"; app_user="${line%%:*}" app_pw="${line#*:}"
fi
if [[ -z "$app_user" || -z "$app_pw" ]] && [[ -t 0 ]]; then
  read -rp "app user: " app_user; read -rsp "app password: " app_pw; echo
fi
[[ -n "$app_user" && -n "$app_pw" ]] || { echo "no app account (SENTRA_APP_USER/SENTRA_APP_PASS, see test/e2e/README.md)"; exit 2; }
# MFA (v0.3.3): one code for the single sign-in of global-setup.ts, unless the CI throwaway secret is given.
app_totp="${SENTRA_APP_TOTP:-}"
if [[ -z "$app_totp" && -z "${SENTRA_APP_TOTP_SECRET:-}" && -t 0 ]]; then
  read -rp "kode authenticator (6 digit) untuk $app_user: " app_totp
fi

# Screenshots (design system showcase) land here; CI uploads them as an artifact. Never committed.
artifacts="${E2E_ARTIFACTS:-$root/test/e2e/artifacts}"
mkdir -p "$artifacts"

# Credential probe from the host BEFORE starting Playwright: exactly one authenticated request. Many specs would
# otherwise each send the wrong password, and every 401 with credentials counts towards the fail2ban jail sentra-web
# (10 per 10 min → this server's public IP is banned, ISS-034). Stop at the first 401.
# curl config strings treat \ as an escape: escape backslashes first, then double quotes.
curl_quote() { local v="${1//\\/\\\\}"; printf '%s' "${v//\"/\\\"}"; }
probe_cfg="$(mktemp)"; chmod 600 "$probe_cfg"
printf 'user = "%s"\n' "$(curl_quote "$user:$pw")" >"$probe_cfg"
insecure=(); [[ "$BASE_URL" == https://localhost* ]] && insecure=(-k)
# The SPA needs only basic_auth (the app login is checked once by global-setup.ts).
code="$(curl -s "${insecure[@]}" -K "$probe_cfg" -o /dev/null -w '%{http_code}' --max-time 10 "${BASE_URL%/}/" || true)"
rm -f "$probe_cfg"
case "$code" in
  200) ;;
  401) echo "STOP: kredensial salah (401) untuk user '$user' — Playwright tidak dijalankan (fail2ban, ISS-034)." >&2; exit 3 ;;
  000) echo "STOP: ${BASE_URL%/} tidak bisa dihubungi — IP ini mungkin di-ban jail sentra-web." >&2; exit 3 ;;
  *)   echo "STOP: probe kredensial mendapat HTTP $code (diharapkan 200)." >&2; exit 3 ;;
esac

# Credentials reach the container through a mode-600 env file, never the process list.
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
umask 077
printf 'SENTRA_WEB_USER=%s\nSENTRA_WEB_PASS=%s\nSENTRA_APP_USER=%s\nSENTRA_APP_PASS=%s\nSENTRA_APP_TOTP=%s\nSENTRA_APP_TOTP_SECRET=%s\nBASE_URL=%s\n' \
  "$user" "$pw" "$app_user" "$app_pw" "$app_totp" "${SENTRA_APP_TOTP_SECRET:-}" "${BASE_URL%/}" >"$tmp/env"

echo "Playwright smoke → ${BASE_URL%/} ($PLAYWRIGHT_IMAGE)"
# Repo mounted read-only; the test package is copied to a scratch dir so npm can install there.
docker run --rm --init --ipc=host --network host --env-file "$tmp/env" \
  --user "$(id -u):$(id -g)" -e HOME=/tmp -e E2E_ARTIFACTS=/artifacts -e E2E_MUTATIONS="${E2E_MUTATIONS:-0}" \
  -v "$root:/repo:ro" -v "$artifacts:/artifacts" "$PLAYWRIGHT_IMAGE" bash -euo pipefail -c '
    mkdir -p /tmp/w/test /tmp/w/web/src/i18n /tmp/w/web/src/test
    cp -r /repo/test/e2e /tmp/w/test/e2e && cp -r /repo/web/src/i18n/locales /tmp/w/web/src/i18n/locales
    cp -r /repo/web/src/test/fixtures /tmp/w/web/src/test/fixtures
    cd /tmp/w/test/e2e && npm ci --no-audit --no-fund --loglevel=error && npx playwright test'
echo "screenshots: $artifacts"
}

main "$@"
