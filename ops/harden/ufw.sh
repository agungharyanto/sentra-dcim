#!/usr/bin/env bash
# Firewall: default deny incoming; allow only the active sshd port(s).
# Idempotent. Variables (Pending decisions D-007/D-008):
#   SSH_ALLOW_FROM  comma-separated CIDRs allowed to reach SSH (empty = anywhere, protected by fail2ban)
#   WG_PORT         WireGuard UDP port to open (empty = closed)
#   OPEN_HTTP       1 = open 80/443 for Caddy (default 0 = closed)
# NOTE: Docker-published ports bypass ufw; every container port must bind to 127.0.0.1.
export HARDEN_STEP=ufw
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"
require_root
apt_install ufw

SSH_ALLOW_FROM="${SSH_ALLOW_FROM:-}"
WG_PORT="${WG_PORT:-}"
OPEN_HTTP="${OPEN_HTTP:-0}"

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
ufw status verbose
