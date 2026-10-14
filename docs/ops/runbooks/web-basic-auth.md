# basic_auth Web UI — DICABUT di v0.3.3

**Modul:** ops · **Tersedia:** v0.2.1–v0.3.2 · **Status:** Dihapus (ISS-026, D-066)

## Tujuan
Catatan sejarah. Sejak v0.3.3 UI web dan API tidak lagi dilindungi basic_auth Caddy: semua data mensyaratkan sesi
aplikasi yang lolos verifikasi dua langkah (MFA, D-062), dan login aplikasi yang gagal dihitung jail fail2ban
`sentra-web` per IP ([fail2ban.md](fail2ban.md)).

## Langkah-langkah (sekali, setelah upgrade ke v0.3.3)
1. `WEB_BASIC_AUTH_USER` / `WEB_BASIC_AUTH_HASH` di `.env` tidak dipakai lagi; hapus barisnya (sebagai pemilik
   `.env`, mis. `sudo -u sentra -H nano /opt/sentra-dcim/.env`). Membiarkannya tidak berbahaya.
2. Hapus file kredensial test lama `~/.config/sentra/web-auth` di server bila ada.
3. `make fail2ban-web` (filter baru `sentra-app-login`, `ignoreip` dari `ADMIN_IPS`).

## Yang publik tanpa sesi sejak v0.3.3
Halaman login (kerangka SPA `index.html` dan aset JS/CSS/font/logo-nya), `/api/v1/health`, `/api/v1/version`
(versi singkat), dan `POST /api/v1/auth/login`. Konten & gambar Pusat Bantuan, OpenAPI, dan semua data hanya lewat
API ber-sesi. Diuji `test/api/public.sh` (inventaris path tanpa sesi).

## Pertanyaan umum / pemecahan masalah
- **Dialog login browser (basic_auth) masih muncul**: cache browser lama; tutup semua tab situs lalu buka lagi.
