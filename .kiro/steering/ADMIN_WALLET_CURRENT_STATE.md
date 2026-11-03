# Crypto Station — Admin Wallet Current State

> Status: ADMIN WALLET SOURCE OF TRUTH — core v0.14/v0.14.1 tetap selesai; sistem lanjut ke Revisi V.1 correctness follow-up
> Updated: 2026-11-04
> Admin Wallet implementation head: `8c60ce1`; current code head: `fe1d906`; docs head: `c0c8700`
> DB workspace: migration v14 (`000014_payout_correctness.sql`); tabel Admin Wallet tetap dari v10
> Mode: SANDBOX / SIMULASI — bukan saldo atau transaksi on-chain
> Live production: BELUM di-deploy/restart; wajib approval eksplisit user

Dokumen ini menyatukan dan menggantikan fungsi dokumentasi dari:

- `ADMIN_WALLET_OPERATIONS_PLAN.md`
- `V014_COMPLETION_REPORT.md`
- `ADMIN_WALLET_V014_REVISION_PLAN.md`
- `V014_REVISION_COMPLETION_REPORT.md`
- `V014_MIGRATION_RUNBOOK.md`
- bagian relevan Admin Wallet pada `revisi_system_V.1.md`

File-file lama tetap dipertahankan sebagai histori. Untuk pekerjaan Admin Wallet berikutnya, baca file ini terlebih dahulu. Bila ada perbedaan status atau klaim, kondisi kode pada commit `8c60ce1` dan hasil verifikasi terbaru di dokumen ini menjadi acuan.

---

## 1. Ringkasan Eksekutif

Admin Wallet sudah berkembang dari menu `📦 Stok` sederhana menjadi panel operasional sandbox dengan kemampuan:

1. Melihat ringkasan aset BNB, ETH, dan SOL.
2. Melihat saldo jurnal, minimum float, status stok, network, hot address, dan estimasi IDR.
3. Mencatat deposit stok manual dari cold owner ke hot wallet melalui preview dan Confirm.
4. Menjamin satu `tx_ref` tidak membuat jurnal ganda.
5. Menolak penggunaan reference yang sama untuk payload/admin berbeda.
6. Membatalkan pending deposit tanpa membuat jurnal.
7. Menyimpan audit success/cancel secara atomik bersama mutasi bisnis.
8. Menampilkan feed transaksi gabungan dan detail transaksi.
9. Menjalankan callback admin melalui allowlist dan routing yang dapat diuji.
10. Menjaga seluruh UI tetap jujur dengan label `SIMULASI`.

Fondasi Admin Wallet dan Revisi V.1 correctness sudah hijau otomatis pada workspace VPS sampai `fe1d906`. Workspace pernah direstart dan data recovery diverifikasi pada correctness head sebelumnya; follow-up binary sudah build/DB-smoke tetapi belum direstart lagi tanpa approval baru. Yang masih belum selesai adalah trial tombol Telegram end-to-end menggunakan akun admin dan akun non-admin, restore drill, CF-08 historical adjustment decision, serta persetujuan terpisah untuk restart workspace berikutnya atau menyentuh live.

---

## 2. Scope yang Sudah Selesai

### 2.1 Panel admin

Panel utama menyediakan:

```text
[💼 Wallet & Aset] [🧾 Transaksi]
[📊 Spread]        [🔢 Limit]
[⏸️ Pause]         [📢 Broadcast]
[🧪 Trial]         [❌ Tutup]
```

Menu Wallet & Aset menyediakan:

```text
[📊 Ringkasan Aset]
[➕ Deposit Stok]
[⚙️ Float Minimum] [⛽ Fee]
[🏦 Alamat Hot]
[⬅️ Panel]
```

Menu `📦 Stok` lama telah diabsorpsi ke Wallet & Aset. Kompatibilitas callback lama dipertahankan sejauh yang dibutuhkan.

### 2.2 Ringkasan aset

Untuk BNB, ETH, dan SOL, admin dapat melihat:

- asset dan network;
- saldo jurnal `house:float`;
- minimum float;
- status `AMAN` atau `TIPIS`;
- hot address publik;
- estimasi nilai IDR;
- alasan ketika estimasi tidak tersedia.

Aturan final:

- Estimasi IDR hanya tampil ketika harga dan FX fresh.
- Tidak ada fallback ke harga basi atau angka hardcoded.
- Perhitungan menggunakan decimal, bukan `float64`.
- Saldo yang ditampilkan adalah saldo jurnal sandbox, bukan saldo blockchain.
- Seluruh tampilan harus menyebut `SIMULASI`.

### 2.3 Deposit stok admin

Flow final:

1. Admin membuka Wallet & Aset.
2. Admin memilih Deposit Stok.
3. Admin memilih BNB/ETH/SOL.
4. Sistem membaca network dan hot address.
5. Jika hot address kosong, flow ditolak dan admin diarahkan untuk mengaturnya.
6. Admin memasukkan units positif.
7. Admin memasukkan reference mock/network.
8. Ledger service membuat atau membuka ulang pending preview.
9. Bot menampilkan units, network, hot address, reference, saldo sebelum, dan proyeksi saldo sesudah.
10. Belum ada jurnal sebelum Confirm.
11. Confirm membuat jurnal dan audit dalam satu transaction.
12. Cancel mengubah status menjadi `cancelled` dan membuat audit dalam satu transaction, tanpa jurnal float-in.
13. Reference cancelled tetap reserved sebagai replay protection dan tidak dapat digunakan ulang.

Makna deposit sandbox:

```text
house:cold  -units
house:float +units
sum coin = 0
```

Tidak ada kaki IDR dan tidak ada klaim bahwa transfer sudah terkonfirmasi di blockchain.

### 2.4 Activity feed

Feed admin menggabungkan:

- `STOCK_IN` — deposit stok admin;
- `PAYOUT_OUT` — payout pembelian crypto user;
- `USER_DEPOSIT_IN` — deposit crypto user ketika jual.

Perilaku:

- limit 10 atau 25;
- merge dari tiga sumber;
- urut timestamp dengan tie-breaker stabil;
- detail tersedia per transaksi;
- reference/hash penuh hanya di detail;
- timestamp ditampilkan dalam WIB;
- data tetap berlabel sandbox/SIMULASI.

### 2.5 Detail transaksi

Detail minimal yang tersedia sesuai jenis transaksi:

- jenis dan internal ID;
- asset/network;
- units;
- status;
- txhash/reference;
- from/to address jika relevan;
- user ID jika relevan;
- journal ID untuk stock deposit;
- admin Telegram ID untuk stock deposit;
- waktu WIB;
- credited IDR/fee/sent units bila tersedia pada tipe terkait.

---

## 3. Arsitektur Final Saat Ini

### 3.1 Database

Migration:

- `migrations/000010_admin_stock_deposits.sql`
- Workspace DB berada pada v10.
- Migration v10 tidak memiliki Goose Down dan diperlakukan irreversible by policy karena data trial/audit harus dipertahankan.

Tabel `admin_stock_deposits` menyimpan:

- admin Telegram ID;
- asset/network;
- units decimal;
- hot address snapshot;
- `tx_ref` unique;
- status;
- journal ID;
- timestamps.

Status utama:

- `pending_confirm`;
- `recorded_mock`;
- `cancelled`;
- `failed` bila dibutuhkan oleh schema/flow berikutnya.

### 3.2 Query sqlc

Query Admin Wallet berada terutama di:

- `db/queries/admin_wallet.sql`
- `db/queries/admin.sql`

Query penting:

- create pending deposit;
- get by ID/ref;
- get for update;
- mark recorded;
- mark cancelled;
- list recent stock deposits;
- list payout/deposit admin;
- detail tiga tipe;
- exact audit lookup melalui `AdminAuditByActionTarget`.

Generated `internal/db/` dan `db/schema/` tidak di-commit. Keduanya diregenerasi di VPS.

### 3.3 Ledger service

File utama:

- `internal/ledger/stock_deposit.go`

API utama:

- `CreateAdminStockDepositPreview`
- `RecordAdminStockDeposit`
- `CancelAdminStockDeposit`

Preview state:

- `created`;
- `identical_pending`;
- `already_recorded`;
- `cancelled`;
- `conflict`;
- `unauthorized`.

Record result memiliki:

- deposit ID;
- journal ID;
- asset/network/units/ref;
- balance;
- `BalanceAvailable`;
- `AlreadyDone`.

Jika jurnal dan audit sudah commit tetapi pembacaan saldo sesudah commit gagal, operation tetap dilaporkan berhasil. UI menyatakan pencatatan berhasil dan saldo terbaru belum tersedia; operation tidak lagi dilaporkan sebagai kegagalan mutasi.

### 3.4 Idempotency dan immutable identity

`tx_ref` adalah idempotency key global.

Retry dianggap identik hanya jika seluruh identity sama:

- admin Telegram ID;
- asset;
- derived network;
- decimal units;
- trimmed destination address;
- tx reference.

Perilaku:

- Reference sama + payload identik + already recorded → return hasil lama, tanpa jurnal baru.
- Reference sama + pending identik → buka kembali preview.
- Reference sama + payload berbeda → typed conflict.
- Reference milik admin lain → unauthorized tanpa membocorkan payload lama.
- Reference cancelled → typed cancelled error, tidak boleh digunakan ulang.

Typed errors:

- `ErrStockDepositRefConflict`
- `ErrStockDepositCancelled`
- `ErrStockDepositUnauthorized`

### 3.5 Transaction dan audit

Confirm transaction:

1. Get/lock deposit.
2. Validasi immutable identity dan status.
3. Create journal `float-in`.
4. Create leg `house:cold -units`.
5. Create leg `house:float +units`.
6. Mark deposit `recorded_mock`.
7. Insert success audit.
8. Commit.

Cancel transaction:

1. Get/lock pending deposit.
2. Check owner/status.
3. Mark cancelled.
4. Insert cancel audit.
5. Commit.

Jika audit gagal sebelum commit, mutasi bisnis ikut rollback.

Audit actions:

- `wallet:stock_deposit:record`
- `wallet:stock_deposit:cancel`

Audit tidak menyimpan hot address penuh. Reference dipendekkan pada audit success.

### 3.6 Bot routing

File utama:

- `internal/bot/adminwallet.go`
- `internal/bot/wallet_sender.go`

Current design:

- callback diparse menjadi `walletCallbackDecision`;
- deny/silent/not-ready/confirm/cancel dipisahkan;
- confirm/cancel memakai `walletMutationService` boundary;
- Telegram I/O memakai `walletSender`;
- malformed confirm/cancel mendapat copy stale-button;
- business mutation tidak berjalan pada deny/malformed;
- wrapper production tetap memakai `liveWalletSender`.

Behavior test baru memverifikasi:

- deny tanpa mutasi;
- not-ready feedback;
- confirm dipanggil satu kali;
- cancel dipanggil satu kali;
- malformed callback tidak melakukan mutasi dan memberi feedback.

### 3.7 Read model

Package:

- `internal/adminwallet`

Tanggung jawab:

- asset summaries;
- fresh/stale estimation;
- merge activity;
- stable sorting;
- transaction detail.

Telegram formatting tetap berada di package bot.

---

## 4. Hardening yang Sudah Dilakukan

### 4.1 Hardening v0.14.1 awal

Perbaikan yang sudah masuk sebelum commit terkini:

- immutable identity comparison;
- typed conflict/cancelled/unauthorized errors;
- cross-admin no-leak copy;
- transactional success/cancel audit;
- concurrent final-invariant test;
- audit rollback tests;
- decision helper dan copy mapping;
- reproducible verification script awal;
- migration v10 runbook;
- correction atas klaim completion report pertama.

### 4.2 Hardening commit `8c60ce1`

Commit terbaru:

`8c60ce1 fix(wallet): harden routing preview audit and verification`

Perubahan:

1. Callback confirm/cancel menggunakan typed decision dan sender/service boundary.
2. Malformed callback mendapat feedback eksplisit.
3. Preview creation dipindahkan ke ledger service.
4. Duplicate preview semantics menggunakan immutable identity yang sama dengan Confirm.
5. Pending identik dapat membuka ulang preview.
6. Post-commit balance read tidak lagi membuat operation sukses terlihat gagal.
7. Test hook menjadi per-service dan dilindungi mutex.
8. Audit test memakai lookup exact action+target.
9. `verify.sh` diperkeras.
10. Migration runbook diperbaiki menggunakan custom-format backup dan restore drill database.
11. Ditambah behavior tests router dan preview-state tests.

Commit sudah:

- dipush ke GitHub `main`;
- direlay ke bare repo VPS;
- di-fetch/reset pada workspace VPS.

---

## 5. Verifikasi Aktual

### 5.1 Verifikasi commit terbaru di VPS

Dijalankan pada workspace VPS setelah `sqlc generate`:

- targeted tests ledger/bot/adminwallet: hijau;
- `go vet ./...`: hijau;
- `go test -count=1 ./...`: seluruh package hijau;
- `go build ./cmd/bot`: sukses;
- `bash scripts/verify.sh --no-db`: `VERIFY OK`.

Package yang terverifikasi meliputi:

- admin;
- adminwallet;
- bot;
- chain;
- ledger;
- price;
- risk;
- settings;
- trial;
- cmd/bot build.

### 5.2 Evidence historis yang tetap relevan

Sebelum commit terbaru, workspace pernah mencatat:

- DB migration v10;
- 88 PASS, 4 SKIP legacy pivot, 0 FAIL pada run revisi terdahulu;
- build binary workspace sukses;
- smoke DB exit 0;
- bot workspace dan `getMe` berhasil;
- service-level trial berhasil.

Angka test historis tidak dijadikan jumlah final setelah commit `8c60ce1`; acuan terbaru adalah full suite package hijau pada VPS.

### 5.3 Yang belum diverifikasi manual

Belum selesai:

- Telegram-tap trial menggunakan akun admin asli;
- Telegram-tap denial menggunakan akun non-admin kedua;
- restart bot workspace setelah trial lalu memastikan feed/detail tetap tersedia;
- deploy/restart live production.

---

## 6. Verification Script

File:

- `scripts/verify.sh`

Exact required versions:

- Go `1.27.3`;
- sqlc `1.29.0`;
- Goose `3.28.0`.

Mode:

```bash
bash scripts/verify.sh --no-db
```

Menjalankan:

- exact version checks;
- Goose Up-only schema extraction;
- sqlc generation/drift checks;
- gofmt check;
- vet;
- test;
- build.

Mode full:

```bash
export DATABASE_URL="[REDACTED]"
export VERIFY_ALLOW_DB_MIGRATION=YES
bash scripts/verify.sh --full
```

Tambahan full mode:

- wajib explicit opt-in;
- menolak URL yang terlihat production;
- menjalankan `goose up`;
- menampilkan migration status;
- menjalankan smoke DB dengan token kosong.

Full mode hanya boleh digunakan terhadap DB workspace/test.

---

## 7. Migration, Backup, dan Restore

### 7.1 Kebijakan migration v10

- v10 irreversible by policy.
- Tidak ada v11 untuk hardening saat ini karena perubahan cukup di service/query layer.
- Data trial, jurnal, dan audit tidak boleh di-drop.
- Perubahan schema berikutnya harus forward/additive.

### 7.2 Backup yang direkomendasikan

Gunakan custom format:

```bash
export DBURL="postgres://cs:[REDACTED]@localhost:5432/cryptostation"
BACKUP="/tmp/cs-backup-pre-<label>-$(date +%Y%m%d-%H%M).dump"
pg_dump "$DBURL" --format=custom --no-owner --no-privileges --file "$BACKUP"
pg_restore --list "$BACKUP" >/dev/null
```

Catatan: laporan lama menyebut backup plain SQL 3,1 MB. Format itu adalah evidence historis, bukan prosedur restore yang direkomendasikan saat ini. Backup berikutnya wajib memakai custom format.

### 7.3 Restore drill

Restore harus dilakukan terlebih dahulu ke dedicated database drill. Prosedur ini bersifat destruktif terhadap database target drill dan wajib memakai approval user serta maintenance window:

```bash
# Hentikan bot/service lebih dulu jika melakukan maintenance nyata.
# Gunakan koneksi maintenance ke database postgres.
export MAINT_URL="postgres://cs:[REDACTED]@localhost:5432/postgres?sslmode=disable"
export RESTORE_DB="cryptostation_restore_drill"
export BACKUP="/tmp/cs-backup-pre-<label>-<ts>.dump"

# Target wajib dedicated untuk drill.
psql "$MAINT_URL" -v ON_ERROR_STOP=1 \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='${RESTORE_DB}' AND pid<>pg_backend_pid();"
psql "$MAINT_URL" -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS ${RESTORE_DB};"
psql "$MAINT_URL" -v ON_ERROR_STOP=1 \
  -c "CREATE DATABASE ${RESTORE_DB};"

pg_restore --exit-on-error --no-owner --no-privileges \
  --dbname="postgres://cs:[REDACTED]@localhost:5432/${RESTORE_DB}?sslmode=disable" \
  "$BACKUP"

# Verifikasi hasil restore:
export DRILL_URL="postgres://cs:[REDACTED]@localhost:5432/${RESTORE_DB}?sslmode=disable"
goose -dir ./migrations postgres "$DRILL_URL" status
psql "$DRILL_URL" -v ON_ERROR_STOP=1 -tAc \
  "SELECT count(*) FROM admin_stock_deposits"
psql "$DRILL_URL" -v ON_ERROR_STOP=1 -tAc \
  "SELECT count(*) FROM admin_stock_deposits WHERE status='recorded_mock' AND journal_id IS NULL"
psql "$DRILL_URL" -v ON_ERROR_STOP=1 -tAc \
  "SELECT j.id, count(e.id) FROM journals j LEFT JOIN ledger_entries e ON e.journal_id=j.id WHERE j.type='float-in' GROUP BY j.id HAVING count(e.id)<>2"
```

Hasil yang diwajibkan:

- Goose status menunjukkan migration v10 sesuai backup.
- Query `recorded_mock` tanpa journal menghasilkan `0`.
- Query float-in dengan jumlah leg bukan dua menghasilkan `0` baris.
- Feed/detail dapat dibaca dari aplikasi yang diarahkan sementara ke drill DB bila diperlukan.

Untuk mengganti database utama dari backup, ulangi prosedur drop/recreate terhadap target utama hanya setelah restore drill hijau, backup terbaru terverifikasi, bot dihentikan, dan approval eksplisit diberikan.

Dilarang mengimpor plain SQL ke schema existing karena dapat menyebabkan duplicate objects, duplicate keys, atau restore parsial.

### 7.4 Health checks

```sql
-- Float-in wajib tepat dua legs:
SELECT j.id, count(e.id)
FROM journals j
LEFT JOIN ledger_entries e ON e.journal_id=j.id
WHERE j.type='float-in'
GROUP BY j.id
HAVING count(e.id)<>2;

-- Recorded stock deposit wajib punya journal:
SELECT count(*)
FROM admin_stock_deposits
WHERE status='recorded_mock' AND journal_id IS NULL;
```

Hasil historis 2026-11-03 sebelum commit terbaru:

- counts `97|100|72`;
- 0 anomali;
- selisih deposits/journals berasal dari tiga jurnal FloatTopup lama pra-v10.

---

## 8. Commit Timeline

### 8.1 Implementasi v0.14

- `94e620f` — migration 000010 + query sqlc.
- `24afdfd` — idempotent stock deposit ledger + tests.
- `b9b1fe5` — asset summary + activity feed.
- `57c597b` — pgtype timestamp fix.
- `a2832cb` — unused import fix.
- `2d43306` — bot wallet flow + views.
- `fd4cf7f` — project documentation v0.14.
- `cc1557f` — completion report.

### 8.2 Hardening v0.14.1 awal

- `dc781d6` — typed conflict + concurrent tests.
- `77fd98a` — transactional audit + fault seam.
- `89bb18e` — unique test reference.
- `0006580` — wallet decision boundary tests.
- `411473e` — conflict error receiver fix.
- `bffe495` — verify script awal.
- `eb3a415` — gofmt.
- `40f2845` — migration runbook awal.
- `69df191` — report correction.
- `0ea9532` — Bash requirement note.
- `9a417d2` — R9 done, R8 pending.
- `9313901` — revision completion report.

### 8.3 Hardening terkini

- `8c60ce1` — routing, preview service, post-commit result, exact audit query, verify script, restore runbook, tests.

Admin Wallet implementation source head: `8c60ce1`. Current repository code head setelah Revisi V.1 correctness follow-up: `fe1d906`; current docs head sebelum sinkronisasi ini: `c0c8700`.

---

## 9. File Implementasi Utama

Database/migration:

- `migrations/000010_admin_stock_deposits.sql`
- `db/queries/admin_wallet.sql`
- `db/queries/admin.sql`

Ledger:

- `internal/ledger/stock_deposit.go`
- `internal/ledger/stock_deposit_test.go`
- `internal/ledger/stock_deposit_conflict_test.go`
- `internal/ledger/stock_deposit_audit_test.go`
- `internal/ledger/stock_deposit_preview_test.go`

Read model:

- `internal/adminwallet/service.go`
- `internal/adminwallet/activity.go`
- corresponding tests.

Bot:

- `internal/bot/adminwallet.go`
- `internal/bot/adminwallet_format.go`
- `internal/bot/wallet_sender.go`
- `internal/bot/wallet_router_test.go`
- `internal/bot/adminwallet_test.go`
- `internal/bot/keyboard.go`
- `internal/bot/session.go`
- `internal/bot/adminflow.go`

Wiring/tooling:

- `cmd/bot/main.go`
- `scripts/verify.sh`

---

## 10. Acceptance Status Aktual

### Selesai dan terverifikasi otomatis

- [x] Panel Wallet & Aset tersedia.
- [x] Ringkasan BNB/ETH/SOL tersedia.
- [x] Estimasi IDR fail-closed.
- [x] Deposit stok memakai preview + Confirm.
- [x] Tidak ada jurnal sebelum Confirm.
- [x] Jurnal cold→float dua kaki dan sum nol.
- [x] Retry identik tidak membuat jurnal kedua.
- [x] Same ref + payload/admin berbeda ditolak.
- [x] Cancelled ref tidak dapat digunakan ulang.
- [x] Cross-admin tidak menerima payload lama.
- [x] Success audit berada dalam transaction yang sama.
- [x] Cancel audit berada dalam transaction yang sama.
- [x] Post-commit balance read failure tidak membuat operation terlihat gagal.
- [x] Feed gabungan dan detail tersedia.
- [x] Callback malformed mendapat feedback.
- [x] Router behavior confirm/cancel/deny/not-ready diuji.
- [x] Preview duplicate states diuji.
- [x] Full suite Go hijau di VPS.
- [x] Vet hijau di VPS.
- [x] Build hijau di VPS.
- [x] Verify no-DB mode hijau.
- [x] Commit dan dual push selesai.
- [x] Live belum disentuh.

### Selesai tetapi evidence bersifat historis/report

- [x] DB v10 diterapkan pada workspace.
- [x] Service-level trial pernah dijalankan.
- [x] DB health check historis menunjukkan 0 anomali.
- [x] Bot workspace pernah hidup dan `getMe` berhasil.

### Belum selesai

- [ ] Trial Telegram admin melalui seluruh tombol asli.
- [ ] Trial callback/detail menggunakan akun non-admin kedua.
- [ ] Verifikasi deny audit setelah trial non-admin.
- [ ] Restart workspace setelah trial dan cek persistence feed/detail.
- [ ] Restore drill custom-format aktual.
- [ ] Approval deploy/restart live.

---

## 11. Trial Telegram yang Masih Wajib

### 11.1 Admin

1. `/admin` → panel menampilkan Wallet & Aset dan Transaksi.
2. Buka ringkasan; cek BNB/ETH/SOL dan label SIMULASI.
3. Pastikan harga basi tidak menghasilkan estimasi palsu.
4. Deposit Stok → SOL.
5. Input `0` dan `abc`; keduanya harus ditolak dan bisa retry.
6. Input units valid.
7. Input reference baru; preview harus tampil.
8. Cancel; saldo dan journal tidak berubah.
9. Coba reference cancelled; harus ditolak dengan copy khusus.
10. Ulangi dengan reference baru lalu Confirm.
11. Double-tap/Confirm ulang; saldo hanya naik sekali.
12. Retry reference identik; hasil lama ditampilkan.
13. Reference sama dengan units berbeda; harus conflict.
14. Buka feed; STOCK_IN terbaru terlihat.
15. Buka detail; journal/admin/ref/address/status/waktu terlihat.
16. Toggle feed 25.
17. Lakukan beli mock; PAYOUT_OUT muncul.
18. Lakukan jual mock; USER_DEPOSIT_IN muncul.
19. Cek Spread/Limit/Pause/Trial/Broadcast tidak rusak.
20. Cek user `/beli`, `/jual`, `/status`, `/harga`, `/saldo`, `/riwayat` tidak rusak.

### 11.2 Non-admin

1. Coba akses callback/menu admin.
2. Coba menebak detail STOCK_IN.
3. Pastikan mendapat denial dan tidak ada data bocor.
4. Verifikasi `admin_audit` mencatat deny untuk Telegram ID tersebut.

### 11.3 Restart recovery

Setelah trial:

1. Restart hanya bot workspace/sandbox jika user menyetujui.
2. Buka kembali feed.
3. Buka detail deposit trial.
4. Pastikan data tetap tersedia.

Trial ini bukan izin deploy/restart live production.

---

## 12. Pending Technical Hardening Non-Blocker

Berikut bukan blocker utama fitur sandbox, tetapi masih layak backlog:

1. Membuat race test transaction overlap yang benar-benar deterministik di titik row-lock/unique-conflict, bukan hanya final invariant dua goroutine.
2. Menguji kegagalan aktual `InsertAdminAudit` melalui isolated DB trigger/repository seam, bukan hanya hook sebelum audit.
3. Menambah commit-failure simulation jika test infrastructure mendukung.
4. Menambah fixture test khusus parser Goose Up/Down pada verify script.
5. Menjalankan restore drill custom-format dan mencatat hasilnya.
6. Menambah telemetry ketika deny audit gagal; access denial sendiri harus tetap fail-closed.
7. Membersihkan generated/untracked artefak VPS (`cs-bot`, `db/schema`, `internal/db`) hanya jika workflow deployment mengizinkan; jangan hapus tanpa memastikan generate pipeline.

---

## 13. Non-Goals dan Larangan

Belum boleh dilakukan dalam scope Admin Wallet sekarang:

- membaca saldo on-chain;
- watcher blockchain;
- auto-detect deposit admin;
- validasi txhash ke node/explorer;
- private key/seed;
- broadcast chain asli;
- chain reconciliation;
- payment gateway/QRIS/bank/withdraw real;
- mengklaim reference mock sebagai transaksi chain valid;
- deploy/restart live tanpa persetujuan eksplisit.

Aturan keamanan:

- Jangan masukkan seed/private key ke repo, DB, log, audit, atau Telegram.
- Jangan menambah saldo hanya dari input message tanpa Confirm.
- Jangan membuat jurnal kedua pada retry.
- Jangan memakai `float64` untuk coin/money.
- Jangan query SQL langsung dari handler untuk business mutation baru.
- Jangan menghapus cancelled rows untuk membersihkan test.
- Jangan mengizinkan reuse `tx_ref` tanpa replay analysis.
- Jangan mengklaim manual trial selesai sebelum benar-benar dilakukan.

---

## 14. Backlog Fase 10 / Stage 2

Setelah gate Stage 2 lengkap, UX yang sama dapat diperluas menjadi real-chain:

1. Chain adapter membaca native hot-wallet balance.
2. Ringkasan menampilkan saldo on-chain, saldo jurnal, dan selisih.
3. Watcher mendeteksi deposit admin.
4. Deposit lifecycle menjadi detected → confirming → confirmed/failed/unknown.
5. Jurnal float-in dibuat setelah confirmation threshold.
6. Reconcile chain vs ledger dapat mem-pause coin.
7. Explorer deep-link per txhash.
8. Alert unknown payout/deposit, node down, dan gas tipis.

Semua ini tetap dilarang sebelum:

- hot wallet baru terpisah siap;
- node provider dipilih;
- fee dan float minimum nyata ditentukan;
- security key environment siap;
- sandbox observation selesai;
- user memberi approval masuk Stage 2.

---

## 15. Next Steps

Urutan rekomendasi:

1. Jalankan trial Telegram admin.
2. Jalankan trial akun non-admin.
3. Verifikasi audit rows hasil trial.
4. Bila disetujui, restart bot workspace lalu cek persistence.
5. Jalankan restore drill custom-format terhadap dedicated drill DB pada maintenance window yang disetujui.
6. Perbarui dokumen ini dengan hasil R8 dan restore drill.
7. Minta approval eksplisit bila ingin deploy/restart live.
8. Setelah Admin Wallet selesai secara manual, kembali ke roadmap `revisi_system_V.1.md` untuk hardening flow user/topup/buy sandbox.

---

## 16. Status Singkat untuk AI Berikutnya

```text
Project: Crypto Station
Feature: Admin Wallet v0.14 + v0.14.1 hardening
Head: 8c60ce1
DB: v10
Mode: sandbox/SIMULASI
Automated VPS: test/vet/build hijau
Verify: --no-db VERIFY OK
Core feature: implemented
Hardening latest: implemented and pushed
Pending: Telegram admin+non-admin trial, restore drill, live approval
Live: jangan disentuh tanpa approval eksplisit
Primary doc: .kiro/steering/ADMIN_WALLET_CURRENT_STATE.md
```
