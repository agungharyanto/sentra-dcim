#!/usr/bin/env bash
# Firewall: default deny incoming; allow only the active sshd port(s).
# Idempotent. Variables (Pending decisions D-007/D-008):
#   SSH_ALLOW_FROM  comma-separated CIDRs allowed to reach SSH (empty = anywhere, protected by fail2ban)
#   WG_PORT         WireGuard UDP port to open (empty = closed)
#   OPEN_HTTP       1 = open 80/tcp and 443/tcp for Caddy. Defaults to UFW_OPEN_HTTP from the repo .env,
#                   else 0 (closed), so re-running all.sh never silently closes a published site.
# NOTE: Docker-published ports bypass ufw; every container port must bind to 127.0.0.1.
export HARDEN_STEP=ufw
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"
require_root
apt_install ufw

SSH_ALLOW_FROM="${SSH_ALLOW_FROM:-}"
WG_PORT="${WG_PORT:-}"
env_file="$(cd "$(dirname "$0")/../.." && pwd)/.env"
if [[ -z "${OPEN_HTTP:-}" && -f "$env_file" ]]; then
  OPEN_HTTP="$(sed -n 's/^UFW_OPEN_HTTP=\([01]\)$/\1/p' "$env_file" | tail -1)"
fi
OPEN_HTTP="${OPEN_HTTP:-0}"
log "open 80/443: $OPEN_HTTP"

mapfile -t ports < <(sshd_ports)
log "sshd ports: ${ports[*]}"

# Allow SSH first so enabling the firewall can never lock us out.
for p in "${ports[@]}"; do
  if [[ -n "$SSH_ALLOW_FROM" ]]; then
    IFS=',' read -ra cidrs <<<"$SSH_ALLOW_FROM"
    for c in "${cidrs[@]}"; do ufw allow proto tcp from "$c" to any port "$p" comment 'sentra: ssh allowlist' >/dev/null; done
    ufw --force delete allow "$p/tcp" >/dev/null 2>&1 || true
  else
    ufw allow "$p/tcp" comment 'sentra: ssh' >/dev/null
  fi
done

ufw default deny incoming >/dev/null
ufw default allow outgoing >/dev/null
ufw default deny routed >/dev/null
ufw logging low >/dev/null

if [[ -n "$WG_PORT" ]]; then
  ufw allow "$WG_PORT/udp" comment 'sentra: wireguard' >/dev/null
fi

if [[ "$OPEN_HTTP" == "1" ]]; then
  ufw allow 80/tcp comment 'sentra: http' >/dev/null
  ufw allow 443/tcp comment 'sentra: https' >/dev/null
else
  for r in 80/tcp 443/tcp; do ufw --force delete allow "$r" >/dev/null 2>&1 || true; done
fi

ufw --force enable >/dev/null

# Oracle Cloud images ship iptables rules (netfilter-persistent) whose catch-all INPUT REJECT sits *before* the ufw
# chains, so every ufw allow except 22/tcp is silently ineffective. Drop that one rule (runtime + persisted) so ufw
# is the single host firewall; the Oracle InstanceServices rules are kept. Done after `ufw enable` so INPUT is never
# left without a deny policy. The VCN Security List is a separate, outer firewall managed in the Oracle console.
oci_rules=/etc/iptables/rules.v4
oci_reject='-A INPUT -j REJECT --reject-with icmp-host-prohibited'
if [[ -f "$oci_rules" ]] && grep -q 'Oracle Cloud Infrastructure' "$oci_rules"; then
  if grep -qxF -- "$oci_reject" "$oci_rules"; then
    [[ -f "$oci_rules.sentra-orig" ]] || cp -p "$oci_rules" "$oci_rules.sentra-orig"
    grep -vxF -- "$oci_reject" "$oci_rules.sentra-orig" | MODE=0644 write_if_changed "$oci_rules" || true
  fi
  while iptables -C INPUT -j REJECT --reject-with icmp-host-prohibited 2>/dev/null; do
    iptables -D INPUT -j REJECT --reject-with icmp-host-prohibited
    log "removed Oracle INPUT REJECT rule that shadowed ufw"
  done
fi

ufw status verbose
