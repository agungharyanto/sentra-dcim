# Domain & HTTPS

**Modul:** admin · **Tersedia sejak:** v0.1.6 · **Status:** Available (khusus administrator server)

## Tujuan
Sentra DCIM diakses dari internet lewat **https://sentra.bajastu.id**. Caddy mengurus sertifikat TLS Let's Encrypt
secara otomatis (terbit & perpanjang), mengalihkan HTTP ke HTTPS, menambahkan security headers, dan menyaring
endpoint yang boleh diakses publik.

## Role yang dibutuhkan
| Aksi | Role |
|---|---|
| Mengakses `/api/v1/health` dan `/api/v1/version` lewat domain | Publik (tanpa login) |
| Mengubah domain, email ACME, atau port publik | Administrator server (`.env`, `sudo` untuk ufw) |
| Akses API lengkap (mis. `/api/v1/ready`) | Administrator di server, lewat listener internal `http://127.0.0.1:8080` |

## Konfigurasi

Nilai di `.env` (tidak di-commit; contoh ada di `.env.example`):

| Variabel | Nilai produksi | Fungsi |
|---|---|---|
| `DOMAIN` | `sentra.bajastu.id` | Nama host publik; record DNS A harus menunjuk ke IP server |
| `ACME_EMAIL` | email tim | Akun Let's Encrypt, untuk pemberitahuan sertifikat bermasalah atau akan kedaluwarsa |
| `CADDY_PUBLIC_HTTP_BIND` | `0.0.0.0:80` | Listener HTTP publik (redirect + challenge ACME HTTP-01) |
| `CADDY_PUBLIC_HTTPS_BIND` | `0.0.0.0:443` | Listener HTTPS publik |
| `CADDY_HTTP_BIND` | `127.0.0.1:8080` | Listener internal API lengkap, **jangan** dibuka ke luar |
| `UFW_OPEN_HTTP` | `1` | `ops/harden/ufw.sh` membuka 80/tcp dan 443/tcp |

## Langkah-langkah

### Menyalakan HTTPS (atau mengganti domain)
1. Pastikan record DNS A `DOMAIN` menunjuk ke IP publik server: `dig +short sentra.bajastu.id`.
2. Isi `DOMAIN` dan `ACME_EMAIL` di `.env`.
3. Buka firewall: `make harden` (atau `sudo ops/harden/ufw.sh`), lalu periksa dengan `sudo ufw status`: 80/tcp dan 443/tcp harus `ALLOW`.
4. Terapkan: `make up`.
5. Pantau penerbitan: `make logs SERVICE=caddy` → cari `certificate obtained successfully`.
6. Uji: `curl -I https://sentra.bajastu.id/api/v1/health` → `HTTP/2 200`. Uji lengkap: `test/api/public.sh`.

### Perilaku di proxy (sampai Auth tersedia di v0.3)
| Permintaan | Hasil |
|---|---|
| `GET`/`HEAD` `/api/v1/health`, `/api/v1/version` | Diteruskan ke aplikasi |
| Path atau metode lain (termasuk `/api/v1/ready`, `/`) | `403` `{"error":{"code":"forbidden",...}}` |
| `http://...` | `308` ke `https://...` |

Header pada setiap respons HTTPS: `Strict-Transport-Security` (1 tahun), `Content-Security-Policy`,
`X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, `Permissions-Policy`.
Header `Server` dan `Via` disembunyikan. TLS minimum 1.2 (1.0/1.1 ditolak); HTTP/3 (UDP) dimatikan.

## Endpoint API terkait
| Metode | Path | Keterangan |
|---|---|---|
| GET/HEAD | `/api/v1/health` | Publik lewat domain |
| GET/HEAD | `/api/v1/version` | Publik lewat domain |
| GET | `/api/v1/ready` | Hanya lewat `http://127.0.0.1:8080` (internal) |

Detail: [api/system.md](../api/system.md).

## Pemecahan masalah
| Gejala | Penyebab umum | Tindakan |
|---|---|---|
| Log `challenge failed` … `NXDOMAIN` | DNS baru dibuat, belum menyebar ke semua validator | Biasanya berhasil sendiri saat dicoba ulang; cek `dig +short` dari beberapa resolver |
| Log `Timeout during connect` | Port 80/443 tidak terjangkau dari internet (firewall upstream/NAT) | Buka port di firewall penyedia; bila mustahil gunakan DNS-01 (lihat di bawah) |
| Browser menolak sertifikat | Sertifikat *staging* atau internal CA (DOMAIN kosong) | Pastikan `DOMAIN` terisi, `make up`, cek issuer: `openssl s_client -connect DOMAIN:443` |
| `403 forbidden` untuk endpoint yang diperlukan | Endpoint belum dibuka publik | Disengaja; akses dari server lewat `127.0.0.1:8080` |

### Opsi DNS-01 (Pending, D-024)
Bila suatu saat port 80/443 tidak bisa dijangkau dari internet, sertifikat tetap bisa diterbitkan lewat challenge
DNS-01. Syaratnya: image Caddy dengan plugin DNS penyedia domain (`xcaddy build --with github.com/caddy-dns/<provider>`)
dan API token DNS di `.env`, lalu tambahkan `tls { dns <provider> {env.DNS_API_TOKEN} }` di blok situs. Saat ini
tidak diperlukan karena HTTP-01 berhasil.

### Catatan keamanan
Port container yang dipublish Docker **melewati ufw**. Yang benar-benar membatasi akses ke 80/443 adalah binding
di `docker-compose.yml`/`.env`, bukan ufw. Hanya Caddy yang boleh dipublish ke `0.0.0.0`; skrip regresi
memeriksa hal ini.
