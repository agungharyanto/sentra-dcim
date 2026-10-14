---
title: Kebijakan sesi & kata sandi
audience: user
order: 30
---

# Kebijakan sesi & kata sandi

## Tujuan

Mengatur berapa lama sesi boleh tidak dipakai, berapa lama paling lama satu sesi berlaku, kapan kata sandi harus
diganti, dan berapa kata sandi lama yang tidak boleh dipakai lagi.

## Siapa yang bisa memakai

| Aksi | Role |
|---|---|
| Mengubah kebijakan | Admin (izin `settings.manage`) |

## Isian

| Isian | Rentang | Default | Keterangan |
|---|---|---|---|
| Batas idle (menit) | 5–240 | 30 | Tanpa aktivitas selama ini, pengguna keluar otomatis. Peringatan muncul 2 menit sebelumnya. |
| Batas absolut (jam) | 1–24 | 12 | Paling lama satu sesi berlaku, aktif atau tidak. Tidak boleh lebih pendek dari batas idle. |
| Masa berlaku kata sandi (hari) | 0 atau 30–365 | 90 | 0 = tidak kedaluwarsa. Tidak berlaku untuk akun yang role-nya hanya Viewer. |
| Peringatan sebelum kedaluwarsa (hari) | 1–60 | 14 | Harus lebih pendek dari masa berlaku. |
| Riwayat kata sandi | 1–24 | 5 | Sejumlah kata sandi terakhir (termasuk yang sekarang) tidak boleh dipakai ulang. |

## Langkah-langkah

1. Buka **Pengaturan** → **Sesi & Kata Sandi**.
2. Ubah angka yang diperlukan lalu **Simpan**. **Kembalikan ke default** muncul pada isian yang sudah diubah.
3. Perubahan tercatat di Audit Log (`settings.update`, nilai lama → baru) dan berlaku dalam beberapa detik.

## Cara kerjanya

- **Apa yang dihitung sebagai aktivitas:** klik, ketukan, ketikan, atau gulir di aplikasi, serta tindakan yang
  menyimpan data. Pembaruan otomatis halaman (mis. jam atau data yang dimuat ulang sendiri) **tidak** dihitung,
  sehingga layar yang dibiarkan terbuka tetap keluar otomatis.
- **Peringatan:** 2 menit sebelum keluar otomatis muncul jendela hitung mundur dengan tombol **Tetap masuk**. Bila
  batas absolut yang tercapai, sesi tidak bisa diperpanjang — simpan pekerjaan lalu masuk kembali.
- Batas idle dan absolut baru berlaku untuk sesi yang dibuat setelah perubahan (idle juga pada aktivitas berikutnya).
- **Kata sandi kedaluwarsa:** mulai masa peringatan, banner muncul di bagian atas aplikasi. Setelah lewat, pengguna
  wajib membuat kata sandi baru setelah masuk sebelum bisa memakai aplikasi.
- Akun **monitoring (kiosk)** tidak terkena batas idle dan absolut — lihat [Pengguna & Role](../users/index.md).

## Endpoint API terkait

| Metode & path | Izin |
|---|---|
| `GET` / `PUT /api/v1/settings/session` (`idle_minutes`, `absolute_hours`) | `settings.manage` |
| `GET` / `PUT /api/v1/settings/password` (`expiry_days`, `warn_days`, `history`) | `settings.manage` |
| `POST /api/v1/auth/activity` (sinyal aktivitas dari aplikasi) | semua pengguna masuk |
