---
title: Masuk dan keluar
audience: user
order: 26
---

# Masuk dan keluar

## Tujuan

Masuk ke Sentra DCIM dengan akun pribadi agar setiap tindakan tercatat atas nama Anda, lalu keluar saat selesai.

## Langkah masuk

1. Buka alamat Sentra DCIM. Selama masa transisi (sampai v0.3.4) browser lebih dulu meminta **kredensial akses
   situs** yang terpisah dari akun Anda; mintalah kepada administrator.
2. Di halaman **Masuk**, isi **Nama pengguna** (atau NIP) dan **Kata sandi**, lalu pilih **Masuk**.
3. Bila akun baru dibuat atau kata sandinya diatur ulang, Anda langsung diminta
   [mengganti kata sandi](ganti-password.md) sebelum memakai aplikasi.
4. Setelah berhasil, Anda dibawa ke halaman yang tadi ingin dibuka (atau [Dasbor](../dashboard/index.md)). Nama
   Anda tampil di kanan atas.

## Pesan yang mungkin muncul

| Pesan | Arti dan tindakan |
|---|---|
| **Nama pengguna atau kata sandi salah.** | Periksa ejaan dan tombol Caps Lock. Pesan ini sengaja sama untuk semua penyebab, termasuk akun terkunci atau nonaktif. |
| **Terlalu banyak percobaan masuk. Coba lagi dalam … menit.** | Tunggu sesuai waktu yang disebut. |
| **Sesi Anda telah berakhir.** | Sesi berakhir karena lama tidak aktif, batas waktu sejak masuk tercapai, atau dicabut. Masuk kembali. |

**Akun terkunci:** setelah 5 kali kata sandi salah, akun dikunci 15 menit lalu terbuka sendiri. Bila Anda lupa
kata sandi, hubungi administrator.

## Keluar

Pilih nama Anda di kanan atas, lalu **Keluar**. Sesi di server langsung dicabut. Menutup browser juga menghapus
cookie sesi, tetapi **Keluar** lebih aman, terutama di komputer bersama.

## Batas waktu sesi

- Sesi berakhir setelah **30 menit tanpa aktivitas**.
- Sesi juga berakhir paling lambat **12 jam setelah masuk**, walau Anda terus aktif.

## Siapa yang bisa memakai

Semua pengguna yang memiliki akun.

## Endpoint API terkait

| Endpoint | Fungsi |
|---|---|
| `POST /api/v1/auth/login` | Masuk; membuat sesi baru |
| `POST /api/v1/auth/logout` | Keluar; mencabut sesi ini |
| `GET /api/v1/auth/me` | Pengguna yang sedang masuk |

Detail lengkap ada di [referensi API](/help/api/reference).
