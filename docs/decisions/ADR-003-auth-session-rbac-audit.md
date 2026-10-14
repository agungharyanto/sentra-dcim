# ADR-003 — Autentikasi, Sesi, RBAC & Audit Log

- **Status:** Accepted (2026-10-14, keputusan pemilik di awal sprint v0.3)
- **Keputusan terkait:** D-046 – D-067; menggantikan basic_auth sementara D-043/D-045 mulai v0.3.4 (ISS-026)

## Konteks
Sentra DCIM butuh akun pengguna, peran, dan jejak audit sebelum modul operasional (v0.4+) dibuka. Pengguna adalah
pegawai/teknisi Diskominfo dan tenant; server tunggal (Go monolith + PostgreSQL + Redis) di belakang Caddy.

## Keputusan

| Area | Keputusan |
|---|---|
| Sesi | **Server-side, token opaque** (bukan JWT). Cookie `__Host-sentra_session` (`Secure`, `HttpOnly`, `SameSite=Strict`, `Path=/`); hanya **SHA-256 token** yang disimpan di DB. Sesi dirotasi saat login. Bisa dicabut seketika (logout, ganti password, admin) |
| Timeout | **Idle 30 menit**, **absolut 12 jam**; sesi kedaluwarsa dibersihkan berkala |
| Identitas | **Akun lokal**, login dengan **username** (boleh NIP; `citext`, tidak peka huruf besar/kecil). SSO/LDAP tidak di v0.3 |
| Password | **argon2id** (parameter di konfigurasi); panjang **12–128**, **tanpa aturan komposisi** (NIST SP 800-63B); verifikasi terhadap hash dummy untuk user tidak dikenal (waktu konstan); pesan error login selalu generik |
| Brute force | **Lockout 5 gagal → 15 menit** per akun + rate limit login per IP dan per username (Redis) |
| CSRF | Metode tidak aman: `Origin`/`Referer` harus = domain publik **dan** header `X-Sentra-CSRF` = cookie `sentra_csrf` (double-submit). `SameSite=Strict` sebagai lapisan tambahan |
| Otorisasi | Middleware **default-deny** untuk `/api/v1/*`; allowlist publik: `health`, `version` (versi singkat), `auth/login`. Test mengenumerasi semua route |
| Admin pertama | Lewat **CLI** (`sentra user create-admin`, password dari stdin, wajib ganti saat login pertama) — tidak ada pendaftaran publik |
| MFA | **TOTP wajib untuk semua role** di v0.3.3 (D-062..D-064; semula "wajib Admin" di v0.3.4) |
| RBAC | **Permission didefinisikan di kode**, **role di DB** (v0.3.2). Role sistem: Admin, NOC, Security, Teknisi, Viewer, Auditor. Izin per route (`Auth.Require`), role dibaca ulang tiap request; rincian D-058 |
| Audit | Tabel `audit_events` **append-only** (UPDATE/DELETE ditolak trigger) sejak v0.3.1; **hash chain** + viewer + ekspor + **retensi default 365 hari** di v0.3.3. Tidak pernah mencatat password, token, atau hash |
| Principal | Konteks request membawa principal `{type, id, username}`; `type=user` sekarang, **token mesin** (API key perangkat/integrasi) **ditunda ke v0.6** |
| basic_auth | Tetap aktif sampai MFA terbukti di produksi, lalu dicabut di **v0.3.3** (ISS-026, D-066, D-067) |

## Alasan
- Sesi opaque bisa dicabut seketika dan tidak membocorkan klaim; JWT tanpa server-side state tidak memenuhi kebutuhan
  revoke & idle timeout, dan dengan state ia tidak lebih sederhana.
- argon2id = rekomendasi OWASP saat ini; tahan GPU. Panjang > komposisi (NIST).
- `__Host-` prefix memaksa `Secure`, `Path=/`, tanpa `Domain` → cookie tidak bisa di-set subdomain lain.
- Default-deny + test enumerasi mencegah endpoint baru lupa dilindungi.

## Konsekuensi
- Setiap request terautentikasi membaca sesi dari DB (indeks pada `token_hash`); `last_seen_at` diperbarui dengan
  throttling.
- Role DB aplikasi saat ini sama dengan owner migrasi — trigger append-only bisa di-drop oleh role itu; pemisahan
  role DB dijadwalkan v0.3.3 (ISS-039).
- Browser yang membuka situs di balik basic_auth mengirim header `Authorization: Basic` sekaligus cookie sesi; app
  mengabaikan header `Authorization` (bukan skema auth app) sampai basic_auth dicabut.
