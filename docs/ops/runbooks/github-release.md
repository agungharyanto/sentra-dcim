# GitHub Release dari Server (`gh`)

**Modul:** ops · **Tersedia sejak:** v0.3.9 · **Keputusan:** D-081

## Tujuan
Membuat GitHub Release untuk rilis penutup sprint (`vX.Y.9`) dari server, dengan isi `docs/releases/vX.Y.9.md`,
setelah `make release` selesai (tag sudah di-push).

## Token yang dipakai

| Hal | Nilai |
|---|---|
| Jenis | Token OAuth GitHub CLI (`gho_…`), dibuat oleh `gh auth login` |
| Scope | `repo`, `gist`, `read:org` — **semua** repo pemilik, bukan hanya `sentra-dcim` |
| Kedaluwarsa | **Tidak ada** (berlaku sampai dicabut) |
| Lokasi | user OS `ubuntu` di server produksi (`gh auth status`) |
| Status risiko | Diterima pemilik (D-081); ditinjau ulang di **v0.12 (Hardening)** |

Push Git tetap memakai deploy key (D-041); `gh` hanya untuk release.

## Langkah
1. Pastikan tag sudah ada di GitHub: `git ls-remote origin refs/tags/vX.Y.9`.
2. Cek login: `gh auth status` (akun pemilik, aktif).
3. Buat release dari working tree `main` yang bersih:
   `gh release create vX.Y.9 --notes-file docs/releases/vX.Y.9.md --title "vX.Y.9 — <Judul>" --verify-tag`
4. Periksa: `gh release view vX.Y.9`.

## Bila server dicurigai disusupi
1. Cabut token di github.com → **Settings → Applications → Authorized OAuth Apps → GitHub CLI → Revoke**
   (mencabut semua token `gh` untuk akun itu, di semua mesin).
2. Di server (bila masih bisa diakses): `gh auth logout -h github.com`.
3. Periksa aktivitas akun (Settings → Security log) dan repo pemilik untuk push/release yang tidak dikenal; lanjutkan
   dengan prosedur pemulihan di `docs/DISASTER_RECOVERY.md` (termasuk mencabut deploy key, D-041).

## Tinjau ulang (v0.12 Hardening)
Pertimbangkan fine-grained token (hanya repo `sentra-dcim`, *Contents: Read and write*, dengan kedaluwarsa) atau
membuat release dari luar server. Bila diganti, catat tanggal kedaluwarsa token di tabel di atas.
