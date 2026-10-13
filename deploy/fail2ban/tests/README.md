# Sampel uji filter fail2ban (sintetis)

`caddy-access.sample.log` berisi baris log akses Caddy **sintetis** (alamat dokumentasi RFC 5737, host contoh).
Diuji oleh `make fail2ban-test` (`scripts/fail2ban-regex-test.sh`) dengan `fail2ban-regex`, juga di CI.

| Baris | Kasus | Harapan |
|---|---|---|
| 1 | 401 + Authorization (Caddy key order) | cocok |
| 2 | 401 + Authorization (reversed key order) | cocok |
| 3 | 401 + Authorization (spaces after separators) | cocok |
| 4 | 401 without Authorization (normal browser challenge) | tidak cocok |
| 5 | 200 with Authorization (correct password) | tidak cocok |
| 6 | 403 with Authorization (other API path) | tidak cocok |
| 7 | 401 + Authorization for another host | tidak cocok |
| 8 | forged keys inside a header value (JSON-escaped) | tidak cocok |
| 9 | 4010-like status prefix | tidak cocok |
| 10 | 401 dari app (login app gagal; `X-Request-Id` di `resp_headers`) | diabaikan (ignoreregex) |
| 11 | 401 basic_auth dengan header request `X-Request-Id` palsu | cocok |

Harapan total: **4 cocok** (`203.0.113.10–13`), 1 diabaikan (ignoreregex), 6 tidak cocok. Baris 2–3 memakai urutan key/spasi berbeda: filter tidak
boleh bergantung pada urutan field JSON.
