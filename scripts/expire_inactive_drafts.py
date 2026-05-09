#!/usr/bin/env python3
"""Expire OpenSpec drafts inactive for >= 14 days.

Calls the openspec service's `POST /v1/intent/expire-inactive` endpoint, which
runs the in-process expiry pass and emits `intent.draft.abandoned.v1` per draft
flipped. Designed to run as a CronJob in the `retention-jobs` chart.

Audit trail: emitted events flow through the standard openspec EventPublisher
to the audit pipeline. Per data-retention.md, abandoned drafts retain their
audit rows even after the draft body is purged.
"""

from __future__ import annotations

import json
import os
import sys
import urllib.request


def main() -> int:
    base = os.environ.get("OPENSPEC_URL", "http://localhost:8084")
    req = urllib.request.Request(
        f"{base}/v1/intent/expire-inactive",
        method="POST",
        headers={"content-type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            body = json.loads(resp.read().decode("utf-8"))
    except Exception as exc:
        print(f"ERROR: could not call openspec service at {base}: {exc}", file=sys.stderr)
        return 2
    print(json.dumps(body))
    return 0


if __name__ == "__main__":
    sys.exit(main())
