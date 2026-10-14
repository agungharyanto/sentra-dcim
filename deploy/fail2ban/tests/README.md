# Sampel uji filter fail2ban (sintetis)

`caddy-access.sample.log` berisi baris log akses Caddy **sintetis** (alamat dokumentasi RFC 5737, host contoh).
Diuji oleh `make fail2ban-test` (`scripts/fail2ban-regex-test.sh`) dengan `fail2ban-regex`, juga di CI. Filter:
`deploy/fail2ban/filter.d/sentra-app-login.conf` (v0.3.3, D-066: login aplikasi & kode MFA gagal).

| Baris | Kasus | Harapan |
|---|---|---|
| 1 | `POST /auth/login` 401 (kata sandi salah) | cocok |
| 2 | `POST /auth/mfa/verify` 401, urutan key JSON terbalik | cocok |
| 3 | `POST /auth/mfa/enrol/confirm` 401, spasi setelah pemisah | cocok |
| 4 | `POST /auth/mfa/recovery-codes` 401 | cocok |
| 5 | `POST /auth/login?next=…` 401 (query string) | cocok |
| 6 | login berhasil (200) | tidak cocok |
| 7 | `GET /auth/me` 401 (belum masuk — normal) | tidak cocok |
| 8 | `POST /auth/logout` 401 | tidak cocok |
| 9 | 401 login untuk host lain | tidak cocok |
| 10 | 429 rate limit | tidak cocok |
| 11 | 403 CSRF | tidak cocok |
| 12 | status/metode/uri/IP palsu di dalam nilai header (di-escape JSON) | tidak cocok |

Harapan total: **5 cocok** (`203.0.113.10–14`), 0 diabaikan, 7 tidak cocok.
