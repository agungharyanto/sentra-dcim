#!/usr/bin/env bash
# Recovery drill (v0.2.9): proves a new server can be built from GitHub alone. Clones the repository fresh into a
# temporary directory, generates a throwaway .env (random secrets, no domain, loopback ports 28xxx), builds every
# image (app, web, caddy) as compose project "sentradrill", waits for 4 healthy containers, runs migrations and the
# smoke test, checks the web UI is served, then removes containers, volumes, images and the clone.
#   ./scripts/recovery-drill.sh                 (DRILL_REF=main by default; any branch/tag on GitHub)
#   DRILL_NO_CACHE=1 ./scripts/recovery-drill.sh   build with --no-cache --pull (like a brand-new server)
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
gen() { openssl rand -hex 16; }
pass=0 fail=0
check() { if [[ "$2" == "0" ]]; then printf '  PASS  %s\n' "$1"; pass=$((pass+1)); else printf '  FAIL  %s\n' "$1"; fail=$((fail+1)); fi; }

cleanup() {
  if [[ -d "$dir/repo" ]]; then
    (cd "$dir/repo" && docker compose -p "$project" down -v --remove-orphans >/dev/null 2>&1) || true
  fi
  docker images --format '{{.Repository}}:{{.Tag}}' | grep -E "^sentra-(app|web|caddy):drill-" | xargs -r docker rmi >/dev/null 2>&1 || true
  rm -rf "$dir"
}
trap cleanup EXIT

echo "recovery drill: $remote ($ref) → $dir, project $project"
git clone -q --depth 1 --branch "$ref" "$remote" "$dir/repo"
cd "$dir/repo"
sha="$(git rev-parse --short HEAD)"
check "fresh clone of $ref ($sha) has no .env and no secrets tracked" "$([[ ! -e .env ]] && ! git ls-files | grep -qE '(^|/)\.env$|\.pem$|\.key$'; echo $?)"

# Throwaway configuration: random secrets, no public domain, loopback ports that do not collide with production.
pw="drill-$(gen)"
hash="$(docker run --rm caddy:2.11.7-alpine caddy hash-password --bcrypt-cost 10 --plaintext "$pw")"
umask 077
sed -e "s/^POSTGRES_PASSWORD=CHANGE_ME/POSTGRES_PASSWORD=$(gen)/" -e "s/^REDIS_PASSWORD=CHANGE_ME/REDIS_PASSWORD=$(gen)/" \
    -e "s/^DOMAIN=.*/DOMAIN=/" -e "s/^ACME_EMAIL=.*/ACME_EMAIL=/" \
    -e "s/^CADDY_PUBLIC_HTTP_BIND=.*/CADDY_PUBLIC_HTTP_BIND=127.0.0.1:28080/" \
    -e "s/^CADDY_PUBLIC_HTTPS_BIND=.*/CADDY_PUBLIC_HTTPS_BIND=127.0.0.1:28443/" \
    -e "s/^CADDY_HTTP_BIND=.*/CADDY_HTTP_BIND=127.0.0.1:28081/" \
    -e "s/^POSTGRES_HOST_PORT=.*/POSTGRES_HOST_PORT=25432/" -e "s/^REDIS_HOST_PORT=.*/REDIS_HOST_PORT=26379/" \
    -e "s/^WEB_BASIC_AUTH_USER=.*/WEB_BASIC_AUTH_USER=drill/" -e "s|^WEB_BASIC_AUTH_HASH=.*|WEB_BASIC_AUTH_HASH='$hash'|" \
    .env.example > .env
umask 022

BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
export VERSION="drill-$sha" COMMIT="$sha" BUILD_DATE
start=$(date +%s)
build_opts=()
[[ -n "${DRILL_NO_CACHE:-}" ]] && build_opts=(--no-cache --pull)
echo "  info  build options: ${build_opts[*]:-(Docker layer cache allowed)}"
check "build all images (app, web, caddy) from the clone" "$(docker compose -p "$project" build "${build_opts[@]}" app caddy >"$dir/build.log" 2>&1; echo $?)"
check "4 containers healthy" "$(docker compose -p "$project" up -d --wait >"$dir/up.log" 2>&1; echo $?)"
echo "  info  build + start: $(( $(date +%s) - start )) s"
check "migrations up" "$(docker compose -p "$project" exec -T app /sentra migrate up >/dev/null 2>&1; echo $?)"
check "smoke test against the drill stack" "$(BASE_URL=http://127.0.0.1:28081 EXPECT_VERSION="drill-$sha" ./test/api/smoke.sh >"$dir/smoke.log" 2>&1; echo $?)"
tail -1 "$dir/smoke.log" | sed 's/^/        | /'
check "web UI behind basic_auth (401 without, 200 with credentials)" "$(
  [[ $(curl -sk -o /dev/null -w '%{http_code}' https://localhost:28443/) == 401 &&
     $(curl -sk -u "drill:$pw" -o /dev/null -w '%{http_code}' https://localhost:28443/dashboard) == 200 ]]; echo $?)"
check "help API serves embedded pages" "$(curl -sf http://127.0.0.1:28081/api/v1/help/index | jq -e '.pages|length>0' >/dev/null; echo $?)"
check "production stack untouched (project sentra still 4 healthy)" "$([[ $(docker ps --filter label=com.docker.compose.project=sentra --filter health=healthy -q | wc -l) -eq 4 ]]; echo $?)"
if [[ $fail -ne 0 ]]; then
  # Keep the logs (the clone directory is removed on exit) and show their tails.
  keep="${DRILL_LOG_DIR:-${TMPDIR:-/tmp}}/sentra-drill-logs-$(date +%Y%m%dT%H%M%S)"
  if mkdir -p "$keep"; then cp "$dir"/*.log "$keep"/ 2>/dev/null || true; fi
  echo "--- logs kept in $keep"
  for f in "$dir/build.log" "$dir/up.log"; do echo "--- tail $(basename "$f"):"; tail -n 25 "$f" 2>/dev/null; done
fi

echo "recovery drill: $pass passed, $fail failed (cleanup: containers, volumes, images, clone)"
[[ $fail -eq 0 ]]
}

main "$@"
