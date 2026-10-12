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
| `caddy` | sentra-caddy (turunan caddy:2.11.7-alpine) | `127.0.0.1:8080` | Reverse proxy + security headers, user 65532 |

Semua container: `read_only`, `no-new-privileges`, `cap_drop: ALL`, batas CPU/memori/PID, `restart: unless-stopped`,
healthcheck, rotasi log 10 MB × 5. Data disimpan di named volume `db_data`, `redis_data`, `caddy_data`, `caddy_config`.

> **Penting:** Docker melewati ufw. Port container **wajib** bind ke `127.0.0.1`; jangan menulis `"8080:8080"` tanpa IP.

## Langkah-langkah

1. Siapkan `.env`: `cp .env.example .env && chmod 600 .env`, isi semua `CHANGE_ME`.
2. Jalankan stack: `make up`; cek status: `make ps` (semua harus `healthy`).
3. Terapkan hardening: `make harden` (= `sudo ops/harden/all.sh`). Aman dijalankan berulang (idempoten).

| Skrip | Fungsi |
|---|---|
| `ops/harden/ufw.sh` | Default deny incoming; hanya port sshd aktif (dideteksi otomatis dari `sshd -T`) yang dibuka. 80/443 tertutup |
| `ops/harden/fail2ban.sh` | Jail `sshd` (mode aggressive), ban 1 jam bertambah s.d. 1 minggu, maks 5 gagal / 10 menit |
| `ops/harden/sysctl.sh` | Anti-spoofing, tanpa ICMP redirect/source route, SYN cookies, pembatasan kernel pointer/dmesg/BPF |
| `ops/harden/unattended-upgrades.sh` | Update keamanan otomatis harian, tanpa reboot otomatis |

### Variabel opsional (keputusan Pending D-007/D-008)

```bash
sudo SSH_ALLOW_FROM="203.0.113.10/32,10.8.0.0/24" ops/harden/ufw.sh   # SSH hanya dari allowlist
sudo WG_PORT=51820 ops/harden/ufw.sh                                  # buka WireGuard
sudo OPEN_HTTP=1 ops/harden/ufw.sh                                    # buka 80/443 saat domain siap
```

## Endpoint API terkait
| Metode | Path | Keterangan |
|---|---|---|
| GET | `/api/v1/health` | Dipakai healthcheck Caddy & container (lihat [api/system.md](../api/system.md)) |

## Pemecahan masalah
- Container `unhealthy`: `make logs SERVICE=<nama>`.
- Terkunci SSH: ufw selalu meng-allow port sshd sebelum `enable`; bila memakai allowlist, pastikan IP Anda termasuk.
