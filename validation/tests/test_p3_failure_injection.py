"""P3 failure-injection acceptance tests: deterministic, offline, and read-only."""
from __future__ import annotations
import json, pathlib, sqlite3, sys, tempfile, unittest
ROOT = pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "validation" / "bin"))
sys.path.insert(0, str(ROOT / "validation_work" / "meme-token-skill"))
import p3_failure_injection as fi  # noqa: E402


class P3FaultInjection(unittest.TestCase):
    def test_all_16_runbook_cases_have_executable_assertions(self):
        self.assertEqual(tuple(fi.CASES), tuple(range(1, 17)))
        results = fi.run_cases()
        self.assertEqual(len(results), 16)
        self.assertTrue(all(x["passed"] for x in results), results)
        self.assertEqual({x["case"] for x in results}, set(range(1, 17)))

    def test_metrics_contract_and_gaps_are_not_quality_pass(self):
        results = fi.run_cases()
        required = {"candidate_count", "processed_count", "failed_count", "provider_requests", "error_class", "duration"}
        for result in results:
            self.assertTrue(required <= result["metrics"].keys(), result)
            self.assertGreaterEqual(result["metrics"]["duration"], 0)
        gaps = next(x for x in results if x["case"] == 2)
        self.assertEqual(gaps["status"], "COMPLETED_WITH_GAPS")
        self.assertNotEqual(gaps["quality_verdict"], "PASS")

    def test_resume_is_idempotent_and_does_not_whiten_old_failure(self):
        result = fi.run_case(14)
        self.assertTrue(result["passed"])
        self.assertEqual(result["evidence"]["records_after"], result["evidence"]["records_after_second_resume"])
        self.assertEqual(result["evidence"]["failure_before"], result["evidence"]["failure_after"])
        self.assertEqual(result["evidence"]["duplicate_records"], 0)

    def test_harness_has_no_transaction_signing_or_write_transport_action(self):
        source = pathlib.Path(fi.__file__).read_text(encoding="utf-8").lower()
        for forbidden in ("sendtransaction", "signtransaction", "sign_message", "eth_send", "walletprivatekey"):
            self.assertNotIn(forbidden, source)
        results = fi.run_cases()
        self.assertTrue(all(x["evidence"].get("write_actions", 0) == 0 for x in results))

    def test_report_and_json_evidence_generation(self):
        with tempfile.TemporaryDirectory() as d:
            report, evidence = fi.generate(pathlib.Path(d))
            text = report.read_text(encoding="utf-8")
            payload = json.loads(evidence.read_text(encoding="utf-8"))
            self.assertIn("# P3 Failure Injection", text)
            self.assertIn("16/16", text)
            self.assertEqual(len(payload["cases"]), 16)
            self.assertTrue(payload["offline"])
            self.assertFalse(payload["live_network_used"])


if __name__ == "__main__": unittest.main()
