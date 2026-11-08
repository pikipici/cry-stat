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
SCAN_SUFFIXES = {".py"}

# The skill source tree may live either at the repo root (standalone skill repo)
# or inside an extraction workspace (this Go project). Resolve whichever exists
# so the guard scans real pipeline code and never silently scans nothing.
SKILL_ROOTS = (ROOT, ROOT / "validation_work" / "meme-token-skill")


def skill_root() -> pathlib.Path:
    for candidate in SKILL_ROOTS:
        if (candidate / "scripts" / "phase2" / "http.py").is_file():
            return candidate
    return ROOT


def scan_dirs() -> tuple[str, ...]:
    base = skill_root()
    dirs = []
    if (base / "scripts").is_dir():
        dirs.append(str((base / "scripts").relative_to(ROOT)) if base != ROOT else "scripts")
    if (ROOT / "validation" / "bin").is_dir():
        dirs.append("validation/bin")
    return tuple(dirs)

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
    for directory in scan_dirs():
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
        base = skill_root()
        for path in python_files():
            # Allowlist keys are relative to the skill tree, so the guard reports
            # the same key whether the skill sits at the repo root or in a
            # validation_work extraction directory.
            try:
                rel = path.relative_to(base).as_posix()
            except ValueError:
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
        self.assertGreater(len(files), 3, f"only scanned {len(files)} files: {[p.name for p in files]}")
        names = {p.name for p in files}
        self.assertIn("redact.py", names)
        self.assertIn("provenance.py", names)
        if (skill_root() / "scripts" / "phase2_cli.py").is_file():
            self.assertIn("phase2_cli.py", names)

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