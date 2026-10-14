---
title: Pengaturan Regional
audience: user
order: 10
---

# Pengaturan Regional

## Tujuan

Menentukan bahasa, zona waktu, dan format tanggal/jam **bawaan** untuk seluruh pengguna. Data waktu tetap disimpan
dalam UTC; pengaturan ini hanya mengubah **tampilan**.

## Isian

| Isian | Pilihan | Default |
|---|---|---|
| Bahasa sistem | Indonesia, English | Indonesia (bisa berbeda bila diatur administrator server) |
| Zona waktu | Nama zona IANA, mis. `Asia/Jakarta` (WIB), `Asia/Makassar` (WITA), `Asia/Jayapura` (WIT), `UTC` | `Asia/Jakarta` |
| Format jam | 24 jam (`14.30`), 12 jam (`2.30 PM`) | 24 jam |
| Format tanggal | `DD/MM/YYYY` (`14/10/2026`), `YYYY-MM-DD` (`2026-10-14`), `DD MMM YYYY` (`14 Okt 2026`) | `DD/MM/YYYY` |

## Langkah-langkah

1. Buka **Pengaturan** → **Regional**.
2. Ubah isian yang diperlukan. **Contoh tampilan** di bawah formulir langsung menunjukkan hasilnya.
3. Pilih **Simpan**. Perubahan berlaku pada pemuatan halaman berikutnya bagi pengguna yang **tidak** punya preferensi
   sendiri untuk isian tersebut.
4. **Kembalikan ke default** di samping isian mengembalikan nilai bawaan (juga tercatat di audit).

Zona waktu yang tidak dikenal ditolak dengan pesan pada isiannya.

## Hubungan dengan preferensi pengguna

Setiap pengguna bisa memilih nilai sendiri di [Preferensi](../auth/preferensi.md). Urutannya:
**preferensi pengguna** → bila "Ikuti sistem", **pengaturan sistem** di halaman ini → bila belum diatur, **default**.

Yang mengikuti pengaturan efektif: semua tanggal & jam di aplikasi (Audit Log, Sesi saya, Pengguna, jam di header),
pemilih tanggal pada filter, dan kolom waktu lokal di ekspor CSV Audit Log (kolom UTC tetap ada).

## Endpoint API terkait

| Metode & path | Izin |
|---|---|
| `GET /api/v1/settings/regional` | `settings.manage` |
| `PUT /api/v1/settings/regional` | `settings.manage` |
| `GET /api/v1/auth/me` (field `regional` = nilai efektif) | semua pengguna masuk |
