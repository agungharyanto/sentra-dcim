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

## Isi backup

Setiap backup adalah satu direktori `/var/backups/sentra/<UTC-stamp>_<label>/`:

| File | Isi |
|---|---|
| `db.dump` | `pg_dump --format=custom --compress=6` database aplikasi |
| `db.toc` | Daftar isi dump (bukti dump terbaca) |
| `globals.sql` | Role & atribut global (`pg_dumpall --globals-only`) |
| `vol_caddy_data.tar.gz`, `vol_caddy_config.tar.gz` | Sertifikat TLS Let's Encrypt, kunci akun ACME & state Caddy |
| `vol_redis_data.tar.gz` | Data Redis (cache/antrian) |
| `manifest.json` | Waktu, label, versi aplikasi, commit, versi skema DB, host |
| `SHA256SUMS` | Checksum semua file — diverifikasi saat backup & sebelum restore |

**Tidak** termasuk: file `.env` (berisi secret). Simpan salinan `.env` di brankas password tim (bukan Git, bukan
folder backup yang sama tanpa enkripsi).
`.env` milik user `sentra` (mode 640, ACL `ubuntu:r`). Ubah **sebagai `sentra`** (`sudo -u sentra -H nano .env`);
jangan `sed -i` sebagai admin — file diganti dan pemiliknya berubah, lalu backup dijalankan sebagai user yang salah
(ISS-024).

Setiap backup/restore/rollback dicatat di tabel `ops_runs` (mulai v0.1.4).

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
console — [ops/runbooks/kernel-rollback.md](ops/runbooks/kernel-rollback.md) (v0.2.9). Kernel `6.17.0-1020-oracle`
dipertahankan sebagai target rollback sampai penutup v0.3 (ISS-036). Setelah boot apa pun: `make post-reboot-check`.
