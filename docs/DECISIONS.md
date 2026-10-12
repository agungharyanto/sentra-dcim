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
| D-008 | 2026-10-12 | Port 80/443 ditutup di ufw sampai domain & TLS siap | Pending/Default | Caddy hanya bind `127.0.0.1:8080` selama v0.1; buka dengan `OPEN_HTTP=1 ops/harden/ufw.sh` |
| D-009 | 2026-10-12 | Tujuan backup offsite | Pending/Default | Backup lokal di `/var/backups/sentra`; hook `BACKUP_OFFSITE_CMD` disiapkan tetapi kosong |
| D-010 | 2026-10-12 | Retensi backup lokal 14 hari | Pending/Default | Variabel `BACKUP_RETENTION_DAYS` |
| D-011 | 2026-10-12 | Redis 8 (lisensi tri: AGPLv3/RSALv2/SSPLv1) dipakai tanpa modifikasi sebagai cache/queue internal | Pending/Default | Alternatif lisensi BSD: Valkey (drop-in). Perlu konfirmasi kebijakan lisensi Diskominfo |
| D-012 | 2026-10-12 | Repo GitHub `agungharyanto/sentra-dcim`, push via deploy key SSH server | Accepted | |
| D-013 | 2026-10-12 | Domain produksi & sertifikat TLS | Pending/Default | Caddyfile memakai `:80` internal sampai domain ditetapkan |
| D-014 | 2026-10-12 | Migrasi DB dijalankan oleh binary app (`sentra migrate up/down/goto`) memakai golang-migrate dengan file tertanam (embed), bukan container terpisah | Accepted | Image tetap distroless, tanpa shell |
| D-015 | 2026-10-12 | Container tidak di-expose ke host kecuali Caddy di `127.0.0.1:8080`; app/DB/Redis hanya di network internal (DB & Redis tetap bisa diakses dari host via `127.0.0.1` untuk ops) | Accepted | Docker melewati ufw, jadi bind 127.0.0.1 wajib |
