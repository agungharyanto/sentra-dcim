# Dasbor

**Modul:** dashboard · **Tersedia sejak:** v0.2.3 · **Status:** Available

## Tujuan
Halaman pertama setelah membuka Sentra DCIM: ringkasan kondisi data center (daya, lingkungan, alarm) untuk tim NOC
dan pengelola. Saat ini **belum ada perangkat atau simulator terhubung**, sehingga Dasbor hanya menampilkan pesan
"Belum ada perangkat terhubung" — tanpa angka atau grafik contoh. Data mulai tampil setelah Telemetry Ingestion
(v0.6).

## Role yang dibutuhkan
| Aksi | Role |
|---|---|
| Melihat Dasbor | Pemilik kredensial basic_auth sementara (sampai Auth v0.3, lihat [admin/web-basic-auth.md](../../admin/web-basic-auth.md)); mulai v0.3: semua role yang login |

## Langkah-langkah
1. Buka `https://sentra.bajastu.id/` dan masukkan user + password. Alamat `/` otomatis diarahkan ke `/dashboard`.
2. Navigasi ada di **sidebar kiri**, dikelompokkan: Pemantauan, Infrastruktur, Operasional, Sistem. Menu yang belum
   siap bertanda **Segera hadir**; membukanya menampilkan penjelasan singkat modul tersebut.
3. **Ciutkan sidebar** dengan tombol di pojok kiri bawah (hanya ikon; arahkan kursor/fokus ke ikon untuk melihat
   nama menu). Pilihan ini diingat browser.
4. Di layar kecil (ponsel/tablet), buka menu lewat tombol **☰** di kiri atas; tutup dengan **Esc** atau tombol ×.
5. Header menampilkan judul halaman dan jejak lokasi (breadcrumb), **jam** menurut zona waktu server (mis. WIB),
   pilihan **bahasa** (ID/EN), dan **tema** (Gelap/Terang/Ikuti sistem).
6. Versi aplikasi tertera di bagian bawah sidebar — sebutkan versi ini saat melaporkan masalah.
7. Pengguna keyboard: tekan **Tab** sekali untuk tautan **Lompat ke konten**.

## Endpoint API terkait
| Metode | Path | Keterangan |
|---|---|---|
| GET | `/api/v1/ui/bootstrap` | Registry menu (grup, status, urutan), bahasa & zona waktu default, versi — lihat [api/ui-bootstrap.md](../../api/ui-bootstrap.md) |

## Pertanyaan umum / pemecahan masalah
- **"Halaman tidak ditemukan"**: alamat tidak ada; gunakan menu di sidebar atau tombol *Kembali ke Dasbor*.
- **"Terjadi kesalahan"**: halaman gagal tampil; sidebar tetap bisa dipakai untuk pindah halaman. Bila berulang,
  laporkan beserta versi aplikasi.
- **Jam berbeda dengan jam lokal**: jam mengikuti zona waktu server (`DISPLAY_TIMEZONE`), bukan zona perangkat Anda.
