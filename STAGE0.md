# Crypto Station — Stage 0 Pack Terkini

> Status: OPEN / MANUAL VALIDATION — belum lengkap, menjadi gate sebelum Stage 2 real-money.
> Updated: 2026-11-04
> Model aktif: transit + auto-kirim dari hot wallet baru khusus bot
> Deposit fiat: QRIS asli lebih dulu; transfer bank manual setelah QRIS stabil 2–4 minggu
> Sandbox saat ini: Revisi V.1 + correctness follow-up automated done (`fe1d906`), DB workspace v14; manual Telegram trial pending

Dokumen lama yang membahas inventory vs hedge sudah tidak berlaku. Crypto Station tidak memakai model bandar custodial. Coin user tidak mengendap di bot; sistem memegang float hot-wallet operasional untuk payout dan menerima deposit coin ketika user menjual.

Stage 0 dikerjakan manual/paralel. Tidak boleh memulai payment gateway asli, node/chain asli, private key, atau uang nyata sebelum checklist gate selesai dan user memberi approval eksplisit.

## S0.1 — Payment Gateway QRIS

Hubungi 2–3 provider secara jujur. Kandidat awal: Xendit, Midtrans, Tripay, Duitku.

Template:

```text
Halo, kami sedang memvalidasi model bisnis sebelum integrasi.

Model: bot Telegram tempat user top-up saldo IDR melalui QRIS, lalu memakai
saldo tersebut untuk membeli BNB/ETH/SOL. Aset yang dibeli otomatis dikirim
ke wallet pribadi user. User juga dapat menjual dengan mengirim aset ke
alamat deposit sistem. Kami ingin mengaktifkan QRIS lebih dulu; transfer bank
manual baru dipertimbangkan setelah QRIS stabil.

Pertanyaan:
1. Apakah model ini diterima? Apa syarat onboarding dan legal entity?
2. Berapa fee QRIS, settlement time, reserve/hold, dan limit per transaksi?
3. Webhook event apa yang tersedia? Bagaimana signature dan retry policy?
4. Bagaimana paid-after-expiry, amount mismatch, refund, dan dispute ditangani?
5. Apakah sandbox tersedia dan berapa lama aktivasi production?
6. Apakah penggunaan untuk transaksi aset kripto dibolehkan secara tertulis?
```

| Provider | Diterima tertulis? | Fee QRIS | Settlement | Limit | Webhook/signature | Sandbox | Catatan |
|---|---|---|---|---|---|---|---|
| Xendit | | | | | | | |
| Midtrans/Tripay/Duitku | | | | | | | |
| Provider 3 | | | | | | | |

Keputusan provider hanya dibuat setelah jawaban tertulis masuk.

## S0.2 — Kurs USDT/IDR

Stage 1 saat ini memakai Indodax sebagai sumber sementara dan fail-closed. Sebelum Stage 2 tentukan sumber utama, pembanding, timestamp, interval, dan divergence threshold.

| Peran | Sumber | Update interval | Timestamp jelas? | Rate limit | Status |
|---|---|---|---|---|---|
| Utama | Indodax sementara | 30 detik saat ini | | | Belum final |
| Pembanding | | | | | Belum dipilih |

- [ ] Sumber utama final dipilih.
- [ ] Sumber pembanding hidup.
- [ ] Nilai divergence `X%` ditetapkan berdasarkan observasi spread normal.
- [ ] Staleness coin dan FX direview untuk production.
- [ ] Kondisi source down/divergence mem-pause beli dan jual.

## S0.3 — Hot Wallet Baru Khusus Bot

Hot wallet harus terpisah dari wallet utama owner.

- [ ] Buat seed EVM baru secara offline.
- [ ] Buat seed SOL baru secara offline.
- [ ] Simpan seed di dua salinan kertas/offline; jangan kirim lewat chat atau commit.
- [ ] Catat satu alamat publik EVM untuk BNB BSC + ETH ERC20.
- [ ] Catat satu alamat publik SOL.
- [ ] Tentukan cold-wallet owner dan prosedur transfer manual cold→hot.
- [ ] Tentukan siapa yang boleh melakukan refill dan bagaimana bukti/audit dicatat.

Alamat publik boleh dimasukkan melalui admin flow sandbox. Private key baru boleh masuk env file mode 600 pada gate Stage 2 + Fase 10.

```text
HOTWALLET_KEY_EVM=[JANGAN ISI DI REPO/CHAT]
HOTWALLET_KEY_SOL=[JANGAN ISI DI REPO/CHAT]
```

## S0.4 — Node Provider dan Chain Operations

Pilih satu provider utama per chain dan catat fallback strategy.

| Chain | Provider | Endpoint class | Rate limit | Fee | Webhook/WS | Status |
|---|---|---|---|---|---|---|
| BSC | | | | | | |
| Ethereum | | | | | | |
| Solana | | | | | | |

- [ ] Provider dan plan dipilih.
- [ ] Rate limit dan SLA diketahui.
- [ ] Metode broadcast, status, confirmation, dan retry diketahui.
- [ ] Node down menghasilkan pause dua arah.
- [ ] Confirmation target direview: BNB 15, ETH 12, SOL 32 (masih usulan/settings).

## S0.5 — Float Minimum dan Prosedur Refill

Tentukan float nyata per coin berdasarkan limit harian, settlement QRIS, payout concurrency, dan buffer gas.

| Asset | Float minimum | Target operasional | Buffer gas | Trigger refill | Owner action |
|---|---:|---:|---:|---:|---|
| BNB | | | | | |
| ETH | | | | | |
| SOL | | | | | |

- Float di bawah minimum → pause beli coin tersebut; jual/deposit tetap boleh.
- Node/source/broadcast abnormal → pause beli dan jual.
- Nilai sandbox/mock tidak boleh disalin otomatis ke production.

## S0.6 — Jadwal Fee Network

| Asset/network | Sumber estimasi | Fee normal | Fee spike | Refresh | Minimum buy yang masuk akal |
|---|---|---:|---:|---|---:|
| BNB/BSC | | | | | |
| ETH/ERC20 | | | | | |
| SOL/Solana | | | | | |

- [ ] Fee estimator/sumber dipilih.
- [ ] Ambang fee >20% units menolak quote.
- [ ] Buffer rebroadcast/gas spike dimasukkan ke economics.
- [ ] Minimum buy Rp25.000 direview terhadap fee nyata, terutama ETH.

## S0.7 — Transfer Bank Manual (Setelah QRIS Stabil)

Bank manual bukan jalur pertama. Putuskan sebelum Fase 5b:

- [ ] Bank dan nomor rekening penerima.
- [ ] Nama pemilik rekening.
- [ ] Kode unik tiga digit dan nominal exact.
- [ ] Expiry 60 menit.
- [ ] Verifikasi hanya berdasarkan mutasi, bukan foto bukti.
- [ ] SLA approval/reject/refund, usulan maksimum 2×24 jam.
- [ ] Satu invoice aktif per user.
- [ ] Prosedur amount mismatch, transfer telat, dan needs_refund.

## S0.8 — Unit Economics

```text
Nominal rata-rata trade: Rp________
Spread buy: ____%     Spread sell: ____%
Revenue kotor/trade: Rp________

Biaya/risiko per trade:
- gas payout: Rp________
- buffer rebroadcast/gas spike: Rp________
- slippage: Rp________
- QRIS/provider: Rp________
- node/VPS/monitoring per trade: Rp________
- support/ops: Rp________

Revenue bersih/trade: Rp________
Biaya tetap bulanan: Rp________
Break-even: ________ trade/bulan

Float yang tertahan saat QRIS settlement: Rp________
Skenario gas 5x + tiga payout unknown: Rp________
Masih tertahan aman? ya/tidak
```

Pertanyaan wajib:

1. Berapa trade per bulan agar impas?
2. Berapa float maksimal yang aman?
3. Apakah minimum transaksi masih masuk akal setelah fee nyata?
4. Apakah tiga payout unknown bersamaan dapat ditahan tanpa double-refund?

## S0.9 — Operational Readiness

- [ ] Unit systemd permanen `cs-bot.service` tersedia sebelum trial panjang.
- [ ] Backup PostgreSQL custom-format terjadwal.
- [ ] Restore drill ke dedicated DB berhasil.
- [ ] Alert source stale, reconcile gagal, float tipis, node down, payout unknown.
- [ ] SLA publik untuk payout unknown dan deposit tak dikenal.
- [ ] Sandbox observation 7 hari memenuhi exit criteria.
- [ ] Trial Admin Wallet admin/non-admin selesai.
- [x] Hardening existing user/buy/payout Phase 2–9 + correctness CF-01–CF-06 selesai otomatis (`fe1d906`).
- [ ] Manual Telegram acceptance admin/non-admin + failed/unknown/retry selesai.
- [ ] Sandbox S1–S6 Top Up production-shaped selesai.

## Gate Stage 2

Semua harus tercentang sebelum real-money/real-chain:

- [ ] Gateway QRIS setuju tertulis.
- [ ] Hot wallet baru siap; alamat publik EVM + SOL terisi; seed offline.
- [ ] Node provider tiap chain hidup.
- [ ] Float minimum dan refill procedure ditetapkan.
- [ ] Sumber kurs utama + pembanding hidup; divergence threshold ditetapkan.
- [ ] Fee network nyata dan ambang 20% berjalan.
- [ ] Unit economics masuk akal.
- [ ] Sandbox exit criteria dan restart recovery lolos.
- [ ] Backup/restore/alerting siap.
- [ ] User memberi approval eksplisit masuk Stage 2.

Checklist operasional ini bukan pernyataan izin hukum. Jangan mengaktifkan gateway/chain nyata hanya karena kode sandbox tersedia.
