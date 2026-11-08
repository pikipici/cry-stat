"""Run the full validation harness: P0 checks that must pass before live data.

Exit code 0 means every gate below is green. Any failure exits non-zero so CI
cannot mistake a broken harness for a passing baseline.
"""
from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
VALIDATION = ROOT / "validation"

GATES = (
    ("project unit tests", ["python", "-m", "unittest", "discover", "-s", "tests"]),
    ("skill validator", ["python", "scripts/validate_skill.py"]),
    ("validation harness tests", ["python", "-m", "unittest", "discover", "-s", "validation/tests"]),
)


def run_gate(name: str, argv: list[str]) -> dict:
    result = subprocess.run(argv, cwd=ROOT, capture_output=True, text=True)
    tail = (result.stdout + result.stderr). strip().splitlines()[-3:]
    return {
        "gate": name,
        "command": " ".join(argv),
        "exit_code": result.returncode,
        "ok": result.returncode == 0,
        "tail": tail,
    }


def main() -> int:
    sys.path.insert(0, str(VALIDATION / "bin"))
    import provenance as prov  # noqa: E402

    run_id = prov.new_run_id("p0")
    results = [run_gate(name, argv) for name, argv in GATES]

    report = prov.evidence_envelope(
        run_id,
        synthetic=False,
        stage="p0",
        payload={"gates": results, "all_passed": all(r["ok"] for r in results)},
    )
    out = Path("validation/reports") / f"p0_harness_{run_id}.json"
    prov.atomic_write_json(ROOT / out, report)

    for item in results:
        status = "PASS" if item["ok"] else "FAIL"
        print(f"[{status}] {item['gate']}  (exit {item['exit_code']})")
        for line in item["tail"]:
            if line.strip():
                print(f"        {line.strip()}")
    print(f"\nrun_id={run_id}")
    print(f"report={out.as_posix()}")
    return 0 if all(r["ok"] for r in results) else 1


if __name__ == "__main__":
    raise SystemExit(main())