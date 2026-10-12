#!/usr/bin/env bash
# Install/refresh the systemd daily backup timer. Idempotent. Run with sudo.
set -euo pipefail
[[ $EUID -eq 0 ]] || { echo "run with sudo" >&2; exit 1; }
src="$(cd "$(dirname "$0")" && pwd)/systemd"
install -m 0644 "$src/sentra-backup.service" "$src/sentra-backup.timer" /etc/systemd/system/
install -d -o sentra -g sentra -m 0700 /var/backups/sentra
systemctl daemon-reload
systemctl enable --now sentra-backup.timer
systemctl list-timers sentra-backup.timer --no-pager
