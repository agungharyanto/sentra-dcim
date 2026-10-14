---
title: Pengguna & Role
audience: admin
menu: users
order: 40
---

# Pengguna & Role

## Tujuan

Mengelola akun pengguna Sentra DCIM: membuat akun baru, menetapkan **role** (peran), mengubah nama dan e-mail,
menonaktifkan akun yang tidak dipakai lagi, dan mengatur ulang kata sandi pengguna yang lupa. Akun **tidak pernah
dihapus** — akun dinonaktifkan agar jejak audit tetap utuh.

## Siapa yang bisa memakai

| Aksi | Role |
|---|---|
| Melihat daftar pengguna dan daftar role | Admin, Auditor |
| Membuat, mengubah, menetapkan role, menonaktifkan/mengaktifkan, reset kata sandi, reset MFA | Admin |

Pengguna tanpa izin tidak melihat menu ini; bila membuka alamatnya langsung, halaman menampilkan *Akses ditolak*.
Daftar role dan izinnya: [Role & izin](roles.md).

## Langkah-langkah

### Membuat pengguna
1. Buka **Pengguna & Role** → **Tambah pengguna**.
2. Isi **nama pengguna** (3–64 karakter: huruf, angka, `.` `_` `-`; boleh NIP), **nama tampilan**, e-mail
   (opsional), lalu pilih minimal satu role.
3. Pilih **Simpan**. Sistem membuat **kata sandi sementara** (format `xxxx-xxxx-xxxx-xxxx`) dan menampilkannya
   **satu kali saja**. Salin dan serahkan kepada pengguna lewat jalur yang aman (langsung, bukan grup chat).
4. Saat masuk pertama kali pengguna wajib mengganti kata sandi sementara itu.

Nama pengguna tidak bisa diubah setelah akun dibuat (dipakai di log audit).

### Mengubah nama, e-mail, atau role
1. Pada daftar pengguna pilih **Kelola** di baris pengguna.
2. Ubah **Profil** (nama tampilan, e-mail) lalu **Simpan profil**, atau centang/hapus centang role lalu
   **Simpan role**.
3. Perubahan role berlaku **seketika** pada permintaan berikutnya pengguna itu — tanpa perlu masuk ulang.

### Menonaktifkan dan mengaktifkan kembali
- **Nonaktifkan** menutup semua sesi pengguna saat itu juga dan menolak login berikutnya (pesan login tetap umum).
- **Aktifkan** mengizinkan login kembali dengan kata sandi yang sama.

### Mengatur ulang kata sandi
1. Pada halaman pengguna pilih **Reset kata sandi** lalu konfirmasi.
2. Kata sandi sementara baru tampil **satu kali**; semua sesi pengguna ditutup dan kunci akun (lockout) dibuka.
3. Pengguna wajib mengganti kata sandi itu saat masuk.

Untuk akun Anda sendiri gunakan [Ganti kata sandi](../auth/ganti-password.md).

### Mereset verifikasi dua langkah (MFA)
Bila pengguna kehilangan HP dan recovery code-nya: pada halaman pengguna pilih **Reset MFA** lalu konfirmasi. MFA dan
recovery code pengguna dihapus, semua sesinya ditutup, dan ia memasang ulang saat masuk berikutnya
([Verifikasi dua langkah](../auth/mfa.md)). Pastikan dulu permintaan itu benar dari pemilik akun (mis. bertemu
langsung). MFA akun Anda sendiri tidak bisa direset di sini. Kolom **MFA belum aktif** di daftar menandai akun yang
belum memasang MFA.

## Pengaman bawaan

| Aturan | Alasan |
|---|---|
| Anda tidak bisa menonaktifkan akun sendiri, mencabut role Admin sendiri, atau me-reset kata sandi sendiri lewat halaman ini | Mencegah terkunci dari aplikasi |
| Admin aktif terakhir tidak bisa kehilangan role Admin atau dinonaktifkan | Selalu ada minimal satu Admin |
| Kata sandi sementara hanya tampil sekali dan tidak pernah dicatat | Kata sandi tidak boleh tersimpan selain sebagai hash |
| Setiap perubahan tercatat di log audit (siapa, kapan, apa) | Akuntabilitas |

Bila semua Admin kehilangan akses, petugas operasional server dapat membuat Admin darurat lewat perintah
administrasi server (lihat [Akun & masuk](../auth/index.md)).

## Endpoint API terkait

| Metode & path | Izin |
|---|---|
| `GET /api/v1/users` (cari `q`, filter `status`, `role`, halaman `limit`/`offset`) | `users.read` |
| `GET /api/v1/users/{id}` | `users.read` |
| `POST /api/v1/users` | `users.manage` |
| `PATCH /api/v1/users/{id}` | `users.manage` |
| `PUT /api/v1/users/{id}/roles` | `users.manage` |
| `POST /api/v1/users/{id}/disable`, `POST /api/v1/users/{id}/enable` | `users.manage` |
| `POST /api/v1/users/{id}/password-reset` | `users.manage` |
| `POST /api/v1/users/{id}/mfa-reset` | `users.manage` |
| `GET /api/v1/roles` | `roles.read` |

Detail skema: [Referensi API](/help/api/reference).
