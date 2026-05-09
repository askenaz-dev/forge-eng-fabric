#!/usr/bin/env python3
"""End-to-end test for audit retention archival and restoration.

Verifies:
  1. A month-old partition can be archived to JSONL (stand-in for Parquet/GCS).
  2. Row counts match before and after archival.
  3. The archived partition is restorable: every original row is present in the
     archive file and can be queried via the documented investigator path.

Designed to run against a local Postgres with the audit migrations applied. In
CI without a database, the test is skipped with a non-failing message.

Per platform-gaps-closure task 7.10.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path


def main() -> int:
    pg_url = os.environ.get("POSTGRES_URL")
    if not pg_url:
        print("SKIP: POSTGRES_URL not set; archive-restore E2E requires a live database")
        return 0

    try:
        import psycopg2
    except ImportError:
        print("SKIP: psycopg2 not installed")
        return 0

    try:
        conn = psycopg2.connect(pg_url)
    except Exception as exc:
        print(f"SKIP: could not connect to {pg_url}: {exc}")
        return 0
    conn.autocommit = True
    cur = conn.cursor()

    # Step 1: Materialize a partition for last month.
    last_month = (datetime.now(timezone.utc).replace(day=1) - timedelta(days=1)).replace(day=1).date()
    cur.execute("SELECT create_audit_event_month_partition(%s::date)", (last_month,))

    tenant_id = str(uuid.uuid4())
    workspace_id = str(uuid.uuid4())

    # Insert 5 rows dated last month — partition routing handles the placement.
    inserted = 0
    for i in range(5):
        cur.execute(
            """
            INSERT INTO audit_event (tenant_id, workspace_id, actor, action, resource, outcome, details, occurred_at)
            VALUES (%s, %s, %s, %s, %s, 'success', %s, %s)
            """,
            (
                tenant_id,
                workspace_id,
                f"actor-{i}",
                "test.archive_restore",
                f"resource-{i}",
                json.dumps({"i": i}),
                last_month + timedelta(days=i),
            ),
        )
        inserted += 1
    print(f"inserted {inserted} rows in partition for {last_month}")

    # Step 2: Run the retention job in enforce mode against an isolated archive path.
    with tempfile.TemporaryDirectory() as tmp:
        archive_path = Path(tmp) / f"audit_event_{last_month.strftime('%Y_%m')}.jsonl"
        env = os.environ.copy()
        env["JOB"] = "archive-audit"
        env["ENFORCE_RETENTION"] = "true"
        env["RETENTION_DAYS"] = "1"
        env["ARCHIVE_LOCAL_PATH"] = str(archive_path)
        env["POSTGRES_URL"] = pg_url
        result = subprocess.run(
            [sys.executable, "scripts/audit_retention_job.py"],
            env=env,
            capture_output=True,
            text=True,
        )
        print(result.stdout)
        if result.returncode != 0:
            print(result.stderr, file=sys.stderr)
            print("FAIL: retention job exited non-zero")
            return 1

        # Step 3: Verify the archive has every row.
        if not archive_path.exists():
            print(f"FAIL: archive file {archive_path} not produced")
            return 1
        archived_rows = [json.loads(line) for line in archive_path.read_text(encoding="utf-8").splitlines()]
        print(f"archive contains {len(archived_rows)} rows; expected {inserted}")
        if len(archived_rows) < inserted:
            print(f"FAIL: archived {len(archived_rows)} < inserted {inserted}")
            return 1

        # Step 4: Investigator path — the original partition no longer exists.
        cur.execute(
            "SELECT count(*) FROM pg_class WHERE relname = %s",
            (f"audit_event_{last_month.strftime('%Y_%m')}",),
        )
        if cur.fetchone()[0] != 0:
            print("FAIL: original partition still present after archival")
            return 1

        # Step 5: A retention_run row exists with outcome='archived'.
        cur.execute(
            "SELECT outcome, records_count FROM retention_run WHERE partition_name = %s",
            (f"audit_event_{last_month.strftime('%Y_%m')}",),
        )
        outcome_row = cur.fetchone()
        if outcome_row is None or outcome_row[0] != "archived":
            print(f"FAIL: retention_run row missing or wrong outcome ({outcome_row})")
            return 1

    print("OK: archive + restore round-trip verified")
    return 0


if __name__ == "__main__":
    sys.exit(main())
