#!/usr/bin/env bash
# Retires an old kernel kept as rollback target (ISS-036, v0.3.9): removes its APT pin, purges exactly its packages
# and regenerates GRUB. Refuses when that kernel is running, when it is the only one, or when the purge would touch
# any other package. Root. Usage: sudo scripts/kernel-retire.sh 6.17.0-1020-oracle [--yes]
# Without --yes it only shows what would happen (apt simulation).
set -euo pipefail

main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/lib/source-rev.sh"
source_rev_line "$0"
[[ $EUID -eq 0 ]] || { echo "run as root (sudo)"; exit 1; }
local ver="${1:?usage: kernel-retire.sh <kernel version, e.g. 6.17.0-1020-oracle> [--yes]}" yes="${2:-}"
[[ "$ver" =~ ^[0-9]+\.[0-9]+\.[0-9]+-[0-9]+-[a-z]+$ ]] || { echo "ERROR: not a kernel version: $ver"; exit 1; }
local running; running="$(uname -r)"
[[ "$running" != "$ver" ]] || { echo "ERROR: $ver is the running kernel"; exit 1; }
ls "/boot/vmlinuz-$running" >/dev/null || { echo "ERROR: running kernel $running has no image in /boot"; exit 1; }

local short="${ver%-*}"            # 6.17.0-1020
local flavour="${ver##*-}"         # oracle
local series; series="$(cut -d. -f1-2 <<<"$short")" # 6.17
mapfile -t pkgs < <(dpkg-query -W -f '${Package} ${Status}\n' 2>/dev/null |
  awk -v v="$ver" -v s="$short" -v f="$flavour" -v ser="$series" '
    $0 ~ /install ok installed/ && ($1 ~ "-" v "$" || $1 == "linux-" f "-" ser "-headers-" s || $1 == "linux-" f "-" ser "-tools-" s) {print $1}')
[[ ${#pkgs[@]} -gt 0 ]] || { echo "nothing installed for $ver"; exit 0; }
echo "packages of $ver: ${pkgs[*]}"

sim="$(apt-get -s purge "${pkgs[@]}" 2>&1)"
mapfile -t touched < <(grep -E '^(Purg|Remv|Inst) ' <<<"$sim" | awk '{print $2}')
for p in "${touched[@]}"; do
  printf '%s\n' "${pkgs[@]}" | grep -qx "$p" || { echo "ERROR: the purge would also touch $p — stopping"; grep -E '^(Purg|Remv|Inst) ' <<<"$sim"; exit 1; }
done
if [[ "$yes" != --yes ]]; then
  echo "simulation OK (${#touched[@]} packages). Re-run with --yes to remove them."
  exit 0
fi

rm -f "/etc/apt/apt.conf.d/99sentra-keep-kernel-${series}"
apt-mark auto "${pkgs[@]}" >/dev/null
DEBIAN_FRONTEND=noninteractive apt-get purge -y "${pkgs[@]}" 2>&1 | grep -E '^(Removing|Purging)' || true
update-grub 2>&1 | sed 's/^/  update-grub: /'

# --- verify ---
ok=1
if dpkg-query -W -f '${Status}\n' "${pkgs[@]}" 2>/dev/null | grep -q 'install ok installed'; then echo "FAIL packages still installed"; ok=0; fi
grep -q "vmlinuz-$ver" /boot/grub/grub.cfg && { echo "FAIL $ver still in grub.cfg"; ok=0; }
grep -q "vmlinuz-$running" /boot/grub/grub.cfg || { echo "FAIL running kernel $running missing from grub.cfg"; ok=0; }
[[ $ok -eq 1 ]] && echo "OK: $ver removed; GRUB boots $running (menu stays visible for the serial console)"
[[ $ok -eq 1 ]]
}

main "$@"
