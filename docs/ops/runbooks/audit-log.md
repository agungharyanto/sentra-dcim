# Audit Log: verifikasi hash chain, anchor, retensi & arsip

**Modul:** audit · **Tersedia sejak:** v0.3.4 · **Status:** Available (khusus administrator server)

Panduan pengguna (menu Audit Log, Admin/Auditor) ada di Pusat Bantuan `docs/help/{id,en}/audit/`. Keputusan:
D-051, D-072 (role DB), D-073 (hash chain), D-075 (viewer).

## Cara kerja singkat
- Tabel `audit_events` hanya bisa INSERT + SELECT oleh role app (`sentra_app`, GRANT di migrasi 000006); UPDATE /
  DELETE / TRUNCATE ditolak hak akses **dan** trigger append-only.
- Trigger `audit_events_chain` (migrasi 000007) mengisi `chain_seq`, `prev_hash`, `hash = sha256(prev_hash ‖
  kanonik(baris))` di bawah advisory lock `sentra.audit_chain`. Bentuk kanonik dihitung ulang mandiri oleh
  `internal/audit/chain`.
- App memverifikasi seluruh chain saat start lalu tiap 24 jam; gagal → log level `error` pesan
  `audit chain verification FAILED` (`broken_seq`, `reason`), siap diteruskan Alarm Engine (v0.5).
- Setiap `ops/backup.sh` menulis `audit-anchor.json` (kepala chain, sebelum dump) dan mencakupnya di `SHA256SUMS`.

## Verifikasi

| Perintah | Kegunaan |
|---|---|
| `make audit-verify` | Chain dihitung ulang + dicocokkan dengan anchor **semua** backup (deteksi penulisan ulang seluruh chain oleh owner DB). Jalankan sebagai `sentra` (direktori backup 0700) |
| `make audit-verify UNTIL=<dir backup>` | Setelah restore: hanya anchor s.d. backup yang di-restore |
| `docker compose exec -T app /sentra audit verify` | Tanpa anchor (sama dengan job harian) |
| `docker compose exec -T app /sentra audit head` | Kepala chain saat ini (JSON anchor) |
| `docker compose exec -T app /sentra audit verify-archive < arsip.jsonl.gz` | Verifikasi arsip pemangkasan offline |

Output JSON: `ok`, `checked`, `head_seq`, `checkpoint_seq`, `anchors_checked`, `broken_seq`, `reason`. Exit code ≠ 0
bila rusak. Alasan yang mungkin: `row content does not match its hash (modified)`, `row with chain_seq N is missing`,
`prev_hash does not match`, `hash differs from the anchor in a backup (chain rewritten)`,
`anchored row N no longer exists (chain truncated)`, `checkpoint hash differs from an anchor`.

### Bila verifikasi gagal
1. Jangan ubah database. Simpan output JSON dan log app (`make logs SERVICE=app`).
2. `make backup` dengan label `incident` (bukti kondisi saat ini).
3. Bandingkan dengan backup lama: restore ke database uji (`ops/restore.sh <dir> --target-db sentra_audit_check
   --yes`) lalu `make audit-verify UNTIL=<dir>` pada salinan tsb untuk menemukan backup terakhir yang utuh.
4. Putuskan bersama pemilik: restore produksi dari backup utuh, atau pertahankan dan catat di `docs/ISSUES.md`.

## Retensi & pemangkasan
- `AUDIT_RETENTION_DAYS` (default 365, minimum 90; nilai di bawahnya membuat app/ops menolak start).
- `make audit-prune` = `ops/audit-prune.sh` (juga dijalankan unit `sentra-backup.service` setelah backup sukses;
  pasang ulang unit dengan `make backup-timer` setelah upgrade ke v0.3.4):
  1. `sentra audit prune-plan` (container `ops`) → segmen prefiks chain yang lebih tua dari retensi.
  2. `sentra audit export --through N` → `$BACKUP_DIR/audit-archive/audit-<dari>-<sampai>.jsonl.gz` (0600), `gzip -t`,
     jumlah baris dicek, `.sha256` ditulis.
  3. `sentra audit prune --through N --archive-name … --archive-sha256 …` sebagai **owner** dalam satu transaksi:
     kunci tabel, verifikasi penuh, ekspor ulang dan bandingkan checksum dengan arsip yang tersimpan (beda → batal),
     simpan checkpoint (`audit_checkpoints`: `through_seq`, `last_hash`, jumlah, rentang waktu, nama & sha256 arsip),
     hapus baris, catat event `audit.prune`, verifikasi ulang, commit.
- Arsip **tidak** terhapus oleh retensi backup; ikutkan ke salinan offsite. Tanpa arsip, isi event yang dipangkas
  hilang, tetapi chain sisa tetap terverifikasi dari checkpoint.

## Rotasi kredensial DB
Lihat [DISASTER_RECOVERY.md](../../DISASTER_RECOVERY.md) bagian "Kredensial database: role owner & role app".
