# Changelog

Format mengikuti [Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/) dan versi mengikuti konvensi proyek
`v0.<sprint>.<sub>` (lihat `CLAUDE.md`).

## [Unreleased] — v0.3.9 Penutup Sprint v0.3

### Ditambahkan
- **Matriks izin** (`TestPermissionMatrix`): 6 role sistem × setiap endpoint ber-`x-sentra-permission` di OpenAPI —
  role dengan izin lolos pemeriksaan izin, role tanpa izin mendapat tepat `403 forbidden`.
- `scripts/kernel-retire.sh`: memensiunkan kernel lama dengan pengaman (bukan kernel aktif, simulasi dulu, menolak bila
  purge menyentuh paket lain), lalu `update-grub` + verifikasi.

### Diubah
- `scripts/full-regression.sh` mengikuti v0.3: migrasi salinan lewat container `ops` (role app tanpa DDL), image compose
  mengikuti versi, `make audit-verify` pada DB live, verifikasi hash chain + anchor pada DB hasil restore, uji port dari
  alamat publik (hanya 22/80/443 terbuka), pemeriksaan kernel pasca-ISS-036.
- `make boot-menu` / `scripts/host-boot-setup.sh` tidak lagi menahan kernel 6.17.
- Dokumen: ROADMAP (v0.3 selesai, v0.3.8 dikosongkan, v0.4 berikutnya), DISASTER_RECOVERY (ringkasan prosedur darurat
  auth, rollback kernel pasca-6.17), DEPENDENCIES (diaudit), runbook rollback kernel.

## [v0.3.7] - 2026-10-15 — Kebijakan Sesi & Kata Sandi

### Ditambahkan
- **Batas sesi yang bisa diatur** (D-069, D-079): Pengaturan › **Sesi & Kata Sandi** — idle (default 30 menit,
  5–240) dan absolut (default 12 jam, 1–24), idle ≤ absolut. Idle hanya diperpanjang oleh aktivitas nyata pengguna
  (klik/ketik/gulir/sentuh → `POST /api/v1/auth/activity`, ≤ 1×/menit) dan tindakan yang menyimpan data; polling dan
  pembaruan otomatis halaman tidak lagi memperpanjang sesi. **Peringatan 2 menit** sebelum keluar otomatis dengan
  hitung mundur dan **Tetap masuk** (saat batas absolut: hanya **Keluar sekarang**).
- **Kedaluwarsa kata sandi** (D-070): default 90 hari (0 = tidak kedaluwarsa), kecuali akun yang role-nya hanya
  Viewer; banner peringatan 14 hari sebelumnya; setelah lewat wajib ganti setelah masuk. **Riwayat 5 kata sandi**
  (termasuk yang sekarang) tidak boleh dipakai ulang (`password_reused`), juga setelah reset oleh Admin.
- **Mode monitoring (kiosk)** per akun Viewer (D-068): `PUT /api/v1/users/{id}/kiosk` + kartu di detail pengguna.
  Sesi tanpa idle/absolut, hanya sah dari IP Allowed IP (dari IP lain dicabut, `auth.session.kiosk_revoked`); hanya
  bisa diaktifkan bila Allowed IP Pantau/Tegakkan dengan ≥ 1 IP (`kiosk_requires_allowlist`); MFA tetap diminta.
- **Halaman pemeliharaan** (D-080): saat app tidak merespons, panel menampilkan halaman statis "Sedang pemeliharaan /
  Under maintenance" (503 + `Retry-After`), API menjawab JSON `maintenance`; tanpa info versi/stack. Diuji di CI.
- `/api/v1/auth/me` membawa `server_time`, `password_expires_at`, `password_expired`, `password_expires_soon`,
  `kiosk`. Migrasi 000010. OpenAPI, Pusat Bantuan ID/EN (Kebijakan sesi & kata sandi, mode monitoring, login, ganti
  kata sandi).

## [v0.3.6] - 2026-10-15 — Settings – Allowed IP

### Ditambahkan
- **Allowed IP untuk panel web** (D-078, migrasi 000009): daftar IP/jaringan (IPv4 + IPv6, keterangan, tanpa
  duplikat, bit host harus nol, paling luas /8 atau /16) dan mode `off` (default) / `monitor` (akses dari luar daftar
  dicatat `security.allowlist_outside`, tidak diblok) / `enforce` (diblok `403 ip_not_allowed`,
  `security.allowlist_blocked`; dicatat teragregasi per IP). Ditegakkan oleh **Caddy** (`forward_auth` ke endpoint
  internal app untuk setiap request) dan oleh app di API publik — bukan ufw. IP = koneksi langsung ke Caddy;
  `X-Forwarded-For`/`X-Real-IP` dari klien diabaikan (diuji). `/api/v1/health` dan `/.well-known/acme-challenge/*`
  selalu terbuka (pemantauan, perpanjangan TLS).
- **Pengaman lockout**: enforce ditolak bila IP penyimpan tidak tercakup; menghapus jaringan yang mencakup IP sendiri
  ditolak saat enforce; peralihan ke enforce dari UI kembali otomatis ke monitor setelah 2 menit kecuali Admin menekan
  **Pertahankan** (`allowlist.confirm` / `allowlist.rollback`). Semua perubahan diaudit.
- **Pintu darurat CLI**: `sentra allowlist list|add|remove|mode off|monitor` (di container app, lewat SSH atau serial
  console; berlaku ≤ 5 detik). Prosedur di `docs/DISASTER_RECOVERY.md`.
- Halaman **Pengaturan › Allowed IP** (izin `settings.manage`) dengan "IP Anda saat ini", mode, daftar, dan jalur yang
  selalu terbuka. API `/api/v1/settings/allowed-ips` (+ `/mode`, `/confirm`), OpenAPI, Pusat Bantuan ID/EN.
- Uji edge Caddy khusus CI (`test/api/allowlist.sh`: mode, di dalam/luar daftar, spoofing header, health/ACME); menolak
  jalan terhadap deployment nyata.

### Diubah
- Rencana v0.3.6: **panel web saja**; allowlist SSH dipindah ke v0.12 bersama WireGuard, tanpa agent ufw (D-037
  digantikan D-078). Rilis dengan mode `off`.
- Preferensi: teks "Ikuti sistem — …" tanpa kurung bersarang.

### Diperbaiki
- **ISS-044**: `test/api/ui.sh` tidak lagi men-skip pemeriksaan sesi diam-diam: tanpa akun aplikasi skrip gagal
  (exit 2) dengan alasan per sumber (variabel env, file auth, terminal); prompt tidak menawarkan "Enter = skip".

## [v0.3.5] - 2026-10-14 — Settings – Regional

### Ditambahkan
- **Fondasi pengaturan generik** (D-076, migrasi 000008): skema per key di kode (tipe, default, pilihan, batas
  min/maks, validator, boleh-override-pengguna), nilai di tabel `settings` hanya bila berbeda dari default,
  override per pengguna di `user_preferences`. API `GET/PUT /api/v1/settings/{group}` (izin baru `settings.manage`,
  Admin): validasi semua-atau-tidak-sama-sekali, `null` = kembali ke default, setiap perubahan tercatat
  `settings.update` (nilai lama → baru). Siap dipakai Allowed IP (v0.3.6) dan kebijakan sesi/kata sandi (v0.3.7).
- **Regional sistem**: bahasa (id/en), zona waktu (IANA, zona tak dikenal ditolak), format jam (24/12), format tanggal
  (`DD/MM/YYYY`, `YYYY-MM-DD`, `DD MMM YYYY`). Default awal dari `.env` (`DEFAULT_LOCALE`, `DISPLAY_TIMEZONE`), lalu
  diatur Admin di **Pengaturan › Regional** (pratinjau langsung, *Kembalikan ke default*).
- **Preferensi pengguna** (Akun › Preferensi, `GET/PUT /api/v1/me/preferences`): keempat nilai bisa "Ikuti sistem"
  atau nilai sendiri; tercatat `user.preferences.update`. `/api/v1/auth/me` mengembalikan nilai efektif (`regional`).
  Bahasa preferensi diterapkan saat masuk; tombol ID/EN di header tetap pilihan browser.
- **Satu helper format** (D-077) untuk semua tanggal/jam di UI (Audit Log, Sesi saya, Keamanan, Pengguna, jam header)
  dari nilai efektif; DB tetap UTC. Ekspor CSV audit: kolom UTC ISO-8601 + kolom lokal dalam zona efektif pengekspor.
- **Pemilih tanggal & jam** design system (kalender Senin-pertama, navigasi panah/Page Up/Down, 12/24 jam, ID/EN)
  menggantikan input tanggal bawaan browser di filter Audit Log (ISS-043).

### Diubah
- Menu **Pengaturan** kini tersedia (khusus `settings.manage`). Dokumen bootstrap UI dan detail `/api/v1/version`
  memakai zona/bahasa sistem dari pengaturan (`.env` tetap jadi nilai awal).

## [v0.3.4] - 2026-10-14 — Audit Log

### Ditambahkan
- **Tampilkan/sembunyikan kata sandi** (D-071): tombol ikon mata di kolom kata sandi login dan form ganti kata
  sandi (wajib & akun) — default tersembunyi, `type="button"`, `aria-label` ID/EN + `aria-pressed`, tersembunyi lagi
  setelah submit, `autocomplete` tidak berubah. Kata sandi sementara (buat pengguna/reset) kini **tersembunyi secara
  default** (nilainya tidak ada di DOM) dengan tombol mata; tombol salin menyalin tanpa menampilkan, dan pesan gagal
  salin tidak memuat nilainya.

- **Role database terpisah** (ISS-039, D-072): app berjalan sebagai `DB_APP_USER` (anggota `sentra_app`, DML saja;
  `audit_events` hanya INSERT+SELECT lewat GRANT); kredensial owner hanya di container one-shot `ops`
  (`make migrate` = migrasi + `sentra db sync-app-role`). Variabel baru `DB_APP_USER`, `DB_APP_PASSWORD` (diisi
  pemilik). `make up` kini menyalakan database, menjalankan migrasi & role, baru app.
- **Hash chain audit log** (D-073, migrasi 000007): setiap event berantai `sha256(prev_hash ‖ kanonik(baris))`,
  diisi trigger di bawah advisory lock (insert paralel tidak bercabang; baris lama di-backfill). Verifikasi
  `sentra audit verify` + job harian di app (gagal → log error). Setiap backup menyimpan kepala chain
  (`audit-anchor.json`); `make audit-verify` mencocokkan chain dengan anchor semua backup (`UNTIL=` setelah restore).
- **Retensi audit log**: `AUDIT_RETENTION_DAYS` (default 365, minimum 90). `make audit-prune` (juga dijalankan
  timer backup harian setelah backup sukses) mengarsipkan event di luar retensi ke `$BACKUP_DIR/audit-archive/`
  (gzip JSON lines + `.sha256`), lalu — sebagai role owner di container `ops` — menghapusnya hanya bila checksum
  arsip cocok dengan ekspor ulang dalam transaksi yang sama, menyimpan checkpoint (hash baris terakhir) di
  `audit_checkpoints` dan mencatat event `audit.prune`. Chain tetap terverifikasi; arsip bisa diverifikasi offline
  dengan `sentra audit verify-archive`.
- **Cakupan event audit**: semua route tulis terdaftar di `api.AuditedRoutes` dan diuji (route tulis baru tanpa
  aksi audit menggagalkan tes). Event baru `auth.mfa.enrol_start` dan `security.csrf_denied`. Penolakan berulang
  (403 `authz.denied`, CSRF) oleh aktor/IP yang sama pada route yang sama dalam 10 menit dicatat sekali, lalu
  diringkas menjadi satu event `….repeated` (jumlah, waktu pertama/terakhir) — log tidak bisa dibanjiri.
  Sanitasi metadata kini rekursif dan juga membuang kunci OTP/recovery code/kunci API; diuji bahwa kata sandi,
  token sesi/CSRF, secret TOTP, kode MFA, dan recovery code tidak pernah masuk audit log.
- **Menu Audit Log** (`/audit`, D-075, izin `audit.read` untuk Admin & Auditor): status integritas hash chain +
  *Verifikasi sekarang*, filter waktu (WIB), pelaku, aksi (atau awalan), hasil, target; tabel/kartu dengan waktu WIB,
  detail event (metadata, hash), *Muat lebih banyak* (keyset). **Ekspor CSV** (`audit.export`) dengan filter yang
  sama, injeksi formula dinetralkan, ekspor tercatat. API `/api/v1/audit/{events,events/export.csv,actions,integrity,
  integrity/verify}` + OpenAPI. Pusat Bantuan *Audit Log* & *Verifikasi integritas* (ID/EN).
- **Pusat Bantuan disaring per izin** (ISS-032, D-074): panduan Admin dan referensi API hanya untuk Admin; panduan
  menu berizin hanya untuk pemegang izinnya; selebihnya 404. Izin baru `help.admin` (Admin).

### Diperbaiki
- Event login gagal karena rate limit tidak lagi menyimpan "username" yang salah format (bisa berupa kata sandi
  yang salah ketik di kolom nama pengguna).
- **ISS-042**: pengaman "Admin aktif terakhir" memakai advisory lock (sebelumnya `FOR UPDATE` yang butuh hak
  UPDATE di `roles`).
- **ISS-041**: `make release` kini bisa merilis versi yang mengganti file jail fail2ban — cek pra-merge memakai
  versi branch yang dirilis.

## [v0.3.3] - 2026-10-14 — MFA & Cabut basic_auth

Keputusan: D-061..D-067 (urutan v0.3.3/v0.3.4 ditukar; MFA wajib untuk semua role).

### Ditambahkan
- **MFA TOTP wajib untuk semua akun** (RFC 6238: SHA1, 6 digit, 30 detik, ±1 langkah; Google Authenticator,
  Microsoft Authenticator, Authy). Setelah kata sandi benar sesi berstatus *mfa_pending* (`403 mfa_required` untuk
  semua selain `auth/me`, `auth/logout`, `auth/mfa/*`). Enrolment: kode QR (SVG dari matriks server) + kunci manual,
  diverifikasi 1 kode; 10 recovery code sekali pakai tampil sekali. Kode yang sudah dipakai ditolak (anti-replay
  atomik per akun). Urutan: MFA lalu ganti kata sandi wajib.
- Secret TOTP terenkripsi AES-256-GCM (key `MFA_ENC_KEY`, terikat ke id akun); recovery code disimpan sebagai
  HMAC-SHA256; tidak ada yang di-log/diaudit. App menolak start tanpa key valid; `make preflight` memeriksanya.
- Rate limit percobaan kode (per IP & per akun, batas sama dengan login) dan lockout bersama kata sandi (5 → 15 menit,
  semua sesi dicabut). Langkah kata sandi tidak lagi me-reset penghitung gagal; reset hanya setelah MFA lolos.
- **Reset MFA** oleh Admin (`POST /api/v1/users/{id}/mfa-reset`, bukan akun sendiri) dan CLI darurat
  `sentra user reset-mfa --username X`; keduanya mencabut semua sesi dan diaudit.
- UI: halaman verifikasi dua langkah (enrolment/verifikasi, recovery code), **Keamanan akun** (status, buat ulang
  recovery code), penanda *MFA belum aktif* dan tombol *Reset MFA* di Pengguna & Role. Pusat Bantuan *Verifikasi dua
  langkah* bergambar (ID/EN); Pusat Bantuan kini mendukung gambar dari `docs/help/assets/` (tervalidasi).
- `/design-system` hanya untuk Admin (izin `ui.design_system`, ISS-029 dimajukan dari v0.3.4).
- CLI darurat `sentra user unlock --username X` (membuka lockout sebelum 15 menit, diaudit `user.unlock`).
- Build web gagal bila ada source map (`*.map` / `sourceMappingURL`) di `dist`.
- Migrasi `000005_mfa` (`user_mfa`, `mfa_recovery_codes`, `sessions.mfa_verified_at`; sesi lama menjadi pending).

### Dihapus
- **basic_auth Caddy** (ISS-026, D-066) setelah MFA terbukti di produksi (D-067): UI web publik hanya berupa kerangka
  SPA dan asetnya; semua data, konten & gambar Pusat Bantuan, dan OpenAPI hanya lewat API ber-sesi yang lolos MFA.
  `WEB_BASIC_AUTH_*` tidak dipakai lagi. Menutup ISS-040 (dialog basic_auth muncul lagi saat login).

### Diubah
- **Jail fail2ban `sentra-web`** kini menghitung login dan kode MFA yang gagal (401 dari app pada `POST` login/MFA)
  per IP: 10 dalam 10 menit → ban 1 jam (bertambah s/d 24 jam); `ignoreip` = localhost + `ADMIN_IPS`. Filter baru
  `sentra-app-login` menggantikan `sentra-caddy-auth` (dihapus otomatis oleh `make fail2ban-web`; `make release`
  menolak bila masih ada). Uji ban nyata memakai login gagal dari network namespace.
- **CSP UI tanpa `'unsafe-inline'`** (ISS-030, D-065): nonce per request dari Caddy (`templates` +
  `{http.request.uuid}`) di header dan `<meta name="csp-nonce">`; `index.html` `no-store` tanpa ETag. `sonner`
  diganti komponen toast sendiri tanpa `<style>`.

### Test
- Go: paket `mfa` (vektor RFC 6238, jendela ±1, replay, enkripsi terikat akun, recovery code), integrasi MFA
  (enrolment, sesi pending, replay lintas sesi, recovery code, lockout & rate limit, kata sandi tidak me-reset
  lockout, reset Admin/CLI, urutan MFA → ganti kata sandi); terbukti gagal bila `MFAGuard` dilepas.
- Web: alur enrolment/verifikasi/recovery/Keamanan akun/reset MFA Admin, toast tanpa `<style>`, nonce; e2e
  `mfa.spec.ts` (halaman mock + screenshot, Keamanan akun, enrolment nyata di browser khusus CI).
- `public.sh` (kini juga di CI): inventaris path tanpa sesi — kerangka SPA untuk semua route dan file tak dikenal
  (`/.env`, `/openapi.yaml`, …), aset yang dimuat halaman login tanpa source map, 401 untuk bootstrap/Pusat Bantuan
  (halaman, gambar, OpenAPI)/data/langkah MFA, login terjangkau (403 CSRF), tanpa `WWW-Authenticate`.
- `ui.sh`: sesi pending → 403 `mfa_required`, verifikasi kode (diminta saat dijalankan), CSP tanpa `'unsafe-inline'`,
  nonce header = meta & berbeda per request, `index.html` no-store.

## [v0.3.2] - 2026-10-14 — RBAC & Pengguna

Sprint v0.3 "Auth, RBAC & Audit Log", sub-versi 2. Keputusan: D-053..D-060.

### Ditambahkan
- **RBAC**: izin di kode (`users.read`, `users.manage`, `roles.read`, `system.version.read`), role di DB (migrasi
  `000004_rbac`: tabel `roles` dengan 6 role sistem dan `user_roles`). Admin = semua izin; Auditor = lihat pengguna
  & role; NOC/Security/Teknisi/Viewer belum punya izin modul (bertambah bersama modulnya). Semua akun v0.3.1 (hanya
  dari `create-admin`) otomatis menjadi Admin; `create-admin` memberi role Admin dalam satu statement.
- Role dibaca ulang setiap request: perubahan role/nonaktif berlaku seketika tanpa login ulang. `GET /auth/me`
  menyertakan `roles` dan `permissions`.
- Middleware izin per route (`403 forbidden`, audit `authz.denied`); route tanpa izin hanya yang ada di
  `api.SelfServiceRoutes` (akun sendiri, bootstrap, Pusat Bantuan).
- **API Pengguna & Role**: `GET/POST /api/v1/users` (cari literal, filter status/role, paging), `GET/PATCH
  /api/v1/users/{id}`, `PUT /users/{id}/roles`, `POST /users/{id}/disable|enable|password-reset`, `GET
  /api/v1/roles`. Kata sandi sementara dibuat server (`xxxx-xxxx-xxxx-xxxx`, ≈92 bit), tampil sekali
  (`Cache-Control: no-store`), wajib diganti; reset & nonaktif mencabut semua sesi pengguna; reset membuka lockout.
  Pengaman: tidak bisa menonaktifkan/mencabut Admin/me-reset akun sendiri, Admin aktif terakhir tidak bisa
  kehilangan akses (serialisasi lewat kunci; sejak v0.3.4 advisory lock, ISS-042). Akun tidak pernah dihapus. Semua perubahan diaudit
  (`user.create|update|roles.update|disable|enable|password.reset`) tanpa kata sandi.
- **UI Pengguna & Role** (`/users`, menu Sistem): daftar (tabel desktop, kartu mobile, cari, filter, paging), tab
  Role (izin + jumlah pengguna), dialog tambah pengguna dengan kata sandi sementara sekali tampil + tombol salin,
  halaman detail (profil, role, nonaktifkan/aktifkan, reset kata sandi dengan konfirmasi). Menu tanpa izin
  disembunyikan; alamatnya menampilkan *Akses ditolak*. `403 forbidden` memicu baca ulang `/auth/me`.
- Pusat Bantuan `users` dan `users/roles` (ID/EN, audience admin); viewer API menampilkan izin per endpoint.

### Diubah
- **`/api/v1/version`**: detail build hanya untuk sesi dengan `system.version.read` (Admin), di domain publik maupun
  listener internal; tanpa sesi `{version}` saja (D-059). Ops yang memakai `.version` tidak terdampak.
- Registry menu punya field `permission`; Caddy meneruskan `/api/v1/users*` dan `/api/v1/roles` (GET HEAD POST PUT
  PATCH) di belakang basic_auth; `DELETE` tetap ditolak proxy.
- **`make release` mengikuti alur worktree** (CLAUDE.md aturan 13): dijalankan di working tree server yang ada di
  `main`, mencari branch `feat/vX.Y.Z-*` berdasarkan nama, lalu menghapus worktree fitur (bila bersih) dan branch
  setelah merge. Sebelumnya mengharuskan branch fitur di-checkout di tree yang sama, sehingga rilis sejak v0.2.9
  di-merge manual.
- **Gerbang fail2ban saat rilis** (D-057): `release.sh` tanpa sudo membandingkan sha256 jail `sentra-web`
  terpasang dengan repo sebelum merge dan sesudah redeploy; beda → berhenti (exit 1) dengan pesan
  *"run make fail2ban-web"*. Cek manual `make fail2ban-web-check`. Render file jail dipindah ke
  `scripts/lib/fail2ban-web.sh` (dipakai installer & pengecek, byte yang dibandingkan identik).

### Dokumentasi
- Keputusan D-053..D-056 dicatat (`/ready` loopback + 403 publik, detail `/version` internal, pemulihan admin via
  CLI, locale halaman login) dan D-057 (gerbang fail2ban).

### Test
- Integrasi: enumerasi semua route dengan akun tanpa role → `403` kecuali self-service, dan izin yang dipaksakan
  router == `x-sentra-permission` OpenAPI (terbukti gagal bila `Require` dihapus atau dokumentasi salah); alur
  admin (buat → login sementara → role seketika → nonaktif mencabut sesi → aktif → reset), validasi & pengaman
  (akun sendiri, Admin terakhir), detail `/version` per izin, migrasi 000004 (akun lama → Admin, down/up).
- Web: 13 tes baru (menu per izin, Akses ditolak, daftar, read-only Auditor, buat + kata sandi sementara, error
  field, akun sendiri, nonaktif & reset, simpan role, tab role, viewer API). e2e `users.spec.ts` (bagian yang
  mengubah akun hanya di stack CI, `E2E_MUTATIONS=1`). `smoke.sh` tanpa akun kini memeriksa `/version` =
  `{version}`; `ui.sh` + cek RBAC sesuai izin akun; drill memeriksa role Admin hasil `create-admin`.
- `scripts/fail2ban-web-check-test.sh` (CI + `make fail2ban-web-check-test`): 14 kasus — pengecek terhadap pohon
  instalasi sementara (tidak terpasang, sinkron, filter diubah di host/repo, log path beda, drop-in hilang, DOMAIN
  kosong), installer menolak override tes, dan `release.sh` di repo git sementara (drift sebelum merge → tanpa
  merge/tag/push; drift sesudah redeploy → exit 1; sinkron → rilis normal). Terbukti gagal bila pengecekan
  sebelum merge dihapus.

## [v0.3.1] - 2026-10-14 — Auth Core

Sprint v0.3 "Auth, RBAC & Audit Log", sub-versi 1. Keputusan: ADR-003, D-046..D-052.

### Ditambahkan
- **Login aplikasi** dengan akun lokal (nama pengguna atau NIP): kata sandi argon2id (parameter di `.env`
  `AUTH_ARGON2_*`, hash di-upgrade otomatis saat login), kebijakan 12–128 karakter tanpa aturan komposisi, pesan gagal
  generik, waktu respons setara untuk pengguna tak dikenal (hash dummy).
- **Sesi server-side opaque**: token acak 256-bit, DB hanya menyimpan SHA-256; cookie `__Host-sentra_session`
  (Secure, HttpOnly, SameSite=Strict, Path=/); idle 30 menit, absolut 12 jam; rotasi saat login; pembersihan sesi
  basi tiap jam.
- **Lockout** 5 salah → 15 menit (terbuka otomatis) dan **rate limit login** per IP (30) & per nama pengguna (10) per
  10 menit di Redis, gagal tertutup (`503 auth_unavailable`).
- **CSRF**: metode yang mengubah data wajib `Origin` = domain aplikasi (`APP_ALLOWED_ORIGINS`) + header
  `X-Sentra-CSRF` = cookie `__Host-sentra_csrf` (double-submit).
- **Default-deny** `/api/v1/*` (`internal/api`): tanpa sesi hanya `health`, `version`, `auth/login` (+ `ready` di
  listener internal). Tes enumerasi semua route gagal bila ada route di luar allowlist yang bisa diakses tanpa sesi.
- Endpoint `POST /api/v1/auth/login`, `POST /auth/logout`, `GET /auth/me`, `POST /auth/password`,
  `GET /auth/sessions`, `DELETE /auth/sessions/{id}` (sesi sendiri saja). Selama wajib ganti kata sandi semua
  endpoint lain menjawab `403 password_change_required`. Ganti kata sandi mencabut semua sesi lain.
- **Audit log inti** (`audit_events`, append-only: UPDATE/DELETE/TRUNCATE ditolak trigger; kolom hash chain
  disiapkan untuk v0.3.3): login berhasil/gagal, lockout, logout, ganti kata sandi, cabut sesi, create-admin. Kata
  sandi, token, hash, dan cookie tidak pernah dicatat.
- CLI `sentra user create-admin --username <nama> [--display-name …]` (kata sandi dari terminal tanpa echo atau satu
  baris stdin; wajib diganti saat masuk pertama) — `docs/DISASTER_RECOVERY.md` *Pemulihan akses admin aplikasi*.
- UI: halaman **Masuk**, **Ganti kata sandi** (paksa & sukarela), **Sesi saya** (tabel desktop, kartu ponsel), menu
  pengguna (nama, Sesi saya, Ganti kata sandi, Keluar) di header; 401 dari API mana pun → halaman Masuk dengan
  pesan sesi berakhir; namespace i18n `auth` (ID/EN) + kode error baru.
- Pusat Bantuan: `auth` (Akun & masuk), `auth/login`, `auth/ganti-password`, `auth/sesi` (ID/EN); OpenAPI: tag
  `auth`, skema keamanan `sessionCookie`, respons error `Unauthenticated`/`Forbidden`/…
- Migrasi `000003_auth` (up/down): `users` (citext), `sessions`, `audit_events`.
- Integration test auth (DB & Redis nyata, DB sekali pakai): login, pesan generik, CSRF, lockout + buka otomatis,
  rate limit, idle & absolut, logout, cabut sesi, ganti kata sandi mencabut sesi lain, must-change, audit + tolak
  UPDATE/DELETE/TRUNCATE, CLI create-admin. E2E: `auth.spec.ts` (screenshot login, error login, ganti kata sandi
  paksa, shell, menu pengguna, Sesi saya desktop+mobile).

### Diubah
- **BREAKING (API):** `/api/v1/ui/bootstrap` dan `/api/v1/help/*` kini butuh sesi (401 `unauthenticated` tanpa
  cookie), juga di listener internal; `/api/v1/ready` hanya tanpa sesi di listener internal.
- Caddy meneruskan `/api/v1/auth/*` (masih di belakang basic_auth sementara sampai v0.3.4, ISS-026). Filter fail2ban
  `sentra-web` mengabaikan 401 yang dijawab app (`X-Request-Id` di `resp_headers`) agar login aplikasi yang salah
  atau sesi habis tidak mem-ban pengguna; sampel uji 9 → 11 baris (4 cocok, 1 diabaikan).
- `test/api/smoke.sh`: cek default-deny & CSRF; `test/api/ui.sh` & `test/e2e`: akun aplikasi
  (`SENTRA_APP_USER/PASS` atau `~/.config/sentra/app-auth`), probe kredensial lewat `/`; CI membuat akun sekali
  pakai lewat CLI. Recovery drill: + langkah CLI → login → wajib ganti sandi → help.
- `/design-system` kini juga butuh sesi (pembatasan role menyusul, ISS-029).

### Diperbaiki
- ISS-038: `source-rev.sh` di bawah `sudo` (*not a git checkout*) — `git -c safe.directory -C`, tanpa mengubah git
  config global root.
- ISS-037: recovery drill gagal keras (exit ≠ 0 di langkah pertama yang gagal, log disimpan) dan memverifikasi
  versi di image = `git describe`.
- ISS-025: `make version-check` mencoba ulang hingga 30 s setelah redeploy.

### Keamanan
- Backup DB kini berisi hash kata sandi & log audit (tetap 0700/0600). App memakai role DB pemilik migrasi —
  dipisah di v0.3.3 (ISS-039).

## [v0.2.9] - 2026-10-14 — UI Shell & Design System Close (penutup sprint v0.2)

Ringkasan sprint, DoD & bukti: `docs/releases/v0.2.9.md`.

### Bagian 2 (setelah reboot)
- Reboot ke kernel **7.0.0-1013-oracle** (6.17 tetap sebagai target rollback, ISS-036); downtime **± 37 detik**
  (perintah reboot 00:19:30 → layanan melayani 00:20:07 WIB), stack hidup sendiri.
- `make post-reboot-check` 17/17; full regression di kernel 7.0 **87/87**; recovery drill 8/8 — hangat
  (`--no-cache --pull`) build + start 92 s, **dingin** (setelah `docker builder prune -af`) 109 s; satu kegagalan build
  dingin sesaat tidak tereproduksi (ISS-037, skrip kini menyimpan log).
- `make test-ui` 41/41 dan `make test-e2e` 24/24 (pemilik, `(clean)`); pemeriksaan visual tanpa temuan baru.
- ROADMAP: sprint v0.2 selesai.

### Bagian 1 (sebelum reboot)

### Ditambahkan
- `make boot-menu` (`scripts/host-boot-setup.sh`, idempoten): menu GRUB 10 detik di serial console
  (`deploy/host/grub/99-sentra-boot-menu.cfg`), kernel 6.17 dipertahankan untuk rollback
  (`deploy/host/apt/99sentra-keep-kernel-6.17` + `apt-mark manual`, ISS-036), docker/containerd/fail2ban enabled.
- `make post-reboot-check` (`scripts/post-reboot-check.sh`): kernel & uptime, service, 4 container + restart policy,
  jail fail2ban & `DOCKER-USER`, smoke, edge, uji ban nyata.
- `make recovery-drill` (`scripts/recovery-drill.sh`): clone bersih dari GitHub → build semua image (termasuk web)
  sebagai project `sentradrill` di port 28xxx → 4 healthy, migrasi, smoke, UI, help → dibersihkan total.
- Runbook `docs/ops/runbooks/kernel-rollback.md` (pilih kernel lama lewat serial console, pin default).
- `scripts/lib/source-rev.sh`: baris pertama setiap skrip test/ops = commit + clean/DIRTY.

### Diubah
- CLAUDE.md **aturan 13**: edit di git worktree terpisah, test pemilik hanya setelah commit+push, skrip dibungkus
  `main "$@"`, hasil dari kode DIRTY tidak sah (ISS-035).
- Skrip test & ops (`test/api/*.sh`, `test/e2e/run.sh`, `scripts/full-regression.sh`, `scripts/fail2ban-*.sh`)
  dibungkus `main "$@"` + baris sumber.
- `scripts/full-regression.sh`: + actionlint (Docker), govulncheck, web-check, fail2ban-regex, bagian **10. Host
  boot & fail2ban** (service enabled, drop-in, jail, `DOCKER-USER`, ban nyata, GRUB, kernel 6.17), cek seluruh pohon
  backup (pemilik, 0700/0600), HEAD sudah di GitHub; **penjaga keras** migrasi up/down: hanya DB uji `sentra_regress`
  (exit 99 bila target kosong/produksi/lain) + cek versi skema produksi tidak berubah.

### Diperbaiki
- `ops/backup.sh`: `.lock` dibuat setelah `umask 077` dan di-`chmod 0600` (sebelumnya 0664).

## [v0.2.5] - 2026-10-13 — Mitigasi DoS basic_auth

### Ditambahkan
- **fail2ban jail `sentra-web`** (D-045, ISS-026): filter `sentra-caddy-auth` hanya menghitung 401 **dengan** header
  `Authorization` (kredensial salah) pada host publik, tidak bergantung urutan key JSON; 10 salah/10 menit → ban 1 jam
  (bertambah sampai 24 jam); blokir `REJECT` di iptables **`DOCKER-USER`** port 80/443; `ignoreip` hanya localhost.
  Dipasang `make fail2ban-web` (idempoten; path log via `docker volume inspect`; gagal jelas bila log tidak ada),
  dicabut `make fail2ban-web-remove`. Drop-in systemd: fail2ban `After=`/`Requires=docker.service`.
- **Access log Caddy ke file** (volume `caddy_logs`) dengan rotasi 20 MiB × 10 dan retensi **30 hari** (berisi IP
  klien); `Authorization` selalu `REDACTED`. Logger stdout tetap.
- `make preflight` **memperingatkan** bila bcrypt cost > 12.
- Test: `make fail2ban-test` (`fail2ban-regex` pada 9 baris log sintetis, termasuk urutan key berbeda & header palsu;
  juga di CI), `make fail2ban-ban-test` (ban nyata dari network namespace → ditolak → unban; varian
  `RESTART=fail2ban|docker`), cek log di `test/api/public.sh` (tumbuh, IP klien, kredensial tidak tercatat).
- Runbook `docs/ops/runbooks/fail2ban.md`; `web-basic-auth.md` diperbarui (hash cost 10).

### Diubah
- Rekomendasi hash basic_auth: `caddy hash-password --bcrypt-cost 10`. Benchmark di server (ARM, 4 vCPU): cost 14 ≈
  **1 030 ms** CPU per percobaan salah, cost 10 ≈ **65 ms** (~16× lebih ringan).

### Catatan
- IPv6 tidak berlaku (tanpa alamat IPv6 publik, tanpa AAAA, Docker tidak publish v6) — dicatat di runbook & ISS-026.
- Uji ban dari container Docker mem-ban IP gateway bridge karena MASQUERADE (ISS-034); uji memakai network namespace.
- Uji reboot sungguhan dijadwalkan di full regression v0.2.9.
- **Diperbaiki sebelum rilis (uji pemilik):** `make test-ui` dengan password salah mem-ban IP publik server (semua cek
  ber-auth terhitung). `test/api/ui.sh` dan `test/e2e/run.sh` kini probe kredensial **sekali** dan berhenti pada 401;
  pesan membedakan kredensial salah, koneksi ditolak (ban), dan TLS/DNS. Kutipan konfigurasi curl meng-escape `\`.
  Koreksi runbook: trafik dari server ke domain publiknya lewat hairpin NAT OCI dan **bisa** terkena ban (ISS-034).

## [v0.2.4] - 2026-10-13 — Pusat Bantuan di UI

### Ditambahkan
- Modul **help**: `GET /api/v1/help/index`, `GET /api/v1/help/pages/{slug}`, `GET /api/v1/help/openapi`
  (di balik basic_auth sementara). Konten `docs/help/{id,en}` dan `api/openapi.yaml` di-embed ke binary
  (`go:embed`); slug hanya dicocokkan ke whitelist di memori (path traversal → 404), locale tidak dikenal → 400.
- Validasi konten sebagai gerbang CI (`go test ./internal/help/...`): front matter, audience, key menu, **slug ID =
  EN**, menu `available` wajib punya halaman, tautan internal valid, tanpa HTML mentah di luar kode, tanpa detail
  infrastruktur (IP, path server, `.env`, user OS, sudo/ssh). Bila konten rusak tetap ter-deploy, **hanya modul help
  yang nonaktif (503 `help_unavailable`)** — app & healthcheck tetap jalan.
- UI Pusat Bantuan (menu **Bantuan** kini *available*): daftar halaman per audience + pencarian (judul & subjudul),
  renderer Markdown aman (react-markdown + remark-gfm; HTML mentah tampil sebagai teks, link eksternal
  `noopener noreferrer`, `javascript:`/`http:` dinetralkan), ganti bahasa memuat ulang halaman yang sama.
- Tombol **?** di header: membuka bantuan menu aktif (front matter `menu`), selain itu beranda Pusat Bantuan.
- **Referensi API** read-only buatan sendiri dari OpenAPI (`/help/api/reference`): endpoint per tag, parameter,
  skema (resolusi `$ref`), kode status; tanpa "Try it out".
- Label akses di viewer API dari ekstensi OpenAPI **`x-sentra-access`** (`public` → Publik, `internal` → Internal,
  `login` → Butuh login); test Go memastikan setiap operasi punya nilai sah dan `public` = allowlist edge Caddy.
- Komponen domain **`CopyButton`** (navigator.clipboard, tanpa library): Copy → Check ±2 detik, `aria-live`
  "Disalin", toast error bila clipboard gagal/tidak ada; tampil di showcase `/design-system`. Dipakai sebagai tombol
  salin path di setiap endpoint viewer API (muncul saat hover/fokus keyboard, selalu terlihat di perangkat sentuh).
- Header endpoint viewer API memakai pola disclosure + *stretched button* (tanpa elemen interaktif bersarang).
- Halaman bantuan ID+EN: Pusat Bantuan, Memulai, Dasbor, Dokumentasi API (admin).
- **govulncheck** di CI (`make vulncheck`).
- Test: Go (kontrak, 25 uji negatif validator, traversal, 503 terisolasi, `x-sentra-access`), Vitest 237,
  Playwright 24 (+ help: navigasi, cari, "?", viewer API, salin path dengan izin clipboard, ID↔EN, gelap/terang,
  mobile/sentuh, axe).

### Diubah
- Dokumen ops/dev keluar dari Pusat Bantuan: runbook ke `docs/ops/runbooks/`, catatan developer ke `docs/dev/`;
  konvensi Pusat Bantuan di `docs/help/README.md`.
- CLAUDE.md aturan 7 (lokasi, front matter, ID+EN setara, wajib per menu available, tanpa detail infrastruktur) dan
  aturan 2 (**pemeriksaan visual screenshot wajib**).
- Route modul kini `<path>/*` (modul memiliki sub-route, mis. `/help/<slug>`).
- Query UI tidak mengulang permintaan yang gagal 4xx (404/400 tidak berubah dengan retry).

### Keamanan
- `golang.org/x/text` 0.38.0 → 0.43.0: GO-2026-5970 & GO-2026-6629, jalur tercapai via pgx (ISS-033).

### Diperbaiki (sebelum rilis, dari pemeriksaan visual)
- Deskripsi OpenAPI di viewer tampil sebagai Markdown mentah (backtick, baris terputus) → dirender dengan renderer
  aman versi ringkas.
- Label "Publik" untuk endpoint tanpa autentikasi yang tidak dibuka di domain publik (`/ready`) → kini dari
  `x-sentra-access` ("Internal").
- Warna sukses tombol salin tertimpa warna hover di tema gelap → warna hover hanya saat idle.

## [v0.2.3] - 2026-10-13 — UI Shell

### ⚠️ BREAKING — kontrak `GET /api/v1/ui/bootstrap`
- Field top-level baru **`menu_groups`** (`[{key, order}]`: monitoring, infrastructure, operations, system).
- Field baru **`menus[].group`**; `menus[].order` kini urutan **di dalam grup**, dan `menus` diurutkan menurut grup.
- Path Dasbor berubah **`/` → `/dashboard`** (UI mengarahkan `/` ke `/dashboard`).
- Klien lain (bila ada) harus menyesuaikan; OpenAPI `0.2.3`.

### Ditambahkan
- Layout shell: sidebar dari registry (dikelompokkan, badge status untuk menu non-available, `aria-current`, versi
  app), bisa diciutkan ke ikon + tooltip (tersimpan di browser); drawer di layar < lg; header dengan breadcrumb,
  judul, jam live zona server (tanpa `aria-live`), bahasa, tema; skip link & landmark.
- Routing dari registry: halaman modul / Segera hadir / Belum terhubung; `/` → `/dashboard`; 404 di dalam shell;
  error boundary per halaman yang reset saat navigasi.
- Dasbor: empty state "Belum ada perangkat terhubung" (tanpa data dummy).
- Penjaga registry ↔ UI: fixture `web/src/test/fixtures/ui-registry.json` ditulis & diverifikasi `go test`
  (`-update`), dan test UI gagal bila menu `available` tidak punya halaman.
- Pusat Bantuan: `docs/help/{id,en}/dashboard/index.md`; `api/ui-bootstrap.md` diperbarui.
- Test: Vitest 206 (shell, routes, error boundary, jam); Playwright `shell.spec.ts` (13 menu, deep link + 404,
  collapse tersimpan, mobile 390 px tanpa scroll horizontal, keyboard, gelap/terang × ID/EN dengan axe).

### Diubah
- Halaman sementara v0.2.1 dihapus; info versi pindah ke sidebar.
- `test/e2e/smoke.spec.ts` kini memeriksa shell & Dasbor; `test/api/{smoke,ui}.sh` mengikuti kontrak baru.
- Artefak CI screenshot berganti nama `design-system-screenshots` → `ui-screenshots` (berisi design system + shell).

### Diperbaiki (sebelum rilis)
- Mode sidebar ciut kehilangan semua gaya: Radix `Slot` (Tooltip `asChild`) mengubah `className` berbentuk fungsi
  milik `NavLink` menjadi string. Kini `className` string + `useMatch`; ada test regresinya.
- Label menu terpotong di sidebar 16 rem karena badge status → lebar 18 rem.

## [v0.2.2] - 2026-10-13 — Design System & Komponen

### Ditambahkan
- Token desain semantik (`web/src/styles/tokens.css`): tema **gelap (default)**, terang, dan ikuti sistem; status
  DCIM normal/warning/critical/offline/maintenance (hijau/amber/merah/abu-abu/ungu; biru khusus link/info); status
  menu available/coming_soon/not_connected.
- Uji kontras otomatis dari `tokens.css` (139 pasangan, kedua tema): teks ≥ 4.5:1, non-teks (StatusDot, border
  badge, indikator fokus, batas input) ≥ 3:1.
- Font Inter + JetBrains Mono self-hosted (variable, subset latin), tanpa CDN.
- Komponen shadcn/ui di repo: Button, Card, Badge, Input, Select, Tabs, Table, Tooltip, DropdownMenu, Dialog,
  Sheet, Skeleton, Toast (sonner). Komponen domain: StatusBadge, StatusDot (ikon + label, tidak warna saja),
  MenuStatusBadge, ThemeSwitcher, LanguageSwitcher, Logo (placeholder, satu titik ganti `assets/brand/logo.svg`).
- Halaman internal `/design-system` (di luar registry menu, dimuat terpisah/lazy) dalam dua tema dan dua bahasa.
- `test/e2e/design-system.spec.ts`: kedua tema, Dialog/Sheet/Tooltip/Select/Dropdown/Toast, ID↔EN, console &
  CSP bersih, **axe-core** 0 pelanggaran serious/critical; screenshot diunggah sebagai artefak CI.
- `docs/design-system.md`, ADR-002, D-044.

### Diubah
- CSP respons UI: `style-src 'self' 'unsafe-inline'` (khusus style; `script-src` tetap `'self'`) setelah Playwright
  membuktikan Radix/sonner diblokir (ISS-027 selesai; nonce → ISS-030).
- ESLint `no-literal-string`: selain teks JSX, hanya atribut yang terlihat pengguna (`title`, `alt`, `placeholder`,
  `aria-label`, …) yang diperiksa; prop teknis (`variant`, `value`) tidak.
- Favicon kini memakai `assets/brand/logo.svg` (satu file logo).
- UI menghormati `prefers-reduced-motion` (transisi/animasi dimatikan, WCAG 2.3.3); e2e berjalan dengan reduced
  motion agar axe mengukur warna yang sudah stabil (sempat gagal di CI saat transisi tombol bahasa).

### Keamanan
- Kebijakan supply chain pnpm `minimumReleaseAge` kini benar-benar ditegakkan: pengecualian otomatis yang ditulis
  pnpm (sejak v0.2.1) dihapus dan paket dipatok ke rilis > 24 jam — `radix-ui 1.8.0`, `lucide-react 1.55.0`,
  `typescript-eslint 8.71.1`, `@testing-library/user-event 14.6.7` (ISS-031).
- Penjaga `pnpm supply:check` di `make web-check`, CI, dan build image: gagal bila ada pengecualian
  `minimumReleaseAge` atau umur rilis diturunkan di bawah 24 jam (ISS-031).

### Keputusan pemilik dicatat
- ROADMAP v0.2.2–v0.2.4, v0.2.9 final; fail2ban basic_auth dijadwalkan v0.2.5 (ISS-026). ISS-029: `/design-system`
  dibatasi role admin/dev di v0.3.

## [v0.2.1] - 2026-10-13 — Frontend Scaffold & i18n Foundation

### Ditambahkan
- UI web di `web/`: React 19 + Vite 8 + TypeScript strict, Tailwind v4 + dasar shadcn/ui, React Router 7, TanStack
  Query (D-042, [ADR-001](decisions/ADR-001-frontend-stack.md)). Build hanya di Docker multi-stage (Node 22, ARM64);
  host tidak butuh Node.
- i18n sejak awal: Bahasa Indonesia (default) & English, namespace `common`/`shell`/`errors`, key bertipe;
  ESLint `i18next/no-literal-string` = error; `pnpm i18n:check` menolak key yang hilang/berlebih.
- Endpoint `GET /api/v1/ui/bootstrap`: nama app, versi, locale & timezone default dari server (`DEFAULT_LOCALE`
  baru, `DISPLAY_TIMEZONE`), registry menu tunggal di backend (Dashboard *available*, modul lain *coming soon*;
  label diterjemahkan di UI). OpenAPI diperbarui.
- Halaman sementara: nama app, versi, bahasa & zona waktu aktif, jam sekarang (UTC → zona tampilan), daftar menu,
  tombol ganti bahasa ID/EN.
- Caddy menyajikan UI di `/` dengan SPA fallback, CSP ketat tanpa `unsafe-inline`, `index.html` `no-cache`, aset
  ber-hash `immutable`.
- **basic_auth sementara** untuk UI & bootstrap sampai Auth v0.3 (D-043, ISS-026). `make preflight` menolak mode
  publik tanpa hash; tanpa hash UI terkunci.
- Test: Vitest (10), Go unit test bootstrap, `test/api/ui.sh` (edge UI), `test/e2e` Playwright (ID↔EN, console &
  CSP), `make web-check`, `make test-ui`, `make test-e2e`; job CI `web` + uji UI & e2e di job build.
- Kerangka Pusat Bantuan dua bahasa `docs/help/{id,en}/` (diisi di v0.2.4).

### Regresi modul v0.2.1
- Web: lint, typecheck, `i18n:check`, Vitest 10/10 (+ uji negatif lint & katalog); Go unit test + golangci-lint 0 issue.
- Server produksi: 4 container healthy; smoke internal 27/27; edge publik 30/30; `test-ui` 32/32 dan `test-e2e`
  Playwright 2/2 (dijalankan pemilik); browser: login, ID↔EN, deep link, console bersih, jam WIB benar.
- CI GitHub (run `38390860588`): semua job lulus, termasuk UI edge 32/32 dan Playwright 2/2.

### Diperbaiki
- Respons error dari handler Caddy (401 basic_auth, 404/405 statis) kini tetap membawa header keamanan (HSTS,
  nosniff, tanpa `Server`) dan body `ErrorBody` JSON (`handle_errors`) — ditemukan saat uji `curl -I` v0.2.1.
- Header CSP ganda (Caddy + app) pada respons API: CSP dari app dibuang di proxy, tinggal satu kebijakan.

### Diubah
- `test/api/public.sh`: `/` dan `/admin` kini 401 (basic_auth), bukan 403. API selain health/version tetap 403.
- Image Caddy kini ditag per versi (`sentra-caddy:<git describe>`), sama seperti app.
- `make caddy-validate` memvalidasi Caddyfile dengan image Caddy resmi (tanpa build UI).

## [v0.1.9 putaran 2] - 2026-10-13 — Foundation Close (tanpa tag baru)

### Diperbaiki
- Uji firewall dari luar lulus: hanya 22/80/443 terbuka; `53/tcp` di nmap terbukti false positive ISP (ISS-005).
- GitHub Release v0.1.9 dibuat pada tag lama `8aeaf87` (ISS-010).
- Akses darurat server Oracle: key SSH cadangan terpasang dan password konsol `ubuntu` disetel
  pemilik, sehingga serial console OCI bisa dipakai (ISS-016). `docs/DISASTER_RECOVERY.md` kini memuat checklist
  langkah demi langkah "kalau SSH hilang" dan cara aman mengubah `ADMIN_IPS`.
- Push ke GitHub dari server Oracle via deploy key baru `sentra-oracle-deploy` (ISS-019, D-041); key lama
  `sentra-dcim-server` tidak dipakai lagi.
- `ops/backup.sh` tidak lagi meng-`chown` direktori backup yang sudah ada bila dijalankan user lain; kini menolak
  dengan petunjuk `sudo -u <pemilik>` (ISS-024).

### Diubah
- Fingerprint key tidak lagi dicatat di repo (D-041, `docs/DISASTER_RECOVERY.md`); disimpan di brankas pemilik.
- Aturan regresi dipertegas (CLAUDE.md aturan 2–3, D-038): full regression **hanya** di penutup sprint `.9`;
  sub-versi `.1–.8` cukup regresi modul terkait. Catatan rilis v0.1.8 dikoreksi — hasil 72/72 dicatat sebagai
  verifikasi migrasi server, bukan full regression (ISS-023).
- Go public di server Oracle (D-039): DNS `sentra.bajastu.id` → `132.226.106.158`; ufw membuka 80/443
  (`UFW_OPEN_HTTP=1`), OCI Security List 80/443, `.env` mode publik. Sertifikat Let's Encrypt terbit; edge test
  publik 26/26 (ISS-015 selesai). Rincian: addendum `docs/releases/v0.1.9.md`.
- Jam OS server kini `Asia/Jakarta` (WIB) — kosmetik untuk log host (D-040); container, DB & penyimpanan tetap UTC,
  backup tetap 02:00 WIB.

## [v0.1.8] - 2026-10-13 — Timezone & Version Fix

### Diubah
- Zona waktu tampilan kini `DISPLAY_TIMEZONE` di `.env`, default **Asia/Jakarta (WIB, UTC+7)**; database & server
  tetap UTC. Nama lama `APP_DISPLAY_TZ` masih dibaca sebagai fallback (D-032).
- Timer backup harian kini 02:00 **Asia/Jakarta** (`OnCalendar=*-*-* 02:00:00 Asia/Jakarta` = 19:00 UTC).
- **Keamanan:** `/api/v1/version` publik hanya mengembalikan `{"version"}`. Commit, tanggal build, versi Go, jam
  server & zona tampilan hanya dari listener internal app (`:8081`, di-proxy Caddy `127.0.0.1:8080` saja); v0.3
  menambah syarat role Admin (D-033). Respons `/version` kini `Cache-Control: no-store`.
- Caddy: situs publik → `app:8080`, listener internal → `app:8081`.
- Dokumen yang menyebut WITA/Asia/Makassar diperbarui ke WIB/Asia/Jakarta (entri CHANGELOG & release notes lama
  dibiarkan sebagai catatan sejarah).

### Diperbaiki
- `/api/v1/version` bisa berbeda dari `git describe` HEAD (ISS-022). `VERSION`/`COMMIT` di Makefile tidak bisa lagi
  ditimpa variabel environment; `make up` selalu `docker compose build app caddy` lalu `make version-check`;
  `scripts/release.sh` me-redeploy stack setelah membuat tag.

### Ditambahkan
- `make version-check`; full regression memeriksa versi berjalan == `git describe` (internal & publik) dan tag
  image `sentra-app:<versi>`; smoke test memeriksa zona & offset sesuai `DISPLAY_TIMEZONE`.
- ROADMAP: i18n wajib sejak v0.2 (D-035); v0.3.5 *Settings – Regional*; v0.3.6 *Settings – Allowed IP*
  (agent host, anti-lockout, MAC tidak dipakai — D-036, D-037).

### Migrasi server ke Oracle Cloud (pekerjaan pra-v0.1.8, D-031)
- Akses darurat di `docs/DISASTER_RECOVERY.md` + Pusat Bantuan `ops/akses-darurat.md`: OCI Serial/VNC Console,
  key SSH cadangan, aturan perubahan SSH (`PasswordAuthentication` tidak diubah tanpa konfirmasi), unban fail2ban.
- fail2ban tidak pernah mem-ban `ADMIN_IPS` dan IP sesi SSH aktif (D-030).
- `ufw.sh` menghapus aturan iptables `REJECT` bawaan image Oracle yang membuat ufw tidak efektif (ISS-017);
  `ops/harden/services.sh` mematikan `rpcbind` (ISS-018).
- Bootstrap: `.env` otomatis dalam mode lokal (tanpa ACME) kecuali `--domain/--acme-email`; `--admin-user` (ACL);
  diuji nyata di server kosong — ISS-003 selesai. `test/api/public.sh` punya mode lokal (CA internal Caddy).

## [v0.1.7] - 2026-10-12 — Caddy Empty Env Fix

### Diperbaiki
- CI gagal sejak v0.1.6: Caddyfile tidak bisa diparse bila `DOMAIN`/`ACME_EMAIL` kosong, karena default
  `{$VAR:default}` Caddy hanya berlaku untuk variabel yang tidak di-set (ISS-014). Default kini di
  `docker-compose.yml` (`localhost`, `admin@localhost`).

### Ditambahkan
- `make preflight` (dipanggil `make up`): menolak `DOMAIN` asli tanpa `ACME_EMAIL`.
- `make caddy-validate`: validasi Caddyfile untuk env produksi & kosong; dijalankan di CI dan full regression.

 - 2026-10-12 — Domain & HTTPS

### Ditambahkan
- HTTPS publik `https://sentra.bajastu.id` via Caddy dengan sertifikat Let's Encrypt otomatis (HTTP-01/TLS-ALPN-01).
  `DOMAIN` dan `ACME_EMAIL` di `.env` (contoh di `.env.example`).
- Redirect HTTP → HTTPS (308) eksplisit; HSTS 1 tahun, CSP, X-Frame-Options, X-Content-Type-Options,
  Referrer-Policy, Permissions-Policy, CORP/COOP; header `Server` & `Via` disembunyikan (termasuk pada redirect).
- Allowlist proxy: hanya `GET`/`HEAD` `/api/v1/health` dan `/api/v1/version` yang publik; lainnya `403` JSON.
- Listener internal `127.0.0.1:8080` (API lengkap) untuk skrip ops dan smoke test.
- `test/api/public.sh`: 25 uji edge publik (TLS issuer & masa berlaku, TLS 1.1 ditolak, headers, allowlist,
  path traversal, redirect); masuk ke `scripts/full-regression.sh`.
- Pusat Bantuan: `docs/help/admin/domain-https.md`.

### Dokumentasi
- DECISIONS D-022 (hotfix pasca-sprint memakai slot cadangan), tertinggal dari commit v0.1.5.

### Diubah
- ufw membuka 80/tcp & 443/tcp; `ops/harden/ufw.sh` membaca `UFW_OPEN_HTTP` dari `.env` (re-run tidak menutup situs).
- Caddy non-root bind 80/443 lewat sysctl container `net.ipv4.ip_unprivileged_port_start=0` (tanpa capability);
  HTTP/3 dimatikan (hanya TCP yang dibuka).
- Backend melayani `HEAD` untuk route `GET` (`curl -I` sebelumnya akan 405).
- Container app memakai `hostname: sentra-app` agar `X-Request-Id` tidak membocorkan ID container.
- CI memakai `DOMAIN` kosong (localhost) dan bind loopback.

## [v0.1.5] - 2026-10-12 — CI Redis Auth (hotfix pasca-sprint)

### Diperbaiki
- CI: Redis kini berjalan dengan `requirepass` seperti produksi. Sebelumnya service Redis tanpa password membuat
  `TestRedis` gagal karena Redis 8 menerima password apa pun bila user `default` tanpa password (ISS-009).
- Integration test `TestRedis` gagal dengan pesan jelas bila `SENTRA_IT_REDIS_PASSWORD` kosong.

 - 2026-10-12 — Full Regression & DoD

### Ditambahkan
- `scripts/full-regression.sh`: 67 check regresi + DoD (static/secret scan, unit, integration, container, API,
  backup & restore ke DB kosong, migrasi up/down, permission, firewall & hardening, status repo).
- Release notes `docs/releases/v0.1.9.md` + log regresi & rollback.

### Diperbaiki
- `ops/rollback.sh`: versi asal dibaca dari `/api/v1/version` yang sedang berjalan (sebelumnya dari git, bisa salah
  bila checkout dilakukan manual sebelum skrip).

### Diuji
- Rollback v0.1.4 → v0.1.3 (16 dtk) dan kembali ke main (15 dtk); restore terisolasi & penuh; firewall dari server.

 - 2026-10-12 — Ops & DR

### Ditambahkan
- `ops/backup.sh`: `pg_dump` custom terkompresi + globals + volume `caddy_data`/`caddy_config`/`redis_data`,
  `manifest.json`, `SHA256SUMS` (diverifikasi), lock, retensi (default 14 hari), hook offsite (Pending), pembersihan
  direktori `.partial` saat gagal.
- `ops/restore.sh`: verifikasi checksum, restore penuh (safety backup otomatis, stop app/caddy, TimescaleDB
  pre/post restore, volume, migrate up, tunggu ready) atau restore terisolasi ke DB lain (`--target-db`).
- `ops/rollback.sh <tag|main>`: backup → migrate down dengan binary saat ini → checkout → `make up` → verifikasi
  `/ready` & `/version`; re-exec dari salinan sementara agar aman terhadap `git checkout`.
- `ops/bootstrap-new-server.sh`: Ubuntu 24.04 kosong → Docker CE, user `sentra`, clone, `.env`, hardening, timer
  backup, `make up`, restore opsional, smoke test; mode `--dry-run`.
- Timer systemd `sentra-backup.timer` (harian 02:00 WITA) + `ops/install-backup-timer.sh`, target `make backup-timer`.
- Migrasi `000002_ops_runs` (log operasi backup/restore/rollback) dengan `down`.
- GitHub Actions CI (`.github/workflows/ci.yml`): gitleaks, golangci-lint, ShellCheck, OpenAPI lint, `sqlc diff`,
  unit + integration test (service TimescaleDB & Redis), build image + cek non-root + API smoke test.
- `docs/DISASTER_RECOVERY.md` lengkap; Pusat Bantuan `docs/help/ops/backup-restore.md`.

 - 2026-10-12 — Backend Skeleton

### Ditambahkan
- Backend Go modular monolith: `cmd/sentra` dengan subcommand `serve`, `migrate up|down|goto|force|version`,
  `healthcheck`, `version`.
- Endpoint `GET /api/v1/health`, `GET /api/v1/ready` (cek database, redis, versi migrasi), `GET /api/v1/version`
  (versi dari tag Git via ldflags, waktu UTC + WITA).
- `internal/platform/`: config dari env (tervalidasi), logging slog JSON (UTC), pool pgx, klien Redis, migrator
  golang-migrate (migrasi di-embed ke binary), HTTP server (request ID, access log, recoverer JSON, security headers,
  batas body 1 MB, timeout, rate limit per IP, real-IP hanya dari proxy tepercaya).
- Graceful shutdown: SIGTERM → readiness 503 (draining) → `Shutdown` dengan timeout.
- Migrasi pertama `000001_init` (extension timescaledb, tabel `system_info`) dengan `down`.
- sqlc (`sqlc.yaml`, repo modul `system`), `api/openapi.yaml` (OpenAPI 3.1, lulus Redocly lint).
- Unit test (config, service, handler, middleware), integration test (migrasi up/down/up pada DB sementara, Redis),
  API smoke test `test/api/smoke.sh`; golangci-lint v2 (`.golangci.yml`).
- Pusat Bantuan: `docs/help/api/system.md`.
- Target Make: `openapi-lint`, `migrate-down/version/goto`, `test-integration`, `test-api`.

### Diubah
- Caddy tidak lagi memakai `trusted_proxies`; `X-Real-IP` di-set dari IP klien sebenarnya dan `True-Client-IP` dibuang
  (mencegah spoofing IP untuk bypass rate limit).
- `make gitleaks` memindai riwayat Git + file yang tidak di-ignore (sebelumnya `.env` lokal ikut terdeteksi).

 - 2026-10-12 — Infra Base

### Ditambahkan
- `docker-compose.yml`: service `db` (TimescaleDB 2.30.2-pg16), `redis` (8.10.2), `app`, `caddy` (2.11.7) dengan
  healthcheck, named volume, port bind `127.0.0.1`, batas resource, restart policy, `read_only`, `cap_drop: ALL`,
  `no-new-privileges`, dan semua container non-root.
- `Dockerfile` backend multi-stage → distroless `nonroot`; image Caddy turunan non-root di port 8080.
- Caddyfile dengan security headers (CSP, HSTS, X-Frame-Options, dll.) dan batas body 10 MB.
- `ops/harden/` idempoten: `ufw.sh`, `fail2ban.sh`, `sysctl.sh`, `unattended-upgrades.sh`, `all.sh`; target `make harden`.
- Pusat Bantuan: `docs/help/ops/infrastruktur.md`.
- Stub backend minimal (`/api/v1/health`) untuk validasi infra; diganti di v0.1.3.

 - 2026-10-12 — Repo Scaffold

### Ditambahkan
- Struktur folder modular monolith (`cmd/`, `internal/`, `api/`, `migrations/`, `deploy/`, `ops/`, `docs/`).
- `CLAUDE.md` berisi 12 aturan permanen proyek.
- `.gitignore`, `.editorconfig`, `.env.example`, `.gitleaks.toml`.
- `Makefile` dengan target `up/down/logs/test/lint/migrate/backup/restore/rollback/release` dan lainnya.
- Dokumen wajib: CHANGELOG, ISSUES, DEPENDENCIES, DECISIONS, ROADMAP, DISASTER_RECOVERY.
- Kerangka Pusat Bantuan `docs/help/README.md`.
- Pre-commit hook gitleaks (`scripts/hooks/pre-commit`, dipasang dengan `make hooks`).
