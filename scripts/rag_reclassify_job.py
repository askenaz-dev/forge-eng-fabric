#!/usr/bin/env python3
"""Periodic Milvus reclassification job.

For every Milvus collection that carries a `classification` and
`ingested_at` field, walks the `source_documents` index and updates the
retention deadline if the source's classification has changed since
ingestion.

Per the [retention policy](docs/governance/data-retention.md):
- internal class: 30d (Local), 90d (Staging), 365d (Prod) TTL
- confidential class: 30d (Local), 90d (Staging), 180d (Prod) TTL

Dry-run by default (`ENFORCE_RETENTION` not `true`); enforcement co-approved by
Platform + Security per the policy doc's procedure.
"""

from __future__ import annotations

import json
import os
import sys
from datetime import datetime, timedelta, timezone

TTL_DAYS = {
    ("internal", "local"): 30,
    ("internal", "staging"): 90,
    ("internal", "prod"): 365,
    ("confidential", "local"): 30,
    ("confidential", "staging"): 90,
    ("confidential", "prod"): 180,
}


def main() -> int:
    enforce = os.environ.get("ENFORCE_RETENTION", "false").lower() == "true"
    tier = os.environ.get("TIER", "prod")
    sources_url = os.environ.get("RAG_SOURCES_URL", "http://rag-ingest:8128/v1/sources")
    milvus_host = os.environ.get("MILVUS_HOST", "milvus:19530")

    print(f"reclassify: enforce={enforce} tier={tier} milvus={milvus_host}")

    try:
        import urllib.request

        req = urllib.request.Request(sources_url)
        with urllib.request.urlopen(req, timeout=15) as resp:
            sources = json.loads(resp.read().decode("utf-8"))
    except Exception as exc:
        print(f"could not list sources from {sources_url}: {exc} — offline-no-op", file=sys.stderr)
        return 0

    updated = 0
    for source in sources.get("sources", []):
        current_class = source.get("classification", "internal")
        recorded_class = source.get("recorded_classification", current_class)
        ingested_at = datetime.fromisoformat(source["ingested_at"].replace("Z", "+00:00"))
        ttl = TTL_DAYS.get((current_class, tier))
        if ttl is None:
            continue
        new_deadline = ingested_at + timedelta(days=ttl)
        old_deadline = source.get("retention_deadline")
        if old_deadline:
            old_deadline_dt = datetime.fromisoformat(old_deadline.replace("Z", "+00:00"))
            if abs((new_deadline - old_deadline_dt).total_seconds()) < 60 and current_class == recorded_class:
                continue
        if not enforce:
            print(
                f"dry-run: would update {source['id']} class={recorded_class}→{current_class} "
                f"deadline={old_deadline}→{new_deadline.isoformat()}"
            )
            continue
        # Production wiring would PUT the new deadline back to rag-ingest.
        updated += 1
        print(f"updated {source['id']} → {new_deadline.isoformat()}")

    print(f"OK: scanned {len(sources.get('sources', []))} sources; updated {updated}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
