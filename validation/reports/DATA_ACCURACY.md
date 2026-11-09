# DATA_ACCURACY

> Status: **PENDING_LIVE_AUDIT / BLOCKED_BY_SAMPLING_SHORTFALL**
>
> This report is synthetic regression evidence only. It is not a P2 PASS and must
> not be interpreted as a completed audit of 100 real tokens. The latest live
> sampling attempt is recorded in `p2_sampling_shortfall.json`.

## Run provenance
- run_id: `p2-20261109T185531Z-bfd042`
- observed_at_utc: `2026-11-09T18:55:31Z`
- synthetic: `true`
- code_version: `meme-token-skill@1.1.0+build=unknown`
- denominator: 100 (collection failures retained: 1)

## Sampling frame
- required strata: new_discovery, active_liquid, low_liquidity, risky_delisted_migrated
- counts: `{"active_liquid": 25, "low_liquidity": 25, "new_discovery": 25, "risky_delisted_migrated": 25}`
- minimum_100: True
- each_stratum_minimum_25: True

## Accuracy summary
- rows_with_discrepancy_or_insufficient_data: 1
- rows_with_unknown_verifiability: 1
- UNKNOWN and DATA_TIDAK_CUKUP are never interpreted as safe or valid.

## Confusion / error taxonomy
| Code | Meaning | Treatment |
|---|---|---|
| INVALID_MINT | Not base58 32-byte mint | Invalid |
| WRONG_TOKEN_PROGRAM | Account owner is not SPL/Token-2022 | Invalid |
| CHAIN_MISMATCH | DEX chain is not Solana | Invalid |
| TARGET_NOT_DEX_BASE | Target is quote/other token | Reject priceUsd and mint match |
| PRICE_MISSING | Base-token USD price absent | UNKNOWN, not zero |
| HOLDER_SEMANTICS_INVALID | Token accounts represented as unique owners | Invalid |
| AUTHORITY_DATA_MISSING | Mint/freeze authority was not observed (null values are valid observations) | UNKNOWN, never safe |
| STALE | Observation exceeds freshness limit | Review |
| COLLECTION_FAILED | Provider request failed | Retain in denominator |
| DATA_TIDAK_CUKUP | Critical field missing | UNKNOWN, never safe |

## Ten manual audit examples
| # | mint | pair | mint | price basis | holders | discrepancy |
|---:|---|---|---|---|---|---|
| 1 | So11111111111111111111111111111111111111112 | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | AUTHORITY_DATA_MISSING|DATA_TIDAK_CUKUP|COLLECTION_FAILED |
| 2 | So11111111111111111111111111111111111111112 | 9wFFmGph15h76UzGCrErN5nG3EqUr6xR3LLuW2hQ2Vj9 | TRUE | TRUE | TRUE | NONE |
| 3 | So11111111111111111111111111111111111111112 | 9wFFmGph15h76UzGCrErN5nG3EqUr6xR3LLuW2hQ2Vj9 | TRUE | TRUE | TRUE | NONE |
| 4 | So11111111111111111111111111111111111111112 | 9wFFmGph15h76UzGCrErN5nG3EqUr6xR3LLuW2hQ2Vj9 | TRUE | TRUE | TRUE | NONE |
| 5 | So11111111111111111111111111111111111111112 | 9wFFmGph15h76UzGCrErN5nG3EqUr6xR3LLuW2hQ2Vj9 | TRUE | TRUE | TRUE | NONE |
| 6 | So11111111111111111111111111111111111111112 | 9wFFmGph15h76UzGCrErN5nG3EqUr6xR3LLuW2hQ2Vj9 | TRUE | TRUE | TRUE | NONE |
| 7 | So11111111111111111111111111111111111111112 | 9wFFmGph15h76UzGCrErN5nG3EqUr6xR3LLuW2hQ2Vj9 | TRUE | TRUE | TRUE | NONE |
| 8 | So11111111111111111111111111111111111111112 | 9wFFmGph15h76UzGCrErN5nG3EqUr6xR3LLuW2hQ2Vj9 | TRUE | TRUE | TRUE | NONE |
| 9 | So11111111111111111111111111111111111111112 | 9wFFmGph15h76UzGCrErN5nG3EqUr6xR3LLuW2hQ2Vj9 | TRUE | TRUE | TRUE | NONE |
| 10 | So11111111111111111111111111111111111111112 | 9wFFmGph15h76UzGCrErN5nG3EqUr6xR3LLuW2hQ2Vj9 | TRUE | TRUE | TRUE | NONE |

## Interpretation constraints
DEX Screener `priceUsd` is accepted only for an exact Solana pair whose base-token mint equals the sampled mint. Largest accounts remain token accounts; owner and beneficial-owner concentration are separate and UNKNOWN unless independently proven. Null authority alone never establishes safety.
