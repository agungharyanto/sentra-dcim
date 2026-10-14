# Uji e2e (Playwright)

Smoke test browser (Chromium) untuk UI web: login aplikasi + verifikasi dua langkah (MFA), ganti bahasa ID ↔ EN,
deep link, tanpa error console dan tanpa pelanggaran CSP (nonce). Dijalankan di container
`mcr.microsoft.com/playwright` (arm64/amd64), host tidak perlu Node maupun browser. Sejak v0.3.3 tidak ada lagi
basic_auth (ISS-026).

```bash
make test-e2e            # = test/e2e/run.sh, BASE_URL default https://$DOMAIN dari .env
BASE_URL=https://localhost make test-e2e
```

Dibutuhkan **akun aplikasi** yang aktif, sudah mengganti kata sandi pertama, dan sudah memasang MFA:
`SENTRA_APP_USER` + `SENTRA_APP_PASS`, atau file `~/.config/sentra/app-auth` (mode 600, satu baris `user:password`;
lokasi lain via `SENTRA_APP_AUTH_FILE`), atau prompt. Kredensial **tidak pernah** disimpan di repo. Lalu `run.sh`
meminta **satu kode 6 digit** dari aplikasi authenticator (atau `SENTRA_APP_TOTP`); secret MFA tidak pernah
disimpan di server. Playwright masuk **sekali** lewat API (`global-setup.ts`, storageState) — satu login atau kode
salah menghentikan run sebelum spec berjalan (lockout akun 5×).

Hanya di CI (akun sekali pakai, `SENTRA_APP_TOTP_SECRET` + `E2E_MUTATIONS=1`): login nyata lewat form dengan kode
baru, serta spec yang membuat/mengubah akun (Pengguna & Role, pemasangan MFA di browser). Terhadap produksi spec itu
di-skip. Layar error login, wajib ganti kata sandi, dan halaman MFA memakai respons tiruan (`page.route`).

`test/api/ui.sh` (`make test-ui`) memakai sumber akun yang sama dan juga meminta satu kode MFA; tanpa akun aplikasi
cek sesi dilewati (`SKIP`, run tidak sah sebagai regresi lengkap). `test/api/public.sh` tidak butuh akun: ia
memeriksa bahwa semua path tanpa sesi ditolak kecuali allowlist (kerangka SPA + asetnya, health, version, login).

Sejak v0.2.2 ada `design-system.spec.ts`: halaman `/design-system` dalam tema gelap & terang, Dialog/Sheet/Tooltip/
Select/Dropdown/Toast, ganti ID↔EN, console & CSP bersih, dan **axe-core** (WCAG 2.1 A/AA) tanpa pelanggaran
*serious/critical*. Browser dijalankan dengan `prefers-reduced-motion: reduce` agar warna terukur setelah stabil.
Screenshot disimpan di `test/e2e/artifacts/` (lokasi lain: `E2E_ARTIFACTS`), di-gitignore, dan diunggah CI sebagai
artefak `ui-screenshots`.

## Angka acuan (v0.3.3)

Jumlah cek yang diharapkan saat semua lulus. Bila berbeda, cari tahu sebabnya sebelum menerima hasil.

| Perintah | Skrip | Acuan | Catatan |
|---|---|---|---|
| `make test-api` | `test/api/smoke.sh` | 30 (31 dengan `EXPECT_VERSION`) | listener internal, tanpa akun; v0.3.3: + `/api/v1/auth/mfa` default-deny |
| `test/api/public.sh` | edge publik tanpa sesi | **72** produksi (CI `localhost`: 71) | v0.3.3: inventaris tanpa sesi (kerangka SPA untuk semua route & file tak dikenal, aset login tanpa source map, 401 untuk Pusat Bantuan/OpenAPI/data/langkah MFA, login terjangkau via 403 CSRF), access log untuk jail; tanpa basic_auth. Jumlah aset bergantung build |
| `make test-ui` | `test/api/ui.sh` | **54** | v0.3.3: tanpa basic_auth (−11 cek), + MFA (403 `mfa_required` sebelum kode, verifikasi kode), CSP nonce (no-store, tanpa `unsafe-inline`, nonce = meta, beda per request). Meminta satu kode MFA |
| `make test-e2e` | Playwright | **36** + 5 skip produksi (CI: **41**) | smoke 2 · design system 2 · shell 9 · help 11 · auth 4 · users 3 · mfa 5 · audit 3 · settings 2. Hanya CI: login nyata lewat form (butuh kode baru), mutasi Pengguna & Role, pemasangan MFA di browser, verifikasi + ekspor CSV audit, perubahan pengaturan & preferensi |
| `make fail2ban-test` | `fail2ban-regex` | 5 cocok / 7 tidak | sampel sintetis login aplikasi & MFA (v0.3.3) |
| `make fail2ban-ban-test` | ban nyata | 10 (jumlah baris "admin IP" bergantung jumlah IP admin) | sudo; login aplikasi gagal dengan pengguna acak dari network namespace |

Login atau kode MFA yang salah ketik oleh pemilik dihitung jail `sentra-web` (10 per 10 menit per IP) dan lockout akun (5×); skrip berhenti pada penolakan pertama. Lihat `docs/ops/runbooks/fail2ban.md`.
