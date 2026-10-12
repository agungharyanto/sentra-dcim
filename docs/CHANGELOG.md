# Changelog

Format mengikuti [Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/) dan versi mengikuti konvensi proyek
`v0.<sprint>.<sub>` (lihat `CLAUDE.md`).

## [v0.1.5] - 2026-10-12 — CI Redis Auth (hotfix pasca-sprint)

### Diperbaiki
- CI: Redis kini berjalan dengan `requirepass` seperti produksi. Sebelumnya service Redis tanpa password membuat
  `TestRedis` gagal karena Redis 8 menerima password apa pun bila user `default` tanpa password (ISS-009).
- Integration test `TestRedis` gagal dengan pesan jelas bila `SENTRA_IT_REDIS_PASSWORD` kosong.

 - 2026-10-12 — Full Regression & DoD

### Ditambahkan
- `scripts/full-regression.sh`: 67 check regresi + DoD (static/secret scan, unit, integration, container, API,
  backup & restore ke DB kosong, migrasi up/down, permission, firewall & hardening, status repo).
- Release notes `docs/releases/v0.1.9.md` + log regresi & rollback.

### Diperbaiki
- `ops/rollback.sh`: versi asal dibaca dari `/api/v1/version` yang sedang berjalan (sebelumnya dari git, bisa salah
  bila checkout dilakukan manual sebelum skrip).

### Diuji
- Rollback v0.1.4 → v0.1.3 (16 dtk) dan kembali ke main (15 dtk); restore terisolasi & penuh; firewall dari server.

 - 2026-10-12 — Ops & DR

### Ditambahkan
- `ops/backup.sh`: `pg_dump` custom terkompresi + globals + volume `caddy_data`/`caddy_config`/`redis_data`,
  `manifest.json`, `SHA256SUMS` (diverifikasi), lock, retensi (default 14 hari), hook offsite (Pending), pembersihan
  direktori `.partial` saat gagal.
- `ops/restore.sh`: verifikasi checksum, restore penuh (safety backup otomatis, stop app/caddy, TimescaleDB
  pre/post restore, volume, migrate up, tunggu ready) atau restore terisolasi ke DB lain (`--target-db`).
- `ops/rollback.sh <tag|main>`: backup → migrate down dengan binary saat ini → checkout → `make up` → verifikasi
  `/ready` & `/version`; re-exec dari salinan sementara agar aman terhadap `git checkout`.
- `ops/bootstrap-new-server.sh`: Ubuntu 24.04 kosong → Docker CE, user `sentra`, clone, `.env`, hardening, timer
  backup, `make up`, restore opsional, smoke test; mode `--dry-run`.
- Timer systemd `sentra-backup.timer` (harian 02:00 WITA) + `ops/install-backup-timer.sh`, target `make backup-timer`.
- Migrasi `000002_ops_runs` (log operasi backup/restore/rollback) dengan `down`.
- GitHub Actions CI (`.github/workflows/ci.yml`): gitleaks, golangci-lint, ShellCheck, OpenAPI lint, `sqlc diff`,
  unit + integration test (service TimescaleDB & Redis), build image + cek non-root + API smoke test.
- `docs/DISASTER_RECOVERY.md` lengkap; Pusat Bantuan `docs/help/ops/backup-restore.md`.

 - 2026-10-12 — Backend Skeleton

### Ditambahkan
- Backend Go modular monolith: `cmd/sentra` dengan subcommand `serve`, `migrate up|down|goto|force|version`,
  `healthcheck`, `version`.
- Endpoint `GET /api/v1/health`, `GET /api/v1/ready` (cek database, redis, versi migrasi), `GET /api/v1/version`
  (versi dari tag Git via ldflags, waktu UTC + WITA).
- `internal/platform/`: config dari env (tervalidasi), logging slog JSON (UTC), pool pgx, klien Redis, migrator
  golang-migrate (migrasi di-embed ke binary), HTTP server (request ID, access log, recoverer JSON, security headers,
  batas body 1 MB, timeout, rate limit per IP, real-IP hanya dari proxy tepercaya).
- Graceful shutdown: SIGTERM → readiness 503 (draining) → `Shutdown` dengan timeout.
- Migrasi pertama `000001_init` (extension timescaledb, tabel `system_info`) dengan `down`.
- sqlc (`sqlc.yaml`, repo modul `system`), `api/openapi.yaml` (OpenAPI 3.1, lulus Redocly lint).
- Unit test (config, service, handler, middleware), integration test (migrasi up/down/up pada DB sementara, Redis),
  API smoke test `test/api/smoke.sh`; golangci-lint v2 (`.golangci.yml`).
- Pusat Bantuan: `docs/help/api/system.md`.
- Target Make: `openapi-lint`, `migrate-down/version/goto`, `test-integration`, `test-api`.

### Diubah
- Caddy tidak lagi memakai `trusted_proxies`; `X-Real-IP` di-set dari IP klien sebenarnya dan `True-Client-IP` dibuang
  (mencegah spoofing IP untuk bypass rate limit).
- `make gitleaks` memindai riwayat Git + file yang tidak di-ignore (sebelumnya `.env` lokal ikut terdeteksi).

 - 2026-10-12 — Infra Base

### Ditambahkan
- `docker-compose.yml`: service `db` (TimescaleDB 2.30.2-pg16), `redis` (8.10.2), `app`, `caddy` (2.11.7) dengan
  healthcheck, named volume, port bind `127.0.0.1`, batas resource, restart policy, `read_only`, `cap_drop: ALL`,
  `no-new-privileges`, dan semua container non-root.
- `Dockerfile` backend multi-stage → distroless `nonroot`; image Caddy turunan non-root di port 8080.
- Caddyfile dengan security headers (CSP, HSTS, X-Frame-Options, dll.) dan batas body 10 MB.
- `ops/harden/` idempoten: `ufw.sh`, `fail2ban.sh`, `sysctl.sh`, `unattended-upgrades.sh`, `all.sh`; target `make harden`.
- Pusat Bantuan: `docs/help/ops/infrastruktur.md`.
- Stub backend minimal (`/api/v1/health`) untuk validasi infra; diganti di v0.1.3.

 - 2026-10-12 — Repo Scaffold

### Ditambahkan
- Struktur folder modular monolith (`cmd/`, `internal/`, `api/`, `migrations/`, `deploy/`, `ops/`, `docs/`).
- `CLAUDE.md` berisi 12 aturan permanen proyek.
- `.gitignore`, `.editorconfig`, `.env.example`, `.gitleaks.toml`.
- `Makefile` dengan target `up/down/logs/test/lint/migrate/backup/restore/rollback/release` dan lainnya.
- Dokumen wajib: CHANGELOG, ISSUES, DEPENDENCIES, DECISIONS, ROADMAP, DISASTER_RECOVERY.
- Kerangka Pusat Bantuan `docs/help/README.md`.
- Pre-commit hook gitleaks (`scripts/hooks/pre-commit`, dipasang dengan `make hooks`).
