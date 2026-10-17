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
| Admin | Pengelola aplikasi | Semua izin: `users.read`, `users.manage`, `roles.read`, `system.version.read`, `ui.design_system`, `audit.read`, `audit.export`, `help.admin`, `settings.manage`, `facility.read`, `facility.manage`, `rack.read`, `rack.manage` |
| NOC | Operator pemantauan 24/7 | `facility.read`, `rack.read` (bertambah saat modul monitoring & alarm hadir) |
| Security | Petugas keamanan & akses fisik | `facility.read`, `rack.read` (bertambah saat modul akses & pengunjung hadir) |
| Teknisi | Teknisi fasilitas & perangkat | `facility.read`, `rack.read` (bertambah saat modul aset & pemeliharaan hadir) |
| Viewer | Pimpinan/pemangku kepentingan, hanya melihat | `facility.read`, `rack.read` |
| Auditor | Pemeriksa internal/eksternal | `users.read`, `roles.read`, `audit.read`, `audit.export`, `facility.read`, `rack.read` |

Daftar terkini selalu tampil di tab **Role** pada menu Pengguna & Role, beserta jumlah pengguna per role.

## Arti izin

| Izin | Mengizinkan |
|---|---|
| `users.read` | Melihat daftar dan detail pengguna |
| `users.manage` | Membuat, mengubah, menetapkan role, menonaktifkan/mengaktifkan, dan reset kata sandi pengguna |
| `roles.read` | Melihat daftar role dan izinnya |
| `system.version.read` | Melihat detail build aplikasi (commit, versi Go, jam server) |
| `ui.design_system` | Membuka halaman showcase komponen tampilan (untuk pengembang) |
| `audit.read` | Melihat audit log, status integritas hash chain, dan menjalankan verifikasi ([Audit Log](../audit/index.md)) |
| `audit.export` | Mengekspor audit log ke CSV (ekspornya sendiri tercatat) |
| `help.admin` | Membaca panduan khusus Admin di Pusat Bantuan, termasuk referensi API |
| `facility.read` | Melihat site, gedung, lantai, ruang, dan pohon [Fasilitas](../facility/index.md) |
| `facility.manage` | Menambah, mengubah, dan mengarsipkan gedung, lantai, ruang (dan site bila global) |
| `rack.read` | Melihat rak |
| `rack.manage` | Menambah, mengubah, mengganti nama, memindah, dan mengarsipkan rak |

## Role per site

Selain role **global** (berlaku di semua site), Admin dapat memberi role **per site** di detail pengguna → **Akses
per site**. Role per site hanya memberi izin data site (`facility.*`, `rack.*`) untuk site itu; izin lain (pengguna,
pengaturan, audit) hanya datang dari role global. Admin tidak bisa diberikan per site.

Contoh: teknisi site Denpasar → tanpa role global, role **Teknisi** per site `DPS1`. Ia hanya melihat data `DPS1`.
Bila ia juga diberi role global Viewer, ia akan melihat **semua** site. Akun harus tetap punya minimal satu role
(global atau per site).

## Endpoint API terkait

`GET /api/v1/roles` (izin `roles.read`) — daftar role, izin per role, dan jumlah pengguna.
`PUT /api/v1/users/{id}/site-roles` (izin `users.manage`) — ganti seluruh role per site seorang pengguna.
