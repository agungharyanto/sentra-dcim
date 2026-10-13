# Pusat Bantuan — Bahasa Indonesia

Folder ini berisi halaman Pusat Bantuan berbahasa **Indonesia** (bahasa wajib dan acuan). Padanan berbahasa Inggris
ada di [`../en/`](../en/README.md). Mulai v0.2.4 halaman ditampilkan di dalam UI sesuai bahasa aktif.

> Status v0.2.3: halaman pertama `dashboard/index.md`. Halaman lama di `docs/help/<modul>/` dipindah ke sini pada v0.2.4 bersama revisi
> CLAUDE.md aturan 7.

## Konvensi

- Path: `docs/help/id/<modul>/<fitur>.md`, dengan padanan `docs/help/en/<modul>/<fitur>.md` (nama file **sama**).
  Halaman utama modul = `<modul>/index.md` (mis. `dashboard/index.md`, sejak v0.2.3). Struktur final dikunci di v0.2.4.
- `<modul>` dan `<fitur>` memakai huruf kecil + tanda hubung, sama dengan `key` menu di registry backend
  (`GET /api/v1/ui/bootstrap`) bila halaman itu milik sebuah menu, mis. `dashboard/ringkasan.md`.
- Isi mengikuti templat di [`../README.md`](../README.md): tujuan, role yang dibutuhkan, langkah, endpoint API terkait.
- Halaman `id` **wajib** ada sebelum fitur dianggap Done; halaman `en` wajib menyusul dalam sub-versi yang sama.
- Nama tombol/menu ditulis persis seperti label di katalog terjemahan (`web/src/i18n/locales/id/*.json`).
- Tanpa secret, password, IP admin, atau fingerprint key.
