# Roadmap Sentra DCIM

> **Scope terkunci.** Fitur baru hanya boleh masuk ke slot versi kosong (sub `.5–.8` atau sprint berikutnya) dan
> wajib dicatat di dokumen ini lebih dulu (CLAUDE.md aturan 10).

Konvensi versi: `v0.<sprint>.<sub>` — `.1–.4` kerja utama, `.5–.8` cadangan, `.9` penutup sprint (full regression + DoD).
Full regression **hanya** di `.9`; sub-versi `.1–.8` cukup regresi modul terkait (CLAUDE.md aturan 2–3, D-038).
Sprint = 1 minggu.

| Versi | Judul | Ruang lingkup | Status |
|---|---|---|---|
| v0.1 | Foundation | Scaffold repo, infra Docker, hardening server, backend skeleton (health/ready/version), Ops & DR (backup/restore/rollback/bootstrap), CI | Selesai (lihat docs/releases/v0.1.9.md) |
| v0.2 | UI Shell & Design System | Shell UI, design system, kerangka Pusat Bantuan; status menu *Available / Coming Soon / Not Connected*; warna status *Normal / Warning / Critical / Offline / Maintenance*; keputusan framework frontend; **i18n wajib sejak awal** (lihat rincian) | Berjalan (v0.2.1–v0.2.5 selesai) |
| v0.3 | Auth, RBAC & Audit Log | Login, sesi, RBAC (Admin, NOC, Security, Viewer, dll), audit log; detail `/api/v1/version` hanya role Admin; tambahan v0.3.5 Settings – Regional, v0.3.6 Settings – Allowed IP | Rencana |
| v0.4 | Facility, Tenant & Asset | Hierarki Site > Gedung > Lantai > Ruang > Rack, tenant, aset | Rencana |
| v0.5 | Dashboard & Alarm Engine | Dashboard, alarm (aktif / diakui / eskalasi / selesai), notifikasi | Rencana |
| v0.6 | Telemetry Ingestion | Adapter + simulator, time-series (TimescaleDB), deteksi offline vs stale | Rencana |
| v0.7 | Power & Environment | Power/PDU/UPS, environment (suhu, kelembapan, asap, kebocoran air), genset & tangki solar | Rencana |
| v0.8 | Access Control | Zona, validasi real-time, perlindungan data biometrik | Rencana |
| v0.9 | Visitor & Movement | Visitor pass, goods movement, cross-connect | Rencana |
| v0.10 | Maintenance & SLA | Maintenance, work order, SLA | Rencana |
| v0.11 | Analytics | Analitik, deteksi anomali, rekomendasi, ekspor laporan | Rencana |
| v0.12 | Hardening | Hardening, pentest, load test, optimasi query, uji restore | Rencana |
| v1.0 | Produksi | UAT, deployment produksi, training, as-built docs | Rencana |
| v1.1+ | Integrasi Perangkat Nyata | Integrasi Modbus, SNMP, controller pintu RFID/Fingerprint/Face nyata (menggantikan simulator) | Rencana |

## Rincian v0.1 Foundation

| Versi | Judul | Isi | Status |
|---|---|---|---|
| v0.1.1 | Repo Scaffold | Struktur folder, CLAUDE.md, .gitignore, .editorconfig, .env.example, Makefile, dokumen wajib, Pusat Bantuan README, pre-commit gitleaks | Selesai |
| v0.1.2 | Infra Base | docker-compose (timescaledb, redis, caddy, app), healthcheck, volume, bind 127.0.0.1, limit, restart; `ops/harden/*.sh` | Selesai |
| v0.1.3 | Backend Skeleton | `/api/v1/health`, `/ready`, `/version`, config env, graceful shutdown, migrasi pertama, OpenAPI, test, lint | Selesai |
| v0.1.4 | Ops & DR | backup/restore/rollback/bootstrap, GitHub Actions CI | Selesai |
| v0.1.5 | CI Redis Auth (hotfix) | Perbaikan CI pertama di GitHub | Selesai |
| v0.1.6 | Domain & HTTPS (tambahan scope) | `sentra.bajastu.id`, Let's Encrypt, ufw 80/443, redirect, HSTS & security headers, allowlist endpoint publik | Selesai |
| v0.1.7 | Caddy Empty Env Fix (hotfix) | Perbaikan CI v0.1.6 (DOMAIN/ACME_EMAIL kosong) | Selesai |
| v0.1.8 | Timezone & Version Fix | Zona tampilan `DISPLAY_TIMEZONE` (default Asia/Jakarta, DB tetap UTC), timer backup 02:00 Asia/Jakarta; `/api/v1/version` == `git describe` (`make up` selalu rebuild + `version-check`, dicek di full regression .9); `/version` publik hanya `{version}`, detail via listener internal. Termasuk pekerjaan migrasi server Oracle (akses darurat, bootstrap nyata — D-031) | Selesai |
| v0.1.9 | Foundation Close (Full Regression & DoD) | Semua test, uji restore, uji rollback, uji firewall, release notes, GitHub Release. Putaran 1 (2026-10-12, server lama). Putaran 2 (server Oracle): **go public** `sentra.bajastu.id` (Let's Encrypt, ufw + OCI Security List 80/443) + full regression & DoD lengkap — D-039 | Selesai (putaran 2: 72/72, go public, nmap luar, GitHub Release) |

## Rincian v0.2 UI Shell & Design System

| Versi | Judul | Isi | Status |
|---|---|---|---|
| v0.2.1 | Frontend Scaffold & i18n Foundation | `web/` (React 19, Vite, TS strict, Tailwind v4 + dasar shadcn, Router 7, TanStack Query; D-042/ADR-001), i18n ID/EN + lint `no-literal-string` + `i18n:check`, `GET /api/v1/ui/bootstrap` (registry menu backend), halaman sementara + ganti bahasa, Caddy SPA + CSP ketat, basic_auth sementara (D-043, ISS-026), e2e Playwright, kerangka `docs/help/{id,en}` | Selesai |
| v0.2.2 | Design System & Komponen | Token & tema (gelap default/terang/sistem), warna status DCIM & menu + uji kontras, font self-hosted, komponen shadcn/Radix + domain (StatusBadge/Dot, MenuStatusBadge, Theme/LanguageSwitcher), showcase `/design-system`, uji CSP Radix (ISS-027 → ADR-002/D-044), axe di e2e | Selesai |
| v0.2.3 | UI Shell | Layout (sidebar registry bergrup + ciut + drawer mobile, header breadcrumb/jam/bahasa/tema, skip link), routing dari registry (modul/Segera hadir/Belum terhubung, 404, error boundary), Dasbor empty state; kontrak bootstrap `menu_groups` + `group` (breaking) | Selesai |
| v0.2.4 | Pusat Bantuan di UI | Modul help (`/api/v1/help/*`, konten di-embed + divalidasi `go test`, 503 terisolasi bila konten rusak), halaman ID/EN dengan front matter, migrasi runbook ke `docs/ops/runbooks` & `docs/dev`, renderer Markdown aman, pencarian, tombol "?", viewer API read-only, govulncheck di CI, revisi CLAUDE.md aturan 2 & 7 | Selesai |
| v0.2.5 | Mitigasi DoS basic_auth (cadangan) | bcrypt cost 10, access log Caddy ke file (rotasi, retensi 30 hari), jail fail2ban `sentra-web` di `DOCKER-USER` (REJECT), drop-in systemd setelah Docker, uji filter & ban nyata (ISS-026, D-045) | Selesai |
| v0.2.9 | Penutup Sprint | Full regression (termasuk `test/e2e`, uji reboot server: fail2ban & DOCKER-USER pulih) + DoD | Rencana |

## Rincian v0.2 — wajib i18n sejak awal

Ditambahkan 2026-10-13 (D-035). Berlaku untuk seluruh UI mulai v0.2, bukan fitur terpisah:

- Bahasa **Indonesia (default)** dan **English**; struktur katalog terjemahan siap ditambah bahasa lain.
- **Tidak ada teks UI hardcode** — semua label, pesan, error, email/notifikasi lewat kunci terjemahan; lint/CI
  menolak string literal di komponen UI.
- Pesan error API memakai `code` stabil (sudah ada di `ErrorBody`) yang diterjemahkan di UI.
- **Locale & timezone default dibaca dari setting server** (sementara `.env`: `DISPLAY_TIMEZONE`; mulai v0.3.5 dari
  Settings – Regional). Format tanggal/jam/angka memakai locale aktif; data tetap UTC.
- Pusat Bantuan tersedia dalam ID (wajib) dan EN.

## Rincian v0.3 — slot tambahan

| Versi | Judul | Isi | Status |
|---|---|---|---|
| v0.3.5 | Settings – Regional | Zona waktu, format jam 12/24, format tanggal, bahasa sistem (default server) + preferensi per-user (override). API `/api/v1/settings/regional` (baca: semua user login; ubah sistem: Admin) dan `/api/v1/me/preferences`; audit log; Pusat Bantuan `docs/help/{id,en}/settings/regional.md`. Menggantikan `DISPLAY_TIMEZONE` di `.env` sebagai sumber utama (`.env` tetap jadi nilai awal) | Rencana |
| v0.3.6 | Settings – Allowed IP | Allowlist IP/CIDR untuk **SSH** dan **panel web**. CRUD via API `/api/v1/settings/allowed-ips` (role Admin) + audit log. Penerapan ke ufw oleh **agent host systemd** terpisah (bukan container app, bukan user aplikasi; D-037). Proteksi lockout: IP admin yang sedang aktif selalu diizinkan, perubahan diterapkan dengan **auto-rollback 2 menit** kecuali dikonfirmasi dari sesi baru. **Alamat MAC tidak dipakai** (D-036). Menggantikan `SSH_ALLOW_FROM`/`ADMIN_IPS` manual (D-007, D-030). Pusat Bantuan `docs/help/{id,en}/settings/allowed-ip.md` | Rencana |
