# Akses Darurat Server (SSH Hilang)

**Modul:** ops · **Tersedia sejak:** v0.1.8 · **Status:** Available

## Tujuan
Memastikan administrator selalu punya jalan masuk ke server Sentra DCIM (Oracle Cloud) walaupun SSH gagal:
key hilang, sshd salah konfigurasi, terkunci firewall, atau IP di-ban fail2ban. Prosedur lengkap ada di
[`docs/DISASTER_RECOVERY.md` → Akses darurat](../../DISASTER_RECOVERY.md#akses-darurat-ssh-hilang).

## Role yang dibutuhkan
| Aksi | Role |
|---|---|
| Membuka Serial/VNC Console, Run Command | Akun Oracle Cloud dengan izin `manage instance-console-connection` / Run Command |
| Menambah key SSH cadangan, set password konsol, unban fail2ban | Administrator server (`sudo`) |
| Mengubah `PasswordAuthentication` atau hardening SSH lain | Administrator server **+ konfirmasi pemilik sistem** |

## Langkah-langkah
1. **Persiapan (sekali, sebelum ada masalah):**
   - pasang **key SSH cadangan** dan uji dari terminal baru;
   - set **password konsol** untuk user `ubuntu` (hanya untuk serial console; SSH tetap key-only);
   - isi `ADMIN_IPS` di `.env` dengan IP/CIDR admin lalu `sudo ops/harden/fail2ban.sh`.
2. **Bila SSH gagal:** OCI Console → Compute → Instances → *instance* → **Console connection** →
   *Launch Cloud Shell connection* → login dengan password konsol.
3. **Bila IP di-ban fail2ban:** `sudo fail2ban-client set sshd unbanip <IP>` (atau `unban --all`).
4. **Bila firewall mengunci:** `sudo ufw allow 22/tcp`, perbaiki, lalu `sudo ops/harden/ufw.sh`.
5. **Sebelum mengubah konfigurasi SSH:** validasi `sudo sshd -t`, `reload` (bukan restart), uji login di sesi terpisah
   sebelum menutup sesi lama.

## Endpoint API terkait
Tidak ada (operasi server). Mulai v0.3.6 allowlist IP dikelola lewat `/api/v1/settings/allowed-ips`.

## Pertanyaan umum / pemecahan masalah
| Gejala | Penyebab umum | Tindakan |
|---|---|---|
| Serial console meminta password tapi tidak bisa login | Password user terkunci (bawaan image Oracle) | Gunakan Run Command atau lepas boot volume; setelah masuk, `sudo passwd ubuntu` |
| SSH *timeout* hanya dari satu IP | IP di-ban fail2ban | Unban via console, tambahkan ke `ADMIN_IPS` |
| Port baru tidak bisa diakses walau `ufw allow` | Security List / NSG VCN belum dibuka | Buka port di OCI Console |
