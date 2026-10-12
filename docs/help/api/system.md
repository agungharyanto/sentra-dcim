# Endpoint Sistem (Health, Ready, Version)

**Modul:** api / system · **Tersedia sejak:** v0.1.3 · **Status:** Available

## Tujuan
Memantau apakah layanan Sentra DCIM hidup, siap melayani, dan versi berapa yang sedang berjalan.
Dipakai oleh healthcheck container, reverse proxy, monitoring eksternal, dan tim NOC saat troubleshooting.

## Role yang dibutuhkan
| Aksi | Role |
|---|---|
| Memanggil `/health`, `/ready`, `/version` | Publik (tanpa login). Tidak membuka data sensitif; detail error hanya di log server |

## Langkah-langkah
1. **Cek hidup (liveness):** `curl -s http://127.0.0.1:8080/api/v1/health` → `{"status":"ok"}`.
2. **Cek siap (readiness):** `curl -s http://127.0.0.1:8080/api/v1/ready | jq` → `status: ready` dan setiap
   `checks.<nama>.status: ok`. Bila `503`, lihat bagian pemecahan masalah.
3. **Cek versi:** `curl -s http://127.0.0.1:8080/api/v1/version | jq` → `version` sesuai tag Git yang di-deploy.
   `server_time_utc` = waktu server (UTC); `server_time_display` = waktu tampilan (WITA, `+08:00`).

## Endpoint API terkait
| Metode | Path | Keterangan |
|---|---|---|
| GET | `/api/v1/health` | Liveness, tanpa cek dependensi. 200 bila proses hidup |
| GET | `/api/v1/ready` | Readiness: `database`, `redis`, `migrations`. 200 siap / 503 tidak siap atau sedang shutdown |
| GET | `/api/v1/version` | Versi (dari tag Git via ldflags), commit, tanggal build, versi Go, waktu server |

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
