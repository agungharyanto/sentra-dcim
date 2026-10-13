#!/usr/bin/env bash
# Shared helpers for hardening scripts. Source only.
set -euo pipefail

log() { printf '[harden:%s] %s\n' "${HARDEN_STEP:-main}" "$*"; }
die() { log "ERROR: $*" >&2; exit 1; }

require_root() { [[ $EUID -eq 0 ]] || die "must run as root (use sudo)"; }

# Detect active sshd ports from the effective sshd configuration.
sshd_ports() {
  local ports
  ports="$(sshd -T 2>/dev/null | awk '$1=="port"{print $2}' | sort -u)"
  [[ -n "$ports" ]] || die "cannot detect sshd port (sshd -T failed)"
  echo "$ports"
}

# Read KEY from the repo .env (last occurrence); prints nothing if unset or the file is missing.
repo_env() {
  local env_file
  env_file="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/.env"
  [[ -f "$env_file" ]] || return 0
  sed -n "s/^$1=//p" "$env_file" | tail -1
}

# Source IPs of currently established SSH sessions (the admins running this script right now).
active_ssh_peers() {
  local p
  for p in $(sshd_ports); do
    ss -Htn state established "( sport = :$p )" | awk '{print $NF}'
  done | sed -E 's/^\[?(::ffff:)?//; s/\]?:[0-9]+$//' | sort -u
}

# Write file only if content differs. Prints "changed" when written.
write_if_changed() {
  local dest="$1" tmp
  tmp="$(mktemp)"
  cat >"$tmp"
  if [[ -f "$dest" ]] && cmp -s "$tmp" "$dest"; then
    rm -f "$tmp"
    return 1
  fi
  install -m "${MODE:-0644}" "$tmp" "$dest"
  rm -f "$tmp"
  log "updated $dest"
  return 0
}

apt_install() {
  local missing=()
  for p in "$@"; do dpkg -s "$p" >/dev/null 2>&1 || missing+=("$p"); done
  if ((${#missing[@]})); then
    log "installing: ${missing[*]}"
    DEBIAN_FRONTEND=noninteractive apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "${missing[@]}"
  fi
}
