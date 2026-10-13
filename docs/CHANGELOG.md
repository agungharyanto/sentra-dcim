# Changelog

Format mengikuti [Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/) dan versi mengikuti konvensi proyek
`v0.<sprint>.<sub>` (lihat `CLAUDE.md`).

## [v0.2.2] - 2026-10-13 — Design System & Komponen

### Ditambahkan
- Token desain semantik (`web/src/styles/tokens.css`): tema **gelap (default)**, terang, dan ikuti sistem; status
  DCIM normal/warning/critical/offline/maintenance (hijau/amber/merah/abu-abu/ungu; biru khusus link/info); status
  menu available/coming_soon/not_connected.
- Uji kontras otomatis dari `tokens.css` (139 pasangan, kedua tema): teks ≥ 4.5:1, non-teks (StatusDot, border
  badge, indikator fokus, batas input) ≥ 3:1.
- Font Inter + JetBrains Mono self-hosted (variable, subset latin), tanpa CDN.
- Komponen shadcn/ui di repo: Button, Card, Badge, Input, Select, Tabs, Table, Tooltip, DropdownMenu, Dialog,
  Sheet, Skeleton, Toast (sonner). Komponen domain: StatusBadge, StatusDot (ikon + label, tidak warna saja),
  MenuStatusBadge, ThemeSwitcher, LanguageSwitcher, Logo (placeholder, satu titik ganti `assets/brand/logo.svg`).
- Halaman internal `/design-system` (di luar registry menu, dimuat terpisah/lazy) dalam dua tema dan dua bahasa.
- `test/e2e/design-system.spec.ts`: kedua tema, Dialog/Sheet/Tooltip/Select/Dropdown/Toast, ID↔EN, console &
  CSP bersih, **axe-core** 0 pelanggaran serious/critical; screenshot diunggah sebagai artefak CI.
- `docs/design-system.md`, ADR-002, D-044.

### Diubah
- CSP respons UI: `style-src 'self' 'unsafe-inline'` (khusus style; `script-src` tetap `'self'`) setelah Playwright
  membuktikan Radix/sonner diblokir (ISS-027 selesai; nonce → ISS-030).
- ESLint `no-literal-string`: selain teks JSX, hanya atribut yang terlihat pengguna (`title`, `alt`, `placeholder`,
  `aria-label`, …) yang diperiksa; prop teknis (`variant`, `value`) tidak.
- Favicon kini memakai `assets/brand/logo.svg` (satu file logo).
- UI menghormati `prefers-reduced-motion` (transisi/animasi dimatikan, WCAG 2.3.3); e2e berjalan dengan reduced
  motion agar axe mengukur warna yang sudah stabil (sempat gagal di CI saat transisi tombol bahasa).

### Keamanan
- Kebijakan supply chain pnpm `minimumReleaseAge` kini benar-benar ditegakkan: pengecualian otomatis yang ditulis
  pnpm (sejak v0.2.1) dihapus dan paket dipatok ke rilis > 24 jam — `radix-ui 1.8.0`, `lucide-react 1.55.0`,
  `typescript-eslint 8.71.1`, `@testing-library/user-event 14.6.7` (ISS-031).

### Keputusan pemilik dicatat
- ROADMAP v0.2.2–v0.2.4, v0.2.9 final; fail2ban basic_auth dijadwalkan v0.2.5 (ISS-026). ISS-029: `/design-system`
  dibatasi role admin/dev di v0.3.

## [v0.2.1] - 2026-10-13 — Frontend Scaffold & i18n Foundation

### Ditambahkan
- UI web di `web/`: React 19 + Vite 8 + TypeScript strict, Tailwind v4 + dasar shadcn/ui, React Router 7, TanStack
  Query (D-042, [ADR-001](decisions/ADR-001-frontend-stack.md)). Build hanya di Docker multi-stage (Node 22, ARM64);
  host tidak butuh Node.
- i18n sejak awal: Bahasa Indonesia (default) & English, namespace `common`/`shell`/`errors`, key bertipe;
  ESLint `i18next/no-literal-string` = error; `pnpm i18n:check` menolak key yang hilang/berlebih.
- Endpoint `GET /api/v1/ui/bootstrap`: nama app, versi, locale & timezone default dari server (`DEFAULT_LOCALE`
  baru, `DISPLAY_TIMEZONE`), registry menu tunggal di backend (Dashboard *available*, modul lain *coming soon*;
  label diterjemahkan di UI). OpenAPI diperbarui.
- Halaman sementara: nama app, versi, bahasa & zona waktu aktif, jam sekarang (UTC → zona tampilan), daftar menu,
  tombol ganti bahasa ID/EN.
- Caddy menyajikan UI di `/` dengan SPA fallback, CSP ketat tanpa `unsafe-inline`, `index.html` `no-cache`, aset
  ber-hash `immutable`.
- **basic_auth sementara** untuk UI & bootstrap sampai Auth v0.3 (D-043, ISS-026). `make preflight` menolak mode
  publik tanpa hash; tanpa hash UI terkunci.
- Test: Vitest (10), Go unit test bootstrap, `test/api/ui.sh` (edge UI), `test/e2e` Playwright (ID↔EN, console &
  CSP), `make web-check`, `make test-ui`, `make test-e2e`; job CI `web` + uji UI & e2e di job build.
- Kerangka Pusat Bantuan dua bahasa `docs/help/{id,en}/` (diisi di v0.2.4).

### Regresi modul v0.2.1
- Web: lint, typecheck, `i18n:check`, Vitest 10/10 (+ uji negatif lint & katalog); Go unit test + golangci-lint 0 issue.
- Server produksi: 4 container healthy; smoke internal 27/27; edge publik 30/30; `test-ui` 32/32 dan `test-e2e`
  Playwright 2/2 (dijalankan pemilik); browser: login, ID↔EN, deep link, console bersih, jam WIB benar.
- CI GitHub (run `38390860588`): semua job lulus, termasuk UI edge 32/32 dan Playwright 2/2.

### Diperbaiki
- Respons error dari handler Caddy (401 basic_auth, 404/405 statis) kini tetap membawa header keamanan (HSTS,
  nosniff, tanpa `Server`) dan body `ErrorBody` JSON (`handle_errors`) — ditemukan saat uji `curl -I` v0.2.1.
- Header CSP ganda (Caddy + app) pada respons API: CSP dari app dibuang di proxy, tinggal satu kebijakan.

### Diubah
- `test/api/public.sh`: `/` dan `/admin` kini 401 (basic_auth), bukan 403. API selain health/version tetap 403.
- Image Caddy kini ditag per versi (`sentra-caddy:<git describe>`), sama seperti app.
- `make caddy-validate` memvalidasi Caddyfile dengan image Caddy resmi (tanpa build UI).

## [v0.1.9 putaran 2] - 2026-10-13 — Foundation Close (tanpa tag baru)

### Diperbaiki
- Uji firewall dari luar lulus: hanya 22/80/443 terbuka; `53/tcp` di nmap terbukti false positive ISP (ISS-005).
- GitHub Release v0.1.9 dibuat pada tag lama `8aeaf87` (ISS-010).
- Akses darurat server Oracle: key SSH cadangan terpasang dan password konsol `ubuntu` disetel
  pemilik, sehingga serial console OCI bisa dipakai (ISS-016). `docs/DISASTER_RECOVERY.md` kini memuat checklist
  langkah demi langkah "kalau SSH hilang" dan cara aman mengubah `ADMIN_IPS`.
- Push ke GitHub dari server Oracle via deploy key baru `sentra-oracle-deploy` (ISS-019, D-041); key lama
  `sentra-dcim-server` tidak dipakai lagi.
- `ops/backup.sh` tidak lagi meng-`chown` direktori backup yang sudah ada bila dijalankan user lain; kini menolak
  dengan petunjuk `sudo -u <pemilik>` (ISS-024).

### Diubah
- Fingerprint key tidak lagi dicatat di repo (D-041, `docs/DISASTER_RECOVERY.md`); disimpan di brankas pemilik.
- Aturan regresi dipertegas (CLAUDE.md aturan 2–3, D-038): full regression **hanya** di penutup sprint `.9`;
  sub-versi `.1–.8` cukup regresi modul terkait. Catatan rilis v0.1.8 dikoreksi — hasil 72/72 dicatat sebagai
  verifikasi migrasi server, bukan full regression (ISS-023).
- Go public di server Oracle (D-039): DNS `sentra.bajastu.id` → `132.226.106.158`; ufw membuka 80/443
  (`UFW_OPEN_HTTP=1`), OCI Security List 80/443, `.env` mode publik. Sertifikat Let's Encrypt terbit; edge test
  publik 26/26 (ISS-015 selesai). Rincian: addendum `docs/releases/v0.1.9.md`.
- Jam OS server kini `Asia/Jakarta` (WIB) — kosmetik untuk log host (D-040); container, DB & penyimpanan tetap UTC,
  backup tetap 02:00 WIB.

## [v0.1.8] - 2026-10-13 — Timezone & Version Fix

### Diubah
- Zona waktu tampilan kini `DISPLAY_TIMEZONE` di `.env`, default **Asia/Jakarta (WIB, UTC+7)**; database & server
  tetap UTC. Nama lama `APP_DISPLAY_TZ` masih dibaca sebagai fallback (D-032).
- Timer backup harian kini 02:00 **Asia/Jakarta** (`OnCalendar=*-*-* 02:00:00 Asia/Jakarta` = 19:00 UTC).
- **Keamanan:** `/api/v1/version` publik hanya mengembalikan `{"version"}`. Commit, tanggal build, versi Go, jam
  server & zona tampilan hanya dari listener internal app (`:8081`, di-proxy Caddy `127.0.0.1:8080` saja); v0.3
  menambah syarat role Admin (D-033). Respons `/version` kini `Cache-Control: no-store`.
- Caddy: situs publik → `app:8080`, listener internal → `app:8081`.
- Dokumen yang menyebut WITA/Asia/Makassar diperbarui ke WIB/Asia/Jakarta (entri CHANGELOG & release notes lama
  dibiarkan sebagai catatan sejarah).

### Diperbaiki
- `/api/v1/version` bisa berbeda dari `git describe` HEAD (ISS-022). `VERSION`/`COMMIT` di Makefile tidak bisa lagi
  ditimpa variabel environment; `make up` selalu `docker compose build app caddy` lalu `make version-check`;
  `scripts/release.sh` me-redeploy stack setelah membuat tag.

### Ditambahkan
- `make version-check`; full regression memeriksa versi berjalan == `git describe` (internal & publik) dan tag
  image `sentra-app:<versi>`; smoke test memeriksa zona & offset sesuai `DISPLAY_TIMEZONE`.
- ROADMAP: i18n wajib sejak v0.2 (D-035); v0.3.5 *Settings – Regional*; v0.3.6 *Settings – Allowed IP*
  (agent host, anti-lockout, MAC tidak dipakai — D-036, D-037).

### Migrasi server ke Oracle Cloud (pekerjaan pra-v0.1.8, D-031)
- Akses darurat di `docs/DISASTER_RECOVERY.md` + Pusat Bantuan `ops/akses-darurat.md`: OCI Serial/VNC Console,
  key SSH cadangan, aturan perubahan SSH (`PasswordAuthentication` tidak diubah tanpa konfirmasi), unban fail2ban.
- fail2ban tidak pernah mem-ban `ADMIN_IPS` dan IP sesi SSH aktif (D-030).
- `ufw.sh` menghapus aturan iptables `REJECT` bawaan image Oracle yang membuat ufw tidak efektif (ISS-017);
  `ops/harden/services.sh` mematikan `rpcbind` (ISS-018).
- Bootstrap: `.env` otomatis dalam mode lokal (tanpa ACME) kecuali `--domain/--acme-email`; `--admin-user` (ACL);
  diuji nyata di server kosong — ISS-003 selesai. `test/api/public.sh` punya mode lokal (CA internal Caddy).

## [v0.1.7] - 2026-10-12 — Caddy Empty Env Fix

### Diperbaiki
- CI gagal sejak v0.1.6: Caddyfile tidak bisa diparse bila `DOMAIN`/`ACME_EMAIL` kosong, karena default
  `{$VAR:default}` Caddy hanya berlaku untuk variabel yang tidak di-set (ISS-014). Default kini di
  `docker-compose.yml` (`localhost`, `admin@localhost`).

### Ditambahkan
- `make preflight` (dipanggil `make up`): menolak `DOMAIN` asli tanpa `ACME_EMAIL`.
- `make caddy-validate`: validasi Caddyfile untuk env produksi & kosong; dijalankan di CI dan full regression.

 - 2026-10-12 — Domain & HTTPS

### Ditambahkan
- HTTPS publik `https://sentra.bajastu.id` via Caddy dengan sertifikat Let's Encrypt otomatis (HTTP-01/TLS-ALPN-01).
  `DOMAIN` dan `ACME_EMAIL` di `.env` (contoh di `.env.example`).
- Redirect HTTP → HTTPS (308) eksplisit; HSTS 1 tahun, CSP, X-Frame-Options, X-Content-Type-Options,
  Referrer-Policy, Permissions-Policy, CORP/COOP; header `Server` & `Via` disembunyikan (termasuk pada redirect).
- Allowlist proxy: hanya `GET`/`HEAD` `/api/v1/health` dan `/api/v1/version` yang publik; lainnya `403` JSON.
- Listener internal `127.0.0.1:8080` (API lengkap) untuk skrip ops dan smoke test.
- `test/api/public.sh`: 25 uji edge publik (TLS issuer & masa berlaku, TLS 1.1 ditolak, headers, allowlist,
  path traversal, redirect); masuk ke `scripts/full-regression.sh`.
- Pusat Bantuan: `docs/help/admin/domain-https.md`.

### Dokumentasi
- DECISIONS D-022 (hotfix pasca-sprint memakai slot cadangan), tertinggal dari commit v0.1.5.

### Diubah
- ufw membuka 80/tcp & 443/tcp; `ops/harden/ufw.sh` membaca `UFW_OPEN_HTTP` dari `.env` (re-run tidak menutup situs).
- Caddy non-root bind 80/443 lewat sysctl container `net.ipv4.ip_unprivileged_port_start=0` (tanpa capability);
  HTTP/3 dimatikan (hanya TCP yang dibuka).
- Backend melayani `HEAD` untuk route `GET` (`curl -I` sebelumnya akan 405).
- Container app memakai `hostname: sentra-app` agar `X-Request-Id` tidak membocorkan ID container.
- CI memakai `DOMAIN` kosong (localhost) dan bind loopback.

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
