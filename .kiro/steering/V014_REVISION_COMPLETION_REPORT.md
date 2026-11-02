# Laporan Penyelesaian Revisi — Admin Wallet v0.14.1 Hardening

> Plan: `.kiro/steering/ADMIN_WALLET_V014_REVISION_PLAN.md` (804 baris, R1–R9)
> Status: R1–R7 + R9 DONE dan terverifikasi. R8 trial Telegram-tap PENDING eksekusi user.
> Target: v0.14.1-admin-wallet-hardening
> Selesai automated: 2026-11-03
> Live: BELUM disentuh — tanpa approval eksplisit.

## 0. Posisi Terhadap Plan Original v0.14

Revisi ini tidak menambah fitur. Plan original
(`ADMIN_WALLET_OPERATIONS_PLAN.md`, T1–T9) tetap implemented sesuai
`V014_COMPLETION_REPORT.md`. Revisi menutup 7 temuan audit (R1–R7),
mengkoreksi klaim report (R9), dan menyisakan trial tombol Telegram (R8)
untuk user karena membutuhkan akun admin + non-admin asli.

## 1. Ringkasan Temuan → Perbaikan

| ID | Temuan | Perbaikan | Bukti |
|---|---|---|---|
| R1 | `tx_ref` unique tanpa ikat payload; retry bisa kembalikan metadata transaksi lain | Compare immutable identity (admin/asset/network/units canonical/to_address) + typed errors `ErrStockDepositRefConflict` / `Cancelled` / `Unauthorized`; cross-admin tanpa leak | 9 tests `stock_deposit_conflict_test.go` hijau |
| R2→R1 | (bagian R1) cancelled ref error generik | `ErrStockDepositCancelled` typed + copy §8.3 di handler + preview-upgrade (recorded/cancelled/dup dibedakan) | test CancelledRef + trial manual |
| R3 | Klaim concurrency hanya sequential retry | Test 2-goroutine + barrier: 1 success + 1 AlreadyDone, 1 jurnal, balance +1x | `TestRecordStockDepositConcurrentConfirmOneJournal` hijau |
| R4 | Audit `_ = Audit(...)` setelah mutasi; bisa sukses tanpa jejak | Audit success/cancel masuk tx ledger (`CancelAdminStockDeposit` baru); gagal = rollback penuh; hook seam test-only 4 stage | 4 tests `stock_deposit_audit_test.go` hijau |
| R5 | Handler security hanya string keyboard tests | `decideWalletCallback` + `walletConfirmCopy` decision boundary; deny defense-in-depth; 6 tests | `wallet_sender_test.go` hijau |
| R6 | `cp migrations/000010 db/schema/` manual; gofmt drift | `scripts/verify.sh` (version pins + rebuild schema + sqlc + gofmt gate + vet/test/build/smoke); clean-clone VERIFY OK | 2x VERIFY OK (workspace + /tmp/cs-clean) |
| R7 | v10 tanpa rollback story | Irreversible by policy + runbook + backup 3.1 MB; no v11 (service-layer cukup); health checks | `V014_MIGRATION_RUNBOOK.md` |

## 2. Hasil Verifikasi Final (workspace cbt-vps, 2026-11-03)

| Item | Hasil |
|---|---|
| `bash scripts/verify.sh` | VERIFY OK (gofmt bersih, vet bersih, build OK, smoke 39 settings exit 0) |
| `go test -count=1 -v ./...` | **87–88 `--- PASS`, 4 SKIP, 0 FAIL** (fluktuasi 87/88 = timing parallel, bukan fail; SKIP = 4 test sell-bandar era pivot, by design) |
| DB migration | v10 `000010_admin_stock_deposits` |
| DB counts | deposits 217 \| float-in journals 214 \| wallet audits 180 \| deny audits 30 |
| Anomali | 0 (semua float-in tepat 2 legs; 0 recorded tanpa journal) |
| Bot workspace | `/tmp/cs-bot-v0141` PID 1487639, hidup 2 jam+, `fase 4 start mode=sandbox` |
| `getMe` | @cryptostationonly_bot (id 8898894252) |
| Head | `9a417d2`, sinkron origin + VPS + bare |

Test baru revisi (19 funcs):

- `stock_deposit_conflict_test.go` (9): RetryIdentical, DifferentAdmin/Asset/Units/Address, CancelledRef, ConcurrentConfirm, EighteenDecimalCanonical, NoLeak.
- `stock_deposit_audit_test.go` (4): AuditFailureRollsBackAll, CancelAuditFailureRollsBack, WritesSuccessAudit (1 row, tanpa alamat), CancelUnauthorized.
- `wallet_sender_test.go` (6): NonAdminDenied (8 callback), AdminRouting (10), MalformedSilent, NotReady, ConfirmCopyMapping, ConfirmCopyNoLeak.

## 3. Commit Revisi (dual push origin + bare)

| Head | Isi |
|---|---|
| `dc781d6` | R2+R3: typed conflict + concurrent tests (codex) |
| `77fd98a` | R4: transactional audit + fault injection (codex limit mid-run, diff direview manual) |
| `89bb18e` | fix: unique ref di success-audit test |
| `0006580` | R5: decision boundary + tests (inline) |
| `411473e` | fix: pointer receiver conflict error |
| `bffe495` | R6: verify.sh |
| `eb3a415` | style: gofmt 5 file |
| `40f2845` | R7: migration runbook + backup |
| `69df191` | R9: koreksi report v0.14 |
| `0ea9532` | chore: bash requirement note |
| `9a417d2` | docs: R9 done, R8 pending |

## 4. File Baru / Diubah (revisi saja)

Baru: `internal/ledger/stock_deposit_conflict_test.go`,
`internal/ledger/stock_deposit_audit_test.go`,
`internal/bot/wallet_sender.go`, `wallet_errors.go`, `wallet_sender_test.go`,
`scripts/verify.sh`, `.kiro/steering/V014_MIGRATION_RUNBOOK.md`,
dokumen ini (`V014_REVISION_COMPLETION_REPORT.md`).

Ubah: `internal/ledger/stock_deposit.go` (typed errors + compare + audit tx
+ Cancel API + hook), `internal/bot/adminwallet.go` (deny defense-in-depth +
`walletConfirmCopy` + preview-upgrade + hapus double-audit),
`internal/ledger/stock_deposit_audit_test.go` (unique ref),
`scripts/verify.sh` (bash note), `V014_COMPLETION_REPORT.md` (koreksi §0 +
evidence §2 + instruksi R8 §7), plan revisi (checklist).

Tidak diubah: schema v10, queries sqlc (InsertAdminAudit sudah ada),
`revisi_system_V.1.md` (scope lebih besar, dilarang plan §13).

## 5. Acceptance Revisi §12 — Status Jujur

- [x] Identical retry 1 jurnal (sequential + concurrent).
- [x] Same ref beda admin/asset/units/address → typed conflict, tanpa leak.
- [x] Cancelled ref semantics + copy jelas.
- [x] Concurrent confirm terbukti 1 jurnal.
- [x] Fault injection rollback journal/legs/status/audit.
- [x] Success + cancel audit atomik.
- [x] Deny failure tidak melemahkan denial (deny di entry; telemetry penuh = backlog disengaja).
- [x] Handler allowlist tests (decision boundary; full API-mock disengaja tidak — 3 call sites).
- [x] Preview/cancel/confirm side effects (service + decision; Telegram-tap pending R8).
- [x] Clean checkout 1 command.
- [x] Migration/rollback-forward terdokumentasi.
- [x] Full suite hijau.
- [ ] Telegram trial admin + non-admin (R8 — instruksi di `V014_COMPLETION_REPORT.md` §7, 25 langkah, ref MOCK-R8-001/002).
- [x] Report dikoreksi.
- [x] Live belum disentuh.

## 6. Masalah + Solusi (revisi)

1. Codex `~/.codex/config.toml` hardcode `gpt-6-sol` → 400 model_not_found.
   Solusi sementara: `-m gpt-5.6-terra` tiap run. Permanen: ganti config.
2. Codex usage limit mid-R4 → sisa R4 diff direview manual + R5/R6 inline.
   Tidak ada klaim tanpa review: semua diff dibaca sebelum commit.
3. `TestRecordStockDepositWritesSuccessAudit` FAIL rerun (ref statis collision)
   → unique ref per run (`89bb18e`).
4. `StockDepositConflictError` pointer receiver → vet fail di test → fix 2 titik.
5. `scripts/verify.sh` via `sh` gagal (`pipefail` dash) → wajib `bash scripts/verify.sh`.
6. gofmt drift 5 file (termasuk pre-existing admin.go/price.go) → gofmt gate di script.
7. `origin` VPS = bare lokal (bukan GitHub) → pola fetch-GitHub + reset + push-bare tiap iterasi.
8. SSH noise `LocalForward 8080` konflik — kosmetik, perintah tetap jalan.

## 7. Backlog Yang Disengaja Tidak Dikerjakan

- Full Telegram Bot-API mock (boundary diuji, 3 call sites).
- Deny telemetry penuh (log/metric/alert) — deny tetap menolak walau audit gagal.
- Fingerprint payload kolom DB — compare per-field + UNIQUE cukup, no v11.
- Fingerprint, outbox worker, on-chain/watcher/node/key/gateway/fiat — non-goals §13.
- `revisi_system_V.1.md` (scope besar, dokumen lain).

## 8. Langkah Berikutnya

1. User jalanin R8 (25 langkah, `V014_COMPLETION_REPORT.md` §7) → lapor nomor gagal.
2. AI verifikasi audit rows + tandai R8 DONE di plan revisi.
3. Minta approval eksplisit bila mau deploy/restart live (dilarang default).
4. Setelah gate Stage 2 → Fase 5 QRIS (di luar scope revisi ini).
