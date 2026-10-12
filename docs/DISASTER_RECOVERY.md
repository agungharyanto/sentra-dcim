# Disaster Recovery

Prosedur backup, restore, rollback, dan pembangunan ulang server Sentra DCIM.
Prosedur ini diuji pada setiap penutup sprint (`.9`); hasil uji terakhir ada di `docs/releases/`.

## Sasaran

| Parameter | Target (Pending/Default, D-009/D-010) |
|---|---|
| RPO (data hilang maksimum) | 24 jam — backup otomatis harian 02:00 WITA (`sentra-backup.timer`) |
| RTO (waktu pulih maksimum) | 2 jam ke server baru |
| Lokasi backup | `/var/backups/sentra` (lokal, mode 0700). Offsite = **Pending** (hook `BACKUP_OFFSITE_CMD`) |
| Retensi | 14 hari (`BACKUP_RETENTION_DAYS`) |

> ⚠️ Selama offsite belum diputuskan, backup hanya ada di disk server yang sama. Salin berkala ke lokasi lain
> secara manual (`scp -r /var/backups/sentra/<dir> ...`).

## Isi backup

Setiap backup adalah satu direktori `/var/backups/sentra/<UTC-stamp>_<label>/`:

| File | Isi |
|---|---|
| `db.dump` | `pg_dump --format=custom --compress=6` database aplikasi |
| `db.toc` | Daftar isi dump (bukti dump terbaca) |
| `globals.sql` | Role & atribut global (`pg_dumpall --globals-only`) |
| `vol_caddy_data.tar.gz`, `vol_caddy_config.tar.gz` | Sertifikat TLS & state Caddy |
| `vol_redis_data.tar.gz` | Data Redis (cache/antrian) |
| `manifest.json` | Waktu, label, versi aplikasi, commit, versi skema DB, host |
| `SHA256SUMS` | Checksum semua file — diverifikasi saat backup & sebelum restore |

**Tidak** termasuk: file `.env` (berisi secret). Simpan salinan `.env` di brankas password tim (bukan Git, bukan
folder backup yang sama tanpa enkripsi).

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

Prasyarat: deploy key SSH server baru sudah ditambahkan ke repo GitHub (read-only cukup), dan file `.env`
lama + direktori backup terakhir sudah disalin ke server (mis. `/root/sentra.env`, `/root/restore/<dir>`).

```bash
# 1. git
sudo apt-get update && sudo apt-get install -y git
# 2. clone (sebagai user dengan sudo; ganti tag sesuai rilis terakhir)
sudo git clone --branch v0.1.9 git@github.com:agungharyanto/sentra-dcim.git /opt/sentra-dcim
# 3. bootstrap: Docker CE, user sentra, .env, hardening, timer backup, make up, restore, smoke test
sudo /opt/sentra-dcim/ops/bootstrap-new-server.sh --ref v0.1.9 --env-file /root/sentra.env --restore-from /root/restore/<dir>
```

Tanpa `--env-file`, `.env` baru dengan secret acak dibuat otomatis. Tanpa `--restore-from`, database kosong.
`--dry-run` menampilkan langkah tanpa mengeksekusi. Setelah selesai: cek `make ps` dan `sudo ufw status`.

> Catatan: langkah 2 di-clone sebagai root memakai key root; langkah 3 mengubah kepemilikan repo ke user `sentra`
> secara rekursif. Agar `sentra` bisa `git fetch` di kemudian hari, pasang deploy key untuk user tersebut.

## Kontak & eskalasi

| Peran | Kontak |
|---|---|
| Pemilik sistem | Diskominfo Provinsi Bali — *Pending* |
| Engineer utama | *Pending* |
