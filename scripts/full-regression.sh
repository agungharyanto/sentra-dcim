#!/usr/bin/env bash
# Full regression + Definition of Done checks for sprint closing (.9) releases.
# shellcheck disable=SC2016 # single-quoted snippets are evaluated by `bash -c` on purpose
# Usage: scripts/full-regression.sh [expected-version]    (run from a clean checkout with the stack up)
# Prints a PASS/FAIL report; exit code 0 only when every check passes.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"
EXPECT_VERSION="${1:-}"
REGRESS_DB="sentra_regress"
pass=0 fail=0 failed=()
out="$(mktemp)"; trap 'rm -f "$out"' EXIT

set -a
# shellcheck source=/dev/null
source .env
set +a

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
app_db() { docker compose exec -T -e DB_NAME="$REGRESS_DB" app /sentra "$@"; }
ready_code() { curl -s -o /dev/null -w '%{http_code}' "http://${CADDY_HTTP_BIND:-127.0.0.1:8080}/api/v1/ready"; }

printf '# Sentra DCIM full regression — %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
printf 'git: %s (%s)  host: %s\n' "$(git describe --tags --always --dirty)" "$(git rev-parse --short HEAD)" "$(hostname)"

section "1. Static analysis & secret scan"
check "gitleaks (history + working tree)" make --no-print-directory gitleaks
check "golangci-lint" golangci-lint run ./...
check "gofmt" bash -c '[[ -z "$(gofmt -l cmd internal migrations test)" ]]'
check "go vet" go vet -tags=integration ./...
check "shellcheck" shellcheck -x ops/*.sh ops/harden/*.sh scripts/*.sh scripts/hooks/pre-commit test/api/*.sh
check "OpenAPI lint (Redocly)" make --no-print-directory openapi-lint
check "sqlc generated code up to date" sqlc diff
check "docker compose config valid" docker compose config -q
if command -v actionlint >/dev/null; then check "actionlint (CI workflow)" actionlint .github/workflows/ci.yml; fi
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
check "API smoke test (internal)${EXPECT_VERSION:+ (expect $EXPECT_VERSION)}" env EXPECT_VERSION="$EXPECT_VERSION" ./test/api/smoke.sh
if [[ -n "${DOMAIN:-}" ]]; then
  check "public edge test https://$DOMAIN (TLS, headers, proxy allowlist)" ./test/api/public.sh
fi

section "6. Backup & restore (isolated database)"
backup_dir=""
check "backup" bash -c './ops/backup.sh regression > /tmp/sentra-regress-backup.path'
backup_dir="$(tail -1 /tmp/sentra-regress-backup.path 2>/dev/null)"; rm -f /tmp/sentra-regress-backup.path
info "backup: $backup_dir ($(du -sh "$backup_dir" 2>/dev/null | cut -f1))"
check "backup checksums" bash -c "cd '$backup_dir' && sha256sum --quiet -c SHA256SUMS"
check "backup files mode 0600, dir 0700" bash -c \
  "[[ \$(stat -c %a '$backup_dir') == 700 && -z \$(find '$backup_dir' -type f ! -perm 600) && \$(stat -c %a '${BACKUP_DIR:-/var/backups/sentra}') == 700 ]]"
check "restore into empty database $REGRESS_DB" ./ops/restore.sh "$backup_dir" --target-db "$REGRESS_DB" --yes
check "row count system_info live == restored" bash -c \
  "[[ \$(psql_db -d '$POSTGRES_DB' -c 'select count(*) from system_info') == \$(psql_db -d '$REGRESS_DB' -c 'select count(*) from system_info') ]]"
# The backup records itself in ops_runs after the dump, so live has exactly one more row.
check "row count ops_runs live == restored + 1" bash -c \
  "[[ \$(psql_db -d '$POSTGRES_DB' -c 'select count(*) from ops_runs') == \$((\$(psql_db -d '$REGRESS_DB' -c 'select count(*) from ops_runs') + 1)) ]]"

section "7. Migrations up/down (restored copy, app binary)"
latest="$(find migrations -name "*.up.sql" | wc -l)"
check "migrate goto 0 (all down)" app_db migrate goto 0
check "schema empty after down" bash -c "[[ \$(psql_db -d '$REGRESS_DB' -c \"select count(*) from pg_tables where schemaname='public' and tablename<>'schema_migrations'\") == 0 ]]"
check "migrate up (all $latest)" app_db migrate up
check "version == latest ($latest), not dirty" bash -c "docker compose exec -T -e DB_NAME='$REGRESS_DB' app /sentra migrate version | grep -qx 'version=$latest dirty=false latest=$latest'"
check "migrate down 1 + up (last migration reversible)" bash -c "docker compose exec -T -e DB_NAME='$REGRESS_DB' app /sentra migrate down 1 && docker compose exec -T -e DB_NAME='$REGRESS_DB' app /sentra migrate up"
check "drop regression database" psql_db -d postgres -c "DROP DATABASE IF EXISTS $REGRESS_DB WITH (FORCE)"
check "live /ready still 200" bash -c "[[ \$(curl -s -o /dev/null -w '%{http_code}' http://${CADDY_HTTP_BIND:-127.0.0.1:8080}/api/v1/ready) == 200 ]]"

section "8. Permissions"
check ".env mode 600" bash -c '[[ $(stat -c %a .env) == 600 ]]'
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

section "10. Repository state"
check "git status clean" bash -c '[[ -z $(git status --porcelain) ]]'
check "mandatory docs present" bash -c 'for f in CLAUDE.md docs/CHANGELOG.md docs/ISSUES.md docs/DEPENDENCIES.md docs/DECISIONS.md docs/ROADMAP.md docs/DISASTER_RECOVERY.md docs/help/README.md api/openapi.yaml; do [[ -s $f ]] || { echo "missing $f"; exit 1; }; done'

printf '\n## Result: %d passed, %d failed\n' "$pass" "$fail"
for f in "${failed[@]}"; do printf '  - FAILED: %s\n' "$f"; done
((fail == 0))
