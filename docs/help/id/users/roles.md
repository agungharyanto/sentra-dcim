---
title: Role & izin
audience: admin
order: 41
---

# Role & izin

## Tujuan

Menjelaskan role (peran) bawaan Sentra DCIM dan izin yang diberikannya, agar Admin dapat memilih role yang paling
sempit untuk setiap pengguna (*least privilege*).

## Cara kerja

- **Izin** (mis. `users.read`) didefinisikan di aplikasi dan diperiksa pada setiap permintaan API.
- **Role** adalah kumpulan izin. Seorang pengguna boleh memiliki lebih dari satu role; izinnya adalah gabungan
  semua role tersebut.
- Semua pengguna yang masuk — apa pun role-nya — dapat memakai Dasbor, Pusat Bantuan, dan pengaturan akun
  sendiri (ganti kata sandi, Sesi saya).

## Role bawaan

| Role | Untuk siapa | Izin saat ini |
|---|---|---|
| Admin | Pengelola aplikasi | Semua izin: `users.read`, `users.manage`, `roles.read`, `system.version.read` |
| NOC | Operator pemantauan 24/7 | — (bertambah saat modul monitoring & alarm hadir) |
| Security | Petugas keamanan & akses fisik | — (bertambah saat modul akses & pengunjung hadir) |
| Teknisi | Teknisi fasilitas & perangkat | — (bertambah saat modul aset & pemeliharaan hadir) |
| Viewer | Pimpinan/pemangku kepentingan, hanya melihat | — (bertambah saat modul baca hadir) |
| Auditor | Pemeriksa internal/eksternal | `users.read`, `roles.read` (log audit menyusul) |

Daftar terkini selalu tampil di tab **Role** pada menu Pengguna & Role, beserta jumlah pengguna per role.

## Arti izin

| Izin | Mengizinkan |
|---|---|
| `users.read` | Melihat daftar dan detail pengguna |
| `users.manage` | Membuat, mengubah, menetapkan role, menonaktifkan/mengaktifkan, dan reset kata sandi pengguna |
| `roles.read` | Melihat daftar role dan izinnya |
| `system.version.read` | Melihat detail build aplikasi (commit, versi Go, jam server) |

## Endpoint API terkait

`GET /api/v1/roles` (izin `roles.read`) — daftar role, izin per role, dan jumlah pengguna.
