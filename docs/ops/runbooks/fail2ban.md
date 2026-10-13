# fail2ban: Ban Login basic_auth yang Salah

**Modul:** ops · **Tersedia sejak:** v0.2.5 · **Status:** Available (sementara, dicabut bersama basic_auth di v0.3)

## Tujuan
Mengurangi beban CPU akibat serbuan login basic_auth yang salah (setiap percobaan = satu hitungan bcrypt, lihat
[web-basic-auth.md](web-basic-auth.md)). Brute force bukan ancaman utama (password acak 24 byte); yang dimitigasi
adalah **DoS CPU**. ISS-026, v0.2.5.

## Cara kerja
| Komponen | Isi |
|---|---|
| Log | Caddy menulis access log JSON ke volume `caddy_logs` (di container `/var/log/caddy/access.log`; di host: `docker volume inspect sentra_caddy_logs -f '{{.Mountpoint}}'`), rotasi 20 MiB × 10 berkas |
| Filter `sentra-caddy-auth` | Hanya baris dengan **status 401 + header `Authorization` ada** (kredensial salah) + host publik. 401 tanpa `Authorization` (challenge browser normal) **tidak** dihitung. Tidak bergantung urutan key JSON |
| Jail `sentra-web` | 10 percobaan salah dalam 10 menit → ban 1 jam; pelanggaran berulang dilipatgandakan sampai maks. 24 jam. `ignoreip` hanya localhost (IP admin **tidak** dikecualikan) |
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

### Test otomatis & kredensial salah
- **Setiap request dengan kredensial salah dihitung**, bukan hanya satu per run. Karena itu `test/api/ui.sh` dan
  `test/e2e/run.sh` melakukan **satu probe kredensial di awal** (`GET /api/v1/ui/bootstrap`); bila 401 keduanya
  berhenti dengan pesan *"kredensial salah"* tanpa mengirim percobaan lain (exit 3). Bila koneksi ditolak (curl
  exit 7) pesannya mengarah ke ban; TLS/DNS/timeout dilaporkan terpisah.
- Jumlah percobaan salah per run: `ui.sh` **1** (probe gagal) atau **1** (cek "password salah" yang disengaja, bila
  kredensial benar); `run.sh` **1** (probe gagal) atau **0**; `public.sh` **1** (cek log). Skenario terburuk —
  password salah, lalu `test-ui` dan `test-e2e` — = 2, jauh di bawah `maxretry` 10 per 10 menit.
- **Trafik dari server ini ke domain publiknya BISA di-ban.** Server tidak memegang IP publiknya secara lokal (IP
  publik di-NAT gateway OCI), sehingga request ke `https://sentra.bajastu.id` dari server keluar lalu masuk lagi
  lewat DNAT → FORWARD → `DOCKER-USER` dan tercatat dengan **IP publik server**. Bila IP itu terkena ban, semua
  test dan `curl` dari server ke domain gagal dengan *connection refused*; pengguna lain tidak terdampak. Pulihkan:
  `sudo fail2ban-client set sentra-web unbanip <IP publik server>` (hanya jail `sentra-web`).
- `make test-e2e` menjalankan Playwright dengan `--network host`: sumbernya host (IP publik server via hairpin OCI),
  **bukan** jaringan bridge Docker, jadi tidak pernah mem-ban IP gateway Docker. Gateway hanya tercatat untuk
  container di jaringan bridge yang memanggil port publik (MASQUERADE, ISS-034) — tidak ada test yang melakukannya.
- **Kenapa tidak menambah pengecualian `ignoreip`:**
  - *Subnet Docker (172.16.0.0/12)*: bila kelak ada proxy/tunnel di dalam Docker yang meneruskan trafik luar
    (ter-MASQUERADE), semua penyerang akan tampil sebagai gateway dan **perlindungan hilang total**. Tidak dipakai.
  - *IP publik server*: tidak bisa dipalsukan (handshake TCP), risikonya rendah, tetapi dengan probe fail-fast tidak
    diperlukan. Opsi ini dicatat bila test dari server kelak sering terkena ban.
- CI tidak memakai fail2ban (hanya `fail2ban-regex` di container).
- Uji ban nyata memakai **network namespace** (bukan container): Docker me-MASQUERADE trafik dari jaringan
  container ke port publik, sehingga yang tercatat & terban adalah IP gateway bridge, bukan sumber uji.
- **Uji reboot sungguhan** (bukan hanya restart service) masuk full regression v0.2.9.

### Mencabut semuanya (v0.3, bersama basic_auth — ISS-026)
```
make fail2ban-web-remove      # hapus jail, filter, drop-in; reload fail2ban (jail sshd tetap)
```
Lalu hapus logger `access_file` di Caddyfile dan volume `caddy_logs` bila tidak dipakai fitur lain
(`docker volume rm sentra_caddy_logs` setelah `make up`).

## Pertanyaan umum / pemecahan masalah
- **`make fail2ban-web`: "access log … not found"**: jalankan `make up` lalu buka situs sekali.
- **Ban tercatat tapi trafik tidak terblokir**: pastikan `iptables -S DOCKER-USER` berisi `-j f2b-sentra-web`;
  jalankan `make fail2ban-web` (memasang ulang) atau `sudo systemctl restart fail2ban`.
- **Peringatan `'allowipv6' not defined`**: dari `fail2ban.conf` bawaan paket Ubuntu, tidak berbahaya.
