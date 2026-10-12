# Catatan Keputusan (Decision Log)

Status: **Accepted** (diputuskan), **Pending/Default** (belum diputuskan pemilik proyek; dipakai nilai default
sampai ada keputusan), **Superseded** (digantikan keputusan lain).

| ID | Tanggal | Keputusan | Status | Catatan |
|---|---|---|---|---|
| D-001 | 2026-10-12 | Backend Go (stable terbaru: 1.27), arsitektur modular monolith: `cmd/sentra`, `internal/<modul>/{handler,service,repo}`, `internal/platform/...` | Accepted | Sesuai brief Sprint v0.1 |
| D-002 | 2026-10-12 | Router `go-chi/chi v5`, DB driver `pgx v5` + `sqlc`, migrasi `golang-migrate`, log `log/slog` JSON | Accepted | |
| D-003 | 2026-10-12 | PostgreSQL 16 + TimescaleDB (image `timescale/timescaledb:2.30.2-pg16`), Redis, Caddy, Docker Compose | Accepted | |
| D-004 | 2026-10-12 | Image backend multi-stage, runtime `gcr.io/distroless/static-debian12:nonroot` (UID 65532) | Accepted | |
| D-005 | 2026-10-12 | Waktu disimpan UTC (`timestamptz`), ditampilkan Asia/Makassar (WITA, UTC+8) | Accepted | |
| D-006 | 2026-10-12 | Framework frontend | Pending/Default | Diputuskan di v0.2 |
| D-007 | 2026-10-12 | Allowlist IP admin / WireGuard untuk SSH & panel | Pending/Default | Variabel `SSH_ALLOW_FROM`, `WG_PORT` di `ops/harden/ufw.sh`; default: SSH terbuka dari mana saja (dilindungi fail2ban), WireGuard tidak dibuka |
| D-008 | 2026-10-12 | Port 80/443 ditutup di ufw sampai domain & TLS siap | Superseded (D-023) | Caddy hanya bind `127.0.0.1:8080` selama v0.1; buka dengan `OPEN_HTTP=1 ops/harden/ufw.sh` |
| D-009 | 2026-10-12 | Tujuan backup offsite | Pending/Default | Backup lokal di `/var/backups/sentra`; hook `BACKUP_OFFSITE_CMD` disiapkan tetapi kosong |
| D-010 | 2026-10-12 | Retensi backup lokal 14 hari | Pending/Default | Variabel `BACKUP_RETENTION_DAYS` |
| D-011 | 2026-10-12 | Redis 8 (lisensi tri: AGPLv3/RSALv2/SSPLv1) dipakai tanpa modifikasi sebagai cache/queue internal | Pending/Default | Alternatif lisensi BSD: Valkey (drop-in). Perlu konfirmasi kebijakan lisensi Diskominfo |
| D-012 | 2026-10-12 | Repo GitHub `agungharyanto/sentra-dcim`, push via deploy key SSH server | Accepted | |
| D-013 | 2026-10-12 | Domain produksi & sertifikat TLS | Superseded (D-023) | Caddyfile memakai `:80` internal sampai domain ditetapkan |
| D-014 | 2026-10-12 | Migrasi DB dijalankan oleh binary app (`sentra migrate up/down/goto`) memakai golang-migrate dengan file tertanam (embed), bukan container terpisah | Accepted | Image tetap distroless, tanpa shell |
| D-016 | 2026-10-12 | `chi/middleware.RealIP` tidak dipakai (deprecated, rentan spoofing). Middleware sendiri: `X-Real-IP` hanya dipercaya dari CIDR `APP_TRUSTED_PROXIES`; Caddy (edge) tidak mempercayai `X-Forwarded-For` klien | Accepted | |
| D-017 | 2026-10-12 | Rate limit in-memory per IP (20 rps, burst 40) selama app 1 replika | Pending/Default | Pindah ke Redis bila app di-scale >1 replika |
| D-018 | 2026-10-12 | `/health`, `/ready`, `/version` publik tanpa auth; detail error hanya di log | Accepted | Ditinjau ulang di v0.3 (Auth) |
| D-019 | 2026-10-12 | Backup otomatis harian 02:00 WITA via systemd timer (bukan cron/container) | Pending/Default | |
| D-020 | 2026-10-12 | Rollback memakai `ops/rollback.sh` dengan target minimum v0.1.3; schema down dijalankan oleh binary versi saat ini | Accepted | |
| D-021 | 2026-10-12 | `.env` tidak ikut backup; salinan disimpan di brankas password tim | Pending/Default | Alternatif: backup `.env` terenkripsi (age/gpg) |
| D-022 | 2026-10-12 | Hotfix setelah penutup sprint (`.9`) memakai slot cadangan kosong pertama (`.5–.8`) sprint tersebut | Pending/Default | v0.1.5 = hotfix CI |
| D-023 | 2026-10-12 | Domain `sentra.bajastu.id`, Let's Encrypt otomatis via Caddy (HTTP-01/TLS-ALPN-01), ufw buka 80/443; `DOMAIN`/`ACME_EMAIL` di `.env` | Accepted | Arahan pemilik proyek, v0.1.6 |
| D-024 | 2026-10-12 | Challenge DNS-01 sebagai cadangan bila 80/443 tak terjangkau | Pending/Default | Tidak dipakai; HTTP-01 berhasil. Butuh plugin caddy-dns + token API DNS |
| D-025 | 2026-10-12 | Sampai Auth (v0.3) hanya `GET/HEAD /api/v1/health` & `/api/v1/version` publik; lainnya 403 di proxy. API lengkap via listener internal `127.0.0.1:8080` | Accepted | |
| D-026 | 2026-10-12 | Caddy non-root bind 80/443 via sysctl `ip_unprivileged_port_start=0` per-container; HTTP/3 dimatikan | Accepted | Tanpa capability tambahan |
| D-015 | 2026-10-12 | Container tidak di-expose ke host kecuali Caddy di `127.0.0.1:8080`; app/DB/Redis hanya di network internal (DB & Redis tetap bisa diakses dari host via `127.0.0.1` untuk ops) | Accepted | Docker melewati ufw, jadi bind 127.0.0.1 wajib |
