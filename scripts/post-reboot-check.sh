#!/usr/bin/env bash
# Post-reboot check (v0.2.9): proves the server came back by itself after a reboot. Root (make post-reboot-check).
# Kernel & uptime, services enabled+active, 4 containers healthy with restart policy, fail2ban jails and the
# DOCKER-USER jump, smoke + public edge tests, and a real fail2ban ban/unban. Exit 0 only if everything passes.
set -uo pipefail

# Whole body in main(): bash parses the entire file before running (CLAUDE.md rule 13).
main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/lib/source-rev.sh"
source_rev_line "$0"
[[ $EUID -eq 0 ]] || { echo "run as root (make post-reboot-check uses sudo)"; exit 1; }
root="$(cd "$(dirname "$0")/.." && pwd)"
pass=0 fail=0
check() { if [[ "$2" == "0" ]]; then printf '  PASS  %s\n' "$1"; pass=$((pass+1)); else printf '  FAIL  %s\n' "$1"; fail=$((fail+1)); fi; }

running="$(uname -r)"
newest="$(find /boot -maxdepth 1 -name 'vmlinuz-*' -printf '%f\n' | sed 's/^vmlinuz-//' | sort -V | tail -1)"
echo "kernel running: $running   newest installed: $newest"
echo "booted: $(uptime -s)   uptime: $(uptime -p)"
check "running the newest installed kernel ($newest)" "$([[ $running == "$newest" ]]; echo $?)"
check "no reboot pending (/var/run/reboot-required absent)" "$([[ ! -e /var/run/reboot-required ]]; echo $?)"
# Kernel 6.17 was the rollback target until v0.3.9; it is retired (scripts/kernel-retire.sh, ISS-036).
check "retired kernel 6.17 has no packages left (ISS-036)" "$(! dpkg -l | awk '/^ii/{print $2}' | grep -q -- '6\.17'; echo $?)"
check "GRUB menu visible 10 s" "$(grep -q 'set timeout_style=menu' /boot/grub/grub.cfg; echo $?)"

for u in docker containerd fail2ban; do
  check "$u enabled + active" "$([[ $(systemctl is-enabled "$u") == enabled && $(systemctl is-active "$u") == active ]]; echo $?)"
done

# Containers come back via restart policy; give them up to 3 minutes after boot.
for _ in $(seq 1 180); do
  [[ "$(docker ps --filter name=sentra- --filter health=healthy -q | wc -l)" -ge 4 ]] && break; sleep 1
done
for s in db redis app caddy; do
  c="sentra-$s-1"
  check "$c healthy, restart unless-stopped" "$([[ $(docker inspect -f '{{.State.Health.Status}} {{.HostConfig.RestartPolicy.Name}}' "$c" 2>/dev/null) == 'healthy unless-stopped' ]]; echo $?)"
done
echo "app version: $(curl -s http://127.0.0.1:8080/api/v1/version | jq -r .version 2>/dev/null)"

for j in sentra-web sshd; do
  check "fail2ban jail $j active" "$(fail2ban-client status "$j" >/dev/null 2>&1; echo $?)"
done
for _ in $(seq 1 30); do iptables -S DOCKER-USER | grep -q -- '-j f2b-sentra-web' && break; sleep 1; done
check "DOCKER-USER jumps to f2b-sentra-web (80,443)" "$(iptables -S DOCKER-USER | grep -q -- '--dports 80,443 -j f2b-sentra-web'; echo $?)"

run() { # name, command... (output indented, PASS/FAIL from exit code)
  local name="$1"; shift
  local out; out="$("$@" 2>&1)"; local rc=$?
  printf '%s\n' "$out" | tail -3 | sed 's/^/        | /'
  check "$name" "$rc"
}
run "smoke (internal listener)" "$root/test/api/smoke.sh"
run "public edge test" "$root/test/api/public.sh"
run "fail2ban ban/unban (real traffic, network namespace)" "$root/scripts/fail2ban-ban-test.sh"

echo "post-reboot check: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
}

main "$@"
