# Crypto Station — Roadmap & Living Plan

> Status: v0.11 — Debug Fase 3+4 (17 temuan → 15 fix, kurs blocker clear, 30 tests hijau)
> Owner: user + hermes
> Last updated: 2026-10-30

## Daftar Isi
- [0. Locked Decisions](#0-locked-decisions-v011)
- [1. Goal](#1-goal)
- [2. Target Users](#2-target-users)
- [3. Stage Model & Gate](#3-stage-model--gate)
- [4. Tech Stack](#4-tech-stack)
- [5. Core Features](#5-core-features)
- [6. User Flows](#6-user-flows)
- [7. Pricing & Risk Engine](#7-pricing--risk-engine)
- [8. Data Model & Ledger](#8-data-model--ledger)
- [9. Security & Anti-abuse](#9-security--anti-abuse)
- [10. API Endpoints & Bot Commands](#10-api-endpoints--bot-commands)
- [11. Blueprint Arsitektur (LOCK v0.5)](#11-blueprint-arsitektur-lock-v05)
- [12. Roadmap](#12-roadmap)
- [13. Unit Economics](#13-unit-economics)
- [14. Risks](#14-risks)
- [15. Open Decisions Tersisa (v0.11)](#15-open-decisions-tersisa-v011)
- [16. Deploy Strategy](#16-deploy-strategy)
- [17. Implementation Notes](#17-implementation-notes)
- [18. Current Next Step](#18-current-next-step)

## 0. Locked Decisions (v0.11)

| # | Keputusan | Pilihan |
|---|-----------|---------|
| 0 | Nama project/bot | **Crypto Station** — display name. Username dicoba urut: @cryptostationbot → @cryptostation_bot → @cryptostationid_bot. Cadangan: StasiunKripto |
| 1 | Model market | **Bandar (broker)** — user beli/jual ke stok sistem, fee dari spread. Bukan P2P, tanpa order book |
| 2 | Peran Telegram | **Full transaksi via bot** — semua flow di dalam bot |
| 3 | Custody | **Custodial ledger internal, tanpa on-chain di MVP.** Stage 1 saldo = simulasi berlabel jelas; Stage 2 saldo IDR nyata + inventory nyata |
| 4 | Sumber harga | **Coin dari Binance public API + spread. Kurs USDT/IDR 1 sumber (nama di Stage 0) + staleness fail-closed.** Umur koin dan kurs terpisah. Interface pembanding siap di Stage 2: selisih > X% pause beli dan jual. Bukan median |
| 5 | Identitas & limit | **Tanpa KTP di MVP.** Topup/withdraw max 5jt/hari/user + limit global harian, enforcement-nya Stage 2. Stage 1 wajib jalan: private chat, allowlist admin, rate limit perintah, audit. Pola mule/sybil dicatat sebagai event, belum dihukum |
| 6 | Input nominal | **Selalu IDR + tombol ALL/MAX buat jual** |
| 7 | Quote lock | **Lock 30 detik, angka pasti mengikat (bukan ±).** Lewat = requote 1 klik |
| 8 | Minimum nominal | **Topup 50rb, beli 25rb, withdraw 100rb** (cek ulang vs fee flat setelah provider fix) |
| 9 | Fee provider | **Ditanggung user, ditampilin di depan** — topup numpang di atas, withdraw motong nominal. Spread murni margin. Fee provider tidak dijurnal sebagai laba |
| 10 | Expiry + approval | **QR expired 30 menit**, withdraw >1jt approve admin via bot, di bawahnya otomatis |
| 11 | Coin MVP | **BNB, ETH, SOL** doang. USDT / lain-lain Pasca-MVP |
| 12 | Fiat in/out | **Masuk QRIS otomatis (provider TBD)**, keluar transfer bank/e-wallet via disbursement |
| 13 | Stage model | **Sandbox-first: Stage 0 validasi → Stage 1 sandbox simulasi → Stage 2 real-money dengan gate checklist.** Stage 1 boleh dikoding tanpa gateway. Gate Stage 2 termasuk sumber kurs kedua hidup + nilai staleness/divergence sudah di-set |
| 14 | Ledger | **Double-entry journal.** IDR = BIGINT, coin = NUMERIC(36,18). Saldo dompet dan counter harian = cache turunan, bukan sumber kebenaran. Spread tidak dijurnal sebagai kas; P&L = selisih vs mid, dihitung dari trade. Lock order: Users → LedgerAccounts → Wallets. Konsumsi quote atomik + rekonsiliasi harian |
| 15 | Pricing bid/ask | **Bid/ask terpisah per coin, spread buy/sell boleh beda** (rentang percobaan 0.7–1%, bukan janji margin). Valuasi pakai bid terakhir yang tidak basi; kalau basi tampil "harga basi", bukan angka diam |
| 16 | Eksposur + circuit breaker | **Dua jenis pause.** Eksposur lewat batas = pause beli saja (jual tetap boleh). Spike, data basi, atau kurs abnormal = pause beli dan jual. Nilai limit uang asli + inventory-vs-hedge diputus Stage 0. Limit sandbox tidak terbawa ke Stage 2 |
| 17 | Pembayaran telat & ambigu | **QR dibayar setelah expired = paid_late.** Nominal sama persis dengan invoice → kredit sekali, tetap makan limit harian; kalau limit jebol jangan kredit, tandai butuh refund. Nominal beda → tidak pernah auto-kredit. **Disbursement time-out = unknown** (tahan di hold sampai rekonsiliasi, tidak auto-refund) |
| 18 | Security baseline | **Stage 1:** private-chat only, secret webhook Telegram, allowlist admin, reset limit ikut WIB, audit before/after. **Stage 2:** PIN wajib untuk tarik + tambah norek, webhook provider verify signature + simpan mentah |
| 19 | Norek | **Multi rekening + cooling-off.** Durasi di Settings. Bukan lock 1 norek/user |
| 20 | Tech stack | **Go 1.27.3 (go.mod min 1.26) + Postgres + pgx/v5 + sqlc + goose v3 + go-telegram/bot v1.27.0 + shopspring/decimal + coder/websocket.** Detail lock di Lampiran A. Buka lagi hanya kalau ada versi mati saat Fase 0 dimulai |
| 21 | Blueprint arsitektur | **Bab 11 LOCK v0.5** — service, kontrak, alur end-to-end, pola error/retry/idempotency, konvensi kode. Pengembangan ngikut blueprint, bukan ngasal |
| 22 | Pola kerja lokal-VPS | **Lokal D:\Program\project-1 coding + git doang (nol runtime). Eksekusi (build/test/migrate/run) di cbt-vps (43.134.111.212, ubuntu, key).** Ngikut pola LMS. Password tidak disimpan di plan/repo/memory |
| 23 | VPS + Fase 0 DONE | **cbt-vps: Ubuntu 24.04, Go 1.27.3, Postgres 16 (port 5432), tz Asia/Jakarta, role cs + db cryptostation + pgcrypto, goose v3.28.0 + sqlc v1.29.0.** Repo: go.mod min 1.26, migrasi 000001 (users+settings 16 seed), skeleton /start /saldo private-chat-only. Smoke: migrate v1 + build + cek DB exit 0. Commit 8cd6ec3. Gotcha: migrasi goose wajib `-- +goose Up` |
| 24 | Fase 1 DONE | **Migrasi 000002 (ledger_accounts/journals/entries/wallets/price_snapshots). sqlc pgx/v5 + numeric→decimal. Settings cache+Reload. PriceService WS bookTicker gabungan + fail-closed + guard bid<=ask. Ledger GrantSandbox idempotent + Reconcile + WIBToday. Bot /start (create+grant) /saldo (real+valuasi bid) /harga. 7 tests hijau (4 price + 3 ledger), commit 8561ffb.** Gotcha: encoding/json Go case-insensitive — key qty B/A nimpa key harga b/a kalau field qty tidak dideklarasikan. Parser wajib deklarasikan BidQty/AskQty terpisah. WS live verified: BNB 705.62/705.63, ETH 2264.99, SOL 96.49 |
| 25 | internal/db generated | **internal/db/ hasil sqlc generate TIDAK di-commit (regenerasi di VPS tiap sync).** db/schema/ juga generated. Yang di-commit: db/queries/*.sql + migrasi + sqlc.yaml |
| 26 | Fase 2 DONE | **Migrasi 000003 (quotes/trades/risk_events + seed exposure_limit). CreateQuote (minimum + expire lama + lock 30s) + ExecuteTrade (lock user → consume atomik → cek saldo → jurnal 4 kaki → cache → trades row, 1 tx). RiskService 2 pause (exposure=tolak beli, spike_stale=tolak semua) + overExposure + RecordEvent. Bot /beli /jual (ALL) + inline Confirm/Batal + /riwayat + /batal. Risk check 2x. 13 tests hijau (9 ledger + 4 price), head fbc4456.** Fix: AmountIdr, dust 1e-6, tr1 unused |
| 27 | Pola git VPS | **VPS /home/ubuntu/crypto-station = clone dari bare (bukan tar-sync). Deploy: push origin+server → fetch origin + reset --hard.** Bare HEAD = main. Backup tar lama di crypto-station-tar-backup (boleh hapus) |
| 28 | Debug pass Fase 1+2 | **10 temuan → 9 fix + 1 catat (head f0223ca). Wajib: snapshot audit ditulis+throttle, reconcile 1 tx repeatable, lock wallet FOR UPDATE, grant by jurnal, exposure short-only, batal expire, cek saldo awal, dead code, FeeNote. Catat Fase 4: refreshWalletCache O(n). 19 tests hijau (13 ledger + 4 price + 2 risk).** Gotcha: ExpireUserQuotes return 1 value; CreateQuote baru expire lama (test lock pakai insert langsung) |
| 29 | refreshWalletCache O(n) | **DICATAT buat Fase 4: scan semua jurnal per trade, melambat seiring tumbuh. Fix = query filter owner di SQL. Sekarang jangan disentuh** |
| 30 | Fase 3 DONE | **Migrasi 000004 (admin_audit + rate_limit + allowlist). Admin allowlist env+settings, Deny tercatat. SetSpread/SetLimit(14-key whitelist)/Pause(off/exposure/spike_stale) + audit + Reload. Rate limit message+callback + RiskEvent. Job reconcile/jam → gagal = pause 3 coin. /approve stub jujur (Stage 2). 27 tests hijau (head 7899e07)** |
| 31 | Fase 4 parsial DONE | **Migrasi 000005 (exposure_snapshots + warn threshold). Trial service: SnapshotExposure + Report (/trial: reconcile + quote status + quote age + rejects + exposure). /broadcast beneran + /trial admin. Job exposure/10 mnt. TRIAL.md checklist T1-T4. 29 tests hijau (head 05695ad). Trial LIVE nunggu token (T1-T4 manual) + exit criteria 7 hari** |
| 32 | Stage 1 kode selesai | **Fase 0-4 kode sandbox komplit. Sisa manual user: (1) token bot + trial live TRIAL.md, (2) Stage 0 STAGE0.md (gateway/kurs/stok). Baru setelah itu gate Stage 2 bisa dinilai** |
| 33 | Debug pass Fase 3+4 | **17 temuan → 15 fix + 2 catat (head 5b5c8f2). BLOCKER CLEAR: kurs refresher 30s Indodax (sebelumnya basi permanen >60s). Wajib: limiter sync, slippage real di /trial, signed exposure, spread decimal, limiter cap, start race, broadcast 40ms, system:pause audit, SafeDecimal path quote. Catat: import order (kosmetik), graceful shutdown OK. FX verified 18149. 30 tests hijau** |
| 34 | Kurs sementara Indodax | **USDT/IDR dari Indodax public ticker (tanpa key), refresh 30s, fetchManualFX di cmd/bot/fx.go. Nama sumber FINAL fix Stage 0 (S0.2) — ganti isi fungsi tanpa ubah pemanggil** |

## 1. Goal

Bikin market jual-beli crypto paling gampang buat orang awam: topup QRIS dari Telegram, beli BNB/ETH/SOL pakai rupiah, jual balik dan tarik ke bank — semua tanpa keluar bot, tanpa ngerti wallet / gas / exchange.

Urutan pencapaian: **buktikan produk dan pricing engine dulu di sandbox**, baru buka uang asli kalau gate Stage 2 lolos.

## 2. Target Users

- User retail awam: punya Telegram + QRIS, beli crypto nominal kecil (25rb–5jt), ga mau ribet exchange
- Admin (lu doang di MVP): approve withdraw gede, ubah spread/limit, pause coin, broadcast pengumuman. Admin = allowlist Telegram ID, bukan akun user yang kebetulan tahu perintah

## 3. Stage Model & Gate

### Stage 0 — Validasi (1–2 minggu, tanpa kode produksi)

Output sebelum Stage 2:
1. **Gateway menjawab terbuka.** Tanya 2–3 provider dengan deskripsi bisnis jujur: diterima/tidak, syarat onboarding, fee QRIS + disbursement, settlement time (catatan: Xendit QRIS T+2 hari kerja, max 20jt/tx), kebutuhan float. Jangan pilih vendor sebelum ada jawaban
2. **Model stok dipilih:** inventory nyata atau hedge + nilai batas eksposur (#16). Sandbox tidak menunggu ini
3. **Sumber kurs dipilih, plus sumber pembanding untuk Stage 2** (#4). Bukan median
4. **Unit economics terisi** (bab 13)

### Stage 1 — Sandbox

- Saldo IDR **simulasi** (misal modal virtual 1jt saat /start, jelas berlabel SIMULASI di setiap layar)
- Harga asli, kurs asli (1 sumber, fail-closed), flow beli/jual/quote lengkap, ledger lengkap, admin, dua jenis pause
- **Tanpa** topup/withdraw nyata, tanpa integrasi gateway (interface payments diisi mock)
- Anti-abuse uang (PIN, name-inquiry, cooling-off, sybil, mule) **belum dihukum**. Event-nya tetap dicatat
- Tujuan: uji pricing engine, UX, ledger, concurrency, rekonsiliasi tanpa risiko uang nyata
- **Exit criteria:**
  - rekonsiliasi ledger selisih 0 minimal 7 hari berturut-turut
  - tidak ada quote dikonsumsi 2x (uji double-tap + retry)
  - kedua jenis pause teruji (eksposur = beli saja; spike/stale/kurs abnormal = beli dan jual)
  - quote ditolak saat sumber down, timestamp kosong, atau umur lewat — tidak ada harga terakhir yang diam-diam dipakai
  - slippage quote vs pasar terukur dan dalam batas
  - allowlist admin menolak Telegram ID di luar daftar

### Stage 2 — Real-money (bersyarat)

Gate checklist, semua harus centang:
- [ ] Gateway setuju tertulis
- [ ] Model stok/hedge + batas eksposur uang asli ter-set (limit sandbox tidak dipakai ulang)
- [ ] Sumber kurs kedua hidup dan fail-closed; ambang selisih X% ter-set
- [ ] Nilai staleness koin dan kurs ter-set
- [ ] Float awal tersedia (untuk settlement T+2 + disbursement)
- [ ] Exit criteria sandbox terpenuhi
- [ ] Unit economics masuk akal

Catatan: checklist ini operasional. Centang penuh bukan pernyataan bahwa bisnisnya boleh dijalankan.

## 4. Tech Stack

**Lock v0.5 (sinkron Lampiran A): Go 1.27.3 (go.mod min 1.26) + Postgres.**

- Bot framework: **github.com/go-telegram/bot v1.27.0** (mendukung Bot API 10.3, daftar resmi, zero-dependency, WithWebhookSecretToken). Worker default 1, biarkan 1
- DB: Postgres + **pgx/v5 + sqlc** (jangan GORM, jangan lib/pq). Uang tidak lewat `float64`. IDR BIGINT/int64, coin NUMERIC(36,18)/decimal
- Migrasi: **goose v3**. Decimal: **shopspring/decimal** (bagi → bulatkan 18 desimal di Go → tulis)
- Harga: Binance spot WS via **coder/websocket** (bookTicker gabungan bnbusdt/ethusdt/solusdt, jangan gorilla, jangan futures); 1 sumber kurs + staleness fail-closed; interface pembanding kosong sampai Stage 2
- Money in/out (Stage 2): QRIS + disbursement 1 vendor, 1 interface. Kandidat: Xendit / Midtrans / Tripay / Duitku (hasil Stage 0)
- Infra: 1 VPS + systemd, satu binary. Webhook Telegram via HTTPS Caddy 443, path acak (bukan token), tolak tanpa secret header. Long-polling buat dev

## 5. Core Features

### Stage 1 (sandbox)
1. Akun nempel telegram id, /start langsung jadi (mode sandbox)
2. Cek harga BNB/ETH/SOL (bid/ask terpisah). Harga basi tidak ditampilkan sebagai angka hidup
3. Beli pakai IDR + quote mengikat 30 detik + confirm
4. Jual balik + quote + tombol ALL
5. Saldo + valuasi total + riwayat 10 terakhir
6. Admin allowlist: /setspread /setlimit /pause /broadcast
7. Dua jenis pause + batas eksposur + rekonsiliasi harian
8. Rate limit perintah + audit before/after + RiskEvents (belum dihukum)

### Stage 2 (real-money, setelah gate)
9. Topup QRIS + webhook idempotent + paid_late exact-match + /topup
10. Withdraw + PIN + name-inquiry + cooling-off norek + threshold approve + state unknown + /tarik /approve
11. Enforcement sybil / mule / limit global. Event Stage 1 jadi baseline, bukan diganti

### Pasca-MVP (jangan disentuh)
- Kirim on-chain ke wallet external (hot wallet, gas, nonce, retry)
- USDT, BTC, coin lain
- KYC penuh, referral, grafik harga, order book / limit order

## 6. User Flows

### FLOW 0 — start / akun
1. /start → akun by telegram id. Stage 1: modal virtual + label SIMULASI di semua layar
2. Bot hanya respons di **private chat**. Group, channel, dan callback dari chat lain diabaikan
3. Stage 2: user bikin PIN sebelum bisa topup/withdraw. PIN bukan pengganti 2FA Telegram
4. Limit harian di backend, reset ikut tanggal **WIB** (Asia/Jakarta, bukan jam server)

### FLOW 1 — topup via QRIS (Stage 2)
1. /topup → input nominal (min 50rb)
2. Invoice + QR expired 30 menit; nominal bayar = pokok + fee QRIS. Selaraskan expiry bot dengan expiry provider, supaya paid_late tidak jadi jalur normal
3. Bayar → webhook → verify signature + idempotent → saldo nambah. Webhook mentah simpan di WebhookEvents
4. Notif masuk. Expired tanpa bayar → hangus, bikin baru. Callback dobel → 1x
5. **Bayar setelah expired → paid_late** (#17):
   - nominal sama persis dengan invoice → kredit pokok sekali, di transaksi yang sama dengan cek limit harian
   - limit jebol → jangan kredit, status butuh refund, manual
   - nominal beda → jangan pernah auto-kredit
   - paid_late tidak mengubah nominal dan tidak membuat quote baru

### FLOW 2 — beli crypto
1. /harga → BNB/ETH/SOL (ask = harga beli user)
2. /beli → pilih coin → input IDR (min 25rb)
3. Cek: harga koin dan kurs tidak basi, sumber hidup, coin tidak di-pause, eksposur belum lewat batas
4. Quote mengikat 30 detik: "dapat 0.051234567 SOL = Rp195.000" (angka pasti, format cuma di tampilan)
5. Confirm → atomik: quote dikonsumsi, jurnal tercatat, cache saldo ikut
6. Lewat → requote 1 klik

### FLOW 3 — jual balik
1. /jual → pilih coin → input IDR atau ALL/MAX
2. Quote 30 detik pakai **bid** → confirm → atomik sama seperti beli
3. Jual ditolak kalau pause-nya jenis spike/stale/kurs abnormal. Jual tetap boleh kalau pause-nya cuma eksposur

### FLOW 4 — withdraw IDR (Stage 2)
1. /tarik → nominal (min 100rb) + pilih/tambah rekening
2. **Norek baru:** PIN + name-inquiry + cooling-off (durasi di Settings). Multi norek boleh (#19)
3. Tampil: nominal, fee, terima bersih
4. Potong saldo ke hold → <1jt langsung disbursement → >1jt pending
5. Pending → /approve → jalan, atau reject + refund dari hold
6. Status akhir: success / failed (refund) / **unknown** (hold tetap, jangan refund, sampai rekonsiliasi)
7. Notif sukses/gagal

### FLOW 5 — pantau
1. /saldo = IDR + tiap coin + total valuasi (pakai **bid terakhir yang tidak basi**)
2. Kalau bid basi, valuasi koin itu tampil "harga basi", bukan angka beku
3. /riwayat = 10 transaksi terakhir
4. /harga = refresh tiap dibuka, bukan push per detik

### FLOW 6 — admin
1. Perintah admin cuma jalan kalau Telegram ID ada di allowlist. Selain itu ditolak dan dicatat
2. /approve (Stage 2) → list pending → approve/reject
3. /setspread → ubah spread buy/sell per coin tanpa restart
4. /setlimit → threshold approval, limit harian, limit eksposur tanpa restart
5. /pause → kill-switch manual per coin. Circuit breaker memakai mekanisme yang sama, tapi jenis pause tetap dibedakan (#16)
6. /broadcast → pengumuman. Semua aksi tercatat di AdminAudit, before/after

## 7. Pricing & Risk Engine

### 7.1 Formula

```
mid_idr = harga_coin_usdt × kurs_usdt_idr
ask     = mid_idr × (1 + spread_buy)    // user beli
bid     = mid_idr × (1 − spread_sell)   // user jual
Beli:  units      = amount_idr / ask
Jual:  amount_idr = units × bid
Valuasi: sum(units × bid terakhir yang tidak basi) + IDR
P&L spread per trade = (harga pakai − mid) × units, tanda ikut sisi
```

Spread buy/sell boleh beda per coin (di Settings). Rentang awal 0.7–1% cuma buat trial. Gerak SOL dalam 30 detik bisa lewat 1%. Itu alasan pause spike ada, bukan alasan mengunci spread kecil sebagai margin aman.

### 7.2 Sumber harga & staleness (versi ringan, jangan dinaikkan ke full)

Tetap 1 sumber di Stage 1. Dua sumber belum menguji apa-apa sebelum quote, ledger, dan slippage lolos sandbox. Yang tidak boleh ditunda adalah fail-closed.

- Coin: Binance public WS/REST, cache in-memory
- Kurs: **1 sumber**. Nama sumber dipilih Stage 0. Tiap snapshot simpan nama sumber + timestamp
- Dua ambang di Settings, jangan satu angka N untuk keduanya:
  - `staleness_coin_ms`, default sandbox 15000
  - `staleness_fx_ms`, default sandbox 60000
- Umur lewat, sumber down, atau timestamp tidak ada → quote **ditolak**, pesan jelas. Jangan pakai harga terakhir diam-diam, jangan ada kurs hardcoded
- Interface pembanding disiapkan, belum diisi di Stage 1
- Stage 2: sumber utama + pembanding. Selisih > X% → pause beli dan jual. Di dalam batas, pakai sumber utama
- Jangan median dari 2 sumber. Median dua angka tidak membuang sumber yang rusak; sumber buruk menarik harga setengah jalan

### 7.3 Eksposur & circuit breaker (masuk MVP, dua jenis pause)

- **Net position** per coin = terjual ke user − terbeli dari user (+ inventory awal)
- Lewat batas → pause **beli** coin itu saja. Jual tetap boleh, karena itu mengurangi eksposur
- Spike (> Y% dalam Z menit), data basi, atau kurs abnormal → pause **beli dan jual**. Jual ke bid basi tidak boleh
- Admin lepas pause via bot; tiap pause/resume masuk audit, termasuk jenisnya
- Limit yang dipakai trial sandbox ditandai dan tidak disalin ke Stage 2
- Log metrik: slippage per trade, net position, quote ditolak, jenis pause

### 7.4 Model stok (uang asli diputus Stage 0)

- **Inventory nyata:** pegang coin beneran, risiko pasar sendiri dalam batas eksposur
- **Hedge otomatis:** transaksi balik di venue lain; butuh akun + API trading + biaya hedge
- Sandbox terkunci: posisi house tercatat saja, tanpa hedge, buat ngukur eksposur. Ini bukan keputusan model stok Stage 2

## 8. Data Model & Ledger

### 8.1 Prinsip

- IDR = `BIGINT` rupiah. Coin = `NUMERIC(36,18)`. Tanpa float
- Semua mutasi lewat **jurnal double-entry dalam 1 transaksi DB**. Saldo dompet dan counter harian = turunan (boleh cache), bukan sumber kebenaran
- Webhook idempotent by ProviderRef + IdempotencyKey
- Konsumsi quote atomik, di transaksi yang sama dengan jurnal:

```sql
UPDATE quotes SET status='used'
WHERE id=$1 AND user_id=$2 AND status='active' AND expires_at > now()
RETURNING *;  -- 0 baris = sudah dipakai/expired/bukan milik user, batalkan transaksi
```

- Lock order saat mutasi: `Users`, lalu `LedgerAccounts`, lalu `Wallets`. Jangan kebalik, supaya tidak deadlock
- `SELECT ... FOR UPDATE` pada baris yang di-lock itu. Limit harian tidak boleh jebol oleh request paralel
- Rekonsiliasi harian: sum jurnal = sum dompet = sum counter yang seharusnya, dan (Stage 2) saldo provider/inventory. Selisih ≠ 0 → alert + pause beli dan jual
- Spread tidak masuk `house:revenue` sebagai kas. Akun itu jangan dipakai untuk menyimpan notional

### 8.2 Templat jurnal

Tiap jurnal, sum Amount per Asset = 0. Spread bukan kaki kas.

Beli `amount` IDR dapat `units` koin:

- user IDR −amount
- house:cash IDR +amount
- house:inventory koin −units
- user koin +units
- P&L spread = (ask − mid) × units, disimpan di trade, bukan di saldo revenue

Jual kebalikannya: user koin berkurang, inventory koin bertambah, cash house berkurang, IDR user bertambah. P&L = (mid − bid) × units.

Topup paid / paid_late yang lolos (fee tidak masuk laba, #9):

- user IDR +pokok
- house:provider-clearing IDR −pokok
- FeeIDR tetap di invoice. Jangan jurnal fee sebagai `house:revenue`

Withdraw, saldo sudah dipotong tapi uang belum tentu sampai:

- minta tarik: user IDR −gross, house:withdraw-hold IDR +gross
- success: hold −gross, house:cash IDR −gross
- failed / reject: balikkan jurnal minta tarik
- unknown: biarkan hold. Jangan refund otomatis

### 8.3 Tabel

```
Users { ID, TelegramID unique, Mode (sandbox/real), Status (active/blocked), PinHash, DailyTopupTotal, DailyWithdrawTotal, LastLimitReset, CreatedAt }
LedgerAccounts { ID, Owner (user:<id> / house:cash / house:inventory / house:withdraw-hold / house:provider-clearing), Asset (IDR/BNB/ETH/SOL) }
Journals { ID, Type (topup/buy/sell/withdraw/refund/adjust), RefTable, RefID, CreatedAt }
LedgerEntries { ID, JournalID, AccountID, Asset, Amount (signed), CreatedAt }  -- per jurnal, total Amount per Asset = 0
Wallets { UserID, Asset, Balance }  -- cache turunan, unique (UserID, Asset)
TopupInvoices { ID, UserID, AmountIDR, FeeIDR, ProviderRef unique, QRPayload, Status (pending/paid/paid_late/needs_refund/expired/failed), ExpiresAt, PaidAt, IdempotencyKey unique }
Quotes { ID, UserID, Side (buy/sell), Asset, AmountIDR, QuotedUnits, Bid, Ask, Mid, SpreadUsed, RateUSDTIDR, RateSource, RateAgeMs, PriceAgeMs, ExpiresAt, Status (active/used/expired) }
Trades { ID, UserID, QuoteID unique, Side, Asset, AmountIDR, Units, PriceUsed, MidUsed, SpreadPnlIDR, JournalID, CreatedAt }
BankAccounts { ID, UserID, BankCode, AccountNo, AccountNameVerified, Status (pending_confirm/active/disabled), ConfirmedAt, CooldownUntil }
Withdrawals { ID, UserID, BankAccountID, AmountIDR, FeeIDR, NetIDR, Status (pending/approved/auto_processing/success/failed/unknown/rejected), ProviderRef unique, ApprovedBy, CreatedAt }
PriceSnapshots { ID, Asset, BinancePrice, SpreadBuy, SpreadSell, Ask, Bid, RateUSDTIDR, RateSource, RateAgeMs, PriceAgeMs, CreatedAt }
ExposureSnapshots { ID, Asset, NetPosition, LimitUsed, CreatedAt }
WebhookEvents { ID, Provider, EventType, RawPayload, SignatureValid, ProcessedAt, CreatedAt }
RiskEvents { ID, UserID, TelegramID, Kind, Payload, CreatedAt }  -- Stage 1 nulis, Stage 2 yang menghukum
Settings { Key, Value, UpdatedBy, UpdatedAt }  -- spread, limit, threshold, cooldown, staleness_coin_ms, staleness_fx_ms, fx_divergence_pct, pause per coin, admin allowlist, rate limit
AdminAudit { ID, AdminTelegramID, Action, TargetID, Before, After, CreatedAt }
```

`house:revenue` tidak dipakai untuk kas spread di MVP. Jangan ditambah "biar lengkap".

### 8.4 Indexes

- Users.TelegramID unique, Wallets(UserID, Asset) unique, Trades.QuoteID unique
- TopupInvoices.ProviderRef unique, Withdrawals.ProviderRef unique
- LedgerEntries(JournalID), LedgerEntries(AccountID, CreatedAt)
- BankAccounts(AccountNo, BankCode) — deteksi 1 norek dipakai banyak akun (enforcement Stage 2, event boleh dari Stage 1 kalau datanya ada)
- RiskEvents(UserID, CreatedAt), RiskEvents(Kind, CreatedAt)

## 9. Security & Anti-abuse

Sunat ini sengaja. Jangan bangun name-inquiry, cooling-off, PIN, atau limit per rekening di Stage 1. Itu butuh gateway dan tidak menguji apa-apa pada saldo simulasi. Jangan juga memasukkan kontrol admin ke ember "log saja".

**Wajib jalan di Stage 1, bukan cuma log:**
- Private-chat only. Update dari group/channel diabaikan
- Allowlist admin (Telegram ID di Settings atau env). Perintah admin dari ID lain ditolak
- Rate limit perintah per akun. Angkanya di Settings
- Reset limit ikut WIB
- Secret webhook Telegram diverifikasi. Signature provider belum ada di Stage 1, jangan dipaksa jadi baseline sekarang
- Audit before/after untuk /setspread /setlimit /pause /broadcast
- Secret di env, bukan repo

**Stage 1 cuma mencatat, Stage 2 yang menghukum:**
- **Sybil:** limit global harian + batas akun per norek. Akun Telegram baru gampang dibuat; limit per-user tidak cukup kalau uangnya asli
- **Mule/fraud:** withdraw cuma ke norek lolos name-inquiry; cooling-off norek baru; flag pola topup lalu langsung tarik tanpa trading
- **SIM swap:** PIN wajib tarik + tambah norek. PIN adalah kontrol aplikasi, bukan pengganti 2FA Telegram
- Event ditulis ke `RiskEvents` (siapa, kapan, jenis aksi), bukan log teks bebas, supaya Stage 2 punya baseline

**Stage 2 tambahan:**
- Webhook provider: verify signature, simpan mentah, batasi siapa yang bisa baca payload
- Enforcement butir sybil / mule / PIN di atas

Peringatan ke user: nilai coin bisa turun, quote cuma 30 detik.

## 10. API Endpoints & Bot Commands

Internal (bukan public):
- POST /webhook/telegram — update bot, secret token wajib sejak Stage 1
- POST /webhook/<provider> — QRIS callback (verify + idempotent) [Stage 2]
- POST /webhook/<provider>-disbursement — status withdraw [Stage 2]
- GET /internal/prices — harga terakhir + umur data + jenis pause (bot + debug)
- Job: refresh kurs; WS Binance; rekonsiliasi harian; cek eksposur

Bot commands:
- User: /start /saldo /topup* /harga /beli /jual /tarik* /riwayat /batal (*Stage 2)
- Admin, allowlist saja: /approve* /setspread /setlimit /pause /broadcast

## 11. Blueprint Arsitektur (LOCK v0.5)

> Bab ini fondasi teknis pengembangan. Plan bilang mau bikin apa (bab 1–10), bab ini bilang sistemnya disusun gimana + aturan yang ngikat semua fase. Kode yang melanggar bab ini = salah, bukan gaya.

### 11.1 Bentuk proses & dependensi modul

Satu binary, satu arah dependensi, tidak boleh muter:

```
/cmd/bot/main.go   — wiring doang: baca env → buka DB → init settings/price/ledger/risk/payments → register handler → start (long-poll dev / webhook prod). Dilarang logika bisnis di sini
/internal/bot/        — handler per command + middleware (private-chat, allowlist admin, rate limit). Handler tipis: parse input → panggil service → format output. Dilarang query SQL langsung, dilarang hitung uang
/internal/settings/   — baca/tulis Settings (cache + reload). Semua angka kunci lewat sini: spread, limit, threshold, minimum, staleness_coin_ms, staleness_fx_ms, fx_divergence_pct, cooldown, eksposur, rate limit, pause per coin, admin allowlist
/internal/price/      — SATU-SATUNYA pintu harga (PriceService). WS bookTicker + kurs REST + cache timestamp + staleness fail-closed + hitung bid/ask. Modul lain dilarang pegang harga mentah
/internal/risk/       — cek eksposur + dua jenis pause + tulis RiskEvents. Dipanggil sebelum quote dibuat dan sebelum confirm dieksekusi
/internal/ledger/     — SATU-SATUNYA pintu mutasi uang. Jurnal double-entry + quote atomik + limit harian + rekonsiliasi. Modul lain dilarang update Wallets langsung
/internal/payments/   — 1 interface QRIS + disbursement. Stage 1 = mock. Jangan isi kolom khusus 1 provider sebelum Stage 0 selesai
/internal/admin/      — approve + setspread/setlimit + pause + broadcast + audit before/after
/migrations/          — goose v3, skema ngikut bab 8
```

Aturan arah: bot → risk → price → ledger → payments. Ledger tidak manggil bot. Price tidak manggil ledger. Risk tidak nulis jurnal (cuma baca posisi + set pause via settings + tulis RiskEvents).

### 11.2 Kontrak antar service (signature konseptual)

```go
// PRICE — satu-satunya sumber harga. Gagal = tolak, jangan fallback
type QuoteInput struct { UserID int64; Side string; Asset string; AmountIDR int64 }
type PricedQuote struct {
  Bid, Ask, Mid decimal.Decimal
  Units decimal.Decimal        // sudah dibagi + dibulatkan 18 desimal di Go
  RateUSDTIDR decimal.Decimal; RateSource string
  PriceAgeMs, RateAgeMs int64
  ExpiresAt time.Time
}
PriceService.GetQuote(ctx, QuoteInput) (PricedQuote, error)
// error = stale | paused | exposure | source_down → handler tampilkan pesan jelas, bukan angka

// RISK — dipanggil 2x: sebelum quote (boleh kasih quote?) + sebelum eksekusi (masih boleh?)
RiskService.CheckBeforeQuote(ctx, asset, side) error   // pause jenis apa pun yang relevan
RiskService.CheckBeforeExecute(ctx, asset, side) error // cek ulang, karena 30 detik bisa berubah
RiskService.RecordEvent(ctx, kind, userID, payload)    // Stage 1: tulis RiskEvents, belum hukum

// LEDGER — satu-satunya yang boleh nulis uang. Semua dalam 1 tx Postgres
LedgerService.ExecuteTrade(ctx, quoteID, userID) error
// di dalam tx, berurutan: lock Users → lock LedgerAccounts → lock Wallets (FOR UPDATE)
// 1) konsumsi quote atomik (UPDATE ... WHERE status='active' AND expires_at > now() RETURNING; 0 baris = gagal)
// 2) cek limit harian (counter = cache, validasi di tx yang sama)
// 3) tulis Journals + LedgerEntries per templat bab 8.2 (sum per aset = 0)
// 4) update cache Wallets. Rollback kalau langkah mana pun gagal

LedgerService.Reconcile(ctx) error // sum jurnal = sum dompet = sum counter; selisih ≠ 0 → alert + pause beli+jual

// PAYMENTS — interface, Stage 1 mock
PaymentsService.CreateQRIS(ctx, amount, fee) (providerRef, qrPayload, expiresAt, error)
PaymentsService.HandleCallback(ctx, rawPayload, signature) error // verify + idempotent + simpan WebhookEvents mentah
PaymentsService.Disburse(ctx, bankCode, accountNo, netAmount) (providerRef, error)
PaymentsService.CheckStatus(ctx, providerRef) (status, error)    // unknown = tahan, jangan refund
```

### 11.3 Alur end-to-end (fondasi flow bab 6)

**Beli/jual (FLOW 2/3):**
handler parse → Risk.CheckBeforeQuote → Price.GetQuote (fail-closed kalau basi) → simpan Quotes row status active → tampil angka pasti + tombol confirm → user confirm → Risk.CheckBeforeExecute → Ledger.ExecuteTrade (quote atomik + jurnal + cache) → tampil hasil. Quote lewat 30 detik = row expired, requote bikin row baru, row lama tidak dipakai ulang.

**Topup paid_late (FLOW 1 edge, #17):**
webhook masuk → simpan WebhookEvents mentah → verify signature → cari invoice by ProviderRef → idempotent (sudah paid = abaikan) → kalau expired: exact-match? nominal sama persis → cek limit harian DALAM tx yang sama → lolos = kredit pokok 1x (jurnal topup), jebol = status needs_refund manual → nominal beda = tidak pernah auto-kredit.

**Withdraw unknown (FLOW 4 edge, #17):**
minta tarik = jurnal user IDR −gross, house:withdraw-hold +gross (uang sudah keluar dari saldo user tapi belum tentu sampai) → disbursement → success: hold −gross, house:cash −gross → failed/reject: jurnal balik → unknown/timeout: **biarkan hold, jangan refund** sampai rekonsiliasi/provider status jelas.

**Jual saat pause (bedakan jenis, #16):**
pause eksposur → beli ditolak, jual tetap jalan (ngurangin eksposur). Pause spike/stale/kurs abnormal → beli DAN jual ditolak (jual ke bid basi dilarang). Handler tidak menentukan jenis — tanya RiskService, tampilkan alasan ke user.

### 11.4 Pola error, retry, idempotency

- Webhook (Telegram + provider): selalu balas 200 cepat, kerja berat di dalam tx + idempotent key. Callback dobel = baris sudah ada = abaikan, bukan error
- WS Binance putus / serverShutdown: tandai harga basi SEGERA, tolak quote baru. REST cuma isi snapshot baru + timestamp baru, tidak menahan harga lama
- Disbursement timeout: status unknown, hold tetap, retry cek status pakai backoff. Dilarang auto-refund dari unknown
- Semua mutasi uang retry-able HANYA lewat kunci idempotent (ProviderRef, QuoteID unique, IdempotencyKey) — bukan retry buta
- Rate limit per akun di middleware (angka di Settings). Admin di luar allowlist = tolak + catat, bukan diam

### 11.5 Konvensi kode yang ngikat

1. Tidak ada float64 di path uang. IDR int64, coin decimal. Bagi → bulatkan 18 desimal di Go → tulis NUMERIC(36,18)
2. Lock order di setiap tx: Users → LedgerAccounts → Wallets. Jangan kebalik (deadlock)
3. Handler bot tipis: tidak ada SQL, tidak ada hitung uang, tidak ada pegang harga mentah
4. Semua angka kunci via Settings (tanpa restart). Tidak ada konstanta spread/limit/staleness hardcode di logika
5. Pause selalu bawa jenis (exposure vs spike_stale). Satu flag boolean tanpa jenis = bug
6. Format rupiah/coin cuma di layer tampilan. Simpan mentah
7. Tiap aksi admin tulis AdminAudit before/after. Tiap pola abuse tulis RiskEvents (bukan log teks)
8. sqlc: query ditulis sendiri, direview. Dilarang Save/Updates generik yang bisa geser saldo di luar jurnal
9. Path webhook acak + secret header. Token bot tidak di path, log, atau URL
10. Test bab A.9 wajib hijau sebelum fase dinyatakan done. Fase tanpa test = belum selesai

### 11.6 Peta fase → fondasi (step pengembangan berurut)

- Fase 0: repo Go 1.27.3 + go.mod min 1.26 + goose + sqlc + skeleton /start /saldo + skema ledger (bab 8.3) + mock payments. Keluar: binary jalan long-poll, migrasi hijau
- Fase 1: PriceService (WS bookTicker + kurs 1 sumber + staleness fail-closed) + LedgerService dasar + /harga. Keluar: quote ditolak saat basi, bukan angka diam
- Fase 2: quote mengikat + ExecuteTrade + RiskService dua jenis pause + /beli /jual /riwayat. Keluar: double-tap 1 menang 1 ditolak
- Fase 3: admin allowlist + /setspread /setlimit /pause /broadcast + audit + RiskEvents + job rekonsiliasi. Keluar: admin luar allowlist ditolak + tercatat
- Fase 4: trial sandbox + ukur slippage + tuning spread + exit criteria bab 3. Keluar: gate Stage 2 bisa dinilai
- Fase 5–7: payments asli + paid_late exact-match + withdraw hold/unknown + enforcement anti-abuse. Syarat: gate Stage 2 centang semua

Checkpoint keras: payments asli, kurs kedua, enforcement mule/sybil = SETELAH gate. Jangan naik ke Stage 1.

## 12. Roadmap

Estimasi kasar, kerja santai, +buffer. Kalibrasi ulang setelah Fase 0.

**Stage 0 — Validasi (1–2 minggu)**
- Tanya 2–3 gateway, pilih sumber kurs + pembanding, pilih model stok uang asli, isi unit economics
- Boleh paralel dengan Fase 0. Jangan paralel dengan pembayaran asli

**Stage 1 — Sandbox (±3–4 minggu)**
- Fase 0 — ✅ DONE 2026-10-29 (repo Go 1.27.3, skeleton /start /saldo, migrasi 000001 users+settings, smoke cbt-vps hijau, commit 8cd6ec3). Skema ledger full + templat jurnal lanjut Fase 1
- Fase 1 — ✅ DONE 2026-10-30 (migrasi 000002 ledger full, PriceService WS live + fail-closed, settings cache, GrantSandbox idempotent + Reconcile, /harga + /saldo real, 7 tests hijau, commit 8561ffb). Kurs masih manual-fase1 (nama sumber fix Stage 0)
- Fase 2 — ✅ DONE 2026-10-30 (migrasi 000003 quotes/trades/risk_events, CreateQuote + ExecuteTrade atomik, RiskService 2 pause, /beli /jual ALL + inline confirm + /riwayat + /batal, 13 tests hijau, head fbc4456)
- Fase 3 — ✅ DONE 2026-10-30 (migrasi 000004 admin_audit, allowlist env+settings, /setspread /setlimit /pause + audit, rate limit, job rekonsiliasi, 27 tests hijau, head 7899e07)
- Fase 4 — ✅ PARSIAL 2026-10-30 (migrasi 000005 exposure_snapshots, trial report + /broadcast + /trial, job exposure, TRIAL.md, 29 tests hijau, head 05695ad). Trial LIVE nunggu token (T1-T4 + exit criteria 7 hari)

**Stage 2 — Real-money (setelah gate lolos; ±3–4 minggu)**
- Fase 5 — topup QRIS + webhook idempotent + paid_late exact-match + /topup (5–7 hari)
- Fase 6 — withdraw + PIN + name-inquiry + cooling-off + threshold approve + unknown + /tarik /approve (4–5 hari)
- Fase 7 — sumber kurs pembanding, enforcement anti-abuse, retry webhook, rekonsiliasi provider, alert, closed trial limit rendah (1–2 minggu)

Checkpoint keras: **jangan coding payments asli sebelum Stage 0 selesai + provider fix.** Mock interface boleh duluan. Jangan naikkan kurs atau anti-abuse ke versi full di Stage 1.

## 13. Unit Economics

Isi di Stage 0. Template:

```
Revenue kotor per trade = nominal × spread
  contoh: Rp200.000 × 1% = Rp2.000
  ini dari SpreadPnl di trade, bukan dari saldo house:revenue
Biaya per trade:
  - hedge / slippage rata-rata
  - infra (VPS, monitoring) / jumlah trade
  - support + operasional
Fee QRIS + disbursement = user (#9), tapi cek: fee flat vs min withdraw 100rb
Break-even = biaya tetap bulanan / revenue bersih per trade
```

Harus kejawab angka: berapa trade/bulan biar impas, dan eksposur max yang ketahan kalau harga gerak 5% melawan posisi.

## 14. Risks

- **Gateway menolak bisnis crypto:** bisa matikan Stage 2. Mitigasi: tanya terbuka di Stage 0; sandbox tetap jalan. Jangan mengunci vendor sebelum ada jawaban
- **Gate operasional bukan izin usaha:** checklist Stage 2 tidak menjawab apakah bisnis ini boleh dijalankan
- **Risiko pasar (posisi terbuka):** klaim coin tanpa pegang. Mitigasi: eksposur limit, pause beli, inventory/hedge sebelum uang asli
- **Volatilitas 30 detik + data basi:** staleness fail-closed, spread adjustable, pause beli dan jual. Spread 0.7–1% tidak menanggung gerak ini
- **Custodial trust:** riwayat transparan, rekonsiliasi, audit trail
- **Sybil + mule:** bab 9. Enforcement Stage 2; Stage 1 hanya event
- **Webhook dobel/palsu + race:** signature + idempotent + quote atomik + lock order
- **SIM swap:** PIN + rate limit, Stage 2. Bukan pengganti 2FA Telegram
- **paid_late sebagai jalan pintas limit:** exact-match + cek limit di transaksi yang sama. Nominal beda tidak pernah auto-kredit
- **Jual ke harga basi:** dicegah dengan pause beli dan jual, terpisah dari pause eksposur
- **Cash flow (float):** QRIS settle T+2, disbursement keluar dari saldo sendiri
- **Secret bocor:** env saja, bukan repo
- **Limit sandbox terbawa ke produksi:** batas eksposur Stage 2 harus di-set ulang, tidak disalin

## 15. Open Decisions Tersisa (v0.11)

Ditutup di v0.11, jangan dibuka lagi tanpa alasan di bawah:

- **Debug Fase 3+4 (#33):** 17 temuan → 15 fix + 2 catat. 30 tests hijau
- **Kurs sementara (#34):** Indodax, final fix Stage 0

- **Fase 3+4 (#30, #31):** kode done, trial live nunggu token
- **Stage 1 selesai (#32):** sisa manual user (token + Stage 0), baru gate Stage 2

- **Debug pass (#28):** 10 temuan → 9 fix + 1 catat (#29). Test 19 hijau
- **refreshWalletCache O(n) (#29):** fix di Fase 4, jangan sekarang

- **Pola git VPS (#27):** clone dari bare, fetch + reset --hard. Tar-sync pensiun

- **Generated code (#25):** internal/db + db/schema regenerasi di VPS, tidak di-commit

- **Pola kerja (#22):** lokal coding + git, eksekusi cbt-vps. Password tidak disimpan di mana-mana

- **Nama (#0):** Crypto Station. Sisa username fix di BotFather
- **Stack (#20):** Go 1.27.3 + pgx/v5 + sqlc + goose v3 + go-telegram/bot v1.27.0 + decimal + coder/websocket (sinkron Lampiran A). Buka lagi hanya kalau ada versi mati saat Fase 0 dimulai
- **Blueprint (#21):** bab 11 LOCK — fondasi pengembangan
- **Norek (#19):** multi + cooling-off, durasi di Settings
- **Umur data:** koin 15 detik, kurs 60 detik, fail-closed. Angka final boleh digeser di Settings, perilakunya tidak
- **Dispute sementara:** owner handle via Telegram, tanpa SLA publik. SLA publik baru sebelum Stage 2, bukan hari ini

Masih terbuka:

1. Payment gateway — setelah gateway jawab. Kandidat: Xendit / Midtrans / Tripay / Duitku. Yang mendesak adalah bertanya, bukan memilih
2. Model stok uang asli — inventory nyata vs hedge + nilai batas eksposur (Stage 0). Sandbox sudah terkunci: posisi house tercatat, tanpa hedge
3. Nama sumber kurs, nama sumber pembanding, dan nilai X% (Stage 0). Default umur data sudah di-set
4. Spread exact — 0.7% vs 1%, buy/sell beda atau sama (trial Fase 4). Jangan dikunci sekarang
5. SLA dispute publik — sebelum Stage 2

## 16. Deploy Strategy

- Dev: long-polling dari cbt-vps (tanpa HTTPS). Lokal nol runtime
- Stage 1: cbt-vps (Ubuntu 24.04, Go 1.27.3, Postgres 16 port 5432, tz Asia/Jakarta) + systemd, satu binary, webhook Telegram via HTTPS (Caddy 443), secret token wajib. DB `cryptostation`, role `cs`
- Stage 2: + webhook provider via HTTPS, alert (rekonsiliasi gagal, eksposur, webhook invalid, sumber kurs mati), backup DB terjadwal
- Env: BOT_TOKEN, WEBHOOK_SECRET, DATABASE_URL, ADMIN_TELEGRAM_IDS. PROVIDER_KEY baru Stage 2. Tidak masuk repo

## 17. Implementation Notes

- Payments asli jangan sebelum Stage 0; mock interface boleh
- Format rupiah/coin cuma di layer tampilan; simpan BIGINT / NUMERIC(36,18)
- Semua angka kunci (spread, limit, threshold, minimum, staleness koin, staleness kurs, divergence, cooldown, eksposur, rate limit) via tabel Settings, tanpa restart
- Tiap mutasi uang lewat templat jurnal bab 8. Tidak ada update saldo lepas dari jurnal
- Lock order: Users → LedgerAccounts → Wallets
- Harga basi, sumber down, timestamp kosong: tolak quote. Jangan fallback
- Pause eksposur dan pause spike/stale tidak boleh jadi satu flag tanpa jenis
- Limit sandbox tidak disalin ke Stage 2
- Test wajib: double-tap confirm, webhook dobel, quote expired detik terakhir, request paralel vs limit harian, jual saat eksposur lewat (boleh), jual saat harga basi (tidak boleh), paid_late nominal sama, paid_late nominal beda, paid_late yang menjebol limit, disbursement unknown tidak refund, perintah admin dari ID di luar allowlist

## 18. Current Next Step

**v0.11 — STOP, lanjut nanti. Posisi tersimpan.**

Terakhir: debug Fase 3+4 hijau (30 tests, head 5b5c8f2), plan v0.11 (head 8f8fe45).

Lanjut nanti mulai dari manual user:
1. Fix username bot di BotFather → BOT_TOKEN → trial live TRIAL.md (T1-T4) → exit criteria 7 hari
2. Stage 0 (STAGE0.md): tanya gateway, sumber kurs final, model stok, unit economics → hasil masuk v0.12

Setelah dua itu kelar → nilai gate Stage 2 → gas Fase 5 (topup QRIS).

Jangan mulai pembayaran asli, sumber kurs kedua, atau enforcement mule/sybil sebelum gate Stage 2.

### Changelog v0.10 → v0.11

- **Debug Fase 3+4 (baru #33):** audit baris-per-baris → 17 temuan → 15 fix + 2 catat (head 5b5c8f2). Blocker: kurs refresher 30s. 30 tests hijau (6 admin + 2 bot + 13 ledger + 4 price + 2 risk + 3 trial)
- **Kurs sementara (baru #34):** Indodax USDT/IDR public, fetchManualFX, verified 18149. Final fix Stage 0

### Changelog v0.9 → v0.10

- **Fase 3 DONE (baru #30):** migrasi 000004 + admin allowlist/audit + rate limit + job reconcile, 27 tests hijau (head 7899e07). Roadmap Fase 3 DONE
- **Fase 4 parsial DONE (baru #31):** migrasi 000005 + trial report/broadcast + TRIAL.md, 29 tests hijau (head 05695ad). Trial live nunggu token
- **Stage 1 selesai (baru #32):** kode sandbox komplit Fase 0-4. Sisa manual user eksplisit

### Changelog v0.8 → v0.9

- **Debug pass (baru #28):** audit baris-per-baris Fase 1+2 → 10 temuan → 9 fix (snapshot audit+throttle, reconcile tx, lock wallet, grant by jurnal, exposure short-only, batal expire, cek saldo awal, dead code, FeeNote). 19 tests hijau (head f0223ca)
- **Catat Fase 4 (baru #29):** refreshWalletCache O(n) — jangan disentuh sekarang
- Stage 0 hasil digeser ke v0.10 (belum masuk)

### Changelog v0.7 → v0.8

- **Fase 2 DONE (baru #26):** migrasi 000003 + CreateQuote/ExecuteTrade + RiskService 2 pause + /beli /jual /riwayat /batal, 13 tests hijau (head fbc4456). Roadmap Fase 2 ditandai DONE
- **Pola git VPS (baru #27):** clone dari bare, fetch + reset --hard. Tar-sync pensiun, backup di crypto-station-tar-backup
- Stage 0 hasil digeser ke v0.9 (belum masuk)

### Changelog v0.6 → v0.7

- **Fase 1 DONE (baru #24):** migrasi 000002 + PriceService WS live + GrantSandbox idempotent + Reconcile + /harga + /saldo real, 7 tests hijau (commit 8561ffb). Roadmap Fase 1 ditandai DONE
- **Generated code (baru #25):** internal/db + db/schema tidak di-commit, regenerasi di VPS tiap sync
- **Gotcha parser:** encoding/json case-insensitive (B nimpa b) — wajib field Qty terpisah + guard bid<=ask + test regresi
- Kurs manual-fase1 dicatat eksplisit (nama sumber fix Stage 0, bukan sekarang)

### Changelog v0.5 → v0.6

- **Fase 0 DONE (baru #23):** repo Go 1.27.3 + migrasi 000001 + skeleton /start /saldo + smoke cbt-vps hijau (commit 8cd6ec3). Roadmap Fase 0 ditandai DONE
- **Pola kerja (baru #22):** lokal coding + git, eksekusi cbt-vps, ngikut LMS. Password tidak disimpan di plan/repo/memory
- **Deploy (bab 16):** generik 1 VPS → konkret cbt-vps (Ubuntu 24.04, Go 1.27.3, Postgres 16:5432, Asia/Jakarta, db cryptostation/role cs)
- Stage 0 pack (STAGE0.md) dicatat sebagai kerja paralel manual, hasilnya masuk v0.9 (belum masuk per v0.8)

### Changelog v0.4 → v0.5

- **Blueprint arsitektur (baru #21, bab 11 LOCK):** bentuk proses + arah dependensi modul, kontrak 4 service (Price/Risk/Ledger/Payments), alur end-to-end (beli-jual, paid_late, unknown, jual-saat-pause), pola error/retry/idempotency, 10 konvensi kode, peta Fase 0–7 → fondasi
- **Stack (revisi #20, sinkron Lampiran A):** Go generik → Go 1.27.3 (go.mod min 1.26); bot → go-telegram/bot v1.27.0; DB → pgx/v5 + sqlc; migrasi → goose v3; decimal + aturan bulatkan-di-Go; harga → coder/websocket spot bookTicker; webhook → Caddy 443 + path acak + secret header
- **Bab 4 + 15 sinkron:** tidak ada lagi referensi go-telegram-bot-api / GORM / Go 1.22 sebagai pilihan. Opsi Node resmi ditutup (#20 hanya dibuka ulang kalau ada versi mati)

### Changelog v0.3 → v0.4

- **Nama (#0, baru):** Crypto Station lock. Username dicoba berurut, cadangan StasiunKripto. Item nama keluar dari Open Decisions
- Promosi file saran menjadi plan resmi; file saran dihapus, PROJECT_IDEAS.md satu-satunya sumber kebenaran

### Changelog v0.2 → v0.3

- **Kurs tetap versi ringan**, tapi fail-closed. Umur koin dan kurs dipisah (15 detik / 60 detik). "Median 2 sumber" dihapus. Stage 2 = sumber utama + pembanding, selisih > X% pause beli dan jual. Sumber kedua masuk gate, bukan cuma pekerjaan Stage 0
- **Anti-abuse tidak dinaikkan ke full.** PIN, name-inquiry, cooling-off, sybil, dan mule tetap Stage 2. Private chat, allowlist admin, rate limit, dan audit dipindah ke wajib Stage 1. Log teks diganti `RiskEvents`
- **#16:** pause eksposur (beli saja) dipisah dari pause spike/stale/kurs abnormal (beli dan jual). Limit sandbox tidak terbawa
- **#17:** "proporsional" diganti exact-match. Limit harian tetap berlaku. Nominal beda tidak pernah auto-kredit. Withdraw unknown duduk di `house:withdraw-hold`
- **#14:** templat jurnal beli/jual/topup/withdraw dikunci. Spread bukan kas. Lock order dikunci. Counter harian = cache. Coin jadi NUMERIC(36,18)
- **#15:** valuasi cuma dari bid yang tidak basi. Rentang 0.7–1% ditandai sebagai angka percobaan
- **#18:** baseline Stage 1 dan kontrol Stage 2 dipisah. Signature provider tidak lagi dipaksa ada di sandbox
- **Ditutup:** stack Go (#20), norek multi + cooling-off (#19), dispute sementara tanpa SLA publik
- **Gate Stage 2** ditambah sumber kurs kedua hidup dan nilai staleness/divergence. Catatan: gate ini operasional, bukan izin usaha

### Changelog v0.1 → v0.2

- **Stage model (baru #13):** sandbox-first. Stage 1 simulasi bisa coding tanpa nunggu gateway; Stage 2 dikunci gate checklist
- **Ledger (baru #14):** single-balance → double-entry journal; coin int → NUMERIC; + quote atomik, FOR UPDATE, rekonsiliasi harian
- **Pricing (baru #15, revisi #4):** spread tunggal → bid/ask pisah per coin; kurs 2-sumber penuh → versi ringan (1 sumber + staleness, interface siap pembanding)
- **Risk (baru #16):** eksposur limit + circuit breaker masuk MVP
- **Edge money (baru #17):** paid_late + unknown
- **Security (baru #18, revisi #5):** private-chat, WIB, PIN; anti-abuse enforcement Stage 2, sandbox belum menghukum pola mule/sybil
- **Custody (revisi #3) + quote (revisi #7):** simulasi Stage 1, quote angka pasti
- **Struktur:** roadmap dipecah Stage 0/1/2 + Fase 0–7; fitur pasca-MVP dinamai eksplisit; unit economics jadi bab sendiri

---

## Lampiran A — Stack Sistem (lock user, sinkron v0.5)

> Status lampiran: LOCK hasil research user, sinkron dengan plan v0.5 (bab 4 + #20).
> Prinsip: yang dipakai versi yang benar-benar hidup saat bot diluncurkan, bukan yang paling familiar.
> Referensi versi: Bot API 10.3 (24 Agu 2026). Go 1.27.3 + 1.26.10 (19 Okt 2026, masih dapat patch keamanan). Go 1.22 di luar dukungan.

### A.1 Runtime — LOCK
- **Go 1.27.3**, `go` di go.mod minimal **1.26**. Jangan Go 1.22
- Alasan: 1.22 sudah di luar dukungan; goose v3.27.1+ butuh Go minimal 1.25.7 (satu alasan lagi tidak mulai dari 1.22)
- Satu binary dari /cmd/bot, config via env + tabel Settings (spread/limit/staleness tanpa restart)
- Lokal coding doang, jalan/test di rdpkhorur. Prod 1 VPS + Postgres + Caddy buat HTTPS webhook
- Log: slog bawaan Go. Job interval: refresh kurs, WS Binance, rekonsiliasi harian, cek eksposur

### A.2 Bot framework — LOCK github.com/go-telegram/bot
- **Pakai: github.com/go-telegram/bot v1.27.0** — menyatakan dukung Bot API 10.3, ada di daftar resmi Telegram, zero-dependency, punya middleware + WithWebhookSecretToken untuk header X-Telegram-Bot-Api-Secret-Token
- Private-chat-only tetap di middleware. Worker default-nya 1 — biarkan 1. Konkurensi uang diselesaikan transaksi Postgres, bukan banyak handler barengan
- **Jangan telebot v4.** Modul terbit masih v4.0.0-beta.10 (16 Jun 2026), bukan rilis stabil. Garis stabil berhenti di v3.3 dan cuma mencakup Bot API 7.1 — tiga generasi di belakang API yang hidup
- **Jangan go-telegram-bot-api.** Tidak ada di daftar resmi Telegram (yang tercantum antara lain gotgbot, telego, go-telegram/bot, telebot). README-nya masih mengajarkan webhook dengan token di path URL + sertifikat self-signed. Untuk bot uang, token tidak boleh muncul di path, access log, atau URL webhook
- gotgbot masih release candidate (v2.0.0-rc.36, 2 Agu 2026) — jangan jadi default

### A.3 Database — LOCK pgx/v5 + sqlc
- **Pakai: pgx/v5 + sqlc.** sqlc resmi mendukung pgx/v5 sejak v1.18.0. lib/pq sudah mode maintenance, jangan untuk kode baru
- **Jangan ikut premium-hub di ORM-nya.** GORM mempercepat CRUD, tapi ledger ini butuh jurnal yang jumlahnya nol per aset. Satu Save/Updates tidak disengaja bisa menggeser saldo di luar jurnal. sqlc menghasilkan query yang ditulis sendiri, jadi kaki jurnal tidak berubah diam-diam

### A.4 Migrasi — LOCK goose v3
- **Pakai: goose v3** (terbaru v3.28.0; sejak v3.27.1 Go minimum 1.25.7)
- golang-migrate tidak salah, tapi tidak ada gunanya keputusan ini dibiarkan terbuka — tutup ke goose kecuali ada alasan khusus nanti

### A.5 Uang — LOCK shopspring/decimal + aturan pembulatan
- **Pakai: shopspring/decimal** (v1.6.0, 18 Okt 2026 — masih hidup)
- Aturan tambahan: **bagi dulu, bulatkan ke 18 desimal di Go, baru tulis ke NUMERIC(36,18).** Kalau pembulatan diserahkan ke Postgres, dua kaki jurnal bisa selisih 1 satuan terkecil dan rekonsiliasi gagal
- IDR tetap int64. Jangan scan NUMERIC ke float64. Tidak ada float64 di path uang (harga, units, amount, fee, P&L)

### A.6 Harga — LOCK coder/websocket + spot bookTicker
- **Pakai: coder/websocket.** Panduan Go (14 Mar 2026) menyatakan gorilla/websocket diarsipkan, penggantinya coder/websocket karena ada context dan tulisannya aman dipakai bersamaan. Jangan gorilla
- **Endpoint spot, bukan futures.** Yang masih dipelihara: wss://stream.binance.com:9443 dan wss://stream.binance.com:443
- Stream yang cocok untuk mid: **bookTicker gabungan bnbusdt, ethusdt, solusdt**
- Koneksi ke stream.binance.com tidak berlaku lebih dari 24 jam, server mengirim serverShutdown sebelum putus. **Putus harus langsung bikin harga jadi basi.** REST hanya boleh mengisi snapshot baru dengan timestamp baru, bukan menahan harga terakhir
- Staleness check di service layer — semua quote lewat satu pintu PriceService.GetQuote()

### A.7 Deploy & webhook — LOCK Caddy 443 + secret header
- **Caddy di 443 tetap benar**, mengurus sertifikat. Webhook cloud Telegram harus HTTPS. HTTP dan port sembarangan hanya berlaku kalau menjalankan Bot API server sendiri — itu tidak perlu untuk MVP
- Aplikasi menolak request tanpa secret token. **Path webhook pakai string acak, bukan bot token**

### A.8 Struktur modul (ngikut plan v0.4 bab 11)
- /cmd/bot, /internal/bot, /internal/ledger, /internal/price, /internal/risk, /internal/payments (interface + mock), /internal/admin, /internal/settings, /migrations
- payments 1 interface, jangan isi kolom khusus 1 provider sebelum Stage 0 selesai

### A.9 Test wajib hari pertama
- Double-tap confirm (quote dipakai 2x → 1 menang, 1 ditolak)
- Request paralel vs limit harian (FOR UPDATE beneran nahan)
- Quote expired detik terakhir
- paid_late nominal sama (kredit 1x) vs nominal beda (tidak kredit) vs jebol limit (needs_refund)
- Disbursement unknown tidak refund otomatis
- Perintah admin dari ID di luar allowlist ditolak + tercatat
- Jual saat pause eksposur (boleh) vs jual saat harga basi (tidak boleh)

### A.10 Status keputusan (dulu A.8 — sudah tutup semua)
1. Bot: go-telegram/bot — TUTUP
2. DB: sqlc + pgx/v5 — TUTUP
3. Migrasi: goose — TUTUP
4. Go 1.22 dan telebot v4 wajib diganti sebelum repo dibuat — dicatat agar Fase 0 tidak mulai dari versi mati
