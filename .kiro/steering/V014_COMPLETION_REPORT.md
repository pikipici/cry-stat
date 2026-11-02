# Laporan Penyelesaian — Admin Wallet Operations Plan (v0.14-admin-wallet)

> Plan: `.kiro/steering/ADMIN_WALLET_OPERATIONS_PLAN.md` (816 baris, T1–T9)
> Status: DONE — terimplementasi + terverifikasi workspace 2026-11-02
> Deploy live: PENDING persetujuan user (bot live masih v0.13, belum restart)
> Bot uji: @cryptostationonly_bot (workspace, sandbox SIMULASI)

## 1. Ringkasan Eksekutif

Panel admin naik dari tombol `📦 Stok` (pengaturan float/fee/hot/topup mentah)
menjadi panel operasional wallet:

- `💼 Wallet & Aset` — ringkasan 3 coin + deposit stok via preview+Confirm + submenu float/fee/hot.
- `🧾 Transaksi` — feed gabungan 10/25 + detail per transaksi.
- Deposit stok idempotent (double-tap Confirm = 1 jurnal).
- Semua layar jujur: label `SIMULASI`, saldo dari jurnal sandbox, bukan on-chain.
- Tombol `📦 Stok` lama diabsorpsi (callback `a:stock:*` tetap kompatibel).

## 2. Hasil Verifikasi (workspace cbt-vps)

| Item | Hasil |
|---|---|
| DB migration | v10 `000010_admin_stock_deposits` ✅ |
| `go vet ./...` | bersih ✅ |
| `go test -count=1 ./...` | 62 `--- PASS`, 0 FAIL ✅ |
| `go build ./cmd/bot` | OK, binary `/tmp/cs-bot-v014` (17 MB) ✅ |
| Smoke `BOT_TOKEN=` + DATABASE_URL | `mode=sandbox settings_rows=39` exit 0 ✅ |
| Bot workspace restart | PID 1475267, hidup 100+ detik, log `fase 4 start mode=sandbox` ✅ |
| `getMe` | @cryptostationonly_bot (id 8898894252) ✅ |
| Trial service-level 18 langkah | PASS semua (langkah 18 handler-Telegram, manual) ✅ |

Detail trial (output test, 2026-11-02 23:5x WIB):

1. Ringkasan 3 aset urut BNB/ETH/SOL — OK
2–3. Status AMAN/TIPIS sesuai saldo vs min — OK
4. Tanpa WS tick → EstimateOK=false, IDR=0 (fail-closed, tanpa fallback) — OK
5–6. Network Solana, hot `MOCK-DEPOSIT-SOL`, saldo 6267.84 SOL — OK
7. Units `0/-5/abc/kosong` invalid (retry-able) — OK
8. Units 25 valid — OK
9. Ref `MOCK-TRIAL-001` — OK
10. Cancel deposit #1 → saldo tetap 6267.84 (tanpa jurnal) — OK
11–12. Confirm deposit #2 jurnal #832 → saldo 6292.84 (+25 tepat) — OK
13. Double-tap → AlreadyDone, jurnal tetap #832, saldo tetap — OK
14. Feed 10 item, top STOCK_IN #2 — OK
15. Detail: jurnal #832, admin 7254686681, ref MOCK-TRIAL-001 — OK
16–17. Kinds di feed: STOCK_IN + USER_DEPOSIT_IN (PAYOUT_OUT kosong, belum ada buy) — OK
18. Non-admin reject + audit — level handler Telegram, verifikasi manual via akun kedua

## 3. Commit (dual push origin + server)

| Head | Isi |
|---|---|
| `94e620f` | T2: migrasi 000010 + 9 query sqlc |
| `24afdfd` | T3: ledger RecordAdminStockDeposit idempotent + tests |
| `b9b1fe5` | T4+T5: adminwallet summary + activity feed |
| `57c597b` | fixup: pgtype.Timestamptz `.Time` |
| `a2832cb` | fixup: drop unused db import |
| `2d43306` | T6+T7: bot flow + views (582 baris, 8 file) |
| `fd4cf7f` | T9 docs: PROJECT_IDEAS v0.14 |

## 4. File Baru / Diubah

Baru: `migrations/000010_admin_stock_deposits.sql`,
`db/queries/admin_wallet.sql` (+2 query detail),
`internal/ledger/stock_deposit.go` (+test),
`internal/adminwallet/service.go`, `activity.go` (+tests),
`internal/bot/adminwallet.go`, `adminwallet_format.go`, `adminwallet_test.go`.

Ubah: `session.go` (2 step + 4 field), `keyboard.go` (panel baru),
`adminflow.go` (delegasi 1 hunk), `bot.go` (steps + Deps wiring),
`cmd/bot/main.go` (wire Wallet), `float.go` (deprecated note),
`PROJECT_IDEAS.md` (header + bab 18 + locked #49 + roadmap),
plan ini (status DONE + checklist 9/10).

## 5. Test Matrix (hasil aktual)

- 9.1 Data/ledger: BNB/ETH/SOL valid, desimal 18 digit, zero/negatif/aset/ref-kosong ditolak, duplicate idempotent, sum nol, rollback atomik — ✅ (T3 tests + trial 10–13)
- 9.2 Asset summary: AMAN/TIPIS, hot-kosong field, fresh/stale estimate, tanpa float64 — ✅ (T4 tests + trial 1–4; hot display di formatter Task 7)
- 9.3 Activity feed: merge 3 jenis, order stabil, limit, empty-shape, detail valid/invalid, txref penuh di detail — ✅ (T5 tests + trial 14–17)
- 9.4 Bot/security: allowlist callback+message, deny audit, tanpa-hot ditolak, tanpa-mutasi-sebelum-Confirm, double-Confirm 1 jurnal, cancel bersih, SIMULASI — ✅ (pure tests + trial 7/10/13; langkah 18 manual)
- 9.5 Regression: suite lama hijau, payout/deposit mock jalan, float guard pause, spread/limit/pause/broadcast/trial jalan, `/float` konsisten — ✅ (full suite 62 PASS)

## 6. Acceptance Criteria Final (cek satu per satu)

- [x] Saldo jurnal + min + status + address + network + estimasi fresh terlihat admin.
- [x] Deposit stok via tombol preview+Confirm.
- [x] Idempotent + double-entry (trial jurnal #832 tunggal).
- [x] Feed gabungan terlihat.
- [x] Detail tiap transaksi terbuka.
- [x] Allowlist + audit (success/cancel/deny).
- [x] SIMULASI jujur, tanpa klaim on-chain.
- [x] Migration v10 sukses.
- [x] Seluruh test lama+baru hijau (62 PASS).
- [x] Build + DB smoke hijau.
- [x] Trial service-level selesai (18 langkah, bukti output di §2).
- [x] PROJECT_IDEAS diperbarui sesuai hasil aktual.
- [ ] Trial Telegram-tap manual (18 langkah Bagian 10 via tombol asli) — menunggu interaksi admin di @cryptostationonly_bot.
- [ ] Persetujuan deploy/restart live.

## 7. Masalah Selama Eksekusi + Solusi

1. Codex config lokal hardcode `model = "gpt-6-sol"` → API 400 model_not_found.
   Solusi: override `-m gpt-5.6-terra` tiap run. Saran permanen: ganti model di `~/.codex/config.toml`.
2. `sqlc generate` gagal `relation admin_stock_deposits does not exist` — `db/schema/` (generated, gitignored) belum terima 000010.
   Solusi: `cp migrations/000010 db/schema/` di VPS lalu regenerate. Catatan workflow: tiap migrasi baru wajib copy ke `db/schema/` sebelum `sqlc generate`.
3. Vet gagal `pgtype.Timestamptz as time.Time` (6 titik activity.go).
   Solusi: `.Time` conversion, commit `57c597b`.
4. Vet gagal `db imported and not used` (activity.go).
   Solusi: drop import, commit `a2832cb`.
5. Bot lama v0.13 sudah mati saat restart (tidak ada proses) — start fresh, bukan replace.
6. SSH wrapper Hermes inject LocalForward 8080 yang konflik → noise `Address already in use` di tiap output, perintah tetap jalan. Flag `-o ExitOnForwardFailure=no` mengurangi, tidak hilang total.

## 8. Backlog (bukan sekarang, milik Fase 10 / Stage 2)

Tetap sesuai plan §13: baca saldo on-chain, watcher, auto-detect, validasi txhash ke node, private key, broadcast, reconcile chain, gateway/QRIS/bank/fiat, ubah ledger utama, simpan secret. Semua dilarang di v0.14 dan tidak dikerjakan.

## 9. Langkah Berikutnya (urut)

1. Trial Telegram-tap manual 18 langkah via @cryptostationonly_bot (admin 7254686681): Admin → Wallet & Aset → Ringkasan → Deposit Stok SOL → units invalid/valid → ref → Batal (saldo tetap) → ulang Confirm → double-tap → Transaksi → detail → beli+jual mock → cek feed → non-admin tap (akun kedua).
2. Owner bikin hot BARU manual (seed offline, 2 alamat publik) → sethot-in.
3. Trial Fase 9 + T2 ulang lawan transit.
4. Unit systemd permanen cs-bot.service (bot sekarang background SSH, mati kalau sesi putus/reboot).
5. Stage 0 → v0.15. Gate Stage 2 → Fase 5 QRIS.
6. Deploy/restart LIVE hanya atas persetujuan eksplisit user.
