"""Field redaction for decision logs and audit emission.

Sensitive keys (case-insensitive substring match) are replaced with `***REDACTED***`.
This is intentionally conservative — better to over-redact than to leak.
"""

from __future__ import annotations

from typing import Any

REDACT_SUBSTRINGS = (
    "password",
    "passwd",
    "secret",
    "token",
    "api_key",
    "apikey",
    "private_key",
    "authorization",
    "auth_token",
    "client_secret",
    "ssn",
    "credit_card",
    "card_number",
)

REDACTED = "***REDACTED***"


def redact(obj: Any) -> Any:
    if isinstance(obj, dict):
        out: dict[str, Any] = {}
        for k, v in obj.items():
            kl = str(k).lower()
            if any(s in kl for s in REDACT_SUBSTRINGS):
                out[k] = REDACTED
            else:
                out[k] = redact(v)
        return out
    if isinstance(obj, list):
        return [redact(v) for v in obj]
    return obj
