# Changelog

Format mengikuti [Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/) dan versi mengikuti konvensi proyek
`v0.<sprint>.<sub>` (lihat `CLAUDE.md`).

## [v0.1.3] - 2026-10-12 — Backend Skeleton

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
