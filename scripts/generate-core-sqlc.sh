#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

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

rm -rf db/schema internal/db
mkdir -p db/schema
"$PY" - <<'PY'
from pathlib import Path
for migration in sorted(Path('migrations').glob('*.sql')):
    text = migration.read_text()
    up = text.split('-- +goose Down', 1)[0]
    Path('db/schema', migration.name).write_text(up)
PY
sqlc generate