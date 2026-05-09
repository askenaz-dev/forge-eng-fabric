#!/usr/bin/env python3
"""Apply per-Workspace Langfuse retention via the Langfuse API.

Reads the retention windows from `docs/governance/data-retention.md` and POSTs
them per Workspace via Langfuse's Project Settings → Retention API.

Run on demand or as a CronJob via the `retention-jobs` chart. Idempotent:
re-applying with the same values is a no-op.

Required env:
  LANGFUSE_HOST            (e.g., https://langfuse.<env>.forge.local)
  LANGFUSE_ADMIN_TOKEN     admin-scope token

Optional env:
  CLASSIFICATION           internal | confidential   (default confidential)
  TIER                     staging | prod            (default prod)
"""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request

# Hardcoded retention table mirroring docs/governance/data-retention.md.
# Update in lockstep — the CI check at scripts/check-retention-policy.py will
# (in a future iteration) cover Langfuse as well.
RETENTION_DAYS = {
    ("internal", "staging"): 30,
    ("internal", "prod"): 90,
    ("confidential", "staging"): 30,
    ("confidential", "prod"): 90,
}


def main() -> int:
    host = os.environ.get("LANGFUSE_HOST")
    token = os.environ.get("LANGFUSE_ADMIN_TOKEN")
    if not host or not token:
        print("ERROR: LANGFUSE_HOST and LANGFUSE_ADMIN_TOKEN are required", file=sys.stderr)
        return 2
    classification = os.environ.get("CLASSIFICATION", "confidential")
    tier = os.environ.get("TIER", "prod")

    days = RETENTION_DAYS.get((classification, tier))
    if days is None:
        print(f"ERROR: no retention defined for ({classification}, {tier})", file=sys.stderr)
        return 2

    # List projects (Workspaces); Langfuse models them as "projects".
    req = urllib.request.Request(
        f"{host}/api/public/projects",
        headers={"authorization": f"Bearer {token}"},
    )
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            projects = json.loads(resp.read().decode("utf-8"))
    except urllib.error.URLError as exc:
        print(f"ERROR: could not list projects: {exc}", file=sys.stderr)
        return 2

    applied = 0
    for project in projects:
        project_id = project["id"]
        body = json.dumps({"retentionDays": days}).encode("utf-8")
        req = urllib.request.Request(
            f"{host}/api/public/projects/{project_id}/retention",
            method="PUT",
            data=body,
            headers={
                "authorization": f"Bearer {token}",
                "content-type": "application/json",
            },
        )
        try:
            urllib.request.urlopen(req, timeout=15).read()
            applied += 1
            print(f"applied retention={days}d to project {project_id}")
        except urllib.error.HTTPError as exc:
            print(f"WARN: project {project_id} returned {exc.code}: {exc.read().decode('utf-8', 'replace')}")
    print(f"OK: retention applied to {applied}/{len(projects)} projects")
    return 0


if __name__ == "__main__":
    sys.exit(main())
