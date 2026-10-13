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

Harapan total: **3 cocok** (`203.0.113.10–12`), 6 diabaikan. Baris 2–3 memakai urutan key/spasi berbeda: filter tidak
boleh bergantung pada urutan field JSON.
