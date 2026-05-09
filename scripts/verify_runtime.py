#!/usr/bin/env python3
"""Call the runtime-registry verify endpoint and render a human-readable summary.

Usage:
    python scripts/verify_runtime.py --runtime <runtime_id> [--workspace <ws>] [--hint key=value ...]
    make verify-runtime RUNTIME=<runtime_id> WORKSPACE=<ws>

Exits non-zero if any check returns `fail`. `warn` is reported but does not fail.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.request


def parse_hints(pairs: list[str]) -> dict:
    out: dict = {}
    for pair in pairs:
        if "=" not in pair:
            print(f"WARN: ignoring hint {pair!r} — expected key=value", file=sys.stderr)
            continue
        k, v = pair.split("=", 1)
        if v.lower() in {"true", "false"}:
            out[k] = v.lower() == "true"
        else:
            out[k] = v
    return out


def color(s: str, code: int) -> str:
    if not sys.stdout.isatty():
        return s
    return f"\033[{code}m{s}\033[0m"


STATUS_FORMAT = {
    "pass": lambda s: color(s, 32),
    "fail": lambda s: color(s, 31),
    "warn": lambda s: color(s, 33),
    "skip": lambda s: color(s, 90),
}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--runtime", required=False, default=os.environ.get("RUNTIME"))
    parser.add_argument("--workspace", required=False, default=os.environ.get("WORKSPACE"))
    parser.add_argument(
        "--base-url",
        default=os.environ.get("RUNTIME_REGISTRY_URL", "http://localhost:8110"),
        help="runtime-registry base URL",
    )
    parser.add_argument("--principal", default=os.environ.get("USER", "make-verify-runtime"))
    parser.add_argument(
        "--hint",
        action="append",
        default=[],
        help="per-check hint as key=value (e.g., kubeconfig_summary='namespace=apps,sa=forge')",
    )
    args = parser.parse_args()

    if not args.runtime:
        print("ERROR: --runtime is required (or set RUNTIME=<id>)", file=sys.stderr)
        return 2

    hints = parse_hints(args.hint)
    body = json.dumps(hints).encode("utf-8") if hints else b""
    url = f"{args.base_url}/v1/runtimes/{args.runtime}/verify"
    req = urllib.request.Request(
        url,
        method="POST",
        data=body,
        headers={
            "content-type": "application/json",
            "x-principal": args.principal,
        },
    )

    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read().decode("utf-8"))
    except urllib.error.URLError as exc:
        print(f"ERROR: could not reach runtime-registry at {url}: {exc}", file=sys.stderr)
        return 2

    overall = data.get("status", "unknown")
    print(f"\nRuntime verification — {data.get('runtime_id', args.runtime)} ({data.get('mode', '?')}, {data.get('type', '?')})")
    print(f"Workspace: {data.get('workspace_id', args.workspace or '-')}")
    print(f"Principal: {data.get('principal', args.principal)}")
    print(f"Started:   {data.get('started_at', '-')}")
    print(f"Ended:     {data.get('ended_at', '-')}")
    print(f"Overall:   {STATUS_FORMAT.get(overall, str)(overall.upper())}")
    print()

    fail_count = 0
    for check in data.get("checks", []):
        status = check.get("status", "?")
        formatter = STATUS_FORMAT.get(status, str)
        print(f"  [{formatter(status.upper().ljust(4))}] {check.get('name')}")
        if check.get("evidence"):
            print(f"           evidence: {check['evidence']}")
        if status in {"fail", "warn"} and check.get("remediation"):
            print(f"           remediation: {check['remediation']}")
        if status == "fail":
            fail_count += 1

    print()
    if fail_count:
        print(f"FAIL: {fail_count} check(s) failed.")
        return 1
    if overall == "warn":
        print("OK with warnings — review remediation hints above.")
    else:
        print("OK: all checks passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
