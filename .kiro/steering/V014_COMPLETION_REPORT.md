# Laporan Penyelesaian — Admin Wallet Operations Plan (v0.14-admin-wallet)
# + Koreksi Revisi v0.14.1 (R1–R9, update 2026-11-03)

> Plan original: `.kiro/steering/ADMIN_WALLET_OPERATIONS_PLAN.md` (DONE workspace)
> Revisi: `.kiro/steering/ADMIN_WALLET_V014_REVISION_PLAN.md` (R1–R7 DONE, R8 pending user, R9 = dokumen ini)
> Deploy live: BELUM — tanpa approval eksplisit (bot live masih v0.13)
> Bot uji workspace: @cryptostationonly_bot, binary /tmp/cs-bot-v0141, PID 1487639, sandbox SIMULASI

## 0. Koreksi Klaim Report v0.14 (R9, evidence aktual)

| Klaim lama | Koreksi |
|---|---|
| "62 tests hijau" | Angka itu = jumlah `--- PASS` subtest+func pada run 2026-11-02. Run 2026-11-03: **88 `--- PASS`, 4 SKIP, 0 FAIL**. SKIP = 4 test sell-bandar era pivot (by design, `t.Skip`), bukan kegagalan. |
| "24 test funcs" | Perkiraan kasar. Jangan pakai jumlah func sebagai metrik — pakai `--- PASS` per run + daftar test baru §2. |
| "Trial 18 langkah PASS" | Itu **service-level** (function call langsung, file temp, sudah dihapus). **Bukan** trial tombol Telegram. R8 (Telegram-tap 21+4+5) BELUM jalan — menunggu user §6. |
| "Audit success/cancel" (implisit durable) | Salah — audit v0.14 pakai `_ = Audit(...)` setelah mutasi (R2 temuan). Sudah diperbaiki R4: audit di dalam tx, failure = rollback. |
| "Idempotent" | Benar hanya untuk retry identik sequential. R2 menutup lubang: payload/admin berbeda → typed conflict, cancelled → typed error, cross-admin tanpa leak, concurrency 2-goroutine terbukti. |
| Handler security "diuji" | v0.14 hanya pure keyboard/string tests. R5 menambah decision-boundary tests (allowlist/routing/copy). Full Telegram-API mock BELUM (disengaja — 3 call sites, boundary diuji). |
| Reproducibility | v0.14 butuh `cp migrations/000010 db/schema/` manual. R6 menutup: `scripts/verify.sh` + clean-clone VERIFY OK. |

## 1. Ringkasan Eksekutif

Panel admin naik dari tombol `📦 Stok` (pengaturan float/fee/hot/topup mentah)
menjadi panel operasional wallet:

- `💼 Wallet & Aset` — ringkasan 3 coin + deposit stok via preview+Confirm + submenu float/fee/hot.
- `🧾 Transaksi` — feed gabungan 10/25 + detail per transaksi.
- Deposit stok idempotent (double-tap Confirm = 1 jurnal).
- Semua layar jujur: label `SIMULASI`, saldo dari jurnal sandbox, bukan on-chain.
- Tombol `📦 Stok` lama diabsorpsi (callback `a:stock:*` tetap kompatibel).

## 2. Hasil Verifikasi (workspace cbt-vps)

| Item | Hasil v0.14 (2026-11-02) | Hasil revisi v0.14.1 (2026-11-03) |
|---|---|---|
| DB migration | v10 ✅ | v10, irreversible by policy ✅ (backup 3.1 MB) |
| `go vet ./...` | bersih ✅ | bersih ✅ |
| `go test -count=1 ./...` | 62 PASS, 0 FAIL | **88 PASS, 4 SKIP (pivot), 0 FAIL** ✅ |
| `go build` | OK ✅ | OK (`/tmp/cs-bot-v0141`, 17 MB) ✅ |
| Smoke | exit 0, 39 settings ✅ | exit 0 via verify.sh ✅ |
| `scripts/verify.sh` | — (belum ada) | VERIFY OK workspace + clean-clone ✅ |
| gofmt gate | — | bersih (5 file drift diperbaiki) ✅ |
| Bot workspace | PID 1475267 (v014) | PID 1487639 (v0141, Fase 4 sandbox) ✅ |
| `getMe` | @cryptostationonly_bot ✅ | sama ✅ |
| Trial service-level | 18 langkah PASS | tetap valid (kode kompatibel) |
| Trial Telegram-tap | BELUM | **BELUM — menunggu user (§6)** |

Test baru revisi (19 funcs, semua hijau 2026-11-03):

- R2/R3 (`stock_deposit_conflict_test.go`, 9): RetryIdentical, DifferentAdmin/Asset/Units/Address conflict, CancelledRef, ConcurrentConfirmOneJournal (2-goroutine), EighteenDecimalCanonical, NoLeak.
- R4 (`stock_deposit_audit_test.go`, 4): AuditFailureRollsBackAll, CancelAuditFailureRollsBack, WritesSuccessAudit (1 row, tanpa alamat), CancelUnauthorized.
- R5 (`wallet_sender_test.go`, 6): NonAdminDenied (8 callback), AdminRouting (10), MalformedSilent, NotReady, ConfirmCopyMapping, ConfirmCopyNoLeak.

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

## 3. Commit (dual push origin + bare server)

v0.14 (7): `94e620f` T2 · `24afdfd` T3 · `b9b1fe5` T4+T5 · `57c597b`+`a2832cb`
fixup · `2d43306` T6+T7 · `fd4cf7f` docs · `cc1557f` report.

Revisi v0.14.1 (7): `dc781d6` R2+R3 · `77fd98a` R4 (codex limit mid-run,
diff direview manual) · `89bb18e` fix unique-ref · `0006580` R5 (inline) ·
`411473e` fix pointer-receiver · `bffe495` R6 verify.sh · `eb3a415` gofmt ·
`40f2845` R7 runbook. Head sekarang menunggu commit R9 ini.

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

## 6. Acceptance Criteria — Status Jujur (revisi §12)

- [x] Identical retry 1 jurnal (sequential + concurrent 2-goroutine).
- [x] Same ref beda admin/asset/units/address → typed conflict, tanpa leak.
- [x] Cancelled ref semantics + copy §8 (typed error + handler copy + preview-upgrade).
- [x] Concurrent confirm terbukti 1 jurnal.
- [x] Fault injection rollback journal/legs/status/audit (hook seam, 2 tests).
- [x] Success + cancel audit atomik (dalam tx; deny path unchanged by design).
- [x] Deny failure tidak melemahkan denial (deny di entry, error audit dibuang tapi akses tetap ditolak; decision test NonAdminDenied; telemetry penuh = backlog).
- [x] Handler-level allowlist tests (decision boundary 6 tests; full API-mock disengaja tidak — 3 call sites).
- [x] Preview/cancel/confirm side effects: cancel-no-journal + confirm-once terbukti service-level + decision tests; Telegram-tap pending R8.
- [x] Clean checkout 1 command (verify.sh, clean-clone VERIFY OK).
- [x] Migration/rollback-forward terdokumentasi (runbook, irreversible, backup 3.1 MB).
- [x] Full suite hijau (88 PASS, 4 SKIP pivot, 0 FAIL).
- [ ] Telegram trial admin + non-admin (R8 — instruksi di bawah).
- [x] Report dikoreksi (§0 dokumen ini).
- [x] Live belum disentuh (tidak ada deploy/restart live selama R1–R9).

## 7. Instruksi Trial Telegram R8 (untuk user, bot v0141 PID 1487639)

Bot: @cryptostationonly_bot. Admin: akun 7254686681. Siapkan akun kedua
(non-admin) di device lain.

Admin (21 langkah, catat hasil tiap langkah):

1. /admin → panel ada `💼 Wallet & Aset` + `🧾 Transaksi` (Stok lama hilang).
2. Wallet & Aset → submenu Ringkasan/Deposit/Float/Fee/Hot/PANEL.
3. Ringkasan: 3 aset + SIMULASI + AMAN/TIPIS + hot + estimasi/harga-basi.
4. Deposit Stok → pilih SOL → network Solana + hot + warning SIMULASI tampil.
5. Ketik `0` → ditolak, bisa retry. Ketik `abc` → ditolak, bisa retry.
6. Ketik `5` → diminta reference.
7. Ketik `MOCK-R8-001` → preview (5 SOL, before/after, tombol Catat/Batal).
8. Batal → "Tidak ada jurnal", saldo tetap (cek via Ringkasan).
9. Ulangi 4–7 dengan `MOCK-R8-002` → Confirm → sukses (ID + jurnal + saldo + Detail).
10. Confirm lagi (atau double-tap cepat pesan preview) → "sudah diproses", saldo tetap.
11. Ketik ref `MOCK-R8-002` di flow baru → copy "sudah pernah dicatat" + jurnal + saldo.
12. Ketik ref `MOCK-R8-001` (cancelled langkah 8) → copy "sudah dibatalkan... buat baru".
13. Transaksi → feed ada STOCK_IN baru paling atas.
14. Detail STOCK_IN → journal/admin/ref/address/status/BIB.
15. Toggle 25 → lebih banyak item (atau empty-note jujur).
16. Beli mock user (flow biasa) → PAYOUT_OUT muncul di feed.
17. Jual mock user → USER_DEPOSIT_IN muncul di feed.
18. Restart bot workspace (minta AI) → feed + detail tetap bisa dibuka.
19. Cek audit: minta AI `RecentAdminAudit` — ada `wallet:stock_deposit:record` 1 row + `wallet:stock_deposit:cancel` 1 row untuk deposit trial.
20. Spread/limit/pause/trial/broadcast tetap jalan (regression).
21. /beli /jual /status user tidak rusak (regression).

Non-admin (4 langkah, akun kedua):

22. Tap tombol Admin / kirim `a:wallet` via deep-link → "Bukan admin."
23. Coba tebak callback detail (`a:act:STOCK_IN:<id>`) → ditolak, tidak bocor.
24. Ketik pesan saat sesi admin (tidak ada sesi — pastikan tidak error).
25. Minta AI cek `admin_audit` ada row `deny:...` untuk ID akun kedua.

Regression (5 langkah): 20–21 di atas + /harga + /saldo + /riwayat.

Lapor balik: nomor langkah yang GAGAL + screenshot/pesan bot. Yang sukses
tidak perlu bukti per langkah — cukup "R8 1–25 OK kecuali ...".

## 8. Masalah Selama Eksekusi + Solusi (v0.14 + revisi)

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
