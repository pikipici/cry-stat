# P3 Live Agent E2E Acceptance

## P3 harness verdict: PASS

P3 resilience harness; not a data-quality PASS or investment verdict.

`COMPLETED_WITH_GAPS` is a runtime completion state, not a quality PASS. The resilience harness may PASS when gaps are preserved, counted, and reported rather than whitened.

`failed_count` definition: Count of processed candidates with any provider unavailable or identity/data-blocking label; gaps remain failures even though processing completed.

## Live runs

| Run | Runtime | Quality | candidates | processed | failed | requests | error_class | duration |
|---|---|---|---:|---:|---:|---:|---|---:|
| e2e-mints-001 | `COMPLETED_WITH_GAPS` | `NOT_PASS` | 3 | 3 | 1 | 15 | `ProviderOrIdentityGap` | 12.928s |
| e2e-mints-002 | `COMPLETED_WITH_GAPS` | `NOT_PASS` | 3 | 3 | 1 | 15 | `ProviderOrIdentityGap` | 14.217s |

Both runs processed the same three mints. One mint has `rpc_mint=unavailable`, remains `BLOCKED_IDENTITY`, and is counted in `failed_count=1`; it is not quality-whitened.

## Verification

- Same-mint repeat consistency: **PASS**.
- e2e-mints-001: output tree, SQLite run/3 unique records/events, no secret leakage, no buy recommendation, and no transaction: **PASS**.
- e2e-mints-002: output tree, SQLite run/3 unique records/events, no secret leakage, no buy recommendation, and no transaction: **PASS**.
- Fault injection: **16/16 detected**.
- Resume invariants: **PASS**; no duplicate and prior failure is not whitened.
- Evidence loss for handled errors: **0**.
- Provider failure -> buy: **0**.

Machine-readable evidence: `agent_e2e_evidence.json`.
