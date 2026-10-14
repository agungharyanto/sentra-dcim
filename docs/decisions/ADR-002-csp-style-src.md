# ADR-002 — CSP `style-src 'unsafe-inline'` untuk Respons UI

- **Status:** Superseded (2026-10-14, v0.3.3) oleh D-065: nonce per request, `sonner` diganti toast sendiri,
  `'unsafe-inline'` dihapus (ISS-030). Isi di bawah dipertahankan sebagai catatan sejarah.
- **Keputusan terkait:** D-044, ADR-001 (stack frontend), ISS-027 (watch item, kini selesai), ISS-030 (nonce)
- **Pemutus:** pemilik proyek (arahan v0.2.2: buktikan dulu, longgarkan hanya `style-src` bila terblokir)

## Konteks

Sejak v0.2.1 respons UI memakai CSP ketat tanpa `unsafe-inline` sama sekali. v0.2.2 menambahkan komponen
shadcn/ui berbasis Radix dan Toast `sonner`. ISS-027 mencatat risiko: beberapa library menyuntik elemen `<style>`.

## Bukti (Playwright, `test/e2e/design-system.spec.ts`, CSP lama `style-src 'self'`)

Dijalankan pada Caddy terisolasi berisi build v0.2.2 (2026-10-13):

| Pemicu | Sumber | Hasil |
|---|---|---|
| Setiap halaman (Toaster dipasang global) | `sonner` — `__insertCSS()` saat modul dimuat | `style-src-elem` diblokir: CSS toast tidak berlaku |
| Buka Dialog / Sheet / Select / DropdownMenu | `react-remove-scroll` → `react-style-singleton` (kunci scroll body) | `style-src-elem` diblokir; elemen `<style>` kosong (`sha256-47DEQ…`) dan berisi aturan scroll-lock |

Kedua tema gagal dengan 10+ pelanggaran per alur, termasuk smoke test halaman utama (karena Toaster global).
Seluruh pemeriksaan fungsional & axe tetap lulus — yang rusak adalah gaya (toast tanpa tata letak, scroll body tidak
terkunci saat overlay terbuka).

## Keputusan

Respons **UI saja** (snippet `(web_static)` di `deploy/caddy/Caddyfile`) memakai:

```
style-src 'self' 'unsafe-inline'
```

Tidak berubah: `script-src 'self'` (tanpa `unsafe-inline`/`unsafe-eval`), `default-src 'self'`, `object-src 'none'`,
`base-uri 'none'`, `frame-ancestors 'none'`, `font-src 'self'`. Respons API tetap `default-src 'none'`.
`test/api/ui.sh` memastikan `'unsafe-inline'` **hanya** muncul di `style-src`.

## Risiko yang diterima

- Inline style tidak dapat menjalankan kode. Risikonya injeksi CSS (mis. eksfiltrasi via selektor atribut + `url()`),
  yang tetap butuh celah injeksi HTML lebih dulu; React meng-escape output dan `img-src`/`connect-src`/`font-src`
  terbatas `'self'` (+ `data:` gambar), sehingga kanal eksfiltrasi ke domain luar tertutup.
- Risiko dinilai rendah dibanding alternatif: membuang Radix/sonner atau mem-fork library.

## Alternatif & perbaikan berikutnya

1. **Nonce per request (rencana, ISS-030):** Caddy `templates` menyisipkan `{http.request.uuid}` ke `<meta>` di
   `index.html` dan ke header CSP (`style-src 'self' 'nonce-…'`); app memanggil `setNonce()` dari `get-nonce`
   (dipakai `react-style-singleton`). Kendala: `index.html` menjadi dinamis, dan `sonner` belum mendukung nonce
   (perlu opsi upstream atau Toast lain). Dikerjakan bila kedua kendala teratasi.
2. Hash per `<style>`: tidak praktis (isi bervariasi per versi library).
3. Mengganti library: biaya tinggi, manfaat kecil.
