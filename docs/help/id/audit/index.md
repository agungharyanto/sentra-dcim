---
title: Audit Log
audience: user
menu: audit
order: 45
---

# Audit Log

## Tujuan

Menelusuri **siapa melakukan apa, kapan, dari mana, dan hasilnya** di Sentra DCIM: login dan logout, verifikasi dua
langkah, ganti kata sandi, pengelolaan pengguna dan role, penolakan akses, serta ekspor dan verifikasi audit itu
sendiri. Catatan audit **tidak bisa diubah atau dihapus** dari aplikasi, dan setiap catatan terkunci dalam
**hash chain** sehingga perubahan diam-diam langsung ketahuan (lihat [Verifikasi integritas](verification.md)).

## Siapa yang bisa memakai

| Aksi | Role |
|---|---|
| Melihat audit log, status integritas, menjalankan verifikasi | Admin, Auditor |
| Mengekspor CSV | Admin, Auditor |

Role lain tidak melihat menu ini; bila membuka alamatnya langsung, halaman menampilkan *Akses ditolak*.

## Langkah-langkah

### Mencari event
1. Buka **Audit Log**. Event terbaru tampil paling atas; waktu ditampilkan dalam **WIB**.
2. Saring dengan **Dari** / **Sampai** (waktu WIB), **Pelaku** (sebagian nama pengguna), **Aksi** (mis.
   `auth.login.failure`, atau awalan seperti `auth` untuk semua aksi login), **Hasil** (berhasil, gagal, ditolak),
   dan **Target** (ID objek, mis. ID pengguna). Pilih **Terapkan**; **Reset** mengosongkan filter.
3. Pilih **Muat lebih banyak** di bawah tabel untuk event yang lebih lama.
4. Pilih sebuah baris untuk melihat **detail**: semua kolom, metadata (keterangan tambahan seperti alasan gagal atau
   jumlah percobaan), ID request, dan hash baris itu.

### Mengekspor CSV
1. Atur filter seperti di atas.
2. Pilih **Ekspor CSV**. File berisi maksimal 50.000 event terbaru yang cocok dengan filter, dengan waktu UTC dan WIB.
3. Ekspor itu sendiri tercatat di audit log (aksi `audit.export`, berikut filter dan jumlah barisnya).

Isi sel yang diawali `=`, `+`, `-`, atau `@` diberi awalan tanda kutip `'` agar Excel/LibreOffice menampilkannya
sebagai teks, bukan menjalankannya sebagai rumus (perlindungan *CSV injection*).

### Membaca event yang sering muncul

| Aksi | Arti |
|---|---|
| `auth.login.password_ok` → `auth.login.success` | Kata sandi benar → kode verifikasi dua langkah benar (login lengkap) |
| `auth.login.failure` | Login gagal; metadata `reason` menyebut sebabnya (mis. `invalid_password`, `unknown_user`, `rate_limited`) |
| `auth.lockout` | Akun terkunci sementara setelah terlalu banyak percobaan salah |
| `auth.mfa.enrol_start`, `auth.mfa.enrol`, `auth.mfa.verify` | Mulai memasang, selesai memasang, dan memakai verifikasi dua langkah |
| `user.*` | Pengelolaan pengguna oleh Admin (buat, ubah, role, nonaktifkan, reset) |
| `authz.denied` | Permintaan ditolak karena tidak punya izin |
| `security.csrf_denied` | Permintaan ditolak pemeriksaan CSRF (mis. dikirim dari situs lain) |
| `….repeated` | Ringkasan penolakan berulang — lihat di bawah |
| `audit.export`, `audit.verify`, `audit.prune` | Ekspor CSV, verifikasi hash chain, pemangkasan sesuai masa simpan |

**Penolakan berulang diringkas.** Bila pengguna atau alamat IP yang sama ditolak berkali-kali pada alamat yang sama
dalam 10 menit, hanya penolakan pertama yang dicatat; sisanya dihitung dan dicatat sebagai **satu** event
`authz.denied.repeated` atau `security.csrf_denied.repeated` (metadata `repeated` = jumlah, `first_at`/`last_at`).
Dengan begitu log tidak bisa dibanjiri.

**Tidak pernah dicatat:** kata sandi (termasuk kata sandi sementara), token sesi atau CSRF, kode verifikasi dua
langkah, secret aplikasi authenticator, dan recovery code. Nama pengguna yang salah format pada login gagal juga tidak
disimpan — sering kali itu kata sandi yang terketik di kolom nama pengguna.

## Masa simpan

Event disimpan **365 hari** (dapat diatur administrator server, minimal 90 hari). Event yang lebih tua dipindahkan ke
arsip terkompresi di lokasi backup sebelum dihapus dari database; hash chain tetap bisa diverifikasi karena hash event
terakhir yang dipangkas disimpan sebagai **checkpoint** (event `audit.prune`).

## Endpoint API terkait

| Metode & path | Izin |
|---|---|
| `GET /api/v1/audit/events` (filter `since`, `until`, `actor`, `action`, `result`, `target_type`, `target_id`; halaman `limit` + `before`) | `audit.read` |
| `GET /api/v1/audit/events/export.csv` (filter sama) | `audit.export` |
| `GET /api/v1/audit/actions` | `audit.read` |
| `GET /api/v1/audit/integrity` | `audit.read` |
| `POST /api/v1/audit/integrity/verify` | `audit.read` |

Detail skema: [Referensi API](/help/api/reference) (khusus Admin).
