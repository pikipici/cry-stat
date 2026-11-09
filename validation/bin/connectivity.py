"""P1 — live API connectivity acceptance.

Proves the pipeline actually reaches Solana Mainnet and DEX Screener, measures
response latency, and shows that timeout / 429 handling preserves provenance.
Exit code 0 only when every locked threshold in config.example.json is met.

Read-only: no transaction is built, signed, or sent. Nothing here touches a key.
"""
from __future__ import annotations

import argparse
import json
import os
import statistics
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "validation" / "bin"))

import provenance as prov  # noqa: E402
from redact import assert_no_secrets, redact_secrets, redact_url  # noqa: E402

DEX_BASE = "https://api.dexscreener.com"
DEFAULT_RPC = "https://api.mainnet-beta.solana.com"
USER_AGENT = "MemeTokenResearchValidation/1.0 (read-only connectivity probe)"


class ProbeError(Exception):
    """A single probe failed in a way that must be recorded, not swallowed."""


class Result:
    """One attempt against one endpoint, with everything needed to audit it."""

    def __init__(self, name: str, provider: str, endpoint: str) -> None:
        self.name = name
        self.provider = provider
        self.endpoint = endpoint
        self.ok = False
        self.latency_ms: float | None = None
        self.http_status: int | None = None
        self.error_kind: str | None = None
        self.error_detail: str | None = None
        self.payload_sha256: str | None = None
        self.retry_after: str | None = None

    def to_dict(self) -> dict[str, Any]:
        return {
            "probe": self.name,
            "provider": self.provider,
            "endpoint": redact_url(self.endpoint),
            "ok": self.ok,
            "latency_ms": None if self.latency_ms is None else round(self.latency_ms, 3),
            "http_status": self.http_status,
            "error_kind": self.error_kind,
            "error_detail": self.error_detail,
            "payload_sha256": self.payload_sha256,
            "retry_after": self.retry_after,
        }


def probe(
    name: str,
    provider: str,
    url: str,
    *,
    payload: dict[str, Any] | None = None,
    timeout: float = 12.0,
    max_retries: int = 2,
) -> Result:
    """One read-only request with bounded retries and full provenance capture."""
    result = Result(name, provider, url)
    if not url.startswith("https://"):
        result.error_kind = "insecure_endpoint"
        result.error_detail = "only HTTPS endpoints are permitted"
        return result

    method = "POST" if payload is not None else "GET"
    body = json.dumps(payload, separators=(",", ":")).encode("utf-8") if payload is not None else None
    request = urllib.request.Request(
        url,
        method=method,
        data=body,
        headers={
            "Accept": "application/json",
            "Content-Type": "application/json",
            "User-Agent": USER_AGENT,
        },
    )

    attempts = 0
    for attempt in range(max_retries + 1):
        attempts += 1
        started = time.perf_counter()
        try:
            with urllib.request.urlopen(request, timeout=timeout) as response:
                raw = response.read(3_000_001)
                elapsed = (time.perf_counter() - started) * 1000
                if len(raw) > 3_000_000:
                    result.error_kind = "response_too_large"
                    result.error_detail = "body exceeds 3 MB"
                    result.latency_ms = elapsed
                    return result
                result.http_status = response.status
                result.payload_sha256 = _sha256(raw)
                result.latency_ms = elapsed
                # Prove the body parses; a 200 with broken JSON is not success.
                try:
                    json.loads(raw.decode("utf-8"))
                except (UnicodeError, ValueError) as exc:
                    result.error_kind = "invalid_json"
                    result.error_detail = type(exc).__name__
                    return result
                result.ok = True
                if attempt > 0:
                    result.error_detail = f"succeeded after {attempt} retr{'y' if attempt == 1 else 'ies'}"
                return result
        except urllib.error.HTTPError as exc:
            elapsed = (time.perf_counter() - started) * 1000
            result.latency_ms = elapsed
            result.http_status = exc.code
            retry_after = exc.headers.get("Retry-After") if exc.headers else None
            result.retry_after = retry_after
            retryable = exc.code in (429, 500, 502, 503, 504)
            if not retryable or attempt == max_retries:
                result.error_kind = "http_error"
                result.error_detail = f"HTTP {exc.code} after {attempts} attempt(s)"
                return result
            try:
                wait = min(3.0, max(0.0, float(retry_after))) if retry_after is not None else 0.45 * 2**attempt
            except (TypeError, ValueError):
                wait = 0.45 * 2**attempt
            time.sleep(wait)
        except (urllib.error.URLError, TimeoutError, OSError) as exc:
            elapsed = (time.perf_counter() - started) * 1000
            result.latency_ms = elapsed
            if attempt == max_retries:
                result.error_kind = "network_error"
                result.error_detail = f"{type(exc).__name__} after {attempts} attempt(s)"
                return result
            time.sleep(0.45 * 2**attempt)
    result.error_kind = "retry_exhausted"
    return result


def _sha256(raw: bytes) -> str:
    import hashlib
    return hashlib.sha256(raw).hexdigest()


def solana_probe(name: str, rpc_url: str, method: str, params: list[Any], timeout: float, retries: int) -> Result:
    return probe(
        name,
        "solana-rpc",
        rpc_url,
        payload={"jsonrpc": "2.0", "id": 1, "method": method, "params": params},
        timeout=timeout,
        max_retries=retries,
    )


def summary(results: list[Result], thresholds: dict[str, Any]) -> dict[str, Any]:
    successes = [r for r in results if r.ok]
    latencies = sorted(r.latency_ms for r in successes if r.latency_ms is not None)
    total = len(results)
    error_rate = (total - len(successes)) / total if total else 1.0

    p50 = statistics.median(latencies) if latencies else None
    p95 = _percentile(latencies, 95) if latencies else None

    checks = {
        "min_successful_calls": len(successes) >= thresholds["min_successful_calls"],
        "max_error_rate": error_rate <= thresholds["max_error_rate"],
        "max_p95_latency_ms": (p95 is not None and p95 <= thresholds["max_p95_latency_ms"]),
    }
    return {
        "total_probes": total,
        "successful": len(successes),
        "failed": total - len(successes),
        "error_rate": round(error_rate, 4),
        "p50_latency_ms": None if p50 is None else round(p50, 3),
        "p95_latency_ms": None if p95 is None else round(p95, 3),
        "checks": checks,
        "all_thresholds_met": all(checks.values()),
        "thresholds_used": thresholds,
    }


def _percentile(sorted_values: list[float], pct: float) -> float:
    """Nearest-rank percentile; small samples make interpolation dishonest."""
    if not sorted_values:
        raise ValueError("no values")
    k = max(0, min(len(sorted_values) - 1, int(round((pct / 100) * len(sorted_values))) - 1))
    return sorted_values[k]


def failure_path_probe(timeout: float) -> dict[str, Any]:
    """Demonstrate that a bad endpoint fails with provenance intact.

    Two cases: a DNS/connection failure and a deliberately impossible URL on the
    real provider. Neither leaks a secret, and both must return a structured
    Result rather than raising.
    """
    out: dict[str, Any] = {}

    dns_result = probe("failure_dns_unresolvable", "negative-control",
                       "https://this-host-does-not-exist.invalid/rpc", timeout=timeout, max_retries=0)
    out["dns_failure"] = dns_result.to_dict()

    timeout_result = probe("failure_timeout_path", "negative-control",
                           "https://api.dexscreener.com/token-profiles/latest/v1",
                           timeout=0.001, max_retries=0)
    out["timeout_path"] = timeout_result.to_dict()

    insecure = probe("failure_insecure_scheme", "negative-control",
                     "http://api.mainnet-beta.solana.com", timeout=timeout, max_retries=0)
    out["insecure_scheme"] = insecure.to_dict()

    return out


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="P1 live connectivity acceptance probe")
    parser.add_argument("--config", default="validation/config.example.json", help="locked threshold config")
    parser.add_argument("--rpc-url", default=os.environ.get("SOLANA_RPC_URL") or DEFAULT_RPC)
    parser.add_argument("--mint", default=os.environ.get("MINT_VALID", "So11111111111111111111111111111111111111112"))
    parser.add_argument("--repeats", type=int, default=11,
                        help="probes per Solana method; 11 gives 13 Solana + 12 DEX = 25 "
                             "total, satisfying the locked min_successful_calls of 20")
    parser.add_argument("--out", default="validation/reports/p1-connectivity")
    args = parser.parse_args(argv)

    config_path = ROOT / args.config
    config = json.loads(config_path.read_text(encoding="utf-8"))
    thresholds = config["p1_connectivity"]
    timeout = float(thresholds.get("timeout_s", 12))
    retries = int(thresholds.get("max_retries", 2))

    run_id = prov.new_run_id("p1")
    results: list[Result] = []

    # Solana RPC: repeat a cheap read to get a latency distribution.
    for i in range(args.repeats):
        results.append(solana_probe(f"getAccountInfo#{i+1}", args.rpc_url, "getAccountInfo",
                                    [args.mint, {"encoding": "jsonParsed", "commitment": "confirmed"}],
                                    timeout, retries))
    results.append(solana_probe("getTokenSupply", args.rpc_url, "getTokenSupply",
                               [args.mint, {"commitment": "confirmed"}], timeout, retries))
    results.append(solana_probe("getTokenLargestAccounts", args.rpc_url, "getTokenLargestAccounts",
                               [args.mint, {"commitment": "confirmed"}], timeout, retries))

    # DEX Screener: both endpoints the pipeline uses.
    for i in range(args.repeats):
        results.append(probe(f"token-pairs#{i+1}", "dexscreener",
                            f"{DEX_BASE}/token-pairs/v1/solana/{args.mint}", timeout=timeout, max_retries=retries))
    results.append(probe("token-profiles-latest", "dexscreener",
                        f"{DEX_BASE}/token-profiles/latest/v1", timeout=timeout, max_retries=retries))

    agg = summary(results, thresholds)
    negative = failure_path_probe(timeout)

    payload = {
        "rpc_endpoint": redact_url(args.rpc_url),
        "rpc_is_public_default": args.rpc_url == DEFAULT_RPC,
        "mint": args.mint,
        "probes": [r.to_dict() for r in results],
        "aggregate": agg,
        "negative_controls": negative,
    }
    envelope = prov.evidence_envelope(run_id, synthetic=False, stage="p1", payload=payload)

    # Last line of defence: refuse to write anything that still looks secret.
    serialised = json.dumps(envelope, indent=2, sort_keys=True, ensure_ascii=False)
    assert_no_secrets(serialised, forbidden=(os.environ.get("SOLANA_RPC_URL") or "",))

    out_dir = ROOT / args.out
    out_dir.mkdir(parents=True, exist_ok=True)
    report_path = out_dir / f"p1_connectivity_{run_id}.json"
    prov.atomic_write_json(report_path, redact_secrets(envelope))
    prov.append_manifest_line(ROOT / "validation" / "manifests" / "p1_evidence.jsonl", {
        "run_id": run_id,
        "stage": "p1",
        "observed_at_utc": envelope["observed_at_utc"],
        "report": str(report_path.relative_to(ROOT)).replace("\\", "/"),
        "all_thresholds_met": agg["all_thresholds_met"],
        "successful": agg["successful"],
        "total": agg["total_probes"],
    })

    _print_summary(run_id, args.rpc_url, results, agg, report_path)
    return 0 if agg["all_thresholds_met"] else 1


def _print_summary(run_id: str, rpc_url: str, results: list[Result], agg: dict[str, Any], path: Path) -> None:
    print(f"run_id={run_id}")
    print(f"rpc={redact_url(rpc_url)}")
    print()
    by_provider: dict[str, list[Result]] = {}
    for r in results:
        by_provider.setdefault(r.provider, []).append(r)
    for provider, items in by_provider.items():
        ok = sum(1 for i in items if i.ok)
        print(f"{provider}: {ok}/{len(items)} ok")
        for item in items:
            mark = "ok  " if item.ok else "FAIL"
            latency = f"{item.latency_ms:.0f}ms" if item.latency_ms is not None else "n/a"
            detail = f" {item.error_kind}: {item.error_detail}" if item.error_kind else ""
            print(f"  [{mark}] {item.name:24s} {latency:>9s} {detail}")
    print()
    print(f"error_rate={agg['error_rate']:.2%}  p50={agg['p50_latency_ms']}ms  p95={agg['p95_latency_ms']}ms")
    for check, passed in agg["checks"].items():
        print(f"  [{'PASS' if passed else 'FAIL'}] {check}")
    print()
    print(f"report={path.relative_to(ROOT).as_posix()}")
    print(f"VERDICT: {'PASS' if agg['all_thresholds_met'] else 'FAIL'}")


if __name__ == "__main__":
    raise SystemExit(main())