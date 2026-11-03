# Laporan Perbaikan Revisi V.1 — Correctness Fix (CF-01 s/d CF-09)

> Tanggal: 2026-11-04
> Audited implementation head: `84095c8`; follow-up correctness head: `fe1d906` (origin + server bare sinkron)
> DB workspace: v14 (`000014_payout_correctness.sql`)
> Mode: SANDBOX / SIMULASI — live tidak disentuh
> Dokumen pelaksana: `.kiro/steering/REVISI_V1_CORRECTNESS_FIX_PLAN.md` (Task 0–10)

Latar: audit koreksi atas completion report Phase 2–9 nemu 9 gap (2 P0, 4 P1, 2 P2, 1 P3).
Semua P0/P1 sudah fixed + hijau. P2/CF-07 clarified, CF-08 proposal menunggu approval,
CF-09 corrected. Sisa: trial tap Telegram manual (Task 9) + keputusan CF-08.

---

## 1. Temuan dan Perbaikan

### CF-01 (P0) — Refund double-credit via crash window

- Masalah: `RefundPayoutTx` commit jurnal sendiri, linkage `refund_journal_id` via call
  terpisah (`bot.go`). Crash di antaranya = jurnal yatim, retry bikin jurnal kedua.
- RED proof: concurrent 2 goroutine → saldo 900000 jadi 1100000 (double-credit).
- Fix: `RefundPayout(userID, payoutID)` — lock `FOR UPDATE` + jurnal + linkage dalam
  1 tx. Typed result: `Created` / `AlreadyDone` / `NotRefundable` / `RaceLost`.
  Unique index `idx_journals_refund_payout_unique` sebagai backstop DB.
- File: `internal/ledger/payout_refund.go` (baru), `bot.go` pindah ke API baru.

### CF-02 (P0) — Refund journal RefID salah

- Masalah: `RefTable='payouts'` tapi `RefID=tradeID` (harusnya payoutID).
- Fix: jurnal refund sekarang `ref_table='payouts'`, `ref_id=payoutID`. Chain test lama
  yang masih pakai API lama ikut dimigrasi (namespace tradeID vs payoutID sempat
  tabrakan angka di DB shared → 107 jurnal artefak test dibersihkan, backup dulu).

### CF-03 (P1) — Worker claim tidak durable

- Masalah: `ClaimQueued` cuma `SELECT FOR UPDATE SKIP LOCKED` — lock lepas habis
  statement, dua worker bisa menang bareng (RED proof: wins=2).
- Fix: `ClaimPayout` fresh-queued-only (`UPDATE ... RETURNING`, kalah =
  `ErrPayoutClaimLost`), reclaim stale broadcasting dipisah
  (`ReclaimStaleBroadcastingAtomic`, threshold 5 menit), `RecoveryScan` = queued + stale.
- File: `internal/chain/claim.go`, `claim_test.go` (baru).

### CF-04/CF-05 (P1) — Zero-row dianggap sukses + error diabaikan

- Masalah: semua `MarkPayout*` pakai `:exec` tanpa cek rows; `_ =` di unknown/failed.
- Fix: helper `mustWin(n, err)` — 0 row = `ErrPayoutStaleTransition`. Semua `_ =`
  transition dihapus. Chain event cuma ditulis bila confirm benar-benar menang.
- Bonus bug ketahuan gara-gara `mustWin`: `unknown → broadcasting` ternyata tidak
  didukung query lama (sebelumnya lolos diam-diam). Fix: `MarkPayoutBroadcasting`
  allow `unknown` sebagai recovery path (existing test `UnknownNoRefund` jadi saksi).

### CF-06 (P1) — Dispatcher dereference sebelum guard

- Masalah: `handleCallback` akses `cb.ID`/`cb.Data` + dispatch `t:`/`a:` sebelum
  validasi; `handleAdminCallback` langsung `cb.From.ID`; guard klaim nil-safe
  terlalu luas.
- Fix: `ValidateCallbackStructure` paling awal (sebelum AnswerCallback + dispatch),
  `GuardCallback` nil-check semua level + private + user, adminflow guard-dulu
  baru allowlist. Ketahuan: `From` itu value bukan pointer (fix vet).
- Test: `GuardCallbackNil` (silent vs loud), `GroupRejected`, `StructureMatrix`.

### CF-07 (P2) — Wording trigger v13 overclaim

- Klarifikasi (bukan bug): trigger v13 = asset-match + domain checks, BUKAN sum-zero.
  Sum-zero per jurnal = `validateLegs` app-level. Tidak perlu migration baru.

### CF-08 (P2) — Imbalance historis SOL -54.693206730894899100

- Evidence read-only: BNB/ETH/IDR = 0; jurnal imbalance terakhir id 1433
  (2026-11-03 pagi, bug A3/A4 era pre-Phase 3); 2800+ jurnal baru 0 imbalance.
- Proposal (BELUM dieksekusi): satu jurnal `adjust` baru (jangan update/delete lama),
  legs `external:adjustment` + `house:inventory`, + rollback plan.
  Detail di `REVISI_V1_COMPLETION_REPORT.md` §10.
- [!] Menunggu approval user. Jangan eksekusi tanpa itu.

### CF-09 (P3) — Report stale

- Header completion report (head `e2d19ee`/DB v13) dikoreksi sebagai snapshot
  2026-11-03 + addendum §10. `PROJECT_IDEAS.md` + `revisi_system_V.1.md` sinkron
  ke DB v14 + pointer FIX_PLAN.

---

## 2. Verifikasi

- `scripts/verify.sh --no-db` = VERIFY OK (gofmt/vet/tests/build bersih).
- Targeted 19 GREEN: 6 refund atomic, 8 claim/transition, 5 callback guard.
- Full suite hijau (`-p 1` + default, 2x), DB v14, build OK, secret-scan bersih.
- Bot workspace direstart ke head terbaru (PID 1894949, sandbox) + backup pre-restart OK.
- Recovery via DB: feed utuh (763 payouts/147 deposits), 24 refund linkage unik
  (0 double), 318 queued siap recovery scan.

---

## 3. Yang Belum (butuh user)

1. **Task 9 tap Telegram manual** — bot jalan, tinggal tap: T1 admin (deposit stok,
   preview/confirm, double-tap idempotency, beli/jual mock), T2 non-admin denial,
   failed refund 1x, unknown warning, callback malformed/group, feed 10/25.
2. **CF-08 adjustment** — eksekusi / tunda / tolak proposal di atas.
3. **4 MD work user** (`TRIAL`, `ADMIN_WALLET_CURRENT_STATE`, `Fase 0-README`,
   `STAGE0`) masih versi lama v0.14.1/v10 — AI tidak menyentuh, sinkronisasi milik user.
4. Sandbox S1–S6, restore drill, live — tetap dilarang tanpa approval eksplisit.

---

## 4. Riwayat Commit Sesi Ini

| Commit | Isi |
|---|---|
| `4eeabfb` | Task 0 reconcile |
| `d128f16` | Task 1 RED 5/5 FAIL |
| `535bfc1` + `6fbd186` | Task 2 migration v14 |
| `e8961fe` | Task 3 atomic refund |
| `9914240` | Task 4 durable claim |
| `c78b51b` | Task 5 guarded transition |
| `2ceaefa` + `08e6876` | Task 6 callback guard |
| `ac072ae` | Task 7 report clarify |
| `19a591a` | Task 8 full verify |
| `d769153` + `50ad758` | Task 10 closure |
| `84095c8` | Task 9 restart + recovery |
| `fe1d906` | Follow-up audit: recovery worker terintegrasi, claim ownership, confirm+event atomik, callback guard di outer dispatcher |

---

## 5. Follow-up Audit Fix (`fe1d906`)

Review setelah laporan awal menemukan tiga gap tambahan. Semuanya sudah ditutup dan diverifikasi di VPS:

1. **Durable claim sebelumnya baru primitive/test.** Sekarang runtime sandbox menjalankan recovery batch tiap 30 detik melalui `RunRecoveryBatch`: scan → atomic claim/reclaim → token-owned processing. API lama `ClaimQueued` dan query SELECT-only dihapus.
2. **Confirmed payout + chain event sebelumnya dua statement non-atomik.** Sekarang keduanya commit dalam satu DB transaction. Fault-injection test membuktikan kegagalan event me-rollback status confirmed dan txhash.
3. **Outer dispatcher callback sebelumnya menyentuh rate limiter sebelum structural guard.** Sekarang `ValidateCallbackStructure` berjalan sebelum limiter, Telegram I/O, atau dispatch namespace.

Regression tests baru:

- wrong claim token tidak dapat memproses payout;
- queued recovery melakukan claim dan finalize;
- forced chain-event failure me-rollback confirmed payout;
- invalid callback ditolak sebelum rate limiter.

Verifikasi head `fe1d906` di `cbt-vps`:

- targeted chain + bot: hijau;
- full `go test -count=1 ./...`: semua package hijau;
- `go vet ./...`: bersih;
- `go build ./cmd/bot`: sukses;
- `bash scripts/verify.sh --no-db`: `VERIFY OK`;
- origin + server bare + workspace sinkron ke `fe1d906`;
- live production tetap tidak disentuh.

Sisa status tidak berubah: manual Telegram tap masih pending dan CF-08 adjustment tetap menunggu approval eksplisit.
