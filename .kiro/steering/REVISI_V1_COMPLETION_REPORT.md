# Crypto Station — Laporan Penyelesaian Revisi Sistem V.1 (Phase 2–9)

> Status: AUTOMATED DONE — hardening user flow existing selesai otomatis; manual/operational acceptance masih pending
> Periode pengerjaan: 2026-11-03 (satu sesi, inline implementation)
> Code head: `e2d19ee` (origin == server bare, dual push sinkron)
> Baseline sebelum revisi: `a46427b` (docs) / `8c60ce1` (code Admin Wallet v0.14.1)
> DB workspace: migration v13 (`000013_ledger_hardening.sql`)
> Mode: SANDBOX / SIMULASI — bukan transaksi on-chain
> Live production: BELUM disentuh; wajib approval eksplisit user
> Toolchain: Go 1.27.3, sqlc 1.29.0, goose 3.28.0, PostgreSQL 16
> Verifikasi: `bash scripts/verify.sh --no-db` = VERIFY OK; `go vet ./...` bersih; `go test -count=1 ./...` semua package hijau (dengan DATABASE_URL workspace)

Dokumen ini laporan penyelesaian Bagian C (Phase 2–9) dari `.kiro/steering/revisi_system_V.1.md`.
Source of truth Admin Wallet tetap `.kiro/steering/ADMIN_WALLET_CURRENT_STATE.md`.
Keputusan produk/transit tetap `PROJECT_IDEAS.md`.

---

## 1. Ringkasan Eksekutif

Revisi V.1 Bagian C menutup 27 temuan audit (A1–A27) dalam 8 phase. Semua phase AUTOMATED DONE:

| Phase | Scope | Commit utama | Status |
|---|---|---|---|
| 2 | Cancel semantics + callback safety (A1/A8/A12/A13/A15/A23) | `1a117cb` + 3 fix | DONE |
| 3 | Ledger invariant buy/deposit (A3/A4) | `360f5ce` | DONE |
| 4 | Durable payout lifecycle (A2) | `be020fc` + 5 fix | DONE |
| 5 | Deposit idempotency + address safety (A14/A17/A18) | `f2afeec` + 3 fix | DONE |
| 6 | Hapus custodial drift (A9/A10/A11) | `300b7fa` + 3 fix | DONE |
| 7 | Onboarding + UX states (A16/A19–A25) | `e7e7ede` | DONE |
| 8 | Admin blockers: broadcast + callback (A5/A6) | `a5fb336` | DONE |
| 9 | Performance + DB hardening (A26/A27) | `16f7da3` + 6 fix | DONE |

Total sesi ini: ~40 commits (feat + fix + style + docs), 3 migration baru (000011–000013), DB workspace v10 → v13.

Yang SENGAJA belum dikerjakan (sesuai plan, bukan kelalaian):
- Sandbox S1–S6 (Top Up Simulasi production-shaped → buy/payout durable UX → read models → E2E)
- Trial manual Telegram admin/non-admin (butuh akun + tap manusia)
- Restart recovery workspace (butuh approval user)
- Restore drill custom-format (butuh maintenance window + approval)
- Deploy/restart live (butuh approval eksplisit)

---

## 2. Posisi Terhadap Plan

Aturan eksekusi plan (Bagian C0) yang dipatuhi:
- Reconciliation pass SEBELUM coding setiap phase — klasifikasi DONE/MASIH VALID/BERUBAH/DROP vs code head aktual, checkpoint ditulis ke plan lalu commit terpisah.
- Satu invariant per commit; tidak ada mega-commit.
- Runtime/build/test/migrate HANYA di `cbt-vps`; lokal coding + git saja.
- Tidak ada payments/chain asli, private key, watcher, gateway, atau live.
- Codex quota mati mid-loop (limit s/d 2 Des 2026) → fallback inline implementation sesuai skill codex-cli-roadmap-task-loop Step 5b.

Aturan yang dilanggar 1x lalu diperbaiki: `python -c` dengan backslash-escape dimakan shell → string Go corrupt (riwayat newline literal). Fix via patch tool. Pelajaran dicatat di §7.

---

## 3. Per Phase: Temuan, Perbaikan, Bukti

### Phase 2 — Cancel semantics + callback safety (A1/A8/A12/A13/A15/A23)

Temuan inti: `ExpireUserQuotes` hanya expire yang lewat waktu — quote future-active tetap bisa di-confirm setelah user tekan Batal. Copy `/batal` selalu klaim sukses. Callback group bisa mutasi. Malformed callback = bare return (spinner gantung). `GetQuote` tanpa owner-scope sebelum risk check.

Perbaikan:
- Migration `000011_quote_cancelled.sql`: status `cancelled` + index partial user-active.
- Queries: `CancelActiveQuotesByUser` (:execrows, return count), `CancelQuoteByID` (:one, owner-scoped), `GetQuoteForUser` (:one, `(id, user_id)`).
- `internal/bot/cancel.go` (baru): `CancelActiveQuotes`, `CancelQuote`, `BatalCopy` 4-way, `CancelAliases` (incl. ❌ Batal).
- `internal/bot/callback_guard.go` (baru): nil-safe + private-only + user + denyCopy typed.
- `bot.go`: `q:no` = cancel per-quote spesifik; `q:ok` = lookup owner-scoped dulu, risk sesudah; `batal()` 4-way jujur; malformed ada feedback.
- `flow.go`: `GuardCallback` di entry; `t:cancel` = navigation-only (quote tidak disentuh); semua malformed = `stale()` feedback.
- `session.go`: MatchButton ❌ batal. `adminflow.go`: ketik batal saat broadcast = cancel, nol kirim.
- Tests: `cancel_test.go` pure unit (BatalCopy 4-way, aliases, guard nil-safety).

Bukti: verify VERIFY OK, bot unit hijau. Manual Telegram trial masih pending.

### Phase 3 — Ledger invariant buy/deposit (A3/A4)

Temuan inti: legs buy `float −sent + fee-collect +fee` → sum coin = 2fee−gross (nol hanya bila fee=0). Legs deposit `float +units` tanpa kaki negatif → sum = +units. Reconcile hanya cek `user:*`, house-blind.

Evidence empiris: global sum workspace SOL = **-54.693206730894899100** (akumulasi imbalance historis).

Perbaikan:
- Buy: `float −gross / external:payout +sent / fee-collect +fee` (fee-collect dipertahankan, tidak rename).
- Deposit: tambah `external:deposit −units` boundary leg.
- `validateLegs` (sum per asset = 0) pre-commit di ExecuteTrade + MockDeposit + stock float-in.
- `ReconcileGlobal` baru (assert sum global = 0); Reconcile lama tetap untuk trial/report.
- Sell legs existing ternyata SUDAH balance → tidak diubah; `house:inventory` naming debt dicatat Phase 6/9.

Bukti: global SOL sum SEBELUM = SESUDAH test run, identik digit terakhir — semua jurnal baru sum nol persis, imbalance lama tidak bertambah. Sell-bandar baru benar-benar dihapus di Phase 6.

### Phase 4 — Durable payout lifecycle (A2)

Temuan inti: `Chain.Payout` dipanggil AFTER trade commit (bot.go:725) — crash window nyata tanpa jejak durable. Mock langsung confirmed, tanpa failed/unknown/refund. Pesan "IDR tidak hilang" tanpa jurnal.

Perbaikan:
- Migration `000012_payout_lifecycle.sql`: mode, attempt_count, last_error, refund_journal_id UNIQUE, broadcast/finalized_at, idx retryable.
- `ExecuteTrade` bikin payout row `queued` ATOMIK dalam tx yang sama (+PayoutID di TradeResult).
- `ProcessPayout` mock state machine: success/failed/unknown/delayed. Failed → sentinel, refund DIORKESTRASI caller via ledger (`RefundPayoutTx` + `SetPayoutRefund` guard IS NULL = exactly-once). Unknown → hold, no refund.
- `ClaimQueued` FOR UPDATE SKIP LOCKED untuk worker/recovery.
- `bot.go`: refund hanya diklaim setelah jurnal ada; refund gagal → "catat, hubungi admin" (jujur); unknown → "jangan ulang transaksi".
- Tests D3 (5 tests): atomic, success idempotent, failed refund-once, unknown no-refund + later success, delayed recovery.

Bukti: 5/5 targeted hijau di workspace DB v12; ReconcileGlobal absolut FAIL di data historis (bukti A3/A4, bukan bug) → test memakai delta assert.

### Phase 5 — Deposit idempotency + address safety (A14/A17/A18)

Temuan inti: tx_hash sama langsung return lama TANPA cek user/asset/units (user B bisa klaim hasil A). Audit ganti alamat before selalu "(kosong)". Validator SOL alnum saja — O/I/l lolos.

Perbaikan:
- `DepositConflictError{user|asset|units}`, no-leak, di precheck + race path. Identical retry tetap once.
- `ReplaceWalletAddress`: read old + upsert + audit actual before/after dalam 1 tx. 2 caller flow.go pindah; audit manual dihapus.
- `validateSolAddress`: base58 decode manual, tepat 32 byte, tanpa dep baru.
- Temuan bonus: string mock SOL lama (41-char) decode cuma 30 byte = BUKAN pubkey valid sedari awal; validator lama yang longgar menutupi. Mock diganti vektor valid (USDC mint, wSOL, system program, 2 generated).

Bukti: D4 tests hijau (conflict user/asset/units, audit before/after, SOL forbidden/valid/no-panic).

### Phase 6 — Hapus custodial drift (A9/A10/A11)

Temuan inti (BUG nyata): `t:sell` set StepAwaitSell lalu langsung return deposit screen — tapi session sudah di-set, sehingga input nominal user nyasar ke `quoteFlow` BANDAR side-sell. `SellAmountPicker` unreachable (cuma test). Riwayat trade-lama. `/jual` slash vs tombol beda.

Perbaikan:
- ALL/MAX ditolak di parse + `/beli` + `/jual SOL ALL` (dijelaskan + ditawari deposit flow).
- Hapus: sellAllFlow, t:sellall (forged = stale), CreateQuoteUnits, ExecuteTrade sell branch, CreateQuote sell, TestSellFlow, sellall_test.go, SellAmountPicker, quoteFlow generik.
- `showSellDeposit` → `sellDepositText` shared slash + tombol + `/jual SOL` langsung deposit screen.
- Riwayat rewrite: merge payout+deposit, sort desc, limit 10, partial/error states, WIB, SIMULASI.

Bukti: repo-wide grep BERSIH untuk sellall/CreateQuoteUnits/dustTolerance/RecentTradesByUser; verify OK.

### Phase 7 — Onboarding + UX states (A16/A19–A25)

Perbaikan:
- A16: start existing → GrantSandbox repair + pesan saldo aktual dinamis (tidak hardcode).
- A19: quote preview baru — SIMULASI, Bayar, bruto, fee+estimasi IDR, net, network, ALAMAT PENUH, lock, warning irreversible.
- A20: statusCmd bedakan error ("tidak tersedia") vs empty ("belum ada") vs partial.
- A21: `Sessions.expired` tracking + `ExpiredCopy` kontekstual (buy/addr/admin/broadcast) di default branch.
- A22: tombol "Simulasi kirim senilai Rp50rb/Rp100rb" + hasil tulis neto aktual.
- A24: `PayoutStatusID(status, refunded)` + `DepositStatusID`; heading "Pengiriman terbaru"; failed tanpa refund = "hubungi admin".
- A25: welcome transit ("kamu"), tombol "✅ Konfirmasi" (callback data tetap).
- Tests: `ux_state_test.go` (mapping, ExpiredCopy, session expired incl. Clear-hapus-catatan).

### Phase 8 — Admin blockers (A5/A6)

Klasifikasi: A5 SUDAH DONE di code (builder a:stockfloat:SOL + parser case) — gap dari audit lama; bukti = test table-driven 4x3 hijau (baru).
A6 MASIH VALID → diperbaiki:
- Teks broadcast → preview + tombol Kirim/Batal (`a:broadcast:send/no`). Kirim HANYA via confirm; session hilang/ganda → "sudah diproses atau kedaluwarsa"; cancel → nol kirim; ketikan saat confirm → arahan.
- Dedup 60 detik per (sender, teks) via `claimBroadcastSend` (pure, unit-test) — double-tap/double-slash ditolak + audit dedup. Slash langsung tetap + dedup.
- `StepAdminBroadcastConfirm` + ExpiredCopy case-nya.

### Phase 9 — Performance + DB hardening (A26/A27)

Perbaikan:
- A26: `SumLedgerByOwner` + `refreshWalletCache(userID)` 1 query targeted (6 call sites jadi 1x panggil). EXPLAIN: Index Scan owner + Bitmap entries (bukan seq scan).
- A27 migration 000013: trigger `trg_entry_asset_match`, CHECK units>0 (deposits/payouts), fee>=0, `sent_units` GENERATED ALWAYS AS (units − fee_units), partial unique 1 active quote/user.
- Fallout GENERATED beruntun (diperbaiki): CreatePayoutQueuedParams tanpa SentUnits; RecentPayouts/GetPayout* SentUnits → pgtype.Numeric; helper baru `chain.NumericToDecimal` dipakai bot (3 titik) + adminwallet.
- `TestParallelActiveQuotesLock` ditulis ulang jadi constraint test (insert kedua ditolak + requote jalan); double-execute tetap dicover `TestDoubleTapOneWins`.
- Tests: equivalence targeted vs full, delta-nol DB, trigger reject.

Bukti: DB v13, EXPLAIN targeted, semua package hijau.

---

## 4. Verification Evidence

Final state (head `e2d19ee`):
- `scripts/verify.sh --no-db`: VERIFY OK (sqlc regen incl. 000011–000013, drift check, gofmt, vet, full tests, build).
- `go vet ./...`: bersih.
- `go test -count=1 ./...` (dengan DATABASE_URL workspace): admin, adminwallet, bot, chain, ledger, price, risk, trial — semua ok.
- Targeted DB suites: D3 payout (5/5), D4 deposit/address/SOL, Phase 9 equivalence/trigger/delta — hijau.
- Migration: workspace v13 (000011 + 000012 + 000013 applied via goose up).
- Push: origin (GitHub) == server bare == `e2d19ee`, dual push sinkron setiap commit.

Evidence historis yang tetap relevan:
- SOL global sum -54.693206730894899100 = warisan imbalance A3/A4 pra-Phase 3; jurnal pasca-Phase 3 sum nol (delta identik).
- EXPLAIN SumLedgerByOwner: Index Scan + Bitmap Heap Scan (targeted).

---

## 5. File Baru & Diubah (ringkas)

Baru (12):
- `migrations/000011_quote_cancelled.sql`, `000012_payout_lifecycle.sql`, `000013_ledger_hardening.sql`
- `internal/bot/cancel.go`, `callback_guard.go`, `cancel_test.go`, `payout_status.go`, `ux_state_test.go`, `admin_safety_test.go`
- `internal/ledger/address.go`, `ledger_balance_test.go`, `ledger_perf_test.go`
- `internal/chain/sol_address_test.go`, `payout_lifecycle_test.go`
- `internal/ledger/deposit_safety_test.go`

Diubah signifikan: `db/queries/quotes_trades.sql`, `chain.sql`, `ledger.sql`; `internal/bot/bot.go`, `flow.go`, `session.go`, `keyboard.go`, `adminflow.go`; `internal/ledger/trade.go`, `deposit.go`, `ledger.go`; `internal/chain/chain.go`; `internal/adminwallet/activity.go`.

Dihapus: `internal/ledger/sellall_test.go`, `sellAllFlow`, `quoteFlow`, `CreateQuoteUnits`, `SellAmountPicker`, `TestSellFlow`.

---

## 6. Masalah Selama Pengerjaan & Solusi

1. **Codex quota mati** (limit s/d 2 Des 2026) — fallback inline implementation per skill Step 5b. Prompt file sudah rinci sehingga eksekusi inline tinggal translasi.
2. **sqlc Row types** (`db.Payout` tidak ada; `GetPayoutRow`/`ClaimQueuedPayoutsRow` per-query) — fix call sites, pelajaran: baca generated `internal/db` di server sebelum tulis code.
3. **ReconcileGlobal absolut FAIL di DB historis kotor** — redesign jadi delta assert (baseline vs sesudah). Keputusan test yang benar: fungsi tetap strict, test historis-aware.
4. **Test tx_hash/userID statis bentrok antar run** (claim limit, deposit conflict, payout user) — pola: unique per run (nanotime/telegramID), limit claim besar.
5. **`python -c` backslash dimakan shell** → string Go corrupt. Solusi: patch tool untuk semua edit Go; `python -c` hanya baca/grep.
6. **Goose tidak support `$$`/multi-`;` function body** — pakai `-- +goose StatementBegin/End`.
7. **Mock SOL lama 30-byte** (audit bonus) — validator strict + vektor valid baru.
8. **Partial unique vs test lama** (`TestParallelActiveQuotesLock` insert 2 active langsung) — test ditulis ulang jadi constraint test.
9. **GitHub push auth transient fail 1x** — retry sukses; token manager valid. Tidak ada yang perlu dibenerin.

---

## 7. Backlog yang Sengaja Tidak Masuk Scope

- Sandbox S1–S6 (Top Up Simulasi production-shaped, durable buy/payout UX, read models, E2E).
- Manual Telegram trial admin/non-admin (TRIAL.md T1–T2) — butuh manusia tap tombol.
- Restart recovery workspace + restore drill — butuh approval + maintenance window.
- Deploy/restart live — butuh approval eksplisit.
- `house:inventory` naming debt (sell-bandar mati, tapi nama akun tetap) — garap saat migrasi account taxonomy.
- Test isolation ephemeral DB per run — sekarang shared workspace + unique keys (cukup; catat).
- Active quote uniqueness: `CreateQuote` expire-otomatis + partial unique = dua lapis (redundan tapi aman).
- Performance benchmark kuantitatif (EXPLAIN ada, tapi belum bench latensi p95).

---

## 8. Langkah Berikutnya

1. Trial manual Telegram Admin Wallet + user flow (T1–T3) dengan akun admin asli + akun non-admin.
2. Restart bot workspace (approval user) → cek persistence feed/detail/idempotency.
3. Restore drill custom-format ke dedicated drill DB (maintenance window + approval).
4. Sandbox S1–S3: adapter contract + schema, topup domain TDD, topup bot UX.
5. Sandbox S4–S6: buy/payout durable TDD, read models, E2E Top Up → Beli → Payout (success/failed/unknown) → recovery.
6. Setelah gate Stage 2 centang semua (STAGE0.md) → Fase 5 QRIS → stabil 2–4 minggu → Fase 5b/6/7/10.

---

## 9. Acceptance Criteria Revisi V.1 — Status Jujur

- [x] Build/test dari clean checkout dengan langkah terdokumentasi (`verify.sh`).
- [x] Cancel aktif mencegah eksekusi (owner-scoped + status cancelled).
- [x] Buy selalu punya durable payout state (queued atomik).
- [x] Failed payout refund tepat sekali (guard refund_journal_id).
- [x] Unknown tidak auto-refund.
- [x] Seluruh jurnal buy/deposit baru balance per asset (validateLegs + trigger).
- [x] Callback private-only dan owner-scoped.
- [x] Flow ALL/MAX tidak dapat dijangkau user (grep BERSIH).
- [x] Slash dan button Jual satu flow.
- [x] Riwayat payout/deposit lifecycle.
- [x] Status bedakan empty/partial/error.
- [x] Quote beli warning + breakdown lengkap.
- [x] Session expired/cancel feedback kontekstual.
- [x] Onboarding `/start` idempotent + setting aktual.
- [x] Audit ganti alamat before/after aktual.
- [x] SOL validator base58/pubkey valid.
- [x] Callback stok admin 4x3 (contract test).
- [x] Broadcast preview+confirm + cancel aman + dedup.
- [x] Full tests hijau di VPS.
- [ ] Smoke bot sandbox manual (butuh manusia).
- [ ] `PROJECT_IDEAS.md` diperbarui hasil aktual (pending — fokus berikutnya bila diminta).
