# P3 Failure Injection

Offline deterministic validation using injected Phase 3 fixture transports. No live network, transaction, signing, wallet, or provider-write action was used.

## Result: 16/16 detected

`COMPLETED_WITH_GAPS` is explicitly **NOT** a quality PASS.

| # | Fault | Runtime status | Quality | Metrics | Detection |
|---:|---|---|---|---|---|
| 1 | All providers down | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=3, error=InjectedProviderFault, duration=0.00038s | all provider evidence unavailable |
| 2 | RPC down; DEX available | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=3, error=InjectedProviderFault, duration=0.000277s | DEX retained while RPC is UNKNOWN |
| 3 | DEX down; RPC available | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=5, error=InjectedProviderFault, duration=0.000326s | RPC retained while DEX is UNKNOWN |
| 4 | HTTP 429 with Retry-After | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=5, error=InjectedProviderFault, duration=0.000342s | transport/protocol error retained as unavailable |
| 5 | HTTP 500 followed by timeout | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=5, error=InjectedProviderFault, duration=0.000257s | transport/protocol error retained as unavailable |
| 6 | Invalid JSON and changed response shape | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=5, error=InjectedProviderFault, duration=0.00031s | transport/protocol error retained as unavailable |
| 7 | Invalid/non-SPL/unknown mint program | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=5, error=InjectedProviderFault, duration=0.000431s | unknown token program blocks identity |
| 8 | Token-2022 unsupported extension | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=5, error=SemanticValidation, duration=0.00049s | unsupported extension review flag |
| 9 | Largest accounts work; owner mapping fails | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=5, error=InjectedProviderFault, duration=0.000449s | account evidence retained; owners not imputed |
| 10 | Reversed base/quote and base-only price | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=5, error=SemanticValidation, duration=0.00074s | quote-side target price rejected |
| 11 | Zero/null/stale/future pool snapshot | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=5, error=SemanticValidation, duration=0.000561s | null liquidity and future timestamp preserved, not promoted to quality PASS |
| 12 | Disk full / SQLite locked / interruption before checkpoint | `PARTIAL_STORAGE_FAILURE` | `PASS` | candidates=1, processed=0, failed=1, requests=0, error=StorageError, duration=1.2e-05s | fault retained and asserted |
| 13 | max-calls reached mid-batch | `PARTIAL_CALL_BUDGET` | `PASS` | candidates=1, processed=1, failed=1, requests=2, error=BudgetExceeded, duration=0.000142s | fault retained and asserted |
| 14 | Resume partial run without duplicate or whitening | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=0, error=ProviderError, duration=0.058223s | fault retained and asserted |
| 15 | Active mint/freeze authority review flag | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=5, error=SemanticValidation, duration=0.000617s | active authorities require review |
| 16 | Valid JSON with contradictory semantics | `COMPLETED_WITH_GAPS` | `NOT_PASS` | candidates=1, processed=1, failed=1, requests=5, error=SemanticValidation, duration=0.000455s | cross-field supply contradiction detected |

## Resume and evidence invariants

- Existing failed records are preserved byte-for-semantics and are not retried/whitened.
- Record identity is keyed by run and mint; resume adds only outstanding candidates.
- Provider failure cannot produce a buy recommendation; all injected results record `is_buy_signal=false` where applicable.
- Full machine-readable evidence: `failure_injection_evidence.json`.
