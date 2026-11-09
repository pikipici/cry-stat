"""Deterministic tests for the existing-live-evidence P3 acceptance report."""
from __future__ import annotations
import json, pathlib, sys, tempfile, unittest
ROOT=pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/"validation"/"bin"))
import p3_e2e_report as e2e  # noqa: E402

class P3E2EReport(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        reports=ROOT/"validation"/"reports"
        cls.data=e2e.build([reports/"e2e-mints-001",reports/"e2e-mints-002"],reports/"failure_injection_evidence.json")
    def test_live_metrics_and_gap_definition(self):
        required=e2e.METRICS
        for run in self.data["runs"]:
            self.assertTrue(required <= run["metrics"].keys())
            self.assertEqual(run["metrics"]["failed_count"],1)
            self.assertEqual(run["metrics"]["provider_requests"],15)
            self.assertEqual(run["quality_verdict"],"NOT_PASS")
        self.assertIn("provider unavailable",self.data["failed_count_definition"])
    def test_trees_sqlite_safety_and_repeat(self):
        for run in self.data["runs"]:
            self.assertTrue(run["checks"]["output_tree"])
            self.assertTrue(run["checks"]["sqlite_records_events"])
            self.assertTrue(run["checks"]["no_secret_leakage"])
            self.assertTrue(run["checks"]["no_buy_recommendation"])
            self.assertTrue(run["checks"]["no_transaction"])
        self.assertTrue(self.data["repeat"]["same_mints"])
        self.assertTrue(self.data["repeat"]["consistent_identity_schema_decision_unknown_and_evidence"])
    def test_acceptance_is_resilience_not_quality_whitening(self):
        self.assertEqual(self.data["verdict"],"PASS")
        self.assertTrue(all(self.data["criteria"].values()))
        self.assertEqual(self.data["fault_injection"]["detected"],16)
        self.assertEqual(self.data["fault_injection"]["evidence_loss_count"],0)
        self.assertEqual(self.data["fault_injection"]["provider_failure_to_buy_count"],0)
    def test_generation_is_deterministic(self):
        with tempfile.TemporaryDirectory() as d:
            target=pathlib.Path(d)
            # Generation takes evidence inputs from one directory, so render/build determinism is asserted directly.
            first=json.dumps(self.data,sort_keys=True)
            second=json.dumps(e2e.build([ROOT/"validation/reports/e2e-mints-001",ROOT/"validation/reports/e2e-mints-002"],ROOT/"validation/reports/failure_injection_evidence.json"),sort_keys=True)
            self.assertEqual(first,second)
            report=target/"AGENT_E2E.md"; report.write_text(e2e.render(self.data),encoding="utf-8")
            self.assertIn("P3 harness verdict: PASS",report.read_text(encoding="utf-8"))

if __name__=="__main__": unittest.main()
