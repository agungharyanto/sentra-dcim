# Akses UI Web Sementara (basic_auth)

**Modul:** ops · **Tersedia sejak:** v0.2.1 · **Diperbarui:** v0.2.5 · **Status:** Available (sementara, dicabut di v0.3)

## Tujuan
Sampai fitur login (Auth, v0.3) tersedia, UI web di `https://sentra.bajastu.id/` dan `GET /api/v1/ui/bootstrap`
dilindungi **basic_auth** di Caddy (D-043, ISS-026). `/api/v1/health` dan `/api/v1/version` tetap publik; API lain
tetap ditolak `403`. Login salah berulang di-ban oleh fail2ban — lihat [fail2ban.md](fail2ban.md) (v0.2.5).

## Role yang dibutuhkan
| Aksi | Role |
|---|---|
| Membuka UI web | Siapa pun yang diberi user + password basic_auth oleh pemilik |
| Mengatur/mengganti password | Admin OS server (`sudo -u sentra`) |

## Langkah-langkah
1. Buat hash password **di server** dengan **bcrypt cost 10** (password diketik dua kali di prompt, tidak tampil,
   tidak masuk command line/history):
   ```
   docker run --rm -it caddy:2.11.7-alpine caddy hash-password --bcrypt-cost 10
   ```
   Gunakan password acak panjang (mis. 24 byte: `openssl rand -base64 24`, simpan di password manager).
   **Kenapa cost 10, bukan default 14:** setiap login dengan password salah memaksa Caddy menghitung bcrypt. Diukur
   di server ini (ARM, 4 vCPU, v0.2.5): cost 14 ≈ **1 030 ms CPU** per percobaan (≈ 4 percobaan/detik sudah
   menghabiskan semua core), cost 10 ≈ **65 ms** (≈ 16× lebih ringan). Dengan password acak 24 byte, cost 10 tetap
   jauh di luar jangkauan brute force; yang dimitigasi adalah beban CPU (DoS). `make preflight` memperingatkan bila
   cost > 12.
2. Edit `.env` sebagai pemiliknya: `sudo -u sentra -H nano /opt/sentra-dcim/.env`, isi:
   ```
   WEB_BASIC_AUTH_USER=<nama-user>
   WEB_BASIC_AUTH_HASH='<hash dari langkah 1>'
   ```
   Hash bcrypt mengandung `$`: **tanda kutip tunggal wajib**. Jangan memakai `sed -i` (ISS-024).
3. `make up`. `make preflight` menolak bila hash kosong/tidak valid pada domain publik.
4. Buka `https://sentra.bajastu.id/`, masukkan user + password. Browser mengingatnya sampai ditutup.
5. Mengganti password: ulangi langkah 1–3. Semua sesi lama otomatis harus login ulang.

### Menjalankan test UI dengan kredensial
`make test-ui` dan `make test-e2e` membaca kredensial dari `SENTRA_WEB_USER`/`SENTRA_WEB_PASS`, file
`~/.config/sentra/web-auth` (mode 600, `user:password`), atau prompt interaktif. Jangan menaruh kredensial di repo.

## Endpoint API terkait
| Metode | Path | Keterangan |
|---|---|---|
| GET | `/` (dan semua path non-API) | UI web (SPA), basic_auth |
| GET | `/api/v1/ui/bootstrap` | Bootstrap UI, basic_auth |
| GET | `/api/v1/health`, `/api/v1/version` | Publik tanpa auth |

## Pertanyaan umum / pemecahan masalah
- **Selalu 401 walau password benar**: hash tidak dikutip tunggal sehingga `$` terpotong — cek `make preflight`.
- **Tanpa isi `.env`**: UI terkunci (hash pengunci bawaan), bukan terbuka.
- **Lupa password**: buat hash baru (langkah 1–3); password lama tidak bisa dipulihkan dari hash.
- **Diblokir setelah salah password berkali-kali** (koneksi ditolak, bukan 401): IP Anda di-ban fail2ban —
  [fail2ban.md](fail2ban.md) → *Unban*.
- **Cek cost hash tanpa menampilkan hash:**
  `grep '^WEB_BASIC_AUTH_HASH=' .env | cut -c1-28 | sed -E 's/.*(\$2[aby]\$[0-9]{2}\$).*/\1/'` → mis. `$2a$10$`.
