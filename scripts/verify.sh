#!/usr/bin/env bash
# Deterministic v0.14.1 verification. Usage: verify.sh [--full|--no-db]
set -euo pipefail
cd "$(dirname "$0")/.."

MODE="${1:---no-db}"
case "$MODE" in --full|--no-db) ;; *) echo "usage: $0 [--full|--no-db]" >&2; exit 2;; esac
fail() { echo "VERIFY FAIL: $*" >&2; exit 1; }
info() { echo "== $*"; }
need_exact() {
  local tool="$1" want="$2" got
  command -v "$tool" >/dev/null || fail "$tool tidak ditemukan"
  case "$tool" in
    go) got="$(go version)"; got="${got#* go}"; got="${got%% *}" ;;
    sqlc) got="$(sqlc version 2>&1)"; got="${got##* }"; got="${got#v}" ;;
    goose) got="$(goose -version 2>&1)"; got="${got##* }"; got="${got#v}" ;;
  esac
  [ "$got" = "$want" ] || fail "$tool=$got, wajib $want"
}
need_exact go 1.27.3
need_exact sqlc 1.29.0
need_exact goose 3.28.0

schema_tmp="$(mktemp -d)"
bin_tmp="$(mktemp)"
trap 'rm -rf "$schema_tmp" "$bin_tmp"' EXIT
mkdir -p "$schema_tmp/schema"
# Parse only Goose Up sections; stop at Down. Null-delimited iteration is safe
# for whitespace in paths and LC_ALL gives deterministic migration ordering.
while IFS= read -r -d '' f; do
  base="$(basename "$f")"
  perl -ne 'if (/^-- \+goose Up\s*$/) {$up=1; next} if (/^-- \+goose Down\s*$/) {$up=0} print if $up' "$f" > "$schema_tmp/schema/$base"
  [ -s "$schema_tmp/schema/$base" ] || fail "Up kosong: $f"
done < <(LC_ALL=C find migrations -maxdepth 1 -type f -name '*.sql' -print0 | sort -zV)

info "schema/sqlc drift check"
diff -ru db/schema "$schema_tmp/schema" || fail "db/schema drift; regenerate from migration Up sections"
sqlc generate
[ -z "$(git diff --name-only -- db/schema internal/db)" ] || fail "generated DB drift"
[ -z "$(gofmt -l ./cmd ./internal ./db 2>/dev/null)" ] || fail "gofmt drift"
go vet ./...
go test -count=1 ./...
go build -o "$bin_tmp" ./cmd/bot
if [ "$MODE" = --full ]; then
  [ "${VERIFY_ALLOW_DB_MIGRATION:-}" = "YES" ] || fail "--full memerlukan VERIFY_ALLOW_DB_MIGRATION=YES (test/workspace DB saja)"
  [ -n "${DATABASE_URL:-}" ] || fail "--full memerlukan DATABASE_URL"
  case "$DATABASE_URL" in *production*|*prod*) fail "DATABASE_URL tampak production; ditolak";; esac
  goose -dir migrations postgres "$DATABASE_URL" up
  goose -dir migrations postgres "$DATABASE_URL" status
  BOT_TOKEN= "$bin_tmp"
else
  info "mode no-db: migration/smoke DB dilewati"
fi
info "VERIFY OK"
