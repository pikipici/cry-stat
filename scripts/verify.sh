#!/usr/bin/env bash
# Crypto Station verify script (v0.14.1 R6).
# Jalankan dengan bash:  bash scripts/verify.sh
# (dash/sh tidak mendukung `set -o pipefail`.)
# Satu command deterministic dari clean checkout:
#   1. cek tool versions (go/sqlc/goose minimum)
#   2. rebuild db/schema dari migrations (tanpa copy manual)
#   3. sqlc generate
#   4. gofmt check
#   5. go vet + go test + go build
#   6. smoke DB bila DATABASE_URL tersedia
#
# Dipakai di cbt-vps (bukan Windows lokal):
#   ./scripts/verify.sh
set -euo pipefail

cd "$(dirname "$0")/.."

GO_MIN="1.26"
SQLC_MIN="1.29.0"
GOOSE_MIN="3.28.0"

fail() { echo "VERIFY FAIL: $*" >&2; exit 1; }
info() { echo "== $*"; }

# 1. Tool versions.
command -v go >/dev/null || fail "go tidak ditemukan di PATH"
command -v sqlc >/dev/null || fail "sqlc tidak ditemukan di PATH"
command -v goose >/dev/null || fail "goose tidak ditemukan di PATH"

GO_VER="$(go version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)"
SQLC_VER="$(sqlc version 2>&1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)"
GOOSE_VER="$(goose -version 2>&1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)"
info "go=$GO_VER sqlc=$SQLC_VER goose=$GOOSE_VER (min go=$GO_MIN sqlc=$SQLC_MIN goose=$GOOSE_MIN)"

ver_ge() { [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -1)" = "$2" ]; }
ver_ge "$GO_VER" "$GO_MIN" || fail "go $GO_VER < min $GO_MIN"
ver_ge "$SQLC_VER" "$SQLC_MIN" || fail "sqlc $SQLC_VER < min $SQLC_MIN"
ver_ge "$GOOSE_VER" "$GOOSE_MIN" || fail "goose $GOOSE_VER < min $GOOSE_MIN"

# 2. Rebuild db/schema dari migrations (deterministic, numeric sort).
#    Ambil isi setelah baris "-- +goose Up"; migration project memakai
#    CREATE TABLE IF NOT EXISTS sehingga aman untuk sqlc schema input.
info "rebuild db/schema dari migrations"
rm -rf db/schema
mkdir -p db/schema
for f in $(ls migrations/*.sql | sort -V); do
  base="$(basename "$f")"
  awk 'BEGIN{show=0} /-- \+goose Up/{show=1; next} show{print}' "$f" > "db/schema/$base"
  [ -s "db/schema/$base" ] || fail "schema kosong untuk $base"
done
ls db/schema | head -20

# 3. sqlc generate.
info "sqlc generate"
sqlc generate

# 4. gofmt check ( writelist kosong = bersih ).
info "gofmt check"
GOFMT_OUT="$(gofmt -l ./cmd ./internal ./db 2>/dev/null || true)"
[ -z "$GOFMT_OUT" ] || fail "file belum gofmt: $GOFMT_OUT"
echo "gofmt bersih"

# 5. vet + test + build.
info "go vet ./..."
go vet ./...
info "go test -count=1 ./..."
go test -count=1 ./...
info "go build ./cmd/bot"
go build -o /tmp/cs-bot ./cmd/bot
echo "BUILD OK: /tmp/cs-bot"

# 6. Smoke DB bila DATABASE_URL tersedia.
if [ -n "${DATABASE_URL:-}" ]; then
  info "smoke DB (token kosong)"
  BOT_TOKEN= /tmp/cs-bot || fail "smoke exit non-zero"
else
  info "DATABASE_URL kosong — smoke dilewati"
fi

echo "VERIFY OK"
