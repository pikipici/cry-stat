# Crypto Station — Admin Wallet Operations Plan

> Status: DONE — terimplementasi + terverifikasi workspace 2026-11-02 (T1-T9, deploy live pending persetujuan)
> Target: v0.14-admin-wallet ✅
> Dibuat: 2026-11-02
> Sumber kebenaran fitur ini: file ini, dengan batas arsitektur tetap mengikuti `PROJECT_IDEAS.md`
> Eksekusi: lokal untuk coding + git; build/test/migrate/run hanya di `cbt-vps` sesuai workflow project

## 0. Tujuan

Menaikkan menu admin dari sekadar pengaturan `📦 Stok` menjadi panel operasional wallet yang memungkinkan admin:

1. Melihat seluruh aset hot wallet BNB, ETH, dan SOL.
2. Melihat alamat deposit hot wallet per aset/network.
3. Mencatat deposit/top-up stok crypto dari cold/owner ke hot wallet.
4. Melihat transaksi terakhir yang diproses bot dalam satu feed admin.
5. Melihat detail transaksi, status, txhash, nominal, alamat, waktu, dan sumber transaksi.
6. Tetap aman dan jujur terhadap mode sekarang: sandbox/mock, belum membaca blockchain asli.

## 1. Batas Scope yang Dikunci

### 1.1 Scope v0.14 ini — wajib dibuat

- Panel admin berbasis tombol, bukan mengandalkan slash command.
- Menu `💼 Wallet & Aset` di panel admin.
- Ringkasan aset BNB/ETH/SOL:
  - saldo jurnal `house:float`;
  - float minimum;
  - status `AMAN`, `TIPIS`, atau `BELUM DISET`;
  - alamat hot publik;
  - network;
  - estimasi nilai IDR memakai bid hidup yang tidak stale;
  - label besar `SIMULASI`.
- Flow deposit stok admin:
  - pilih aset;
  - tampilkan network + alamat tujuan;
  - admin memasukkan units dan txhash/reference mock;
  - tampil preview;
  - admin menekan Confirm;
  - baru setelah Confirm jurnal `float-in` dibuat;
  - deposit dicatat sebagai record khusus dan muncul di riwayat.
- Feed transaksi terakhir bot, gabungan:
  - admin stock deposit / float-in;
  - payout beli user;
  - deposit jual user;
  - maksimal default 10 item, tombol 10/25 jika diperlukan;
  - urut terbaru berdasarkan timestamp, dengan tie-breaker stabil.
- Detail transaksi dari tombol inline.
- Audit admin untuk setiap deposit stok yang berhasil atau ditolak.
- Test unit/service/query/bot flow.
- Dokumentasi trial manual.

### 1.2 Tidak masuk scope v0.14 — dilarang dikerjakan

- Membaca saldo on-chain asli.
- Watcher blockchain.
- Auto-detect transfer admin.
- Validasi txhash ke node/explorer.
- Memegang private key/seed.
- Broadcast transaksi on-chain.
- Reconcile jurnal vs blockchain.
- Payment gateway, QRIS, bank, withdraw fiat.
- Mengubah model ledger utama.
- Menyimpan secret/private key dalam DB, repo, log, audit, atau Telegram.

Semua kemampuan on-chain asli tetap milik Fase 10 dan hanya boleh setelah gate Stage 2 lengkap.

## 2. Perilaku yang Diharapkan

### 2.1 Navigasi admin

Panel admin target:

```text
Panel Admin (SIMULASI)

[💼 Wallet & Aset] [🧾 Transaksi]
[📊 Spread]        [🔢 Limit]
[⏸️ Pause]         [📢 Broadcast]
[🧪 Trial]         [❌ Tutup]
```

`📦 Stok` lama diganti/diabsorpsi ke `💼 Wallet & Aset`. Jangan membuat dua menu dengan fungsi tumpang tindih.

Menu Wallet & Aset:

```text
Wallet & Aset (SIMULASI)

[📊 Ringkasan Aset]
[➕ Deposit Stok]
[⚙️ Float Minimum] [⛽ Fee]
[🏦 Alamat Hot]
[⬅️ Panel]
```

### 2.2 Ringkasan aset

Contoh output:

```text
💼 Aset Hot — SIMULASI

BNB · BSC
Saldo jurnal: 2.500000 BNB
Estimasi: Rp17.625.000
Minimum: 1 BNB
Status: ✅ AMAN
Hot: 0x1234...abcd

ETH · ERC20
Saldo jurnal: 0.200000 ETH
Estimasi: Rp7.100.000
Minimum: 1 ETH
Status: ⚠️ TIPIS
Hot: belum diatur

SOL · Solana
Saldo jurnal: 120 SOL
Estimasi: Rp...
Minimum: 100 SOL
Status: ✅ AMAN
Hot: Abcd...xyz

Harga/estimasi: timestamp atau umur snapshot
Catatan: saldo di atas berasal dari jurnal sandbox, bukan saldo blockchain.
```

Aturan:

- Jangan tampilkan estimasi IDR kalau harga atau FX stale/down.
- Jika stale, saldo units tetap tampil dan estimasi menjadi `tidak tersedia (harga basi)`.
- Jangan fallback ke harga lama atau angka hardcoded.
- Alamat dipendekkan untuk daftar, tetapi detail harus bisa menampilkan alamat penuh.
- BNB memakai network `BSC`, ETH `ERC20`, SOL `SOL`/`Solana` secara konsisten.

### 2.3 Deposit stok admin

Flow target:

1. Admin membuka `💼 Wallet & Aset`.
2. Tekan `➕ Deposit Stok`.
3. Pilih BNB/ETH/SOL.
4. Bot menampilkan:
   - aset dan network;
   - alamat hot tujuan;
   - saldo jurnal saat ini;
   - peringatan `SIMULASI — bot belum memantau blockchain`.
5. Jika alamat hot kosong, flow berhenti. Admin diarahkan ke `🏦 Alamat Hot`.
6. Admin mengetik jumlah units positif.
7. Bot meminta txhash/reference.
8. Sandbox menerima reference non-kosong dengan format terbatas:
   - txhash mock: `MOCK-...`; atau
   - txhash format chain yang valid secara sintaks ringan;
   - tidak mengklaim bahwa transaksi sudah ada di blockchain.
9. Bot menampilkan preview:

```text
Konfirmasi Deposit Stok — SIMULASI
Aset: SOL
Network: Solana
Jumlah: 25 SOL
Ke hot: Abcd...xyz
Reference: MOCK-ADMIN-001
Saldo sebelum: 120 SOL
Saldo setelah: 145 SOL

[✅ Catat Deposit] [❌ Batal]
```

10. Hanya callback Confirm yang membuat record deposit + jurnal.
11. Double-tap Confirm harus idempotent: satu sukses, berikutnya menampilkan `sudah diproses`.
12. Sukses menampilkan ID record, journal ID, saldo baru, dan tombol `Lihat Detail`.

### 2.4 Makna deposit pada sandbox

- Deposit stok adalah deklarasi manual admin bahwa aset berpindah dari `house:cold` ke `house:float`.
- Jurnal tetap dua kaki:
  - `house:cold` asset `-units`;
  - `house:float` asset `+units`.
- Jangan membuat IDR leg.
- Jangan menambah saldo lewat update langsung.
- Jangan menyebut `confirmed on-chain`; status sandbox harus eksplisit `recorded_mock` atau istilah setara.

### 2.5 Feed transaksi terakhir bot

Satu daftar gabungan, bukan tiga layar terpisah.

Jenis event:

- `STOCK_IN` — deposit stok admin.
- `PAYOUT_OUT` — crypto dikirim ke user setelah beli.
- `USER_DEPOSIT_IN` — crypto masuk dari user saat jual.

Format ringkas:

```text
🧾 10 Transaksi Terakhir — SIMULASI

1. ➕ STOCK_IN · SOL · 25 SOL
   recorded_mock · MOCK-ADMIN-001 · 02 Nov 22:58 WIB

2. ➖ PAYOUT_OUT · BNB · 0.015 BNB
   confirmed · MOCK-175 · user #42 · 02 Nov 22:50 WIB

3. 📥 USER_DEPOSIT_IN · ETH · 0.02 ETH
   credited · MOCK-DEP-... · user #18 · 02 Nov 22:41 WIB
```

Setiap item memiliki tombol detail. Jika terlalu panjang, tampilkan maksimal 5 item per halaman dengan `⬅️` dan `➡️`, tetapi query limit awal tetap 10 agar implementasi murah.

Detail minimal:

- tipe transaksi;
- internal ID;
- asset + network;
- units;
- fee dan sent units untuk payout;
- credited IDR untuk user deposit;
- status;
- txhash/reference penuh;
- from/to address jika ada;
- user ID internal jika relevan, tanpa membocorkan data rahasia;
- journal ID jika stock deposit;
- admin Telegram ID untuk stock deposit;
- waktu WIB;
- label mock/sandbox.

## 3. Kondisi Kode Saat Ini

Executor wajib membaca file berikut sebelum coding, jangan scan seluruh repo tanpa kebutuhan:

1. `PROJECT_IDEAS.md` — keputusan arsitektur dan batas Stage 2.
2. `.kiro/steering/ADMIN_WALLET_OPERATIONS_PLAN.md` — plan ini.
3. `internal/bot/adminflow.go` — callback dan session admin sekarang.
4. `internal/bot/keyboard.go` — tombol admin sekarang.
5. `internal/bot/session.go` — state machine input.
6. `internal/admin/admin.go` — allowlist, audit, setting stok.
7. `internal/ledger/float.go` — `FloatTopup` sekarang.
8. `internal/ledger/deposit.go` — deposit user mock.
9. `internal/chain/chain.go` — payout dan address validation.
10. `db/queries/chain.sql`, `db/queries/admin.sql`, `db/queries/ledger.sql`.
11. `migrations/000006_chain_transit.sql` sampai `000009_hot_addresses.sql`.
12. Test terkait: `internal/admin/stock_test.go`, `internal/ledger/float_test.go`, `internal/bot/session_test.go`, `internal/ledger/deposit_test.go`, `internal/chain/chain_test.go`.

Kondisi existing yang harus dipertahankan:

- `FloatTopup(asset, units)` sudah membuat jurnal `float-in` dua kaki.
- Ringkasan stok lama memakai `FloatStatus`.
- Menu stok sekarang punya lihat/float/fee/hot/topup.
- Admin allowlist wajib dicek pada message dan callback.
- Semua aksi admin harus tercatat di `admin_audit`.
- Generated `internal/db/` dan `db/schema/` tidak di-commit; regenerate di VPS.

## 4. Desain Data

### 4.1 Migration baru

Buat `migrations/000010_admin_stock_deposits.sql`.

Tabel yang direkomendasikan:

```sql
CREATE TABLE admin_stock_deposits (
  id BIGSERIAL PRIMARY KEY,
  admin_telegram_id BIGINT NOT NULL,
  asset TEXT NOT NULL CHECK (asset IN ('BNB','ETH','SOL')),
  network TEXT NOT NULL CHECK (network IN ('BSC','ERC20','SOL')),
  units NUMERIC(36,18) NOT NULL CHECK (units > 0),
  to_address TEXT NOT NULL,
  tx_ref TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL CHECK (status IN ('pending_confirm','recorded_mock','cancelled','failed')),
  journal_id BIGINT UNIQUE REFERENCES journals(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  recorded_at TIMESTAMPTZ
);

CREATE INDEX idx_admin_stock_deposits_created
  ON admin_stock_deposits(created_at DESC, id DESC);
CREATE INDEX idx_admin_stock_deposits_asset_created
  ON admin_stock_deposits(asset, created_at DESC, id DESC);
```

Catatan desain:

- `tx_ref UNIQUE` adalah idempotency key utama.
- Jangan gunakan txhash palsu sebagai bukti chain; nama kolom `tx_ref` lebih jujur.
- `journal_id UNIQUE` memastikan satu deposit tidak mengarah ke banyak jurnal.
- `to_address` disnapshot saat deposit dibuat; perubahan setting hot address setelah itu tidak mengubah histori.
- Tidak menyimpan private key, raw seed, atau secret.
- Migration wajib punya `-- +goose Up`. Tambahkan Down hanya jika pola project memang memakai Down; jangan merusak konvensi existing.

### 4.2 Query sqlc baru

Tambahkan file `db/queries/admin_wallet.sql` supaya scope jelas.

Query minimum:

- `CreatePendingAdminStockDeposit`.
- `GetAdminStockDepositForUpdate`.
- `MarkAdminStockDepositRecorded`.
- `MarkAdminStockDepositCancelled` bila pending record dibuat sebelum preview.
- `GetAdminStockDepositByID`.
- `GetAdminStockDepositByRef`.
- `ListRecentAdminStockDeposits`.
- `ListRecentPayoutsAdmin`.
- `ListRecentUserDepositsAdmin`.

Strategi feed murah dan aman:

- Jangan langsung membuat query UNION kompleks jika tipe NUMERIC/nullable menyulitkan sqlc.
- Boleh ambil tiga list kecil (`limit <= 25`) lalu merge + sort di service Go.
- Tie-breaker: `CreatedAt DESC`, lalu type rank, lalu `ID DESC`.
- Hindari N+1 query untuk detail list.

## 5. Struktur Service

### 5.1 Ledger service

Refactor `FloatTopup` agar idempotent dan merekam deposit secara atomik.

API target konseptual:

```go
type RecordStockDepositInput struct {
    AdminTelegramID int64
    Asset           string
    Units           decimal.Decimal
    ToAddress       string
    TxRef           string
}

type RecordStockDepositResult struct {
    DepositID   int64
    JournalID   int64
    Asset       string
    Network     string
    Units       decimal.Decimal
    TxRef       string
    Balance     decimal.Decimal
    AlreadyDone bool
}

func (s *Service) RecordAdminStockDeposit(
    ctx context.Context,
    in RecordStockDepositInput,
) (RecordStockDepositResult, error)
```

Transaksi DB wajib:

1. Validasi asset dan units sebelum membuka tx.
2. Begin transaction.
3. Insert/get deposit berdasarkan `tx_ref`.
4. Lock record deposit `FOR UPDATE`.
5. Jika status `recorded_mock` dan journal ada, return hasil idempotent tanpa jurnal baru.
6. Validasi status masih bisa diproses.
7. Buat jurnal `float-in`.
8. Pastikan account `house:cold` dan `house:float`.
9. Buat dua ledger entries dengan sum nol.
10. Mark deposit `recorded_mock`, isi journal ID dan recorded_at.
11. Commit.
12. Baca/return saldo float terbaru.

Jangan melakukan audit admin di dalam ledger package; audit tetap di admin/bot orchestration agar dependensi tidak berputar.

Kompatibilitas:

- `FloatTopup` lama boleh dipertahankan sementara sebagai wrapper untuk test lama, tetapi UI baru wajib memakai API idempotent baru.
- Jika wrapper membuat reference otomatis, tandai deprecated dan jangan dipakai handler.

### 5.2 Admin wallet service/read model

Boleh dibuat `internal/adminwallet/service.go` untuk memisahkan read model dari `internal/admin/admin.go` yang sudah besar.

Tanggung jawab:

- Build `AssetSummary` untuk BNB/ETH/SOL.
- Mengambil saldo jurnal lewat ledger/query yang sudah benar.
- Mengambil float minimum dan hot address dari settings.
- Mengambil bid hidup melalui PriceService untuk estimasi IDR.
- Menghasilkan network mapping terpusat.
- Mengambil dan merge transaksi terakhir.
- Mengambil detail event berdasarkan type+ID.

Kontrak konseptual:

```go
type AssetSummary struct {
    Asset          string
    Network        string
    Balance        decimal.Decimal
    Minimum        decimal.Decimal
    HotAddress     string
    Status         string
    EstimatedIDR   int64
    EstimateOK     bool
    EstimateReason string
    PriceAgeMs     int64
}

type ActivityItem struct {
    Kind      string
    ID        int64
    Asset     string
    Network   string
    Units     decimal.Decimal
    Status    string
    TxRef     string
    UserID    *int64
    CreatedAt time.Time
}
```

Jangan memasukkan Telegram formatting ke service. Service mengembalikan data, bot layer memformat.

## 6. Session dan Callback Bot

### 6.1 Session state

Tambahkan state eksplisit, jangan overload `StepAdminStock` untuk semua tahap:

- `StepAdminStockDepositUnits`
- `StepAdminStockDepositRef`

Field session minimum:

- Coin/Asset.
- Units sebagai string canonical atau decimal-safe string.
- Hot address snapshot.
- TxRef jika diperlukan sebelum confirm.

Session bersifat in-memory dan boleh hilang saat restart karena belum ada mutasi sampai Confirm. Jangan menulis jurnal pada tahap input units/reference.

### 6.2 Callback naming

Gunakan callback pendek dan konsisten agar tidak melebihi limit Telegram:

- `a:wallet`
- `a:wallet:assets`
- `a:wallet:deposit`
- `a:wdep:BNB`
- `a:wdepok:<id-or-short-token>`
- `a:wdepno:<id-or-short-token>`
- `a:activity`
- `a:activity:10`
- `a:act:<kind>:<id>`

Jika preview belum disimpan di DB, callback Confirm tidak boleh bergantung pada data panjang di callback. Pilihan yang disarankan:

- simpan pending deposit row sebelum preview dan callback membawa ID; atau
- simpan state server-side dengan token pendek.

Rekomendasi biaya rendah: pending row di DB, karena sekaligus memberi idempotency dan tahan retry Telegram. Pastikan pending row belum membuat jurnal.

### 6.3 Allowlist dan keamanan

Setiap entry wajib:

- Callback prefix `a:` dicek `IsAdmin` sebelum query/detail.
- Message saat session dicek ulang `IsAdmin`.
- Akses ilegal memanggil `Admin.Deny`.
- Detail transaksi tidak boleh bisa dibuka user biasa dengan callback ID hasil tebakan.
- Asset, status, kind, ID, units, dan txref harus divalidasi server-side.
- Jangan percaya session/callback sebagai sumber saldo atau address; re-read pending record saat Confirm.

## 7. Formatting dan UX

Buat formatter terpisah agar handler tidak membengkak, misalnya:

- `internal/bot/adminwallet_format.go`
- `formatAssetSummary(...)`
- `formatActivityList(...)`
- `formatActivityDetail(...)`
- `shortAddress(...)`
- `shortTxRef(...)`
- `formatWIB(...)`

Aturan copy:

- Selalu tulis `SIMULASI` pada layar aset, deposit, transaksi, dan detail.
- Gunakan bahasa sederhana untuk admin awam.
- Bedakan:
  - `Saldo jurnal`;
  - `Estimasi IDR`;
  - `Alamat hot`;
  - `Reference mock`.
- Jangan mengatakan `saldo wallet on-chain` pada v0.14.
- Jangan mengatakan `deposit terkonfirmasi blockchain`.
- Semua timestamp admin ditampilkan WIB.

## 8. Urutan Implementasi Hemat Cost

Executor wajib mengerjakan berurutan. Jangan lompat ke UI sebelum service/query stabil.

### Task 1 — Baseline dan dokumen

Files:
- `.kiro/steering/ADMIN_WALLET_OPERATIONS_PLAN.md`
- `PROJECT_IDEAS.md` hanya diperbarui setelah implementasi selesai dan terverifikasi.

Steps:
1. Baca file minimum pada Bagian 3.
2. Cek `git status`, branch, dan head.
3. Jalankan baseline test di VPS.
4. Catat jumlah test dan migration version sebelum perubahan.

Acceptance:
- Baseline hijau atau blocker terdokumentasi sebelum coding.

### Task 2 — Migration + sqlc query

Files:
- `migrations/000010_admin_stock_deposits.sql`
- `db/queries/admin_wallet.sql`

Steps:
1. Tambah tabel/index.
2. Tambah query CRUD/state transition/list.
3. Sync ke VPS.
4. Jalankan goose up.
5. Jalankan sqlc generate di VPS.
6. Build compile-only.

Acceptance:
- DB menjadi v10.
- Migration repeat check tidak mengubah apa pun.
- Generated code compile.
- Tidak ada secret di schema/query.

### Task 3 — Ledger idempotent stock deposit (TDD)

Files:
- `internal/ledger/float.go`
- `internal/ledger/float_test.go`

Tests ditulis dulu:
1. Deposit valid membuat satu record + satu jurnal dua kaki.
2. Jumlah kaki per asset = 0.
3. `house:cold` berkurang dan `house:float` bertambah.
4. TxRef sama diproses dua kali menghasilkan satu jurnal.
5. Asset invalid ditolak tanpa row/jurnal.
6. Units nol/negatif ditolak.
7. Reference kosong ditolak.
8. Failure tengah transaksi rollback record/journal/entries.

Acceptance:
- Semua test baru hijau.
- Tidak ada direct update saldo.
- API lama tidak merusak test existing.

### Task 4 — Asset summary service (TDD)

Files rekomendasi:
- `internal/adminwallet/service.go`
- `internal/adminwallet/service_test.go`

Tests:
1. Seluruh BNB/ETH/SOL selalu muncul dan urut tetap.
2. Status aman saat balance >= minimum.
3. Status tipis saat balance < minimum.
4. Hot address kosong menjadi `BELUM DISET`/warning.
5. Estimasi IDR benar saat bid+FX fresh.
6. Harga stale menghasilkan `EstimateOK=false`, tidak fallback.
7. Error satu asset tidak menghilangkan asset lain; tampil reason.

Acceptance:
- Service tidak mengandung formatting Telegram.
- Tidak menggunakan float64 untuk units/harga/estimasi uang.

### Task 5 — Unified activity feed (TDD)

Files:
- `db/queries/admin_wallet.sql`
- `internal/adminwallet/activity.go`
- `internal/adminwallet/activity_test.go`

Tests:
1. Merge tiga jenis event.
2. Urut terbaru stabil.
3. Limit 10 benar.
4. Detail STOCK_IN lengkap.
5. Detail PAYOUT_OUT lengkap.
6. Detail USER_DEPOSIT_IN lengkap.
7. Kind/ID invalid tidak bocor data atau panic.
8. List kosong memberi state yang jelas.

Acceptance:
- Tidak ada N+1 query untuk list.
- Txref/address tidak hilang pada detail.

### Task 6 — Bot keyboard + flow deposit (TDD)

Files:
- `internal/bot/keyboard.go`
- `internal/bot/adminflow.go` atau pecah `internal/bot/adminwallet.go`
- `internal/bot/session.go`
- tests bot/session baru.

Steps:
1. Tambah menu Wallet & Aset serta Transaksi.
2. Migrasikan aksi stok lama ke submenu Wallet & Aset.
3. Implement pilih coin → units → reference → preview → confirm.
4. Implement cancel/back di setiap tahap.
5. Confirm memanggil ledger idempotent API.
6. Audit sukses dengan before/after yang tidak mengandung secret.
7. Double callback menampilkan sudah diproses.

Tests:
1. Non-admin ditolak di callback.
2. Non-admin ditolak saat input session.
3. Deposit tanpa hot address ditolak sebelum input units.
4. Units invalid tidak menghapus session sehingga bisa retry.
5. Reference invalid bisa retry.
6. Cancel membersihkan session/pending state.
7. Confirm sekali menambah float.
8. Confirm dua kali tetap satu jurnal.
9. Navigation back tidak melakukan mutasi.

Acceptance:
- Tidak ada jurnal sebelum Confirm.
- Handler tetap tipis.
- Semua callback admin tetap allowlist-protected.

### Task 7 — Bot asset + activity views

Files:
- `internal/bot/adminwallet_format.go`
- `internal/bot/adminwallet.go`
- formatter tests.

Steps:
1. Render asset summary.
2. Render activity list.
3. Tambah detail buttons.
4. Render detail per kind.
5. Tambah refresh/back buttons.

Acceptance:
- Semua layar berlabel SIMULASI.
- Harga stale tidak tampil sebagai estimasi hidup.
- Output muat dalam limit Telegram; jika panjang, pecah message secara deterministik.

### Task 8 — Full verification di VPS

Commands/pipeline konseptual:
1. Sync/commit sesuai workflow git project.
2. `goose ... up`.
3. `sqlc generate`.
4. `gofmt` file berubah.
5. `go test ./...`.
6. `go vet ./...` jika baseline mendukung.
7. `go build -o /tmp/cs-bot ./cmd/bot`.
8. Smoke DB dengan token kosong.
9. Jalankan bot workspace/sandbox.
10. Trial manual checklist Bagian 10.

Acceptance:
- Migration v10.
- Semua test hijau.
- Build dan smoke exit 0.
- Tidak ada panic/error baru pada log.
- Bot lama tetap bisa /start, /harga, /beli, /jual, /status.

### Task 9 — Dokumentasi dan checkpoint

Files:
- `PROJECT_IDEAS.md`
- `TRIAL.md` atau file trial baru jika ingin menghindari isi lama.
- plan ini: status task diperbarui.

Steps:
1. Catat commit/head, jumlah test, DB version.
2. Tandai fitur yang benar-benar terverifikasi saja.
3. Catat bahwa data masih jurnal sandbox, bukan chain.
4. Jangan menandai on-chain selesai.
5. Commit dan push setelah test hijau.
6. Deploy live hanya jika user eksplisit menyuruh; untuk project ini live sandbox bot tetap perlu persetujuan sebelum restart/mutasi yang berisiko.

## 9. Test Matrix Wajib

### 9.1 Data/ledger

- [ ] Deposit valid BNB.
- [ ] Deposit valid ETH.
- [ ] Deposit valid SOL.
- [ ] Units decimal 18 digit tidak kehilangan presisi.
- [ ] Zero/negative ditolak.
- [ ] Asset lain ditolak.
- [ ] TxRef kosong ditolak.
- [ ] TxRef duplicate idempotent.
- [ ] Journal sum per asset nol.
- [ ] Rollback atomik saat insert entry gagal.

### 9.2 Asset summary

- [ ] Balance >= min = AMAN.
- [ ] Balance < min = TIPIS.
- [ ] Hot address kosong = warning.
- [ ] Fresh price = estimasi IDR.
- [ ] Stale coin = estimasi unavailable.
- [ ] Stale FX = estimasi unavailable.
- [ ] Tidak ada float64 pada money path.

### 9.3 Activity feed

- [ ] Tiga jenis event tergabung.
- [ ] Order timestamp stabil.
- [ ] Limit benar.
- [ ] Empty state benar.
- [ ] Detail type/id valid.
- [ ] Detail type/id invalid aman.
- [ ] Txhash/reference tampil penuh hanya di detail.

### 9.4 Bot/security

- [ ] Admin allowlist callback.
- [ ] Admin allowlist message/session.
- [ ] Unauthorized access diaudit.
- [ ] Deposit tanpa hot address ditolak.
- [ ] Tidak ada mutasi sebelum Confirm.
- [ ] Double Confirm satu jurnal.
- [ ] Cancel tidak membuat jurnal.
- [ ] Session clear setelah sukses/cancel.
- [ ] Semua layar berlabel SIMULASI.

### 9.5 Regression

- [ ] Existing 45 test tetap hijau.
- [ ] `/beli` payout mock tetap jalan.
- [ ] `/jual` deposit mock tetap jalan.
- [ ] Float guard tetap mem-pause beli ketika tipis.
- [ ] Admin spread/limit/pause/broadcast/trial tetap jalan.
- [ ] `/float` lama bila dipertahankan tidak memberi data berbeda dari summary baru.

## 10. Trial Manual Admin

Jalankan setelah automated tests hijau:

1. Buka Admin → Wallet & Aset.
2. Ringkasan menampilkan tiga aset dan label SIMULASI.
3. Cek status AMAN/TIPIS sesuai saldo vs min.
4. Cek harga stale scenario tidak mengarang estimasi.
5. Pilih Deposit Stok SOL.
6. Pastikan network dan hot address benar.
7. Input units invalid, pastikan bisa retry.
8. Input units valid.
9. Input reference mock.
10. Batal di preview, pastikan saldo tidak berubah.
11. Ulangi dan Confirm.
12. Cek saldo bertambah tepat.
13. Tekan Confirm lama lagi/double-tap; saldo tidak bertambah lagi.
14. Buka Transaksi; STOCK_IN ada paling atas.
15. Buka detail; deposit ID, journal ID, admin, address, ref, waktu tersedia.
16. Lakukan satu beli mock user; PAYOUT_OUT muncul.
17. Lakukan satu jual mock user; USER_DEPOSIT_IN muncul.
18. Coba callback dari akun non-admin; ditolak dan audit tercatat.

## 11. Acceptance Criteria Final

Fitur dianggap DONE hanya jika seluruh poin berikut terpenuhi:

- [ ] Admin dapat melihat saldo jurnal BNB/ETH/SOL, minimum, status, address, network, dan estimasi IDR fresh.
- [ ] Admin dapat mencatat deposit stok melalui flow tombol dengan preview+Confirm.
- [ ] Deposit stok idempotent dan double-entry.
- [ ] Admin dapat melihat feed gabungan transaksi terakhir bot.
- [ ] Admin dapat membuka detail tiap transaksi.
- [ ] Semua aksi dilindungi allowlist dan audit.
- [ ] Semua layar jujur menyebut SIMULASI dan tidak mengklaim saldo on-chain.
- [ ] Migration v10 sukses.
- [ ] Seluruh test lama+baru hijau.
- [ ] Build dan DB smoke hijau di VPS.
- [ ] Trial manual selesai dengan bukti hasil.
- [ ] `PROJECT_IDEAS.md` diperbarui sesuai hasil aktual, bukan rencana.

## 12. Larangan Implementasi

- Jangan memasukkan seed/private key ke bot.
- Jangan meminta seed/private key lewat Telegram.
- Jangan menganggap txRef sebagai transaksi chain tervalidasi.
- Jangan menambah saldo float hanya dari input message tanpa Confirm.
- Jangan membuat jurnal kedua ketika callback di-retry.
- Jangan memakai float64 untuk coin, harga, atau nilai IDR.
- Jangan query SQL langsung dari bot handler.
- Jangan mengupdate wallet/cache di luar ledger service.
- Jangan menghilangkan allowlist check demi mempermudah test.
- Jangan menyentuh payments/on-chain real pada task ini.
- Jangan deploy/restart live tanpa persetujuan user yang eksplisit.

## 13. Backlog Setelah v0.14 — Bukan Sekarang

Fase 10/on-chain nanti dapat mengganti sumber data tanpa mengubah UX utama:

1. Chain adapter membaca native balance hot address.
2. Asset summary menampilkan `saldo on-chain`, `saldo jurnal`, dan `selisih`.
3. Watcher mendeteksi deposit admin berdasarkan txhash/address.
4. Status deposit berubah `detected → confirming → confirmed/failed/unknown`.
5. Jurnal float-in dibuat hanya setelah confirmation threshold.
6. Reconcile otomatis mem-pause coin jika chain vs ledger berbeda.
7. Explorer deep-link per txhash.
8. Alert untuk deposit unknown, payout unknown, node down, dan saldo gas tipis.

Backlog ini baru dieksekusi setelah Stage 2 gate lengkap dan node provider dipilih.

## 14. Progress Checklist Executor

- [x] T1 Baseline diverifikasi (VPS v9, tests hijau, 62 runs)
- [x] T2 Migration + query selesai (000010, v10, sqlc OK, build OK)
- [x] T3 Ledger idempotent selesai (RecordAdminStockDeposit + 8 tests)
- [x] T4 Asset summary selesai (3 coin, fail-closed estimate)
- [x] T5 Unified activity selesai (merge 3 sumber + detail)
- [x] T6 Bot deposit flow selesai (pending-row preview+Confirm)
- [x] T7 Bot views/detail selesai (formatter SIMULASI + WIB)
- [x] T8 Full VPS verification selesai (vet+62 tests+build+smoke exit 0)
- [x] T9 Docs + commit/push selesai (PROJECT_IDEAS v0.14, trial manual pending restart workspace)
- [ ] Persetujuan deploy/restart live diperoleh
