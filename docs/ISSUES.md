# Daftar Issue

Format: ID, judul, prioritas (P1 kritis – P4 rendah), versi ditemukan, versi diperbaiki, status.

## Open

| ID | Judul | Prioritas | Ditemukan | Diperbaiki | Catatan |
|---|---|---|---|---|---|
| ISS-019 | Server baru (Oracle) belum punya kredensial push ke GitHub: remote HTTPS tanpa token/deploy key, `git push` gagal | P1 | v0.1.8 | — | Commit & tag tersimpan lokal; push menunggu deploy key (write) atau token dari pemilik. Lihat laporan v0.1.8 |
| ISS-016 | Hanya satu key SSH terpasang, password `ubuntu` terkunci: serial console OCI belum bisa login | P1 | v0.1.8 | — | Keputusan pemilik: pasang key cadangan + set password konsol (prosedur di `docs/DISASTER_RECOVERY.md` → Akses darurat). `PasswordAuthentication` sudah `no` bawaan image Oracle — tidak diubah |
| ISS-015 | Sertifikat Let's Encrypt belum diterbitkan di server baru | P2 | v0.1.8 | — | DNS sudah → `132.226.106.158` (2026-10-13), ufw 80/443 terbuka. Menunggu pemilik membuka Ingress TCP 80 & 443 (0.0.0.0/0) di OCI Security List, lalu mode publik di v0.1.9 putaran 2 |
| ISS-010 | GitHub Release v0.1.9 belum dibuat: deploy key tidak bisa membuat Release, perlu `gh auth login` (token akun) | P3 | v0.1.9 | — | Lihat perintah di `docs/releases/v0.1.9.md` |
| ISS-005 | Uji firewall dari luar (nmap) belum dijalankan | P3 | v0.1.9 | — | Perintah di `docs/releases/v0.1.9.md`. Server Oracle (mode lokal): harapan hanya 22 open; setelah DNS: 22, 80, 443 |

## Solved

| ID | Judul | Prioritas | Ditemukan | Diperbaiki | Catatan |
|---|---|---|---|---|---|
| ISS-023 | v0.1.8 (sub-versi) menjalankan full regression 72/72 — menyalahi aturan: full regression hanya di `.9` | P3 | v0.1.8 | v0.1.9 | Aturan dipertegas di CLAUDE.md (D-038); catatan v0.1.8 dikoreksi; verifikasi penuh server Oracle dipindah ke v0.1.9 Foundation Close putaran 2 |
| ISS-022 | `/api/v1/version` bisa berbeda dari `git describe` HEAD: (a) `VERSION ?=` di Makefile tertimpa env `VERSION` (mis. setelah `. /etc/os-release` → `24.04.5 LTS (Noble Numbat)`); (b) setelah commit/tag container tidak di-rebuild | P2 | v0.1.7 | v0.1.8 | `VERSION :=`, `make up` = `build` + `up` + `version-check`; `release.sh` redeploy setelah tag; cek di full regression (internal, public, tag image) |
| ISS-003 | `bootstrap-new-server.sh` belum pernah dijalankan pada server Ubuntu 24.04 benar-benar kosong | P2 | v0.1.4 | v0.1.8 | Dijalankan nyata di VM Oracle baru (2 m 19 d): 4 container healthy, smoke 22/22, full regression baseline 69/70 (gagal = ISS-018). Temuan: ISS-017, ISS-018, ISS-020, ISS-021 |
| ISS-017 | Firewall host tidak efektif di Oracle: iptables bawaan image (`REJECT` di INPUT) dievaluasi sebelum chain ufw, jadi `ufw allow` selain 22 tidak berlaku | P2 | v0.1.8 | v0.1.8 | `ufw.sh` menghapus aturan itu (runtime + `/etc/iptables/rules.v4`, salinan `.sentra-orig`) |
| ISS-018 | `rpcbind` (port 111) bawaan image Oracle listen di semua interface | P3 | v0.1.8 | v0.1.8 | Diblokir ufw, tetapi kini dimatikan & di-mask oleh `ops/harden/services.sh` bila tidak ada NFS |
| ISS-020 | `.env` hasil bootstrap berisi domain placeholder + `UFW_OPEN_HTTP=1`: Caddy akan mencoba ACME untuk domain palsu dan 80/443 terbuka | P2 | v0.1.8 | v0.1.8 | `.env` otomatis kini mode lokal; mode publik lewat `--domain/--acme-email` |
| ISS-021 | Bootstrap: pesan akhir `git describe` gagal (*dubious ownership*, root vs repo milik `sentra`) | P4 | v0.1.8 | v0.1.8 | Dijalankan sebagai `sentra` |
| ISS-001 | Push ke GitHub ditolak: deploy key server belum terdaftar | P1 | v0.1.1 | v0.1.5 | Deploy key (write) terdaftar 2026-10-12; `main` + tag v0.1.1–v0.1.9 ter-push |
| ISS-004 | Workflow CI belum pernah berjalan di GitHub | P2 | v0.1.4 | v0.1.5 | Run pertama menemukan ISS-009 |
| ISS-009 | CI integration test gagal: service Redis tanpa password menerima password apa pun | P2 | v0.1.5 | v0.1.5 | CI menjalankan Redis dengan `requirepass` |
| ISS-002 | IP klien asli tersembunyi oleh docker userland-proxy saat 80/443 dibuka | P2 | v0.1.3 | v0.1.6 | Port publish `0.0.0.0` memakai DNAT iptables: access log Caddy mencatat IP eksternal asli (mis. validator Let's Encrypt). userland-proxy hanya dipakai untuk loopback |
| ISS-014 | CI build gagal sejak v0.1.6: Caddyfile tak valid bila `DOMAIN`/`ACME_EMAIL` kosong | P2 | v0.1.6 | v0.1.7 | Default di compose + `make caddy-validate` di CI/regresi. Lolos di server karena `.env` produksi terisi |
| ISS-011 | Redirect HTTP→HTTPS otomatis Caddy membocorkan header `Server: Caddy` | P4 | v0.1.6 | v0.1.6 | Blok `http://` eksplisit dengan `header -Server` |
| ISS-012 | `curl -I` (HEAD) ke endpoint GET akan 405 | P3 | v0.1.6 | v0.1.6 | `middleware.GetHead` |
| ISS-013 | `X-Request-Id` membocorkan ID container (hostname) | P4 | v0.1.6 | v0.1.6 | `hostname: sentra-app` |
| ISS-006 | Label versi asal di log `rollback.sh` salah bila checkout manual dilakukan sebelum skrip | P4 | v0.1.9 | v0.1.9 | Versi asal kini dari `/api/v1/version` |
| ISS-007 | `trusted_proxies private_ranges` di Caddy memungkinkan spoofing IP via `X-Forwarded-For` dari klien yang masuk lewat docker-proxy | P2 | v0.1.3 | v0.1.3 | Caddy = edge, `trusted_proxies` dihapus; `X-Real-IP` di-set dari IP klien |
| ISS-008 | `backup.sh` meninggalkan direktori `.partial` bila gagal via `die` | P3 | v0.1.4 | v0.1.4 | Trap `EXIT` + pembersihan `.partial` lama |
