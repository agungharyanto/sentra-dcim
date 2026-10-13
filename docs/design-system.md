# Design System Sentra DCIM

Sumber kebenaran tampilan UI web (`web/`). Berlaku sejak v0.2.2. Showcase interaktif: **`/design-system`**
(halaman internal developer, tidak ada di registry menu, di balik basic_auth; mulai v0.3 dibatasi role admin/dev,
ISS-029).

## 1. Token

Semua warna adalah **token semantik** di `web/src/styles/tokens.css` (CSS variables, nilai `oklch(L C H)` tanpa
alpha). Tailwind memetakannya di `globals.css` (`@theme inline`) menjadi kelas seperti `bg-status-critical-bg`,
`text-menu-available-fg`.

- **Dilarang** memakai warna mentah di komponen (`bg-red-500`, `#ff0000`, `text-[oklch(...)]`). Tambah token dulu.
- `:root` = tema **light**, `.dark` = tema **dark** (default). Token yang tidak didefinisikan ulang di `.dark`
  mewarisi nilai light.
- Kelompok token:

| Kelompok | Token | Pakai untuk |
|---|---|---|
| Permukaan & teks | `background`, `foreground`, `card`, `popover`, `muted`, `muted-foreground`, `accent`, `border`, `input`, `ring` | Latar halaman/kartu, teks, garis, field, indikator fokus |
| Aksi | `primary`, `secondary`, `destructive` (+ `-foreground`) | Tombol & badge umum |
| Link & info | `link`, `info`, `info-bg`, `info-fg` | **Satu-satunya** pemakaian biru |
| Status DCIM | `status-{normal,warning,critical,offline,maintenance}-{solid,fg,bg,border}` | StatusBadge, StatusDot |
| Status menu | `menu-{available,coming-soon,not-connected}-{fg,bg,border}` | MenuStatusBadge |

### Kontras (diuji otomatis)

`web/src/styles/tokens.test.ts` membaca `tokens.css` langsung dan menghitung rasio WCAG di **kedua tema**:

- **Teks ≥ 4.5:1** (WCAG 1.4.3): teks di latar/kartu/popover, `muted-foreground`, `link`, setiap `*-fg` di atas
  `*-bg` badge, teks tombol di atas warnanya.
- **Non-teks ≥ 3:1** (WCAG 1.4.11): `status-*-solid` (StatusDot/ikon) dan `status-*-border`, `menu-*-border`,
  `ring` (indikator fokus), dan `input` (batas field) terhadap `background`, `card`, `popover`.

Mengubah token yang gagal kontras membuat `make web-check` dan CI gagal. Selain itu `test/e2e` menjalankan axe-core
(WCAG 2.1 A/AA) di halaman showcase pada kedua tema dan kedua bahasa: 0 pelanggaran *serious/critical*.

## 2. Status DCIM

| Status | Warna | Ikon lucide | Label ID / EN |
|---|---|---|---|
| `normal` | hijau | `CircleCheck` | Normal / Normal |
| `warning` | amber | `TriangleAlert` | Peringatan / Warning |
| `critical` | merah | `OctagonAlert` | Kritis / Critical |
| `offline` | abu-abu | `CircleOff` | Offline / Offline |
| `maintenance` | ungu | `Wrench` | Pemeliharaan / Maintenance |

Aturan:

1. **Ikon + label, tidak pernah warna saja** (buta warna, layar monokrom, cetak). Pakai `StatusBadge` atau
   `StatusDot`; jangan membuat titik warna sendiri.
2. `StatusDot hideLabel` hanya untuk ruang sempit: label tetap ada untuk pembaca layar (`sr-only`) dan tooltip
   (`title`).
3. **Biru tidak dipakai untuk status** — biru khusus link dan info.
4. Label dari katalog i18n `ds:status.<status>`, tidak pernah ditulis di komponen.

Status menu (registry backend, `GET /api/v1/ui/bootstrap`): `available` (hijau, `CircleCheck`), `coming_soon`
(info/biru, `Clock`), `not_connected` (abu-abu, garis putus-putus, `Unplug`) — komponen `MenuStatusBadge`, label
`shell:menuStatus.<status>`.

## 3. Tema

- Pilihan: **Gelap (default)**, Terang, Ikuti sistem — komponen `ThemeSwitcher`. Disimpan di `localStorage`
  (`sentra.theme`); nilai rusak atau storage yang menolak akses → kembali ke gelap. Preferensi per-user menyusul di
  v0.3.5 (Settings – Regional).
- "Ikuti sistem" mengikuti `prefers-color-scheme` secara langsung (berubah saat OS berganti tema).
- **Kedip gelap→terang saat load** bagi pengguna yang memilih terang: `index.html` statis memakai
  `class="dark"` dan tema yang tersimpan baru diterapkan saat JavaScript berjalan. Ini **diterima** (keputusan
  pemilik, v0.2.2) karena menghindarinya butuh inline script, yang dilarang CSP `script-src 'self'`.

### Gerak

Semua transisi/animasi dimatikan bila pengguna memilih `prefers-reduced-motion: reduce` (WCAG 2.3.3, aturan global
di `globals.css`). Jangan menambah animasi yang wajib dilihat untuk memahami status.

## 4. Tipografi

Inter (teks UI) dan JetBrains Mono (kode, ID perangkat, angka teknis) — variable font **self-hosted**, subset
**latin** saja, dari `@fontsource-variable/*` (`web/src/styles/fonts.css`). Tidak ada CDN; CSP `font-src 'self'`.
Gunakan `font-mono` + `tabular-nums` untuk kolom angka.

## 5. Komponen

| Lapisan | Lokasi | Isi |
|---|---|---|
| Primitif (shadcn/ui, kode di repo) | `web/src/components/ui/` | Button, Card, Badge, Input, Select, Tabs, Table, Tooltip, DropdownMenu, Dialog, Sheet, Skeleton, Toast (`sonner.tsx`) |
| Domain | `web/src/components/domain/` | StatusBadge, StatusDot, MenuStatusBadge, ThemeSwitcher (+ ThemeProvider), LanguageSwitcher, Logo |

### Menambah komponen

1. Ambil kode shadcn/ui (gaya *new-york*, primitif dari paket `radix-ui`) ke `components/ui/<nama>.tsx`; sesuaikan
   agar hanya memakai token semantik.
2. **Tidak ada teks hardcode**: teks terlihat (termasuk `sr-only`, `aria-label`, `title`, `placeholder`, `alt`)
   lewat `t()`. ESLint `i18next/no-literal-string` menolak teks JSX dan atribut-atribut tersebut.
3. Tambah key di **semua** locale (`web/src/i18n/locales/{id,en}/*.json`); `pnpm i18n:check` harus lulus.
4. Tampilkan di `/design-system` dan tambahkan interaksinya ke `test/e2e/design-system.spec.ts` (console bersih,
   tanpa pelanggaran CSP, axe lulus di kedua tema).
5. Catat library baru di `docs/DEPENDENCIES.md`.

### Toast & overlay dan CSP

Dialog, Sheet, Select, DropdownMenu (via `react-remove-scroll`) dan Toast (`sonner`) menyuntik elemen `<style>`
saat runtime. Karena itu respons UI memakai `style-src 'self' 'unsafe-inline'`; `script-src` tetap `'self'`.
Alasan & rencana perbaikan (nonce): [ADR-002](decisions/ADR-002-csp-style-src.md), D-044.

## 6. Logo

Placeholder di **`web/src/assets/brand/logo.svg`** — satu-satunya titik penggantian. File yang sama dipakai komponen
`Logo` (header) dan sebagai favicon (`web/index.html`). Ganti file itu (SVG persegi, latar sendiri) lalu `make up`;
tidak ada referensi lain. Logo resmi Diskominfo Provinsi Bali menunggu dari pemilik.
