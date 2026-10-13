# Sentra DCIM

Sistem Data Center Infrastructure Management (DCIM) untuk Dinas Komunikasi, Informatika dan Statistik
Provinsi Bali.

## Mulai cepat

```bash
cp .env.example .env && chmod 600 .env   # isi semua nilai CHANGE_ME
make hooks                              # pasang pre-commit gitleaks
make up                                 # build + jalankan container + migrasi
make test-api                           # smoke test API
```

Jalankan `make help` untuk daftar perintah lengkap.

## Dokumentasi

| Dokumen | Isi |
|---|---|
| [CLAUDE.md](CLAUDE.md) | Aturan permanen proyek |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Roadmap & scope terkunci |
| [docs/CHANGELOG.md](docs/CHANGELOG.md) | Riwayat perubahan |
| [docs/DECISIONS.md](docs/DECISIONS.md) | Catatan keputusan |
| [docs/DEPENDENCIES.md](docs/DEPENDENCIES.md) | Dependensi + versi + lisensi |
| [docs/ISSUES.md](docs/ISSUES.md) | Issue open & solved |
| [docs/DISASTER_RECOVERY.md](docs/DISASTER_RECOVERY.md) | Backup, restore, rollback, server baru |
| [docs/help/](docs/help/README.md) | Pusat Bantuan (ID/EN, tampil di UI) |
| [docs/ops/runbooks/](docs/ops/runbooks/) | Runbook ops server (tidak tampil di UI) |
| [docs/dev/](docs/dev/) | Catatan developer |
| [api/openapi.yaml](api/openapi.yaml) | Spesifikasi REST API |
