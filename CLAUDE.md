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
   **Pemeriksaan visual screenshot wajib** di setiap regresi yang menyentuh UI: buka screenshot Playwright
   (`test/e2e/artifacts/`, artefak CI `ui-screenshots`) dan laporkan temuan — test fungsional & axe tidak menangkap
   masalah tata letak.
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
7. **Pusat Bantuan.** Halaman ada di `docs/help/{id,en}/<slug>/index.md` (+ `<slug>/<sub>.md`), front matter
   `title`, `audience` (`user`|`admin`), `menu` (key registry, opsional), `order`. **ID dan EN wajib setara** (slug
   identik). Setiap menu berstatus `available` wajib punya halaman dengan `menu: <key>` (tujuan, langkah, role,
   endpoint API terkait). Tanpa detail infrastruktur (IP, path server, `.env`, user OS). Dokumen developer/ops
   (runbook, deploy, recovery) di `docs/ops/runbooks/` & `docs/dev/`, **bukan** di Pusat Bantuan. Semua aturan ini
   divalidasi `go test ./internal/help/...` (CI gagal). Konvensi: `docs/help/README.md`. Tanpa halaman = belum Done.
8. **Dokumen wajib:** `docs/CHANGELOG.md`, `docs/ISSUES.md` (open & solved + versi ditemukan/diperbaiki),
   `docs/DEPENDENCIES.md` (semua driver/library/framework/tools + versi + lisensi + fungsi), `docs/DECISIONS.md`,
   `docs/ROADMAP.md`, `docs/DISASTER_RECOVERY.md`.
9. **Rilis produksi wajib:** backup DB + volume penting, migrasi backward-compatible dengan `down`,
   prosedur rollback teruji, tag Git, changelog.
10. **Scope terkunci** = `docs/ROADMAP.md`. Fitur baru → slot versi kosong (.5–.8 atau sprint berikut), catat di ROADMAP.
11. **Laporan ke user singkat:** tag, hasil test, issue terbuka, keputusan yang dibutuhkan.
12. **Bahasa.** Komunikasi & dokumentasi (`docs/`, Pusat Bantuan, CHANGELOG, ISSUES) dalam Bahasa Indonesia.
    Kode, nama variabel, pesan commit, komentar kode dalam Bahasa Inggris.
13. **Kode yang diuji = kode di GitHub.** Edit dilakukan di **git worktree terpisah** (`/opt/sentra-worktrees/<versi>`),
    tidak pernah di working tree server `/opt/sentra-dcim` (yang tetap di `main` bersih). Pemilik diminta menjalankan test
    **hanya setelah commit + push**. Setiap skrip test/ops membungkus isinya dalam `main "$@"` (bash mem-parse seluruh
    file sebelum jalan) dan baris pertama outputnya mencetak commit + `clean`/`DIRTY` (`scripts/lib/source-rev.sh`).
    Hasil dari kode `DIRTY` tidak sah sebagai bukti regresi (ISS-035).

## Ringkasan teknis

- **Stack:** Go (modular monolith), chi, pgx + sqlc, golang-migrate, slog JSON; PostgreSQL 16 + TimescaleDB, Redis,
  Caddy, Docker Compose. Lihat `docs/DECISIONS.md` dan `docs/DEPENDENCIES.md`.
- **Struktur:** `cmd/sentra` (entrypoint), `internal/<modul>/{handler,service,repo}`, `internal/platform/...`
  (config, db, cache, httpserver, logging), `migrations/`, `api/openapi.yaml`, `deploy/`, `ops/`, `docs/`.
- **Waktu:** simpan UTC, tampilkan menurut `DISPLAY_TIMEZONE` di `.env` (default Asia/Jakarta, WIB, UTC+7). Jam OS server = Asia/Jakarta (kosmetik, D-040); container & DB tetap UTC.
- **Perintah:** `make help` — `up/down/logs/test/lint/migrate/backup/restore/rollback/release`.
- **Server:** Oracle Cloud (Ubuntu 24.04 aarch64) sejak 2026-10-13; admin OS `ubuntu`, aplikasi milik user `sentra`,
  repo di `/opt/sentra-dcim`, backup di `/var/backups/sentra`; port sshd dibaca otomatis oleh `ops/harden/ufw.sh`.
  Akses darurat (serial console, key cadangan, unban): `docs/DISASTER_RECOVERY.md`. **Jangan ubah
  `PasswordAuthentication` tanpa konfirmasi pemilik.** Domain publik `https://sentra.bajastu.id` (Caddy + Let's Encrypt;
  publik sejak 2026-10-13; butuh ufw + OCI Security List 80/443); hanya `/api/v1/health` & `/api/v1/version` (`{version}` saja) publik,
  API lengkap via `http://127.0.0.1:8080`.
