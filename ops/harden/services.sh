#!/usr/bin/env bash
# Disable network services shipped by cloud images that Sentra does not use. Idempotent.
# rpcbind (port 111, pulled in by nfs-common on Oracle Cloud images) is only needed for NFS mounts; it is left
# alone when an NFS mount exists or is configured in /etc/fstab.
export HARDEN_STEP=services
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"
require_root

if systemctl list-unit-files rpcbind.service >/dev/null 2>&1; then
  if [[ -n "$(findmnt -n -t nfs,nfs4)" ]] || grep -qE '^[^#].*\snfs4?\s' /etc/fstab; then
    log "rpcbind kept: NFS in use"
  elif [[ "$(systemctl is-enabled rpcbind.socket 2>/dev/null)" != masked ]]; then
    systemctl disable --now rpcbind.socket rpcbind.service >/dev/null 2>&1 || true
    systemctl mask rpcbind.socket rpcbind.service >/dev/null 2>&1
    log "updated: rpcbind disabled and masked (no NFS)"
  fi
fi
