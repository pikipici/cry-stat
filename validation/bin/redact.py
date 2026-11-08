"""Secret redaction for logs, reports, and archived evidence.

Every byte written to validation/logs, validation/raw, or validation/reports goes
through redact_secrets() first. API keys must never reach disk, a report, or a
screenshot. This module is deliberately dependency-free so it can be imported by
any script in validation/bin without pulling in the phase code.
"""
from __future__ import annotations

import re
from typing import Any
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

REDACTED = "[REDACTED]"

# Query parameter names whose values are always secrets, regardless of length.
SENSITIVE_QUERY_KEYS = frozenset({
    "api_key", "apikey", "api-key", "key", "token", "access_token",
    "auth", "authorization", "secret", "password", "passwd", "pwd",
    "signature", "sig", "session", "sessionid", "jwt", "bearer",
    "x-api-key", "auth_token", "refresh_token",
})

# Header names whose values are always secrets.
SENSITIVE_HEADERS = frozenset({
    "authorization", "x-api-key", "api-key", "x-auth-token",
    "cookie", "set-cookie", "proxy-authorization",
})

# Value shapes that look like credentials even when the key name is innocuous.
_VALUE_PATTERNS = (
    re.compile(r"\bBearer\s+[A-Za-z0-9._\-+/=]{8,}", re.IGNORECASE),
    re.compile(r"\beyJ[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}"),  # JWT
    re.compile(r"\bsk-[A-Za-z0-9_\-]{16,}"),                                       # OpenAI-style (incl. sk-proj-)
    re.compile(r"\bghp_[A-Za-z0-9]{20,}"),                                         # GitHub PAT
    re.compile(r"\bxox[baprs]-[A-Za-z0-9\-]{10,}"),                                # Slack
    re.compile(r"\b[A-Fa-f0-9]{64}\b"),                                            # bare 64-hex (private key / hash)
)

# Matches "Bearer <token>" so the scheme word survives while the token does not.
_BEARER = re.compile(r"(?i)\b(Bearer)(\s+)([A-Za-z0-9._\-+/=]{8,})")

# key=value and key: value assignments with a sensitive key name.
_ASSIGNMENT = re.compile(
    r"(?i)\b(" + "|".join(sorted(SENSITIVE_QUERY_KEYS)) + r")\b(\s*[:=]\s*)(\"[^\"]*\"|'[^']*'|[^\s,;&)\]}]+)"
)

# Solana secret keys are 64-byte arrays rendered as base58 or JSON arrays.
_SOLANA_JSON_KEY = re.compile(r"\[\s*(?:\d{1,3}\s*,\s*){63}\d{1,3}\s*\]")


def redact_url(url: str) -> str:
    """Redact credentials in the userinfo and the query string of a URL."""
    try:
        parts = urlsplit(url)
    except ValueError:
        return REDACTED
    netloc = parts.netloc
    if "@" in netloc:
        userinfo, _, host = netloc.rpartition("@")
        if ":" in userinfo:
            user, _ = userinfo.split(":", 1)
            netloc = f"{user}:{REDACTED}@{host}"
        else:
            netloc = f"{REDACTED}@{host}"
    query = parts.query
    if query:
        redacted_pairs = []
        for key, value in parse_qsl(query, keep_blank_values=True):
            if key.lower() in SENSITIVE_QUERY_KEYS and value:
                redacted_pairs.append((key, REDACTED))
            else:
                redacted_pairs.append((key, value))
        query = urlencode(redacted_pairs, safe="[]")
    return urlunsplit((parts.scheme, netloc, parts.path, query, parts.fragment))


def redact_text(text: str) -> str:
    """Redact secrets from free-form text such as logs or stack traces."""
    if not text:
        return text
    out = text
    # Bearer first: the assignment rule below would otherwise eat the colon and
    # leave the token exposed.
    out = _BEARER.sub(lambda m: f"{m.group(1)}{m.group(2)}{REDACTED}", out)
    out = _ASSIGNMENT.sub(lambda m: f"{m.group(1)}{m.group(2)}{REDACTED}", out)
    for pattern in _VALUE_PATTERNS:
        out = pattern.sub(REDACTED, out)
    out = _SOLANA_JSON_KEY.sub(REDACTED, out)
    # Any https URL embedded in prose gets its query/userinfo scrubbed.
    out = re.sub(r"https?://[^\s\"'<>]+", lambda m: redact_url(m.group(0)), out)
    return out


def redact_headers(headers: dict[str, str]) -> dict[str, str]:
    """Return a copy of headers with sensitive values replaced."""
    return {
        k: (REDACTED if k.lower() in SENSITIVE_HEADERS else v)
        for k, v in headers.items()
    }


def redact_secrets(value: Any) -> Any:
    """Recursively redact secrets from dicts, lists, strings, and scalars.

    Safe to call on any JSON-decoded structure before writing it to disk.
    """
    if isinstance(value, str):
        return redact_text(value)
    if isinstance(value, dict):
        out = {}
        for key, item in value.items():
            if isinstance(key, str) and key.lower().replace("-", "_") in SENSITIVE_QUERY_KEYS:
                out[key] = REDACTED
            else:
                out[key] = redact_secrets(item)
        return out
    if isinstance(value, (list, tuple)):
        return [redact_secrets(item) for item in value]
    return value


def assert_no_secrets(payload: str, *, forbidden: tuple[str, ...] = ()) -> None:
    """Raise if payload still contains a known secret or an obvious credential.

    Used as a last line of defence immediately before writing evidence to disk.
    """
    for secret in forbidden:
        if secret and secret in payload:
            raise ValueError("evidence still contains a known secret value")
    for pattern in _VALUE_PATTERNS:
        if pattern.search(payload):
            raise ValueError("evidence still contains a credential-shaped value")
    lowered = payload.lower()
    for key in SENSITIVE_QUERY_KEYS:
        if f"{key}=" in lowered and REDACTED.lower() not in lowered:
            raise ValueError(f"evidence still contains an unredacted {key}= parameter")