---
title: Dokumentasi API
audience: admin
order: 90
---

# Dokumentasi API

Sentra DCIM dibangun **API first**: setiap fitur yang tampil di antarmuka tersedia sebagai REST API di bawah
`/api/v1`. Antarmuka web hanyalah salah satu pemakai API tersebut.

[Buka referensi API](/help/api/reference) — daftar endpoint per kelompok, lengkap dengan metode, path, parameter,
skema permintaan/respons, dan kode status. Referensi ini **hanya baca**: tidak ada tombol untuk menjalankan
permintaan dari browser.

## Akses

| Endpoint | Akses saat ini |
|---|---|
| `/api/v1/health`, `/api/v1/version` | Publik, tanpa masuk |
| `/api/v1/auth/login` | Tanpa sesi (halaman Masuk) |
| `/api/v1/auth/*`, `/api/v1/ui/bootstrap`, `/api/v1/help/*` | Pengguna yang sudah masuk (cookie sesi) |
| Endpoint lain | Belum dibuka untuk umum; token untuk integrasi mesin menyusul (v0.6) |

Permintaan yang mengubah data (`POST`, `DELETE`, …) wajib membawa header `X-Sentra-CSRF` berisi nilai cookie
`__Host-sentra_csrf` dan dikirim dari domain aplikasi; bila tidak, respons `403 csrf_failed`. Lihat
[Masuk dan keluar](../auth/login.md).

## Konvensi

- **Waktu** dikirim dalam UTC (RFC 3339). Zona tampilan diatur server (bawaan `Asia/Jakarta`).
- **Error** memakai format tetap `{"error": {"code", "message", "request_id"}}`. Gunakan `code` (mis.
  `not_found`, `rate_limited`) untuk logika; `message` hanya untuk dibaca manusia.
- Setiap respons membawa header `X-Request-Id` — sertakan saat melaporkan masalah.
- **Batas permintaan** per alamat klien; bila terlampaui, respons `429` dengan header `Retry-After`.
- Versi API ada di path (`/api/v1`). Perubahan yang tidak kompatibel dicatat di changelog sebagai *breaking*.
