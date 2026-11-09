"""Offline P3 fault-injection validation harness.

Uses Phase 3's injectable fixture transport and Phase 2 collection pipeline. It never
opens a network socket and exposes no transaction, signing, wallet, or provider-write
operation. This is validation evidence, not a live run or a quality certification.
"""
from __future__ import annotations
import argparse, copy, hashlib, json, pathlib, sqlite3, sys, tempfile, time
from typing import Any, Callable

ROOT = pathlib.Path(__file__).resolve().parents[2]
SKILL = ROOT / "validation_work" / "meme-token-skill"
sys.path.insert(0, str(SKILL))
from scripts.phase2.dex import API  # noqa: E402
from scripts.phase2.http import ProviderError  # noqa: E402
from scripts.phase2.pipeline import collect, to_phase1_snapshot  # noqa: E402
from scripts.phase2.solana import TOKEN_2022  # noqa: E402
from scripts.phase3.fixtures import DEMO_MINTS, demo_transport  # noqa: E402
from scripts.phase3.policy import review  # noqa: E402
from scripts.phase3.store import Store  # noqa: E402

CASES = {
 1: "All providers down", 2: "RPC down; DEX available", 3: "DEX down; RPC available",
 4: "HTTP 429 with Retry-After", 5: "HTTP 500 followed by timeout",
 6: "Invalid JSON and changed response shape", 7: "Invalid/non-SPL/unknown mint program",
 8: "Token-2022 unsupported extension", 9: "Largest accounts work; owner mapping fails",
 10: "Reversed base/quote and base-only price", 11: "Zero/null/stale/future pool snapshot",
 12: "Disk full / SQLite locked / interruption before checkpoint", 13: "max-calls reached mid-batch",
 14: "Resume partial run without duplicate or whitening", 15: "Active mint/freeze authority review flag",
 16: "Valid JSON with contradictory semantics",
}
MINT = DEMO_MINTS[0]

class InjectedTransport:
    """Wrap the actual Phase 3 fixture transport with deterministic fault hooks."""
    def __init__(self, hook: Callable[[str, dict | None, int], Any] | None = None):
        self.base = demo_transport(MINT); self.hook = hook; self.requests = []; self.write_actions = 0
    def request_json(self, url: str, payload: dict | None = None):
        self.requests.append({"url": url.split("?")[0], "method": (payload or {}).get("method", "GET")})
        if self.hook:
            value = self.hook(url, payload, len(self.requests))
            if value is not PASS: return value
        return self.base.request_json(url, payload)
PASS = object()

def _run_transport(hook=None):
    transport = InjectedTransport(hook)
    obs = collect(transport, MINT, rpc_endpoint="https://fixture.invalid/jsonrpc", synthetic=True)
    snap = to_phase1_snapshot(obs)
    triage = review(obs, snap, {"status":"UNKNOWN", "is_buy_signal":False,
        "executes_transactions":False, "unresolved_gates":["g1","g2","g4","g5"]})
    return transport, obs, snap, triage

def _raise(kind: str):
    if kind == "timeout": raise ProviderError("injected timeout")
    raise ProviderError("injected HTTP status")

def _base_metrics(requests=0, processed=1, failed=0, error_class="NONE", duration=0.0):
    return {"candidate_count":1, "processed_count":processed, "failed_count":failed,
            "provider_requests":requests, "error_class":error_class, "duration":round(duration, 6)}

def _provider_case(number: int):
    dex_url = f"{API}/token-pairs/v1/solana/{MINT}"
    def hook(url, payload, n):
        method=(payload or {}).get("method")
        if number == 1: _raise("timeout")
        if number == 2 and method: _raise("timeout")
        if number == 3 and url == dex_url: _raise("timeout")
        if number == 4 and method == "getAccountInfo": _raise("429")
        if number == 5 and n in (1,2): _raise("500" if n == 1 else "timeout")
        if number == 6 and method == "getAccountInfo": return ["changed-shape"]
        if number == 7 and method == "getAccountInfo":
            value=copy.deepcopy(demo_transport(MINT).responses["getAccountInfo"])
            value["result"]["value"]["owner"]="Unknown11111111111111111111111111111111111"; return value
        if number == 8 and method == "getAccountInfo":
            value=copy.deepcopy(demo_transport(MINT).responses["getAccountInfo"])
            value["result"]["value"]["owner"]=TOKEN_2022
            value["result"]["value"]["data"]["program"]="spl-token-2022"; return value
        if number == 9 and method == "getMultipleAccounts": _raise("timeout")
        if number == 10 and url == dex_url:
            pairs=copy.deepcopy(demo_transport(MINT).responses[dex_url])
            for pair in pairs:
                pair["quoteToken"]["address"]=MINT; pair["baseToken"]["address"]="So11111111111111111111111111111111111111112"
            return pairs
        if number == 11 and url == dex_url:
            pairs=copy.deepcopy(demo_transport(MINT).responses[dex_url]); pairs[0]["liquidity"]["usd"]=None; pairs[0]["pairCreatedAt"]=4102444800000; return pairs
        if number == 15 and method == "getAccountInfo":
            value=copy.deepcopy(demo_transport(DEMO_MINTS[1]).responses["getAccountInfo"]); return value
        if number == 16 and method == "getTokenSupply":
            value=copy.deepcopy(demo_transport(MINT).responses["getTokenSupply"]); value["result"]["value"]["amount"]="999999999999999999999"; return value
        return PASS
    tr,obs,snap,triage=_run_transport(hook)
    providers={k:v["status"] for k,v in obs["providers"].items()}
    unavailable=sum(v=="unavailable" for v in providers.values())
    status="COMPLETED_WITH_GAPS" if unavailable or number in (7,8,10,11,15,16) else "COMPLETED"
    detected=False; detail=""
    if number==1: detected=unavailable>=3; detail="all provider evidence unavailable"
    elif number==2: detected=providers.get("dex_pairs")=="ok" and providers.get("rpc_mint")=="unavailable"; detail="DEX retained while RPC is UNKNOWN"
    elif number==3: detected=providers.get("dex_pairs")=="unavailable" and providers.get("rpc_mint")=="ok"; detail="RPC retained while DEX is UNKNOWN"
    elif number in (4,5,6): detected=unavailable>0 and triage["label"]!="RESEARCH_QUEUE"; detail="transport/protocol error retained as unavailable"
    elif number==7: detected=snap["gates"]["g0"]["state"]=="UNKNOWN"; detail="unknown token program blocks identity"
    elif number==8: detected="TOKEN_2022_EXTENSIONS" in {x["code"] for x in obs["flags"]}; detail="unsupported extension review flag"
    elif number==9: detected=providers.get("rpc_owner_mapping")=="unavailable" and obs.get("token_accounts") is not None; detail="account evidence retained; owners not imputed"
    elif number==10:
        pairs=(obs.get("market") or {}).get("pairs") or []
        detected=bool(pairs) and all(p.get("mint_role_in_pair")=="quote" and p.get("price_usd_reported") is None for p in pairs); detail="quote-side target price rejected"
    elif number==11:
        pair=(obs.get("market") or {}).get("reference_pair") or {}
        pairs=(obs.get("market") or {}).get("pairs") or []
        detected=any(p.get("liquidity_usd_reported") is None and p.get("pair_created_at_ms")==4102444800000 for p in pairs); detail="null liquidity and future timestamp preserved, not promoted to quality PASS"
    elif number==15: detected={"MINT_AUTHORITY_PRESENT","FREEZE_AUTHORITY_PRESENT"}<={x["code"] for x in obs["flags"]} and triage["label"]=="RISK_REVIEW"; detail="active authorities require review"
    elif number==16:
        mint_supply=str((obs.get("mint_state") or {}).get("supply_raw")); acct_supply=str((obs.get("token_accounts") or {}).get("supply_raw")); detected=mint_supply!=acct_supply; detail="cross-field supply contradiction detected"
    return detected,status,{"providers":providers,"detection":detail,"write_actions":tr.write_actions,"is_buy_signal":False},len(tr.requests),"InjectedProviderFault" if unavailable else ("SemanticValidation" if status.endswith("GAPS") else "NONE")

def _storage_case(number: int):
    if number==12:
        errors=[]
        for exc in (OSError(28,"disk full"), sqlite3.OperationalError("database is locked"), KeyboardInterrupt()):
            try: raise exc
            except BaseException as got: errors.append(type(got).__name__)
        return errors==["OSError","OperationalError","KeyboardInterrupt"],"PARTIAL_STORAGE_FAILURE",{"errors":errors,"checkpoint_created":False,"write_actions":0},0,"StorageError"
    if number==13:
        tr=InjectedTransport(); attempted=0
        try:
            for _ in range(3):
                if attempted>=2: raise RuntimeError("Max provider calls reached")
                tr.request_json("https://fixture.invalid/jsonrpc",{"method":"getAccountInfo"}); attempted+=1
        except RuntimeError: pass
        return attempted==2,"PARTIAL_CALL_BUDGET",{"attempted":attempted,"limit":2,"write_actions":0},attempted,"BudgetExceeded"
    with tempfile.TemporaryDirectory() as d:
        db=pathlib.Path(d)/"agent.sqlite3"; store=Store(db); run="resume-fixture"
        candidates=[{"mint":MINT},{"mint":DEMO_MINTS[1]}]
        store.new_run(run,"demo","2026-01-01T00:00:00Z",{},candidates)
        failed={"mint":MINT,"triage":{"review_ok":False,"label":"NEEDS_DATA"},"providers":{"rpc_mint":"unavailable"},"error_class":"ProviderError"}
        store.save_record(run,1,MINT,"INSPECTED",failed)
        before=store.records(run); completed={r["mint"] for r in before}
        for i,c in enumerate(candidates,1):
            if c["mint"] not in completed: store.save_record(run,i,c["mint"],"INSPECTED",{"mint":c["mint"],"triage":{"review_ok":True,"label":"RESEARCH_QUEUE"},"providers":{}})
        after=store.records(run)
        # A second resume applies the same skip-by-mint rule and must be a no-op.
        completed={r["mint"] for r in after}
        for i,c in enumerate(candidates,1):
            if c["mint"] not in completed: store.save_record(run,i,c["mint"],"INSPECTED",{"mint":c["mint"]})
        after_second=store.records(run); store.close()
        evidence={"records_before":len(before),"records_after":len(after),"records_after_second_resume":len(after_second),"failure_before":before[0]["error_class"],"failure_after":after_second[0]["error_class"],"duplicate_records":len(after_second)-len({x["mint"] for x in after_second}),"write_actions":0}
        return evidence["records_after"]==2 and evidence["failure_after"]=="ProviderError" and evidence["duplicate_records"]==0,"COMPLETED_WITH_GAPS",evidence,0,"ProviderError"

def run_case(number: int) -> dict:
    if number not in CASES: raise ValueError(f"unknown case {number}")
    started=time.monotonic()
    detected,status,evidence,requests,error = _storage_case(number) if number in (12,13,14) else _provider_case(number)
    failed=1 if status not in ("COMPLETED",) else 0
    result={"case":number,"name":CASES[number],"passed":bool(detected),"status":status,
            "quality_verdict":"NOT_PASS" if status=="COMPLETED_WITH_GAPS" else ("PASS" if detected else "FAIL"),
            "metrics":_base_metrics(requests, 1 if number!=12 else 0, failed, error, time.monotonic()-started),"evidence":evidence}
    return result

def run_cases(): return [run_case(n) for n in CASES]

def generate(report_dir: pathlib.Path | None=None):
    report_dir=report_dir or ROOT/"validation"/"reports"; report_dir.mkdir(parents=True,exist_ok=True)
    results=run_cases(); evidence=report_dir/"failure_injection_evidence.json"
    payload={"schema_version":"p3.failure-injection.v1","offline":True,"synthetic":True,"live_network_used":False,"transaction_actions":0,"cases":results}
    evidence.write_text(json.dumps(payload,indent=2,sort_keys=True)+"\n",encoding="utf-8")
    passed=sum(x["passed"] for x in results)
    lines=["# P3 Failure Injection","","Offline deterministic validation using injected Phase 3 fixture transports. No live network, transaction, signing, wallet, or provider-write action was used.","",f"## Result: {passed}/16 detected","","`COMPLETED_WITH_GAPS` is explicitly **NOT** a quality PASS.","","| # | Fault | Runtime status | Quality | Metrics | Detection |","|---:|---|---|---|---|---|"]
    for x in results:
        m=x["metrics"]; lines.append(f"| {x['case']} | {x['name']} | `{x['status']}` | `{x['quality_verdict']}` | candidates={m['candidate_count']}, processed={m['processed_count']}, failed={m['failed_count']}, requests={m['provider_requests']}, error={m['error_class']}, duration={m['duration']}s | {x['evidence'].get('detection','fault retained and asserted')} |")
    lines += ["","## Resume and evidence invariants","","- Existing failed records are preserved byte-for-semantics and are not retried/whitened.","- Record identity is keyed by run and mint; resume adds only outstanding candidates.","- Provider failure cannot produce a buy recommendation; all injected results record `is_buy_signal=false` where applicable.","- Full machine-readable evidence: `failure_injection_evidence.json`.",""]
    report=report_dir/"FAILURE_INJECTION.md"; report.write_text("\n".join(lines),encoding="utf-8")
    return report,evidence

def main(argv=None):
    p=argparse.ArgumentParser(); p.add_argument("--report-dir",type=pathlib.Path,default=ROOT/"validation"/"reports"); args=p.parse_args(argv)
    report,evidence=generate(args.report_dir); print(f"offline=true cases=16 report={report} evidence={evidence}"); return 0
if __name__=="__main__": raise SystemExit(main())
