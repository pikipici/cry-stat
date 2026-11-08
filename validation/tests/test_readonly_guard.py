"""P0 task 4 — static guard against write/signing transaction code.

The research pipeline must stay read-only. If anyone ever adds a call that could
sign or submit a transaction, this test fails loudly before it can reach a run.

Deliberately text-based rather than AST-based: a keyword can appear through
getattr, dynamic dispatch, or a string built at runtime, and a substring scan
catches all of those. False positives are resolved through the explicit
ALLOWLIST below, and every allowlist entry must state its reason.
"""
from __future__ import annotations

import pathlib
import re
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
SCAN_DIRS = ("scripts", "validation/bin")
SCAN_SUFFIXES = {".py"}

# Names that would allow signing, submitting, or funding a transaction.
FORBIDDEN = (
    "sendTransaction",
    "sendRawTransaction",
    "signTransaction",
    "signAllTransactions",
    "signMessage",
    "simulateTransaction",  # simulation can still leak intent and belongs to a different scope
    "Keypair",
    "from_secret_key",
    "from_string",           # solders Keypair constructor
    "seed_phrase",
    "mnemonic",
    "private_key",
    "secret_key",
    "secretKey",
    "requestAirdrop",
    "createAssociatedTokenAccount",
    "swap",
    "raydium",
    "jupiter",
)

# file relative path -> (token, justification). Text-only mentions are allowed
# when the surrounding statement is a read-only disclaimer, not an operation.
ALLOWLIST: dict[tuple[str, str], str] = {
    ("scripts/phase2/__init__.py", "swap"):
        "module docstring disclaimer: 'No swaps, private keys, wallets, token "
        "purchase signals, or transaction submission.'",
    ("scripts/phase3/report.py", "swap"):
        "report footer disclaimer: 'No output can be interpreted as a buy "
        "signal. No wallet access, swaps, or orders.'",
}


def python_files() -> list[pathlib.Path]:
    files: list[pathlib.Path] = []
    for directory in SCAN_DIRS:
        base = ROOT / directory
        if not base.is_dir():
            continue
        for path in base.rglob("*"):
            if path.is_file() and path.suffix in SCAN_SUFFIXES:
                files.append(path)
    return sorted(files)


class ReadOnlyGuardTests(unittest.TestCase):
    def test_no_signing_or_sending_constructs(self):
        findings: list[str] = []
        for path in python_files():
            rel = path.relative_to(ROOT).as_posix()
            text = path.read_text(encoding="utf-8", errors="replace")
            for line_number, line in enumerate(text.splitlines(), start=1):
                for token in FORBIDDEN:
                    if token not in line:
                        continue
                    if (rel, token) in ALLOWLIST:
                        continue
                    findings.append(f"{rel}:{line_number}: {token} -> {line.strip()}")
        self.assertEqual(findings, [], "write/signing construct found:\n" + "\n".join(findings))

    def test_guard_itself_would_catch_a_violation(self):
        """The scanner must be able to fail; otherwise it is decoration."""
        sample = "client.sendTransaction(signed_tx)"
        caught = any(token in sample for token in FORBIDDEN)
        self.assertTrue(caught, "scanner no longer detects sendTransaction")

    def test_scan_actually_covers_files(self):
        files = python_files()
        self.assertGreater(len(files), 5, f"only scanned {len(files)} files")
        names = {p.name for p in files}
        self.assertIn("phase2_cli.py", names)
        self.assertIn("redact.py", names)

    def test_only_https_endpoints_are_constructed(self):
        """No plain http:// endpoint may be hardcoded in pipeline code."""
        offenders: list[str] = []
        pattern = re.compile(r"http://(?!localhost|127\.0\.0\.1|example\.invalid)")
        for path in python_files():
            rel = path.relative_to(ROOT).as_posix()
            for line_number, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), start=1):
                if pattern.search(line):
                    offenders.append(f"{rel}:{line_number}: {line.strip()}")
        self.assertEqual(offenders, [], "non-HTTPS endpoint found:\n" + "\n".join(offenders))


if __name__ == "__main__":
    unittest.main()