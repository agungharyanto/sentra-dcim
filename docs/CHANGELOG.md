# Changelog

Format mengikuti [Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/) dan versi mengikuti konvensi proyek
`v0.<sprint>.<sub>` (lihat `CLAUDE.md`).

## [v0.1.2] - 2026-10-12 — Infra Base

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
