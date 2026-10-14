---
title: Preferensi
audience: user
order: 29
---

# Preferensi

## Tujuan

Memilih bahasa, zona waktu, format jam, dan format tanggal **untuk akun Anda sendiri**, berbeda dari pengaturan
sistem — misalnya petugas di WITA yang ingin melihat jam `Asia/Makassar`.

## Siapa yang bisa memakai

Semua pengguna yang sudah masuk (tanpa izin khusus).

## Langkah-langkah

1. Buka menu akun (nama Anda di kanan atas) → **Preferensi**.
2. Untuk setiap isian pilih nilai sendiri, atau **Ikuti sistem** (nilai sistem saat ini ditampilkan di sampingnya).
3. Pilih **Simpan**. Tampilan langsung berganti; **Contoh tampilan** menunjukkan hasilnya.

Tombol bahasa **ID/EN** di header juga menyimpan pilihan bahasa sebagai preferensi akun Anda.

Selama sebuah isian "Ikuti sistem", perubahan oleh Admin di [Pengaturan Regional](../settings/regional.md) ikut
berlaku untuk Anda. Setiap perubahan preferensi tercatat di Audit Log (`user.preferences.update`).

## Endpoint API terkait

| Metode & path | Izin |
|---|---|
| `GET /api/v1/me/preferences` | semua pengguna masuk |
| `PUT /api/v1/me/preferences` (body `{"values": {"regional.timezone": "Asia/Makassar"}}`, `null` = ikuti sistem) | semua pengguna masuk |
