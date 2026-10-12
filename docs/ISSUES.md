# Daftar Issue

Format: ID, judul, prioritas (P1 kritis – P4 rendah), versi ditemukan, versi diperbaiki, status.

## Open

| ID | Judul | Prioritas | Ditemukan | Diperbaiki | Catatan |
|---|---|---|---|---|---|
| ISS-002 | Docker userland-proxy menyembunyikan IP klien asli (semua klien terlihat sebagai IP gateway) → rate limit & audit log per IP tidak akurat saat 80/443 dibuka | P2 | v0.1.3 | — | Wajib sebelum `OPEN_HTTP=1`: set `"userland-proxy": false` di `/etc/docker/daemon.json` atau Caddy `network_mode: host`. Target v0.2/v0.12 |
| ISS-001 | Push ke GitHub ditolak: deploy key server (`~/.ssh/github_sentra.pub`) belum terdaftar di repo `agungharyanto/sentra-dcim` | P1 | v0.1.1 | — | Commit & tag dibuat lokal dengan `NO_PUSH=1`; push menyusul dengan `git push origin main --follow-tags` |

## Solved

| ID | Judul | Prioritas | Ditemukan | Diperbaiki | Catatan |
|---|---|---|---|---|---|
| — | — | — | — | — | — |
