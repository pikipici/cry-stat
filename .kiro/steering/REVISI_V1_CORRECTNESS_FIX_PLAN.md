# Crypto Station — Revisi V.1 Correctness Fix Plan

> **Untuk AI executor:** eksekusi task secara berurutan dengan TDD. Jangan melanjutkan ke Sandbox S1–S6 sebelum seluruh blocker P0/P1 di dokumen ini hijau.
>
> Status: READY FOR EXECUTION — audit koreksi terhadap completion report Phase 2–9
> Audit baseline: repository HEAD `476c78c`; implementation head yang dilaporkan `e2d19ee`
> DB workspace yang dilaporkan: migration v13
> Mode: SANDBOX / SIMULASI
> Live production: DILARANG deploy/restart tanpa approval eksplisit user
> Runtime/build/test/migrate: `cbt-vps`; lokal hanya coding + Git

## 1. Goal

Menutup correctness gap yang ditemukan setelah review `.kiro/steering/REVISI_V1_COMPLETION_REPORT.md`, terutama pada durable payout lifecycle, refund exactly-once, worker claim, state transition, dan callback safety.

Target akhir:

1. Satu payout gagal menghasilkan paling banyak satu refund journal.
2. Refund journal dan linkage ke payout commit atomik dalam satu DB transaction.
3. Worker hanya dapat memenangkan satu claim yang durable untuk satu payout attempt.
4. Setiap state transition mendeteksi zero-row update dan stale/racing transition.
5. Callback tidak dereference payload/message/from sebelum guard.
6. Test mencakup concurrency dan crash-window, bukan hanya happy path.
7. Completion report dikoreksi berdasarkan evidence aktual.

## 2. Temuan Audit dan Severity

| ID | Severity | Temuan | Evidence utama | Status |
|---|---|---|---|---|
| CF-01 | P0 | Refund journal commit terpisah dari `refund_journal_id`, sehingga crash/retry bisa double-credit | `internal/ledger/ledger.go:215-262`, `internal/bot/bot.go:733-735`, `db/queries/chain.sql:83-84` | OPEN |
| CF-02 | P0 | Refund journal memakai `RefTable=payouts` tetapi `RefID=tradeID` | `internal/ledger/ledger.go:237-241` | OPEN |
| CF-03 | P1 | `ClaimQueued` hanya SELECT lock tanpa transaction/lease durable | `db/queries/chain.sql:60-65`, `internal/chain/chain.go:226-233` | OPEN |
| CF-04 | P1 | Transition payout memakai `:exec`; zero-row update dianggap sukses | `db/queries/chain.sql:67-84`, `internal/chain/chain.go:180-215` | OPEN |
| CF-05 | P1 | Error transition unknown/failed diabaikan | `internal/chain/chain.go:187,214` | OPEN |
| CF-06 | P1 | Dispatcher callback dereference sebelum guard; klaim nil-safe/private-only terlalu luas | `internal/bot/bot.go:654-690`, `internal/bot/adminflow.go` | OPEN |
| CF-07 | P2 | DB trigger v13 hanya asset-match, bukan sum-zero per jurnal | `migrations/000013_ledger_hardening.sql:6-22` | OPEN — clarify report, optional DB design |
| CF-08 | P2 | Historical global SOL imbalance masih `-54.693206730894899100` | completion report §4 | OPEN — remediation decision required |
| CF-09 | P3 | Completion report stale: head dan `PROJECT_IDEAS.md` status tidak lagi aktual | completion report header/§9 vs HEAD `476c78c` | OPEN |

## 3. Locked Implementation Decisions

1. Refund exactly-once harus dijamin DB transaction, bukan urutan dua service call dari handler.
2. Ledger tetap satu-satunya pintu pembuat jurnal; orchestration refund dipindahkan ke method ledger/application-service yang menerima `payoutID` dan mengunci payout.
3. Refund journal wajib memakai `RefTable='payouts'` dan `RefID=payoutID`.
4. Idempotency refund diperkuat dengan unique journal identity untuk payout atau constraint ekuivalen; jangan hanya mengandalkan nullable pointer.
5. Claim worker memakai atomic state transition `UPDATE ... RETURNING`, bukan SELECT lock yang langsung lepas.
6. Transition SQL wajib mengembalikan row/count; zero-row adalah typed stale-transition error.
7. Callback guard dijalankan sebelum dispatch `t:*`, `a:*`, dan `q:*`, serta sebelum dereference nested fields.
8. Historical imbalance tidak boleh “diperbaiki” dengan update/delete langsung. Buat proposal adjustment journal terpisah dan tunggu approval user sebelum mutasi DB.
9. Tidak ada payment/chain asli, private key, watcher nyata, atau live deployment dalam scope ini.

## 4. Scope dan Non-goals

### In scope

- Payout refund atomicity dan idempotency.
- Durable worker claim/state transition.
- Callback entry-point safety.
- Concurrency/fault-injection tests.
- Migration forward/additive setelah v13 bila dibutuhkan.
- Koreksi completion report dan roadmap status.

### Non-goals

- Sandbox S1–S6 Top Up.
- Payment gateway asli.
- Chain/node asli.
- Historical ledger adjustment tanpa approval.
- Restore drill dan live deployment.
- Account taxonomy rename `house:inventory`.

## 5. Status Legend

- `[ ]` belum dikerjakan
- `[~]` sedang dikerjakan
- `[x]` selesai dan terverifikasi
- `[!]` blocked/perlu keputusan user

---

## 6. Task-by-Task Implementation Plan

### Task 0 — Preflight dan Reconciliation

**Objective:** memastikan AI bekerja dari HEAD aktual dan tidak menimpa perubahan Markdown user.

**Files:**
- Read: `.kiro/steering/REVISI_V1_COMPLETION_REPORT.md`
- Read: `.kiro/steering/revisi_system_V.1.md`
- Read: file payout/callback yang disebut pada §2
- Update: checkpoint §8 dokumen ini

**Steps:**

- [ ] Jalankan `git status --short --branch` dan `git log --oneline -20`.
- [ ] Pisahkan perubahan existing user dari scope correctness fix; jangan reset/revert.
- [ ] Catat HEAD lokal, `origin/main`, server bare, dan migration workspace.
- [ ] Verifikasi ulang CF-01 sampai CF-06 terhadap kode HEAD; isi status `MASIH VALID/BERUBAH/DONE/DROP` dengan `file:line`.
- [ ] Commit checkpoint reconciliation sebagai docs-only commit.

**Expected:** tidak ada coding sebelum verdict tiap blocker tercatat.

**Commit:** `docs(revision): reconcile payout correctness gaps against current head`

### Task 1 — RED Tests untuk Atomic Refund Exactly-once

**Objective:** mereproduksi double-refund dan crash window sebelum implementasi.

**Files:**
- Modify/Create test: `internal/ledger/payout_refund_test.go`
- Inspect: `internal/chain/payout_lifecycle_test.go`
- Inspect: `internal/ledger/ledger.go`

**Tests wajib:**

- [ ] Dua goroutine refund payout yang sama → tepat satu sukses/one effective refund.
- [ ] Retry setelah refund selesai → return hasil lama, tidak membuat jurnal kedua.
- [ ] Failure injection sebelum audit/linkage/commit → tidak ada saldo dan linkage parsial.
- [ ] Failure injection setelah operasi application-service kembali sukses → payout punya tepat satu `refund_journal_id`.
- [ ] Journal identity memakai payout ID, bukan trade ID.
- [ ] Unknown payout ditolak dan tidak membuat refund.

**RED command di VPS:**

```bash
cd /home/ubuntu/crypto-station
sqlc generate
go test -count=1 ./internal/ledger -run 'Test.*PayoutRefund' -v
```

**Expected sebelum fix:** minimal concurrency/crash-window test FAIL.

**Commit:** `test(ledger): reproduce payout refund race and crash window`

### Task 2 — Migration v14 untuk Refund Identity/Claim (Jika Reconciliation Menyetujui)

**Objective:** menambah constraint minimum yang membuat exactly-once dan claim durable bisa ditegakkan.

**Files:**
- Create: `migrations/000014_payout_correctness.sql`
- Modify: `db/queries/chain.sql`
- Generated di VPS: `internal/db/`, `db/schema/` (jangan commit)

**Recommended schema:**

- Unique journal identity untuk `(ref_table, ref_id, type)` khusus refund payout, atau unique partial index yang ekuivalen.
- Claim metadata minimal bila lease dibutuhkan: `claim_token`, `claimed_at`; pilih hanya jika atomic queued→broadcasting belum cukup.
- Forward/additive; jangan drop data historis.
- Goose statement harus kompatibel dengan parser repo.

**Pre-migration checks:**

- [ ] Cek duplicate refund journal existing berdasarkan payout identity.
- [ ] Cek payout failed/refunded yang linkage-nya tidak konsisten.
- [ ] Backup workspace DB bila migration mengubah constraint/index penting.

**Verification:**

```bash
goose -dir ./migrations postgres "$DATABASE_URL" status
goose -dir ./migrations postgres "$DATABASE_URL" up
sqlc generate
```

**Expected:** DB workspace v14; tidak ada data dihapus.

**Commit:** `feat(db): enforce payout refund identity and durable claim`

### Task 3 — GREEN: Atomic Refund Application Service

**Objective:** membuat lock payout, jurnal refund, wallet cache, dan payout linkage dalam satu transaction.

**Files:**
- Modify: `internal/ledger/ledger.go` atau create `internal/ledger/payout_refund.go`
- Modify: `db/queries/chain.sql`
- Modify: `internal/bot/bot.go`
- Test: `internal/ledger/payout_refund_test.go`

**Required design:**

```text
BEGIN
  SELECT payout FOR UPDATE
  validate payout belongs to user/trade and status=failed
  if refund_journal_id exists: return existing result (AlreadyDone)
  create refund journal(ref_table=payouts, ref_id=payout.id)
  create balanced IDR legs
  refresh wallet cache
  UPDATE payout SET refund_journal_id=? WHERE id=? AND refund_journal_id IS NULL RETURNING ...
  COMMIT
```

**Rules:**

- Handler tidak lagi memanggil `RefundPayoutTx` lalu `SetRefundJournal` terpisah.
- Typed result: `Created`, `AlreadyDone`, `NotRefundable`, `Conflict`.
- Zero-row linkage menyebabkan rollback, bukan sukses palsu.
- Jangan mengabaikan error.
- Kalau payout unknown, queued, broadcasting, atau confirmed → no refund.

**GREEN verification:**

```bash
go test -count=1 ./internal/ledger -run 'Test.*PayoutRefund' -v
go test -count=1 ./internal/chain -run 'Test.*Payout' -v
```

**Expected:** semua tests Task 1 PASS.

**Commit:** `fix(ledger): make payout refund atomic and exactly once`

### Task 4 — RED/GREEN Durable Worker Claim

**Objective:** satu payout hanya dimenangkan satu worker pada satu attempt.

**Files:**
- Modify: `db/queries/chain.sql`
- Modify: `internal/chain/chain.go`
- Modify: `internal/chain/payout_lifecycle_test.go`

**Preferred query pattern:** atomic claim dengan CTE + `UPDATE ... RETURNING`, misalnya memilih queued/retryable row lalu langsung mengubah state/claim metadata dalam statement yang sama.

**Tests wajib:**

- [ ] Dua workers claim paralel → payout ID tidak overlap.
- [ ] Claim memperbarui state/attempt secara atomik.
- [ ] Stale broadcasting dapat direclaim hanya berdasarkan policy/lease eksplisit.
- [ ] Fresh broadcasting tidak dapat diclaim worker kedua.
- [ ] Restart recovery mengambil queued/stale work, bukan terminal work.

**Commands:**

```bash
go test -count=1 ./internal/chain -run 'Test.*Claim|Test.*Recovery' -v
```

**Commit RED:** `test(chain): reproduce duplicate payout worker claim`

**Commit GREEN:** `fix(chain): claim queued payouts atomically`

### Task 5 — Guard Semua Payout State Transition

**Objective:** tidak ada transition yang dianggap sukses jika SQL memengaruhi nol row.

**Files:**
- Modify: `db/queries/chain.sql`
- Modify: `internal/chain/chain.go`
- Test: `internal/chain/payout_lifecycle_test.go`

**Changes:**

- [ ] Ubah transition penting ke `:one ... RETURNING` atau `:execrows`.
- [ ] Tambah typed `ErrPayoutStaleTransition`/ekuivalen.
- [ ] `MarkPayoutBroadcasting`, confirmed, unknown, failed, refund-link wajib cek hasil.
- [ ] Hapus semua `_ =` yang mengabaikan error transition.
- [ ] Chain event hanya dibuat setelah confirmed transition benar-benar menang.

**Tests:**

- [ ] Confirm dari state ilegal → typed error, tanpa chain event.
- [ ] Failed/unknown zero-row → typed error.
- [ ] Dua finalizer paralel → hanya satu terminal transition menang.
- [ ] Confirmed idempotent retry mengembalikan hasil lama.

**Verification:**

```bash
go test -count=1 ./internal/chain -run 'Test.*Payout|Test.*Transition' -v
```

**Commit:** `fix(chain): enforce payout transition row ownership`

### Task 6 — Callback Guard Sebelum Dereference/Dispatch

**Objective:** semua callback malformed/non-private/unknown aman tanpa panic atau mutasi.

**Files:**
- Modify: `internal/bot/bot.go`
- Modify: `internal/bot/callback_guard.go`
- Modify: `internal/bot/flow.go`
- Modify: `internal/bot/adminflow.go`
- Test: `internal/bot/callback_guard_test.go` atau existing router tests

**Required order:**

```text
if cb == nil → return safely
answer callback only if ID available
validate From + Message + nested Message + Chat
private-chat guard
registered/blocked user handling
then dispatch t:/a:/q:
```

Admin allowlist tetap dicek setelah structural/private guard dan sebelum business action.

**Tests wajib:**

- [ ] nil callback.
- [ ] nil `From`.
- [ ] nil `Message`/nested message.
- [ ] group/channel callback.
- [ ] malformed `q:`, `t:`, `a:`.
- [ ] non-admin forged admin callback.
- [ ] semua kasus di atas tidak panic dan tidak memanggil mutation service.

**Verification:**

```bash
go test -count=1 ./internal/bot -run 'Test.*Callback|Test.*Router' -v
```

**Commit:** `fix(bot): guard callback structure before dispatch`

### Task 7 — Ledger Invariant Report dan Historical Imbalance Decision

**Objective:** membedakan invariant write baru dari historical DB correctness secara jujur.

**Files:**
- Modify: `.kiro/steering/REVISI_V1_COMPLETION_REPORT.md`
- Optional create after approval only: dedicated adjustment plan, bukan migration langsung

**Steps:**

- [ ] Jalankan read-only global reconciliation dan catat nilai per asset.
- [ ] Verifikasi jurnal pasca-Phase 3 sum-zero berdasarkan timestamp/ID boundary.
- [ ] Koreksi wording: v13 asset-match trigger bukan DB sum-zero trigger.
- [ ] Buat proposal adjustment journal untuk imbalance historis, berisi asal, amount, counterpart, audit reason, dan rollback strategy.
- [!] Jangan menjalankan adjustment tanpa approval eksplisit user.

**Commit:** `docs(ledger): clarify new-write invariant and historical imbalance`

### Task 8 — Full Verification dan Fault-focused Regression

**Objective:** membuktikan koreksi bekerja dari clean workspace generation.

**Run di `cbt-vps`:**

```bash
cd /home/ubuntu/crypto-station
export PATH=$PATH:/usr/local/go/bin
sqlc generate
gofmt -w <touched-go-files>
go vet ./...
go test -count=1 ./internal/ledger ./internal/chain ./internal/bot
go test -count=1 ./...
go build ./cmd/bot
bash scripts/verify.sh --no-db
```

Jika migration v14 dibuat, jalankan `scripts/verify.sh --full` hanya terhadap DB workspace/test dengan opt-in yang diwajibkan script.

**Acceptance:**

- [ ] Targeted concurrency/fault tests hijau.
- [ ] Full tests hijau.
- [ ] Vet/build hijau.
- [ ] Verify script `VERIFY OK`.
- [ ] DB migration sesuai expected version.
- [ ] Tidak ada secret di diff/log/docs.
- [ ] Live tidak disentuh.

**Commit:** hanya fix hasil verification yang benar-benar diperlukan; jangan gabungkan unrelated cleanup.

### Task 9 — Manual Workspace Trial

**Objective:** menguji flow yang tidak dapat dibuktikan automated test.

**Prerequisite:** approval user untuk restart bot workspace/sandbox. Bukan live.

**Checklist:**

- [ ] Failed payout mock → saldo refund satu kali.
- [ ] Double-tap/retry failed payout → tidak double-credit.
- [ ] Unknown payout → tidak refund dan Status memberi warning.
- [ ] Recovery queued/delayed setelah restart workspace.
- [ ] Callback malformed/group/non-admin tidak mutasi dan tidak bocor data.
- [ ] Existing Admin Wallet feed/detail tidak regresi.

Catat evidence ke `TRIAL.md` dan completion report. Automated test tidak boleh dicatat sebagai manual PASS.

### Task 10 — Documentation Closure

**Objective:** membuat status akhir konsisten dan tidak overclaim.

**Files:**
- Modify: `.kiro/steering/REVISI_V1_COMPLETION_REPORT.md`
- Modify: `.kiro/steering/revisi_system_V.1.md`
- Modify: `PROJECT_IDEAS.md`
- Modify: `TRIAL.md`
- Modify: `.kiro/steering/ADMIN_WALLET_CURRENT_STATE.md` jika evidence terkait berubah

**Required corrections:**

- Head aktual dan migration version.
- CF-01–CF-09 status + evidence.
- Pisahkan implemented/automated/manual/workspace/live.
- Jangan sebut Phase 4 DONE sebelum atomic refund, claim, transition, dan recovery tests hijau.
- `PROJECT_IDEAS.md` tidak lagi ditandai pending bila sudah sinkron.
- Historical imbalance tetap open sampai adjustment disetujui dan dieksekusi.

**Commit:** `docs(revision): close payout correctness remediation with evidence`

---

## 7. Final Acceptance Criteria

Correctness fix baru boleh berstatus `AUTOMATED DONE` jika:

- [ ] Refund payout atomik dengan linkage dalam satu transaction.
- [ ] Satu payout gagal maksimal satu refund journal meskipun concurrency/retry/crash simulation.
- [ ] Refund journal mereferensikan payout ID yang benar.
- [ ] Worker claim atomic dan non-overlap.
- [ ] Zero-row state transition tidak dianggap sukses.
- [ ] Tidak ada error transition yang sengaja diabaikan.
- [ ] Chain event tidak dibuat jika confirmed transition kalah.
- [ ] Callback guard mendahului dereference dan dispatch seluruh namespace.
- [ ] Targeted + full VPS verification hijau.
- [ ] Completion report sudah dikoreksi.

Status `MANUAL ACCEPTANCE DONE` membutuhkan tambahan:

- [ ] Trial failed/unknown/retry Telegram.
- [ ] Restart recovery workspace.
- [ ] Admin/non-admin callback trial.

Status `LIVE` tidak termasuk plan ini dan tetap memerlukan approval eksplisit terpisah.

## 8. Execution Checkpoint

| Task | Status | SHA | Evidence | Notes |
|---|---|---|---|---|
| 0 Preflight/reconciliation | [x] | `476c78c` | git status/log/remote verified 2026-11-04; HEAD==origin/main==server bare `476c78c`; work MD: 4 modified (Fase0-README/STAGE0/TRIAL/ADMIN_WALLET_STATE, masih head lama v0.14.1/v10) + 1 untracked (FIX_PLAN ini); CF-01 s/d CF-06 MASIH VALID lawan kode HEAD (lihat §9) | Docs-only, no code touched |
| 1 Refund RED tests | [x] | `d128f16` | VPS RED 5/5 FAIL 2026-11-04: ConcurrentOnce double-credit (900000→1100000, okCount=2); Retry jurnal kedua (jid 2053→2054); Identity ref_id=0 bukan payoutID/129; Unknown ke-refund; CrashWindow jurnal 2063 commit linkage NULL | RED done, fix di Task 2-3 |
| 2 Migration v14 | [x] | `535bfc1`+`6fbd186` | DB workspace v14 (000014 OK 2026-11-04); sqlc generate OK; gofmt clean; vet+build OK; chain pkg hijau; constraint idx_journals_refund_payout_unique TERBUKTI jalan (duplicate 502/503 dari RED test dibersihkan dulu: backup /tmp/cs-backup-pre-t2cleanup-*.dump, hapus 4 jurnal test, recompute wallet 1048/1049=900000 truth); Concurrent+Retry PASS via constraint kasar (loser = raw SQL error, belum typed); Unknown+CrashWindow+Identity masih FAIL (behavior fix Task 3-5) | Constraint minimum done; pre-migration duplicate adalah artefak RED test sendiri |
| 3 Atomic refund | [x] | `e8961fe` | VPS GREEN 6/6 2026-11-04 (ConcurrentOnce/Retry/Identity/UnknownRejected/CrashWindow/WrongUser); full suite hijau semua pkg + build OK; gofmt/vet bersih; API baru RefundPayout(userID,payoutID) lock FOR UPDATE+jurnal(ref payouts/payoutID)+linkage 1 tx, typed Created/AlreadyDone/NotRefundable/RaceLost; bot.go caller pindah ke API baru; RefundPayoutTx lama deprecated (masih dipakai chain test existing, hapus di closure) | CF-01/CF-02 behavior fixed; crash window tertutup |
| 4 Durable claim | [x] | `9914240` | VPS GREEN 2026-11-04: NoOverlap/BatchNoOverlap/StaleReclaim/RestartQueued + existing QueuedRecovery; ClaimPayout fresh-queued-only, stale reclaim terpisah (ReclaimStaleBroadcastingAtomic), RecoveryScan queued+stale; full suite hijau + build OK; 1 flaky ledger FAIL sekali (DB shared race) lalu 59/59 PASS dua run | CF-03 fixed; ClaimQueued lama tetap utk kompat, worker baru pakai ClaimIDs/Batch |
| 5 Guarded transitions | [x] | `c78b51b` | VPS GREEN 2026-11-04: ConfirmIllegal/FailedUnknownZeroRow/TwoFinalizerOneWins/ConfirmedIdempotent + full suite hijau + build OK; mustWin() helper, zero-row=ErrPayoutStaleTransition, semua `_ =` transition dihapus, chain event cuma bila confirm menang; BONUS BUG: unknown→broadcasting ternyata ga didukung query lama (ketahuan karena mustWin, sebelumnya lolos diam-diam) → MarkPayoutBroadcasting allow unknown (recovery path, existing test UnknownNoRefund jadi saksi) | CF-04/CF-05 fixed |
| 6 Callback guard | [ ] | | | |
| 7 Ledger/report clarification | [ ] | | | Historical adjustment needs approval |
| 8 Full verification | [ ] | | | VPS only |
| 9 Manual workspace trial | [ ] | | | Needs user approval/restart |
| 10 Documentation closure | [ ] | | | |

## 9. Current Next Step

Task 0 DONE 2026-11-04 — reconciliation verdict lawan HEAD `476c78c`:

- CF-01 MASIH VALID — `internal/ledger/ledger.go:215-262` RefundPayoutTx commit jurnal sendiri; linkage via `internal/bot/bot.go:733-735` RefundPayoutTx + SetRefundJournal terpisah; crash window nyata.
- CF-02 MASIH VALID — `internal/ledger/ledger.go:237-241` RefTable=payouts tapi RefID=tradeID.
- CF-03 MASIH VALID — `db/queries/chain.sql:60-65` ClaimQueuedPayouts SELECT FOR UPDATE SKIP LOCKED tanpa update atomik; `internal/chain/chain.go:226-233` lepas setelah statement.
- CF-04 MASIH VALID — `db/queries/chain.sql:67-84` semua MarkPayout* `:exec`; `internal/chain/chain.go:180-215` tanpa cek rows affected.
- CF-05 MASIH VALID — `internal/chain/chain.go:187` dan `:214` pakai `_ =` untuk MarkPayoutUnknown/MarkPayoutFailed.
- CF-06 MASIH VALID — `internal/bot/bot.go:654-657` dereference cb.ID/cb.Data sebelum guard; guard nil-safe+private baru di q: branch `:680-690`, t:/a: dispatch `:659-667` tanpa guard sama.
- CF-07/08/09 ikut OPEN per §2 (butuh cek 000013 di VPS + decision adjustment + koreksi report).
- Work MD user (4 modified + 1 untracked) TIDAK disentuh task ini.

AI executor mulai **Task 1 — RED Tests untuk Atomic Refund Exactly-once**. Jangan menulis migration atau source code sebelum RED test di-commit.
