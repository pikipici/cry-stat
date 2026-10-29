# Crypto Station — Stage 0 Pack (kerja paralel dengan Fase 0)

> Jalankan manual oleh user (pakai akun sendiri). Hasilnya masuk ke plan v0.6.
> Fase 0 tidak menunggu file ini — payments asli baru butuh jawaban gateway.

## S0.1 — Tanya 2-3 gateway (yang mendesak bertanya, bukan memilih)

Kirim ke Xendit + 2 kandidat (Midtrans / Tripay / Duitku). Deskripsi bisnis jujur.

Template pesan:

```
Halo, kami sedang validasi model bisnis sebelum integrasi.

Model: aplikasi bot Telegram di mana user top-up saldo IDR via QRIS,
lalu memakai saldo tersebut untuk membeli/menjual aset kripto
(BNB, ETH, SOL) dengan sistem. Dana keluar via disbursement
transfer bank/e-wallet ke rekening user sendiri.

Pertanyaan:
1. Apakah model bisnis ini diterima di platform Anda? Syarat onboarding apa?
2. Fee QRIS masuk berapa %? Ada fee tetap per transaksi?
3. Fee disbursement keluar berapa per transaksi (bank + e-wallet)?
4. Settlement QRIS berapa lama (T+berapa)? Ada dana float/ditahan?
5. Limit per transaksi QRIS? Limit harian disbursement?
6. Webhook: event apa saja (paid, expired, disbursement status)? Ada signature?
7. Kriteria reject: volume kecil (<100jt/bln) masalah? Perlu badan hukum?
8. Sandbox tersedia? Berapa lama aktivasi production?
```

Catat jawaban per provider di tabel:

| Provider | Diterima? | Fee QRIS | Fee disb. | Settlement | Limit/tx | Webhook sign | Sandbox | Catatan |
|----------|-----------|----------|-----------|------------|----------|--------------|---------|---------|
| Xendit | | | | T+2 / max 20jt (klaim docs) | | | | |
| ... | | | | | | | | |
| ... | | | | | | | | |

Keputusan provider = setelah jawaban masuk, bukan sekarang.

## S0.2 — Sumber kurs USDT/IDR (utama + pembanding)

Pilih nama sumber utama + pembanding untuk Stage 2 (bab 7.2 plan).
Syarat: ada API publik, update <60 detik, timestamp jelas.

Kandidat: ticker Indodax USDT/IDR, ticker Tokocrypto, rate internal provider, lainnya.

| Peran | Sumber | Update interval | Ada timestamp? | Free? |
|-------|--------|-----------------|----------------|-------|
| Utama | | | | |
| Pembanding | | | | |

Nilai X% (selisih abnormal → pause beli+jual): tentukan setelah lihat spread normal antar sumber (usulan awal 1-2%).

Stage 1 jalan dengan sumber utama saja + fail-closed. Pembanding hidup sebelum gate Stage 2.

## S0.3 — Model stok uang asli (inventory vs hedge)

Pilih SATU sebelum gate Stage 2 (sandbox terkunci: posisi house tercatat, tanpa hedge):

- **Inventory nyata:** pegang BNB/ETH/SOL beneran senilai batas eksposur. Risiko pasar sendiri. Butuh: akun exchange + modal awal + prosedur top-up stok.
- **Hedge otomatis:** setiap trade user dibalas trade berlawanan di venue lain. Butuh: akun + API trading + biaya hedge masuk unit economics.

Nilai batas eksposur per coin (Rp): ___________ (usulan: mulai kecil, misal 5-10jt/coin, naikkan setelah trial).

## S0.4 — Unit economics (isi angka)

```
Asumsi per trade: nominal rata-rata Rp________ × spread ____% = Rp________ /trade

Biaya per trade:
- hedge/slippage rata-rata: Rp________
- fee provider (walau ditanggung user, pengaruh ke minat): QRIS ____% + disb. Rp________
- infra (VPS ____rb/bln + lain) / ____ trade = Rp________
- support/ops: Rp________

Revenue bersih/trade = Rp________
Break-even = biaya tetap/bln Rp________ / bersih per trade = ________ trade/bln

Eksposur max: kalau harga gerak 5% melawan posisi Rp________ → rugi Rp________ (ketahan? ya/tidak)
```

Dua pertanyaan wajib kejawab angka:
1. Berapa trade/bulan biar impas? ________
2. Eksposur max yang ketahan kalau -5%? Rp________

## S0.5 — Checklist masuk plan v0.6

- [ ] S0.1 jawaban 2-3 gateway tercatat, provider dipilih
- [ ] S0.2 sumber kurs utama + pembanding + X% fix
- [ ] S0.3 model stok + batas eksposur uang asli fix
- [ ] S0.4 unit economics masuk akal
- [ ] Gate Stage 2 bisa dinilai centang/belum
