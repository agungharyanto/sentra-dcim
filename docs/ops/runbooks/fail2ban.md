# fail2ban: Ban IP dengan Login Aplikasi Gagal

**Modul:** ops · **Tersedia sejak:** v0.2.5 · **Diperbarui:** v0.3.3 (login aplikasi + MFA, basic_auth dicabut) ·
**Status:** Available

## Tujuan
Menghentikan tebakan kata sandi atau kode MFA dari satu alamat IP yang mencoba banyak akun. Per akun aplikasi sudah
mengunci 5× gagal → 15 menit dan membatasi laju per IP/akun; jail ini menambahkan ban di jaringan sehingga satu IP
tidak bisa terus memicu lockout akun lain (D-066). Sampai v0.3.2 jail yang sama menghitung login basic_auth salah
(ISS-026, dicabut di v0.3.3).

## Cara kerja
| Komponen | Isi |
|---|---|
| Log | Caddy menulis access log JSON ke volume `caddy_logs` (di container `/var/log/caddy/access.log`; di host: `docker volume inspect sentra_caddy_logs -f '{{.Mountpoint}}'`), rotasi 20 MiB × 10 berkas |
| Filter `sentra-app-login` | Hanya `POST` ke `/api/v1/auth/login`, `/auth/mfa/verify`, `/auth/mfa/enrol/confirm`, `/auth/mfa/recovery-codes` dengan **status 401** (kata sandi salah, pengguna tak dikenal, kode MFA salah/terpakai) + host publik. Tidak dihitung: `GET /auth/me` 401 (belum masuk, normal), 403 CSRF, 429. Tidak bergantung urutan key JSON; nilai header tidak bisa memalsukan field |
| Jail `sentra-web` | 10 login/kode gagal dari satu IP dalam 10 menit → ban 1 jam; pelanggaran berulang dilipatgandakan sampai maks. 24 jam. `ignoreip` = localhost + **`ADMIN_IPS`** di `.env` (divalidasi installer) |
| Blokir | `iptables` chain **`DOCKER-USER`** → `f2b-sentra-web`, port 80/443, `REJECT` (klien langsung gagal terhubung). Port publik di-DNAT ke container Caddy, jadi trafiknya lewat FORWARD/`DOCKER-USER`, bukan INPUT |
| Urutan boot | Drop-in systemd `fail2ban.service.d/sentra-docker.conf`: `After=`/`Requires=docker.service` → fail2ban ikut restart setelah Docker dan memasang ulang aturannya |
| Jail `sshd` | Tidak disentuh (file terpisah `jail.d/sentra.local`) |

File sumber (tanpa secret): `deploy/fail2ban/`. Dipasang oleh `make fail2ban-web` (idempoten, sudo).

**IPv6:** tidak berlaku saat ini — instance tidak punya alamat IPv6 global, domain tidak punya record AAAA, dan
Docker tidak mempublikasikan 80/443 lewat IPv6 (dicek v0.2.5). Bila IPv6 diaktifkan kelak, tambahkan blokir di
`ip6tables DOCKER-USER` dan uji ulang.

## Role yang dibutuhkan
| Aksi | Role |
|---|---|
| Semua langkah di halaman ini | Admin OS server (`sudo`) |

## Langkah-langkah

### Pasang / perbarui
```
make up                 # volume log harus ada dan berisi minimal satu request
make fail2ban-web       # pasang filter, jail, drop-in; verifikasi jail & DOCKER-USER
make fail2ban-test      # uji filter dengan sampel log sintetis (fail2ban-regex)
make fail2ban-ban-test  # uji ban nyata dari network namespace uji (198.51.100.10) → ditolak → unban
```
`make fail2ban-web` gagal dengan pesan jelas bila volume/log belum ada atau DOMAIN tidak valid.

### Saat rilis (D-057)
`make release` tidak memakai sudo. Sebelum merge dan sesudah redeploy, `scripts/fail2ban-web-check.sh`
membandingkan sha256 filter, jail dan drop-in terpasang dengan hasil render repo. Bila beda atau belum
terpasang, rilis berhenti (exit 1) dengan pesan *"run make fail2ban-web"*:
- gagal **sebelum merge** → belum ada yang di-merge/tag; jalankan `make fail2ban-web`, lalu ulangi `make release`;
- gagal **sesudah redeploy** (rilis mengubah filter/jail) → tag sudah di-push; cukup jalankan `make fail2ban-web`.

Cek manual kapan saja: `make fail2ban-web-check`. Tesnya (`make fail2ban-web-check-test`, juga di CI) memakai
direktori & repo git sementara, tanpa sudo.

### Cek status & IP yang di-ban
```
sudo fail2ban-client status sentra-web          # jumlah gagal, IP yang sedang di-ban
sudo iptables -S f2b-sentra-web                 # aturan REJECT aktif
sudo iptables -S DOCKER-USER                    # harus ada: -j f2b-sentra-web untuk 80,443
sudo tail -f /var/log/fail2ban.log | grep sentra-web
```

### Unban (termasuk bila Anda sendiri terkena)
Gejala: browser tidak bisa terhubung sama sekali ke `https://sentra.bajastu.id` (bukan 401), sementara dari jaringan
lain (mis. hotspot HP) bisa. SSH tetap bisa dipakai (jail ini hanya memblokir 80/443). Bila yang gagal adalah test
atau `curl` **dari server**, yang terban kemungkinan IP publik server sendiri (lihat *Test otomatis* di bawah).
**Jangan** memakai `unban --all` dan jangan menyentuh jail `sshd`.
```
sudo fail2ban-client status sentra-web           # temukan IP Anda di "Banned IP list"
sudo fail2ban-client set sentra-web unbanip <IP>
```
Bila SSH juga tidak bisa: itu jail `sshd`, ikuti [akses-darurat.md](akses-darurat.md).

### Lokasi log
| Log | Lokasi |
|---|---|
| Access log Caddy (input fail2ban) | volume `caddy_logs`: `$(docker volume inspect sentra_caddy_logs -f '{{.Mountpoint}}')/access.log`, atau `docker compose exec caddy tail /var/log/caddy/access.log` |
| Keputusan fail2ban | `/var/log/fail2ban.log` |
| Konfigurasi terpasang | `/etc/fail2ban/filter.d/sentra-caddy-auth.conf`, `/etc/fail2ban/jail.d/sentra-web.local`, `/etc/systemd/system/fail2ban.service.d/sentra-docker.conf` |

### Kebijakan data: retensi log akses
Access log berisi **IP klien** (data pribadi). Kebijakan: **disimpan maksimal 30 hari** (`roll_keep_for 720h` di
Caddyfile) dan maksimal ~220 MB (`roll_size 20MiB`, `roll_keep 10`), mana yang lebih dulu tercapai. Log ini **tidak**
ikut backup (`BACKUP_VOLUMES`). Nilai header `Authorization` tidak pernah tercatat (Caddy menulis `REDACTED`).

### Test otomatis & login salah
- `public.sh`, `ui.sh`, dan `make test-e2e` tidak mengirim login yang salah (cek CSRF menjawab 403 sebelum kredensial
  diperiksa, jadi tidak dihitung). Yang terhitung hanya salah ketik kata sandi/kode MFA oleh pemilik saat menjalankan
  test: skrip berhenti pada login/kode pertama yang ditolak (1 hitungan per run).
- **Trafik dari server ini ke domain publiknya BISA di-ban** (IP publik server via hairpin NAT OCI). Bila terjadi,
  semua test dan `curl` dari server ke domain gagal dengan *connection refused*. Pulihkan:
  `sudo fail2ban-client set sentra-web unbanip <IP publik server>` (hanya jail `sentra-web`), atau tambahkan IP itu
  ke `ADMIN_IPS` lalu `make fail2ban-web`.
- `ADMIN_IPS` dikecualikan dari ban. **Jangan** menambahkan subnet Docker (172.16.0.0/12): trafik luar yang
  ter-MASQUERADE akan tampil sebagai gateway dan perlindungan hilang.
- Uji ban nyata (`make fail2ban-ban-test`) memakai **network namespace** yang mengirim login gagal dengan nama
  pengguna acak tak dikenal (tidak mengunci akun asli; meninggalkan baris audit `auth.login.failure`).
- CI tidak memakai fail2ban (hanya `fail2ban-regex` di container).

### Mencabut jail
```
make fail2ban-web-remove      # hapus jail, filter, drop-in; reload fail2ban (jail sshd tetap)
```

## Pertanyaan umum / pemecahan masalah
- **`make fail2ban-web`: "access log … not found"**: jalankan `make up` lalu buka situs sekali.
- **`ADMIN_IPS: invalid IP/CIDR`**: perbaiki format di `.env` (dipisah koma, mis. `203.0.113.5,198.51.100.0/24`).
- **Setelah upgrade ke v0.3.3**: `make fail2ban-web` memasang filter baru dan menghapus filter lama
  `sentra-caddy-auth.conf`; `make release` menolak melanjutkan selama filter lama masih ada.
- **Ban tercatat tapi trafik tidak terblokir**: pastikan `iptables -S DOCKER-USER` berisi `-j f2b-sentra-web`;
  jalankan `make fail2ban-web` (memasang ulang) atau `sudo systemctl restart fail2ban`.
- **Peringatan `'allowipv6' not defined`**: dari `fail2ban.conf` bawaan paket Ubuntu, tidak berbahaya.
