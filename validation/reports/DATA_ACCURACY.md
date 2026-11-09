# DATA_ACCURACY

> **Verdict: BLOCKED — P2 is not a PASS.**
>
> The real sampling gate completed with 100 distinct mints and 25 items in each
> stratum. DEX Screener evidence succeeded for all 100 rows. During the accepted
> batch window, the public Solana RPC returned HTTP 403 for `getAccountInfo`,
> `getTokenSupply`, and `getTokenLargestAccounts` on all 100 rows. Therefore all
> on-chain security, supply, and holder fields remain `UNKNOWN` / `DATA_TIDAK_CUKUP`.
> A dedicated RPC endpoint is required before P2 can be reassessed.

## Run provenance
- run_id: `p2-20261109T204215Z-203e0a`
- observed_at_utc: `2026-11-09T20:44:19Z`
- synthetic: `false`
- code_version: `meme-token-skill@1.1.0+build=unknown`
- denominator: 100 (collection failures retained: 0)

## Sampling frame
- required strata: new_discovery, active_liquid, low_liquidity, risky_delisted_migrated
- counts: `{"active_liquid": 25, "low_liquidity": 25, "new_discovery": 25, "risky_delisted_migrated": 25}`
- minimum_100: True
- each_stratum_minimum_25: True

## Accuracy summary
- rows_with_discrepancy_or_insufficient_data: 100
- rows_with_unknown_verifiability: 100
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
| 1 | 22JKmY9YZfnQfb3pGYb6hctL5Jyvv3zhL3nQ8oyNuoYp | BFVfdYwBQxgReXaKfdSxohSP1GpXcEa1emRQuLe8w52h | TRUE | TRUE | UNKNOWN | AUTHORITY_DATA_MISSING|DATA_TIDAK_CUKUP |
| 2 | 2j5SaS7xy776qCBpyPQbZjyQSAtKiFgrwjfErthnW2ZM | Fe7bpDYCs74GqiCUF14HwgUMToPKV1d9mDYVX5BTohFr | TRUE | TRUE | UNKNOWN | AUTHORITY_DATA_MISSING|DATA_TIDAK_CUKUP |
| 3 | 2zMMhcVQEXDtdE6vsFS7S7D5oUodfJHE8vd1gnBouauv | FAqh648xeeaTqL7du49sztp9nfj5PjRQrfvaMccyd9cz | TRUE | TRUE | UNKNOWN | AUTHORITY_DATA_MISSING|DATA_TIDAK_CUKUP |
| 4 | 33jENUQnjoitkMFpvNJcSHyLAHFWJuHwT6E3E8CNpump | 6iep3EgqksbZwgzMBCyAJtiVtVYkCgL9BnPBEwLR8AGQ | TRUE | TRUE | UNKNOWN | AUTHORITY_DATA_MISSING|DATA_TIDAK_CUKUP |
| 5 | 4k3Dyjzvzp8eMZWUXbBCjEvwSkkk59S5iCNLY3QrkX6R | 6UmmUiYoBjSrhakAobJw8BvkmJtDVxaeBtbt7rxWo1mg | TRUE | TRUE | UNKNOWN | AUTHORITY_DATA_MISSING|DATA_TIDAK_CUKUP |
| 6 | 5UUH9RTDiSpq6HKS6bp4NdU9PNJpXRXuiw6ShBTBhgH2 | 4w2cysotX6czaUGmmWg13hDpY4QEMG2CzeKYEQyK9Ama | TRUE | TRUE | UNKNOWN | AUTHORITY_DATA_MISSING|DATA_TIDAK_CUKUP |
| 7 | 5tCju6YNxHq5zrA6tGndr6F7TK42mpUFmeE31cSFpump | 8ewuF2o8ACro7fqWepZ3tKhkbBjndZU4Ldk9scRPw83d | TRUE | TRUE | UNKNOWN | AUTHORITY_DATA_MISSING|DATA_TIDAK_CUKUP |
| 8 | 6GmAFSYs4gk3FDao5FzzySQpPZaWsa4rUJHacpMpUNgx | zxTpi4BtaWX3mgdAPoezkMD1hxx8CdeCfrqXMWvSCLX | TRUE | TRUE | UNKNOWN | AUTHORITY_DATA_MISSING|DATA_TIDAK_CUKUP |
| 9 | 6p6xgHyF7AeE6TZkSmFsko444wqoP15icUSqi2jfGiPN | 3C5YE97HADPDxZehYq9Cis8AXr9aNyrUsczKzE1nDbW9 | TRUE | TRUE | UNKNOWN | AUTHORITY_DATA_MISSING|DATA_TIDAK_CUKUP |
| 10 | 7vfCXTUXx5WJV5JADk17DUJ4ksgau7utNKj4b963voxs | HktfL7iwGKT5QHjywQkcDnZXScoh811k7akrMZJkCcEF | TRUE | TRUE | UNKNOWN | AUTHORITY_DATA_MISSING|DATA_TIDAK_CUKUP |

## Interpretation constraints
DEX Screener `priceUsd` is accepted only for an exact Solana pair whose base-token mint equals the sampled mint. Largest accounts remain token accounts; owner and beneficial-owner concentration are separate and UNKNOWN unless independently proven. Null authority alone never establishes safety.
