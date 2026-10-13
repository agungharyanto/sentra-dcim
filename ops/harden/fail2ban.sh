#!/usr/bin/env bash
# fail2ban jail for sshd on the detected port(s), with incremental bans. Idempotent.
# Never-banned addresses (ignoreip): loopback, SSH_ALLOW_FROM, ADMIN_IPS (env or repo .env, comma-separated
# IPs/CIDRs) and the source IPs of SSH sessions established while this script runs (active admins).
# Unban from the Oracle serial console: see docs/DISASTER_RECOVERY.md "Akses darurat".
export HARDEN_STEP=fail2ban
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"
require_root
apt_install fail2ban

ports="$(sshd_ports | paste -sd, -)"
ADMIN_IPS="${ADMIN_IPS:-$(repo_env ADMIN_IPS)}"
for a in ${ADMIN_IPS//,/ }; do
  [[ "$a" =~ ^[0-9A-Fa-f:.]+(/[0-9]{1,3})?$ ]] || die "ADMIN_IPS: invalid IP/CIDR '$a'"
done
mapfile -t active < <(active_ssh_peers)
log "active ssh peers whitelisted: ${active[*]:-none}"
ignoreip="127.0.0.1/8 ::1${SSH_ALLOW_FROM:+ ${SSH_ALLOW_FROM//,/ }}${ADMIN_IPS:+ ${ADMIN_IPS//,/ }}${active[*]:+ ${active[*]}}"

if write_if_changed /etc/fail2ban/jail.d/sentra.local <<CONF
# Managed by sentra-dcim ops/harden/fail2ban.sh — do not edit by hand.
[DEFAULT]
backend = systemd
banaction = nftables
banaction_allports = nftables[type=allports]
ignoreip = ${ignoreip}
bantime = 1h
bantime.increment = true
bantime.maxtime = 1w
findtime = 10m
maxretry = 5

[sshd]
enabled = true
port = ${ports}
mode = aggressive
CONF
then
  systemctl restart fail2ban
fi
systemctl enable --now fail2ban >/dev/null 2>&1
fail2ban-client status sshd
