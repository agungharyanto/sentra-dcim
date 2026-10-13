# Daftar Dependensi

Semua driver, library, framework, dan tools yang dipakai Sentra DCIM. Perbarui tabel ini setiap kali dependensi
ditambah, diubah versinya, atau dihapus.

## Runtime & infrastruktur

| Nama | Versi | Lisensi | Fungsi |
|---|---|---|---|
| Ubuntu Server | 24.04 LTS (aarch64, Oracle Cloud) | Berbagai (GPL dll.) | Sistem operasi server |
| Oracle Cloud Agent (`snap oracle-cloud-agent`) | 1.61.0 (bawaan image) | Oracle (UPL/proprietary) | Agent OCI: monitoring, Run Command (jalur pemulihan darurat) |
| Docker Engine | 29.9.0 | Apache-2.0 | Runtime container |
| Docker Compose | 5.6.0 (plugin) | Apache-2.0 | Orkestrasi container lokal/server |
| TimescaleDB (`timescale/timescaledb:2.30.2-pg16`) | 2.30.2 / PostgreSQL 16 | Timescale License (TSL) + Apache-2.0 (core); PostgreSQL License | Database relasional + time-series |
| Redis (`redis:8.10.2-alpine`) | 8.10.2 | AGPLv3 / RSALv2 / SSPLv1 (tri-license) | Cache, rate-limit store, queue ringan (lihat D-011) |
| Caddy (`caddy:2.11.7-alpine`) | 2.11.7 | Apache-2.0 | Reverse proxy, TLS otomatis, security headers |
| Let's Encrypt (ACME v2, layanan eksternal) | — | ISRG Subscriber Agreement | CA sertifikat TLS gratis untuk `sentra.bajastu.id`, diperpanjang otomatis oleh Caddy |
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
| go.yaml.in/yaml/v3 | 3.0.5 | MIT / Apache-2.0 | Front matter Pusat Bantuan & konversi `api/openapi.yaml` → JSON (v0.2.4). Fork resmi yang dipelihara organisasi YAML (`gopkg.in/yaml.v3` tidak dipelihara lagi) |
| golang.org/x/text (tidak langsung, via pgx) | 0.43.0 | BSD-3-Clause | Dinaikkan dari 0.38.0 karena GO-2026-5970 & GO-2026-6629 (ISS-033) |
| Dependensi tidak langsung (pgpassfile, pgservicefile, puddle, xxhash, x/sys, x/sync 0.24.0, atomic) | lihat `go.sum` | MIT / BSD | Dipakai oleh pgx, go-redis, migrate |

## Frontend (web/, v0.2.1 — D-042, ADR-001)

Versi = hasil resolusi `web/pnpm-lock.yaml`. Build hanya di Docker (`web/Dockerfile`), host tidak butuh Node.
Kebijakan supply chain pnpm `minimumReleaseAge` (rilis < 24 jam ditolak) **aktif tanpa pengecualian** (ISS-031):
pilih versi terbaru yang sudah melewati jendela itu.

| Nama | Versi | Lisensi | Fungsi / alasan |
|---|---|---|---|
| Node.js (`node:22.23.3-alpine`) | 22.23.3 (LTS) | MIT | Runtime build & test UI di Docker (multi-arch, ARM64) |
| pnpm (via corepack, `packageManager`) | 12.11.2 | MIT | Package manager; lockfile ketat, `--frozen-lockfile` |
| react, react-dom | 19.3.0 | MIT | Library UI |
| react-router | 7.18.4 | MIT | Routing SPA (library mode) |
| @tanstack/react-query | 5.104.2 | MIT | Fetch/cache state API (bootstrap, nanti semua modul) |
| i18next | 26.4.4 | MIT | Mesin i18n (katalog, interpolasi, fallback `id`) |
| react-i18next | 17.0.16 | MIT | Binding i18next untuk React (`useTranslation`) |
| tailwindcss, @tailwindcss/vite | 4.3.3 | MIT | Utility CSS + plugin Vite (design system) |
| tw-animate-css | 1.4.0 | MIT | Animasi standar shadcn/ui (Tailwind v4) |
| class-variance-authority | 0.7.1 | Apache-2.0 | Varian komponen (konvensi shadcn/ui) |
| clsx | 2.1.1 | MIT | Gabung className kondisional (`cn()`) |
| tailwind-merge | 3.7.0 | MIT | Resolusi konflik kelas Tailwind (`cn()`) |
| lucide-react | 1.55.0 | ISC | Ikon (nama ikon menu dari registry backend) |
| shadcn/ui | — (kode disalin, `components.json`) | MIT | Pola komponen di atas Radix; komponen + Radix ditambahkan di v0.2.2 |
| radix-ui | 1.8.0 | MIT | Primitif aksesibel (Dialog/Sheet, Select, Tabs, Tooltip, DropdownMenu, Slot) untuk komponen shadcn/ui (v0.2.2) |
| react-markdown | 10.1.0 | MIT | Renderer Markdown Pusat Bantuan (v0.2.4): tanpa raw HTML (tanpa rehype-raw), `urlTransform` hanya https/mailto/route app |
| remark-gfm | 4.0.1 | MIT | GitHub Flavored Markdown (tabel, daftar, strikethrough) untuk halaman bantuan |
| sonner | 2.0.8 | MIT | Toast (pengganti resmi Toast lama shadcn/ui); menyuntik `<style>` → ADR-002 |
| @fontsource-variable/inter | 5.3.0 | OFL-1.1 | Font UI self-hosted (variable, hanya subset latin yang dibundel) |
| @fontsource-variable/jetbrains-mono | 5.3.0 | OFL-1.1 | Font monospace self-hosted (kode, ID perangkat, angka teknis), subset latin |
| vite | 8.3.4 | MIT | Bundler & dev server; output statis ber-hash tanpa inline script |
| @vitejs/plugin-react | 6.1.2 | MIT | Dukungan React/JSX di Vite |
| typescript | 6.0.3 | Apache-2.0 | Type checking strict (7.0 belum didukung typescript-eslint) |
| @types/react, @types/react-dom | 19.3.0 | MIT | Tipe React |
| @types/node | 22.20.5 | MIT | Tipe Node (config & skrip `i18n:check`) |
| eslint, @eslint/js | 10.12.0 / 10.0.1 | MIT | Linter |
| typescript-eslint | 8.71.1 | MIT | Aturan ESLint untuk TypeScript (`strict`) |
| eslint-plugin-react-hooks | 7.1.1 | MIT | Aturan hooks React |
| eslint-plugin-i18next | 6.1.5 | ISC | `no-literal-string` = error: tidak ada teks UI hardcode |
| globals | 17.13.0 | MIT | Definisi global browser/node untuk ESLint |
| vitest | 5.0.4 | MIT | Unit test (i18n, format, render halaman ID/EN) |
| jsdom | 30.1.2 | MIT | DOM untuk Vitest |
| @testing-library/react, user-event, jest-dom | 16.3.3 / 14.6.7 / 7.0.1 | MIT | Render & interaksi komponen di test |
| @playwright/test + image `mcr.microsoft.com/playwright:v1.64.0-noble` | 1.64.0 | Apache-2.0 | Smoke test browser (Chromium) `test/e2e/`, multi-arch |
| @axe-core/playwright | 4.13.0 | MPL-2.0 | Cek aksesibilitas otomatis (WCAG 2.1 A/AA) di `test/e2e` (v0.2.2) |
| actions/upload-artifact | v7 | MIT | CI: unggah screenshot showcase design system (tidak di-commit) |

## Tools pengembangan & CI

| Nama | Versi | Lisensi | Fungsi |
|---|---|---|---|
| gitleaks | 8.30.1 | MIT | Deteksi secret (pre-commit & CI) |
| golangci-lint | 2.14.0 | GPL-3.0 | Linter Go (meta-linter) |
| golang-migrate CLI | 4.20.1 | MIT | Migrasi skema DB (CLI opsional; app memakai library yang sama) |
| sqlc | 1.31.1 | MIT | Generate kode Go type-safe dari query SQL |
| Redocly CLI (`redocly/cli:2.62.1`) | 2.62.1 | MIT | Validasi `api/openapi.yaml` |
| govulncheck (`golang.org/x/vuln`) | 1.9.0 | BSD-3-Clause | Pemindaian kerentanan dependensi Go yang benar-benar terpanggil (CI job Lint, `make vulncheck`, v0.2.4) |
| GCC | (paket Ubuntu) | GPL-3.0 | Dibutuhkan race detector (`go test -race`, CGO) |
| actionlint | 1.7.12 | MIT | Validasi workflow GitHub Actions (lokal, `/usr/local/bin`) |
| actions/checkout | v7 | MIT | CI: checkout repo |
| actions/setup-go | v7 | MIT | CI: instal Go |
| golangci/golangci-lint-action | v9 | MIT | CI: jalankan golangci-lint |
| docker/setup-buildx-action | v4 | Apache-2.0 | CI: Docker Buildx |
| docker/build-push-action | v7 | Apache-2.0 | CI: build image (tanpa push) |
| systemd timer | (Ubuntu 24.04) | LGPL-2.1 | Penjadwalan backup harian |
| GNU Make | 4.3 | GPL-3.0 | Task runner (`Makefile`) |
| ufw | 0.36.2 | GPL-3.0 | Firewall host |
| acl (`setfacl`/`getfacl`) | 2.3.2 | GPL-2.0 / LGPL-2.1 | ACL repo & `.env` untuk user admin (`bootstrap --admin-user`) |
| fail2ban | 1.0.2 | GPL-2.0 | Proteksi brute-force SSH |
| unattended-upgrades | 2.9.1 (paket Ubuntu) | GPL-2.0 | Update keamanan otomatis |
| apt-listchanges | 3.27 | GPL-2.0 | Ringkasan changelog paket saat upgrade |
| ShellCheck | 0.9.0 | GPL-3.0 | Linter skrip shell (`ops/`, `scripts/`) |
| Golang builder image (`golang:1.27.2-bookworm`) | 1.27.2 | BSD-3-Clause | Stage build image backend |
