---
title: Allowed IP
audience: user
order: 20
---

# Allowed IP

## Tujuan

Membatasi **siapa yang boleh membuka panel web** Sentra DCIM berdasarkan alamat IP atau jaringan (mis. hanya dari
jaringan kantor dan ruang NOC). Berlaku untuk tampilan web dan API. Pembatasan akses administrasi
server menyusul di v0.12 bersama VPN WireGuard.

## Siapa yang bisa memakai

| Aksi | Role |
|---|---|
| Melihat dan mengubah daftar serta mode | Admin (izin `settings.manage`) |

## Mode

| Mode | Perilaku |
|---|---|
| **Nonaktif** (default) | Semua IP boleh. Tidak ada yang dicatat. |
| **Pantau** | Semua IP boleh, tetapi akses dari IP di luar daftar dicatat di Audit Log (`security.allowlist_outside`). Pakai mode ini beberapa hari untuk memastikan daftar sudah lengkap. |
| **Tegakkan** | Hanya IP dalam daftar yang boleh; yang lain mendapat pesan "Akses dari alamat IP ini tidak diizinkan" (`security.allowlist_blocked` di Audit Log). |

Di semua mode, **pemeriksaan kesehatan** layanan dan **perpanjangan sertifikat HTTPS** tetap terbuka (ditampilkan di
bagian "Selalu terbuka").

## Langkah-langkah

1. Buka **Pengaturan** → **Allowed IP**. Kartu **IP Anda saat ini** menunjukkan alamat yang dilihat server dan
   apakah sudah tercakup daftar.
2. Tambahkan jaringan: isi **IP atau jaringan (CIDR)** — satu alamat (`A.B.C.D`) atau jaringan
   (`A.B.C.0/24`, IPv6 juga bisa) — dan **Keterangan**, lalu **Tambah**. **Tambahkan IP saya** menambahkan alamat
   Anda sendiri.
3. Pilih mode **Pantau**, simpan, lalu periksa Audit Log (filter aksi `security`) selama beberapa hari: setiap IP sah
   yang muncul di sana perlu ditambahkan.
4. Pilih **Tegakkan** dan simpan. Konfirmasi dialognya. Dalam **2 menit** tekan **Pertahankan** pada banner yang
   muncul. Bila tidak ditekan (mis. Anda ternyata terkunci), mode **otomatis kembali ke Pantau**.

### Pengaman agar Anda tidak terkunci

- **Tegakkan** tidak bisa disimpan bila IP Anda saat ini tidak tercakup daftar.
- Saat Tegakkan aktif, menghapus jaringan yang mencakup IP Anda sendiri ditolak.
- Konfirmasi 2 menit + kembali otomatis ke Pantau (lihat langkah 4).
- Bila semua cara gagal, petugas operasional server dapat menonaktifkan Allowed IP dari server (prosedur darurat
  tercatat di dokumen pemulihan bencana).

### Catatan

- IP diambil dari koneksi langsung ke server. Header yang dikirim peramban (mis. `X-Forwarded-For`) tidak dipercaya,
  sehingga tidak bisa dipalsukan.
- Pengguna di balik NAT kantor tampil dengan IP publik kantor. IP rumah/seluler biasanya berubah-ubah; gunakan
  jaringan kantor atau VPN.
- Jaringan paling luas yang diterima: /8 (IPv4) atau /16 (IPv6). Bit host harus nol: tulis `A.B.C.0/24`, bukan
  `A.B.C.5/24`.
- Setiap perubahan daftar dan mode tercatat di Audit Log (`allowlist.add`, `allowlist.update`, `allowlist.remove`,
  `allowlist.mode`, `allowlist.confirm`, `allowlist.rollback`).

## Endpoint API terkait

| Metode & path | Izin |
|---|---|
| `GET /api/v1/settings/allowed-ips` | `settings.manage` |
| `POST /api/v1/settings/allowed-ips` (`{"cidr", "label"}`) | `settings.manage` |
| `PATCH /api/v1/settings/allowed-ips/{id}` (`{"label"}`) | `settings.manage` |
| `DELETE /api/v1/settings/allowed-ips/{id}` | `settings.manage` |
| `PUT /api/v1/settings/allowed-ips/mode` (`{"mode": "off" \| "monitor" \| "enforce"}`) | `settings.manage` |
| `POST /api/v1/settings/allowed-ips/confirm` | `settings.manage` |
