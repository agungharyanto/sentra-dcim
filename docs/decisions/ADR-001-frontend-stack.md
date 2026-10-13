# ADR-001 — Stack Frontend Sentra DCIM

- **Status:** Accepted (2026-10-13, v0.2.1)
- **Keputusan terkait:** D-042 (ADR ini), D-043 (basic_auth sementara), D-035 (i18n wajib sejak v0.2)
- **Pemutus:** pemilik proyek

## Konteks

Sprint v0.2 membangun shell UI dan design system. Prinsip yang mengikat: API first (UI hanya konsumen
`/api/v1/...`), i18n wajib sejak awal (ID default + EN, tanpa teks UI hardcode), security first (CSP ketat, aset
self-hosted), server produksi Oracle A1 (**ARM64**) tanpa Node di host.

## Keputusan

| Area | Pilihan | Catatan |
|---|---|---|
| Framework | **React 19** + **TypeScript 6.0 (strict)** | `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `verbatimModuleSyntax` |
| Build | **Vite 8** | Output statis ber-hash, tanpa inline script (`modulePreload.polyfill=false`, `assetsInlineLimit=0`) |
| i18n | **i18next** + **react-i18next** | Locale `id` (default, katalog acuan) & `en`; namespace `common`, `shell`, `errors` |
| Styling | **Tailwind CSS v4** (`@tailwindcss/vite`) + **shadcn/ui** (Radix) | v0.2.1 hanya dasar: `components.json`, `cn()`, token CSS. Komponen (dan Radix) di v0.2.2 |
| Routing | **React Router 7** (library mode) | Tanpa framework mode/SSR: SPA statis di belakang Caddy |
| Data | **TanStack Query 5** | Cache & status request API |
| Package manager | **pnpm 12** (dipatok via `packageManager`, lewat corepack) | Lockfile `pnpm-lock.yaml`, `--frozen-lockfile` di build |
| Runtime build | **Node 22 LTS** (`node:22.23.3-alpine`) | Hanya di Docker multi-stage (`web/Dockerfile`), multi-arch → ARM64 |
| Lint | ESLint 10 + typescript-eslint + react-hooks + **eslint-plugin-i18next** | `i18next/no-literal-string` = **error** (mode `jsx-only`) untuk `src/**/*.tsx` |
| Test | **Vitest 5** + Testing Library + jsdom; **Playwright 1.64** (e2e, container) | e2e di `test/e2e/`, dipakai ulang di full regression `.9` |
| Serving | **Caddy** menyajikan `/srv/web` (disalin dari image web) | SPA fallback ke `index.html`; CSP tanpa `unsafe-inline` |
| Lokasi | `web/` di root repo | `src/{app,components,features,i18n,lib,styles}`, alias `@/` |

### Konvensi i18n

- Key: `namespace:area.elemen`, contoh `shell:sidebar.dashboard`. Key bertipe (TypeScript `CustomTypeOptions`):
  key yang tidak ada di katalog `id` gagal di `pnpm typecheck`.
- `pnpm i18n:check`: semua locale wajib punya namespace, key, dan variabel interpolasi yang **identik** dengan `id`;
  gagal bila ada yang hilang/berlebih/kosong.
- Locale & timezone default dari server (`GET /api/v1/ui/bootstrap` ← `DEFAULT_LOCALE`, `DISPLAY_TIMEZONE`).
  Pilihan bahasa pengguna disimpan di browser (`localStorage`) dan mengalahkan default server. Mulai v0.3.5
  sumbernya Settings – Regional.
- Tanggal/angka lewat `Intl` (`src/lib/format.ts`) dengan locale aktif + timezone server, jam 24 jam. Data tetap UTC.
- Error API diterjemahkan dari `error.code` (`errors:code.<code>`), kode tak dikenal → `errors:code.unknown`.
- Label menu tidak dikirim backend; UI menerjemahkan `menus[].key`.

## Alternatif yang ditolak

- **TypeScript 7.0** (port native): `typescript-eslint` 8.71 baru mendukung `<6.1`. Naik versi ketika didukung.
- **React Router framework mode / Next.js**: butuh server Node di runtime; tidak perlu untuk SPA internal.
- **npm/yarn**: pnpm lebih ketat (tanpa phantom dependency) dan cepat; konsisten dengan `--frozen-lockfile`.
- **Build di host**: menambah Node di server produksi; ditolak (host hanya Docker).
- **Container web terpisah (nginx)**: menambah satu container & hop; Caddy sudah jadi edge dan bisa menyajikan statis.

## Konsekuensi

- `make up` membangun image `sentra-web` (target `dist`, hanya artefak) lalu menyalinnya ke image Caddy
  (`additional_contexts: web: service:web`). Image `dist` dibangun **dari** stage `check`: tidak ada yang ter-deploy
  bila lint/typecheck/i18n/test gagal.
- `make web-check` dan job CI `web` menjalankan pemeriksaan yang sama tanpa Node di host/runner.
- CSP `style-src 'self'` tanpa `unsafe-inline`: perlu diverifikasi terhadap komponen Radix di v0.2.2 (ISS-027).
- Font wajib self-hosted (saat ini system font); tidak boleh memuat dari CDN.
