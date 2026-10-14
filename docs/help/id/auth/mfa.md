---
title: Verifikasi dua langkah (MFA)
audience: user
order: 26
---

# Verifikasi dua langkah (MFA)

## Tujuan

Setiap akun Sentra DCIM wajib memakai **verifikasi dua langkah**: selain kata sandi, Anda memasukkan kode 6 digit
dari aplikasi authenticator di HP. Bila kata sandi Anda bocor, orang lain tetap tidak bisa masuk tanpa HP Anda.

Aplikasi yang didukung (gratis): **Google Authenticator**, **Microsoft Authenticator**, atau **Authy**. Semua
memakai standar yang sama (kode berbasis waktu, 6 digit, berganti setiap 30 detik).

## Siapa yang bisa memakai

Semua pengguna, apa pun role-nya. Tanpa MFA, akun tidak bisa memakai menu apa pun.

## Langkah-langkah: memasang pertama kali

Langkah ini muncul otomatis saat Anda masuk pertama kali (atau setelah Admin mereset MFA Anda).

![Halaman Pasang verifikasi dua langkah dengan tiga langkah dan tombol Mulai](/api/v1/help/assets/help-mfa-intro-id.png)

### 1. Pasang aplikasi authenticator di HP

Buka Play Store (Android) atau App Store (iPhone), cari **Google Authenticator** (atau Microsoft Authenticator /
Authy), lalu pasang.

### 2. Tambahkan akun baru di aplikasi

Buka aplikasi, lalu tekan tombol **+** (Tambah akun / Add a code).

![Layar aplikasi authenticator dengan tombol tambah di kanan bawah](/api/v1/help/assets/mfa-phone-add.svg)

### 3. Pindai kode QR dari Sentra DCIM

Di Sentra DCIM pilih **Mulai** sampai kode QR tampil. Di HP pilih **Pindai kode QR** dan arahkan kamera ke kode
QR di layar komputer.

![Halaman Sentra DCIM dengan kode QR, akun, kunci penyiapan, dan kolom kode 6 digit (data contoh)](/api/v1/help/assets/help-mfa-qr-id.png)

![Kamera HP membingkai kode QR untuk dipindai](/api/v1/help/assets/mfa-phone-scan.svg)

Tidak bisa memindai (mis. kamera rusak)? Di aplikasi pilih **Masukkan kunci penyiapan** (*Enter a setup key*),
lalu ketik **Akun** dan **Kunci** yang tampil di bawah kode QR. Jenis kunci: **Berbasis waktu**.

### 4. Ketik kode 6 digit untuk mengaktifkan

Aplikasi menampilkan akun **Sentra DCIM** dengan kode 6 digit yang berganti setiap 30 detik. Ketik kode itu di kolom
**Kode 6 digit**, lalu pilih **Aktifkan**.

![Aplikasi menampilkan kode 6 digit akun Sentra DCIM dengan penghitung waktu](/api/v1/help/assets/mfa-phone-code.svg)

### 5. Simpan 10 recovery code

Setelah aktif, Sentra DCIM menampilkan **10 recovery code** — **hanya sekali**. Simpan di tempat aman (pengelola
kata sandi, atau dicetak/ditulis dan disimpan di lemari terkunci), **jangan** di HP yang sama. Centang *Saya sudah
menyimpan recovery code*, lalu pilih **Lanjutkan**.

![Daftar 10 recovery code dengan peringatan hanya tampil sekali dan kotak centang konfirmasi (kode contoh)](/api/v1/help/assets/help-mfa-recovery-id.png)

## Langkah-langkah: masuk sehari-hari

1. Masuk dengan nama pengguna dan kata sandi seperti biasa.
2. Buka aplikasi authenticator, lihat kode 6 digit akun **Sentra DCIM**.
3. Ketik kode itu lalu pilih **Verifikasi**. Kode yang sudah dipakai tidak bisa dipakai lagi; bila ditolak, tunggu
   kode berikutnya (maksimal 30 detik).

![Halaman Verifikasi dua langkah dengan kolom kode 6 digit dan tautan recovery code](/api/v1/help/assets/help-mfa-verify-id.png)

## HP hilang, rusak, atau ganti HP

- **Masih punya recovery code:** pada halaman verifikasi pilih *HP tidak ada? Pakai recovery code*, ketik salah satu
  kode. Setiap recovery code hanya berlaku sekali. Setelah masuk, buka **Keamanan akun** dan buat ulang recovery code.
- **Tidak punya recovery code:** hubungi Admin. Admin dapat **mereset MFA** akun Anda; saat masuk berikutnya Anda
  memasang ulang dari langkah 1.
- **Ganti HP:** minta Admin mereset MFA, lalu pasang ulang di HP baru.

## Keamanan akun

Menu pengguna (pojok kanan atas) → **Keamanan akun** menampilkan status MFA, sejak kapan aktif, dan sisa recovery
code. Di sana Anda bisa **membuat ulang recovery code** (butuh kode 6 digit saat ini; semua kode lama langsung tidak
berlaku).

## Pengaman

| Aturan | Keterangan |
|---|---|
| Kode salah berkali-kali | Dihitung bersama kata sandi salah: 5 kali → akun terkunci 15 menit dan semua sesi ditutup |
| Kode sekali pakai | Kode yang sudah diterima tidak bisa dipakai lagi |
| Rahasia MFA | Disimpan terenkripsi; tidak ada yang bisa melihatnya lagi setelah dipasang, termasuk Admin |

## Endpoint API terkait

| Metode & path | Keterangan |
|---|---|
| `GET /api/v1/auth/mfa` | Status MFA akun sendiri |
| `POST /api/v1/auth/mfa/enrol` | Mulai pemasangan (kode QR + kunci) |
| `POST /api/v1/auth/mfa/enrol/confirm` | Aktifkan dengan kode pertama; mengembalikan recovery code |
| `POST /api/v1/auth/mfa/verify` | Verifikasi saat masuk (kode atau recovery code) |
| `POST /api/v1/auth/mfa/recovery-codes` | Buat ulang recovery code |
