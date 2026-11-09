# P1 — LIVE API CONNECTIVITY ACCEPTANCE

| Field | Value |
|---|---|
| Stage | P1 — live connectivity acceptance |
| First run (UTC) | 2026-11-08T13:20:38Z → run `p1-20261108T132038Z-815cb5` — **FAIL** |
| Corrected run (UTC) | 2026-11-08T13:21:24Z → run `p1-20261108T132124Z-720d57` — **PASS** |
| Code under test | `meme-token-skill@1.1.0+build=2950b1126c3dfe4e` |
| Threshold config | `thresholds-v1.0.0` (locked in P0, sha256 `34a6139...d209`, **never edited**) |
| RPC endpoint | `https://api.mainnet-beta.solana.com` (public default) |
| Status | **PASS** (with a documented caveat — see §4) |

## 1. Verdict

| Check | Threshold | Run 1 | Run 2 | Result |
|---|---|---|---|---|
| min_successful_calls | ≥ 20 | 15 probes total | **24 successful** | PASS |
| max_error_rate | ≤ 5 % | 6.67 % | **4.00 %** | PASS |
| max_p95_latency_ms | ≤ 4000 ms | 107.2 ms | **104.5 ms** | PASS |

Run 1 failed. Run 2 passed. **The thresholds were not touched between runs.**
What changed is documented in §3, and it matters that the change was made to the
harness, not to the acceptance criteria.

## 2. What was measured (run 2, the accepted run)

25 probes across two providers:

| Provider | Probes | OK | Failed |
|---|---|---|---|
| Solana RPC (public mainnet-beta) | 13 | 12 | 1 |
| DEX Screener | 12 | 12 | 0 |

Latency: p50 = 79.1 ms, p95 = 104.5 ms. Every successful call returned parseable
JSON with a recorded `payload_sha256`.

The single failure is identical in both runs and is the finding that matters:

> `getTokenLargestAccounts` → **HTTP 429 after 3 attempts**, on
> `api.mainnet-beta.solana.com`.

## 3. Why run 1 failed, and why the fix was legitimate

### 3.1 The harness bug

Run 1 issued 15 probes. The locked config requires ≥ 20 successful calls. My
`--repeats` default of 6 produced too small a sample for the threshold to be
meaningful, let alone satisfiable. That is a defect in **my probe plan**, written
after the threshold was already locked.

Two ways to "fix" it existed:

| Option | Verdict |
|---|---|
| Lower `min_successful_calls` to 15 | **Rejected.** Retuning an acceptance criterion after seeing results is exactly the dishonesty the runbook forbids. |
| Raise the probe count to meet the pre-registered minimum | **Chosen.** The threshold says "at least 20 calls"; the harness now makes 25. The criterion is unchanged. |

`--repeats` default went from 6 → 11 (13 Solana + 12 DEX = 25 total). The
threshold file's sha256 is still `34a6139...d209`, byte-identical to P0.

### 3.2 The 429 is not a harness bug

`getTokenLargestAccounts` failed in run 1 and failed again in run 2. Raising the
sample size did not "fix" it — it moved the error rate under the 5 % ceiling
because the denominator grew. **That is a borderline pass, and it should be read
as one.** At 24/25 the error rate is 4.00 %; a second 429 in the same run would
have produced 7.7 % and failed again. The public endpoint is at its limit.

## 4. Caveat that must travel with this PASS

This P1 PASS **does not** establish production capacity.

- The public RPC returned 429 on one specific method under a 25-call burst.
- Success depends on where the shared node's quota happened to sit that second.
- Repeating this run at a different time may fail again.
- A dedicated endpoint (Helius / QuickNode / Triton) has **not** been exercised.

What the PASS does establish: reachability, correct latency, correct retry
behaviour, clean provenance on failure, and no secret leakage.

What it does not establish: headroom. P7 (72 h soak) is where capacity gets
settled, and it should not be run against a public endpoint.

## 5. Evidence the run produced

| Claim | Evidence |
|---|---|
| Solana Mainnet reachable | 12/13 RPC calls returned real results in 63–107 ms |
| DEX Screener reachable | 12/12 probes succeeded on both endpoints |
| 429 handled without crashing | Degraded to a structured record; run continued to completion |
| Provenance preserved on failure | Failed probe still carries `run_id`, UTC timestamp, redacted endpoint, status 429, latency |
| Latency acceptable | p95 = 104.5 ms, 38× under the 4000 ms ceiling |
| No secret leaked | Report passed `assert_no_secrets()` before write; endpoint stored with no key |

## 6. Negative controls

Three deliberate failure probes, all recorded rather than raised:

| Control | Endpoint | Result |
|---|---|---|
| DNS / connection failure | `https://this-host-does-not-exist.invalid/rpc` | `network_error`, retries exhausted, structured |
| Timeout path | DEX Screener with `timeout=0.001` | timeout recorded, no exception escaped |
| Insecure scheme | `http://api.mainnet-beta.solana.com` | `insecure_endpoint`, rejected **before any socket opened** |

The third control is the only `http://` literal in the pipeline, and the static
guard asserts it exists — deleting it fails the suite, because without it there
is no evidence that plaintext is refused.

## 7. What P1 does not prove

- Nothing about a premium RPC endpoint. Its rate limits are unknown to this report.
- Nothing about sustained load. 25 probes over seconds is not a soak test; that is P7.
- Nothing about data accuracy. A reachable endpoint returning wrong data passes P1 and fails P2.
- Nothing about Python 3.11/3.12.
- Nothing about DEX Screener under concurrency.

## 8. Reproduction

```bash
cd validation_work/meme-token-skill

# Public endpoint (what both runs used)
python validation/bin/connectivity.py \
  --config validation/config.example.json \
  --out validation/reports/p1-connectivity

# Dedicated endpoint
export SOLANA_RPC_URL="https://YOUR-RPC-HOST/?api-key=YOUR_KEY"
python validation/bin/connectivity.py --out validation/reports/p1-connectivity
```

Exit code 0 only when every locked threshold is met.

## 9. Evidence artifacts

| Artifact | Path |
|---|---|
| Run 1 report (FAIL) | `validation/reports/p1-connectivity/p1_connectivity_p1-20261108T132038Z-815cb5.json` |
| Run 2 report (PASS) | `validation/reports/p1-connectivity/p1_connectivity_p1-20261108T132124Z-720d57.json` |
| Append-only manifest | `validation/manifests/p1_evidence.jsonl` |
| Probe implementation | `validation/bin/connectivity.py` |

Both runs are kept on disk. The failed run is not deleted; deleting it would
erase the evidence that the public endpoint rate-limits.
