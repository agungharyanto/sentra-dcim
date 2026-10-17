---
title: Fasilitas
audience: user
menu: facility
order: 35
---

# Fasilitas

## Tujuan

Mencatat **di mana semuanya berada**: hierarki **Site › Gedung › Lantai › Ruang › Rak**. Data ini menjadi dasar
modul berikutnya (tenant, aset/perangkat, kabel, monitoring). Menu **Fasilitas** punya dua tab:

- **Hierarki** — pohon di kiri, detail yang dipilih di kanan (di layar kecil bertumpuk).
- **Daftar rak** — semua rak dengan pencarian, filter site/status, urutan, dan arsip.

## Hierarki

| Tingkat | Isi | Catatan |
|---|---|---|
| **Site** | Lokasi data center | Kode site (mis. `DPS1`) unik dan tidak bisa diubah; status Direncanakan / Aktif / Tidak aktif |
| **Gedung** | Bangunan di site | Kode gedung opsional |
| **Lantai** | Lantai di gedung | Level: 0 = lantai dasar, negatif = basement |
| **Ruang** | Ruang di lantai | Jenis ruang: Ruang server, **Meet-me room (MMR)**, Ruang daya, Ruang pendingin, NOC, Gudang, Staging, Lainnya |
| **Rak** | Rak di ruang | Nama bebas, lihat [Rak dan penamaannya](rack.md) |

Nama unik di dalam induknya (gedung per site, lantai per gedung, ruang per lantai) tanpa membedakan huruf
besar/kecil. Nama rak unik **per site**.

### Meet-me room (MMR)

Ruang berjenis **MMR** adalah titik pertemuan kabel antar-penyewa dan operator (cross-connect). Di pohon dan daftar
rak ruang MMR ditandai ikon kabel. Penelusuran kabel rak ↔ rak dan rak ↔ MMR hadir di versi berikutnya (v0.4).

## Langkah-langkah

1. Buka **Fasilitas** → **Hierarki**. Pilih node di pohon (klik, atau panah atas/bawah lalu Enter; panah
   kanan/kiri membuka/menutup cabang).
2. **Tambah site** (tombol di atas pohon) lalu, pada detail tiap node, **Tambah gedung / lantai / ruang / rak**.
3. **Ubah** mengubah data node; untuk rak tersedia juga **Ubah nama**.
4. **Arsipkan** menyembunyikan node tanpa menghapusnya. Node yang masih punya isi aktif tidak bisa diarsipkan —
   arsipkan isinya dulu (dari bawah: rak → ruang → lantai → gedung → site).
5. Di **Daftar rak**, centang **Tampilkan arsip** untuk melihat rak yang diarsipkan. Klik nama rak untuk membukanya
   di hierarki.

### Arsip, bukan hapus

Data fasilitas tidak pernah dihapus: arsip menyimpan riwayat dan semua perubahan tercatat di
[Audit Log](../audit/index.md) (nilai sebelum dan sesudah). Nama yang diarsipkan boleh dipakai lagi; kode site dan
asset tag rak tetap tercatat dan tidak bisa dipakai ulang.

### "Data sudah diubah pengguna lain"

Setiap data punya versi. Bila orang lain menyimpan perubahan setelah Anda membuka form, simpanan Anda ditolak
dengan pesan **Data sudah diubah pengguna lain** — pilih **Muat ulang**, periksa versi terbaru, lalu ulangi
perubahan Anda. Ini mencegah perubahan saling menimpa.

## Siapa yang bisa memakai

| Aksi | Role |
|---|---|
| Melihat hierarki dan rak | Semua role (Admin, NOC, Security, Teknisi, Viewer, Auditor) — izin `facility.read`, `rack.read` |
| Menambah, mengubah, mengarsipkan gedung/lantai/ruang | Admin — izin `facility.manage` |
| Menambah, mengubah, mengganti nama, memindah, mengarsipkan rak | Admin — izin `rack.manage` |
| Menambah dan mengarsipkan **site** | Admin dengan akses **global** |

### Akses per site

Role bisa diberikan **global** (berlaku di semua site) atau **per site** (Pengguna & Role → detail pengguna →
**Akses per site**). Pengguna dengan role per site hanya melihat data site tersebut: site lain tidak muncul di
pohon maupun daftar, dan alamat datanya dijawab "tidak ditemukan". Agar pengguna benar-benar terbatas pada site
tertentu, **jangan** beri role global. Admin selalu global. Lihat [Role & izin](../users/roles.md).

## Endpoint API terkait

| Endpoint | Izin |
|---|---|
| `GET /api/v1/facility/tree` | `facility.read` |
| `GET /api/v1/sites`, `GET /api/v1/sites/{id}` (juga `buildings`, `floors`, `rooms`) | `facility.read` |
| `POST /api/v1/sites`, `PATCH /api/v1/sites/{id}`, `POST /api/v1/sites/{id}/archive` (juga `buildings`, `floors`, `rooms`) | `facility.manage` |
| `GET /api/v1/racks`, `GET /api/v1/racks/{id}` | `rack.read` |
| `POST /api/v1/racks`, `PATCH /api/v1/racks/{id}`, `POST /api/v1/racks/{id}/archive` | `rack.manage` |

Daftar mendukung `q`, filter induk (`site_id`, `building_id`, `floor_id`, `room_id`), `status`, `room_type`,
`include_archived`, `sort`, `limit`/`offset`. Perubahan wajib header `If-Match` berisi `ETag` terakhir
(`428` bila tidak ada, `412` bila data sudah diubah orang lain). Rincian di [Dokumentasi API](../api/index.md) dan
[referensi API](/help/api/reference).
