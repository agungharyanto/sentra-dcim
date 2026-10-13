# Rollback Kernel lewat Serial Console

**Modul:** ops · **Tersedia sejak:** v0.2.9 · **Status:** Available

## Tujuan
Bila server tidak bisa boot dengan kernel baru (mis. `7.0.0-1013-oracle`, v0.2.9) atau layanan rusak setelah boot,
pilih kernel sebelumnya (`6.17.0-1020-oracle`) dari menu GRUB lewat **OCI Serial Console**, lalu (bila perlu)
jadikan default. Kernel lama sengaja dipertahankan sampai penutup sprint v0.3 (ISS-036).

## Prasyarat (sudah disiapkan oleh `make boot-menu`)
| Hal | Nilai |
|---|---|
| Menu GRUB | tampil **10 detik** (`/etc/default/grub.d/99-sentra-boot-menu.cfg`, menimpa `50-cloudimg-settings.cfg` yang menyembunyikan menu) |
| Terminal GRUB | `console` (konsol EFI = yang tampil di serial console OCI Arm) |
| Kernel lama | `linux-image/modules/modules-extra-6.17.0-1020-oracle`: `apt-mark manual` + `APT::NeverAutoRemove` (`/etc/apt/apt.conf.d/99sentra-keep-kernel-6.17`) — tidak dihapus unattended-upgrades |
| Akses konsol | user `ubuntu` + password konsol (brankas pemilik), lihat [akses-darurat.md](akses-darurat.md) |

Cek cepat: `sudo grep -E "timeout(_style)?=" /boot/grub/grub.cfg | head -3` → `menu` dan `10`;
`apt-mark showmanual | grep 6.17` → tiga paket.

## Role yang dibutuhkan
| Aksi | Role |
|---|---|
| Serial console, memilih kernel | Pemilik akun OCI + password konsol `ubuntu` |
| Mengubah default kernel | Admin OS (`sudo`) |

## Langkah: boot sekali dengan kernel lama
1. **Buka serial console sebelum reboot**: OCI Console → Compute → Instances → *instance* → **Console connection**
   → *Launch Cloud Shell connection*. Biarkan terbuka.
2. Reboot (dari SSH: `sudo systemctl reboot`, atau OCI Console → *Reboot*). Bila SSH sudah tidak bisa: OCI Console
   → *Reboot* (bukan *Force* kecuali macet).
3. Di serial console tunggu menu GRUB (≤ 10 detik setelah firmware). Tekan **↓** agar hitung mundur berhenti.
4. Pilih **Advanced options for Ubuntu** → **Ubuntu, with Linux 6.17.0-1020-oracle** → **Enter**.
   Bila tombol panah tidak terkirim lewat terminal web, menu GRUB juga menerima **Ctrl+N** (bawah) dan **Ctrl+P**
   (atas). Jangan tekan **e** (mode edit) kecuali tahu yang dilakukan; **Esc** kembali ke menu.
5. Login (SSH atau konsol), cek `uname -r` → `6.17.0-1020-oracle`, lalu `make post-reboot-check`.

Pilihan ini **hanya untuk sekali boot**: boot berikutnya kembali ke entri default (kernel terbaru).

## Langkah: jadikan kernel lama default (bila 7.0 bermasalah permanen)
```
sudo sed -n "s/^\s*menuentry '\(Ubuntu, with Linux 6.17.0-1020-oracle\)'.*/\1/p" /boot/grub/grub.cfg   # pastikan judul
echo 'GRUB_DEFAULT="Advanced options for Ubuntu>Ubuntu, with Linux 6.17.0-1020-oracle"' | \
  sudo tee /etc/default/grub.d/98-sentra-default-kernel.cfg
sudo update-grub
```
Catat di `docs/ISSUES.md` (kernel default dipin, alasan, versi). Membatalkan: hapus file `98-…cfg` lalu
`sudo update-grub`.

## Bila menu GRUB tidak muncul di serial console
- Firmware OCI Arm menampilkan konsol EFI di serial console; bila menu tetap tidak terlihat, boot tetap jalan ke
  kernel default setelah 10 detik — tidak ada risiko macet di menu.
- Alternatif tanpa menu: dari sistem yang sudah boot (SSH/konsol) jalankan
  `sudo grub-reboot "Advanced options for Ubuntu>Ubuntu, with Linux 6.17.0-1020-oracle" && sudo reboot`
  (sekali boot dengan kernel lama).
- Bila sistem sama sekali tidak bisa boot: **boot volume backup** yang dibuat pemilik di OCI Console sebelum reboot
  (pulihkan ke instance), atau jalur #5 di [akses-darurat.md](akses-darurat.md).

## Setelah boot (kernel mana pun)
`make post-reboot-check` (sudo): kernel & uptime, service enabled, 4 container healthy, jail `sentra-web` & `sshd`,
`DOCKER-USER` → `f2b-sentra-web`, smoke & edge, uji ban nyata.
