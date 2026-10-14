---
title: Pengaturan
audience: user
menu: settings
order: 50
---

# Pengaturan

## Tujuan

Mengatur perilaku Sentra DCIM untuk **semua pengguna**: **Regional** (bahasa, zona waktu, format jam & tanggal, v0.3.5)
dan **Allowed IP** (siapa yang boleh membuka panel web, v0.3.6). Kebijakan sesi & kata sandi menyusul sesuai roadmap.

## Siapa yang bisa memakai

| Aksi | Role |
|---|---|
| Melihat dan mengubah pengaturan sistem | Admin (izin `settings.manage`) |
| Mengubah preferensi regional **sendiri** | Semua pengguna — lewat [Preferensi](../auth/preferensi.md), bukan menu ini |

Role lain tidak melihat menu ini; bila membuka alamatnya langsung, halaman menampilkan *Akses ditolak*.

## Prinsip

- Setiap pengaturan punya **nilai bawaan (default)**. Selama belum diubah, sistem memakai default; **Kembalikan ke
  default** menghapus perubahan.
- Nilai divalidasi sebelum disimpan (pilihan yang diizinkan, nama zona waktu yang dikenal). Bila satu isian salah,
  tidak ada yang disimpan.
- Setiap perubahan **tercatat di Audit Log** (aksi `settings.update`): siapa, kapan, nilai lama → nilai baru.

Rincian: [Regional](regional.md), [Allowed IP](allowed-ip.md).

## Endpoint API terkait

| Metode & path | Izin |
|---|---|
| `GET /api/v1/settings/{group}` (mis. `regional`) | `settings.manage` |
| `PUT /api/v1/settings/{group}` (body `{"values": {"nama": nilai atau null}}`) | `settings.manage` |

Detail skema: [Referensi API](/help/api/reference) (khusus Admin).
