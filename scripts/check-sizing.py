#!/usr/bin/env python3
"""Verify that umbrella Helm tier values match the sizing tables in docs/platform-enablement.md.

Exits non-zero if a divergence is detected. Designed for CI but runnable locally:

    python scripts/check-sizing.py

Strategy: parse the "Per-service Kubernetes requests/limits (Staging defaults)" table from
the enablement doc, then verify every entry has a corresponding service block in the Staging
umbrella values file with matching `replicas`, `resources.requests` and `resources.limits`.

This is the initial implementation referenced by task 1.5 of the `platform-gaps-closure`
change. It is intentionally permissive: missing umbrella values entries are reported but only
fail when a value is present and divergent. Once the umbrella chart lands, flip the
ALLOW_MISSING flag to False to require full coverage.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import yaml


REPO_ROOT = Path(__file__).resolve().parent.parent
DOC_PATH = REPO_ROOT / "docs" / "platform-enablement.md"
UMBRELLA_VALUES = REPO_ROOT / "infra" / "helm" / "forge-platform" / "values-staging.yaml"

ALLOW_MISSING = True

TABLE_HEADER_RE = re.compile(
    r"^\| Service \| Replicas \| CPU req \| CPU limit \| Mem req \| Mem limit \|"
)
ROW_RE = re.compile(
    r"^\| `(?P<service>[a-z0-9\-]+)`(?:\s*\([a-z]+\))?\s*"
    r"\| (?P<replicas>\d+)\s*"
    r"\| (?P<cpu_req>[0-9]+m)\s*"
    r"\| (?P<cpu_lim>[0-9]+m)\s*"
    r"\| (?P<mem_req>[0-9]+(?:Mi|Gi))\s*"
    r"\| (?P<mem_lim>[0-9]+(?:Mi|Gi))\s*\|$"
)


def parse_doc_sizing() -> dict[str, dict[str, str]]:
    """Return a dict keyed by service name with replicas + requests/limits expectations."""
    if not DOC_PATH.exists():
        print(f"ERROR: enablement doc not found at {DOC_PATH}", file=sys.stderr)
        sys.exit(2)

    expected: dict[str, dict[str, str]] = {}
    in_table = False
    with DOC_PATH.open(encoding="utf-8") as fh:
        for line in fh:
            line = line.rstrip()
            if TABLE_HEADER_RE.match(line):
                in_table = True
                continue
            if in_table:
                if not line.startswith("| `"):
                    if line.startswith("|---"):
                        continue
                    in_table = False
                    continue
                m = ROW_RE.match(line)
                if not m:
                    continue
                expected[m.group("service")] = {
                    "replicas": int(m.group("replicas")),
                    "cpu_req": m.group("cpu_req"),
                    "cpu_lim": m.group("cpu_lim"),
                    "mem_req": m.group("mem_req"),
                    "mem_lim": m.group("mem_lim"),
                }
    return expected


def load_umbrella_values() -> dict | None:
    if not UMBRELLA_VALUES.exists():
        return None
    with UMBRELLA_VALUES.open(encoding="utf-8") as fh:
        return yaml.safe_load(fh) or {}


def check(expected: dict[str, dict], values: dict | None) -> list[str]:
    if values is None:
        if ALLOW_MISSING:
            print(f"WARN: umbrella values file not found at {UMBRELLA_VALUES} — skipping comparison")
            return []
        return [f"umbrella values file missing at {UMBRELLA_VALUES}"]

    errors: list[str] = []
    for service, exp in expected.items():
        block = values.get(service)
        if block is None:
            if ALLOW_MISSING:
                continue
            errors.append(f"{service}: missing entry in umbrella values")
            continue
        replicas = block.get("replicaCount") or block.get("replicas")
        if replicas is not None and replicas != exp["replicas"]:
            errors.append(
                f"{service}: replicas {replicas} != doc {exp['replicas']}"
            )
        resources = block.get("resources", {})
        actual_cpu_req = resources.get("requests", {}).get("cpu")
        actual_cpu_lim = resources.get("limits", {}).get("cpu")
        actual_mem_req = resources.get("requests", {}).get("memory")
        actual_mem_lim = resources.get("limits", {}).get("memory")
        for key, doc_key, actual in (
            ("cpu request", "cpu_req", actual_cpu_req),
            ("cpu limit", "cpu_lim", actual_cpu_lim),
            ("mem request", "mem_req", actual_mem_req),
            ("mem limit", "mem_lim", actual_mem_lim),
        ):
            if actual is not None and actual != exp[doc_key]:
                errors.append(
                    f"{service}: {key} {actual} != doc {exp[doc_key]}"
                )
    return errors


def main() -> int:
    expected = parse_doc_sizing()
    if not expected:
        print("ERROR: could not parse any sizing rows from the enablement doc", file=sys.stderr)
        return 2
    print(f"Parsed {len(expected)} service sizing rows from {DOC_PATH.name}")
    values = load_umbrella_values()
    errors = check(expected, values)
    if errors:
        print("Sizing divergence detected:")
        for err in errors:
            print(f"  - {err}")
        return 1
    print("OK: umbrella tier values match the sizing document (or umbrella not yet authored).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
