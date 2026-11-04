# Crypto Station → Personal AI Meme Trading Bot — Pivot Roadmap & Living Plan

> Status: v0.2 — IMPLEMENTATION IN PROGRESS; P0.1 DONE, P0.2 NEXT
> Owner: user/owner Telegram `7254686681` + Hermes
> Last updated: 2026-11-04
> Pivot baseline: `503f5fb`; current implementation head: `385ad9e`
> Baseline database: workspace v14
> Skill baseline audited read-only: `meme-token-skill-v1.1.0.zip`
> Skill SHA-256: `07e506260e11ad0d4f9346f39cd0dbce47f6f56e87953c92d6a5d8dcccd10bd8`
> Runtime policy: coding + Git lokal; generate/test/build/runtime di `cbt-vps`
> Safety: belum ada real wallet signing, real broker order, atau auto-trading yang disetujui

Dokumen ini adalah **approved design direction untuk pivot** setelah user mengikuti rekomendasi audit skill. `CRYPTO_STATION_CURRENT_STATE.md` tetap menjadi sumber fakta implementasi legacy sampai Task P0.2 mengonsolidasikan statusnya; dokumen ini menjadi sumber desain target dan urutan implementasi. ACK v0.2 tidak mengizinkan real execution.

---

## Daftar Isi

0. Locked Decisions
1. Goal & Product Boundary
2. Target User & Control Model
3. Legacy Disposition
4. Skill v1.1.0 Audit & Runtime Contract
5. Target Architecture
6. Core Features
7. End-to-End Flows
8. AI Provider Setup
9. AI Decision Contract
10. Deterministic Risk Engine
11. Venue & Market Adapters
12. Data Model
13. Internal Interfaces
14. Telegram Screens
15. Security & Secret Management
16. Reliability, Idempotency & Reconciliation
17. Evaluation & Promotion Gates
18. Migration Phases
19. Testing Strategy
20. Deploy & Rollback
21. Risks & Non-goals
22. Open Decisions
23. Acceptance Criteria
24. Task-by-Task P0–P2
25. Current Next Step

---

## 0. Locked Decisions (v0.2)

| # | Keputusan | Pilihan |
|---|---|---|
| 1 | Produk | Bot trading meme token pribadi, bukan exchange publik |
| 2 | Akses | Owner-only, Telegram ID `7254686681`; deny-by-default |
| 3 | Chain target | Solana dahulu; BSC expansion; Robinhood Chain discovery sampai identitas teknis terverifikasi |
| 4 | Robinhood Chain | Klaim chain dari materi skill, bukan broker/Robinhood Crypto; `UNVERIFIED_DISCOVERY`, bukan execution venue |
| 5 | Peran AI | Menafsirkan evidence dan membuat proposal; bukan sumber evidence, gate, atau izin eksekusi |
| 6 | Strategy source | Skill buatan user, versioned dan terus diperbarui; bukan strategi hardcoded di core bot |
| 7 | Skill baseline | `meme-token-skill-v1.1.0.zip`, SHA-256 tercatat di header |
| 8 | AI configuration | Endpoint, API key, dan model dapat disetel melalui Telegram seperti provider setup Hermes |
| 9 | AI compatibility | OpenAI-compatible default; provider non-compatible lewat adapter |
| 10 | Execution progression | Research → Offline Simulation → Prospective Paper → Advisory → Confirm → Auto |
| 11 | Risk authority | Deterministic risk engine selalu lebih berkuasa daripada keputusan AI |
| 12 | Wallet | Dedicated hot wallet per chain/venue; bukan wallet utama owner |
| 13 | Asset identity | Chain/venue + contract/mint/instrument ID; ticker tidak cukup |
| 14 | Provider boundary | Semua market data, AI, DEX, RPC, dan broker melalui interface/adapter |
| 15 | Secrets | Tidak boleh masuk Git, log, prompt history, audit payload, atau pesan balasan Telegram |
| 16 | Auditability | Snapshot data, skill version/hash, prompt contract, AI output, risk verdict, quote, approval, order, fill, dan reconcile wajib tertaut |
| 17 | AI output | JSON schema ketat; invalid/ambiguous output = reject, bukan diperbaiki diam-diam untuk trading |
| 18 | Legacy | Public user/IDR/QRIS/payout/admin-stock roadmap dibekukan; tidak dikembangkan selama pivot |
| 19 | Live gate | Real signing/order dan Auto memerlukan approval eksplisit terpisah setelah paper/shadow gates |
| 20 | Failure default | Stale data, provider down, skill invalid, AI timeout, schema invalid, risk unknown, atau reconcile mismatch = NO TRADE |
| 21 | Skill authority | v1.1.0 immutable research/safety kernel; semantics evidence, G0–G5, `UNKNOWN`, dan `NO TRADE` dipertahankan |
| 22 | Current skill scope | Research-only + offline simulation; bukan live signal/profitable strategy |
| 23 | Execution prerequisite | G0, G1, G2 resolved deterministically; `UNKNOWN` memblokir real execution |
| 24 | Phase 3 score | Inspectability bukan confidence profit, safety score, atau buy score |
| 25 | AI rollout | Evidence/schema compatibility dahulu; AI explanation/proposal setelahnya |
| 26 | Python runtime | Prompt/schema integration dahulu; script hanya isolated runner tanpa wallet key dan network default |
| 27 | Database pivot | Database trading baru; legacy DB v14 read-only/archive |
| 28 | AI topology | Satu primary + optional shadow evaluator; tanpa model voting MVP |
| 29 | Base currency | USD/USDC canonical; IDR display opsional |
| 30 | Signing | Service terisolasi; env key bukan default real funds |
| 31 | Legacy runtime | Sandbox legacy boleh hidup tetapi frozen sampai owner shell P1 menggantikannya |
| 32 | Reproducibility | Run mencatat skill version/hash, contract/config hash, dan evidence IDs |

---

## 1. Goal & Product Boundary

Membangun owner-only trading orchestrator yang:

1. menemukan atau menerima kandidat meme token/instrument;
2. mengumpulkan market/security/evidence snapshot;
3. menjalankan skill versi aktif melalui AI provider pilihan owner;
4. menghasilkan `BUY`, `SELL`, `HOLD`, `SKIP`, atau `NO_TRADE` dalam JSON terstruktur;
5. menerapkan hard risk limits deterministik;
6. melakukan paper trade, meminta konfirmasi, atau mengeksekusi otomatis sesuai mode;
7. memantau order/fill/position/PnL dan melakukan reconciliation;
8. memberi kontrol penuh melalui Telegram.

Bot **bukan**:

- exchange untuk user umum;
- custody service milik pihak lain;
- jaminan profit;
- AI bebas yang boleh mengeksekusi tool apa pun;
- sistem yang mempercayai ticker, social hype, atau output LLM tanpa evidence/risk checks;
- high-frequency trading engine;
- copy-trading atau market-manipulation tool.

---

## 2. Target User & Control Model

### 2.1 Role

Hanya satu role pada MVP: `OWNER`.

Owner dapat:

- mengelola AI provider dan skill package;
- memilih execution mode;
- mengaktifkan/menonaktifkan venue;
- mengatur risk limits;
- menambah candidate/watchlist;
- meminta analisis;
- confirm/reject trade proposal;
- pause venue atau global kill switch;
- melihat positions, PnL, orders, evidence, dan health.

### 2.2 Authorization

- Private chat only.
- Telegram allowlist only.
- Semua callback divalidasi sebelum dereference/I/O/dispatch.
- Setting sensitif dan perubahan mode membutuhkan preview + confirm.
- Aktivasi `AUTO` membutuhkan re-confirmation dengan challenge singkat dan cooldown.
- Bot command lama untuk user biasa dinonaktifkan setelah cutover.

---

## 3. Legacy Disposition

### 3.1 Keep dan adaptasi

| Legacy capability | Target use |
|---|---|
| Owner/admin allowlist | Owner-only authorization |
| Callback structural guard | Semua control callback |
| PostgreSQL + pgx/sqlc | State, audit, decision, order, position |
| Decimal-safe math | Price, quantity, fee, PnL |
| Idempotency | Decision, proposal, approval, submission, fill |
| Durable claim/recovery worker | Analysis jobs, order jobs, reconciliation |
| Audit log | Config, skill, AI, risk, execution lifecycle |
| Pause mechanism | Global/venue/asset/reasoned pause |
| Telegram inline preview/confirm | Setting, trade proposal, mode promotion |
| Build/deploy runbook | Workspace workflow |

### 3.2 Modify heavily

| Legacy | Pivot |
|---|---|
| Price service BNB/ETH/SOL | Multi-venue market-data adapters |
| Payout lifecycle | Order/submission/fill lifecycle |
| Admin wallet | Portfolio/positions/wallet health |
| Trial metrics | Paper/shadow evaluation metrics |
| Ledger | Trading cash/asset journal or position accounting; design finalized before migration |
| `/admin` | Owner control center |

### 3.3 Retire/freeze

- Public registration and public-user keyboard.
- IDR user balances.
- QRIS/top-up/bank/manual payment roadmap.
- Buy/sell service for public users.
- User wallet-address and payout-to-user flows.
- User coin deposit flow.
- BNB/ETH/SOL fixed catalog.
- Admin stock deposit/float UX.
- Broadcast to users.
- Stage 0 gateway economics.
- Historical CF-08 adjustment: park; do not mutate until legacy data retention/migration policy is approved.

### 3.4 Migration rule

Legacy tables are not dropped in the first pivot phases. Mark/read-only first, export backup, prove new system independently, then decide archive/drop in a dedicated migration with rollback.

---

## 4. Skill v1.1.0 Audit & Runtime Contract

### 4.1 Read-only audit result

ZIP contains:

- `SKILL.md`, README, changelog, build information;
- Solana-oriented research connectors and G0–G5 gates;
- bounded deterministic research agent;
- offline backtest/walk-forward/paper replay;
- schemas, tests, references, templates, and Python scripts.

Important boundaries from the skill itself:

- v1.1.0 is a research and offline-simulation package, **not an execution strategy proven on live data**.
- Built-in examples are synthetic.
- Live connectors have not passed complete public-network acceptance.
- Phase 3 ranks research inspectability, not profit probability.
- Phase 4 is single-token spot-long simulation, not portfolio allocation or real-time broker paper trading.
- `NO TRADE` and `DATA_TIDAK_CUKUP` are valid outcomes.
- Skill scripts were inspected/listed but not executed during roadmap creation.

### 4.2 Skill package lifecycle

`upload/import → quarantine → safe extract → manifest scan → checksum → static policy scan → schema validation → offline tests → shadow evaluation → activate`

Requirements:

- Reject zip-slip paths, symlinks, oversized archives, nested archive bombs, executables outside allowlist, and missing `SKILL.md`.
- Record package SHA-256, semantic version, uploaded time, source filename, compatibility version, and validation report.
- Never execute package scripts during upload/import.
- Execution runs in isolated worker/container with no wallet keys and deny-by-default network/filesystem.
- Active skill is immutable; update creates a new version.
- Rollback switches active version pointer; never edits historical decision records.
- Keep exact skill version/hash on every analysis and trade.

### 4.3 Skill runtime contract

Core bot supplies a bounded context, not arbitrary tool access:

- normalized market snapshot;
- token/instrument identity;
- security/evidence facts with source + observed timestamps;
- portfolio and risk context;
- execution mode;
- skill version/hash;
- strict output schema.

The skill may request additional approved evidence through a capability allowlist, but cannot:

- access wallet keys;
- submit orders;
- alter risk limits;
- enable venues or Auto mode;
- read AI/API credentials;
- issue raw shell commands in the trading service.

### 4.4 Compatibility

Add an internal `strategy_contract_version`. Skill activation fails if its declared contract is unsupported. A migration/adapter may translate old research output into the current decision schema, but must remain deterministic and tested.

---

## 5. Target Architecture

```text
Telegram Owner UI
       |
Owner Command/Callback Guard
       |
Control Service ─────────────── Config & Secret Vault
       |
Scheduler / Candidate Intake
       |
Market & Evidence Collector
  | Solana | BSC | Robinhood | Social/Research sources
       |
Immutable Market Snapshot
       |
Skill Runtime + AI Provider Adapter
       |
Validated AI Decision JSON
       |
Deterministic Risk Engine
       |
Trade Proposal / Approval Gate
       |
Execution Orchestrator
  | Jupiter/Solana | BSC Router/Aggregator | Robinhood Adapter
       |
Order/Tx Monitor + Reconciliation
       |
Portfolio / Position / PnL + Alerts
       |
PostgreSQL Audit & Evidence Store
```

### 5.1 Separation of authority

- AI proposes.
- Risk engine permits/rejects/caps.
- Execution adapter submits only a signed, persisted, idempotent intent.
- Owner mode determines whether approval is required.
- Reconciler determines authoritative final state from venue/chain evidence.

### 5.2 Process split target

Initial deployment may remain one Go binary plus isolated skill runner, but package boundaries must support later separation:

- `control`: Telegram/config/modes;
- `research`: snapshots/skill/AI;
- `risk`: deterministic policies;
- `execution`: intents/adapters/reconciliation;
- `portfolio`: positions/PnL;
- `worker`: durable jobs.

---

## 6. Core Features

### 6.1 Owner dashboard

- Mode badge: PAPER / ADVISORY / CONFIRM / AUTO.
- Global pause and kill switch.
- Venue health and enabled state.
- AI provider/model health.
- Active skill version/hash.
- Portfolio equity, exposure, daily PnL/drawdown.
- Pending proposals/orders/reconciliation alerts.

### 6.2 Candidate/watchlist

- Add by Solana mint, BSC contract, or Robinhood instrument identifier.
- Discovery adapters may suggest candidates but cannot auto-trade by discovery alone.
- Deduplicate by canonical venue + asset ID.
- Allowlist, denylist, notes, tags, and strategy assignment.

### 6.3 Analysis

- Bounded on-demand and scheduled runs.
- Evidence freshness and provenance.
- G0–G5-compatible research statuses.
- AI structured decision.
- Human-readable summary generated from validated JSON.

### 6.4 Trading

- Quote preview.
- Risk verdict and capped size.
- Confirm/reject in Confirm mode.
- Paper execution with conservative fill assumptions.
- Real execution only after gate.
- Order/transaction state monitoring.
- Partial fill, failed, expired, unknown, replaced, and reconciliation states.

### 6.5 Portfolio

- Venue-separated balances.
- Open/closed positions.
- Average entry, realized/unrealized PnL.
- Fees, gas, slippage, price impact.
- TP/SL rules and alerts.
- No assumption that planned stop can always fill.

---

## 7. End-to-End Flows

### 7.1 AI provider setup

`AI Settings → Add Provider → Name → Endpoint → API Key → Model → Test → Preview → Confirm → Activate`

Expected behavior:

- key input message is deleted where Telegram allows;
- persisted encrypted;
- displayed masked;
- test validates auth, model, latency, and structured-output support;
- failed test does not activate provider.

### 7.2 Skill update

`Skill → Import ZIP → checksum/metadata → quarantine validation → offline tests → shadow comparison → Preview → Activate`

Activation never changes historical runs. Rollback is one confirmed action.

### 7.3 Analyze candidate

`candidate → identity validation → snapshot collection → freshness gate → skill context → AI → schema validation → risk precheck → decision record → owner notification`

### 7.4 Confirm-mode trade

`AI BUY/SELL → risk PASS/CAPPED → executable quote → proposal persisted → owner Confirm → quote refresh → risk recheck → intent persisted → submit → monitor → reconcile → position/PnL update`

A stale proposal cannot execute. Confirm uses proposal ID + immutable digest; changed size/quote/asset requires a new proposal.

### 7.5 Auto-mode trade

Same path as Confirm, except owner approval is replaced by a valid Auto policy lease. AI never calls adapters directly. Before submission, all of these must still pass:

- active mode lease;
- fresh data and quote;
- active skill/provider versions unchanged;
- deterministic risk;
- venue health;
- idempotency and concurrency guard.

### 7.6 Emergency stop

`Kill Switch → immediate stop accepting new intents → cancel cancellable orders → continue monitor/reconcile existing orders → notify owner → require explicit recovery checklist`

Kill switch must not abandon in-flight transactions.

---

## 8. AI Provider Setup

### 8.1 Provider profile

Fields:

- display name;
- adapter type (`openai_compatible` initially);
- base URL;
- model;
- encrypted API key reference;
- timeout;
- max retries for analysis only;
- optional temperature/max output tokens;
- structured-output capability;
- enabled/primary/fallback status;
- last test timestamp/result/latency.

### 8.2 Endpoint rules

- HTTPS required except explicit localhost development.
- Block private/link-local/metadata IPs by default to reduce SSRF.
- Resolve and re-check DNS/IP on connection.
- No arbitrary headers from Telegram except an allowlisted provider config.
- Strict response-size and timeout limits.

### 8.3 Credential handling

Recommended secure path:

1. preferred: create credential via server CLI/env, then select profile in Telegram;
2. supported convenience: send key in private chat, immediately encrypt and delete source message;
3. never show full key again;
4. key rotation creates audit metadata without key content;
5. master encryption key comes only from server environment or secret manager.

### 8.4 Fallback

Fallback AI may be used for analysis availability, but:

- record provider/model actually used;
- validate against the same schema;
- rerun deterministic risk;
- in Confirm/Auto, fallback behavior must be explicitly enabled;
- disagreement between primary/fallback may force `NO_TRADE` according to policy.

---

## 9. AI Decision Contract

### 9.1 Required JSON shape

```json
{
  "schema_version": "trade.decision.v1",
  "decision_id": "uuid",
  "action": "BUY|SELL|HOLD|SKIP|NO_TRADE",
  "venue": "SOLANA|BSC|ROBINHOOD",
  "asset_id": "canonical identifier",
  "snapshot_id": "uuid",
  "skill_version": "1.1.0",
  "skill_sha256": "hex",
  "confidence": "0.00-1.00",
  "suggested_notional_usd": "decimal string",
  "entry_thesis": "short text",
  "invalidation": "short text",
  "stop_loss": {"type": "NONE|PRICE|PERCENT", "value": "decimal|null"},
  "take_profit": [{"price_or_percent": "decimal", "size_percent": "decimal"}],
  "max_holding_seconds": 0,
  "risk_flags": ["string"],
  "missing_evidence": ["string"],
  "reasoning_summary": "bounded text"
}
```

No hidden chain-of-thought is required or stored. Store concise rationale/evidence references only.

### 9.2 Validation

Reject decision when:

- invalid JSON/schema/version;
- venue or asset mismatch;
- snapshot/skill hash mismatch;
- decimal malformed or outside bounds;
- unsupported action;
- stale snapshot;
- missing G0 identity;
- BUY with required evidence unresolved;
- prompt injection markers from external content influence control fields;
- response exceeds limits.

### 9.3 Prompt assembly

Use fixed layers:

1. immutable system safety contract;
2. validated skill instructions;
3. structured market/evidence data treated as untrusted data;
4. portfolio/risk context;
5. exact JSON schema.

External token/social content must never be inserted as executable instructions.

---

## 10. Deterministic Risk Engine

### 10.1 Hard limits

- Max notional per trade.
- Max exposure per asset.
- Max exposure per venue.
- Max total portfolio exposure.
- Daily realized + configured mark-to-market loss limit.
- Drawdown limit.
- Max open positions.
- Max trade frequency/cooldown.
- Max slippage.
- Max price impact.
- Minimum liquidity/depth.
- Maximum participation in observed volume.
- Maximum quote age and snapshot age.
- Gas/fee cap.
- Token security hard rejects.
- Venue-specific market-hours/order-type rules.

### 10.2 Verdict

Risk engine returns:

- `PASS`;
- `PASS_CAPPED` with deterministic reduced size;
- `REJECT` with reason codes;
- `PAUSE_REQUIRED`.

Risk engine cannot be overridden by AI. Owner override, if ever added, requires a separately designed high-risk flow and is not in MVP.

### 10.3 Venue-specific checks

Solana:

- mint/program identity;
- Token-2022 extensions;
- mint/freeze authority status;
- executable buy and sell quote;
- route/liquidity/price impact;
- blockhash/priority fee/transaction simulation.

BSC:

- chain ID and contract bytecode;
- owner/roles/proxy/upgradeability;
- tax/blacklist/pause/mint behavior;
- buy and sell simulation/honeypot indicators;
- allowance strategy and revocation;
- router/liquidity/MEV risk.

Robinhood:

- instrument availability;
- account authorization;
- market hours/order support;
- buying power/position quantity;
- API rate/order restrictions;
- order status/fill semantics.

---

## 11. Venue & Market Adapters

### 11.1 Common contracts

`MarketDataAdapter`:

- ResolveAsset
- GetSnapshot
- GetQuote
- GetBalances
- GetOpenOrders
- GetOrderStatus
- Health

`ExecutionAdapter`:

- ValidateIntent
- Submit
- Cancel
- GetStatus
- Reconcile

`ExecutionAdapter` never receives AI text; only validated deterministic `ExecutionIntent`.

### 11.2 Solana

Recommended initial route: Jupiter adapter + trusted Solana RPC adapter. Exact API/version must be verified at implementation time.

### 11.3 BSC

Provider/router remains open. Candidate options must be evaluated for:

- quote and transaction construction;
- sell simulation;
- route quality;
- API stability/rate limits;
- allowance and MEV protection;
- testnet/mainnet behavior.

### 11.4 Robinhood

Robinhood integration is conditional on an officially supported, account-appropriate API and permitted instruments/actions. Do not automate browser login, scrape credentials, or depend on unofficial reverse-engineered endpoints for real-money execution. If official execution is unavailable, Robinhood remains read-only/advisory or manual handoff.

---

## 12. Data Model

Proposed new tables (names provisional):

### 12.1 Configuration

- `owner_settings`
- `ai_provider_profiles`
- `encrypted_credentials`
- `skill_packages`
- `skill_validation_runs`
- `execution_modes`
- `risk_policies`
- `venue_configs`
- `pause_states`

### 12.2 Research

- `assets`
- `watchlist_items`
- `market_snapshots`
- `evidence_items`
- `analysis_runs`
- `ai_decisions`
- `risk_verdicts`

### 12.3 Execution

- `trade_proposals`
- `execution_intents`
- `orders`
- `order_events`
- `fills`
- `chain_transactions`
- `reconciliation_runs`

### 12.4 Portfolio

- `venue_accounts`
- `asset_balances`
- `positions`
- `position_lots`
- `pnl_snapshots`
- `portfolio_snapshots`

### 12.5 Audit/jobs

- `audit_events`
- `jobs`
- `job_attempts`
- `alerts`

### 12.6 Critical constraints

- unique skill SHA-256;
- unique canonical `(venue, asset_id)`;
- unique analysis idempotency key;
- unique proposal digest;
- unique intent idempotency key;
- unique provider order/tx identifier per venue;
- append-only order/fill/audit events;
- numeric values use decimal/numeric, never float for money/quantity;
- immutable snapshot and AI decision payload hashes;
- encrypted secret blobs separated from normal settings.

### 12.7 Legacy data

Legacy v1–v14 tables remain read-only during early pivot. New migrations start at v15 unless a clean new database is chosen in Open Decision #1.

---

## 13. Internal Interfaces

Suggested Go packages:

```text
cmd/bot
internal/ownerui
internal/config
internal/secrets
internal/skill
internal/ai
internal/research
internal/market
internal/risk
internal/execution
internal/portfolio
internal/reconcile
internal/jobs
internal/audit
internal/venue/solana
internal/venue/bsc
internal/venue/robinhood
```

Skill runner target:

```text
cmd/skill-runner or isolated Python worker
```

Rules:

- Core domain interfaces live outside provider packages.
- Provider SDK types do not leak into domain records.
- All external calls carry timeout, request ID, rate-limit handling, and response provenance.
- AI/skill worker has no signing keys.
- Signing/execution service never receives raw social text or arbitrary prompt instructions.

---

## 14. Telegram Screens

Main owner keyboard after pivot:

```text
[Dashboard] [Analyze]
[Watchlist] [Positions]
[Orders]    [History]
[AI & Skill] [Risk]
[Venues]    [Pause/Kill]
```

### 14.1 AI & Skill

- Provider status.
- Add/Test/Activate/Rotate/Disable provider.
- Active model.
- Import/list/validate/activate/rollback skill.
- Shadow comparison report.

### 14.2 Analyze

- Select venue.
- Paste asset ID or select watchlist.
- Collect snapshot.
- Show evidence freshness.
- Run AI analysis.
- Show decision + risk verdict.
- Create paper/proposal according to mode.

### 14.3 Risk

- Global and venue limits.
- Preview before mutation.
- Explain active rejects/pauses.
- No secret values displayed.

### 14.4 Proposal

Must show:

- venue + canonical asset;
- BUY/SELL;
- AI suggestion and confidence;
- deterministic approved/capped size;
- fresh quote, fee, slippage, price impact;
- risk flags/missing evidence;
- skill/provider/model/snapshot versions;
- expiry;
- Confirm/Reject.

---

## 15. Security & Secret Management

- Dedicated wallets/accounts with minimal capital.
- Separate key per chain/environment.
- Key material outside DB where possible; signing boundary isolated.
- API credentials encrypted at rest with env/secret-manager master key.
- Telegram credential messages deleted where possible.
- Never log request Authorization headers, private keys, seeds, signed raw transactions, or complete AI keys.
- Mask identifiers in normal UI while retaining safe audit references.
- SSRF protection for custom AI endpoint.
- ZIP import hardening and sandboxed skill execution.
- Prompt-injection boundary: external content is data only.
- Dependency pinning and checksum verification.
- Egress allowlist for signing/execution process.
- Backups exclude plaintext secrets; restore drills include encrypted credential handling.
- Real execution requires explicit owner approval and fresh backup/rollback procedure.

---

## 16. Reliability, Idempotency & Reconciliation

### 16.1 Job states

`queued → claimed → running → succeeded | retryable | failed | unknown`

Use atomic claim token + lease expiry. Retry only operations with deterministic idempotency semantics.

### 16.2 Execution states

`proposed → approved → submitting → submitted → partially_filled → filled | cancelled | failed | unknown`

On-chain may use `built → signed → broadcast → confirmed | failed | unknown` as substate.

### 16.3 Crash windows

- Persist intent before external submission.
- Generate stable client/order reference where provider supports it.
- Never interpret timeout as failure without reconcile.
- Unknown state blocks duplicate submission until authoritative lookup.
- Position/PnL updates derive from fills/confirmed evidence, not optimistic AI decision.

### 16.4 Reconciliation

Periodic and on-demand:

- local open orders vs provider;
- balances vs recorded fills;
- chain transaction status;
- positions/lots/PnL consistency;
- stale claimed jobs;
- active mode/skill/provider drift.

Mismatch can pause the affected venue or global execution.

---

## 17. Evaluation & Promotion Gates

### Gate A — Skill structural acceptance

- safe extraction passes;
- metadata/version/hash valid;
- schemas valid;
- offline test suite passes;
- no prohibited capability;
- reproducible report.

### Gate B — Research live acceptance

- Solana and BSC data connectors verified on public networks;
- Robinhood integration status confirmed;
- freshness/provenance correct;
- sellability/security gaps remain explicit;
- no synthetic data mislabeled real.

### Gate C — Historical evaluation

- point-in-time dataset with provenance;
- no lookahead;
- delisted/rug/failed-exit cases included;
- walk-forward/OOS;
- conservative fees/slippage/impact;
- baseline comparison;
- no promotion based only on win rate.

### Gate D — Prospective paper

Minimum criteria must be locked later, but report at least:

- sample size and duration;
- expectancy/net PnL after modeled costs;
- max drawdown;
- reject/no-trade behavior;
- stale/provider failure behavior;
- proposal-to-fill drift;
- strategy/version stability.

### Gate E — Advisory/Confirm

- owner manual review confirms decisions are interpretable;
- quote refresh and risk recheck tested;
- duplicate confirm cannot duplicate trade;
- recovery/reconciliation drills pass.

### Gate F — Limited real Confirm

- dedicated low-balance wallets/accounts;
- tiny hard caps;
- venue-by-venue enablement;
- signing/key audit;
- backup/restore/kill-switch drill;
- explicit approval.

### Gate G — Auto

- shadow comparison against Confirm mode;
- stable skill version for defined observation window;
- no unresolved reconciliation incident;
- daily loss/drawdown controls proven;
- Auto lease expires automatically;
- explicit approval per venue.

---

## 18. Migration Phases

### Phase P0 — Pivot freeze & baseline (1 day)

- Freeze legacy feature development.
- Export current schema/status/runbook evidence.
- Mark legacy commands deprecated but keep rollback path.
- Reconcile this plan with current-state doc after user ACK.
- No DB mutation beyond documentation.

### Phase P1 — Domain skeleton & safe config (2–3 days)

- New domain interfaces and enums.
- v15 config/skill/provider tables or separate DB decision.
- Encrypted credential vault.
- Owner-only Telegram shell.
- Global pause/mode state.
- Tests first.

### Phase P2 — Skill registry & isolated validation (3–5 days)

- Safe ZIP import.
- Metadata/hash/version store.
- Static policy and schema validation.
- Isolated offline test runner without wallet keys.
- Activate/rollback.
- Register v1.1.0 baseline.

### Phase P3 — AI provider management (2–4 days)

- OpenAI-compatible adapter.
- Telegram endpoint/key/model setup.
- Test Connection.
- JSON schema validation.
- Secret masking/rotation/deletion.
- SSRF and timeout controls.

### Phase P4 — Market/evidence snapshots (5–8 days)

- Canonical asset model.
- Solana live read-only connector acceptance.
- BSC live read-only connector and security facts.
- Robinhood capability discovery and adapter decision.
- Immutable snapshots + provenance.
- Watchlist and on-demand Analyze.

### Phase P5 — AI decision + deterministic risk (4–6 days)

- Prompt assembly boundary.
- `trade.decision.v1` schema.
- AI decision storage.
- Risk policies/verdicts.
- Dashboard/reason codes.
- PAPER/ADVISORY only.

### Phase P6 — Paper execution & portfolio (5–8 days)

- Paper venue adapters.
- Conservative fill/fee/impact model.
- Orders/fills/positions/lots/PnL.
- Recovery and reconcile.
- Prospective paper scheduler.

### Phase P7 — Historical/shadow evaluation (ongoing, minimum window TBD)

- Import point-in-time datasets.
- Walk-forward/OOS and stress.
- Prospective paper metrics.
- Compare skill versions.
- Promotion report; no automatic promotion.

### Phase P8 — Confirm-mode real adapters (venue-by-venue, 5–10 days each)

Order recommended:

1. Solana/Jupiter.
2. BSC provider/router.
3. Robinhood Chain hanya jika discovery membuktikan chain ID, RPC, explorer, docs, dan execution venue resmi.

Each venue needs separate key/account, test, tiny cap, incident runbook, and approval.

### Phase P9 — Limited Auto (after Gate G)

- Expiring Auto policy lease.
- Scheduled decision runs.
- Pre-submit revalidation.
- Daily stop and drawdown lock.
- Kill switch and incident alerts.
- Venue-by-venue rollout.

### Phase P10 — Legacy archive

- Backup and restore drill.
- Export legacy data.
- Remove old Telegram routes.
- Archive or migrate tables only after approval.
- Consolidate documentation into one final source of truth.

---

## 19. Testing Strategy

### 19.1 TDD layers

- Unit: schema, risk policies, canonical identity, state transitions.
- Contract: AI/provider/venue adapters against fixtures.
- Integration: Postgres transactions, claim leases, idempotency, encryption.
- Fault injection: timeout after submit, duplicate callback, stale quote, provider conflict.
- Security: zip-slip, archive bomb, SSRF, secret redaction, prompt injection.
- Replay: deterministic snapshots/skill/AI fixture outputs.
- E2E paper: candidate → decision → risk → proposal/order → fill → position/PnL.

### 19.2 Critical cases

- AI says BUY but risk rejects.
- AI schema invalid.
- Skill changes between analysis and confirm.
- Quote changes before confirm.
- Double-tap confirm.
- Crash after submit before response.
- Provider reports unknown/partial fill.
- Sell route disappears.
- Daily loss threshold crossed with in-flight order.
- Kill switch during submission.
- Key rotation while job is running.
- Fallback AI disagrees.
- Telegram non-owner and group callback.

### 19.3 Verification environment

- Local: coding and Git only.
- `cbt-vps`: generate, DB integration, tests, build, sandbox runtime.
- Live real-money processes remain separate and require explicit approval.

---

## 20. Deploy & Rollback

### 20.1 Environments

- `dev/test`: deterministic fixtures and isolated DB when possible.
- `workspace sandbox`: paper/advisory; no real signing keys.
- `live confirm`: dedicated tiny-cap credentials, later.
- `live auto`: separate explicit gate, later.

### 20.2 Deployment

- Dual push origin + server bare.
- Server reset to verified commit.
- sqlc generate/migrate/test/vet/build/no-token smoke.
- Backup before schema/runtime changes.
- Restart workspace only with approval during pivot until policy is revisited.
- Health verifies bot, DB, AI provider, active skill, venue adapters, and worker.

### 20.3 Rollback

- Code rollback must preserve forward DB compatibility.
- Skill rollback changes active pointer.
- Provider rollback changes active profile.
- Mode rollback always permits safer direction (`AUTO → CONFIRM → ADVISORY → PAPER`).
- Never rollback by deleting fills/order events/audit.

---

## 21. Risks & Non-goals

### 21.1 Primary risks

- AI hallucination or malformed recommendation.
- Prompt injection through token/social content.
- Skill update changes behavior unexpectedly.
- Synthetic/backtest overfitting.
- Rug/honeypot/unsellable token.
- Slippage, MEV, thin liquidity, failed exit.
- Private key/API key compromise.
- Provider outage/stale/conflicting data.
- Duplicate order during timeout/retry.
- Incorrect PnL from partial fills/fees.
- Klaim Robinhood Chain bisa tidak terverifikasi atau tidak memiliki stack execution yang layak.
- Cross-venue asset identity mismatch.
- Auto mode enabled with insufficient evidence.

### 21.2 Non-goals v0.2

- Public users or multi-tenant accounts.
- Leverage, margin, futures, shorts, options.
- HFT/latency arbitrage.
- Guaranteed stop-loss fills.
- AI overriding risk.
- Autonomous skill self-modification.
- Unofficial credential scraping/browser automation for broker execution.
- Real-money execution before gates.

---

## 22. Open Decisions Tersisa (v0.2)

1. **BSC provider/router:** pilih setelah read-only quote + sell-simulation spike.
2. **Robinhood Chain identity:** verifikasi nama resmi, chain ID, RPC, explorer, native asset, docs, EVM compatibility, DEX/router, dan mainnet/testnet. Gagal verifikasi = tetap `UNVERIFIED_DISCOVERY`.
3. **Market data vendors:** exact Solana/BSC historical point-in-time sources.
4. **Social data:** sources, rate limits, provenance, dan inclusion phase.
5. **Paper promotion metrics:** minimum duration/sample, drawdown, expectancy, dan incident-free criteria.
6. **Position accounting:** FIFO lots direkomendasikan; treatment transfer/cross-chain ditetapkan saat P6.
7. **CF-08 legacy SOL:** rekomendasi sudah locked untuk tidak dimutasi selama pivot; final archive disposition di P10.

---

## 23. Acceptance Criteria

The pivot design is ready for implementation only when:

- [x] User ACKs skill-first direction dan Locked Decisions #1–#32.
- [x] Database, AI topology, skill runtime, signing direction, base currency, dan legacy runtime resolved untuk P0–P2.
- [x] Legacy feature freeze explicit secara desain; implementasi freeze dilakukan P0.
- [ ] No real credentials or funds are introduced during P0–P5.
- [ ] Skill v1.1.0 is treated as unproven research input, not profitable strategy.
- [ ] Risk engine remains deterministic and non-bypassable.
- [ ] AI/provider/skill/execution responsibilities are separated.
- [ ] Paper/Advisory/Confirm/Auto promotion gates remain distinct.
- [ ] Every venue can be paused independently and globally.
- [ ] Real execution remains approval-gated venue-by-venue.

---

## 24. Task-by-Task Implementation Plan P0–P2

> Status legend: `[ ]` pending, `[~]` in progress, `[x]` done, `[!]` blocked.
> Scope P0–P2 tetap research-only. Tidak ada AI API key, wallet key, signing, swap, atau real order.

### P0 — Pivot freeze & immutable baseline

**Progress:** 1/3 tasks done. P0.1 implemented and automated-verified; runtime activation remains pending explicit restart approval.

#### Task P0.1 — Freeze legacy product routes `[x]`

**Objective:** legacy exchange tetap dapat di-rollback tetapi tidak lagi menerima pengembangan/flow baru setelah owner shell cutover.

**Files:**
- Modify: `internal/bot/bot.go`
- Modify: `internal/bot/keyboard.go`
- Test: `internal/bot/session_test.go`
- Docs: `.kiro/steering/CRYPTO_STATION_CURRENT_STATE.md`

**Steps (TDD):**
1. Tulis test bahwa owner hanya melihat pivot/frozen notice dan entry owner shell; public user route lama tidak dipromosikan.
2. Jalankan `go test ./internal/bot -run 'Test.*PivotFreeze' -count=1`; expected RED.
3. Implement feature flag `PRODUCT_MODE=legacy_sandbox|meme_research` dengan default aman `legacy_sandbox` sampai deployment approval.
4. GREEN targeted test, lalu `go test -count=1 -p=1 ./...` di `cbt-vps`.
5. Commit: `feat(bot): add explicit product pivot mode`.

**Acceptance:** tidak ada route yang dihapus; mode bisa rollback; mode baru owner-only dan tidak menyatakan trading aktif.

**Evidence 2026-11-04 (`385ad9e`):** RED terbukti (`MainMenuForMode`/product-mode API belum ada); GREEN targeted; full `go test -count=1 -p=1 ./...`, `go vet ./...`, build, dan no-token DB smoke hijau di `cbt-vps`. `PRODUCT_MODE` default/fallback tetap `legacy_sandbox`; `meme_research` membekukan message + callback legacy dan hanya memberi owner tombol `Research` dengan label `RESEARCH ONLY`. Source local/origin/server/workspace sinkron di `385ad9e`; runtime process masih binary lama dan belum restart/aktif `meme_research`.

#### Task P0.2 — Consolidate project source of truth

**Objective:** satu dokumen aktif menyatakan legacy frozen + pivot v0.2 tanpa menghilangkan runbook historis.

**Files:**
- Modify/merge: `.kiro/steering/AI_MEME_TRADING_PIVOT_PLAN.md`
- Retire after merge: `.kiro/steering/CRYPTO_STATION_CURRENT_STATE.md`

**Steps:**
1. Salin fakta runtime, migration/backup/restore, commit history, dan known CF-08 discrepancy.
2. Cari semua referensi nama dokumen lama.
3. Hapus dokumen lama hanya setelah seluruh unique operational content masuk.
4. Verify `git ls-files '*.md'`, `git diff --check`, heading/line counts.
5. Commit: `docs: make AI meme trading roadmap the single source of truth`.

#### Task P0.3 — Register immutable skill artifact manifest

**Objective:** mencatat artifact v1.1.0 tanpa menjalankan isi ZIP.

**Files:**
- Create: `skills/meme-token/v1.1.0/manifest.json`
- Create: `internal/skill/manifest.go`
- Test: `internal/skill/manifest_test.go`
- Modify: `.gitignore` hanya bila raw ZIP memang diputuskan tetap external/untracked.

**Steps (TDD):**
1. Test wished-for parser untuk name/version/SHA-256/contract/capabilities/prohibitions.
2. RED: package/API belum ada.
3. Implement parser dan verify SHA-256 terhadap file yang diberikan saat import; jangan vendor/extract ZIP pada task ini.
4. Test mismatch hash, unsupported contract, missing prohibition.
5. Commit: `feat(skill): add immutable artifact manifest contract`.

**Acceptance:** manifest membedakan `research_only=true`, `executes_transactions=false`, `is_buy_signal=false`.

### P1 — Clean trading DB + schema compatibility + owner shell

#### Task P1.1 — Bootstrap clean trading database config

**Objective:** trading research state tidak bercampur legacy ledger v14.

**Files:**
- Create: `internal/tradingconfig/config.go`
- Test: `internal/tradingconfig/config_test.go`
- Modify: `cmd/bot/main.go`
- Create: `.env.trading.example` tanpa secret nyata.

**Steps (TDD):**
1. Test config mewajibkan `TRADING_DATABASE_URL` pada `meme_research`, tetapi tidak pada legacy mode.
2. RED.
3. Implement parser, validation, redacted errors.
4. Test tidak ada DSN/key bocor.
5. Commit: `feat(config): add isolated trading database configuration`.

#### Task P1.2 — Create migration 000001 research foundation

**Objective:** persist immutable skill/evidence/run metadata di DB trading baru.

**Files:**
- Create: `trading/migrations/000001_research_foundation.sql`
- Create: `trading/queries/skills.sql`
- Create: `trading/queries/research.sql`
- Create/update: `trading/sqlc.yaml`
- Test: `internal/research/repository_integration_test.go`

**Tables minimum:**
- `skill_packages`
- `research_runs`
- `evidence_items`
- `market_snapshots`
- `gate_results`
- `audit_events`

**Constraints:** immutable payload hash; unique artifact SHA; run captures skill/config hashes; UTC timestamps; no secret columns.

**Steps (TDD):** migration on empty DB → insert immutable run graph → duplicate/hash mutation rejected → sqlc generate → tests green.

**Verify:** migrate up/down only against disposable trading test DB; `go test -count=1 ./internal/research/...`.

**Commit:** `feat(research): add isolated research persistence`.

#### Task P1.3 — Implement skill schema compatibility layer

**Objective:** preserve v1.1.0 contracts before creating bot-specific contracts.

**Files:**
- Create: `internal/skill/contracts/research_snapshot.go`
- Create: `internal/skill/contracts/phase2_observation.go`
- Create: `internal/skill/contracts/phase3_run.go`
- Create: `internal/skill/contracts/phase4_report.go`
- Tests: matching `*_test.go` fixtures under `internal/skill/testdata/`.

**Steps (TDD):**
1. Copy safe synthetic JSON fixtures as testdata with provenance notice.
2. Test exact enums, null/unknown preservation, `research_priority_only=true`, `is_buy_signal=false`, `executes_transactions=false`.
3. RED then minimal typed decoder/validator.
4. Reject Phase 3 score conversion helper/API entirely; no `ToBuyConfidence` function.
5. Commit: `feat(skill): preserve v1.1 compatibility contracts`.

#### Task P1.4 — Build owner-only research shell

**Objective:** Telegram menu mencerminkan kemampuan skill aktual, bukan trading yang belum ada.

**Files:**
- Create: `internal/ownerui/keyboard.go`
- Create: `internal/ownerui/handler.go`
- Tests: `internal/ownerui/*_test.go`
- Modify: `internal/bot/bot.go`, `cmd/bot/main.go`

**Menu P1:**
`[Research] [Watchlist] [Skill] [Runs] [System] [Pause]`

**Steps (TDD):** owner sees menu; non-owner denied; group denied; screens label `RESEARCH ONLY`; no Buy/Sell/Auto buttons.

**Commit:** `feat(ownerui): add research-only control shell`.

### P2 — Safe skill registry + bounded research runner

#### Task P2.1 — Safe ZIP intake and quarantine

**Objective:** accept versioned skill package without executing it.

**Files:**
- Create: `internal/skill/archive.go`
- Create: `internal/skill/registry.go`
- Tests: `internal/skill/archive_test.go`, fixture archives under `internal/skill/testdata/archives/`.

**Security tests first:** zip-slip, absolute path, symlink, nested archive, file/count/size cap, duplicate normalized path, missing SKILL.md, hash mismatch.

**Implementation:** extract only to quarantine; static inventory; store metadata/validation result; no Python invocation/network.

**Commit:** `feat(skill): quarantine and validate skill archives`.

#### Task P2.2 — Register baseline v1.1.0

**Objective:** import user artifact as inactive/validated metadata baseline.

**Files:** no source change expected beyond optional safe import command.
- Create: `cmd/skillctl/main.go` if needed.
- Test: `cmd/skillctl/main_test.go` or service tests.

**Flow:** `inspect → checksum → static validate → register inactive → explicit activate`.

**Acceptance:** SHA exactly `07e506260e11ad0d4f9346f39cd0dbce47f6f56e87953c92d6a5d8dcccd10bd8`; scripts remain unexecuted; audit row exists.

**Commit:** `feat(skill): register meme research v1.1.0 baseline`.

#### Task P2.3 — Build bounded research job state machine

**Objective:** translate Phase 3 one-shot semantics into durable bounded jobs without claiming realtime autonomy.

**Files:**
- Create: `internal/research/job.go`
- Create: `internal/research/service.go`
- Create: `internal/research/worker.go`
- Create: `trading/queries/jobs.sql`
- Tests: unit + Postgres integration.

**States:** `queued → claimed → running → completed|completed_with_gaps|partial|failed`.

**Invariants:** atomic claim token/lease; max candidates/calls/seconds; immutable run config; resume does not silently refresh completed evidence.

**Commit:** `feat(research): add bounded durable research runs`.

#### Task P2.4 — Add prompt/schema-only AI compatibility boundary (no provider call)

**Objective:** define future AI explanation input/output without enabling AI yet.

**Files:**
- Create: `internal/ai/contracts.go`
- Create: `internal/ai/prompt.go`
- Tests: `internal/ai/contracts_test.go`, `prompt_test.go`.

**Contract:** input references validated evidence IDs/gates; output `EXPLAIN|REQUEST_MORE_DATA|NO_TRADE_PROPOSAL`; no BUY/SELL execution authority; no hidden chain-of-thought storage.

**Tests:** untrusted social/token text cannot modify system fields; unresolved G0–G2 cannot become executable proposal; artifact/config hashes required.

**Commit:** `feat(ai): define evidence-bound explanation contract`.

#### Task P2.5 — P0–P2 full verification and checkpoint

**Verify on `cbt-vps`:**
1. `sqlc generate` for trading schema.
2. Migrate empty disposable trading DB.
3. `go test -count=1 -p=1 ./...`.
4. `go vet ./...`.
5. Build bot + skillctl.
6. No-token/no-AI-key smoke.
7. Owner/non-owner Telegram manual trial after explicit restart approval.
8. Confirm legacy DB v14 unchanged and no real secret/fund introduced.

**Commit:** `docs: close skill-first foundation P0-P2`.

### P0–P2 completion gate

P0–P2 selesai hanya jika:

- [ ] one source of truth aktif;
- [ ] skill v1.1.0 immutable dan reproducible;
- [ ] clean trading DB berjalan;
- [ ] four skill contracts preserved;
- [ ] owner UI jujur `RESEARCH ONLY`;
- [ ] durable bounded runs recoverable;
- [ ] AI boundary belum bisa mengeksekusi;
- [ ] legacy DB tidak dimutasi;
- [ ] full automated verification green;
- [ ] manual Telegram trial terpisah dan jujur.

---

## 25. Current Next Step

Roadmap v0.2 sedang dieksekusi. **P0.1 DONE secara code + automated verification; belum runtime-activated.**

Urutan berikutnya:

1. **Task P0.1 selesai otomatis tetapi belum diaktifkan di runtime.**
2. Lanjut **Task P0.2**: konsolidasi source of truth.
3. Jangan mengaktifkan `meme_research` runtime sampai user memberi approval restart workspace.
4. Jangan masukkan AI key, wallet key, signing, swap, atau real funds selama P0–P2.
5. BSC provider, historical vendor, social source, dan Robinhood Chain identity tetap discovery backlog, bukan blocker P0–P2.

Prompt lanjut yang valid:

- `gas P0.2` — konsolidasi dokumentasi menjadi satu source of truth.
- `review P0-P2 dulu` — audit task plan tanpa coding.
- `ubah task <ID>` — revisi scope sebelum coding.
- `gap audit v0.2` — cari gap tambahan sebelum implementasi.
