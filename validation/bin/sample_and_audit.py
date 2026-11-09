"""P2 cross-provider data accuracy audit (read-only Solana RPC + DEX Screener).

Offline fixture mode is deterministic. Live mode only invokes read RPC methods and
HTTP GET; it never constructs, signs, simulates, or sends a transaction.
"""
from __future__ import annotations

import argparse, csv, hashlib, json, os, sys, time, urllib.error, urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "validation" / "bin"))
import provenance as prov  # noqa: E402
from redact import assert_no_secrets, redact_secrets, redact_url  # noqa: E402

DEFAULT_RPC = "https://api.mainnet-beta.solana.com"
DEX_BASE = "https://api.dexscreener.com"
DEX_PROFILES_URL = f"{DEX_BASE}/token-profiles/latest/v1"
DEX_LATEST_BOOSTS_URL = f"{DEX_BASE}/token-boosts/latest/v1"
DEX_TOP_BOOSTS_URL = f"{DEX_BASE}/token-boosts/top/v1"
JUPITER_TOKENS_URL = "https://cache.jup.ag/tokens"
ARCHIVED_TOKENS_URL = "https://raw.githubusercontent.com/solana-labs/token-list/main/src/tokens/solana.tokenlist.json"
GECKO_TRENDING_URLS = tuple(f"https://api.geckoterminal.com/api/v2/networks/solana/trending_pools?page={page}&include=base_token" for page in range(1, 6))
BROWSER_UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/131.0 Safari/537.36 P2ReadOnly/1.0"
TOKEN_PROGRAMS = {"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA": "SPL", "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb": "TOKEN_2022"}
STRATA = ("new_discovery", "active_liquid", "low_liquidity", "risky_delisted_migrated")
COLUMNS = ("mint", "chain", "source", "observed_at_utc", "rpc_slot", "dex_pair", "primary_quote_token", "mint_match", "price_basis_match", "supply_match", "holder_semantics_valid", "freshness_ok", "discrepancy_code", "discrepancy_explanation", "reviewer", "raw_sha256")
UNKNOWN = "UNKNOWN"


def load_local_env(path: Path) -> None:
    """Load simple KEY=VALUE entries without overriding the process environment."""
    if not path.exists():
        return
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        key, value = key.strip(), value.strip()
        if value[:1] == value[-1:] and value[:1] in {"\"", "'"}:
            value = value[1:-1]
        if key and key not in os.environ:
            os.environ[key] = value


def _b58decode(value: str) -> bytes:
    alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
    n = 0
    for c in value:
        if c not in alphabet: raise ValueError("non-base58 character")
        n = n * 58 + alphabet.index(c)
    body = n.to_bytes((n.bit_length() + 7) // 8, "big") if n else b""
    return b"\0" * (len(value) - len(value.lstrip("1"))) + body


def _b58encode(value: bytes) -> str:
    alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
    n = int.from_bytes(value, "big"); out = ""
    while n:
        n, r = divmod(n, 58); out = alphabet[r] + out
    return "1" * (len(value) - len(value.lstrip(b"\0"))) + out


def valid_mint(value: Any) -> bool:
    try: return isinstance(value, str) and len(_b58decode(value)) == 32
    except ValueError: return False


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(redact_secrets(value), sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()


def _status(condition: bool | None) -> str:
    return UNKNOWN if condition is None else ("TRUE" if condition else "FALSE")


def normalize(sample: dict[str, Any]) -> dict[str, str]:
    """Normalize one observation without inventing absent facts."""
    mint, rpc, dex = sample.get("mint", ""), sample.get("rpc") or {}, sample.get("dex") or {}
    account, supply, largest = rpc.get("account_info") or {}, rpc.get("supply") or {}, rpc.get("largest_accounts")
    pair = dex.get("pair") or {}
    codes: list[str] = []
    chain_ok = pair.get("chainId") == "solana" if pair else None
    address_ok = valid_mint(mint)
    program = account.get("owner")
    program_ok = program in TOKEN_PROGRAMS if program is not None else None
    parsed_info = (((account.get("data") or {}).get("parsed") or {}).get("info") or {})
    # A present null explicitly means revoked; an absent key means unobserved.
    authorities_observed = all(k in parsed_info for k in ("mintAuthority", "freezeAuthority")) if account else None
    base = (pair.get("baseToken") or {}).get("address")
    quote = (pair.get("quoteToken") or {}).get("address")
    mint_match = (chain_ok and base == mint) if pair else None
    # DEX priceUsd is usable only when target is exactly the base token.
    price_ok = (base == mint and pair.get("priceUsd") is not None) if pair else None
    rpc_supply = supply.get("amount")
    # The parsed mint account and getTokenSupply are independent RPC views of
    # the same raw integer supply. An explicit fixture expectation takes
    # precedence, otherwise compare those exact raw values without float casts.
    expected_supply = sample.get("expected_supply_raw")
    if expected_supply is None:
        expected_supply = parsed_info.get("supply")
    supply_match = None if rpc_supply is None or expected_supply is None else str(rpc_supply) == str(expected_supply)
    holder_ok = None if largest is None else isinstance(largest, list) and all("address" in x for x in largest) and not sample.get("holders_are_unique_owners", False)
    critical = [address_ok, program_ok, authorities_observed, mint_match, price_ok, holder_ok]
    if not address_ok: codes.append("INVALID_MINT")
    if program_ok is False: codes.append("WRONG_TOKEN_PROGRAM")
    if authorities_observed is None or authorities_observed is False: codes.append("AUTHORITY_DATA_MISSING")
    if pair and chain_ok is False: codes.append("CHAIN_MISMATCH")
    if pair and base != mint: codes.append("TARGET_NOT_DEX_BASE")
    if pair and pair.get("priceUsd") is None: codes.append("PRICE_MISSING")
    if holder_ok is False: codes.append("HOLDER_SEMANTICS_INVALID")
    if any(x is None for x in critical) or authorities_observed is False: codes.append("DATA_TIDAK_CUKUP")
    observed = sample.get("observed_at_utc")
    age = sample.get("age_seconds")
    if age is None and observed is not None and sample.get("received_at_utc") is not None:
        try:
            start = datetime.fromisoformat(str(observed).replace("Z", "+00:00"))
            end = datetime.fromisoformat(str(sample["received_at_utc"]).replace("Z", "+00:00"))
            age = (end - start).total_seconds()
            if age < 0: age = None
        except (TypeError, ValueError, OverflowError):
            age = None
    try:
        fresh = None if observed is None or age is None else float(age) <= float(sample.get("freshness_limit_seconds", 300))
    except (TypeError, ValueError, OverflowError):
        fresh = None
    if fresh is False: codes.append("STALE")
    if any(x is None for x in (mint_match, price_ok, supply_match, holder_ok, fresh)) and "DATA_TIDAK_CUKUP" not in codes:
        codes.append("DATA_TIDAK_CUKUP")
    if not pair and "no Solana DEX pair observed" in str(sample.get("selection_rationale", "")):
        codes.append("DOCUMENTED_NO_PAIR")
    if sample.get("collection_error"): codes.append("COLLECTION_FAILED")
    raw_hash = hashlib.sha256(canonical_bytes(sample)).hexdigest()
    explanation = "; ".join(codes) if codes else "No discrepancy detected for verifiable fields"
    return {"mint": str(mint), "chain": "solana" if address_ok else UNKNOWN, "source": str(sample.get("source") or UNKNOWN),
        "observed_at_utc": str(observed or UNKNOWN), "rpc_slot": str(supply.get("slot") or rpc.get("slot") or UNKNOWN),
        "dex_pair": str(pair.get("pairAddress") or UNKNOWN), "primary_quote_token": str(quote or UNKNOWN),
        "mint_match": _status(mint_match), "price_basis_match": _status(price_ok), "supply_match": _status(supply_match),
        "holder_semantics_valid": _status(holder_ok), "freshness_ok": _status(fresh),
        "discrepancy_code": "|".join(dict.fromkeys(codes)) if codes else "NONE", "discrepancy_explanation": explanation,
        "reviewer": str(sample.get("reviewer") or "UNREVIEWED"), "raw_sha256": raw_hash}


def validate_sampling(samples: list[dict[str, Any]]) -> dict[str, Any]:
    counts = {s: 0 for s in STRATA}
    for item in samples:
        if item.get("stratum") in counts: counts[item["stratum"]] += 1
    distinct = len({x.get("mint") for x in samples if valid_mint(x.get("mint"))})
    return {"total": len(samples), "distinct_valid_mints": distinct, "counts": counts,
            "minimum_100": len(samples) >= 100 and distinct >= 100,
            "strata_minimum_25": all(v >= 25 for v in counts.values()), "failed_in_denominator": sum(bool(x.get("collection_error")) for x in samples)}


def build_sampling_frame(profiles: Any, documented: list[dict[str, Any]], pair_lookup: Any, observed_at: str, source_urls: dict[str, str] | None = None) -> list[dict[str, Any]]:
    """Apply observable, predeclared rules; labels make no claim of launch age or misconduct."""
    feeds = profiles if isinstance(profiles, dict) else {"profiles": profiles, "latest_boosts": [], "top_boosts": []}
    feed_labels = {"profiles":"DEX Screener latest token profiles", "latest_boosts":"DEX Screener latest token boosts", "top_boosts":"DEX Screener top token boosts"}
    discovery = [(x.get("tokenAddress"), name) for name, records in feeds.items() for x in records if x.get("chainId") == "solana"]
    pm = [mint for mint, _ in discovery]
    dm = [x.get("address") for x in documented]
    candidates = list(dict.fromkeys(x for x in pm + dm if valid_mint(x)))
    pair_map = pair_lookup(candidates); selected: list[dict[str, Any]] = []; used: set[str] = set(); source_urls = source_urls or {}
    def add(mint: str, stratum: str, rationale: str, source: str) -> None:
        if mint not in used and sum(x["stratum"] == stratum for x in selected) < 25:
            candidate = next((x for x in documented if x.get("address") == mint), {})
            used.add(mint); selected.append({"mint":mint,"stratum":stratum,"source":source,"selection_rationale":rationale,
                "provenance":{"observed_at_utc":observed_at,"discovery_feed_urls":[source_urls.get(k) for k in ("profiles_url","latest_boosts_url","top_boosts_url") if source_urls.get(k)],"tokens_url":source_urls.get("tokens_url"),
                "candidate_source_urls":candidate.get("_candidate_source_urls", []),"pair_source_url":next((p.get("_source_url") for p in pair_map.get(mint,[]) if p.get("_source_url")),f"{DEX_BASE}/token-pairs/v1/solana/{mint}"),"rule_version":"p2-frame-v1"}})
    metrics: dict[str, tuple[list[Any],float,int]] = {}
    for mint in dm:
        pairs=[p for p in pair_map.get(mint,[]) if p.get("chainId")=="solana" and (p.get("baseToken") or {}).get("address")==mint]
        liq=max((float((p.get("liquidity") or {}).get("usd") or 0) for p in pairs),default=0)
        tx=max((sum(int(v or 0) for v in ((p.get("txns") or {}).get("h24") or {}).values()) for p in pairs),default=0); metrics[mint]=(pairs,liq,tx)
    for mint,(pairs,liq,tx) in metrics.items():
        if pairs and liq>=100000 and tx>=100:
            candidate=next((x for x in documented if x.get("address")==mint), {})
            origins=candidate.get("_candidate_sources") or ["Jupiter documented token list"]
            add(mint,"active_liquid",f"observable max liquidity_usd={liq:g} and h24_txns={tx}"," + ".join(origins+["DEX Screener pairs"]))
    # Allocate active tokens first so discovery feeds cannot consume the scarce
    # active stratum. Discovery semantics remain feed-based and launch age is not
    # inferred; cross-stratum mint uniqueness is preserved.
    for mint, feed in discovery: add(mint,"new_discovery",f"present in {feed_labels[feed]}; launch age not assumed",feed_labels[feed])
    for mint,(pairs,liq,tx) in metrics.items():
        if pairs and (liq<=5000 or tx<=5): add(mint,"low_liquidity",f"observable max liquidity_usd={liq:g} or h24_txns={tx} meets low-activity rule","Jupiter verified token list + DEX Screener pairs")
    for mint,(pairs,liq,tx) in metrics.items():
        if not pairs: add(mint,"risky_delisted_migrated","documented mint; no Solana DEX pair observed at sampling time","Jupiter verified token list + DEX Screener no-pair observation")
    return selected


def _request(url: str, payload: dict[str, Any] | None, timeout: float, retries: int = 3) -> Any:
    if not url.startswith("https://"): raise ValueError("HTTPS required")
    body = json.dumps(payload).encode() if payload else None
    req = urllib.request.Request(url, data=body, method="POST" if body else "GET", headers={"Content-Type":"application/json", "User-Agent":BROWSER_UA, "Accept":"application/json"})
    for attempt in range(retries + 1):
        try:
            with urllib.request.urlopen(req, timeout=timeout) as response: return json.loads(response.read(20_000_000))
        except (urllib.error.HTTPError, urllib.error.URLError, TimeoutError):
            if attempt >= retries: raise
            time.sleep(min(8.0, 0.75 * (2 ** attempt)))


def parse_gecko_base_tokens(payload: Any, source_url: str) -> list[dict[str, Any]]:
    """Resolve pool base-token relationships through JSON:API included resources."""
    if not isinstance(payload, dict): return []
    included = payload.get("included") if isinstance(payload.get("included"), list) else []
    resources = {x.get("id"): x for x in included if isinstance(x, dict) and x.get("type") == "token" and isinstance(x.get("id"), str)}
    result: list[dict[str, Any]] = []; seen: set[str] = set()
    for pool in payload.get("data") if isinstance(payload.get("data"), list) else []:
        if not isinstance(pool, dict): continue
        relationships = pool.get("relationships") if isinstance(pool.get("relationships"), dict) else {}
        base = relationships.get("base_token") if isinstance(relationships.get("base_token"), dict) else {}
        ref = base.get("data") if isinstance(base.get("data"), dict) else {}
        resource = resources.get(ref.get("id"), {})
        attributes = resource.get("attributes") if isinstance(resource.get("attributes"), dict) else {}
        resource_id = resource.get("id") if isinstance(resource.get("id"), str) else ""
        mint = attributes.get("address")
        # Gecko JSON:API token resources commonly encode network_address in id;
        # accept it only after resolving the relationship to an included token.
        if not valid_mint(mint) and resource_id.startswith("solana_"):
            mint = resource_id.removeprefix("solana_")
        if valid_mint(mint) and mint not in seen:
            seen.add(mint); result.append({"address":mint,"_candidate_sources":["GeckoTerminal Solana trending pools"],"_candidate_source_urls":[source_url]})
    return result


def fetch_sampling_sources(timeout: float) -> tuple[dict[str, list[dict[str, Any]]], list[dict[str, Any]], dict[str, Any]]:
    """Fetch discovery and documented lists, falling back only when Jupiter fails."""
    feeds = {"profiles": _request(DEX_PROFILES_URL, None, timeout), "latest_boosts": _request(DEX_LATEST_BOOSTS_URL, None, timeout), "top_boosts": _request(DEX_TOP_BOOSTS_URL, None, timeout)}
    token_url = JUPITER_TOKENS_URL
    try:
        documented = _request(token_url, None, timeout)
    except Exception:
        token_url = ARCHIVED_TOKENS_URL
        raw = _request(token_url, None, timeout)
        documented = raw.get("tokens", []) if isinstance(raw, dict) else raw
    gecko: list[dict[str, Any]] = []
    for page, url in enumerate(GECKO_TRENDING_URLS):
        try: gecko.extend(parse_gecko_base_tokens(_request(url, None, timeout), url))
        except (urllib.error.HTTPError, urllib.error.URLError, TimeoutError): pass
        if page + 1 < len(GECKO_TRENDING_URLS): time.sleep(1.0)
    by_mint: dict[str, dict[str, Any]] = {}
    for token in gecko:
        mint=token["address"]
        if mint not in by_mint: by_mint[mint]=token
        else: by_mint[mint]["_candidate_source_urls"].extend(u for u in token["_candidate_source_urls"] if u not in by_mint[mint]["_candidate_source_urls"])
    documented = (documented if isinstance(documented, list) else []) + list(by_mint.values())
    return ({k: v if isinstance(v, list) else [] for k,v in feeds.items()}, documented,
            {"profiles_url": DEX_PROFILES_URL, "latest_boosts_url":DEX_LATEST_BOOSTS_URL, "top_boosts_url":DEX_TOP_BOOSTS_URL, "tokens_url": token_url, "gecko_trending_urls":list(GECKO_TRENDING_URLS)})


def select_documented_candidates(tokens: list[dict[str, Any]], limit: int) -> list[dict[str, Any]]:
    """Deterministically span the full Jupiter list, preferring observable tags without relying on source order."""
    valid = [x for x in tokens if valid_mint(x.get("address"))]
    # A documented CoinGecko identity is a reproducible candidate-priority signal,
    # not proof of liquidity. Final active classification still requires live DEX
    # liquidity and transaction thresholds below.
    gecko = sorted((x for x in valid if x.get("_candidate_source_urls")), key=lambda x: x["address"])
    identified = sorted(
        (x for x in valid if (x.get("extensions") or {}).get("coingeckoId")),
        key=lambda x: (str((x.get("extensions") or {}).get("coingeckoId")), x["address"]),
    )
    tagged = sorted((x for x in valid if x.get("tags")), key=lambda x: hashlib.sha256(x["address"].encode()).digest())
    ordered = sorted(valid, key=lambda x: hashlib.sha256(x["address"].encode()).digest())
    chosen = gecko[:limit]
    seen = {x["address"] for x in chosen}
    for pool in (identified, tagged, ordered):
        additions=[x for x in pool if x["address"] not in seen][:max(0,limit-len(chosen))]
        chosen.extend(additions); seen.update(x["address"] for x in additions)
    return chosen


def fetch_pair_map(mints: list[str], timeout: float, delay: float = 0.25) -> dict[str, list[dict[str, Any]]]:
    """Use DEX Screener's documented batch token endpoint (max 30 addresses/request)."""
    result = {mint: [] for mint in mints}
    for start in range(0, len(mints), 30):
        chunk = mints[start:start + 30]
        url = f"{DEX_BASE}/tokens/v1/solana/{','.join(chunk)}"
        pairs = _request(url, None, timeout)
        for pair in pairs if isinstance(pairs, list) else []:
            base = (pair.get("baseToken") or {}).get("address")
            if base in result:
                record = dict(pair); record["_source_url"] = url; result[base].append(record)
        if start + 30 < len(mints): time.sleep(delay)
    return result


def collect_live(item: dict[str, Any], rpc_url: str, timeout: float, rpc_delay: float = 0.35) -> dict[str, Any]:
    """Collect independent evidence; one throttled provider method cannot discard another."""
    mint = item["mint"]; observed = prov.utc_now()
    rpc: dict[str, Any] = {"account_info": {}, "supply": {}, "largest_accounts": None}
    dex: dict[str, Any] = {}; status: dict[str, Any] = {}

    def safe_error(exc: Exception) -> dict[str, Any]:
        # Never persist exception text or URLs: either may contain provider secrets.
        error: dict[str, Any] = {"type": type(exc).__name__}
        if isinstance(exc, urllib.error.HTTPError): error["http_status"] = exc.code
        return error

    def call(method: str, params: list[Any]) -> dict[str, Any] | None:
        try:
            # One bounded attempt per method. Cross-run retries are handled by a
            # new immutable checkpoint; long in-call retries amplify public-RPC
            # 403/429 bursts and destroy snapshot consistency.
            response = _request(rpc_url, {"jsonrpc":"2.0","id":1,"method":method,"params":params}, timeout, retries=0)
            if not isinstance(response, dict): raise TypeError("invalid response")
            if response.get("error"):
                error = response["error"] if isinstance(response["error"], dict) else {}
                detail: dict[str, Any] = {"type":"JSONRPCError"}
                if isinstance(error.get("code"), int): detail["code"] = error["code"]
                status[method] = {"status":"error", "error":detail}; return None
            status[method] = {"status":"ok"}; return response
        except Exception as exc:
            status[method] = {"status":"error", "error":safe_error(exc)}; return None

    ai = call("getAccountInfo", [mint, {"encoding":"jsonParsed","commitment":"confirmed"}])
    if ai is not None: rpc["account_info"] = (ai.get("result") or {}).get("value") or {}
    time.sleep(rpc_delay)
    sp = call("getTokenSupply", [mint, {"commitment":"confirmed"}])
    if sp is not None:
        result=sp.get("result") or {}; rpc["supply"] = dict(result.get("value") or {}, slot=(result.get("context") or {}).get("slot"))
    time.sleep(rpc_delay)
    la = call("getTokenLargestAccounts", [mint, {"commitment":"confirmed"}])
    if la is not None: rpc["largest_accounts"] = (la.get("result") or {}).get("value")
    try:
        pairs = _request(f"{DEX_BASE}/token-pairs/v1/solana/{mint}", None, timeout)
        exact = [p for p in (pairs or []) if p.get("chainId") == "solana" and (p.get("baseToken") or {}).get("address") == mint]
        pair = max(exact, key=lambda p: float((p.get("liquidity") or {}).get("usd") or 0), default=None)
        if pair: dex["pair"] = pair
        status["dex_pairs"] = {"status":"ok"}
    except Exception as exc:
        status["dex_pairs"] = {"status":"error", "error":safe_error(exc)}
    out = dict(item, observed_at_utc=observed, received_at_utc=prov.utc_now(), rpc=rpc, dex=dex, collection_status=status)
    if all(record["status"] == "error" for record in status.values()): out["collection_error"] = "all_requests_failed"
    return out


def collect_with_checkpoint(items: list[dict[str, Any]], path: Path, collector: Any, rpc_url: str, timeout: float, rpc_delay: float) -> list[dict[str, Any]]:
    """Resume by mint; every completed observation, including failures, is durable."""
    path.parent.mkdir(parents=True, exist_ok=True); done: dict[str, dict[str, Any]] = {}
    if path.exists():
        for line in path.read_text(encoding="utf-8").splitlines():
            try:
                row=json.loads(line); done[row["mint"]]=row
            except (json.JSONDecodeError, KeyError): pass
    with path.open("a",encoding="utf-8",newline="\n") as f:
        for item in items:
            if item["mint"] not in done:
                row=collector(item,rpc_url,timeout,rpc_delay); done[item["mint"]]=row
                f.write(json.dumps(row,sort_keys=True)+"\n"); f.flush(); os.fsync(f.fileno())
    return [done[x["mint"]] for x in items]


def write_csv(path: Path, rows: list[dict[str, str]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="") as f:
        w=csv.DictWriter(f, fieldnames=COLUMNS); w.writeheader(); w.writerows(rows)


def evaluate_acceptance(sampling: dict[str, Any], rows: list[dict[str, str]]) -> dict[str, Any]:
    """Evaluate the explicit runbook P2 gates; UNKNOWN is never counted as a match."""
    fields=("mint_match","price_basis_match","supply_match","holder_semantics_valid","freshness_ok")
    verifiable=[r[f] for r in rows for f in fields if r[f] != UNKNOWN]
    matching=sum(v == "TRUE" for v in verifiable)
    unknown=sum(r[f] == UNKNOWN for r in rows for f in fields)
    critical_unknown_rows=sum(any(r[f] == UNKNOWN for f in fields) for r in rows)
    marked_unknown_rows=sum(any(r[f] == UNKNOWN for f in fields) and "DATA_TIDAK_CUKUP" in r["discrepancy_code"] for r in rows)
    silent_mint_mismatch=sum(r["mint_match"] == "FALSE" and "TARGET_NOT_DEX_BASE" not in r["discrepancy_code"] and "CHAIN_MISMATCH" not in r["discrepancy_code"] for r in rows)
    invalid_price_or_holder=sum(r["price_basis_match"] == "FALSE" or r["holder_semantics_valid"] == "FALSE" for r in rows)
    documented_no_pair=sum("DOCUMENTED_NO_PAIR" in r["discrepancy_code"] and r["mint_match"] == UNKNOWN and r["price_basis_match"] == UNKNOWN for r in rows)
    silent_no_pair=sum(r["dex_pair"] == UNKNOWN and r["discrepancy_code"] == "NONE" for r in rows)
    rate=matching/len(verifiable) if verifiable else 0.0
    gates={"distinct_balanced_100":sampling["total"] == 100 and sampling["distinct_valid_mints"] == 100 and all(v == 25 for v in sampling["counts"].values()),
           "zero_silent_mint_mismatch":silent_mint_mismatch == 0,
           "zero_invalid_price_holder_semantics":invalid_price_or_holder == 0,
           "all_critical_missing_marked_unknown":critical_unknown_rows == marked_unknown_rows,
           "verifiable_match_rate_at_least_98pct":rate >= .98,
           "zero_silent_no_pair":silent_no_pair == 0}
    return {"verdict":"PASS" if all(gates.values()) else "FAIL", "gates":gates,
            "verifiable_fields":len(verifiable), "verifiable_matches":matching, "verifiable_match_rate":rate,
            "unverifiable_fields":unknown, "critical_unknown_rows":critical_unknown_rows,
            "critical_unknown_rows_marked":marked_unknown_rows, "silent_mint_mismatch":silent_mint_mismatch,
            "invalid_price_or_holder_semantics":invalid_price_or_holder,
            "documented_unverifiable_no_pair":documented_no_pair, "silent_success_no_pair":silent_no_pair}


def write_report(path: Path, run_id: str, sampling: dict[str, Any], rows: list[dict[str,str]], synthetic: bool) -> None:
    failures=sum(r["discrepancy_code"] != "NONE" for r in rows); unknown=sum("UNKNOWN" in (r[c] for c in ("mint_match","price_basis_match","supply_match","holder_semantics_valid","freshness_ok")) for r in rows)
    acceptance=evaluate_acceptance(sampling,rows)
    examples=rows[:10]
    text=f"""# DATA_ACCURACY\n\n## Run provenance\n- run_id: `{run_id}`\n- observed_at_utc: `{prov.utc_now()}`\n- synthetic: `{str(synthetic).lower()}`\n- code_version: `{prov.code_version()}`\n- denominator: {len(rows)} (collection failures retained: {sampling['failed_in_denominator']})\n\n## Sampling frame\n- required strata: {', '.join(STRATA)}\n- counts: `{json.dumps(sampling['counts'], sort_keys=True)}`\n- minimum_100: {sampling['minimum_100']}\n- each_stratum_minimum_25: {sampling['strata_minimum_25']}\n\n## Accuracy summary\n- rows_with_discrepancy_or_insufficient_data: {failures}\n- rows_with_unknown_verifiability: {unknown}\n- UNKNOWN and DATA_TIDAK_CUKUP are never interpreted as safe or valid.\n\n## P2 acceptance verdict\n- verdict: **{acceptance['verdict']}**\n- distinct_balanced_100: {acceptance['gates']['distinct_balanced_100']}\n- silent_mint_mismatch: {acceptance['silent_mint_mismatch']} (required: 0)\n- invalid_price_or_holder_semantics: {acceptance['invalid_price_or_holder_semantics']} (required: 0)\n- critical_unknown_rows_marked: {acceptance['critical_unknown_rows_marked']}/{acceptance['critical_unknown_rows']} (required: 100%)\n- verifiable_normalized_fields_matching: {acceptance['verifiable_matches']}/{acceptance['verifiable_fields']} ({acceptance['verifiable_match_rate']:.2%}; required: >=98%)\n- unverifiable_fields: {acceptance['unverifiable_fields']} (reported separately, not counted as matches)\n- documented_unverifiable_no_pair: {acceptance['documented_unverifiable_no_pair']}\n- silent_success_no_pair: {acceptance['silent_success_no_pair']} (required: 0)\n\n## Confusion / error taxonomy\n| Code | Meaning | Treatment |\n|---|---|---|\n| INVALID_MINT | Not base58 32-byte mint | Invalid |\n| WRONG_TOKEN_PROGRAM | Account owner is not SPL/Token-2022 | Invalid |\n| CHAIN_MISMATCH | DEX chain is not Solana | Invalid |\n| TARGET_NOT_DEX_BASE | Target is quote/other token | Reject priceUsd and mint match |\n| PRICE_MISSING | Base-token USD price absent | UNKNOWN, not zero |\n| HOLDER_SEMANTICS_INVALID | Token accounts represented as unique owners | Invalid |
| AUTHORITY_DATA_MISSING | Mint/freeze authority was not observed (null values are valid observations) | UNKNOWN, never safe |\n| STALE | Observation exceeds freshness limit | Review |\n| COLLECTION_FAILED | Provider request failed | Retain in denominator |\n| DATA_TIDAK_CUKUP | Critical field missing | UNKNOWN, never safe |\n\n## Ten manual audit examples\n| # | mint | pair | mint | price basis | holders | discrepancy |\n|---:|---|---|---|---|---|---|\n"""
    for i,r in enumerate(examples,1): text += f"| {i} | {r['mint']} | {r['dex_pair']} | {r['mint_match']} | {r['price_basis_match']} | {r['holder_semantics_valid']} | {r['discrepancy_code']} |\n"
    text += "\n## Interpretation constraints\nDEX Screener `priceUsd` is accepted only for an exact Solana pair whose base-token mint equals the sampled mint. Largest accounts remain token accounts; owner and beneficial-owner concentration are separate and UNKNOWN unless independently proven. Null authority alone never establishes safety.\n"
    path.parent.mkdir(parents=True, exist_ok=True); path.write_text(text, encoding="utf-8", newline="\n")


def main(argv: list[str] | None=None) -> int:
    load_local_env(ROOT / "validation" / ".env")
    p=argparse.ArgumentParser(); p.add_argument("--input"); p.add_argument("--build-frame",action="store_true"); p.add_argument("--frame-output",default="validation/normalized/p2_sampling_frame.json"); p.add_argument("--shortfall-output",default="validation/reports/p2_sampling_shortfall.json"); p.add_argument("--candidate-limit",type=int,default=6000); p.add_argument("--live", action="store_true"); p.add_argument("--checkpoint",default="validation/checkpoints/p2_live_audit.jsonl"); p.add_argument("--rpc-delay",type=float,default=.35); p.add_argument("--rpc-url", default=os.environ.get("SOLANA_RPC_URL") or DEFAULT_RPC); p.add_argument("--timeout",type=float,default=12); p.add_argument("--csv",default="validation/normalized/audit_100_tokens.csv"); p.add_argument("--report",default="validation/reports/DATA_ACCURACY.md"); args=p.parse_args(argv)
    if args.build_frame:
        observed=prov.utc_now(); profiles,documented,urls=fetch_sampling_sources(args.timeout)
        # Hash ordering spans the complete Jupiter list and avoids provider source-order bias.
        documented=select_documented_candidates(documented,max(100,args.candidate_limit))
        frame=build_sampling_frame(profiles,documented,lambda m:fetch_pair_map(m,args.timeout),observed,urls)
        sampling=validate_sampling(frame); complete=sampling["minimum_100"] and sampling["strata_minimum_25"]
        output=ROOT/(args.frame_output if complete else args.shortfall_output); output.parent.mkdir(parents=True,exist_ok=True)
        artifact={"status":"COMPLETE" if complete else "SHORTFALL","observed_at_utc":observed,"synthetic":False,"sampling":sampling,"sources":urls,"candidate_limit":args.candidate_limit}
        if complete:
            output.write_text(json.dumps(frame,indent=2)+"\n",encoding="utf-8")
        else:
            artifact["message"]="No frame emitted: 100 distinct valid mints and 25 per stratum were not observed"
            output.write_text(json.dumps(artifact,indent=2)+"\n",encoding="utf-8")
        print(f"status={artifact['status']} counts={sampling['counts']} distinct={sampling['distinct_valid_mints']} output={output.relative_to(ROOT)}")
        return 0 if complete else 2
    if not args.input: p.error("--input is required unless --build-frame is used")
    samples=json.loads((ROOT/args.input).read_text(encoding="utf-8")); run_id=prov.new_run_id("p2")
    if args.live: samples=collect_with_checkpoint(samples,ROOT/args.checkpoint,collect_live,args.rpc_url,args.timeout,args.rpc_delay)
    sampling=validate_sampling(samples); rows=[normalize(x) for x in samples]
    write_csv(ROOT/args.csv,rows); write_report(ROOT/args.report,run_id,sampling,rows,not args.live)
    manifest={"run_id":run_id,"stage":"p2","observed_at_utc":prov.utc_now(),"synthetic":not args.live,"input":args.input,"csv":args.csv,"report":args.report,"sampling":sampling,"rpc_endpoint":redact_url(args.rpc_url) if args.live else None}
    serial=json.dumps(redact_secrets(manifest)); assert_no_secrets(serial,forbidden=(os.environ.get("SOLANA_RPC_URL") or "",)); prov.append_manifest_line(ROOT/"validation/manifests/p2_evidence.jsonl",json.loads(serial))
    print(f"rows={len(rows)} failures_retained={sampling['failed_in_denominator']} csv={args.csv} report={args.report}")
    return 0 if sampling["minimum_100"] and sampling["strata_minimum_25"] else 2

if __name__ == "__main__": raise SystemExit(main())
