---
title: Ganti kata sandi
audience: user
order: 27
---

# Ganti kata sandi

## Tujuan

Mengganti kata sandi akun Anda: **wajib** pada saat pertama masuk (kata sandi sementara dari administrator), dan
kapan saja secara sukarela, misalnya bila Anda curiga kata sandi diketahui orang lain.

## Ketentuan kata sandi

- Panjang **12–128 karakter**. Tidak ada aturan wajib huruf besar, angka, atau simbol.
- Frasa panjang yang mudah Anda ingat (mis. beberapa kata acak) lebih aman daripada kata pendek yang rumit.
- Harus berbeda dari kata sandi saat ini **dan dari beberapa kata sandi terakhir** (default 5, diatur Admin); bila
  tidak, muncul pesan "Kata sandi ini sudah pernah dipakai baru-baru ini". Jangan memakai kata sandi yang sama dengan
  layanan lain.
- Kata sandi **berlaku 90 hari** (diatur Admin; akun yang role-nya hanya Viewer tidak kedaluwarsa). Mulai 14 hari
  sebelum habis, banner di bagian atas aplikasi mengingatkan Anda. Setelah lewat, Anda wajib membuat kata sandi baru
  setelah masuk. Lihat [Kebijakan sesi & kata sandi](../settings/session-password.md).

## Langkah — kata sandi pertama

1. Masuk dengan kata sandi sementara. Halaman **Ganti kata sandi** muncul otomatis; menu lain belum bisa dibuka.
2. Isi **Kata sandi saat ini** (kata sandi sementara), **Kata sandi baru**, dan **Ulangi kata sandi baru**.
3. Pilih **Simpan kata sandi**. Anda langsung diteruskan ke aplikasi.

## Langkah — penggantian sukarela

1. Pilih nama Anda di kanan atas, lalu **Ganti kata sandi**.
2. Isi ketiga kolom, lalu pilih **Simpan kata sandi**.

Setelah kata sandi diganti, **semua sesi lain** akun Anda (perangkat lain) otomatis keluar. Sesi yang sedang
dipakai tetap aktif. Lihat [Sesi saya](sesi.md).

## Pesan yang mungkin muncul

| Pesan | Tindakan |
|---|---|
| **Kata sandi saat ini salah.** | Periksa kata sandi lama Anda. |
| **Kata sandi baru harus 12–128 karakter.** | Pakai kata sandi yang lebih panjang. |
| **Ulangan kata sandi baru tidak sama.** | Ketik ulang dengan teliti. |

## Siapa yang bisa memakai

Semua pengguna, untuk akun sendiri. Lupa kata sandi? Hubungi administrator.

## Endpoint API terkait

| Endpoint | Fungsi |
|---|---|
| `POST /api/v1/auth/password` | Ganti kata sandi sendiri; mencabut sesi lain |
