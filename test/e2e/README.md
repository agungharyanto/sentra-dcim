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
