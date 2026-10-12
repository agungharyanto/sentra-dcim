#!/usr/bin/env bash
# Automatic security updates (no automatic reboot). Idempotent.
export HARDEN_STEP=unattended-upgrades
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"
require_root
apt_install unattended-upgrades apt-listchanges

write_if_changed /etc/apt/apt.conf.d/20auto-upgrades <<'CONF' || true
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
CONF

write_if_changed /etc/apt/apt.conf.d/52sentra-unattended-upgrades <<'CONF' || true
// Managed by sentra-dcim ops/harden/unattended-upgrades.sh — do not edit by hand.
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "false";
Unattended-Upgrade::SyslogEnable "true";
CONF

systemctl enable --now unattended-upgrades >/dev/null 2>&1
unattended-upgrade --dry-run >/dev/null 2>&1 && log "dry-run OK"
systemctl is-active unattended-upgrades
