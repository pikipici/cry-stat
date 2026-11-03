# Crypto Station — Runtime & Verification Guide Terkini

> Status: CURRENT WORKSPACE GUIDE — Revisi V.1 + correctness follow-up automated done; manual Telegram trial masih pending.
> Updated: 2026-11-04
> Code head: `fe1d906`; docs head: `c0c8700`; DB workspace: v14
> Eksekusi runtime: `cbt-vps`; lokal dipakai untuk coding + Git saja
> Live production: jangan deploy/restart tanpa approval eksplisit user

File ini dulu hanya berisi scaffold Fase 0. Sekarang fungsinya adalah panduan singkat untuk generate, verify, migrate, build, dan smoke workspace.

## Source of Truth

Baca berurutan:

1. `.kiro/steering/ADMIN_WALLET_CURRENT_STATE.md` — kondisi Admin Wallet terkini.
2. `.kiro/steering/revisi_system_V.1.md` — roadmap hardening sistem user/topup/buy.
3. `PROJECT_IDEAS.md` — keputusan produk, model transit, dan gate Stage 2.
4. `TRIAL.md` — checklist trial manual terkini.
5. `STAGE0.md` — pekerjaan validasi eksternal sebelum Stage 2.

## Environment

Secret dan connection string asli tidak boleh masuk repo atau output dokumentasi.

Variabel utama:

```text
DATABASE_URL=[REDACTED]
BOT_TOKEN=[REDACTED atau kosong untuk DB smoke]
ADMIN_TELEGRAM_IDS=7254686681
HOTWALLET_KEY_EVM=belum dipakai di sandbox
HOTWALLET_KEY_SOL=belum dipakai di sandbox
```

Private key/seed tidak boleh dimasukkan sebelum gate Stage 2 dan Fase 10 disetujui.

## Toolchain yang Dikunci

- Go 1.27.3 (`go.mod` minimum 1.26)
- PostgreSQL 16
- Goose 3.28.0
- sqlc 1.29.0
- pgx/v5

Generated `internal/db/` dan `db/schema/` tidak di-commit; regenerate di VPS.

## Workspace VPS

```bash
cd /home/ubuntu/crypto-station
export PATH=$PATH:/usr/local/go/bin
export DATABASE_URL="[REDACTED]"

sqlc generate
bash scripts/verify.sh --no-db
```

`--no-db` menjalankan version check, schema extraction, sqlc drift check, gofmt, vet, tests, dan build.

## Full Verification Workspace DB

Hanya untuk DB workspace/test dan wajib explicit opt-in:

```bash
cd /home/ubuntu/crypto-station
export PATH=$PATH:/usr/local/go/bin
export DATABASE_URL="[REDACTED]"
export VERIFY_ALLOW_DB_MIGRATION=YES
bash scripts/verify.sh --full
```

Jangan arahkan mode full ke production.

## Migration Manual

```bash
cd /home/ubuntu/crypto-station
export DATABASE_URL="[REDACTED]"
goose -dir ./migrations postgres "$DATABASE_URL" status
goose -dir ./migrations postgres "$DATABASE_URL" up
```

Workspace saat ini berada di migration v14 (`000014_payout_correctness.sql`). Migration v10 tetap irreversible by policy; migration 000011–000014 bersifat forward/additive sesuai roadmap revisi.

## Build dan Smoke DB

```bash
cd /home/ubuntu/crypto-station
sqlc generate
go build -o /tmp/cs-bot ./cmd/bot
BOT_TOKEN= DATABASE_URL="[REDACTED]" /tmp/cs-bot
```

Token kosong dipakai untuk smoke DB, bukan menjalankan bot Telegram.

## Bot Workspace

Workspace pernah direstart dan recovery data diverifikasi pada correctness head sebelumnya. Follow-up `fe1d906` sudah build, test, verify, dan DB-smoke hijau, tetapi proses bot belum direstart lagi ke binary tersebut karena restart workspace memerlukan persetujuan user. Restart/deploy live membutuhkan approval eksplisit terpisah.

Follow-up `fe1d906` menambahkan runtime durable payout recovery setiap 30 detik: bounded scan → atomic claim/reclaim → claim-token ownership → processing. API SELECT-only lama sudah dihapus; confirmed payout + chain event sekarang commit atomik.

Sesudah approval restart workspace ke `fe1d906`:

1. Verifikasi `getMe`/startup log.
2. Jalankan `TRIAL.md` T1–T3.
3. Cek feed dan detail tetap tersedia setelah restart.
4. Dokumentasikan hasil; jangan mengklaim trial selesai sebelum evidence ada.

## Backup sebelum Operasi Berisiko

Gunakan custom-format:

```bash
export DBURL="[REDACTED]"
BACKUP="/tmp/cs-backup-pre-<label>-$(date +%Y%m%d-%H%M).dump"
pg_dump "$DBURL" --format=custom --no-owner --no-privileges --file "$BACKUP"
pg_restore --list "$BACKUP" >/dev/null
```

Restore drill wajib ke dedicated database drill terlebih dahulu. Prosedur lengkap ada di `.kiro/steering/ADMIN_WALLET_CURRENT_STATE.md`.

## Posisi Berikutnya

1. Dengan approval user, restart bot workspace ke `fe1d906` dan verifikasi recovery-worker log.
2. Trial Telegram Admin Wallet + user payout flow dengan akun admin.
3. Trial denial/detail/callback dengan akun non-admin dan verifikasi audit rows.
4. Putuskan CF-08 historical SOL adjustment: eksekusi/tunda/tolak; jangan mutasi tanpa approval.
5. Jalankan restore drill custom-format bila maintenance window disetujui.
6. Setelah manual acceptance, lanjut Sandbox S1–S6 production-shaped.

Payments asli, chain asli, private key, watcher nyata, dan deploy/restart live tetap dilarang sebelum gate dan approval eksplisit.
