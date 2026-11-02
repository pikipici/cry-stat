# Crypto Station — Admin Wallet v0.14 Revision Plan

> Status: IN PROGRESS — R1–R7 + verify DONE (head 40f2845), R8 trial menunggu user, R9 report pending R8
> Target: v0.14.1-admin-wallet-hardening
> Dibuat: 2026-11-02
> Basis audit: HEAD `cc1557f`, `V014_COMPLETION_REPORT.md`, dan kode aktual
> Scope: menutup gap verifikasi dan hardening implementasi v0.14 tanpa menambah fitur on-chain
> Live: dilarang deploy/restart tanpa persetujuan eksplisit user

---

## 0. Tujuan

Plan ini merevisi hasil `ADMIN_WALLET_OPERATIONS_PLAN.md` yang sudah diimplementasikan oleh AI lain. Fitur utama v0.14 tetap dipertahankan; revisi hanya menutup gap berikut:

1. Idempotency `tx_ref` harus aman terhadap payload/admin berbeda.
2. Audit success/cancel/deny harus durable atau kegagalannya terlihat jelas.
3. Coverage harus benar-benar membuktikan rollback, concurrency, allowlist, cancel, dan double-confirm.
4. Build/test harus reproducible dari clean checkout.
5. Semantik cancel dan penggunaan ulang reference harus diputuskan secara eksplisit.
6. Migration rollback/operational recovery harus terdokumentasi.
7. Trial Telegram asli 18 langkah harus diselesaikan sebelum status full DONE.
8. Completion report harus dikoreksi berdasarkan bukti aktual, bukan klaim.

Plan ini bukan redesign fitur dan bukan Fase 10. Tidak ada saldo on-chain, watcher, node, private key, broadcast chain, gateway, atau fiat real.

---

## 1. Kondisi Aktual yang Dipertahankan

Bagian berikut sudah benar secara kode dan tidak perlu ditulis ulang:

- Panel `💼 Wallet & Aset`.
- Panel `🧾 Transaksi`.
- Ringkasan BNB/ETH/SOL.
- Estimasi IDR fail-closed.
- Preview sebelum confirm deposit stok.
- Jurnal `house:cold → house:float` dalam satu DB transaction.
- Sequential duplicate confirm menghasilkan satu jurnal.
- Ownership check pada confirm/cancel.
- Unified activity feed `STOCK_IN`, `PAYOUT_OUT`, `USER_DEPOSIT_IN`.
- Detail tiga tipe transaksi.
- Label `SIMULASI`.
- Live production belum disentuh.

Executor dilarang membongkar area tersebut kecuali diperlukan langsung untuk acceptance revisi ini.

---

## 2. Temuan Audit yang Harus Ditutup

### R1 — Idempotency key tidak terikat immutable payload

Severity: High

Evidence:

- `migrations/000010_admin_stock_deposits.sql`: `tx_ref` unique global.
- `internal/ledger/stock_deposit.go`: retry lookup hanya memakai `TxRef`.
- Record existing tidak dibandingkan dengan admin, asset, units, network, atau address.

Risiko:

- Reference sama dengan payload berbeda dianggap sukses terhadap transaksi lama.
- Caller lain dapat menerima metadata transaksi yang bukan request-nya.
- “Idempotent” hanya benar untuk asumsi retry identik, belum enforced.

Expected behavior:

- Key sama + payload immutable identik → return result lama, `AlreadyDone=true`.
- Key sama + salah satu field berbeda → typed conflict, tanpa jurnal/mutasi.
- Conflict tidak membocorkan detail transaksi lama ke caller yang tidak berhak.

Immutable identity minimum:

- `admin_telegram_id`;
- `asset`;
- `network`;
- canonical `units`;
- normalized `to_address`;
- `tx_ref`.

### R2 — Audit tidak durable

Severity: Medium–High

Evidence:

- Audit success/cancel/deny dipanggil setelah mutasi dan error dibuang.
- Mutasi dapat sukses saat audit gagal.

Expected behavior:

Pilih satu strategi dan dokumentasikan:

A. Recommended: transactional audit untuk success/cancel.
- Ledger/service menerima audit metadata.
- Deposit state+journal+audit ditulis dalam transaction yang sama.
- Cancel state+audit juga atomik.

B. Alternative: durable outbox.
- Mutasi menulis audit outbox dalam transaction yang sama.
- Worker membuat `admin_audit` exactly once.
- UI/report dapat menunjukkan audit pending/failed.

Untuk deny yang tidak berada dalam transaction bisnis:

- Error audit tidak boleh dibuang diam-diam.
- Log structured + metric/alert minimum.
- Handler tetap menolak akses walaupun audit storage sedang gagal.

### R3 — Automated coverage tidak sesuai klaim report

Severity: Medium

Missing coverage:

- fault-injection rollback;
- concurrent double-confirm;
- same ref different payload/admin;
- handler-level non-admin callback;
- handler-level admin message/session;
- deny audit persisted/failure path;
- preview no mutation;
- cancel no journal;
- audit failure semantics.

Expected behavior:

Test matrix report hanya boleh ditandai centang jika test benar-benar ada dan dijalankan.

### R4 — Clean checkout belum reproducible

Severity: Medium–High

Evidence:

- `internal/db` generated tidak tracked.
- `db/schema` generated tidak tersedia dari clean checkout.
- `sqlc generate` memerlukan migration baru disalin manual ke schema.

Expected behavior:

Satu command deterministic dari clean checkout harus:

1. membentuk schema input sqlc dari migrations;
2. menjalankan sqlc generate;
3. menjalankan gofmt/check;
4. menjalankan test/vet/build.

Tidak boleh bergantung pada file generated sisa run sebelumnya atau langkah copy manual yang hanya diketahui operator.

### R5 — Semantik cancel reference belum eksplisit

Severity: Low–Medium

Current behavior:

- Preview membuat pending row.
- Cancel mengubah status ke `cancelled`.
- `tx_ref` tetap reserved selamanya karena unique global.

Keputusan yang direkomendasikan:

- Pertahankan reservation permanen untuk audit dan replay protection.
- Copy UI harus menjelaskan reference sudah pernah dipakai/dibatalkan.
- Jangan menyebut cancel “menghapus” deposit.
- Retry ref cancelled harus return typed `ErrReferenceCancelled`, bukan generic duplicate.

Alternatif reuse ref hanya boleh dipilih jika ada business reason kuat dan tetap aman terhadap replay. Default plan: tidak boleh reuse.

### R6 — Migration tidak punya rollback operational story

Severity: Low–Medium

Expected behavior:

Pilih:

- Tambahkan `-- +goose Down` yang aman bila belum ada data penting; atau
- Tandai migration irreversible dan buat documented forward-fix/backup-restore procedure.

Karena v10 sudah digunakan workspace dan memiliki data trial, rekomendasi utama: jangan drop data secara otomatis. Dokumentasikan migration irreversible + backup/restore + forward migration strategy.

### R7 — Trial Telegram asli belum selesai

Severity: Completion blocker

Service-level verification tidak menggantikan real callback/message/session interaction. Trial harus dilakukan melalui bot Telegram workspace dan akun non-admin kedua.

---

## 3. Desain Idempotency yang Benar

### 3.1 Typed result/error

Tambahkan domain errors:

```go
var (
    ErrStockDepositRefConflict  = errors.New("stock deposit reference conflict")
    ErrStockDepositCancelled    = errors.New("stock deposit reference cancelled")
    ErrStockDepositUnauthorized = errors.New("stock deposit unauthorized")
)
```

Jangan parse string error di handler. Gunakan `errors.Is` atau typed error struct.

Recommended conflict detail internal:

```go
type StockDepositConflictError struct {
    Field string
}
```

User-facing copy tidak perlu membocorkan nilai lama.

### 3.2 Canonicalization

Sebelum compare/insert:

- Asset uppercase.
- Network derived dari asset atau divalidasi mapping tetap.
- Units parsed dengan decimal dan disimpan canonical 18-decimal-compatible form.
- EVM address normalized sesuai kebijakan project; jangan lowercase Solana.
- Trim whitespace reference/address.
- Admin ID harus nonzero dan allowlisted di orchestration boundary.

### 3.3 Conflict algorithm

Dalam transaction:

1. Lookup/insert by `tx_ref`.
2. Lock row `FOR UPDATE`.
3. Compare immutable identity.
4. Mismatch → rollback + typed conflict.
5. Status `recorded_mock` + identical payload → return original result.
6. Status `pending_confirm` + identical payload → lanjut confirm.
7. Status `cancelled` → `ErrStockDepositCancelled`.
8. Status `failed` → jangan retry otomatis tanpa explicit recovery policy.

### 3.4 Authorization

Walaupun handler sudah memeriksa owner, service harus defense-in-depth:

- Existing row `admin_telegram_id` berbeda → unauthorized/conflict.
- Jangan return units/address/journal ID transaksi lama kepada caller berbeda.

### 3.5 DB support

`tx_ref UNIQUE` tetap dipertahankan.

Opsional namun direkomendasikan:

- Tambah normalized hash/fingerprint immutable payload agar compare sederhana dan audit-friendly.
- Jika memakai fingerprint, gunakan canonical serialization; jangan hash string concatenation ambigu.
- Jangan menjadikan fingerprint pengganti per-field validation pada awal implementasi.

---

## 4. Desain Audit Durability

### 4.1 Recommended transaction boundary

Buat service operation yang menerima actor/action metadata dan menulis audit memakai query object transaction yang sama.

Conceptual API:

```go
type RecordAdminStockDepositInput struct {
    AdminTelegramID int64
    Asset           string
    Units           decimal.Decimal
    ToAddress       string
    TxRef           string
}
```

Service transaction:

1. Validate/lock deposit.
2. Create journal + entries.
3. Mark deposit recorded.
4. Insert `admin_audit` success dengan before/after sanitized.
5. Commit.

Jika audit insert gagal, seluruh operation rollback.

Cancel transaction:

1. Lock pending deposit owner-scoped.
2. Transition ke cancelled.
3. Insert cancel audit.
4. Commit.

### 4.2 Audit content

Success:

- action: `wallet:stock_deposit:record`;
- actor admin Telegram ID;
- target deposit ID;
- before: status pending + balance summary bila tersedia;
- after: recorded_mock + asset + units + journal ID + ref pendek/sanitized.

Cancel:

- action: `wallet:stock_deposit:cancel`;
- target deposit ID;
- before pending;
- after cancelled.

Dilarang menyimpan:

- private key/seed;
- secret;
- raw sensitive payload;
- full address bila audit access dianggap luas; gunakan policy konsisten.

### 4.3 Audit failure tests

Inject audit insert failure dan pastikan:

- journal tidak commit;
- deposit tidak berubah recorded/cancelled;
- balance tidak berubah;
- retry setelah failure dapat sukses sekali.

---

## 5. Test Harness yang Wajib Ditambahkan

### 5.1 Ledger/service tests

Tambahkan:

1. `TestRecordStockDepositRetryIdenticalReturnsAlreadyDone`
2. `TestRecordStockDepositSameRefDifferentAdminConflicts`
3. `TestRecordStockDepositSameRefDifferentAssetConflicts`
4. `TestRecordStockDepositSameRefDifferentUnitsConflicts`
5. `TestRecordStockDepositSameRefDifferentAddressConflicts`
6. `TestRecordStockDepositCancelledRefCannotConfirm`
7. `TestRecordStockDepositConcurrentConfirmOneJournal`
8. `TestRecordStockDepositAuditFailureRollsBackAll`
9. `TestCancelStockDepositAuditFailureRollsBack`
10. `TestRecordStockDepositEighteenDecimalCanonicalRetry`

Concurrency test:

- Gunakan dua goroutine dengan start barrier.
- Keduanya confirm deposit yang sama.
- Hasil yang diperbolehkan: satu normal success, satu AlreadyDone.
- DB assertions: satu journal, dua legs, satu audit success, balance naik sekali.

### 5.2 Fault injection

Jangan mengandalkan DB shared error random.

Gunakan salah satu:

- repository/query interface fake yang gagal pada titik tertentu;
- DB trigger sementara dalam isolated test schema;
- transaction test hook khusus package internal yang tidak masuk production behavior.

Failure points minimum:

- create journal;
- first leg;
- second leg;
- update deposit status;
- insert audit;
- commit.

Setiap failure harus meninggalkan invariant awal.

### 5.3 Bot handler tests

Test handler nyata dengan fake Bot API/client boundary atau HTTP test transport:

- non-admin wallet callback ditolak;
- deny audit dipanggil;
- admin callback diterima;
- non-admin message saat wallet session ditolak;
- preview hanya membuat pending row, bukan journal;
- cancel callback tidak membuat journal;
- confirm callback exactly once;
- stale/malformed callback memberi feedback;
- callback deposit milik admin lain ditolak;
- SIMULASI muncul pada output.

Pure helper tests yang hanya memeriksa callback string tidak boleh dianggap handler security coverage.

### 5.4 Integration isolation

- Gunakan DB/schema isolated per test run.
- Apply migrations dari nol.
- Jangan bergantung pada journal ID atau saldo trial yang sudah ada.
- Cleanup deterministic.
- Jangan menjalankan integration tests paralel bila masih memakai DB shared.

---

## 6. Reproducible sqlc/Build Pipeline

### 6.1 Target command

Sediakan satu script, contoh:

```text
scripts/verify.sh
```

Tanggung jawab:

1. Validasi tool version.
2. Rebuild `db/schema` dari ordered migrations secara deterministic.
3. `sqlc generate`.
4. `gofmt`/format check.
5. `go test -count=1 ./...`.
6. `go vet ./...`.
7. `go build -o /tmp/cs-bot ./cmd/bot`.
8. Optional smoke DB jika `DATABASE_URL` tersedia.

Windows local tidak wajib menjalankan runtime, tetapi script harus bekerja di cbt-vps clean clone.

### 6.2 Schema generation

Jangan lagi melakukan manual:

```text
cp migrations/000010 db/schema/
```

Buat deterministic generation:

- sort migration numeric;
- ambil bagian Goose Up yang relevan;
- atau arahkan sqlc ke migration files jika tool/version mendukung dengan aman;
- clear output schema sebelum regenerate agar migration deleted/renamed tidak tertinggal.

### 6.3 Version pinning

Dokumentasikan/pin:

- Go;
- sqlc;
- goose.

Fail dengan pesan jelas jika versi tidak sesuai minimum.

### 6.4 Clean checkout acceptance

Di VPS temporary directory:

1. Clone/fetch commit target.
2. Tidak menyalin generated artifacts dari workspace lama.
3. Jalankan verify script.
4. Pastikan test/vet/build selesai.
5. Simpan command + exit code dalam completion report.

---

## 7. Migration dan Data Compatibility

### 7.1 Jangan merusak trial data

Workspace sudah memiliki deposit trial dan journal. Revision migration harus additive/forward-only.

Dilarang:

- drop table v10;
- rewrite journal existing;
- menghapus cancelled rows;
- me-reset sequence;
- menggunakan destructive cleanup tanpa explicit backup.

### 7.2 Optional revision migration

Hanya buat migration v11 bila schema benar-benar perlu berubah, misalnya:

- payload fingerprint;
- audit outbox;
- status/index constraint baru.

Jika perbaikan cukup di service/query, jangan membuat migration kosong.

### 7.3 Backup dan rollback

Sebelum migration revision di workspace:

- backup DB;
- catat migration version;
- verifikasi restore command/runbook;
- jangan menaruh credentials dalam report.

Untuk v10, dokumentasikan:

- migration irreversible karena data audit/trial dipertahankan;
- rollback aplikasi dilakukan dengan forward-compatible binary atau restore backup;
- v11 forward fix lebih aman daripada drop v10.

---

## 8. UX Revisi

### 8.1 Duplicate identical ref

Copy:

```text
Deposit stok ini sudah pernah dicatat.
Jurnal: #...
Saldo saat ini: ...
```

### 8.2 Ref conflict

Copy:

```text
Reference ini sudah pernah digunakan untuk data lain.
Gunakan reference transaksi yang benar atau hubungi operator.
```

Jangan tampilkan payload transaksi lama.

### 8.3 Cancelled ref

Copy:

```text
Reference ini sebelumnya sudah dibatalkan dan tidak dapat digunakan ulang.
Buat reference baru untuk pencatatan baru.
```

### 8.4 Audit/system error

Jika operation rollback karena audit/storage failure:

```text
Deposit belum dicatat karena sistem gagal menyimpan jejak audit.
Saldo tidak berubah. Coba lagi.
```

Pastikan copy tersebut benar berdasarkan DB assertions.

---

## 9. Urutan Implementasi Hemat Cost

### Task R1 — Baseline dan bukti reproducibility

1. Catat HEAD/status.
2. Buat clean checkout VPS.
3. Reproduce generate/test gap.
4. Simpan evidence.
5. Jangan memperbaiki fitur sebelum baseline jelas.

Deliverable:

- baseline commands dan hasil;
- daftar generated dependency.

### Task R2 — Idempotency conflict TDD

1. Tulis failing tests same-ref/different-payload.
2. Tambah canonical comparison + typed errors.
3. Pertahankan identical retry behavior.
4. Jalankan test targeted dan suite.

Gate:

- seluruh immutable mismatch ditolak;
- tidak ada data leak;
- identical retry satu jurnal.

### Task R3 — Concurrent confirm TDD

1. Tulis two-goroutine test.
2. Verifikasi row lock/unique behavior.
3. Fix hanya jika test menemukan race.
4. Assert satu journal/audit/balance delta.

### Task R4 — Transactional audit

1. Refactor success/cancel audit ke transaction boundary.
2. Tambah failure injection.
3. Pastikan rollback penuh.
4. Jangan mengubah deny security behavior.

Gate:

- success/cancel tidak mungkin commit tanpa audit.

### Task R5 — Handler-level security tests

1. Tambah fake Telegram transport/boundary.
2. Test admin/non-admin/stale/malformed.
3. Test preview/cancel/confirm side effects.
4. Pastikan callback spinner selalu dijawab.

### Task R6 — Reproducible generate/verify script

1. Buat schema generation deterministic.
2. Pin/check tools.
3. Jalankan clean checkout verification.
4. Dokumentasikan satu command.

### Task R7 — Migration/runbook hardening

1. Putuskan perlu/tidak v11.
2. Backup workspace DB sebelum migrate.
3. Dokumentasikan irreversible v10.
4. Verify upgrade/forward-fix.

### Task R8 — Telegram manual trial

Jalankan 18 langkah dari report melalui tombol asli, bukan service call.

Tambahan wajib:

- same ref different units;
- same ref different admin/account;
- double-tap cepat;
- callback stale;
- audit row verification;
- restart workspace dan buka kembali feed/detail.

### Task R9 — Correct completion report

Perbarui `V014_COMPLETION_REPORT.md`:

- bedakan automated, manual, dan report-only evidence;
- koreksi jumlah/jenis tests;
- tambahkan commit hardening;
- tandai trial Telegram setelah benar-benar dilakukan;
- jangan centang live deploy tanpa approval.

---

## 10. Verification Commands di VPS

Command exact menyesuaikan env existing dan wajib meredaksi credential:

```bash
git status --short
git log -1 --oneline
./scripts/verify.sh
```

Targeted tests contoh:

```bash
go test -count=1 ./internal/ledger -run 'TestRecordStockDeposit'
go test -count=1 ./internal/bot -run 'TestAdminWallet'
go test -count=1 ./internal/adminwallet
```

Full gates:

```bash
go test -count=1 ./...
go vet ./...
go build -o /tmp/cs-bot-v0141 ./cmd/bot
```

DB verification:

- migration version expected;
- journal sums zero;
- one journal per stock deposit;
- one success audit per recorded deposit;
- cancelled row has zero journal;
- conflict requests create no new rows/journals.

Jangan menampilkan `DATABASE_URL`, bot token, private address secret, atau credential dalam report.

---

## 11. Telegram Trial Checklist

Admin account:

- [ ] Buka Admin → Wallet & Aset.
- [ ] Ringkasan BNB/ETH/SOL tampil.
- [ ] Harga stale tidak mengarang estimasi.
- [ ] Deposit stok SOL.
- [ ] Units invalid dapat retry.
- [ ] Ref valid → preview.
- [ ] Cancel → saldo/journal tidak berubah.
- [ ] Ref cancelled tidak dapat digunakan ulang dan copy jelas.
- [ ] Ref baru → Confirm.
- [ ] Double-tap cepat → satu jurnal.
- [ ] Retry identical → AlreadyDone.
- [ ] Same ref different units → conflict.
- [ ] Feed menampilkan STOCK_IN.
- [ ] Detail menampilkan journal/admin/ref/address/status.
- [ ] Restart bot workspace.
- [ ] Feed/detail tetap dapat dibuka.

Non-admin account:

- [ ] Tombol/callback admin ditolak.
- [ ] Session/input admin ditolak.
- [ ] Deny audit benar-benar tersimpan.
- [ ] Tidak ada detail transaksi admin bocor.

Regression:

- [ ] Spread tetap jalan.
- [ ] Limit tetap jalan.
- [ ] Pause tetap jalan.
- [ ] Trial report tetap jalan.
- [ ] Beli/jual/status user tidak rusak.

---

## 12. Acceptance Criteria Final v0.14.1

Revisi dianggap DONE hanya jika:

- [ ] Identical retry menghasilkan satu journal dan result lama.
- [ ] Same ref berbeda admin/asset/units/address ditolak typed conflict.
- [ ] Cancelled ref memiliki semantics dan copy yang jelas.
- [ ] Concurrent confirm terbukti satu journal.
- [ ] Fault injection membuktikan rollback journal/legs/status/audit.
- [ ] Success dan cancel audit atomik/durable.
- [ ] Deny audit failure tidak melemahkan access denial dan menghasilkan telemetry.
- [ ] Handler-level allowlist tests tersedia.
- [ ] Preview/cancel/confirm side effects diuji pada handler level.
- [ ] Clean checkout dapat generate/test/vet/build dengan satu command.
- [ ] Migration/rollback-forward strategy terdokumentasi.
- [ ] Full suite hijau di VPS.
- [ ] Telegram trial admin + non-admin selesai.
- [ ] Completion report dikoreksi dengan evidence aktual.
- [ ] Live tetap belum disentuh tanpa approval eksplisit.

---

## 13. Non-Goals dan Larangan

Tidak masuk revisi:

- saldo on-chain;
- watcher blockchain;
- node provider;
- validasi txhash ke explorer/node;
- private key/seed;
- payout chain asli;
- payment gateway/QRIS/bank real;
- perubahan ledger buy/deposit user di luar kebutuhan regression;
- deploy/restart live tanpa izin.

Larangan:

- Jangan menghapus row cancelled untuk “membersihkan” test.
- Jangan mengizinkan reuse tx_ref tanpa replay analysis.
- Jangan menyelesaikan audit dengan sekadar `_ = Audit(...)`.
- Jangan mengklaim concurrency tested melalui sequential retry test.
- Jangan mengklaim handler tested melalui helper string test.
- Jangan memakai DB trial shared tanpa isolation untuk fault tests.
- Jangan menulis secret ke report.
- Jangan mengubah `revisi_system_V.1.md` menjadi status done; dokumen itu punya scope lebih besar.

---

## 14. File Minimum untuk Executor

Baca berurutan:

1. `.kiro/steering/ADMIN_WALLET_V014_REVISION_PLAN.md`
2. `.kiro/steering/V014_COMPLETION_REPORT.md`
3. `internal/ledger/stock_deposit.go`
4. `internal/ledger/stock_deposit_test.go`
5. `internal/bot/adminwallet.go`
6. `internal/bot/adminwallet_test.go`
7. `internal/bot/adminflow.go`
8. `internal/adminwallet/service.go`
9. `internal/adminwallet/activity.go`
10. `db/queries/admin_wallet.sql`
11. `migrations/000010_admin_stock_deposits.sql`
12. `sqlc.yaml`
13. `internal/admin/admin.go`
14. Plan original hanya pada acceptance section yang relevan.

Jangan membaca seluruh roadmap kecuali menemukan kontradiksi produk.

---

## 15. Progress Checklist

- [x] R1 Baseline clean-checkout evidence (gap reproduced: no internal/db+db/schema; tool go1.27.3/sqlc1.29.0/goose3.28.0)
- [x] R2 Idempotency payload conflict (typed errors + canonical compare, 9 tests hijau)
- [x] R3 Concurrent confirm test (2-goroutine, 1 jurnal — hijau)
- [x] R4 Transactional/durable audit (success/cancel atomik + fault injection, 4 tests hijau; codex limit mid-run, diff direview manual)
- [x] R5 Handler-level security tests (decision boundary + 6 tests hijau, inline)
- [x] R6 Reproducible sqlc/build pipeline (scripts/verify.sh VERIFY OK + clean-clone VERIFY OK + gofmt gate)
- [x] R7 Migration/runbook hardening (v10 irreversible, no v11, backup 3.1MB, health 97|100|72 bersih)
- [ ] R8 Telegram admin+non-admin trial (bot v0141 PID 1487639 siap — menunggu eksekusi user)
- [ ] R9 Completion report correction
- [x] Commit + dual push (head 40f2845)
- [x] Workspace verification (verify.sh hijau)
- [ ] Live approval obtained
