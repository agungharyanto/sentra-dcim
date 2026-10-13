# Endpoint Sistem (Health, Ready, Version)

**Modul:** api / system · **Tersedia sejak:** v0.1.3 · **Diperbarui:** v0.1.8 · **Status:** Available

## Tujuan
Memantau apakah layanan Sentra DCIM hidup, siap melayani, dan versi berapa yang sedang berjalan.
Dipakai oleh healthcheck container, reverse proxy, monitoring eksternal, dan tim NOC saat troubleshooting.

## Role yang dibutuhkan
| Aksi | Role |
|---|---|
| `/health`, `/version` (hanya nomor versi) lewat domain publik | Publik (tanpa login) |
| `/ready` dan `/version` lengkap (commit, build, waktu server) | Akses ke server (listener internal `127.0.0.1:8080`). Mulai v0.3: role **Admin** |

Detail error tidak pernah dikirim ke klien, hanya ke log server.

## Langkah-langkah
1. **Cek hidup (liveness):** `curl -s http://127.0.0.1:8080/api/v1/health` → `{"status":"ok"}`.
2. **Cek siap (readiness):** `curl -s http://127.0.0.1:8080/api/v1/ready | jq` → `status: ready` dan setiap
   `checks.<nama>.status: ok`. Bila `503`, lihat bagian pemecahan masalah.
3. **Cek versi (internal, di server):** `curl -s http://127.0.0.1:8080/api/v1/version | jq` → `version` sama
   dengan `git describe --tags --always --dirty` dari kode yang ter-checkout (cek cepat: `make version-check`).
   `server_time_utc` = waktu server (UTC); `server_time_display` = waktu tampilan menurut `DISPLAY_TIMEZONE`
   (default `Asia/Jakarta`, WIB, `+07:00`).
4. **Cek versi (publik):** `curl -s https://sentra.bajastu.id/api/v1/version` → hanya `{"version":"v0.1.8"}`.
   Commit, tanggal build, versi Go dan jam server sengaja tidak dibuka ke publik.

### Mengubah zona waktu tampilan
1. Edit `.env`: `DISPLAY_TIMEZONE=Asia/Jakarta` (nama IANA, mis. `Asia/Makassar`, `Asia/Jayapura`, `UTC`).
   Nama lama `APP_DISPLAY_TZ` masih dibaca bila `DISPLAY_TIMEZONE` kosong.
2. `make up`. Data di database tetap UTC; hanya tampilan yang berubah.
3. Mulai v0.3.5 zona waktu, format jam & tanggal serta bahasa diatur lewat menu **Settings – Regional**.

## Endpoint API terkait
| Metode | Path | Keterangan |
|---|---|---|
| GET | `/api/v1/health` | Liveness, tanpa cek dependensi. 200 bila proses hidup |
| GET | `/api/v1/ready` | Readiness: `database`, `redis`, `migrations`. 200 siap / 503 tidak siap atau sedang shutdown |
| GET | `/api/v1/version` | Publik: `{version}` saja. Internal: + commit, tanggal build, versi Go, waktu server UTC & tampilan |

Spesifikasi lengkap: [`api/openapi.yaml`](../../../api/openapi.yaml).

### Perilaku umum API (berlaku untuk semua endpoint)
- Header `X-Request-Id` di setiap respons — sertakan saat melapor masalah.
- Format error: `{"error":{"code":"not_found","message":"...","request_id":"..."}}`.
- Rate limit per IP klien: 20 req/detik (burst 40). Bila terlampaui: `429` + header `Retry-After`.
- Batas body request 1 MB; timeout request 30 detik.

## Pemecahan masalah
| Gejala | Penyebab umum | Tindakan |
|---|---|---|
| `checks.database: unavailable` | DB mati/password salah | `make logs SERVICE=db`, cek `.env` |
| `checks.redis: unavailable` | Redis mati/password salah | `make logs SERVICE=redis` |
| `checks.migrations: unavailable` | Skema DB belum dimigrasi atau *dirty* | `make migrate-version`, lalu `make migrate` |
| `/ready` 503 sesaat saat deploy | Server sedang graceful shutdown (draining) | Normal; tunggu container baru `healthy` |
| `version` ≠ `git describe` / `make version-check` gagal | Container belum di-rebuild setelah commit/tag | `make up` (selalu rebuild image dengan versi Git saat ini) |
| `version` berakhiran `-dirty` | Ada perubahan belum di-commit saat build | Commit/bersihkan working tree, lalu `make up` |
