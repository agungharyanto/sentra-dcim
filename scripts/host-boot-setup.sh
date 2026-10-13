#!/usr/bin/env bash
# Host boot preparation (v0.2.9), idempotent, root:
#  - GRUB menu visible for 10 s (serial console) → /etc/default/grub.d/99-sentra-boot-menu.cfg + update-grub
#  - keep kernel 6.17.0-1020-oracle as rollback target (apt-mark manual + APT::NeverAutoRemove)
#  - docker, containerd, fail2ban enabled at boot
set -euo pipefail

main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/lib/source-rev.sh"
source_rev_line "$0"
[[ $EUID -eq 0 ]] || { echo "run as root (make boot-menu uses sudo)"; exit 1; }
root="$(cd "$(dirname "$0")/.." && pwd)"
keep_kernel=6.17.0-1020-oracle

changed=0
install_if_changed() { # src dst
  if [[ -f "$2" ]] && cmp -s "$1" "$2"; then echo "unchanged $2"; return; fi
  install -D -m 0644 -o root -g root "$1" "$2"; echo "installed $2"; changed=1
}
install_if_changed "$root/deploy/host/grub/99-sentra-boot-menu.cfg" /etc/default/grub.d/99-sentra-boot-menu.cfg
grub_changed=$changed
install_if_changed "$root/deploy/host/apt/99sentra-keep-kernel-6.17" /etc/apt/apt.conf.d/99sentra-keep-kernel-6.17

if [[ $grub_changed -eq 1 ]] || ! grep -q 'set timeout_style=menu' /boot/grub/grub.cfg; then
  update-grub 2>&1 | sed 's/^/  update-grub: /'
fi

pkgs=()
for p in "linux-image-$keep_kernel" "linux-modules-$keep_kernel" "linux-modules-extra-$keep_kernel"; do
  dpkg -s "$p" >/dev/null 2>&1 && pkgs+=("$p")
done
[[ ${#pkgs[@]} -gt 0 ]] || { echo "ERROR: kernel $keep_kernel is not installed"; exit 1; }
apt-mark manual "${pkgs[@]}" >/dev/null
systemctl enable docker.service containerd.service fail2ban.service >/dev/null 2>&1

# --- verify ---
ok=1
if ! { grep -q 'set timeout_style=menu' /boot/grub/grub.cfg && grep -qE 'set timeout=10' /boot/grub/grub.cfg; }; then
  echo "FAIL grub.cfg menu/timeout"; ok=0
fi
grep -q "vmlinuz-$keep_kernel" /boot/grub/grub.cfg || { echo "FAIL $keep_kernel missing from grub.cfg"; ok=0; }
if apt-get -s autoremove 2>/dev/null | grep -qE "^Remv .*$keep_kernel"; then echo "FAIL autoremove would remove $keep_kernel"; ok=0; fi
for u in docker containerd fail2ban; do [[ "$(systemctl is-enabled "$u")" == enabled ]] || { echo "FAIL $u not enabled"; ok=0; }; done
[[ $ok -eq 1 ]] && echo "OK: GRUB menu 10 s, kernel $keep_kernel kept (manual + NeverAutoRemove), docker/containerd/fail2ban enabled"
[[ $ok -eq 1 ]]
}

main "$@"
