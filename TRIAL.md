# Crypto Station — Trial Sandbox Fase 4 (checklist manual, butuh BOT_TOKEN)

> Status: menunggu token bot (username fix di BotFather). Bagian otomatis (metrics)
> sudah hijau; bagian live di bawah jalan setelah bot long-poll dengan token asli.

## T1 — Smoke commands (1 user)
- [ ] /start → akun + grant 1jt + label SIMULASI
- [ ] /harga → 3 coin bid/ask (WS live, bukan basi)
- [ ] /beli SOL 50000 → quote + Confirm → sukses, cek /saldo
- [ ] /jual SOL ALL → sukses, cek /saldo
- [ ] /riwayat → 10 terakhir tampil
- [ ] /batal → quote aktif expired

## T2 — Admin (allowlist = telegram ID lu)
- [ ] /setspread SOL buy 0.008 → berubah, audit tercatat
- [ ] /setlimit min_buy_idr 25000 → berubah
- [ ] /pause SOL exposure → /beli SOL ditolak, /jual SOL tetap boleh
- [ ] /pause SOL spike_stale → beli DAN jual ditolak
- [ ] /pause SOL off → normal lagi
- [ ] /trial → report reconcile OK + quote age + exposure
- [ ] /broadcast halo → user terima
- [ ] Perintah admin dari ID lain → "Bukan admin" + tercatat di admin_audit

## T3 — Exit criteria (bab 3 plan)
- [ ] Reconcile selisih 0 selama 7 hari berturut-turut (catat tanggal mulai: ____)
- [ ] Tidak ada quote dikonsumsi 2x (double-tap test hijau + trial live)
- [ ] Kedua jenis pause teruji (T2 di atas)
- [ ] Quote ditolak saat sumber down/stale (matikan WS / tunggu basi → /beli ditolak)
- [ ] Slippage quote vs pasar dalam batas (lihat /trial quote age, target <20s)
- [ ] Allowlist menolak ID luar (T2 di atas)

## T4 — Tuning spread
- Spread awal 1%. Setelah 20+ trades, lihat P&L vs slippage:
  - P&L rata-rata per trade: Rp____
  - Quote age rata-rata: ____s
  - Keputusan: tetap 1% / turun 0.8% / naik (catat + /setspread)
