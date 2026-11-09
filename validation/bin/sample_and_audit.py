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
JUPITER_TOKENS_URL = "https://cache.jup.ag/tokens"
ARCHIVED_TOKENS_URL = "https://raw.githubusercontent.com/solana-labs/token-list/main/src/tokens/solana.tokenlist.json"
BROWSER_UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/131.0 Safari/537.36 P2ReadOnly/1.0"
TOKEN_PROGRAMS = {"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA": "SPL", "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb": "TOKEN_2022"}
STRATA = ("new_discovery", "active_liquid", "low_liquidity", "risky_delisted_migrated")
COLUMNS = ("mint", "chain", "source", "observed_at_utc", "rpc_slot", "dex_pair", "primary_quote_token", "mint_match", "price_basis_match", "supply_match", "holder_semantics_valid", "freshness_ok", "discrepancy_code", "discrepancy_explanation", "reviewer", "raw_sha256")
UNKNOWN = "UNKNOWN"


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
    expected_supply = sample.get("expected_supply_raw")
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
    fresh = None if observed is None or age is None else float(age) <= float(sample.get("freshness_limit_seconds", 300))
    if fresh is False: codes.append("STALE")
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


def build_sampling_frame(profiles: list[dict[str, Any]], documented: list[dict[str, Any]], pair_lookup: Any, observed_at: str, source_urls: dict[str, str] | None = None) -> list[dict[str, Any]]:
    """Apply observable, predeclared rules; labels make no claim of launch age or misconduct."""
    pm = [x.get("tokenAddress") for x in profiles if x.get("chainId") == "solana"]
    dm = [x.get("address") for x in documented]
    candidates = list(dict.fromkeys(x for x in pm + dm if valid_mint(x)))
    pair_map = pair_lookup(candidates); selected: list[dict[str, Any]] = []; used: set[str] = set(); source_urls = source_urls or {}
    def add(mint: str, stratum: str, rationale: str, source: str) -> None:
        if mint not in used and sum(x["stratum"] == stratum for x in selected) < 25:
            used.add(mint); selected.append({"mint":mint,"stratum":stratum,"source":source,"selection_rationale":rationale,
                "provenance":{"observed_at_utc":observed_at,"profiles_url":source_urls.get("profiles_url"),"tokens_url":source_urls.get("tokens_url"),
                "pair_source_url":next((p.get("_source_url") for p in pair_map.get(mint,[]) if p.get("_source_url")),f"{DEX_BASE}/token-pairs/v1/solana/{mint}"),"rule_version":"p2-frame-v1"}})
    for mint in pm: add(mint,"new_discovery","present in DEX Screener latest token profiles; launch age not assumed","DEX Screener latest token profiles")
    metrics: dict[str, tuple[list[Any],float,int]] = {}
    for mint in dm:
        pairs=[p for p in pair_map.get(mint,[]) if p.get("chainId")=="solana" and (p.get("baseToken") or {}).get("address")==mint]
        liq=max((float((p.get("liquidity") or {}).get("usd") or 0) for p in pairs),default=0)
        tx=max((sum(int(v or 0) for v in ((p.get("txns") or {}).get("h24") or {}).values()) for p in pairs),default=0); metrics[mint]=(pairs,liq,tx)
    for mint,(pairs,liq,tx) in metrics.items():
        if pairs and liq>=100000 and tx>=100: add(mint,"active_liquid",f"observable max liquidity_usd={liq:g} and h24_txns={tx}","Jupiter verified token list + DEX Screener pairs")
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


def fetch_sampling_sources(timeout: float) -> tuple[list[dict[str, Any]], list[dict[str, Any]], dict[str, str]]:
    """Fetch discovery and documented lists, falling back only when Jupiter fails."""
    profiles = _request(DEX_PROFILES_URL, None, timeout)
    token_url = JUPITER_TOKENS_URL
    try:
        documented = _request(token_url, None, timeout)
    except Exception:
        token_url = ARCHIVED_TOKENS_URL
        raw = _request(token_url, None, timeout)
        documented = raw.get("tokens", []) if isinstance(raw, dict) else raw
    return (profiles if isinstance(profiles, list) else [], documented if isinstance(documented, list) else [],
            {"profiles_url": DEX_PROFILES_URL, "tokens_url": token_url})


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
    mint = item["mint"]; observed = prov.utc_now(); rpc: dict[str, Any] = {}
    try:
        def call(method: str, params: list[Any]) -> dict[str, Any]:
            return _request(rpc_url, {"jsonrpc":"2.0","id":1,"method":method,"params":params}, timeout)
        ai = call("getAccountInfo", [mint, {"encoding":"jsonParsed","commitment":"confirmed"}]); time.sleep(rpc_delay)
        sp = call("getTokenSupply", [mint, {"commitment":"confirmed"}])
        time.sleep(rpc_delay)
        la = call("getTokenLargestAccounts", [mint, {"commitment":"confirmed"}])
        rpc = {"account_info": (ai.get("result") or {}).get("value") or {}, "supply": dict((sp.get("result") or {}).get("value") or {}, slot=(sp.get("result") or {}).get("context",{}).get("slot")), "largest_accounts": (la.get("result") or {}).get("value")}
        pairs = _request(f"{DEX_BASE}/token-pairs/v1/solana/{mint}", None, timeout)
        exact = [p for p in (pairs or []) if p.get("chainId") == "solana" and (p.get("baseToken") or {}).get("address") == mint]
        pair = max(exact, key=lambda p: float((p.get("liquidity") or {}).get("usd") or 0), default=None)
        out = dict(item, observed_at_utc=observed, received_at_utc=prov.utc_now(), rpc=rpc, dex={"pair": pair} if pair else {})
    except Exception as exc:
        out = dict(item, observed_at_utc=observed, rpc=rpc, dex={}, collection_error=type(exc).__name__)
    return out


def write_csv(path: Path, rows: list[dict[str, str]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="") as f:
        w=csv.DictWriter(f, fieldnames=COLUMNS); w.writeheader(); w.writerows(rows)


def write_report(path: Path, run_id: str, sampling: dict[str, Any], rows: list[dict[str,str]], synthetic: bool) -> None:
    failures=sum(r["discrepancy_code"] != "NONE" for r in rows); unknown=sum("UNKNOWN" in (r[c] for c in ("mint_match","price_basis_match","supply_match","holder_semantics_valid","freshness_ok")) for r in rows)
    examples=rows[:10]
    text=f"""# DATA_ACCURACY\n\n## Run provenance\n- run_id: `{run_id}`\n- observed_at_utc: `{prov.utc_now()}`\n- synthetic: `{str(synthetic).lower()}`\n- code_version: `{prov.code_version()}`\n- denominator: {len(rows)} (collection failures retained: {sampling['failed_in_denominator']})\n\n## Sampling frame\n- required strata: {', '.join(STRATA)}\n- counts: `{json.dumps(sampling['counts'], sort_keys=True)}`\n- minimum_100: {sampling['minimum_100']}\n- each_stratum_minimum_25: {sampling['strata_minimum_25']}\n\n## Accuracy summary\n- rows_with_discrepancy_or_insufficient_data: {failures}\n- rows_with_unknown_verifiability: {unknown}\n- UNKNOWN and DATA_TIDAK_CUKUP are never interpreted as safe or valid.\n\n## Confusion / error taxonomy\n| Code | Meaning | Treatment |\n|---|---|---|\n| INVALID_MINT | Not base58 32-byte mint | Invalid |\n| WRONG_TOKEN_PROGRAM | Account owner is not SPL/Token-2022 | Invalid |\n| CHAIN_MISMATCH | DEX chain is not Solana | Invalid |\n| TARGET_NOT_DEX_BASE | Target is quote/other token | Reject priceUsd and mint match |\n| PRICE_MISSING | Base-token USD price absent | UNKNOWN, not zero |\n| HOLDER_SEMANTICS_INVALID | Token accounts represented as unique owners | Invalid |
| AUTHORITY_DATA_MISSING | Mint/freeze authority was not observed (null values are valid observations) | UNKNOWN, never safe |\n| STALE | Observation exceeds freshness limit | Review |\n| COLLECTION_FAILED | Provider request failed | Retain in denominator |\n| DATA_TIDAK_CUKUP | Critical field missing | UNKNOWN, never safe |\n\n## Ten manual audit examples\n| # | mint | pair | mint | price basis | holders | discrepancy |\n|---:|---|---|---|---|---|---|\n"""
    for i,r in enumerate(examples,1): text += f"| {i} | {r['mint']} | {r['dex_pair']} | {r['mint_match']} | {r['price_basis_match']} | {r['holder_semantics_valid']} | {r['discrepancy_code']} |\n"
    text += "\n## Interpretation constraints\nDEX Screener `priceUsd` is accepted only for an exact Solana pair whose base-token mint equals the sampled mint. Largest accounts remain token accounts; owner and beneficial-owner concentration are separate and UNKNOWN unless independently proven. Null authority alone never establishes safety.\n"
    path.parent.mkdir(parents=True, exist_ok=True); path.write_text(text, encoding="utf-8", newline="\n")


def main(argv: list[str] | None=None) -> int:
    p=argparse.ArgumentParser(); p.add_argument("--input"); p.add_argument("--build-frame",action="store_true"); p.add_argument("--frame-output",default="validation/normalized/p2_sampling_frame.json"); p.add_argument("--shortfall-output",default="validation/reports/p2_sampling_shortfall.json"); p.add_argument("--candidate-limit",type=int,default=750); p.add_argument("--live", action="store_true"); p.add_argument("--rpc-delay",type=float,default=.35); p.add_argument("--rpc-url", default=os.environ.get("SOLANA_RPC_URL") or DEFAULT_RPC); p.add_argument("--timeout",type=float,default=12); p.add_argument("--csv",default="validation/normalized/audit_100_tokens.csv"); p.add_argument("--report",default="validation/reports/DATA_ACCURACY.md"); args=p.parse_args(argv)
    if args.build_frame:
        observed=prov.utc_now(); profiles,documented,urls=fetch_sampling_sources(args.timeout)
        # Bound pair requests while retaining source order; profiles are always included.
        documented=documented[:max(100,args.candidate_limit)]
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
    if args.live: samples=[collect_live(x,args.rpc_url,args.timeout,args.rpc_delay) for x in samples]
    sampling=validate_sampling(samples); rows=[normalize(x) for x in samples]
    write_csv(ROOT/args.csv,rows); write_report(ROOT/args.report,run_id,sampling,rows,not args.live)
    manifest={"run_id":run_id,"stage":"p2","observed_at_utc":prov.utc_now(),"synthetic":not args.live,"input":args.input,"csv":args.csv,"report":args.report,"sampling":sampling,"rpc_endpoint":redact_url(args.rpc_url) if args.live else None}
    serial=json.dumps(redact_secrets(manifest)); assert_no_secrets(serial,forbidden=(os.environ.get("SOLANA_RPC_URL") or "",)); prov.append_manifest_line(ROOT/"validation/manifests/p2_evidence.jsonl",json.loads(serial))
    print(f"rows={len(rows)} failures_retained={sampling['failed_in_denominator']} csv={args.csv} report={args.report}")
    return 0 if sampling["minimum_100"] and sampling["strata_minimum_25"] else 2

if __name__ == "__main__": raise SystemExit(main())
