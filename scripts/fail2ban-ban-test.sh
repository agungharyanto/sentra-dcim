#!/usr/bin/env bash
# Real ban/unban test of the fail2ban jail sentra-web (failed app sign-ins since v0.3.3, D-066). Needs root and the
# installed jail.
#
# A throwaway network namespace with a routed (non-NAT) veth and the documentation address 198.51.100.10 sends failed
# app logins (random unknown usernames: no real account is locked; each attempt leaves an auth.login.failure audit
# row) to this host's private IP:443. Its packets take exactly the path of an Internet client —
# PREROUTING DNAT → FORWARD → DOCKER-USER — with the source address preserved, until it is banned; the test proves
# the connection is then rejected and unbans it. Admin IPs are never involved and are checked not to be banned.
#
# Why not a Docker container: Docker MASQUERADEs traffic from container networks to published ports, so Caddy would
# log (and fail2ban would ban) the bridge gateway (e.g. 172.19.0.1) instead of the tester. Internet clients are not
# masqueraded (the rules only match Docker subnets).
#
#   sudo scripts/fail2ban-ban-test.sh                   full cycle
#   sudo RESTART=docker   scripts/fail2ban-ban-test.sh  ...and restart Docker while banned (ban must survive)
#   sudo RESTART=fail2ban scripts/fail2ban-ban-test.sh  ...and restart fail2ban while banned
# Note: RESTART=docker restarts every container (short outage of the site). Requires curl and iproute2 on the host.
set -euo pipefail

# Whole body in main(): bash parses the entire file before running, so editing the script mid-run cannot make it
# re-execute or skip parts (CLAUDE.md rule 13; the 43-vs-41 checks incident of v0.2.5, ISS-035).
main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/lib/source-rev.sh"
source_rev_line "$0"

root="$(cd "$(dirname "$0")/.." && pwd)"
JAIL=sentra-web
NS=sentra-f2b VETH_HOST=f2b-host VETH_NS=f2b-ns GW_IP=198.51.100.9 TESTER_IP=198.51.100.10
ATTEMPTS=11   # > maxretry (10)
pass=0 fail=0

[[ $EUID -eq 0 ]] || { echo "run as root"; exit 1; }
check() { if [[ "$2" == "0" ]]; then printf '  PASS  %s\n' "$1"; pass=$((pass+1)); else printf '  FAIL  %s\n' "$1"; fail=$((fail+1)); fi; }

domain="$(sed -n 's/^DOMAIN=//p' "$root/.env" | tail -1 | tr -d "'\"")"
host_ip="$(ip -4 route get 1.1.1.1 | awk '{for (i=1;i<NF;i++) if ($i=="src") print $(i+1)}')"
[[ -n "$domain" && -n "$host_ip" ]] || { echo "cannot determine DOMAIN or host IP"; exit 1; }

cleanup() {
  fail2ban-client set "$JAIL" unbanip "$TESTER_IP" >/dev/null 2>&1 || true
  ip link del "$VETH_HOST" >/dev/null 2>&1 || true
  ip netns del "$NS" >/dev/null 2>&1 || true
}
trap cleanup EXIT

start_tester() {
  ip netns add "$NS"
  ip link add "$VETH_HOST" type veth peer name "$VETH_NS"
  ip link set "$VETH_NS" netns "$NS"
  ip addr add "$GW_IP/30" dev "$VETH_HOST" && ip link set "$VETH_HOST" up
  ip -n "$NS" addr add "$TESTER_IP/30" dev "$VETH_NS"
  ip -n "$NS" link set lo up && ip -n "$NS" link set "$VETH_NS" up
  ip -n "$NS" route add default via "$GW_IP"
}
# HTTP status of /api/v1/health from the tester ("000" + curl exit code when the connection is refused/blocked).
probe() {
  local out rc=0
  out="$(ip netns exec "$NS" curl -s -o /dev/null -w '%{http_code}' --max-time 5 --resolve "$domain:443:$host_ip" "https://$domain/api/v1/health")" || rc=$?
  echo "$out/$rc"
}
# One failed app login from the tester (unknown user → 401 invalid_credentials).
fail_login() {
  ip netns exec "$NS" curl -s -o /dev/null -w '%{http_code}' --max-time 5 --resolve "$domain:443:$host_ip" \
    -H "Origin: https://$domain" -H 'X-Sentra-CSRF: login' -H 'Content-Type: application/json' \
    -d "{\"username\":\"f2b.test.$RANDOM\",\"password\":\"not-a-real-password\"}" "https://$domain/api/v1/auth/login" || true
}
banned() { fail2ban-client status "$JAIL" | grep -q "Banned IP list:.*\b${TESTER_IP//./\\.}\b"; }
total_failed() { fail2ban-client status "$JAIL" | sed -n 's/.*Total failed:[[:space:]]*//p'; }
# The jail tails its log with backend=polling: right after a fail2ban restart (e.g. ops/harden re-run earlier in the
# full regression) lines written before the first poll are skipped. Repeat failed logins until the jail counts one.
wait_watching() {
  local base; base="$(total_failed)"
  for _ in $(seq 1 15); do
    fail_login >/dev/null; sleep 1
    (( $(total_failed) > base )) && return 0
  done
  return 1
}
wait_banned() { for _ in $(seq 1 40); do banned && return 0; sleep 0.5; done; return 1; }
admin_ips() { sed -n 's/^ADMIN_IPS=//p' "$root/.env" | tr -d "'\"" | tr ',' ' '; who | grep -oE '\(([0-9]{1,3}\.){3}[0-9]{1,3}\)' | tr -d '()'; }

echo "fail2ban ban test → https://$domain via $host_ip from $TESTER_IP (jail $JAIL)"
fail2ban-client status "$JAIL" >/dev/null || { echo "jail $JAIL is not running (make fail2ban-web)"; exit 1; }
cleanup; start_tester

check "tester reaches the site before the test (health 200)" "$([[ $(probe) == 200/0 ]]; echo $?)"
check "a failed app login answers 401" "$([[ $(fail_login) == 401 ]]; echo $?)"
check "jail $JAIL counts the tester's failed logins (log watcher ready)" "$(wait_watching; echo $?)"
for _ in $(seq 2 "$ATTEMPTS"); do fail_login >/dev/null; done
check "tester IP banned after $ATTEMPTS failed app logins (> maxretry 10)" "$(wait_banned; echo $?)"
check "DOCKER-USER → f2b-$JAIL rejects $TESTER_IP" "$(iptables -S "f2b-$JAIL" | grep -q -- "-s $TESTER_IP/32 .*-j REJECT"; echo $?)"
r="$(probe)"; check "banned tester's connection is rejected (got ${r})" "$([[ ${r%%/*} == 000 && ${r##*/} != 0 ]]; echo $?)"

if [[ -n "${RESTART:-}" ]]; then
  echo "  ...  restarting $RESTART while $TESTER_IP is banned"
  systemctl restart "$RESTART"
  # Containers come back by their restart policy: wait until the site really answers 200 again, so a rejected
  # connection below proves the ban — not a container that is still starting.
  if [[ "$RESTART" == docker ]]; then
    for _ in $(seq 1 180); do curl -sf -o /dev/null "https://$domain/api/v1/health" && break; sleep 1; done
  fi
  check "after restart $RESTART: site healthy again (health 200 from the host)" "$(curl -sf -o /dev/null "https://$domain/api/v1/health"; echo $?)"
  for _ in $(seq 1 60); do fail2ban-client status "$JAIL" >/dev/null 2>&1 && break; sleep 1; done
  check "after restart $RESTART: jail $JAIL active" "$(fail2ban-client status "$JAIL" >/dev/null 2>&1; echo $?)"
  check "after restart $RESTART: sshd jail active" "$(fail2ban-client status sshd >/dev/null 2>&1; echo $?)"
  check "after restart $RESTART: ban kept" "$(wait_banned; echo $?)"
  for _ in $(seq 1 20); do iptables -S DOCKER-USER | grep -q -- "-j f2b-$JAIL" && break; sleep 0.5; done
  check "after restart $RESTART: DOCKER-USER still jumps to f2b-$JAIL" "$(iptables -S DOCKER-USER | grep -q -- "-j f2b-$JAIL"; echo $?)"
  r="$(probe)"; check "after restart $RESTART: connection still rejected (got ${r})" "$([[ ${r%%/*} == 000 && ${r##*/} != 0 ]]; echo $?)"
fi

for ip in $(admin_ips); do
  check "admin IP is not banned" "$(! fail2ban-client status "$JAIL" | grep -q "\b${ip//./\\.}\b"; echo $?)"
done
check "site still answers from the host (health 200)" "$(curl -sf -o /dev/null "https://$domain/api/v1/health"; echo $?)"

fail2ban-client set "$JAIL" unbanip "$TESTER_IP" >/dev/null
check "unban removes the tester from the jail" "$(! banned; echo $?)"
r="$(probe)"; check "after unban the tester reaches the site again (got ${r})" "$([[ $r == 200/0 ]]; echo $?)"

echo "fail2ban ban test: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
}

main "$@"
