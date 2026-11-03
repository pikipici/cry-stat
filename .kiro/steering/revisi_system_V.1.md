# Crypto Station — Revisi Sistem V.1

> Status: ACTIVE ROADMAP — Admin Wallet v0.14/v0.14.1 selesai otomatis; hardening sistem user/topup/buy masih pending
> Audit awal: commit `a16b2a6`
> Progress code terkini: `8c60ce1`; repository/docs sebelum sinkronisasi seluruh Markdown: `a46427b`
> Updated: 2026-11-03
> Fokus: debug sistem existing, integritas transaksi, dan UX sisi user Telegram
> Eksekusi runtime: build/test/migrate/run di `cbt-vps`, bukan lokal
> Source of truth Admin Wallet: `.kiro/steering/ADMIN_WALLET_CURRENT_STATE.md`
> Prioritas berikutnya: trial manual Admin Wallet, lalu fase user/topup/buy sandbox pada Bagian G dokumen ini

---

## 0. Tujuan Dokumen

Dokumen ini disiapkan supaya AI lain bisa memperbaiki sistem dengan biaya konteks lebih rendah tanpa mengaudit ulang seluruh repository.

Target revisi:

1. Menutup bug yang bisa menyebabkan transaksi tetap berjalan setelah dibatalkan.
2. Menutup risiko saldo IDR terdebit tanpa payout atau refund yang durable.
3. Memperbaiki jurnal coin agar benar-benar double-entry.
4. Menghapus sisa flow custodial lama (`ALL/MAX`) yang bertentangan dengan model transit.
5. Menyatukan UX tombol dan slash command.
6. Membuat setiap error, empty state, timeout, dan callback memberi feedback jelas.
7. Membuat Saldo, Riwayat, dan Status mengikuti lifecycle payout/deposit.
8. Memperkuat idempotensi, private-chat guard, audit alamat, dan test coverage.
9. Memulihkan pipeline build/test yang reproducible dari clean checkout.

Dokumen ini tidak mengimplementasikan perubahan. AI executor wajib mengikuti fase dan acceptance criteria, bukan melakukan refactor besar sekaligus.

---

## 1. Sumber Kebenaran dan Batas Produk

Keputusan yang tidak boleh dibuka ulang:

- Model aktif adalah transit/non-custodial arah user.
- Beli: IDR user diproses, coin otomatis dikirim ke wallet pribadi user.
- Jual: user mengirim coin ke alamat deposit sistem, lalu menerima IDR.
- Coin tidak disimpan sebagai saldo user di bot.
- `ALL/MAX` jual sudah dibatalkan.
- Sandbox wajib berlabel `SIMULASI`.
- Stage sekarang belum membaca blockchain asli.
- Payments asli, node asli, private key, watcher chain, dan saldo on-chain bukan scope revisi ini.
- Semua mutasi uang harus melalui jurnal double-entry.
- Semua callback dan message transaksi hanya boleh berjalan di private chat.
- Admin harus tetap allowlist-only.

Referensi utama:

- `PROJECT_IDEAS.md` untuk keputusan produk/transit dan gate Stage 2.
- `.kiro/steering/ADMIN_WALLET_CURRENT_STATE.md` untuk kondisi Admin Wallet terkini.
- Dokumen ini untuk roadmap hardening sistem user, topup, buy/payout, ledger, dan UX.

Jika ada kontradiksi antara kode lama dan model transit di `PROJECT_IDEAS.md`, model transit adalah pemenangnya. Untuk Admin Wallet, kondisi commit `8c60ce1` dan `ADMIN_WALLET_CURRENT_STATE.md` mengalahkan plan/report lama yang sudah dihapus.

### 1.1 Instruksi wajib untuk AI executor sebelum menyentuh kode

Dokumen ini berasal dari audit awal commit `a16b2a6`, sedangkan baseline kode yang sudah diverifikasi sekarang adalah `8c60ce1`. Karena itu, checkbox pending bukan bukti otomatis bahwa implementasinya belum ada. AI executor **wajib melakukan reconciliation pass terhadap code head aktual** sebelum menulis test atau kode.

Urutan preflight yang tidak boleh dilewati:

1. Baca berurutan:
   - `.kiro/steering/ADMIN_WALLET_CURRENT_STATE.md`;
   - bagian ini dan `BAGIAN I — CHECKPOINT PROGRESS EXECUTOR`;
   - bagian Pivot, Locked Decisions, Ledger, dan Current Next Step di `PROJECT_IDEAS.md`;
   - `TRIAL.md` untuk acceptance manual yang masih pending.
2. Jalankan `git status --short --branch` dan `git log --oneline -20`. Jangan menghapus, menimpa, atau me-reset perubahan user yang belum di-commit.
3. Catat baseline aktual: repository HEAD, code head relevan, migration DB workspace, automated verification, manual trial, workspace restart, dan live deployment sebagai status yang terpisah.
4. Untuk setiap temuan/task yang akan dikerjakan, klasifikasikan dulu:
   - `DONE` — sudah ada dan teruji pada code head sekarang;
   - `MASIH VALID` — gap masih nyata dan aman dieksekusi;
   - `BERUBAH` — masalah masih ada tetapi kontrak/file/solusinya sudah bergeser;
   - `DROP` — tidak lagi berlaku akibat pivot transit atau implementasi baru.
5. Verifikasi klasifikasi dengan membaca kode, query, migration, dan test terkait. Jangan percaya commit message atau checkbox plan saja.
6. Tulis hasil reconciliation singkat ke checkpoint dokumen ini sebelum implementasi fase pertama. Jangan audit ulang seluruh repository jika bukti yang dibutuhkan sudah tersedia.

Aturan eksekusi:

- Jangan menjalankan seluruh roadmap secara borongan. Kerjakan satu fase kecil pada satu waktu, mulai dari **Phase 2 — cancel/callback safety user**, lalu Phase 3 ledger invariant, kecuali reconciliation membuktikan urutan dependensinya berubah.
- Gunakan TDD untuk bug/invariant: test gagal yang mereproduksi gap → implementasi minimum → targeted test → full verification.
- Runtime, generate, migration, test, build, dan smoke dijalankan di `cbt-vps`; lokal hanya coding + Git.
- Sebelum migration atau operasi DB: cek migration terbaru agar nomor tidak bentrok dengan v10, gunakan perubahan forward/additive, backup bila operasi berisiko, dan jangan memakai DB live.
- Jangan mengubah handler langsung untuk memanipulasi saldo. Semua mutasi uang/coin harus melalui application service + jurnal double-entry dan idempotency boundary.
- Pertahankan model transit: tidak ada saldo coin user, tidak ada `ALL/MAX`, beli menghasilkan payout, dan jual berasal dari deposit user.
- Pertahankan label `SIMULASI`; mock tidak boleh dipresentasikan sebagai QRIS, transfer bank, txhash, atau saldo on-chain nyata.
- Jangan memasukkan seed/private key/credential ke source, test fixture, log, audit, dokumentasi, atau chat.
- Jangan deploy/restart live, mengaktifkan gateway/chain asli, atau memasukkan private key tanpa approval eksplisit user dan gate Stage 2 lengkap.

Quality gate per fase:

1. Targeted tests untuk package yang disentuh hijau.
2. `go vet ./...`, `go test -count=1 ./...`, build bot, dan `bash scripts/verify.sh --no-db` hijau di VPS.
3. Jika schema berubah, jalankan full verification hanya pada DB workspace/test dengan opt-in yang diwajibkan script.
4. Lakukan manual smoke untuk flow Telegram yang berubah; automated test tidak boleh diklaim sebagai manual trial.
5. Commit satu scope logis, push ke GitHub + bare VPS sesuai workflow, lalu verifikasi workspace berada pada commit tersebut.
6. Update checkpoint di dokumen ini: SHA, test evidence, migration, manual trial, workspace status, dan next task.
7. Berhenti dan laporkan jika acceptance gagal, kontrak plan bertentangan dengan kode aktual, atau dibutuhkan keputusan produk. Jangan menebak.

Definition of handoff yang jujur:

- `implemented` ≠ `automated verified`;
- `automated verified` ≠ `manual Telegram trial passed`;
- `workspace restarted/deployed` ≠ `live deployed`;
- `live deployed` hanya boleh ditulis setelah approval eksplisit dan verifikasi live.

### 1.2 Urutan eksekusi yang direkomendasikan

1. Reconcile task plan terhadap code head aktual dan tandai statusnya.
2. Tutup acceptance Admin Wallet yang masih pending: admin trial, non-admin denial/audit, restart recovery, dan restore drill bila maintenance window disetujui.
3. Phase 2 — cancel semantics + callback/private-chat safety user.
4. Phase 3 — ledger invariant buy/deposit per asset.
5. Phase 4 — durable payout lifecycle, refund-once, unknown no-refund, dan restart recovery.
6. Phase 5–7 — deposit/address safety, hapus drift custodial, lalu Saldo/Status/Riwayat lifecycle.
7. Sandbox S1–S3 — Top Up Simulasi production-shaped.
8. Sandbox S4–S6 — durable buy/payout, unified read models, dan E2E restart trial.
9. Phase 9 performance/DB hardening hanya setelah correctness path stabil dan terukur.

Jika suatu fase ternyata sudah sebagian dikerjakan, jangan mengulang implementasi. Pecah hanya gap tersisa menjadi task kecil dan pertahankan test/behavior yang sudah benar.

---

## 2. Metode dan Batas Audit

Yang diperiksa:

- `internal/bot/bot.go`
- `internal/bot/flow.go`
- `internal/bot/keyboard.go`
- `internal/bot/adminflow.go`
- `internal/bot/session.go`
- `internal/ledger/ledger.go`
- `internal/ledger/trade.go`
- `internal/ledger/deposit.go`
- `internal/chain/chain.go`
- `internal/admin/admin.go`
- `db/queries/*.sql`
- `migrations/000001` sampai `000009`
- test ledger, chain, admin, bot, risk, dan trial yang tersedia

Verifikasi lokal aman:

- Git head: `a16b2a6`.
- Working tree saat audit: `.kiro/` untracked karena berisi dokumen plan.
- `go test ./...` tidak dapat mencapai test execution dari clean local checkout karena package generated `internal/db` tidak tersedia.
- `sqlc.yaml` juga menunjuk `db/schema`, sementara directory schema tidak tersedia di checkout.
- Tidak ada migration, generate, deploy, restart, atau perubahan DB yang dilakukan.

Kesimpulan verifikasi:

- Temuan dalam dokumen ini berasal dari trace kode, SQL, migration, dan kontrak existing.
- Test runtime penuh harus dilakukan di VPS setelah prerequisite sqlc dipulihkan.
- Jangan mengklaim seluruh suite hijau hanya berdasarkan audit statis.

---

## 3. Ringkasan Eksekutif

### P0 — blocker integritas dan side effect

1. `/batal` dan tombol Batal tidak benar-benar membatalkan quote aktif.
2. Trade buy commit lebih dulu; payout dilakukan sesudahnya tanpa queued work/refund atomik.
3. Jurnal buy coin tidak balance per asset.
4. Jurnal deposit coin tidak balance per asset.
5. Callback stok admin memiliki format builder/parser berbeda sehingga tombol mati.
6. Teks `❌ Batal` pada broadcast admin bisa terkirim ke seluruh user.
7. Build/test clean checkout tidak reproducible karena contract/generated sqlc hilang.

### P1 — bug user flow dan security boundary

8. Cancel picker/admin dapat meng-expire quote yang tidak berkaitan.
9. `/riwayat` masih model trade lama, bukan payout/deposit transit.
10. Flow `ALL/MAX` dan sell-wallet lama masih dapat dijangkau.
11. `/jual SOL` berbeda dari tombol Jual → SOL.
12. Callback tidak menegakkan private-chat-only secara konsisten.
13. Callback blocked/unregistered/stale dapat gagal diam-diam.
14. Deposit idempotency tidak memverifikasi owner/asset/units saat tx hash conflict.
15. Quote lookup membaca data sebelum ownership diverifikasi.
16. `/start` tidak memperbaiki user existing yang belum mendapat grant.
17. Audit pergantian alamat selalu menganggap alamat lama kosong.
18. Validasi alamat SOL bukan base58 yang benar.

### P2 — UX dan maintainability

19. Quote beli tidak memperingatkan risiko salah alamat dengan cukup jelas.
20. Status DB error disamarkan sebagai “belum ada transaksi”.
21. Session timeout berubah menjadi “Perintah tidak dikenal”.
22. Simulasi `Rp50rb masuk` tidak berarti kredit neto Rp50.000.
23. `/batal` memberi false confirmation saat tidak ada proses.
24. Saldo memakai heading “Terkirim” untuk status yang belum tentu berhasil.
25. Welcome user lama, istilah, dan menu helper masih stale.
26. Cache wallet melakukan full-ledger scan pada setiap mutasi.
27. Constraint/index/migration/test isolation perlu diperkuat.

---

# BAGIAN A — TEMUAN DEBUG DAN SPESIFIKASI PERBAIKAN

## A1. Cancel quote aktif tidak bekerja

Severity: P0 / Critical

Evidence:

- `db/queries/quotes_trades.sql` — `ExpireUserQuotes` hanya mengubah quote dengan `expires_at <= now()`.
- `internal/bot/bot.go` — `/batal` dan callback quote Batal memanggil query tersebut lalu menyatakan quote dibatalkan.
- `internal/bot/flow.go` — cancel picker juga memakai operasi yang sama.

Root cause:

Nama query dan asumsi caller tidak sama dengan predikat SQL. Quote yang masih valid 30 detik tetap `active` dan masih bisa dikonfirmasi setelah user menekan Batal.

Required change:

Pisahkan tiga operasi:

1. `ExpireElapsedQuotes(userID)` untuk housekeeping quote yang waktunya lewat.
2. `CancelActiveQuotesByUser(userID)` untuk `/batal` kontekstual.
3. `CancelQuoteByID(userID, quoteID)` untuk tombol Batal quote spesifik.

Ketentuan:

- Gunakan predicate `status='active'`.
- Quote ID wajib owner-scoped.
- Return affected row/count supaya copy tidak berbohong.
- Tambahkan status `cancelled` bila perubahan schema diterima; jika tetap memakai `expired`, dokumentasikan bahwa status itu mencampur timeout dan cancel manual. Rekomendasi: status terpisah `cancelled`.

Acceptance criteria:

- Quote future-expiry tidak bisa dieksekusi setelah Batal.
- Confirm callback lama mengembalikan “Quote sudah dibatalkan”.
- User A tidak bisa membatalkan quote user B.
- Menutup picker alamat/admin tidak membatalkan quote trade.
- Tidak ada false success ketika tidak ada quote aktif.

Tests:

- `TestCancelFutureQuoteBlocksExecute`
- `TestCancelQuoteOwnerScoped`
- `TestCancelNoActiveQuoteReturnsZero`
- `TestPickerCancelKeepsActiveQuote`
- `TestAdminCloseKeepsUserQuote`

---

## A2. Trade dan payout tidak memiliki durable state machine atomik

Severity: P0 / Financial integrity

Evidence:

- `internal/ledger/trade.go` — `ExecuteTrade` mengonsumsi quote, menulis jurnal/trade, lalu commit.
- `internal/bot/bot.go` — `Chain.Payout` dipanggil sesudah commit.
- Error payout hanya menghasilkan pesan “IDR tidak hilang”, tanpa refund journal.
- Crash setelah trade commit tetapi sebelum payout meninggalkan kewajiban tanpa queued work yang durable.

Root cause:

Boundary transaction berhenti pada trade, sedangkan payout dianggap side effect langsung dari handler.

Required architecture:

Dalam satu transaksi DB:

1. Lock user/account/wallet sesuai urutan proyek.
2. Consume quote secara owner-scoped.
3. Tulis jurnal trade.
4. Buat trade row.
5. Buat payout row `queued` dengan unique `trade_id`.
6. Commit.

Sesudah commit:

- Worker/service memproses payout queued secara idempotent.
- `queued → broadcasting → confirmed` untuk sukses.
- Kegagalan final: `failed` + refund journal tepat sekali.
- Timeout/ambigu: `unknown`, jangan refund otomatis.
- Restart harus dapat melanjutkan payout queued.

Jangan lakukan network/chain call di dalam DB transaction.

Acceptance criteria:

- Tidak ada trade buy tanpa payout row.
- Crash setelah commit tidak menghilangkan kewajiban payout.
- Retry tidak membuat payout/txhash/refund ganda.
- `failed` menghasilkan satu refund.
- `unknown` tidak menghasilkan refund.
- Saldo, Status, dan Riwayat menampilkan lifecycle aktual.

Tests:

- failure injection sebelum/selepas commit;
- process restart simulation;
- duplicate worker claim;
- confirmed retry;
- failed refund once;
- unknown no-refund;
- queued payout visible to user.

---

## A3. Jurnal buy coin tidak balance

Severity: P0 / Ledger invariant

Evidence:

Current logic di `internal/ledger/trade.go`:

- `house:float = -sentUnits`
- `house:fee-collect = +feeUnits`
- `sentUnits = units - feeUnits`

Total coin bukan nol.

Root cause:

Tidak ada external payout/chain-clearing account dan gross/net/fee direction tidak dimodelkan konsisten.

Required decision:

Gunakan template konseptual berikut untuk buy:

```text
house:float       -gross_units
external:payout   +sent_units
house:fee-clearing +fee_units
--------------------------------
sum asset coin = 0
```

Nama account boleh disesuaikan, tetapi makna wajib jelas:

- `external:payout` mewakili coin yang keluar dari boundary ledger menuju user.
- `fee-clearing` mewakili bagian gross yang dialokasikan untuk network fee, bukan revenue.
- Jangan menyebut fee sebagai laba.

Enforcement:

- Sebelum commit, validasi sum entries per asset = 0.
- Pertimbangkan DB-level protection pada fase hardening.
- Reconcile harus mencakup akun house, bukan hanya `user:*`.

Acceptance criteria:

- Fee nol dan nonnol sama-sama balance.
- Precision 18 decimal tidak menyisakan dust.
- Corrupt/missing leg membuat transaction rollback.
- Float movement sesuai definisi gross yang disepakati.

---

## A4. Jurnal deposit/jual coin tidak balance

Severity: P0 / Ledger invariant

Evidence:

`internal/ledger/deposit.go` menulis `house:float +units`, tetapi tidak memiliki kaki coin negatif. IDR balance, coin tidak.

Required template:

```text
external:deposit  -units_in
house:float       +units_in
house:cash IDR    -credited_idr
user:<id> IDR     +credited_idr
```

`external:deposit` adalah boundary account, bukan saldo user internal.

Acceptance criteria:

- Coin dan IDR masing-masing sum nol.
- Retry event identik tidak membuat jurnal kedua.
- Tx conflict payload berbeda tidak dianggap retry sukses.
- Reconcile dapat mendeteksi satu kaki yang hilang.

Catatan dokumentasi:

`PROJECT_IDEAS.md` juga perlu diperbarui setelah implementasi karena template deposit saat ini tidak menampilkan external leg walaupun prinsipnya mewajibkan sum nol.

---

## A5. Tombol stok admin mati karena callback contract drift

Severity: P0 / Existing feature broken

Evidence:

- `internal/bot/keyboard.go` menghasilkan `a:stockfloat:SOL`, `a:stockfee:ETH`, dan seterusnya — tiga segmen.
- `internal/bot/adminflow.go` hanya menangani aksi tersebut pada `len(parts)==4` dan membaca coin dari `parts[3]`.

Root cause:

Builder dan parser tidak memakai satu callback contract.

Required change:

Pilih satu format. Rekomendasi:

```text
a:stock:float:SOL
a:stock:fee:ETH
a:stock:hot:BNB
a:stock:topup:SOL
```

Buat parser helper bertipe, jangan menyebarkan indeks `parts` ke banyak switch.

Acceptance criteria:

- Seluruh 4 aksi × 3 coin mencapai prompt benar.
- Invalid/stale callback memberi feedback, tidak diam.
- Test table-driven mengunci builder-parser compatibility.

Catatan:

Perbaikan ini harus sinkron dengan `ADMIN_WALLET_OPERATIONS_PLAN.md`; jangan memperbaiki callback lama lalu membuat format ketiga pada fitur baru.

---

## A6. Cancel broadcast dapat mengirim “❌ Batal” ke semua user

Severity: P0 / Mass side effect

Evidence:

- Prompt menyuruh mengetik `❌ Batal`.
- `MatchButton` hanya mengenali `batal` atau `/batal`.
- `StepAdminBroadcast` mengirim semua teks langsung melalui `doBroadcast`.

Required UX/state machine:

1. Admin tekan Broadcast.
2. Admin ketik pesan.
3. Bot menampilkan preview.
4. Tombol `✅ Kirim ke N user` dan `❌ Batal`.
5. Hanya callback confirm yang melakukan broadcast.

Cancel aliases yang wajib dikenali:

- `Batal`
- `❌ Batal`
- `/batal`
- callback cancel
- tombol back admin

Acceptance criteria:

- Seluruh cancel path menghasilkan nol kiriman target.
- Preview tidak mengirim apa pun.
- Double confirm tidak broadcast dua kali; gunakan operation token/idempotency record atau disable durable state yang aman.
- Audit menyimpan jumlah target/sukses/gagal, bukan isi secret.

---

## A7. Build/test clean checkout tidak reproducible

Severity: P0 / Engineering blocker

Evidence:

- `sqlc.yaml` menunjuk `./db/schema` dan output `./internal/db`.
- Keduanya tidak tersedia pada clean checkout saat audit.
- Package application mengimpor `internal/db`, sehingga `go test ./...` gagal saat setup.

Required decision:

Tetapkan pipeline deterministic. Pilihan rekomendasi:

1. Source schema dibentuk dari migration secara deterministic ke `db/schema/schema.sql`.
2. Jalankan `sqlc generate` sebelum build/test di CI/VPS.
3. Tambahkan script/Make target tunggal:
   - generate schema;
   - sqlc generate;
   - gofmt;
   - go test;
   - go vet;
   - check generated drift.
4. Pin versi sqlc/goose.
5. Dokumentasikan apakah generated `internal/db` di-commit atau tidak.

Karena keputusan project sekarang menyatakan generated tidak di-commit, minimum requirement:

- Script bootstrap dari clean clone harus menghasilkan package dengan satu command.
- CI/VPS tidak boleh bergantung pada file generated sisa run lama.

Acceptance criteria:

- Clean clone + documented prerequisite menghasilkan build hijau.
- `go test ./...` dan `go vet ./...` dapat berjalan tanpa langkah rahasia.
- Schema hasil migration dan schema sqlc tidak drift.

---

## A8. Cancel navigation memiliki side effect global ke quote

Severity: P1

Evidence:

- `t:cancel` membersihkan session dan memanggil expire quotes.
- Admin close juga meng-expire quote user.

Required change:

Pisahkan intent:

- Navigation cancel: clear current session/view only.
- Quote cancel: quote-specific operation.
- Admin panel close: admin session only.

Acceptance criteria:

- Menutup Alamat, Jual picker, atau Admin tidak memengaruhi quote Beli aktif.
- User mendapat copy kontekstual: `Proses dibatalkan`, bukan selalu `Quote dibatalkan`.

---

## A9. Riwayat masih memakai model custodial/trade lama

Severity: P1 / Contract drift

Evidence:

- `/riwayat` mengambil `RecentTradesByUser`.
- Tidak menampilkan payout/deposit lifecycle, txhash, confirms, atau status.

Required user timeline:

Gabungkan:

- payout beli user;
- deposit/jual user.

Urut total terbaru, limit total 10. Jangan mengambil 10 per tabel lalu menampilkan 20 tanpa limit gabungan.

Entry payout:

- waktu WIB;
- Beli;
- nominal IDR;
- gross/fee/net coin;
- status translated;
- txhash pendek;
- tombol Detail/Status.

Entry deposit:

- waktu WIB;
- Jual/deposit;
- units masuk;
- fee bila ada;
- IDR credited;
- confirms/required;
- status;
- txhash pendek.

Acceptance criteria:

- Trade tanpa payout tidak ditampilkan sebagai terkirim.
- Payout queued/failed/unknown terlihat.
- Deposit pending/manual/credited terlihat.
- Empty/error state dibedakan.

---

## A10. Sisa ALL/MAX dan sell-wallet harus dihapus dari surface user

Severity: P1

Evidence:

- Parser masih menerima `ALL`/`MAX`.
- `sellAllFlow`, `SellAmountPicker`, callback `t:sellall:*`, dan ledger `CreateQuoteUnits` masih ada.
- Model transit menyatakan ALL/MAX dibatalkan.

Required change:

- Hapus callback publik sellall.
- Hapus keyboard `Jual SEMUA`.
- `/jual SOL ALL` tidak boleh membuat quote.
- Isolasi/hapus service legacy setelah memastikan tidak dipakai migration/admin internal.
- Hapus skipped tests lama setelah coverage flow deposit pengganti tersedia.

Acceptance criteria:

- Satu-satunya jalur Jual user adalah deposit coin.
- Search repository tidak menemukan user-facing copy ALL/MAX.
- Forged callback `t:sellall:SOL` ditolak dengan pesan stale/unsupported.

---

## A11. Slash `/jual` dan tombol Jual tidak konsisten

Severity: P1 / UX

Current behavior:

- Tombol Jual → picker → alamat deposit + simulasi.
- `/jual SOL` mengabaikan coin dan menyuruh user menekan tombol Jual lagi.

Expected behavior:

- `/jual` → coin picker.
- `/jual SOL` → layar deposit SOL yang sama dengan tombol.
- `/jual XYZ` → validasi coin + pilihan valid.
- `/jual SOL ALL` → jelaskan ALL tidak berlaku dan tawarkan flow deposit SOL.

Implementation rule:

Buat satu function/service presenter `showSellDeposit(asset)` yang dipakai tombol dan slash agar copy tidak drift.

---

## A12. Callback private-chat guard belum menyeluruh

Severity: P1 / Security boundary

Evidence:

- Message diperiksa private chat.
- Callback diproses tanpa guard chat type yang setara.

Required change:

Pasang middleware/guard bersama sebelum dispatch seluruh callback:

- private chat only;
- callback message nil-safe;
- blocked/user existence;
- rate limit;
- ownership pada resource.

Acceptance criteria:

- Callback group/supergroup/channel tidak melakukan mutasi.
- Inline/no-message callback tidak panic.
- User lain tidak dapat mengeksekusi quote yang bukan miliknya.
- Callback selalu di-answer agar spinner Telegram berhenti.

---

## A13. Callback dapat gagal diam-diam

Severity: P1 / UX reliability

Cases:

- User belum `/start`.
- User blocked.
- Callback lama/stale.
- Callback malformed.
- Session sudah expired.

Expected copy:

- Belum terdaftar: `Mulai dulu dengan /start.`
- Blocked: `Akun diblokir. Hubungi admin.`
- Stale: `Tombol ini sudah kedaluwarsa. Mulai ulang proses.`
- Malformed: generic safe error, internal log detail.

Rule:

Tidak boleh ada valid user-facing callback branch yang sekadar `return` tanpa `AnswerCallbackQuery` atau edit/send feedback.

---

## A14. Idempotency deposit tidak memvalidasi immutable payload

Severity: P1

Evidence:

Saat `tx_hash` sudah ada, current code mengembalikan record lama tanpa memastikan user, asset, dan units sama.

Required change:

Idempotent success hanya jika seluruh identity sama:

- network/chain;
- tx hash;
- event/log index jika nanti real chain;
- asset;
- units;
- attributed user.

Mismatch harus:

- return typed conflict;
- status/manual review bila real watcher;
- tidak memberi kredit;
- tidak membocorkan detail deposit user lain.

Tests:

- identical retry success once;
- same hash different user conflict;
- same hash different asset conflict;
- same hash different units conflict;
- two-user race.

---

## A15. Quote lookup dilakukan sebelum ownership check

Severity: P1

Required change:

- Query quote menggunakan `(quote_id, user_id)`.
- Risk check hanya dilakukan setelah ownership valid.
- Tidak ada oracle yang membedakan quote user lain dari quote tidak ada.

Acceptance criteria:

Forged qid user lain menghasilkan response generik dan tidak memanggil execution/risk berdasarkan data milik user lain.

---

## A16. `/start` tidak menjamin onboarding lengkap

Severity: P1

Evidence:

Grant sandbox hanya dipanggil saat user baru dibuat. User existing tanpa grant tidak diperbaiki, walaupun `GrantSandbox` sudah idempotent.

Required change:

Buat operation `EnsureSandboxOnboarding`:

1. Ensure user exists.
2. Ensure grant journal exists exactly once.
3. Return actual grant amount/status.
4. Render message dari actual setting/result.

Acceptance criteria:

- Retry setelah grant failure memperbaiki akun.
- Dua `/start` paralel menghasilkan satu user dan satu grant.
- User yang menghabiskan saldo tidak mendapat grant kedua.
- Pesan tidak hardcode Rp1.000.000; memakai setting aktual.

---

## A17. Audit ganti alamat kehilangan nilai sebelumnya

Severity: P1 / Audit integrity

Required change:

Pindahkan replace-address ke service transaction:

1. Read/lock current address.
2. Validate new address.
3. Upsert/replace.
4. Audit actual before/after.
5. Commit atomik.

Acceptance criteria:

- First set: before empty.
- Replace: before A, after B.
- Failure tidak menulis audit sukses.
- Parallel replace menghasilkan ordering yang dapat dijelaskan.

---

## A18. Validator SOL bukan base58 valid

Severity: P1 / Pre-chain safety

Current validation menerima seluruh alphanumeric, termasuk karakter yang dilarang base58.

Required change:

- Gunakan library base58 teruji.
- Decode dan validasi panjang public key Solana.
- Tolak `0`, `O`, `I`, `l`.
- Fuzz test untuk no panic.

Jangan menambahkan network call hanya untuk validasi format pada revisi ini.

---

## A19. Quote beli kurang aman dan kurang mudah dipahami

Severity: P2 / User safety

Current deficiencies:

- Alamat hanya dipendekkan.
- Tidak ada warning irreversibilitas yang tegas.
- Fee lebih banyak tampil sebagai units coin, bukan ekuivalen rupiah.
- Tidak ada gross/net breakdown yang konsisten.

Required quote format:

```text
Konfirmasi Pembelian — SIMULASI

Bayar: Rp50.000
Dapat sebelum biaya: 0.00... SOL
Biaya jaringan: 0.00... SOL (±Rp...)
Diterima: 0.00... SOL
Network: Solana
Alamat tujuan:
<alamat penuh yang mudah disalin>
Harga berlaku: 30 detik

⚠️ Pastikan alamat dan network benar. Pengiriman crypto tidak dapat dibatalkan.

[✅ Konfirmasi] [✏️ Ganti Alamat]
[❌ Batal]
```

Acceptance criteria:

- Warning selalu ada pada setiap quote.
- Address penuh tersedia sebelum confirm.
- Fee coin, estimasi IDR, gross, net, dan total debit konsisten.
- Harga stale tidak ditampilkan sebagai quote aktif.
- Golden/snapshot test copy untuk BNB/ETH/SOL.

---

## A20. Status menyamarkan DB error sebagai empty state

Severity: P2 / Trust

Required state handling:

- Kedua query sukses + kosong → `Belum ada transaksi.`
- Payout gagal, deposit sukses → tampilkan deposit + warning hasil parsial.
- Deposit gagal, payout sukses → tampilkan payout + warning hasil parsial.
- Keduanya gagal → `Status sedang tidak tersedia. Coba lagi.`

Internal error harus dilog dengan correlation/context, tetapi detail SQL tidak boleh tampil ke user.

---

## A21. Session timeout tidak punya feedback kontekstual

Severity: P2 / UX

Current behavior:

Session dihapus setelah timeout; input berikutnya menjadi “Perintah tidak dikenal”.

Required change:

Session API harus membedakan:

- missing;
- active;
- expired.

Copy:

- Buy: `Sesi beli sudah habis. Tekan Beli untuk mulai lagi.`
- Address: `Sesi alamat sudah habis. Buka Menu → Alamat.`
- Admin: `Sesi admin sudah habis. Buka Admin lagi.`

Tests harus memakai injectable clock, bukan sleep.

---

## A22. Copy simulasi jual Rp50rb menyesatkan

Severity: P2 / UX-number contract

Current behavior:

Button `Rp50rb masuk` mengubah Rp50rb ke units berdasarkan bid, lalu fee coin dipotong. Kredit aktual lebih kecil dari label.

Pilih satu semantics dan kunci:

Rekomendasi biaya rendah:

- Label: `Simulasi kirim senilai Rp50rb`.
- Preview sebelum mutate:
  - gross units;
  - fee;
  - net units;
  - bid;
  - estimasi IDR diterima.
- Confirm baru membuat mock deposit.

Jangan menyebut `Rp50rb masuk` kecuali kredit neto benar-benar Rp50.000.

---

## A23. `/batal` harus kontekstual

Severity: P2

Expected behavior:

- Session input aktif → `Proses dibatalkan.`
- Quote aktif → `Quote dibatalkan.`
- Keduanya aktif → sebutkan keduanya atau prioritaskan layar aktif dengan jelas.
- Tidak ada proses → `Tidak ada proses aktif.`

Return count/result dari setiap cancel operation; jangan hardcode success.

---

## A24. Saldo dan status payout perlu bahasa lifecycle

Severity: P2 / User comprehension

Ganti heading `Terkirim` menjadi `Pengiriman terbaru`.

Mapping user-facing:

- `queued` → `Menunggu diproses`
- `broadcasting` → `Sedang dikirim`
- `confirmed` → `Berhasil`
- `failed` → `Gagal — dana dikembalikan` hanya jika refund benar-benar posted
- `unknown` → `Perlu pengecekan — jangan ulang transaksi`

Jangan menyatakan refund sebelum journal refund confirmed.

---

## A25. Welcome, istilah, dan menu stale

Severity: P2

Required cleanup:

- Returning user `/start` harus mendapat home ringkas yang sama dengan model transit.
- Jelaskan: `Beli dikirim ke wallet pribadi. Jual dengan mengirim coin ke alamat deposit.`
- `MainMenu()` lama dan `replyMenu` stale dihapus atau diarahkan ke satu source of truth.
- Batal tetap contextual, bukan tombol permanen.
- Standardisasi istilah user:
  - Confirm → Konfirmasi
  - units → jumlah coin
  - tx → hash transaksi
  - harga lock → harga berlaku
  - fee → biaya jaringan
- Pilih tone produk konsisten. Rekomendasi user-facing: `kamu`; admin internal boleh lebih teknis.

---

## A26. Cache wallet melakukan full-ledger scan

Severity: P2 / Performance

Evidence:

`refreshWalletCache` memakai aggregate seluruh owner/asset dan dapat dipanggil dua kali per trade.

Required change:

Tahap aman:

1. Tambah query targeted `WHERE owner=$1 AND asset=$2`.
2. Refresh hanya wallet yang berubah.
3. Setelah correctness stabil, pertimbangkan delta update pada row yang sudah di-lock.
4. Reconcile periodik tetap menghitung dari ledger sebagai source of truth.

Acceptance criteria:

- Hasil balance identik dengan full aggregation.
- Query plan memakai index account/entry.
- Tidak ada O(total ledger) per trade.
- Benchmark/EXPLAIN dilakukan di VPS staging dengan data representatif.

---

## A27. Database constraints, migration, dan test harness perlu hardening

Severity: P2

Required upgrades:

### Ledger constraints

- Pastikan entry asset cocok dengan ledger account asset.
- Pertimbangkan FK komposit atau hilangkan kolom asset redundan.
- Enforce `units > 0`, `fee >= 0`, `sent_units = units - fee_units`, confirms nonnegative.
- Tambah idempotency key jurnal untuk grant/refund/event penting.

### Quote concurrency

- Cegah dua active quote per user dengan partial unique index atau user/advisory lock.
- Jangan mengabaikan error expire/replace quote.

### Payout/outbox

- Gunakan insert-on-conflict, bukan read-then-insert.
- Event/outbox dibuat atomik dengan payout.
- Jangan membuang error insert chain event.

### Migration quality

- Kurangi `IF NOT EXISTS` yang dapat menyembunyikan schema drift.
- Tambahkan migration verification dari empty DB dan upgrade snapshot.
- Jika Down tidak disediakan, tandai migration irreversible dengan jelas.

### Test isolation

- Gunakan DB/schema ephemeral per run.
- Jangan memakai DB shared tanpa cleanup.
- Pisahkan unit/integration/migration/concurrency tests.
- Semua error return dalam test wajib diperiksa.

---

# BAGIAN B — REDESIGN UX USER YANG DIREKOMENDASIKAN

## B1. Struktur menu

Menu utama tetap sederhana:

```text
[Beli] [Jual]
[Harga] [Menu]
```

Submenu akun:

```text
[Saldo] [Riwayat]
[Status] [Alamat]
[Kembali]
```

Principles:

- Maksimal empat primary choices di home.
- Progressive disclosure untuk mengurangi beban pilihan.
- Aksi Batal hanya muncul saat ada proses yang bisa dibatalkan.
- Slash command tetap fallback dan harus menghasilkan flow yang sama.

## B2. Beli

Target flow:

1. Beli.
2. Pilih coin.
3. Jika alamat belum ada: langsung tawarkan Tambah Alamat.
4. Input nominal IDR dengan quick choices opsional: 25rb, 50rb, 100rb, nominal lain.
5. Preview lengkap dan aman.
6. Konfirmasi.
7. Status pengiriman jelas.

Required UX details:

- Selalu tampilkan network.
- Alamat penuh sebelum confirm.
- Gross, fee, net diterima.
- Waktu quote tersisa.
- Tombol Ganti Alamat.
- Error domain punya next action.

Error mapping:

- saldo kurang → tampilkan saldo tersedia;
- nominal terlalu kecil → tampilkan minimum/saran;
- fee >20% → sarankan nominal lebih besar;
- harga stale → tombol Coba Lagi;
- float tipis → `Pembelian sementara tidak tersedia`, bukan istilah exposure;
- alamat belum ada → tombol Tambah Alamat.

## B3. Jual/deposit

Target flow:

1. Jual.
2. Pilih coin.
3. Tampilkan alamat deposit, network, confirmation requirement, dan warning.
4. Sandbox: pilih simulasi nominal → preview net → Confirm.
5. Real chain nanti: watcher, bukan tombol “sudah kirim” yang langsung mengkredit.

Copy wajib:

- `Kirim hanya SOL melalui network Solana.`
- `Salah network dapat menyebabkan aset hilang.`
- `IDR masuk setelah konfirmasi cukup.`

## B4. Saldo

Target output:

- Saldo IDR paling atas.
- Pengiriman terbaru maksimal 3.
- Status translated.
- Tombol `Lihat Riwayat` dan `Cek Status`.
- Tidak ada valuasi portfolio coin.

## B5. Riwayat

Timeline gabungan payout/deposit. Fokus pada apa yang user butuhkan:

- Apa transaksinya?
- Berapa nilainya?
- Kapan?
- Sudah selesai atau belum?
- Hash/reference apa?
- Apa yang harus dilakukan jika bermasalah?

## B6. Status

Status adalah layar operasional untuk transaksi belum final:

- queued/broadcasting/unknown payout;
- pending/manual deposit;
- confirmed/credited recent bila perlu.

Jangan duplikasi seluruh Riwayat. Status memprioritaskan item yang butuh perhatian.

## B7. Alamat

Target flow:

- List tiga coin + network + alamat pendek/status kosong.
- Detail menampilkan alamat penuh.
- Ganti alamat memakai preview old → new.
- Warning sebelum replace.
- Audit actual before/after.
- Stage 2 nanti PIN + cooling-off, bukan sekarang.

## B8. Empty, loading, partial, error states

Setiap layar harus memiliki empat state berbeda:

1. Empty: query sukses, data kosong.
2. Success: data lengkap.
3. Partial: sebagian sumber gagal.
4. Error: tidak dapat memuat.

Dilarang mengubah DB error menjadi empty state.

---

# BAGIAN C — URUTAN IMPLEMENTASI AI EXECUTOR

## C0. Aturan eksekusi

- Jangan implementasikan semua temuan dalam satu commit.
- Satu invariant/root cause per commit atau kelompok yang sangat terkait.
- Test regression ditulis lebih dulu untuk bug P0/P1.
- Coding lokal, runtime/test/migration di VPS.
- Jangan deploy/restart live tanpa instruksi eksplisit user.
- Admin Wallet sudah diimplementasikan dan di-hardening terpisah; jangan buka ulang scope tersebut kecuali mengikuti backlog di `ADMIN_WALLET_CURRENT_STATE.md`.

## Phase 1 — Pulihkan verification pipeline

Files:

- `sqlc.yaml`
- script generate/build baru bila diperlukan
- dokumentasi bootstrap

Steps:

1. Verifikasi clean clone state di VPS workspace.
2. Tetapkan source schema sqlc.
3. Generate `internal/db` deterministically.
4. Jalankan baseline build/test/vet.
5. Catat baseline failures tanpa memperbaiki secara acak.

Gate:

- Clean generate + build berjalan.
- Baseline test dapat dieksekusi.

## Phase 2 — Cancel semantics dan callback safety

Scope:

- A1, A8, A12, A13, A15, A23.

Steps:

1. Tambah owner-scoped cancel queries.
2. Pisahkan navigation cancel dari quote cancel.
3. Tambah callback middleware private-chat/user-state.
4. Tambah stale/malformed feedback.
5. Perbaiki `/batal` copy berdasarkan affected state.

Gate:

- Quote tidak bisa dieksekusi setelah cancel.
- Cancel picker/admin tidak mengubah quote.
- Callback non-private tidak memutasi data.

## Phase 3 — Ledger accounting invariant

Scope:

- A3, A4, bagian ledger A27.

Steps:

1. Dokumentasikan account taxonomy.
2. Tulis failing journal-sum tests.
3. Perbaiki buy template.
4. Perbaiki deposit template.
5. Tambah validation sum per asset sebelum commit.
6. Perluas reconcile ke account house/mock chain boundary.

Gate:

- Semua jurnal baru balance per asset.
- Corrupt leg terdeteksi.
- Existing wallet cache tetap sesuai.

## Phase 4 — Durable payout lifecycle

Scope:

- A2 + payout/outbox A27.

Steps:

1. Create payout queued atomik dengan trade.
2. Buat processor idempotent.
3. Implement failed refund once.
4. Implement unknown hold/no-refund.
5. Recovery queued setelah restart.
6. Update Saldo/Status menggunakan lifecycle.

Gate:

- Tidak ada buy trade tanpa payout row.
- Failure injection tidak kehilangan saldo.

## Phase 5 — Deposit idempotency dan address safety

Scope:

- A14, A17, A18.

Steps:

1. Typed conflict untuk tx hash mismatch.
2. Strengthen identity fields.
3. Replace-address transactional audit.
4. Base58 decode validation.

Gate:

- Retry sama exactly-once.
- Conflict beda owner/payload ditolak.
- Audit alamat akurat.

## Phase 6 — Hapus contract drift custodial

Scope:

- A9, A10, A11.

Steps:

1. Tutup forged callback sellall.
2. Hapus UI/copy ALL/MAX.
3. Satukan `/jual` dan button flow.
4. Ubah Riwayat ke payout/deposit timeline.
5. Hapus dead/skipped custodial tests setelah replacement coverage hijau.

Gate:

- Tidak ada user path ke saldo coin internal.
- Riwayat mengikuti transit lifecycle.

## Phase 7 — Onboarding dan UX state

Scope:

- A16, A19–A25.

Steps:

1. Ensure sandbox onboarding idempotent.
2. Redesign quote preview.
3. Error mapping typed.
4. Session expiry context.
5. Status partial/error handling.
6. Saldo lifecycle wording.
7. Standardisasi copy/menu/welcome.

Gate:

- Seluruh user flow utama punya success/empty/partial/error/expired/cancel state.
- Tidak ada raw technical error ke user.

## Phase 8 — Admin existing blockers

Scope:

- A5, A6.

Steps:

1. Satukan callback contract.
2. Test 4 aksi × 3 coin.
3. Broadcast preview/confirm/idempotency.
4. Setelah hijau, baru lanjut admin wallet plan.

## Phase 9 — Performance dan DB hardening

Scope:

- A26, sisa A27.

Steps:

1. Targeted wallet aggregation.
2. Align indexes dengan query.
3. Active quote uniqueness/concurrency.
4. Domain constraints.
5. Migration verification.
6. Ephemeral test DB.
7. Retention plan snapshot/event tables.

Gate:

- Explain plan terverifikasi di VPS.
- Concurrent tests hijau.
- Migration from empty dan prior snapshot hijau.

---

# BAGIAN D — TEST MATRIX WAJIB

## D1. Cancel

- [ ] Active future quote benar-benar cancelled.
- [ ] Confirm lama gagal.
- [ ] Owner scope aman.
- [ ] Cancel picker tidak membatalkan quote.
- [ ] Admin close tidak membatalkan quote.
- [ ] No-op cancel memberi copy yang benar.

## D2. Ledger

- [ ] Buy fee zero balance.
- [ ] Buy fee nonzero balance.
- [ ] Deposit coin+IDR balance.
- [ ] Precision 18 decimal.
- [ ] Missing leg rollback.
- [ ] Account asset mismatch ditolak.
- [ ] Reconcile house corruption gagal.

## D3. Payout

- [ ] Trade dan queued payout atomik.
- [ ] Crash recovery.
- [ ] Double worker exactly-once.
- [ ] Failed refund once.
- [ ] Unknown no-refund.
- [ ] Confirmed no duplicate.

## D4. Deposit

- [ ] Identical retry exactly-once.
- [ ] Same hash different user conflict.
- [ ] Same hash different asset conflict.
- [ ] Same hash different units conflict.
- [ ] Two events same second tetap unik.

## D5. Callback/security

- [ ] Private callback succeeds.
- [ ] Group callback rejected/no mutation.
- [ ] Nil message callback no panic.
- [ ] Blocked feedback.
- [ ] Unregistered feedback.
- [ ] Stale callback feedback.
- [ ] Forged quote ownership safe.
- [ ] Forged sellall unsupported.

## D6. User UX

- [ ] `/jual` opens picker.
- [ ] `/jual SOL` opens SOL deposit.
- [ ] `/jual XYZ` validation.
- [ ] Buy quote warning/address/gross-fee-net.
- [ ] Session expired contextual message.
- [ ] Status empty vs partial vs error.
- [ ] Riwayat merged and limited total 10.
- [ ] Saldo status labels correct.
- [ ] Returning `/start` copy current.
- [ ] Dynamic sandbox grant amount.

## D7. Admin existing

- [ ] Stock callbacks 4 × 3.
- [ ] Invalid callback feedback.
- [ ] Broadcast cancel sends zero.
- [ ] Preview sends zero.
- [ ] Confirm sends once.
- [ ] Double confirm does not resend.

## D8. Build/DB

- [ ] Clean sqlc generate.
- [ ] `go test ./...`.
- [ ] `go vet ./...`.
- [ ] Build bot.
- [ ] Empty DB migrate latest.
- [ ] Upgrade prior DB snapshot.
- [ ] Generated drift check.
- [ ] Integration tests isolated.

---

# BAGIAN E — ACCEPTANCE CRITERIA REVISI V.1

Revisi Sistem V.1 hanya dianggap selesai bila:

- [ ] Build/test dapat dijalankan dari clean checkout dengan langkah terdokumentasi.
- [ ] Cancel aktif benar-benar mencegah eksekusi.
- [ ] Buy selalu memiliki durable payout state.
- [ ] Failed payout memiliki refund journal tepat sekali.
- [ ] Unknown payout tidak auto-refund.
- [ ] Seluruh jurnal buy/deposit balance per asset.
- [ ] Callback private-only dan owner-scoped.
- [ ] Flow ALL/MAX tidak lagi dapat dijangkau user.
- [ ] Slash dan button Jual memakai flow yang sama.
- [ ] Riwayat memakai payout/deposit lifecycle.
- [ ] Status membedakan empty/partial/error.
- [ ] Quote beli memperlihatkan warning dan breakdown lengkap.
- [ ] Session expired/cancel selalu memberi feedback kontekstual.
- [ ] Onboarding `/start` idempotent dan memakai nilai setting aktual.
- [ ] Audit ganti alamat menyimpan before/after sebenarnya.
- [ ] SOL validator benar-benar base58/public-key valid.
- [ ] Tombol stok admin existing bekerja.
- [ ] Broadcast memiliki preview+confirm dan cancel aman.
- [ ] Full tests hijau di VPS.
- [ ] Smoke bot sandbox diverifikasi manual.
- [ ] `PROJECT_IDEAS.md` diperbarui berdasarkan hasil aktual.

---

# BAGIAN F — NON-GOALS DAN LARANGAN

Jangan dikerjakan dalam revisi ini:

- private key/seed;
- saldo blockchain nyata;
- watcher node nyata;
- payment gateway;
- QRIS/bank/withdraw real;
- Fase 10 chain real;
- KYC/PIN/cooling-off production;
- deploy/restart live tanpa persetujuan eksplisit user.

Larangan teknis:

- Jangan patch symptom tanpa regression test.
- Jangan menelan error lalu menampilkan empty state.
- Jangan memakai float64 untuk money/coin path.
- Jangan menulis saldo langsung di luar ledger service.
- Jangan melakukan chain call di dalam transaction DB.
- Jangan mengklaim refund jika journal refund belum ada.
- Jangan mengklaim confirmed on-chain pada mode mock.
- Jangan menyimpan secret di audit/log/Telegram.
- Jangan menggabungkan seluruh revisi dalam satu commit besar.

---

# BAGIAN G — BLUEPRINT SANDBOX PROPER, PRODUCTION-SHAPED

> Keputusan tambahan: revisi ini harus membuat flow user benar-benar proper untuk simulasi, tetapi struktur domain, state machine, idempotensi, dan adapter harus siap dipakai pada real case. Implementasi real provider tetap dilarang sekarang.

## G0. Definisi “proper sementara untuk simulasi”

Sistem dianggap proper sandbox bila:

1. User dapat top up saldo IDR melalui invoice simulasi, bukan hanya grant `/start`.
2. User dapat membeli crypto memakai saldo simulasi.
3. Buy menghasilkan payout durable dengan lifecycle yang sama seperti production.
4. Semua mutasi uang/coin memakai jurnal double-entry yang balance.
5. Duplicate click, callback retry, worker retry, dan process restart bersifat idempotent.
6. Failure dan unknown state dapat disimulasikan serta terlihat di Saldo/Status/Riwayat.
7. Handler bot tidak mengetahui apakah payment/chain adapter mock atau real.
8. Seluruh layar dan record sandbox jelas berlabel `SIMULASI`/`mock`.
9. Mengganti mock ke real nanti dilakukan lewat adapter + config + operational gate, bukan menulis ulang business flow.

Sandbox proper BUKAN:

- menambah saldo langsung dari tombol tanpa invoice;
- menganggap semua payout selalu sukses seketika;
- membuat txhash random tanpa record durable;
- menaruh seluruh logic mock di handler Telegram;
- memakai status `success` untuk semua scenario;
- menyebut transaksi mock sebagai pembayaran/on-chain asli.

## G1. Target end-to-end user sandbox

Flow utama yang wajib lolos:

```text
/start
  → akun sandbox + grant onboarding idempotent
  → Top Up
  → pilih nominal/channel simulasi
  → invoice pending
  → Simulasikan Pembayaran
  → payment event mock idempotent
  → jurnal topup
  → saldo IDR bertambah
  → Beli
  → pilih coin + nominal
  → quote fresh + alamat + fee + net
  → Confirm
  → trade + payout queued atomik
  → mock chain worker
  → confirmed / failed+refund / unknown
  → Saldo + Status + Riwayat konsisten
```

Jual/deposit mock tetap dipertahankan dan diselaraskan dengan lifecycle yang sama, tetapi fokus mandatory blueprint ini adalah Top Up IDR → Beli → Payout.

## G2. Arsitektur adapter: mock sekarang, real nanti

### G2.1 Payment provider boundary

Definisikan interface domain; nama exact boleh menyesuaikan Go style:

```go
type PaymentProvider interface {
    CreateTopup(ctx context.Context, req CreateTopupRequest) (ProviderInvoice, error)
    ParseEvent(ctx context.Context, raw []byte, signature string) (PaymentEvent, error)
    GetTopupStatus(ctx context.Context, providerRef string) (ProviderTopupStatus, error)
}
```

Kontrak:

- `CreateTopup` hanya membuat invoice provider, tidak mengkredit wallet.
- `ParseEvent` menghasilkan event canonical, tidak query Telegram.
- Kredit dilakukan oleh application/domain service setelah verifikasi + idempotency.
- Mock dan real mengimplementasikan interface yang sama.
- Handler bot memanggil TopupService, bukan MockPayment langsung.

Mock implementation:

- `internal/payments/mock` atau struktur equivalent.
- Provider name: `mock`.
- Provider reference unik dan durable.
- Bisa menghasilkan paid, paid_late, failed, dan amount mismatch untuk test.
- Tidak memakai network call.

Real implementation nanti:

- `internal/payments/<provider>`.
- Signature verification dan raw webhook event.
- Tidak mengubah kontrak service/bot.

### G2.2 Chain payout boundary

Interface canonical:

```go
type ChainProvider interface {
    ValidateAddress(asset, address string) error
    EstimateFee(ctx context.Context, asset string) (decimal.Decimal, error)
    Broadcast(ctx context.Context, req BroadcastRequest) (BroadcastResult, error)
    GetTransaction(ctx context.Context, asset, txHash string) (ChainTransactionStatus, error)
}
```

Kontrak:

- Payout row dibuat sebelum `Broadcast`.
- Broadcast menggunakan durable payout ID/idempotency key.
- Mock adapter dapat dikonfigurasi success/failed/unknown/delay.
- Real adapter nanti mengurus node/nonce/gas, bukan handler.
- Tidak ada private key pada sandbox mock.

### G2.3 Dependency injection/config

Config minimum:

```text
APP_MODE=sandbox|real
PAYMENT_PROVIDER=mock|<real-provider>
CHAIN_PROVIDER=mock|<real-chain-adapter>
```

Fail-closed rules:

- `APP_MODE=real` + mock provider harus ditolak saat startup kecuali explicit development override yang tidak tersedia pada production unit.
- `APP_MODE=sandbox` selalu menampilkan label SIMULASI.
- Mode disimpan/snapshot pada invoice, trade, payout, deposit, dan event penting; jangan hanya membaca env saat render histori.

## G3. Top Up saldo IDR simulasi yang proper

### G3.1 UX menu

Tambahkan Top Up pada submenu akun atau Saldo screen agar menu utama tetap ramping:

```text
Saldo IDR: Rp250.000 — SIMULASI

[➕ Top Up Simulasi]
[🧾 Riwayat] [⬅️ Kembali]
```

Flow:

1. User tekan Top Up.
2. Pilih nominal cepat: Rp50rb, Rp100rb, Rp250rb, Nominal lain.
3. Validasi `min_topup_idr`, maximum sandbox, dan satu invoice aktif.
4. Pilih channel `QRIS Simulasi`. Bank simulasi bukan mandatory pada V.1 agar sesuai keputusan QRIS-first.
5. Bot membuat invoice pending.
6. Tampilkan preview/invoice dengan expiry.
7. Tombol sandbox-only `🧪 Simulasikan Pembayaran`.
8. User memilih scenario test pada mode trial bila diizinkan:
   - Bayar normal;
   - Bayar terlambat exact amount;
   - Nominal salah;
   - Gagal/expired.
9. Payment event diproses melalui service yang sama dengan webhook real nanti.
10. Saldo hanya bertambah setelah event valid dan jurnal committed.

### G3.2 Data model production-shaped

Buat migration baru setelah migration latest saat implementasi. Nomor migration harus ditentukan dari repository aktual, jangan hardcode jika plan admin wallet sudah memakai nomor yang sama.

Table minimum:

```sql
CREATE TABLE topup_invoices (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  mode TEXT NOT NULL CHECK (mode IN ('sandbox','real')),
  provider TEXT NOT NULL,
  channel TEXT NOT NULL CHECK (channel IN ('qris','bank')),
  amount_idr BIGINT NOT NULL CHECK (amount_idr > 0),
  fee_idr BIGINT NOT NULL DEFAULT 0 CHECK (fee_idr >= 0),
  expected_total_idr BIGINT NOT NULL CHECK (expected_total_idr > 0),
  provider_ref TEXT NOT NULL UNIQUE,
  idempotency_key TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL CHECK (status IN (
    'pending','paid','paid_late','amount_mismatch',
    'needs_refund','expired','failed','cancelled'
  )),
  expires_at TIMESTAMPTZ NOT NULL,
  paid_at TIMESTAMPTZ,
  journal_id BIGINT UNIQUE REFERENCES journals(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Tambahkan partial unique satu active invoice per user:

```sql
UNIQUE (user_id) WHERE status = 'pending'
```

Karena partial unique constraint perlu index syntax PostgreSQL, implementasikan sebagai `CREATE UNIQUE INDEX`.

Event table minimum:

```sql
CREATE TABLE payment_events (
  id BIGSERIAL PRIMARY KEY,
  provider TEXT NOT NULL,
  provider_event_id TEXT NOT NULL,
  provider_ref TEXT NOT NULL,
  event_type TEXT NOT NULL,
  amount_idr BIGINT,
  signature_valid BOOLEAN NOT NULL,
  raw_payload TEXT NOT NULL DEFAULT '',
  processed_status TEXT NOT NULL CHECK (processed_status IN (
    'received','processed','duplicate','rejected','error'
  )),
  processing_error TEXT NOT NULL DEFAULT '',
  received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at TIMESTAMPTZ,
  UNIQUE (provider, provider_event_id)
);
```

Sandbox security/privacy:

- Raw payload mock tidak boleh berisi secret.
- Production nanti raw payload access harus dibatasi.
- Jangan menaruh QR/provider secret di invoice/event.

### G3.3 Topup state machine

```text
pending
 ├─ valid paid before expiry ───────────→ paid
 ├─ exact paid after expiry + limit ok ─→ paid_late
 ├─ exact paid late + limit exceeded ───→ needs_refund
 ├─ amount differs ─────────────────────→ amount_mismatch
 ├─ expiry without payment ─────────────→ expired
 ├─ provider final failure ─────────────→ failed
 └─ user cancel before paid ────────────→ cancelled
```

Terminal states tidak boleh kembali ke pending.

Rules:

- `paid`/`paid_late` hanya boleh memiliki satu topup journal.
- Duplicate provider event return success/idempotent tanpa kredit kedua.
- Nominal berbeda tidak pernah auto-credit.
- Paid-late tetap menjalankan daily limit dalam transaksi yang sama.
- `needs_refund` tidak menambah saldo.
- Expiry user interface tidak berarti provider event terlambat boleh dibuang; event tetap diproses menurut paid-late rules.

### G3.4 Journal topup IDR

Canonical journal:

```text
house:provider-clearing IDR  -amount_idr
user:<id> IDR                +amount_idr
sum IDR = 0
```

Fee provider:

- Disimpan di invoice.
- Tidak dijurnal sebagai revenue.
- Jangan kredit `expected_total_idr`; kredit pokok `amount_idr` sesuai kontrak.

Atomic transaction:

1. Lock invoice/user/accounts/wallet dengan urutan konsisten.
2. Check event idempotency.
3. Check current invoice state.
4. Check amount/expiry/limit.
5. Create journal + entries bila eligible.
6. Update wallet cache.
7. Update invoice state/journal ID.
8. Mark event processed.
9. Commit.

### G3.5 Mock payment event transport

Tombol `Simulasikan Pembayaran` tidak boleh langsung memanggil `CreditWallet`.

Ia harus:

1. Membuat canonical mock provider event.
2. Memanggil entry point event processor yang sama dengan webhook real.
3. Menggunakan `provider_event_id` unik dari persisted operation/update token.
4. Menampilkan hasil state machine.

Dengan demikian, real provider nanti hanya mengganti transport event, bukan logic credit.

## G4. Buy crypto simulasi yang proper

### G4.1 Canonical state machine

Quote:

```text
active → used
active → cancelled
active → expired
```

Trade:

```text
created → payout pending
```

Payout:

```text
queued → broadcasting → confirmed
queued/broadcasting → failed → refunded
queued/broadcasting → unknown
unknown → confirmed
unknown → failed → refunded
```

Jangan membuat status trade `success` hanya karena IDR sudah didebit. Outcome user mengikuti payout.

### G4.2 Atomic confirmation boundary

Saat user Confirm:

Dalam satu DB transaction:

1. Validate owner dan active/expiry quote.
2. Re-check risk, price freshness policy, float, dan address snapshot policy.
3. Lock user → ledger accounts → wallet.
4. Consume quote exactly once.
5. Create trade.
6. Create balanced IDR/coin journal sesuai template yang sudah diperbaiki.
7. Create payout row `queued`, unique by trade ID.
8. Update derived wallet cache.
9. Commit.

Sesudah commit:

- Trigger/wake mock payout processor.
- Response boleh berkata `Pembelian tercatat, pengiriman sedang diproses`, bukan langsung confirmed kecuali processor benar-benar sudah menulis confirmed.

### G4.3 Payout model production-shaped

Pastikan payout memiliki minimum:

- mode;
- user/trade ID;
- asset/network;
- gross units;
- fee units;
- sent units;
- to address snapshot;
- idempotency key;
- tx hash nullable;
- status;
- attempt count;
- last error category/message sanitized;
- next retry at;
- broadcast/confirmed/failed timestamps;
- refund journal ID unique nullable;
- created/updated timestamps.

Jangan simpan private key atau signed raw transaction pada sandbox.

### G4.4 Mock payout processor

Mock processor harus terpisah dari bot handler dan memiliki deterministic test controls.

Modes:

- `success`: queued → broadcasting → confirmed + `MOCK-...` txhash.
- `failed`: queued/broadcasting → failed → refund journal once.
- `unknown`: queued/broadcasting → unknown; saldo tidak direfund.
- `delayed`: tetap queued sampai worker tick berikutnya.

Control untuk test:

- dependency/fake policy injected, bukan global random;
- default sandbox user flow boleh success;
- admin/trial-only dapat memilih failure scenario;
- user umum tidak diberi tombol untuk mengeksploitasi refund/state test.

Worker claiming:

- Query `FOR UPDATE SKIP LOCKED` atau equivalent safe claim.
- Attempt count dan retry schedule durable.
- Duplicate workers tidak broadcast dua kali.
- Mock txhash deterministic dari payout ID/attempt atau persisted UUID.

### G4.5 Failed payout refund

Refund journal harus merupakan reversal yang eksplisit dan balance.

Rules:

- Satu payout maksimal satu refund journal.
- Refund hanya pada final `failed`.
- `unknown` tidak refund.
- UI hanya berkata `dana dikembalikan` setelah refund journal committed.
- Retry failed/refund callback tidak mengkredit dua kali.

### G4.6 Quote preview user

Wajib menampilkan:

- mode SIMULASI;
- nominal IDR yang didebit;
- gross coin;
- fee coin + estimasi IDR;
- net coin diterima;
- network;
- address penuh/copyable;
- harga berlaku dan countdown/copy waktu;
- warning irreversible;
- Konfirmasi, Ganti Alamat, Batal.

## G5. Saldo, Status, dan Riwayat untuk sandbox proper

### G5.1 Saldo

Tampilkan:

- saldo IDR simulasi;
- tombol Top Up Simulasi;
- maksimal 3 payout terakhir dengan translated status;
- CTA Status/Riwayat.

### G5.2 Status

Prioritaskan nonterminal/attention:

- invoice pending/paid-late mismatch/needs_refund;
- payout queued/broadcasting/unknown/failed-without-refund;
- deposit pending/manual.

Confirmed/paid terbaru boleh tampil terbatas untuk konteks.

### G5.3 Riwayat

Unified timeline:

- topup invoice/payment;
- payout buy;
- user crypto deposit/sell.

Jangan menampilkan trade internal sebagai sukses bila payout belum confirmed.

Setiap entry:

- timestamp WIB;
- type;
- amount/units;
- status user-facing;
- reference pendek;
- detail action.

## G6. Error taxonomy dan user copy

Buat typed/domain errors, jangan parse string mentah di handler:

- `ErrInsufficientBalance`
- `ErrQuoteExpired`
- `ErrQuoteCancelled`
- `ErrQuoteAlreadyUsed`
- `ErrActiveInvoiceExists`
- `ErrInvoiceExpired`
- `ErrAmountMismatch`
- `ErrLimitExceeded`
- `ErrPayoutUnknown`
- `ErrPriceStale`
- `ErrFloatInsufficient`
- `ErrAddressMissing`
- `ErrInvalidAddress`

Bot formatter memetakan error ke copy + next action. Internal cause/log tetap tersedia, tetapi jangan bocor ke user.

## G7. Recovery dan scheduled jobs sandbox

Job minimum:

1. Expire pending invoice yang lewat expiry.
2. Expire active quote yang lewat expiry.
3. Process queued/retryable payout.
4. Reconcile ledger/wallet/payout obligations.
5. Alert/report failed/unknown states ke admin trial.

Restart requirements:

- Pending invoice tetap ada.
- Paid event retry tetap idempotent.
- Queued payout diproses lagi.
- Unknown tidak berubah otomatis menjadi failed/refund tanpa evidence.
- Session Telegram boleh hilang, tetapi durable invoice/quote/payout tetap bisa dibuka lewat Status.

## G8. Migration dan compatibility strategy

AI executor wajib lebih dulu melihat migration latest dan plan admin wallet agar nomor tidak bentrok.

Recommended separation:

1. Migration topup invoice/payment events.
2. Migration payout lifecycle columns/indexes.
3. Migration ledger account taxonomy/constraints bila aman dipisah.

Compatibility:

- Existing mock payouts/deposits harus dimigrasikan/backfilled dengan `mode='sandbox'`.
- Existing confirmed mock payout tidak boleh diproses ulang worker.
- Existing txhash unique dipertahankan atau dipindahkan dengan migration aman.
- Backfill harus deterministic dan repeat-safe.
- Jangan memakai `IF NOT EXISTS` untuk menutupi schema yang salah; verifikasi definition.

## G9. Implementasi bertahap hemat cost

### Sandbox Phase S1 — contract dan schema

Files expected:

- payment/chain interfaces;
- migrations topup + payout lifecycle;
- sqlc queries;
- typed domain errors.

Acceptance:

- compile;
- migration up from v9/latest;
- existing data backfill safe;
- no bot UX yet.

### Sandbox Phase S2 — topup domain TDD

Implement:

- Create invoice;
- Process payment event;
- paid/paid_late/mismatch/needs_refund/expired;
- balanced journal;
- exactly-once credit.

Acceptance:

- service tests + DB integration/concurrency hijau.

### Sandbox Phase S3 — topup bot UX

Implement:

- Top Up button;
- amount picker/input;
- invoice screen;
- simulate payment through event processor;
- status/detail/cancel.

Acceptance:

- tidak ada direct credit dari handler;
- all screens SIMULASI;
- cancel/expiry/retry clear.

### Sandbox Phase S4 — buy/payout durable TDD

Implement:

- quote confirm atomic trade+payout;
- mock processor;
- success/failed/unknown/delayed;
- refund once;
- recovery.

Acceptance:

- failure injection/concurrency/restart tests hijau.

### Sandbox Phase S5 — user read models

Implement:

- Saldo;
- Status;
- unified Riwayat;
- detail topup/payout/deposit;
- partial/error states.

Acceptance:

- lifecycle display konsisten dengan DB.

### Sandbox Phase S6 — E2E trial

Scenario wajib:

1. User baru `/start`.
2. Topup normal Rp100rb → saldo bertambah sekali.
3. Duplicate payment event → saldo tidak bertambah lagi.
4. Beli SOL Rp50rb → payout confirmed mock.
5. Payout failed → refund once.
6. Payout unknown → tidak refund, Status memberi warning.
7. Invoice expired.
8. Paid late exact amount.
9. Paid late limit exceeded → needs_refund tanpa kredit.
10. Amount mismatch → tidak kredit.
11. Restart dengan queued payout → diproses setelah hidup.
12. Saldo/Status/Riwayat konsisten untuk seluruh scenario.

## G10. Test matrix sandbox proper

### Topup invoice

- [ ] Minimum/maximum validation.
- [ ] Satu active invoice per user.
- [ ] Provider ref/idempotency unique.
- [ ] Cancel pending.
- [ ] Expiry job.
- [ ] Paid normal exactly once.
- [ ] Duplicate event exactly once.
- [ ] Paid late exact + limit OK.
- [ ] Paid late limit exceeded → needs_refund.
- [ ] Amount mismatch no credit.
- [ ] Event invoice unknown rejected/manual-safe.
- [ ] Concurrent duplicate event one journal.
- [ ] Journal IDR sum zero.

### Buy/quote

- [ ] Missing address rejected with CTA.
- [ ] Stale price rejected.
- [ ] Float thin rejected.
- [ ] Insufficient IDR rejected.
- [ ] Fee > threshold rejected.
- [ ] Cancel active blocks confirm.
- [ ] Double confirm one trade+payout.
- [ ] Quote owner scope.
- [ ] Quote preview numbers internally consistent.

### Payout

- [ ] Trade+payout atomic.
- [ ] Success transition.
- [ ] Failed refund once.
- [ ] Unknown no-refund.
- [ ] Unknown later confirmed.
- [ ] Unknown later failed+refund.
- [ ] Delayed survives restart.
- [ ] Duplicate worker safe.
- [ ] Txhash unique.
- [ ] Coin journal sum zero.

### User read side

- [ ] Saldo reflects topup/debit/refund.
- [ ] Status prioritizes nonterminal.
- [ ] Riwayat total order across topup/payout/deposit.
- [ ] DB partial error clearly labeled.
- [ ] No technical raw error.
- [ ] All sandbox records/screens labeled SIMULASI.

## G11. Readiness gate untuk real adapter nanti

Sandbox implementation baru disebut “real-ready” jika:

- [ ] Bot handler hanya bergantung pada application services/interfaces.
- [ ] Mock payment event memakai processor yang sama dengan webhook real.
- [ ] Mock payout worker memakai lifecycle yang sama dengan real chain worker.
- [ ] State transition tervalidasi server-side.
- [ ] Idempotency keys/unique constraints tersedia.
- [ ] Retry/restart recovery teruji.
- [ ] Ledger balance per asset teruji.
- [ ] Unknown state tidak auto-refund.
- [ ] Secrets tidak ada pada schema/log/chat.
- [ ] Mode sandbox/real fail-closed.
- [ ] User read models tidak bergantung pada mock-only assumptions.

Walaupun gate teknis di atas lolos, aktivasi real tetap membutuhkan Stage 2 operational gate di `PROJECT_IDEAS.md`: gateway tertulis, hot wallet baru, node provider, fee/float, kurs pembanding, unit economics, dan sandbox observation.

## G12. Definition of Done tambahan

Selain acceptance Revisi V.1, flow dasar dianggap proper sandbox bila:

- [ ] Top Up Simulasi tersedia sebagai invoice lifecycle, bukan direct balance edit.
- [ ] Payment event duplicate tidak double-credit.
- [ ] User dapat memakai saldo topup untuk Beli.
- [ ] Confirm buy atomik membuat trade+payout queued.
- [ ] Mock payout success/failed/unknown dapat diuji.
- [ ] Failed payout refund tepat sekali.
- [ ] Unknown payout tidak direfund.
- [ ] Saldo, Status, Riwayat membaca durable state yang sama.
- [ ] Restart tidak kehilangan invoice/payout work.
- [ ] Adapter mock dapat diganti tanpa mengubah handler/domain state machine.
- [ ] Full automated tests dan manual E2E sandbox hijau di VPS.

---

# BAGIAN H — FILE MINIMUM UNTUK AI EXECUTOR

Baca berurutan agar hemat token:

1. `.kiro/steering/revisi_system_V.1.md`
2. Bagian Pivot, Locked Decisions, User Flows, Ledger, Current Next Step di `PROJECT_IDEAS.md`
3. `internal/bot/bot.go`
4. `internal/bot/flow.go`
5. `internal/bot/session.go`
6. `internal/bot/keyboard.go`
7. `internal/bot/adminflow.go`
8. `internal/ledger/trade.go`
9. `internal/ledger/deposit.go`
10. `internal/ledger/ledger.go`
11. `internal/chain/chain.go`
12. `db/queries/quotes_trades.sql`
13. `db/queries/ledger.sql`
14. `db/queries/chain.sql`
15. migrations terkait saja
16. tests terkait fase yang sedang dikerjakan

Jangan membaca seluruh 894 baris roadmap berulang kali. Ambil bagian yang relevan dengan task aktif.

---

# BAGIAN I — CHECKPOINT PROGRESS EXECUTOR

## Reconciliation gate sebelum fase berikutnya

AI executor berikutnya wajib mengisi tabel ini dari pemeriksaan code head aktual sebelum mengubah kode. Gunakan satu baris per phase/temuan yang akan disentuh; jangan menyalin semua temuan tanpa verifikasi.

| Scope/temuan | Status (`DONE`/`MASIH VALID`/`BERUBAH`/`DROP`) | Evidence file/test/SHA | Gap tersisa | Next action |
|---|---|---|---|---|
| Admin Wallet v0.14/v0.14.1 | DONE otomatis; manual acceptance pending | `8c60ce1`, `ADMIN_WALLET_CURRENT_STATE.md` | Trial admin/non-admin, restart recovery, restore drill | Jalankan acceptance yang disetujui |
| Phase 2 cancel/callback safety user — RECONCILED 2026-11-03 vs code head `8c60ce1` | MASIH VALID (A1, A8, A15, A23 inti); BERUBAH sebagian (A12 rate-limit callback sudah ada); klasifikasi per temuan di bawah | `db/queries/quotes_trades.sql`, `internal/bot/bot.go:650-685,751-770`, `internal/bot/flow.go:22-40,130-144`, `migrations/000003_quotes_trades.sql`, `internal/bot/session.go` | 6 query/handler/callback/migration edits + RED regression tests | Tunggu approval + SSH ke cbt-vps pulih, lalu Phase 2 TDD per C0 |
| Phase 3 ledger invariant buy/deposit — RECONCILED 2026-11-03 vs head `417c841` | MASIH VALID inti (A3 buy, A4 deposit, Reconcile house-blind); A26 targeted-agg = Phase 9, Phase 3 cukup pre-commit validation | `internal/ledger/trade.go:299-328`, `deposit.go:93-97`, `ledger.go` Reconcile/refreshWalletCache | Perbaiki legs + validate sum=0 + Reconcile house-aware | Gas TDD inline |
### Reconciliation detail Phase 3 (2026-11-03, head `417c841`)
| Temuan | Klasifikasi | Evidence | Gap tersisa |
|---|---|---|---|
| A3 jurnal buy coin tidak balance | MASIH VALID | legs buy `trade.go:299-305`: float −sentUnits, fee-collect +feeUnits → sum coin = 2fee−gross, NOL hanya bila fee=0. IDR legs (±amount) balance | Template `house:float −gross / external:payout +sent / fee-clearing +fee` (nama account final saat implementasi, makna dikunci plan) |
| A3b sisi sell masih bandar | MASIH VALID (perluasan) | legs sell `trade.go:322-327`: user coin −units, `house:inventory` +units — sisa model custodial v0.11; transit = user kirim coin ke sistem (flow deposit), bukan trade sell dari saldo | Samakan dengan template deposit transit; hapus inventory leg dari user-sell path |
| A4 jurnal deposit coin tidak balance | MASIH VALID | legs `deposit.go:93-97`: float +units, cash −IDR, user +IDR → coin sum = +units ≠ 0, tanpa kaki negatif | Tambah `external:deposit −units_in` boundary account |
| Reconcile buta house | MASIH VALID | `ledger.go` Reconcile filter `user:` saja, house* diskip → corrupt float/fee-collect/cash lolos | Perluas ke house accounts; pertahankan user-vs-wallet check existing |
| A26 refreshWalletCache O(n) | VALID tapi PHASE 9 | `refreshWalletCache` panggil `SumLedgerByOwnerAsset` (full scan) tiap trade, 2x per trade user | Phase 3: tambah `validateLegs` pre-commit (O(legs), murah). Targeted query = Phase 9 |
| Referensi pola benar | DONE (acuan) | `stock_deposit.go` float-in: cold −units + float +units = balance | Tiru pola ini untuk buy/deposit |
| Blocker | TIDAK ADA | Go+sqlc+goose di cbt-vps OK; codex quota mati → inline | Gas inline, verify --no-db + targeted tests |

### Reconciliation detail Phase 2 (2026-11-03, head `8c60ce1`)

| Temuan | Klasifikasi | Evidence | Gap tersisa |
|---|---|---|---|
| A1 cancel quote aktif tidak bekerja | MASIH VALID | `ExpireUserQuotes` (`quotes_trades.sql:23-25`) masih predicate `expires_at <= now()` saja, tanpa status `cancelled`. `bot.go:682,769-770` + `flow.go:38` memanggilnya dari `/batal`, `q:no`, `t:cancel` lalu klaim "dibatalkan" — quote future-expiry tetap `active` + `ConsumeQuote` (:11-15) tetap bisa `used`. `createQuoteInner` (`trade.go:112`) expire quote lama saat quote baru dibuat, tapi tidak menutup lubang cancel. CHECK status quotes (`000003:31`) hanya `active/used/expired` — belum ada `cancelled` | Query baru `CancelActiveQuotesByUser` + `CancelQuoteByID` owner-scoped + status `cancelled` (migration baru) + RED tests D1 |
| A8 cancel navigation side effect global | MASIH VALID | `flow.go:34-40` `t:cancel` = `Session.Clear` + `ExpireUserQuotes` (expire SEMUA quote user). `/batal` (`bot.go:751-766`) = `Session.Clear` + `expireActive` + copy selalu "Quote aktif dibatalkan" walau 0 row. Admin close path sama | Pisah navigation-cancel vs quote-cancel; return count; copy kontekstual A23 |
| A12 callback private-chat guard | BERUBAH sebagian | Message path SUDAH private-only (`bot.go:62-65`) + callback rate-limit SUDAH ada (`bot.go:52-55`). TAPI callback path (`handleCallback:650`, `handleFlowCallback` flow.go:22) TIDAK cek `Chat.Type` — group/supergroup callback tetap bisa mutasi. Juga `cb.Message.Message` nil-unsafe (`bot.go:677-678`, `flow.go:27-28`), dan `q:%d:ok` path (`bot.go:687`) `GetQuote(qid)` TANPA owner-scope sebelum `CheckBeforeExecute` | Guard middleware callback: private-chat + nil-safe + ownership; ini overlap A15 |
| A13 callback gagal diam-diam | MASIH VALID | `bot.go:670-671` malformed `q:` = bare `return` tanpa Answer; `flow.go:30-32,43-44,66-67` invalid len/coin = bare `return` tanpa feedback (spinner gantung). `handleFlowCallback:23-26` unregistered/blocked = bare `return`. Positive: `handleCallback:652` + rate-limit branch (`:53`) sudah Answer | Semua branch harus AnswerCallback + copy typed (belum `/start`, blocked, stale, malformed) |
| A15 quote lookup sebelum ownership | MASIH VALID | `bot.go:687` `GetQuote(ctx, qid)` by ID saja (`quotes_trades.sql:17-21` `WHERE id=$1`), baru `Risk.CheckBeforeExecute` (`:692`) pakai `qrow.Asset/Side` milik quote — forged qid user lain bocor ke risk path sebelum `ExecuteTrade` owner-check. Belum ada oracle-exact tapi boundary salah urutan | Query `(quote_id, user_id)` scoped + ownership dulu, risk sesudah; generic response forged |
| A23 `/batal` false confirmation | MASIH VALID | `batal()` (`bot.go:751-766`) selalu return "Quote aktif dibatalkan." — tidak bedakan session-cancel vs quote-cancel vs no-op. `expireActive` return error saja, bukan row count | Return count dari cancel op; 4 copy kontekstual (session / quote / keduanya / tidak ada) |
| A5/A6 admin existing blockers (Phase 8 reminder) | DONE di Admin Wallet path; A6 broadcast preview SISA | Commit `8c60ce1` typed routing + sender/service boundary (`wallet_sender.go`, `wallet_router_test.go`); `ADMIN_WALLET_CURRENT_STATE.md` §3.6 | Broadcast preview/confirm/idempotency (Phase 8 scope) diverifikasi saat trial, bukan Phase 2 |
| Blocker eksekusi | INFRA | `ssh rdpkhorur` timeout 2026-11-03 (host 157.15.40.83:7120). Remote `cbt-vps` (43.134.111.212:22) belum dicoba sesi ini. Lokal: Go TIDAK ada — build/test/vet HANYA di VPS. Dual remote: `origin`=GitHub cry-stat + `server`=cbt-vps bare repo | Pulihkan SSH (coba `cbt-vps` langsung) sebelum codex/TDD loop; tanpa VPS tidak ada verify |

Template update per fase selesai:

```text
Phase:
Baseline SHA:
Result SHA:
Classification before work:
Files changed:
Migration:
Targeted tests:
Full VPS verification:
Manual Telegram trial:
Workspace deployed/restarted:
Live deployed: NO kecuali ada approval eksplisit + evidence
Remaining risk:
Current Next Step:
```

## Progress selesai di jalur Admin Wallet

- [x] Admin Wallet v0.14 diimplementasikan: ringkasan aset, deposit stok, activity feed, detail.
- [x] Migration DB v10 diterapkan di workspace.
- [x] Idempotency payload/admin, typed conflict/cancel/unauthorized selesai.
- [x] Success/cancel audit dibuat atomik dengan mutasi.
- [x] Callback confirm/cancel memakai typed routing + sender/service boundary.
- [x] Preview creation dan duplicate semantics dipindahkan ke ledger service.
- [x] Post-commit balance-read failure tidak lagi melaporkan mutasi sukses sebagai gagal.
- [x] Exact audit query, router tests, dan preview-state tests ditambahkan.
- [x] Full test, vet, build hijau di VPS pada code head `8c60ce1`.
- [x] `scripts/verify.sh --no-db` menghasilkan `VERIFY OK` di VPS.
- [x] Dokumentasi Admin Wallet dikonsolidasikan ke `ADMIN_WALLET_CURRENT_STATE.md`.
- [x] Commit/push code `8c60ce1` dan steering `b4db4a1` selesai ke GitHub + bare VPS.
- [ ] Trial Telegram Admin Wallet dengan akun admin asli.
- [ ] Trial denial/detail dengan akun non-admin kedua.
- [ ] Restore drill custom-format.
- [ ] Approval deploy/restart live.

## Roadmap hardening sistem user yang masih pending

- [x] Phase 1 — verification pipeline minimum tersedia melalui `scripts/verify.sh`; full DB/restore drill masih pending.
- [x] Phase 2 — cancel/callback safety user. AUTOMATED DONE 2026-11-03 (commits `1a117cb`+`e94a6f3`+`00ee7ee`+`33eac5c`, `scripts/verify.sh --no-db` VERIFY OK di VPS). Migration `000011_quote_cancelled.sql` (status `cancelled` + idx user-active). Queries `CancelActiveQuotesByUser`/`CancelQuoteByID`/`GetQuoteForUser`. `cancel.go` (CancelActiveQuotes/CancelQuote/BatalCopy 4-way/CancelAliases incl ❌). `callback_guard.go` (nil-safe+private-only+user, denyCopy typed). `cancel_test.go` pure unit hijau. `bot.go`: q:no per-quote cancel, q:ok owner-scoped lookup, malformed feedback, batal() 4-way, q: guard. `flow.go`: GuardCallback entry, t:cancel navigation-only, stale() feedback semua malformed. `session.go`: MatchButton ❌ batal. `adminflow.go`: broadcast-input cancel = nol kirim. Manual Telegram trial + workspace restart masih pending.
- [x] Phase 3 — ledger invariants buy/deposit user. AUTOMATED DONE 2026-11-03 (commit `360f5ce`, verify --no-db VERIFY OK, targeted DB tests hijau). Buy: float -gross/external:payout +sent/fee-collect +fee. Deposit: +external:deposit -units. validateLegs pre-commit di trade/deposit/stock. ReconcileGlobal baru. Evidence: global SOL sum workspace IDENTIK -54.693206730894899100 sebelum/sesudah test run = jurnal baru sum nol persis. Manual Telegram trial + workspace restart masih pending.
- [x] Phase 4 — durable payout lifecycle. AUTOMATED DONE 2026-11-03 (commits `be020fc`+fixes s/d `cc32b79`, verify --no-db VERIFY OK, 5/5 targeted D3 hijau di workspace DB v12). 000012 kolom lifecycle; payout queued atomik di ExecuteTrade (+PayoutID); ProcessPayout success/failed/unknown/delayed; refund via ledger tepat sekali (SetPayoutRefund guard); unknown no-refund; ClaimQueued SKIP LOCKED; bot copy jujur refund/unknown. Workspace DB v12. Manual Telegram trial + restart masih pending. | RECONCILE NOTE: `bot.go:725` Chain.Payout AFTER trade commit (crash window nyata), `chain.go:96-131` langsung confirmed tanpa failed/unknown/refund, pesan "IDR tidak hilang" tanpa jurnal, tanpa worker/recovery. Skema payouts SUDAH lifecycle-ready (queued/broadcasting/confirmed/failed/unknown + trade_id UNIQUE + idx status) tapi kode tak memakai. Konsumer tunggal bot.go:725 = aman refactor | Gas TDD inline: 000012 kolom lifecycle, payout queued atomik di ExecuteTrade, mock processor success/failed/unknown, refund-once, recovery, tests D3 |
- [x] Phase 5 — deposit/address safety. AUTOMATED DONE 2026-11-03 (s/d `87e8135`, verify VERIFY OK, chain+ledger DB hijau). A14: DepositConflictError user/asset/units no-leak (precheck + race path). A17: ReplaceWalletAddress tx-atomik, audit actual before/after, callers flow ganti. A18: base58 strict 32-byte; mock lama 30-byte diganti vektor valid; test O/I/l/0 + no-panic. | RECONCILE NOTE (tetap dibaca histori): (deposit.go:41 tx sama langsung return tanpa cek user/asset/units; race path :79 sama; user B bisa klaim hasil user A); A17 MASIH VALID (flow.go:192,268 before selalu "(kosong)", alamat lama tak dibaca; audit non-atomik best-effort); A18 MASIH VALID (chain.go:67-71 alnum saja — O/I/l lolos; test existing pass kebetulan karena ada char 0 yang memang di luar 1-9; O tanpa 0 = lolos). Admin.Audit non-tx; ledger bisa InsertAdminAudit via WithTx (db shared, no cycle). Design: typed ErrDepositConflict+no-leak, ledger.ReplaceWalletAddress tx-atomik, base58 decode manual 32-byte (no dep baru) | Gas inline |
- [x] Phase 6 — hapus custodial drift. AUTOMATED DONE 2026-11-03 (s/d 5a595a3, verify VERIFY OK, ledger+chain+bot DB hijau). Sell-bandar mati total: ALL/MAX ditolak, sellAllFlow/t:sellall/CreateQuoteUnits/ExecuteTrade-sell/TestSellFlow/sellall/SellAmountPicker hapus. StepAwaitSell nyasar bandar ditutup. Slash+tombol satu deposit flow. Riwayat transit merge payout+deposit. Test rerun-safe (tx_hash nanotime). | RECONCILE NOTE (tetap dibaca histori): `0b023fd` | SEMUA MASIH VALID: parseTradeArgs terima ALL/MAX (bot.go:534), sellAllFlow hidup (594), t:sellall (flow.go:148), SellAmountPicker UNREACHABLE (cuma test), StepAwaitSell DI-SET tapi layar sell langsung return deposit (flow.go:113-133) sehingga input nominal user nyasar ke quoteFlow BANDAR side-sell (BUG nyata A11), quoteFlow side-sell + CreateQuote sell + CreateQuoteUnits + ExecuteTrade sell-inventory reachable via lubang itu, riwayat trade-lama (bot.go:772), /jual slash cuma arahan vs tombol deposit. TestSellFlow + 3 sellall SKIP = hapus. | Gas: hapus sell-bandar total + riwayat transit + showSellDeposit shared + tutup forged t:sellall |
- [x] Phase 7 — user UX states. AUTOMATED DONE 2026-11-03 (commit `e7e7ede`, verify VERIFY OK, bot unit hijau). A16 start grant-fix + saldo dinamis; A19 quote bruto/fee-IDR/net/alamat penuh/warning; A20 status error/empty/partial; A21 ExpiredStep + copy; A22 label senilai + neto; A24 mapping ID + heading; A25 welcome transit + Konfirmasi. Manual Telegram trial masih pending. | RECONCILE NOTE (tetap dibaca histori): A16 MASIH VALID (start existing tanpa grant-fix, welcome hardcode Rp1jt; GrantSandbox idempotent tinggal panggil); A19 MASIH VALID (quoteFlowBuy alamat pendek, tanpa warning/fee-IDR/gross-net); A20-status MASIH VALID (statusCmd samarkan DB error jadi Belum ada); A21 MASIH VALID (expired -> Perintah tidak dikenal; Sessions.Get idle-buta); A22 MASIH VALID (label Rp50rb masuk, hasil tulis IDR+padahal neto); A23 DONE P2; A24 MASIH VALID (heading Terkirim, status mentah queued); A25 MASIH VALID sebagian (welcome returning stale, tombol Confirm EN). A16-A25 scope jelas, gas inline |
- [x] Phase 8 — admin existing blockers. AUTOMATED DONE 2026-11-03 (commit `a5fb336`, verify VERIFY OK, bot unit hijau). A5 contract test 4x3 hijau. A6 preview+confirm+dedup60s, kirim hanya via callback, session ganda ditolak, cancel nol kirim. Slash langsung + dedup. Manual Telegram trial masih pending. | RECONCILE NOTE (tetap dibaca histori): A5 DONE (builder a:stockfloat:SOL + parser case, adminflow.go:132-140); A6 MASIH VALID (StepAdminBroadcast teks langsung doBroadcast tanpa preview/confirm; double-tap/double-slash = double broadcast). |
- [x] Phase 9 — RECONCILED 2026-11-03 vs head e5a8e4b | A26 MASIH VALID (refreshWalletCache SumLedgerByOwnerAsset full-scan, 2x/trade; callers trade/deposit/refund/grant); A27: entry-account asset match tanpa DB enforce; deposits/payouts tanpa units>0/fee>=0; sent bisa drift dari units-fee; 103 active quotes tapi 0 user >1 active = partial unique AMAN; test isolation nanotime OK kecuali trade_test legacy (cek). Gas: SumLedgerByOwner+refresh 1q; 000013 trigger+checks+sent GENERATED+partial unique; JournalSums/trigger/equivalence tests; EXPLAIN |
- [ ] Sandbox S1 — adapter contract + schema.
- [ ] Sandbox S2 — topup domain TDD.
- [ ] Sandbox S3 — topup bot UX.
- [ ] Sandbox S4 — durable buy/payout TDD.
- [ ] Sandbox S5 — Saldo/Status/Riwayat read models.
- [ ] Sandbox S6 — E2E trial + restart recovery.
- [x] Full VPS verification untuk scope Admin Wallet terkini.
- [ ] Manual sandbox trial end-to-end user.
- [x] Steering docs terkini dan sudah dikonsolidasikan.
- [x] Commit/push progress Admin Wallet selesai.
- [ ] Deploy approval obtained.

---

## Final Recommendation Terkini

Urutan paling aman dari posisi code head `8c60ce1` dan repository/docs head `a46427b`:

1. Selesaikan trial manual Admin Wallet admin+non-admin dan verifikasi audit rows.
2. Jalankan restore drill custom-format pada dedicated drill DB bila maintenance window disetujui.
3. Mulai Phase 2–3: cancel semantics user dan ledger balance buy/deposit.
4. Bangun durable payout queued+refund/unknown state machine.
5. Bangun Top Up Simulasi production-shaped melalui Sandbox S1–S3.
6. Satukan flow Jual transit, hapus ALL/MAX, dan perbarui Riwayat/Status/Saldo.
7. Jalankan E2E sandbox Top Up → Beli → Payout success/failed/unknown → recovery.
8. Jangan masuk real gateway/chain atau restart live sebelum gate dan approval eksplisit.

Admin Wallet sekarang dapat dipercaya untuk scope jurnal sandbox-nya. Prioritas teknis berikutnya berpindah ke fondasi user: topup IDR simulasi, ledger buy/deposit yang balance, payout durable, dan UX lifecycle.
