# Runbook Migrasi — Admin Stock Deposits v10 (v0.14.1 R7)

> Status: v10 irreversible by policy (data trial + audit dipertahankan).
> DB workspace: cryptostation @ cbt-vps, migration v10.
> Backup terakhir: `/tmp/cs-backup-pre-v0141-20261103-0023.sql` (3,1 MB, full dump `--no-owner`).
> Kredensial: TIDAK ada di dokumen ini. Gunakan env server / `.env` 600.

## 1. Keputusan

- v10 (`000010_admin_stock_deposits`) IRREVERSIBLE: tidak ada `-- +goose Down`.
  Alasan: 97 deposit trial + 100 jurnal float-in + 72 audit wallet aktif;
  drop otomatis menghancurkan jejak audit.
- TIDAK ada migration v11: perbaikan R2–R5 murni service/query layer
  (typed errors, transactional audit, decision boundary). Tidak ada
  perubahan schema (fingerprint payload dinilai tidak perlu —
  compare per-field langsung + tx_ref UNIQUE cukup).
- Rollback aplikasi = forward-compatible binary: binary lama (pre-R4)
  tetap bisa membaca tabel v10 (kolom tidak berubah); audit baru
  (`wallet:stock_deposit:*`) hanya diabaikan binary lama.
- Forward fix lebih aman daripada drop v10. Bila schema berubah di
  masa depan, buat v11 additive (kolom nullable / tabel baru), bukan
  ALTER destructive.

## 2. Upgrade Procedure (workspace)

> Gunakan hanya DB workspace/test. Full verify memang menjalankan `goose up`, sehingga wajib opt-in eksplisit. Backup dilakukan sebelum migrasi.

```bash
cd /home/ubuntu/crypto-station
git fetch https://github.com/pikipici/cry-stat.git main
git reset --hard FETCH_HEAD
export DATABASE_URL="postgres://cs:[REDACTED]@localhost:5432/cryptostation?sslmode=disable"
export VERIFY_ALLOW_DB_MIGRATION=YES

# Backup custom-format dulu (§3), kemudian generate/vet/test/build + migrate + status + smoke:
bash scripts/verify.sh --full
git push /home/ubuntu/git-repos/crypto-station.git main
```

## 3. Backup Procedure (sebelum migrate / revisi besar)

Pakai custom format supaya restore terkontrol dengan `pg_restore`:

```bash
export DBURL="postgres://cs:[REDACTED]@localhost:5432/cryptostation"
BACKUP="/tmp/cs-backup-pre-<label>-$(date +%Y%m%d-%H%M).dump"
pg_dump "$DBURL" --format=custom --no-owner --no-privileges --file "$BACKUP"
pg_restore --list "$BACKUP" >/dev/null
ls -lh "$BACKUP"
```

Restore darurat bersifat destruktif dan WAJIB approval user + maintenance window. Jangan restore ke DB aktif. Contoh aman: restore dulu ke database drill baru, verifikasi, baru putuskan cutover.

```bash
# Hentikan bot/service lebih dulu. Gunakan koneksi maintenance ke database postgres.
export MAINT_URL="postgres://cs:[REDACTED]@localhost:5432/postgres?sslmode=disable"
export RESTORE_DB="cryptostation_restore_drill"
export BACKUP="/tmp/cs-backup-pre-<label>-<ts>.dump"

# Target drill harus dedicated; drop/recreate hanya setelah approval.
psql "$MAINT_URL" -v ON_ERROR_STOP=1 -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='${RESTORE_DB}' AND pid<>pg_backend_pid();"
psql "$MAINT_URL" -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS ${RESTORE_DB};"
psql "$MAINT_URL" -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${RESTORE_DB};"
pg_restore --exit-on-error --no-owner --no-privileges --dbname="postgres://cs:[REDACTED]@localhost:5432/${RESTORE_DB}?sslmode=disable" "$BACKUP"

# Restore drill verification:
DRILL_URL="postgres://cs:[REDACTED]@localhost:5432/${RESTORE_DB}?sslmode=disable"
goose -dir ./migrations postgres "$DRILL_URL" status
psql "$DRILL_URL" -v ON_ERROR_STOP=1 -tAc "SELECT count(*) FROM admin_stock_deposits"
psql "$DRILL_URL" -v ON_ERROR_STOP=1 -tAc "SELECT count(*) FROM admin_stock_deposits WHERE status='recorded_mock' AND journal_id IS NULL"
```

Untuk mengganti database utama dari backup, lakukan prosedur drop/recreate yang sama terhadap target utama hanya setelah drill hijau, backup terbaru terverifikasi, bot berhenti, dan approval eksplisit diberikan. Jangan memakai import plain SQL ke schema existing karena dapat menghasilkan duplicate object/key atau restore parsial.

## 4. Health Checks (DB verification §10 plan)

```bash
export DBURL="postgres://cs:[REDACTED]@localhost:5432/cryptostation"
# semua float-in tepat 2 legs (harapkan 0 baris):
psql "$DBURL" -tAc "SELECT j.id, count(e.id) FROM journals j LEFT JOIN ledger_entries e ON e.journal_id=j.id WHERE j.type='float-in' GROUP BY j.id HAVING count(e.id)<>2"
# recorded tanpa journal (harapkan 0):
psql "$DBURL" -tAc "SELECT count(*) FROM admin_stock_deposits WHERE status='recorded_mock' AND journal_id IS NULL"
# counts:
psql "$DBURL" -tAc "SELECT (SELECT count(*) FROM admin_stock_deposits), (SELECT count(*) FROM journals WHERE type='float-in'), (SELECT count(*) FROM admin_audit WHERE action LIKE 'wallet:%')"
```

Hasil 2026-11-03: `97|100|72`, 0 anomali.
(Selisih 97 vs 100 = 3 jurnal float-in dari era FloatTopup lama pra-v10.)

## 5. Forward-Fix Strategy (bila v11 dibutuhkan)

1. Tambah migration additive saja (`ADD COLUMN ... NULL` / tabel baru / index).
2. Jangan `DROP COLUMN`, `ALTER TYPE`, atau backfill blocking tanpa maintenance window.
3. Binary baru harus membaca data lama (default aman untuk NULL).
4. Backup dulu (§3), verify.sh hijau, baru `goose up`.
