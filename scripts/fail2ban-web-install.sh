#!/usr/bin/env bash
# Installs (or removes with --remove) the fail2ban jail `sentra-web` that bans clients sending wrong basic_auth
# credentials (ISS-026, v0.2.5). Idempotent: unchanged files are left alone. Needs root.
#   sudo scripts/fail2ban-web-install.sh            install / update
#   sudo scripts/fail2ban-web-install.sh --remove   remove everything (v0.3, together with basic_auth)
# Does not touch the existing sshd jail or /etc/fail2ban/jail.d/sentra.local.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
src="$root/deploy/fail2ban"
filter_dst=/etc/fail2ban/filter.d/sentra-caddy-auth.conf
jail_dst=/etc/fail2ban/jail.d/sentra-web.local
dropin_dst=/etc/systemd/system/fail2ban.service.d/sentra-docker.conf

die() { echo "fail2ban-web: ERROR: $*" >&2; exit 1; }
log() { echo "fail2ban-web: $*"; }
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

# --- inputs ----------------------------------------------------------------------------------------------------
domain="$(sed -n 's/^DOMAIN=//p' "$root/.env" 2>/dev/null | tail -1 | tr -d "'\"")"
[[ "$domain" =~ ^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$ ]] || die "DOMAIN in .env must be the public host name (got '$domain')"
host_re="${domain//./\\.}"   # host names only contain [A-Za-z0-9.-]: escaping the dots is enough

# Log path from the caddy_logs volume of this compose project (never a hardcoded project name).
project="$(cd "$root" && docker compose config --format json 2>/dev/null | python3 -c 'import json,sys;print(json.load(sys.stdin)["name"])')" ||
  die "cannot read the compose project name (docker compose config)"
volume="$(docker volume ls -q --filter "label=com.docker.compose.project=$project" --filter "label=com.docker.compose.volume=caddy_logs")"
[[ -n "$volume" ]] || die "volume caddy_logs of project '$project' not found — run 'make up' first"
mountpoint="$(docker volume inspect -f '{{.Mountpoint}}' "$volume")"
logpath="$mountpoint/access.log"
[[ -f "$logpath" ]] || die "access log $logpath not found — run 'make up' and send one request to https://$domain/"

# --- install (only what changed) -------------------------------------------------------------------------------
changed=0
install_if_changed() { # src dst
  if [[ -f "$2" ]] && cmp -s "$1" "$2"; then log "unchanged $2"; return; fi
  install -D -m 0644 -o root -g root "$1" "$2"; log "installed $2"; changed=1
}
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# Plain string substitution (sed would eat the backslashes of the host regex).
jail="$(<"$src/jail.d/sentra-web.local.in")"
jail="${jail//@HOST_RE@/$host_re}"
jail="${jail//@LOGPATH@/$logpath}"
printf '%s\n' "$jail" > "$tmp/sentra-web.local"
grep -qE '@(HOST_RE|LOGPATH)@' "$tmp/sentra-web.local" && die "unfilled placeholder in jail template"

install_if_changed "$src/filter.d/sentra-caddy-auth.conf" "$filter_dst"
install_if_changed "$tmp/sentra-web.local" "$jail_dst"
dropin_changed=$changed; changed=0
install_if_changed "$src/systemd/sentra-docker.conf" "$dropin_dst"
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
log "OK: jail sentra-web active (host $domain, log $logpath), sshd jail untouched, DOCKER-USER → f2b-sentra-web"
