"""Tests for validation/bin/redact.py — P0 task 6.

Every case here is a secret that must never reach validation/logs or reports.
Run with: python -m unittest discover -s validation/tests -v
"""
from __future__ import annotations

import pathlib
import sys
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "validation" / "bin"))

from redact import (  # noqa: E402
    REDACTED,
    assert_no_secrets,
    redact_headers,
    redact_secrets,
    redact_text,
    redact_url,
)


class RedactUrlTests(unittest.TestCase):
    def test_query_api_key_is_redacted(self):
        url = "https://mainnet.helius-rpc.com/?api-key=deadbeefcafe1234"
        self.assertNotIn("deadbeefcafe1234", redact_url(url))
        self.assertIn(REDACTED, redact_url(url))

    def test_multiple_sensitive_query_keys_are_redacted(self):
        url = "https://rpc.example.com/path?api_key=abc123&token=xyz789&cluster=mainnet"
        out = redact_url(url)
        self.assertNotIn("abc123", out)
        self.assertNotIn("xyz789", out)
        self.assertIn("cluster=mainnet", out)

    def test_userinfo_password_is_redacted_but_user_kept(self):
        url = "https://research:hunter2@rpc.example.com/v1"
        out = redact_url(url)
        self.assertNotIn("hunter2", out)
        self.assertIn("research", out)

    def test_nonsecret_query_survives(self):
        url = "https://api.dexscreener.com/token-pairs/v1/solana/So11111111111111111111111111111111111111112"
        self.assertEqual(redact_url(url), url)


class RedactTextTests(unittest.TestCase):
    def test_assignment_forms(self):
        for text in (
            "SOLANA_RPC_URL=https://rpc.example.com/?api-key=abc123def456",
            "api_key: abc123def456",
            "api_key='abc123def456'",
            'token="abc123def456"',
        ):
            with self.subTest(text=text):
                self.assertNotIn("abc123def456", redact_text(text))

    def test_bearer_token(self):
        out = redact_text("Authorization: Bearer eyJhbGciOiJIUzI1NiJ9abc.def.ghi")
        self.assertNotIn("eyJhbGciOiJIUzI1NiJ9abc", out)

    def test_jwt_is_redacted(self):
        jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
        self.assertNotIn(jwt, redact_text(f"token {jwt} leaked"))

    def test_64_hex_private_key_is_redacted(self):
        key = "a" * 64
        self.assertNotIn(key, redact_text(f"secret key material: {key}"))

    def test_solana_json_keypair_array_is_redacted(self):
        array = "[" + ",".join(str(i % 256) for i in range(64)) + "]"
        self.assertNotIn(array, redact_text(f"keypair={array}"))

    def test_prefixed_provider_keys(self):
        for token in ("sk-proj-abcdefghijklmnopqrstuv", "ghp_abcdefghijklmnopqrstuvwx", "xoxb-123456789012-abcdefghij"):
            with self.subTest(token=token):
                self.assertNotIn(token, redact_text(f"found {token}"))

    def test_url_inside_prose_is_scrubbed(self):
        text = "call https://rpc.example.com/?api-key=supersecretvalue now"
        self.assertNotIn("supersecretvalue", redact_text(text))

    def test_ordinary_prose_is_untouched(self):
        text = "research run completed with 3 candidates and 42 request calls"
        self.assertEqual(redact_text(text), text)


class RedactStructuredTests(unittest.TestCase):
    def test_dict_key_named_api_key_is_redacted(self):
        out = redact_secrets({"api_key": "abcdef123456", "cluster": "mainnet"})
        self.assertEqual(out["api_key"], REDACTED)
        self.assertEqual(out["cluster"], "mainnet")

    def test_nested_structures(self):
        out = redact_secrets({
            "providers": [
                {"name": "solana", "url": "https://rpc.x/?api-key=leakme123"},
                {"name": "dexscreener", "headers": {"Authorization": "Bearer leakmetoken"}},
            ],
            "notes": "password=hunter2failed",
        })
        flat = repr(out)
        for secret in ("leakme123", "leakmetoken", "hunter2failed"):
            self.assertNotIn(secret, flat)

    def test_headers_helper(self):
        out = redact_headers({"Authorization": "Bearer abc", "Accept": "application/json"})
        self.assertEqual(out["Authorization"], REDACTED)
        self.assertEqual(out["Accept"], "application/json")

    def test_scalars_pass_through(self):
        for value in (1, 1.5, True, None):
            self.assertEqual(redact_secrets(value), value)


class AssertNoSecretsTests(unittest.TestCase):
    def test_raises_on_known_secret(self):
        with self.assertRaises(ValueError):
            assert_no_secrets("config used sk_TEST123", forbidden=("sk_TEST123",))

    def test_raises_on_credential_shape(self):
        with self.assertRaises(ValueError):
            assert_no_secrets("Authorization: Bearer abcdefghijklmnop")

    def test_passes_on_redacted_payload(self):
        assert_no_secrets("url=https://rpc.example.com/?api-key=[REDACTED]")

    def test_unredacted_query_key_is_caught(self):
        with self.assertRaises(ValueError):
            assert_no_secrets("see https://rpc.example.com/?api_key=abc")


if __name__ == "__main__":
    unittest.main()