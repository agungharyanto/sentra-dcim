# Pusat Bantuan Sentra DCIM

Pusat Bantuan berisi panduan setiap menu/fitur Sentra DCIM. Mulai v0.2, halaman-halaman ini ditampilkan langsung
di dalam aplikasi; sebelum itu dibaca dari repo.

## Aturan penulisan

Setiap fitur yang selesai **wajib** memiliki halaman `docs/help/<modul>/<fitur>.md` (CLAUDE.md aturan 7).
Fitur tanpa halaman bantuan dianggap **belum Done**. Gunakan templat berikut:

```markdown
# <Nama Fitur>

**Modul:** <modul> · **Tersedia sejak:** vX.Y.Z · **Status:** Available | Coming Soon | Not Connected

## Tujuan
Apa yang diselesaikan fitur ini dan untuk siapa.

## Role yang dibutuhkan
| Aksi | Role |
|---|---|

## Langkah-langkah
1. ...

## Endpoint API terkait
| Metode | Path | Keterangan |
|---|---|---|

## Pertanyaan umum / pemecahan masalah
```

## Daftar halaman

| Modul | Fitur | Halaman | Sejak |
|---|---|---|---|
| Ops | Infrastruktur & hardening server | [ops/infrastruktur.md](ops/infrastruktur.md) | v0.1.2 |
| Ops | Backup, restore & rollback | [ops/backup-restore.md](ops/backup-restore.md) | v0.1.4 |
| Ops | Akses darurat server (SSH hilang) | [ops/akses-darurat.md](ops/akses-darurat.md) | v0.1.8 |
| Admin | Domain & HTTPS | [admin/domain-https.md](admin/domain-https.md) | v0.1.6 |
| Admin | Akses UI web sementara (basic_auth) | [admin/web-basic-auth.md](admin/web-basic-auth.md) | v0.2.1 |
| API | Endpoint sistem (health, ready, version) | [api/system.md](api/system.md) | v0.1.3 |
| API | Bootstrap UI (bahasa, zona waktu, registry menu) | [api/ui-bootstrap.md](api/ui-bootstrap.md) | v0.2.1 |

Mulai v0.2 Pusat Bantuan dua bahasa: konvensi di [id/README.md](id/README.md) dan [en/README.md](en/README.md);
halaman di atas dipindah ke `id/` pada v0.2.4.

## Dokumentasi API

Spesifikasi lengkap REST API ada di [`api/openapi.yaml`](../../api/openapi.yaml) (OpenAPI 3.1). Mulai v0.2
spesifikasi ini dirender di Pusat Bantuan.
