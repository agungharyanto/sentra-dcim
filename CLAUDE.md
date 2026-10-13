# CLAUDE.md — Aturan Permanen Proyek Sentra DCIM

Sentra DCIM adalah sistem Data Center Infrastructure Management untuk Diskominfo Provinsi Bali.
Aturan di bawah **wajib dipatuhi di setiap sesi**. Bila ada konflik, aturan ini menang atas kebiasaan default.

## Aturan

1. **Versi & sprint.** Sprint = 1 minggu. Versi `v0.<sprint>.<sub>`. Sub `.1–.4` = kerja utama,
   `.5–.8` = cadangan fitur tambahan, `.9` = penutup sprint. Produksi = `v1.0.0`.
   Integrasi perangkat IoT/pintu nyata = `v1.1+`; sebelumnya semua perangkat lewat **adapter interface + simulator**
   (siap tempel: Modbus, SNMP, controller pintu RFID/Fingerprint/Face).
2. **Alur tiap sub-versi.** Branch `feat/vX.Y.Z-<slug>` → commit (Conventional Commits) → regresi modul terkait →
   `merge --no-ff` ke `main` → tag annotated `vX.Y.Z "<Judul>"` → push `main` + tag.
   GitHub selalu sama/lebih maju dari server; `git status` bersih di akhir.
   Sub-versi `.1–.8` **hanya** regresi modul terkait — **tidak pernah** full regression (`scripts/full-regression.sh`).
   Bila verifikasi penuh dibutuhkan (mis. pindah server), kerjakan sebagai penutup sprint `.9`, bukan di sub-versi.
3. **Penutup sprint (.9) — satu-satunya tempat full regression.** FULL regression (unit + integrasi + API + smoke) + Definition of Done:
   semua file di GitHub, gitleaks lulus (tanpa secret), container healthy, API test lulus, migrasi up/down lulus,
   permission diuji, firewall diuji, backup + restore diuji, dokumentasi diperbarui, git status bersih,
   push & tag sukses. Hasil ditulis di `docs/releases/vX.Y.9.md` + GitHub Release.
4. **Secret tidak pernah masuk repo.** `.env`, `*.pem`, `*.key`, `credentials*`, dump backup ada di `.gitignore`;
   hanya `.env.example` yang di-commit. Pre-commit hook gitleaks wajib aktif (`make hooks`).
5. **Security first.** Port container bind ke `127.0.0.1` kecuali reverse proxy (Docker melewati ufw).
   Least privilege, container non-root, validasi input, query terparameter, rate limit, security headers, audit log.
6. **API first.** Setiap modul punya REST `/api/v1/...`, spesifikasi di `api/openapi.yaml`; UI hanya konsumen API;
   dokumentasi API tampil di Pusat Bantuan.
7. **Pusat Bantuan.** Setiap menu/fitur yang selesai wajib punya `docs/help/<modul>/<fitur>.md`
   (tujuan, langkah, role yang dibutuhkan, endpoint API terkait). Tanpa halaman bantuan = belum Done.
8. **Dokumen wajib:** `docs/CHANGELOG.md`, `docs/ISSUES.md` (open & solved + versi ditemukan/diperbaiki),
   `docs/DEPENDENCIES.md` (semua driver/library/framework/tools + versi + lisensi + fungsi), `docs/DECISIONS.md`,
   `docs/ROADMAP.md`, `docs/DISASTER_RECOVERY.md`.
9. **Rilis produksi wajib:** backup DB + volume penting, migrasi backward-compatible dengan `down`,
   prosedur rollback teruji, tag Git, changelog.
10. **Scope terkunci** = `docs/ROADMAP.md`. Fitur baru → slot versi kosong (.5–.8 atau sprint berikut), catat di ROADMAP.
11. **Laporan ke user singkat:** tag, hasil test, issue terbuka, keputusan yang dibutuhkan.
12. **Bahasa.** Komunikasi & dokumentasi (`docs/`, Pusat Bantuan, CHANGELOG, ISSUES) dalam Bahasa Indonesia.
    Kode, nama variabel, pesan commit, komentar kode dalam Bahasa Inggris.

## Ringkasan teknis

- **Stack:** Go (modular monolith), chi, pgx + sqlc, golang-migrate, slog JSON; PostgreSQL 16 + TimescaleDB, Redis,
  Caddy, Docker Compose. Lihat `docs/DECISIONS.md` dan `docs/DEPENDENCIES.md`.
- **Struktur:** `cmd/sentra` (entrypoint), `internal/<modul>/{handler,service,repo}`, `internal/platform/...`
  (config, db, cache, httpserver, logging), `migrations/`, `api/openapi.yaml`, `deploy/`, `ops/`, `docs/`.
- **Waktu:** simpan UTC, tampilkan menurut `DISPLAY_TIMEZONE` di `.env` (default Asia/Jakarta, WIB, UTC+7).
- **Perintah:** `make help` — `up/down/logs/test/lint/migrate/backup/restore/rollback/release`.
- **Server:** Oracle Cloud (Ubuntu 24.04 aarch64) sejak 2026-10-13; admin OS `ubuntu`, aplikasi milik user `sentra`,
  repo di `/opt/sentra-dcim`, backup di `/var/backups/sentra`; port sshd dibaca otomatis oleh `ops/harden/ufw.sh`.
  Akses darurat (serial console, key cadangan, unban): `docs/DISASTER_RECOVERY.md`. **Jangan ubah
  `PasswordAuthentication` tanpa konfirmasi pemilik.** Domain publik `https://sentra.bajastu.id` (Caddy + Let's Encrypt;
  mode lokal sampai DNS dipindah, ISS-015); hanya `/api/v1/health` & `/api/v1/version` (`{version}` saja) publik,
  API lengkap via `http://127.0.0.1:8080`.
