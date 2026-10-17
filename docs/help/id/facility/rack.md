---
title: Rak dan penamaannya
audience: user
order: 36
---

# Rak dan penamaannya

## Tujuan

Menjelaskan aturan nama rak, data yang dicatat untuk setiap rak, arti status, dan cara memindah rak.

## Aturan nama rak

Nama rak **bebas** mengikuti kebiasaan site Anda (mis. `R-A01`, `MMR-01`, `Baris A/03`):

- 1–64 karakter: huruf A–Z/a–z, angka, spasi, dan `.` `_` `-` `/`.
- Diawali huruf atau angka. Spasi di awal/akhir dibuang otomatis.
- **Unik per site** tanpa membedakan huruf besar/kecil (`r-a01` = `R-A01`) di antara rak yang tidak diarsipkan.
- Rak dihubungkan ke data lain lewat ID internal, jadi **mengganti nama tidak memutus apa pun**. Setiap
  penggantian nama tercatat di Audit Log sebagai `rack.rename` dengan nama lama dan baru.

## Data rak

| Data | Keterangan |
|---|---|
| Asset tag | Opsional; unik selamanya (tetap tercatat walau rak diarsipkan) |
| Baris, posisi di baris | Opsional, untuk denah (mis. baris `A`, posisi `3`) |
| Tinggi (U) | 1–60, bawaan 42U |
| Nomor U awal, nomor U menurun | Penomoran unit: bawaan mulai U1 dari bawah; centang "menurun" bila U1 di atas |
| Lebar | 19 atau 23 inci |
| Kedalaman, beban maks., daya maks. | Opsional (mm, kg, kW) |

## Status rak

| Status | Arti |
|---|---|
| Direncanakan | Belum terpasang (bawaan rak baru) |
| Aktif | Terpasang dan dipakai |
| Dipesan | Disiapkan/dipesan untuk penyewa atau proyek |
| Dekomisioning | Sedang dikosongkan untuk dibongkar |
| Pensiun | Tidak dipakai lagi (biasanya lalu diarsipkan) |

Status selalu tampil sebagai warna **dan** ikon **dan** teks.

## Memindah rak

**Ubah** → pilih **Ruang** lain. Rak hanya bisa dipindah ke ruang di **site yang sama**; perpindahan antar-site
ditolak.

## Siapa yang bisa memakai

Melihat: semua role (`rack.read`). Menambah, mengubah, mengganti nama, memindah, dan mengarsipkan: Admin
(`rack.manage`). Lihat [Fasilitas](index.md).

## Endpoint API terkait

`GET /api/v1/racks`, `GET /api/v1/racks/{id}` (`rack.read`); `POST /api/v1/racks`, `PATCH /api/v1/racks/{id}`,
`POST /api/v1/racks/{id}/archive` (`rack.manage`, PATCH/arsip wajib `If-Match`).
