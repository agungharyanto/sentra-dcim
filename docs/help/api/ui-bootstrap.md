# Endpoint Bootstrap UI

**Modul:** api / ui · **Tersedia sejak:** v0.2.1 · **Diperbarui:** v0.2.3 · **Status:** Available

## Tujuan
Memberi UI web semua yang dibutuhkan saat start: nama aplikasi, versi, bahasa default, daftar bahasa yang didukung,
zona waktu tampilan, dan **registry menu** (satu sumber di backend). UI tidak menyimpan daftar menu sendiri.

## Role yang dibutuhkan
| Aksi | Role |
|---|---|
| `GET /api/v1/ui/bootstrap` lewat domain publik | Kredensial **basic_auth sementara** (sampai Auth v0.3, ISS-026) |
| `GET /api/v1/ui/bootstrap` di server (`127.0.0.1:8080`) | Akses ke server |

## Langkah-langkah
1. Di server: `curl -s http://127.0.0.1:8080/api/v1/ui/bootstrap | jq`.
2. Lewat domain publik (basic_auth): `curl -s -u <user> https://sentra.bajastu.id/api/v1/ui/bootstrap | jq`
   (curl menanyakan password; jangan menulis password di command line).
3. Arti field:
   - `default_locale` — bahasa default (`DEFAULT_LOCALE` di `.env`, default `id`). Pengguna bisa mengganti bahasa
     di UI; pilihannya disimpan di browser dan mengalahkan default ini.
   - `timezone` — zona waktu tampilan (`DISPLAY_TIMEZONE`, default `Asia/Jakarta`). Data tetap UTC.
   - `menu_groups[]` — seksi sidebar: `key` (`monitoring`, `infrastructure`, `operations`, `system`) dan `order`.
     Label diterjemahkan UI dari `shell:groups.<key>`.
   - `menus[]` — `key`, `path`, `icon` (nama ikon lucide), `status` (`available` / `coming_soon` /
     `not_connected`), `group` (salah satu `menu_groups`), `order` (urutan di dalam grup). **Label tidak dikirim**:
     UI menerjemahkan `key` dari katalog `shell:sidebar.<key>`.
   - **Berubah di v0.2.3 (breaking):** `menu_groups` dan `menus[].group` baru; path Dasbor `/` → `/dashboard`.

### Mengubah bahasa default
1. Edit `.env` sebagai pemiliknya: `sudo -u sentra -H nano /opt/sentra-dcim/.env` → `DEFAULT_LOCALE=en`.
2. `make up`. Nilai selain `id`/`en` ditolak saat start (lihat log app).

### Menambah/mengubah menu (pengembang)
1. Edit `groups`/`registry` di `internal/ui/service/service.go`.
2. Perbarui fixture bersama: `go test ./internal/ui/service -update` (menulis `web/src/test/fixtures/ui-registry.json`;
   tanpa ini `go test` gagal).
3. Tambahkan label di **semua** katalog `web/src/i18n/locales/*/shell.json`: `sidebar.<key>`, `menuDescription.<key>`
   (dan `groups.<key>` untuk grup baru).
4. Menu berstatus `available` **wajib** punya halaman di `moduleRegistry` (`web/src/app/routes.tsx`) — test
   `routes.test.tsx` gagal bila belum. `coming_soon`/`not_connected` otomatis memakai halaman standar.
5. `go test ./internal/ui/...` dan `make web-check`.

## Endpoint API terkait
| Metode | Path | Keterangan |
|---|---|---|
| GET | `/api/v1/ui/bootstrap` | Dokumen bootstrap UI (`UiBootstrap` di `api/openapi.yaml`), `Cache-Control: no-store` |
| GET | `/api/v1/version` | Versi publik; sama dengan `version` di bootstrap |

## Pertanyaan umum / pemecahan masalah
- **401 di browser/curl**: basic_auth belum diisi atau salah — lihat [admin/web-basic-auth.md](../admin/web-basic-auth.md).
- **UI menampilkan "Konfigurasi aplikasi gagal dimuat"**: cek `make ps` (app healthy?) dan endpoint di langkah 1.
