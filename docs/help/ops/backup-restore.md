# Backup, Restore & Rollback

**Modul:** ops · **Tersedia sejak:** v0.1.4 · **Status:** Available (khusus administrator server)

## Tujuan
Melindungi data Sentra DCIM dari kehilangan/kerusakan dan memungkinkan kembali ke versi aplikasi sebelumnya
dengan aman. Prosedur lengkap & sasaran RPO/RTO: [docs/DISASTER_RECOVERY.md](../../DISASTER_RECOVERY.md).

## Role yang dibutuhkan
| Aksi | Role |
|---|---|
| Backup, restore uji (`--target-db`) | Administrator server (user `sentra`, grup `docker`) |
| Restore penuh, rollback | Administrator server — **wajib** koordinasi jendela maintenance (layanan berhenti beberapa menit) |
| Memasang timer backup, bootstrap server baru | Administrator server dengan `sudo` |

## Langkah-langkah

### Backup manual
1. `make backup` (atau `ops/backup.sh <label>`).
2. Baris terakhir output = lokasi backup, mis. `/var/backups/sentra/20261012T180000Z_manual`.

Backup otomatis berjalan setiap hari 02:00 WITA (`systemctl list-timers sentra-backup.timer`).
Pasang dengan `make backup-timer`.

### Uji restore (tanpa mengganggu produksi)
1. `ops/restore.sh /var/backups/sentra/<dir> --target-db sentra_restore_check --yes`
2. Periksa data di database `sentra_restore_check`, lalu hapus: `DROP DATABASE sentra_restore_check;`

### Restore penuh
1. Umumkan maintenance.
2. `ops/restore.sh /var/backups/sentra/<dir>` → ketik `restore` untuk konfirmasi.
3. Skrip otomatis membuat safety backup `*_pre-restore`, memulihkan DB + volume, menyalakan stack,
   dan menunggu `/api/v1/ready` = 200.

### Rollback versi aplikasi
1. `ops/rollback.sh v0.1.3` → ketik `rollback`.
2. Kembali ke versi terbaru: `git checkout main && ops/rollback.sh main`.

## Endpoint API terkait
| Metode | Path | Keterangan |
|---|---|---|
| GET | `/api/v1/ready` | Dipakai skrip untuk memastikan stack siap setelah restore/rollback |
| GET | `/api/v1/version` | Dipakai rollback untuk memverifikasi versi yang berjalan = tag target |

Riwayat operasi tersimpan di tabel database `ops_runs` (API untuk melihatnya direncanakan setelah v0.3 Auth).

## Pemecahan masalah
| Gejala | Tindakan |
|---|---|
| `checksum verification failed` | Backup rusak/terubah — gunakan backup lain |
| `/ready` 503 `migrations` setelah restore | Backup dari versi lebih baru; checkout tag yang sesuai lalu `make up` |
| Rollback gagal di tengah jalan | Jalankan `ops/restore.sh <path backup yang dicetak di awal rollback>` |
| `another backup is running` | Tunggu backup sebelumnya selesai (lock `/var/backups/sentra/.lock`) |
