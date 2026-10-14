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

Sejak v0.3.1 juga dibutuhkan **akun aplikasi** (login sesi) yang aktif dan sudah mengganti kata sandi pertamanya:
`SENTRA_APP_USER` + `SENTRA_APP_PASS`, atau file `~/.config/sentra/app-auth` (mode 600, satu baris
`user:password`; lokasi lain via `SENTRA_APP_AUTH_FILE`), atau prompt. Playwright masuk **sekali** lewat API
(`global-setup.ts`, storageState) — satu login salah menghentikan run sebelum spec berjalan (lockout akun 5×).
Spec `auth.spec.ts` menambah satu login nyata lewat form lalu Keluar (sesi itu saja yang dicabut).
Layar error login dan wajib ganti kata sandi memakai respons tiruan (`page.route`), jadi akun tidak terkunci dan
kata sandinya tidak pernah diubah oleh test.

`test/api/ui.sh` (`make test-ui`) memakai sumber kredensial yang sama; tanpa akun aplikasi cek sesi dilewati
(`SKIP`, run tidak sah sebagai regresi lengkap).

Sejak v0.2.2 ada `design-system.spec.ts`: halaman `/design-system` dalam tema gelap & terang, Dialog/Sheet/Tooltip/
Select/Dropdown/Toast, ganti ID↔EN, console & CSP bersih, dan **axe-core** (WCAG 2.1 A/AA) tanpa pelanggaran
*serious/critical*. Browser dijalankan dengan `prefers-reduced-motion: reduce` agar warna terukur setelah stabil.
Screenshot disimpan di `test/e2e/artifacts/` (lokasi lain: `E2E_ARTIFACTS`), di-gitignore, dan diunggah CI sebagai
artefak `ui-screenshots`.

## Angka acuan (v0.3.2)

Jumlah cek yang diharapkan saat semua lulus. Bila berbeda, cari tahu sebabnya sebelum menerima hasil.

| Perintah | Skrip | Acuan | Catatan |
|---|---|---|---|
| `make test-api` | `test/api/smoke.sh` | 29 (30 dengan `EXPECT_VERSION`) | listener internal, tanpa akun; v0.3.2: detail `/version` (5 cek) diganti 1 cek `{version}` saja, + 2 default-deny (`users`, `roles`) |
| `test/api/public.sh` | edge publik | 36 | termasuk 401 basic_auth & cek access log (v0.2.5) |
| `make test-ui` | `test/api/ui.sh` | **59** | v0.3.2: + `/api/v1/users` 401 tanpa basic_auth dan tanpa sesi, 5 cek RBAC sesuai izin akun (users, roles, PATCH lewat proxy, DELETE ditolak, detail `/version`). v0.3.1: + `/api/v1/auth/me` tanpa basic_auth, 3 path 401 tanpa sesi, 2 CSRF login, cek sesi (cookie, me, bootstrap, help, CSRF logout, logout). Probe basic_auth (`/`) dan login aplikasi di awal **bukan** cek; 401 → berhenti (exit 3) |
| `make test-e2e` | Playwright | **30** + 1 skip (CI: **31**) | smoke 2 · design system 2 · shell 9 · help 11 · auth 4 · users 3 (bagian yang membuat/mengubah akun hanya jalan dengan `E2E_MUTATIONS=1` di stack CI — tidak pernah terhadap produksi) |
| `make fail2ban-test` | `fail2ban-regex` | 3 cocok / 6 tidak | sampel sintetis |
| `make fail2ban-ban-test` | ban nyata | 9 · `RESTART=fail2ban` 14 · `RESTART=docker` 15 | sudo; jumlah baris "admin IP" bergantung jumlah IP admin |

Percobaan kredensial salah per run (fail2ban `sentra-web`, maxretry 10/10 menit): `ui.sh` 1, `run.sh` 0 (1 bila
probe gagal), `public.sh` 1. Login **aplikasi** per run: `ui.sh` 1, `make test-e2e` 2 (CI + 1 akun e2e sekali pakai) (rate limit per pengguna 10/10
menit); 401 dari aplikasi tidak dihitung fail2ban. Lihat `docs/ops/runbooks/fail2ban.md`.
