#!/usr/bin/env bash
# Recovery drill (v0.2.9, hardened v0.3.1 — ISS-037): proves a new server can be built from GitHub alone.
# Clones the repository fresh into a temporary directory, generates a throwaway .env (random secrets, no domain,
# loopback ports 28xxx), builds every image (app, web, caddy) as compose project "sentradrill" with its own image
# names (sentra-drill-*, never touching production images), verifies the version baked into the image equals
# `git describe` of the clone, starts the stack WITHOUT building, runs migrations, the smoke test, UI and login/help checks,
# then removes containers, volumes, images and the clone.
#
# Fails hard: the first failing step stops the drill with a non-zero exit, prints the log tail and keeps the logs.
#   ./scripts/recovery-drill.sh                   DRILL_REF=main by default (any branch/tag on GitHub)
#   DRILL_NO_CACHE=1 ./scripts/recovery-drill.sh  build with --no-cache --pull (like a brand-new server)
set -euo pipefail

main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/lib/source-rev.sh"
source_rev_line "$0"
local_root="$(cd "$(dirname "$0")/.." && pwd)"
remote="$(git -C "$local_root" remote get-url origin)"
ref="${DRILL_REF:-main}"
project=sentradrill
dir="$(mktemp -d /tmp/sentra-drill.XXXXXX)"
logs="${DRILL_LOG_DIR:-${TMPDIR:-/tmp}}/sentra-drill-logs-$(date +%Y%m%dT%H%M%S)"
steps=0
gen() { openssl rand -hex 16; }

compose() { docker compose -p "$project" -f docker-compose.yml -f "$dir/drill.override.yml" "$@"; }

cleanup() {
  local rc=$? problems=0
  if [[ -f "$dir/drill.override.yml" ]]; then
    (cd "$dir/repo" && compose down -v --remove-orphans >"$dir/down.log" 2>&1) ||
      { echo "  WARN  cleanup: compose down failed (see $logs/down.log)"; problems=1; }
  fi
  mapfile -t imgs < <(docker images --format '{{.Repository}}:{{.Tag}}' | awk '/^sentra-drill-(app|web|caddy):/')
  if [[ ${#imgs[@]} -gt 0 ]]; then
    docker rmi "${imgs[@]}" >"$dir/rmi.log" 2>&1 || { echo "  WARN  cleanup: image removal failed"; problems=1; }
  fi
  if [[ $rc -ne 0 || $problems -ne 0 ]]; then
    mkdir -p "$logs"; cp "$dir"/*.log "$logs"/ 2>/dev/null || echo "  WARN  no logs to keep"
    echo "--- logs kept in $logs"
  fi
  rm -rf "$dir"
  local left; left="$(docker ps -aq --filter "label=com.docker.compose.project=$project" | wc -l)"
  [[ "$left" -eq 0 ]] || { echo "  WARN  cleanup: $left drill container(s) left"; problems=1; }
  if [[ $rc -eq 0 && $problems -ne 0 ]]; then exit 1; fi
}
trap cleanup EXIT

# step NAME LOGFILE CMD... — runs CMD; on failure prints the log tail and exits non-zero immediately.
step() {
  local name="$1" log="$2"; shift 2
  if "$@" >"$log" 2>&1; then
    printf '  PASS  %s\n' "$name"; steps=$((steps + 1))
  else
    local rc=$?
    printf '  FAIL  %s (exit %d)\n' "$name" "$rc"
    echo "--- tail $(basename "$log"):"; tail -n 30 "$log"
    exit 1
  fi
}

# drill_migrate starts the database, then migrates and creates the app login role with the owner's credentials.
drill_migrate() {
  compose up -d --wait --no-build db redis &&
    compose run --rm --no-deps ops migrate up &&
    compose run --rm --no-deps ops db sync-app-role
}

# drill_login proves the auth path end to end on the throwaway stack: `sentra user create-admin` (password on
# stdin), login through Caddy (session cookie + CSRF), MFA enrolment (v0.3.3), the forced password
# change, the help API, and
# the Admin role granted by create-admin (user list, /version details; v0.3.2).
# Passwords are random, exist only in this drill and are never printed.
drill_login() {
  local apw npw base=https://localhost:28443 jar="$dir/cookies" code csrf
  apw="admin-$(gen)"; npw="admin-new-$(gen)"
  printf '%s\n' "$apw" | compose exec -T app /sentra user create-admin --username drill.admin
  local c=(curl -sk -b "$jar" -c "$jar" -H "Origin: $base" -H 'Content-Type: application/json'
    -o "$dir/body" -w '%{http_code}')
  code="$("${c[@]}" -H 'X-Sentra-CSRF: login' -d "{\"username\":\"drill.admin\",\"password\":\"$apw\"}" \
    "$base/api/v1/auth/login")"
  echo "login: $code"; [[ $code == 200 ]] || return 1
  csrf="$(awk '$6=="__Host-sentra_csrf"{print $7}' "$jar")"
  code="$("${c[@]}" "$base/api/v1/help/index")"
  echo "help before MFA: $code $(jq -r .error.code "$dir/body")"
  [[ $code == 403 ]] && jq -e '.error.code=="mfa_required"' "$dir/body" >/dev/null || return 1
  code="$("${c[@]}" -X POST -H "X-Sentra-CSRF: $csrf" "$base/api/v1/auth/mfa/enrol")"
  echo "mfa enrol: $code"; [[ $code == 200 ]] || return 1
  local secret totp
  secret="$(jq -r .secret "$dir/body")"; rm -f "$dir/body"
  totp="$(printf '%s' "$secret" | ./test/lib/totp.py)"
  code="$("${c[@]}" -H "X-Sentra-CSRF: $csrf" -d "{\"code\":\"$totp\"}" "$base/api/v1/auth/mfa/enrol/confirm")"
  echo "mfa confirm: $code $(jq '.recovery_codes|length' "$dir/body")"; [[ $code == 200 ]] || return 1
  code="$("${c[@]}" "$base/api/v1/help/index")"
  echo "help before password change: $code $(jq -r .error.code "$dir/body")"
  [[ $code == 403 ]] && jq -e '.error.code=="password_change_required"' "$dir/body" >/dev/null || return 1
  code="$("${c[@]}" -H "X-Sentra-CSRF: $csrf" \
    -d "{\"current_password\":\"$apw\",\"new_password\":\"$npw\"}" "$base/api/v1/auth/password")"
  echo "password change: $code"; [[ $code == 204 ]] || return 1
  code="$("${c[@]}" "$base/api/v1/help/index")"
  echo "help after password change: $code"
  [[ $code == 200 ]] && jq -e '.pages|length>0' "$dir/body" >/dev/null || return 1
  code="$("${c[@]}" "$base/api/v1/users")"
  echo "users as Admin: $code"
  [[ $code == 200 ]] && jq -e '[.users[].roles[]]|index("admin") != null' "$dir/body" >/dev/null || return 1
  code="$("${c[@]}" "$base/api/v1/version")"
  echo "version details as Admin: $code"
  [[ $code == 200 ]] && jq -e '.commit' "$dir/body" >/dev/null || return 1
  code="$("${c[@]}" -X POST -H "X-Sentra-CSRF: $csrf" "$base/api/v1/auth/logout")"
  echo "logout: $code"; [[ $code == 204 ]] || return 1
  code="$("${c[@]}" "$base/api/v1/auth/me")"
  echo "me after logout: $code"; [[ $code == 401 ]]
}

echo "recovery drill: $remote ($ref) → $dir, project $project"
step "clone $ref from GitHub (full history for tags)" "$dir/clone.log" \
  git clone -q --filter=blob:none --branch "$ref" "$remote" "$dir/repo"
cd "$dir/repo"
describe="$(git describe --tags --always)"
sha="$(git rev-parse --short HEAD)"
echo "  info  clone HEAD $sha, git describe $describe"
step "fresh clone has no .env and no secrets tracked" "$dir/secrets.log" \
  bash -c '[[ ! -e .env ]] && ! git ls-files | grep -E "(^|/)\.env$|\.pem$|\.key$"'

# Throwaway configuration: random secrets, no public domain, loopback ports that do not collide with production.
umask 077
sed -e "s/^POSTGRES_PASSWORD=CHANGE_ME/POSTGRES_PASSWORD=$(gen)/" -e "s/^REDIS_PASSWORD=CHANGE_ME/REDIS_PASSWORD=$(gen)/" \
    -e "s|^MFA_ENC_KEY=$|MFA_ENC_KEY=$(openssl rand -base64 32)|" -e "s/^DB_APP_PASSWORD=$/DB_APP_PASSWORD=$(openssl rand -hex 24)/" \
    -e "s/^DOMAIN=.*/DOMAIN=/" -e "s/^ACME_EMAIL=.*/ACME_EMAIL=/" \
    -e "s/^CADDY_PUBLIC_HTTP_BIND=.*/CADDY_PUBLIC_HTTP_BIND=127.0.0.1:28080/" \
    -e "s/^CADDY_PUBLIC_HTTPS_BIND=.*/CADDY_PUBLIC_HTTPS_BIND=127.0.0.1:28443/" \
    -e "s/^CADDY_HTTP_BIND=.*/CADDY_HTTP_BIND=127.0.0.1:28081/" \
    -e "s/^POSTGRES_HOST_PORT=.*/POSTGRES_HOST_PORT=25432/" -e "s/^REDIS_HOST_PORT=.*/REDIS_HOST_PORT=26379/" \
    .env.example > .env
echo "APP_ALLOWED_ORIGINS=https://localhost:28443" >> .env
umask 022
# Own image names: the drill must never overwrite or delete production images (same version tag possible).
cat > "$dir/drill.override.yml" <<YAML
services:
  app:   { image: "sentra-drill-app:\${VERSION}" }
  ops:   { image: "sentra-drill-app:\${VERSION}" }
  web:   { image: "sentra-drill-web:\${VERSION}" }
  caddy: { image: "sentra-drill-caddy:\${VERSION}" }
YAML

BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
export VERSION="$describe" COMMIT="$sha" BUILD_DATE
build_opts=()
[[ -n "${DRILL_NO_CACHE:-}" ]] && build_opts=(--no-cache --pull)
echo "  info  build options: ${build_opts[*]:-(Docker layer cache allowed)}"
start=$(date +%s)
step "build all images (app, web, caddy) from the clone" "$dir/build.log" compose build "${build_opts[@]}" app caddy
echo "  info  build: $(( $(date +%s) - start )) s"
step "image version == git describe ($describe)" "$dir/image.log" \
  bash -c "v=\$(docker run --rm --entrypoint /sentra 'sentra-drill-app:$describe' version | jq -r .version); echo \"image: \$v\"; [[ \"\$v\" == '$describe' ]]"
# Database first, then migrations + the app login role (ops container, owner credentials, ISS-039), then the app.
step "migrations up + app login role (ops container)" "$dir/migrate.log" drill_migrate
step "4 containers healthy (up --no-build)" "$dir/up.log" compose up -d --wait --no-build
echo "  info  build + start: $(( $(date +%s) - start )) s"
step "smoke test against the drill stack (expects $describe)" "$dir/smoke.log" \
  env BASE_URL=http://127.0.0.1:28081 EXPECT_VERSION="$describe" ./test/api/smoke.sh
tail -n 1 "$dir/smoke.log" | sed 's/^/        | /'
step "login page public, data API needs a session (no basic_auth)" "$dir/ui.log" bash -c \
  "[[ \$(curl -sk -o /dev/null -w '%{http_code}' https://localhost:28443/login) == 200 &&
     \$(curl -sk -o /dev/null -w '%{http_code}' https://localhost:28443/api/v1/ui/bootstrap) == 401 ]]"
step "admin via CLI, login, MFA enrolment, forced password change, help API, Admin role (session)" "$dir/login.log" drill_login
# shellcheck disable=SC2016 # evaluated by the inner bash
step "production stack untouched (project sentra still 4 healthy)" "$dir/prod.log" \
  bash -c '[[ $(docker ps --filter label=com.docker.compose.project=sentra --filter health=healthy -q | wc -l) -eq 4 ]]'

echo "recovery drill: $steps steps passed (cleanup: containers, volumes, images, clone)"
}

main "$@"
