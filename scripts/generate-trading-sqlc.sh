#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
command -v python3 >/dev/null || { echo "python3 is required" >&2; exit 1; }
command -v sqlc >/dev/null || { echo "sqlc is required" >&2; exit 1; }
[[ "$(sqlc version)" == "v1.29.0" ]] || { echo "sqlc v1.29.0 is required" >&2; exit 1; }
rm -rf trading/schema internal/tradingdb
mkdir -p trading/schema
python3 - <<'PY'
from pathlib import Path
for migration in sorted(Path('trading/migrations').glob('*.sql')):
    text = migration.read_text()
    up = text.split('-- +goose Down', 1)[0]
    Path('trading/schema', migration.name).write_text(up)
PY
sqlc generate -f trading/sqlc.yaml
