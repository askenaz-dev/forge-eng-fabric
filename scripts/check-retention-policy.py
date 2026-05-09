#!/usr/bin/env python3
"""Verify Loki/Tempo/Prometheus retention values match the policy document.

Fails the build if `infra/helm/observability/values-{staging,prod}.yaml` diverge
from the retention table in `docs/governance/data-retention.md` for these data
types.

The check is permissive about the policy doc's exact format (Markdown tables
can drift). It walks the retention matrix table, extracts the per-tier days
for `Loki`, `Tempo`, and `Prometheus`, and asserts a strict match.

Run via `make retention-policy-check` or directly:
    python scripts/check-retention-policy.py
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parent.parent
POLICY = REPO / "docs" / "governance" / "data-retention.md"
HELM_OBS = REPO / "infra" / "helm" / "observability"


# Mapping from Helm key to the row prefix in the policy table.
HELM_TO_POLICY = {
    ("loki", "internal_days"): {"row": "Platform logs (Loki)", "class": "internal"},
    ("loki", "confidential_days"): {"row": "Platform logs (Loki)", "class": "confidential"},
    ("tempo", "internal_days"): {"row": "Platform traces (Tempo)", "class": "internal"},
    ("tempo", "confidential_days"): {"row": "Platform traces (Tempo)", "class": "confidential"},
    ("prometheus", "internal_days"): {"row": "Platform metrics (Prometheus / Mimir)", "class": "internal"},
}

DAYS_RE = re.compile(r"(\d+)\s*d", re.IGNORECASE)


def parse_policy() -> dict[tuple[str, str], dict[str, int]]:
    """Return {(row_prefix, class): {tier: days}} parsed from the matrix table."""
    out: dict[tuple[str, str], dict[str, int]] = {}
    if not POLICY.exists():
        print(f"ERROR: policy doc not found at {POLICY}", file=sys.stderr)
        sys.exit(2)
    in_matrix = False
    for line in POLICY.read_text(encoding="utf-8").splitlines():
        if line.startswith("| Data type "):
            in_matrix = True
            continue
        if not in_matrix:
            continue
        if not line.startswith("|"):
            in_matrix = False
            continue
        parts = [p.strip() for p in line.strip().strip("|").split("|")]
        if len(parts) < 7 or parts[0] in ("---", "Data type"):
            continue
        row = parts[0]
        cls = parts[1].strip("`")
        # parts[2..]: Local, Staging operational, Staging archive, Prod operational, Prod archive, ...
        try:
            staging = parts[3]
            prod = parts[5]
        except IndexError:
            continue
        tiers: dict[str, int] = {}
        m = DAYS_RE.search(staging)
        if m:
            tiers["staging"] = int(m.group(1))
        m = DAYS_RE.search(prod)
        if m:
            tiers["prod"] = int(m.group(1))
        if tiers:
            out[(row, cls)] = tiers
    return out


def load_helm(tier: str) -> dict:
    path = HELM_OBS / f"values-{tier}.yaml"
    if not path.exists():
        print(f"WARN: {path} missing — skipping {tier}", file=sys.stderr)
        return {}
    return yaml.safe_load(path.read_text(encoding="utf-8")) or {}


def main() -> int:
    policy = parse_policy()
    if not policy:
        print("ERROR: could not parse retention matrix from policy doc", file=sys.stderr)
        return 2
    print(f"parsed {len(policy)} retention rows from {POLICY.name}")

    errors: list[str] = []
    for tier in ("staging", "prod"):
        values = load_helm(tier)
        if not values:
            continue
        for (svc, key), spec in HELM_TO_POLICY.items():
            block = values.get(svc) or {}
            retention = block.get("retention") or {}
            actual = retention.get(key)
            if actual is None:
                errors.append(f"[{tier}] {svc}.{key} missing in values-{tier}.yaml")
                continue
            policy_row = policy.get((spec["row"], spec["class"])) or {}
            expected = policy_row.get(tier)
            if expected is None:
                errors.append(f"[{tier}] {spec['row']}/{spec['class']} not found in policy")
                continue
            if int(actual) != int(expected):
                errors.append(
                    f"[{tier}] {svc}.{key} = {actual} but policy says {expected} for {spec['row']}/{spec['class']}"
                )

    if errors:
        print("Retention divergence detected:")
        for e in errors:
            print(f"  - {e}")
        return 1
    print("OK: Loki/Tempo/Prometheus retention values match the policy document.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
