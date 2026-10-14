---
title: Verifikasi integritas audit log
audience: user
order: 10
---

# Verifikasi integritas audit log

## Tujuan

Memastikan tidak ada catatan audit yang diubah, dihapus, atau disisipkan diam-diam — bahkan oleh pihak yang punya akses
langsung ke database.

## Cara kerjanya

- Setiap event diberi nomor urut (**seq**) dan **hash** SHA-256 yang dihitung dari isi event **ditambah hash event
  sebelumnya**. Event-event itu membentuk rantai (*hash chain*): mengubah satu event membuat hash-nya tidak cocok, dan
  menghapus satu event memutus urutan nomornya.
- Hash dihitung oleh database saat event ditulis, bergiliran satu per satu, sehingga rantai tidak bisa bercabang
  walaupun banyak event masuk bersamaan. Aplikasi tidak bisa mengubah atau menghapus event (hak akses database-nya
  hanya menambah dan membaca).
- **Verifikasi** menghitung ulang seluruh rantai secara mandiri dan membandingkannya. Aplikasi menjalankannya
  **otomatis setiap hari**; kegagalan dicatat sebagai kesalahan di log sistem (siap diteruskan ke Alarm Engine).
- **Anchor di backup:** setiap backup harian menyimpan hash event terakhir saat itu. Kalau seseorang menulis ulang
  *seluruh* rantai (sehingga rantai tampak konsisten lagi), hash yang tersimpan di backup tidak lagi cocok.
  Pemeriksaan terhadap anchor dijalankan petugas operasional server.

## Langkah-langkah

1. Buka **Audit Log**. Kartu **Integritas hash chain** menampilkan hasil verifikasi terakhir: **Utuh**, **Rusak**,
   atau **Belum diverifikasi** (sejak aplikasi terakhir dijalankan ulang), beserta waktu dan jumlah event yang
   diperiksa.
2. Pilih **Verifikasi sekarang** untuk menghitung ulang saat ini. Hanya satu verifikasi berjalan pada satu waktu.
   Verifikasi manual tercatat sebagai event `audit.verify`.
3. Bila hasilnya **Rusak**, kartu menyebut nomor event (**seq**) pertama yang bermasalah dan jenisnya: isi berubah,
   event hilang, atau rantai ditulis ulang.

## Bila verifikasi gagal

1. **Jangan** mengubah apa pun di server; catat waktu dan nomor event yang dilaporkan.
2. Laporkan ke Admin dan petugas operasional server. Mereka membandingkan rantai dengan anchor di backup dan dengan
   arsip, lalu — bila perlu — memulihkan database dari backup terakhir yang masih utuh.
3. Ekspor CSV di sekitar nomor event tersebut sebagai bahan investigasi.

Setelah database dipulihkan dari backup, verifikasi dijalankan lagi untuk memastikan rantai hasil pemulihan utuh.

## Endpoint API terkait

| Metode & path | Izin |
|---|---|
| `GET /api/v1/audit/integrity` | `audit.read` |
| `POST /api/v1/audit/integrity/verify` | `audit.read` |
