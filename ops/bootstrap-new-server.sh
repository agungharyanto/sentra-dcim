#!/usr/bin/env bash
# Bootstrap Sentra DCIM on a fresh Ubuntu 24.04 server.
# Installs dependencies (Docker CE, git, make, jq, ufw, fail2ban), clones the repo, prepares .env,
# applies hardening, starts the stack and (optionally) restores a backup.
#
# Usage (as a sudo-capable user, see docs/DISASTER_RECOVERY.md):
#   sudo ops/bootstrap-new-server.sh [--ref vX.Y.Z] [--env-file path] [--restore-from backup-dir]
#        [--domain fqdn --acme-email addr] [--admin-user name] [--dry-run]
# Without --env-file a .env with random secrets is generated. It runs in *local mode* (Caddy on localhost with its
# internal CA, public listeners on loopback, 80/443 closed) unless --domain/--acme-email are given, so a server
# whose DNS does not point to it yet never requests Let's Encrypt certificates.
# --admin-user grants an existing sudo user (e.g. `ubuntu` on Oracle Cloud) ACL read/write on the repository and
# read-only on .env (group/other stay without access) so it can run git, make and docker compose; backups stay
# owned by APP_USER (run them with `sudo -u sentra`).
# Env: REPO_URL (default git@github.com:agungharyanto/sentra-dcim.git), APP_USER (default sentra),
#      APP_DIR (default /opt/sentra-dcim)
set -euo pipefail

REPO_URL="${REPO_URL:-git@github.com:agungharyanto/sentra-dcim.git}"
APP_USER="${APP_USER:-sentra}"
APP_DIR="${APP_DIR:-/opt/sentra-dcim}"
ref="main" env_file="" restore_from="" domain="" acme_email="" admin_user="" dry_run=0

while (($#)); do
  case "$1" in
    --ref) ref="${2:?}"; shift ;;
    --env-file) env_file="${2:?}"; shift ;;
    --restore-from) restore_from="${2:?}"; shift ;;
    --domain) domain="${2:?}"; shift ;;
    --acme-email) acme_email="${2:?}"; shift ;;
    --admin-user) admin_user="${2:?}"; shift ;;
    --dry-run) dry_run=1 ;;
    *) echo "unknown option $1" >&2; exit 2 ;;
  esac
  shift
done

step() { printf '\n==> %s\n' "$*"; }
run() { if ((dry_run)); then printf '    [dry-run] %s\n' "$*"; else "$@"; fi; }
as_app() { run sudo -u "$APP_USER" -H "$@"; }

[[ $EUID -eq 0 ]] || { echo "run with sudo" >&2; exit 1; }
[[ -z "$domain" || "$domain" =~ ^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$ ]] || { echo "invalid --domain" >&2; exit 2; }
[[ -z "$acme_email" || "$acme_email" =~ ^[^@[:space:]]+@[^@[:space:]]+$ ]] || { echo "invalid --acme-email" >&2; exit 2; }
[[ -n "$domain" && -z "$acme_email" ]] && { echo "--domain requires --acme-email" >&2; exit 2; }
[[ -z "$admin_user" ]] || id "$admin_user" >/dev/null 2>&1 || { echo "--admin-user $admin_user does not exist" >&2; exit 2; }
. /etc/os-release
[[ "$ID" == "ubuntu" && "$VERSION_ID" == "24.04" ]] || echo "WARNING: tested on Ubuntu 24.04 only (found $PRETTY_NAME)"

step "1/7 base packages"
run apt-get update -qq
run env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq ca-certificates curl git make jq ufw fail2ban unattended-upgrades openssl acl

step "2/7 Docker CE (official repository)"
if ! command -v docker >/dev/null 2>&1; then
  run install -m 0755 -d /etc/apt/keyrings
  run curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  run chmod a+r /etc/apt/keyrings/docker.asc
  if ((!dry_run)); then
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable" \
      >/etc/apt/sources.list.d/docker.list
  fi
  run apt-get update -qq
  run env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi
run systemctl enable --now docker

step "3/7 application user '$APP_USER'"
id "$APP_USER" >/dev/null 2>&1 || run useradd -m -s /bin/bash "$APP_USER"
run usermod -aG docker "$APP_USER"
run install -d -o "$APP_USER" -g "$APP_USER" -m 0755 "$APP_DIR"
run install -d -o "$APP_USER" -g "$APP_USER" -m 0700 /var/backups/sentra

step "4/7 clone $REPO_URL @ $ref"
if [[ -d "$APP_DIR/.git" ]]; then
  run chown -R "$APP_USER:$APP_USER" "$APP_DIR"
  as_app git -C "$APP_DIR" fetch --tags origin || echo "    WARNING: fetch failed (no key for $APP_USER?); using local refs"
else
  as_app git clone "$REPO_URL" "$APP_DIR"
fi
as_app git -C "$APP_DIR" checkout -q "$ref"

step "5/7 environment file"
if [[ -n "$env_file" ]]; then
  run install -o "$APP_USER" -g "$APP_USER" -m 0600 "$env_file" "$APP_DIR/.env"
elif [[ ! -f "$APP_DIR/.env" ]]; then
  if ((!dry_run)); then
    gen() { openssl rand -base64 48 | tr -d '/+=\n' | cut -c1-32; }
    if [[ -n "$domain" ]]; then
      mode=(-e "s/^DOMAIN=.*/DOMAIN=$domain/" -e "s/^ACME_EMAIL=.*/ACME_EMAIL=$acme_email/")
    else
      mode=(-e "s/^DOMAIN=.*/DOMAIN=localhost/" -e "s/^ACME_EMAIL=.*/ACME_EMAIL=admin@localhost/"
            -e "s/^CADDY_PUBLIC_HTTP_BIND=.*/CADDY_PUBLIC_HTTP_BIND=127.0.0.1:80/"
            -e "s/^CADDY_PUBLIC_HTTPS_BIND=.*/CADDY_PUBLIC_HTTPS_BIND=127.0.0.1:443/"
            -e "s/^UFW_OPEN_HTTP=.*/UFW_OPEN_HTTP=0/")
    fi
    (umask 077; sed -e "s/^POSTGRES_PASSWORD=CHANGE_ME/POSTGRES_PASSWORD=$(gen)/" \
        -e "s/^REDIS_PASSWORD=CHANGE_ME/REDIS_PASSWORD=$(gen)/" \
        -e "s|^MFA_ENC_KEY=$|MFA_ENC_KEY=$(openssl rand -base64 32)|" "${mode[@]}" "$APP_DIR/.env.example" >"$APP_DIR/.env")
    chown "$APP_USER:$APP_USER" "$APP_DIR/.env"
    chmod 0600 "$APP_DIR/.env"
  fi
  echo "    generated $APP_DIR/.env with random secrets (${domain:-local mode, DOMAIN=localhost}) — store a copy in the password vault"
fi
if ((!dry_run)) && grep -qE '^DOMAIN=sentra\.example\.go\.id$' "$APP_DIR/.env"; then
  echo "    WARNING: DOMAIN/ACME_EMAIL in $APP_DIR/.env are still placeholders; edit them, then: make -C $APP_DIR up"
fi
if [[ -n "$admin_user" ]]; then
  # Admin gets rwX on the repo (default ACLs keep new files shared both ways) and read-only on .env.
  run setfacl -R -m "u:$admin_user:rwX,d:u:$admin_user:rwX,d:u:$APP_USER:rwX" "$APP_DIR"
  run setfacl -b "$APP_DIR/.env"
  run chmod 0600 "$APP_DIR/.env"
  run setfacl -m "u:$admin_user:r" "$APP_DIR/.env"
  sudo -u "$admin_user" -H git config --global --get-all safe.directory 2>/dev/null | grep -qxF "$APP_DIR" ||
    run sudo -u "$admin_user" -H git config --global --add safe.directory "$APP_DIR"
  echo "    $admin_user: ACL rwX on $APP_DIR, read-only on .env; backups/restores run as: sudo -u $APP_USER -H ..."
fi

step "6/7 hardening + backup timer"
run "$APP_DIR/ops/harden/all.sh"
run "$APP_DIR/ops/install-backup-timer.sh"

step "7/7 start stack"
as_app make -C "$APP_DIR" up
if [[ -n "$restore_from" ]]; then
  as_app "$APP_DIR/ops/restore.sh" "$restore_from" --yes --no-safety-backup
fi
as_app make -C "$APP_DIR" test-api

step "done — Sentra DCIM $(sudo -u "$APP_USER" git -C "$APP_DIR" describe --tags --always 2>/dev/null || echo "$ref") is running"
