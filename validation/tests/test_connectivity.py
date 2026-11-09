"""Tests for validation/bin/connectivity.py — P1 threshold and statistics logic.

No network is touched: these exercise the pure aggregation, percentile, and
result-serialisation paths that decide PASS/FAIL.
"""
from __future__ import annotations

import pathlib
import sys
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "validation" / "bin"))

import connectivity as conn  # noqa: E402

THRESHOLDS = {
    "min_successful_calls": 20,
    "max_error_rate": 0.05,
    "max_p95_latency_ms": 4000,
}


def ok_result(name: str, latency: float) -> conn.Result:
    r = conn.Result(name, "solana-rpc", "https://api.mainnet-beta.solana.com")
    r.ok = True
    r.latency_ms = latency
    r.http_status = 200
    r.payload_sha256 = "0" * 64
    return r


def bad_result(name: str, latency: float = 80.0) -> conn.Result:
    r = conn.Result(name, "solana-rpc", "https://api.mainnet-beta.solana.com")
    r.latency_ms = latency
    r.http_status = 429
    r.error_kind = "http_error"
    r.error_detail = "HTTP 429 after 3 attempt(s)"
    return r


class PercentileTests(unittest.TestCase):
    def test_nearest_rank_on_small_sample(self):
        values = sorted([10.0, 20.0, 30.0, 40.0])
        self.assertEqual(conn._percentile(values, 50), 20.0)
        self.assertEqual(conn._percentile(values, 95), 40.0)

    def test_single_value(self):
        self.assertEqual(conn._percentile([42.0], 95), 42.0)

    def test_empty_raises(self):
        with self.assertRaises(ValueError):
            conn._percentile([], 95)


class SummaryTests(unittest.TestCase):
    def test_all_good_passes(self):
        results = [ok_result(f"p{i}", 90.0 + i) for i in range(25)]
        agg = conn.summary(results, THRESHOLDS)
        self.assertTrue(agg["all_thresholds_met"])
        self.assertEqual(agg["successful"], 25)
        self.assertEqual(agg["error_rate"], 0.0)

    def test_too_few_calls_fails(self):
        results = [ok_result(f"p{i}", 90.0) for i in range(15)]
        agg = conn.summary(results, THRESHOLDS)
        self.assertFalse(agg["checks"]["min_successful_calls"])
        self.assertFalse(agg["all_thresholds_met"])

    def test_error_rate_above_ceiling_fails(self):
        results = [ok_result(f"p{i}", 90.0) for i in range(18)]
        results += [bad_result(f"b{i}") for i in range(2)]  # 10 %
        agg = conn.summary(results, THRESHOLDS)
        self.assertFalse(agg["checks"]["max_error_rate"])
        self.assertFalse(agg["all_thresholds_met"])

    def test_one_failure_in_twenty_five_is_under_ceiling(self):
        results = [ok_result(f"p{i}", 90.0) for i in range(24)]
        results.append(bad_result("b0"))
        agg = conn.summary(results, THRESHOLDS)
        self.assertEqual(agg["error_rate"], 0.04)
        self.assertTrue(agg["checks"]["max_error_rate"])

    def test_slow_p95_fails_latency_check(self):
        results = [ok_result(f"p{i}", 5000.0) for i in range(25)]
        agg = conn.summary(results, THRESHOLDS)
        self.assertFalse(agg["checks"]["max_p95_latency_ms"])
        self.assertFalse(agg["all_thresholds_met"])

    def test_latency_ignores_failed_probes(self):
        results = [ok_result(f"p{i}", 100.0) for i in range(24)]
        results.append(bad_result("b0", latency=9999.0))
        agg = conn.summary(results, THRESHOLDS)
        self.assertEqual(agg["p95_latency_ms"], 100.0)

    def test_empty_results_cannot_pass(self):
        agg = conn.summary([], THRESHOLDS)
        self.assertFalse(agg["all_thresholds_met"])
        self.assertEqual(agg["error_rate"], 1.0)


class ResultSerialisationTests(unittest.TestCase):
    def test_endpoint_is_redacted_in_output(self):
        r = conn.Result("x", "solana-rpc", "https://rpc.example.com/?api-key=SECRETVALUE123")
        out = r.to_dict()
        self.assertNotIn("SECRETVALUE123", out["endpoint"])
        self.assertIn("[REDACTED]", out["endpoint"])

    def test_failed_probe_keeps_status_and_error(self):
        out = bad_result("x").to_dict()
        self.assertFalse(out["ok"])
        self.assertEqual(out["http_status"], 429)
        self.assertEqual(out["error_kind"], "http_error")

    def test_latency_rounded(self):
        r = ok_result("x", 12.3456789)
        self.assertEqual(r.to_dict()["latency_ms"], 12.346)


class InsecureSchemeTests(unittest.TestCase):
    def test_plaintext_scheme_rejected_without_network(self):
        result = conn.probe("x", "negative-control", "http://api.mainnet-beta.solana.com",
                            timeout=1.0, max_retries=0)
        self.assertFalse(result.ok)
        self.assertEqual(result.error_kind, "insecure_endpoint")
        self.assertIsNone(result.latency_ms, "must fail before any timing/connection")


if __name__ == "__main__":
    unittest.main()