# Changelog

Format mengikuti [Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/) dan versi mengikuti konvensi proyek
`v0.<sprint>.<sub>` (lihat `CLAUDE.md`).

## [v0.1.1] - 2026-10-12 — Repo Scaffold

### Ditambahkan
- Struktur folder modular monolith (`cmd/`, `internal/`, `api/`, `migrations/`, `deploy/`, `ops/`, `docs/`).
- `CLAUDE.md` berisi 12 aturan permanen proyek.
- `.gitignore`, `.editorconfig`, `.env.example`, `.gitleaks.toml`.
- `Makefile` dengan target `up/down/logs/test/lint/migrate/backup/restore/rollback/release` dan lainnya.
- Dokumen wajib: CHANGELOG, ISSUES, DEPENDENCIES, DECISIONS, ROADMAP, DISASTER_RECOVERY.
- Kerangka Pusat Bantuan `docs/help/README.md`.
- Pre-commit hook gitleaks (`scripts/hooks/pre-commit`, dipasang dengan `make hooks`).
