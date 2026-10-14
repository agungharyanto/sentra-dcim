# shellcheck shell=bash
# Shared by scripts/fail2ban-web-install.sh (root, installs) and scripts/fail2ban-web-check.sh (no root, release
# drift check): renders the expected jail sentra-web files from deploy/fail2ban into a directory, so both compare
# exactly the same bytes. Callers define die().
#
# Test overrides (scripts/fail2ban-web-check-test.sh only): F2B_DEST_ROOT prefixes the installed paths,
# F2B_LOGPATH skips the Docker volume lookup, F2B_ADMIN_IPS replaces ADMIN_IPS of .env.

# shellcheck disable=SC2034 # the destinations are read by the caller
f2b_dests() { # sets filter_dst jail_dst dropin_dst
  local p="${F2B_DEST_ROOT:-}"
  filter_dst="$p/etc/fail2ban/filter.d/sentra-app-login.conf"
  jail_dst="$p/etc/fail2ban/jail.d/sentra-web.local"
  dropin_dst="$p/etc/systemd/system/fail2ban.service.d/sentra-docker.conf"
}

# f2b_render <repo root> <out dir>: writes sentra-app-login.conf, sentra-web.local, sentra-docker.conf;
# sets f2b_logpath.
f2b_render() {
  local root="$1" out="$2" src="$1/deploy/fail2ban" domain host_re project volume mountpoint jail
  domain="$({ sed -n 's/^DOMAIN=//p' "$root/.env" 2>/dev/null || true; } | tail -1 | tr -d "'\"")"
  [[ "$domain" =~ ^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$ ]] || die "DOMAIN in .env must be the public host name (got '$domain')"
  host_re="${domain//./\\.}"   # host names only contain [A-Za-z0-9.-]: escaping the dots is enough

  if [[ -n "${F2B_LOGPATH:-}" ]]; then
    f2b_logpath="$F2B_LOGPATH"
  else
    # Log path from the caddy_logs volume of this compose project (never a hardcoded project name).
    project="$(cd "$root" && docker compose config --format json 2>/dev/null | python3 -c 'import json,sys;print(json.load(sys.stdin)["name"])')" ||
      die "cannot read the compose project name (docker compose config)"
    volume="$(docker volume ls -q --filter "label=com.docker.compose.project=$project" --filter "label=com.docker.compose.volume=caddy_logs")"
    [[ -n "$volume" ]] || die "volume caddy_logs of project '$project' not found — run 'make up' first"
    mountpoint="$(docker volume inspect -f '{{.Mountpoint}}' "$volume")"
    f2b_logpath="$mountpoint/access.log"
  fi

  # Plain string substitution (sed would eat the backslashes of the host regex).
  jail="$(<"$src/jail.d/sentra-web.local.in")"
  # ignoreip: loopback + ADMIN_IPS (comma-separated IPs/CIDRs in .env), each validated.
  local ignore="127.0.0.1/8 ::1" admin_ips a
  admin_ips="$({ sed -n 's/^ADMIN_IPS=//p' "$root/.env" 2>/dev/null || true; } | tail -1 | tr -d "'\"")"
  if [[ -n "${F2B_ADMIN_IPS+x}" ]]; then admin_ips="$F2B_ADMIN_IPS"; fi
  for a in ${admin_ips//,/ }; do
    [[ "$a" =~ ^[0-9A-Fa-f:.]+(/[0-9]{1,3})?$ ]] || die "ADMIN_IPS: invalid IP/CIDR '$a'"
    ignore+=" $a"
  done
  jail="${jail//@HOST_RE@/$host_re}"
  jail="${jail//@IGNOREIP@/$ignore}"
  jail="${jail//@LOGPATH@/$f2b_logpath}"
  printf '%s\n' "$jail" > "$out/sentra-web.local"
  grep -qE '@(HOST_RE|LOGPATH|IGNOREIP)@' "$out/sentra-web.local" && die "unfilled placeholder in jail template"
  cp "$src/filter.d/sentra-app-login.conf" "$out/sentra-app-login.conf"
  cp "$src/systemd/sentra-docker.conf" "$out/sentra-docker.conf"
}
