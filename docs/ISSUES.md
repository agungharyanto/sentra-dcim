# Daftar Issue

Format: ID, judul, prioritas (P1 kritis – P4 rendah), versi ditemukan, versi diperbaiki, status.

## Open

| ID | Judul | Prioritas | Ditemukan | Diperbaiki | Catatan |
|---|---|---|---|---|---|
| ISS-002 | Docker userland-proxy menyembunyikan IP klien asli (semua klien terlihat sebagai IP gateway) → rate limit & audit log per IP tidak akurat saat 80/443 dibuka | P2 | v0.1.3 | — | Wajib sebelum `OPEN_HTTP=1`: set `"userland-proxy": false` di `/etc/docker/daemon.json` atau Caddy `network_mode: host`. Target v0.2/v0.12 |
| ISS-003 | `bootstrap-new-server.sh` baru diuji `--dry-run`; belum dijalankan pada server Ubuntu 24.04 benar-benar kosong | P2 | v0.1.4 | — | Uji pada VM baru sebelum v1.0 (target v0.12) |
| ISS-010 | GitHub Release v0.1.9 belum dibuat: deploy key tidak bisa membuat Release, perlu `gh auth login` (token akun) | P3 | v0.1.9 | — | Lihat perintah di `docs/releases/v0.1.9.md` |
| ISS-005 | Uji firewall dari luar (nmap) belum dijalankan | P3 | v0.1.9 | — | Perintah ada di `docs/releases/v0.1.9.md`; dijalankan pemilik proyek dari jaringan lain |

## Solved

| ID | Judul | Prioritas | Ditemukan | Diperbaiki | Catatan |
|---|---|---|---|---|---|
| ISS-001 | Push ke GitHub ditolak: deploy key server belum terdaftar | P1 | v0.1.1 | v0.1.5 | Deploy key (write) terdaftar 2026-10-12; `main` + tag v0.1.1–v0.1.9 ter-push |
| ISS-004 | Workflow CI belum pernah berjalan di GitHub | P2 | v0.1.4 | v0.1.5 | Run pertama menemukan ISS-009 |
| ISS-009 | CI integration test gagal: service Redis tanpa password menerima password apa pun | P2 | v0.1.5 | v0.1.5 | CI menjalankan Redis dengan `requirepass` |
| ISS-006 | Label versi asal di log `rollback.sh` salah bila checkout manual dilakukan sebelum skrip | P4 | v0.1.9 | v0.1.9 | Versi asal kini dari `/api/v1/version` |
| ISS-007 | `trusted_proxies private_ranges` di Caddy memungkinkan spoofing IP via `X-Forwarded-For` dari klien yang masuk lewat docker-proxy | P2 | v0.1.3 | v0.1.3 | Caddy = edge, `trusted_proxies` dihapus; `X-Real-IP` di-set dari IP klien |
| ISS-008 | `backup.sh` meninggalkan direktori `.partial` bila gagal via `die` | P3 | v0.1.4 | v0.1.4 | Trap `EXIT` + pembersihan `.partial` lama |
