---
title: Sesi saya
audience: user
order: 28
---

# Sesi saya

## Tujuan

Melihat semua perangkat yang sedang masuk dengan akun Anda dan mencabut sesi yang tidak Anda kenali atau tidak
dipakai lagi, misalnya browser di komputer bersama.

## Langkah

1. Pilih nama Anda di kanan atas, lalu **Sesi saya**.
2. Tabel menampilkan setiap sesi aktif: **Perangkat** (browser dan sistem operasi), **Alamat IP**, waktu **Masuk**,
   **Aktivitas terakhir**, dan **Berakhir paling lambat**. Sesi yang sedang Anda pakai bertanda **Sesi ini**.
3. Pilih **Cabut** pada sesi lain untuk mengeluarkannya. Perangkat tersebut harus masuk lagi.
4. Untuk mengakhiri sesi ini, pakai **Keluar** di menu nama Anda.

Di ponsel, setiap sesi tampil sebagai kartu dengan isi yang sama.

**Ada sesi yang tidak Anda kenali?** Cabut sesi itu, lalu segera [ganti kata sandi](ganti-password.md). Penggantian
kata sandi juga mengeluarkan semua sesi lain sekaligus.

## Siapa yang bisa memakai

Semua pengguna; hanya sesi **milik sendiri** yang terlihat dan bisa dicabut.

## Endpoint API terkait

| Endpoint | Fungsi |
|---|---|
| `GET /api/v1/auth/sessions` | Daftar sesi aktif milik sendiri |
| `DELETE /api/v1/auth/sessions/{id}` | Cabut salah satu sesi sendiri |
