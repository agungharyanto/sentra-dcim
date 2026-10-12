# Roadmap Sentra DCIM

> **Scope terkunci.** Fitur baru hanya boleh masuk ke slot versi kosong (sub `.5–.8` atau sprint berikutnya) dan
> wajib dicatat di dokumen ini lebih dulu (CLAUDE.md aturan 10).

Konvensi versi: `v0.<sprint>.<sub>` — `.1–.4` kerja utama, `.5–.8` cadangan, `.9` penutup sprint (full regression + DoD).
Sprint = 1 minggu.

| Versi | Judul | Ruang lingkup | Status |
|---|---|---|---|
| v0.1 | Foundation | Scaffold repo, infra Docker, hardening server, backend skeleton (health/ready/version), Ops & DR (backup/restore/rollback/bootstrap), CI | Selesai (lihat docs/releases/v0.1.9.md) |
| v0.2 | UI Shell & Design System | Shell UI, design system, kerangka Pusat Bantuan; status menu *Available / Coming Soon / Not Connected*; warna status *Normal / Warning / Critical / Offline / Maintenance*; keputusan framework frontend | Rencana |
| v0.3 | Auth, RBAC & Audit Log | Login, sesi, RBAC (Admin, NOC, Security, Viewer, dll), audit log | Rencana |
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
| v0.1.5–v0.1.8 | (cadangan) | — | Kosong |
| v0.1.9 | Full Regression & DoD | Semua test, uji restore, uji rollback, uji firewall, release notes, GitHub Release | Selesai (push & GitHub Release menunggu ISS-001) |
