#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Resolve a working Python interpreter. On Windows, `python3` is often the
# Microsoft Store alias stub, so probe for a real interpreter before failing.
PY=""
for candidate in python3 python py; do
  if command -v "$candidate" >/dev/null 2>&1 && "$candidate" -c 'import sys; sys.exit(0 if sys.version_info.major == 3 else 1)' >/dev/null 2>&1; then
    PY="$candidate"
    break
  fi
done
[[ -n "$PY" ]] || { echo "python3 (3.x) is required" >&2; exit 1; }

command -v sqlc >/dev/null || { echo "sqlc is required" >&2; exit 1; }
[[ "$(sqlc version)" == "v1.29.0" ]] || { echo "sqlc v1.29.0 is required" >&2; exit 1; }
rm -rf trading/schema internal/tradingdb
mkdir -p trading/schema
"$PY" - <<'PY'
from pathlib import Path
for migration in sorted(Path('trading/migrations').glob('*.sql')):
    text = migration.read_text()
    up = text.split('-- +goose Down', 1)[0]
    Path('trading/schema', migration.name).write_text(up)
PY
sqlc generate -f trading/sqlc.yaml