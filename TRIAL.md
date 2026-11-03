# Crypto Station — Trial Sandbox Terkini

> Status: PENDING MANUAL TELEGRAM TRIAL — correctness automation hijau; workspace restart/recovery sudah dilakukan, tap admin/non-admin belum selesai.
> Updated: 2026-11-04
> Code head: `fe1d906`; docs head: `c0c8700`
> DB workspace: v14 (`000014_payout_correctness.sql`)
> Mode: SANDBOX / SIMULASI — bukan transaksi on-chain
> Workspace bot: restart pernah dilakukan pada correctness head sebelumnya; binary `fe1d906` sudah build + DB smoke, tetapi belum direstart lagi setelah follow-up tanpa approval baru.
> Live production: belum disentuh; jangan deploy/restart tanpa approval eksplisit user

Dokumen lama Fase 4 yang masih memakai `/jual SOL ALL` sudah tidak berlaku. Model aktif adalah transit: beli menghasilkan payout mock ke wallet user; jual adalah deposit mock ke sistem. `ALL/MAX` sudah dibatalkan.

## Prasyarat

- [x] Kode Admin Wallet v0.14/v0.14.1 tersedia dan dipush.
- [x] Full test, vet, build, dan `scripts/verify.sh --no-db` hijau di VPS.
- [x] DB workspace berada di migration v14.
- [x] Correctness automated verification hijau pada `fe1d906`.
- [x] Workspace restart/recovery pernah diverifikasi pada `50ad758` (feed/data/linkage tetap tersedia).
- [ ] Restart bot workspace ke binary follow-up `fe1d906` sebelum tap trial, jika user menyetujui.
- [ ] Akun admin asli tersedia.
- [ ] Akun non-admin kedua tersedia untuk denial test.

Restart di dokumen ini hanya berlaku untuk bot workspace/sandbox dan tetap perlu persetujuan user. Ini bukan izin menyentuh live production.

## T1 — Trial Admin Wallet (akun admin)

- [ ] `/admin` menampilkan Wallet & Aset dan Transaksi.
- [ ] Ringkasan BNB/ETH/SOL tampil dengan label SIMULASI.
- [ ] Harga/FX basi tidak menghasilkan estimasi IDR palsu.
- [ ] Deposit Stok → SOL.
- [ ] Input `0` dan `abc` ditolak dan flow bisa retry.
- [ ] Input units valid + reference baru menghasilkan preview.
- [ ] Sebelum Confirm, saldo dan jurnal belum berubah.
- [ ] Cancel tidak membuat jurnal.
- [ ] Reference cancelled ditolak saat dipakai ulang.
- [ ] Reference baru + Confirm membuat satu jurnal cold→float.
- [ ] Double-tap/Confirm ulang tidak menggandakan saldo atau jurnal.
- [ ] Retry reference identik menampilkan hasil lama.
- [ ] Reference sama dengan units berbeda ditolak sebagai conflict.
- [ ] Feed menampilkan `STOCK_IN` terbaru.
- [ ] Detail menampilkan journal/admin/ref/address/status/waktu WIB.
- [ ] Toggle feed 10/25 bekerja.
- [ ] Beli mock menghasilkan `PAYOUT_OUT`.
- [ ] Jual/deposit mock menghasilkan `USER_DEPOSIT_IN`.
- [ ] Spread, Limit, Pause, Trial, dan Broadcast tidak regresi.
- [ ] User commands `/beli`, `/jual`, `/status`, `/harga`, `/saldo`, `/riwayat` tidak regresi.

## T2 — Trial Non-admin

- [ ] Non-admin mencoba callback/menu admin dan ditolak.
- [ ] Non-admin mencoba menebak detail `STOCK_IN` dan tidak mendapat data.
- [ ] Tidak ada business mutation pada request yang ditolak.
- [ ] `admin_audit` mencatat denial untuk Telegram ID non-admin.

## T3 — Restart Recovery Workspace

- [x] Workspace restart awal sudah dilakukan dengan backup pre-restart.
- [x] Feed/data tetap terbaca setelah restart (evidence report: 763 payout, 147 deposit).
- [x] Refund linkage pasca-restart unik (24 linkage, 0 double pada evidence report).
- [x] Durable recovery pipeline follow-up sudah automated-tested dan terintegrasi ke runtime (`fe1d906`).
- [ ] Restart workspace ke binary `fe1d906` lalu verifikasi log recovery worker, jika user menyetujui.
- [ ] Setelah tap trial, buka ulang feed/detail dan ulangi reference untuk persistence/idempotency manual.

## T4 — Restore Drill

- [ ] Buat backup PostgreSQL custom-format.
- [ ] Validasi backup dengan `pg_restore --list`.
- [ ] Restore ke dedicated database drill, bukan DB utama.
- [ ] Goose status menunjukkan v14.
- [ ] `recorded_mock` tanpa journal = 0.
- [ ] Jurnal `float-in` dengan jumlah leg bukan dua = 0 baris.
- [ ] Feed/detail dapat dibaca dari DB drill bila diperlukan.

Restore drill bersifat operasional dan hanya dijalankan pada maintenance window yang disetujui.

## T5 — Exit Criteria Sandbox yang Masih Berlaku

- [ ] Reconcile ledger selisih 0 selama 7 hari berturut-turut.
- [ ] Quote/Confirm dan stock deposit tidak dapat dikonsumsi dua kali.
- [ ] Pause float-tipis menolak beli saja; spike/stale menolak beli dan jual.
- [ ] Quote ditolak saat source stale/down, alamat tidak ada, atau fee terlalu besar.
- [ ] Payout mock confirmed memiliki txhash mock dan tidak nyangkut queued.
- [ ] Allowlist admin menolak ID luar.
- [ ] Semua layar dan record trial tetap berlabel SIMULASI.

## Hasil Trial

Isi setelah eksekusi nyata:

- Tanggal mulai: __________
- Code head: __________
- Bot workspace restart: ya/tidak
- Admin trial: PASS/FAIL
- Non-admin denial: PASS/FAIL
- Restart recovery: PASS/FAIL
- Restore drill: PASS/FAIL/NOT RUN
- Temuan: __________

Jangan menandai item selesai hanya berdasarkan automated test. Checklist ini membutuhkan evidence manual Telegram atau evidence operasional sesuai bagiannya.
