"""Run identity, provenance, and atomic evidence writing — P0 tasks 5 and 7.

Every validation artifact must carry a run_id, UTC timestamps, code/config
version, and a synthetic flag. This module centralises that so no script
invents its own convention.
"""
from __future__ import annotations

import hashlib
import json
import os
import platform
import secrets
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[2]
VALIDATION = ROOT / "validation"
CONFIG_PATH = VALIDATION / "config.example.json"
SKILL_VERSION = "1.1.0"
CODE_VERSION_FILE = ROOT / "BUILD_INFO.md"


def utc_now() -> str:
    """Second-resolution UTC timestamp, always with a Z suffix."""
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def new_run_id(prefix: str | None = None) -> str:
    """Short, sortable, collision-resistant run identifier."""
    if prefix is None:
        prefix = os.environ.get("RUN_ID_PREFIX") or "pre5"
    prefix = "".join(ch for ch in prefix if ch.isalnum() or ch in "-_") or "pre5"
    stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    return f"{prefix}-{stamp}-{secrets.token_hex(3)}"


def code_version() -> str:
    """Identify the code under test by hashing BUILD_INFO.md plus the skill tag."""
    try:
        digest = hashlib.sha256(CODE_VERSION_FILE.read_bytes()).hexdigest()
    except OSError:
        digest = "unknown"
    return f"meme-token-skill@{SKILL_VERSION}+build={digest[:16]}"


def config_version() -> tuple[str, str]:
    """Return (config_version, sha256) of the locked threshold file."""
    try:
        raw = CONFIG_PATH.read_bytes()
    except OSError:
        return "unavailable", "unknown"
    digest = hashlib.sha256(raw).hexdigest()
    try:
        version = json.loads(raw.decode("utf-8")).get("config_version", "unknown")
    except (UnicodeError, json.JSONDecodeError):
        version = "unparsable"
    return str(version), digest


def archive_sha256() -> str:
    """sha256 of the source archive recorded in manifests, if present."""
    candidates = sorted(ROOT.parent.glob("meme-token-skill-v*.zip"))
    if not candidates:
        return "not-present-in-workspace"
    return hashlib.sha256(candidates[0].read_bytes()).hexdigest()


def environment() -> dict[str, Any]:
    """Environment fingerprint for ENVIRONMENT_AND_REPRODUCTION.md."""
    cfg_version, cfg_sha = config_version()
    return {
        "python": sys.version.split()[0],
        "python_implementation": platform.python_implementation(),
        "platform": platform.platform(),
        "system": platform.system(),
        "machine": platform.machine(),
        "code_version": code_version(),
        "config_version": cfg_version,
        "config_sha256": cfg_sha,
        "archive_sha256": archive_sha256(),
        "cwd": str(Path.cwd()),
    }


def evidence_envelope(run_id: str, *, synthetic: bool, stage: str, payload: dict[str, Any]) -> dict[str, Any]:
    """Standard wrapper every evidence record carries."""
    cfg_version, cfg_sha = config_version()
    return {
        "run_id": run_id,
        "stage": stage,
        "observed_at_utc": utc_now(),
        "synthetic": synthetic,
        "code_version": code_version(),
        "config_version": cfg_version,
        "config_sha256": cfg_sha,
        "environment": environment(),
        "payload": payload,
    }


def atomic_write_json(path: Path, data: Any) -> Path:
    """Write JSON atomically: temp file in the same directory, then rename.

    A crash mid-write must never leave a half-written evidence file that looks
    complete. os.replace is atomic on both POSIX and Windows.
    """
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, tmp_name = tempfile.mkstemp(dir=str(path.parent), prefix=path.name + ".", suffix=".tmp")
    try:
        with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as handle:
            json.dump(data, handle, indent=2, sort_keys=True, ensure_ascii=False)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(tmp_name, path)
    except BaseException:
        try:
            os.unlink(tmp_name)
        except OSError:
            pass
        raise
    return path


def content_addressed_path(directory: Path, data: Any, *, suffix: str = ".json") -> Path:
    """Raw evidence is content-addressed: the filename IS the sha256 of the bytes.

    Rewriting a record therefore creates a new file instead of mutating history,
    which makes the archive append-only by construction.
    """
    blob = json.dumps(data, indent=2, sort_keys=True, ensure_ascii=False).encode("utf-8") + b"\n"
    digest = hashlib.sha256(blob).hexdigest()
    return Path(directory) / f"{digest}{suffix}"


def append_manifest_line(manifest_path: Path, record: dict[str, Any]) -> Path:
    """Append one JSONL provenance record. Never rewrites existing lines."""
    manifest_path = Path(manifest_path)
    manifest_path.parent.mkdir(parents=True, exist_ok=True)
    with manifest_path.open("a", encoding="utf-8", newline="\n") as handle:
        handle.write(json.dumps(record, sort_keys=True, ensure_ascii=False) + "\n")
        handle.flush()
        os.fsync(handle.fileno())
    return manifest_path