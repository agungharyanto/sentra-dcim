#!/usr/bin/env bash
# Browser smoke test (Playwright, Chromium) in mcr.microsoft.com/playwright — no Node or browser on the host.
# Usage: test/e2e/run.sh   (BASE_URL defaults to https://$DOMAIN from .env)
# Credentials: same sources as test/api/ui.sh (env SENTRA_WEB_USER/SENTRA_WEB_PASS, ~/.config/sentra/web-auth, prompt).
set -euo pipefail

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

# Screenshots (design system showcase) land here; CI uploads them as an artifact. Never committed.
artifacts="${E2E_ARTIFACTS:-$root/test/e2e/.artifacts}"
mkdir -p "$artifacts"

# Credentials reach the container through a mode-600 env file, never the process list.
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
umask 077
printf 'SENTRA_WEB_USER=%s\nSENTRA_WEB_PASS=%s\nBASE_URL=%s\n' "$user" "$pw" "${BASE_URL%/}" >"$tmp/env"

echo "Playwright smoke → ${BASE_URL%/} ($PLAYWRIGHT_IMAGE)"
# Repo mounted read-only; the test package is copied to a scratch dir so npm can install there.
docker run --rm --init --ipc=host --network host --env-file "$tmp/env" \
  --user "$(id -u):$(id -g)" -e HOME=/tmp -e E2E_ARTIFACTS=/artifacts \
  -v "$root:/repo:ro" -v "$artifacts:/artifacts" "$PLAYWRIGHT_IMAGE" bash -euo pipefail -c '
    mkdir -p /tmp/w/test /tmp/w/web/src/i18n
    cp -r /repo/test/e2e /tmp/w/test/e2e && cp -r /repo/web/src/i18n/locales /tmp/w/web/src/i18n/locales
    cd /tmp/w/test/e2e && npm ci --no-audit --no-fund --loglevel=error && npx playwright test'
echo "screenshots: $artifacts"
