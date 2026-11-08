# ENVIRONMENT AND REPRODUCTION — P0 Baseline

| Field | Value |
|---|---|
| Stage | P0 — baseline, inventory, security |
| Executed at (UTC) | 2026-11-08T13:09:27Z |
| Code under test | `meme-token-skill@1.1.0+build=2950b1126c3dfe4e` |
| Source archive | `meme-token-skill-v1.1.0.zip` |
| Archive sha256 | `07e506260e11ad0d4f9346f39cd0dbce47f6f56e87953c92d6a5d8dcccd10bd8` |
| Threshold config | `thresholds-v1.0.0` |
| Config sha256 | `34a613957e3655e4a537d08556d90ddc9f1d2a1cbab1cf7964be184bde8cd209` |
| Status | **PASS** (baseline only; live connectivity is P1) |

## 1. Environment

| Item | Value |
|---|---|
| Python | 3.14.2 (CPython) |
| OS | Windows-11-10.0.26200-SP0 |
| Architecture | AMD64 |
| Shell | Git Bash (MSYS) |
| Working dir | `validation_work/meme-token-skill` |

Note on Python: the project's own CI expectation is 3.11/3.12. This run used
3.14.2, the interpreter available on the machine. Every test below passed on
3.14.2, which is evidence the code is not pinned to an older minor. It is **not**
evidence about 3.11/3.12 specifically.

Note on `python3` vs `python`: on this Windows host the `python3` command resolves
to the Microsoft Store alias stub, which exits non-zero without running anything.
All commands here use `python`. Any automation that must work on both platforms
should probe for a real 3.x interpreter rather than calling `python3` blindly.

## 2. Baseline test results

| Check | Command | Result |
|---|---|---|
| Unit + integration tests | `python -m unittest discover -s tests -v` | **68 tests, OK** |
| Skill structure validator | `python scripts/validate_skill.py` | **PASS** — 68 Markdown and 15 JSON files checked |
| Validation harness tests | `python -m unittest discover -s validation/tests -v` | **39 tests, OK** |

Raw logs are archived at `baseline/unittest.log` and `baseline/validate_skill.log`
outside the skill tree; they are reproduced verbatim by re-running the commands.

Exit codes: `unittest=0`, `validate_skill=0`. No test was failing before this
session began, so there is no pre-existing failure to report separately.

## 3. Network call inventory (P0 task 3)

Every outbound call the pipeline can make, by module:

| # | Provider | Endpoint | Method | Auth | Transport / timeout | Retries |
|---|---|---|---|---|---|---|
| 1 | DEX Screener | `https://api.dexscreener.com/token-profiles/latest/v1` | GET | none | `HttpClient`, 12 s | 2, exponential + `Retry-After` |
| 2 | DEX Screener | `https://api.dexscreener.com/token-pairs/v1/solana/{mint}` | GET | none | `HttpClient`, 12 s | 2, exponential + `Retry-After` |
| 3 | Solana RPC | `getAccountInfo` | POST | URL query key (optional) | `HttpClient`, 12 s | 2 within phase2, 1 within phase3 agent |
| 4 | Solana RPC | `getTokenSupply` | POST | URL query key (optional) | same | same |
| 5 | Solana RPC | `getTokenLargestAccounts` | POST | URL query key (optional) | same | same |

Source-of-truth locations: `scripts/phase2/http.py` (all transport),
`scripts/phase2/dex.py` (endpoints 1–2), `scripts/phase2/solana.py` (endpoints 3–5),
`scripts/phase2/cli.py` (pause 0.8 s discover / 1.1 s profiles / 0.3 s inspect),
`scripts/phase3/agent.py` (timeout 12 s clamped to 1..30, pause 0.85 s clamped to 0..10).

### Rate-limit and retry behaviour

- Retryable status codes: `429, 500, 502, 503, 504`.
- On `429` with a `Retry-After` header the wait is `min(3s, retry_after)`; without
  the header it is `0.45 * 2^attempt` (0.45 s, 0.9 s).
- `4xx` other than 429 is **not** retried; it raises `ProviderError` immediately.
- Response bodies are capped at 3,000,000 bytes and rejected above that.
- Non-HTTPS endpoints are refused before any socket is opened.
- These are coarse client-side pauses, **not** a documented provider quota. No
  provider limit is asserted here because none was verified against vendor docs.

### Caching

There is no HTTP cache. Every run re-fetches. Historical output directories are
inputs for replay only and are never presented as live observations.

### Write-transaction paths

None. See §4.

## 4. Read-only security audit (P0 task 4)

| Check | Method | Result |
|---|---|---|
| Signing / sending calls | static scan of `scripts/` for 18 forbidden tokens | **none found** |
| Private key material | scan for `private_key`, `secret_key`, `seed_phrase`, `mnemonic`, 64-byte keypair arrays | **none found** |
| `sendTransaction` / `signTransaction` / `signAllTransactions` | scan | **none found** |
| Airdrop / account creation | scan for `requestAirdrop`, `createAssociatedTokenAccount` | **none found** |
| DEX swap integrations | scan for `swap`, `raydium`, `jupiter` | **2 docstring disclaimers only** (allowlisted with reasons) |
| Plaintext HTTP endpoints | regex for `http://` excluding localhost | **none found** |
| HTTPS enforcement | `HttpClient.request_json` raises before connecting on non-HTTPS | enforced in code |

The static guard lives at `validation/tests/test_readonly_guard.py` and runs as
part of the harness suite, so it fails the build if anyone adds a write path.
Its allowlist carries a written justification per entry rather than a bare
suppression.

### Two allowlisted false positives

| File | Token | Why it is safe |
|---|---|---|
| `scripts/phase2/__init__.py` | `swap` | Module docstring: "No swaps, private keys, wallets, token purchase signals, or transaction submission." |
| `scripts/phase3/report.py` | `swap` | Report footer: "No output can be interpreted as a buy signal. No wallet access, swaps, or orders." |

Both are statements that the code does **not** do the thing. Neither is an
operation.

## 5. Secret redaction (P0 task 6)

`validation/bin/redact.py` provides `redact_secrets()`, `redact_url()`,
`redact_headers()`, `redact_text()`, and `assert_no_secrets()`.

Covered shapes: API key query parameters, URL userinfo passwords, `Authorization`
and `Cookie` headers, `Bearer` tokens, JWTs, `sk-`/`ghp_`/`xox*` prefixed keys,
bare 64-hex strings, and 64-byte Solana keypair JSON arrays. 20 tests in
`validation/tests/test_redact.py` assert that each shape is removed and that
ordinary prose is left untouched.

`assert_no_secrets()` is the last line of defence before an evidence file is
written: it raises rather than silently persisting a leaked value.

## 6. Provenance and immutable evidence (P0 tasks 5 and 7)

`validation/bin/provenance.py` provides:

| Function | Guarantee |
|---|---|
| `new_run_id()` | sortable, filename-safe, collision-resistant |
| `utc_now()` | second-resolution UTC with `Z` suffix |
| `code_version()` | skill tag + sha256 prefix of `BUILD_INFO.md` |
| `config_version()` | version string **and** sha256 of the threshold file |
| `evidence_envelope()` | every record carries run_id, stage, UTC time, `synthetic`, code/config version, environment |
| `atomic_write_json()` | temp file + `os.replace`, so a crash cannot leave a half-written record |
| `content_addressed_path()` | raw evidence filename **is** the sha256 of its bytes → append-only by construction |
| `append_manifest_line()` | JSONL provenance that only ever appends |

## 7. Locked acceptance thresholds

`validation/config.example.json` was written **before** any live data was
collected, versioned `thresholds-v1.0.0`, sha256 recorded above.

Silent edits are forbidden. Any change after the first live observation must bump
`config_version` and be recorded in `validation/checkpoints/config_history.json`,
producing a new experiment rather than a quiet retune.

Key thresholds locked for later stages:

| Stage | Threshold |
|---|---|
| P1 | p95 latency ≤ 4000 ms, error rate ≤ 5 %, ≥ 20 successful calls |
| P2 | cross-provider agreement ≥ 95 %, supply delta ≤ 0.01 % |
| P3 | ≤ 60 calls, ≤ 240 s, ≤ 3 candidates; `COMPLETED_WITH_GAPS` ≠ PASS |
| P4 | ≥ 30 days, missing candles ≤ 2 % |
| P5 | ≥ 100 trades, ≥ 3 folds, zero train/test overlap |
| P6 | ≥ 30 calendar days, no backfill |
| P7 | ≥ 72 h soak, memory growth ≤ 256 MB |

## 8. Reproduction

```bash
# 1. Verify the archive before extracting anything
sha256sum meme-token-skill-v1.1.0.zip
# expect 07e506260e11ad0d4f9346f39cd0dbce47f6f56e87953c92d6a5d8dcccd10bd8

# 2. Extract into a clean workspace
mkdir -p validation_work && cd validation_work
unzip -o ../meme-token-skill-v1.1.0.zip

# 3. Baseline project tests
cd meme-token-skill
python -m unittest discover -s tests -v
python scripts/validate_skill.py

# 4. Validation harness tests (redaction, read-only guard, provenance)
python -m unittest discover -s validation/tests -v

# 5. Confirm the locked config hash has not drifted
python -c "import sys; sys.path.insert(0,'validation/bin'); import provenance as p; print(p.config_version())"
```

## 9. Pass / fail summary

| P0 criterion | Status |
|---|---|
| Baseline tests executed and archived | PASS |
| Archive hash, Python version, OS, UTC time recorded | PASS |
| Network call inventory complete | PASS |
| No signing/sending code, static guard in place | PASS |
| Thresholds locked before data collection | PASS |
| `redact_secrets()` implemented and tested | PASS |
| run_id, UTC, code/config version, synthetic flag in all reports | PASS |
| No secret, private key, or synthetic-as-live data in the repo | PASS |

**P0 verdict: PASS.** P1 (live connectivity) is now unblocked but has not been run.

## 10. What P0 did not prove

- Nothing about live Solana or DEX Screener reachability — no network call was made.
- Nothing about provider rate limits or quotas; those are client-side guesses only.
- Nothing about 3.11/3.12 specifically; only 3.14.2 was exercised.
- Nothing about data correctness; that is P2.

## 11. Exact next commands for P1

```bash
cd validation_work/meme-token-skill
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"   # smoke only

python validation/bin/connectivity.py \
  --config validation/config.example.json \
  --out validation/reports/p1-connectivity
```

`connectivity.py` is the P1 entry point and is not yet implemented; P0 ends here
by design.