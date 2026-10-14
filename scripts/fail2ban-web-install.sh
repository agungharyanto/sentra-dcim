#!/usr/bin/env bash
# Installs (or removes with --remove) the fail2ban jail `sentra-web`: bans client IPs with repeated failed APP
# sign-ins (password or MFA code, v0.3.3, D-066; until v0.3.2 it counted wrong basic_auth logins, ISS-026).
# Exempt: localhost + ADMIN_IPS of .env. Idempotent: unchanged files are left alone. Needs root.
#   sudo scripts/fail2ban-web-install.sh            install / update
#   sudo scripts/fail2ban-web-install.sh --remove   remove everything
# Does not touch the existing sshd jail or /etc/fail2ban/jail.d/sentra.local.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/fail2ban-web.sh
source "$root/scripts/lib/fail2ban-web.sh"
f2b_dests

die() { echo "fail2ban-web: ERROR: $*" >&2; exit 1; }
log() { echo "fail2ban-web: $*"; }
[[ -z "${F2B_DEST_ROOT:-}${F2B_LOGPATH:-}${F2B_ADMIN_IPS:-}" ]] || die "F2B_DEST_ROOT/F2B_LOGPATH are test overrides of the check script only"
[[ $EUID -eq 0 ]] || die "run as root (make fail2ban-web uses sudo)"
command -v fail2ban-client >/dev/null || die "fail2ban is not installed (apt-get install fail2ban)"

if [[ "${1:-}" == "--remove" ]]; then
  rm -f "$jail_dst" "$filter_dst" "$dropin_dst"
  rmdir --ignore-fail-on-non-empty "$(dirname "$dropin_dst")" 2>/dev/null || true
  systemctl daemon-reload
  fail2ban-client reload >/dev/null   # stops the removed jail and its DOCKER-USER rules
  log "removed jail sentra-web, filter and systemd drop-in"
  iptables -S DOCKER-USER | grep -q f2b-sentra-web && die "f2b-sentra-web still referenced in DOCKER-USER"
  exit 0
fi

# --- install (only what changed) -------------------------------------------------------------------------------
changed=0
install_if_changed() { # src dst
  if [[ -f "$2" ]] && cmp -s "$1" "$2"; then log "unchanged $2"; return; fi
  install -D -m 0644 -o root -g root "$1" "$2"; log "installed $2"; changed=1
}
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
f2b_render "$root" "$tmp"
logpath="$f2b_logpath"
[[ -f "$logpath" ]] || die "access log $logpath not found — run 'make up' and send one request to the public domain"

install_if_changed "$tmp/sentra-app-login.conf" "$filter_dst"
# The basic_auth filter of ISS-026 (v0.2.5–v0.3.2) is replaced by sentra-app-login (v0.3.3).
legacy=/etc/fail2ban/filter.d/sentra-caddy-auth.conf
if [[ -f "$legacy" ]]; then rm -f "$legacy"; log "removed legacy $legacy"; changed=1; fi
install_if_changed "$tmp/sentra-web.local" "$jail_dst"
dropin_changed=$changed; changed=0
install_if_changed "$tmp/sentra-docker.conf" "$dropin_dst"
[[ $changed -eq 1 ]] && systemctl daemon-reload
changed=$((changed | dropin_changed))

fail2ban-client -t >/dev/null || die "fail2ban configuration test failed (fail2ban-client -t)"
if [[ $changed -eq 1 ]] || ! fail2ban-client status sentra-web >/dev/null 2>&1; then
  systemctl restart fail2ban
  log "fail2ban restarted"
fi

# --- verify ----------------------------------------------------------------------------------------------------
for _ in $(seq 1 20); do fail2ban-client status sentra-web >/dev/null 2>&1 && break; sleep 0.5; done
fail2ban-client status sentra-web >/dev/null 2>&1 || die "jail sentra-web is not running"
fail2ban-client status sshd >/dev/null 2>&1 || die "jail sshd is not running any more"
for _ in $(seq 1 20); do iptables -S DOCKER-USER | grep -q -- '-j f2b-sentra-web' && break; sleep 0.5; done
iptables -S DOCKER-USER | grep -q -- '-j f2b-sentra-web' || die "DOCKER-USER does not jump to f2b-sentra-web"
[[ "$(fail2ban-client get sentra-web logpath | grep -c "$logpath")" -ge 1 ]] || die "jail does not watch $logpath"
log "OK: jail sentra-web active (log $logpath), sshd jail untouched, DOCKER-USER → f2b-sentra-web"
