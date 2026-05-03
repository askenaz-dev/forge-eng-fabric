from __future__ import annotations

import hashlib
import hmac


def provenance_signature(*, secret: str, source_ref: str, text: str) -> str:
    payload = f"{source_ref}\n{text}".encode()
    return hmac.new(secret.encode("utf-8"), payload, hashlib.sha256).hexdigest()


def verify_provenance(*, secret: str, source_ref: str, text: str, signature: str | None) -> bool:
    if not signature:
        return False
    expected = provenance_signature(secret=secret, source_ref=source_ref, text=text)
    return hmac.compare_digest(expected, signature)
