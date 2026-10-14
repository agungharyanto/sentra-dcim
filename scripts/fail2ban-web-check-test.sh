#!/usr/bin/env bash
# Tests the release fail2ban drift check (D-057) without root and without touching /etc or the real repository:
#   1. scripts/fail2ban-web-check.sh against a temporary install tree (missing / in sync / drift / no DOMAIN);
#   2. scripts/fail2ban-web-install.sh refuses the test overrides;
#   3. scripts/release.sh in a throwaway git repo with a stubbed check: drift before the merge stops the release
#      with nothing merged, tagged or pushed; drift after the redeploy exits 1; in sync releases normally;
#      a .env that is not owner-only (ISS-046) stops it before the merge too.
set -euo pipefail

main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/lib/source-rev.sh"
source_rev_line "$0"

repo="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
pass=0; fail=0

# --- 1. check script --------------------------------------------------------------------------------------------
local fake="$tmp/repo" dest="$tmp/dest"
mkdir -p "$fake/scripts/lib" "$fake/deploy"
cp "$repo/scripts/fail2ban-web-check.sh" "$repo/scripts/fail2ban-web-install.sh" "$fake/scripts/"
cp "$repo/scripts/lib/fail2ban-web.sh" "$repo/scripts/lib/source-rev.sh" "$fake/scripts/lib/"
cp -r "$repo/deploy/fail2ban" "$fake/deploy/"
echo "DOMAIN=sentra.example.go.id" > "$fake/.env"
export F2B_DEST_ROOT="$dest" F2B_LOGPATH=/var/lib/docker/volumes/test_caddy_logs/_data/access.log F2B_ADMIN_IPS=""

run_check "nothing installed → exit 1 + make fail2ban-web" 1 "run: make fail2ban-web"
install_fake
run_check "installed from this repo → exit 0" 0 "OK — jail sentra-web files match"
echo "# local edit" >> "$dest/etc/fail2ban/filter.d/sentra-app-login.conf"
run_check "filter edited on the host → exit 1" 1 "DRIFT .*sentra-app-login.conf"
install_fake
echo "# newer rule" >> "$fake/deploy/fail2ban/filter.d/sentra-app-login.conf"
run_check "filter changed in the repo → exit 1" 1 "DRIFT .*sentra-app-login.conf"
cp "$repo/deploy/fail2ban/filter.d/sentra-app-login.conf" "$fake/deploy/fail2ban/filter.d/"
F2B_LOGPATH=/var/lib/docker/volumes/other_caddy_logs/_data/access.log run_check "jail log path differs → exit 1" 1 "DRIFT .*sentra-web.local"
rm "$dest/etc/systemd/system/fail2ban.service.d/sentra-docker.conf"
run_check "systemd drop-in missing → exit 1" 1 "MISSING .*sentra-docker.conf"
install_fake
# ADMIN_IPS go into ignoreip; a changed list is drift, an invalid entry stops the render.
F2B_ADMIN_IPS="198.51.100.7,203.0.113.0/28" install_fake
if grep -q 'ignoreip  = 127.0.0.1/8 ::1 198.51.100.7 203.0.113.0/28' "$dest/etc/fail2ban/jail.d/sentra-web.local"; then ok "ADMIN_IPS rendered into ignoreip" 0; else ok "ADMIN_IPS rendered into ignoreip" 1; fi
F2B_ADMIN_IPS="198.51.100.7" run_check "ADMIN_IPS changed → exit 1" 1 "DRIFT .*sentra-web.local"
F2B_ADMIN_IPS="not-an-ip" run_check "invalid ADMIN_IPS → exit 2" 2 "ADMIN_IPS: invalid"
install_fake
touch "$dest/etc/fail2ban/filter.d/sentra-caddy-auth.conf"
run_check "legacy basic_auth filter still installed → exit 1" 1 "LEGACY .*sentra-caddy-auth.conf"
rm "$dest/etc/fail2ban/filter.d/sentra-caddy-auth.conf"
mv "$fake/.env" "$fake/.env.off"
run_check "no DOMAIN in .env → exit 2" 2 "DOMAIN in .env"
mv "$fake/.env.off" "$fake/.env"

local out rc=0
out="$("$fake/scripts/fail2ban-web-install.sh" 2>&1)" || rc=$?
expect "installer refuses test overrides" "$rc" 1 "$out" "test overrides of the check script only"
unset F2B_DEST_ROOT F2B_LOGPATH F2B_ADMIN_IPS

# --- 2. release.sh with a stubbed check ------------------------------------------------------------------------
export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.invalid GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.invalid
release_case before 1 "nothing merged yet"
local r=0
[[ -z "$(git -C "$tmp/rel" tag -l v9.9.9)" && -z "$(git -C "$tmp/origin.git" tag -l v9.9.9)" ]] || r=1
[[ "$(git -C "$tmp/rel" rev-list --count main)" == 1 && "$(wc -l < "$tmp/rel.calls")" == 1 ]] || r=1
ok "drift before merge: no merge, no tag, no push, check ran once" $r
release_case after 1 "already tagged and pushed"
r=0; [[ -n "$(git -C "$tmp/origin.git" tag -l v9.9.9)" ]] || r=1
ok "drift after redeploy: tag pushed, exit 1 tells to update the jail" $r
release_case none 0 "released v9.9.9"
r=0; [[ -n "$(git -C "$tmp/origin.git" tag -l v9.9.9)" && -z "$(git -C "$tmp/rel" branch -l 'feat/v9.9.9-*')" && ! -d "$tmp/rel-wt" ]] || r=1
ok "in sync: merged, tagged, pushed, worktree + branch removed" $r
# ISS-041: the release changes the jail. main's files no longer match the installed jail, the branch's do: the
# pre-merge check must use the branch and pass; after the merge main = branch and passes too.
release_case changed 0 "released v9.9.9"
r=0; [[ "$(wc -l < "$tmp/rel.calls")" == 2 && -n "$(git -C "$tmp/origin.git" tag -l v9.9.9)" ]] || r=1
ok "jail-changing release: checked against the branch before the merge, main after" $r
# ISS-046: a .env readable by group/other stops the release before anything is merged.
ENV_MODE=640 release_case none 1 "ISS-046"
r=0; [[ "$(git -C "$tmp/rel" rev-list --count main)" == 1 && -z "$(git -C "$tmp/origin.git" tag -l v9.9.9)" && ! -e "$tmp/rel.calls" ]] || r=1
ok ".env 0640: release refused, nothing merged, tagged or pushed" $r

echo "fail2ban-web-check-test: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
}

ok() { if [[ "$2" -eq 0 ]]; then echo "  PASS $1"; pass=$((pass + 1)); else echo "  FAIL $1"; fail=$((fail + 1)); fi; }

expect() { # name rc want_rc output pattern
  if [[ "$2" -eq "$3" ]] && grep -qE -- "$5" <<<"$4"; then ok "$1" 0; else ok "$1 (exit $2)" 1; printf '    | %s\n' "${4//$'\n'/$'\n'    | }"; fi
}

run_check() { # name want_rc pattern
  local out rc=0
  out="$("$fake/scripts/fail2ban-web-check.sh" 2>&1)" || rc=$?
  expect "$1" "$rc" "$2" "$out" "$3"
}

install_fake() { # what fail2ban-web-install.sh would install, without root
  local r; r="$(mktemp -d)"
  ( die() { echo "$*" >&2; exit 2; }
    # shellcheck source=lib/fail2ban-web.sh
    source "$fake/scripts/lib/fail2ban-web.sh"; f2b_dests; f2b_render "$fake" "$r"
    install -D -m 0644 "$r/sentra-app-login.conf" "$filter_dst"
    install -D -m 0644 "$r/sentra-web.local" "$jail_dst"
    install -D -m 0644 "$r/sentra-docker.conf" "$dropin_dst" )
  rm -rf "$r"
}

# release_case <stub fails: before|after|none> <want exit> <pattern>: fresh origin + main tree + feature worktree.
release_case() {
  rm -rf "$tmp/origin.git" "$tmp/rel" "$tmp/rel-wt" "$tmp/rel.calls"
  git init -q --bare -b main "$tmp/origin.git"
  git init -q -b main "$tmp/rel"
  mkdir -p "$tmp/rel/scripts/lib"
  cp "$repo/scripts/release.sh" "$repo/scripts/env-perm-check.sh" "$tmp/rel/scripts/"
  cp "$repo/scripts/lib/source-rev.sh" "$tmp/rel/scripts/lib/"
  # Stub: records each call; fails on the call named by $1 (1st = before merge, 2nd = after redeploy).
  cat > "$tmp/rel/scripts/fail2ban-web-check.sh" <<STUB
#!/usr/bin/env bash
echo x >> "$tmp/rel.calls"; n=\$(wc -l < "$tmp/rel.calls")
case "$1" in before) [[ \$n -eq 1 ]] && exit 1;; after) [[ \$n -eq 2 ]] && exit 1;; esac
# 'changed': only a check script that knows the new jail (marker added on the feature branch) is in sync.
[[ "$1" == changed ]] && ! grep -qx '# NEW-JAIL' "\$0" && exit 1
exit 0
STUB
  chmod +x "$tmp/rel/scripts/fail2ban-web-check.sh"
  echo .env > "$tmp/rel/.gitignore"
  echo "DOMAIN=" > "$tmp/rel/.env"; chmod "${ENV_MODE:-600}" "$tmp/rel/.env"
  git -C "$tmp/rel" add -A && git -C "$tmp/rel" commit -qm init
  git -C "$tmp/rel" remote add origin "$tmp/origin.git"
  git -C "$tmp/rel" push -q origin main
  git -C "$tmp/rel" worktree add -q -b feat/v9.9.9-test "$tmp/rel-wt" main
  echo change > "$tmp/rel-wt/feature.txt"
  [[ "$1" == changed ]] && echo "# NEW-JAIL" >> "$tmp/rel-wt/scripts/fail2ban-web-check.sh"
  git -C "$tmp/rel-wt" add -A && git -C "$tmp/rel-wt" commit -qm "feat: test"
  local out rc=0
  out="$(cd "$tmp/rel" && ./scripts/release.sh v9.9.9 "Test" 2>&1)" || rc=$?
  expect "release.sh, check fails: $1 → exit $2" "$rc" "$2" "$out" "$3"
}

main "$@"
