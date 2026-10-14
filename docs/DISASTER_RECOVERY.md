# Disaster Recovery

Prosedur backup, restore, rollback, dan pembangunan ulang server Sentra DCIM.
Prosedur ini diuji pada setiap penutup sprint (`.9`); hasil uji terakhir ada di `docs/releases/`.

## Sasaran

| Parameter | Target (Pending/Default, D-009/D-010) |
|---|---|
| RPO (data hilang maksimum) | 24 jam — backup otomatis harian 02:00 Asia/Jakarta / WIB (`sentra-backup.timer`) |
| RTO (waktu pulih maksimum) | 2 jam ke server baru |
| Lokasi backup | `/var/backups/sentra` (lokal, mode 0700). Offsite = **Pending** (hook `BACKUP_OFFSITE_CMD`) |
| Retensi | 14 hari (`BACKUP_RETENTION_DAYS`) |

> ⚠️ Selama offsite belum diputuskan, backup hanya ada di disk server yang sama. Salin berkala ke lokasi lain
> secara manual (`scp -r /var/backups/sentra/<dir> ...`).

## Akses darurat (SSH hilang)

Latar belakang: server lama (v0.1) ditinggalkan karena akses SSH hilang. Server saat ini adalah VM **Oracle Cloud
Infrastructure (OCI)**, Ubuntu 24.04 aarch64, user admin `ubuntu`. Bagian ini wajib dibaca sebelum mengubah
konfigurasi SSH, firewall, atau fail2ban.

### Jalur pemulihan (urut prioritas)

| # | Jalur | Kapan dipakai | Prasyarat |
|---|---|---|---|
| 1 | SSH dengan **key cadangan** | Key utama hilang/rusak | Key cadangan sudah terpasang (lihat di bawah) |
| 2 | **OCI Serial Console** (Console Connection) | SSH tidak bisa sama sekali: sshd mati, firewall/fail2ban mengunci, salah konfigurasi jaringan | Akses OCI Console + IAM policy `manage instance-console-connection`; **user Linux dengan password** (serial console = login lokal `ttyAMA0`, tidak memakai `authorized_keys`) |
| 3 | **OCI VNC Console** | Sama seperti #2, bila butuh tampilan grafis | Sama seperti #2 |
| 4 | OCI **Run Command** (Oracle Cloud Agent) | Menjalankan perintah root tanpa login (mis. menambah key, unban) | Plugin *Compute Instance Run Command* aktif + dynamic group & IAM policy. Agent terpasang (`snap oracle-cloud-agent`) |
| 5 | Lepas boot volume → pasang ke VM lain | Semua jalur di atas gagal | Edit `/home/ubuntu/.ssh/authorized_keys` dari VM lain, pasang kembali |

> ✅ Status 2026-10-13 (ISS-016 selesai): terpasang **dua** key SSH untuk `ubuntu` (utama + cadangan; fingerprint dicatat
> pemilik di brankas) dan password konsol `ubuntu` sudah disetel pemilik (disimpan di brankas pemilik, bukan di repo/server). `PasswordAuthentication` tetap
> `no` (bawaan image Oracle, `/etc/ssh/sshd_config.d/60-cloudimg-settings.conf`): password hanya berlaku untuk
> serial/VNC console, bukan SSH.

### Checklist: kalau SSH hilang

Kerjakan berurutan; berhenti di langkah yang berhasil. **Jangan** menghapus/mengubah VM sebelum langkah 6.

1. **Pastikan masalahnya SSH, bukan jaringan/server mati.** `curl -I https://sentra.bajastu.id/api/v1/health`
   (200 = VM hidup). Coba dari jaringan lain (mis. hotspot HP) — beda hasil = IP Anda di-ban fail2ban.
2. **Coba key cadangan** (jalur #1): `ssh -i ~/.ssh/sentra_backup -o IdentitiesOnly=yes ubuntu@132.226.106.158`.
3. **Cek OCI Console**: instance *Running*? Security List subnet masih mengizinkan TCP 22 dari IP Anda?
   (perubahan Security List/NSG berlaku tanpa restart).
4. **Serial Console** (jalur #2): buat Console Connection → login `ubuntu` + password konsol (brankas). Lalu:
   - di-ban: `sudo fail2ban-client set sshd unbanip <IP>` (lihat *fail2ban* di bawah);
   - firewall: `sudo ufw allow 22/tcp` atau `sudo ufw disable` sementara, lalu `sudo ops/harden/ufw.sh`;
   - sshd: `sudo sshd -t` (tampilkan error konfigurasi), perbaiki file di `/etc/ssh/sshd_config.d/`,
     `sudo systemctl restart ssh`;
   - key hilang: tambahkan public key baru ke `/home/ubuntu/.ssh/authorized_keys` (mode 600).
5. **Run Command** (jalur #4) bila password konsol tidak diketahui: jalankan perintah langkah 4 sebagai root, atau
   `passwd ubuntu` untuk menyetel password konsol baru.
6. **Boot volume** (jalur #5): stop VM → detach boot volume → attach ke VM lain → perbaiki `authorized_keys` /
   `sshd_config.d` / `ufw` → attach kembali → start.
7. Setelah pulih: catat penyebab di `docs/ISSUES.md`, perbarui `ADMIN_IPS`, uji ulang kedua key.

Membuka serial console: OCI Console → Compute → Instances → *instance* → **Console connection** → *Create local
connection* (unggah public key SSH) → *Launch Cloud Shell connection* (paling mudah) atau salin perintah SSH yang
diberikan. Tekan Enter sampai muncul prompt `login:`, masuk dengan user yang memiliki password.

### Menyiapkan password konsol (sekali, dengan konfirmasi pemilik)

Password ini **hanya** untuk login lokal serial/VNC console; SSH tetap key-only selama `PasswordAuthentication no`.

```bash
sudo passwd ubuntu        # password kuat, simpan di brankas password tim
sudo passwd -S ubuntu     # harus "P" (bukan "L")
```

### Menambah key SSH cadangan

Buat key kedua di perangkat **lain** (laptop cadangan / brankas), bukan di server:

```bash
# di perangkat admin
ssh-keygen -t ed25519 -C "sentra-backup-$(date +%Y%m%d)" -f ~/.ssh/sentra_backup
```

Pasang di server (dari sesi SSH yang sedang aktif), lalu **uji dari terminal baru tanpa menutup sesi lama**:

```bash
# di server — tempel isi sentra_backup.pub
install -d -m 700 ~/.ssh
echo 'ssh-ed25519 AAAA... sentra-backup-YYYYMMDD' >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
# di perangkat admin, terminal BARU
ssh -i ~/.ssh/sentra_backup -o IdentitiesOnly=yes ubuntu@<IP>   # harus berhasil
```

Simpan private key cadangan di brankas password tim (bukan di Git, bukan di server). Opsional: tambahkan juga
key cadangan saat membuat Console Connection agar serial console bisa dibuka dari perangkat cadangan.

### Aturan perubahan SSH (wajib)

1. **Jangan** mengubah `PasswordAuthentication` (mematikan *atau* menyalakan) tanpa konfirmasi eksplisit pemilik.
2. Sebelum hardening SSH apa pun (password off, ganti port, `AllowUsers`, allowlist IP):
   - pastikan minimal **dua** key terpasang dan password konsol berfungsi (uji login serial console);
   - ubah konfigurasi lewat file di `/etc/ssh/sshd_config.d/`, validasi dengan `sudo sshd -t`;
   - `sudo systemctl reload ssh` (sesi aktif tidak terputus);
   - **uji login dengan key di sesi/terminal terpisah** (sesi lama tetap terbuka) — baru tutup sesi lama setelah
     login baru berhasil;
   - bila ganti port: buka port baru di ufw **dan** Security List OCI lebih dulu, jalankan ulang
     `sudo ops/harden/ufw.sh` dan `sudo ops/harden/fail2ban.sh` (port dibaca otomatis dari `sshd -T`).
3. Bila uji gagal: kembalikan file konfigurasi dari sesi lama, `sudo sshd -t && sudo systemctl reload ssh`.

### fail2ban: whitelist admin & unban

`ops/harden/fail2ban.sh` tidak pernah mem-ban (`ignoreip`): loopback, `SSH_ALLOW_FROM`, `ADMIN_IPS` di `.env`
(daftar IP/CIDR dipisah koma) dan IP sesi SSH yang **sedang aktif** saat skrip dijalankan. IP admin rumahan/seluler
biasanya dinamis — tambahkan rentang ISP ke `ADMIN_IPS` bila perlu, lalu jalankan ulang skrip:

```bash
sudo ops/harden/fail2ban.sh
sudo grep ignoreip /etc/fail2ban/jail.d/sentra.local
```

Bila admin terkunci (koneksi SSH *timeout* / *connection refused* dari satu IP, sementara dari IP lain bisa),
masuk lewat serial console (atau Run Command) lalu:

```bash
sudo fail2ban-client status sshd                 # daftar IP yang di-ban
sudo fail2ban-client set sshd unbanip <IP>       # unban satu IP
sudo fail2ban-client unban --all                 # unban semua (darurat)
sudo systemctl stop fail2ban                     # bila perlu, matikan sementara; nyalakan lagi setelah pulih
```

Lalu tambahkan IP tersebut ke `ADMIN_IPS` dan jalankan ulang `sudo ops/harden/fail2ban.sh`.

Ubah `.env` sebagai pemiliknya (`sudo -u sentra -H nano /opt/sentra-dcim/.env`), bukan `sed -i` (ISS-024).
Nilai `ADMIN_IPS` saat ini hanya ada di `.env` server (tidak dicatat di repo); IP admin dari ISP rumahan bisa berubah.

### Firewall berlapis di OCI

Lalu lintas masuk melewati **dua** firewall terpisah:

1. **Security List / NSG VCN** (dikelola di OCI Console) — port harus dibuka di sini lebih dulu.
2. **ufw** di host (`ops/harden/ufw.sh`). Image Oracle memasang aturan iptables `netfilter-persistent`
   (`/etc/iptables/rules.v4`) dengan `REJECT` yang berada **sebelum** chain ufw, sehingga semua `ufw allow`
   selain 22/tcp tidak berlaku. `ufw.sh` menghapus satu aturan itu (salinan asli:
   `/etc/iptables/rules.v4.sentra-orig`) dan mempertahankan aturan `InstanceServices` milik Oracle.

Bila port sudah dibuka di ufw tetapi tetap timeout dari luar, buktikan dulu paket sampai ke VM **sebelum** memicu
Let's Encrypt (jatah 5 validasi gagal/jam): `sudo tcpdump -ni enp0s6 'dst port 80 or dst port 443'` sambil
`curl -I http://<ip-publik>/` dari luar. Tidak ada paket = masalah di Security List (rule harus Ingress, di Security
List yang ter-attach ke subnet VM) atau NSG; ada paket = periksa host. Uji ke IP publik sendiri dari dalam VM tidak
bisa dipakai (OCI tidak meneruskan hairpin). Rule `REJECT icmp-host-prohibited` bawaan Oracle di chain FORWARD
berada setelah ACCEPT Docker, sehingga tidak memblokir port Caddy.

Bila firewall host mengunci SSH: dari serial console jalankan `sudo ufw allow 22/tcp` (atau `sudo ufw disable`
sementara), perbaiki, lalu `sudo ops/harden/ufw.sh`.

## Allowed IP: terkunci dari panel web (v0.3.6+, D-078)

Gejala: panel web/API menjawab `403` "Akses dari alamat IP ini tidak diizinkan" (`ip_not_allowed`) karena mode
**enforce** aktif dan IP Anda (mis. IP kantor berubah) tidak ada di daftar. `/api/v1/health` tetap 200.

Allowed IP **tidak** menyentuh SSH maupun ufw — server tetap bisa diakses lewat SSH (jalur #1) atau, bila SSH pun
gagal, **OCI Serial Console** (jalur #2 di atas: login `ubuntu` + password konsol dari brankas). Setelah masuk:

```bash
cd /opt/sentra-dcim
# Lihat mode & daftar
docker compose exec -T app /sentra allowlist list
# Pintu darurat: berhenti memblokir (berlaku ≤ 5 detik, tanpa restart; tercatat di audit sebagai "cli")
docker compose exec -T app /sentra allowlist mode off        # atau: mode monitor (tetap mencatat, tidak memblokir)
# Opsional: tambahkan IP yang benar, lalu aktifkan lagi enforce dari UI (dengan pengaman + konfirmasi 2 menit)
docker compose exec -T app /sentra allowlist add 203.0.113.10 "Kantor (IP baru)"
docker compose exec -T app /sentra allowlist remove 198.51.100.7/32
```

`sentra allowlist mode enforce` dari CLI ditolak tanpa `--force` (tanpa pengaman lockout dan tanpa auto-rollback —
hanya untuk uji); aktifkan enforce dari **Pengaturan › Allowed IP**.

Bila container app tidak berjalan sama sekali, Caddy menampilkan halaman **"Sedang pemeliharaan"** (503, sejak v0.3.7;
API menjawab JSON `maintenance`) — bukan 403; Allowed IP tidak berperan — pulihkan app dulu (`make up`, log: `make logs SERVICE=app`). Sebagai jalan terakhir tanpa app, mode bisa
diubah langsung di database (owner, container `ops`/`db`):
`DELETE FROM settings WHERE key IN ('allowlist.mode','allowlist.pending_until');` (= mode off), lalu restart app.

Pengaman yang mencegah keadaan ini lebih dulu: enforce hanya bisa disimpan bila IP penyimpan tercakup; menghapus
jaringan yang mencakup IP sendiri ditolak saat enforce; setiap peralihan ke enforce dari UI **kembali otomatis ke
monitor setelah 2 menit** kecuali Admin menekan *Pertahankan* dari koneksi yang masih tersambung.

## Isi backup

Setiap backup adalah satu direktori `/var/backups/sentra/<UTC-stamp>_<label>/`:

| File | Isi |
|---|---|
| `db.dump` | `pg_dump --format=custom --compress=6` database aplikasi |
| `db.toc` | Daftar isi dump (bukti dump terbaca) |
| `globals.sql` | Role & atribut global (`pg_dumpall --globals-only`) |
| `vol_caddy_data.tar.gz`, `vol_caddy_config.tar.gz` | Sertifikat TLS Let's Encrypt, kunci akun ACME & state Caddy |
| `vol_redis_data.tar.gz` | Data Redis (cache/antrian) |
| `audit-anchor.json` | Kepala hash chain audit log saat backup (`seq`, `hash`; v0.3.4, D-073) — **anchor eksternal**, diambil sebelum dump |
| `manifest.json` | Waktu, label, versi aplikasi, commit, versi skema DB, host |
| `SHA256SUMS` | Checksum semua file — diverifikasi saat backup & sebelum restore |

Sejak v0.3.1 `db.dump` juga berisi **hash kata sandi** (argon2id), hash token sesi, dan log audit — perlakukan
backup sebagai data rahasia (direktori 0700, file 0600, sudah diterapkan `ops/backup.sh`).

**Tidak** termasuk: file `.env` (berisi secret). Simpan salinan `.env` di brankas password tim (bukan Git, bukan
folder backup yang sama tanpa enkripsi).
`.env` milik user `sentra` (mode 640, ACL `ubuntu:r`). Ubah **sebagai `sentra`** (`sudo -u sentra -H nano .env`);
jangan `sed -i` sebagai admin — file diganti dan pemiliknya berubah, lalu backup dijalankan sebagai user yang salah
(ISS-024).

Setiap backup/restore/rollback dicatat di tabel `ops_runs` (mulai v0.1.4).

**Arsip audit log** (v0.3.4): event audit di luar masa simpan (`AUDIT_RETENTION_DAYS`, default 365, min 90)
dipindahkan oleh `make audit-prune` (juga timer backup harian, setelah backup sukses) ke
`/var/backups/sentra/audit-archive/audit-<dari>-<sampai>.jsonl.gz` + `.sha256`. Direktori ini **tidak** ikut
retensi backup (14 hari) — simpan selamanya dan ikutkan ke salinan offsite. Database hanya menyimpan checkpoint
(hash baris terakhir yang dipangkas) di `audit_checkpoints`. Verifikasi arsip offline:

```bash
sudo -u sentra -H sh -c 'cd /var/backups/sentra/audit-archive && sha256sum -c audit-*.sha256'
docker compose exec -T app /sentra audit verify-archive < /var/backups/sentra/audit-archive/<file>.jsonl.gz
# last_hash harus = audit_checkpoints.last_hash (through_seq yang sama); first_prev_hash = checkpoint sebelumnya
```

## Backup

```bash
make backup                      # = ops/backup.sh manual
ops/backup.sh pre-upgrade        # label bebas: a-z, 0-9, -
systemctl list-timers sentra-backup.timer   # jadwal otomatis
```

## Restore

```bash
# Uji restore ke database terpisah (aman, stack produksi tidak disentuh)
ops/restore.sh /var/backups/sentra/<dir> --target-db sentra_restore_check --yes

# Restore penuh ke database & volume produksi (app/caddy dihentikan sementara;
# safety backup otomatis diambil lebih dulu dengan label pre-restore)
ops/restore.sh /var/backups/sentra/<dir>
```

Restore memverifikasi checksum, membuat ulang database, menjalankan `timescaledb_pre_restore()` →
`pg_restore` → `timescaledb_post_restore()`, memastikan versi skema sama dengan manifest, memulihkan volume,
menyalakan stack, `migrate up`, dan menunggu `/api/v1/ready` = 200.

### Verifikasi hash chain audit setelah restore (v0.3.4+)

Restore membuat ulang role `sentra_app` lalu menjalankan `make migrate` (migrasi + `sentra db sync-app-role`, butuh
`DB_APP_PASSWORD` di `.env`). Setelah stack sehat, buktikan log audit hasil restore utuh dan cocok dengan backup:

```bash
# Chain dihitung ulang + dicocokkan dengan anchor backup s.d. backup yang di-restore (UNTIL = nama direktorinya).
# Backup yang LEBIH BARU memuat riwayat yang memang dibatalkan oleh restore; jangan ikutkan.
sudo -u sentra -H make -C /opt/sentra-dcim audit-verify UNTIL=<UTC-stamp>_<label>
```

Hasil `"ok": true` + `anchors_checked` ≥ 1 = utuh. Gagal → jangan pakai stack itu untuk operasi; restore backup
sebelumnya dan ulangi, lalu catat di `docs/ISSUES.md`. Verifikasi harian di app juga terlihat di menu **Audit Log**
(kartu *Integritas hash chain*). Tanpa restore, `make audit-verify` (tanpa `UNTIL`) memakai anchor semua backup —
menangkap penulisan ulang seluruh chain oleh owner database.

> Jika backup berasal dari versi aplikasi yang **lebih baru** dari kode yang ter-checkout (versi skema backup >
> migrasi di binary), `/ready` akan 503 (`migrations: unavailable`). Checkout versi yang sesuai lebih dulu.

## Rollback versi

```bash
ops/rollback.sh v0.1.3           # turun ke tag
git checkout main && ops/rollback.sh main   # naik kembali ke versi terbaru
```

Urutan otomatis: cek working tree bersih → **backup** (label `rollback`) → bila tag target punya migrasi lebih
sedikit, `migrate goto <versi target>` dengan **binary saat ini** (binary lama tidak punya file `down` migrasi
baru) → `git checkout` tag → `make up` (build image tag + `migrate up`) → verifikasi `/ready` 200 dan `/version`
= tag. Target minimum: `v0.1.3`. Bila gagal: `ops/restore.sh <backup yang dicetak>`.

Aturan migrasi (CLAUDE.md aturan 9): setiap migrasi wajib punya `down` dan backward-compatible.

> **Rollback melewati v0.3.1** (ke `v0.2.x`): `down` migrasi `000003_auth` menghapus tabel `users`, `sessions`, dan
> `audit_events` beserta isinya (akun, sesi, log audit). Backup otomatis `rollback` menyimpannya; naik kembali ke
> v0.3.1+ lalu `ops/restore.sh <backup>` bila data itu dibutuhkan.

## Prosedur darurat auth — ringkasan (v0.3.9)

Semua dari server (SSH, atau OCI Serial Console bila SSH pun gagal), di `/opt/sentra-dcim`, tercatat di audit sebagai
`system`/`cli`. Rincian tiap baris ada di bagian di bawahnya.

| Keadaan | Perintah | Catatan |
|---|---|---|
| Semua Admin kehilangan akses | `sudo -u sentra docker compose exec app /sentra user create-admin --username <nama>` | Kata sandi diketik (tidak lewat argumen); wajib ganti saat masuk |
| Akun terkunci (5× salah) | `sudo -u sentra docker compose exec app /sentra user unlock --username <nama>` | Atau tunggu 15 menit |
| HP + recovery code hilang | `sudo -u sentra docker compose exec app /sentra user reset-mfa --username <nama>` | Pastikan identitas dulu; semua sesi dicabut |
| Terkunci oleh Allowed IP | `sudo -u sentra docker compose exec -T app /sentra allowlist mode off` | Berlaku ≤ 5 detik; aktifkan lagi dari UI |
| `MFA_ENC_KEY` hilang | Pulihkan `.env` dari cadangan; bila tidak ada: key baru + `reset-mfa` semua akun | Lihat bagian MFA |
| Kata sandi Admin kedaluwarsa (v0.3.7) | Tidak perlu tindakan server: setelah masuk + MFA, aplikasi meminta kata sandi baru | Bukan lockout |
| Layar monitoring (kiosk) keluar sendiri | Periksa IP layar ada di Allowed IP; sesi dicabut bila dari IP lain (`auth.session.kiosk_revoked`) | Masuk ulang setelah IP benar |
| App mati (halaman "Sedang pemeliharaan") | `make up`; log `make logs SERVICE=app` | Caddy menjawab 503 + halaman statis |

## Pemulihan akses admin aplikasi (v0.3.1+)

Akun aplikasi (login Sentra DCIM) terpisah dari akun OS server.

**Pengguna biasa lupa kata sandi / akun terkunci / harus dicabut (v0.3.2+):** Admin memakai menu **Pengguna &
Role** di aplikasi — *Reset kata sandi* (kata sandi sementara tampil sekali, semua sesi ditutup, lockout dibuka) atau
*Nonaktifkan*. Tidak perlu akses server.

**Semua Admin kehilangan akses** (lupa kata sandi, terkunci, atau akun Admin dinonaktifkan): buat **Admin darurat
dari server** lewat CLI di container app; tidak ada jalur lewat web. Akun dari CLI langsung ber-role Admin.

```bash
cd /opt/sentra-dcim
sudo -u sentra docker compose exec app /sentra user create-admin --username <nama-atau-NIP> [--display-name "<Nama>"]
# Kata sandi diketik dua kali tanpa tampil di layar (12–128 karakter). Jangan lewat argumen/riwayat shell.
# Non-interaktif (mis. dari pengelola rahasia): satu baris di stdin
#   ... docker compose exec -T app /sentra user create-admin --username <nama> < file-mode-600
```

- Akun baru **wajib ganti kata sandi** saat pertama masuk; kata sandi sementara tidak berlaku lagi setelahnya.
- Nama pengguna yang sudah ada ditolak (`username already exists`): pakai nama lain (mis. `admin.darurat`), lalu
  setelah masuk tangani akun lama di menu Pengguna & Role (reset kata sandi atau nonaktifkan). Setelah keadaan
  normal, nonaktifkan akun darurat bila tidak dipakai lagi (akun tidak pernah dihapus — jejak audit).
- Aplikasi menolak mencabut role Admin atau menonaktifkan **Admin aktif terakhir** (dan akun sendiri), sehingga
  kondisi "tanpa Admin" lewat aplikasi tidak bisa terjadi; CLI tetap jalur cadangan.
- Akun terkunci karena salah kata sandi terbuka sendiri setelah 15 menit (`AUTH_LOCK_DURATION`).
- Setiap pembuatan tercatat di `audit_events` (`user.create_admin`, aktor `system`/`cli`), tanpa kata sandi.
- Rate limit login ada di Redis; bila Redis mati, login ditolak (`503 auth_unavailable`, gagal tertutup) — pulihkan
  Redis dulu (`docker compose ps`, `make logs SERVICE=redis`).

## MFA: HP hilang, Admin terkunci, atau `MFA_ENC_KEY` hilang (v0.3.3+)

Semua akun wajib MFA (D-062). Pemulihan, dari yang paling ringan:

1. **Pengguna masih punya recovery code** → masuk dengan recovery code, lalu *Keamanan akun* → buat ulang.
2. **Pengguna kehilangan HP & recovery code** → Admin: menu Pengguna & Role → pengguna → **Reset MFA**.
3. **Admin terakhir kehilangan HP & recovery code** (atau tidak ada Admin lain) → dari server, CLI darurat:
   ```bash
   cd /opt/sentra-dcim
   sudo -u sentra docker compose exec app /sentra user reset-mfa --username <nama-atau-NIP>
   ```
   MFA & recovery code akun dihapus, semua sesinya dicabut; saat masuk berikutnya akun memasang MFA lagi. Tercatat di
   `audit_events` (`user.mfa.reset`, aktor `system/cli`). Lakukan hanya setelah identitas pemilik akun dipastikan.
4. **Akun terkunci** (5× kata sandi/kode MFA salah): terbuka sendiri setelah 15 menit (`AUTH_LOCK_DURATION`), tercatat
   `auth.lockout` di `audit_events`. Lockout tidak bisa membuat akun terkunci selamanya: tiap lockout selalu berakhir,
   rate limit per IP/akun membatasi pemicunya, dan jail fail2ban `sentra-web` mem-ban IP
   penyerang. Bila Admin harus masuk sebelum 15 menit, buka darurat dari server:
   ```bash
   sudo -u sentra docker compose exec app /sentra user unlock --username <nama-atau-NIP>
   ```
   Tercatat `user.unlock` (aktor `system/cli`). Bila penguncian berulang, cek `audit_events` (`auth.login.failure`,
   `auth.mfa.verify` gagal) untuk IP sumber.
5. **`MFA_ENC_KEY` hilang/rusak** (mis. `.env` hilang tanpa backup): secret TOTP tidak bisa dibuka lagi → semua
   login gagal di langkah MFA. Pulihkan `.env` dari backup bila ada (`MFA_ENC_KEY` ikut di `.env`, ditambah ke cadangan
   offline bersama kata sandi DB). Bila tidak ada: buat key baru (`openssl rand -base64 32` → `MFA_ENC_KEY` di `.env`,
   diisi pemilik), `make up`, lalu reset MFA **semua** akun via CLI (`reset-mfa` per pengguna); semua pengguna
   memasang ulang. **Jangan** mengganti key yang masih berfungsi: tidak ada rotasi otomatis.

## Kredensial database: role owner & role app (v0.3.4+, ISS-039)

| Role | Dipakai oleh | Hak | Kredensial |
|---|---|---|---|
| `POSTGRES_USER` (owner) | container `ops` saja: migrasi, `db sync-app-role`, pemangkasan audit; skrip backup/restore di container db | DDL, owner tabel | `POSTGRES_PASSWORD` di `.env` |
| `DB_APP_USER` (default `sentra_app_login`, anggota `sentra_app`) | container `app` (server & CLI pengguna) | hanya DML yang di-GRANT; `audit_events` hanya INSERT+SELECT; tanpa DDL/TRUNCATE | `DB_APP_PASSWORD` di `.env` |

**Rotasi password role app** (pemilik mengisi nilai, tidak lewat chat):
```bash
cd /opt/sentra-dcim
sudo -u sentra -H nano .env               # ganti DB_APP_PASSWORD (openssl rand -hex 24)
sudo -u sentra make migrate               # sync-app-role memasang password baru
sudo -u sentra docker compose up -d app   # app memakai password baru
```
**Rotasi password owner** (`POSTGRES_PASSWORD`): ganti di database dulu
(`docker compose exec db psql -U "$POSTGRES_USER" -c '\password'`), lalu `.env` dengan nilai yang sama, lalu
`make up`. Image Postgres hanya memakai `POSTGRES_PASSWORD` saat inisialisasi volume pertama.

Setelah **restore** ke server baru: `make up` (lewat `make migrate`) membuat ulang role login app dari `.env`; grant
ikut dump (`restore.sh` memastikan role grup `sentra_app` ada sebelum `pg_restore`).

## Server baru (Ubuntu 24.04 kosong) — maksimal 5 perintah

Prasyarat: deploy key SSH server baru sudah ditambahkan ke repo GitHub (read-only cukup untuk clone; *Allow write access* bila server ini juga push — lihat D-041; bila langsung `Permission denied`, tunggu sebentar lalu ulangi `ssh -T git@github.com`), dan file `.env`
lama + direktori backup terakhir sudah disalin ke server (mis. `/root/sentra.env`, `/root/restore/<dir>`).

```bash
# 1. git
sudo apt-get update && sudo apt-get install -y git
# 2. clone (sebagai user dengan sudo; ganti tag sesuai rilis terakhir)
sudo git clone --branch v0.1.9 git@github.com:agungharyanto/sentra-dcim.git /opt/sentra-dcim
# 3. bootstrap: Docker CE, user sentra, .env, hardening, timer backup, make up, restore, smoke test
sudo /opt/sentra-dcim/ops/bootstrap-new-server.sh --ref v0.1.9 --env-file /root/sentra.env --restore-from /root/restore/<dir>
```

Opsi bootstrap:

| Opsi | Fungsi |
|---|---|
| `--env-file <path>` | Pakai `.env` lama (secret sama dengan backup). Tanpa opsi ini `.env` baru dengan secret acak dibuat otomatis |
| `--domain <fqdn> --acme-email <email>` | `.env` baru dalam **mode publik** (Let's Encrypt, 80/443 dibuka). Tanpa opsi ini `.env` baru dibuat dalam **mode lokal**: `DOMAIN=localhost` (CA internal Caddy), listener publik di `127.0.0.1`, `UFW_OPEN_HTTP=0` — tidak ada permintaan sertifikat sebelum DNS siap |
| `--admin-user <user>` | Beri user admin cloud (mis. `ubuntu` di Oracle) ACL baca/tulis repo + baca `.env`, dan `safe.directory` Git. Backup tetap milik `sentra` |
| `--restore-from <dir>` | Restore backup setelah stack menyala. Tanpa opsi ini database kosong |
| `--dry-run` | Tampilkan langkah tanpa eksekusi |

Pindah ke mode publik setelah record DNS A `sentra.bajastu.id` mengarah ke IP server baru: buka 80/443 di
Security List OCI, isi `DOMAIN`, `ACME_EMAIL`, `CADDY_PUBLIC_HTTP_BIND=0.0.0.0:80`,
`CADDY_PUBLIC_HTTPS_BIND=0.0.0.0:443`, `UFW_OPEN_HTTP=1` di `.env`, lalu `sudo ops/harden/ufw.sh` dan
`make up`, verifikasi `test/api/public.sh`. Sertifikat lama ikut dipulihkan dari volume `caddy_data` bila
`--restore-from` dipakai. Setelah selesai: cek `make ps` dan `sudo ufw status`.

### Catatan Oracle Cloud (uji nyata 2026-10-13, ISS-003)

Diuji pada VM OCI Ubuntu 24.04.5 aarch64 (4 OCPU, 23 GiB) dengan repo sudah di-clone ke `/opt/sentra-dcim`:

```bash
sudo /opt/sentra-dcim/ops/bootstrap-new-server.sh --ref <tag> --admin-user ubuntu   # ±2,5 menit
```

- User admin bawaan `ubuntu` (bukan `root`); SSH key-only, password terkunci — baca **Akses darurat** dulu.
- Image Oracle memasang iptables `REJECT` sebelum ufw dan `rpcbind` (port 111) — keduanya ditangani otomatis
  oleh `ops/harden/ufw.sh` dan `ops/harden/services.sh`.
- Port publik harus dibuka juga di Security List / NSG VCN (di luar server).
- Semua image container tersedia untuk `arm64`.

> Catatan: langkah 2 di-clone sebagai root memakai key root; langkah 3 mengubah kepemilikan repo ke user `sentra`
> secara rekursif. Agar `sentra` bisa `git fetch` di kemudian hari, pasang deploy key untuk user tersebut.

## Kontak & eskalasi

| Peran | Kontak |
|---|---|
| Pemilik sistem | Diskominfo Provinsi Bali — *Pending* |
| Engineer utama | *Pending* |

## Rollback kernel

Bila server tidak bisa boot atau rusak setelah pembaruan kernel: pilih kernel sebelumnya dari menu GRUB lewat serial
console — [ops/runbooks/kernel-rollback.md](ops/runbooks/kernel-rollback.md). Kernel `6.17.0-1020-oracle` sudah
**dipensiunkan di v0.3.9** (ISS-036, `scripts/kernel-retire.sh`) setelah `7.0.0-1013-oracle` berjalan stabil; menu GRUB
tetap tampil 10 detik dan memuat kernel yang ada (kernel lama berikutnya otomatis tersedia setelah pembaruan kernel
berikut, sebelum unattended-upgrades menghapusnya). Bila tidak ada kernel lain yang bisa boot: pulihkan **backup boot
volume OCI** (dibuat pemilik sebelum v0.3.9) atau pasang kernel lama dari serial console (`apt install
linux-image-<versi>`). Setelah boot apa pun: `make post-reboot-check`.
