#!/usr/bin/env bash
# Full regression + Definition of Done checks for sprint closing (.9) releases.
# shellcheck disable=SC2016 # single-quoted snippets are evaluated by `bash -c` on purpose
# Usage: scripts/full-regression.sh [expected-version]    (run from a clean checkout with the stack up)
#        expected-version defaults to `git describe --tags --always --dirty` of HEAD.
# Prints a PASS/FAIL report; exit code 0 only when every check passes.
set -uo pipefail

# Whole body in main(): bash parses the entire file before running, so editing the script mid-run cannot make it
# re-execute or skip parts (CLAUDE.md rule 13; the 43-vs-41 checks incident of v0.2.5, ISS-035).
main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/lib/source-rev.sh"
source_rev_line "$0"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"
EXPECT_VERSION="${1:-$(git describe --tags --always --dirty)}"
REGRESS_DB="sentra_regress"
pass=0 fail=0 failed=()
out="$(mktemp)"; trap 'rm -f "$out"' EXIT

set -a
# shellcheck source=/dev/null
source .env
set +a

# Backups belong to the app user (owner of .env, `sentra`); an admin user (e.g. `ubuntu`, see
# ops/bootstrap-new-server.sh --admin-user) runs the regression and executes backup/restore as that user.
APP_USER="$(stat -L -c %U .env)"   # -L: in a separate worktree .env is a symlink owned by the admin user
as_app() { if [[ "$(id -un)" == "$APP_USER" ]]; then "$@"; else sudo -u "$APP_USER" -H "$@"; fi; }
export APP_USER
export -f as_app

section() { printf '\n## %s\n' "$*"; }
check() { # name, command...
  local name="$1"; shift
  if "$@" >"$out" 2>&1; then
    printf '  PASS  %s\n' "$name"; pass=$((pass + 1))
  else
    printf '  FAIL  %s\n' "$name"; fail=$((fail + 1)); failed+=("$name")
    sed 's/^/        | /' "$out" | tail -15
  fi
}
info() { printf '  info  %s\n' "$*"; }
psql_db() { docker compose exec -T db sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" exec psql -X -q -v ON_ERROR_STOP=1 -tA -U "$POSTGRES_USER" "$@"' sh "$@"; }
export -f psql_db
# Migrations up/down run ONLY against the isolated, restored copy $REGRESS_DB — never the production database.
# Hard guard: the target name must be the regression copy, differ from POSTGRES_DB, and the app must report it.
assert_regress_target() {
  if [[ -z "$REGRESS_DB" || "$REGRESS_DB" == "$POSTGRES_DB" || ! "$REGRESS_DB" =~ ^sentra_regress$ ]]; then
    echo "REFUSING: migration target '$REGRESS_DB' is not the isolated regression copy (production DB: '$POSTGRES_DB')" >&2
    exit 99
  fi
}
app_db() {
  assert_regress_target || exit 99
  docker compose exec -T -e DB_NAME="$REGRESS_DB" app /sentra "$@"
}
export -f assert_regress_target app_db
export REGRESS_DB
ready_code() { curl -s -o /dev/null -w '%{http_code}' "http://${CADDY_HTTP_BIND:-127.0.0.1:8080}/api/v1/ready"; }

printf '# Sentra DCIM full regression — %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
printf 'git: %s (%s)  host: %s\n' "$(git describe --tags --always --dirty)" "$(git rev-parse --short HEAD)" "$(hostname)"

section "1. Static analysis & secret scan"
check "gitleaks (history + working tree)" make --no-print-directory gitleaks
check "golangci-lint" golangci-lint run ./...
check "gofmt" bash -c '[[ -z "$(gofmt -l cmd internal migrations test)" ]]'
check "go vet" go vet -tags=integration ./...
check "shellcheck" shellcheck -x ops/*.sh ops/harden/*.sh scripts/*.sh scripts/lib/*.sh scripts/hooks/pre-commit test/api/*.sh test/e2e/*.sh
check "OpenAPI lint (Redocly)" make --no-print-directory openapi-lint
check "sqlc generated code up to date" sqlc diff
check "docker compose config valid" docker compose config -q
check "Caddyfile valid (production-like + empty env)" make --no-print-directory caddy-validate
check "compose renders Caddy defaults for empty DOMAIN/ACME_EMAIL" bash -c \
  "DOMAIN= ACME_EMAIL= docker compose config --format json | jq -e '.services.caddy.environment | .DOMAIN==\"localhost\" and .ACME_EMAIL==\"admin@localhost\"' >/dev/null"
check "actionlint (CI workflow, rhysd/actionlint:1.7.12)" docker run --rm -v "$ROOT:/r:ro" -w /r rhysd/actionlint:1.7.12 .github/workflows/ci.yml
check "govulncheck (reachable Go vulnerabilities)" make --no-print-directory vulncheck
check "web: supply:check, lint, typecheck, i18n:check, Vitest (Docker)" make --no-print-directory web-check
check "fail2ban filter (fail2ban-regex, synthetic samples)" make --no-print-directory fail2ban-test
check "no .env / keys tracked by git" bash -c '! git ls-files | grep -E "(^|/)\.env$|\.pem$|\.key$|credentials"'
check "pre-commit hook active" bash -c '[[ "$(git config core.hooksPath)" == "scripts/hooks" ]]'

section "2. Unit tests"
check "go test -race (unit)" make --no-print-directory test
info "coverage: $(go tool cover -func=coverage.out 2>/dev/null | awk '/^total:/{print $3}')"
rm -f coverage.out

section "3. Integration tests (TimescaleDB + Redis)"
check "go test -tags=integration" make --no-print-directory test-integration

section "4. Containers"
for s in db redis app caddy; do
  c="sentra-$s-1"
  check "$s healthy" bash -c "[[ \"\$(docker inspect -f '{{.State.Health.Status}}' $c)\" == healthy ]]"
  check "$s non-root user" bash -c "u=\$(docker inspect -f '{{.Config.User}}' $c); [[ -n \$u && \$u != 0* && \$u != root* ]]"
  check "$s read-only rootfs + no-new-privileges + cap_drop ALL" bash -c \
    "[[ \$(docker inspect -f '{{.HostConfig.ReadonlyRootfs}} {{.HostConfig.SecurityOpt}} {{.HostConfig.CapDrop}}' $c) == 'true [no-new-privileges:true] [ALL]' ]]"
  check "$s restart policy unless-stopped + memory limit" bash -c \
    "[[ \$(docker inspect -f '{{.HostConfig.RestartPolicy.Name}}' $c) == unless-stopped && \$(docker inspect -f '{{.HostConfig.Memory}}' $c) -gt 0 ]]"
  check "$s not privileged, no docker.sock mount" bash -c \
    "[[ \$(docker inspect -f '{{.HostConfig.Privileged}}' $c) == false ]] && ! docker inspect -f '{{range .Mounts}}{{.Source}} {{end}}' $c | grep -q docker.sock"
done
check "published ports bound to 127.0.0.1, except Caddy 80/443" bash -c \
  '! docker ps --format "{{.Names}} {{.Ports}}" | awk "{n=\$1; \$1=\"\"; gsub(/, /,\"\n\"n\" \"); print n\" \"\$0}" | grep -- "->" | grep -vE "^[^ ]+ +127\.0\.0\.1:" | grep -vE "^sentra-caddy-1 +(0\.0\.0\.0|\[::\]):(80|443)->(80|443)/tcp$"'

section "5. API tests"
check "running /api/v1/version == git describe HEAD ($EXPECT_VERSION)" bash -c \
  "[[ \$(curl -fsS http://${CADDY_HTTP_BIND:-127.0.0.1:8080}/api/v1/version | jq -r .version) == '$EXPECT_VERSION' ]]"
check "app image built for $EXPECT_VERSION" bash -c \
  "[[ \$(docker inspect -f '{{.Config.Image}}' sentra-app-1) == 'sentra-app:$EXPECT_VERSION' ]]"
check "API smoke test (internal, expect $EXPECT_VERSION)" env EXPECT_VERSION="$EXPECT_VERSION" ./test/api/smoke.sh
if [[ -n "${DOMAIN:-}" ]]; then
  check "public edge test https://$DOMAIN (TLS, headers, proxy allowlist, version only)" \
    env EXPECT_VERSION="$EXPECT_VERSION" ./test/api/public.sh
fi

section "6. Backup & restore (isolated database)"
backup_dir=""
path_file="$(mktemp)"
check "backup" bash -c "as_app ./ops/backup.sh regression > '$path_file'"
backup_dir="$(tail -1 "$path_file" 2>/dev/null)"; rm -f "$path_file"
info "backup: $backup_dir ($(as_app du -sh "$backup_dir" 2>/dev/null | cut -f1))"
check "backup checksums" as_app bash -c "cd '$backup_dir' && sha256sum --quiet -c SHA256SUMS"
check "backup files mode 0600, dir 0700" as_app bash -c \
  "[[ \$(stat -c %a '$backup_dir') == 700 && -z \$(find '$backup_dir' -type f ! -perm 600) && \$(stat -c %a '${BACKUP_DIR:-/var/backups/sentra}') == 700 ]]"
check "whole backup tree: owner $APP_USER, dirs 0700, files 0600 (incl. .lock)" as_app bash -c \
  "d='${BACKUP_DIR:-/var/backups/sentra}'; [[ -z \$(find \"\$d\" ! -user '$APP_USER') && -z \$(find \"\$d\" -type d ! -perm 700) && -z \$(find \"\$d\" -type f ! -perm 600) ]]"
check "restore into empty database $REGRESS_DB" as_app ./ops/restore.sh "$backup_dir" --target-db "$REGRESS_DB" --yes
check "row count system_info live == restored" bash -c \
  "[[ \$(psql_db -d '$POSTGRES_DB' -c 'select count(*) from system_info') == \$(psql_db -d '$REGRESS_DB' -c 'select count(*) from system_info') ]]"
# The backup records itself in ops_runs after the dump, so live has exactly one more row.
check "row count ops_runs live == restored + 1" bash -c \
  "[[ \$(psql_db -d '$POSTGRES_DB' -c 'select count(*) from ops_runs') == \$((\$(psql_db -d '$REGRESS_DB' -c 'select count(*) from ops_runs') + 1)) ]]"

section "7. Migrations up/down (restored copy, app binary)"
# Hard stop for the whole run if the target is not the regression copy (exit 99 is never caught by `check`).
assert_regress_target
info "migration target: database '$REGRESS_DB' in container sentra-db-1 (production database '$POSTGRES_DB' is never targeted)"
latest="$(find migrations -name "*.up.sql" | wc -l)"
check "migrate goto 0 (all down)" app_db migrate goto 0
check "schema empty after down" bash -c "[[ \$(psql_db -d '$REGRESS_DB' -c \"select count(*) from pg_tables where schemaname='public' and tablename<>'schema_migrations'\") == 0 ]]"
check "migrate up (all $latest)" app_db migrate up
check "version == latest ($latest), not dirty" bash -c "app_db migrate version | grep -qx 'version=$latest dirty=false latest=$latest'"
check "production DB untouched: still at version $latest" bash -c "docker compose exec -T app /sentra migrate version | grep -qx 'version=$latest dirty=false latest=$latest'"
check "migrate down 1 + up (last migration reversible)" bash -c "app_db migrate down 1 && app_db migrate up"
check "drop regression database" psql_db -d postgres -c "DROP DATABASE IF EXISTS $REGRESS_DB WITH (FORCE)"
check "live /ready still 200" bash -c "[[ \$(curl -s -o /dev/null -w '%{http_code}' http://${CADDY_HTTP_BIND:-127.0.0.1:8080}/api/v1/ready) == 200 ]]"

section "8. Permissions"
# 0600; with --admin-user the ACL adds read for that one user only (stat shows the ACL mask as 640).
check ".env owner-only (+ admin ACL read), no group/other access" bash -c \
  '[[ $(stat -L -c %a .env) =~ ^6[04]0$ ]] && getfacl -cp .env | grep -qx "group::---" && getfacl -cp .env | grep -qx "other::---" && [[ -z $(getfacl -cp .env | grep -E "^user:[^:]+:.*w") ]]'
check "ops scripts not world-writable" bash -c '[[ -z $(find ops scripts -type f -perm -o+w) ]]'
check "DB rejects wrong password" bash -c "! docker compose exec -T db sh -c 'PGPASSWORD=wrong psql -h 127.0.0.1 -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -c \"select 1\"'"
check "Redis rejects unauthenticated command" bash -c "docker compose exec -T redis sh -c 'REDISCLI_AUTH= redis-cli get x' 2>&1 | grep -q NOAUTH"
check "app container has no shell (distroless)" bash -c '! docker compose exec -T app sh -c true 2>/dev/null'
check "DB port not reachable from non-loopback IP" bash -c "ip=\$(hostname -I | awk '{print \$1}'); ! timeout 3 bash -c \"</dev/tcp/\$ip/5432\" 2>/dev/null"
check "Redis port not reachable from non-loopback IP" bash -c "ip=\$(hostname -I | awk '{print \$1}'); ! timeout 3 bash -c \"</dev/tcp/\$ip/6379\" 2>/dev/null"
check "Caddy port not reachable from non-loopback IP" bash -c "ip=\$(hostname -I | awk '{print \$1}'); ! timeout 3 bash -c \"</dev/tcp/\$ip/8080\" 2>/dev/null"

section "9. Firewall & host hardening"
ssh_ports="$(sudo sshd -T | awk '$1=="port"{print $2}' | paste -sd' ')"
allowed_ports="$ssh_ports"
[[ "${UFW_OPEN_HTTP:-0}" == "1" ]] && allowed_ports="$ssh_ports 80 443"
info "sshd ports: $ssh_ports; allowed inbound: $allowed_ports"
check "ufw active, default deny incoming" bash -c 'sudo ufw status verbose | grep -q "Status: active" && sudo ufw status verbose | grep -q "Default: deny (incoming)"'
check "ufw allows only: $allowed_ports" bash -c "[[ -z \$(sudo ufw status | awk 'NR>4 && NF{print \$1}' | sed 's#/.*##' | sort -u | grep -vxE '$(echo "$allowed_ports" | tr ' ' '|')') ]]"
if [[ "${UFW_OPEN_HTTP:-0}" == "1" ]]; then
  check "80/443 open in ufw (UFW_OPEN_HTTP=1)" bash -c 'sudo ufw status | grep -qE "^80/tcp +ALLOW" && sudo ufw status | grep -qE "^443/tcp +ALLOW"'
else
  check "80/443 closed in ufw" bash -c '! sudo ufw status | grep -qE "^(80|443)/tcp"'
fi
check "only allowed ports listen on non-loopback addresses" bash -c \
  "[[ -z \$(sudo ss -Htln | awk '{print \$4}' | grep -vE '^(127\.|\[::1\]|\[::ffff:127\.)' | sed -E 's/.*:([0-9]+)\$/\1/' | sort -u | grep -vxE '$(echo "$allowed_ports" | tr ' ' '|')') ]]"
check "fail2ban sshd jail active" sudo fail2ban-client status sshd
check "sysctl hardening applied" bash -c '[[ $(sysctl -n net.ipv4.tcp_syncookies) == 1 && $(sysctl -n kernel.kptr_restrict) == 2 && $(sysctl -n net.ipv4.conf.all.accept_redirects) == 0 ]]'
check "unattended-upgrades enabled" bash -c 'systemctl is-enabled unattended-upgrades && grep -q "Unattended-Upgrade \"1\"" /etc/apt/apt.conf.d/20auto-upgrades'
check "backup timer scheduled" systemctl is-active sentra-backup.timer
check "hardening idempotent (no changes on re-run)" bash -c '! sudo ./ops/harden/all.sh 2>&1 | grep -E "updated|installing"'

section "10. Host boot & fail2ban (v0.2.5, v0.2.9)"
for u in docker containerd fail2ban; do
  check "$u enabled + active at boot" bash -c "[[ \$(systemctl is-enabled $u) == enabled && \$(systemctl is-active $u) == active ]]"
done
check "fail2ban drop-in: After=/Requires=docker.service" bash -c "systemctl show fail2ban -p After -p Requires | grep -c docker.service | grep -qx 2"
check "fail2ban jail sentra-web active" sudo fail2ban-client status sentra-web
check "DOCKER-USER jumps to f2b-sentra-web (80,443)" bash -c "sudo iptables -S DOCKER-USER | grep -q -- '--dports 80,443 -j f2b-sentra-web'"
check "fail2ban real ban/unban (network namespace)" sudo ./scripts/fail2ban-ban-test.sh
check "GRUB menu visible 10 s (serial console)" bash -c "sudo grep -q 'set timeout_style=menu' /boot/grub/grub.cfg"
check "kernel 6.17.0-1020-oracle kept for rollback (manual + not autoremovable)" bash -c \
  "apt-mark showmanual | grep -qx linux-image-6.17.0-1020-oracle && ! apt-get -s autoremove 2>/dev/null | grep -q '^Remv .*6.17.0-1020'"
info "kernel: $(uname -r); reboot pending: $([[ -e /var/run/reboot-required ]] && echo yes || echo no)"

section "11. Repository state"
check "git status clean" bash -c '[[ -z $(git status --porcelain) ]]'
check "HEAD pushed to GitHub (origin has this commit)" bash -c 'git fetch -q origin && git branch -r --contains HEAD | grep -q origin/'
check "mandatory docs present" bash -c 'for f in CLAUDE.md docs/CHANGELOG.md docs/ISSUES.md docs/DEPENDENCIES.md docs/DECISIONS.md docs/ROADMAP.md docs/DISASTER_RECOVERY.md docs/help/README.md api/openapi.yaml; do [[ -s $f ]] || { echo "missing $f"; exit 1; }; done'

printf '\n## Result: %d passed, %d failed\n' "$pass" "$fail"
for f in "${failed[@]}"; do printf '  - FAILED: %s\n' "$f"; done
((fail == 0))
}

main "$@"
