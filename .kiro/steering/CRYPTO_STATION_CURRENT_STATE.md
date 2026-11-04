# Crypto Station — Current State, Roadmap, Runbook & Gate

> **SATU-SATUNYA SOURCE OF TRUTH PROJECT**
>
> Status: Revisi V.1 Phase 2–9 + correctness follow-up AUTOMATED DONE
> Updated: 2026-11-04
> Code head: `fe1d906`
> Documentation consolidation baseline: `3cb2838`
> DB workspace: v14 (`000014_payout_correctness.sql`)
> Mode: SANDBOX / SIMULASI — bukan transaksi on-chain atau uang nyata
> Runtime/build/test/migrate: `cbt-vps`; lokal hanya coding + Git
> Live production: BELUM disentuh; deploy/restart live wajib approval eksplisit user

Dokumen ini menggantikan seluruh roadmap, current-state, completion report, trial checklist, Stage 0 pack, dan runtime guide sebelumnya. Histori lengkap tetap tersedia di Git sebelum commit konsolidasi. AI berikutnya cukup membaca **AI Quick Resume**, **Current Next Step**, lalu bagian task yang relevan.

---

## Daftar Isi

1. AI Quick Resume
2. Product Model & Locked Decisions
3. Current Architecture & Safety Invariants
4. Current Feature State
5. Progress Timeline
6. Revisi V.1 & Correctness Evidence
7. Manual Trial Checklist
8. Stage 0 & Stage 2 Gate
9. Sandbox S1–S6 Roadmap
10. Runtime & Verification Runbook
11. Migration, Backup & Restore Runbook
12. Risks, Non-goals & Open Decisions
13. Commit Timeline
14. Current Next Step

---

## 1. AI Quick Resume

### Posisi aktual

- Model produk: **transit/non-custodial arah user**.
- Beli: saldo IDR user → quote → trade + payout durable → koin mock/real dikirim ke wallet pribadi user.
- Jual: user mengirim coin ke alamat deposit sistem → konfirmasi → IDR dikreditkan.
- Coin user **tidak mengendap sebagai saldo di bot**; `ALL/MAX` sudah dihapus.
- Admin Wallet v0.14/v0.14.1 selesai: ringkasan aset, deposit stok, feed transaksi, detail, idempotency, audit.
- Revisi V.1 Phase 2–9 selesai otomatis.
- Correctness CF-01–CF-06 selesai; CF-07 clarified; CF-09 corrected.
- Follow-up `fe1d906` menutup gap runtime recovery, claim ownership, atomic confirm+event, dan outer callback guard.
- DB workspace v14; live belum disentuh.

### Verification terakhir

Pada `cbt-vps`, head `fe1d906`:

- targeted chain + bot tests: hijau;
- `go test -count=1 ./...`: seluruh package hijau;
- `go vet ./...`: bersih;
- `go build ./cmd/bot`: sukses;
- `bash scripts/verify.sh --no-db`: `VERIFY OK`;
- DB smoke binary token-kosong: sukses, 39 settings terbaca;
- origin + server bare + workspace sinkron di `3cb2838` sebelum restart.
- 2026-11-04 workspace direstart ke binary build dari code `fe1d906`/repo `3cb2838`; runtime sandbox hidup di tmux `cs-bot` setelah melewati tick recovery 30 detik.
- Telegram `getMe` OK untuk `@cryptostationonly_bot`; webhook kosong; pending update 0; DB tetap v14 dan 7 setting wajib tersedia.

### Belum selesai

1. Manual Telegram trial admin + non-admin.
2. CF-08 historical SOL imbalance decision — butuh approval eksplisit.
3. Restore drill custom-format — butuh maintenance window + approval.
4. Sandbox S1–S6 Top Up production-shaped.
5. Stage 0 eksternal: gateway, node, fee/float, kurs pembanding, economics, hot wallet.
6. Live/real-money/real-chain tetap dilarang sebelum gate penuh.

### Aturan kerja

- Jangan menebak status dari checkbox lama; periksa Git/code aktual.
- Jangan reset/revert perubahan user tanpa izin.
- Semua runtime, generate, migration, test, build, smoke di `cbt-vps`.
- Gunakan TDD: RED → GREEN → full verification.
- Satu scope logis per commit; push origin + server bare; verifikasi workspace.
- Pisahkan klaim: implemented ≠ automated verified ≠ manual passed ≠ workspace restarted ≠ live deployed.

---

## 2. Product Model & Locked Decisions

### 2.1 Goal

Jual-beli BNB/ETH/SOL paling sederhana untuk user Telegram: bayar dengan IDR, aset otomatis dikirim ke wallet pribadi; jual dengan mengirim aset ke deposit sistem. Sandbox membuktikan pricing, ledger, payout, recovery, dan UX sebelum uang/chain nyata.

### 2.2 Model dana dan custody

1. **Transit + auto-kirim.** Model bandar custodial lama dibatalkan.
2. Coin user tidak disimpan di bot.
3. Sistem memegang hot-wallet float operasional per coin.
4. Hot wallet harus **baru dan terpisah** dari wallet utama owner:
   - satu key/alamat EVM untuk BNB BSC + ETH ERC20;
   - satu key/alamat SOL;
   - seed kertas/offline dua salinan;
   - key tidak masuk repo, chat, log, audit, atau DB.
5. Cold owner tetap offline; refill cold→hot manual + jurnal float-in.
6. Stage 1 memakai mock txhash dan mock float dengan label `SIMULASI`.
7. Stage 2 baru memakai `HOTWALLET_KEY_EVM` dan `HOTWALLET_KEY_SOL` dari env file mode 600 setelah approval.

### 2.3 Fiat in/out

- Jalur pertama: QRIS otomatis.
- Bank manual baru setelah QRIS stabil 2–4 minggu.
- Bank manual memakai kode unik, nominal exact, expiry 60 menit, verifikasi mutasi—not foto bukti.
- QRIS/payment asli, disbursement, PIN, name inquiry, cooling-off, dan enforcement fraud belum aktif.

### 2.4 Coin/network

| Asset | Network tetap | Alamat |
|---|---|---|
| BNB | BSC | `0x` + 40 hex |
| ETH | ERC20 | `0x` + 40 hex |
| SOL | Solana | base58 valid, decoded 32 byte |

Tidak ada pilihan multi-network pada MVP.

### 2.5 Pricing

- Coin price: Binance spot `bookTicker`.
- FX Stage 1: Indodax USDT/IDR sementara, refresh 30 detik, fail-closed.
- Quote lock: 30 detik.
- IDR: `BIGINT`; coin/rate: `NUMERIC(36,18)` + `shopspring/decimal`.
- Tidak ada `float64` pada path uang.
- Fee network ditanggung user dan tampil transparan.
- Quote ditolak jika fee >20% units, source stale/down, alamat hilang, float tipis, atau pause aktif.
- Stage 2 membutuhkan source FX pembanding dan divergence threshold.

### 2.6 Risk/pause

- Float di bawah minimum → pause beli coin tersebut; jual/deposit tetap boleh.
- Source stale/divergence, node down, spike, broadcast macet, reconcile gagal → pause beli dan jual.
- Jenis pause tidak boleh digabung menjadi boolean tanpa alasan.

### 2.7 Security baseline

- Private-chat-only.
- Admin allowlist-only; admin Telegram ID saat ini `7254686681`.
- Rate limit message dan callback.
- Audit before/after untuk mutasi admin.
- Secret hanya di env.
- Webhook path acak + secret header saat webhook dipakai.
- Stage 2: PIN, name inquiry, cooling-off, sybil/mule enforcement, signature provider.

---

## 3. Current Architecture & Safety Invariants

### 3.1 Stack

- Go 1.27.3; `go.mod` minimum 1.26
- PostgreSQL 16
- pgx/v5 + sqlc 1.29.0
- Goose 3.28.0
- go-telegram/bot v1.27.0
- shopspring/decimal
- coder/websocket
- satu binary `cmd/bot`

Generated `internal/db/` dan `db/schema/` tidak di-commit; regenerate di VPS.

### 3.2 Module boundaries

```text
cmd/bot            wiring + background jobs
internal/bot       parse/guard/format Telegram, tanpa SQL/hitung uang
internal/settings  settings cache/reload
internal/price     satu-satunya pintu harga/FX
internal/risk      pause/float/risk events
internal/ledger    satu-satunya pintu jurnal/saldo/refund
internal/chain     address, payout/deposit mock, claim/recovery
internal/admin     settings/admin audit
internal/adminwallet read model Wallet & Aset/Transaksi
internal/trial     sandbox metrics/report
migrations         forward SQL migrations
```

### 3.3 Ledger invariants

- Semua mutasi lewat double-entry journal.
- Sum setiap jurnal per asset = 0, ditegakkan `validateLegs` application-level.
- Trigger v13 hanya memastikan entry asset cocok dengan account asset; bukan sum-zero trigger.
- Lock order: Users → LedgerAccounts → Wallets; payout row lock saat refund.
- Wallet adalah cache turunan; jurnal sumber kebenaran.
- Global historical SOL masih imbalance karena jurnal lama; write baru pasca-Phase 3 balance.

### 3.4 Buy/payout lifecycle

```text
quote active
  → confirm owner-scoped
  → trade + payout queued atomik
  → scan/claim atau immediate processor
  → broadcasting
  → confirmed | failed | unknown
```

- `confirmed`: txhash + chain event commit atomik.
- `failed`: refund journal + payout linkage commit atomik, maksimal sekali.
- `unknown`: tidak auto-refund.
- `delayed/queued`: recovery worker memproses ulang.

### 3.5 Durable recovery

Follow-up `fe1d906`:

- runtime sandbox worker tick 30 detik;
- batch maksimal 25;
- scan queued + stale broadcasting;
- atomic `UPDATE ... RETURNING` claim/reclaim;
- `claim_token` ownership wajib sebelum processing;
- stale threshold 5 menit;
- SELECT-only `ClaimQueued` lama dihapus;
- per-item error dicatat tanpa membatalkan sukses item lain.

### 3.6 Refund exactly-once

`RefundPayout(userID,payoutID)` dalam satu DB transaction:

1. lock payout `FOR UPDATE`;
2. validasi owner, status failed, dan linkage;
3. ambil amount IDR dari trade;
4. buat refund journal `ref_table='payouts'`, `ref_id=payoutID`;
5. buat balanced IDR legs;
6. refresh wallet cache;
7. set `refund_journal_id` guarded;
8. commit.

Migration v14 menambah unique partial index satu refund journal per payout.

### 3.7 Callback safety

- Structural validation dilakukan di outer dispatcher sebelum limiter, Telegram I/O, atau dispatch.
- Guard memvalidasi callback ID, From, Message, nested Message, private chat, user status.
- Admin allowlist diperiksa setelah structural/private guard.
- Malformed/stale callback tidak boleh memutasi dan mendapat copy aman bila memungkinkan.

---

## 4. Current Feature State

### 4.1 User

- `/start`: create/repair sandbox grant idempotent, saldo aktual dinamis.
- `/harga`: BNB/ETH/SOL bid/ask fresh-only.
- `/alamat`: alamat per coin; audit old/new aktual; SOL validator strict.
- `/beli`: address required, quote breakdown bruto/fee/net/network/full address/warning.
- Confirm: owner-scoped, cancel-safe, trade+payout durable.
- `/jual`: satu flow deposit transit; slash dan tombol konsisten.
- `ALL/MAX`: ditolak dan tidak dapat menjangkau flow bandar lama.
- `/saldo`: IDR + payout terbaru dengan status Indonesia.
- `/status`: empty/partial/error dibedakan; prioritaskan nonterminal.
- `/riwayat`: unified payout + deposit, WIB, SIMULASI.
- `/batal`: session + quote cancel dengan copy empat kondisi.

### 4.2 Admin Wallet

Panel:

```text
[Wallet & Aset] [Transaksi]
[Spread]        [Limit]
[Pause]         [Broadcast]
[Trial]         [Tutup]
```

Wallet & Aset:

- Ringkasan BNB/ETH/SOL: journal float, minimum, AMAN/TIPIS, network, hot address, fresh IDR estimate.
- Deposit stok: asset → units → reference → preview → Confirm/Cancel.
- Tidak ada jurnal sebelum Confirm.
- Confirm: `house:cold -units`, `house:float +units`, sum nol.
- `tx_ref` global idempotency; immutable identity termasuk admin/asset/network/units/address/ref.
- Cancelled reference tetap reserved.
- Success/cancel audit berada dalam transaction bisnis.

Transaksi:

- Feed 10/25 merge `STOCK_IN`, `PAYOUT_OUT`, `USER_DEPOSIT_IN`.
- Stable sort, timestamp WIB, detail per tipe.
- Full ref/hash hanya di detail.
- Semua layar berlabel SIMULASI.

### 4.3 Admin controls

- spread/limit/float/fee/hot address;
- pause off/float/spike-stale;
- broadcast preview + Confirm/Cancel + dedup 60 detik;
- trial/reconcile report;
- semua allowlist + audit.

---

## 5. Progress Timeline

| Milestone | Status | Evidence |
|---|---|---|
| Fase 0 scaffold | DONE | `8cd6ec3`, DB v1 |
| Fase 1 price + ledger | DONE | `8561ffb`, 7 tests historis |
| Fase 2 quote/trade/risk | DONE | `fbc4456` |
| Fase 3 admin/security | DONE | `7899e07` |
| Fase 4 trial metrics | PARTIAL MANUAL | `05695ad`, automated done |
| Fase 8 transit payout mock | DONE | `caf4d63` series |
| Fase 9 deposit mock/menu/status | DONE | `5ed66f7`, DB v8 |
| Stok v0.13 | DONE | `31d14a4` series, DB v9 |
| Admin Wallet v0.14 | DONE | `94e620f`…`2d43306`, DB v10 |
| Admin Wallet v0.14.1 hardening | AUTOMATED DONE | `8c60ce1` |
| Revisi V.1 Phase 2–9 | AUTOMATED DONE | through `e2d19ee`/reports, DB v13 |
| Correctness CF-01–CF-06 | AUTOMATED DONE | through `84095c8`, DB v14 |
| Follow-up runtime correctness | AUTOMATED DONE | `fe1d906` |
| Manual Telegram acceptance | PENDING | see §7 |
| Sandbox S1–S6 Top Up | NOT STARTED | see §9 |
| Stage 2 real-money/chain | BLOCKED BY GATE | see §8 |

---

## 6. Revisi V.1 & Correctness Evidence

### 6.1 Phase 2–9

| Phase | Hasil |
|---|---|
| 2 | quote cancel status/index; owner-scoped cancel/confirm; callback guard; honest copy |
| 3 | buy/deposit boundary legs; `validateLegs`; global reconciliation function |
| 4 | payout queued atomic; success/failed/unknown/delayed lifecycle |
| 5 | deposit conflict by user/asset/units; atomic address audit; strict SOL validation |
| 6 | sell-bandar/ALL/MAX removed; unified transit sell/history |
| 7 | onboarding repair; safe quote UX; session expiry; lifecycle status copy |
| 8 | admin stock contract tests; broadcast preview/confirm/dedup |
| 9 | targeted wallet aggregation; v13 DB constraints/generated sent units |

### 6.2 Correctness findings

| ID | Status | Resolution |
|---|---|---|
| CF-01 refund crash-window | DONE | journal + linkage one transaction |
| CF-02 wrong refund RefID | DONE | payout ID identity |
| CF-03 non-durable claim | DONE | atomic claim/reclaim + runtime worker |
| CF-04 zero-row transition | DONE | `execrows` + typed stale transition |
| CF-05 ignored transition errors | DONE | errors checked; no silent transition |
| CF-06 callback dereference-before-guard | DONE | outer structural guard |
| CF-07 trigger overclaim | CLARIFIED | trigger asset-match, sum-zero app-level |
| CF-08 historical SOL imbalance | PENDING APPROVAL | proposal below |
| CF-09 stale report | DONE | consolidated document becomes current truth |

### 6.3 Tests khusus

- Atomic refund: concurrency, retry, identity, unknown rejection, crash-window, wrong user.
- Claim/recovery: no overlap, batch no overlap, stale reclaim, queued recovery.
- Transition: illegal state, zero-row, parallel finalizer, idempotent confirmed.
- Follow-up: wrong claim token, recovery finalize, confirmed-event rollback.
- Callback: nil/structure/group/outer-dispatch-before-limiter.

### 6.4 CF-08 historical SOL imbalance

Read-only evidence 2026-11-04:

- BNB/ETH/IDR global sums: 0.
- SOL: `-54.693206730894899100`.
- Last known imbalanced journal: ID 1433, pre-Phase 3.
- 2.800+ newer journals checked: no new imbalance.

Proposal—**belum dieksekusi**:

1. Satu journal `adjust`, `ref_table='system'`, alasan eksplisit.
2. Legs:
   - `external:adjustment -54.693206730894899100 SOL`
   - `house:inventory +54.693206730894899100 SOL`
3. Jangan update/delete jurnal lama.
4. Rollback memakai reversal journal.
5. Verifikasi `ReconcileGlobal` SOL=0 dan user-wallet reconcile tetap hijau.

Status: menunggu user memilih **eksekusi / tunda / tolak**. Jangan mutasi tanpa approval eksplisit.

---

## 7. Manual Trial Checklist

### Prasyarat

- [x] Automated correctness hijau pada `fe1d906`.
- [x] DB workspace v14.
- [x] Workspace pernah direstart pada correctness head awal; feed/data/linkage terbaca.
- [x] User menyetujui dan workspace direstart ke binary `fe1d906` pada 2026-11-04.
- [ ] Akun admin asli tersedia.
- [ ] Akun non-admin kedua tersedia.

### T1 — Admin Wallet

- [ ] `/admin` menampilkan Wallet & Aset dan Transaksi.
- [ ] Ringkasan BNB/ETH/SOL + SIMULASI.
- [ ] Stale price/FX tidak menghasilkan estimasi palsu.
- [ ] Deposit SOL: invalid input ditolak; preview benar; belum ada jurnal.
- [ ] Cancel tidak membuat jurnal dan ref cancelled tidak reusable.
- [ ] Confirm membuat satu cold→float journal.
- [ ] Double-tap/retry tidak menggandakan jurnal/saldo.
- [ ] Same ref + changed units conflict.
- [ ] Feed/detail/10-25 toggle benar.
- [ ] Beli mock muncul `PAYOUT_OUT`; jual mock muncul `USER_DEPOSIT_IN`.
- [ ] Spread/Limit/Pause/Trial/Broadcast tidak regresi.

### T2 — User payout lifecycle

- [ ] `/start`, `/harga`, `/alamat`, `/beli`, `/jual`, `/saldo`, `/status`, `/riwayat`, `/batal` berfungsi.
- [ ] Failed mock payout refund tepat sekali.
- [ ] Double-tap/retry failed tidak double-credit.
- [ ] Unknown tidak refund dan Status memberi warning.
- [ ] Delayed/queued diproses recovery worker setelah tick/restart.
- [ ] Cancel active quote mencegah Confirm.
- [ ] ALL/MAX ditolak dan mengarah ke deposit flow.

### T3 — Non-admin/callback

- [ ] Non-admin admin menu/callback ditolak.
- [ ] Guess detail stock tidak bocor.
- [ ] `admin_audit` mencatat denial.
- [ ] Malformed/group callback tidak mutasi dan tidak panic.

### T4 — Restart persistence

- [x] Restart workspace ke `fe1d906` setelah approval.
- [x] Verifikasi startup, runtime tetap hidup setelah tick recovery, `getMe`, webhook kosong, DB v14, dan setting wajib. Worker tidak mengeluarkan log khusus ketika tidak ada item/error.
- [ ] Feed/detail/references tetap tersedia.
- [ ] Queued/stale payout diproses tanpa overlap.

### T5 — Exit observation

- [ ] Reconcile selisih 0 selama 7 hari untuk write baru (historical CF-08 dipisahkan).
- [ ] Tidak ada quote/payout/refund/stock deposit dikonsumsi dua kali.
- [ ] Pause float dan spike-stale teruji.
- [ ] Semua sandbox UI/record berlabel SIMULASI.

---

## 8. Stage 0 & Stage 2 Gate

### 8.1 Gateway QRIS

Hubungi 2–3 provider secara jujur. Tanyakan:

1. Apakah model topup IDR → beli crypto → auto-send wallet diterima tertulis?
2. Syarat onboarding/legal entity?
3. Fee, settlement, reserve/hold, limit?
4. Webhook event/signature/retry?
5. Paid-after-expiry, mismatch, refund/dispute?
6. Sandbox dan production activation?

| Provider | Diterima tertulis | Fee | Settlement | Limit | Signature | Sandbox | Catatan |
|---|---|---|---|---|---|---|---|
| Xendit | | | | | | | |
| Midtrans/Tripay/Duitku | | | | | | | |
| Provider 3 | | | | | | | |

### 8.2 FX

- [ ] Source utama final.
- [ ] Source pembanding hidup.
- [ ] Timestamp/rate-limit diketahui.
- [ ] Divergence threshold ditetapkan.
- [ ] Production staleness reviewed.

### 8.3 Hot wallet

- [ ] Seed EVM baru offline.
- [ ] Seed SOL baru offline.
- [ ] Dua salinan backup offline.
- [ ] Alamat publik EVM + SOL dicatat.
- [ ] Refill cold→hot procedure + owner/audit.
- [ ] Key env 600 baru saat Fase 10 approved.

### 8.4 Node providers

| Chain | Provider | Rate limit | Fee | WS/webhook | SLA/status |
|---|---|---|---|---|---|
| BSC | | | | | |
| Ethereum | | | | | |
| Solana | | | | | |

- [ ] Broadcast/status/retry method diketahui.
- [ ] Node-down pause dua arah.
- [ ] Confirmation target reviewed: BNB 15, ETH 12, SOL 32.

### 8.5 Float & fee

| Asset | Float minimum | Target | Gas buffer | Refill trigger |
|---|---:|---:|---:|---:|
| BNB | | | | |
| ETH | | | | |
| SOL | | | | |

| Network | Fee source | Normal | Spike | Refresh | Min buy viable |
|---|---|---:|---:|---|---:|
| BSC | | | | | |
| ERC20 | | | | | |
| Solana | | | | | |

### 8.6 Unit economics

```text
Average trade: Rp________
Buy spread: ____%   Sell spread: ____%
Gross revenue/trade: Rp________
Gas + rebroadcast buffer: Rp________
Slippage: Rp________
QRIS/provider: Rp________
Node/VPS/monitoring per trade: Rp________
Support/ops: Rp________
Net/trade: Rp________
Fixed cost/month: Rp________
Break-even: ______ trade/month
Float held during settlement: Rp________
Gas 5x + 3 unknown payouts: Rp________
```

### 8.7 Bank manual (setelah QRIS stabil)

- [ ] Bank/account/name.
- [ ] Kode unik + exact amount.
- [ ] Expiry 60 menit.
- [ ] Mutasi-only verification.
- [ ] SLA approval/refund.
- [ ] One active invoice/user.

### 8.8 Operational readiness

- [ ] Permanen `cs-bot.service`.
- [ ] Scheduled custom-format backup.
- [ ] Restore drill sukses.
- [ ] Alert stale/reconcile/float/node/unknown.
- [ ] Public dispute SLA.
- [ ] Manual trial + 7-day observation.
- [ ] Sandbox S1–S6 complete.

### 8.9 Stage 2 gate—semua wajib

- [ ] Gateway QRIS setuju tertulis.
- [ ] Hot wallet baru + seed offline + public addresses.
- [ ] Node provider tiap chain.
- [ ] Float/refill procedure.
- [ ] Main + comparison FX, divergence threshold.
- [ ] Real network fee schedule + 20% reject.
- [ ] Unit economics viable.
- [ ] Sandbox exit/restart recovery/manual trial.
- [ ] Backup/restore/alerting.
- [ ] Approval eksplisit user masuk Stage 2.

Gate operasional bukan pernyataan legal permission.

---

## 9. Sandbox S1–S6 Roadmap

Belum dimulai. Jangan melompat ke real gateway/chain.

### S1 — Contract & schema

- payment adapter interface;
- topup invoices/payment events migrations;
- payout lifecycle compatibility/backfill;
- typed domain errors;
- compile + migration from v14.

### S2 — Topup domain TDD

- create invoice;
- process event exactly once;
- paid/paid_late/mismatch/needs_refund/expired;
- balanced IDR journal;
- concurrency/idempotency tests.

### S3 — Topup bot UX

- Top Up button;
- amount picker/input;
- invoice screen;
- simulate event via same processor as real webhook;
- status/detail/cancel;
- semua SIMULASI.

### S4 — Durable buy/payout TDD refinement

- production-shaped adapter boundary;
- success/failed/unknown/delayed;
- refund once;
- recovery/restart;
- failure-injection/concurrency.

### S5 — User read models

- Saldo;
- Status priority;
- unified topup/payout/deposit Riwayat;
- detail and partial/error states.

### S6 — E2E

1. New user.
2. Topup Rp100k once.
3. Duplicate event no extra credit.
4. Buy SOL Rp50k confirmed mock.
5. Failed refund once.
6. Unknown no refund.
7. Expired invoice.
8. Paid late exact.
9. Paid late limit exceeded → needs_refund.
10. Amount mismatch no credit.
11. Restart queued payout recovery.
12. Saldo/Status/Riwayat consistent.

---

## 10. Runtime & Verification Runbook

### 10.1 Environment

```text
DATABASE_URL=[REDACTED]
BOT_TOKEN=[REDACTED atau kosong untuk DB smoke]
ADMIN_TELEGRAM_IDS=7254686681
HOTWALLET_KEY_EVM=belum dipakai di sandbox
HOTWALLET_KEY_SOL=belum dipakai di sandbox
```

### 10.2 Sync workspace

```bash
cd /home/ubuntu/crypto-station
git fetch origin main
git reset --hard FETCH_HEAD
export PATH=$PATH:/usr/local/go/bin:/home/ubuntu/go/bin
sqlc generate
```

Generated artifacts dapat muncul untracked di VPS (`internal/db`, `db/schema`, binary). Jangan commit.

### 10.3 Verify no DB mutation

```bash
cd /home/ubuntu/crypto-station
export PATH=$PATH:/usr/local/go/bin:/home/ubuntu/go/bin
bash scripts/verify.sh --no-db
```

Melakukan version checks, schema/sqlc drift, gofmt, vet, tests, build.

### 10.4 Full workspace verification

Hanya DB workspace/test:

```bash
export DATABASE_URL="[REDACTED]"
export VERIFY_ALLOW_DB_MIGRATION=YES
bash scripts/verify.sh --full
```

### 10.5 Manual commands

```bash
sqlc generate
go vet ./...
go test -count=1 ./...
go build -o /tmp/cs-bot ./cmd/bot
BOT_TOKEN= DATABASE_URL="[REDACTED]" /tmp/cs-bot
```

### 10.6 Migration

```bash
goose -dir ./migrations postgres "$DATABASE_URL" status
goose -dir ./migrations postgres "$DATABASE_URL" up
```

Current workspace: v14. v10 irreversible by policy; later migrations forward/additive.

### 10.7 Git workflow

```bash
git add <scope-files>
git commit -m "type(scope): message"
git push origin main
GIT_SSH_COMMAND="ssh -o ClearAllForwardings=yes" git push server main
ssh -o ClearAllForwardings=yes cbt-vps \
  'cd /home/ubuntu/crypto-station && git fetch origin main && git reset --hard FETCH_HEAD'
```

`ClearAllForwardings=yes` menghindari konflik local forward port 8080 dari SSH config.

### 10.8 Restart policy

- Workspace restart: perlu approval user.
- Live restart/deploy: perlu approval eksplisit terpisah.
- Sebelum restart: backup sesuai risiko, build binary, catat old PID/head.
- Sesudah restart: verify startup/getMe/log/head, lalu trial.

---

## 11. Migration, Backup & Restore Runbook

### 11.1 Migration timeline

| Version | Scope |
|---|---|
| v1 | users/settings |
| v2 | ledger/wallet/price |
| v3 | quotes/trades/risk |
| v4 | admin audit/rate limit |
| v5 | exposure snapshots |
| v6 | wallet addresses/payout/deposit |
| v7–v8 | deposit credit/journal types |
| v9 | hot addresses/stock |
| v10 | admin stock deposits |
| v11 | quote cancelled |
| v12 | payout lifecycle |
| v13 | ledger/domain hardening |
| v14 | refund identity + durable claim metadata |

### 11.2 Backup custom-format

```bash
export DBURL="[REDACTED]"
BACKUP="/tmp/cs-backup-pre-<label>-$(date +%Y%m%d-%H%M).dump"
pg_dump "$DBURL" --format=custom --no-owner --no-privileges --file "$BACKUP"
pg_restore --list "$BACKUP" >/dev/null
```

Plain SQL bukan prosedur restore yang direkomendasikan.

### 11.3 Restore drill—destruktif hanya terhadap DB drill

Wajib approval + maintenance window:

```bash
export MAINT_URL="postgres://cs:[REDACTED]@localhost:5432/postgres?sslmode=disable"
export RESTORE_DB="cryptostation_restore_drill"
export BACKUP="/tmp/cs-backup-pre-<label>-<ts>.dump"

psql "$MAINT_URL" -v ON_ERROR_STOP=1 \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='${RESTORE_DB}' AND pid<>pg_backend_pid();"
psql "$MAINT_URL" -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS ${RESTORE_DB};"
psql "$MAINT_URL" -v ON_ERROR_STOP=1 \
  -c "CREATE DATABASE ${RESTORE_DB};"

pg_restore --exit-on-error --no-owner --no-privileges \
  --dbname="postgres://cs:[REDACTED]@localhost:5432/${RESTORE_DB}?sslmode=disable" \
  "$BACKUP"

export DRILL_URL="postgres://cs:[REDACTED]@localhost:5432/${RESTORE_DB}?sslmode=disable"
goose -dir ./migrations postgres "$DRILL_URL" status
psql "$DRILL_URL" -v ON_ERROR_STOP=1 -tAc \
  "SELECT count(*) FROM admin_stock_deposits WHERE status='recorded_mock' AND journal_id IS NULL"
psql "$DRILL_URL" -v ON_ERROR_STOP=1 -tAc \
  "SELECT j.id, count(e.id) FROM journals j LEFT JOIN ledger_entries e ON e.journal_id=j.id WHERE j.type='float-in' GROUP BY j.id HAVING count(e.id)<>2"
```

Expected:

- Goose status v14.
- Recorded stock deposit tanpa journal = 0.
- Float-in bukan dua legs = 0 rows.
- Feed/detail readable.
- Refund payout identity unique.
- Claim metadata/index tersedia.

Jangan restore ke DB utama sebelum drill hijau, backup verified, bot stopped, dan approval eksplisit.

### 11.4 Health queries

```sql
SELECT j.id, count(e.id)
FROM journals j
LEFT JOIN ledger_entries e ON e.journal_id=j.id
WHERE j.type='float-in'
GROUP BY j.id
HAVING count(e.id)<>2;

SELECT count(*)
FROM admin_stock_deposits
WHERE status='recorded_mock' AND journal_id IS NULL;

SELECT ref_id, count(*)
FROM journals
WHERE type='refund' AND ref_table='payouts'
GROUP BY ref_id
HAVING count(*)>1;
```

---

## 12. Risks, Non-goals & Open Decisions

### Risks aktif

- Gateway dapat menolak model crypto.
- Salah alamat irreversible.
- Hot key theft → batasi float + env 600 + offline seed.
- Node down/gas spike/nonce stuck.
- Payout unknown/support burden.
- Historical SOL imbalance CF-08.
- Shared workspace DB tests butuh unique per-run IDs dan delta assertions.
- Bot workspace belum service systemd permanen.

### Non-goals saat ini

- Real QRIS/bank/disbursement.
- Real chain/node/private key.
- KYC penuh/referral/order book/multi-network.
- HSM/KMS sebelum pasca-MVP.
- Historical journal update/delete.
- Live deploy tanpa approval.

### Open decisions

1. CF-08: execute/tunda/tolak adjustment.
2. Payment gateway.
3. Node provider per chain.
4. Float minimum/replenishment.
5. Fee source/schedule.
6. FX primary/comparison/divergence.
7. Exact spread.
8. Confirmation counts.
9. Public dispute SLA.
10. Bank manual account/SLA.
11. Public hot addresses.
12. Permanent systemd unit.

---

## 13. Commit Timeline

Key commits:

- `8cd6ec3` Fase 0
- `8561ffb` Fase 1
- `fbc4456` Fase 2
- `7899e07` Fase 3
- `05695ad` Fase 4 automated
- `5ed66f7` transit/deposit Fase 9
- `31d14a4`… stock v0.13
- `94e620f`…`2d43306` Admin Wallet v0.14
- `8c60ce1` Admin Wallet v0.14.1 hardening
- `1a117cb` Phase 2 revision base
- `360f5ce` Phase 3 ledger invariant
- `be020fc` Phase 4 payout base
- `f2afeec` Phase 5 safety base
- `300b7fa` Phase 6 custodial removal
- `e7e7ede` Phase 7 UX
- `a5fb336` Phase 8 admin
- `16f7da3` Phase 9 DB hardening
- `e8961fe` atomic refund
- `9914240` durable claim primitives
- `c78b51b` guarded transitions
- `2ceaefa` callback guard
- `84095c8` correctness closure/restart evidence
- `fe1d906` runtime recovery + atomic confirm/event + outer guard
- `c0c8700` follow-up docs
- `bdcd5e4` pre-consolidation docs sync

---

## 14. Current Next Step

Urutan aman:

1. **Jalankan manual Telegram trial §7** dengan admin dan non-admin. Workspace sandbox sudah hidup di binary `fe1d906`.
2. **Pilih CF-08:** execute / tunda / tolak. Adjustment tidak boleh berjalan tanpa jawaban eksplisit.
3. **Restore drill** bila maintenance window disetujui.
4. Setelah manual acceptance, kickoff **Sandbox S1** dengan reconciliation terhadap code HEAD dan TDD.
5. Paralel: user menyelesaikan Stage 0 eksternal §8.

Dilarang:

- memulai uang/QRIS/chain/key nyata sebelum Stage 2 gate penuh;
- menganggap automated test sebagai manual Telegram PASS;
- mengubah historical journals langsung;
- restart/deploy live tanpa approval eksplisit.
