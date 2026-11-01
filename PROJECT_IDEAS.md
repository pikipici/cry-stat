# Crypto Station — Roadmap & Living Plan

> Status: v0.12-done — Fase 8+9 transit LIVE (41 tests hijau, DB v8, bot active)
> Owner: user + hermes
> Last updated: 2026-11-01

## PIVOT v0.12 (baca dulu sebelum bab lain)

v0.11 ke bawah = bandar custodial: coin ditahan di saldo internal, user bolak-balik
beli/jual, ada valuasi + exposure bandar. ITU DIBATALKAN.

v0.12 = transit + auto-kirim: user beli → koin OTOMATIS dikirim on-chain ke wallet
pribadi user (bukan ngendap di bot). Jual-balik = user kirim koin ke alamat deposit
sistem → nunggu konfirmasi chain → sistem bayar IDR. Fee network + gas DITANGGUNG
USER dan tampil di quote sebelum Confirm.

Implikasi:
- Saldo coin di bot = transit (nol setelah terkirim), bukan tempat nyimpen.
- /saldo = IDR + riwayat kirim (txhash), bukan valuasi portofolio.
- Exposure bandar (#16) berubah arti → float hot-wallet + pause node/stok.
- Kirim on-chain NAIK dari pasca-MVP ke inti Stage 2 (Fase 8-9 baru).
- Yang tetap kepake 100%: price engine, quote lock 30s, ledger double-entry,
  rate limit, allowlist/audit, tombol T1/T2, reconcile (diperluas on-chain).
- Menu UX berlapis (v0.12-ux): utama = Beli/Jual/Harga/Menu, submenu Akun =
  Saldo/Riwayat/Status/Alamat, Batal contextual (inline quote, bukan permanen).
- Status gabung payout + deposit (1 pantau semua). /cekdeposit redirect ke Status.

## Daftar Isi
- [PIVOT v0.12](#pivot-v012-baca-dulu-sebelum-bab-lain)
- [0. Locked Decisions](#0-locked-decisions-v012)
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
- [15. Open Decisions Tersisa (v0.12)](#15-open-decisions-tersisa-v012)
- [16. Deploy Strategy](#16-deploy-strategy)
- [17. Implementation Notes](#17-implementation-notes)
- [18. Current Next Step](#18-current-next-step)

## 0. Locked Decisions (v0.12)

| # | Keputusan | Pilihan |
|---|-----------|---------|
| 0 | Nama project/bot | **Crypto Station** — display name. Username dicoba urut: @cryptostationbot → @cryptostation_bot → @cryptostationid_bot. Cadangan: StasiunKripto |
| 1 | Model market | **Transit + auto-kirim (v0.12, REVISI dari bandar v0.11).** User beli → koin dikirim on-chain ke wallet pribadi user otomatis. Jual-balik = user kirim koin ke deposit sistem → konfirmasi chain → IDR. Bukan P2P, tanpa order book. Saldo coin di bot = transit, bukan simpanan |
| 2 | Peran Telegram | **Full transaksi via bot** — semua flow di dalam bot |
| 3 | Custody | **Non-custodial arah user (v0.12, REVISI).** Coin user TIDAK ngendap di bot — terkirim ke wallet pribadi. Sistem pegang hot-wallet float buat kirim + alamat deposit buat terima. Stage 1 sandbox = simulasi kirim (txhash mock berlabel); Stage 2 = on-chain beneran |
| 4 | Sumber harga | **Coin dari Binance public API + spread. Kurs USDT/IDR 1 sumber (nama di Stage 0) + staleness fail-closed.** Umur koin dan kurs terpisah. Interface pembanding siap di Stage 2: selisih > X% pause beli dan jual. Bukan median |
| 5 | Identitas & limit | **Tanpa KTP di MVP.** Topup/withdraw max 5jt/hari/user + limit global harian, enforcement-nya Stage 2. Stage 1 wajib jalan: private chat, allowlist admin, rate limit perintah, audit. Pola mule/sybil dicatat sebagai event, belum dihukum |
| 6 | Input nominal | **Selalu IDR.** Tombol ALL/MAX jual DIBATALKAN v0.12 — tidak ada saldo coin ngendap, jadi tidak ada yang di-ALL-kan. Jual = user kirim coin ke deposit, nominal ikut jumlah yang masuk chain |
| 7 | Quote lock | **Lock 30 detik, angka pasti mengikat (bukan ±).** Lewat = requote 1 klik |
| 8 | Minimum nominal | **Topup 50rb, beli 25rb, withdraw 100rb** (cek ulang vs fee flat setelah provider fix) |
| 9 | Fee provider + network | **Semua fee DITANGGUNG USER, tampil di quote sebelum Confirm (v0.12, PERLUASAN).** Topup numpang di atas, withdraw motong nominal, fee network/gas motong units yang dikirim. Spread murni margin. Fee tidak dijurnal sebagai laba |
| 10 | Expiry + approval | **QR expired 30 menit**, withdraw >1jt approve admin via bot, di bawahnya otomatis |
| 11 | Coin MVP | **BNB (BSC), ETH (ERC20), SOL (Solana) doang.** 1 coin = 1 network TETAP (tanpa pilihan network di MVP — cegah salah chain). USDT / lain-lain Pasca-MVP |
| 12 | Fiat in/out | **Masuk QRIS otomatis (provider TBD)**, keluar transfer bank/e-wallet via disbursement |
| 13 | Stage model | **Sandbox-first: Stage 0 validasi → Stage 1 sandbox simulasi → Stage 2 real-money dengan gate checklist.** Stage 1 boleh dikoding tanpa gateway. Gate Stage 2 termasuk sumber kurs kedua hidup + nilai staleness/divergence sudah di-set |
| 14 | Ledger | **Double-entry journal.** IDR = BIGINT, coin = NUMERIC(36,18). Saldo dompet dan counter harian = cache turunan, bukan sumber kebenaran. Spread tidak dijurnal sebagai kas; P&L = selisih vs mid, dihitung dari trade. Lock order: Users → LedgerAccounts → Wallets. Konsumsi quote atomik + rekonsiliasi harian |
| 15 | Pricing bid/ask | **Bid/ask terpisah per coin, spread buy/sell boleh beda** (rentang percobaan 0.7–1%, bukan janji margin). Quote beli = ask + fee network ditampilkan terpisah (harga coin + fee kirim). Tidak ada valuasi portofolio v0.12 — coin langsung terkirim, bukan disimpan |
| 16 | Eksposur + circuit breaker | **Dua jenis pause, MAKNA BARU v0.12.** Float hot-wallet di bawah batas kirim = pause beli (jual/deposit tetap boleh). Node mati, data basi, kurs abnormal, atau broadcast kirim macet = pause beli dan jual. Batas = float hot-wallet per coin (bukan posisi bandar). Nilai float + prosedur isi ulang diputus Stage 0. Limit sandbox tidak terbawa ke Stage 2 |
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
| 35 | Alamat wallet user (BARU v0.12) | **1 alamat per coin per user, network TETAP (BNB=BSC, ETH=ERC20, SOL=Solana).** Validasi format per network sebelum simpan. Ganti alamat = PIN (Stage 2) + cooling-off + audit. Salah alamat = hilang — peringatan tampil tiap Confirm. Sandbox: alamat mock, kirim = txhash mock |
| 36 | Kirim otomatis (BARU v0.12) | **Tiap beli sukses OTOMATIS kirim ke alamat tersimpan, tanpa tombol tarik.** Tanpa alamat tersimpan = beli DITOLAK ("daftarin alamat dulu"). Status payout: queued → broadcasting → confirmed (txhash) / failed (refund IDR otomatis) / unknown (tahan, jangan refund, sampai rekonsiliasi) |
| 37 | Jual = deposit on-chain (BARU v0.12) | **User kirim coin ke alamat deposit sistem (1 alamat per coin, tampil di bot).** Nunggu N konfirmasi (BNB 15, ETH 12, SOL 32 — di Settings) → sistem bayar IDR (bid − spread − fee). Deposit tak teridentifikasi (tanpa memo/tag user) = pending manual, tidak auto-kredit |
| 38 | Hot-wallet + node (BARU v0.12) | **Sistem pegang hot-wallet float per coin + 1 node provider per chain (nama fix Stage 0).** Float minimum di Settings; di bawahnya = pause beli. Isi ulang = transfer manual owner + jurnal. Rekonsiliasi = jurnal vs saldo chain (bukan cuma internal). Key hot-wallet di env HSM/KMS eventually — MVP di env file 600, TIDAK di repo |
| 39 | Fee network di quote (BARU v0.12) | **Estimasi fee per network di Settings (di-refresh job, bukan hardcode).** Quote beli tampil 3 baris: harga coin + fee kirim + total IDR. Fee > 20% nominal = tolak quote ("nominal terlalu kecil untuk fee"). Sandbox pakai fee mock tetap per coin |
| 40 | Riwayat = kirim, bukan valuasi (BARU v0.12) | **/saldo = IDR + daftar payout terakhir (coin, units terkirim, txhash, status).** Tidak ada total valuasi portofolio, tidak ada tombol ALL. /riwayat = 10 payout/deposit terakhir + txhash/confirm count |
| 41 | Menu UX berlapis (BARU v0.12-ux) | **Utama = Beli/Jual/Harga/Menu (+ Admin). Submenu Akun = Saldo/Riwayat/Status/Alamat + Kembali.** Batal = contextual (inline quote/picker + /batal), bukan tombol permanen. Alasan: 9 tombol flat bikin awam bingung; navigasi hafal posisi, aksi dibantu icon inline |
| 42 | Status 1 pantau (BARU v0.12) | **/status + tombol Status = payout + deposit sekaligus.** /cekdeposit redirect ke Status (kompat slash). Menu tidak nambah |

## 1. Goal

Jual-beli crypto paling gampang buat orang awam: bayar IDR dari Telegram, koin
(BNB/ETH/SOL) OTOMATIS terkirim ke wallet crypto pribadi user — tanpa ngerti
exchange, tanpa tarik manual, tanpa saldo ngendap di bot.

Urutan pencapaian: **buktikan pricing + kirim otomatis dulu di sandbox (txhash mock)**,
baru buka uang asli + on-chain beneran kalau gate Stage 2 lolos.

## 2. Target Users

- User retail awam: punya Telegram + QRIS + wallet crypto sendiri (TrustWallet/MetaMask/Phantom),
  beli nominal kecil (25rb–5jt), mau koin langsung masuk wallet dia, ga mau ribet exchange
- Admin (lu doang di MVP): isi float hot-wallet, ubah spread/limit/fee, pause coin,
  broadcast pengumuman. Admin = allowlist Telegram ID, bukan akun user yang kebetulan tahu perintah

## 3. Stage Model & Gate

### Stage 0 — Validasi (1–2 minggu, tanpa kode produksi)

Output sebelum Stage 2:
1. **Gateway menjawab terbuka.** Tanya 2–3 provider dengan deskripsi bisnis jujur: diterima/tidak, syarat onboarding, fee QRIS + disbursement, settlement time (catatan: Xendit QRIS T+2 hari kerja, max 20jt/tx), kebutuhan float. Jangan pilih vendor sebelum ada jawaban
2. **Node + hot-wallet dipilih (v0.12, REVISI dari model stok #16 lama):** 1 node provider per chain (nama + fee + rate limit) + float minimum per coin + prosedur isi ulang manual. Inventory-vs-hedge v0.11 DIBATALKAN — tidak ada posisi bandar
3. **Sumber kurs dipilih, plus sumber pembanding untuk Stage 2** (#4). Bukan median
4. **Jadwal fee network per chain terisi** (#39): fee rata-rata kirim BNB/ETH/SOL + sumber estimasi + ambang tolak (>20% nominal)
5. **Unit economics terisi** (bab 13, + biaya gas hot-wallet per kirim)

### Stage 1 — Sandbox

- Saldo IDR **simulasi** (misal modal virtual 1jt saat /start, jelas berlabel SIMULASI di setiap layar)
- Harga asli, kurs asli (1 sumber, fail-closed), flow beli + kirim-mock + quote lengkap, ledger lengkap, admin, dua jenis pause
- Kirim on-chain = **txhash mock berlabel jelas** (bukan chain beneran). Alamat = string mock tervalidasi format
- **Tanpa** topup/withdraw nyata, tanpa integrasi gateway, tanpa node beneran (interface chain diisi mock)
- Anti-abuse uang (PIN, name-inquiry, cooling-off, sybil, mule) **belum dihukum**. Event-nya tetap dicatat
- Tujuan: uji pricing engine, UX beli-kirim, ledger transit, concurrency, rekonsiliasi tanpa risiko uang nyata
- **Exit criteria:**
  - rekonsiliasi ledger selisih 0 minimal 7 hari berturut-turut
  - tidak ada quote dikonsumsi 2x (uji double-tap + retry)
  - kedua jenis pause teruji (float tipis = beli saja; spike/stale/kurs abnormal/node mati = beli dan jual)
  - quote ditolak saat sumber down, timestamp kosong, umur lewat, tanpa alamat tersimpan, atau fee > 20% nominal
  - slippage quote vs pasar terukur dan dalam batas
  - allowlist admin menolak Telegram ID di luar daftar
  - payout mock 100% confirmed + txhash tercatat (tidak ada yang nyangkut queued)

### Stage 2 — Real-money (bersyarat)

Gate checklist, semua harus centang:
- [ ] Gateway setuju tertulis
- [ ] Node provider per chain hidup + float hot-wallet minimum ter-set + prosedur isi ulang jalan (limit sandbox tidak dipakai ulang)
- [ ] Sumber kurs kedua hidup dan fail-closed; ambang selisih X% ter-set
- [ ] Nilai staleness koin dan kurs ter-set
- [ ] Jadwal fee network per chain ter-set + ambang tolak 20% jalan
- [ ] Float awal tersedia (untuk settlement T+2 + disbursement + gas hot-wallet)
- [ ] Exit criteria sandbox terpenuhi
- [ ] Unit economics masuk akal (termasuk gas per kirim)

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
2. Daftarin alamat wallet per coin (/alamat BNB 0x... — validasi format, 1 per coin)
3. Cek harga BNB/ETH/SOL (bid/ask terpisah). Harga basi tidak ditampilkan sebagai angka hidup
4. Beli pakai IDR + quote 30 detik (harga + fee kirim + total) + Confirm → OTOMATIS kirim-mock (txhash mock)
5. Jual = tampilkan alamat deposit + nunggu mock-konfirmasi → IDR masuk (potong spread + fee)
6. Saldo IDR + payout terakhir (txhash, status) + riwayat 10 terakhir
7. Admin allowlist: /setspread /setlimit /pause /broadcast + fee network + float
8. Dua jenis pause + float hot-wallet mock + rekonsiliasi harian (termasuk payout)
9. Rate limit perintah + audit before/after + RiskEvents (belum dihukum)

### Stage 2 (real-money, setelah gate)
10. Topup QRIS + webhook idempotent + paid_late exact-match + /topup
11. Kirim on-chain BENERAN: hot-wallet + nonce + retry + txhash asli + status unknown + /status
12. Deposit on-chain: watcher konfirmasi + kredit IDR + deposit tak dikenal = manual
13. Withdraw IDR + PIN + name-inquiry + cooling-off norek + threshold approve + unknown + /tarik /approve
14. Enforcement sybil / mule / limit global. Event Stage 1 jadi baseline, bukan diganti

### Pasca-MVP (jangan disentuh)
- USDT, BTC, coin lain + pilihan network multi (sekarang 1 coin = 1 network tetap)
- KYC penuh, referral, grafik harga, order book / limit order
- HSM/KMS buat key hot-wallet (MVP = env file 600)

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

### FLOW 2 — beli + auto-kirim (INTI v0.12)
1. /harga → BNB/ETH/SOL (ask = harga beli user)
2. /beli → pilih coin → input IDR (min 25rb). WAJIB sudah ada alamat tersimpan — belum ada = ditolak ("daftarin alamat dulu via /alamat")
3. Cek: harga koin dan kurs tidak basi, sumber hidup, coin tidak di-pause, float hot-wallet cukup, fee ≤ 20% nominal
4. Quote mengikat 30 detik, 3 BARIS: "harga 0.02867 SOL = Rp50.000 + fee kirim RpX = total RpY. dikirim ke 0xabc... (BSC). Confirm?"
5. Confirm → atomik: quote dikonsumsi, jurnal tercatat → payout row queued → (Stage 1: mock confirmed + txhash mock; Stage 2: broadcast on-chain)
6. Sukses = "Terkirim 0.02867 SOL ke 0xabc... tx MOCK-xxx". Coin TIDAK masuk saldo bot
7. Lewat → requote 1 klik. Failed broadcast → refund IDR otomatis. Unknown → tahan, /status cek

### FLOW 2b — alamat wallet (/alamat, BARU v0.12)
1. /alamat BNB 0x... → validasi format BSC (0x + 40 hex). ETH sama. SOL = base58 32-44 char
2. 1 alamat per coin. Ganti = timpa + audit + (Stage 2: PIN + cooling-off)
3. Tanpa alamat = beli DITOLAK. Bukan opsional

### FLOW 3 — jual via deposit (REVISI v0.12)
1. /jual → pilih coin → bot tampilkan alamat DEPOSIT sistem + "kirim coin ke sini, lalu ketik /cekdeposit"
2. User kirim dari wallet pribadi → watcher nunggu N konfirmasi (BNB 15, ETH 12, SOL 32)
3. Cukup konfirmasi → IDR masuk (bid − spread − fee network) → notif
4. Deposit tak teridentifikasi = pending manual, tidak auto-kredit
5. Jual ditolak kalau pause spike/stale/kurs abnormal/node mati. Jual tetap boleh kalau pause-nya cuma float tipis

### FLOW 4 — withdraw IDR (Stage 2)
1. /tarik → nominal (min 100rb) + pilih/tambah rekening
2. **Norek baru:** PIN + name-inquiry + cooling-off (durasi di Settings). Multi norek boleh (#19)
3. Tampil: nominal, fee, terima bersih
4. Potong saldo ke hold → <1jt langsung disbursement → >1jt pending
5. Pending → /approve → jalan, atau reject + refund dari hold
6. Status akhir: success / failed (refund) / **unknown** (hold tetap, jangan refund, sampai rekonsiliasi)
7. Notif sukses/gagal

### FLOW 5 — pantau (REVISI v0.12)
1. /saldo = IDR + payout terakhir (coin, units terkirim, alamat tujuan, txhash, status)
2. Tidak ada valuasi portofolio — coin langsung terkirim, bukan disimpan
3. /riwayat = 10 payout/deposit terakhir (txhash + confirm count + status)
4. /harga = refresh tiap dibuka, bukan push per detik
5. /status = cek payout unknown (Stage 2; Stage 1 selalu confirmed-mock)

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
bid     = mid_idr × (1 − spread_sell)   // user jual (deposit)
Beli:  units      = amount_idr / ask
      fee_units  = estimasi_fee_network (per coin, di Settings)
      terkirim   = units − fee_units   // yang sampai ke wallet user
      total_idr  = amount_idr (fee motong units, bukan nambah IDR — tampil terpisah)
Jual:  amount_idr = units_masuk × bid − fee_potong
P&L spread per trade = (harga pakai − mid) × units, tanda ikut sisi
Tolak quote kalau fee_units > 20% units ("nominal terlalu kecil untuk fee")
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

### 7.3 Float hot-wallet & circuit breaker (v0.12, REVISI dari eksposur bandar)

- **Float** per coin = saldo hot-wallet sistem siap kirim. Di bawah `float_min_<coin>` → pause **beli** coin itu saja. Deposit/jual tetap boleh (nambah float)
- Spike (> Y% dalam Z menit), data basi, kurs abnormal, node mati, atau broadcast macet → pause **beli dan jual**
- Admin lepas pause via bot; tiap pause/resume masuk audit, termasuk jenisnya
- Float yang dipakai trial sandbox (mock) ditandai dan tidak disalin ke Stage 2
- Log metrik: slippage per trade, float per coin, payout gagal/unknown, quote ditolak, jenis pause

### 7.4 Hot-wallet (uang asli diputus Stage 0)

- **Float nyata:** pegang BNB/ETH/SOL di hot-wallet senilai batas kirim harian + buffer. Risiko: key curi, nonce macet, gas spike. Mitigasi: key di env 600 (MVP) → HSM/KMS, nonce manager, fee estimator, pause otomatis
- **Isi ulang:** transfer manual owner dari cold wallet + jurnal float-in. Bukan hedge, bukan inventory bandar
- **Node:** 1 provider per chain (nama + endpoint + rate limit fix Stage 0). Node mati = pause beli dan jual coin itu
- Sandbox terkunci: float mock tercatat saja, payout = txhash mock. Ini bukan keputusan float Stage 2

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
- Rekonsiliasi harian: sum jurnal = sum dompet = sum counter yang seharusnya, dan (Stage 2) saldo provider + saldo hot-wallet ON-CHAIN. Payout confirmed wajib ada txhash; unknown → alert + pause coin itu. Selisih ≠ 0 → alert + pause beli dan jual
- Spread tidak masuk `house:revenue` sebagai kas. Akun itu jangan dipakai untuk menyimpan notional

### 8.2 Templat jurnal

Tiap jurnal, sum Amount per Asset = 0. Spread bukan kaki kas.

Beli `amount` IDR kirim `terkirim` koin ke wallet user (v0.12, REVISI — coin tidak ngendap):

- user IDR −amount
- house:cash IDR +amount
- house:float koin −terkirim (keluar hot-wallet)
- house:fee-collect koin +fee_units (fee network terkumpul, bukan laba — buat bayar gas)
- P&L spread = (ask − mid) × units, disimpan di trade, bukan di saldo revenue
- Payout row: queued → confirmed (txhash) / failed (refund IDR) / unknown (tahan)

Jual via deposit (v0.12, REVISI): user kirim `units_masuk` ke deposit sistem, cukup
konfirmasi → IDR masuk:

- house:float koin +units_masuk (masuk hot-wallet)
- user IDR +amount_bayar (bid − spread − fee)
- house:cash IDR −amount_bayar
- P&L = (mid − bid) × units. Deposit row: pending → credited / manual

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
WalletAddresses { ID, UserID, Asset (BNB/ETH/SOL), Address, Network (BSC/ERC20/SOL), Status (active/replaced), CreatedAt }  -- BARU v0.12, unique (UserID, Asset) where active
Payouts { ID, UserID, TradeID unique, Asset, Units, FeeUnits, SentUnits, ToAddress, Network, TxHash unique, Status (queued/broadcasting/confirmed/failed/unknown), Confirms, CreatedAt }  -- BARU v0.12
Deposits { ID, UserID, Asset, FromAddress, TxHash unique, Units, Confirms, RequiredConfirms, Status (pending/credited/manual), CreatedAt }  -- BARU v0.12
LedgerAccounts { ID, Owner (user:<id> / house:cash / house:float / house:fee-collect / house:withdraw-hold / house:provider-clearing), Asset (IDR/BNB/ETH/SOL) }
Journals { ID, Type (topup/buy/sell/payout/float-in/deposit/withdraw/refund/adjust), RefTable, RefID, CreatedAt }
LedgerEntries { ID, JournalID, AccountID, Asset, Amount (signed), CreatedAt }  -- per jurnal, total Amount per Asset = 0
Wallets { UserID, Asset, Balance }  -- cache turunan, unique (UserID, Asset). Coin user normalnya NOL (transit)
TopupInvoices { ID, UserID, AmountIDR, FeeIDR, ProviderRef unique, QRPayload, Status (pending/paid/paid_late/needs_refund/expired/failed), ExpiresAt, PaidAt, IdempotencyKey unique }
Quotes { ID, UserID, Side (buy/sell), Asset, AmountIDR, QuotedUnits, FeeUnits, ToAddress, Bid, Ask, Mid, SpreadUsed, RateUSDTIDR, RateSource, RateAgeMs, PriceAgeMs, ExpiresAt, Status (active/used/expired) }  -- +FeeUnits+ToAddress v0.12
Trades { ID, UserID, QuoteID unique, Side, Asset, AmountIDR, Units, PriceUsed, MidUsed, SpreadPnlIDR, JournalID, CreatedAt }
BankAccounts { ID, UserID, BankCode, AccountNo, AccountNameVerified, Status (pending_confirm/active/disabled), ConfirmedAt, CooldownUntil }
Withdrawals { ID, UserID, BankAccountID, AmountIDR, FeeIDR, NetIDR, Status (pending/approved/auto_processing/success/failed/unknown/rejected), ProviderRef unique, ApprovedBy, CreatedAt }
PriceSnapshots { ID, Asset, BinancePrice, SpreadBuy, SpreadSell, Ask, Bid, RateUSDTIDR, RateSource, RateAgeMs, PriceAgeMs, CreatedAt }
FloatSnapshots { ID, Asset, FloatBalance, FloatMin, CreatedAt }  -- v0.12, GANTI ExposureSnapshots (net position bandar tidak ada lagi)
ChainEvents { ID, Asset, TxHash, EventType, RawPayload, Confirms, ProcessedAt, CreatedAt }  -- BARU v0.12 (webhook node / watcher)
WebhookEvents { ID, Provider, EventType, RawPayload, SignatureValid, ProcessedAt, CreatedAt }
RiskEvents { ID, UserID, TelegramID, Kind, Payload, CreatedAt }  -- Stage 1 nulis, Stage 2 yang menghukum
Settings { Key, Value, UpdatedBy, UpdatedAt }  -- spread, limit, fee network, float min, confirms, deposit address, threshold, cooldown, staleness_coin_ms, staleness_fx_ms, fx_divergence_pct, pause per coin, admin allowlist, rate limit
AdminAudit { ID, AdminTelegramID, Action, TargetID, Before, After, CreatedAt }
```

`house:revenue` tidak dipakai untuk kas spread di MVP. Jangan ditambah "biar lengkap".

### 8.4 Indexes

- Users.TelegramID unique, Wallets(UserID, Asset) unique, Trades.QuoteID unique
- Payouts.TxHash unique, Payouts.TradeID unique, Deposits.TxHash unique
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
- Key hot-wallet: env file 600, TIDAK di repo/log. Rotasi manual + jurnal float-in/out
- Ganti alamat wallet: PIN + cooling-off + audit (salah alamat = hilang, tanggung user)
- Enforcement butir sybil / mule / PIN di atas

Peringatan ke user: nilai coin bisa turun, quote cuma 30 detik, salah alamat = hilang,
fee network motong koin yang diterima.

## 10. API Endpoints & Bot Commands

Internal (bukan public):
- POST /webhook/telegram — update bot, secret token wajib sejak Stage 1
- POST /webhook/<provider> — QRIS callback (verify + idempotent) [Stage 2]
- POST /webhook/<provider>-disbursement — status withdraw [Stage 2]
- POST /webhook/chain/<coin> — watcher deposit/konfirmasi (verify + idempotent) [Stage 2]
- GET /internal/prices — harga terakhir + umur data + jenis pause (bot + debug)
- GET /internal/float — float hot-wallet per coin + minimum (bot + debug) [v0.12]
- Job: refresh kurs; WS Binance; fee estimator per chain; watcher deposit; retry payout unknown; rekonsiliasi harian (termasuk on-chain); cek float

Bot commands:
- User: /start /saldo /harga /beli /jual /alamat /cekdeposit /status /riwayat /batal (*Stage 2: /topup /tarik)
- Admin, allowlist saja: /approve* /setspread /setlimit /pause /setfee /setfloat /broadcast /trial (*Stage 2)

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
/internal/chain/        — 1 interface kirim + watcher per coin (v0.12, BARU). Stage 1 = mock (txhash MOCK-xxx, selalu confirmed). Stage 2 = node beneran (broadcast + nonce + retry + konfirmasi). Modul lain dilarang broadcast langsung
/internal/admin/      — approve + setspread/setlimit + pause + setfee/setfloat + broadcast + audit before/after
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

// CHAIN — satu-satunya pintu on-chain (v0.12, BARU). Stage 1 = mock.
ChainService.EstimateFee(ctx, asset) (feeUnits, error) // dari Settings (Stage 1 mock tetap) / estimator node (Stage 2)
ChainService.Payout(ctx, tradeID, asset, toAddress, units) (txHash, error) // broadcast + catat Payouts queued→confirmed/failed/unknown
ChainService.CheckPayout(ctx, payoutID) (status, confirms, error) // unknown = tahan, retry backoff, jangan refund
ChainService.WatchDeposits(ctx) error // watcher: tx masuk ke deposit address → cukup confirms → kredit IDR
ChainService.ValidateAddress(asset, address) error // format per network, tanpa node

// PAYMENTS — interface, Stage 1 mock
PaymentsService.CreateQRIS(ctx, amount, fee) (providerRef, qrPayload, expiresAt, error)
PaymentsService.HandleCallback(ctx, rawPayload, signature) error // verify + idempotent + simpan WebhookEvents mentah
PaymentsService.Disburse(ctx, bankCode, accountNo, netAmount) (providerRef, error)
PaymentsService.CheckStatus(ctx, providerRef) (status, error)    // unknown = tahan, jangan refund
```

### 11.3 Alur end-to-end (fondasi flow bab 6)

**Beli + auto-kirim (FLOW 2, v0.12):**
handler parse → cek alamat tersimpan (tidak ada = tolak) → Risk.CheckBeforeQuote (float cukup?) → Price.GetQuote + Chain.EstimateFee (fee > 20% = tolak) → simpan Quotes row (+FeeUnits+ToAddress) → tampil 3 baris + tombol confirm → user confirm → Risk.CheckBeforeExecute → Ledger.ExecuteTrade (quote atomik + jurnal transit) → Chain.Payout (mock txhash / broadcast) → tampil txhash. Coin TIDAK masuk Wallets user. Quote lewat 30 detik = row expired, requote bikin row baru.

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
- Fase 8 (v0.12, BARU): chain mock + /alamat + quote 3 baris + payout mock + jurnal transit. Syarat: v0.12 dikunci. Keluar: beli → txhash mock, coin tidak ngendap
- Fase 9 (v0.12, BARU): deposit watcher mock + /jual deposit + /cekdeposit + /status + float mock. Keluar: jual → IDR masuk, tombol ALL mati
- Fase 5–7: payments asli + paid_late exact-match + withdraw hold/unknown + enforcement anti-abuse. Syarat: gate Stage 2 centang semua
- Fase 10 (v0.12, BARU): chain beneran — hot-wallet + node + broadcast + retry unknown + rekonsiliasi on-chain. Syarat: gate Stage 2 (termasuk node + float) centang semua

Checkpoint keras: payments asli, chain beneran, kurs kedua, enforcement mule/sybil = SETELAH gate. Jangan naik ke Stage 1.

## 12. Roadmap

Estimasi kasar, kerja santai, +buffer. Kalibrasi ulang setelah Fase 0.

**Stage 0 — Validasi (1–2 minggu)**
- Tanya 2–3 gateway, pilih sumber kurs + pembanding, pilih node + float hot-wallet, jadwal fee network, isi unit economics
- Boleh paralel dengan Fase 0. Jangan paralel dengan pembayaran/chain asli

**Stage 1 — Sandbox (±4–5 minggu, +Fase 8-9 v0.12)**
- Fase 0 — ✅ DONE 2026-10-29 (repo Go 1.27.3, skeleton /start /saldo, migrasi 000001 users+settings, smoke cbt-vps hijau, commit 8cd6ec3). Skema ledger full + templat jurnal lanjut Fase 1
- Fase 1 — ✅ DONE 2026-10-30 (migrasi 000002 ledger full, PriceService WS live + fail-closed, settings cache, GrantSandbox idempotent + Reconcile, /harga + /saldo real, 7 tests hijau, commit 8561ffb). Kurs masih manual-fase1 (nama sumber fix Stage 0)
- Fase 2 — ✅ DONE 2026-10-30 (migrasi 000003 quotes/trades/risk_events, CreateQuote + ExecuteTrade atomik, RiskService 2 pause, /beli /jual ALL + inline confirm + /riwayat + /batal, 13 tests hijau, head fbc4456)
- Fase 3 — ✅ DONE 2026-10-30 (migrasi 000004 admin_audit, allowlist env+settings, /setspread /setlimit /pause + audit, rate limit, job rekonsiliasi, 27 tests hijau, head 7899e07)
- Fase 4 — ✅ PARSIAL 2026-10-30 (migrasi 000005 exposure_snapshots, trial report + /broadcast + /trial, job exposure, TRIAL.md, 29 tests hijau, head 05695ad). Trial LIVE: T1 slash 6/6 ✅, bug ALL #1 ketemu + fix, T1-button + T2-button dikode (head a8a43d9), trial button HOLD nunggu v0.12
- Fase 8 — ✅ DONE 2026-10-31 (migrasi 000006 chain transit: wallet_addresses/payouts/deposits/chain_events/float + quotes fee_units/to_address. internal/chain mock: validate/format per network, fee_mock_*, payout MOCK-tx idempotent. CreateQuoteBuy + jurnal transit house:float/fee-collect, coin user NOL. Bot: /alamat + daftar inline t:addr + beli wajib alamat + quote 3 baris + txhash + /status. Trial live: alamat valid/invalid ✅, beli 50rb → MOCK-175 ✅, saldo transit ✅. Head caf4d63/bb9770a/b7b610b)
- Fase 9 — ✅ DONE 2026-11-01 (migrasi 000007 credited_idr + 000008 journal types payout/deposit/float-in. MockDeposit idempotent-by-tx + jurnal float/IDR + FloatStatus. Bot: Jual = deposit address + simulasi masuk t:depsim, Status gabung payout+deposit, /cekdeposit redirect, jual-bandar mati total. Menu UX berlapis (#41) + Status 1 pantau (#42). 41 tests hijau, DB v8, head 5ed66f7)

**Stage 2 — Real-money (setelah gate lolos; ±4–5 minggu)**
- Fase 5 — topup QRIS + webhook idempotent + paid_late exact-match + /topup (5–7 hari)
- Fase 6 — withdraw + PIN + name-inquiry + cooling-off + threshold approve + unknown + /tarik /approve (4–5 hari)
- Fase 7 — sumber kurs pembanding, enforcement anti-abuse, retry webhook, rekonsiliasi provider, alert, closed trial limit rendah (1–2 minggu)
- Fase 10 (v0.12) — chain beneran: hot-wallet + node per chain + broadcast + retry unknown + watcher deposit + rekonsiliasi on-chain (1–2 minggu)

Checkpoint keras: **jangan coding payments/chain asli sebelum Stage 0 selesai + provider + node fix.** Mock interface boleh duluan. Jangan naikkan kurs atau anti-abuse ke versi full di Stage 1.

## 13. Unit Economics

Isi di Stage 0. Template:

```
Revenue kotor per trade = nominal × spread
  contoh: Rp200.000 × 1% = Rp2.000
  ini dari SpreadPnl di trade, bukan dari saldo house:revenue
Biaya per trade:
  - gas hot-wallet per kirim (v0.12, BARU — user bayar fee, tapi sisihkan buffer re-broadcast)
  - slippage rata-rata
  - infra (VPS, node provider, monitoring) / jumlah trade
  - support + operasional
Fee QRIS + disbursement + network = user (#9), tapi cek: fee flat vs min withdraw 100rb vs fee > 20% tolak
Break-even = biaya tetap bulanan / revenue bersih per trade
```

Harus kejawab angka: berapa trade/bulan biar impas, dan float max yang ketahan kalau
gas spike 5x + 3 payout unknown nyangkut bareng.

## 14. Risks

- **Gateway menolak bisnis crypto:** bisa matikan Stage 2. Mitigasi: tanya terbuka di Stage 0; sandbox tetap jalan. Jangan mengunci vendor sebelum ada jawaban
- **Gate operasional bukan izin usaha:** checklist Stage 2 tidak menjawab apakah bisnis ini boleh dijalankan
- **Salah alamat = hilang (BARU v0.12):** user salah ketik alamat, coin terkirim dan tidak bisa ditarik. Mitigasi: validasi format + tampilkan alamat di quote + peringatan tiap Confirm + cooling-off ganti alamat
- **Key hot-wallet curi (BARU v0.12):** seluruh float raib. Mitigasi: env 600, bukan repo; float = batas kirim harian + buffer doang (bukan semua dana); rotasi + HSM/KMS pasca-MVP
- **Nonce macet / gas spike (BARU v0.12):** payout antre, user komplain. Mitigasi: nonce manager, fee estimator, retry backoff, status unknown jujur + /status, pause beli kalau macet > N menit
- **Deposit tak dikenal (BARU v0.12):** user kirim tanpa identifikasi → IDR tidak auto-masuk. Mitigasi: alamat deposit unik per user kalau bisa; kalau 1 alamat global = pending manual + SLA jelas
- **Node mati (BARU v0.12):** tidak bisa broadcast / cek konfirmasi. Mitigasi: pause coin itu dua arah, watcher retry, provider cadangan pasca-MVP
- **Volatilitas 30 detik + data basi:** staleness fail-closed, spread adjustable, pause beli dan jual. Spread 0.7–1% tidak menanggung gerak ini
- **Sybil + mule:** bab 9. Enforcement Stage 2; Stage 1 hanya event
- **Webhook dobel/palsu + race:** signature + idempotent + quote atomik + lock order + payout idempotent by TradeID
- **SIM swap:** PIN + rate limit, Stage 2. Bukan pengganti 2FA Telegram
- **paid_late sebagai jalan pintas limit:** exact-match + cek limit di transaksi yang sama. Nominal beda tidak pernah auto-kredit
- **Cash flow (float):** QRIS settle T+2, disbursement + gas keluar dari saldo sendiri
- **Secret bocor:** env saja, bukan repo
- **Limit sandbox terbawa ke produksi:** float + fee mock Stage 1 harus di-set ulang, tidak disalin

## 15. Open Decisions Tersisa (v0.12)

Ditutup di v0.12, jangan dibuka lagi tanpa alasan di bawah:

- **PIVOT transit (#35–40):** bandar custodial v0.11 DIBATALKAN. Beli = auto-kirim ke wallet user, jual = deposit on-chain, fee network user, hot-wallet float, 1 coin = 1 network tetap
- **Fase 8 DONE (2026-10-31):** chain mock + alamat + quote 3 baris + payout mock + jurnal transit. Trial live ✅ (MOCK-175). Head caf4d63/bb9770a/b7b610b
- **Fase 9 DONE (2026-11-01):** deposit mock + jual deposit + Status gabung + jual-bandar mati + menu UX berlapis (#41) + Status 1 pantau (#42). 41 tests hijau, DB v8, head 5ed66f7
- **Debug Fase 3+4 (#33):** 17 temuan → 15 fix + 2 catat. 30 tests hijau
- **Kurs sementara (#34):** Indodax, final fix Stage 0

- **T1 slash live (#T1):** 6/6 ✅ (@cryptostationonly_bot). Bug ALL #1 (double-rounding +1.4e-7) ketemu → fix CreateQuoteUnits + dust clamp (kedaluwarsa v0.12: ALL mati total)
- **T2 slash:** BELUM trial (kalah prioritas sama pivot). T2-button dikode (head a8a43d9) tapi lawan model bandar — perlu trial ulang lawan transit

Masih terbuka:

1. Payment gateway — setelah gateway jawab. Kandidat: Xendit / Midtrans / Tripay / Duitku. Yang mendesak adalah bertanya, bukan memilih
2. Node provider per chain — nama + endpoint + fee + rate limit (Stage 0, v0.12 ganti model stok lama)
3. Float minimum per coin + prosedur isi ulang (Stage 0)
4. Jadwal fee network per chain + sumber estimasi (Stage 0)
5. Nama sumber kurs, nama sumber pembanding, dan nilai X% (Stage 0). Default umur data sudah di-set
6. Spread exact — 0.7% vs 1%, buy/sell beda atau sama (trial Fase 4). Jangan dikunci sekarang
7. Konfirmasi deposit per coin — BNB 15 / ETH 12 / SOL 32 (usulan, boleh geser di Settings)
8. SLA dispute publik — sebelum Stage 2 (termasuk deposit tak dikenal + payout unknown)

## 16. Deploy Strategy

- Dev: long-polling dari cbt-vps (tanpa HTTPS). Lokal nol runtime
- Stage 1: cbt-vps (Ubuntu 24.04, Go 1.27.3, Postgres 16 port 5432, tz Asia/Jakarta) + systemd, satu binary, webhook Telegram via HTTPS (Caddy 443), secret token wajib. DB `cryptostation`, role `cs`
- Stage 2: + webhook provider via HTTPS, alert (rekonsiliasi gagal, eksposur, webhook invalid, sumber kurs mati), backup DB terjadwal
- Env: BOT_TOKEN, WEBHOOK_SECRET, DATABASE_URL, ADMIN_TELEGRAM_IDS. PROVIDER_KEY + HOTWALLET_KEY_<BNB/ETH/SOL> baru Stage 2. Tidak masuk repo

## 17. Implementation Notes

- Payments/chain asli jangan sebelum Stage 0; mock interface boleh
- Format rupiah/coin cuma di layer tampilan; simpan BIGINT / NUMERIC(36,18)
- Semua angka kunci (spread, limit, fee network, float min, confirms, threshold, minimum, staleness koin, staleness kurs, divergence, cooldown, rate limit) via tabel Settings, tanpa restart
- Tiap mutasi uang lewat templat jurnal bab 8. Tidak ada update saldo lepas dari jurnal
- Lock order: Users → LedgerAccounts → Wallets. Payout/deposit ikut lock user yang sama
- Harga basi, sumber down, timestamp kosong, tanpa alamat, fee > 20%: tolak quote. Jangan fallback
- Pause float-tipis dan pause node/spike/stale tidak boleh jadi satu flag tanpa jenis
- Float + fee mock sandbox tidak disalin ke Stage 2
- Test wajib: double-tap confirm, webhook dobel, quote expired detik terakhir, request paralel vs limit harian, beli tanpa alamat (ditolak), fee > 20% (ditolak), payout unknown tidak refund, deposit pending tidak auto-kredit, paid_late nominal sama, paid_late nominal beda, paid_late yang menjebol limit, disbursement unknown tidak refund, perintah admin dari ID di luar allowlist

## 18. Current Next Step

**v0.12-done — Fase 8+9 transit LIVE. Posisi tersimpan.**

Terakhir: head 5ed66f7 (menu berlapis), DB v8, bot active, 41 tests hijau 0 fail.
Fase 8 trial live ✅ (alamat + payout MOCK-175 + saldo transit). Fase 9 kode live,
trial deposit (Jual → depsim → Status) + trial menu berlapis BELUM jalan.

Lanjut nanti (urut):
1. Trial Fase 9 live: Jual → SOL → Rp50rb masuk → Saldo → Status (gabung payout+deposit)
2. Trial menu berlapis: /start → Menu → Saldo → Kembali → Beli (quote ✅❌)
3. Trial T2 ulang lawan transit (setspread/setlimit/pause/trial/broadcast)
4. Stage 0 (STAGE0.md revisi): gateway + node + float + fee network + unit economics
   → hasil masuk v0.13

Setelah gate Stage 2 centang semua → Fase 5-7 (fiat) + Fase 10 (chain beneran).

Jangan mulai payments/chain asli, kurs kedua, atau enforcement mule/sybil sebelum gate.

### Changelog v0.12 → v0.12-done

- **Fase 8 DONE:** migrasi 000006 + internal/chain mock + CreateQuoteBuy + jurnal transit + /alamat + quote 3 baris + payout MOCK-tx. Trial live ✅ (heads caf4d63/bb9770a/b7b610b)
- **Fase 9 DONE:** migrasi 000007 + 000008 + MockDeposit idempotent + jual deposit + Status gabung + jual-bandar mati + menu UX berlapis (#41) + Status 1 pantau (#42). 41 tests hijau, DB v8 (head 5ed66f7)
- **Sell-bandar DIHAPUS:** ALL/SELL units, dust clamp, TestSellFlow + sellall (skip pivot), kode unreachable dibersihkan

### Changelog v0.11 → v0.12

- **PIVOT transit + auto-kirim (bab PIVOT, #1 #3 #6 #9 #11 #15 #16 revisi, baru #35–40):** bandar custodial DIBATALKAN. Beli = otomatis kirim ke wallet user, jual = deposit on-chain, fee network user tampil di quote, hot-wallet float ganti exposure bandar, 1 coin = 1 network tetap, ALL mati
- **Flows (FLOW 2/2b/3/5 revisi):** quote 3 baris + wajib alamat, /alamat + /cekdeposit + /status baru, /saldo = payout bukan valuasi
- **Data model (bab 8 revisi):** WalletAddresses + Payouts + Deposits + ChainEvents + FloatSnapshots (ganti ExposureSnapshots), Quotes +FeeUnits+ToAddress, akun house:float + house:fee-collect, jurnal transit
- **Fase baru (11.6 + bab 12):** Fase 8 (chain mock), Fase 9 (deposit mock), Fase 10 (chain beneran). Stage 0 + gate + risks + economics ikut pivot
- **Status kode:** masih v0.11 (a8a43d9). T1 slash 6/6 ✅, bug ALL #1 fix, trial button HOLD

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
