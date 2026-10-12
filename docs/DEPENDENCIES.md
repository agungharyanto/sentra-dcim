# Daftar Dependensi

Semua driver, library, framework, dan tools yang dipakai Sentra DCIM. Perbarui tabel ini setiap kali dependensi
ditambah, diubah versinya, atau dihapus.

## Runtime & infrastruktur

| Nama | Versi | Lisensi | Fungsi |
|---|---|---|---|
| Ubuntu Server | 24.04 LTS | Berbagai (GPL dll.) | Sistem operasi server |
| Docker Engine | 29.9.0 | Apache-2.0 | Runtime container |
| Docker Compose | 5.6.0 (plugin) | Apache-2.0 | Orkestrasi container lokal/server |
| TimescaleDB (`timescale/timescaledb:2.30.2-pg16`) | 2.30.2 / PostgreSQL 16 | Timescale License (TSL) + Apache-2.0 (core); PostgreSQL License | Database relasional + time-series |
| Redis (`redis:8.10.2-alpine`) | 8.10.2 | AGPLv3 / RSALv2 / SSPLv1 (tri-license) | Cache, rate-limit store, queue ringan (lihat D-011) |
| Caddy (`caddy:2.11.7-alpine`) | 2.11.7 | Apache-2.0 | Reverse proxy, TLS otomatis, security headers |
| Distroless static (`gcr.io/distroless/static-debian12:nonroot`) | debian12 | Apache-2.0 | Base image runtime backend (non-root, tanpa shell) |

## Backend (Go)

| Nama | Versi | Lisensi | Fungsi |
|---|---|---|---|
| Go | 1.27.2 | BSD-3-Clause | Bahasa & toolchain backend |
| github.com/go-chi/chi/v5 | 5.4.0 | MIT | HTTP router & middleware (RequestID, WrapResponseWriter, Timeout) |
| github.com/jackc/pgx/v5 | 5.11.0 | MIT | Driver PostgreSQL + connection pool (`pgxpool`) |
| github.com/redis/go-redis/v9 | 9.23.0 | BSD-2-Clause | Klien Redis |
| github.com/golang-migrate/migrate/v4 | 4.20.1 | MIT | Migrasi skema (library, driver `pgx/v5`, source `iofs`) |
| golang.org/x/time | 0.16.0 | BSD-3-Clause | Token bucket rate limiter |
| Dependensi tidak langsung (pgpassfile, pgservicefile, puddle, xxhash, x/sys, x/text, x/sync, atomic) | lihat `go.sum` | MIT / BSD | Dipakai oleh pgx, go-redis, migrate |

## Tools pengembangan & CI

| Nama | Versi | Lisensi | Fungsi |
|---|---|---|---|
| gitleaks | 8.30.1 | MIT | Deteksi secret (pre-commit & CI) |
| golangci-lint | 2.14.0 | GPL-3.0 | Linter Go (meta-linter) |
| golang-migrate CLI | 4.20.1 | MIT | Migrasi skema DB (CLI opsional; app memakai library yang sama) |
| sqlc | 1.31.1 | MIT | Generate kode Go type-safe dari query SQL |
| Redocly CLI (`redocly/cli:2.62.1`) | 2.62.1 | MIT | Validasi `api/openapi.yaml` |
| GCC | (paket Ubuntu) | GPL-3.0 | Dibutuhkan race detector (`go test -race`, CGO) |
| GNU Make | 4.3 | GPL-3.0 | Task runner (`Makefile`) |
| ufw | 0.36.2 | GPL-3.0 | Firewall host |
| fail2ban | 1.0.2 | GPL-2.0 | Proteksi brute-force SSH |
| unattended-upgrades | 2.9.1 (paket Ubuntu) | GPL-2.0 | Update keamanan otomatis |
| apt-listchanges | 3.27 | GPL-2.0 | Ringkasan changelog paket saat upgrade |
| ShellCheck | 0.9.0 | GPL-3.0 | Linter skrip shell (`ops/`, `scripts/`) |
| Golang builder image (`golang:1.27.2-bookworm`) | 1.27.2 | BSD-3-Clause | Stage build image backend |
