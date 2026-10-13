# Uji e2e (Playwright)

Smoke test browser (Chromium) untuk UI web: login basic_auth sementara (ISS-026), ganti bahasa ID ↔ EN, deep link,
tanpa error console dan tanpa pelanggaran CSP. Dijalankan di container `mcr.microsoft.com/playwright` (arm64/amd64),
host tidak perlu Node maupun browser. Dipakai ulang di full regression penutup sprint (`.9`).

```bash
make test-e2e            # = test/e2e/run.sh, BASE_URL default https://$DOMAIN dari .env
BASE_URL=https://localhost make test-e2e
```

Kredensial basic_auth **tidak pernah** disimpan di repo. Urutan sumber:

1. variabel lingkungan `SENTRA_WEB_USER` + `SENTRA_WEB_PASS`;
2. file `~/.config/sentra/web-auth` (mode 600, satu baris `user:password`; lokasi lain via `SENTRA_WEB_AUTH_FILE`);
3. prompt interaktif (hanya bila ada TTY).

`test/api/ui.sh` (`make test-ui`) memakai sumber kredensial yang sama.

Sejak v0.2.2 ada `design-system.spec.ts`: halaman `/design-system` dalam tema gelap & terang, Dialog/Sheet/Tooltip/
Select/Dropdown/Toast, ganti ID↔EN, console & CSP bersih, dan **axe-core** (WCAG 2.1 A/AA) tanpa pelanggaran
*serious/critical*. Browser dijalankan dengan `prefers-reduced-motion: reduce` agar warna terukur setelah stabil.
Screenshot disimpan di `test/e2e/artifacts/` (lokasi lain: `E2E_ARTIFACTS`), di-gitignore, dan diunggah CI sebagai
artefak `ui-screenshots`.

## Angka acuan (v0.2.5)

Jumlah cek yang diharapkan saat semua lulus. Bila berbeda, cari tahu sebabnya sebelum menerima hasil.

| Perintah | Skrip | Acuan | Catatan |
|---|---|---|---|
| `make test-api` | `test/api/smoke.sh` | 28 | listener internal |
| `test/api/public.sh` | edge publik | 36 | termasuk 401 basic_auth & cek access log (v0.2.5) |
| `make test-ui` | `test/api/ui.sh` | **41** | 33 baris `check` + loop (2 endpoint publik, 6 path tanpa login, 3 path API 403). Probe kredensial di awal **bukan** cek; bila 401 skrip berhenti (exit 3) |
| `make test-e2e` | Playwright | **24** | smoke 2 · design system 2 · shell 9 · help 11 |
| `make fail2ban-test` | `fail2ban-regex` | 3 cocok / 6 tidak | sampel sintetis |
| `make fail2ban-ban-test` | ban nyata | 9 · `RESTART=fail2ban` 14 · `RESTART=docker` 15 | sudo; jumlah baris "admin IP" bergantung jumlah IP admin |

Percobaan kredensial salah per run (fail2ban `sentra-web`, maxretry 10/10 menit): `ui.sh` 1, `run.sh` 0 (1 bila
probe gagal), `public.sh` 1. Lihat `docs/ops/runbooks/fail2ban.md`.
