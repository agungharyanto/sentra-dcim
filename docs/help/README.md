# Pusat Bantuan — konvensi penulisan (developer)

File ini untuk pengembang dan **tidak** tampil di UI. Halaman yang tampil di menu **Pusat Bantuan** ada di
`docs/help/{id,en}/`, di-embed ke binary backend (`docs/help/embed.go`) dan disajikan lewat
`GET /api/v1/help/index` dan `GET /api/v1/help/pages/{slug}` (CLAUDE.md aturan 7, v0.2.4).

## Lokasi & slug

- `docs/help/<locale>/<slug>/index.md` — halaman utama sebuah topik/modul; slug = nama folder (`dashboard`).
- `docs/help/<locale>/<slug>/<sub>.md` — halaman tambahan; slug = `<slug>/<sub>`.
- Slug memakai huruf kecil, angka, `-`, dan `/`. **Slug ID = slug EN** (set identik, dicek `go test`).
- Hanya audience **user** dan **admin** (pengguna aplikasi). Dokumen developer/ops (runbook server, deploy,
  recovery) ada di `docs/ops/runbooks/` dan `docs/dev/`, **bukan** di sini.

## Front matter (wajib)

```yaml
---
title: Dasbor            # judul halaman (wajib)
audience: user           # user | admin  (v0.3: dipakai menyaring halaman per role, ISS-032)
menu: dashboard          # opsional: key registry menu; tombol "?" pada menu itu membuka halaman ini
order: 30                # urutan di daftar (kecil = atas)
---
```

## Aturan isi (divalidasi otomatis, `go test ./internal/help/...` — CI gagal bila dilanggar)

- Setiap menu berstatus `available` di registry **wajib** punya halaman dengan `menu: <key>` di kedua bahasa.
- `menu` harus key registry yang ada; `audience` hanya `user`/`admin`; field lain ditolak.
- Tautan antarhalaman: path `.md` relatif (`../dashboard/index.md`) — target harus ada. Tautan absolut hanya ke
  route aplikasi (`/dashboard`, `/help/api/reference`). Tautan eksternal `https://`.
- **Tanpa HTML mentah** (tag `<...>`) di luar blok kode/inline code. Placeholder seperti `<user>` boleh di dalam
  `` `kode` ``.
- **Tanpa detail infrastruktur**: alamat IPv4, path server (`/opt/`, `/etc/`, `/home/`), file `.env`, nama user OS
  (`ubuntu`, `root`), `sudo`, `ssh`. Detail seperti itu milik runbook ops.
- Bahasa Indonesia adalah acuan; versi EN setara isinya.
- Tanpa Mermaid/diagram berbasis skrip.

## Isi saat ini

| Slug | ID | EN | Audience | Menu |
|---|---|---|---|---|
| `help` | Pusat Bantuan | Help Center | user | `help` |
| `getting-started` | Memulai | Getting started | user | — |
| `dashboard` | Dasbor | Dashboard | user | `dashboard` |
| `api` | Dokumentasi API | API documentation | admin | — |

Referensi API (viewer read-only, dari `api/openapi.yaml`): route UI `/help/api/reference`.
