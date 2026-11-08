# validation/ — Pre-Phase-5 real-world validation harness

This directory holds the reproducibility layer for the Pre-Phase-5 acceptance
work described in `PRE_PHASE5_REAL_WORLD_VALIDATION_RUNBOOK.md`. Nothing here
trades, signs, or sends. Every network path is read-only.

## Layout

| Path | Purpose | Committed? |
|---|---|---|
| `bin/` | Validation CLIs and shared helpers | yes |
| `tests/` | Harness unit tests (redaction, read-only guard, provenance) | yes |
| `schemas/` | JSON Schemas for evidence and metrics records | yes |
| `fixtures/` | Synthetic inputs only, always `synthetic=true` | yes (small) |
| `config.example.json` | **Locked** acceptance thresholds | yes |
| `manifests/` | Append-only JSONL provenance per dataset | yes |
| `raw/` | Immutable, content-addressed API responses | **no** — git-ignored |
| `normalized/` | Derived data referencing raw records | **no** |
| `checkpoints/` | Resume state, config history | **no** |
| `logs/` | Structured logs, secrets redacted | **no** |
| `reports/` | Acceptance reports | yes |
| `exports/` | CSV exports for the Phase 4 engine | **no** |

## Run the P0 harness

```bash
python validation/bin/run_p0_harness.py
```

Exit code 0 means all three gates passed: project unit tests, the skill structure
validator, and the validation harness tests. The JSON report lands in
`validation/reports/p0_harness_<run_id>.json` and carries the full provenance
envelope.

## Run only the harness tests

```bash
python -m unittest discover -s validation/tests -v
```

## Modules

| Module | Responsibility |
|---|---|
| `bin/redact.py` | Strip credentials from URLs, headers, text, and nested JSON before anything is written to disk |
| `bin/provenance.py` | run_id, UTC stamps, code/config versions, atomic writes, content-addressed raw evidence, append-only manifests |
| `bin/run_p0_harness.py` | Execute the P0 gates and emit a provenance-stamped report |

## Rules that the harness enforces

1. **Read-only.** `tests/test_readonly_guard.py` scans `scripts/` for signing,
   sending, key material, and swap constructs. Adding one fails the suite.
2. **HTTPS only.** Plain `http://` endpoints outside localhost fail the suite.
3. **No secrets on disk.** Every write path runs through `redact.py`; a leaked
   value inside a structured report raises through `assert_no_secrets()`.
4. **Provenance always.** No evidence file is valid without a run_id, a UTC
   timestamp, the code version, the config version, and a `synthetic` flag.
5. **Append-only.** Raw evidence is content-addressed, manifests are JSONL that
   only ever append. Corrections create new records referencing the old ones.

## Threshold changes

`config.example.json` is versioned `thresholds-v1.0.0`, locked before any live
data was collected. Editing it after observation creates a **new experiment**:

1. bump `config_version`
2. append the change and its reason to `validation/checkpoints/config_history.json`
3. record both hashes in the affected stage report

A silent edit invalidates the acceptance claim.

## Environment notes

- On Windows, call `python`, not `python3`: the latter resolves to the Microsoft
  Store alias stub and exits without running anything.
- The project baseline was executed on CPython 3.14.2. The project's own CI
  target is 3.11/3.12; a run there is still pending and is tracked in
  `reports/ENVIRONMENT_AND_REPRODUCTION.md`.
- `SOLANA_RPC_URL` must be HTTPS. A public endpoint is fine for smoke tests and
  is explicitly not a production capacity assumption.