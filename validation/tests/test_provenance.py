"""Tests for validation/bin/provenance.py — P0 tasks 5 and 7.

Covers atomic writes, content-addressed raw evidence, append-only manifests, and
the fact that the run envelope always carries the required provenance fields.
"""
from __future__ import annotations

import json
import pathlib
import sys
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "validation" / "bin"))

import provenance as prov  # noqa: E402


class RunIdentityTests(unittest.TestCase):
    def test_run_id_is_unique(self):
        ids = {prov.new_run_id() for _ in range(50)}
        self.assertEqual(len(ids), 50)

    def test_run_id_is_url_and_filename_safe(self):
        run_id = prov.new_run_id("pre5 test/bad prefix")
        for ch in run_id:
            self.assertTrue(ch.isalnum() or ch in "-_", f"unsafe char {ch!r} in {run_id}")

    def test_utc_timestamp_format(self):
        stamp = prov.utc_now()
        self.assertTrue(stamp.endswith("Z"))
        self.assertRegex(stamp, r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$")

    def test_code_version_is_stable(self):
        self.assertEqual(prov.code_version(), prov.code_version())
        self.assertIn("meme-token-skill@", prov.code_version())

    def test_config_version_matches_locked_file(self):
        version, digest = prov.config_version()
        self.assertEqual(version, "thresholds-v1.0.0")
        self.assertEqual(len(digest), 64)


class EnvelopeTests(unittest.TestCase):
    def test_envelope_has_every_required_field(self):
        envelope = prov.evidence_envelope("run-x", synthetic=False, stage="p1", payload={"ok": True})
        for field in (
            "run_id", "stage", "observed_at_utc", "synthetic",
            "code_version", "config_version", "config_sha256", "environment", "payload",
        ):
            self.assertIn(field, envelope)
        self.assertFalse(envelope["synthetic"])
        self.assertTrue(envelope["observed_at_utc"].endswith("Z"))

    def test_environment_captures_python_and_platform(self):
        env = prov.environment()
        for field in ("python", "platform", "code_version", "config_version", "archive_sha256"):
            self.assertIn(field, env)
            self.assertNotEqual(env[field], "")


class AtomicWriteTests(unittest.TestCase):
    def test_write_and_read_back(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / "nested" / "report.json"
            prov.atomic_write_json(path, {"a": 1})
            self.assertEqual(json.loads(path.read_text(encoding="utf-8")), {"a": 1})

    def test_no_temp_files_left_behind(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / "x.json"
            prov.atomic_write_json(path, {"a": 1})
            leftovers = [p.name for p in pathlib.Path(tmp).iterdir() if p.name.endswith(".tmp")]
            self.assertEqual(leftovers, [])

    def test_overwrite_replaces_content_atomically(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / "x.json"
            prov.atomic_write_json(path, {"v": 1})
            prov.atomic_write_json(path, {"v": 2})
            self.assertEqual(json.loads(path.read_text(encoding="utf-8"))["v"], 2)


class ContentAddressedTests(unittest.TestCase):
    def test_same_content_same_filename(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = pathlib.Path(tmp)
            a = prov.content_addressed_path(directory, {"k": "v"})
            b = prov.content_addressed_path(directory, {"k": "v"})
            self.assertEqual(a, b)

    def test_different_content_different_filename(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = pathlib.Path(tmp)
            a = prov.content_addressed_path(directory, {"k": 1})
            b = prov.content_addressed_path(directory, {"k": 2})
            self.assertNotEqual(a, b)

    def test_filename_is_the_sha256_of_the_bytes(self):
        import hashlib
        with tempfile.TemporaryDirectory() as tmp:
            directory = pathlib.Path(tmp)
            data = {"mint": "So11111111111111111111111111111111111111112", "synthetic": False}
            path = prov.content_addressed_path(directory, data)
            blob = json.dumps(data, indent=2, sort_keys=True, ensure_ascii=False).encode("utf-8") + b"\n"
            self.assertEqual(path.stem, hashlib.sha256(blob).hexdigest())


class AppendOnlyManifestTests(unittest.TestCase):
    def test_lines_accumulate_and_never_rewrite(self):
        with tempfile.TemporaryDirectory() as tmp:
            manifest = pathlib.Path(tmp) / "manifests" / "evidence.jsonl"
            prov.append_manifest_line(manifest, {"seq": 1})
            prov.append_manifest_line(manifest, {"seq": 2})
            prov.append_manifest_line(manifest, {"seq": 3})
            lines = [json.loads(line) for line in manifest.read_text(encoding="utf-8").splitlines()]
            self.assertEqual([line["seq"] for line in lines], [1, 2, 3])

    def test_records_stay_sorted_and_parseable(self):
        with tempfile.TemporaryDirectory() as tmp:
            manifest = pathlib.Path(tmp) / "m.jsonl"
            prov.append_manifest_line(manifest, {"b": 2, "a": 1})
            raw = manifest.read_text(encoding="utf-8").strip()
            self.assertEqual(raw, '{"a": 1, "b": 2}')


if __name__ == "__main__":
    unittest.main()