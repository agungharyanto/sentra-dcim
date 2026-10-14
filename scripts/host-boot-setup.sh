#!/usr/bin/env bash
# Host boot preparation (v0.2.9), idempotent, root:
#  - GRUB menu visible for 10 s (serial console) → /etc/default/grub.d/99-sentra-boot-menu.cfg + update-grub
#  (until v0.3.9 it also pinned kernel 6.17.0-1020-oracle as rollback target; retired with scripts/kernel-retire.sh, ISS-036)
#  - docker, containerd, fail2ban enabled at boot
set -euo pipefail

main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/lib/source-rev.sh"
source_rev_line "$0"
[[ $EUID -eq 0 ]] || { echo "run as root (make boot-menu uses sudo)"; exit 1; }
root="$(cd "$(dirname "$0")/.." && pwd)"

changed=0
install_if_changed() { # src dst
  if [[ -f "$2" ]] && cmp -s "$1" "$2"; then echo "unchanged $2"; return; fi
  install -D -m 0644 -o root -g root "$1" "$2"; echo "installed $2"; changed=1
}
install_if_changed "$root/deploy/host/grub/99-sentra-boot-menu.cfg" /etc/default/grub.d/99-sentra-boot-menu.cfg
grub_changed=$changed

if [[ $grub_changed -eq 1 ]] || ! grep -q 'set timeout_style=menu' /boot/grub/grub.cfg; then
  update-grub 2>&1 | sed 's/^/  update-grub: /'
fi

systemctl enable docker.service containerd.service fail2ban.service >/dev/null 2>&1

# --- verify ---
ok=1
if ! { grep -q 'set timeout_style=menu' /boot/grub/grub.cfg && grep -qE 'set timeout=10' /boot/grub/grub.cfg; }; then
  echo "FAIL grub.cfg menu/timeout"; ok=0
fi
grep -q "vmlinuz-$(uname -r)" /boot/grub/grub.cfg || { echo "FAIL running kernel missing from grub.cfg"; ok=0; }
for u in docker containerd fail2ban; do [[ "$(systemctl is-enabled "$u")" == enabled ]] || { echo "FAIL $u not enabled"; ok=0; }; done
[[ $ok -eq 1 ]] && echo "OK: GRUB menu 10 s, docker/containerd/fail2ban enabled"
[[ $ok -eq 1 ]]
}

main "$@"
