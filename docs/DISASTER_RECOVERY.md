# Disaster Recovery

> Dokumen ini dilengkapi bertahap. Prosedur backup, restore, rollback, dan bootstrap server baru
> diselesaikan di v0.1.4 dan diuji di v0.1.9.

## Sasaran

| Parameter | Target awal (Pending/Default) |
|---|---|
| RPO (data hilang maksimum) | 24 jam (backup harian) |
| RTO (waktu pulih maksimum) | 2 jam ke server baru |
| Lokasi backup | `/var/backups/sentra` (lokal); offsite = Pending (D-009) |

## Komponen yang wajib dibackup

| Komponen | Cara | Keterangan |
|---|---|---|
| Database PostgreSQL/TimescaleDB | `pg_dump -Fc` terkompresi | Data utama |
| Volume `caddy_data` | arsip tar | Sertifikat TLS & state Caddy |
| File `.env` server | disalin manual ke brankas password | **Tidak** pernah ke Git |
