#!/usr/bin/env bash
# Run all hardening steps in a safe order. Idempotent; safe to re-run.
set -euo pipefail
dir="$(cd "$(dirname "$0")" && pwd)"
for step in sysctl ufw fail2ban unattended-upgrades; do
  "$dir/$step.sh"
done
