# Infrastruktur & Hardening Server

**Modul:** ops · **Tersedia sejak:** v0.1.2 · **Status:** Available (khusus administrator server)

## Tujuan
Menjelaskan komponen container Sentra DCIM dan cara menerapkan hardening server Ubuntu 24.04.

## Role yang dibutuhkan
| Aksi | Role |
|---|---|
| Menjalankan/menghentikan container | Administrator server (user Linux grup `docker`) |
| Menjalankan hardening | Administrator server dengan `sudo` |

## Komponen container

| Service | Image | Port host | Keterangan |
|---|---|---|---|
| `db` | timescale/timescaledb:2.30.2-pg16 | `127.0.0.1:5432` | PostgreSQL 16 + TimescaleDB, user `postgres` (70) |
| `redis` | redis:8.10.2-alpine | `127.0.0.1:6379` | Wajib password, AOF aktif, user `redis` (999) |
| `app` | sentra-app (distroless) | — (internal) | Backend Go, user `nonroot` (65532) |
| `caddy` | sentra-caddy:`<versi>` (turunan caddy:2.11.7-alpine + UI web statis dari `web/`) | `0.0.0.0:80`, `0.0.0.0:443` (publik), `127.0.0.1:8080` (internal) | Reverse proxy, TLS Let's Encrypt, security headers, UI web di `/` (basic_auth sementara, [admin/web-basic-auth.md](../admin/web-basic-auth.md)), user 65532 — lihat [admin/domain-https.md](../admin/domain-https.md) |
| `web` | sentra-web:`<versi>` | — | **Hanya build** (profil `build`, tidak pernah dijalankan): artefak UI yang disalin ke image caddy |

Semua container: `read_only`, `no-new-privileges`, `cap_drop: ALL`, batas CPU/memori/PID, `restart: unless-stopped`,
healthcheck, rotasi log 10 MB × 5. Data disimpan di named volume `db_data`, `redis_data`, `caddy_data`, `caddy_config`.

> **Penting:** Docker melewati ufw. Port container **wajib** bind ke `127.0.0.1`; jangan menulis `"8080:8080"` tanpa IP.
> Satu-satunya pengecualian: Caddy 80/443 (reverse proxy publik).

## Langkah-langkah

1. Siapkan `.env`: `cp .env.example .env && chmod 600 .env`, isi semua `CHANGE_ME`.
2. Jalankan stack: `make up`; cek status: `make ps` (semua harus `healthy`).
3. Terapkan hardening: `make harden` (= `sudo ops/harden/all.sh`). Aman dijalankan berulang (idempoten).

| Skrip | Fungsi |
|---|---|
| `ops/harden/services.sh` | Mematikan & mem-mask `rpcbind` (port 111, bawaan image Oracle) bila tidak ada NFS |
| `ops/harden/ufw.sh` | Default deny incoming; port sshd aktif (dideteksi otomatis dari `sshd -T`) + 80/443 bila `UFW_OPEN_HTTP=1` di `.env`. Di Oracle Cloud menghapus aturan iptables `REJECT` bawaan image yang menutupi ufw |
| `ops/harden/fail2ban.sh` | Jail `sshd` (mode aggressive), ban 1 jam bertambah s.d. 1 minggu, maks 5 gagal / 10 menit. Tidak pernah mem-ban `ADMIN_IPS` (`.env`) dan IP sesi SSH yang aktif saat dijalankan. Cara unban: `docs/DISASTER_RECOVERY.md` → Akses darurat |
| `ops/harden/sysctl.sh` | Anti-spoofing, tanpa ICMP redirect/source route, SYN cookies, pembatasan kernel pointer/dmesg/BPF |
| `ops/harden/unattended-upgrades.sh` | Update keamanan otomatis harian, tanpa reboot otomatis |

### Variabel opsional (keputusan Pending D-007/D-008)

```bash
sudo SSH_ALLOW_FROM="203.0.113.10/32,10.8.0.0/24" ops/harden/ufw.sh   # SSH hanya dari allowlist
sudo WG_PORT=51820 ops/harden/ufw.sh                                  # buka WireGuard
sudo OPEN_HTTP=0 ops/harden/ufw.sh                                    # tutup 80/443 sementara (override .env)
```

## Endpoint API terkait
| Metode | Path | Keterangan |
|---|---|---|
| GET | `/api/v1/health` | Dipakai healthcheck Caddy & container (lihat [api/system.md](../api/system.md)) |

## Pemecahan masalah
- Container `unhealthy`: `make logs SERVICE=<nama>`.
- Terkunci SSH: ufw selalu meng-allow port sshd sebelum `enable`; bila memakai allowlist, pastikan IP Anda termasuk.
